package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestEchoExecutor_Basic(t *testing.T) {
	echo := NewEchoExecutor()
	ctx := context.Background()

	// Test basic echo
	params := ToolParameters{
		"message": "Hello, World!",
	}

	result, err := echo.Execute(ctx, params)
	if err != nil {
		t.Errorf("Echo execution failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}

	if result.Result != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %v", result.Result)
	}

	// Verify metadata
	metadata := result.Metadata
	if metadata["original_message"] != "Hello, World!" {
		t.Errorf("Expected original_message 'Hello, World!', got %v", metadata["original_message"])
	}
}

func TestEchoExecutor_Uppercase(t *testing.T) {
	echo := NewEchoExecutor()
	ctx := context.Background()

	// Test with uppercase
	params := ToolParameters{
		"message":   "hello world",
		"uppercase": true,
	}

	result, err := echo.Execute(ctx, params)
	if err != nil {
		t.Errorf("Echo execution failed: %v", err)
	}

	if result.Result != "HELLO WORLD" {
		t.Errorf("Expected 'HELLO WORLD', got %v", result.Result)
	}

	// Verify metadata
	metadata := result.Metadata
	if metadata["uppercase"] != true {
		t.Errorf("Expected uppercase true in metadata, got %v", metadata["uppercase"])
	}
}

func TestEchoExecutor_Repeat(t *testing.T) {
	echo := NewEchoExecutor()
	ctx := context.Background()

	// Test with repeat
	params := ToolParameters{
		"message": "test",
		"repeat":  3,
	}

	result, err := echo.Execute(ctx, params)
	if err != nil {
		t.Errorf("Echo execution failed: %v", err)
	}

	if result.Result != "test test test" {
		t.Errorf("Expected 'test test test', got %v", result.Result)
	}
}

func TestEchoExecutor_ValidationErrors(t *testing.T) {
	echo := NewEchoExecutor()
	ctx := context.Background()

	// Test missing required parameter
	params := ToolParameters{}

	_, err := echo.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for missing required parameter")
	}

	// Test invalid repeat value (too small)
	params = ToolParameters{
		"message": "test",
		"repeat":  0,
	}

	_, err = echo.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for repeat < 1")
	}

	// Test invalid repeat value (too large)
	params = ToolParameters{
		"message": "test",
		"repeat":  101,
	}

	_, err = echo.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for repeat > 100")
	}
}

func TestEchoExecutor_Schema(t *testing.T) {
	echo := NewEchoExecutor()
	schema := echo.Schema()

	if schema.Name != "echo" {
		t.Errorf("Expected schema name 'echo', got '%s'", schema.Name)
	}

	if len(schema.Required) != 1 || schema.Required[0] != "message" {
		t.Errorf("Expected required parameter 'message', got %v", schema.Required)
	}

	if len(schema.Examples) == 0 {
		t.Error("Expected schema to have examples")
	}
}

func TestSleepExecutor_Basic(t *testing.T) {
	sleep := NewSleepExecutor()
	ctx := context.Background()

	// Test basic sleep (short duration)
	params := ToolParameters{
		"duration_ms": 10,
	}

	start := time.Now()
	result, err := sleep.Execute(ctx, params)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Sleep execution failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.ErrorMessage)
	}

	// Should have slept for approximately 10ms
	if duration < 8*time.Millisecond || duration > 50*time.Millisecond {
		t.Errorf("Sleep duration out of expected range: %v", duration)
	}

	// Check result message
	expectedMessage := "Slept for 10 milliseconds"
	if result.Result != expectedMessage {
		t.Errorf("Expected '%s', got %v", expectedMessage, result.Result)
	}
}

func TestSleepExecutor_CustomMessage(t *testing.T) {
	sleep := NewSleepExecutor()
	ctx := context.Background()

	// Test with custom message
	params := ToolParameters{
		"duration_ms": 5,
		"message":     "Custom wake up message",
	}

	result, err := sleep.Execute(ctx, params)
	if err != nil {
		t.Errorf("Sleep execution failed: %v", err)
	}

	if result.Result != "Custom wake up message" {
		t.Errorf("Expected 'Custom wake up message', got %v", result.Result)
	}
}

