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

// GrepExecutor implements grep-like functionality with syntax highlighting and context awareness
type GrepExecutor struct {
	contextDetector context.ContextDetector
	maxResults      int
	maxFileSize     int64
}

// NewGrepExecutor creates a new grep executor
func NewGrepExecutor() *GrepExecutor {
	return &GrepExecutor{
		contextDetector: context.NewDefaultContextDetector(),
		maxResults:      1000,
		maxFileSize:     10 * 1024 * 1024, // 10MB
	}
}

// Execute performs grep-style search with enhanced formatting
func (e *GrepExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	// Extract parameters
	pattern, ok := params["pattern"].(string)
	if !ok || pattern == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "pattern parameter is required",
		}, fmt.Errorf("pattern parameter missing")
	}
	
	// Get grep options
	searchPath := e.getStringParam(params, "path", ".")
	caseSensitive := e.getBoolParam(params, "case_sensitive", false)
	invertMatch := e.getBoolParam(params, "invert_match", false)
	wholeWord := e.getBoolParam(params, "whole_word", false)
	lineNumbers := e.getBoolParam(params, "line_numbers", true)
	includeHidden := e.getBoolParam(params, "include_hidden", false)
	recursive := e.getBoolParam(params, "recursive", true)
	contextBefore := e.getIntParam(params, "context_before", 0)
	contextAfter := e.getIntParam(params, "context_after", 0)
	maxResults := e.getIntParam(params, "max_results", e.maxResults)
	fileTypes := e.getStringSliceParam(params, "file_types", nil)
	excludePatterns := e.getStringSliceParam(params, "exclude", nil)
	highlightMatches := e.getBoolParam(params, "highlight", true)
	colorOutput := e.getBoolParam(params, "color", true)
	
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
	
	// Perform grep search
	grepResults, stats, err := e.performGrep(GrepOptions{
		Pattern:         pattern,
		SearchPath:      absPath,
		CaseSensitive:   caseSensitive,
		InvertMatch:     invertMatch,
		WholeWord:       wholeWord,
		LineNumbers:     lineNumbers,
		IncludeHidden:   includeHidden,
		Recursive:       recursive,
		ContextBefore:   contextBefore,
		ContextAfter:    contextAfter,
		MaxResults:      maxResults,
		FileTypes:       fileTypes,
		ExcludePatterns: excludePatterns,
		HighlightMatches: highlightMatches,
		ColorOutput:     colorOutput,
		ProjectContext:  projectContext,
	})
	
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("grep search failed: %v", err),
		}, err
	}
	
	// Format results
	resultData := map[string]interface{}{
		"pattern":        pattern,
		"search_path":    absPath,
		"total_matches":  stats.TotalMatches,
		"files_searched": stats.FilesSearched,
		"files_matched":  stats.FilesWithMatches,
		"results":        grepResults,
		"execution_time": time.Since(startTime).String(),
		"statistics":     stats,
		"formatted_output": e.formatGrepOutput(grepResults, GrepOptions{
			LineNumbers:     lineNumbers,
			HighlightMatches: highlightMatches,
			ColorOutput:     colorOutput,
		}),
	}
	
	// Generate summary
	summary := e.generateGrepSummary(pattern, grepResults, stats, time.Since(startTime))
	
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

// GrepOptions configures grep behavior
type GrepOptions struct {
	Pattern          string
	SearchPath       string
	CaseSensitive    bool
	InvertMatch      bool
	WholeWord        bool
	LineNumbers      bool
	IncludeHidden    bool
	Recursive        bool
	ContextBefore    int
	ContextAfter     int
	MaxResults       int
	FileTypes        []string
	ExcludePatterns  []string
	HighlightMatches bool
	ColorOutput      bool
	ProjectContext   *context.ProjectContext
}

// GrepResult represents a grep search result
type GrepResult struct {
	File            string        `json:"file"`
	RelativePath    string        `json:"relative_path"`
	Language        string        `json:"language,omitempty"`
	Matches         []GrepMatch   `json:"matches"`
	TotalMatches    int           `json:"total_matches"`
	FileSize        int64         `json:"file_size"`
	ModTime         time.Time     `json:"mod_time"`
}

// GrepMatch represents a single grep match with context
type GrepMatch struct {
	LineNumber       int      `json:"line_number"`
	Content          string   `json:"content"`
	HighlightedContent string `json:"highlighted_content,omitempty"`
	MatchPositions   []int    `json:"match_positions"` // [start, end] pairs
	ContextBefore    []ContextLine `json:"context_before,omitempty"`
	ContextAfter     []ContextLine `json:"context_after,omitempty"`
	MatchType        string   `json:"match_type"` // "match" or "invert"
}

// ContextLine represents a context line around a match
type ContextLine struct {
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
	Type       string `json:"type"` // "before", "after"
}

