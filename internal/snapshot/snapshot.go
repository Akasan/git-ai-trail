package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Akasan/git-ai-trail/internal/git"
)

type Snapshot struct {
	Timestamp   time.Time         `json:"timestamp"`
	Files       map[string]string `json:"files"`
	Model       string            `json:"model,omitempty"`
	Agent       string            `json:"agent,omitempty"`
	PromptHash  string            `json:"prompt_hash,omitempty"`
	PromptShort string            `json:"prompt_short,omitempty"`
}

type Mark struct {
	Timestamp   time.Time `json:"timestamp"`
	Model       string    `json:"model,omitempty"`
	Agent       string    `json:"agent,omitempty"`
	PromptHash  string    `json:"prompt_hash,omitempty"`
	PromptShort string    `json:"prompt_short,omitempty"`
}

func GetSnapshotDir() (string, error) {
	gitDir, err := git.GetGitDir()
	if err != nil {
		return "", err
	}
	snapshotDir := filepath.Join(gitDir, "ai-trail")
	return snapshotDir, nil
}

func Save(files map[string]string, mark Mark) error {
	snapshotDir, err := GetSnapshotDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return err
	}

	snapshot := Snapshot{
		Timestamp:   mark.Timestamp,
		Files:       files,
		Model:       mark.Model,
		Agent:       mark.Agent,
		PromptHash:  mark.PromptHash,
		PromptShort: mark.PromptShort,
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	snapshotFile := filepath.Join(snapshotDir, id+".json")

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(snapshotFile, data, 0644)
}

func LoadAll() ([]Snapshot, error) {
	snapshotDir, err := GetSnapshotDir()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		return []Snapshot{}, nil
	}

	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return nil, err
	}

	var snapshots []Snapshot
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(snapshotDir, entry.Name()))
		if err != nil {
			continue
		}

		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

func Clear() error {
	snapshotDir, err := GetSnapshotDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		return nil
	}

	return os.RemoveAll(snapshotDir)
}

func HashPrompt(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(hash[:])
}

func TruncatePrompt(prompt string, maxLen int) string {
	if len(prompt) <= maxLen {
		return prompt
	}
	return prompt[:maxLen] + "..."
}
