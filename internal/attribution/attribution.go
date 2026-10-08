package attribution

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/user/git-ai-trail/internal/git"
	"github.com/user/git-ai-trail/internal/notes"
	"github.com/user/git-ai-trail/internal/snapshot"
)

const defaultFuzzyThreshold = 0.5

func getFuzzyThreshold() float64 {
	thresholdStr, err := git.ConfigGet("ai-trail.fuzzyThreshold")
	if err != nil {
		return defaultFuzzyThreshold
	}
	
	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil || threshold < 0 || threshold > 1 {
		return defaultFuzzyThreshold
	}
	
	return threshold
}

func Compute(commit string, snapshots []snapshot.Snapshot) (*notes.Attribution, error) {
	files, err := git.GetCommitFiles(commit)
	if err != nil {
		return nil, err
	}
	
	attr := &notes.Attribution{
		SchemaVersion: notes.SchemaVersion,
		ToolVersion:   "0.1.0",
		Files:         make(map[string]notes.FileAttribution),
		Marks:         make([]notes.MarkInfo, 0),
	}
	
	for _, snap := range snapshots {
		attr.Marks = append(attr.Marks, notes.MarkInfo{
			Timestamp:   snap.Timestamp.Format("2006-01-02T15:04:05Z"),
			Model:       snap.Model,
			Agent:       snap.Agent,
			PromptHash:  snap.PromptHash,
			PromptShort: snap.PromptShort,
		})
	}
	
	for _, file := range files {
		fileAttr, err := computeFileAttribution(commit, file, snapshots)
		if err != nil {
			continue
		}
		attr.Files[file] = fileAttr
	}
	
	return attr, nil
}

func computeFileAttribution(commit, path string, snapshots []snapshot.Snapshot) (notes.FileAttribution, error) {
	finalContent, err := git.GetFileAtCommit(commit, path)
	if err != nil {
		return notes.FileAttribution{}, err
	}
	
	finalLines := strings.Split(finalContent, "\n")
	
	lineKinds := make([]string, len(finalLines))
	for i := range lineKinds {
		lineKinds[i] = "human"
	}
	
	aiSnapshot := make(map[int]string)
	for _, snap := range snapshots {
		if content, ok := snap.Files[path]; ok {
			snapLines := strings.Split(content, "\n")
			
			matched := matchLines(snapLines, finalLines)
			for snapLine, finalLine := range matched {
				if finalLine != -1 {
					aiSnapshot[finalLine] = snapLines[snapLine]
				}
			}
		}
	}
	
	threshold := getFuzzyThreshold()
	
	for lineNum, snapContent := range aiSnapshot {
		if lineNum >= len(finalLines) {
			continue
		}
		
		finalLine := finalLines[lineNum]
		trimmedFinal := strings.TrimSpace(finalLine)
		trimmedSnap := strings.TrimSpace(snapContent)
		
		if trimmedFinal == trimmedSnap {
			lineKinds[lineNum] = "ai"
		} else if len(trimmedFinal) > 0 && len(trimmedSnap) > 0 {
			similarity := calculateSimilarity(trimmedFinal, trimmedSnap)
			if similarity >= threshold {
				lineKinds[lineNum] = "ai-modified"
			}
		}
	}
	
	ranges := compressRanges(lineKinds)
	
	return notes.FileAttribution{
		Path:   path,
		Ranges: ranges,
	}, nil
}

func calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	
	longer := s1
	shorter := s2
	if len(s2) > len(s1) {
		longer = s2
		shorter = s1
	}
	
	if len(longer) == 0 {
		return 0.0
	}
	
	matches := 0
	for i := 0; i < len(shorter); i++ {
		if i < len(longer) && shorter[i] == longer[i] {
			matches++
		}
	}
	
	return float64(matches) / float64(len(longer))
}