// GrepStatistics tracks grep search performance
type GrepStatistics struct {
	FilesSearched    int            `json:"files_searched"`
	FilesWithMatches int            `json:"files_with_matches"`
	FilesSkipped     int            `json:"files_skipped"`
	TotalMatches     int            `json:"total_matches"`
	SearchDuration   time.Duration  `json:"search_duration"`
	LanguageStats    map[string]int `json:"language_stats"`
}

// performGrep executes the grep search
func (e *GrepExecutor) performGrep(opts GrepOptions) ([]GrepResult, *GrepStatistics, error) {
	var results []GrepResult
	stats := &GrepStatistics{
		LanguageStats: make(map[string]int),
	}
	
	// Compile regex pattern
	searchRegex, err := e.compilePattern(opts.Pattern, opts.CaseSensitive, opts.WholeWord)
	if err != nil {
		return nil, stats, fmt.Errorf("invalid pattern: %v", err)
	}
	
	// Determine search strategy
	if opts.Recursive {
		err = e.searchRecursive(opts.SearchPath, opts, searchRegex, &results, stats)
	} else {
		err = e.searchSingleFile(opts.SearchPath, opts, searchRegex, &results, stats)
	}
	
	return results, stats, err
}

// searchRecursive performs recursive directory search
func (e *GrepExecutor) searchRecursive(searchPath string, opts GrepOptions, searchRegex *regexp.Regexp, results *[]GrepResult, stats *GrepStatistics) error {
	return filepath.WalkDir(searchPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if entry.IsDir() {
			return nil
		}
		
		// Apply filters
		if !e.shouldProcessFile(path, entry, opts) {
			stats.FilesSkipped++
			return nil
		}
		
		// Search file
		fileResult, err := e.searchFile(path, opts, searchRegex)
		if err != nil {
			stats.FilesSkipped++
			return nil
		}
		
		stats.FilesSearched++
		
		if fileResult.TotalMatches > 0 {
			stats.FilesWithMatches++
			stats.TotalMatches += fileResult.TotalMatches
			
			if fileResult.Language != "" {
				stats.LanguageStats[fileResult.Language]++
			}
			
			*results = append(*results, *fileResult)
			
			// Stop if max results reached
			if stats.TotalMatches >= opts.MaxResults {
				return filepath.SkipAll
			}
		}
		
		return nil
	})
}

// searchSingleFile performs search on a single file
func (e *GrepExecutor) searchSingleFile(filePath string, opts GrepOptions, searchRegex *regexp.Regexp, results *[]GrepResult, stats *GrepStatistics) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("path is a directory, use recursive option")
	}
	
	entry := &fileEntry{name: filepath.Base(filePath), info: info}
	if !e.shouldProcessFile(filePath, entry, opts) {
		return fmt.Errorf("file filtered out by options")
	}
	
	fileResult, err := e.searchFile(filePath, opts, searchRegex)
	if err != nil {
		return err
	}
	
	stats.FilesSearched = 1
	if fileResult.TotalMatches > 0 {
		stats.FilesWithMatches = 1
		stats.TotalMatches = fileResult.TotalMatches
		*results = append(*results, *fileResult)
	}
	
	return nil
}

// searchFile searches within a single file
func (e *GrepExecutor) searchFile(filePath string, opts GrepOptions, searchRegex *regexp.Regexp) (*GrepResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	info, _ := file.Stat()
	relPath, _ := filepath.Rel(opts.SearchPath, filePath)
	
	result := &GrepResult{
		File:         filePath,
		RelativePath: relPath,
		Language:     e.detectLanguage(filePath, opts.ProjectContext),
		Matches:      []GrepMatch{},
		FileSize:     info.Size(),
		ModTime:      info.ModTime(),
	}
	
	scanner := bufio.NewScanner(file)
	lineNumber := 1
	var allLines []string
	
	// Read all lines for context support
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	
	// Process each line
	for i, line := range allLines {
		matches := searchRegex.MatchString(line)
		
		// Apply invert match logic
		shouldInclude := matches
		if opts.InvertMatch {
			shouldInclude = !matches
		}
		
		if shouldInclude {
			match := GrepMatch{
				LineNumber: i + 1,
				Content:    line,
				MatchType:  "match",
			}
			
			if opts.InvertMatch {
				match.MatchType = "invert"
			} else {
				// Find match positions for highlighting
				match.MatchPositions = e.findMatchPositions(line, searchRegex)
				if opts.HighlightMatches {
					match.HighlightedContent = e.highlightMatches(line, match.MatchPositions, opts.ColorOutput)
				}
			}
			
			// Add context lines
			if opts.ContextBefore > 0 || opts.ContextAfter > 0 {
				match.ContextBefore = e.getContextBefore(allLines, i, opts.ContextBefore)
				match.ContextAfter = e.getContextAfter(allLines, i, opts.ContextAfter)
			}
			
			result.Matches = append(result.Matches, match)
		}
		
		lineNumber++
	}
	
	result.TotalMatches = len(result.Matches)
	return result, scanner.Err()
}

