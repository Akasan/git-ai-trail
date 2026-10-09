package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Akasan/git-ai-trail/internal/git"
)

const ConfigFileName = ".git-ai-trail.json"

type Config struct {
	Verify VerifyConfig `json:"verify,omitempty"`
}

type VerifyConfig struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	BaseURL  string `json:"baseURL,omitempty"`
}

var (
	loadOnce     sync.Once
	cachedConfig *Config
	loadErr      error
)

func ResetCache() {
	loadOnce = sync.Once{}
	cachedConfig = nil
	loadErr = nil
}

func Load() (*Config, error) {
	loadOnce.Do(func() {
		repoRoot, err := git.GetRepoRoot()
		if err != nil {
			loadErr = err
			return
		}

		configPath := filepath.Join(repoRoot, ConfigFileName)
		
		info, err := os.Stat(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				cachedConfig = &Config{}
				return
			}
			loadErr = err
			return
		}
		
		if info.IsDir() {
			fmt.Fprintf(os.Stderr, "Warning: %s is a directory, not a file. Using default configuration.\n", ConfigFileName)
			cachedConfig = &Config{}
			return
		}
		
		data, err := os.ReadFile(configPath)
		if err != nil {
			loadErr = err
			return
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", ConfigFileName, err)
			fmt.Fprintf(os.Stderr, "Using default configuration. Please check the JSON syntax.\n")
			cachedConfig = &Config{}
			return
		}

		cachedConfig = &cfg
	})
	
	return cachedConfig, loadErr
}

func GetVerifyProvider() string {
	personalProvider, err := git.ConfigGet("ai-trail.verifyProvider")
	if err == nil && personalProvider != "" {
		return personalProvider
	}

	cfg, err := Load()
	if err == nil && cfg.Verify.Provider != "" {
		return cfg.Verify.Provider
	}

	return ""
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

	return ""
}

func GetVerifyBaseURL() string {
	envBaseURL := os.Getenv("GIT_AI_TRAIL_BASE_URL")
	if envBaseURL != "" {
		return envBaseURL
	}

	personalBaseURL, err := git.ConfigGet("ai-trail.verifyBaseURL")
	if err == nil && personalBaseURL != "" {
		return personalBaseURL
	}

	return ""
}

func InferProvider(model string) string {
	if model == "" {
		return ""
	}
	
	lower := strings.ToLower(model)
	
	if hasPrefix(lower, "claude") {
		return "anthropic"
	}
	
	if hasPrefix(lower, "gpt-") || hasPrefix(lower, "chatgpt-") || 
	   lower == "o1" || lower == "o3" || lower == "o4" ||
	   hasPrefix(lower, "o1-") || hasPrefix(lower, "o3-") || hasPrefix(lower, "o4-") {
		return "openai"
	}
	
	if hasPrefix(lower, "grok") {
		return "xai"
	}
	
	return ""
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func GetDefaultModel(provider string) string {
	switch provider {
	case "openai":
		return "gpt-6.1-sol"
	case "xai":
		return "grok-4.7"
	case "anthropic":
		return "claude-sonnet-4-5"
	default:
		return "claude-sonnet-4-5"
	}
}

func ValidateBaseURL(baseURL, provider string) error {
	if baseURL == "" {
		return nil
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid baseURL: %w", err)
	}

	if u.Scheme != "https" && !strings.HasPrefix(u.Host, "localhost:") && !strings.HasPrefix(u.Host, "127.0.0.1:") && u.Host != "localhost" && u.Host != "127.0.0.1" {
		return fmt.Errorf("baseURL must use https (got %s). Only localhost is allowed over http for testing", u.Scheme)
	}

	return nil
}

func ResolveProviderAndModel() (provider, model string) {
	provider = GetVerifyProvider()
	model = GetVerifyModel()
	
	if model != "" && provider == "" {
		inferredProvider := InferProvider(model)
		if inferredProvider != "" {
			provider = inferredProvider
		}
	}
	
	if provider != "" && model != "" {
		inferredFromModel := InferProvider(model)
		if inferredFromModel != "" && inferredFromModel != provider {
			return "", ""
		}
	}
	
	if provider == "" {
		provider = "anthropic"
	}
	
	if model == "" {
		model = GetDefaultModel(provider)
	}
	
	return provider, model
}
