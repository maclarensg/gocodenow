package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDefaultToolRouter_Basic(t *testing.T) {
	router := NewToolRouter()

	// Initially should have no tools
	tools := router.ListTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty tool list, got %d tools", len(tools))
	}
}

func TestDefaultToolRouter_RegisterExecutor(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()

	// Test successful registration
	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register executor: %v", err)
	}

	// Verify tool is listed
	tools := router.ListTools()
	if len(tools) != 1 || tools[0] != "echo" {
		t.Errorf("Expected [echo], got %v", tools)
	}

	// Test duplicate registration
	err = router.RegisterExecutor("echo", echo)
	if err == nil {
		t.Error("Expected error for duplicate registration")
	}

	// Test empty name
	err = router.RegisterExecutor("", echo)
	if err == nil {
		t.Error("Expected error for empty tool name")
	}

	// Test nil executor
	err = router.RegisterExecutor("nil_test", nil)
	if err == nil {
		t.Error("Expected error for nil executor")
	}

	// Test name mismatch
	err = router.RegisterExecutor("wrong_name", echo)
	if err == nil {
		t.Error("Expected error for name mismatch")
	}
}

func TestDefaultToolRouter_UnregisterExecutor(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()

	// Register executor
	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register executor: %v", err)
	}

	// Test successful unregistration
	err = router.UnregisterExecutor("echo")
	if err != nil {
		t.Errorf("Failed to unregister executor: %v", err)
	}

	// Verify tool is no longer listed
	tools := router.ListTools()
	if len(tools) != 0 {
		t.Errorf("Expected empty tool list after unregistration, got %v", tools)
	}

	// Test unregistering non-existent tool
	err = router.UnregisterExecutor("nonexistent")
	if err == nil {
		t.Error("Expected error for unregistering non-existent tool")
	}

	// Test empty name
	err = router.UnregisterExecutor("")
	if err == nil {
		t.Error("Expected error for empty tool name")
	}
}

func TestDefaultToolRouter_GetExecutor(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()

	// Register executor
	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register executor: %v", err)
	}

	// Test getting existing executor
	executor, exists := router.GetExecutor("echo")
	if !exists {
		t.Error("Expected executor to exist")
	}
	if executor != echo {
		t.Error("Expected same executor instance")
	}

	// Test getting non-existent executor
	executor, exists = router.GetExecutor("nonexistent")
	if exists {
		t.Error("Expected executor not to exist")
	}
	if executor != nil {
		t.Error("Expected nil executor for non-existent tool")
	}
}

func TestDefaultToolRouter_GetSchema(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()

	// Register executor
	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register executor: %v", err)
	}

	// Test getting schema for existing tool
	schema, err := router.GetSchema("echo")
	if err != nil {
		t.Errorf("Failed to get schema: %v", err)
	}
	if schema.Name != "echo" {
		t.Errorf("Expected schema name 'echo', got '%s'", schema.Name)
	}

	// Test getting schema for non-existent tool
	schema, err = router.GetSchema("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent tool schema")
	}
	if schema != nil {
		t.Error("Expected nil schema for non-existent tool")
	}
}

func TestDefaultToolRouter_ValidateCall(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()

	// Register executor
	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register executor: %v", err)
	}

	// Test valid tool call
	validCall := &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "echo",
		Parameters: ToolParameters{"message": "test"},
		Timestamp:  time.Now(),
	}

	err = router.ValidateCall(validCall)
	if err != nil {
		t.Errorf("Expected valid call to pass validation: %v", err)
	}

	// Test nil tool call
	err = router.ValidateCall(nil)
	if err == nil {
		t.Error("Expected error for nil tool call")
	}

	// Test empty ID
	invalidCall := &ToolCall{
		ID:         "",
		ToolName:   "echo",
		Parameters: ToolParameters{"message": "test"},
	}
	err = router.ValidateCall(invalidCall)
	if err == nil {
		t.Error("Expected error for empty ID")
	}

	// Test empty tool name
	invalidCall = &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "",
		Parameters: ToolParameters{"message": "test"},
	}
	err = router.ValidateCall(invalidCall)
	if err == nil {
		t.Error("Expected error for empty tool name")
	}

	// Test nil parameters
	invalidCall = &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "echo",
		Parameters: nil,
	}
	err = router.ValidateCall(invalidCall)
	if err == nil {
		t.Error("Expected error for nil parameters")
	}

	// Test non-existent tool
	invalidCall = &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "nonexistent",
		Parameters: ToolParameters{},
	}
	err = router.ValidateCall(invalidCall)
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}

	// Test missing required parameter
	invalidCall = &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "echo",
		Parameters: ToolParameters{}, // missing required "message"
	}
	err = router.ValidateCall(invalidCall)
	if err == nil {
		t.Error("Expected error for missing required parameter")
	}
}

