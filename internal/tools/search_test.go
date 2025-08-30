package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchFilesExecutor(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "search-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test files
	testFiles := map[string]string{
		"main.go":           "package main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}",
		"utils.go":          "package main\n\nfunc helper() {}\n",
		"test_file.txt":     "This is a test file",
		"config.json":       `{"name": "test", "version": "1.0"}`,
		"src/component.js":  "function Component() { return 'test'; }",
		"src/style.css":     "body { color: red; }",
		".hidden":           "hidden content",
		"docs/README.md":    "# Test Project",
	}
	
	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		dir := filepath.Dir(fullPath)
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	
	executor := NewSearchFilesExecutor()
	
	// Test basic file search
	t.Run("BasicSearch", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern": "*.go",
			"path":    tempDir,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		// Check that Go files were found
		resultData := result.Result.(map[string]interface{})
		files := resultData["files"].([]FileMatch)
		
		foundGo := 0
		for _, file := range files {
			if filepath.Ext(file.Name) == ".go" {
				foundGo++
			}
		}
		
		if foundGo != 2 {
			t.Errorf("Expected 2 Go files, found %d", foundGo)
		}
	})
	
	// Test regex search
	t.Run("RegexSearch", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":        ".*\\.js$",
			"path":           tempDir,
			"regex":          true,
			"case_sensitive": false,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		files := resultData["files"].([]FileMatch)
		
		if len(files) != 1 {
			t.Errorf("Expected 1 JS file, found %d", len(files))
		}
	})
	
	// Test file type filtering
	t.Run("FileTypeFilter", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":    "*",
			"path":       tempDir,
			"file_types": []interface{}{"json", "md"},
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		files := resultData["files"].([]FileMatch)
		
		// Should find config.json and README.md
		if len(files) != 2 {
			t.Errorf("Expected 2 files (json + md), found %d", len(files))
		}
	})
	
	// Test hidden files
	t.Run("HiddenFiles", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":       "*hidden*",
			"path":          tempDir,
			"include_hidden": true,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		files := resultData["files"].([]FileMatch)
		
		if len(files) == 0 {
			t.Error("Expected to find hidden file")
		}
	})
	
	// Test max results limit
	t.Run("MaxResults", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":     "*",
			"path":        tempDir,
			"max_results": 3,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		files := resultData["files"].([]FileMatch)
		
		if len(files) > 3 {
			t.Errorf("Expected max 3 files, found %d", len(files))
		}
	})
}

func TestFindInFilesExecutor(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "findinfiles-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test files with content
	testFiles := map[string]string{
		"main.go":     "package main\n\nfunc main() {\n\tfmt.Println(\"Hello World\")\n\tfmt.Printf(\"Testing\")\n}",
		"utils.go":    "package main\n\nfunc helper() {\n\treturn \"Hello\"\n}",
		"test.txt":    "Hello there!\nThis is a test file\nHello again!",
		"config.json": `{"greeting": "Hello", "name": "test"}`,
	}
	
	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	
	executor := NewFindInFilesExecutor()
	
	// Test basic content search
	t.Run("BasicContentSearch", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern": "Hello",
			"path":    tempDir,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		matches := resultData["matches"].([]ContentMatch)
		
		// Should find matches in multiple files
		if len(matches) < 2 {
			t.Errorf("Expected matches in at least 2 files, found %d", len(matches))
		}
		
		// Check total match count
		totalMatches := resultData["total_matches"].(int)
		if totalMatches < 3 {
			t.Errorf("Expected at least 3 total matches, found %d", totalMatches)
		}
	})
	
	// Test case sensitive search
	t.Run("CaseSensitiveSearch", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":        "hello", // lowercase
			"path":           tempDir,
			"case_sensitive": true,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		totalMatches := resultData["total_matches"].(int)
		
		// Should find fewer matches with case sensitivity
		if totalMatches > 1 {
			t.Errorf("Expected 1 or fewer matches with case sensitivity, found %d", totalMatches)
		}
	})
	
	// Test regex search
	t.Run("RegexSearch", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern": `fmt\.\w+`,
			"path":    tempDir,
			"regex":   true,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		totalMatches := resultData["total_matches"].(int)
		
		// Should find fmt.Println and fmt.Printf
		if totalMatches < 2 {
			t.Errorf("Expected at least 2 regex matches, found %d", totalMatches)
		}
	})
	
	// Test file type filtering
	t.Run("FileTypeFilter", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":    "test",
			"path":       tempDir,
			"file_types": []interface{}{"go"},
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		matches := resultData["matches"].([]ContentMatch)
		
		// Should only find matches in Go files
		for _, match := range matches {
			if filepath.Ext(match.File) != ".go" {
				t.Errorf("Found match in non-Go file: %s", match.File)
			}
		}
	})
	
	// Test context lines
	t.Run("ContextLines", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern": "Hello",
			"path":    tempDir,
			"context": 1,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		matches := resultData["matches"].([]ContentMatch)
		
		// Check that context lines are included
		foundContext := false
		for _, match := range matches {
			for _, lineMatch := range match.Matches {
				if len(lineMatch.ContextBefore) > 0 || len(lineMatch.ContextAfter) > 0 {
					foundContext = true
					break
				}
			}
			if foundContext {
				break
			}
		}
		
		if !foundContext {
			t.Error("Expected context lines to be included")
		}
	})
}

