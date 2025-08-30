package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gocodenow/internal/context"
)

// FindInFilesExecutor implements content search across project files
type FindInFilesExecutor struct {
	contextDetector context.ContextDetector
	maxResults      int
	maxFileSize     int64 // Skip files larger than this (in bytes)
}

// NewFindInFilesExecutor creates a new content search executor
func NewFindInFilesExecutor() *FindInFilesExecutor {
	return &FindInFilesExecutor{
		contextDetector: context.NewDefaultContextDetector(),
		maxResults:      1000,
		maxFileSize:     10 * 1024 * 1024, // 10MB
	}
}

// Execute performs content search across files
func (e *FindInFilesExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	// Extract parameters
	pattern, ok := params["pattern"].(string)
	if !ok || pattern == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "pattern parameter is required",
		}, fmt.Errorf("pattern parameter missing")
	}
	
	// Get search options
	searchPath := e.getStringParam(params, "path", ".")
	caseSensitive := e.getBoolParam(params, "case_sensitive", false)
	useRegex := e.getBoolParam(params, "regex", false)
	wholeWord := e.getBoolParam(params, "whole_word", false)
	includeHidden := e.getBoolParam(params, "include_hidden", false)
	fileTypes := e.getStringSliceParam(params, "file_types", nil)
	excludePatterns := e.getStringSliceParam(params, "exclude", nil)
	maxResults := e.getIntParam(params, "max_results", e.maxResults)
	contextLines := e.getIntParam(params, "context", 0)
	
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
	
	// Perform content search
	searchResults, stats, err := e.searchContent(ContentSearchOptions{
		Pattern:         pattern,
		SearchPath:      absPath,
		CaseSensitive:   caseSensitive,
		UseRegex:        useRegex,
		WholeWord:       wholeWord,
		IncludeHidden:   includeHidden,
		FileTypes:       fileTypes,
		ExcludePatterns: excludePatterns,
		MaxResults:      maxResults,
		ContextLines:    contextLines,
		ProjectContext:  projectContext,
	})
	
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("content search failed: %v", err),
		}, err
	}
	
	// Format results
	resultData := map[string]interface{}{
		"pattern":        pattern,
		"search_path":    absPath,
		"total_matches":  stats.TotalMatches,
		"files_searched": stats.FilesSearched,
		"files_matched":  stats.FilesWithMatches,
		"matches":        searchResults,
		"execution_time": time.Since(startTime).String(),
		"statistics":     stats,
		"options": map[string]interface{}{
			"case_sensitive": caseSensitive,
			"regex":          useRegex,
			"whole_word":     wholeWord,
			"include_hidden": includeHidden,
			"file_types":     fileTypes,
			"exclude":        excludePatterns,
			"max_results":    maxResults,
			"context":        contextLines,
		},
	}
	
	// Generate summary
	summary := e.generateContentSearchSummary(pattern, searchResults, stats, time.Since(startTime))
	
	return &ToolResult{
		Success:   true,
		Result:    resultData,
		Duration:  time.Since(startTime),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"summary":        summary,
			"match_count":    stats.TotalMatches,
			"file_count":     stats.FilesWithMatches,
			"files_searched": stats.FilesSearched,
		},
	}, nil
}

// ContentSearchOptions configures content search behavior
type ContentSearchOptions struct {
	Pattern         string
	SearchPath      string
	CaseSensitive   bool
	UseRegex        bool
	WholeWord       bool
	IncludeHidden   bool
	FileTypes       []string
	ExcludePatterns []string
	MaxResults      int
	ContextLines    int
	ProjectContext  *context.ProjectContext
}

// ContentMatch represents a content match with context
type ContentMatch struct {
	File         string               `json:"file"`
	RelativePath string               `json:"relative_path"`
	Language     string               `json:"language,omitempty"`
	Matches      []LineMatch          `json:"matches"`
	MatchCount   int                  `json:"match_count"`
	FileSize     int64                `json:"file_size"`
	ModTime      time.Time            `json:"mod_time"`
	Metadata     map[string]string    `json:"metadata,omitempty"`
}

