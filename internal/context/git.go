package context

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultGitDetector implements Git repository detection and ignore pattern parsing
type DefaultGitDetector struct{}

// NewGitDetector creates a new Git detector
func NewGitDetector() *DefaultGitDetector {
	return &DefaultGitDetector{}
}

// DetectGitInfo analyzes Git repository information
func (d *DefaultGitDetector) DetectGitInfo(workspacePath string) (*GitInfo, error) {
	gitInfo := &GitInfo{
		IsRepo:         false,
		RootPath:       "",
		CurrentBranch:  "",
		RemoteURL:      "",
		HasUncommitted: false,
		IgnorePatterns: []string{},
	}
	
	// Check if this is a Git repository
	if !d.isGitRepository(workspacePath) {
		return gitInfo, nil
	}
	
	gitInfo.IsRepo = true
	
	// Find Git root
	gitRoot, err := d.findGitRoot(workspacePath)
	if err == nil {
		gitInfo.RootPath = gitRoot
	}
	
	// Get current branch
	if branch, err := d.getCurrentBranch(workspacePath); err == nil {
		gitInfo.CurrentBranch = branch
	}
	
	// Get remote URL
	if remoteURL, err := d.getRemoteURL(workspacePath); err == nil {
		gitInfo.RemoteURL = remoteURL
	}
	
	// Check for uncommitted changes
	gitInfo.HasUncommitted = d.hasUncommittedChanges(workspacePath)
	
	// Parse ignore patterns
	if patterns, err := d.parseIgnorePatterns(gitInfo.RootPath); err == nil {
		gitInfo.IgnorePatterns = patterns
	}
	
	return gitInfo, nil
}

// ParseIgnoreFile parses .gitignore or similar ignore files
func (d *DefaultGitDetector) ParseIgnoreFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	var patterns []string
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		patterns = append(patterns, line)
	}
	
	return patterns, scanner.Err()
}

// IsIgnored checks if a file path should be ignored based on patterns
func (d *DefaultGitDetector) IsIgnored(filePath string, patterns []string) bool {
	// Normalize the path
	filePath = filepath.Clean(filePath)
	
	for _, pattern := range patterns {
		if d.matchesGitIgnorePattern(filePath, pattern) {
			return true
		}
	}
	
	return false
}

// isGitRepository checks if the given path is within a Git repository
func (d *DefaultGitDetector) isGitRepository(path string) bool {
	// Check for .git directory
	gitPath := filepath.Join(path, ".git")
	if info, err := os.Stat(gitPath); err == nil {
		return info.IsDir() || info.Mode().IsRegular() // Could be a file in case of git worktrees
	}
	
	// Check parent directories
	parent := filepath.Dir(path)
	if parent == path {
		return false // Reached root
	}
	
	return d.isGitRepository(parent)
}

// findGitRoot finds the root directory of the Git repository
func (d *DefaultGitDetector) findGitRoot(startPath string) (string, error) {
	current := startPath
	
	for {
		gitPath := filepath.Join(current, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return current, nil
		}
		
		parent := filepath.Dir(current)
		if parent == current {
			break // Reached filesystem root
		}
		current = parent
	}
	
	return "", &ContextError{
		Type:    "git_root_not_found",
		Message: "Git repository root not found",
		Path:    startPath,
	}
}

// getCurrentBranch gets the current Git branch name
func (d *DefaultGitDetector) getCurrentBranch(workspacePath string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = workspacePath
	
	output, err := cmd.Output()
	if err != nil {
		// Fallback: try reading HEAD file directly
		return d.getBranchFromHEAD(workspacePath)
	}
	
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "HEAD", nil // Detached HEAD state
	}
	
	return branch, nil
}

// getBranchFromHEAD reads branch information from .git/HEAD file
func (d *DefaultGitDetector) getBranchFromHEAD(workspacePath string) (string, error) {
	gitRoot, err := d.findGitRoot(workspacePath)
	if err != nil {
		return "", err
	}
	
	headFile := filepath.Join(gitRoot, ".git", "HEAD")
	content, err := os.ReadFile(headFile)
	if err != nil {
		return "", err
	}
	
	headContent := strings.TrimSpace(string(content))
	
	// Check if HEAD points to a branch
	if strings.HasPrefix(headContent, "ref: refs/heads/") {
		return strings.TrimPrefix(headContent, "ref: refs/heads/"), nil
	}
	
	// Detached HEAD (contains commit hash)
	return "HEAD", nil
}

