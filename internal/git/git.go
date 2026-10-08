package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RevParse(ref string) (string, error) {
	cmd := exec.Command("git", "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func GetGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	gitDir := strings.TrimSpace(string(out))
	return filepath.Abs(gitDir)
}

func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

func GetChangedFiles() ([]string, error) {
	tracked, err := exec.Command("git", "diff", "--name-only", "HEAD").Output()
	if err != nil {
		return nil, err
	}

	untracked, err := exec.Command("git", "ls-files", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, err
	}

	var files []string
	if len(tracked) > 0 {
		files = append(files, strings.Split(strings.TrimSpace(string(tracked)), "\n")...)
	}
	if len(untracked) > 0 {
		untrackedList := strings.Split(strings.TrimSpace(string(untracked)), "\n")
		for _, f := range untrackedList {
			if f != "" {
				files = append(files, f)
			}
		}
	}

	return files, nil
}

func GetFileContent(path string, ref string) (string, error) {
	cmd := exec.Command("git", "show", ref+":"+path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func GetWorkingTreeContent(path string) (string, error) {
	cmd := exec.Command("git", "show", ":"+path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func Commit(args []string) error {
	cmd := exec.Command("git", append([]string{"commit"}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func GetLastCommit() (string, error) {
	return RevParse("HEAD")
}

func AddNote(ref, commit, message string) error {
	cmd := exec.Command("git", "notes", "--ref", ref, "add", "-f", "-m", message, commit)
	return cmd.Run()
}

func GetNote(ref, commit string) (string, error) {
	cmd := exec.Command("git", "notes", "--ref", ref, "show", commit)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func GetCommitFiles(commit string) ([]string, error) {
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", commit)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n"), nil
}

func Blame(file string) ([]BlameLine, error) {
	cmd := exec.Command("git", "blame", "--porcelain", file)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseBlame(string(out)), nil
}

type BlameLine struct {
	Commit  string
	LineNum int
	Content string
}

func parseBlame(output string) []BlameLine {
	var result []BlameLine
	lines := strings.Split(output, "\n")
	var currentCommit string
	var lineNum int

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "\t") {
			result = append(result, BlameLine{
				Commit:  currentCommit,
				LineNum: lineNum,
				Content: strings.TrimPrefix(line, "\t"),
			})
		} else {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if len(parts[0]) == 40 {
					currentCommit = parts[0]
					_, _ = fmt.Sscanf(parts[2], "%d", &lineNum)
				}
			}
		}
	}

	return result
}

func CommitLog(args []string) ([]CommitInfo, error) {
	cmdArgs := append([]string{"log", "--pretty=format:%H\t%s\t%an\t%at"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 4 {
			commits = append(commits, CommitInfo{
				Hash:    parts[0],
				Subject: parts[1],
				Author:  parts[2],
				Date:    parts[3],
			})
		}
	}

	return commits, nil
}

type CommitInfo struct {
	Hash    string
	Subject string
	Author  string
	Date    string
}

func ConfigSet(key, value string) error {
	cmd := exec.Command("git", "config", key, value)
	return cmd.Run()
}

func ConfigAdd(key, value string) error {
	cmd := exec.Command("git", "config", "--add", key, value)
	return cmd.Run()
}

func ConfigGet(key string) (string, error) {
	cmd := exec.Command("git", "config", "--get", key)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func ShowFile(commit, path string) ([]byte, error) {
	cmd := exec.Command("git", "show", commit+":"+path)
	return cmd.Output()
}

func DiffFile(oldCommit, newCommit, path string) (string, error) {
	var cmd *exec.Cmd
	if oldCommit == "" {
		cmd = exec.Command("git", "show", newCommit+":"+path)
	} else {
		cmd = exec.Command("git", "diff", oldCommit, newCommit, "--", path)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func GetFileAtCommit(commit, path string) (string, error) {
	cmd := exec.Command("git", "show", commit+":"+path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func GetParentCommit(commit string) (string, error) {
	cmd := exec.Command("git", "rev-parse", commit+"^")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func DiffWithParent(commit, path string) (added []int, removed []int, err error) {
	parent, err := GetParentCommit(commit)
	if err != nil {
		cmd := exec.Command("git", "show", commit, "--", path)
		out, _ := cmd.Output()
		return parseUnifiedDiff(string(out))
	}

	cmd := exec.Command("git", "diff", parent, commit, "--", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}

	return parseUnifiedDiff(string(out))
}

func parseUnifiedDiff(diff string) (added []int, removed []int, err error) {
	lines := strings.Split(diff, "\n")
	newLineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				rangeStr := parts[2]
				rangeStr = strings.TrimPrefix(rangeStr, "+")
				var start int
				_, _ = fmt.Sscanf(rangeStr, "%d", &start)
				newLineNum = start - 1
			}
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			newLineNum++
			added = append(added, newLineNum)
		} else if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			newLineNum++
		}
	}

	return added, removed, nil
}

func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

func GetShowPrefix() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-prefix")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func GetDiffAddedLines(commit, path string) (map[int]bool, error) {
	parent, err := GetParentCommit(commit)
	var cmd *exec.Cmd
	isRootCommit := err != nil

	if isRootCommit {
		cmd = exec.Command("git", "show", commit+":"+path)
	} else {
		cmd = exec.Command("git", "diff", "-U0", parent, commit, "--", path)
	}

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	added := make(map[int]bool)

	if isRootCommit {
		content := strings.TrimSuffix(string(out), "\n")
		lines := strings.Split(content, "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = []string{}
		}
		for i := range lines {
			added[i+1] = true
		}
		return added, nil
	}

	lines := strings.Split(string(out), "\n")
	var lineNum int

	for _, line := range lines {
		if strings.HasPrefix(line, "\\") {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				rangeStr := parts[2]
				rangeStr = strings.TrimPrefix(rangeStr, "+")
				var start, count int
				if strings.Contains(rangeStr, ",") {
					_, _ = fmt.Sscanf(rangeStr, "%d,%d", &start, &count)
				} else {
					_, _ = fmt.Sscanf(rangeStr, "%d", &start)
					count = 1
				}
				lineNum = start - 1
			}
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			lineNum++
			added[lineNum] = true
		} else if !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") && !strings.HasPrefix(line, "@@") {
			lineNum++
		}
	}

	return added, nil
}
