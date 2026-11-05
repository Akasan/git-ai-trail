package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/Akasan/git-ai-trail/internal/config"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/llm"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

const VerifyNotesRef = "refs/notes/ai-trail-verify"

type VerificationRecord struct {
	SchemaVersion int                       `json:"schema_version"`
	Verifications map[string]HunkVerifyInfo `json:"verifications"`
}

type HunkVerifyInfo struct {
	HunkID    string `json:"hunk_id"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Verdict   bool   `json:"verdict"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
	Verifier  string `json:"verifier"`
	Model     string `json:"model"`
}

type Hunk struct {
	CommitHash string
	FilePath   string
	StartLine  int
	EndLine    int
	Lines      []string
	Ranges     []notes.LineRange
}

func (h Hunk) ID() string {
	return fmt.Sprintf("%s:%s:%d-%d", h.CommitHash, h.FilePath, h.StartLine, h.EndLine)
}

func (h Hunk) Diff() string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("--- %s\n", h.FilePath))
	buf.WriteString(fmt.Sprintf("+++ %s\n", h.FilePath))
	buf.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.StartLine, len(h.Lines), h.StartLine, len(h.Lines)))
	for _, line := range h.Lines {
		buf.WriteString("+" + line + "\n")
	}
	return buf.String()
}

func Verify(args []string) error {
	return VerifyWithClient(args, nil)
}

