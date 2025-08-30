// Git integration tools for repository analysis and change tracking
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gocodenow/internal/context"
)

// GitStatusExecutor provides git status information with project context awareness
type GitStatusExecutor struct {
	contextDetector context.ContextDetector
}

// GitLogExecutor provides git log analysis with commit history insights
type GitLogExecutor struct {
	contextDetector context.ContextDetector
}

// GitDiffExecutor provides git diff visualization with syntax awareness
type GitDiffExecutor struct {
	contextDetector context.ContextDetector
}

// GitBlameExecutor provides line-by-line authorship information
type GitBlameExecutor struct {
	contextDetector context.ContextDetector
}

// GitStatusOptions configures git status analysis
type GitStatusOptions struct {
	Path           string   `json:"path"`
	IncludeUntracked bool   `json:"include_untracked"`
	ShowBranch     bool     `json:"show_branch"`
	ShowRemote     bool     `json:"show_remote"`
	FileTypes      []string `json:"file_types,omitempty"`
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
}

// GitLogOptions configures git log analysis
type GitLogOptions struct {
	Path         string `json:"path"`
	MaxCommits   int    `json:"max_commits"`
	Since        string `json:"since,omitempty"`        // e.g., "2023-01-01", "1 week ago"
	Until        string `json:"until,omitempty"`        // e.g., "2023-12-31", "yesterday"
	Author       string `json:"author,omitempty"`       // Filter by author
	Branch       string `json:"branch,omitempty"`       // Specific branch (default: current)
	OneLine      bool   `json:"one_line"`              // Show one line per commit
	ShowStats    bool   `json:"show_stats"`            // Show file change statistics
	ShowGraph    bool   `json:"show_graph"`            // Show commit graph
	FileTypes    []string `json:"file_types,omitempty"` // Filter by file extensions
}

// GitDiffOptions configures git diff analysis
type GitDiffOptions struct {
	Path          string   `json:"path"`
	Revision      string   `json:"revision,omitempty"`     // Compare with specific revision
	Staged        bool     `json:"staged"`                 // Show staged changes
	Cached        bool     `json:"cached"`                 // Alias for staged
	NameOnly      bool     `json:"name_only"`              // Show only file names
	Stat          bool     `json:"stat"`                   // Show diffstat
	WordDiff      bool     `json:"word_diff"`              // Show word-level differences
	Context       int      `json:"context"`                // Number of context lines (default: 3)
	IgnoreWhitespace bool  `json:"ignore_whitespace"`      // Ignore whitespace changes
	FileTypes     []string `json:"file_types,omitempty"`   // Filter by file extensions
	MaxLines      int      `json:"max_lines"`              // Limit output lines
}

// GitBlameOptions configures git blame analysis
type GitBlameOptions struct {
	FilePath     string `json:"file_path"`              // Specific file to blame
	StartLine    int    `json:"start_line,omitempty"`   // Start line number
	EndLine      int    `json:"end_line,omitempty"`     // End line number
	ShowEmail    bool   `json:"show_email"`             // Show email addresses
	ShowDate     bool   `json:"show_date"`              // Show commit dates
	IgnoreRevs   []string `json:"ignore_revs,omitempty"` // Revisions to ignore
	IgnoreWhitespace bool `json:"ignore_whitespace"`     // Ignore whitespace changes
}

// Git result structures
type GitStatusResult struct {
	Branch          string              `json:"branch"`
	Ahead           int                 `json:"ahead"`
	Behind          int                 `json:"behind"`
	Modified        []GitFileStatus     `json:"modified"`
	Added           []GitFileStatus     `json:"added"`
	Deleted         []GitFileStatus     `json:"deleted"`
	Renamed         []GitFileStatus     `json:"renamed"`
	Untracked       []GitFileStatus     `json:"untracked"`
	Staged          []GitFileStatus     `json:"staged"`
	TotalChanges    int                 `json:"total_changes"`
	LanguageStats   map[string]int      `json:"language_stats"`
	ExecutionTime   string              `json:"execution_time"`
}

type GitLogResult struct {
	Commits         []GitCommit        `json:"commits"`
	TotalCommits    int                `json:"total_commits"`
	Authors         map[string]int     `json:"authors"`
	FileStats       map[string]int     `json:"file_stats"`
	LanguageStats   map[string]int     `json:"language_stats"`
	DateRange       GitDateRange       `json:"date_range"`
	ExecutionTime   string             `json:"execution_time"`
}

type GitDiffResult struct {
	Files           []GitDiffFile      `json:"files"`
	TotalFiles      int                `json:"total_files"`
	Additions       int                `json:"additions"`
	Deletions       int                `json:"deletions"`
	LanguageStats   map[string]int     `json:"language_stats"`
	FormattedDiff   string             `json:"formatted_diff"`
	ExecutionTime   string             `json:"execution_time"`
}

