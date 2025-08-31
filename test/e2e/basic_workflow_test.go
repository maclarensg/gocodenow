package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gocodenow/internal/tools"
)

// TestBasicFileWorkflow tests a complete file operation workflow
func TestBasicFileWorkflow(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "gocodenow_basic_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup tool router with file operations
	router := tools.NewToolRouter()
	
	// Create security policy for testing
	policy := &tools.FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        1024 * 1024,
	}
	
	// Register file tools
	router.RegisterExecutor("write_file", tools.NewWriteFileExecutor(policy))
	router.RegisterExecutor("read_file", tools.NewReadFileExecutor(policy))
	router.RegisterExecutor("echo", tools.NewEchoExecutor())

	ctx := context.Background()

	// Scenario: User wants to create a Python script
	t.Log("=== Scenario: Creating and verifying a Python script ===")

	// Step 1: Write Python file
	pythonContent := `#!/usr/bin/env python3
"""
Simple calculator program
"""

def add(a, b):
    return a + b

def main():
    result = add(5, 3)
    print(f"5 + 3 = {result}")

if __name__ == "__main__":
    main()
`

	writeCall := &tools.ToolCall{
		ID:       "write-python-001",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": filepath.Join(tempDir, "calculator.py"),
			"content":   pythonContent,
		},
		Timestamp: time.Now(),
	}

	t.Log("Executing write_file tool...")
	writeResult, err := router.Execute(ctx, writeCall)
	if err != nil {
		t.Fatalf("Write tool execution failed: %v", err)
	}

	if !writeResult.Success {
		t.Fatalf("Write tool should succeed: %s", writeResult.ErrorMessage)
	}

	t.Log("✓ File write completed successfully")

	// Step 2: Verify file exists and has correct content
	readCall := &tools.ToolCall{
		ID:       "read-python-001", 
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			"path": filepath.Join(tempDir, "calculator.py"),
		},
		Timestamp: time.Now(),
	}

	t.Log("Executing read_file tool...")
	readResult, err := router.Execute(ctx, readCall)
	if err != nil {
		t.Fatalf("Read tool execution failed: %v", err)
	}

	if !readResult.Success {
		t.Fatalf("Read tool should succeed: %s", readResult.ErrorMessage)
	}

	// Verify content matches
	if readResult.Result.(string) != pythonContent {
		t.Error("File content verification failed")
		t.Logf("Expected length: %d", len(pythonContent))
		t.Logf("Actual length: %d", len(readResult.Result.(string)))
	}

	t.Log("✓ File content verification successful")

	// Step 3: Test echo tool for simple interaction
	echoCall := &tools.ToolCall{
		ID:       "echo-001",
		ToolName: "echo",
		Parameters: map[string]interface{}{
			"message": "Python calculator created successfully!",
		},
		Timestamp: time.Now(),
	}

	t.Log("Executing echo tool...")
	echoResult, err := router.Execute(ctx, echoCall)
	if err != nil {
		t.Fatalf("Echo tool execution failed: %v", err)
	}

	if !echoResult.Success {
		t.Fatalf("Echo tool should succeed: %s", echoResult.ErrorMessage)
	}

	expectedEcho := "Python calculator created successfully!"
	if echoResult.Result.(string) != expectedEcho {
		t.Errorf("Echo result mismatch. Expected: %q, Got: %q", 
			expectedEcho, echoResult.Result.(string))
	}

	t.Log("✓ Echo tool verification successful")

	// Verify execution times are reasonable
	if writeResult.Duration <= 0 {
		t.Error("Write operation should have positive duration")
	}

	if readResult.Duration <= 0 {
		t.Error("Read operation should have positive duration")
	}

	t.Log("=== End-to-end workflow completed successfully ===")
}

