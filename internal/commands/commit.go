package commands

import (
	"fmt"

	"github.com/user/git-ai-trail/internal/attribution"
	"github.com/user/git-ai-trail/internal/git"
	"github.com/user/git-ai-trail/internal/notes"
	"github.com/user/git-ai-trail/internal/snapshot"
)

func Commit(args []string) error {
	snapshots, err := snapshot.LoadAll()
	if err != nil {
		return err
	}
	
	if err := git.Commit(args); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}
	
	commit, err := git.GetLastCommit()
	if err != nil {
		return fmt.Errorf("failed to get last commit: %w", err)
	}
	
	if len(snapshots) > 0 {
		attr, err := attribution.Compute(commit, snapshots)
		if err != nil {
			return fmt.Errorf("failed to compute attribution: %w", err)
		}
		
		if err := notes.Save(commit, *attr); err != nil {
			return fmt.Errorf("failed to save attribution: %w", err)
		}
		
		ai, aiMod, human := notes.ComputeStats(attr)
		fmt.Printf("Attribution recorded: %s\n", notes.FormatStats(ai, aiMod, human))
	}
	
	if err := snapshot.Clear(); err != nil {
		return fmt.Errorf("failed to clear snapshots: %w", err)
	}
	
	return nil
}

func Record(args []string) error {
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
	
	snapshots, err := snapshot.LoadAll()
	if err != nil {
		return err
	}
	
	if len(snapshots) == 0 {
		return fmt.Errorf("no AI snapshots recorded; run 'git ai-trail mark' first")
	}
	
	attr, err := attribution.Compute(resolvedCommit, snapshots)
	if err != nil {
		return fmt.Errorf("failed to compute attribution: %w", err)
	}
	
	if err := notes.Save(resolvedCommit, *attr); err != nil {
		return fmt.Errorf("failed to save attribution: %w", err)
	}
	
	ai, aiMod, human := notes.ComputeStats(attr)
	fmt.Printf("Attribution recorded for %s: %s\n", resolvedCommit[:7], notes.FormatStats(ai, aiMod, human))
	
	if err := snapshot.Clear(); err != nil {
		return fmt.Errorf("failed to clear snapshots: %w", err)
	}
	
	return nil
}
