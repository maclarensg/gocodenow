package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// EchoExecutor is a simple tool that echoes back the input
type EchoExecutor struct {
	*BaseExecutor
}

// NewEchoExecutor creates a new echo tool executor
func NewEchoExecutor() *EchoExecutor {
	base := NewBaseExecutor("echo", "Echoes back the provided message")
	
	// Define schema
	base.AddRequiredParam("message", "string", "The message to echo back")
	base.AddOptionalParam("uppercase", "boolean", "Whether to convert message to uppercase", false)
	base.AddOptionalParam("repeat", "integer", "Number of times to repeat the message", 1)
	
	// Add examples
	base.AddExample(ToolParameters{
		"message": "Hello, World!",
	})
	base.AddExample(ToolParameters{
		"message":   "test",
		"uppercase": true,
		"repeat":    3,
	})
	
	return &EchoExecutor{BaseExecutor: base}
}

// Execute echoes back the message with optional transformations
func (e *EchoExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get required parameters
	message, err := GetRequiredStringParam(params, "message")
	if err != nil {
		return nil, err
	}
	
	// Get optional parameters
	uppercase, _ := GetBoolParam(params, "uppercase")
	repeat, exists := GetIntParam(params, "repeat")
	if !exists {
		repeat = 1
	}
	
	// Validate repeat parameter
	if repeat < 1 {
		return nil, &ToolValidationError{
			ToolName:  e.Name(),
			Parameter: "repeat",
			Message:   "repeat must be at least 1",
		}
	}
	if repeat > 100 {
		return nil, &ToolValidationError{
			ToolName:  e.Name(),
			Parameter: "repeat",
			Message:   "repeat cannot exceed 100",
		}
	}
	
	// Transform message
	result := message
	if uppercase {
		result = strings.ToUpper(result)
	}
	
	// Repeat message
	if repeat > 1 {
		parts := make([]string, repeat)
		for i := 0; i < repeat; i++ {
			parts[i] = result
		}
		result = strings.Join(parts, " ")
	}
	
	return &ToolResult{
		Success: true,
		Result:  result,
		Metadata: map[string]interface{}{
			"original_message": message,
			"uppercase":        uppercase,
			"repeat_count":     repeat,
		},
	}, nil
}

// SleepExecutor is a tool that sleeps for a specified duration
type SleepExecutor struct {
	*BaseExecutor
}

// NewSleepExecutor creates a new sleep tool executor
func NewSleepExecutor() *SleepExecutor {
	base := NewBaseExecutor("sleep", "Sleeps for the specified duration")
	
	// Define schema
	base.AddRequiredParam("duration_ms", "integer", "Duration to sleep in milliseconds")
	base.AddOptionalParam("message", "string", "Optional message to return after sleeping", "")
	
	// Add examples
	base.AddExample(ToolParameters{
		"duration_ms": 1000,
	})
	base.AddExample(ToolParameters{
		"duration_ms": 500,
		"message":     "Woke up!",
	})
	
	return &SleepExecutor{BaseExecutor: base}
}

// Execute sleeps for the specified duration
func (s *SleepExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get required parameters
	durationMs, exists := GetIntParam(params, "duration_ms")
	if !exists {
		return nil, &ToolValidationError{
			ToolName:  s.Name(),
			Parameter: "duration_ms",
			Message:   "duration_ms is required and must be an integer",
		}
	}
	
	// Validate duration
	if durationMs < 0 {
		return nil, &ToolValidationError{
			ToolName:  s.Name(),
			Parameter: "duration_ms",
			Message:   "duration_ms cannot be negative",
		}
	}
	if durationMs > 60000 { // Max 1 minute
		return nil, &ToolValidationError{
			ToolName:  s.Name(),
			Parameter: "duration_ms",
			Message:   "duration_ms cannot exceed 60000 (1 minute)",
		}
	}
	
	// Get optional message
	message, _ := GetStringParam(params, "message")
	if message == "" {
		message = fmt.Sprintf("Slept for %d milliseconds", durationMs)
	}
	
	// Sleep with context cancellation support
	duration := time.Duration(durationMs) * time.Millisecond
	timer := time.NewTimer(duration)
	defer timer.Stop()
	
	select {
	case <-ctx.Done():
		return nil, &ToolExecutionError{
			ToolName:    s.Name(),
			Message:     "sleep was cancelled",
			Cause:       ctx.Err(),
			Recoverable: true,
		}
	case <-timer.C:
		// Sleep completed successfully
	}
	
	return &ToolResult{
		Success: true,
		Result:  message,
		Metadata: map[string]interface{}{
			"duration_ms": durationMs,
		},
	}, nil
}

// CalculatorExecutor performs basic arithmetic operations
type CalculatorExecutor struct {
	*BaseExecutor
}

// NewCalculatorExecutor creates a new calculator tool executor
func NewCalculatorExecutor() *CalculatorExecutor {
	base := NewBaseExecutor("calculator", "Performs basic arithmetic operations")
	
	// Define schema
	base.AddRequiredParam("operation", "string", "The operation to perform (add, subtract, multiply, divide)")
	base.AddRequiredParam("a", "number", "First operand")
	base.AddRequiredParam("b", "number", "Second operand")
	
	// Add examples
	base.AddExample(ToolParameters{
		"operation": "add",
		"a":         10,
		"b":         5,
	})
	base.AddExample(ToolParameters{
		"operation": "divide",
		"a":         20.5,
		"b":         4,
	})
	
	return &CalculatorExecutor{BaseExecutor: base}
}

