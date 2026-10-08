package commands

import (
	"fmt"

	"github.com/user/git-ai-trail/internal/git"
	"github.com/user/git-ai-trail/internal/notes"
)

func Blame(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: git ai-trail blame <file>")
	}
	
	file := args[0]
	
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
		
		kind := notes.GetLineKind(attr, file, line.LineNum)
		
		kindLabel := "?"
		switch kind {
		case "ai":
			kindLabel = "AI"
		case "ai-modified":
			kindLabel = "AI*"
		case "human":
			kindLabel = "HUM"
		default:
			kindLabel = "???"
		}
		
		fmt.Printf("%s %s %4d) %s\n",
			line.Commit[:7], kindLabel, line.LineNum, line.Content)
	}
	
	return nil
}
