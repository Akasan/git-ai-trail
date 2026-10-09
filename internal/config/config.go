package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Akasan/git-ai-trail/internal/git"
)

const ConfigFileName = ".git-ai-trail.json"

type Config struct {
	Verify VerifyConfig `json:"verify,omitempty"`
}

type VerifyConfig struct {
	Model string `json:"model,omitempty"`
}

func Load() (*Config, error) {
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(repoRoot, ConfigFileName)
	
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	
	if info.IsDir() {
		fmt.Fprintf(os.Stderr, "Warning: %s is a directory, not a file. Using default configuration.\n", ConfigFileName)
		return &Config{}, nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", ConfigFileName, err)
		fmt.Fprintf(os.Stderr, "Using default configuration. Please check the JSON syntax.\n")
		return &Config{}, nil
	}

	return &cfg, nil
}

func GetVerifyModel() string {
	personalModel, err := git.ConfigGet("ai-trail.verifyModel")
	if err == nil && personalModel != "" {
		return personalModel
	}

	cfg, err := Load()
	if err == nil && cfg.Verify.Model != "" {
		return cfg.Verify.Model
	}

	return "claude-sonnet-4-5"
}
