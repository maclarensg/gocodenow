package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"gocodenow/internal/context"
)

// FuzzyFinderExecutor implements fuzzy file finding with intelligent ranking
type FuzzyFinderExecutor struct {
	contextDetector context.ContextDetector
	maxResults      int
}

// NewFuzzyFinderExecutor creates a new fuzzy finder executor
func NewFuzzyFinderExecutor() *FuzzyFinderExecutor {
	return &FuzzyFinderExecutor{
		contextDetector: context.NewDefaultContextDetector(),
		maxResults:      50,
	}
}

// Execute performs fuzzy file search with intelligent ranking
func (e *FuzzyFinderExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	// Extract parameters
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "query parameter is required",
		}, fmt.Errorf("query parameter missing")
	}
	
	// Get fuzzy search options
	searchPath := e.getStringParam(params, "path", ".")
	caseSensitive := e.getBoolParam(params, "case_sensitive", false)
	includeHidden := e.getBoolParam(params, "include_hidden", false)
	fileTypes := e.getStringSliceParam(params, "file_types", nil)
	excludePatterns := e.getStringSliceParam(params, "exclude", nil)
	maxResults := e.getIntParam(params, "max_results", e.maxResults)
	scoreThreshold := e.getFloatParam(params, "score_threshold", 0.1)
	preferRecent := e.getBoolParam(params, "prefer_recent", true)
	
	// Resolve absolute path
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve path: %v", err),
		}, err
	}
	
	// Detect project context
	projectContext, err := e.contextDetector.DetectContext(absPath)
	if err != nil {
		projectContext = nil
	}
	
	// Perform fuzzy search
	fuzzyResults, stats, err := e.performFuzzySearch(FuzzyOptions{
		Query:           query,
		SearchPath:      absPath,
		CaseSensitive:   caseSensitive,
		IncludeHidden:   includeHidden,
		FileTypes:       fileTypes,
		ExcludePatterns: excludePatterns,
		MaxResults:      maxResults,
		ScoreThreshold:  scoreThreshold,
		PreferRecent:    preferRecent,
		ProjectContext:  projectContext,
	})
	
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("fuzzy search failed: %v", err),
		}, err
	}
	
	// Format results
	resultData := map[string]interface{}{
		"query":          query,
		"search_path":    absPath,
		"total_matches":  len(fuzzyResults),
		"results":        fuzzyResults,
		"execution_time": time.Since(startTime).String(),
		"statistics":     stats,
		"options": map[string]interface{}{
			"case_sensitive":   caseSensitive,
			"include_hidden":   includeHidden,
			"file_types":       fileTypes,
			"exclude":          excludePatterns,
			"max_results":      maxResults,
			"score_threshold":  scoreThreshold,
			"prefer_recent":    preferRecent,
		},
	}
	
	// Generate summary
	summary := e.generateFuzzySummary(query, fuzzyResults, stats, time.Since(startTime))
	
	return &ToolResult{
		Success:   true,
		Result:    resultData,
		Duration:  time.Since(startTime),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"summary":        summary,
			"match_count":    len(fuzzyResults),
			"files_scanned":  stats.FilesScanned,
		},
	}, nil
}

// FuzzyOptions configures fuzzy search behavior
type FuzzyOptions struct {
	Query           string
	SearchPath      string
	CaseSensitive   bool
	IncludeHidden   bool
	FileTypes       []string
	ExcludePatterns []string
	MaxResults      int
	ScoreThreshold  float64
	PreferRecent    bool
	ProjectContext  *context.ProjectContext
}