// compilePattern compiles the search pattern into a regex
func (e *GrepExecutor) compilePattern(pattern string, caseSensitive, wholeWord bool) (*regexp.Regexp, error) {
	regexPattern := pattern
	
	if wholeWord {
		regexPattern = `\b` + regexp.QuoteMeta(pattern) + `\b`
	}
	
	flags := ""
	if !caseSensitive {
		flags += "i"
	}
	
	if flags != "" {
		regexPattern = fmt.Sprintf("(?%s)%s", flags, regexPattern)
	}
	
	return regexp.Compile(regexPattern)
}

// findMatchPositions finds all match positions in a line
func (e *GrepExecutor) findMatchPositions(line string, regex *regexp.Regexp) []int {
	var positions []int
	matches := regex.FindAllStringIndex(line, -1)
	
	for _, match := range matches {
		positions = append(positions, match[0], match[1])
	}
	
	return positions
}

// highlightMatches applies syntax highlighting to matches
func (e *GrepExecutor) highlightMatches(line string, positions []int, colorOutput bool) string {
	if len(positions) == 0 {
		return line
	}
	
	// ANSI color codes
	highlightStart := "\033[31;1m" // Bold red
	highlightEnd := "\033[0m"      // Reset
	
	if !colorOutput {
		highlightStart = "["
		highlightEnd = "]"
	}
	
	var result strings.Builder
	lastEnd := 0
	
	// Process matches in pairs (start, end)
	for i := 0; i < len(positions); i += 2 {
		start := positions[i]
		end := positions[i+1]
		
		// Add text before match
		result.WriteString(line[lastEnd:start])
		
		// Add highlighted match
		result.WriteString(highlightStart)
		result.WriteString(line[start:end])
		result.WriteString(highlightEnd)
		
		lastEnd = end
	}
	
	// Add remaining text
	result.WriteString(line[lastEnd:])
	
	return result.String()
}

// getContextBefore retrieves context lines before a match
func (e *GrepExecutor) getContextBefore(lines []string, currentIndex, contextCount int) []ContextLine {
	var context []ContextLine
	
	start := currentIndex - contextCount
	if start < 0 {
		start = 0
	}
	
	for i := start; i < currentIndex; i++ {
		context = append(context, ContextLine{
			LineNumber: i + 1,
			Content:    lines[i],
			Type:       "before",
		})
	}
	
	return context
}

// getContextAfter retrieves context lines after a match
func (e *GrepExecutor) getContextAfter(lines []string, currentIndex, contextCount int) []ContextLine {
	var context []ContextLine
	
	end := currentIndex + contextCount + 1
	if end > len(lines) {
		end = len(lines)
	}
	
	for i := currentIndex + 1; i < end; i++ {
		context = append(context, ContextLine{
			LineNumber: i + 1,
			Content:    lines[i],
			Type:       "after",
		})
	}
	
	return context
}

// formatGrepOutput creates grep-style formatted output
func (e *GrepExecutor) formatGrepOutput(results []GrepResult, opts GrepOptions) string {
	var output strings.Builder
	
	for _, result := range results {
		if len(results) > 1 {
			// File separator for multiple files
			output.WriteString(fmt.Sprintf("\n=== %s ===\n", result.RelativePath))
		}
		
		for _, match := range result.Matches {
			// Context before
			for _, ctx := range match.ContextBefore {
				if opts.LineNumbers {
					output.WriteString(fmt.Sprintf("%d-", ctx.LineNumber))
				}
				output.WriteString(fmt.Sprintf("%s\n", ctx.Content))
			}
			
			// Main match line
			if opts.LineNumbers {
				output.WriteString(fmt.Sprintf("%d:", match.LineNumber))
			}
			
			if opts.HighlightMatches && match.HighlightedContent != "" {
				output.WriteString(match.HighlightedContent)
			} else {
				output.WriteString(match.Content)
			}
			output.WriteString("\n")
			
			// Context after
			for _, ctx := range match.ContextAfter {
				if opts.LineNumbers {
					output.WriteString(fmt.Sprintf("%d-", ctx.LineNumber))
				}
				output.WriteString(fmt.Sprintf("%s\n", ctx.Content))
			}
			
			// Add separator between matches if context is shown
			if len(match.ContextBefore) > 0 || len(match.ContextAfter) > 0 {
				output.WriteString("--\n")
			}
		}
	}
	
	return strings.TrimSuffix(output.String(), "--\n")
}

