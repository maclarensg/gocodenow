package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileSecurityPolicy_CheckPath(t *testing.T) {
	policy := NewDefaultFileSecurityPolicy()
	
	// Test allowed paths
	testCases := []struct {
		name        string
		path        string
		expectError bool
		description string
	}{
		{"relative path", "./test.txt", false, "relative paths should be allowed"},
		{"current dir", "test.txt", false, "current directory should be allowed"},
		{"absolute blocked", "/etc/passwd", true, "blocked paths should be rejected"},
		{"blocked extension", "malware.exe", true, "blocked extensions should be rejected"},
		{"allowed extension", "config.json", false, "allowed extensions should pass"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.CheckPath(tc.path)
			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.path)
			}
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.path, err)
			}
		})
	}
}

func TestReadFileExecutor_Basic(t *testing.T) {
	// Create test file in current directory
	testFile := "test_read_basic.txt"
	testContent := "Hello, World!\nThis is a test file.\nLine 3"
	
	// Clean up after test
	defer os.Remove(testFile)
	
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	policy := NewDefaultFileSecurityPolicy()
	reader := NewReadFileExecutor(policy)
	ctx := context.Background()
	
	// Test basic file reading
	params := ToolParameters{
		"path": testFile,
	}
	
	result, err := reader.Execute(ctx, params)
	if err != nil {
		t.Errorf("Read execution failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	if !strings.Contains(result.Result.(string), "Hello, World!") {
		t.Errorf("Expected file content in result, got: %v", result.Result)
	}
	
	// Check metadata
	metadata := result.Metadata
	if metadata["file_path"] != testFile {
		t.Errorf("Expected file_path in metadata, got %v", metadata["file_path"])
	}
}

func TestReadFileExecutor_LineFilter(t *testing.T) {
	// Create test file in current directory
	testFile := "test_read_filter.txt"
	testContent := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5"
	
	// Clean up after test
	defer os.Remove(testFile)
	
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	policy := NewDefaultFileSecurityPolicy()
	reader := NewReadFileExecutor(policy)
	ctx := context.Background()
	
	// Test line filtering
	params := ToolParameters{
		"path":       testFile,
		"start_line": 2,
		"max_lines":  3,
	}
	
	result, err := reader.Execute(ctx, params)
	if err != nil {
		t.Errorf("Read execution failed: %v", err)
	}
	
	resultStr := result.Result.(string)
	if !strings.Contains(resultStr, "Line 2") || !strings.Contains(resultStr, "Line 3") || !strings.Contains(resultStr, "Line 4") {
		t.Errorf("Expected lines 2-4 in result, got: %s", resultStr)
	}
	
	if strings.Contains(resultStr, "Line 1") || strings.Contains(resultStr, "Line 5") {
		t.Errorf("Unexpected lines in filtered result: %s", resultStr)
	}
}

func TestReadFileExecutor_NonExistentFile(t *testing.T) {
	policy := NewDefaultFileSecurityPolicy()
	reader := NewReadFileExecutor(policy)
	ctx := context.Background()
	
	params := ToolParameters{
		"path": "nonexistent_file.txt",
	}
	
	_, err := reader.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	
	if execError, ok := err.(*ToolExecutionError); ok {
		if !strings.Contains(execError.Message, "does not exist") {
			t.Errorf("Expected 'does not exist' in error message, got: %s", execError.Message)
		}
	} else {
		t.Errorf("Expected ToolExecutionError, got %T", err)
	}
}

func TestWriteFileExecutor_Basic(t *testing.T) {
	testFile := "test_write_basic.txt"
	testContent := "This is test content"
	
	// Clean up after test
	defer os.Remove(testFile)
	
	policy := NewDefaultFileSecurityPolicy()
	writer := NewWriteFileExecutor(policy)
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path": testFile,
		"content":   testContent,
	}
	
	result, err := writer.Execute(ctx, params)
	if err != nil {
		t.Errorf("Write execution failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	// Verify file was created
	if _, err := os.Stat(testFile); err != nil {
		t.Errorf("File was not created: %v", err)
	}
	
	// Verify file content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Cannot read written file: %v", err)
	}
	
	if string(content) != testContent {
		t.Errorf("Expected %q, got %q", testContent, string(content))
	}
}

func TestWriteFileExecutor_Backup(t *testing.T) {
	testFile := "test_write_backup.txt"
	originalContent := "Original content"
	newContent := "New content"
	
	// Clean up after test
	defer os.Remove(testFile)
	defer func() {
		// Clean up any backup files that might have been created
		files, _ := filepath.Glob(testFile + ".bak.*")
		for _, f := range files {
			os.Remove(f)
		}
	}()
	
	// Create original file
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}
	
	policy := NewDefaultFileSecurityPolicy()
	writer := NewWriteFileExecutor(policy)
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path":     testFile,
		"content":       newContent,
		"create_backup": true,
	}
	
	result, err := writer.Execute(ctx, params)
	if err != nil {
		t.Errorf("Write execution failed: %v", err)
	}
	
	// Check if backup was created
	metadata := result.Metadata
	backupCreated, ok := metadata["backup_created"].(bool)
	if !ok || !backupCreated {
		t.Error("Expected backup to be created")
	}
	
	backupPath, ok := metadata["backup_path"].(string)
	if !ok {
		t.Error("Expected backup_path in metadata")
	} else {
		// Verify backup file exists and has original content
		backupContent, err := os.ReadFile(backupPath)
		if err != nil {
			t.Errorf("Cannot read backup file: %v", err)
		}
		if string(backupContent) != originalContent {
			t.Errorf("Expected backup to have original content %q, got %q", originalContent, string(backupContent))
		}
	}
	
	// Verify original file has new content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Cannot read updated file: %v", err)
	}
	if string(content) != newContent {
		t.Errorf("Expected file to have new content %q, got %q", newContent, string(content))
	}
}

