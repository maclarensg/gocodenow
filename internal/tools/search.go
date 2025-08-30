// Package search provides advanced file and content search capabilities with project context awareness
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gocodenow/internal/context"
)

// SearchFilesExecutor implements project-aware file search with regex and glob patterns
type SearchFilesExecutor struct {
	contextDetector context.ContextDetector
	maxResults      int
	maxDepth        int
}

// NewSearchFilesExecutor creates a new file search executor
func NewSearchFilesExecutor() *SearchFilesExecutor {
	return &SearchFilesExecutor{
		contextDetector: context.NewDefaultContextDetector(),
		maxResults:      100,
		maxDepth:        20,
	}
}

// Execute performs file search with pattern matching
func (e *SearchFilesExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
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
	includeHidden := e.getBoolParam(params, "include_hidden", false)
	fileTypes := e.getStringSliceParam(params, "file_types", nil)
	excludePatterns := e.getStringSliceParam(params, "exclude", nil)
	maxResults := e.getIntParam(params, "max_results", e.maxResults)
	
	// Resolve absolute path
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve path: %v", err),
		}, err
	}
	
	// Detect project context for smart filtering
	projectContext, err := e.contextDetector.DetectContext(absPath)
	if err != nil {
		// Continue without context if detection fails
		projectContext = nil
	}
	
	// Perform file search
	searchResults, err := e.searchFiles(SearchOptions{
		Pattern:         pattern,
		SearchPath:      absPath,
		CaseSensitive:   caseSensitive,
		UseRegex:        useRegex,
		IncludeHidden:   includeHidden,
		FileTypes:       fileTypes,
		ExcludePatterns: excludePatterns,
		MaxResults:      maxResults,
		ProjectContext:  projectContext,
	})
	
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("search failed: %v", err),
		}, err
	}
	
	// Format results
	resultData := map[string]interface{}{
		"pattern":        pattern,
		"search_path":    absPath,
		"total_matches":  len(searchResults),
		"files":          searchResults,
		"execution_time": time.Since(startTime).String(),
		"options": map[string]interface{}{
			"case_sensitive": caseSensitive,
			"regex":          useRegex,
			"include_hidden": includeHidden,
			"file_types":     fileTypes,
			"exclude":        excludePatterns,
			"max_results":    maxResults,
		},
	}
	
	// Generate summary
	summary := e.generateSearchSummary(pattern, searchResults, time.Since(startTime))
	
	return &ToolResult{
		Success:   true,
		Result:    resultData,
		Duration:  time.Since(startTime),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"summary":      summary,
			"result_count": len(searchResults),
		},
	}, nil
}

// SearchOptions configures file search behavior
type SearchOptions struct {
	Pattern         string
	SearchPath      string
	CaseSensitive   bool
	UseRegex        bool
	IncludeHidden   bool
	FileTypes       []string
	ExcludePatterns []string
	MaxResults      int
	ProjectContext  *context.ProjectContext
}

// FileMatch represents a matched file with metadata
type FileMatch struct {
	Path         string            `json:"path"`
	RelativePath string            `json:"relative_path"`
	Name         string            `json:"name"`
	Directory    string            `json:"directory"`
	Size         int64             `json:"size"`
	ModTime      time.Time         `json:"mod_time"`
	Extension    string            `json:"extension"`
	Language     string            `json:"language,omitempty"`
	Score        float64           `json:"score"`
	MatchType    string            `json:"match_type"` // "exact", "partial", "regex", "glob"
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// searchFiles performs the actual file search with pattern matching
func (e *SearchFilesExecutor) searchFiles(opts SearchOptions) ([]FileMatch, error) {
	var matches []FileMatch
	var searchPattern *regexp.Regexp
	
	// Compile regex pattern if needed
	if opts.UseRegex {
		regexPattern := opts.Pattern
		if !opts.CaseSensitive {
			regexPattern = "(?i)" + regexPattern
		}
		
		var err error
		searchPattern, err = regexp.Compile(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %v", err)
		}
	}
	
	err := filepath.WalkDir(opts.SearchPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		// Skip directories
		if entry.IsDir() {
			return nil
		}
		
		// Apply project-aware filtering
		if opts.ProjectContext != nil && e.shouldIgnoreFile(path, opts.ProjectContext) {
			return nil
		}
		
		// Skip hidden files if not requested
		if !opts.IncludeHidden && strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		
		// Apply exclude patterns
		for _, exclude := range opts.ExcludePatterns {
			if matched, _ := filepath.Match(exclude, entry.Name()); matched {
				return nil
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
				return nil
			}
		}
		
		// Check if file matches pattern
		match, matchType, score := e.matchesPattern(entry.Name(), opts.Pattern, opts.CaseSensitive, opts.UseRegex, searchPattern)
		if !match {
			return nil
		}
		
		// Get file info
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		
		relPath, _ := filepath.Rel(opts.SearchPath, path)
		
		// Detect language if project context is available
		language := ""
		if opts.ProjectContext != nil {
			for _, lang := range opts.ProjectContext.Languages {
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if ext != "" {
					ext = ext[1:] // Remove the dot
					for _, langExt := range lang.Extensions {
						if langExt == ext {
							language = lang.Name
							break
						}
					}
				}
				if language != "" {
					break
				}
			}
		}
		
		fileMatch := FileMatch{
			Path:         path,
			RelativePath: relPath,
			Name:         entry.Name(),
			Directory:    filepath.Dir(relPath),
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			Extension:    filepath.Ext(entry.Name()),
			Language:     language,
			Score:        score,
			MatchType:    matchType,
		}
		
		matches = append(matches, fileMatch)
		
		// Stop if we've reached max results
		if len(matches) >= opts.MaxResults {
			return filepath.SkipAll
		}
		
		return nil
	})
	
	// Sort results by score (descending) then by path
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].RelativePath < matches[j].RelativePath
	})
	
	return matches, err
}