// shouldProcessFile determines if a file should be processed
func (e *GrepExecutor) shouldProcessFile(path string, entry interface{}, opts GrepOptions) bool {
	var name string
	var size int64
	
	if de, ok := entry.(os.DirEntry); ok {
		name = de.Name()
		if info, err := de.Info(); err == nil {
			size = info.Size()
		}
	} else if fe, ok := entry.(*fileEntry); ok {
		name = fe.name
		size = fe.info.Size()
	} else {
		return false
	}
	
	// Skip hidden files
	if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
		return false
	}
	
	// Check file size
	if size > e.maxFileSize {
		return false
	}
	
	// Apply exclude patterns
	for _, exclude := range opts.ExcludePatterns {
		if matched, _ := filepath.Match(exclude, name); matched {
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
	
	// Project context filtering
	if opts.ProjectContext != nil {
		if relPath, err := filepath.Rel(opts.ProjectContext.WorkspaceRoot, path); err == nil {
			for _, pattern := range opts.ProjectContext.IgnorePatterns {
				if e.matchesIgnorePattern(relPath, pattern) {
					return false
				}
			}
		}
	}
	
	// Skip binary files
	if e.isBinaryFile(name) {
		return false
	}
	
	return true
}

// detectLanguage detects file language from context
func (e *GrepExecutor) detectLanguage(filePath string, projectContext *context.ProjectContext) string {
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

// isBinaryFile checks if file is likely binary
func (e *GrepExecutor) isBinaryFile(filename string) bool {
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

// matchesIgnorePattern checks ignore patterns
func (e *GrepExecutor) matchesIgnorePattern(path, pattern string) bool {
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

// generateGrepSummary creates a summary of grep results
func (e *GrepExecutor) generateGrepSummary(pattern string, results []GrepResult, stats *GrepStatistics, duration time.Duration) string {
	if stats.TotalMatches == 0 {
		return fmt.Sprintf("No matches found for '%s' (searched %d files in %v)", 
			pattern, stats.FilesSearched, duration)
	}
	
	summary := fmt.Sprintf("Found %d matches in %d files (searched %d files in %v):",
		stats.TotalMatches, stats.FilesWithMatches, stats.FilesSearched, duration)
	
	if len(stats.LanguageStats) > 0 {
		summary += "\n  Languages:"
		for lang, count := range stats.LanguageStats {
			summary += fmt.Sprintf(" %s(%d)", lang, count)
		}
	}
	
	return summary
}

// Helper types
type fileEntry struct {
	name string
	info os.FileInfo
}

func (fe *fileEntry) Name() string       { return fe.name }
func (fe *fileEntry) IsDir() bool        { return fe.info.IsDir() }
func (fe *fileEntry) Type() os.FileMode  { return fe.info.Mode().Type() }
func (fe *fileEntry) Info() (os.FileInfo, error) { return fe.info, nil }

// Utility methods (same as previous executors)
func (e *GrepExecutor) getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func (e *GrepExecutor) getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

func (e *GrepExecutor) getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func (e *GrepExecutor) getStringSliceParam(params map[string]interface{}, key string, defaultValue []string) []string {
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

// GetSchema returns the tool schema for the grep executor
func (e *GrepExecutor) GetSchema() *ToolSchema {
	return &ToolSchema{
		Name:        "grep",
		Description: "Advanced grep-style text search with syntax highlighting, context lines, and project awareness",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]interface{}{
					"type":        "string",
					"description": "Regular expression or text pattern to search for",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File or directory to search in",
					"default":     ".",
				},
				"case_sensitive": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether search should be case sensitive",
					"default":     false,
				},
				"invert_match": map[string]interface{}{
					"type":        "boolean",
					"description": "Show lines that don't match the pattern",
					"default":     false,
				},
				"whole_word": map[string]interface{}{
					"type":        "boolean",
					"description": "Match whole words only",
					"default":     false,
				},
				"line_numbers": map[string]interface{}{
					"type":        "boolean",
					"description": "Show line numbers in output",
					"default":     true,
				},
				"recursive": map[string]interface{}{
					"type":        "boolean",
					"description": "Search directories recursively",
					"default":     true,
				},
				"context_before": map[string]interface{}{
					"type":        "integer",
					"description": "Number of context lines before matches",
					"default":     0,
				},
				"context_after": map[string]interface{}{
					"type":        "integer",
					"description": "Number of context lines after matches",
					"default":     0,
				},
				"highlight": map[string]interface{}{
					"type":        "boolean",
					"description": "Highlight matched text",
					"default":     true,
				},
				"color": map[string]interface{}{
					"type":        "boolean",
					"description": "Use ANSI colors for highlighting",
					"default":     true,
				},
				"include_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files",
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
					"description": "Patterns to exclude",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of matches",
					"default":     1000,
				},
			},
			"required": []string{"pattern"},
		},
	}
}