func TestWriteFileExecutor_AppendMode(t *testing.T) {
	testFile := "test_write_append.txt"
	originalContent := "Original content"
	appendContent := "\nAppended content"
	
	// Clean up after test
	defer os.Remove(testFile)
	
	// Create original file
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}
	
	policy := NewDefaultFileSecurityPolicy()
	writer := NewWriteFileExecutor(policy)
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path": testFile,
		"content":   appendContent,
		"append":    true,
	}
	
	_, err := writer.Execute(ctx, params)
	if err != nil {
		t.Errorf("Write execution failed: %v", err)
	}
	
	// Verify file has both original and appended content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Cannot read file: %v", err)
	}
	
	expectedContent := originalContent + appendContent
	if string(content) != expectedContent {
		t.Errorf("Expected %q, got %q", expectedContent, string(content))
	}
}

func TestEditFileExecutor_Replace(t *testing.T) {
	testFile := "test_edit_replace.txt"
	originalContent := "Hello World\nThis is a test\nHello Universe"
	
	// Clean up after test
	defer os.Remove(testFile)
	defer func() {
		// Clean up any backup files that might have been created
		files, _ := filepath.Glob(testFile + ".bak.*")
		for _, f := range files {
			os.Remove(f)
		}
	}()
	
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	editor := NewEditFileExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path": testFile,
		"operation": "replace",
		"find":      "Hello",
		"replace":   "Hi",
	}
	
	result, err := editor.Execute(ctx, params)
	if err != nil {
		t.Errorf("Edit execution failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	// Verify file content was changed
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Cannot read edited file: %v", err)
	}
	
	contentStr := string(content)
	if !strings.Contains(contentStr, "Hi World") || !strings.Contains(contentStr, "Hi Universe") {
		t.Errorf("Expected replacements not found in: %s", contentStr)
	}
	
	if strings.Contains(contentStr, "Hello") {
		t.Errorf("Original text still found after replacement: %s", contentStr)
	}
}

func TestEditFileExecutor_PreviewMode(t *testing.T) {
	testFile := "test_edit_preview.txt"
	originalContent := "Hello World"
	
	// Clean up after test
	defer os.Remove(testFile)
	
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	editor := NewEditFileExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path": testFile,
		"operation": "replace",
		"find":      "Hello",
		"replace":   "Hi",
		"preview":   true,
	}
	
	result, err := editor.Execute(ctx, params)
	if err != nil {
		t.Errorf("Edit execution failed: %v", err)
	}
	
	// File should not be changed in preview mode
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Cannot read file: %v", err)
	}
	
	if string(content) != originalContent {
		t.Errorf("File was modified in preview mode. Expected %q, got %q", originalContent, string(content))
	}
	
	// Result should contain diff
	resultStr := result.Result.(string)
	if !strings.Contains(resultStr, "Preview of changes") {
		t.Errorf("Expected preview text in result: %s", resultStr)
	}
}