func TestSleepExecutor_Cancellation(t *testing.T) {
	sleep := NewSleepExecutor()

	// Create context that cancels quickly
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	// Try to sleep longer than the context timeout
	params := ToolParameters{
		"duration_ms": 100,
	}

	result, err := sleep.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error due to context cancellation")
	}

	if result != nil && result.Success {
		t.Error("Expected failed result due to cancellation")
	}
}

func TestSleepExecutor_ValidationErrors(t *testing.T) {
	sleep := NewSleepExecutor()
	ctx := context.Background()

	// Test missing duration
	params := ToolParameters{}

	_, err := sleep.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for missing duration_ms")
	}

	// Test negative duration
	params = ToolParameters{
		"duration_ms": -10,
	}

	_, err = sleep.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for negative duration")
	}

	// Test too long duration
	params = ToolParameters{
		"duration_ms": 70000, // > 60 seconds
	}

	_, err = sleep.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for duration > 60 seconds")
	}
}

func TestCalculatorExecutor_BasicOperations(t *testing.T) {
	calc := NewCalculatorExecutor()
	ctx := context.Background()

	tests := []struct {
		operation string
		a, b      float64
		expected  float64
	}{
		{"add", 5, 3, 8},
		{"subtract", 10, 4, 6},
		{"multiply", 3, 7, 21},
		{"divide", 15, 3, 5},
		{"+", 2.5, 1.5, 4},
		{"-", 8.7, 3.2, 5.5},
		{"*", 2, 3.5, 7},
		{"/", 9, 2, 4.5},
	}

	for _, test := range tests {
		params := ToolParameters{
			"operation": test.operation,
			"a":         test.a,
			"b":         test.b,
		}

		result, err := calc.Execute(ctx, params)
		if err != nil {
			t.Errorf("Calculator execution failed for %s: %v", test.operation, err)
			continue
		}

		if !result.Success {
			t.Errorf("Expected successful execution for %s, got error: %s", test.operation, result.ErrorMessage)
			continue
		}

		resultValue, ok := result.Result.(float64)
		if !ok {
			t.Errorf("Expected float64 result for %s, got %T", test.operation, result.Result)
			continue
		}

		// Use a small epsilon for floating point comparison
		epsilon := 1e-10
		if resultValue < test.expected-epsilon || resultValue > test.expected+epsilon {
			t.Errorf("Expected %f for %s(%f, %f), got %f", test.expected, test.operation, test.a, test.b, resultValue)
		}
	}
}

func TestCalculatorExecutor_DivisionByZero(t *testing.T) {
	calc := NewCalculatorExecutor()
	ctx := context.Background()

	params := ToolParameters{
		"operation": "divide",
		"a":         10,
		"b":         0,
	}

	result, err := calc.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for division by zero")
	}

	if result != nil && result.Success {
		t.Error("Expected failed result for division by zero")
	}
}

func TestCalculatorExecutor_ValidationErrors(t *testing.T) {
	calc := NewCalculatorExecutor()
	ctx := context.Background()

	// Test missing parameters
	params := ToolParameters{
		"operation": "add",
		"a":         5,
		// missing "b"
	}

	_, err := calc.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for missing parameter 'b'")
	}

	// Test invalid operation
	params = ToolParameters{
		"operation": "invalid",
		"a":         5,
		"b":         3,
	}

	_, err = calc.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for invalid operation")
	}

	// Test non-numeric parameters
	params = ToolParameters{
		"operation": "add",
		"a":         "not a number",
		"b":         3,
	}

	_, err = calc.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for non-numeric parameter 'a'")
	}
}

func TestCalculatorExecutor_TypeConversion(t *testing.T) {
	calc := NewCalculatorExecutor()
	ctx := context.Background()

	// Test with mixed int and float
	params := ToolParameters{
		"operation": "add",
		"a":         5,     // int
		"b":         3.5,   // float64
	}

	result, err := calc.Execute(ctx, params)
	if err != nil {
		t.Errorf("Calculator execution failed: %v", err)
	}

	expected := 8.5
	if result.Result != expected {
		t.Errorf("Expected %f, got %v", expected, result.Result)
	}
}