// TestErrorScenarios tests how tools handle error conditions
func TestErrorScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocodenow_error_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	router := tools.NewToolRouter()
	policy := &tools.FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        100, // Very small limit to test size errors
	}
	
	router.RegisterExecutor("read_file", tools.NewReadFileExecutor(policy))
	router.RegisterExecutor("write_file", tools.NewWriteFileExecutor(policy))

	ctx := context.Background()

	t.Log("=== Scenario: Testing error handling ===")

	// Test 1: Read non-existent file
	t.Log("Testing read of non-existent file...")
	
	readCall := &tools.ToolCall{
		ID:       "read-missing-001",
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			"path": filepath.Join(tempDir, "missing.txt"),
		},
		Timestamp: time.Now(),
	}

	result, err := router.Execute(ctx, readCall)
	// File read tools may return execution errors for missing files
	if err != nil {
		t.Log("✓ Missing file error handled correctly via execution error")
		return
	}

	// Or they may return a failed result  
	if result.Success {
		t.Error("Reading missing file should fail")
	}

	if result.ErrorMessage == "" {
		t.Error("Failed operation should have error message")
	}

	t.Log("✓ Missing file error handled correctly")

	// Test 2: Write file that exceeds size limit
	t.Log("Testing file size limit...")
	
	largeContent := make([]byte, 200) // Exceeds our 100 byte limit
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	writeCall := &tools.ToolCall{
		ID:       "write-large-001",
		ToolName: "write_file", 
		Parameters: map[string]interface{}{
			"file_path": filepath.Join(tempDir, "large.txt"),
			"content":   string(largeContent),
		},
		Timestamp: time.Now(),
	}

	result, err = router.Execute(ctx, writeCall)
	if err != nil {
		t.Fatalf("Tool execution should not error: %v", err)
	}

	if result.Success {
		t.Error("Writing oversized file should fail")
	}

	t.Log("✓ File size limit enforced correctly")

	t.Log("=== Error scenarios completed successfully ===")
}

// TestToolChaining tests multiple tools working together
func TestToolChaining(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocodenow_chain_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	router := tools.NewToolRouter()
	policy := &tools.FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        1024 * 1024,
	}
	
	router.RegisterExecutor("write_file", tools.NewWriteFileExecutor(policy))
	router.RegisterExecutor("read_file", tools.NewReadFileExecutor(policy))
	router.RegisterExecutor("echo", tools.NewEchoExecutor())

	ctx := context.Background()

	t.Log("=== Scenario: Tool chaining workflow ===")

	// Chain: Write config → Read config → Echo summary
	
	// Step 1: Write configuration
	configContent := `{
  "app": "gocodenow",
  "version": "1.0.0",
  "features": ["file_ops", "bash", "git"]
}`

	writeCall := &tools.ToolCall{
		ID:       "chain-write-001",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": filepath.Join(tempDir, "app.json"),
			"content":   configContent,
		},
		Timestamp: time.Now(),
	}

	t.Log("Step 1: Writing config file...")
	writeResult, err := router.Execute(ctx, writeCall)
	if err != nil || !writeResult.Success {
		t.Fatalf("Write step failed: %v, %s", err, writeResult.ErrorMessage)
	}

	// Step 2: Read the config back
	readCall := &tools.ToolCall{
		ID:       "chain-read-001",
		ToolName: "read_file", 
		Parameters: map[string]interface{}{
			"path": filepath.Join(tempDir, "app.json"),
		},
		Timestamp: time.Now(),
	}

	t.Log("Step 2: Reading config file...")
	readResult, err := router.Execute(ctx, readCall)
	if err != nil || !readResult.Success {
		t.Fatalf("Read step failed: %v, %s", err, readResult.ErrorMessage)
	}

	// Verify content integrity
	if readResult.Result.(string) != configContent {
		t.Error("Config content was corrupted in write/read cycle")
	}

	// Step 3: Echo a summary
	echoCall := &tools.ToolCall{
		ID:       "chain-echo-001", 
		ToolName: "echo",
		Parameters: map[string]interface{}{
			"message": "Configuration file created and verified successfully",
		},
		Timestamp: time.Now(),
	}

	t.Log("Step 3: Echoing summary...")
	echoResult, err := router.Execute(ctx, echoCall)
	if err != nil || !echoResult.Success {
		t.Fatalf("Echo step failed: %v, %s", err, echoResult.ErrorMessage)
	}

	// Verify all tools executed within reasonable time
	totalDuration := writeResult.Duration + readResult.Duration + echoResult.Duration
	if totalDuration > time.Second {
		t.Errorf("Tool chain took too long: %v", totalDuration)
	}

	t.Log("✓ Tool chaining completed successfully")
	t.Logf("Total execution time: %v", totalDuration)

	t.Log("=== Tool chaining workflow completed ===")
}

