package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestToolExecutionWorkflow tests the complete workflow of tool execution
func TestToolExecutionWorkflow(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "tool_integration_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Initialize router
	router := NewToolRouter()
	
	// Create a security policy that allows the temp directory
	policy := &FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        1024 * 1024, // 1MB
	}
	
	// Register file operation executors
	router.RegisterExecutor("read_file", NewReadFileExecutor(policy))
	router.RegisterExecutor("write_file", NewWriteFileExecutor(policy))
	router.RegisterExecutor("edit_file", NewEditFileExecutor())
	
	ctx := context.Background()
	
	// Test 1: Write a file
	testFilePath := filepath.Join(tempDir, "integration_test.txt")
	initialContent := "This is a test file.\nLine 2 of content.\nLine 3 for testing."
	
	writeCall := &ToolCall{
		ID:       "write-001",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": testFilePath,
			"content":   initialContent,
		},
		Timestamp: time.Now(),
	}
	
	writeResult, err := router.Execute(ctx, writeCall)
	if err != nil {
		t.Fatalf("Write operation failed: %v", err)
	}
	
	if !writeResult.Success {
		t.Errorf("Write operation should succeed: %s", writeResult.ErrorMessage)
	}
	
	// Test 2: Read the file back
	readCall := &ToolCall{
		ID:       "read-001",
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			"path": testFilePath,
		},
		Timestamp: time.Now(),
	}
	
	readResult, err := router.Execute(ctx, readCall)
	if err != nil {
		t.Fatalf("Read operation failed: %v", err)
	}
	
	if !readResult.Success {
		t.Errorf("Read operation should succeed: %s", readResult.ErrorMessage)
	}
	
	// Verify the content matches
	if readResult.Result.(string) != initialContent {
		t.Errorf("Read content doesn't match written content.\nExpected: %q\nGot: %q", 
			initialContent, readResult.Result.(string))
	}
}

// TestBashToolIntegration tests bash tool execution workflow
func TestBashToolIntegration(t *testing.T) {
	// Skip on systems where basic commands might not be available
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping bash integration test in CI environment")
	}
	
	router := NewToolRouter()
	router.RegisterExecutor("bash", NewBashExecutor())
	
	ctx := context.Background()
	
	// Test simple echo command
	echoCall := &ToolCall{
		ID:       "bash-001",
		ToolName: "bash",
		Parameters: map[string]interface{}{
			"command": "echo 'Hello, Integration Test!'",
		},
		Timestamp: time.Now(),
	}
	
	result, err := router.Execute(ctx, echoCall)
	if err != nil {
		t.Fatalf("Bash execution failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Bash command should succeed: %s", result.ErrorMessage)
	}
	
	expectedOutput := "Hello, Integration Test!\n"
	if result.Result.(string) != expectedOutput {
		t.Errorf("Bash output mismatch.\nExpected: %q\nGot: %q", 
			expectedOutput, result.Result.(string))
	}
}

// TestToolExecutionMetrics tests tool execution time tracking
func TestToolExecutionMetrics(t *testing.T) {
	router := NewToolRouter()
	router.RegisterExecutor("echo", NewEchoExecutor())
	router.RegisterExecutor("sleep", NewSleepExecutor())
	
	ctx := context.Background()
	
	// Test execution time tracking
	sleepCall := &ToolCall{
		ID:       "sleep-001",
		ToolName: "sleep",
		Parameters: map[string]interface{}{
			"duration": "100ms",
		},
		Timestamp: time.Now(),
	}
	
	start := time.Now()
	result, err := router.Execute(ctx, sleepCall)
	elapsed := time.Since(start)
	
	if err != nil {
		t.Fatalf("Sleep execution failed: %v", err)
	}
	
	if !result.Success {
		t.Errorf("Sleep should succeed: %s", result.ErrorMessage)
	}
	
	// Verify the execution took approximately the right amount of time
	if elapsed < 90*time.Millisecond {
		t.Errorf("Sleep execution too fast: %v", elapsed)
	}
	
	if elapsed > 200*time.Millisecond {
		t.Errorf("Sleep execution too slow: %v", elapsed)
	}
	
	// Check that the result has duration information
	if result.Duration <= 0 {
		t.Error("Result should have positive duration")
	}
}

// TestConcurrentToolExecution tests parallel tool execution
func TestConcurrentToolExecution(t *testing.T) {
	router := NewToolRouter()
	router.RegisterExecutor("echo", NewEchoExecutor())
	
	ctx := context.Background()
	
	// Execute multiple tools concurrently
	numJobs := 10
	results := make(chan *ToolResult, numJobs)
	errors := make(chan error, numJobs)
	
	for i := 0; i < numJobs; i++ {
		go func(id int) {
			call := &ToolCall{
				ID:       fmt.Sprintf("concurrent-%d", id),
				ToolName: "echo",
				Parameters: map[string]interface{}{
					"message": fmt.Sprintf("Message %d", id),
				},
				Timestamp: time.Now(),
			}
			
			result, err := router.Execute(ctx, call)
			if err != nil {
				errors <- err
				return
			}
			
			results <- result
		}(i)
	}
	
	// Collect results
	successCount := 0
	for i := 0; i < numJobs; i++ {
		select {
		case result := <-results:
			if result.Success {
				successCount++
			}
		case err := <-errors:
			t.Errorf("Concurrent execution error: %v", err)
		case <-time.After(5 * time.Second):
			t.Errorf("Timeout waiting for concurrent execution %d", i)
		}
	}
	
	if successCount != numJobs {
		t.Errorf("Expected %d successful executions, got %d", numJobs, successCount)
	}
}

