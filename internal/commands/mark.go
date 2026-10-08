package commands

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/snapshot"
)

type HookInput struct {
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

func Mark(args []string) error {
	fs := flag.NewFlagSet("mark", flag.ExitOnError)
	model := fs.String("model", "", "AI model name")
	agent := fs.String("agent", "", "AI agent/editor name")
	prompt := fs.String("prompt", "", "Prompt text")
	promptFile := fs.String("prompt-file", "", "Read prompt from file")
	quiet := fs.Bool("quiet", false, "Suppress output")
	quietShort := fs.Bool("q", false, "Suppress output")
	stdinJSON := fs.Bool("stdin-json", false, "Read file path from hook JSON on stdin")

	if err := fs.Parse(args); err != nil {
		return err
	}

	isQuiet := *quiet || *quietShort

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return err
	}

	var paths []string

	if *stdinJSON {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}

		var input HookInput
		if err := json.Unmarshal(data, &input); err != nil {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse JSON, ignoring stdin\n")
			}
		} else if input.ToolInput.FilePath != "" {
			absPath := input.ToolInput.FilePath
			if !filepath.IsAbs(absPath) {
				absPath, err = filepath.Abs(absPath)
				if err != nil {
					return fmt.Errorf("failed to resolve path: %w", err)
				}
			}

			relPath, err := filepath.Rel(repoRoot, absPath)
			if err != nil {
				return fmt.Errorf("file is outside repository: %w", err)
			}

			paths = []string{relPath}
		}
	}

	if len(paths) == 0 {
		if fs.NArg() > 0 {
			paths = fs.Args()
		} else {
			changedFiles, err := git.GetChangedFiles()
			if err != nil {
				return err
			}
			paths = changedFiles
		}
	}

	if len(paths) == 0 {
		if !isQuiet {
			fmt.Println("No changes to mark")
		}
		return nil
	}

	promptText := *prompt
	if *promptFile != "" {
		data, err := os.ReadFile(*promptFile)
		if err != nil {
			return fmt.Errorf("failed to read prompt file: %w", err)
		}
		promptText = string(data)
	}

	files := make(map[string]string)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	for _, path := range paths {
		var absPath string
		if filepath.IsAbs(path) {
			absPath = path
		} else {
			absPath = filepath.Join(cwd, path)
		}

		absPath, err = filepath.Abs(absPath)
		if err != nil {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to resolve %s: %v\n", path, err)
			}
			continue
		}

		relPath, err := filepath.Rel(repoRoot, absPath)
		if err != nil {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "Warning: %s is outside repository: %v\n", path, err)
			}
			continue
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", path, err)
			}
			continue
		}

		headContent := ""
		headBytes, err := git.ShowFile("HEAD", relPath)
		if err == nil {
			headContent = string(headBytes)
		}

		files[relPath] = string(content)

		if headContent != "" {
			snapshotDir, err := snapshot.GetSnapshotDir()
			if err == nil {
				baselineFile := filepath.Join(snapshotDir, "baseline_"+relPath)
				os.MkdirAll(filepath.Dir(baselineFile), 0755)
				os.WriteFile(baselineFile, []byte(headContent), 0644)
			}
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no valid files to mark")
	}

	mark := snapshot.Mark{
		Timestamp: time.Now().UTC(),
		Model:     *model,
		Agent:     *agent,
	}

	if promptText != "" {
		mark.PromptHash = snapshot.HashPrompt(promptText)
		mark.PromptShort = snapshot.TruncatePrompt(promptText, 200)
	}

	if err := snapshot.Save(files, mark); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	if !isQuiet {
		fmt.Printf("Marked %d file(s) as AI-generated\n", len(files))
		if *model != "" {
			fmt.Printf("  Model: %s\n", *model)
		}
		if *agent != "" {
			fmt.Printf("  Agent: %s\n", *agent)
		}
	}

	return nil
}
