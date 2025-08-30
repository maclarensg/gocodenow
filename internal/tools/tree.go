package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gocodenow/internal/context"
)

// TreeExecutor implements directory tree visualization with project awareness
type TreeExecutor struct {
	contextDetector context.ContextDetector
}

// NewTreeExecutor creates a new tree visualization executor
func NewTreeExecutor() *TreeExecutor {
	return &TreeExecutor{
		contextDetector: context.NewDefaultContextDetector(),
	}
}

// Execute generates a visual directory tree
func (e *TreeExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	// Extract parameters
	searchPath := e.getStringParam(params, "path", ".")
	maxDepth := e.getIntParam(params, "max_depth", 10)
	showHidden := e.getBoolParam(params, "show_hidden", false)
	showSizes := e.getBoolParam(params, "show_sizes", false)
	showDates := e.getBoolParam(params, "show_dates", false)
	dirsFirst := e.getBoolParam(params, "dirs_first", true)
	fileTypes := e.getStringSliceParam(params, "file_types", nil)
	excludePatterns := e.getStringSliceParam(params, "exclude", nil)
	includeGitignored := e.getBoolParam(params, "include_gitignored", false)
	colorOutput := e.getBoolParam(params, "color", true)
	sortBy := e.getStringParam(params, "sort_by", "name") // name, size, date
	
	// Resolve absolute path
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve path: %v", err),
		}, err
	}
	
	// Verify path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("path does not exist: %s", absPath),
		}, err
	}
	
	// Detect project context
	projectContext, err := e.contextDetector.DetectContext(absPath)
	if err != nil {
		projectContext = nil
	}
	
	// Build tree structure
	treeRoot, stats, err := e.buildTree(TreeOptions{
		RootPath:         absPath,
		MaxDepth:         maxDepth,
		ShowHidden:       showHidden,
		ShowSizes:        showSizes,
		ShowDates:        showDates,
		DirsFirst:        dirsFirst,
		FileTypes:        fileTypes,
		ExcludePatterns:  excludePatterns,
		IncludeGitignored: includeGitignored,
		ColorOutput:      colorOutput,
		SortBy:           sortBy,
		ProjectContext:   projectContext,
	})
	
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to build tree: %v", err),
		}, err
	}
	
	// Generate visual tree representation
	visualTree := e.renderTree(treeRoot, TreeOptions{
		ShowSizes:   showSizes,
		ShowDates:   showDates,
		ColorOutput: colorOutput,
	})
	
	// Format results
	resultData := map[string]interface{}{
		"path":           absPath,
		"tree_structure": treeRoot,
		"visual_tree":    visualTree,
		"statistics":     stats,
		"execution_time": time.Since(startTime).String(),
		"options": map[string]interface{}{
			"max_depth":         maxDepth,
			"show_hidden":       showHidden,
			"show_sizes":        showSizes,
			"show_dates":        showDates,
			"dirs_first":        dirsFirst,
			"file_types":        fileTypes,
			"exclude":           excludePatterns,
			"include_gitignored": includeGitignored,
			"color":             colorOutput,
			"sort_by":           sortBy,
		},
	}
	
	// Generate summary
	summary := e.generateTreeSummary(absPath, stats, time.Since(startTime))
	
	return &ToolResult{
		Success:   true,
		Result:    resultData,
		Duration:  time.Since(startTime),
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"summary":     summary,
			"total_items": stats.TotalItems,
			"directories": stats.Directories,
			"files":       stats.Files,
		},
	}, nil
}

// TreeOptions configures tree generation behavior
type TreeOptions struct {
	RootPath          string
	MaxDepth          int
	ShowHidden        bool
	ShowSizes         bool
	ShowDates         bool
	DirsFirst         bool
	FileTypes         []string
	ExcludePatterns   []string
	IncludeGitignored bool
	ColorOutput       bool
	SortBy            string
	ProjectContext    *context.ProjectContext
}

