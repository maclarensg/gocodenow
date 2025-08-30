package ui

import (
	"context"
	"time"

	"gocodenow/internal/llm"
	"gocodenow/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// Message processing workflow messages

// MessageProcessingStartedMsg indicates that message processing has begun
type MessageProcessingStartedMsg struct {
	ConversationID string
	ProcessorCmd   tea.Cmd // Command to continue processing
}

// MessageProcessingErrorMsg indicates an error during message processing
type MessageProcessingErrorMsg struct {
	ConversationID string
	Error          error
}

// MessageProcessingCompleteMsg indicates successful completion of message processing
type MessageProcessingCompleteMsg struct {
	ConversationID string
	FinalResponse  string
}

// LLM streaming messages

// LLMStreamingStartedMsg indicates that LLM streaming has begun
type LLMStreamingStartedMsg struct {
	ConversationID string
	StreamChannel  <-chan *llm.StreamEvent
	ProcessorCmd   tea.Cmd // Command to handle streaming
}

// StreamingContentUpdateMsg represents incremental content from LLM streaming
type StreamingContentUpdateMsg struct {
	ConversationID string
	Content        string // Incremental content chunk
	Complete       bool   // True when streaming is finished
}

// LLMResponseCompleteMsg indicates LLM response is complete (with or without tools)
type LLMResponseCompleteMsg struct {
	ConversationID string
	Response       string
	ToolCalls      []llm.ToolCall
	TokenUsage     llm.TokenUsage
}

// Tool execution messages

// ToolExecutionStartedMsg indicates that tool execution has begun
type ToolExecutionStartedMsg struct {
	ConversationID string
	ToolCalls      []types.ToolCall
	ProcessorCmd   tea.Cmd // Command to execute tools
}

// ToolExecutionCompleteMsg indicates all tools have finished execution
type ToolExecutionCompleteMsg struct {
	ConversationID string
	ToolResults    []types.ToolResult
}

// Error handling and retry messages

// ToolExecutionErrorMsg indicates a tool execution failed
type ToolExecutionErrorMsg struct {
	ConversationID        string
	ToolCallID            string
	Error                 error
	Recoverable           bool // Whether this error can be retried
	RequiresIntervention  bool // Whether user intervention is needed
}

// RetryToolExecutionMsg requests retry of a failed tool execution
type RetryToolExecutionMsg struct {
	ConversationID string
	ToolCallID     string
	MaxRetries     int
	NewParameters  map[string]interface{} // Optional new parameters for retry
}

// UserInterventionRequiredMsg indicates user input is needed to continue
type UserInterventionRequiredMsg struct {
	ConversationID string
	ToolCallID     string
	InterventionType string // "confirm", "choose", "input"
	Message        string
	Options        []string // For choice interventions
}

// UserInterventionResponseMsg contains the user's response to an intervention
type UserInterventionResponseMsg struct {
	ConversationID string
	ToolCallID     string
	Response       string
	Approved       bool // For confirmations
}

// Conversation state update messages

// ConversationStateUpdateMsg updates conversation state information
type ConversationStateUpdateMsg struct {
	ConversationID string
	Status         types.ConversationStatus
	ExecutionTime  *time.Duration
	TokenUsage     *types.TokenUsage
}

// Tool streaming messages (for long-running tools)

// ToolStreamingStartedMsg indicates a tool has started streaming output
type ToolStreamingStartedMsg struct {
	ConversationID string
	ToolCallID     string
	ToolName       string
}

// ToolStreamingCompleteMsg indicates tool streaming has finished
type ToolStreamingCompleteMsg struct {
	ConversationID string
	ToolCallID     string
	Success        bool
	FinalResult    string
}

// UI state management messages

// RefreshConversationViewMsg requests a refresh of the conversation display
type RefreshConversationViewMsg struct {
	ConversationID string
	ScrollToBottom bool
}

// ShowToolExecutionDetailsMsg requests showing detailed tool execution info
type ShowToolExecutionDetailsMsg struct {
	ConversationID string
	ToolCallID     string
}

// Animation and progress messages

// StartProgressAnimationMsg starts progress animation for a conversation
type StartProgressAnimationMsg struct {
	ConversationID string
}

// StopProgressAnimationMsg stops progress animation for a conversation
type StopProgressAnimationMsg struct {
	ConversationID string
}

// MessageProcessor command factories

// ProcessMessageCmd creates a command to process a user message
func ProcessMessageCmd(processor *MessageProcessor, userInput, modelName string) tea.Cmd {
	return func() tea.Msg {
		return processor.ProcessMessage(context.Background(), userInput, modelName)
	}
}

// RetryFailedToolCmd creates a command to retry a failed tool execution
func RetryFailedToolCmd(processor *MessageProcessor, conversationID, toolCallID string) tea.Cmd {
	return func() tea.Msg {
		return RetryToolExecutionMsg{
			ConversationID: conversationID,
			ToolCallID:     toolCallID,
			MaxRetries:     3, // Default retry limit
		}
	}
}

// Helper functions for message handling

// IsProcessingComplete returns true if the message indicates processing is complete
func IsProcessingComplete(msg tea.Msg) bool {
	switch msg.(type) {
	case MessageProcessingCompleteMsg, ToolExecutionCompleteMsg:
		return true
	default:
		return false
	}
}

// IsProcessingError returns true if the message indicates an error occurred
func IsProcessingError(msg tea.Msg) bool {
	switch msg.(type) {
	case MessageProcessingErrorMsg, ToolExecutionErrorMsg:
		return true
	default:
		return false
	}
}

// GetConversationID extracts the conversation ID from various message types
func GetConversationID(msg tea.Msg) string {
	switch m := msg.(type) {
	case MessageProcessingStartedMsg:
		return m.ConversationID
	case MessageProcessingErrorMsg:
		return m.ConversationID
	case MessageProcessingCompleteMsg:
		return m.ConversationID
	case LLMStreamingStartedMsg:
		return m.ConversationID
	case StreamingContentUpdateMsg:
		return m.ConversationID
	case ToolExecutionStartedMsg:
		return m.ConversationID
	case ToolExecutionCompleteMsg:
		return m.ConversationID
	case ToolExecutionErrorMsg:
		return m.ConversationID
	case UserInterventionRequiredMsg:
		return m.ConversationID
	case ConversationStateUpdateMsg:
		return m.ConversationID
	default:
		return ""
	}
}