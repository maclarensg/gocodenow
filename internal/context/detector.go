package context

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultContextDetector implements the ContextDetector interface
type DefaultContextDetector struct {
	languageDetector   LanguageDetector
	buildSystemDetector BuildSystemDetector
	gitDetector        GitDetector
	cacheManager       CacheManager
	options            DetectionOptions
}

// NewContextDetector creates a new context detector with default components
func NewContextDetector(options DetectionOptions) *DefaultContextDetector {
	return &DefaultContextDetector{
		languageDetector:    NewLanguageDetector(),
		buildSystemDetector: NewBuildSystemDetector(),
		gitDetector:         NewGitDetector(),
		cacheManager:        NewCacheManager(),
		options:             options,
	}
}

// DetectContext analyzes the workspace and returns comprehensive project context
func (d *DefaultContextDetector) DetectContext(workspacePath string) (*ProjectContext, error) {
	// Normalize the workspace path
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, &ContextError{
			Type:    "path_error",
			Message: "failed to resolve absolute path",
			Path:    workspacePath,
			Cause:   err,
		}
	}
	
	// Check if it's a valid workspace
	if !d.IsValidWorkspace(absPath) {
		return nil, &ContextError{
			Type:    "invalid_workspace",
			Message: "not a valid workspace directory",
			Path:    absPath,
		}
	}
	
	// Try to load from cache if enabled
	if d.options.UseCache {
		if cached, err := d.cacheManager.LoadContext(absPath); err == nil {
			if d.cacheManager.IsValidCache(cached) {
				return cached, nil
			}
		}
	}
	
	// Find workspace root
	workspaceRoot, err := d.GetWorkspaceRoot(absPath)
	if err != nil {
		workspaceRoot = absPath // Use provided path as fallback
	}
	
	// Create base context
	ctx := &ProjectContext{
		WorkspaceRoot: workspaceRoot,
		DetectedAt:    time.Now(),
		LastUpdated:   time.Now(),
		CacheVersion:  "1.0",
	}
	
	// Detect languages
	if languages, err := d.languageDetector.DetectLanguages(workspaceRoot); err == nil {
		ctx.Languages = languages
		if len(languages) > 0 {
			// Find primary language (highest confidence or file count)
			primary := languages[0]
			for _, lang := range languages {
				if lang.Primary || lang.Confidence > primary.Confidence {
					primary = lang
				}
			}
			ctx.PrimaryLanguage = primary.Name
		}
	}
	
	// Detect build systems
	if buildSystems, err := d.buildSystemDetector.DetectBuildSystems(workspaceRoot); err == nil {
		ctx.BuildSystems = buildSystems
		if len(buildSystems) > 0 {
			ctx.ProjectName = buildSystems[0].Name
		}
	}
	
	// Detect Git information
	if gitInfo, err := d.gitDetector.DetectGitInfo(workspaceRoot); err == nil {
		ctx.Git = *gitInfo
		ctx.IgnorePatterns = append(ctx.IgnorePatterns, gitInfo.IgnorePatterns...)
	}
	
	// Collect file statistics if requested
	if d.options.IncludeStats {
		if stats, err := d.collectFileStats(workspaceRoot); err == nil {
			ctx.Stats = *stats
		}
	}
	
	// Set project name fallback
	if ctx.ProjectName == "" {
		ctx.ProjectName = filepath.Base(workspaceRoot)
	}
	
	// Save to cache if enabled
	if d.options.UseCache {
		d.cacheManager.SaveContext(ctx)
	}
	
	return ctx, nil
}

