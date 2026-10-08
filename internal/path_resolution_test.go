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

func TestRelativePathFromSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-relpath-*")
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

	if err := os.WriteFile(filepath.Join("pkg", "a.py"), []byte("# Initial\n# AI edit\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir("pkg"); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "a.py"}); err != nil {
		t.Fatalf("Mark a.py from subdirectory failed: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit"}); err != nil {
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
		t.Errorf("Expected 1 AI line, got %d (R1 regression: wrong file marked)", ai)
	}

	t.Log("✓ Relative path from subdirectory resolves correctly")
}

func TestAmbiguousFileNames(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-ambiguous-*")
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

	if err := os.WriteFile("util.py", []byte("# Root util\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("pkg", "util.py"), []byte("# Pkg util\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join("pkg", "util.py"), []byte("# Pkg util\n# AI edit pkg\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir("pkg"); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "util.py"}); err != nil {
		t.Fatalf("Mark util.py from pkg/ failed: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit"}); err != nil {
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
		t.Errorf("Expected 1 AI line, got %d (R1 regression: marked wrong util.py)", ai)
	}

	t.Log("✓ Ambiguous file names resolved correctly from subdirectory")
}

func TestParentDirPathFromSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-parentdir-*")
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

	if err := os.WriteFile("root.py", []byte("# Root file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("root.py", []byte("# Root file\n# AI edit\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir("pkg"); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "../root.py"}); err != nil {
		t.Fatalf("Mark ../root.py from subdirectory failed: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit"}); err != nil {
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
		t.Errorf("Expected 1 AI line, got %d", ai)
	}

	t.Log("✓ Parent directory path from subdirectory works correctly")
}

func TestAbsolutePathMark(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-abspath-*")
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

	if err := os.WriteFile("test.py", []byte("# Initial\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile("test.py", []byte("# Initial\n# AI edit\n"), 0644); err != nil {
		t.Fatal(err)
	}

	absPath := filepath.Join(tmpDir, "test.py")
	if err := commands.Mark([]string{"--agent", "test", absPath}); err != nil {
		t.Fatalf("Mark with absolute path failed: %v", err)
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := commands.Commit([]string{"-m", "Commit"}); err != nil {
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
		t.Errorf("Expected 1 AI line, got %d", ai)
	}

	t.Log("✓ Absolute path mark works correctly")
}

func TestPathOutsideRepository(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-outside-*")
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

	outsideFile := filepath.Join(os.TempDir(), "outside.py")
	if err := os.WriteFile(outsideFile, []byte("# Outside\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Remove(outsideFile)
	}()

	err = commands.Mark([]string{"--agent", "test", outsideFile})
	if err == nil || err.Error() != "no valid files to mark" {
		t.Errorf("Expected 'no valid files to mark' error for outside path, got: %v", err)
	}

	err = commands.Mark([]string{"--agent", "test", "../../etc/hostname"})
	if err == nil || err.Error() != "no valid files to mark" {
		t.Errorf("Expected 'no valid files to mark' error for outside relative path, got: %v", err)
	}

	t.Log("✓ Paths outside repository are correctly rejected")
}