func TestErrorExecutor_ValidationError(t *testing.T) {
	errorTool := NewErrorExecutor()
	ctx := context.Background()

	params := ToolParameters{
		"error_type": "validation",
		"message":    "Test validation error",
	}

	_, err := errorTool.Execute(ctx, params)
	if err == nil {
		t.Error("Expected validation error")
	}

	// Should be a ToolValidationError
	if _, ok := err.(*ToolValidationError); !ok {
		t.Errorf("Expected ToolValidationError, got %T", err)
	}

	if !strings.Contains(err.Error(), "Test validation error") {
		t.Errorf("Expected error message to contain 'Test validation error', got: %s", err.Error())
	}
}

func TestErrorExecutor_ExecutionError(t *testing.T) {
	errorTool := NewErrorExecutor()
	ctx := context.Background()

	params := ToolParameters{
		"error_type": "execution",
		"message":    "Test execution error",
		"recoverable": false,
	}

	_, err := errorTool.Execute(ctx, params)
	if err == nil {
		t.Error("Expected execution error")
	}

	// Should be a ToolExecutionError
	execError, ok := err.(*ToolExecutionError)
	if !ok {
		t.Errorf("Expected ToolExecutionError, got %T", err)
	}

	if execError.Recoverable {
		t.Error("Expected non-recoverable error")
	}

	if !strings.Contains(err.Error(), "Test execution error") {
		t.Errorf("Expected error message to contain 'Test execution error', got: %s", err.Error())
	}
}

func TestErrorExecutor_InvalidErrorType(t *testing.T) {
	errorTool := NewErrorExecutor()
	ctx := context.Background()

	params := ToolParameters{
		"error_type": "invalid",
	}

	_, err := errorTool.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for invalid error_type")
	}

	// Should be a ToolValidationError for invalid error_type
	if _, ok := err.(*ToolValidationError); !ok {
		t.Errorf("Expected ToolValidationError, got %T", err)
	}
}

// Test parameter validation helpers
func TestParameterHelpers(t *testing.T) {
	params := ToolParameters{
		"string_param": "test_value",
		"int_param":    42,
		"float_param":  3.14,
		"bool_param":   true,
		"map_param": map[string]interface{}{
			"key": "value",
		},
		"slice_param": []interface{}{"a", "b", "c"},
	}

	// Test GetStringParam
	if value, ok := GetStringParam(params, "string_param"); !ok || value != "test_value" {
		t.Errorf("GetStringParam failed: got %v, %v", value, ok)
	}

	if _, ok := GetStringParam(params, "nonexistent"); ok {
		t.Error("GetStringParam should return false for nonexistent key")
	}

	// Test GetRequiredStringParam
	if value, err := GetRequiredStringParam(params, "string_param"); err != nil || value != "test_value" {
		t.Errorf("GetRequiredStringParam failed: got %v, %v", value, err)
	}

	if _, err := GetRequiredStringParam(params, "nonexistent"); err == nil {
		t.Error("GetRequiredStringParam should return error for nonexistent key")
	}

	// Test GetIntParam
	if value, ok := GetIntParam(params, "int_param"); !ok || value != 42 {
		t.Errorf("GetIntParam failed: got %v, %v", value, ok)
	}

	// Test with float64 (should convert to int)
	if value, ok := GetIntParam(params, "float_param"); !ok || value != 3 {
		t.Errorf("GetIntParam with float64 failed: got %v, %v", value, ok)
	}

	// Test GetBoolParam
	if value, ok := GetBoolParam(params, "bool_param"); !ok || value != true {
		t.Errorf("GetBoolParam failed: got %v, %v", value, ok)
	}

	// Test GetMapParam
	if value, ok := GetMapParam(params, "map_param"); !ok || value["key"] != "value" {
		t.Errorf("GetMapParam failed: got %v, %v", value, ok)
	}

	// Test GetSliceParam
	if value, ok := GetSliceParam(params, "slice_param"); !ok || len(value) != 3 {
		t.Errorf("GetSliceParam failed: got %v, %v", value, ok)
	}
}