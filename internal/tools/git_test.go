package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test Git Status Executor
func TestGitStatusExecutor(t *testing.T) {
	// Create temporary git repo for testing
	tmpDir := createTempGitRepo(t)
	defer os.RemoveAll(tmpDir)

	executor := NewGitStatusExecutor()

	tests := []struct {
		name     string
		params   map[string]interface{}
		wantErr  bool
	}{
		{
			name: "valid git repo status",
			params: map[string]interface{}{
				"path":             tmpDir,
				"include_untracked": true,
				"show_branch":      true,
			},
			wantErr: false,
		},
		{
			name: "non-git directory",
			params: map[string]interface{}{
				"path": "/tmp",
			},
			wantErr: true,
		},
		{
			name: "with file type filtering",
			params: map[string]interface{}{
				"path":       tmpDir,
				"file_types": []interface{}{"go", "txt"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(tt.params)
			
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("expected result but got nil")
				}
				if !result.Success {
					t.Errorf("expected success but got: %s", result.ErrorMessage)
				}

				// Validate result structure
				statusResult, ok := result.Result.(*GitStatusResult)
				if !ok {
					t.Error("expected GitStatusResult")
				} else {
					if statusResult.ExecutionTime == "" {
						t.Error("execution time should be set")
					}
					if statusResult.Branch == "" && tt.params["show_branch"] == true {
						t.Error("branch should be set when show_branch is true")
					}
				}
			}
		})
	}
}

func TestGitStatusExecutor_ParseOptions(t *testing.T) {
	executor := NewGitStatusExecutor()

	tests := []struct {
		name   string
		params map[string]interface{}
		want   *GitStatusOptions
	}{
		{
			name:   "default options",
			params: map[string]interface{}{},
			want: &GitStatusOptions{
				Path:           ".",
				IncludeUntracked: true,
				ShowBranch:     true,
				ShowRemote:     true,
			},
		},
		{
			name: "custom options",
			params: map[string]interface{}{
				"path":             "/custom/path",
				"include_untracked": false,
				"show_branch":      false,
				"file_types":       []interface{}{"go", "js"},
			},
			want: &GitStatusOptions{
				Path:           "/custom/path",
				IncludeUntracked: false,
				ShowBranch:     false,
				ShowRemote:     true,
				FileTypes:      []string{"go", "js"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := executor.parseStatusOptions(tt.params)
			if err != nil {
				t.Errorf("parseStatusOptions() error = %v", err)
				return
			}
			
			if got.Path != tt.want.Path {
				t.Errorf("Path = %v, want %v", got.Path, tt.want.Path)
			}
			if got.IncludeUntracked != tt.want.IncludeUntracked {
				t.Errorf("IncludeUntracked = %v, want %v", got.IncludeUntracked, tt.want.IncludeUntracked)
			}
			if got.ShowBranch != tt.want.ShowBranch {
				t.Errorf("ShowBranch = %v, want %v", got.ShowBranch, tt.want.ShowBranch)
			}
		})
	}
}

// Test Git Log Executor
func TestGitLogExecutor(t *testing.T) {
	// Create temporary git repo for testing
	tmpDir := createTempGitRepo(t)
	defer os.RemoveAll(tmpDir)

	executor := NewGitLogExecutor()

	tests := []struct {
		name     string
		params   map[string]interface{}
		wantErr  bool
	}{
		{
			name: "valid git repo log",
			params: map[string]interface{}{
				"path":        tmpDir,
				"max_commits": 10,
			},
			wantErr: false,
		},
		{
			name: "with author filter",
			params: map[string]interface{}{
				"path":   tmpDir,
				"author": "Test Author",
			},
			wantErr: false,
		},
		{
			name: "with file type filter",
			params: map[string]interface{}{
				"path":       tmpDir,
				"file_types": []interface{}{"go"},
				"show_stats": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(tt.params)
			
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("expected result but got nil")
				}
				if !result.Success {
					t.Errorf("expected success but got: %s", result.ErrorMessage)
				}

				// Validate result structure
				logResult, ok := result.Result.(*GitLogResult)
				if !ok {
					t.Error("expected GitLogResult")
				} else {
					if logResult.ExecutionTime == "" {
						t.Error("execution time should be set")
					}
					if logResult.Authors == nil {
						t.Error("authors map should be initialized")
					}
				}
			}
		})
	}
}

