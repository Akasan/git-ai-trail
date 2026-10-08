package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func TestSubdirectoryStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-subdir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	origDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(origDir)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.name", "Test User").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatal(err)
	}

	if err := commands.Init([]string{}); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir("pkg", 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join("pkg", "a.py"), []byte("# Initial\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join("pkg", "a.py"), []byte("# Initial\n# Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("pkg", "b.py"), []byte("# New file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test"}); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	if err := os.Chdir("pkg"); err != nil {
		t.Fatal(err)
	}

	if err := commands.Status([]string{}); err != nil {
		t.Fatalf("Status from subdirectory failed: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit from root"}); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Load attribution failed: %v", err)
	}

	ai, _, _ := notes.ComputeStats(attr)
	if ai != 2 {
		t.Errorf("Expected 2 AI lines, got %d", ai)
	}

	t.Log("✓ Subdirectory operations work correctly")
}

func TestNoArgsMarkFromSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-mark-subdir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	origDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(origDir)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.name", "Test User").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.email", "test@example.com").Run(); err != nil {
		t.Fatal(err)
	}

	if err := commands.Init([]string{}); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir("pkg", 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join("pkg", "a.py"), []byte("# Initial\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join("pkg", "a.py"), []byte("# Initial\n# Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir("pkg"); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test"}); err != nil {
		t.Fatalf("Mark from subdirectory failed: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit after mark from subdir"}); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Fatalf("Load attribution failed: %v", err)
	}

	ai, _, _ := notes.ComputeStats(attr)
	if ai != 1 {
		t.Errorf("Expected 1 AI line, got %d (N10 regression: mark from subdir failed)", ai)
	}

	t.Log("✓ No-args mark from subdirectory works correctly")
}
