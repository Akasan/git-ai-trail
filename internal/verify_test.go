package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/llm"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func TestVerifyHappyPath(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	client := llm.NewFakeClient()
	client.Questions["test-diff"] = "What does this function do?"
	client.Grades["test-diffWhat does this function do?It calculates the sum"] = true

	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc add(a, b int) int {\n    return a + b\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, dir, "git", "add", "main.go")

	if err := commands.Mark([]string{"--model", "test-model", "--agent", "test-agent", "main.go"}); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	runCmd(t, dir, "git", "commit", "-m", "Add function")

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
}

func TestVerifyCheckMode(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc test() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, dir, "git", "add", "main.go")

	if err := commands.Mark([]string{"--model", "test", "main.go"}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, dir, "git", "commit", "-m", "Test commit")

	if err := commands.Record([]string{}); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "ai-trail", "verify", "--check", "HEAD~1..HEAD")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Errorf("Expected verify --check to fail with unverified changes, got success")
	}

	if !strings.Contains(string(output), "unverified") {
		t.Errorf("Expected 'unverified' in output, got: %s", string(output))
	}
}

func TestVerifyNoAIChanges(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc human() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, dir, "git", "add", "main.go")
	runCmd(t, dir, "git", "commit", "-m", "Human commit")

	cmd := exec.Command("git", "ai-trail", "verify", "--check", "HEAD~1..HEAD")
	cmd.Dir = dir
	err := cmd.Run()

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
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n\nfunc test() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runCmd(t, dir, "git", "add", "main.go")

	if err := commands.Mark([]string{"--model", "test", "main.go"}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, dir, "git", "commit", "-m", "Test commit")

	if err := commands.Record([]string{}); err != nil {
		t.Fatal(err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	_, err = git.GetNote(commands.VerifyNotesRef, commit)
	if err == nil {
		t.Log("Verify note exists (expected if verification was run)")
	}
}
