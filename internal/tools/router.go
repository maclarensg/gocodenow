package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultToolRouter implements the ToolRouter interface
type DefaultToolRouter struct {
	executors map[string]ToolExecutor
	mu        sync.RWMutex
}

// NewToolRouter creates a new instance of DefaultToolRouter
func NewToolRouter() ToolRouter {
	return &DefaultToolRouter{
		executors: make(map[string]ToolExecutor),
	}
}

// RegisterExecutor registers a tool executor
func (r *DefaultToolRouter) RegisterExecutor(toolName string, executor ToolExecutor) error {
	if toolName == "" {
		return &ToolValidationError{
			ToolName: toolName,
			Message:  "tool name cannot be empty",
		}
	}
	
	if executor == nil {
		return &ToolValidationError{
			ToolName: toolName,
			Message:  "executor cannot be nil",
		}
	}
	
	// Validate that the executor's name matches
	if executor.Name() != toolName {
		return &ToolValidationError{
			ToolName: toolName,
			Message:  fmt.Sprintf("executor name '%s' does not match registration name '%s'", executor.Name(), toolName),
		}
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Check for duplicate registration
	if _, exists := r.executors[toolName]; exists {
		return &ToolValidationError{
			ToolName: toolName,
			Message:  fmt.Sprintf("tool '%s' is already registered", toolName),
		}
	}
	
	r.executors[toolName] = executor
	return nil
}

// UnregisterExecutor removes a tool executor
func (r *DefaultToolRouter) UnregisterExecutor(toolName string) error {
	if toolName == "" {
		return &ToolValidationError{
			ToolName: toolName,
			Message:  "tool name cannot be empty",
		}
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.executors[toolName]; !exists {
		return &ToolValidationError{
			ToolName: toolName,
			Message:  fmt.Sprintf("tool '%s' is not registered", toolName),
		}
	}
	
	delete(r.executors, toolName)
	return nil
}

// Execute runs a tool call and returns the result
func (r *DefaultToolRouter) Execute(ctx context.Context, toolCall *ToolCall) (*ToolResult, error) {
	startTime := time.Now()
	
	// Validate the tool call first
	if err := r.ValidateCall(toolCall); err != nil {
		return &ToolResult{
			ID:           uuid.New().String(),
			ToolCallID:   toolCall.ID,
			ToolName:     toolCall.ToolName,
			Success:      false,
			Result:       nil,
			ErrorMessage: err.Error(),
			Duration:     time.Since(startTime),
			Timestamp:    time.Now(),
		}, err
	}
	
	// Get the executor
	executor, exists := r.GetExecutor(toolCall.ToolName)
	if !exists {
		duration := time.Since(startTime)
		execError := &ToolExecutionError{
			ToolName:    toolCall.ToolName,
			ToolCallID:  toolCall.ID,
			Message:     fmt.Sprintf("no executor registered for tool '%s'", toolCall.ToolName),
			Duration:    duration,
			Recoverable: false,
		}
		
		return &ToolResult{
			ID:           uuid.New().String(),
			ToolCallID:   toolCall.ID,
			ToolName:     toolCall.ToolName,
			Success:      false,
			Result:       nil,
			ErrorMessage: execError.Error(),
			Duration:     duration,
			Timestamp:    time.Now(),
		}, execError
	}
	
	// Create execution context with timeout if specified
	execCtx := ctx
	var cancel context.CancelFunc
	if toolCall.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, toolCall.Timeout)
		defer cancel()
	}
	
	// Execute the tool
	result, err := executor.Execute(execCtx, toolCall.Parameters)
	duration := time.Since(startTime)
	
	if err != nil {
		// Handle execution error
		var execError *ToolExecutionError
		if te, ok := err.(*ToolExecutionError); ok {
			execError = te
			execError.Duration = duration
		} else {
			execError = &ToolExecutionError{
				ToolName:    toolCall.ToolName,
				ToolCallID:  toolCall.ID,
				Message:     "tool execution failed",
				Cause:       err,
				Duration:    duration,
				Recoverable: true, // Most errors are recoverable
			}
		}
		
		return &ToolResult{
			ID:           uuid.New().String(),
			ToolCallID:   toolCall.ID,
			ToolName:     toolCall.ToolName,
			Success:      false,
			Result:       nil,
			ErrorMessage: execError.Error(),
			Duration:     duration,
			Timestamp:    time.Now(),
			Metadata: map[string]interface{}{
				"recoverable": execError.Recoverable,
			},
		}, execError
	}
	
	// Success case
	if result == nil {
		result = &ToolResult{
			ID:         uuid.New().String(),
			ToolCallID: toolCall.ID,
			ToolName:   toolCall.ToolName,
			Success:    true,
			Duration:   duration,
			Timestamp:  time.Now(),
		}
	} else {
		// Ensure result has proper metadata
		result.ID = uuid.New().String()
		result.ToolCallID = toolCall.ID
		result.ToolName = toolCall.ToolName
		result.Duration = duration
		result.Timestamp = time.Now()
	}
	
	return result, nil
}

