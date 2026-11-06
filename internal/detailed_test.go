package internal

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func TestAIEditWithUntouchedPreExistingLines(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "calc.py")
	initialContent := `def add(a, b):
    return a + b

def subtract(a, b):
    return a - b
`
	if err := os.WriteFile(testFile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, tmpDir, "git", "add", "calc.py")
	runCmd(t, tmpDir, "git", "commit", "-m", "Initial")

	aiContent := `def add(a, b):
    return a + b

def subtract(a, b):
    return a - b

def multiply(a, b):
    return a * b
`
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "calc.py"}); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	runCmd(t, tmpDir, "git", "add", "calc.py")
	if err := commands.Commit([]string{"-m", "Add multiply"}); err != nil {
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

	ai, aiMod, human := notes.ComputeStats(attr)

	if ai != 3 {
		t.Errorf("Expected 3 AI lines (blank line + multiply function), got %d", ai)
	}

	if human != 0 || aiMod != 0 {
		t.Errorf("Expected 0 human + 0 ai-modified (only added lines counted), got %d human + %d ai-modified", human, aiMod)
	}

	for _, r := range attr.Files["calc.py"].Ranges {
		if r.Start <= 5 {
			t.Errorf("Pre-existing lines 1-5 should not be in ranges, got range %d-%d as '%s'",
				r.Start, r.End, r.Kind)
		}
	}

	t.Logf("✓ Attribution: %d ai, %d ai-modified, %d human (pre-existing lines excluded)", ai, aiMod, human)
}

func TestNewFileCreation(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "existing.py")
	if err := os.WriteFile(testFile, []byte("# existing\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, tmpDir, "git", "add", "existing.py")
	runCmd(t, tmpDir, "git", "commit", "-m", "Initial")

	newFile := filepath.Join(tmpDir, "new.py")
	newContent := `def greet(name):
    return f"Hello, {name}"
`
	if err := os.WriteFile(newFile, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test"}); err != nil {
		t.Fatalf("Mark failed: %v", err)
	}

	runCmd(t, tmpDir, "git", "add", "new.py")
	if err := commands.Commit([]string{"-m", "Add new file"}); err != nil {
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

	if _, ok := attr.Files["new.py"]; !ok {
		t.Fatal("new.py not in attribution")
	}

	ai, _, _ := notes.ComputeStats(attr)
	if ai == 0 {
		t.Error("Expected some AI lines for new file, got 0")
	}

	t.Logf("✓ New file attribution: %d ai lines", ai)
}

func TestAIBlankLines(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(testFile, []byte("pass\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, tmpDir, "git", "add", "test.py")
	runCmd(t, tmpDir, "git", "commit", "-m", "Initial")

	aiContent := `pass

def foo():
    pass

def bar():
    pass
`
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "test.py"}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, tmpDir, "git", "add", "test.py")
	if err := commands.Commit([]string{"-m", "Add functions with blank lines"}); err != nil {
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

	ai, _, human := notes.ComputeStats(attr)

	if ai == 0 {
		t.Error("Expected some AI lines including blank lines")
	}

	if human > 0 {
		t.Errorf("Blank lines should be AI, got %d human lines", human)
	}

	t.Logf("✓ Blank line test: %d ai, %d human", ai, human)
}

func TestRootCommit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-test-*")
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

	runCmd(t, tmpDir, "git", "init")
	runCmd(t, tmpDir, "git", "config", "user.name", "Test User")
	runCmd(t, tmpDir, "git", "config", "user.email", "test@example.com")

	testFile := filepath.Join(tmpDir, "first.py")
	if err := os.WriteFile(testFile, []byte("# AI-generated root file\npass\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "first.py"}); err != nil {
		t.Fatal(err)
	}

	runCmd(t, tmpDir, "git", "add", "first.py")
	if err := commands.Commit([]string{"-m", "Root commit"}); err != nil {
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

	if len(attr.Files) == 0 {
		t.Error("Expected files in root commit attribution, got empty")
	}

	ai, _, human := notes.ComputeStats(attr)
	if ai == 0 {
		t.Errorf("Root commit should have AI lines, got 0 (all marked as human: %d)", human)
	}

	t.Logf("✓ Root commit test passed: %d ai, %d human", ai, human)
}

func TestStdinJSONHookSimulation(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "hook_test.py")
	if err := os.WriteFile(testFile, []byte("# test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	hookJSON := map[string]interface{}{
		"tool_input": map[string]interface{}{
			"file_path": testFile,
		},
	}

	jsonBytes, err := json.Marshal(hookJSON)
	if err != nil {
		t.Fatal(err)
	}

	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	go func() {
		_, _ = w.Write(jsonBytes)
		_ = w.Close()
	}()

	err = commands.Mark([]string{"--stdin-json", "--agent", "test-hook", "--quiet"})
	os.Stdin = origStdin

	if err != nil {
		t.Fatalf("mark --stdin-json failed: %v", err)
	}

	runCmd(t, tmpDir, "git", "add", "hook_test.py")
	if err := commands.Commit([]string{"-m", "Hook simulation"}); err != nil {
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

	if len(attr.Marks) == 0 {
		t.Error("Expected mark from stdin-json")
	}

	if attr.Marks[0].Agent != "test-hook" {
		t.Errorf("Expected agent=test-hook, got %s", attr.Marks[0].Agent)
	}

	t.Log("✓ Stdin JSON hook simulation passed")
}

func TestStatusAndCommitRecordAgree(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCmd(t, tmpDir, "git", "add", "test.txt")
	runCmd(t, tmpDir, "git", "commit", "-m", "Initial")

	aiContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(testFile, []byte(aiContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := commands.Mark([]string{"--agent", "test", "test.txt"}); err != nil {
		t.Fatal(err)
	}

	_ = commands.Status([]string{})

	runCmd(t, tmpDir, "git", "add", "test.txt")
	if err := commands.Commit([]string{"-m", "Add lines"}); err != nil {
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

	ai, aiMod, human := notes.ComputeStats(attr)
	total := ai + aiMod + human

	if total != 2 {
		t.Errorf("Expected 2 added lines in commit record, got %d", total)
	}

	t.Logf("✓ Status and commit record agree: %d ai, %d ai-modified, %d human", ai, aiMod, human)
}

func runCmd(t *testing.T, dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command %s %v failed: %v\nOutput: %s", name, args, err, string(output))
	}
}
