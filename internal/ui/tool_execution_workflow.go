package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gocodenow/internal/models"
	"gocodenow/internal/security"
	"gocodenow/internal/tools"
	"gocodenow/internal/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// ToolExecutionWorkflow manages the complete tool execution lifecycle
type ToolExecutionWorkflow struct {
	toolRouter        tools.ToolRouter
	conversations     *models.ConversationHistory
	securityService   *security.ConfirmationService
	resourceMonitor   *security.ResourceMonitor
	
	// Execution tracking
	executionStates   map[string]*ExecutionState // conversationID -> state
	retrySettings     RetrySettings
	mu                sync.RWMutex
}

// ExecutionState tracks the state of tool executions for a conversation
type ExecutionState struct {
	ConversationID    string
	ToolCalls         []types.ToolCall
	ToolResults       []types.ToolResult
	Status            ExecutionStatus
	StartTime         time.Time
	CompletionTime    *time.Time
	ErrorCount        int
	RetryCount        int
	PendingRetries    map[string]int // toolCallID -> retry count
	mu                sync.RWMutex
}

// ExecutionStatus represents the overall status of tool execution for a conversation
type ExecutionStatus int

const (
	ExecutionStatusPending ExecutionStatus = iota
	ExecutionStatusRunning
	ExecutionStatusCompleted
	ExecutionStatusFailed
	ExecutionStatusInterventionRequired
)

func (s ExecutionStatus) String() string {
	switch s {
	case ExecutionStatusPending:
		return "pending"
	case ExecutionStatusRunning:
		return "running"
	case ExecutionStatusCompleted:
		return "completed"
	case ExecutionStatusFailed:
		return "failed"
	case ExecutionStatusInterventionRequired:
		return "intervention_required"
	default:
		return "unknown"
	}
}

// RetrySettings configures retry behavior
type RetrySettings struct {
	MaxRetries        int           // Maximum number of retries per tool
	RetryDelay        time.Duration // Delay between retries
	BackoffMultiplier float64       // Exponential backoff multiplier
	RetriableErrors   []string      // Error patterns that are retriable
}

// DefaultRetrySettings returns sensible default retry settings
func DefaultRetrySettings() RetrySettings {
	return RetrySettings{
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		BackoffMultiplier: 2.0,
		RetriableErrors: []string{
			"timeout",
			"connection",
			"network",
			"temporary",
			"rate limit",
		},
	}
}

// NewToolExecutionWorkflow creates a new tool execution workflow manager
func NewToolExecutionWorkflow(
	toolRouter tools.ToolRouter,
	conversations *models.ConversationHistory,
	securityService *security.ConfirmationService,
	resourceMonitor *security.ResourceMonitor,
) *ToolExecutionWorkflow {
	return &ToolExecutionWorkflow{
		toolRouter:       toolRouter,
		conversations:    conversations,
		securityService:  securityService,
		resourceMonitor:  resourceMonitor,
		executionStates:  make(map[string]*ExecutionState),
		retrySettings:    DefaultRetrySettings(),
	}
}

// ExecuteToolCalls starts execution of tool calls for a conversation
func (w *ToolExecutionWorkflow) ExecuteToolCalls(ctx context.Context, conversationID string, toolCalls []types.ToolCall) tea.Cmd {
	return func() tea.Msg {
		// Create execution state
		state := &ExecutionState{
			ConversationID: conversationID,
			ToolCalls:      toolCalls,
			Status:         ExecutionStatusPending,
			StartTime:      time.Now(),
			PendingRetries: make(map[string]int),
		}
		
		w.mu.Lock()
		w.executionStates[conversationID] = state
		w.mu.Unlock()
		
		// Start execution
		return w.executeToolsSequentially(ctx, state)
	}
}