// Test Git Diff Executor
func TestGitDiffExecutor(t *testing.T) {
	// Create temporary git repo for testing
	tmpDir := createTempGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a change to diff
	testFile := filepath.Join(tmpDir, "test.go")
	os.WriteFile(testFile, []byte("package main\n\nfunc main() {\n\tprintln(\"modified\")\n}\n"), 0644)

	executor := NewGitDiffExecutor()

	tests := []struct {
		name     string
		params   map[string]interface{}
		wantErr  bool
	}{
		{
			name: "valid git diff",
			params: map[string]interface{}{
				"path": tmpDir,
			},
			wantErr: false,
		},
		{
			name: "staged diff",
			params: map[string]interface{}{
				"path":   tmpDir,
				"staged": true,
			},
			wantErr: false,
		},
		{
			name: "name only diff",
			params: map[string]interface{}{
				"path":      tmpDir,
				"name_only": true,
			},
			wantErr: false,
		},
		{
			name: "with context lines",
			params: map[string]interface{}{
				"path":    tmpDir,
				"context": 5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(tt.params)
			
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("expected result but got nil")
				}
				if !result.Success {
					t.Errorf("expected success but got: %s", result.ErrorMessage)
				}

				// Validate result structure
				diffResult, ok := result.Result.(*GitDiffResult)
				if !ok {
					t.Error("expected GitDiffResult")
				} else {
					if diffResult.ExecutionTime == "" {
						t.Error("execution time should be set")
					}
					if diffResult.LanguageStats == nil {
						t.Error("language stats should be initialized")
					}
				}
			}
		})
	}
}

// Test Git Blame Executor
func TestGitBlameExecutor(t *testing.T) {
	// Create temporary git repo for testing
	tmpDir := createTempGitRepo(t)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.go")

	executor := NewGitBlameExecutor()

	tests := []struct {
		name     string
		params   map[string]interface{}
		wantErr  bool
	}{
		{
			name: "valid file blame",
			params: map[string]interface{}{
				"file_path": testFile,
			},
			wantErr: false,
		},
		{
			name: "with line range",
			params: map[string]interface{}{
				"file_path":  testFile,
				"start_line": 1,
				"end_line":   3,
			},
			wantErr: false,
		},
		{
			name: "show email and date",
			params: map[string]interface{}{
				"file_path":  testFile,
				"show_email": true,
				"show_date":  true,
			},
			wantErr: false,
		},
		{
			name: "nonexistent file",
			params: map[string]interface{}{
				"file_path": "/nonexistent/file.go",
			},
			wantErr: true,
		},
		{
			name: "missing file_path parameter",
			params: map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(tt.params)
			
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("expected result but got nil")
				}
				if !result.Success {
					t.Errorf("expected success but got: %s", result.ErrorMessage)
				}

				// Validate result structure
				blameResult, ok := result.Result.(*GitBlameResult)
				if !ok {
					t.Error("expected GitBlameResult")
				} else {
					if blameResult.ExecutionTime == "" {
						t.Error("execution time should be set")
					}
					if blameResult.FilePath != tt.params["file_path"] {
						t.Error("file path should match input")
					}
					if blameResult.Authors == nil {
						t.Error("authors map should be initialized")
					}
				}
			}
		})
	}
}

// Test Tool Interface Implementations
func TestGitToolInterfaces(t *testing.T) {
	tests := []struct {
		name        string
		executor    interface {
			Name() string
			Description() string
		}
		expectedName string
	}{
		{
			name:         "GitStatusExecutor",
			executor:     NewGitStatusExecutor(),
			expectedName: "git_status",
		},
		{
			name:         "GitLogExecutor", 
			executor:     NewGitLogExecutor(),
			expectedName: "git_log",
		},
		{
			name:         "GitDiffExecutor",
			executor:     NewGitDiffExecutor(),
			expectedName: "git_diff",
		},
		{
			name:         "GitBlameExecutor",
			executor:     NewGitBlameExecutor(),
			expectedName: "git_blame",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if name := tt.executor.Name(); name != tt.expectedName {
				t.Errorf("Name() = %v, want %v", name, tt.expectedName)
			}
			if desc := tt.executor.Description(); desc == "" {
				t.Error("Description() should not be empty")
			}
		})
	}
}

// Test Git Status Summary Generation
func TestGitStatusSummaryGeneration(t *testing.T) {
	executor := NewGitStatusExecutor()

	tests := []struct {
		name   string
		result *GitStatusResult
		want   string
	}{
		{
			name: "clean working tree",
			result: &GitStatusResult{
				TotalChanges: 0,
			},
			want: "Working tree clean",
		},
		{
			name: "with changes",
			result: &GitStatusResult{
				Modified:     []GitFileStatus{{}, {}},
				Added:        []GitFileStatus{{}},
				Untracked:    []GitFileStatus{{}, {}, {}},
				TotalChanges: 6,
				Branch:       "main",
				Ahead:        2,
				Behind:       1,
			},
			want: "Repository status: 2 modified, 1 added, 3 untracked on branch 'main' (ahead 2, behind 1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.generateStatusSummary(tt.result)
			if !strings.Contains(got, "Working tree clean") && !strings.Contains(got, "Repository status") {
				t.Errorf("generateStatusSummary() = %v, expected format not found", got)
			}
		})
	}
}

