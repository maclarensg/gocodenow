package types

import (
	"testing"
	"time"
)

func TestConversationStatus_String(t *testing.T) {
	tests := []struct {
		status   ConversationStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusExecuting, "executing"},  
		{StatusCompleted, "completed"},
		{StatusError, "error"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if got := test.status.String(); got != test.expected {
				t.Errorf("ConversationStatus.String() = %v, want %v", got, test.expected)
			}
		})
	}
}

func TestConversationBlock_Creation(t *testing.T) {
	block := &ConversationBlock{
		ID:            "test-id-123",
		UserInput:     "Test user input",
		LLMResponse:   "Test assistant response",
		Timestamp:     time.Now(),
		Status:        StatusPending,
		ExecutionTime: time.Millisecond * 500,
	}

	if block.ID != "test-id-123" {
		t.Errorf("ID = %v, want test-id-123", block.ID)
	}

	if block.UserInput != "Test user input" {
		t.Errorf("UserInput = %v, want 'Test user input'", block.UserInput)
	}

	if block.LLMResponse != "Test assistant response" {
		t.Errorf("LLMResponse = %v, want 'Test assistant response'", block.LLMResponse)
	}

	if block.Status != StatusPending {
		t.Errorf("Status = %v, want StatusPending", block.Status)
	}

	if block.ExecutionTime != time.Millisecond*500 {
		t.Errorf("ExecutionTime = %v, want 500ms", block.ExecutionTime)
	}
}

func TestTokenUsage_Basic(t *testing.T) {
	usage := TokenUsage{
		InputTokens:  150,
		OutputTokens: 250,
	}

	if usage.InputTokens != 150 {
		t.Errorf("InputTokens = %v, want 150", usage.InputTokens)
	}

	if usage.OutputTokens != 250 {
		t.Errorf("OutputTokens = %v, want 250", usage.OutputTokens)
	}

	// Test that total can be calculated
	total := usage.InputTokens + usage.OutputTokens
	if total != 400 {
		t.Errorf("Total tokens = %v, want 400", total)
	}
}

func TestToolCall_Basic(t *testing.T) {
	toolCall := ToolCall{
		ID:         "call-123",
		ToolName:   "test-tool",
		Parameters: map[string]interface{}{"param": "value"},
		Timestamp:  time.Now(),
	}

	if toolCall.ID != "call-123" {
		t.Errorf("ID = %v, want call-123", toolCall.ID)
	}

	if toolCall.ToolName != "test-tool" {
		t.Errorf("ToolName = %v, want test-tool", toolCall.ToolName)
	}

	if toolCall.Parameters["param"] != "value" {
		t.Error("Parameters should contain expected values")
	}
}

func TestFileOperation_Basic(t *testing.T) {
	fileOp := FileOperation{
		ID:            "file-op-123",
		OperationType: "write",
		FilePath:      "/test/file.txt",
		Success:       true,
		Timestamp:     time.Now(),
	}

	if fileOp.OperationType != "write" {
		t.Errorf("OperationType = %v, want write", fileOp.OperationType)
	}

	if fileOp.FilePath != "/test/file.txt" {
		t.Errorf("FilePath = %v, want /test/file.txt", fileOp.FilePath)
	}

	if !fileOp.Success {
		t.Error("Success should be true")
	}
}

// Test edge cases and validation
func TestConversationBlock_EdgeCases(t *testing.T) {
	// Test empty conversation block
	block := &ConversationBlock{}

	if block.ID != "" {
		t.Error("Empty block ID should be empty string")
	}

	if block.Status != ConversationStatus(0) {
		t.Error("Empty block status should be zero value")
	}
}

func TestTokenUsage_ZeroValues(t *testing.T) {
	usage := TokenUsage{}

	if usage.InputTokens != 0 {
		t.Error("Zero TokenUsage should have 0 input tokens")
	}

	if usage.OutputTokens != 0 {
		t.Error("Zero TokenUsage should have 0 output tokens")
	}
}

// Benchmark tests
func BenchmarkConversationStatus_String(b *testing.B) {
	status := StatusCompleted

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = status.String()
	}
}

func BenchmarkConversationBlock_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block := &ConversationBlock{
			ID:          "benchmark-id",
			UserInput:   "Benchmark input",
			LLMResponse: "Benchmark response",
			Timestamp:   time.Now(),
			Status:      StatusCompleted,
		}
		_ = block
	}
}