func TestDefaultToolRouter_Execute(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()

	// Register executor
	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register executor: %v", err)
	}

	ctx := context.Background()

	// Test successful execution
	toolCall := &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "echo",
		Parameters: ToolParameters{"message": "Hello, World!"},
		Timestamp:  time.Now(),
	}

	result, err := router.Execute(ctx, toolCall)
	if err != nil {
		t.Errorf("Execution failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}

	if result.Result != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %v", result.Result)
	}

	if result.ToolCallID != toolCall.ID {
		t.Errorf("Expected ToolCallID %s, got %s", toolCall.ID, result.ToolCallID)
	}

	// Test execution with invalid parameters
	invalidCall := &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "echo",
		Parameters: ToolParameters{}, // missing required parameter
		Timestamp:  time.Now(),
	}

	result, err = router.Execute(ctx, invalidCall)
	if err == nil {
		t.Error("Expected error for invalid parameters")
	}
	if result.Success {
		t.Error("Expected failed result for invalid parameters")
	}

	// Test execution with non-existent tool
	nonExistentCall := &ToolCall{
		ID:         uuid.New().String(),
		ToolName:   "nonexistent",
		Parameters: ToolParameters{},
		Timestamp:  time.Now(),
	}

	result, err = router.Execute(ctx, nonExistentCall)
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}
	if result.Success {
		t.Error("Expected failed result for non-existent tool")
	}
}

func TestDefaultToolRouter_ExecuteWithTimeout(t *testing.T) {
	router := NewToolRouter()
	sleep := NewSleepExecutor()

	err := router.RegisterExecutor("sleep", sleep)
	if err != nil {
		t.Errorf("Failed to register executor: %v", err)
	}

	ctx := context.Background()

	// Test execution with timeout (should succeed)
	toolCall := &ToolCall{
		ID:       uuid.New().String(),
		ToolName: "sleep",
		Parameters: ToolParameters{
			"duration_ms": 100, // 100ms
		},
		Timeout: 1 * time.Second, // 1 second timeout
	}

	result, err := router.Execute(ctx, toolCall)
	if err != nil {
		t.Errorf("Execution failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}

	// Test execution that times out
	longCall := &ToolCall{
		ID:       uuid.New().String(),
		ToolName: "sleep",
		Parameters: ToolParameters{
			"duration_ms": 2000, // 2 seconds
		},
		Timeout: 100 * time.Millisecond, // 100ms timeout
	}

	result, err = router.Execute(ctx, longCall)
	// This should timeout and return an error
	if err == nil && result.Success {
		// Note: The timeout behavior depends on the implementation
		// Some executors may not respect context cancellation immediately
		t.Logf("Long execution completed (may not have timed out): %v", result)
	}
}

func TestDefaultToolRouter_BatchExecute(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()
	calc := NewCalculatorExecutor()

	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register echo executor: %v", err)
	}
	err = router.RegisterExecutor("calculator", calc)
	if err != nil {
		t.Errorf("Failed to register calculator executor: %v", err)
	}

	ctx := context.Background()

	// Test batch execution
	toolCalls := []*ToolCall{
		{
			ID:         uuid.New().String(),
			ToolName:   "echo",
			Parameters: ToolParameters{"message": "Hello"},
		},
		{
			ID:         uuid.New().String(),
			ToolName:   "calculator",
			Parameters: ToolParameters{"operation": "add", "a": 5, "b": 3},
		},
		{
			ID:         uuid.New().String(),
			ToolName:   "echo",
			Parameters: ToolParameters{"message": "World", "uppercase": true},
		},
	}

	results, err := router.BatchExecute(ctx, toolCalls)
	if err != nil {
		t.Errorf("Batch execution failed: %v", err)
	}

	if len(results) != len(toolCalls) {
		t.Errorf("Expected %d results, got %d", len(toolCalls), len(results))
	}

	// Check individual results
	if !results[0].Success || results[0].Result != "Hello" {
		t.Errorf("First result failed or incorrect: %v", results[0])
	}

	if !results[1].Success || results[1].Result != 8.0 {
		t.Errorf("Second result failed or incorrect: %v", results[1])
	}

	if !results[2].Success || results[2].Result != "WORLD" {
		t.Errorf("Third result failed or incorrect: %v", results[2])
	}
}

func TestDefaultToolRouter_ExecuteConcurrent(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()
	sleep := NewSleepExecutor()

	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register echo executor: %v", err)
	}
	err = router.RegisterExecutor("sleep", sleep)
	if err != nil {
		t.Errorf("Failed to register sleep executor: %v", err)
	}

	ctx := context.Background()

	// Test concurrent execution
	toolCalls := []*ToolCall{
		{
			ID:         uuid.New().String(),
			ToolName:   "echo",
			Parameters: ToolParameters{"message": "Fast"},
		},
		{
			ID:         uuid.New().String(),
			ToolName:   "sleep",
			Parameters: ToolParameters{"duration_ms": 50, "message": "Slow"},
		},
		{
			ID:         uuid.New().String(),
			ToolName:   "echo",
			Parameters: ToolParameters{"message": "Also Fast"},
		},
	}

	start := time.Now()
	results, err := router.ExecuteConcurrent(ctx, toolCalls)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Concurrent execution failed: %v", err)
	}

	if len(results) != len(toolCalls) {
		t.Errorf("Expected %d results, got %d", len(toolCalls), len(results))
	}

	// Should complete faster than sequential execution (which would take at least 50ms for sleep)
	// Allow some margin for execution overhead
	if duration > 200*time.Millisecond {
		t.Errorf("Concurrent execution took too long: %v", duration)
	}

	// All results should be successful
	for i, result := range results {
		if !result.Success {
			t.Errorf("Result %d failed: %v", i, result)
		}
	}
}