// executeToolsSequentially executes tools one by one with proper error handling
func (w *ToolExecutionWorkflow) executeToolsSequentially(ctx context.Context, state *ExecutionState) tea.Msg {
	state.mu.Lock()
	state.Status = ExecutionStatusRunning
	state.mu.Unlock()
	
	// Send status update
	tea.Sequence(
		func() tea.Msg {
			return ConversationStateUpdateMsg{
				ConversationID: state.ConversationID,
				Status:         types.StatusExecuting,
			}
		},
	)()
	
	var allResults []types.ToolResult
	hasFailures := false
	
	// Execute each tool call
	for _, toolCall := range state.ToolCalls {
		// Check if execution was cancelled
		select {
		case <-ctx.Done():
			return w.handleExecutionCancellation(state, ctx.Err())
		default:
		}
		
		// Execute single tool with retry logic
		result := w.executeToolWithRetry(ctx, state, toolCall)
		allResults = append(allResults, result)
		
		// Update progress
		tea.Sequence(
			func() tea.Msg {
				status := "completed"
				if !result.Success {
					status = "failed"
					hasFailures = true
				}
				
				return ToolExecutionUpdateMsg{
					ConversationID: state.ConversationID,
					ToolCallID:     toolCall.ID,
					Status:         status,
					Result:         fmt.Sprintf("%v", result.Result),
				}
			},
		)()
		
		// Handle streaming output if the tool supports it
		if len(result.StreamingOutput) > 0 {
			for _, chunk := range result.StreamingOutput {
				tea.Sequence(
					func() tea.Msg {
						return StreamingOutputUpdateMsg{
							ConversationID: state.ConversationID,
							ToolCallID:     toolCall.ID,
							ChunkData:      chunk.Data,
							ChunkType:      chunk.Type,
							IsComplete:     false,
						}
					},
				)()
			}
		}
	}
	
	// Update final state
	state.mu.Lock()
	state.ToolResults = allResults
	state.CompletionTime = timePtr(time.Now())
	
	if hasFailures {
		state.Status = ExecutionStatusFailed
		state.ErrorCount = w.countFailedResults(allResults)
	} else {
		state.Status = ExecutionStatusCompleted
	}
	state.mu.Unlock()
	
	// Update conversation
	w.updateConversationWithResults(state.ConversationID, allResults)
	
	// Clean up execution state
	w.mu.Lock()
	delete(w.executionStates, state.ConversationID)
	w.mu.Unlock()
	
	return ToolExecutionCompleteMsg{
		ConversationID: state.ConversationID,
		ToolResults:    allResults,
	}
}

// executeToolWithRetry executes a single tool with retry logic
func (w *ToolExecutionWorkflow) executeToolWithRetry(ctx context.Context, state *ExecutionState, toolCall types.ToolCall) types.ToolResult {
	var lastResult types.ToolResult
	var lastError error
	
	retryCount := 0
	maxRetries := w.retrySettings.MaxRetries
	
	for retryCount <= maxRetries {
		// Check security permissions before execution
		if w.securityService != nil {
			if confirmed, err := w.requestToolExecutionPermission(toolCall); err != nil || !confirmed {
				return types.ToolResult{
					ID:           uuid.New().String(),
					ToolCallID:   toolCall.ID,
					Success:      false,
					ErrorMessage: fmt.Sprintf("Security check failed: %v", err),
					Duration:     0,
					Timestamp:    time.Now(),
				}
			}
		}
		
		// Execute with resource monitoring
		var tracker *security.ResourceTracker
		if w.resourceMonitor != nil {
			var err error
			tracker, err = security.NewResourceTracker(
				w.resourceMonitor,
				fmt.Sprintf("tool_%s", toolCall.ToolName),
				map[string]interface{}{
					"conversation_id": state.ConversationID,
					"tool_call_id":   toolCall.ID,
					"retry_count":    retryCount,
				},
			)
			if err != nil {
				return types.ToolResult{
					ID:           uuid.New().String(),
					ToolCallID:   toolCall.ID,
					Success:      false,
					ErrorMessage: fmt.Sprintf("Resource monitoring failed: %v", err),
					Duration:     0,
					Timestamp:    time.Now(),
				}
			}
			defer tracker.Close()
		}
		
		// Convert to tools package format
		internalToolCall := &tools.ToolCall{
			ID:         toolCall.ID,
			ToolName:   toolCall.ToolName,
			Parameters: toolCall.Parameters,
			Timeout:    30 * time.Second, // Default timeout
		}
		
		// Execute the tool
		result, err := w.toolRouter.Execute(ctx, internalToolCall)
		
		if err == nil && result.Success {
			// Success - convert and return
			return w.convertToolResult(result)
		}
		
		// Handle failure
		lastError = err
		if result != nil {
			lastResult = w.convertToolResult(result)
		} else {
			lastResult = types.ToolResult{
				ID:           uuid.New().String(),
				ToolCallID:   toolCall.ID,
				Success:      false,
				ErrorMessage: err.Error(),
				Duration:     0,
				Timestamp:    time.Now(),
			}
		}
		
		// Check if error is retriable
		if !w.isRetriableError(lastError) {
			// Send error message for non-retriable errors
			tea.Sequence(
				func() tea.Msg {
					return ToolExecutionErrorMsg{
						ConversationID:       state.ConversationID,
						ToolCallID:           toolCall.ID,
						Error:                lastError,
						Recoverable:          false,
						RequiresIntervention: w.requiresUserIntervention(lastError),
					}
				},
			)()
			break
		}
		
		retryCount++
		if retryCount <= maxRetries {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(w.retrySettings.RetryDelay) * 
				pow(w.retrySettings.BackoffMultiplier, float64(retryCount-1)))
			
			// Wait before retry
			select {
			case <-ctx.Done():
				lastResult.ErrorMessage = fmt.Sprintf("Execution cancelled during retry: %v", ctx.Err())
				return lastResult
			case <-time.After(delay):
				// Continue to retry
			}
			
			// Update retry count in state
			state.mu.Lock()
			state.RetryCount++
			state.PendingRetries[toolCall.ID] = retryCount
			state.mu.Unlock()
			
			// Notify about retry
			tea.Sequence(
				func() tea.Msg {
					return ToolExecutionUpdateMsg{
						ConversationID: state.ConversationID,
						ToolCallID:     toolCall.ID,
						Status:         "retrying",
						Progress:       fmt.Sprintf("Retry attempt %d/%d", retryCount, maxRetries),
					}
				},
			)()
		}
	}
	
	// All retries exhausted
	if retryCount > maxRetries {
		lastResult.ErrorMessage = fmt.Sprintf("Max retries (%d) exceeded: %s", maxRetries, lastResult.ErrorMessage)
		
		// Send error message for exhausted retries
		tea.Sequence(
			func() tea.Msg {
				return ToolExecutionErrorMsg{
					ConversationID:       state.ConversationID,
					ToolCallID:           toolCall.ID,
					Error:                lastError,
					Recoverable:          true, // Could potentially be retried with different parameters
					RequiresIntervention: true, // User intervention required after max retries
				}
			},
		)()
	}
	
	return lastResult
}

