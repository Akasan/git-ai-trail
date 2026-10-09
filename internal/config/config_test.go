package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Akasan/git-ai-trail/internal/git"
)

func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "git-ai-trail-config-test-*")
	if err != nil {
		t.Fatal(err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		_ = os.Chdir(origDir); _ = os.RemoveAll(tmpDir); t.Fatal(err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
		t.Fatal(err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cleanup := func() {
		_ = os.Chdir(origDir)
		_ = os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestConfigLoad(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	t.Run("no config file", func(t *testing.T) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Verify.Model != "" {
			t.Errorf("Expected empty model, got %q", cfg.Verify.Model)
		}
	})

	t.Run("with config file", func(t *testing.T) {
		repoRoot, _ := git.GetRepoRoot()
		configPath := filepath.Join(repoRoot, ConfigFileName)

		testConfig := Config{
			Verify: VerifyConfig{
				Model: "claude-haiku-4-5",
			},
		}

		data, _ := json.Marshal(testConfig)
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.Verify.Model != "claude-haiku-4-5" {
			t.Errorf("Expected model 'claude-haiku-4-5', got %q", cfg.Verify.Model)
		}
	})
}

func TestGetVerifyModelPrecedence(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repoRoot, _ := git.GetRepoRoot()
	configPath := filepath.Join(repoRoot, ConfigFileName)

	t.Run("default only", func(t *testing.T) {
		model := GetVerifyModel()
		if model != "claude-sonnet-4-5" {
			t.Errorf("Expected default 'claude-sonnet-4-5', got %q", model)
		}
	})

	t.Run("repo config overrides default", func(t *testing.T) {
		testConfig := Config{
			Verify: VerifyConfig{
				Model: "claude-haiku-4-5",
			},
		}
		data, _ := json.Marshal(testConfig)
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		model := GetVerifyModel()
		if model != "claude-haiku-4-5" {
			t.Errorf("Expected repo config 'claude-haiku-4-5', got %q", model)
		}

		os.Remove(configPath)
	})

	t.Run("git config overrides repo config", func(t *testing.T) {
		testConfig := Config{
			Verify: VerifyConfig{
				Model: "claude-haiku-4-5",
			},
		}
		data, _ := json.Marshal(testConfig)
		_ = os.WriteFile(configPath, data, 0644)

		cmd := exec.Command("git", "config", "ai-trail.verifyModel", "claude-sonnet-4-6")
		_ = cmd.Run()

		model := GetVerifyModel()
		if model != "claude-sonnet-4-6" {
			t.Errorf("Expected git config 'claude-sonnet-4-6', got %q", model)
		}

		cmd = exec.Command("git", "config", "--unset", "ai-trail.verifyModel")
		_ = cmd.Run()
		_ = os.Remove(configPath)
	})

	t.Run("git config overrides default", func(t *testing.T) {
		cmd := exec.Command("git", "config", "ai-trail.verifyModel", "claude-sonnet-4-6")
		_ = cmd.Run()

		model := GetVerifyModel()
		if model != "claude-sonnet-4-6" {
			t.Errorf("Expected git config 'claude-sonnet-4-6', got %q", model)
		}

		cmd = exec.Command("git", "config", "--unset", "ai-trail.verifyModel")
		_ = cmd.Run()
	})
}

func TestConfigSource(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repoRoot, _ := git.GetRepoRoot()
	configPath := filepath.Join(repoRoot, ConfigFileName)

	tests := []struct {
		name         string
		setupGitConf bool
		gitConfValue string
		setupRepoConf bool
		repoConfValue string
		wantModel    string
		wantSource   string
	}{
		{
			name:       "default",
			wantModel:  "claude-sonnet-4-5",
			wantSource: "default",
		},
		{
			name:          "repo config",
			setupRepoConf: true,
			repoConfValue: "claude-haiku-4-5",
			wantModel:     "claude-haiku-4-5",
			wantSource:    "repo config",
		},
		{
			name:         "git config",
			setupGitConf: true,
			gitConfValue: "claude-sonnet-4-6",
			wantModel:    "claude-sonnet-4-6",
			wantSource:   "git config",
		},
		{
			name:          "git config overrides repo config",
			setupGitConf:  true,
			gitConfValue:  "claude-sonnet-4-6",
			setupRepoConf: true,
			repoConfValue: "claude-haiku-4-5",
			wantModel:     "claude-sonnet-4-6",
			wantSource:    "git config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = exec.Command("git", "config", "--unset", "ai-trail.verifyModel").Run()
			_ = os.Remove(configPath)

			if tt.setupGitConf {
				cmd := exec.Command("git", "config", "ai-trail.verifyModel", tt.gitConfValue)
				if err := cmd.Run(); err != nil {
					t.Fatal(err)
				}
			}

			if tt.setupRepoConf {
				testConfig := Config{
					Verify: VerifyConfig{
						Model: tt.repoConfValue,
					},
				}
				data, _ := json.Marshal(testConfig)
				if err := os.WriteFile(configPath, data, 0644); err != nil {
					t.Fatal(err)
				}
			}

			model := GetVerifyModel()
			if model != tt.wantModel {
				t.Errorf("GetVerifyModel() = %q, want %q (source: %s)", model, tt.wantModel, tt.wantSource)
			}

			_ = exec.Command("git", "config", "--unset", "ai-trail.verifyModel").Run()
			_ = os.Remove(configPath)
		})
	}
}

func TestInvalidJSON(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repoRoot, _ := git.GetRepoRoot()
	configPath := filepath.Join(repoRoot, ConfigFileName)

	if err := os.WriteFile(configPath, []byte("invalid json{"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Errorf("Load should not error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
	
	if cfg.Verify.Model != "" {
		t.Errorf("Expected empty model for invalid JSON, got %q", cfg.Verify.Model)
	}
	
	model := GetVerifyModel()
	if model != "claude-sonnet-4-5" {
		t.Errorf("Expected default model for invalid JSON, got %q", model)
	}
}
