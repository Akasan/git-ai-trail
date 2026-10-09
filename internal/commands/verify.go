package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	check := fs.Bool("check", false, "Check mode: non-interactive, exit non-zero if unverified AI changes exist")
	if err := fs.Parse(args); err != nil {
		return err
	}

	revRange := "HEAD"
	if fs.NArg() > 0 {
		revRange = fs.Arg(0)
	} else {
		upstream, err := getUpstreamBranch()
		if err == nil && upstream != "" {
			revRange = upstream + "..HEAD"
		} else {
			mainBranch := findMainBranch()
			if mainBranch != "" {
				revRange = mainBranch + "..HEAD"
			}
		}
	}

	commits, err := getCommitsInRange(revRange)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		if *check {
			return nil
		}
		fmt.Println("No commits to verify")
		return nil
	}

	hunks, err := collectUnverifiedHunks(commits)
	if err != nil {
		return err
	}

	if len(hunks) == 0 {
		if *check {
			return nil
		}
		fmt.Println("No unverified AI-attributed changes found")
		return nil
	}

	if *check {
		fmt.Fprintf(os.Stderr, "Found %d unverified AI-attributed hunk(s):\n", len(hunks))
		for _, h := range hunks {
			fmt.Fprintf(os.Stderr, "  %s (%s:%d-%d)\n", h.CommitHash[:7], h.FilePath, h.StartLine, h.EndLine)
		}
		os.Exit(1)
	}

	apiKey, err := llm.GetAPIKey()
	if err != nil {
		return err
	}

	model, err := git.ConfigGet("ai-trail.verifyModel")
	if err != nil || model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	client := llm.NewAnthropicClient(apiKey, model)

	verifier, err := getVerifierIdentity()
	if err != nil {
		return err
	}

	fmt.Printf("Found %d unverified AI-attributed hunk(s)\n\n", len(hunks))

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
			reader := bufio.NewReader(os.Stdin)
			answer, err = reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read answer: %w", err)
			}

			answer = strings.TrimSpace(answer)

			if answer == "quit" {
				fmt.Println("Exiting verification")
				return nil
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

	fmt.Println("Verification complete")
	return nil
}

func getCommitsInRange(revRange string) ([]string, error) {
	output, err := git.Run("rev-list", revRange)
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

func collectUnverifiedHunks(commits []string) ([]Hunk, error) {
	var hunks []Hunk

	for _, commit := range commits {
		attr, err := notes.Load(commit)
		if err != nil {
			continue
		}

		verifyRecord, _ := loadVerificationRecord(commit)

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

				hunks = append(hunks, hunk)
			}
		}
	}

	return hunks, nil
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
		return nil, err
	}

	var record VerificationRecord
	if err := json.Unmarshal([]byte(data), &record); err != nil {
		return nil, err
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