// FuzzyMatch represents a fuzzy search match with detailed scoring
type FuzzyMatch struct {
	File            string            `json:"file"`
	RelativePath    string            `json:"relative_path"`
	Name            string            `json:"name"`
	Directory       string            `json:"directory"`
	Extension       string            `json:"extension"`
	Language        string            `json:"language,omitempty"`
	Size            int64             `json:"size"`
	ModTime         time.Time         `json:"mod_time"`
	Score           float64           `json:"score"`
	MatchPositions  []int             `json:"match_positions"` // Character positions that matched
	ScoreBreakdown  *ScoreBreakdown   `json:"score_breakdown"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// ScoreBreakdown provides detailed scoring information
type ScoreBreakdown struct {
	BaseScore       float64 `json:"base_score"`        // Fuzzy matching score
	NameBonus       float64 `json:"name_bonus"`        // Bonus for matching filename vs path
	ExtensionBonus  float64 `json:"extension_bonus"`   // Bonus for matching file extension
	RecencyBonus    float64 `json:"recency_bonus"`     // Bonus for recently modified files
	LanguageBonus   float64 `json:"language_bonus"`    // Bonus for primary project language
	DirectoryPenalty float64 `json:"directory_penalty"` // Penalty for deep nested files
	FinalScore      float64 `json:"final_score"`       // Combined final score
}

// FuzzyStatistics tracks fuzzy search performance
type FuzzyStatistics struct {
	FilesScanned     int               `json:"files_scanned"`
	FilesMatched     int               `json:"files_matched"`
	FilesFiltered    int               `json:"files_filtered"`
	SearchDuration   time.Duration     `json:"search_duration"`
	LanguageStats    map[string]int    `json:"language_stats"`
	AverageScore     float64           `json:"average_score"`
	ScoreDistribution map[string]int   `json:"score_distribution"` // score ranges
}

// performFuzzySearch executes the fuzzy search algorithm
func (e *FuzzyFinderExecutor) performFuzzySearch(opts FuzzyOptions) ([]FuzzyMatch, *FuzzyStatistics, error) {
	var allMatches []FuzzyMatch
	stats := &FuzzyStatistics{
		LanguageStats:     make(map[string]int),
		ScoreDistribution: make(map[string]int),
	}
	
	err := filepath.WalkDir(opts.SearchPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if entry.IsDir() {
			return nil
		}
		
		stats.FilesScanned++
		
		// Apply basic filtering
		if !e.shouldProcessFile(path, entry, opts) {
			stats.FilesFiltered++
			return nil
		}
		
		// Perform fuzzy matching
		match, score, positions := e.fuzzyMatch(opts.Query, entry.Name(), path, opts)
		
		if match && score >= opts.ScoreThreshold {
			info, err := entry.Info()
			if err != nil {
				return nil
			}
			
			relPath, _ := filepath.Rel(opts.SearchPath, path)
			language := e.detectLanguage(path, opts.ProjectContext)
			
			// Calculate enhanced score with bonuses/penalties
			scoreBreakdown := e.calculateEnhancedScore(
				score, entry.Name(), path, info, language, opts)
			
			fuzzyMatch := FuzzyMatch{
				File:           path,
				RelativePath:   relPath,
				Name:           entry.Name(),
				Directory:      filepath.Dir(relPath),
				Extension:      filepath.Ext(entry.Name()),
				Language:       language,
				Size:           info.Size(),
				ModTime:        info.ModTime(),
				Score:          scoreBreakdown.FinalScore,
				MatchPositions: positions,
				ScoreBreakdown: scoreBreakdown,
			}
			
			allMatches = append(allMatches, fuzzyMatch)
			stats.FilesMatched++
			
			if language != "" {
				stats.LanguageStats[language]++
			}
		}
		
		return nil
	})
	
	// Sort matches by score (descending)
	sort.Slice(allMatches, func(i, j int) bool {
		return allMatches[i].Score > allMatches[j].Score
	})
	
	// Limit results
	if len(allMatches) > opts.MaxResults {
		allMatches = allMatches[:opts.MaxResults]
	}
	
	// Calculate statistics
	if len(allMatches) > 0 {
		totalScore := 0.0
		for _, match := range allMatches {
			totalScore += match.Score
			
			// Score distribution
			scoreRange := e.getScoreRange(match.Score)
			stats.ScoreDistribution[scoreRange]++
		}
		stats.AverageScore = totalScore / float64(len(allMatches))
	}
	
	return allMatches, stats, err
}

// fuzzyMatch performs the core fuzzy matching algorithm
func (e *FuzzyFinderExecutor) fuzzyMatch(query, filename, fullPath string, opts FuzzyOptions) (bool, float64, []int) {
	if query == "" {
		return true, 1.0, []int{}
	}
	
	// Normalize case if needed
	searchQuery := query
	searchTarget := filename
	if !opts.CaseSensitive {
		searchQuery = strings.ToLower(query)
		searchTarget = strings.ToLower(filename)
	}
	
	// Try exact match first (highest score)
	if strings.Contains(searchTarget, searchQuery) {
		index := strings.Index(searchTarget, searchQuery)
		positions := make([]int, len(searchQuery))
		for i := 0; i < len(searchQuery); i++ {
			positions[i] = index + i
		}
		return true, 1.0, positions
	}
	
	// Try fuzzy matching
	return e.fuzzyMatchAlgorithm(searchQuery, searchTarget)
}

// fuzzyMatchAlgorithm implements a fuzzy string matching algorithm
func (e *FuzzyFinderExecutor) fuzzyMatchAlgorithm(query, target string) (bool, float64, []int) {
	if len(query) == 0 {
		return true, 1.0, []int{}
	}
	
	if len(target) == 0 {
		return false, 0.0, []int{}
	}
	
	queryRunes := []rune(query)
	targetRunes := []rune(target)
	
	var matches []int
	queryIdx := 0
	
	// Find character matches
	for targetIdx, targetRune := range targetRunes {
		if queryIdx < len(queryRunes) && queryRunes[queryIdx] == targetRune {
			matches = append(matches, targetIdx)
			queryIdx++
		}
	}
	
	// Check if all query characters were matched
	if queryIdx != len(queryRunes) {
		return false, 0.0, []int{}
	}
	
	// Calculate score based on match quality
	score := e.calculateFuzzyScore(query, target, matches)
	
	return true, score, matches
}

// calculateFuzzyScore computes a score for fuzzy matches
func (e *FuzzyFinderExecutor) calculateFuzzyScore(query, target string, matches []int) float64 {
	if len(matches) == 0 {
		return 0.0
	}
	
	// Base score: ratio of matched characters to target length
	baseScore := float64(len(matches)) / float64(len(target))
	
	// Bonus for consecutive matches
	consecutiveBonus := 0.0
	consecutiveCount := 0
	for i := 1; i < len(matches); i++ {
		if matches[i] == matches[i-1]+1 {
			consecutiveCount++
		} else {
			if consecutiveCount > 0 {
				consecutiveBonus += float64(consecutiveCount) * 0.1
				consecutiveCount = 0
			}
		}
	}
	if consecutiveCount > 0 {
		consecutiveBonus += float64(consecutiveCount) * 0.1
	}
	
	// Bonus for matching at word boundaries
	boundaryBonus := 0.0
	targetRunes := []rune(target)
	for _, pos := range matches {
		if pos == 0 || !unicode.IsLetter(targetRunes[pos-1]) {
			boundaryBonus += 0.1
		}
	}
	
	// Penalty for gaps between matches
	gapPenalty := 0.0
	if len(matches) > 1 {
		totalGap := matches[len(matches)-1] - matches[0] - len(matches) + 1
		gapPenalty = float64(totalGap) * 0.01
	}
	
	finalScore := baseScore + consecutiveBonus + boundaryBonus - gapPenalty
	
	// Ensure score is between 0 and 1
	if finalScore > 1.0 {
		finalScore = 1.0
	} else if finalScore < 0.0 {
		finalScore = 0.0
	}
	
	return finalScore
}

// calculateEnhancedScore applies various bonuses and penalties to improve ranking
func (e *FuzzyFinderExecutor) calculateEnhancedScore(baseScore float64, filename, fullPath string, info os.FileInfo, language string, opts FuzzyOptions) *ScoreBreakdown {
	breakdown := &ScoreBreakdown{
		BaseScore: baseScore,
	}
	
	// Bonus for matching filename vs full path
	nameQuery := opts.Query
	if !opts.CaseSensitive {
		nameQuery = strings.ToLower(nameQuery)
		filename = strings.ToLower(filename)
	}
	
	if strings.Contains(filename, nameQuery) {
		breakdown.NameBonus = 0.2
	}
	
	// Bonus for matching file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if strings.Contains(strings.ToLower(opts.Query), ext) {
		breakdown.ExtensionBonus = 0.1
	}
	
	// Recency bonus (prefer recently modified files)
	if opts.PreferRecent {
		age := time.Since(info.ModTime())
		if age < 24*time.Hour {
			breakdown.RecencyBonus = 0.15
		} else if age < 7*24*time.Hour {
			breakdown.RecencyBonus = 0.1
		} else if age < 30*24*time.Hour {
			breakdown.RecencyBonus = 0.05
		}
	}
	
	// Language bonus (prefer primary project language)
	if language != "" && opts.ProjectContext != nil && language == opts.ProjectContext.PrimaryLanguage {
		breakdown.LanguageBonus = 0.1
	}
	
	// Directory depth penalty (prefer files closer to root)
	relPath, _ := filepath.Rel(opts.SearchPath, fullPath)
	depth := strings.Count(relPath, string(filepath.Separator))
	if depth > 3 {
		breakdown.DirectoryPenalty = float64(depth-3) * 0.05
	}
	
	// Calculate final score
	breakdown.FinalScore = breakdown.BaseScore + 
		breakdown.NameBonus + 
		breakdown.ExtensionBonus + 
		breakdown.RecencyBonus + 
		breakdown.LanguageBonus - 
		breakdown.DirectoryPenalty
	
	// Ensure score is between 0 and 1
	if breakdown.FinalScore > 1.0 {
		breakdown.FinalScore = 1.0
	} else if breakdown.FinalScore < 0.0 {
		breakdown.FinalScore = 0.0
	}
	
	return breakdown
}

// shouldProcessFile determines if a file should be processed
func (e *FuzzyFinderExecutor) shouldProcessFile(path string, entry os.DirEntry, opts FuzzyOptions) bool {
	name := entry.Name()
	
	// Skip hidden files
	if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
		return false
	}
	
	// Apply exclude patterns
	for _, pattern := range opts.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return false
		}
	}
	
	// Filter by file types
	if len(opts.FileTypes) > 0 {
		ext := strings.ToLower(filepath.Ext(name))
		if ext != "" {
			ext = ext[1:] // Remove dot
		}
		
		found := false
		for _, fileType := range opts.FileTypes {
			if strings.EqualFold(ext, fileType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Apply project context filtering
	if opts.ProjectContext != nil {
		relPath, err := filepath.Rel(opts.ProjectContext.WorkspaceRoot, path)
		if err == nil {
			for _, pattern := range opts.ProjectContext.IgnorePatterns {
				if e.matchesIgnorePattern(relPath, pattern) {
					return false
				}
			}
		}
	}
	
	return true
}

// detectLanguage detects file language from project context
func (e *FuzzyFinderExecutor) detectLanguage(filePath string, projectContext *context.ProjectContext) string {
	if projectContext == nil {
		return ""
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		ext = ext[1:] // Remove dot
		
		for _, lang := range projectContext.Languages {
			for _, langExt := range lang.Extensions {
				if langExt == ext {
					return lang.Name
				}
			}
		}
	}
	
	return ""
}

// matchesIgnorePattern checks if a path matches ignore patterns
func (e *FuzzyFinderExecutor) matchesIgnorePattern(path, pattern string) bool {
	if strings.HasPrefix(pattern, "!") {
		return false
	}
	
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(path, pattern+"/") || path == pattern
	}
	
	if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
		return true
	}
	
	if strings.Contains(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	
	return false
}

// getScoreRange categorizes scores into ranges for statistics
func (e *FuzzyFinderExecutor) getScoreRange(score float64) string {
	switch {
	case score >= 0.9:
		return "excellent (0.9-1.0)"
	case score >= 0.7:
		return "good (0.7-0.9)"
	case score >= 0.5:
		return "fair (0.5-0.7)"
	case score >= 0.3:
		return "poor (0.3-0.5)"
	default:
		return "weak (0.0-0.3)"
	}
}

// generateFuzzySummary creates a summary of fuzzy search results
func (e *FuzzyFinderExecutor) generateFuzzySummary(query string, results []FuzzyMatch, stats *FuzzyStatistics, duration time.Duration) string {
	if len(results) == 0 {
		return fmt.Sprintf("No fuzzy matches found for '%s' (scanned %d files in %v)", 
			query, stats.FilesScanned, duration)
	}
	
	summary := fmt.Sprintf("Found %d fuzzy matches for '%s' (scanned %d files in %v):",
		len(results), query, stats.FilesScanned, duration)
	
	summary += fmt.Sprintf("\n  Average score: %.2f", stats.AverageScore)
	
	// Show score distribution
	if len(stats.ScoreDistribution) > 0 {
		summary += "\n  Score distribution:"
		for scoreRange, count := range stats.ScoreDistribution {
			summary += fmt.Sprintf(" %s:%d", scoreRange, count)
		}
	}
	
	// Show language breakdown
	if len(stats.LanguageStats) > 0 {
		summary += "\n  Languages:"
		for lang, count := range stats.LanguageStats {
			summary += fmt.Sprintf(" %s(%d)", lang, count)
		}
	}
	
	// Show top matches
	topCount := 5
	if len(results) < topCount {
		topCount = len(results)
	}
	
	summary += fmt.Sprintf("\n  Top %d matches:", topCount)
	for i := 0; i < topCount; i++ {
		match := results[i]
		summary += fmt.Sprintf("\n    %s (%.2f)", match.RelativePath, match.Score)
	}
	
	return summary
}

// Utility methods
func (e *FuzzyFinderExecutor) getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func (e *FuzzyFinderExecutor) getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

func (e *FuzzyFinderExecutor) getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func (e *FuzzyFinderExecutor) getFloatParam(params map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := params[key].(float64); ok {
		return val
	}
	if val, ok := params[key].(int); ok {
		return float64(val)
	}
	return defaultValue
}

func (e *FuzzyFinderExecutor) getStringSliceParam(params map[string]interface{}, key string, defaultValue []string) []string {
	if val, ok := params[key].([]interface{}); ok {
		result := make([]string, len(val))
		for i, v := range val {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	if val, ok := params[key].([]string); ok {
		return val
	}
	return defaultValue
}

// GetSchema returns the tool schema for the fuzzy finder executor
func (e *FuzzyFinderExecutor) GetSchema() *ToolSchema {
	return &ToolSchema{
		Name:        "fuzzy_find",
		Description: "Intelligent fuzzy file search with advanced scoring and ranking based on relevance, recency, and project context",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Fuzzy search query (partial filename or pattern)",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory to search in (default: current directory)",
					"default":     ".",
				},
				"case_sensitive": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether search should be case sensitive",
					"default":     false,
				},
				"include_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files in search",
					"default":     false,
				},
				"file_types": map[string]interface{}{
					"type":        "array",
					"description": "Filter by file extensions",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"exclude": map[string]interface{}{
					"type":        "array",
					"description": "Patterns to exclude from search",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return",
					"default":     50,
				},
				"score_threshold": map[string]interface{}{
					"type":        "number",
					"description": "Minimum score threshold for matches (0.0-1.0)",
					"default":     0.1,
				},
				"prefer_recent": map[string]interface{}{
					"type":        "boolean",
					"description": "Give bonus points to recently modified files",
					"default":     true,
				},
			},
			"required": []string{"query"},
		},
	}
}