func TestEditFileExecutor_InsertLine(t *testing.T) {
	testFile := "test_edit_insert.txt"
	originalContent := "Line 1\nLine 2\nLine 3"
	
	// Clean up after test
	defer os.Remove(testFile)
	defer func() {
		// Clean up any backup files that might have been created
		files, _ := filepath.Glob(testFile + ".bak.*")
		for _, f := range files {
			os.Remove(f)
		}
	}()
	
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	editor := NewEditFileExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path":   testFile,
		"operation":   "insert_line",
		"line_number": 2,
		"text":        "Inserted Line",
	}
	
	_, err := editor.Execute(ctx, params)
	if err != nil {
		t.Errorf("Edit execution failed: %v", err)
	}
	
	// Verify line was inserted
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Cannot read file: %v", err)
	}
	
	lines := strings.Split(string(content), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines after insertion, got %d", len(lines))
	}
	
	if lines[1] != "Inserted Line" {
		t.Errorf("Expected 'Inserted Line' at line 2, got %q", lines[1])
	}
}

func TestEditFileExecutor_DeleteLine(t *testing.T) {
	testFile := "test_edit_delete.txt"
	originalContent := "Line 1\nLine 2\nLine 3"
	
	// Clean up after test
	defer os.Remove(testFile)
	defer func() {
		// Clean up any backup files that might have been created
		files, _ := filepath.Glob(testFile + ".bak.*")
		for _, f := range files {
			os.Remove(f)
		}
	}()
	
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	editor := NewEditFileExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path":   testFile,
		"operation":   "delete_line",
		"line_number": 2,
	}
	
	_, err := editor.Execute(ctx, params)
	if err != nil {
		t.Errorf("Edit execution failed: %v", err)
	}
	
	// Verify line was deleted
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Errorf("Cannot read file: %v", err)
	}
	
	lines := strings.Split(string(content), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines after deletion, got %d", len(lines))
	}
	
	if strings.Contains(string(content), "Line 2") {
		t.Errorf("Line 2 should have been deleted: %s", string(content))
	}
}