// matchesPattern determines if a filename matches the search pattern
func (e *SearchFilesExecutor) matchesPattern(filename, pattern string, caseSensitive, useRegex bool, compiledRegex *regexp.Regexp) (bool, string, float64) {
	if !caseSensitive {
		filename = strings.ToLower(filename)
		pattern = strings.ToLower(pattern)
	}
	
	// Exact match (highest score)
	if filename == pattern {
		return true, "exact", 1.0
	}
	
	// Regex match
	if useRegex && compiledRegex != nil {
		if compiledRegex.MatchString(filename) {
			return true, "regex", 0.9
		}
		return false, "", 0.0
	}
	
	// Glob pattern match
	if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") {
		if matched, _ := filepath.Match(pattern, filename); matched {
			return true, "glob", 0.8
		}
	}
	
	// Substring match
	if strings.Contains(filename, pattern) {
		// Calculate score based on match position and length
		index := strings.Index(filename, pattern)
		score := 0.7
		
		// Bonus for matching at the beginning
		if index == 0 {
			score += 0.2
		}
		
		// Bonus for longer matches relative to filename
		matchRatio := float64(len(pattern)) / float64(len(filename))
		score += matchRatio * 0.1
		
		return true, "partial", score
	}
	
	return false, "", 0.0
}

// shouldIgnoreFile checks if a file should be ignored based on project context
func (e *SearchFilesExecutor) shouldIgnoreFile(filePath string, projectContext *context.ProjectContext) bool {
	if projectContext.Git.IsRepo {
		// Check against ignore patterns
		relPath, err := filepath.Rel(projectContext.WorkspaceRoot, filePath)
		if err != nil {
			return false
		}
		
		for _, pattern := range projectContext.IgnorePatterns {
			if e.matchesIgnorePattern(relPath, pattern) {
				return true
			}
		}
	}
	
	return false
}

// matchesIgnorePattern checks if a path matches an ignore pattern (simplified gitignore-style matching)
func (e *SearchFilesExecutor) matchesIgnorePattern(path, pattern string) bool {
	// Skip negation patterns for now (would need more complex logic)
	if strings.HasPrefix(pattern, "!") {
		return false
	}
	
	// Directory patterns
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(path, pattern+"/") || path == pattern
	}
	
	// Glob matching
	if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
		return true
	}
	
	// Path prefix matching
	if strings.Contains(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	
	return false
}

// generateSearchSummary creates a human-readable summary of search results
func (e *SearchFilesExecutor) generateSearchSummary(pattern string, results []FileMatch, duration time.Duration) string {
	if len(results) == 0 {
		return fmt.Sprintf("No files found matching pattern '%s' (search took %v)", pattern, duration)
	}
	
	// Group results by match type
	matchTypeCounts := make(map[string]int)
	languageCounts := make(map[string]int)
	
	for _, result := range results {
		matchTypeCounts[result.MatchType]++
		if result.Language != "" {
			languageCounts[result.Language]++
		}
	}
	
	summary := fmt.Sprintf("Found %d files matching '%s' in %v:", len(results), pattern, duration)
	
	// Add match type breakdown
	if len(matchTypeCounts) > 1 {
		summary += "\n  Match types:"
		for matchType, count := range matchTypeCounts {
			summary += fmt.Sprintf(" %s(%d)", matchType, count)
		}
	}
	
	// Add language breakdown if available
	if len(languageCounts) > 0 {
		summary += "\n  Languages:"
		for lang, count := range languageCounts {
			summary += fmt.Sprintf(" %s(%d)", lang, count)
		}
	}
	
	// Show top results
	topCount := 5
	if len(results) < topCount {
		topCount = len(results)
	}
	
	summary += fmt.Sprintf("\n  Top %d matches:", topCount)
	for i := 0; i < topCount; i++ {
		result := results[i]
		summary += fmt.Sprintf("\n    %s (%.2f)", result.RelativePath, result.Score)
	}
	
	return summary
}

// Utility methods for parameter extraction
func (e *SearchFilesExecutor) getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func (e *SearchFilesExecutor) getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

func (e *SearchFilesExecutor) getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func (e *SearchFilesExecutor) getStringSliceParam(params map[string]interface{}, key string, defaultValue []string) []string {
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

// GetSchema returns the tool schema for the search files executor
func (e *SearchFilesExecutor) GetSchema() *ToolSchema {
	return &ToolSchema{
		Name:        "search_files",
		Description: "Search for files by name using patterns, regex, or glob matching with project context awareness",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Search pattern (filename, glob pattern, or regex)",
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
					"description": "Patterns to exclude from results",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results to return",
					"default":     100,
				},
			},
			"required": []string{"pattern"},
		},
	}
}