// LineMatch represents a single line match with context
type LineMatch struct {
	LineNumber    int      `json:"line_number"`
	Content       string   `json:"content"`
	MatchStart    int      `json:"match_start"`
	MatchEnd      int      `json:"match_end"`
	MatchedText   string   `json:"matched_text"`
	ContextBefore []string `json:"context_before,omitempty"`
	ContextAfter  []string `json:"context_after,omitempty"`
}

// SearchStatistics tracks search performance and results
type SearchStatistics struct {
	FilesSearched    int           `json:"files_searched"`
	FilesWithMatches int           `json:"files_with_matches"`
	FilesSkipped     int           `json:"files_skipped"`
	TotalMatches     int           `json:"total_matches"`
	LargestFile      string        `json:"largest_file,omitempty"`
	SearchDuration   time.Duration `json:"search_duration"`
	LanguageStats    map[string]int `json:"language_stats,omitempty"`
}

// searchContent performs the actual content search
func (e *FindInFilesExecutor) searchContent(opts ContentSearchOptions) ([]ContentMatch, *SearchStatistics, error) {
	var results []ContentMatch
	stats := &SearchStatistics{
		LanguageStats: make(map[string]int),
	}
	
	var searchRegex *regexp.Regexp
	var err error
	
	// Prepare search pattern
	searchPattern := opts.Pattern
	if opts.WholeWord && !opts.UseRegex {
		searchPattern = `\b` + regexp.QuoteMeta(searchPattern) + `\b`
		opts.UseRegex = true
	}
	
	if opts.UseRegex {
		regexPattern := searchPattern
		if !opts.CaseSensitive {
			regexPattern = "(?i)" + regexPattern
		}
		
		searchRegex, err = regexp.Compile(regexPattern)
		if err != nil {
			return nil, stats, fmt.Errorf("invalid regex pattern: %v", err)
		}
	}
	
	err = filepath.WalkDir(opts.SearchPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if entry.IsDir() {
			return nil
		}
		
		// Apply filters
		if !e.shouldSearchFile(path, entry, opts) {
			stats.FilesSkipped++
			return nil
		}
		
		// Search file content
		fileMatches, err := e.searchFileContent(path, opts, searchRegex)
		if err != nil {
			stats.FilesSkipped++
			return nil // Continue on file errors
		}
		
		stats.FilesSearched++
		
		if len(fileMatches.Matches) > 0 {
			stats.FilesWithMatches++
			stats.TotalMatches += fileMatches.MatchCount
			
			// Track language statistics
			if fileMatches.Language != "" {
				stats.LanguageStats[fileMatches.Language]++
			}
			
			results = append(results, *fileMatches)
			
			// Stop if we've reached max results
			if stats.TotalMatches >= opts.MaxResults {
				return filepath.SkipAll
			}
		}
		
		return nil
	})
	
	return results, stats, err
}

