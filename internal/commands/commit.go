package commands

import (
	"fmt"

	"github.com/Akasan/git-ai-trail/internal/git"
)

func Commit(args []string) error {
	if err := git.Commit(args); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	if err := Record([]string{}); err != nil {
		return err
	}

	return nil
}