// ListTools returns the names of all registered tools
func (r *DefaultToolRouter) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	tools := make([]string, 0, len(r.executors))
	for toolName := range r.executors {
		tools = append(tools, toolName)
	}
	return tools
}

// GetExecutor returns the executor for a given tool name
func (r *DefaultToolRouter) GetExecutor(toolName string) (ToolExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	executor, exists := r.executors[toolName]
	return executor, exists
}

// GetSchema returns the schema for a given tool
func (r *DefaultToolRouter) GetSchema(toolName string) (*ToolSchema, error) {
	executor, exists := r.GetExecutor(toolName)
	if !exists {
		return nil, &ToolValidationError{
			ToolName: toolName,
			Message:  fmt.Sprintf("no executor registered for tool '%s'", toolName),
		}
	}
	
	schema := executor.Schema()
	return &schema, nil
}

// ValidateCall validates a tool call before execution
func (r *DefaultToolRouter) ValidateCall(toolCall *ToolCall) error {
	if toolCall == nil {
		return &ToolValidationError{
			Message: "tool call cannot be nil",
		}
	}
	
	if toolCall.ID == "" {
		return &ToolValidationError{
			ToolName: toolCall.ToolName,
			Message:  "tool call ID cannot be empty",
		}
	}
	
	if toolCall.ToolName == "" {
		return &ToolValidationError{
			ToolName: toolCall.ToolName,
			Message:  "tool name cannot be empty",
		}
	}
	
	if toolCall.Parameters == nil {
		return &ToolValidationError{
			ToolName: toolCall.ToolName,
			Message:  "parameters cannot be nil",
		}
	}
	
	// Get executor to validate parameters
	executor, exists := r.GetExecutor(toolCall.ToolName)
	if !exists {
		return &ToolValidationError{
			ToolName: toolCall.ToolName,
			Message:  fmt.Sprintf("no executor registered for tool '%s'", toolCall.ToolName),
		}
	}
	
	// Validate parameters using the executor
	if err := executor.ValidateParameters(toolCall.Parameters); err != nil {
		if ve, ok := err.(*ToolValidationError); ok {
			ve.ToolName = toolCall.ToolName
			return ve
		}
		return &ToolValidationError{
			ToolName: toolCall.ToolName,
			Message:  "parameter validation failed: " + err.Error(),
		}
	}
	
	return nil
}

// GetAllSchemas returns schemas for all registered tools
func (r *DefaultToolRouter) GetAllSchemas() map[string]*ToolSchema {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	schemas := make(map[string]*ToolSchema)
	for toolName, executor := range r.executors {
		schema := executor.Schema()
		schemas[toolName] = &schema
	}
	
	return schemas
}

// ExecuteWithTimeout is a convenience method to execute a tool call with a specific timeout
func (r *DefaultToolRouter) ExecuteWithTimeout(ctx context.Context, toolCall *ToolCall, timeout time.Duration) (*ToolResult, error) {
	toolCall.Timeout = timeout
	return r.Execute(ctx, toolCall)
}

// BatchExecute executes multiple tool calls and returns their results
// This method executes tools sequentially - for parallel execution, use ExecuteConcurrent
func (r *DefaultToolRouter) BatchExecute(ctx context.Context, toolCalls []*ToolCall) ([]*ToolResult, error) {
	results := make([]*ToolResult, len(toolCalls))
	
	for i, toolCall := range toolCalls {
		result, err := r.Execute(ctx, toolCall)
		if err != nil {
			// For batch execution, we collect the error in the result but continue processing
			results[i] = result
		} else {
			results[i] = result
		}
	}
	
	return results, nil
}

// ExecuteConcurrent executes multiple tool calls concurrently
func (r *DefaultToolRouter) ExecuteConcurrent(ctx context.Context, toolCalls []*ToolCall) ([]*ToolResult, error) {
	if len(toolCalls) == 0 {
		return []*ToolResult{}, nil
	}
	
	results := make([]*ToolResult, len(toolCalls))
	errors := make([]error, len(toolCalls))
	
	var wg sync.WaitGroup
	
	for i, toolCall := range toolCalls {
		wg.Add(1)
		go func(index int, tc *ToolCall) {
			defer wg.Done()
			result, err := r.Execute(ctx, tc)
			results[index] = result
			errors[index] = err
		}(i, toolCall)
	}
	
	wg.Wait()
	
	// Check if any executions failed
	var firstError error
	for _, err := range errors {
		if err != nil && firstError == nil {
			firstError = err
		}
	}
	
	return results, firstError
}