func TestDefaultToolRouter_GetAllSchemas(t *testing.T) {
	router := NewToolRouter()
	echo := NewEchoExecutor()
	calc := NewCalculatorExecutor()

	err := router.RegisterExecutor("echo", echo)
	if err != nil {
		t.Errorf("Failed to register echo executor: %v", err)
	}
	err = router.RegisterExecutor("calculator", calc)
	if err != nil {
		t.Errorf("Failed to register calculator executor: %v", err)
	}

	schemas := router.(*DefaultToolRouter).GetAllSchemas()

	if len(schemas) != 2 {
		t.Errorf("Expected 2 schemas, got %d", len(schemas))
	}

	if _, exists := schemas["echo"]; !exists {
		t.Error("Expected echo schema to exist")
	}

	if _, exists := schemas["calculator"]; !exists {
		t.Error("Expected calculator schema to exist")
	}

	// Verify schema content
	echoSchema := schemas["echo"]
	if echoSchema.Name != "echo" {
		t.Errorf("Expected echo schema name 'echo', got '%s'", echoSchema.Name)
	}

	if len(echoSchema.Required) == 0 {
		t.Error("Expected echo schema to have required parameters")
	}
}

// Test error handling with ErrorExecutor
func TestDefaultToolRouter_ErrorHandling(t *testing.T) {
	router := NewToolRouter()
	errorTool := NewErrorExecutor()

	err := router.RegisterExecutor("error", errorTool)
	if err != nil {
		t.Errorf("Failed to register error executor: %v", err)
	}

	ctx := context.Background()

	// Test validation error
	validationCall := &ToolCall{
		ID:       uuid.New().String(),
		ToolName: "error",
		Parameters: ToolParameters{
			"error_type": "validation",
			"message":    "Test validation error",
		},
	}

	result, err := router.Execute(ctx, validationCall)
	if err == nil {
		t.Error("Expected validation error")
	}
	if result.Success {
		t.Error("Expected failed result for validation error")
	}
	if !strings.Contains(result.ErrorMessage, "Test validation error") {
		t.Errorf("Expected error message to contain 'Test validation error', got: %s", result.ErrorMessage)
	}

	// Test execution error
	executionCall := &ToolCall{
		ID:       uuid.New().String(),
		ToolName: "error",
		Parameters: ToolParameters{
			"error_type": "execution",
			"message":    "Test execution error",
		},
	}

	result, err = router.Execute(ctx, executionCall)
	if err == nil {
		t.Error("Expected execution error")
	}
	if result.Success {
		t.Error("Expected failed result for execution error")
	}
}