func TestListDirectoryExecutor_Basic(t *testing.T) {
	lister := NewListDirectoryExecutor()
	ctx := context.Background()
	
	// List current directory
	params := ToolParameters{
		"path": ".",
	}
	
	result, err := lister.Execute(ctx, params)
	if err != nil {
		t.Errorf("List execution failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	resultStr := result.Result.(string)
	
	// Basic check that we got some content (listing should not be empty)
	if len(resultStr) == 0 {
		t.Error("Expected non-empty directory listing")
	}
	
	// Should contain file_ops.go and file_ops_test.go which we know exist
	if !strings.Contains(resultStr, "file_ops.go") {
		t.Errorf("Expected file_ops.go in listing: %s", resultStr)
	}
}

func TestListDirectoryExecutor_Recursive(t *testing.T) {
	lister := NewListDirectoryExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"path":      ".",
		"recursive": true,
	}
	
	result, err := lister.Execute(ctx, params)
	if err != nil {
		t.Errorf("List execution failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	resultStr := result.Result.(string)
	
	// Basic check that we got some content (listing should not be empty)
	if len(resultStr) == 0 {
		t.Error("Expected non-empty recursive directory listing")
	}
	
	// Should contain file_ops.go which we know exists
	if !strings.Contains(resultStr, "file_ops.go") {
		t.Errorf("Expected file_ops.go in recursive listing: %s", resultStr)
	}
}

func TestFilePermissionsExecutor_Get(t *testing.T) {
	// Change to temp directory to work with relative paths
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	testFile := "perm_test.txt"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	permExec := NewFilePermissionsExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path": testFile,
		"operation": "get",
	}
	
	result, err := permExec.Execute(ctx, params)
	if err != nil {
		t.Errorf("Permission get failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	resultStr := result.Result.(string)
	if !strings.Contains(resultStr, "644") {
		t.Errorf("Expected permission 644 in result: %s", resultStr)
	}
}

func TestFilePermissionsExecutor_Set(t *testing.T) {
	// Change to temp directory to work with relative paths
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	testFile := "perm_set_test.txt"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	permExec := NewFilePermissionsExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path":   testFile,
		"operation":   "set",
		"permissions": "644",
	}
	
	result, err := permExec.Execute(ctx, params)
	if err != nil {
		t.Errorf("Permission set failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	// Verify permissions were changed
	info, err := os.Stat(testFile)
	if err != nil {
		t.Errorf("Cannot stat file: %v", err)
	}
	
	expectedMode := os.FileMode(0644)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Expected permissions %o, got %o", expectedMode, info.Mode().Perm())
	}
}

func TestFilePermissionsExecutor_DangerousPermissions(t *testing.T) {
	// Change to temp directory to work with relative paths
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	testFile := "danger_test.txt"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	permExec := NewFilePermissionsExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path":   testFile,
		"operation":   "set",
		"permissions": "777", // World writable - dangerous
		"force":       false,
	}
	
	_, err = permExec.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for dangerous permissions without force flag")
	}
	
	if execError, ok := err.(*ToolExecutionError); ok {
		if !strings.Contains(execError.Message, "dangerous") {
			t.Errorf("Expected 'dangerous' in error message, got: %s", execError.Message)
		}
	} else {
		t.Errorf("Expected ToolExecutionError, got %T", err)
	}
}

func TestFilePermissionsExecutor_SymbolicPermissions(t *testing.T) {
	// Change to temp directory to work with relative paths
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	testFile := "symbolic_test.txt"
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	permExec := NewFilePermissionsExecutor()
	ctx := context.Background()
	
	params := ToolParameters{
		"file_path":   testFile,
		"operation":   "set",
		"permissions": "rwxr-xr-x", // 755 in symbolic
		"force":       true, // Override safety check
	}
	
	result, err := permExec.Execute(ctx, params)
	if err != nil {
		t.Errorf("Permission set with symbolic failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}
	
	// Verify permissions were set correctly
	info, err := os.Stat(testFile)
	if err != nil {
		t.Errorf("Cannot stat file: %v", err)
	}
	
	expectedMode := os.FileMode(0755)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Expected permissions %o, got %o", expectedMode, info.Mode().Perm())
	}
}

func TestFileSecurityPolicy_CheckSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a regular file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Create a symlink
	symlinkFile := filepath.Join(tmpDir, "test_link.txt")
	if err := os.Symlink(testFile, symlinkFile); err != nil {
		t.Skipf("Cannot create symlink (may not be supported): %v", err)
	}
	
	policy := NewDefaultFileSecurityPolicy()
	
	// Regular file should pass
	if err := policy.CheckSymlink(testFile); err != nil {
		t.Errorf("Regular file should pass symlink check: %v", err)
	}
	
	// Symlink should be blocked by default policy
	if err := policy.CheckSymlink(symlinkFile); err == nil {
		t.Error("Symlink should be blocked by default policy")
	}
	
	// Allow symlinks and test again
	policy.AllowSymlinks = true
	if err := policy.CheckSymlink(symlinkFile); err != nil {
		t.Errorf("Symlink should be allowed when policy permits: %v", err)
	}
}

// Test context cancellation
func TestFileOperations_ContextCancellation(t *testing.T) {
	// Change to temp directory to work with relative paths
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Test with EditFileExecutor as it has good cancellation support
	testFile := "cancel_test.txt"
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	editor := NewEditFileExecutor()
	
	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	
	// Give some time for timeout
	time.Sleep(2 * time.Millisecond)
	
	params := ToolParameters{
		"file_path": testFile,
		"operation": "replace",
		"find":      "test",
		"replace":   "cancelled",
	}
	
	_, err = editor.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}
	
	if execError, ok := err.(*ToolExecutionError); ok {
		if !strings.Contains(execError.Message, "cancelled") {
			t.Errorf("Expected 'cancelled' in error message, got: %s", execError.Message)
		}
	} else {
		t.Errorf("Expected ToolExecutionError, got %T", err)
	}
}