// TreeNode represents a node in the directory tree
type TreeNode struct {
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	RelativePath string        `json:"relative_path"`
	Type         string        `json:"type"` // "directory" or "file"
	Size         int64         `json:"size"`
	ModTime      time.Time     `json:"mod_time"`
	Language     string        `json:"language,omitempty"`
	Extension    string        `json:"extension,omitempty"`
	IsHidden     bool          `json:"is_hidden"`
	IsIgnored    bool          `json:"is_ignored"`
	Depth        int           `json:"depth"`
	Children     []*TreeNode   `json:"children,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// TreeStatistics tracks tree generation statistics
type TreeStatistics struct {
	TotalItems       int               `json:"total_items"`
	Directories      int               `json:"directories"`
	Files            int               `json:"files"`
	HiddenItems      int               `json:"hidden_items"`
	IgnoredItems     int               `json:"ignored_items"`
	TotalSize        int64             `json:"total_size"`
	MaxDepthReached  int               `json:"max_depth_reached"`
	LanguageStats    map[string]int    `json:"language_stats"`
	ExtensionStats   map[string]int    `json:"extension_stats"`
	ProcessingTime   time.Duration     `json:"processing_time"`
}

// buildTree constructs the directory tree structure
func (e *TreeExecutor) buildTree(opts TreeOptions) (*TreeNode, *TreeStatistics, error) {
	stats := &TreeStatistics{
		LanguageStats:  make(map[string]int),
		ExtensionStats: make(map[string]int),
	}
	
	rootInfo, err := os.Stat(opts.RootPath)
	if err != nil {
		return nil, stats, err
	}
	
	root := &TreeNode{
		Name:         filepath.Base(opts.RootPath),
		Path:         opts.RootPath,
		RelativePath: ".",
		Type:         "directory",
		Size:         rootInfo.Size(),
		ModTime:      rootInfo.ModTime(),
		Depth:        0,
		Children:     []*TreeNode{},
	}
	
	if rootInfo.IsDir() {
		e.buildTreeRecursive(root, opts, stats)
	}
	
	return root, stats, nil
}

// buildTreeRecursive recursively builds the tree structure
func (e *TreeExecutor) buildTreeRecursive(node *TreeNode, opts TreeOptions, stats *TreeStatistics) {
	// Stop if max depth reached
	if node.Depth >= opts.MaxDepth {
		return
	}
	
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		return
	}
	
	// Filter and sort entries
	var filteredEntries []os.DirEntry
	for _, entry := range entries {
		if e.shouldIncludeEntry(entry, node.Path, opts) {
			filteredEntries = append(filteredEntries, entry)
		}
	}
	
	// Sort entries
	e.sortEntries(filteredEntries, opts.SortBy, opts.DirsFirst)
	
	// Process each entry
	for _, entry := range filteredEntries {
		childPath := filepath.Join(node.Path, entry.Name())
		relPath, _ := filepath.Rel(opts.RootPath, childPath)
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		child := &TreeNode{
			Name:         entry.Name(),
			Path:         childPath,
			RelativePath: relPath,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			IsHidden:     strings.HasPrefix(entry.Name(), "."),
			Depth:        node.Depth + 1,
		}
		
		// Determine type and collect stats
		if entry.IsDir() {
			child.Type = "directory"
			stats.Directories++
		} else {
			child.Type = "file"
			stats.Files++
			stats.TotalSize += child.Size
			
			// Set extension and detect language
			child.Extension = filepath.Ext(entry.Name())
			if child.Extension != "" {
				ext := strings.ToLower(child.Extension[1:]) // Remove dot
				stats.ExtensionStats[ext]++
				
				// Detect language from project context
				if opts.ProjectContext != nil {
					for _, lang := range opts.ProjectContext.Languages {
						for _, langExt := range lang.Extensions {
							if langExt == ext {
								child.Language = lang.Name
								stats.LanguageStats[lang.Name]++
								break
							}
						}
						if child.Language != "" {
							break
						}
					}
				}
			}
		}
		
		// Check if ignored by git
		child.IsIgnored = e.isIgnoredByGit(relPath, opts.ProjectContext)
		if child.IsIgnored {
			stats.IgnoredItems++
		}
		if child.IsHidden {
			stats.HiddenItems++
		}
		
		stats.TotalItems++
		
		// Update max depth reached
		if child.Depth > stats.MaxDepthReached {
			stats.MaxDepthReached = child.Depth
		}
		
		// Recursively process directories
		if entry.IsDir() {
			child.Children = []*TreeNode{}
			e.buildTreeRecursive(child, opts, stats)
		}
		
		node.Children = append(node.Children, child)
	}
}

// shouldIncludeEntry determines if an entry should be included in the tree
func (e *TreeExecutor) shouldIncludeEntry(entry os.DirEntry, parentPath string, opts TreeOptions) bool {
	name := entry.Name()
	
	// Check hidden files
	if !opts.ShowHidden && strings.HasPrefix(name, ".") {
		return false
	}
	
	// Check exclude patterns
	for _, pattern := range opts.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return false
		}
	}
	
	// Check file types filter
	if len(opts.FileTypes) > 0 && !entry.IsDir() {
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
	
	// Check git ignore patterns
	if !opts.IncludeGitignored && opts.ProjectContext != nil {
		entryPath := filepath.Join(parentPath, name)
		relPath, err := filepath.Rel(opts.ProjectContext.WorkspaceRoot, entryPath)
		if err == nil && e.isIgnoredByGit(relPath, opts.ProjectContext) {
			return false
		}
	}
	
	return true
}

// sortEntries sorts directory entries based on the specified criteria
func (e *TreeExecutor) sortEntries(entries []os.DirEntry, sortBy string, dirsFirst bool) {
	sort.Slice(entries, func(i, j int) bool {
		// Directories first if requested
		if dirsFirst {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
		}
		
		switch sortBy {
		case "size":
			info1, err1 := entries[i].Info()
			info2, err2 := entries[j].Info()
			if err1 == nil && err2 == nil {
				return info1.Size() > info2.Size() // Larger first
			}
		case "date":
			info1, err1 := entries[i].Info()
			info2, err2 := entries[j].Info()
			if err1 == nil && err2 == nil {
				return info1.ModTime().After(info2.ModTime()) // Newer first
			}
		default: // name
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		}
		
		// Fallback to name sorting
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})
}

// isIgnoredByGit checks if a path is ignored by git patterns
func (e *TreeExecutor) isIgnoredByGit(relPath string, projectContext *context.ProjectContext) bool {
	if projectContext == nil || !projectContext.Git.IsRepo {
		return false
	}
	
	for _, pattern := range projectContext.IgnorePatterns {
		if e.matchesGitPattern(relPath, pattern) {
			return true
		}
	}
	
	return false
}

// matchesGitPattern checks if a path matches a git ignore pattern
func (e *TreeExecutor) matchesGitPattern(path, pattern string) bool {
	if strings.HasPrefix(pattern, "!") {
		return false // Skip negation patterns for simplicity
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

// renderTree creates the visual ASCII tree representation
func (e *TreeExecutor) renderTree(root *TreeNode, opts TreeOptions) string {
	var lines []string
	lines = append(lines, e.formatTreeNode(root, "", true, opts))
	e.renderTreeRecursive(root, "", true, &lines, opts)
	return strings.Join(lines, "\n")
}

// renderTreeRecursive recursively renders tree nodes
func (e *TreeExecutor) renderTreeRecursive(node *TreeNode, prefix string, isLast bool, lines *[]string, opts TreeOptions) {
	if len(node.Children) == 0 {
		return
	}
	
	for i, child := range node.Children {
		isChildLast := i == len(node.Children)-1
		
		// Choose appropriate tree characters
		var connector, childPrefix string
		if isChildLast {
			connector = "└── "
			childPrefix = prefix + "    "
		} else {
			connector = "├── "
			childPrefix = prefix + "│   "
		}
		
		// Format and add the child node
		line := prefix + connector + e.formatTreeNode(child, "", false, opts)
		*lines = append(*lines, line)
		
		// Recursively process children
		e.renderTreeRecursive(child, childPrefix, isChildLast, lines, opts)
	}
}

// formatTreeNode formats a single tree node for display
func (e *TreeExecutor) formatTreeNode(node *TreeNode, prefix string, isRoot bool, opts TreeOptions) string {
	var parts []string
	
	// Node name with colors if enabled
	name := node.Name
	if opts.ColorOutput {
		if node.Type == "directory" {
			name = fmt.Sprintf("\033[34;1m%s\033[0m", name) // Bold blue for directories
		} else if node.Language != "" {
			// Color files by language
			switch node.Language {
			case "Go":
				name = fmt.Sprintf("\033[36m%s\033[0m", name) // Cyan
			case "JavaScript", "TypeScript":
				name = fmt.Sprintf("\033[33m%s\033[0m", name) // Yellow
			case "Python":
				name = fmt.Sprintf("\033[32m%s\033[0m", name) // Green
			case "Rust":
				name = fmt.Sprintf("\033[31m%s\033[0m", name) // Red
			default:
				name = fmt.Sprintf("\033[37m%s\033[0m", name) // White
			}
		}
		
		// Special formatting for hidden files
		if node.IsHidden {
			name = fmt.Sprintf("\033[90m%s\033[0m", name) // Gray
		}
		
		// Special formatting for ignored files
		if node.IsIgnored {
			name = fmt.Sprintf("\033[90;2m%s\033[0m", name) // Dark gray
		}
	}
	
	parts = append(parts, name)
	
	// Add size information
	if opts.ShowSizes && node.Type == "file" {
		parts = append(parts, fmt.Sprintf("(%s)", e.formatSize(node.Size)))
	}
	
	// Add date information
	if opts.ShowDates {
		parts = append(parts, fmt.Sprintf("[%s]", node.ModTime.Format("2006-01-02 15:04")))
	}
	
	// Add language tag
	if node.Language != "" && opts.ColorOutput {
		parts = append(parts, fmt.Sprintf("<%s>", node.Language))
	}
	
	result := strings.Join(parts, " ")
	
	// Add root prefix if needed
	if isRoot {
		result = prefix + result
	}
	
	return result
}

// formatSize formats file size in human-readable format
func (e *TreeExecutor) formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// generateTreeSummary creates a summary of the tree structure
func (e *TreeExecutor) generateTreeSummary(path string, stats *TreeStatistics, duration time.Duration) string {
	summary := fmt.Sprintf("Directory tree for '%s' (generated in %v):", filepath.Base(path), duration)
	summary += fmt.Sprintf("\n  Total: %d items (%d directories, %d files)", 
		stats.TotalItems, stats.Directories, stats.Files)
	
	if stats.TotalSize > 0 {
		summary += fmt.Sprintf("\n  Size: %s total", e.formatSize(stats.TotalSize))
	}
	
	if stats.HiddenItems > 0 || stats.IgnoredItems > 0 {
		summary += fmt.Sprintf("\n  Filtered: %d hidden, %d ignored", stats.HiddenItems, stats.IgnoredItems)
	}
	
	summary += fmt.Sprintf("\n  Depth: %d levels explored", stats.MaxDepthReached)
	
	// Show language breakdown
	if len(stats.LanguageStats) > 0 {
		summary += "\n  Languages:"
		for lang, count := range stats.LanguageStats {
			summary += fmt.Sprintf(" %s(%d)", lang, count)
		}
	}
	
	return summary
}

// Utility methods
func (e *TreeExecutor) getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func (e *TreeExecutor) getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}

func (e *TreeExecutor) getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(int); ok {
		return val
	}
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func (e *TreeExecutor) getStringSliceParam(params map[string]interface{}, key string, defaultValue []string) []string {
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

// GetSchema returns the tool schema for the tree executor
func (e *TreeExecutor) GetSchema() *ToolSchema {
	return &ToolSchema{
		Name:        "tree",
		Description: "Generate a visual directory tree with project context awareness, language detection, and filtering options",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Directory path to visualize (default: current directory)",
					"default":     ".",
				},
				"max_depth": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum depth to traverse",
					"default":     10,
				},
				"show_hidden": map[string]interface{}{
					"type":        "boolean",
					"description": "Include hidden files and directories",
					"default":     false,
				},
				"show_sizes": map[string]interface{}{
					"type":        "boolean",
					"description": "Display file sizes",
					"default":     false,
				},
				"show_dates": map[string]interface{}{
					"type":        "boolean",
					"description": "Display modification dates",
					"default":     false,
				},
				"dirs_first": map[string]interface{}{
					"type":        "boolean",
					"description": "Sort directories before files",
					"default":     true,
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
					"description": "Patterns to exclude from tree",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"include_gitignored": map[string]interface{}{
					"type":        "boolean",
					"description": "Include files ignored by git",
					"default":     false,
				},
				"color": map[string]interface{}{
					"type":        "boolean",
					"description": "Use ANSI colors in output",
					"default":     true,
				},
				"sort_by": map[string]interface{}{
					"type":        "string",
					"description": "Sort entries by: name, size, or date",
					"enum":        []string{"name", "size", "date"},
					"default":     "name",
				},
			},
		},
	}
}