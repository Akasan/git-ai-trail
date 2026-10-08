package notes

import (
	"encoding/json"
	"fmt"

	"github.com/Akasan/git-ai-trail/internal/git"
)

const (
	NotesRef      = "refs/notes/ai-trail"
	SchemaVersion = 1
)

type Attribution struct {
	SchemaVersion int                        `json:"schema_version"`
	ToolVersion   string                     `json:"tool_version"`
	Files         map[string]FileAttribution `json:"files"`
	Marks         []MarkInfo                 `json:"marks,omitempty"`
}

type FileAttribution struct {
	Path   string      `json:"path"`
	Ranges []LineRange `json:"ranges"`
}

type LineRange struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Kind  string `json:"kind"`
}

type MarkInfo struct {
	Timestamp   string `json:"timestamp"`
	Model       string `json:"model,omitempty"`
	Agent       string `json:"agent,omitempty"`
	PromptHash  string `json:"prompt_hash,omitempty"`
	PromptShort string `json:"prompt_short,omitempty"`
}

func Save(commit string, attr Attribution) error {
	data, err := json.MarshalIndent(attr, "", "  ")
	if err != nil {
		return err
	}

	return git.AddNote(NotesRef, commit, string(data))
}

func Load(commit string) (*Attribution, error) {
	data, err := git.GetNote(NotesRef, commit)
	if err != nil {
		return nil, err
	}

	var attr Attribution
	if err := json.Unmarshal([]byte(data), &attr); err != nil {
		return nil, err
	}

	return &attr, nil
}

func GetLineKind(attr *Attribution, file string, line int) string {
	if attr == nil {
		return "unknown"
	}

	fileAttr, ok := attr.Files[file]
	if !ok {
		return "unknown"
	}

	for _, r := range fileAttr.Ranges {
		if line >= r.Start && line <= r.End {
			return r.Kind
		}
	}

	return "unknown"
}

func ComputeStats(attr *Attribution) (ai, aiModified, human int) {
	for _, fileAttr := range attr.Files {
		for _, r := range fileAttr.Ranges {
			lines := r.End - r.Start + 1
			switch r.Kind {
			case "ai":
				ai += lines
			case "ai-modified":
				aiModified += lines
			case "human":
				human += lines
			}
		}
	}
	return ai, aiModified, human
}

func FormatStats(ai, aiModified, human int) string {
	total := ai + aiModified + human
	if total == 0 {
		return "0 lines (0% AI)"
	}

	aiPercent := float64(ai+aiModified) / float64(total) * 100
	return fmt.Sprintf("%d lines (%d ai, %d ai-modified, %d human) - %.1f%% AI",
		total, ai, aiModified, human, aiPercent)
}
