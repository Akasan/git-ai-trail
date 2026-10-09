package config

import (
	"encoding/json"
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
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
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