// Test Git Log Duration Formatting
func TestGitLogDurationFormatting(t *testing.T) {
	executor := NewGitLogExecutor()

	tests := []struct {
		name     string
		duration time.Duration
		wantUnit string
	}{
		{
			name:     "hours",
			duration: 5 * time.Hour,
			wantUnit: "hours",
		},
		{
			name:     "days",
			duration: 72 * time.Hour,
			wantUnit: "days",
		},
		{
			name:     "months",
			duration: 45 * 24 * time.Hour,
			wantUnit: "months",
		},
		{
			name:     "years", 
			duration: 400 * 24 * time.Hour,
			wantUnit: "years",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.formatDuration(tt.duration)
			if !strings.Contains(got, tt.wantUnit) {
				t.Errorf("formatDuration() = %v, should contain %v", got, tt.wantUnit)
			}
		})
	}
}

// Test Git Diff Output Limiting
func TestGitDiffOutputLimiting(t *testing.T) {
	executor := NewGitDiffExecutor()

	tests := []struct {
		name     string
		output   string
		maxLines int
		want     int // expected number of lines
	}{
		{
			name:     "no limit",
			output:   "line1\nline2\nline3\nline4",
			maxLines: 0,
			want:     4,
		},
		{
			name:     "within limit",
			output:   "line1\nline2",
			maxLines: 5,
			want:     2,
		},
		{
			name:     "exceeds limit",
			output:   "line1\nline2\nline3\nline4\nline5",
			maxLines: 3,
			want:     4, // 3 original lines + 1 truncation message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.limitOutput(tt.output, tt.maxLines)
			lines := strings.Split(got, "\n")
			if len(lines) != tt.want {
				t.Errorf("limitOutput() produced %d lines, want %d", len(lines), tt.want)
			}
			
			if tt.maxLines > 0 && len(strings.Split(tt.output, "\n")) > tt.maxLines {
				if !strings.Contains(got, "truncated") {
					t.Error("expected truncation message")
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkGitStatusExecutor(b *testing.B) {
	tmpDir := createTempGitRepo(b)
	defer os.RemoveAll(tmpDir)

	executor := NewGitStatusExecutor()
	params := map[string]interface{}{
		"path": tmpDir,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGitLogExecutor(b *testing.B) {
	tmpDir := createTempGitRepo(b)
	defer os.RemoveAll(tmpDir)

	executor := NewGitLogExecutor()
	params := map[string]interface{}{
		"path":        tmpDir,
		"max_commits": 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to create temporary git repository for testing
func createTempGitRepo(t interface{}) string {
	var tmpDir string
	var err error
	
	// Handle both testing.T and testing.B
	switch v := t.(type) {
	case *testing.T:
		tmpDir, err = os.MkdirTemp("", "git-test-")
		if err != nil {
			v.Fatal(err)
		}
	case *testing.B:
		tmpDir, err = os.MkdirTemp("", "git-test-")
		if err != nil {
			v.Fatal(err)
		}
	default:
		panic("invalid test type")
	}

	// Initialize git repository
	if err := runGitCommand(tmpDir, "init"); err != nil {
		os.RemoveAll(tmpDir)
		switch v := t.(type) {
		case *testing.T:
			v.Fatal(err)
		case *testing.B:
			v.Fatal(err)
		}
	}

	// Configure git user
	runGitCommand(tmpDir, "config", "user.name", "Test Author")
	runGitCommand(tmpDir, "config", "user.email", "test@example.com")

	// Create initial file and commit
	testFile := filepath.Join(tmpDir, "test.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		os.RemoveAll(tmpDir)
		switch v := t.(type) {
		case *testing.T:
			v.Fatal(err)
		case *testing.B:
			v.Fatal(err)
		}
	}

	// Add and commit
	runGitCommand(tmpDir, "add", ".")
	runGitCommand(tmpDir, "commit", "-m", "Initial commit")

	// Create additional commits for testing
	for i := 1; i <= 3; i++ {
		additionalFile := filepath.Join(tmpDir, fmt.Sprintf("file%d.go", i))
		content := fmt.Sprintf("package main\n\n// File %d\nfunc file%d() {}\n", i, i)
		os.WriteFile(additionalFile, []byte(content), 0644)
		runGitCommand(tmpDir, "add", ".")
		runGitCommand(tmpDir, "commit", "-m", fmt.Sprintf("Add file%d.go", i))
		
		// Add small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	return tmpDir
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	_, err := cmd.Output()
	return err
}