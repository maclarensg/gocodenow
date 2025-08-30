package context

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewContextDetector(t *testing.T) {
	opts := DefaultDetectionOptions()
	detector := NewContextDetector(opts)
	
	if detector == nil {
		t.Fatal("NewContextDetector returned nil")
	}
	
	if detector.options.MaxDepth != opts.MaxDepth {
		t.Errorf("Expected MaxDepth %d, got %d", opts.MaxDepth, detector.options.MaxDepth)
	}
}

func TestDefaultDetectionOptions(t *testing.T) {
	opts := DefaultDetectionOptions()
	
	if opts.MaxDepth != 10 {
		t.Errorf("Expected MaxDepth 10, got %d", opts.MaxDepth)
	}
	
	if opts.CacheMaxAge != 24*time.Hour {
		t.Errorf("Expected CacheMaxAge 24h, got %v", opts.CacheMaxAge)
	}
	
	if !opts.IncludeStats {
		t.Error("Expected IncludeStats to be true")
	}
	
	if !opts.UseCache {
		t.Error("Expected UseCache to be true")
	}
}

func TestIsValidWorkspace(t *testing.T) {
	detector := NewDefaultContextDetector()
	
	// Test with valid directory (current directory)
	wd, _ := os.Getwd()
	if !detector.IsValidWorkspace(wd) {
		t.Error("Current working directory should be a valid workspace")
	}
	
	// Test with non-existent directory
	if detector.IsValidWorkspace("/non/existent/path") {
		t.Error("Non-existent path should not be a valid workspace")
	}
	
	// Test with file instead of directory
	tempFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()
	
	if detector.IsValidWorkspace(tempFile.Name()) {
		t.Error("File should not be a valid workspace")
	}
}