func TestGrepExecutor(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "grep-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test files
	testFiles := map[string]string{
		"sample.txt": "line 1: Hello World\nline 2: Testing grep\nline 3: Hello there\nline 4: Another line",
		"data.log":   "INFO: Starting application\nERROR: Something went wrong\nINFO: Processing complete",
	}
	
	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	
	executor := NewGrepExecutor()
	
	// Test basic grep
	t.Run("BasicGrep", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern": "Hello",
			"path":    tempDir,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		totalMatches := resultData["total_matches"].(int)
		
		if totalMatches != 2 {
			t.Errorf("Expected 2 matches for 'Hello', found %d", totalMatches)
		}
	})
	
	// Test invert match
	t.Run("InvertMatch", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":      "Hello",
			"path":         filepath.Join(tempDir, "sample.txt"),
			"invert_match": true,
			"recursive":    false,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		totalMatches := resultData["total_matches"].(int)
		
		// Should find lines that don't contain "Hello"
		if totalMatches != 2 {
			t.Errorf("Expected 2 inverted matches, found %d", totalMatches)
		}
	})
	
	// Test context lines
	t.Run("ContextLines", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":        "grep",
			"path":           filepath.Join(tempDir, "sample.txt"),
			"recursive":      false,
			"context_before": 1,
			"context_after":  1,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		results := resultData["results"].([]GrepResult)
		
		if len(results) != 1 {
			t.Fatalf("Expected 1 file result, got %d", len(results))
		}
		
		// Check context lines
		match := results[0].Matches[0]
		if len(match.ContextBefore) != 1 || len(match.ContextAfter) != 1 {
			t.Errorf("Expected 1 context line before and after, got %d before, %d after", 
				len(match.ContextBefore), len(match.ContextAfter))
		}
	})
	
	// Test line numbers
	t.Run("LineNumbers", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":      "ERROR",
			"path":         tempDir,
			"line_numbers": true,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		results := resultData["results"].([]GrepResult)
		
		if len(results) != 1 {
			t.Fatalf("Expected 1 file result, got %d", len(results))
		}
		
		match := results[0].Matches[0]
		if match.LineNumber != 2 {
			t.Errorf("Expected line number 2, got %d", match.LineNumber)
		}
	})
	
	// Test whole word matching
	t.Run("WholeWord", func(t *testing.T) {
		params := map[string]interface{}{
			"pattern":    "line",
			"path":       filepath.Join(tempDir, "sample.txt"),
			"recursive":  false,
			"whole_word": true,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		totalMatches := resultData["total_matches"].(int)
		
		// Should match "line" as whole word in 4 lines
		if totalMatches != 4 {
			t.Errorf("Expected 4 whole word matches for 'line', found %d", totalMatches)
		}
	})
}

