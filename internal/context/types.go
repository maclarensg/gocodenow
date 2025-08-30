// Package context provides project context detection and workspace management
//
// This package analyzes the current workspace to understand:
// - Project structure and build systems
// - Programming languages used
// - Git repository information
// - File ignore patterns
// - Project metadata and dependencies
package context

import (
	"time"
)

// Language represents a programming language detected in the project
type Language struct {
	Name         string  `json:"name"`
	Extensions   []string `json:"extensions"`
	Confidence   float64 `json:"confidence"` // 0.0 to 1.0
	FileCount    int     `json:"file_count"`
	LineCount    int     `json:"line_count,omitempty"`
	Primary      bool    `json:"primary"` // Is this the primary language?
}

// BuildSystem represents a detected build system or package manager
type BuildSystem struct {
	Type         string            `json:"type"`         // "go", "npm", "cargo", "maven", etc.
	ConfigFile   string            `json:"config_file"`  // "go.mod", "package.json", etc.
	Name         string            `json:"name"`         // Project name from config
	Version      string            `json:"version"`      // Project version
	Dependencies []string          `json:"dependencies"` // List of dependencies
	Scripts      map[string]string `json:"scripts,omitempty"` // Available scripts/commands
}

// GitInfo represents Git repository information
type GitInfo struct {
	IsRepo          bool     `json:"is_repo"`
	RootPath        string   `json:"root_path"`
	CurrentBranch   string   `json:"current_branch"`
	RemoteURL       string   `json:"remote_url,omitempty"`
	HasUncommitted  bool     `json:"has_uncommitted"`
	IgnorePatterns  []string `json:"ignore_patterns"`
}

// FileStats represents statistics about files in the project
type FileStats struct {
	TotalFiles      int                    `json:"total_files"`
	TotalLines      int                    `json:"total_lines"`
	LanguageBreakdown map[string]int       `json:"language_breakdown"` // lang -> file count
	DirectoryCount  int                    `json:"directory_count"`
	LargestFiles    []FileInfo             `json:"largest_files,omitempty"`
}

// FileInfo represents information about a specific file
type FileInfo struct {
	Path         string    `json:"path"`
	RelativePath string    `json:"relative_path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Language     string    `json:"language,omitempty"`
	LineCount    int       `json:"line_count,omitempty"`
}

// ProjectContext represents the complete context of a project workspace
type ProjectContext struct {
	// Basic project information
	WorkspaceRoot   string      `json:"workspace_root"`
	ProjectName     string      `json:"project_name"`
	DetectedAt      time.Time   `json:"detected_at"`
	
	// Language and build system information
	Languages       []Language    `json:"languages"`
	BuildSystems    []BuildSystem `json:"build_systems"`
	PrimaryLanguage string        `json:"primary_language"`
	
	// Git repository information
	Git             GitInfo      `json:"git"`
	
	// File and directory information
	Stats           FileStats    `json:"stats"`
	
	// Ignore patterns from various sources
	IgnorePatterns  []string     `json:"ignore_patterns"`
	
	// Cached metadata for performance
	CacheVersion    string       `json:"cache_version"`
	LastUpdated     time.Time    `json:"last_updated"`
}

// ContextDetector defines the interface for project context detection
type ContextDetector interface {
	// DetectContext analyzes the workspace and returns project context
	DetectContext(workspacePath string) (*ProjectContext, error)
	
	// UpdateContext refreshes an existing context with new information
	UpdateContext(ctx *ProjectContext) error
	
	// IsValidWorkspace checks if a path represents a valid workspace
	IsValidWorkspace(path string) bool
	
	// GetWorkspaceRoot finds the root directory of the workspace
	GetWorkspaceRoot(startPath string) (string, error)
}

// LanguageDetector defines the interface for programming language detection
type LanguageDetector interface {
	// DetectLanguages analyzes files and returns detected languages
	DetectLanguages(workspacePath string) ([]Language, error)
	
	// AnalyzeFile determines the language of a specific file
	AnalyzeFile(filePath string) (*Language, error)
	
	// GetLanguageByExtension returns language info for a file extension
	GetLanguageByExtension(extension string) (*Language, bool)
}

// BuildSystemDetector defines the interface for build system detection
type BuildSystemDetector interface {
	// DetectBuildSystems finds and analyzes build system configurations
	DetectBuildSystems(workspacePath string) ([]BuildSystem, error)
	
	// AnalyzeBuildFile parses a specific build configuration file
	AnalyzeBuildFile(filePath string) (*BuildSystem, error)
	
	// GetSupportedSystems returns a list of supported build systems
	GetSupportedSystems() []string
}

// GitDetector defines the interface for Git repository detection
type GitDetector interface {
	// DetectGitInfo analyzes Git repository information
	DetectGitInfo(workspacePath string) (*GitInfo, error)
	
	// ParseIgnoreFile parses .gitignore or similar ignore files
	ParseIgnoreFile(filePath string) ([]string, error)
	
	// IsIgnored checks if a file path should be ignored
	IsIgnored(filePath string, patterns []string) bool
}

// CacheManager defines the interface for project context caching
type CacheManager interface {
	// SaveContext saves project context to cache
	SaveContext(ctx *ProjectContext) error
	
	// LoadContext loads project context from cache
	LoadContext(workspacePath string) (*ProjectContext, error)
	
	// InvalidateCache removes cached context for a workspace
	InvalidateCache(workspacePath string) error
	
	// IsValidCache checks if cached context is still valid
	IsValidCache(ctx *ProjectContext) bool
}

// DetectionOptions configures context detection behavior
type DetectionOptions struct {
	// IncludeStats whether to collect detailed file statistics
	IncludeStats bool
	
	// MaxDepth maximum directory depth to analyze
	MaxDepth int
	
	// FollowSymlinks whether to follow symbolic links
	FollowSymlinks bool
	
	// IgnoreHidden whether to ignore hidden files and directories
	IgnoreHidden bool
	
	// UseCache whether to use cached results when available
	UseCache bool
	
	// CacheMaxAge maximum age of cached results to consider valid
	CacheMaxAge time.Duration
	
	// Languages specific languages to detect (empty = all)
	Languages []string
	
	// BuildSystems specific build systems to detect (empty = all)
	BuildSystems []string
}

// DefaultDetectionOptions returns sensible default detection options
func DefaultDetectionOptions() DetectionOptions {
	return DetectionOptions{
		IncludeStats:   true,
		MaxDepth:       10,
		FollowSymlinks: false,
		IgnoreHidden:   true,
		UseCache:       true,
		CacheMaxAge:    24 * time.Hour,
		Languages:      nil, // Detect all
		BuildSystems:   nil, // Detect all
	}
}

// Error types for context detection
type ContextError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Cause   error  `json:"cause,omitempty"`
}

func (e *ContextError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ContextError) Unwrap() error {
	return e.Cause
}

// Common error types
var (
	ErrInvalidWorkspace   = &ContextError{Type: "invalid_workspace", Message: "not a valid workspace"}
	ErrWorkspaceNotFound  = &ContextError{Type: "workspace_not_found", Message: "workspace root not found"}
	ErrPermissionDenied   = &ContextError{Type: "permission_denied", Message: "permission denied accessing workspace"}
	ErrCacheCorrupted     = &ContextError{Type: "cache_corrupted", Message: "cached context data is corrupted"}
)