// TestToolExecutionWithTimeout tests context timeout handling
func TestToolExecutionWithTimeout(t *testing.T) {
	router := NewToolRouter()
	router.RegisterExecutor("sleep", NewSleepExecutor())
	
	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	// Try to execute a long-running operation
	longCall := &ToolCall{
		ID:       "timeout-001",
		ToolName: "sleep",
		Parameters: map[string]interface{}{
			"duration": "200ms", // Longer than timeout
		},
		Timestamp: time.Now(),
	}
	
	start := time.Now()
	result, err := router.Execute(ctx, longCall)
	elapsed := time.Since(start)
	
	// Should either get a context timeout error or a failed result
	if err == nil && result.Success {
		t.Error("Expected timeout or failure for long-running operation")
	}
	
	// Should complete quickly due to timeout
	if elapsed > 100*time.Millisecond {
		t.Errorf("Timeout handling took too long: %v", elapsed)
	}
}

// TestToolExecutionErrorHandling tests error propagation
func TestToolExecutionErrorHandling(t *testing.T) {
	router := NewToolRouter()
	router.RegisterExecutor("error", NewErrorExecutor())
	
	ctx := context.Background()
	
	// Test tool that always fails
	errorCall := &ToolCall{
		ID:       "error-001",
		ToolName: "error",
		Parameters: map[string]interface{}{
			"message": "Intentional test error",
		},
		Timestamp: time.Now(),
	}
	
	result, err := router.Execute(ctx, errorCall)
	
	// Should get a result (not an execution error)
	if err != nil {
		t.Errorf("Execution should not error, got: %v", err)
	}
	
	// But the result should indicate failure
	if result.Success {
		t.Error("Error tool should not succeed")
	}
	
	if result.ErrorMessage == "" {
		t.Error("Failed tool should have error message")
	}
	
	expectedError := "Intentional test error"
	if result.ErrorMessage != expectedError {
		t.Errorf("Error message mismatch.\nExpected: %q\nGot: %q", 
			expectedError, result.ErrorMessage)
	}
}

// TestToolSchema tests tool schema functionality
func TestToolSchema(t *testing.T) {
	router := NewToolRouter()
	router.RegisterExecutor("echo", NewEchoExecutor())
	
	// Test getting schema
	schema, err := router.GetSchema("echo")
	if err != nil {
		t.Fatalf("Failed to get echo schema: %v", err)
	}
	
	if schema == nil {
		t.Error("Schema should not be nil")
	}
	
	// Test invalid tool schema
	_, err = router.GetSchema("nonexistent")
	if err == nil {
		t.Error("Should get error for nonexistent tool schema")
	}
}

// TestInvalidToolParameters tests parameter validation
func TestInvalidToolParameters(t *testing.T) {
	router := NewToolRouter()
	router.RegisterExecutor("read_file", NewReadFileExecutor(nil))
	
	ctx := context.Background()
	
	// Test missing required parameter
	invalidCall := &ToolCall{
		ID:       "invalid-001",
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			// Missing "path" parameter
		},
		Timestamp: time.Now(),
	}
	
	result, err := router.Execute(ctx, invalidCall)
	
	// Should handle gracefully (result with error, not execution error)
	if err != nil {
		t.Errorf("Should handle invalid parameters gracefully, got: %v", err)
	}
	
	if result.Success {
		t.Error("Invalid parameters should result in failed execution")
	}
}

// TestUnknownTool tests handling of unknown tools
func TestUnknownTool(t *testing.T) {
	router := NewToolRouter()
	
	ctx := context.Background()
	
	unknownCall := &ToolCall{
		ID:       "unknown-001",
		ToolName: "nonexistent_tool",
		Parameters: map[string]interface{}{
			"param": "value",
		},
		Timestamp: time.Now(),
	}
	
	_, err := router.Execute(ctx, unknownCall)
	
	// Should get an error for unknown tool
	if err == nil {
		t.Error("Should get error for unknown tool")
	}
}

// TestBatchToolExecution tests executing multiple tools in sequence
func TestBatchToolExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "batch_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	router := NewToolRouter()
	router.RegisterExecutor("write_file", NewWriteFileExecutor(nil))
	router.RegisterExecutor("read_file", NewReadFileExecutor(nil))
	router.RegisterExecutor("echo", NewEchoExecutor())
	
	ctx := context.Background()
	
	// Create multiple tool calls
	calls := []*ToolCall{
		{
			ID:       "batch-001",
			ToolName: "write_file",
			Parameters: map[string]interface{}{
				"path":    filepath.Join(tempDir, "batch1.txt"),
				"content": "Batch file 1",
			},
			Timestamp: time.Now(),
		},
		{
			ID:       "batch-002",
			ToolName: "write_file",
			Parameters: map[string]interface{}{
				"path":    filepath.Join(tempDir, "batch2.txt"),
				"content": "Batch file 2",
			},
			Timestamp: time.Now(),
		},
		{
			ID:       "batch-003",
			ToolName: "echo",
			Parameters: map[string]interface{}{
				"message": "Batch complete",
			},
			Timestamp: time.Now(),
		},
	}
	
	// Execute all calls
	results := make([]*ToolResult, len(calls))
	for i, call := range calls {
		result, err := router.Execute(ctx, call)
		if err != nil {
			t.Errorf("Batch execution %d failed: %v", i, err)
			continue
		}
		results[i] = result
	}
	
	// Verify all succeeded
	for i, result := range results {
		if result == nil {
			t.Errorf("Result %d is nil", i)
			continue
		}
		
		if !result.Success {
			t.Errorf("Batch execution %d failed: %s", i, result.ErrorMessage)
		}
	}
	
	// Verify files were created
	for i := 1; i <= 2; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("batch%d.txt", i))
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Batch file %d should exist", i)
		}
	}
}