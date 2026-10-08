package commands

import (
	"fmt"

	"github.com/Akasan/git-ai-trail/internal/attribution"
	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
	"github.com/Akasan/git-ai-trail/internal/snapshot"
)

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

	committedFiles, err := git.GetCommitFiles(resolvedCommit)
	if err != nil {
		return fmt.Errorf("failed to get commit files: %w", err)
	}

	inCommit := make(map[string]bool, len(committedFiles))
	for _, f := range committedFiles {
		inCommit[f] = true
	}

	var relevant []snapshot.Snapshot
	for _, s := range snapshots {
		for f := range s.Files {
			if inCommit[f] {
				relevant = append(relevant, s)
				break
			}
		}
	}
	snapshots = relevant

	if len(snapshots) == 0 {
		return nil
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

	if err := snapshot.ClearFiles(committedFiles); err != nil {
		return fmt.Errorf("failed to clear snapshots: %w", err)
	}

	return nil
}