// requestToolExecutionPermission requests permission for tool execution
func (w *ToolExecutionWorkflow) requestToolExecutionPermission(toolCall types.ToolCall) (bool, error) {
	// For high-risk tools, require explicit confirmation
	switch toolCall.ToolName {
	case "bash_command", "write_file", "delete_file":
		details := make(map[string]interface{})
		if params, ok := toolCall.Parameters["command"]; ok {
			details["command"] = params
		}
		if params, ok := toolCall.Parameters["file_path"]; ok {
			details["file_path"] = params
		}
		
		return w.securityService.RequestCommandConfirmation(fmt.Sprintf("%v", toolCall.Parameters))
	default:
		return true, nil // Auto-approve low-risk tools
	}
}

// isRetriableError determines if an error should be retried
func (w *ToolExecutionWorkflow) isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	
	errorMsg := err.Error()
	for _, pattern := range w.retrySettings.RetriableErrors {
		if contains(errorMsg, pattern) {
			return true
		}
	}
	
	// Check for specific error types
	if toolErr, ok := err.(*tools.ToolExecutionError); ok {
		return toolErr.Recoverable
	}
	
	return false
}

// requiresUserIntervention determines if an error requires user intervention
func (w *ToolExecutionWorkflow) requiresUserIntervention(err error) bool {
	if err == nil {
		return false
	}
	
	errorMsg := err.Error()
	
	// Errors that require user intervention
	interventionPatterns := []string{
		"permission denied",
		"access denied",
		"authentication required",
		"file not found",
		"directory not found",
		"invalid path",
		"security violation",
	}
	
	for _, pattern := range interventionPatterns {
		if contains(errorMsg, pattern) {
			return true
		}
	}
	
	// Check for specific error types that require intervention
	if toolErr, ok := err.(*tools.ToolExecutionError); ok {
		return !toolErr.Recoverable
	}
	
	return false
}

