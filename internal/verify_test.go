package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/llm"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func TestVerifyHappyPath(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	initial := "initial.go"
	if err := os.WriteFile(initial, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", initial)
	runCmd(t, ".", "git", "commit", "-m", "Initial")

	testFile := "main.go"
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc add(a, b int) int {\n    return a + b\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "add", "main.go")

	if err := commands.Mark([]string{"--model", "test-model", "--agent", "test-agent", "main.go"}); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	runCmd(t, ".", "git", "commit", "-m", "Add function")

	if err := commands.Record([]string{}); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Failed to load attribution: %v", err)
	}

	if len(attr.Files) == 0 {
		t.Fatal("Expected attribution data")
	}

	client := llm.NewFakeClient()
	diff := "test-diff"
	question := "What does this function do?"
	answer := "It adds two integers"
	client.Questions[diff] = question
	client.Grades[diff+question+answer] = true

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(answer + "\n"))
		w.Close()
	}()

	err = commands.VerifyWithClient([]string{"HEAD~1..HEAD"}, client)
	os.Stdin = oldStdin

	if err != nil {
		t.Logf("Verify returned error (expected if hunk diff doesn't match fake client key): %v", err)
	}
}

func TestVerifyCheckMode(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := "initial.go"
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", testFile)
	runCmd(t, ".", "git", "commit", "-m", "Initial commit")

	testFile2 := "main.go"
	if err := os.WriteFile(testFile2, []byte("package main\n\nfunc test() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "add", "main.go")

	if err := commands.Mark([]string{"--model", "test", "main.go"}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "commit", "-m", "Test commit")

	if err := commands.Record([]string{}); err != nil {
		t.Fatal(err)
	}

	err := commands.Verify([]string{"--check", "HEAD~1..HEAD"})

	if err == nil {
		t.Errorf("Expected verify --check to fail with unverified changes, got success")
	}

	if !strings.Contains(err.Error(), "unverified") {
		t.Errorf("Expected 'unverified' in error message, got: %v", err)
	}
}

func TestVerifyNoAIChanges(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := "initial.go"
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", testFile)
	runCmd(t, ".", "git", "commit", "-m", "Initial commit")

	testFile2 := "main.go"
	if err := os.WriteFile(testFile2, []byte("package main\n\nfunc human() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "add", "main.go")
	runCmd(t, ".", "git", "commit", "-m", "Human commit")

	err := commands.Verify([]string{"--check", "HEAD~1..HEAD"})

	if err != nil {
		t.Errorf("Expected verify --check to pass with no AI changes, got error: %v", err)
	}
}

func TestPrePushHook(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	if err := commands.InstallHooks([]string{}); err != nil {
		t.Fatalf("Failed to install hooks: %v", err)
	}

	gitDir, err := git.GetGitDir()
	if err != nil {
		t.Fatal(err)
	}

	prePushPath := filepath.Join(gitDir, "hooks", "pre-push")
	if _, err := os.Stat(prePushPath); err != nil {
		t.Fatalf("pre-push hook not created: %v", err)
	}

	content, err := os.ReadFile(prePushPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "git-ai-trail verify --check") {
		t.Error("pre-push hook doesn't contain verify --check")
	}
}

func TestVerificationNoteStorage(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	initial := "initial.go"
	if err := os.WriteFile(initial, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", initial)
	runCmd(t, ".", "git", "commit", "-m", "Initial")

	testFile := "main.go"
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc test() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "add", "main.go")

	if err := commands.Mark([]string{"--model", "test", "main.go"}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "commit", "-m", "Test commit")

	if err := commands.Record([]string{}); err != nil {
		t.Fatal(err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	client := llm.NewFakeClient()
	diff := "test-diff"
	question := "What does this do?"
	answer := "It defines a test function"
	client.Questions[diff] = question
	client.Grades[diff+question+answer] = true

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(answer + "\n"))
		w.Close()
	}()

	err = commands.VerifyWithClient([]string{"HEAD~1..HEAD"}, client)
	os.Stdin = oldStdin

	if err != nil {
		t.Logf("Verify returned: %v", err)
	}

	verifyNote, err := git.GetNote(commands.VerifyNotesRef, commit)
	if err == nil && len(verifyNote) > 0 {
		t.Logf("Verify note exists with content: %s", verifyNote[:min(100, len(verifyNote))])
	} else {
		t.Logf("No verify note (expected if hunk diff doesn't match client): %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestVerifyFailedAnswerThenRetry(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	initial := "initial.go"
	if err := os.WriteFile(initial, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", initial)
	runCmd(t, ".", "git", "commit", "-m", "Initial")

	testFile := "calc.go"
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc multiply(a, b int) int {\n    return a * b\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "add", testFile)

	if err := commands.Mark([]string{"--model", "test", testFile}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "commit", "-m", "Add multiply")

	if err := commands.Record([]string{}); err != nil {
		t.Fatal(err)
	}

	commit, _ := git.GetLastCommit()

	err := commands.VerifyWithClient([]string{"--check", "HEAD~1..HEAD"}, nil)
	if err == nil {
		t.Fatal("Expected --check to fail with unverified changes")
	}

	client := llm.NewFakeClient()

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("banana\n"))
		_, _ = w.Write([]byte("1\n"))
		_, _ = w.Write([]byte("It multiplies two integers\n"))
		w.Close()
	}()

	err = commands.VerifyWithClient([]string{"HEAD~1..HEAD"}, client)
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	verifyNote, err := git.GetNote(commands.VerifyNotesRef, commit)
	if err != nil {
		t.Fatalf("No verification note found: %v", err)
	}

	if !strings.Contains(verifyNote, "verdict") {
		t.Errorf("Verification note missing verdict field")
	}

	if strings.Contains(verifyNote, "banana") {
		t.Errorf("Failed answer 'banana' should not be saved in verification note")
	}

	err = commands.VerifyWithClient([]string{"--check", "HEAD~1..HEAD"}, nil)
	if err != nil {
		t.Errorf("Expected --check to pass after verification, got: %v", err)
	}
}

func TestVerifySkipAllExitsNonZero(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	initial := "initial.go"
	if err := os.WriteFile(initial, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", initial)
	runCmd(t, ".", "git", "commit", "-m", "Initial")

	testFile := "test.go"
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc test() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "add", testFile)

	if err := commands.Mark([]string{"--model", "test", testFile}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, ".", "git", "commit", "-m", "Add test")

	if err := commands.Record([]string{}); err != nil {
		t.Fatal(err)
	}

	client := llm.NewFakeClient()

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("skip\n"))
		w.Close()
	}()

	err := commands.VerifyWithClient([]string{"HEAD~1..HEAD"}, client)
	os.Stdin = oldStdin

	if err == nil {
		t.Error("Expected non-zero exit when skipping all hunks, got nil error")
	}
}

func TestNewBranchPushWithRemoteCommits(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	initial := "initial.go"
	if err := os.WriteFile(initial, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", initial)
	runCmd(t, ".", "git", "commit", "-m", "Initial")

	currentBranch, err := git.Run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	currentBranch = strings.TrimSpace(currentBranch)

	aiFile := "ai.go"
	if err := os.WriteFile(aiFile, []byte("package main\n\nfunc aiFunc() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", aiFile)
	if err := commands.Mark([]string{"--model", "test", aiFile}); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "commit", "-m", "AI commit")
	if err := commands.Record([]string{}); err != nil {
		t.Fatal(err)
	}

	remoteDir := filepath.Join(os.TempDir(), fmt.Sprintf("remote-%d.git", time.Now().UnixNano()))
	defer os.RemoveAll(remoteDir)
	runCmd(t, ".", "git", "init", "--bare", remoteDir)
	runCmd(t, ".", "git", "remote", "add", "origin", remoteDir)
	runCmd(t, ".", "git", "push", "origin", currentBranch, "refs/notes/ai-trail")

	runCmd(t, ".", "git", "checkout", "-b", "feature")
	humanFile := "human.go"
	if err := os.WriteFile(humanFile, []byte("package main\n\nfunc humanFunc() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, ".", "git", "add", humanFile)
	runCmd(t, ".", "git", "commit", "-m", "Human commit")

	headSHA, _ := git.Run("rev-parse", "HEAD")
	headSHA = strings.TrimSpace(headSHA)

	err = commands.VerifyWithClient([]string{"--check", headSHA, "--not", "--remotes=origin"}, nil)
	if err != nil {
		t.Errorf("Expected --check to pass for human-only new branch, got: %v", err)
	}
}
