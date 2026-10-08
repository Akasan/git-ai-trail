package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
	"github.com/Akasan/git-ai-trail/internal/snapshot"
)

func setupTestRepo(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-test-*")
	if err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	cleanup := func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}

	if err := os.Chdir(tmpDir); err != nil {
		cleanup()
		t.Fatal(err)
	}

	if err := exec.Command("git", "init").Run(); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.name", "Test User").Run(); err != nil {
		cleanup()
		t.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.email", "test@example.com").Run(); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

func TestEndToEndFlow(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.go")
	initialContent := `package main

func main() {
	println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "add", "test.go").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatal(err)
	}

	aiContent := `package main

func main() {
	println("hello")
	println("world")
}
`
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := commands.Mark([]string{"--model", "gpt-4", "--agent", "test", "test.go"})
	if err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	snapshots, err := snapshot.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(snapshots))
	}

	if snapshots[0].Model != "gpt-4" {
		t.Errorf("Expected model gpt-4, got %s", snapshots[0].Model)
	}

	if err := exec.Command("git", "add", "test.go").Run(); err != nil {
		t.Fatal(err)
	}
	err = commands.Commit([]string{"-m", "Add AI code"})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit failed: %v", err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Load attribution failed: %v", err)
	}

	if attr.SchemaVersion != notes.SchemaVersion {
		t.Errorf("Expected schema version %d, got %d", notes.SchemaVersion, attr.SchemaVersion)
	}

	if len(attr.Marks) != 1 {
		t.Errorf("Expected 1 mark, got %d", len(attr.Marks))
	}

	if attr.Marks[0].Model != "gpt-4" {
		t.Errorf("Expected mark model gpt-4, got %s", attr.Marks[0].Model)
	}

	ai, aiMod, human := notes.ComputeStats(attr)
	if ai == 0 {
		t.Error("Expected some AI lines, got 0")
	}

	t.Logf("Stats: %d ai, %d ai-modified, %d human", ai, aiMod, human)
}

func TestAIModifiedFlow(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatal(err)
	}

	aiContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := commands.Mark([]string{"test.txt"})
	if err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	modifiedContent := "line1\nline2x\nline3\n"
	if err := os.WriteFile(testFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	err = commands.Commit([]string{"-m", "Add modified AI code"})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit failed: %v", err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Load attribution failed: %v", err)
	}

	ai, aiMod, human := notes.ComputeStats(attr)
	if aiMod == 0 {
		t.Error("Expected some ai-modified lines, got 0")
	}

	t.Logf("Stats: %d ai, %d ai-modified, %d human", ai, aiMod, human)
}

func TestMultipleMarks(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatal(err)
	}

	content1 := "line1\nline2\n"
	if err := os.WriteFile(testFile, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	err := commands.Mark([]string{"--model", "gpt-3.5", "test.txt"})
	if err != nil {
		t.Fatalf("First mark failed: %v", err)
	}

	content2 := "line1\nline2\nline3\n"
	if err := os.WriteFile(testFile, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	err = commands.Mark([]string{"--model", "gpt-4", "test.txt"})
	if err != nil {
		t.Fatalf("Second mark failed: %v", err)
	}

	snapshots, err := snapshot.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}

	if len(snapshots) != 2 {
		t.Fatalf("Expected 2 snapshots, got %d", len(snapshots))
	}

	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	err = commands.Commit([]string{"-m", "Multiple AI marks"})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit failed: %v", err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Load attribution failed: %v", err)
	}

	if len(attr.Marks) != 2 {
		t.Errorf("Expected 2 marks in attribution, got %d", len(attr.Marks))
	}
}

func TestRecordCommand(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatal(err)
	}

	aiContent := "line1\nline2\n"
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := commands.Mark([]string{"test.txt"})
	if err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Regular commit").Run(); err != nil {
		t.Fatal(err)
	}

	err = commands.Record([]string{})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit failed: %v", err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Load attribution failed: %v", err)
	}

	if attr == nil {
		t.Fatal("Expected attribution, got nil")
	}
}

func TestStatusCommand(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatal(err)
	}

	aiContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := commands.Mark([]string{"test.txt"})
	if err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	err = commands.Status([]string{})
	if err != nil {
		if strings.Contains(err.Error(), "no AI snapshots") {
			t.Skip("Status requires snapshots")
		}
		t.Fatalf("Status failed: %v", err)
	}
}

func TestFuzzyMatchingThreshold(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("original line\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatal(err)
	}

	aiContent := "original line\nAI generated line\nAnother AI line\n"
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	err := commands.Mark([]string{"test.txt"})
	if err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	modifiedContent := "original line\nAI generated line modified\nCompletely different human line\n"
	if err := os.WriteFile(testFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", "test.txt").Run(); err != nil {
		t.Fatal(err)
	}
	err = commands.Commit([]string{"-m", "Test fuzzy matching"})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatalf("GetLastCommit failed: %v", err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Load attribution failed: %v", err)
	}

	ai, aiMod, human := notes.ComputeStats(attr)

	if aiMod == 0 {
		t.Error("Expected lightly edited AI line to be marked as ai-modified")
	}

	if human == 0 {
		t.Error("Expected unrelated human line to be marked as human")
	}

	t.Logf("Fuzzy matching test: %d ai, %d ai-modified, %d human", ai, aiMod, human)
}
