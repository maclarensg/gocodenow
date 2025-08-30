// Package tools provides the framework for executing LLM tool calls
// 
// This package defines the core interfaces and types for a pluggable tool execution
// system that can handle various types of operations including file operations,
// bash commands, search operations, and more.
//
// Example usage:
//
//	// Create a tool router
//	router := NewToolRouter()
//
//	// Register tool executors
//	router.RegisterExecutor("read_file", &ReadFileExecutor{})
//	router.RegisterExecutor("bash_command", &BashExecutor{})
//
//	// Execute a tool call
//	result, err := router.Execute(ctx, toolCall)
//
package tools

import (
	"context"
	"time"
)

// ToolExecutor defines the interface for executing specific tool operations
type ToolExecutor interface {
	// Execute runs the tool with the given parameters and returns the result
	Execute(ctx context.Context, params ToolParameters) (*ToolResult, error)
	
	// Name returns the name of this tool (e.g., "read_file", "bash_command")
	Name() string
	
	// Description returns a human-readable description of what this tool does
	Description() string
	
	// ValidateParameters checks if the provided parameters are valid for this tool
	ValidateParameters(params ToolParameters) error
	
	// Schema returns the JSON schema for this tool's parameters
	Schema() ToolSchema
}

// ToolParameters represents the parameters passed to a tool
type ToolParameters map[string]interface{}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	// Unique identifier for this tool call
	ID string `json:"id"`
	
	// Name of the tool to execute (e.g., "read_file", "bash_command")
	ToolName string `json:"tool_name"`
	
	// Parameters to pass to the tool
	Parameters ToolParameters `json:"parameters"`
	
	// Timestamp when this tool call was created
	Timestamp time.Time `json:"timestamp"`
	
	// Optional timeout for tool execution
	Timeout time.Duration `json:"timeout,omitempty"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	// Unique identifier for this result (should match the ToolCall ID)
	ID string `json:"id"`
	
	// ID of the ToolCall that produced this result
	ToolCallID string `json:"tool_call_id"`
	
	// Name of the tool that was executed
	ToolName string `json:"tool_name"`
	
	// Whether the tool execution was successful
	Success bool `json:"success"`
	
	// The actual result data (could be string, JSON, etc.)
	Result interface{} `json:"result"`
	
	// Error message if the execution failed
	ErrorMessage string `json:"error_message,omitempty"`
	
	// How long the tool took to execute
	Duration time.Duration `json:"duration"`
	
	// Timestamp when this result was created
	Timestamp time.Time `json:"timestamp"`
	
	// Additional metadata about the execution
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ToolSchema defines the parameter schema for a tool
type ToolSchema struct {
	// Tool name
	Name string `json:"name"`
	
	// Tool description
	Description string `json:"description"`
	
	// JSON schema for parameters
	Parameters map[string]interface{} `json:"parameters"`
	
	// Required parameter names
	Required []string `json:"required,omitempty"`
	
	// Examples of valid parameter sets
	Examples []ToolParameters `json:"examples,omitempty"`
}

// ToolRouter manages and routes tool calls to appropriate executors
type ToolRouter interface {
	// RegisterExecutor registers a tool executor
	RegisterExecutor(toolName string, executor ToolExecutor) error
	
	// UnregisterExecutor removes a tool executor
	UnregisterExecutor(toolName string) error
	
	// Execute runs a tool call and returns the result
	Execute(ctx context.Context, toolCall *ToolCall) (*ToolResult, error)
	
	// ListTools returns the names of all registered tools
	ListTools() []string
	
	// GetExecutor returns the executor for a given tool name
	GetExecutor(toolName string) (ToolExecutor, bool)
	
	// GetSchema returns the schema for a given tool
	GetSchema(toolName string) (*ToolSchema, error)
	
	// ValidateCall validates a tool call before execution
	ValidateCall(toolCall *ToolCall) error
	
	// BatchExecute executes multiple tool calls sequentially
	BatchExecute(ctx context.Context, toolCalls []*ToolCall) ([]*ToolResult, error)
	
	// ExecuteConcurrent executes multiple tool calls concurrently
	ExecuteConcurrent(ctx context.Context, toolCalls []*ToolCall) ([]*ToolResult, error)
}

// ToolExecutionError represents an error during tool execution
type ToolExecutionError struct {
	ToolName    string        `json:"tool_name"`
	ToolCallID  string        `json:"tool_call_id"`
	Message     string        `json:"message"`
	Cause       error         `json:"cause,omitempty"`
	Duration    time.Duration `json:"duration"`
	Recoverable bool          `json:"recoverable"`
}

func (e *ToolExecutionError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func (e *ToolExecutionError) Unwrap() error {
	return e.Cause
}

// ToolValidationError represents a parameter validation error
type ToolValidationError struct {
	ToolName   string `json:"tool_name"`
	Parameter  string `json:"parameter"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

func (e *ToolValidationError) Error() string {
	return e.Message
}

// Common tool parameter validation helpers

// GetStringParam safely retrieves a string parameter
func GetStringParam(params ToolParameters, key string) (string, bool) {
	if value, exists := params[key]; exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetRequiredStringParam retrieves a required string parameter or returns an error
func GetRequiredStringParam(params ToolParameters, key string) (string, error) {
	value, exists := GetStringParam(params, key)
	if !exists {
		return "", &ToolValidationError{
			Parameter: key,
			Message:   "required string parameter '" + key + "' is missing",
		}
	}
	if value == "" {
		return "", &ToolValidationError{
			Parameter: key,
			Message:   "required string parameter '" + key + "' cannot be empty",
		}
	}
	return value, nil
}

// GetIntParam safely retrieves an integer parameter
func GetIntParam(params ToolParameters, key string) (int, bool) {
	if value, exists := params[key]; exists {
		switch v := value.(type) {
		case int:
			return v, true
		case float64:
			return int(v), true
		}
	}
	return 0, false
}

// GetBoolParam safely retrieves a boolean parameter
func GetBoolParam(params ToolParameters, key string) (bool, bool) {
	if value, exists := params[key]; exists {
		if b, ok := value.(bool); ok {
			return b, true
		}
	}
	return false, false
}

// GetMapParam safely retrieves a map parameter
func GetMapParam(params ToolParameters, key string) (map[string]interface{}, bool) {
	if value, exists := params[key]; exists {
		if m, ok := value.(map[string]interface{}); ok {
			return m, true
		}
	}
	return nil, false
}

// GetSliceParam safely retrieves a slice parameter
func GetSliceParam(params ToolParameters, key string) ([]interface{}, bool) {
	if value, exists := params[key]; exists {
		if s, ok := value.([]interface{}); ok {
			return s, true
		}
	}
	return nil, false
}