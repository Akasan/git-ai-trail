package main

import (
	"fmt"
	"os"

	"github.com/Akasan/git-ai-trail/internal/commands"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "mark":
		if err := commands.Mark(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := commands.Status(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "commit":
		if err := commands.Commit(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "record":
		if err := commands.Record(args); err != nil {
			if err.Error() != "" {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(1)
		}
	case "install-hooks":
		if err := commands.InstallHooks(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "blame":
		if err := commands.Blame(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "log":
		if err := commands.Log(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "show":
		if err := commands.Show(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "init":
		if err := commands.Init(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Printf("git-ai-trail version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	usage := `git-ai-trail - Track AI-generated code changes in Git

Usage:
  git ai-trail mark [options] [paths...]     Record current changes as AI-generated
  git ai-trail status                        Show AI attribution for uncommitted changes
  git ai-trail commit [git args...]          Commit with AI attribution
  git ai-trail record [<commit>]             Add attribution record to an existing commit
  git ai-trail install-hooks                 Install post-commit hook
  git ai-trail init                          Initialize repository for AI tracking
  git ai-trail blame <file>                  Show AI attribution per line
  git ai-trail log [git args...]             Show commits with AI metrics
  git ai-trail show [<commit>]               Show raw attribution record
  git ai-trail version                       Show version
  git ai-trail help                          Show this help

Mark options:
  --model NAME           AI model name
  --agent NAME           AI agent/editor name
  --prompt TEXT          Prompt text (truncated in storage)
  --prompt-file PATH     Read prompt from file
  --quiet, -q            Suppress output

Examples:
  git ai-trail mark --model gpt-4 --agent cursor
  git ai-trail status
  git ai-trail commit -m "Add feature"
  git ai-trail blame main.go
  git ai-trail log --since="1 week ago"
`
	fmt.Print(usage)
}
