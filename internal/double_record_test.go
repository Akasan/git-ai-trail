package internal

import (
	"os"
	"os/exec"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func TestDoubleRecordWithHook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-double-record-*")
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
	if err := exec.Command("git", "commit", "-m", "Commit a only").Run(); err != nil {
		t.Fatal(err)
	}

	commit1, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	if err := commands.Record([]string{commit1}); err != nil {
		t.Fatalf("First record failed: %v", err)
	}

	if err := commands.Record([]string{commit1}); err != nil {
		t.Fatalf("Second record failed: %v", err)
	}

	attr1, err := notes.Load(commit1)
	if err != nil {
		t.Fatalf("Load attribution for commit1 failed: %v", err)
	}

	ai1, _, human1 := notes.ComputeStats(attr1)
	if ai1 != 1 {
		t.Errorf("Commit 1: expected 1 AI line, got %d (B1 regression: double record overwrote correct attribution)", ai1)
	}
	if human1 != 0 {
		t.Errorf("Commit 1: expected 0 human lines, got %d (B1 regression: unrelated snapshot counted)", human1)
	}

	if err := exec.Command("git", "add", "b.py").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Commit b").Run(); err != nil {
		t.Fatal(err)
	}

	commit2, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	if err := commands.Record([]string{commit2}); err != nil {
		t.Fatalf("Record commit2 failed: %v", err)
	}

	attr2, err := notes.Load(commit2)
	if err != nil {
		t.Fatalf("Load attribution for commit2 failed: %v", err)
	}

	ai2, _, _ := notes.ComputeStats(attr2)
	if ai2 != 1 {
		t.Errorf("Commit 2: expected 1 AI line, got %d", ai2)
	}

	t.Log("✓ Double record with unrelated snapshots works correctly")
}

func TestRecordSkipsUnrelatedSnapshots(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-unrelated-*")
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

	if err := os.WriteFile("y.py", []byte("# AI y\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := commands.Mark([]string{"--agent", "test", "y.py"}); err != nil {
		t.Fatalf("Mark y failed: %v", err)
	}

	if err := os.WriteFile("x.py", []byte("# Initial\n# Human edit\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", "x.py").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Commit x only").Run(); err != nil {
		t.Fatal(err)
	}

	commit, err := git.GetLastCommit()
	if err != nil {
		t.Fatal(err)
	}

	if err := commands.Record([]string{commit}); err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	attr, err := notes.Load(commit)
	if err != nil {
		t.Log("✓ Record correctly skipped (no relevant snapshots, no note created)")
		return
	}

	if attr == nil || len(attr.Files) == 0 {
		t.Log("✓ Record correctly created empty note (no relevant snapshots)")
		return
	}

	ai, _, human := notes.ComputeStats(attr)
	if ai == 0 && human == 1 {
		t.Log("✓ Human-only commit recorded (no AI attribution from unrelated snapshot)")
	} else {
		t.Errorf("Expected 0 ai / 1 human, got ai=%d human=%d (unrelated snapshot may have been counted)", ai, human)
	}
}