func matchLines(snapLines, finalLines []string) map[int]int {
	matched := make(map[int]int)
	usedFinalLines := make(map[int]bool)
	threshold := getFuzzyThreshold()
	
	for i, snapLine := range snapLines {
		bestMatch := -1
		bestSimilarity := 0.0
		
		trimmedSnap := strings.TrimSpace(snapLine)
		if len(trimmedSnap) == 0 {
			continue
		}
		
		for j, finalLine := range finalLines {
			if usedFinalLines[j] {
				continue
			}
			
			trimmedFinal := strings.TrimSpace(finalLine)
			if len(trimmedFinal) == 0 {
				continue
			}
			
			if trimmedSnap == trimmedFinal {
				matched[i] = j
				usedFinalLines[j] = true
				bestMatch = -1
				break
			}
			
			similarity := calculateSimilarity(trimmedSnap, trimmedFinal)
			if similarity >= threshold && similarity > bestSimilarity {
				bestSimilarity = similarity
				bestMatch = j
			}
		}
		
		if bestMatch != -1 && bestSimilarity >= threshold {
			matched[i] = bestMatch
			usedFinalLines[bestMatch] = true
		}
	}
	
	return matched
}

func compressRanges(lineKinds []string) []notes.LineRange {
	if len(lineKinds) == 0 {
		return []notes.LineRange{}
	}
	
	var ranges []notes.LineRange
	currentKind := lineKinds[0]
	startLine := 1
	
	for i := 1; i < len(lineKinds); i++ {
		if lineKinds[i] != currentKind {
			ranges = append(ranges, notes.LineRange{
				Start: startLine,
				End:   i,
				Kind:  currentKind,
			})
			currentKind = lineKinds[i]
			startLine = i + 1
		}
	}
	
	ranges = append(ranges, notes.LineRange{
		Start: startLine,
		End:   len(lineKinds),
		Kind:  currentKind,
	})
	
	return ranges
}

func ComputeForWorkingTree(repoRoot string) (map[string]FileStats, error) {
	changedFiles, err := git.GetChangedFiles()
	if err != nil {
		return nil, err
	}
	
	snapshots, err := snapshot.LoadAll()
	if err != nil {
		return nil, err
	}
	
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no AI snapshots recorded; run 'git ai-trail mark' first")
	}
	
	stats := make(map[string]FileStats)
	
	for _, file := range changedFiles {
		filePath := filepath.Join(repoRoot, file)
		currentContent, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		
		fileStats := computeFileStats(file, string(currentContent), snapshots)
		stats[file] = fileStats
	}
	
	return stats, nil
}

type FileStats struct {
	AI         int
	AIModified int
	Human      int
}

func computeFileStats(path, currentContent string, snapshots []snapshot.Snapshot) FileStats {
	currentLines := strings.Split(currentContent, "\n")
	
	lineKinds := make([]string, len(currentLines))
	for i := range lineKinds {
		lineKinds[i] = "human"
	}
	
	aiSnapshot := make(map[int]string)
	for _, snap := range snapshots {
		if content, ok := snap.Files[path]; ok {
			snapLines := strings.Split(content, "\n")
			
			matched := matchLines(snapLines, currentLines)
			for snapLine, currentLine := range matched {
				if currentLine != -1 {
					aiSnapshot[currentLine] = snapLines[snapLine]
				}
			}
		}
	}
	
	threshold := getFuzzyThreshold()
	
	for lineNum, snapContent := range aiSnapshot {
		if lineNum >= len(currentLines) {
			continue
		}
		
		currentLine := currentLines[lineNum]
		trimmedCurrent := strings.TrimSpace(currentLine)
		trimmedSnap := strings.TrimSpace(snapContent)
		
		if trimmedCurrent == trimmedSnap {
			lineKinds[lineNum] = "ai"
		} else if len(trimmedCurrent) > 0 && len(trimmedSnap) > 0 {
			similarity := calculateSimilarity(trimmedCurrent, trimmedSnap)
			if similarity >= threshold {
				lineKinds[lineNum] = "ai-modified"
			}
		}
	}
	
	stats := FileStats{}
	for _, kind := range lineKinds {
		switch kind {
		case "ai":
			stats.AI++
		case "ai-modified":
			stats.AIModified++
		case "human":
			stats.Human++
		}
	}
	
	return stats
}

func DiffFiles(oldPath, newPath string) ([]string, error) {
	cmd := exec.Command("diff", "-u", oldPath, newPath)
	out, _ := cmd.Output()
	
	var added []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added = append(added, strings.TrimPrefix(line, "+"))
		}
	}
	
	return added, nil
}

func ReadFileLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	
	return lines, scanner.Err()
}