// convertToolResult converts from tools.ToolResult to types.ToolResult
func (w *ToolExecutionWorkflow) convertToolResult(result *tools.ToolResult) types.ToolResult {
	return types.ToolResult{
		ID:           result.ID,
		ToolCallID:   result.ToolCallID,
		Success:      result.Success,
		Result:       fmt.Sprintf("%v", result.Result),
		ErrorMessage: result.ErrorMessage,
		Duration:     result.Duration,
		Timestamp:    result.Timestamp,
	}
}

// updateConversationWithResults updates the conversation with tool results
func (w *ToolExecutionWorkflow) updateConversationWithResults(conversationID string, results []types.ToolResult) {
	conv, err := w.conversations.GetConversationByID(conversationID)
	if err != nil || conv == nil {
		return
	}
	
	// Update tool results
	conv.ToolResults = results
	
	// Calculate total execution time
	var totalDuration time.Duration
	for _, result := range results {
		totalDuration += result.Duration
	}
	conv.ExecutionTime = totalDuration
	
	// Update status based on results
	allSuccess := true
	for _, result := range results {
		if !result.Success {
			allSuccess = false
			break
		}
	}
	
	if allSuccess {
		conv.Status = types.StatusCompleted
	} else {
		conv.Status = types.StatusError
	}
}

// handleExecutionCancellation handles cancellation of tool execution
func (w *ToolExecutionWorkflow) handleExecutionCancellation(state *ExecutionState, err error) tea.Msg {
	state.mu.Lock()
	state.Status = ExecutionStatusFailed
	state.CompletionTime = timePtr(time.Now())
	state.mu.Unlock()
	
	return ToolExecutionErrorMsg{
		ConversationID:       state.ConversationID,
		ToolCallID:           "",
		Error:                fmt.Errorf("tool execution cancelled: %w", err),
		Recoverable:          false,
		RequiresIntervention: false,
	}
}

// countFailedResults counts the number of failed tool results
func (w *ToolExecutionWorkflow) countFailedResults(results []types.ToolResult) int {
	count := 0
	for _, result := range results {
		if !result.Success {
			count++
		}
	}
	return count
}

// GetExecutionState returns the current execution state for a conversation
func (w *ToolExecutionWorkflow) GetExecutionState(conversationID string) *ExecutionState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	if state, exists := w.executionStates[conversationID]; exists {
		return state
	}
	return nil
}

// RetryToolCall retries a specific tool call with updated parameters
func (w *ToolExecutionWorkflow) RetryToolCall(ctx context.Context, conversationID, toolCallID string, newParameters map[string]interface{}) tea.Cmd {
	return func() tea.Msg {
		// Get the execution state
		state := w.GetExecutionState(conversationID)
		if state == nil {
			return ToolExecutionErrorMsg{
				ConversationID:       conversationID,
				ToolCallID:           toolCallID,
				Error:                fmt.Errorf("execution state not found for conversation %s", conversationID),
				Recoverable:          false,
				RequiresIntervention: false,
			}
		}
		
		// Find the tool call to retry
		var toolCall types.ToolCall
		found := false
		for _, tc := range state.ToolCalls {
			if tc.ID == toolCallID {
				toolCall = tc
				if newParameters != nil {
					toolCall.Parameters = newParameters
				}
				found = true
				break
			}
		}
		
		if !found {
			return ToolExecutionErrorMsg{
				ConversationID:       conversationID,
				ToolCallID:           toolCallID,
				Error:                fmt.Errorf("tool call %s not found", toolCallID),
				Recoverable:          false,
				RequiresIntervention: false,
			}
		}
		
		// Reset retry count for this tool
		state.mu.Lock()
		delete(state.PendingRetries, toolCallID)
		state.mu.Unlock()
		
		// Retry the tool execution
		result := w.executeToolWithRetry(ctx, state, toolCall)
		
		// Update the tool result in the state
		state.mu.Lock()
		for i, existingResult := range state.ToolResults {
			if existingResult.ToolCallID == toolCallID {
				state.ToolResults[i] = result
				break
			}
		}
		state.mu.Unlock()
		
		// Notify completion
		return ToolExecutionUpdateMsg{
			ConversationID: conversationID,
			ToolCallID:     toolCallID,
			Status: func() string {
				if result.Success {
					return "completed"
				}
				return "failed"
			}(),
			Result: result.Result,
		}
	}
}

// Helper functions

func timePtr(t time.Time) *time.Time {
	return &t
}

func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexSubstring(s, substr) >= 0
}

func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}