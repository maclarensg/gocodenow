package ui

import (
	"context"
	"gocodenow/internal/models"
	"gocodenow/internal/types"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// hasRunningTools checks if any conversation has running tools that need animation
func (m *Model) hasRunningTools() bool {
	blocks := m.conversations.GetBlocks()
	for _, block := range blocks {
		if block.Status == types.StatusExecuting {
			return true
		}
		// Check if any individual tools are still running
		for _, toolCall := range block.ToolCalls {
			hasResult := false
			for _, result := range block.ToolResults {
				if result.ToolCallID == toolCall.ID {
					hasResult = true
					break
				}
			}
			if !hasResult {
				return true // Tool is still running
			}
		}
	}
	return false
}

// scheduleAnimationTick creates a command to trigger the next animation frame
func (m *Model) scheduleAnimationTick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return AnimationTickMsg{Timestamp: t}
	})
}

// StartAnimation initiates animation for running tools
func (m *Model) StartAnimation() tea.Cmd {
	if m.hasRunningTools() {
		return m.scheduleAnimationTick()
	}
	return nil
}

// handleStreamingOutputUpdate processes streaming output updates
func (m *Model) handleStreamingOutputUpdate(msg StreamingOutputUpdateMsg) {
	blocks := m.conversations.GetBlocks()
	for i, block := range blocks {
		if block.ID == msg.ConversationID {
			// Find the tool result to update
			for j, result := range block.ToolResults {
				if result.ToolCallID == msg.ToolCallID {
					// Add the streaming chunk
					chunk := types.StreamingChunk{
						Timestamp: time.Now(),
						Data:      msg.ChunkData,
						Type:      msg.ChunkType,
					}
					
					// Initialize streaming output if needed
					if block.ToolResults[j].StreamingOutput == nil {
						block.ToolResults[j].StreamingOutput = make([]types.StreamingChunk, 0)
					}
					
					// Append the new chunk
					block.ToolResults[j].StreamingOutput = append(
						block.ToolResults[j].StreamingOutput, chunk)
					
					// Update streaming status
					block.ToolResults[j].IsStreaming = !msg.IsComplete
					block.ToolResults[j].StreamComplete = msg.IsComplete
					
					// If streaming is complete, consolidate result
					if msg.IsComplete {
						consolidatedResult := m.consolidateStreamingOutput(block.ToolResults[j].StreamingOutput)
						block.ToolResults[j].Result = consolidatedResult
					}
					
					// Update the block
					blocks[i] = block
					return
				}
			}
			
			// If no tool result exists yet, create one for streaming
			if !msg.IsComplete {
				newResult := types.ToolResult{
					ID:         "result-" + msg.ToolCallID,
					ToolCallID: msg.ToolCallID,
					Success:    false, // Will be updated when complete
					IsStreaming: true,
					StreamComplete: false,
					StreamingOutput: []types.StreamingChunk{{
						Timestamp: time.Now(),
						Data:      msg.ChunkData,
						Type:      msg.ChunkType,
					}},
					Timestamp: time.Now(),
				}
				block.ToolResults = append(block.ToolResults, newResult)
				blocks[i] = block
			}
			return
		}
	}
}

// consolidateStreamingOutput combines streaming chunks into a single result
func (m *Model) consolidateStreamingOutput(chunks []types.StreamingChunk) string {
	var result strings.Builder
	
	for _, chunk := range chunks {
		switch chunk.Type {
		case "stdout":
			result.WriteString(chunk.Data)
		case "stderr":
			result.WriteString("[ERR] " + chunk.Data)
		case "info":
			result.WriteString("[INFO] " + chunk.Data)
		case "error":
			result.WriteString("[ERROR] " + chunk.Data)
		default:
			result.WriteString(chunk.Data)
		}
	}
	
	return result.String()
}

// SetMessageProcessor sets the message processor for the model
func (m *Model) SetMessageProcessor(processor *MessageProcessor) {
	m.messageProcessor = processor
}

// GetConversationHistory returns the conversation history instance
func (m *Model) GetConversationHistory() *models.ConversationHistory {
	return m.conversations
}

// Message processing handlers

// handleStreamingContentUpdate processes streaming content updates from LLM
func (m *Model) handleStreamingContentUpdate(msg StreamingContentUpdateMsg) {
	// Find the conversation and update its LLM response
	conv, err := m.conversations.GetConversationByID(msg.ConversationID)
	if err == nil && conv != nil {
		// Append the new content to the existing response
		conv.LLMResponse += msg.Content
		
		// Update status if streaming is complete
		if msg.Complete {
			conv.Status = types.StatusCompleted
		}
	}
}

// handleToolExecutionComplete processes tool execution completion
func (m *Model) handleToolExecutionComplete(msg ToolExecutionCompleteMsg) {
	conv, err := m.conversations.GetConversationByID(msg.ConversationID)
	if err == nil && conv != nil {
		// Update tool results
		conv.ToolResults = msg.ToolResults
		conv.Status = types.StatusCompleted
		
		// Calculate total execution time
		var totalDuration time.Duration
		for _, result := range msg.ToolResults {
			totalDuration += result.Duration
		}
		conv.ExecutionTime = totalDuration
	}
}

