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

func TestPartialCommitMultiFileSnapshot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-partial-*")
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

	if err := os.WriteFile("a.py", []byte("# Initial a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("b.py", []byte("# Initial b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("a.py", []byte("# Initial a\n# AI edit a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("b.py", []byte("# Initial b\n# AI edit b\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test"}); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	if err := exec.Command("git", "add", "a.py").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit a only"}); err != nil {
		t.Fatalf("Commit a failed: %v", err)
	}

	commit1, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	attr1, err := notes.Load(commit1)
	if err != nil {
		t.Fatalf("Load attribution for commit1 failed: %v", err)
	}

	ai1, _, _ := notes.ComputeStats(attr1)
	if ai1 != 1 {
		t.Errorf("Commit 1: expected 1 AI line, got %d", ai1)
	}

	if err := exec.Command("git", "add", "b.py").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit b"}); err != nil {
		t.Fatalf("Commit b failed: %v", err)
	}

	commit2, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	attr2, err := notes.Load(commit2)
	if err != nil {
		t.Fatalf("Load attribution for commit2 failed: %v", err)
	}

	ai2, _, _ := notes.ComputeStats(attr2)
	if ai2 != 1 {
		t.Errorf("Commit 2: expected 1 AI line, got %d (N6 regression: snapshot was deleted after commit 1)", ai2)
	}

	t.Log("✓ Partial commit preserves uncommitted files' snapshots")
}

func TestSnapshotCleanupWithInvalidFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-cleanup-*")
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

	if err := os.WriteFile("x.py", []byte("# Initial\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("x.py", []byte("# Initial\n# AI 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := commands.Mark([]string{"--agent", "test", "x.py"}); err != nil {
		t.Fatalf("Mark 1 failed: %v", err)
	}

	if err := os.WriteFile("y.py", []byte("# AI 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := commands.Mark([]string{"--agent", "test", "y.py"}); err != nil {
		t.Fatalf("Mark 2 failed: %v", err)
	}

	gitDir, _ := git.GetGitDir()
	snapshotDir := filepath.Join(gitDir, "ai-trail")

	emptyFile := filepath.Join(snapshotDir, "0000000000000000000.json")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", "x.py").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit x"}); err != nil {
		t.Fatalf("Commit x failed: %v", err)
	}

	commit1, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	attr1, err := notes.Load(commit1)
	if err != nil {
		t.Fatalf("Load attribution for commit1 failed: %v", err)
	}

	ai1, _, _ := notes.ComputeStats(attr1)
	if ai1 != 1 {
		t.Errorf("Commit 1: expected 1 AI line, got %d", ai1)
	}

	if err := exec.Command("git", "add", "y.py").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit y"}); err != nil {
		t.Fatalf("Commit y failed: %v", err)
	}

	commit2, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	attr2, err := notes.Load(commit2)
	if err != nil {
		t.Fatalf("Load attribution for commit2 failed: %v", err)
	}

	ai2, _, _ := notes.ComputeStats(attr2)
	if ai2 != 1 {
		t.Errorf("Commit 2: expected 1 AI line, got %d (N12 regression: wrong snapshot deleted)", ai2)
	}

	t.Log("✓ Snapshot cleanup handles invalid files correctly")
}