// TestConcurrentToolExecution tests tools running in parallel  
func TestConcurrentToolExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocodenow_concurrent_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	router := tools.NewToolRouter()
	policy := &tools.FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        1024 * 1024,
	}
	
	router.RegisterExecutor("write_file", tools.NewWriteFileExecutor(policy))
	router.RegisterExecutor("echo", tools.NewEchoExecutor())

	ctx := context.Background()

	t.Log("=== Scenario: Concurrent tool execution ===")

	numJobs := 5
	results := make(chan *tools.ToolResult, numJobs)
	errors := make(chan error, numJobs)

	// Launch multiple write operations concurrently
	for i := 0; i < numJobs; i++ {
		go func(id int) {
			call := &tools.ToolCall{
				ID:       fmt.Sprintf("concurrent-write-%d", id),
				ToolName: "write_file",
				Parameters: map[string]interface{}{
					"file_path": filepath.Join(tempDir, fmt.Sprintf("file%d.txt", id)),
					"content":   fmt.Sprintf("Content for file %d", id),
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
			} else {
				t.Errorf("Concurrent job failed: %s", result.ErrorMessage)
			}
		case err := <-errors:
			t.Errorf("Concurrent job error: %v", err)
		case <-time.After(5 * time.Second):
			t.Errorf("Timeout waiting for concurrent job %d", i)
		}
	}

	if successCount != numJobs {
		t.Errorf("Expected %d successful jobs, got %d", numJobs, successCount)
	}

	// Verify all files were created
	for i := 0; i < numJobs; i++ {
		filePath := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Concurrent file %d was not created", i)
		}
	}

	t.Log("✓ All concurrent operations completed successfully")
	t.Log("=== Concurrent execution test completed ===")
}

// TestRealWorldScenario simulates a realistic user workflow
func TestRealWorldScenario(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gocodenow_realworld_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	router := tools.NewToolRouter()
	policy := &tools.FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        1024 * 1024,
	}
	
	router.RegisterExecutor("write_file", tools.NewWriteFileExecutor(policy))
	router.RegisterExecutor("read_file", tools.NewReadFileExecutor(policy))

	ctx := context.Background()

	t.Log("=== Real-world scenario: Creating a simple web project ===")

	// User: "Create a simple HTML page with CSS styling"
	
	// Step 1: Create HTML file
	htmlContent := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>My Simple Page</title>
    <link rel="stylesheet" href="style.css">
</head>
<body>
    <header>
        <h1>Welcome to My Simple Page</h1>
    </header>
    <main>
        <p>This is a simple HTML page created with gocodenow!</p>
    </main>
</body>
</html>`

	htmlCall := &tools.ToolCall{
		ID:       "create-html",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": filepath.Join(tempDir, "index.html"),
			"content":   htmlContent,
		},
		Timestamp: time.Now(),
	}

	t.Log("Creating HTML file...")
	result, err := router.Execute(ctx, htmlCall)
	if err != nil || !result.Success {
		t.Fatalf("HTML creation failed: %v, %s", err, result.ErrorMessage)
	}

	// Step 2: Create CSS file
	cssContent := `body {
    font-family: Arial, sans-serif;
    line-height: 1.6;
    margin: 0;
    padding: 0;
    background-color: #f4f4f4;
}

header {
    background: #333;
    color: white;
    text-align: center;
    padding: 1rem;
}

main {
    padding: 2rem;
    max-width: 800px;
    margin: 0 auto;
    background: white;
    box-shadow: 0 0 10px rgba(0,0,0,0.1);
}`

	cssCall := &tools.ToolCall{
		ID:       "create-css",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": filepath.Join(tempDir, "style.css"),
			"content":   cssContent,
		},
		Timestamp: time.Now(),
	}

	t.Log("Creating CSS file...")
	result, err = router.Execute(ctx, cssCall)
	if err != nil || !result.Success {
		t.Fatalf("CSS creation failed: %v, %s", err, result.ErrorMessage)
	}

	// Step 3: Verify both files exist and have correct content
	htmlReadCall := &tools.ToolCall{
		ID:       "verify-html",
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			"path": filepath.Join(tempDir, "index.html"),
		},
		Timestamp: time.Now(),
	}

	t.Log("Verifying HTML file...")
	result, err = router.Execute(ctx, htmlReadCall)
	if err != nil || !result.Success {
		t.Fatalf("HTML verification failed: %v, %s", err, result.ErrorMessage)
	}

	if result.Result.(string) != htmlContent {
		t.Error("HTML content verification failed")
	}

	cssReadCall := &tools.ToolCall{
		ID:       "verify-css",
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			"path": filepath.Join(tempDir, "style.css"),
		},
		Timestamp: time.Now(),
	}

	t.Log("Verifying CSS file...")
	result, err = router.Execute(ctx, cssReadCall)
	if err != nil || !result.Success {
		t.Fatalf("CSS verification failed: %v, %s", err, result.ErrorMessage)
	}

	if result.Result.(string) != cssContent {
		t.Error("CSS content verification failed")
	}

	t.Log("✓ Simple web project created successfully")
	t.Log("✓ HTML and CSS files verified")
	t.Log("=== Real-world scenario completed ===")
}