// Execute performs the arithmetic operation
func (c *CalculatorExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get operation
	operation, err := GetRequiredStringParam(params, "operation")
	if err != nil {
		return nil, err
	}
	
	// Get operands
	aValue, aExists := params["a"]
	bValue, bExists := params["b"]
	
	if !aExists {
		return nil, &ToolValidationError{
			ToolName:  c.Name(),
			Parameter: "a",
			Message:   "parameter 'a' is required",
		}
	}
	if !bExists {
		return nil, &ToolValidationError{
			ToolName:  c.Name(),
			Parameter: "b",
			Message:   "parameter 'b' is required",
		}
	}
	
	// Convert to float64
	var a, b float64
	
	switch v := aValue.(type) {
	case int:
		a = float64(v)
	case float64:
		a = v
	case float32:
		a = float64(v)
	default:
		return nil, &ToolValidationError{
			ToolName:  c.Name(),
			Parameter: "a",
			Message:   "parameter 'a' must be a number",
		}
	}
	
	switch v := bValue.(type) {
	case int:
		b = float64(v)
	case float64:
		b = v
	case float32:
		b = float64(v)
	default:
		return nil, &ToolValidationError{
			ToolName:  c.Name(),
			Parameter: "b",
			Message:   "parameter 'b' must be a number",
		}
	}
	
	// Perform operation
	var result float64
	var resultDescription string
	
	switch strings.ToLower(operation) {
	case "add", "+":
		result = a + b
		resultDescription = fmt.Sprintf("%.2f + %.2f = %.2f", a, b, result)
	case "subtract", "-":
		result = a - b
		resultDescription = fmt.Sprintf("%.2f - %.2f = %.2f", a, b, result)
	case "multiply", "*":
		result = a * b
		resultDescription = fmt.Sprintf("%.2f * %.2f = %.2f", a, b, result)
	case "divide", "/":
		if b == 0 {
			return nil, &ToolExecutionError{
				ToolName:    c.Name(),
				Message:     "division by zero is not allowed",
				Recoverable: true,
			}
		}
		result = a / b
		resultDescription = fmt.Sprintf("%.2f / %.2f = %.2f", a, b, result)
	default:
		return nil, &ToolValidationError{
			ToolName:  c.Name(),
			Parameter: "operation",
			Message:   fmt.Sprintf("unsupported operation '%s'. Supported: add, subtract, multiply, divide", operation),
		}
	}
	
	return &ToolResult{
		Success: true,
		Result:  result,
		Metadata: map[string]interface{}{
			"operation":   operation,
			"operand_a":   a,
			"operand_b":   b,
			"description": resultDescription,
		},
	}, nil
}

// ErrorExecutor is a tool that always fails (for testing error handling)
type ErrorExecutor struct {
	*BaseExecutor
}

// NewErrorExecutor creates a new error tool executor
func NewErrorExecutor() *ErrorExecutor {
	base := NewBaseExecutor("error", "A tool that always fails (for testing)")
	
	// Define schema
	base.AddRequiredParam("error_type", "string", "Type of error to generate (validation, execution, timeout)")
	base.AddOptionalParam("message", "string", "Custom error message", "")
	base.AddOptionalParam("recoverable", "boolean", "Whether the error should be marked as recoverable", true)
	
	// Add examples
	base.AddExample(ToolParameters{
		"error_type": "execution",
		"message":    "Something went wrong",
	})
	
	return &ErrorExecutor{BaseExecutor: base}
}

// Execute always returns an error
func (e *ErrorExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	errorType, err := GetRequiredStringParam(params, "error_type")
	if err != nil {
		return nil, err
	}
	
	message, _ := GetStringParam(params, "message")
	if message == "" {
		message = fmt.Sprintf("Generated %s error", errorType)
	}
	
	recoverable, exists := GetBoolParam(params, "recoverable")
	if !exists {
		recoverable = true
	}
	
	switch strings.ToLower(errorType) {
	case "validation":
		return nil, &ToolValidationError{
			ToolName: e.Name(),
			Message:  message,
		}
	case "execution":
		return nil, &ToolExecutionError{
			ToolName:    e.Name(),
			Message:     message,
			Recoverable: recoverable,
		}
	case "timeout":
		// Simulate a long operation that will be cancelled
		select {
		case <-ctx.Done():
			return nil, &ToolExecutionError{
				ToolName:    e.Name(),
				Message:     "operation timed out",
				Cause:       ctx.Err(),
				Recoverable: true,
			}
		case <-time.After(10 * time.Second):
			return nil, &ToolExecutionError{
				ToolName:    e.Name(),
				Message:     message,
				Recoverable: recoverable,
			}
		}
	default:
		return nil, &ToolValidationError{
			ToolName:  e.Name(),
			Parameter: "error_type",
			Message:   fmt.Sprintf("unsupported error_type '%s'. Supported: validation, execution, timeout", errorType),
		}
	}
}