func TestTreeExecutor(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "tree-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create test directory structure
	dirs := []string{
		"src",
		"src/components",
		"docs",
		"tests",
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tempDir, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	
	files := map[string]string{
		"main.go":              "package main",
		"README.md":            "# Test Project",
		"src/app.go":           "package main",
		"src/components/ui.go": "package components",
		"docs/guide.md":        "# Guide",
		"tests/main_test.go":   "package main",
		".gitignore":           "*.tmp\n*.log",
	}
	
	for filePath, content := range files {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	
	executor := NewTreeExecutor()
	
	// Test basic tree generation
	t.Run("BasicTree", func(t *testing.T) {
		params := map[string]interface{}{
			"path": tempDir,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		if !result.Success {
			t.Fatalf("Expected success, got error: %s", result.ErrorMessage)
		}
		
		resultData := result.Result.(map[string]interface{})
		visualTree := resultData["visual_tree"].(string)
		
		// Check that tree contains expected elements
		if !strings.Contains(visualTree, "src") {
			t.Error("Tree should contain 'src' directory")
		}
		
		if !strings.Contains(visualTree, "main.go") {
			t.Error("Tree should contain 'main.go' file")
		}
	})
	
	// Test depth limiting
	t.Run("MaxDepth", func(t *testing.T) {
		params := map[string]interface{}{
			"path":      tempDir,
			"max_depth": 1,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		stats := resultData["statistics"].(*TreeStatistics)
		maxDepthReached := stats.MaxDepthReached
		
		if maxDepthReached > 1 {
			t.Errorf("Expected max depth of 1, got %d", maxDepthReached)
		}
	})
	
	// Test file type filtering
	t.Run("FileTypeFilter", func(t *testing.T) {
		params := map[string]interface{}{
			"path":       tempDir,
			"file_types": []interface{}{"go"},
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		visualTree := resultData["visual_tree"].(string)
		
		// Should contain Go files
		if !strings.Contains(visualTree, "main.go") {
			t.Error("Tree should contain Go files")
		}
		
		// Should not contain non-Go files
		if strings.Contains(visualTree, "README.md") {
			t.Error("Tree should not contain non-Go files with filter applied")
		}
	})
	
	// Test hidden files
	t.Run("HiddenFiles", func(t *testing.T) {
		params := map[string]interface{}{
			"path":        tempDir,
			"show_hidden": true,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		visualTree := resultData["visual_tree"].(string)
		
		// Should contain .gitignore file
		if !strings.Contains(visualTree, ".gitignore") {
			t.Error("Tree should contain hidden files when show_hidden is true")
		}
	})
	
	// Test size information
	t.Run("ShowSizes", func(t *testing.T) {
		params := map[string]interface{}{
			"path":       tempDir,
			"show_sizes": true,
		}
		
		result, err := executor.Execute(params)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		
		resultData := result.Result.(map[string]interface{})
		visualTree := resultData["visual_tree"].(string)
		
		// Should contain size information (parentheses with size)
		if !strings.Contains(visualTree, "(") || !strings.Contains(visualTree, "B)") {
			t.Error("Tree should contain size information when show_sizes is true")
		}
	})
}

// Benchmark tests
func BenchmarkSearchFiles(b *testing.B) {
	// Use current directory for benchmarking
	executor := NewSearchFilesExecutor()
	params := map[string]interface{}{
		"pattern":     "*.go",
		"path":        ".",
		"max_results": 50,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Execute(params)
	}
}

func BenchmarkFindInFiles(b *testing.B) {
	executor := NewFindInFilesExecutor()
	params := map[string]interface{}{
		"pattern":     "func",
		"path":        ".",
		"max_results": 100,
		"file_types":  []interface{}{"go"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Execute(params)
	}
}

func BenchmarkGrep(b *testing.B) {
	executor := NewGrepExecutor()
	params := map[string]interface{}{
		"pattern":     "package",
		"path":        ".",
		"max_results": 100,
		"file_types":  []interface{}{"go"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Execute(params)
	}
}

func BenchmarkTree(b *testing.B) {
	executor := NewTreeExecutor()
	params := map[string]interface{}{
		"path":      ".",
		"max_depth": 5,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = executor.Execute(params)
	}
}