package commands

import (
	"fmt"
	"path/filepath"

	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
)

func Blame(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: git ai-trail blame <file>")
	}

	file := args[0]

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return err
	}

	prefix, err := git.GetShowPrefix()
	if err != nil {
		prefix = ""
	}

	var repoPath string
	if filepath.IsAbs(file) {
		repoPath, err = filepath.Rel(repoRoot, file)
		if err != nil {
			return fmt.Errorf("file is outside repository: %w", err)
		}
	} else {
		repoPath = filepath.Join(prefix, file)
	}

	blameLines, err := git.Blame(file)
	if err != nil {
		return fmt.Errorf("git blame failed: %w", err)
	}

	commitCache := make(map[string]*notes.Attribution)

	for _, line := range blameLines {
		attr, ok := commitCache[line.Commit]
		if !ok {
			loadedAttr, err := notes.Load(line.Commit)
			if err != nil {
				commitCache[line.Commit] = nil
			} else {
				commitCache[line.Commit] = loadedAttr
			}
			attr = commitCache[line.Commit]
		}

		kind := notes.GetLineKind(attr, repoPath, line.LineNum)

		var kindLabel string
		switch kind {
		case "ai":
			kindLabel = "AI "
		case "ai-modified":
			kindLabel = "AI*"
		case "human":
			kindLabel = "HUM"
		default:
			kindLabel = "???"
		}

		fmt.Printf("%s %-3s %4d) %s\n",
			line.Commit[:7], kindLabel, line.LineNum, line.Content)
	}

	return nil
}
