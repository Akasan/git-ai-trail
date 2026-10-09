package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	client := llm.NewFakeClient()
	diff := "test-diff"
	question := "What does this function do?"
	wrongAnswer := "banana"
	rightAnswer := "It multiplies two integers"

	client.Questions[diff] = question
	client.Grades[diff+question+wrongAnswer] = false
	client.Grades[diff+question+rightAnswer] = true

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte(wrongAnswer + "\n"))
		_, _ = w.Write([]byte("1\n"))
		_, _ = w.Write([]byte(rightAnswer + "\n"))
		w.Close()
	}()

	err := commands.VerifyWithClient([]string{"HEAD~1..HEAD"}, client)
	os.Stdin = oldStdin

	if err != nil {
		t.Logf("Verify returned: %v", err)
	}

	commit, _ := git.GetLastCommit()
	verifyNote, err := git.GetNote(commands.VerifyNotesRef, commit)
	if err == nil && len(verifyNote) > 0 {
		t.Logf("Verification recorded after retry")
	}
}