// shouldSearchFile determines if a file should be searched
func (e *FindInFilesExecutor) shouldSearchFile(path string, entry os.DirEntry, opts ContentSearchOptions) bool {
	// Skip hidden files if not requested
	if !opts.IncludeHidden && strings.HasPrefix(entry.Name(), ".") {
		return false
	}
	
	// Check file size
	if info, err := entry.Info(); err == nil {
		if info.Size() > e.maxFileSize {
			return false
		}
	}
	
	// Apply exclude patterns
	for _, exclude := range opts.ExcludePatterns {
		if matched, _ := filepath.Match(exclude, entry.Name()); matched {
			return false
		}
	}
	
	// Filter by file types
	if len(opts.FileTypes) > 0 {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != "" {
			ext = ext[1:] // Remove the dot
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
	
	// Apply project-aware filtering
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
	
	// Skip binary files (basic check)
	if e.isBinaryFile(entry.Name()) {
		return false
	}
	
	return true
}

// searchFileContent searches for pattern matches within a file
func (e *FindInFilesExecutor) searchFileContent(filePath string, opts ContentSearchOptions, searchRegex *regexp.Regexp) (*ContentMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	info, _ := file.Stat()
	relPath, _ := filepath.Rel(opts.SearchPath, filePath)
	
	// Detect language
	language := e.detectLanguage(filePath, opts.ProjectContext)
	
	match := &ContentMatch{
		File:         filePath,
		RelativePath: relPath,
		Language:     language,
		Matches:      []LineMatch{},
		FileSize:     info.Size(),
		ModTime:      info.ModTime(),
	}
	
	scanner := bufio.NewScanner(file)
	lineNumber := 1
	var lines []string // Store lines for context
	
	// Read all lines first if context is needed
	if opts.ContextLines > 0 {
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		
		// Search through stored lines
		for i, line := range lines {
			lineMatches := e.findLineMatches(line, opts, searchRegex)
			
			for _, lineMatch := range lineMatches {
				lineMatch.LineNumber = i + 1
				
				// Add context lines
				if opts.ContextLines > 0 {
					// Context before
					startIdx := i - opts.ContextLines
					if startIdx < 0 {
						startIdx = 0
					}
					for j := startIdx; j < i; j++ {
						lineMatch.ContextBefore = append(lineMatch.ContextBefore, lines[j])
					}
					
					// Context after
					endIdx := i + opts.ContextLines + 1
					if endIdx > len(lines) {
						endIdx = len(lines)
					}
					for j := i + 1; j < endIdx; j++ {
						lineMatch.ContextAfter = append(lineMatch.ContextAfter, lines[j])
					}
				}
				
				match.Matches = append(match.Matches, lineMatch)
			}
		}
	} else {
		// Stream processing for better memory usage
		for scanner.Scan() {
			line := scanner.Text()
			lineMatches := e.findLineMatches(line, opts, searchRegex)
			
			for _, lineMatch := range lineMatches {
				lineMatch.LineNumber = lineNumber
				match.Matches = append(match.Matches, lineMatch)
			}
			
			lineNumber++
		}
	}
	
	match.MatchCount = len(match.Matches)
	return match, scanner.Err()
}

// findLineMatches finds all matches within a single line
func (e *FindInFilesExecutor) findLineMatches(line string, opts ContentSearchOptions, searchRegex *regexp.Regexp) []LineMatch {
	var matches []LineMatch
	
	if opts.UseRegex && searchRegex != nil {
		// Regex matching
		regexMatches := searchRegex.FindAllStringIndex(line, -1)
		for _, match := range regexMatches {
			matches = append(matches, LineMatch{
				Content:     line,
				MatchStart:  match[0],
				MatchEnd:    match[1],
				MatchedText: line[match[0]:match[1]],
			})
		}
	} else {
		// Simple string matching
		searchText := opts.Pattern
		targetText := line
		
		if !opts.CaseSensitive {
			searchText = strings.ToLower(searchText)
			targetText = strings.ToLower(line)
		}
		
		index := 0
		for {
			pos := strings.Index(targetText[index:], searchText)
			if pos == -1 {
				break
			}
			
			actualPos := index + pos
			matches = append(matches, LineMatch{
				Content:     line,
				MatchStart:  actualPos,
				MatchEnd:    actualPos + len(opts.Pattern),
				MatchedText: line[actualPos : actualPos+len(opts.Pattern)],
			})
			
			index = actualPos + 1
		}
	}
	
	return matches
}

// detectLanguage attempts to detect the programming language of a file
func (e *FindInFilesExecutor) detectLanguage(filePath string, projectContext *context.ProjectContext) string {
	if projectContext == nil {
		return ""
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		ext = ext[1:] // Remove the dot
		
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

// isBinaryFile checks if a file is likely binary based on extension
func (e *FindInFilesExecutor) isBinaryFile(filename string) bool {
	binaryExtensions := []string{
		".exe", ".dll", ".so", ".dylib", ".a", ".lib", ".bin", ".obj", ".o",
		".class", ".jar", ".war", ".zip", ".tar", ".gz", ".rar", ".7z",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".ico",
		".mp3", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".pdf",
		".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	}
	
	ext := strings.ToLower(filepath.Ext(filename))
	for _, binaryExt := range binaryExtensions {
		if ext == binaryExt {
			return true
		}
	}
	
	return false
}

// matchesIgnorePattern checks if a path matches an ignore pattern
func (e *FindInFilesExecutor) matchesIgnorePattern(path, pattern string) bool {
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

// generateContentSearchSummary creates a human-readable summary
func (e *FindInFilesExecutor) generateContentSearchSummary(pattern string, results []ContentMatch, stats *SearchStatistics, duration time.Duration) string {
	if stats.TotalMatches == 0 {
		return fmt.Sprintf("No matches found for '%s' (searched %d files in %v)", pattern, stats.FilesSearched, duration)
	}
	
	summary := fmt.Sprintf("Found %d matches in %d files (searched %d files in %v):", 
		stats.TotalMatches, stats.FilesWithMatches, stats.FilesSearched, duration)
	
	// Add language breakdown
	if len(stats.LanguageStats) > 0 {
		summary += "\n  Languages:"
		for lang, count := range stats.LanguageStats {
			summary += fmt.Sprintf(" %s(%d)", lang, count)
		}
	}
	
	// Show top files with most matches
	if len(results) > 0 {
		// Sort by match count
		sortedResults := make([]ContentMatch, len(results))
		copy(sortedResults, results)
		
		// Simple sort by match count
		for i := 0; i < len(sortedResults)-1; i++ {
			for j := i + 1; j < len(sortedResults); j++ {
				if sortedResults[j].MatchCount > sortedResults[i].MatchCount {
					sortedResults[i], sortedResults[j] = sortedResults[j], sortedResults[i]
				}
			}
		}
		
		topCount := 5
		if len(sortedResults) < topCount {
			topCount = len(sortedResults)
		}
		
		summary += fmt.Sprintf("\n  Top %d files:", topCount)
		for i := 0; i < topCount; i++ {
			result := sortedResults[i]
			summary += fmt.Sprintf("\n    %s (%d matches)", result.RelativePath, result.MatchCount)
		}
	}
	
	return summary
}

// Utility methods (same as SearchFilesExecutor)
func (e *FindInFilesExecutor) getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func (e *FindInFilesExecutor) getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

func (e *FindInFilesExecutor) getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func (e *FindInFilesExecutor) getStringSliceParam(params map[string]interface{}, key string, defaultValue []string) []string {
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

// GetSchema returns the tool schema for the find in files executor
func (e *FindInFilesExecutor) GetSchema() *ToolSchema {
	return &ToolSchema{
		Name:        "find_in_files",
		Description: "Search for text content within files across the project with context-aware filtering",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Text pattern to search for (string or regex)",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory to search in (default: current directory)",
					"default":     ".",
				},
				"case_sensitive": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the search should be case sensitive",
					"default":     false,
				},
				"regex": map[string]interface{}{
					"type":        "boolean",
					"description": "Treat pattern as a regular expression",
					"default":     false,
				},
				"whole_word": map[string]interface{}{
					"type":        "boolean",
					"description": "Match whole words only",
					"default":     false,
				},
				"include_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files (starting with .)",
					"default":     false,
				},
				"file_types": map[string]interface{}{
					"type":        "array",
					"description": "Filter by file extensions (without dots)",
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
					"description": "Maximum number of matches to return",
					"default":     1000,
				},
				"context": map[string]interface{}{
					"type":        "integer",
					"description": "Number of context lines to show around matches",
					"default":     0,
				},
			},
			"required": []string{"pattern"},
		},
	}
}