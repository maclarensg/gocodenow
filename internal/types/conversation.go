package types

import (
	"time"
)

// ConversationBlock represents a single conversation exchange
type ConversationBlock struct {
	// Core fields
	ID          string    `json:"id"`
	UserInput   string    `json:"user_input"`
	LLMResponse string    `json:"llm_response"`
	ModelName   string    `json:"model_name"`
	Timestamp   time.Time `json:"timestamp"`
	
	// Status and metrics  
	Status        ConversationStatus `json:"status"`
	TokenUsage    TokenUsage         `json:"token_usage"`
	ExecutionTime time.Duration      `json:"execution_time"`
	ErrorMessage  string             `json:"error_message,omitempty"`
	
	// Tool execution data
	ToolCalls      []ToolCall      `json:"tool_calls"`
	ToolResults    []ToolResult    `json:"tool_results"`
	FileOperations []FileOperation `json:"file_operations"`
	
	// UI state (backward compatibility)
	Expanded bool `json:"expanded"`
	
	// Legacy fields for backward compatibility (deprecated)
	User      string   `json:"user,omitempty"`      // mapped to UserInput
	Assistant string   `json:"assistant,omitempty"` // mapped to LLMResponse
	Tools     []string `json:"tools,omitempty"`     // mapped to ToolCalls
	Files     []string `json:"files,omitempty"`     // mapped to FileOperations
}

// ConversationStatus represents the status of a conversation
type ConversationStatus int

const (
	StatusPending ConversationStatus = iota
	StatusExecuting
	StatusCompleted
	StatusError
	StatusConfigError // Configuration-related error
)

// String returns the string representation of ConversationStatus
func (s ConversationStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusExecuting:
		return "executing"
	case StatusCompleted:
		return "completed"
	case StatusError:
		return "error"
	case StatusConfigError:
		return "config_error"
	default:
		return "unknown"
	}
}

// ParseConversationStatus parses a string into ConversationStatus
func ParseConversationStatus(s string) ConversationStatus {
	switch s {
	case "pending":
		return StatusPending
	case "executing":
		return StatusExecuting
	case "completed":
		return StatusCompleted
	case "error":
		return StatusError
	case "config_error":
		return StatusConfigError
	default:
		return StatusPending // Default fallback
	}
}

// TokenUsage tracks token consumption for the conversation
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ToolCall represents a single tool call made by the LLM
type ToolCall struct {
	ID         string                 `json:"id"`
	ToolName   string                 `json:"tool_name"`
	Parameters map[string]interface{} `json:"parameters"`
	Timestamp  time.Time              `json:"timestamp"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ID           string        `json:"id"`
	ToolCallID   string        `json:"tool_call_id"`
	Success      bool          `json:"success"`
	Result       string        `json:"result"`
	ErrorMessage string        `json:"error_message,omitempty"`
	Duration     time.Duration `json:"duration"`
	Timestamp    time.Time     `json:"timestamp"`
	
	// Streaming output support
	StreamingOutput []StreamingChunk `json:"streaming_output,omitempty"`
	IsStreaming     bool             `json:"is_streaming"`
	StreamComplete  bool             `json:"stream_complete"`
}

// StreamingChunk represents a chunk of streaming output
type StreamingChunk struct {
	Timestamp time.Time `json:"timestamp"`
	Data      string    `json:"data"`
	Type      string    `json:"type"` // stdout, stderr, info, error
}

// FileOperation represents a file operation performed during tool execution
type FileOperation struct {
	ID              string    `json:"id"`
	ToolResultID    string    `json:"tool_result_id"`
	OperationType   string    `json:"operation_type"` // read, write, edit, delete, etc.
	FilePath        string    `json:"file_path"`
	OldContent      string    `json:"old_content,omitempty"`
	NewContent      string    `json:"new_content,omitempty"`
	Success         bool      `json:"success"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}