// UpdateContext refreshes an existing context with new information
func (d *DefaultContextDetector) UpdateContext(ctx *ProjectContext) error {
	if ctx == nil {
		return &ContextError{Type: "invalid_context", Message: "context is nil"}
	}
	
	// Update timestamp
	ctx.LastUpdated = time.Now()
	
	// Re-detect components that might have changed
	if languages, err := d.languageDetector.DetectLanguages(ctx.WorkspaceRoot); err == nil {
		ctx.Languages = languages
	}
	
	if buildSystems, err := d.buildSystemDetector.DetectBuildSystems(ctx.WorkspaceRoot); err == nil {
		ctx.BuildSystems = buildSystems
	}
	
	if gitInfo, err := d.gitDetector.DetectGitInfo(ctx.WorkspaceRoot); err == nil {
		ctx.Git = *gitInfo
	}
	
	if d.options.IncludeStats {
		if stats, err := d.collectFileStats(ctx.WorkspaceRoot); err == nil {
			ctx.Stats = *stats
		}
	}
	
	// Save updated context to cache
	if d.options.UseCache {
		return d.cacheManager.SaveContext(ctx)
	}
	
	return nil
}

// IsValidWorkspace checks if a path represents a valid workspace
func (d *DefaultContextDetector) IsValidWorkspace(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	
	if !info.IsDir() {
		return false
	}
	
	// Check if directory is readable
	if _, err := os.ReadDir(path); err != nil {
		return false
	}
	
	return true
}

// GetWorkspaceRoot finds the root directory of the workspace
func (d *DefaultContextDetector) GetWorkspaceRoot(startPath string) (string, error) {
	current := startPath
	
	for {
		// Check for common workspace root indicators
		if d.isWorkspaceRoot(current) {
			return current, nil
		}
		
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}
		current = parent
	}
	
	// If no clear workspace root found, use the start path
	return startPath, &ContextError{
		Type:    "workspace_root_not_found",
		Message: "could not determine workspace root",
		Path:    startPath,
	}
}

// isWorkspaceRoot checks if a directory appears to be a workspace root
func (d *DefaultContextDetector) isWorkspaceRoot(path string) bool {
	// Common workspace root indicators
	indicators := []string{
		".git",
		".gitignore",
		"go.mod",
		"package.json",
		"Cargo.toml",
		"pom.xml",
		"build.gradle",
		"CMakeLists.txt",
		"Makefile",
		".project",
		".vscode",
		".idea",
		"pyproject.toml",
		"setup.py",
		"composer.json",
		"Gemfile",
	}
	
	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			return true
		}
	}
	
	return false
}

// collectFileStats gathers statistics about files in the workspace
func (d *DefaultContextDetector) collectFileStats(workspacePath string) (*FileStats, error) {
	stats := &FileStats{
		LanguageBreakdown: make(map[string]int),
		LargestFiles:      make([]FileInfo, 0),
	}
	
	err := filepath.WalkDir(workspacePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		// Skip if too deep
		relPath, _ := filepath.Rel(workspacePath, path)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > d.options.MaxDepth {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Skip hidden files if configured
		if d.options.IgnoreHidden && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Count directories
		if entry.IsDir() {
			stats.DirectoryCount++
			return nil
		}
		
		// Process files
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		
		stats.TotalFiles++
		
		// Detect file language
		if lang, err := d.languageDetector.AnalyzeFile(path); err == nil && lang != nil {
			stats.LanguageBreakdown[lang.Name]++
		}
		
		// Track largest files (keep top 10)
		fileInfo := FileInfo{
			Path:         path,
			RelativePath: relPath,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
		}
		
		if len(stats.LargestFiles) < 10 {
			stats.LargestFiles = append(stats.LargestFiles, fileInfo)
		} else {
			// Replace smallest if current is larger
			minIdx := 0
			for i, f := range stats.LargestFiles {
				if f.Size < stats.LargestFiles[minIdx].Size {
					minIdx = i
				}
			}
			if fileInfo.Size > stats.LargestFiles[minIdx].Size {
				stats.LargestFiles[minIdx] = fileInfo
			}
		}
		
		return nil
	})
	
	return stats, err
}

// Helper function to create a new context detector with default options
func NewDefaultContextDetector() *DefaultContextDetector {
	return NewContextDetector(DefaultDetectionOptions())
}