func VerifyWithClient(args []string, client llm.Client) error {
	var checkFlag bool
	var language string
	var excludes []string
	var revArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--check":
			checkFlag = true
		case arg == "--lang":
			if i+1 >= len(args) {
				return fmt.Errorf("--lang requires a value")
			}
			i++
			language = args[i]
		case strings.HasPrefix(arg, "--lang="):
			language = strings.TrimPrefix(arg, "--lang=")
		case arg == "--exclude":
			if i+1 >= len(args) {
				return fmt.Errorf("--exclude requires a pattern")
			}
			i++
			excludes = append(excludes, args[i])
		case strings.HasPrefix(arg, "--exclude="):
			excludes = append(excludes, strings.TrimPrefix(arg, "--exclude="))
		default:
			revArgs = append(revArgs, arg)
		}
	}

	if language == "" {
		language = config.GetVerifyLanguage()
	}

	excludes = append(excludes, config.GetVerifyExcludes()...)
	excludeMatchers, err := compileExcludes(excludes)
	if err != nil {
		return err
	}

	if len(revArgs) == 0 {
		upstream, err := getUpstreamBranch()
		if err == nil && upstream != "" {
			revArgs = []string{upstream + "..HEAD"}
		} else {
			mainBranch := findMainBranch()
			if mainBranch != "" {
				revArgs = []string{mainBranch + "..HEAD"}
			} else {
				fmt.Fprintf(os.Stderr, "Warning: no upstream or main branch found, checking all commits on HEAD\n")
				revArgs = []string{"HEAD", "--not", "--remotes"}
				commits, err := getCommitsInRange(revArgs...)
				if err == nil && len(commits) == 0 {
					revArgs = []string{"HEAD"}
				}
			}
		}
	}

	commits, err := getCommitsInRange(revArgs...)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		if checkFlag {
			return nil
		}
		
		currentBranch, _ := git.Run("rev-parse", "--abbrev-ref", "HEAD")
		currentBranch = strings.TrimSpace(currentBranch)
		
		if currentBranch == "HEAD" {
			fmt.Fprintf(os.Stderr, "Warning: on detached HEAD with no unverified commits in specified range\n")
		} else if (currentBranch == "main" || currentBranch == "master") && len(revArgs) > 0 && strings.Contains(strings.Join(revArgs, " "), "..HEAD") {
			fmt.Fprintf(os.Stderr, "Warning: on '%s' branch with no commits to verify in range\n", currentBranch)
			fmt.Fprintf(os.Stderr, "If you have no remote/upstream, use: git ai-trail verify HEAD --not --remotes\n")
		}
		fmt.Println("No commits to verify")
		return nil
	}

	hunks, excludedCount, err := collectUnverifiedHunks(commits, excludeMatchers)
	if err != nil {
		return err
	}

	if excludedCount > 0 {
		fmt.Fprintf(os.Stderr, "Excluded %d unverified AI-attributed hunk(s) matching exclude patterns\n", excludedCount)
	}

	if len(hunks) == 0 {
		if checkFlag {
			return nil
		}
		fmt.Println("No unverified AI-attributed changes found")
		return nil
	}

	if checkFlag {
		fmt.Fprintf(os.Stderr, "Found %d unverified AI-attributed hunk(s):\n", len(hunks))
		for _, h := range hunks {
			fmt.Fprintf(os.Stderr, "  %s (%s:%d-%d)\n", h.CommitHash[:7], h.FilePath, h.StartLine, h.EndLine)
		}
		return fmt.Errorf("unverified AI-attributed changes found")
	}

	var provider, model, baseURL string
	if client == nil {
		provider, model = config.ResolveProviderAndModel()
		if provider == "" || model == "" {
			return fmt.Errorf("provider and model configuration conflict: verify.provider implies one provider but verify.model implies another. Please ensure they are consistent")
		}
		
		baseURL = config.GetVerifyBaseURL()
		
		if err := config.ValidateBaseURL(baseURL, provider); err != nil {
			return err
		}
		
		fmt.Fprintf(os.Stderr, "Using provider: %s, model: %s\n", provider, model)
		if baseURL != "" {
			defaultBaseURLs := map[string]string{
				"anthropic": "https://api.anthropic.com/v1",
				"openai":    "https://api.openai.com/v1",
				"xai":       "https://api.x.ai/v1",
			}
			if baseURL != defaultBaseURLs[provider] {
				u, _ := url.Parse(baseURL)
				fmt.Fprintf(os.Stderr, "Using custom baseURL: %s\n", u.Host)
			}
		}
		
		var err error
		client, err = llm.NewClient(provider, model, baseURL, language)
		if err != nil {
			return err
		}
	} else {
		provider, model = config.ResolveProviderAndModel()
		if provider == "" || model == "" {
			provider = "anthropic"
			model = config.GetDefaultModel(provider)
		}
		fmt.Fprintf(os.Stderr, "Using provider: %s, model: %s\n", provider, model)
	}

	verifier, err := getVerifierIdentity()
	if err != nil {
		return err
	}

	fmt.Printf("Using provider: %s, model: %s\n", provider, model)
	fmt.Printf("Question language: %s\n", language)
	fmt.Printf("Found %d unverified AI-attributed hunk(s)\n\n", len(hunks))

	reader := bufio.NewReader(os.Stdin)
	verifiedCount := 0

	for i, hunk := range hunks {
		fmt.Printf("=== Hunk %d/%d ===\n", i+1, len(hunks))
		fmt.Printf("Commit: %s\n", hunk.CommitHash[:7])
		fmt.Printf("File: %s (lines %d-%d)\n\n", hunk.FilePath, hunk.StartLine, hunk.EndLine)

		fmt.Println("Code:")
		for j, line := range hunk.Lines {
			fmt.Printf("  %d: %s\n", hunk.StartLine+j, line)
		}
		fmt.Println()

		fmt.Println("Generating question...")
		question, err := client.GenerateQuestion(hunk.Diff())
		if err != nil {
			return fmt.Errorf("failed to generate question: %w", err)
		}

		fmt.Printf("\nQuestion:\n%s\n\n", question)

		var answer string
		var verdict bool
		var reason string
		for {
			fmt.Print("Your answer (or 'skip' to skip, 'quit' to exit): ")
			answer, err = reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read answer: %w", err)
			}

			answer = strings.TrimSpace(answer)

			if answer == "quit" {
				fmt.Printf("\nExiting verification (%d/%d hunks verified)\n", verifiedCount, len(hunks))
				return fmt.Errorf("verification incomplete: %d unverified hunk(s) remain", len(hunks)-verifiedCount)
			}

			if answer == "skip" {
				fmt.Println("Skipping this hunk")
				break
			}

			if answer == "" {
				fmt.Println("Please provide an answer, or type 'skip' to skip")
				continue
			}

			fmt.Println("\nGrading answer...")
			verdict, reason, err = client.GradeAnswer(hunk.Diff(), question, answer)
			if err != nil {
				return fmt.Errorf("failed to grade answer: %w", err)
			}

			if verdict {
				fmt.Printf("✓ PASS: %s\n\n", reason)
				if err := saveVerification(hunk, question, answer, verdict, reason, verifier, model); err != nil {
					return fmt.Errorf("failed to save verification: %w", err)
				}
				verifiedCount++
				break
			} else {
				fmt.Printf("✗ FAIL: %s\n\n", reason)
				fmt.Println("Would you like to:")
				fmt.Println("  1. Try again")
				fmt.Println("  2. Skip this hunk")
				fmt.Print("Choice (1/2): ")

				choice, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read choice: %w", err)
				}
				choice = strings.TrimSpace(choice)

				if choice == "2" {
					fmt.Println("Skipping this hunk")
					break
				}
			}
		}
	}

	if verifiedCount < len(hunks) {
		fmt.Printf("\nVerification incomplete: %d/%d hunks verified\n", verifiedCount, len(hunks))
		return fmt.Errorf("%d unverified hunk(s) remain", len(hunks)-verifiedCount)
	}

	fmt.Println("\nVerification complete: all hunks verified")
	return nil
}

