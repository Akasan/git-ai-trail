package commands

import (
	"fmt"

	"github.com/user/git-ai-trail/internal/attribution"
	"github.com/user/git-ai-trail/internal/git"
)

func Status(args []string) error {
	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		return err
	}
	
	stats, err := attribution.ComputeForWorkingTree(repoRoot)
	if err != nil {
		return err
	}
	
	if len(stats) == 0 {
		fmt.Println("No uncommitted changes")
		return nil
	}
	
	totalAI := 0
	totalAIModified := 0
	totalHuman := 0
	
	fmt.Println("AI attribution for uncommitted changes:")
	fmt.Println()
	
	for file, fileStat := range stats {
		totalAI += fileStat.AI
		totalAIModified += fileStat.AIModified
		totalHuman += fileStat.Human
		
		total := fileStat.AI + fileStat.AIModified + fileStat.Human
		if total == 0 {
			continue
		}
		
		aiPercent := float64(fileStat.AI+fileStat.AIModified) / float64(total) * 100
		
		fmt.Printf("%s:\n", file)
		fmt.Printf("  %d ai, %d ai-modified, %d human (%.1f%% AI)\n",
			fileStat.AI, fileStat.AIModified, fileStat.Human, aiPercent)
		fmt.Println()
	}
	
	grandTotal := totalAI + totalAIModified + totalHuman
	if grandTotal > 0 {
		overallPercent := float64(totalAI+totalAIModified) / float64(grandTotal) * 100
		fmt.Printf("Overall: %d ai, %d ai-modified, %d human (%.1f%% AI)\n",
			totalAI, totalAIModified, totalHuman, overallPercent)
	}
	
	return nil
}