func TestDetectContext(t *testing.T) {
	// Create a temporary workspace for testing
	tempDir, err := os.MkdirTemp("", "test-workspace")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create some test files
	createTestFile(t, tempDir, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`)
	
	createTestFile(t, tempDir, "go.mod", `module test-project

go 1.19

require (
	github.com/example/dep v1.0.0
)`)
	
	createTestFile(t, tempDir, ".gitignore", `*.tmp
.env
node_modules/`)
	
	// Initialize git repository
	if err := os.Mkdir(filepath.Join(tempDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	
	// Test context detection
	detector := NewDefaultContextDetector()
	ctx, err := detector.DetectContext(tempDir)
	if err != nil {
		t.Fatalf("DetectContext failed: %v", err)
	}
	
	// Verify basic context properties
	if ctx.WorkspaceRoot != tempDir {
		t.Errorf("Expected WorkspaceRoot %s, got %s", tempDir, ctx.WorkspaceRoot)
	}
	
	if ctx.ProjectName != "test-project" {
		t.Errorf("Expected ProjectName test-project, got %s", ctx.ProjectName)
	}
	
	if !ctx.Git.IsRepo {
		t.Error("Expected Git.IsRepo to be true")
	}
	
	// Check if Go language was detected
	foundGo := false
	for _, lang := range ctx.Languages {
		if lang.Name == "Go" {
			foundGo = true
			break
		}
	}
	if !foundGo {
		t.Error("Go language should have been detected")
	}
	
	// Check if Go build system was detected
	foundGoBuild := false
	for _, bs := range ctx.BuildSystems {
		if bs.Type == "go" {
			foundGoBuild = true
			break
		}
	}
	if !foundGoBuild {
		t.Error("Go build system should have been detected")
	}
}

func TestLanguageDetection(t *testing.T) {
	detector := NewLanguageDetector()
	
	// Test extension-based detection
	lang, exists := detector.GetLanguageByExtension("go")
	if !exists {
		t.Error("Go language should be supported")
	}
	if lang.Name != "Go" {
		t.Errorf("Expected language name Go, got %s", lang.Name)
	}
	
	// Test unsupported extension
	_, exists = detector.GetLanguageByExtension("xyz")
	if exists {
		t.Error("xyz extension should not be supported")
	}
	
	// Test file analysis with temporary Go file
	tempDir, err := os.MkdirTemp("", "lang-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	goFile := filepath.Join(tempDir, "test.go")
	createTestFile(t, tempDir, "test.go", `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`)
	
	detectedLang, err := detector.AnalyzeFile(goFile)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	
	if detectedLang == nil {
		t.Fatal("Expected language to be detected")
	}
	
	if detectedLang.Name != "Go" {
		t.Errorf("Expected Go, got %s", detectedLang.Name)
	}
	
	if detectedLang.LineCount != 7 {
		t.Errorf("Expected 7 lines, got %d", detectedLang.LineCount)
	}
}

func TestBuildSystemDetection(t *testing.T) {
	detector := NewBuildSystemDetector()
	
	// Test supported systems
	systems := detector.GetSupportedSystems()
	expectedSystems := []string{"go", "npm", "cargo", "pip", "poetry"}
	
	for _, expected := range expectedSystems {
		found := false
		for _, system := range systems {
			if system == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected build system %s to be supported", expected)
		}
	}
	
	// Test Go module parsing
	tempDir, err := os.MkdirTemp("", "build-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	goModFile := filepath.Join(tempDir, "go.mod")
	createTestFile(t, tempDir, "go.mod", `module github.com/example/project

go 1.19

require (
	github.com/stretchr/testify v1.8.0
	golang.org/x/sync v0.1.0
)`)
	
	buildSystem, err := detector.AnalyzeBuildFile(goModFile)
	if err != nil {
		t.Fatalf("AnalyzeBuildFile failed: %v", err)
	}
	
	if buildSystem.Type != "go" {
		t.Errorf("Expected type go, got %s", buildSystem.Type)
	}
	
	if buildSystem.Name != "project" {
		t.Errorf("Expected name project, got %s", buildSystem.Name)
	}
	
	if len(buildSystem.Dependencies) == 0 {
		t.Error("Expected dependencies to be found")
	}
}

func TestGitDetection(t *testing.T) {
	detector := NewGitDetector()
	
	// Create temporary directory with Git structure
	tempDir, err := os.MkdirTemp("", "git-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create .git directory
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatal(err)
	}
	
	// Create .gitignore
	createTestFile(t, tempDir, ".gitignore", `*.log
*.tmp
.env
node_modules/
dist/`)
	
	// Test Git detection
	gitInfo, err := detector.DetectGitInfo(tempDir)
	if err != nil {
		t.Fatalf("DetectGitInfo failed: %v", err)
	}
	
	if !gitInfo.IsRepo {
		t.Error("Expected IsRepo to be true")
	}
	
	if gitInfo.RootPath != tempDir {
		t.Errorf("Expected RootPath %s, got %s", tempDir, gitInfo.RootPath)
	}
	
	if len(gitInfo.IgnorePatterns) == 0 {
		t.Error("Expected ignore patterns to be found")
	}
	
	// Test ignore pattern matching
	testCases := []struct {
		filePath string
		ignored  bool
	}{
		{"test.log", true},
		{"temp.tmp", true},
		{"main.go", false},
		{"node_modules/package.json", true},
		{"dist/bundle.js", true},
	}
	
	for _, tc := range testCases {
		ignored := detector.IsIgnored(tc.filePath, gitInfo.IgnorePatterns)
		if ignored != tc.ignored {
			t.Errorf("File %s: expected ignored=%v, got %v", tc.filePath, tc.ignored, ignored)
		}
	}
}

func TestCacheManager(t *testing.T) {
	// Use a temporary cache directory for testing
	tempCacheDir, err := os.MkdirTemp("", "cache-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempCacheDir)
	
	cacheManager := NewCacheManagerWithOptions(tempCacheDir, time.Hour)
	
	// Create test context
	ctx := &ProjectContext{
		WorkspaceRoot:   "/test/workspace",
		ProjectName:     "test-project",
		PrimaryLanguage: "Go",
		DetectedAt:      time.Now(),
		CacheVersion:    "1.0",
	}
	
	// Test saving context
	err = cacheManager.SaveContext(ctx)
	if err != nil {
		t.Fatalf("SaveContext failed: %v", err)
	}
	
	// Test loading context
	loadedCtx, err := cacheManager.LoadContext("/test/workspace")
	if err != nil {
		t.Fatalf("LoadContext failed: %v", err)
	}
	
	if loadedCtx.ProjectName != ctx.ProjectName {
		t.Errorf("Expected ProjectName %s, got %s", ctx.ProjectName, loadedCtx.ProjectName)
	}
	
	// Test cache validity
	if !cacheManager.IsValidCache(loadedCtx) {
		t.Error("Cache should be valid for recently saved context")
	}
	
	// Test cache info
	info, err := cacheManager.GetCacheInfo()
	if err != nil {
		t.Fatalf("GetCacheInfo failed: %v", err)
	}
	
	if info.TotalFiles != 1 {
		t.Errorf("Expected 1 cache file, got %d", info.TotalFiles)
	}
	
	// Test cache invalidation
	err = cacheManager.InvalidateCache("/test/workspace")
	if err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}
	
	// Verify cache was invalidated
	_, err = cacheManager.LoadContext("/test/workspace")
	if err == nil {
		t.Error("Expected error when loading invalidated cache")
	}
}

func TestContextError(t *testing.T) {
	baseErr := &ContextError{
		Type:    "test_error",
		Message: "test message",
		Path:    "/test/path",
	}
	
	if baseErr.Error() != "test message" {
		t.Errorf("Expected error message 'test message', got '%s'", baseErr.Error())
	}
	
	wrappedErr := &ContextError{
		Type:    "wrapped_error",
		Message: "wrapped message",
		Cause:   baseErr,
	}
	
	expected := "wrapped message: test message"
	if wrappedErr.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, wrappedErr.Error())
	}
	
	if wrappedErr.Unwrap() != baseErr {
		t.Error("Unwrap() should return the original error")
	}
}

// Helper function to create test files
func createTestFile(t *testing.T, dir, filename, content string) {
	filePath := filepath.Join(dir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file %s: %v", filePath, err)
	}
}

// Benchmark tests
func BenchmarkDetectContext(b *testing.B) {
	// Use current directory for benchmarking
	wd, _ := os.Getwd()
	detector := NewDefaultContextDetector()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.DetectContext(wd)
	}
}

func BenchmarkLanguageDetection(b *testing.B) {
	detector := NewLanguageDetector()
	
	// Create a temporary Go file for testing
	tempDir, _ := os.MkdirTemp("", "bench-test")
	defer os.RemoveAll(tempDir)
	
	goFile := filepath.Join(tempDir, "test.go")
	os.WriteFile(goFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.AnalyzeFile(goFile)
	}
}

func BenchmarkCacheOperations(b *testing.B) {
	tempCacheDir, _ := os.MkdirTemp("", "cache-bench")
	defer os.RemoveAll(tempCacheDir)
	
	cacheManager := NewCacheManagerWithOptions(tempCacheDir, time.Hour)
	
	ctx := &ProjectContext{
		WorkspaceRoot:   "/bench/workspace",
		ProjectName:     "bench-project",
		DetectedAt:      time.Now(),
		CacheVersion:    "1.0",
	}
	
	b.Run("Save", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx.WorkspaceRoot = fmt.Sprintf("/bench/workspace-%d", i)
			_ = cacheManager.SaveContext(ctx)
		}
	})
	
	b.Run("Load", func(b *testing.B) {
		// Save once for loading
		_ = cacheManager.SaveContext(ctx)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = cacheManager.LoadContext(ctx.WorkspaceRoot)
		}
	})
}