func getCommitsInRange(revArgs ...string) ([]string, error) {
	cmdArgs := append([]string{"rev-list"}, revArgs...)
	output, err := git.Run(cmdArgs...)
	if err != nil {
		return nil, err
	}

	output = strings.TrimSpace(output)
	if output == "" {
		return []string{}, nil
	}

	commits := strings.Split(output, "\n")
	var result []string
	for _, c := range commits {
		c = strings.TrimSpace(c)
		if c != "" {
			result = append(result, c)
		}
	}

	for i := 0; i < len(result)/2; i++ {
		result[i], result[len(result)-1-i] = result[len(result)-1-i], result[i]
	}

	return result, nil
}

func collectUnverifiedHunks(commits []string, excludes []*regexp.Regexp) ([]Hunk, int, error) {
	var hunks []Hunk
	var excludedCount int
	var hasParseErrors bool

	for _, commit := range commits {
		attr, err := notes.Load(commit)
		if err != nil {
			if strings.Contains(err.Error(), "invalid character") || strings.Contains(err.Error(), "unexpected") {
				fmt.Fprintf(os.Stderr, "Error: failed to parse attribution notes for %s: %v\n", commit[:7], err)
				fmt.Fprintf(os.Stderr, "\nThis is likely caused by squashing commits with `git rebase -i`.\n")
				fmt.Fprintf(os.Stderr, "When squashing, git concatenates notes from multiple commits, creating invalid JSON.\n\n")
				fmt.Fprintf(os.Stderr, "To fix this, re-attribute the AI-generated lines:\n")
				fmt.Fprintf(os.Stderr, "  1. Mark the AI-generated files: git ai-trail mark <files>\n")
				fmt.Fprintf(os.Stderr, "  2. Amend the commit with new notes: git -c notes.rewriteMode=ignore commit --amend --no-edit\n")
				fmt.Fprintf(os.Stderr, "\nAfter fixing, run verification: git ai-trail verify\n")
				fmt.Fprintf(os.Stderr, "\nWARNING: Do NOT delete the attribution notes, as that would allow unverified AI code through.\n")
				return nil, 0, fmt.Errorf("unparseable attribution notes (squash corruption)")
			}
			continue
		}

		verifyRecord, err := loadVerificationRecord(commit)
		if err != nil && err.Error() != "not found" {
			if strings.Contains(err.Error(), "invalid character") || strings.Contains(err.Error(), "unexpected") {
				fmt.Fprintf(os.Stderr, "Error: failed to parse verification notes for %s: %v\n", commit[:7], err)
				fmt.Fprintf(os.Stderr, "This is likely caused by squashing commits.\n")
				fmt.Fprintf(os.Stderr, "To fix, re-verify the commit after fixing attribution notes (see above):\n")
				fmt.Fprintf(os.Stderr, "  git ai-trail verify %s^..%s\n", commit[:7], commit[:7])
				return nil, 0, fmt.Errorf("unparseable verification notes (squash corruption)")
			}
			fmt.Fprintf(os.Stderr, "Warning: failed to parse verification notes for %s: %v\n", commit[:7], err)
			hasParseErrors = true
		}

		for filePath, fileAttr := range attr.Files {
			fileContent, err := git.GetFileAtCommit(commit, filePath)
			if err != nil {
				continue
			}

			fileContent = strings.TrimSuffix(fileContent, "\n")
			lines := strings.Split(fileContent, "\n")
			if len(lines) == 1 && lines[0] == "" {
				lines = []string{}
			}

			aiRanges := []notes.LineRange{}
			for _, r := range fileAttr.Ranges {
				if r.Kind == "ai" || r.Kind == "ai-modified" {
					aiRanges = append(aiRanges, r)
				}
			}

			if len(aiRanges) == 0 {
				continue
			}

			mergedRanges := mergeAdjacentRanges(aiRanges)

			for _, r := range mergedRanges {
				hunk := Hunk{
					CommitHash: commit,
					FilePath:   filePath,
					StartLine:  r.Start,
					EndLine:    r.End,
					Ranges:     []notes.LineRange{r},
				}

				if r.Start > 0 && r.End <= len(lines) {
					hunk.Lines = lines[r.Start-1 : r.End]
				}

				if verifyRecord != nil {
					if _, verified := verifyRecord.Verifications[hunk.ID()]; verified {
						continue
					}
				}

				if isExcluded(filePath, excludes) {
					excludedCount++
					continue
				}

				hunks = append(hunks, hunk)
			}
		}
	}

	if hasParseErrors {
		return hunks, excludedCount, fmt.Errorf("verification notes contain parsing errors (possibly from squash/rebase); please run 'git notes remove refs/notes/ai-trail-verify <commit>' for affected commits and re-verify")
	}

	return hunks, excludedCount, nil
}