type GitBlameResult struct {
	FilePath        string             `json:"file_path"`
	Lines           []GitBlameLine     `json:"lines"`
	Authors         map[string]int     `json:"authors"`
	TotalLines      int                `json:"total_lines"`
	ExecutionTime   string             `json:"execution_time"`
}

type GitFileStatus struct {
	Path         string  `json:"path"`
	Status       string  `json:"status"`  // M, A, D, R, etc.
	Language     string  `json:"language"`
	Size         int64   `json:"size"`
	RelativePath string  `json:"relative_path"`
}

type GitCommit struct {
	Hash         string                 `json:"hash"`
	ShortHash    string                 `json:"short_hash"`
	Author       string                 `json:"author"`
	Email        string                 `json:"email"`
	Date         time.Time              `json:"date"`
	Subject      string                 `json:"subject"`
	Body         string                 `json:"body,omitempty"`
	FilesChanged []string               `json:"files_changed,omitempty"`
	Stats        *GitCommitStats        `json:"stats,omitempty"`
}

type GitCommitStats struct {
	FilesChanged int `json:"files_changed"`
	Insertions   int `json:"insertions"`
	Deletions    int `json:"deletions"`
}

type GitDiffFile struct {
	Path       string `json:"path"`
	OldPath    string `json:"old_path,omitempty"`
	Status     string `json:"status"`
	Language   string `json:"language"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	IsBinary   bool   `json:"is_binary"`
}

type GitBlameLine struct {
	LineNumber int       `json:"line_number"`
	Hash       string    `json:"hash"`
	ShortHash  string    `json:"short_hash"`
	Author     string    `json:"author"`
	Email      string    `json:"email,omitempty"`
	Date       time.Time `json:"date"`
	Content    string    `json:"content"`
}

type GitDateRange struct {
	Earliest time.Time `json:"earliest"`
	Latest   time.Time `json:"latest"`
}

// Constructor functions
func NewGitStatusExecutor() *GitStatusExecutor {
	return &GitStatusExecutor{
		contextDetector: context.NewDefaultContextDetector(),
	}
}

func NewGitLogExecutor() *GitLogExecutor {
	return &GitLogExecutor{
		contextDetector: context.NewDefaultContextDetector(),
	}
}

func NewGitDiffExecutor() *GitDiffExecutor {
	return &GitDiffExecutor{
		contextDetector: context.NewDefaultContextDetector(),
	}
}

func NewGitBlameExecutor() *GitBlameExecutor {
	return &GitBlameExecutor{
		contextDetector: context.NewDefaultContextDetector(),
	}
}

// GitStatusExecutor implementation
func (e *GitStatusExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	opts, err := e.parseStatusOptions(params)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Invalid parameters: %v", err),
		}, err
	}

	// Get project context
	ctx, err := e.contextDetector.DetectContext(opts.Path)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to get project context: %v", err),
		}, err
	}

	// Check if we're in a Git repository
	if !e.isGitRepo(opts.Path) {
		return &ToolResult{
			Success: false,
			ErrorMessage: "Not a git repository",
		}, fmt.Errorf("not a git repository")
	}

	result := &GitStatusResult{
		LanguageStats: make(map[string]int),
	}

	// Get branch information
	if opts.ShowBranch {
		branch, ahead, behind, err := e.getBranchInfo(opts.Path)
		if err == nil {
			result.Branch = branch
			result.Ahead = ahead
			result.Behind = behind
		}
	}

	// Get status information
	err = e.parseGitStatus(opts.Path, opts, result, ctx)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to get git status: %v", err),
		}, err
	}

	result.ExecutionTime = time.Since(startTime).String()
	
	summary := e.generateStatusSummary(result)

	return &ToolResult{
		Success: true,
		Result:  result,
		Metadata: map[string]interface{}{
			"summary":        summary,
			"execution_time": result.ExecutionTime,
			"project_type":   ctx.PrimaryLanguage,
		},
	}, nil
}

func (e *GitStatusExecutor) parseStatusOptions(params map[string]interface{}) (*GitStatusOptions, error) {
	opts := &GitStatusOptions{
		Path:           ".",
		IncludeUntracked: true,
		ShowBranch:     true,
		ShowRemote:     true,
	}

	if path, ok := params["path"].(string); ok {
		opts.Path = path
	}

	if untracked, ok := params["include_untracked"].(bool); ok {
		opts.IncludeUntracked = untracked
	}

	if branch, ok := params["show_branch"].(bool); ok {
		opts.ShowBranch = branch
	}

	if remote, ok := params["show_remote"].(bool); ok {
		opts.ShowRemote = remote
	}

	if fileTypes, ok := params["file_types"].([]interface{}); ok {
		opts.FileTypes = make([]string, len(fileTypes))
		for i, ft := range fileTypes {
			if str, ok := ft.(string); ok {
				opts.FileTypes[i] = str
			}
		}
	}

	if excludes, ok := params["exclude_patterns"].([]interface{}); ok {
		opts.ExcludePatterns = make([]string, len(excludes))
		for i, exc := range excludes {
			if str, ok := exc.(string); ok {
				opts.ExcludePatterns[i] = str
			}
		}
	}

	return opts, nil
}

func (e *GitStatusExecutor) isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (e *GitStatusExecutor) getBranchInfo(path string) (string, int, int, error) {
	cmd := exec.Command("git", "-C", path, "status", "-b", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return "", 0, 0, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 {
		return "", 0, 0, fmt.Errorf("no branch information")
	}

	branchLine := lines[0]
	if !strings.HasPrefix(branchLine, "## ") {
		return "", 0, 0, fmt.Errorf("invalid branch line format")
	}

	branchInfo := strings.TrimPrefix(branchLine, "## ")
	
	// Parse branch name and tracking info
	parts := strings.Split(branchInfo, "...")
	branch := parts[0]
	
	ahead, behind := 0, 0
	if len(parts) > 1 && strings.Contains(branchInfo, "[") {
		// Parse [ahead N, behind M] format
		re := regexp.MustCompile(`\[ahead (\d+)(?:, behind (\d+))?\]`)
		matches := re.FindStringSubmatch(branchInfo)
		if len(matches) > 1 {
			ahead, _ = strconv.Atoi(matches[1])
		}
		if len(matches) > 2 && matches[2] != "" {
			behind, _ = strconv.Atoi(matches[2])
		}
	}

	return branch, ahead, behind, nil
}

func (e *GitStatusExecutor) parseGitStatus(path string, opts *GitStatusOptions, result *GitStatusResult, ctx *context.ProjectContext) error {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	if opts.IncludeUntracked {
		cmd.Args = append(cmd.Args, "-u")
	}

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		filePath := strings.TrimSpace(line[3:])
		
		// Handle renamed files (format: "R  old -> new")
		if strings.Contains(filePath, " -> ") {
			parts := strings.Split(filePath, " -> ")
			if len(parts) == 2 {
				filePath = parts[1] // Use new name
			}
		}

		// Apply file type filtering
		if len(opts.FileTypes) > 0 {
			ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
			found := false
			for _, ft := range opts.FileTypes {
				if ext == ft {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Apply exclude patterns
		if e.shouldExcludeFile(filePath, opts.ExcludePatterns) {
			continue
		}

		fileStatus := e.createFileStatus(path, filePath, status, ctx)
		
		// Categorize by status
		switch {
		case strings.Contains(status, "M"):
			result.Modified = append(result.Modified, fileStatus)
		case strings.Contains(status, "A"):
			result.Added = append(result.Added, fileStatus)
		case strings.Contains(status, "D"):
			result.Deleted = append(result.Deleted, fileStatus)
		case strings.Contains(status, "R"):
			result.Renamed = append(result.Renamed, fileStatus)
		case strings.Contains(status, "?"):
			result.Untracked = append(result.Untracked, fileStatus)
		}

		// Check if staged
		if status[0] != ' ' && status[0] != '?' {
			result.Staged = append(result.Staged, fileStatus)
		}

		// Update language statistics
		if fileStatus.Language != "" {
			result.LanguageStats[fileStatus.Language]++
		}
	}

	result.TotalChanges = len(result.Modified) + len(result.Added) + 
		len(result.Deleted) + len(result.Renamed) + len(result.Untracked)

	return nil
}

func (e *GitStatusExecutor) shouldExcludeFile(filePath string, excludePatterns []string) bool {
	for _, pattern := range excludePatterns {
		matched, _ := filepath.Match(pattern, filepath.Base(filePath))
		if matched {
			return true
		}
	}
	return false
}

func (e *GitStatusExecutor) createFileStatus(basePath, filePath, status string, ctx *context.ProjectContext) GitFileStatus {
	fullPath := filepath.Join(basePath, filePath)
	language := detectLanguageFromPath(filePath)
	
	var size int64
	if info, err := os.Stat(fullPath); err == nil {
		size = info.Size()
	}

	return GitFileStatus{
		Path:         fullPath,
		Status:       status,
		Language:     language,
		Size:         size,
		RelativePath: filePath,
	}
}

func (e *GitStatusExecutor) generateStatusSummary(result *GitStatusResult) string {
	if result.TotalChanges == 0 {
		return "Working tree clean"
	}

	parts := []string{}
	
	if len(result.Modified) > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", len(result.Modified)))
	}
	if len(result.Added) > 0 {
		parts = append(parts, fmt.Sprintf("%d added", len(result.Added)))
	}
	if len(result.Deleted) > 0 {
		parts = append(parts, fmt.Sprintf("%d deleted", len(result.Deleted)))
	}
	if len(result.Untracked) > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", len(result.Untracked)))
	}

	summary := fmt.Sprintf("Repository status: %s", strings.Join(parts, ", "))
	
	if result.Branch != "" {
		branchInfo := fmt.Sprintf(" on branch '%s'", result.Branch)
		if result.Ahead > 0 || result.Behind > 0 {
			tracking := []string{}
			if result.Ahead > 0 {
				tracking = append(tracking, fmt.Sprintf("ahead %d", result.Ahead))
			}
			if result.Behind > 0 {
				tracking = append(tracking, fmt.Sprintf("behind %d", result.Behind))
			}
			branchInfo += fmt.Sprintf(" (%s)", strings.Join(tracking, ", "))
		}
		summary += branchInfo
	}

	return summary
}

// GitLogExecutor implementation
func (e *GitLogExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	opts, err := e.parseLogOptions(params)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Invalid parameters: %v", err),
		}, err
	}

	// Get project context
	ctx, err := e.contextDetector.DetectContext(opts.Path)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to get project context: %v", err),
		}, err
	}

	// Check if we're in a Git repository
	if !e.isGitRepo(opts.Path) {
		return &ToolResult{
			Success: false,
			ErrorMessage: "Not a git repository",
		}, fmt.Errorf("not a git repository")
	}

	result := &GitLogResult{
		Authors:       make(map[string]int),
		FileStats:     make(map[string]int),
		LanguageStats: make(map[string]int),
	}

	// Parse git log
	err = e.parseGitLog(opts, result, ctx)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to parse git log: %v", err),
		}, err
	}

	result.ExecutionTime = time.Since(startTime).String()
	summary := e.generateLogSummary(result)

	return &ToolResult{
		Success: true,
		Result:  result,
		Metadata: map[string]interface{}{
			"summary":        summary,
			"execution_time": result.ExecutionTime,
			"project_type":   ctx.PrimaryLanguage,
		},
	}, nil
}

func (e *GitLogExecutor) parseLogOptions(params map[string]interface{}) (*GitLogOptions, error) {
	opts := &GitLogOptions{
		Path:       ".",
		MaxCommits: 50,
		OneLine:    false,
		ShowStats:  false,
		ShowGraph:  false,
	}

	if path, ok := params["path"].(string); ok {
		opts.Path = path
	}

	if maxCommits, ok := params["max_commits"].(int); ok {
		opts.MaxCommits = maxCommits
	}

	if since, ok := params["since"].(string); ok {
		opts.Since = since
	}

	if until, ok := params["until"].(string); ok {
		opts.Until = until
	}

	if author, ok := params["author"].(string); ok {
		opts.Author = author
	}

	if branch, ok := params["branch"].(string); ok {
		opts.Branch = branch
	}

	if oneLine, ok := params["one_line"].(bool); ok {
		opts.OneLine = oneLine
	}

	if showStats, ok := params["show_stats"].(bool); ok {
		opts.ShowStats = showStats
	}

	if showGraph, ok := params["show_graph"].(bool); ok {
		opts.ShowGraph = showGraph
	}

	if fileTypes, ok := params["file_types"].([]interface{}); ok {
		opts.FileTypes = make([]string, len(fileTypes))
		for i, ft := range fileTypes {
			if str, ok := ft.(string); ok {
				opts.FileTypes[i] = str
			}
		}
	}

	return opts, nil
}

func (e *GitLogExecutor) isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (e *GitLogExecutor) parseGitLog(opts *GitLogOptions, result *GitLogResult, ctx *context.ProjectContext) error {
	// Build git log command
	args := []string{"-C", opts.Path, "log", "--pretty=format:%H|%h|%an|%ae|%ad|%s|%b", "--date=iso"}
	
	if opts.MaxCommits > 0 {
		args = append(args, fmt.Sprintf("-%d", opts.MaxCommits))
	}

	if opts.Since != "" {
		args = append(args, "--since="+opts.Since)
	}

	if opts.Until != "" {
		args = append(args, "--until="+opts.Until)
	}

	if opts.Author != "" {
		args = append(args, "--author="+opts.Author)
	}

	if opts.Branch != "" {
		args = append(args, opts.Branch)
	}

	if opts.ShowStats {
		args = append(args, "--stat", "--name-only")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git log failed: %v", err)
	}

	// Parse commits
	commitBlocks := strings.Split(strings.TrimSpace(string(output)), "\n\n")
	for _, block := range commitBlocks {
		if block == "" {
			continue
		}

		commit, err := e.parseCommitBlock(block, opts.ShowStats)
		if err != nil {
			continue // Skip malformed commits
		}

		// Apply file type filtering
		if len(opts.FileTypes) > 0 && len(commit.FilesChanged) > 0 {
			filteredFiles := []string{}
			for _, file := range commit.FilesChanged {
				ext := strings.TrimPrefix(filepath.Ext(file), ".")
				for _, ft := range opts.FileTypes {
					if ext == ft {
						filteredFiles = append(filteredFiles, file)
						break
					}
				}
			}
			if len(filteredFiles) == 0 {
				continue // Skip commits with no matching files
			}
			commit.FilesChanged = filteredFiles
		}

		result.Commits = append(result.Commits, commit)
		
		// Update statistics
		result.Authors[commit.Author]++
		
		for _, file := range commit.FilesChanged {
			result.FileStats[file]++
			language := detectLanguageFromPath(file)
			if language != "" {
				result.LanguageStats[language]++
			}
		}
	}

	result.TotalCommits = len(result.Commits)
	
	// Calculate date range
	if len(result.Commits) > 0 {
		result.DateRange.Latest = result.Commits[0].Date
		result.DateRange.Earliest = result.Commits[len(result.Commits)-1].Date
	}

	return nil
}

func (e *GitLogExecutor) parseCommitBlock(block string, includeStats bool) (GitCommit, error) {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return GitCommit{}, fmt.Errorf("empty commit block")
	}

	// Parse the main commit line
	parts := strings.Split(lines[0], "|")
	if len(parts) < 6 {
		return GitCommit{}, fmt.Errorf("invalid commit format")
	}

	commit := GitCommit{
		Hash:      parts[0],
		ShortHash: parts[1],
		Author:    parts[2],
		Email:     parts[3],
		Subject:   parts[5],
	}

	// Parse date
	if date, err := time.Parse("2006-01-02 15:04:05 -0700", parts[4]); err == nil {
		commit.Date = date
	}

	// Parse body (if present)
	if len(parts) > 6 && parts[6] != "" {
		commit.Body = strings.TrimSpace(parts[6])
	}

	// Parse file list (if stats enabled)
	if includeStats && len(lines) > 1 {
		files := []string{}
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line != "" && !strings.Contains(line, "file") && !strings.Contains(line, "changed") {
				files = append(files, line)
			}
		}
		commit.FilesChanged = files
	}

	return commit, nil
}

func (e *GitLogExecutor) generateLogSummary(result *GitLogResult) string {
	if len(result.Commits) == 0 {
		return "No commits found"
	}

	summary := fmt.Sprintf("Found %d commits", result.TotalCommits)
	
	if len(result.Authors) > 0 {
		summary += fmt.Sprintf(" from %d authors", len(result.Authors))
		
		// Find most active author
		maxCommits := 0
		topAuthor := ""
		for author, count := range result.Authors {
			if count > maxCommits {
				maxCommits = count
				topAuthor = author
			}
		}
		if topAuthor != "" {
			summary += fmt.Sprintf(" (most active: %s with %d commits)", topAuthor, maxCommits)
		}
	}

	if !result.DateRange.Earliest.IsZero() && !result.DateRange.Latest.IsZero() {
		duration := result.DateRange.Latest.Sub(result.DateRange.Earliest)
		summary += fmt.Sprintf(" spanning %s", e.formatDuration(duration))
	}

	return summary
}

func (e *GitLogExecutor) formatDuration(d time.Duration) string {
	if d < time.Hour*24 {
		return fmt.Sprintf("%.1f hours", d.Hours())
	} else if d < time.Hour*24*30 {
		return fmt.Sprintf("%.1f days", d.Hours()/24)
	} else if d < time.Hour*24*365 {
		return fmt.Sprintf("%.1f months", d.Hours()/(24*30))
	} else {
		return fmt.Sprintf("%.1f years", d.Hours()/(24*365))
	}
}

// GitDiffExecutor implementation
func (e *GitDiffExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	opts, err := e.parseDiffOptions(params)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Invalid parameters: %v", err),
		}, err
	}

	// Get project context
	ctx, err := e.contextDetector.DetectContext(opts.Path)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to get project context: %v", err),
		}, err
	}

	// Check if we're in a Git repository
	if !e.isGitRepo(opts.Path) {
		return &ToolResult{
			Success: false,
			ErrorMessage: "Not a git repository",
		}, fmt.Errorf("not a git repository")
	}

	result := &GitDiffResult{
		LanguageStats: make(map[string]int),
	}

	// Parse git diff
	err = e.parseGitDiff(opts, result, ctx)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to parse git diff: %v", err),
		}, err
	}

	result.ExecutionTime = time.Since(startTime).String()
	summary := e.generateDiffSummary(result)

	return &ToolResult{
		Success: true,
		Result:  result,
		Metadata: map[string]interface{}{
			"summary":        summary,
			"execution_time": result.ExecutionTime,
			"project_type":   ctx.PrimaryLanguage,
		},
	}, nil
}

func (e *GitDiffExecutor) parseDiffOptions(params map[string]interface{}) (*GitDiffOptions, error) {
	opts := &GitDiffOptions{
		Path:      ".",
		Context:   3,
		MaxLines:  1000,
	}

	if path, ok := params["path"].(string); ok {
		opts.Path = path
	}

	if revision, ok := params["revision"].(string); ok {
		opts.Revision = revision
	}

	if staged, ok := params["staged"].(bool); ok {
		opts.Staged = staged
	}

	if cached, ok := params["cached"].(bool); ok {
		opts.Cached = cached
	}

	if nameOnly, ok := params["name_only"].(bool); ok {
		opts.NameOnly = nameOnly
	}

	if stat, ok := params["stat"].(bool); ok {
		opts.Stat = stat
	}

	if wordDiff, ok := params["word_diff"].(bool); ok {
		opts.WordDiff = wordDiff
	}

	if context, ok := params["context"].(int); ok {
		opts.Context = context
	}

	if ignoreWS, ok := params["ignore_whitespace"].(bool); ok {
		opts.IgnoreWhitespace = ignoreWS
	}

	if maxLines, ok := params["max_lines"].(int); ok {
		opts.MaxLines = maxLines
	}

	if fileTypes, ok := params["file_types"].([]interface{}); ok {
		opts.FileTypes = make([]string, len(fileTypes))
		for i, ft := range fileTypes {
			if str, ok := ft.(string); ok {
				opts.FileTypes[i] = str
			}
		}
	}

	return opts, nil
}

func (e *GitDiffExecutor) isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (e *GitDiffExecutor) parseGitDiff(opts *GitDiffOptions, result *GitDiffResult, ctx *context.ProjectContext) error {
	// Build git diff command
	args := []string{"-C", opts.Path, "diff"}

	if opts.Staged || opts.Cached {
		args = append(args, "--cached")
	}

	if opts.NameOnly {
		args = append(args, "--name-only")
	}

	if opts.Stat {
		args = append(args, "--stat")
	}

	if opts.WordDiff {
		args = append(args, "--word-diff")
	}

	if opts.Context > 0 {
		args = append(args, fmt.Sprintf("-U%d", opts.Context))
	}

	if opts.IgnoreWhitespace {
		args = append(args, "--ignore-all-space")
	}

	if opts.Revision != "" {
		args = append(args, opts.Revision)
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git diff failed: %v", err)
	}

	diffOutput := string(output)
	result.FormattedDiff = e.limitOutput(diffOutput, opts.MaxLines)

	// Parse diff for file information
	err = e.parseDiffFiles(diffOutput, result, ctx, opts.FileTypes)
	if err != nil {
		return fmt.Errorf("failed to parse diff files: %v", err)
	}

	return nil
}

func (e *GitDiffExecutor) parseDiffFiles(diffOutput string, result *GitDiffResult, ctx *context.ProjectContext, fileTypes []string) error {
	lines := strings.Split(diffOutput, "\n")
	var currentFile *GitDiffFile
	
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// New file diff
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				filePath := strings.TrimPrefix(parts[3], "b/")
				
				// Apply file type filtering
				if len(fileTypes) > 0 {
					ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
					found := false
					for _, ft := range fileTypes {
						if ext == ft {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}

				currentFile = &GitDiffFile{
					Path:     filePath,
					Language: detectLanguageFromPath(filePath),
				}
				result.Files = append(result.Files, *currentFile)
			}
		} else if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			// File path information
			if currentFile != nil && strings.HasPrefix(line, "---") {
				path := strings.TrimPrefix(line, "--- a/")
				if path != "/dev/null" {
					currentFile.OldPath = path
				}
			}
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			// Addition
			if currentFile != nil {
				currentFile.Additions++
				result.Additions++
			}
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// Deletion
			if currentFile != nil {
				currentFile.Deletions++
				result.Deletions++
			}
		} else if strings.Contains(line, "Binary files") {
			// Binary file
			if currentFile != nil {
				currentFile.IsBinary = true
			}
		}
	}

	result.TotalFiles = len(result.Files)
	
	// Update language statistics
	for _, file := range result.Files {
		if file.Language != "" {
			result.LanguageStats[file.Language]++
		}
	}

	return nil
}

func (e *GitDiffExecutor) limitOutput(output string, maxLines int) string {
	if maxLines <= 0 {
		return output
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}

	limitedLines := lines[:maxLines]
	limitedLines = append(limitedLines, fmt.Sprintf("... (truncated %d lines)", len(lines)-maxLines))
	return strings.Join(limitedLines, "\n")
}

func (e *GitDiffExecutor) generateDiffSummary(result *GitDiffResult) string {
	if result.TotalFiles == 0 {
		return "No changes found"
	}

	summary := fmt.Sprintf("Changes in %d files", result.TotalFiles)
	
	if result.Additions > 0 || result.Deletions > 0 {
		changes := []string{}
		if result.Additions > 0 {
			changes = append(changes, fmt.Sprintf("+%d", result.Additions))
		}
		if result.Deletions > 0 {
			changes = append(changes, fmt.Sprintf("-%d", result.Deletions))
		}
		summary += fmt.Sprintf(" (%s)", strings.Join(changes, ", "))
	}

	return summary
}

// GitBlameExecutor implementation
func (e *GitBlameExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	startTime := time.Now()
	
	opts, err := e.parseBlameOptions(params)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Invalid parameters: %v", err),
		}, err
	}

	// Get project context (for validation)
	_, err = e.contextDetector.DetectContext(filepath.Dir(opts.FilePath))
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to get project context: %v", err),
		}, err
	}

	// Check if file exists
	if _, err := os.Stat(opts.FilePath); os.IsNotExist(err) {
		return &ToolResult{
			Success: false,
			ErrorMessage: "File does not exist",
		}, fmt.Errorf("file does not exist: %s", opts.FilePath)
	}

	result := &GitBlameResult{
		FilePath: opts.FilePath,
		Authors:  make(map[string]int),
	}

	// Parse git blame
	err = e.parseGitBlame(opts, result)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage: fmt.Sprintf("Failed to parse git blame: %v", err),
		}, err
	}

	result.ExecutionTime = time.Since(startTime).String()
	summary := e.generateBlameSummary(result)

	return &ToolResult{
		Success: true,
		Result:  result,
		Metadata: map[string]interface{}{
			"summary":        summary,
			"execution_time": result.ExecutionTime,
		},
	}, nil
}

func (e *GitBlameExecutor) isGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	_, err := os.Stat(gitDir)
	return err == nil
}

func (e *GitBlameExecutor) parseBlameOptions(params map[string]interface{}) (*GitBlameOptions, error) {
	opts := &GitBlameOptions{
		ShowEmail: false,
		ShowDate:  true,
	}

	filePath, ok := params["file_path"].(string)
	if !ok || filePath == "" {
		return nil, fmt.Errorf("file_path parameter is required")
	}
	opts.FilePath = filePath

	if startLine, ok := params["start_line"].(int); ok {
		opts.StartLine = startLine
	}

	if endLine, ok := params["end_line"].(int); ok {
		opts.EndLine = endLine
	}

	if showEmail, ok := params["show_email"].(bool); ok {
		opts.ShowEmail = showEmail
	}

	if showDate, ok := params["show_date"].(bool); ok {
		opts.ShowDate = showDate
	}

	if ignoreWS, ok := params["ignore_whitespace"].(bool); ok {
		opts.IgnoreWhitespace = ignoreWS
	}

	if ignoreRevs, ok := params["ignore_revs"].([]interface{}); ok {
		opts.IgnoreRevs = make([]string, len(ignoreRevs))
		for i, rev := range ignoreRevs {
			if str, ok := rev.(string); ok {
				opts.IgnoreRevs[i] = str
			}
		}
	}

	return opts, nil
}

func (e *GitBlameExecutor) parseGitBlame(opts *GitBlameOptions, result *GitBlameResult) error {
	// Build git blame command
	args := []string{"blame", "--porcelain"}

	if opts.IgnoreWhitespace {
		args = append(args, "-w")
	}

	for _, rev := range opts.IgnoreRevs {
		args = append(args, "--ignore-rev", rev)
	}

	if opts.StartLine > 0 && opts.EndLine > 0 {
		args = append(args, fmt.Sprintf("-L%d,%d", opts.StartLine, opts.EndLine))
	} else if opts.StartLine > 0 {
		args = append(args, fmt.Sprintf("-L%d,+1", opts.StartLine))
	}

	args = append(args, opts.FilePath)

	// Change to the repository directory
	repoDir := filepath.Dir(opts.FilePath)
	if !e.isGitRepo(repoDir) {
		// Find git repo root
		for repoDir != "/" {
			if e.isGitRepo(repoDir) {
				break
			}
			repoDir = filepath.Dir(repoDir)
		}
	}
	
	// Make file path relative to repo
	relPath, err := filepath.Rel(repoDir, opts.FilePath)
	if err != nil {
		relPath = opts.FilePath
	}
	
	// Update args to use relative path
	args[len(args)-1] = relPath
	
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git blame failed: %v", err)
	}

	// Parse blame output
	err = e.parseBlameOutput(string(output), result)
	if err != nil {
		return fmt.Errorf("failed to parse blame output: %v", err)
	}

	return nil
}

func (e *GitBlameExecutor) parseBlameOutput(output string, result *GitBlameResult) error {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	commitInfo := make(map[string]GitCommit)
	
	i := 0
	lineNumber := 1
	
	for i < len(lines) {
		if lines[i] == "" {
			i++
			continue
		}

		// Parse commit line (hash linenum finallinenum groupsize)
		parts := strings.Fields(lines[i])
		if len(parts) < 2 {
			i++
			continue
		}

		hash := parts[0]
		
		// Look for commit info if not cached
		if _, exists := commitInfo[hash]; !exists {
			commit := GitCommit{
				Hash:      hash,
				ShortHash: hash[:gitMin(7, len(hash))],
			}
			
			// Parse commit details
			for j := i + 1; j < len(lines); j++ {
				line := lines[j]
				if strings.HasPrefix(line, "author ") {
					commit.Author = strings.TrimPrefix(line, "author ")
				} else if strings.HasPrefix(line, "author-mail ") {
					email := strings.TrimPrefix(line, "author-mail ")
					commit.Email = strings.Trim(email, "<>")
				} else if strings.HasPrefix(line, "author-time ") {
					timeStr := strings.TrimPrefix(line, "author-time ")
					if timestamp, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
						commit.Date = time.Unix(timestamp, 0)
					}
				} else if strings.HasPrefix(line, "\t") {
					// This is the content line
					break
				}
			}
			
			commitInfo[hash] = commit
		}

		// Find the content line (starts with tab)
		contentLine := ""
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "\t") {
				contentLine = strings.TrimPrefix(lines[j], "\t")
				i = j + 1
				break
			}
		}

		commit := commitInfo[hash]
		blameLine := GitBlameLine{
			LineNumber: lineNumber,
			Hash:       commit.Hash,
			ShortHash:  commit.ShortHash,
			Author:     commit.Author,
			Email:      commit.Email,
			Date:       commit.Date,
			Content:    contentLine,
		}

		result.Lines = append(result.Lines, blameLine)
		result.Authors[commit.Author]++
		lineNumber++
	}

	result.TotalLines = len(result.Lines)
	return nil
}

func (e *GitBlameExecutor) generateBlameSummary(result *GitBlameResult) string {
	if result.TotalLines == 0 {
		return "No blame information available"
	}

	summary := fmt.Sprintf("Blame for %s: %d lines", filepath.Base(result.FilePath), result.TotalLines)
	
	if len(result.Authors) > 0 {
		summary += fmt.Sprintf(" from %d authors", len(result.Authors))
		
		// Find most active author
		maxLines := 0
		topAuthor := ""
		for author, count := range result.Authors {
			if count > maxLines {
				maxLines = count
				topAuthor = author
			}
		}
		if topAuthor != "" {
			percentage := float64(maxLines) / float64(result.TotalLines) * 100
			summary += fmt.Sprintf(" (most: %s %.1f%%)", topAuthor, percentage)
		}
	}

	return summary
}

func gitMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// detectLanguageFromPath detects programming language from file path based on extension
func detectLanguageFromPath(filePath string) string {
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
	
	// Common language mappings
	langMap := map[string]string{
		"go":     "Go",
		"js":     "JavaScript", 
		"ts":     "TypeScript",
		"py":     "Python",
		"java":   "Java",
		"cpp":    "C++",
		"cc":     "C++",
		"cxx":    "C++",
		"c":      "C",
		"h":      "C",
		"hpp":    "C++",
		"rs":     "Rust",
		"rb":     "Ruby",
		"php":    "PHP",
		"swift":  "Swift",
		"kt":     "Kotlin",
		"scala":  "Scala",
		"sh":     "Shell",
		"bash":   "Shell",
		"zsh":    "Shell",
		"fish":   "Shell",
		"ps1":    "PowerShell",
		"sql":    "SQL",
		"html":   "HTML",
		"css":    "CSS",
		"scss":   "SCSS",
		"sass":   "SASS",
		"less":   "Less",
		"xml":    "XML",
		"json":   "JSON",
		"yaml":   "YAML",
		"yml":    "YAML",
		"toml":   "TOML",
		"md":     "Markdown",
		"tex":    "TeX",
		"r":      "R",
		"R":      "R",
		"m":      "Objective-C",
		"mm":     "Objective-C++",
		"pl":     "Perl",
		"pm":     "Perl",
		"lua":    "Lua",
		"vim":    "Vim",
		"hs":     "Haskell",
		"clj":    "Clojure",
		"ex":     "Elixir",
		"exs":    "Elixir",
		"erl":    "Erlang",
		"dart":   "Dart",
	}
	
	if lang, exists := langMap[ext]; exists {
		return lang
	}
	
	return "" // Unknown language
}

// Tool interface implementations
func (e *GitStatusExecutor) Name() string         { return "git_status" }
func (e *GitStatusExecutor) Description() string  { 
	return "Get git repository status with file changes, branch information, and project context awareness" 
}
func (e *GitLogExecutor) Name() string            { return "git_log" }
func (e *GitLogExecutor) Description() string     { 
	return "Analyze git commit history with author statistics, file change tracking, and timeline analysis" 
}
func (e *GitDiffExecutor) Name() string           { return "git_diff" }
func (e *GitDiffExecutor) Description() string    { 
	return "Show git differences with syntax-aware formatting, change statistics, and context-sensitive output" 
}
func (e *GitBlameExecutor) Name() string          { return "git_blame" }
func (e *GitBlameExecutor) Description() string   { 
	return "Show line-by-line authorship information with commit details and change history" 
}