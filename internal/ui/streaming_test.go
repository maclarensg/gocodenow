package ui

import (
	"context"
	"testing"
	"time"

	"gocodenow/internal/types"
)

func TestStreamingAnimation(t *testing.T) {
	// Create a model with in-memory conversation history
	model := New("test-model", ConnectionStatus{
		Connected: true,
		ModelName: "test-model",
		Error:     "",
	})

	// Add a conversation with executing status
	convID, err := model.conversations.AddConversation("Test input", "", types.StatusExecuting)
	if err != nil {
		t.Fatalf("Failed to add conversation: %v", err)
	}

	// Mark as processing
	model.processingMessages[convID] = true

	// Test that hasRunningTools returns true
	if !model.hasRunningTools() {
		t.Error("Expected hasRunningTools to return true for executing conversation")
	}

	// Test that animation tick is scheduled
	tick := model.scheduleAnimationTick()
	if tick == nil {
		t.Error("Expected animation tick to be scheduled")
	}

	// Execute the tick command
	msg := tick()
	if _, ok := msg.(AnimationTickMsg); !ok {
		t.Errorf("Expected AnimationTickMsg, got %T", msg)
	}

	// Update with the animation message
	updatedModel, cmd := model.Update(msg)
	if cmd == nil {
		t.Error("Expected animation to schedule next tick")
	}

	// Verify model is updated
	if updatedModel == nil {
		t.Error("Expected updated model")
	}
}

func TestStreamingCancellation(t *testing.T) {
	// Create a model
	model := New("test-model", ConnectionStatus{
		Connected: true,
		ModelName: "test-model",
		Error:     "",
	})

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	convID := "test-conv-123"

	// Track the cancellation
	model.cancelFunctions[convID] = cancel
	model.processingMessages[convID] = true

	// Add the conversation
	_, err := model.conversations.AddConversation("Test input", "", types.StatusExecuting)
	if err != nil {
		t.Fatalf("Failed to add conversation: %v", err)
	}

	// Test single ESC (should not cancel)
	model, _ = model.handleEscapeKey()
	if len(model.cancelFunctions) == 0 {
		t.Error("Single ESC should not cancel streams")
	}

	// Test double ESC within timeout (should cancel)
	model.lastEscapeTime = time.Now().Add(-100 * time.Millisecond) // Set recent escape time
	model, _ = model.handleEscapeKey()

	// Should have cancelled and cleaned up
	if len(model.cancelFunctions) != 0 {
		t.Error("Double ESC should cancel all streams")
	}
	if len(model.processingMessages) != 0 {
		t.Error("Double ESC should clean up processing messages")
	}

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		// Good, context was cancelled
	default:
		t.Error("Expected context to be cancelled")
	}
}

func TestMessageProcessingWorkflow(t *testing.T) {
	// Create a model with a mock conversation history
	model := New("test-model", ConnectionStatus{
		Connected: true,
		ModelName: "test-model",
		Error:     "",
	})

	// Test MessageProcessingStartedMsg handling
	convID := "test-conv-456"
	startMsg := MessageProcessingStartedMsg{
		ConversationID: convID,
		ProcessorCmd:   nil,
	}

	// Add a temp cancellation function
	_, cancel := context.WithCancel(context.Background())
	tempID := "temp_123456"
	model.cancelFunctions[tempID] = cancel

	// Process the start message
	updatedModel, _ := model.Update(startMsg)
	model = updatedModel.(*Model)

	// Check that processing is tracked
	if !model.processingMessages[convID] {
		t.Error("Expected conversation to be marked as processing")
	}

	// Check that cancellation was moved from temp to real ID
	if _, exists := model.cancelFunctions[tempID]; exists {
		t.Error("Expected temp cancellation to be removed")
	}
	if _, exists := model.cancelFunctions[convID]; !exists {
		t.Error("Expected real conversation ID to have cancellation function")
	}

	// Test MessageProcessingCompleteMsg handling
	completeMsg := MessageProcessingCompleteMsg{
		ConversationID: convID,
		FinalResponse:  "Test response",
	}

	updatedModel, _ = model.Update(completeMsg)
	model = updatedModel.(*Model)

	// Check that processing and cancellation are cleaned up
	if model.processingMessages[convID] {
		t.Error("Expected conversation processing to be cleaned up")
	}
	if _, exists := model.cancelFunctions[convID]; exists {
		t.Error("Expected cancellation function to be cleaned up")
	}
}

func TestStatusIconAnimation(t *testing.T) {
	renderer := &Renderer{}

	// Test different status icons
	testCases := []struct {
		status       types.ConversationStatus
		expectSpinner bool
	}{
		{types.StatusPending, false},
		{types.StatusExecuting, true},
		{types.StatusCompleted, false},
		{types.StatusError, false},
		{types.StatusConfigError, false},
	}

	for _, tc := range testCases {
		icon := renderer.getStatusIcon(tc.status)
		
		if tc.expectSpinner {
			// For executing status, should return spinner characters
			spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			found := false
			for _, char := range spinnerChars {
				if icon == char {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected executing status to return spinner character, got %s", icon)
			}
		} else {
			// For non-executing status, should not be a spinner character
			spinnerChars := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			for _, char := range spinnerChars {
				if icon == char {
					t.Errorf("Expected non-executing status %v to not return spinner character %s", tc.status, icon)
				}
			}
		}
	}
}

func TestCancellationCleanup(t *testing.T) {
	model := New("test-model", ConnectionStatus{
		Connected: true,
		ModelName: "test-model",
		Error:     "",
	})

	// Add multiple conversations being processed
	convIDs := []string{"conv1", "conv2", "conv3"}
	contexts := make([]context.Context, len(convIDs))
	
	for i, convID := range convIDs {
		ctx, cancel := context.WithCancel(context.Background())
		contexts[i] = ctx
		model.cancelFunctions[convID] = cancel
		model.processingMessages[convID] = true
	}

	// Cancel all streams
	model, _ = model.cancelAllStreams()

	// Verify all are cleaned up
	if len(model.cancelFunctions) != 0 {
		t.Error("Expected all cancellation functions to be cleaned up")
	}
	if len(model.processingMessages) != 0 {
		t.Error("Expected all processing messages to be cleaned up")
	}

	// Verify all contexts are cancelled
	for i, ctx := range contexts {
		select {
		case <-ctx.Done():
			// Good
		default:
			t.Errorf("Expected context %d to be cancelled", i)
		}
	}
}