// compileExcludes converts gitignore-style patterns into regexps matched
// against repository-relative paths:
//   - a pattern without "/" matches a file or directory name at any depth
//   - a leading "/" anchors the pattern to the repository root
//   - a trailing "/" matches everything under that directory
//   - "*" and "?" do not cross "/", "**" does
func compileExcludes(patterns []string) ([]*regexp.Regexp, error) {
	var res []*regexp.Regexp
	for _, pattern := range patterns {
		p := strings.TrimSpace(pattern)
		if p == "" {
			continue
		}

		dirOnly := strings.HasSuffix(p, "/")
		p = strings.TrimSuffix(p, "/")
		anchored := strings.Contains(p, "/")
		p = strings.TrimPrefix(p, "/")

		var buf strings.Builder
		buf.WriteString("^")
		if !anchored {
			buf.WriteString("(?:.*/)?")
		}
		for i := 0; i < len(p); i++ {
			switch {
			case strings.HasPrefix(p[i:], "**/"):
				buf.WriteString("(?:.*/)?")
				i += 2
			case strings.HasPrefix(p[i:], "**"):
				buf.WriteString(".*")
				i++
			case p[i] == '*':
				buf.WriteString("[^/]*")
			case p[i] == '?':
				buf.WriteString("[^/]")
			default:
				buf.WriteString(regexp.QuoteMeta(p[i : i+1]))
			}
		}
		if dirOnly {
			buf.WriteString("/.*$")
		} else {
			buf.WriteString("(?:/.*)?$")
		}

		re, err := regexp.Compile(buf.String())
		if err != nil {
			return nil, fmt.Errorf("invalid exclude pattern %q: %w", pattern, err)
		}
		res = append(res, re)
	}
	return res, nil
}

func isExcluded(filePath string, excludes []*regexp.Regexp) bool {
	for _, re := range excludes {
		if re.MatchString(filePath) {
			return true
		}
	}
	return false
}

func mergeAdjacentRanges(ranges []notes.LineRange) []notes.LineRange {
	if len(ranges) == 0 {
		return ranges
	}

	sorted := make([]notes.LineRange, len(ranges))
	copy(sorted, ranges)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Start > sorted[j].Start {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	merged := []notes.LineRange{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		last := &merged[len(merged)-1]
		current := sorted[i]

		if current.Start <= last.End+1 {
			if current.End > last.End {
				last.End = current.End
			}
		} else {
			merged = append(merged, current)
		}
	}

	return merged
}

func saveVerification(hunk Hunk, question, answer string, verdict bool, reason, verifier, model string) error {
	record, err := loadVerificationRecord(hunk.CommitHash)
	if err != nil {
		record = &VerificationRecord{
			SchemaVersion: 1,
			Verifications: make(map[string]HunkVerifyInfo),
		}
	}

	record.Verifications[hunk.ID()] = HunkVerifyInfo{
		HunkID:    hunk.ID(),
		Question:  question,
		Answer:    answer,
		Verdict:   verdict,
		Reason:    reason,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Verifier:  verifier,
		Model:     model,
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return git.AddNote(VerifyNotesRef, hunk.CommitHash, string(data))
}

func loadVerificationRecord(commit string) (*VerificationRecord, error) {
	data, err := git.GetNote(VerifyNotesRef, commit)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}

	var record VerificationRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return &record, nil
}

func getVerifierIdentity() (string, error) {
	name, err := git.ConfigGet("user.name")
	if err != nil {
		name = "unknown"
	}

	email, err := git.ConfigGet("user.email")
	if err != nil {
		email = "unknown"
	}

	return fmt.Sprintf("%s <%s>", name, email), nil
}

func getUpstreamBranch() (string, error) {
	currentBranch, err := git.GetCurrentBranch()
	if err != nil {
		return "", err
	}

	upstream, err := git.ConfigGet(fmt.Sprintf("branch.%s.merge", currentBranch))
	if err != nil {
		return "", err
	}

	upstream = strings.TrimPrefix(upstream, "refs/heads/")

	remote, err := git.ConfigGet(fmt.Sprintf("branch.%s.remote", currentBranch))
	if err == nil && remote != "" && remote != "." {
		upstream = remote + "/" + upstream
	}

	return upstream, nil
}

func findMainBranch() string {
	candidates := []string{"origin/main", "origin/master", "main", "master"}
	for _, candidate := range candidates {
		_, err := git.RevParse(candidate)
		if err == nil {
			return candidate
		}
	}
	return ""
}