// handleToolExecutionError processes tool execution errors
func (m *Model) handleToolExecutionError(msg ToolExecutionErrorMsg) {
	conv, err := m.conversations.GetConversationByID(msg.ConversationID)
	if err == nil && conv != nil {
		// Find the specific tool call and mark as failed
		for _, toolCall := range conv.ToolCalls {
			if toolCall.ID == msg.ToolCallID {
				// Create error result
				errorResult := types.ToolResult{
					ID:           "error-" + msg.ToolCallID,
					ToolCallID:   msg.ToolCallID,
					Success:      false,
					ErrorMessage: msg.Error.Error(),
					Timestamp:    time.Now(),
				}
				
				// Add to results if not already present
				found := false
				for j, result := range conv.ToolResults {
					if result.ToolCallID == msg.ToolCallID {
						conv.ToolResults[j] = errorResult
						found = true
						break
					}
				}
				if !found {
					conv.ToolResults = append(conv.ToolResults, errorResult)
				}
				
				break
			}
		}
		
		// Update conversation status
		if msg.Recoverable {
			conv.Status = types.StatusPending // Can be retried
		} else {
			conv.Status = types.StatusError // Failed permanently
		}
	}
}

// handleUserInterventionRequired processes user intervention requests
func (m *Model) handleUserInterventionRequired(msg UserInterventionRequiredMsg) (tea.Model, tea.Cmd) {
	// For now, create a simple intervention handler
	// In a full implementation, this would show a dialog or prompt
	
	switch msg.InterventionType {
	case "confirm":
		// Auto-approve for now - in real implementation, show confirmation dialog
		return m, func() tea.Msg {
			return UserInterventionResponseMsg{
				ConversationID: msg.ConversationID,
				ToolCallID:     msg.ToolCallID,
				Response:       "approved",
				Approved:       true,
			}
		}
		
	case "choose":
		// Auto-select first option for now - in real implementation, show choice dialog
		response := ""
		if len(msg.Options) > 0 {
			response = msg.Options[0]
		}
		
		return m, func() tea.Msg {
			return UserInterventionResponseMsg{
				ConversationID: msg.ConversationID,
				ToolCallID:     msg.ToolCallID,
				Response:       response,
				Approved:       true,
			}
		}
		
	default:
		// Default to approval
		return m, func() tea.Msg {
			return UserInterventionResponseMsg{
				ConversationID: msg.ConversationID,
				ToolCallID:     msg.ToolCallID,
				Response:       "approved",
				Approved:       true,
			}
		}
	}
}

// handleConversationStateUpdate processes conversation state updates
func (m *Model) handleConversationStateUpdate(msg ConversationStateUpdateMsg) {
	conv, err := m.conversations.GetConversationByID(msg.ConversationID)
	if err == nil && conv != nil {
		conv.Status = msg.Status
		
		if msg.ExecutionTime != nil {
			conv.ExecutionTime = *msg.ExecutionTime
		}
		
		if msg.TokenUsage != nil {
			conv.TokenUsage = *msg.TokenUsage
		}
	}
}

// isProcessingMessage returns true if a conversation is currently being processed
func (m *Model) isProcessingMessage(conversationID string) bool {
	return m.processingMessages[conversationID]
}

// handleUserInterventionResponse processes user responses to intervention requests
func (m *Model) handleUserInterventionResponse(msg UserInterventionResponseMsg) (tea.Model, tea.Cmd) {
	if !msg.Approved {
		// User declined or cancelled - mark tool as failed
		conv, err := m.conversations.GetConversationByID(msg.ConversationID)
		if err == nil && conv != nil {
			for i, result := range conv.ToolResults {
				if result.ToolCallID == msg.ToolCallID {
					conv.ToolResults[i].Success = false
					conv.ToolResults[i].ErrorMessage = "User cancelled operation"
					break
				}
			}
			conv.Status = types.StatusError
		}
		return m, nil
	}
	
	// User approved - handle based on response
	switch msg.Response {
	case "retry":
		// Retry the tool execution
		if m.messageProcessor != nil && m.messageProcessor.toolWorkflow != nil {
			cmd := m.messageProcessor.toolWorkflow.RetryToolCall(
				context.Background(),
				msg.ConversationID,
				msg.ToolCallID,
				nil, // Use original parameters
			)
			return m, cmd
		}
		
	case "skip":
		// Skip this tool and continue with others
		conv, err := m.conversations.GetConversationByID(msg.ConversationID)
		if err == nil && conv != nil {
			for i, result := range conv.ToolResults {
				if result.ToolCallID == msg.ToolCallID {
					conv.ToolResults[i].Success = false
					conv.ToolResults[i].ErrorMessage = "Skipped by user"
					conv.ToolResults[i].Result = "SKIPPED"
					break
				}
			}
		}
		
	case "abort":
		// Abort the entire conversation
		conv, err := m.conversations.GetConversationByID(msg.ConversationID)
		if err == nil && conv != nil {
			conv.Status = types.StatusError
		}
	}
	
	return m, nil
}