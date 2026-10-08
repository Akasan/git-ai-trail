package attribution

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Akasan/git-ai-trail/internal/git"
	"github.com/Akasan/git-ai-trail/internal/notes"
	"github.com/Akasan/git-ai-trail/internal/snapshot"
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

	sort.Strings(files)

	attr := &notes.Attribution{
		SchemaVersion: notes.SchemaVersion,
		ToolVersion:   "0.1.0",
		Files:         make(map[string]notes.FileAttribution),
		Marks:         make([]notes.MarkInfo, 0),
	}

	for _, snap := range snapshots {
		attr.Marks = append(attr.Marks, notes.MarkInfo{
			Timestamp:   snap.Timestamp.UTC().Format(time.RFC3339),
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

	finalContent = strings.TrimSuffix(finalContent, "\n")
	finalLines := strings.Split(finalContent, "\n")
	if len(finalLines) == 1 && finalLines[0] == "" {
		finalLines = []string{}
	}

	addedLines, err := git.GetDiffAddedLines(commit, path)
	if err != nil {
		return notes.FileAttribution{}, err
	}

	lineKinds := make([]string, len(finalLines))
	for i := range lineKinds {
		if !addedLines[i+1] {
			lineKinds[i] = "unchanged"
		} else {
			lineKinds[i] = "human"
		}
	}

	threshold := getFuzzyThreshold()

	for _, snap := range snapshots {
		snapContent, ok := snap.Files[path]
		if !ok {
			continue
		}

		snapContent = strings.TrimSuffix(snapContent, "\n")
		snapLines := strings.Split(snapContent, "\n")
		if len(snapLines) == 1 && snapLines[0] == "" {
			snapLines = []string{}
		}

		matched := matchLinesWithLCS(snapLines, finalLines, threshold)

		for snapIdx, finalIdx := range matched {
			if finalIdx == -1 || finalIdx >= len(finalLines) {
				continue
			}

			if !addedLines[finalIdx+1] {
				continue
			}

			snapLine := snapLines[snapIdx]
			finalLine := finalLines[finalIdx]

			if strings.TrimSpace(snapLine) == strings.TrimSpace(finalLine) {
				lineKinds[finalIdx] = "ai"
			} else {
				similarity := levenshteinSimilarity(snapLine, finalLine)
				if similarity >= threshold {
					lineKinds[finalIdx] = "ai-modified"
				}
			}
		}
	}

	ranges := compressRangesExcludingUnchanged(lineKinds)

	return notes.FileAttribution{
		Path:   path,
		Ranges: ranges,
	}, nil
}

func matchLinesWithLCS(snapLines, finalLines []string, threshold float64) map[int]int {
	matched := make(map[int]int)

	if len(snapLines) == 0 || len(finalLines) == 0 {
		return matched
	}

	lcs := computeLCS(snapLines, finalLines, threshold)

	for i := range lcs {
		if lcs[i][0] >= 0 && lcs[i][1] >= 0 {
			matched[lcs[i][0]] = lcs[i][1]
		}
	}

	return matched
}

func computeLCS(a, b []string, threshold float64) [][2]int {
	m, n := len(a), len(b)
	
	simCache := make(map[[2]int]float64)
	getSimilarity := func(i, j int) float64 {
		key := [2]int{i, j}
		if sim, ok := simCache[key]; ok {
			return sim
		}
		
		s1, s2 := a[i], b[j]
		
		if s1 == s2 {
			simCache[key] = 1.0
			return 1.0
		}
		
		len1, len2 := len(s1), len(s2)
		if len1 == 0 || len2 == 0 {
			simCache[key] = 0.0
			return 0.0
		}
		
		lenDiff := len1 - len2
		if lenDiff < 0 {
			lenDiff = -lenDiff
		}
		maxLen := len1
		if len2 > maxLen {
			maxLen = len2
		}
		if float64(lenDiff)/float64(maxLen) > (1.0 - threshold) {
			simCache[key] = 0.0
			return 0.0
		}
		
		sim := levenshteinSimilarity(s1, s2)
		simCache[key] = sim
		return sim
	}
	
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			sim := getSimilarity(i-1, j-1)
			if sim >= threshold {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	var result [][2]int
	i, j := m, n
	for i > 0 && j > 0 {
		if getSimilarity(i-1, j-1) >= threshold {
			result = append([][2]int{{i - 1, j - 1}}, result...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return result
}

func levenshteinSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	dist := levenshteinDistance(s1, s2)
	maxLen := max(len(s1), len(s2))
	return 1.0 - float64(dist)/float64(maxLen)
}

func levenshteinDistance(s1, s2 string) int {
	m, n := len(s1), len(s2)
	if m == 0 {
		return n
	}
	if n == 0 {
		return m
	}

	prev := make([]int, n+1)
	curr := make([]int, n+1)

	for j := 0; j <= n; j++ {
		prev[j] = j
	}

	for i := 1; i <= m; i++ {
		curr[0] = i
		for j := 1; j <= n; j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			curr[j] = min(
				min(curr[j-1]+1, prev[j]+1),
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[n]
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

func compressRangesExcludingUnchanged(lineKinds []string) []notes.LineRange {
	if len(lineKinds) == 0 {
		return []notes.LineRange{}
	}

	var ranges []notes.LineRange
	var currentKind string
	var startLine int

	for i := 0; i < len(lineKinds); i++ {
		if lineKinds[i] == "unchanged" {
			if currentKind != "" && currentKind != "unchanged" {
				ranges = append(ranges, notes.LineRange{
					Start: startLine,
					End:   i,
					Kind:  currentKind,
				})
				currentKind = ""
			}
			continue
		}

		if currentKind == "" || currentKind == "unchanged" {
			currentKind = lineKinds[i]
			startLine = i + 1
		} else if lineKinds[i] != currentKind {
			ranges = append(ranges, notes.LineRange{
				Start: startLine,
				End:   i,
				Kind:  currentKind,
			})
			currentKind = lineKinds[i]
			startLine = i + 1
		}
	}

	if currentKind != "" && currentKind != "unchanged" {
		ranges = append(ranges, notes.LineRange{
			Start: startLine,
			End:   len(lineKinds),
			Kind:  currentKind,
		})
	}

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
		return nil, fmt.Errorf("no AI snapshots")
	}

	stats := make(map[string]FileStats)
	sort.Strings(changedFiles)

	for _, file := range changedFiles {
		filePath := filepath.Join(repoRoot, file)
		currentContent, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		fileStats := computeFileStats(file, string(currentContent), snapshots, repoRoot)
		stats[file] = fileStats
	}

	return stats, nil
}

type FileStats struct {
	AI         int
	AIModified int
	Human      int
}

func computeFileStats(path, currentContent string, snapshots []snapshot.Snapshot, repoRoot string) FileStats {
	currentContent = strings.TrimSuffix(currentContent, "\n")
	currentLines := strings.Split(currentContent, "\n")
	if len(currentLines) == 1 && currentLines[0] == "" {
		currentLines = []string{}
	}

	headContent, err := git.ShowFile("HEAD", path)
	var addedLines map[int]bool
	if err == nil {
		addedLines = computeWorkingTreeAddedLines(string(headContent), currentContent)
	} else {
		addedLines = make(map[int]bool)
		for i := 1; i <= len(currentLines); i++ {
			addedLines[i] = true
		}
	}

	lineKinds := make([]string, len(currentLines))
	for i := range lineKinds {
		if !addedLines[i+1] {
			lineKinds[i] = "unchanged"
		} else {
			lineKinds[i] = "human"
		}
	}

	threshold := getFuzzyThreshold()

	for _, snap := range snapshots {
		snapContent, ok := snap.Files[path]
		if !ok {
			continue
		}

		snapContent = strings.TrimSuffix(snapContent, "\n")
		snapLines := strings.Split(snapContent, "\n")
		if len(snapLines) == 1 && snapLines[0] == "" {
			snapLines = []string{}
		}

		matched := matchLinesWithLCS(snapLines, currentLines, threshold)

		for snapIdx, currentIdx := range matched {
			if currentIdx == -1 || currentIdx >= len(currentLines) {
				continue
			}

			if !addedLines[currentIdx+1] {
				continue
			}

			snapLine := snapLines[snapIdx]
			currentLine := currentLines[currentIdx]

			if strings.TrimSpace(snapLine) == strings.TrimSpace(currentLine) {
				lineKinds[currentIdx] = "ai"
			} else {
				similarity := levenshteinSimilarity(snapLine, currentLine)
				if similarity >= threshold {
					lineKinds[currentIdx] = "ai-modified"
				}
			}
		}
	}

	stats := FileStats{}
	for i, kind := range lineKinds {
		if kind == "unchanged" {
			continue
		}
		if !addedLines[i+1] {
			continue
		}

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

func computeWorkingTreeAddedLines(headContent, currentContent string) map[int]bool {
	headContent = strings.TrimSuffix(headContent, "\n")
	currentContent = strings.TrimSuffix(currentContent, "\n")

	headLines := strings.Split(headContent, "\n")
	currentLines := strings.Split(currentContent, "\n")

	if len(headLines) == 1 && headLines[0] == "" {
		headLines = []string{}
	}
	if len(currentLines) == 1 && currentLines[0] == "" {
		currentLines = []string{}
	}

	added := make(map[int]bool)

	matched := make(map[int]bool)
	for _, headLine := range headLines {
		for j, currentLine := range currentLines {
			if matched[j] {
				continue
			}
			if headLine == currentLine {
				matched[j] = true
				break
			}
		}
	}

	for i := 0; i < len(currentLines); i++ {
		if !matched[i] {
			added[i+1] = true
		}
	}

	return added
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
