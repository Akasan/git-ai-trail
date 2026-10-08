package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/user/git-ai-trail/internal/git"
	"github.com/user/git-ai-trail/internal/snapshot"
)

func Mark(args []string) error {
	fs := flag.NewFlagSet("mark", flag.ExitOnError)
	model := fs.String("model", "", "AI model name")
	agent := fs.String("agent", "", "AI agent/editor name")
	prompt := fs.String("prompt", "", "Prompt text")
	promptFile := fs.String("prompt-file", "", "Read prompt from file")
	quiet := fs.Bool("quiet", false, "Suppress output")
	quietShort := fs.Bool("q", false, "Suppress output")
	
	if err := fs.Parse(args); err != nil {
		return err
	}
	
	isQuiet := *quiet || *quietShort
	
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return err
	}
	
	var paths []string
	if fs.NArg() > 0 {
		paths = fs.Args()
	} else {
		changedFiles, err := git.GetChangedFiles()
		if err != nil {
			return err
		}
		paths = changedFiles
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
	for _, path := range paths {
		absPath := filepath.Join(repoRoot, path)
		content, err := os.ReadFile(absPath)
		if err != nil {
			if !isQuiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", path, err)
			}
			continue
		}
		files[path] = string(content)
	}
	
	if len(files) == 0 {
		return fmt.Errorf("no valid files to mark")
	}
	
	mark := snapshot.Mark{
		Timestamp: time.Now(),
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