// getRemoteURL gets the URL of the origin remote
func (d *DefaultGitDetector) getRemoteURL(workspacePath string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workspacePath
	
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(output)), nil
}

// hasUncommittedChanges checks if there are uncommitted changes in the repository
func (d *DefaultGitDetector) hasUncommittedChanges(workspacePath string) bool {
	// Check for staged changes
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = workspacePath
	if err := cmd.Run(); err != nil {
		return true // Has staged changes
	}
	
	// Check for unstaged changes
	cmd = exec.Command("git", "diff", "--quiet")
	cmd.Dir = workspacePath
	if err := cmd.Run(); err != nil {
		return true // Has unstaged changes
	}
	
	// Check for untracked files
	cmd = exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = workspacePath
	output, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		return true // Has untracked files
	}
	
	return false
}

// parseIgnorePatterns collects ignore patterns from various sources
func (d *DefaultGitDetector) parseIgnorePatterns(gitRoot string) ([]string, error) {
	var allPatterns []string
	
	// Parse .gitignore files
	gitignoreFiles := []string{
		filepath.Join(gitRoot, ".gitignore"),
		filepath.Join(gitRoot, ".git", "info", "exclude"),
	}
	
	for _, ignoreFile := range gitignoreFiles {
		if patterns, err := d.ParseIgnoreFile(ignoreFile); err == nil {
			allPatterns = append(allPatterns, patterns...)
		}
	}
	
	// Add common ignore patterns that Git uses by default
	defaultPatterns := []string{
		".git/",
		"*.tmp",
		"*.temp",
		"*~",
		".DS_Store",
		"Thumbs.db",
	}
	
	allPatterns = append(allPatterns, defaultPatterns...)
	
	// Look for .clignore files (custom ignore files)
	if patterns, err := d.parseClignoreFiles(gitRoot); err == nil {
		allPatterns = append(allPatterns, patterns...)
	}
	
	return allPatterns, nil
}

// parseClignoreFiles looks for and parses .clignore files
func (d *DefaultGitDetector) parseClignoreFiles(rootPath string) ([]string, error) {
	var allPatterns []string
	
	err := filepath.WalkDir(rootPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if entry.IsDir() {
			return nil
		}
		
		if entry.Name() == ".clignore" {
			if patterns, err := d.ParseIgnoreFile(path); err == nil {
				allPatterns = append(allPatterns, patterns...)
			}
		}
		
		return nil
	})
	
	return allPatterns, err
}

// matchesGitIgnorePattern checks if a file path matches a gitignore-style pattern
func (d *DefaultGitDetector) matchesGitIgnorePattern(filePath, pattern string) bool {
	// Handle negation patterns
	if strings.HasPrefix(pattern, "!") {
		return false // Negation patterns need special handling in context
	}
	
	// Normalize paths
	filePath = strings.TrimPrefix(filePath, "./")
	pattern = strings.TrimPrefix(pattern, "./")
	
	// Directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		pattern = strings.TrimSuffix(pattern, "/")
		return d.matchesPattern(filePath, pattern) || strings.HasPrefix(filePath, pattern+"/")
	}
	
	// Exact match
	if pattern == filePath {
		return true
	}
	
	// Pattern matching
	return d.matchesPattern(filePath, pattern)
}

// matchesPattern performs glob-style pattern matching
func (d *DefaultGitDetector) matchesPattern(filePath, pattern string) bool {
	// Simple glob pattern matching
	if strings.Contains(pattern, "*") {
		return d.globMatch(filePath, pattern)
	}
	
	// Directory prefix matching
	if strings.Contains(filePath, "/") && strings.Contains(pattern, "/") {
		return strings.HasPrefix(filePath, pattern)
	}
	
	// Filename matching
	fileName := filepath.Base(filePath)
	return fileName == pattern || strings.HasPrefix(fileName, pattern)
}

// globMatch performs basic glob pattern matching
func (d *DefaultGitDetector) globMatch(text, pattern string) bool {
	// Convert glob pattern to regex-like matching
	// This is a simplified implementation
	
	if pattern == "*" {
		return true
	}
	
	if pattern == "*.*" {
		return strings.Contains(text, ".")
	}
	
	if strings.HasPrefix(pattern, "*.") {
		ext := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(text, ext)
	}
	
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(text, prefix)
	}
	
	// More complex patterns would need a proper glob library
	return strings.Contains(text, strings.ReplaceAll(pattern, "*", ""))
}