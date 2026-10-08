package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/commands"
	"github.com/Akasan/git-ai-trail/internal/git"
)

func TestHookBackupAndChaining(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-hook-*")
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

	gitDir, err := git.GetGitDir()
	if err != nil {
		t.Fatal(err)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	postCommitPath := filepath.Join(hooksDir, "post-commit")
	backupPath := postCommitPath + ".backup"

	userHook := `#!/bin/sh
echo "User hook executed"
`

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(postCommitPath, []byte(userHook), 0755); err != nil {
		t.Fatal(err)
	}

	if err := commands.InstallHooks([]string{}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("Expected backup to be created")
	}

	differentHook := `#!/bin/sh
echo "Different user hook"
`

	if err := os.WriteFile(postCommitPath, []byte(differentHook), 0755); err != nil {
		t.Fatal(err)
	}

	err = commands.InstallHooks([]string{})
	if err == nil {
		t.Fatal("Expected error when installing over different hook with existing backup (N4b)")
	}

	if !strings.Contains(err.Error(), "differs from backup") {
		t.Errorf("Expected error about differing backup, got: %v", err)
	}

	t.Log("✓ Hook installation correctly refuses to overwrite different user hook")
}

func TestHookReinstallSameHook(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-ai-trail-hook-reinstall-*")
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

	gitDir, err := git.GetGitDir()
	if err != nil {
		t.Fatal(err)
	}

	hooksDir := filepath.Join(gitDir, "hooks")
	postCommitPath := filepath.Join(hooksDir, "post-commit")

	userHook := `#!/bin/sh
echo "User hook executed"
`

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(postCommitPath, []byte(userHook), 0755); err != nil {
		t.Fatal(err)
	}

	if err := commands.InstallHooks([]string{}); err != nil {
		t.Fatal(err)
	}

	if err := commands.InstallHooks([]string{}); err != nil {
		t.Fatalf("Second install should succeed (hook already installed): %v", err)
	}

	if err := os.WriteFile(postCommitPath, []byte(userHook), 0755); err != nil {
		t.Fatal(err)
	}

	if err := commands.InstallHooks([]string{}); err != nil {
		t.Fatalf("Reinstall with same backup should succeed: %v", err)
	}

	t.Log("✓ Hook installation handles reinstall correctly")
}
