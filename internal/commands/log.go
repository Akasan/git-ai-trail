package commands

import (
	"fmt"
	"sort"
	"time"

	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func Log(args []string) error {
	commits, err := git.CommitLog(args)
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}

	for _, commit := range commits {
		attr, err := notes.Load(commit.Hash)

		if err != nil {
			fmt.Printf("%s %s\n", commit.Hash[:7], commit.Subject)
			fmt.Printf("  Author: %s\n", commit.Author)
			fmt.Printf("  AI: No attribution data\n")
		} else {
			ai, aiMod, human := notes.ComputeStats(attr)
			total := ai + aiMod + human
			aiPercent := 0.0
			if total > 0 {
				aiPercent = float64(ai+aiMod) / float64(total) * 100
			}

			fmt.Printf("%s %s\n", commit.Hash[:7], commit.Subject)
			fmt.Printf("  Author: %s\n", commit.Author)
			fmt.Printf("  AI: %d lines (%.1f%%) - %d ai, %d ai-modified, %d human\n",
				total, aiPercent, ai, aiMod, human)

			if len(attr.Marks) > 0 {
				for _, mark := range attr.Marks {
					if mark.Model != "" || mark.Agent != "" {
						fmt.Printf("  Mark: ")
						if mark.Model != "" {
							fmt.Printf("model=%s ", mark.Model)
						}
						if mark.Agent != "" {
							fmt.Printf("agent=%s ", mark.Agent)
						}
						if mark.Timestamp != "" {
							ts, err := time.Parse(time.RFC3339, mark.Timestamp)
							if err == nil {
								fmt.Printf("at=%s", ts.Local().Format("2006-01-02 15:04:05"))
							}
						}
						fmt.Println()
					}
				}
			}
		}
		fmt.Println()
	}

	return nil
}

func Show(args []string) error {
	var commit string
	if len(args) > 0 {
		commit = args[0]
	} else {
		var err error
		commit, err = git.GetLastCommit()
		if err != nil {
			return fmt.Errorf("failed to get last commit: %w", err)
		}
	}

	resolvedCommit, err := git.RevParse(commit)
	if err != nil {
		return fmt.Errorf("invalid commit: %w", err)
	}

	attr, err := notes.Load(resolvedCommit)
	if err != nil {
		return fmt.Errorf("no attribution data for commit %s", resolvedCommit[:7])
	}

	fmt.Printf("Commit: %s\n", resolvedCommit)
	fmt.Printf("Schema Version: %d\n", attr.SchemaVersion)
	fmt.Printf("Tool Version: %s\n", attr.ToolVersion)
	fmt.Println()

	if len(attr.Marks) > 0 {
		fmt.Println("Marks:")
		for i, mark := range attr.Marks {
			fmt.Printf("  [%d]\n", i+1)
			if mark.Timestamp != "" {
				fmt.Printf("    Timestamp: %s\n", mark.Timestamp)
			}
			if mark.Model != "" {
				fmt.Printf("    Model: %s\n", mark.Model)
			}
			if mark.Agent != "" {
				fmt.Printf("    Agent: %s\n", mark.Agent)
			}
			if mark.PromptHash != "" {
				fmt.Printf("    Prompt Hash: %s\n", mark.PromptHash)
			}
			if mark.PromptShort != "" {
				fmt.Printf("    Prompt: %s\n", mark.PromptShort)
			}
		}
		fmt.Println()
	}

	fmt.Println("Files:")
	ai, aiMod, human := notes.ComputeStats(attr)

	var paths []string
	for path := range attr.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		fileAttr := attr.Files[path]
		fileAI := 0
		fileAIMod := 0
		fileHuman := 0

		for _, r := range fileAttr.Ranges {
			lines := r.End - r.Start + 1
			switch r.Kind {
			case "ai":
				fileAI += lines
			case "ai-modified":
				fileAIMod += lines
			case "human":
				fileHuman += lines
			}
		}

		total := fileAI + fileAIMod + fileHuman
		aiPercent := 0.0
		if total > 0 {
			aiPercent = float64(fileAI+fileAIMod) / float64(total) * 100
		}

		fmt.Printf("  %s: %d lines (%.1f%% AI)\n", path, total, aiPercent)
		fmt.Printf("    %d ai, %d ai-modified, %d human\n", fileAI, fileAIMod, fileHuman)
	}

	fmt.Println()
	fmt.Printf("Total: %s\n", notes.FormatStats(ai, aiMod, human))

	return nil
}
