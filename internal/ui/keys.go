package ui

import (
	"context"
	"fmt"
	"time"

	"gocodenow/internal/types"
	tea "github.com/charmbracelet/bubbletea"
)

// handleKeyMsg handles keyboard input messages
func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
		
	case "esc":
		// Handle ESC for cancellation - need to track double-tap
		return m.handleEscapeKey()

	case "ctrl+j", "ctrl+enter":
		// Insert new line in textarea (reliable cross-terminal)
		if m.textarea.Focused() {
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'\n'},
			})
			return m, cmd
		}

	case "enter":
		// If textarea is focused and has content, send the message
		if m.textarea.Focused() {
			inputText := m.textarea.Value()
			if inputText != "" {
				// Clear the textarea immediately
				m.textarea.Reset()
				
				// Process the message using the message processor
				if m.messageProcessor != nil {
					// Create cancellable context for this message
					ctx, cancel := context.WithCancel(context.Background())
					
					// Generate a temporary conversation ID to track cancellation
					// (the real ID will be created by the processor)
					tempID := fmt.Sprintf("temp_%d", time.Now().UnixNano())
					m.cancelFunctions[tempID] = cancel
					
					cmd := m.messageProcessor.ProcessMessage(ctx, inputText, m.modelName)
					return m, cmd
				} else {
					// Fallback for when message processor is not available
					errorMsg := "Message processor not available. Please check configuration."
					if _, err := m.conversations.AddConversation(inputText, errorMsg, types.StatusConfigError); err != nil {
						return m, nil
					}
					m.updateViewportContentAndScrollToBottom()
					return m, nil
				}
			}
			return m, nil
		} else {
			// Toggle conversation expansion
			m.conversations.ToggleExpanded()
			// Update viewport content but preserve scroll position
			m.updateViewportContent()
			return m, nil
		}

	case "tab":
		// Toggle focus between conversations and textarea
		if m.textarea.Focused() {
			m.textarea.Blur()
		} else {
			m.textarea.Focus()
		}
		// Update viewport content to show selection highlighting
		m.updateViewportContent()
		return m, nil

	case "up":
		// If textarea is not focused, navigate conversations
		if !m.textarea.Focused() {
			m.conversations.SelectUp()
			// Ensure selected item is visible and highlighted
			m.ensureSelectedVisible()
			return m, nil
		}

	case "down":
		// If textarea is not focused, navigate conversations  
		if !m.textarea.Focused() {
			m.conversations.SelectDown()
			// Ensure selected item is visible and highlighted
			m.ensureSelectedVisible()
			return m, nil
		}

	// PgUp/PgDown removed - use up/down arrows for navigation

	case "home", "ctrl+home":
		if !m.textarea.Focused() {
			m.conversations.SetSelected(0)
			m.viewport.GotoTop()
			m.ensureSelectedVisible()
			return m, nil
		}

	case "end", "ctrl+end":
		if !m.textarea.Focused() {
			blocks := m.conversations.GetBlocks()
			if len(blocks) > 0 {
				m.conversations.SetSelected(len(blocks) - 1)
			}
			// Use our reliable scroll to bottom method
			m.updateViewportContentAndScrollToBottom()
			return m, nil
		}
	}

	// Let textarea handle all other key events when focused
	if m.textarea.Focused() {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleEscapeKey handles escape key for cancelling streaming responses
func (m *Model) handleEscapeKey() (*Model, tea.Cmd) {
	now := time.Now()
	
	// Check if this is a double-tap (within 500ms)
	if now.Sub(m.lastEscapeTime) < 500*time.Millisecond {
		// Double-tap detected - cancel all running streams
		return m.cancelAllStreams()
	}
	
	// Single tap - just record the time
	m.lastEscapeTime = now
	return m, nil
}

// cancelAllStreams cancels all currently running streaming operations
func (m *Model) cancelAllStreams() (*Model, tea.Cmd) {
	cancelCount := 0
	
	// Cancel all active streams
	for convID, cancelFunc := range m.cancelFunctions {
		// Cancel the context
		cancelFunc()
		
		// Update conversation status to cancelled (using error status for now)
		m.conversations.UpdateConversationStatus(convID, "error")
		if conv, err := m.conversations.GetConversationByID(convID); err == nil && conv != nil {
			// Update with cancellation message
			m.conversations.UpdateConversationResponse(convID, "Cancelled by user", conv.TokenUsage)
		}
		
		// Clean up tracking
		delete(m.cancelFunctions, convID)
		delete(m.processingMessages, convID)
		cancelCount++
	}
	
	if cancelCount > 0 {
		// Update viewport to show cancellation
		m.updateViewportContent()
		// TODO: Show a brief "Cancelled X operations" message
	}
	
	return m, nil
}