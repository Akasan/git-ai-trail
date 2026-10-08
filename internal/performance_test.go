package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func runCmdP(t testing.TB, dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Command %s %v failed: %v", name, args, err)
	}
}

func setupTestRepoP(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-perf-*")
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

	runCmdP(t, tmpDir, "git", "init")
	runCmdP(t, tmpDir, "git", "config", "user.name", "Test User")
	runCmdP(t, tmpDir, "git", "config", "user.email", "test@example.com")

	if err := commands.Init([]string{}); err != nil {
		cleanup()
		t.Fatal(err)
	}

	return tmpDir, cleanup
}

func TestLargeFilePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large file performance test in short mode")
	}

	tmpDir, cleanup := setupTestRepoP(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "large.py")

	var lines []string
	for i := 0; i < 1500; i++ {
		lines = append(lines, fmt.Sprintf("x%d = compute_value(%d, \"padding string number %d\")", i, i, i))
	}
	initialContent := strings.Join(lines, "\n") + "\n"

	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}
	runCmdP(t, tmpDir, "git", "add", "large.py")
	runCmdP(t, tmpDir, "git", "commit", "-m", "Initial large file")

	aiLines := make([]string, len(lines))
	copy(aiLines, lines)
	for i := 750; i < 800; i++ {
		aiLines[i] = fmt.Sprintf("# AI edit line %d", i)
	}
	aiContent := strings.Join(aiLines, "\n") + "\n"

	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "large.py"}); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	runCmd(t, tmpDir, "git", "add", "large.py")
	if err := commands.Commit([]string{"-m", "AI edits to large file"}); err != nil {
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
	if ai != 50 {
		t.Errorf("Expected 50 AI lines, got %d", ai)
	}

	t.Log("✓ Large file (1500 lines) performance test passed")
}

func BenchmarkLargeFileAttribution(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	origDir, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(origDir)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		b.Fatal(err)
	}

	if err := exec.Command("git", "init").Run(); err != nil {
		b.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.name", "Test User").Run(); err != nil {
		b.Fatal(err)
	}
	if err := exec.Command("git", "config", "user.email", "test@example.com").Run(); err != nil {
		b.Fatal(err)
	}

	testFile := filepath.Join(tmpDir, "large.py")

	var lines []string
	for i := 0; i < 1500; i++ {
		lines = append(lines, fmt.Sprintf("x%d = compute_value(%d, \"padding string number %d\")", i, i, i))
	}
	initialContent := strings.Join(lines, "\n") + "\n"

	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		b.Fatal(err)
	}
	if err := exec.Command("git", "add", "large.py").Run(); err != nil {
		b.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial").Run(); err != nil {
		b.Fatal(err)
	}

	aiLines := make([]string, len(lines))
	copy(aiLines, lines)
	for i := 750; i < 800; i++ {
		aiLines[i] = fmt.Sprintf("# AI edit line %d", i)
	}
	aiContent := strings.Join(aiLines, "\n") + "\n"

	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		b.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "large.py"}); err != nil {
		b.Fatalf("Mark failed: %v", err)
	}

	if err := exec.Command("git", "add", "large.py").Run(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = commands.Commit([]string{"-m", fmt.Sprintf("Commit %d", i)})
	}
}
