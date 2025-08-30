package ui

import (
	"context"
	"fmt"
	"gocodenow/internal/models"
	"gocodenow/internal/types"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// ConnectionStatus represents the connection state
type ConnectionStatus struct {
	Connected bool
	ModelName string
	Error     string
}

// ToolExecutionUpdateMsg represents real-time tool execution updates
type ToolExecutionUpdateMsg struct {
	ConversationID string
	ToolCallID     string
	Status         string // "running", "completed", "failed"
	Progress       string // Optional progress message
	Result         string // Tool result when completed
}

// ConversationStatusUpdateMsg represents conversation status changes
type ConversationStatusUpdateMsg struct {
	ConversationID string
	Status         string
	StatusIcon     string
}

// AnimationTickMsg triggers UI animation updates
type AnimationTickMsg struct {
	Timestamp time.Time
}

// StreamingOutputUpdateMsg represents real-time streaming output updates
type StreamingOutputUpdateMsg struct {
	ConversationID string
	ToolCallID     string
	ChunkData      string
	ChunkType      string // stdout, stderr, info, error
	IsComplete     bool   // True when streaming is complete
}


// Model represents the main TUI model
type Model struct {
	conversations       *models.ConversationHistory
	viewport            viewport.Model
	textarea            textarea.Model
	modelName           string
	status              string
	connected           bool
	connectionErr       string
	width               int
	height              int
	messageProcessor    *MessageProcessor
	processingMessages  map[string]bool // Track which conversations are being processed
	confirmationDialog  *ConfirmationDialog
	shortcutManager     *ShortcutManager
	tooltipManager      *TooltipManager
	onboardingFlow      *OnboardingFlow
	themeManager        *ThemeManager
	accessibilityMgr    *AccessibilityManager
	exportImportDialog  *ExportImportDialog
}

// New creates a new TUI model
func New(modelName string, connStatus ConnectionStatus) *Model {
	ta := textarea.New()
	ta.Placeholder = "Type your message here..."
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true

	// Set status based on connection
	status := "Ready"
	if !connStatus.Connected {
		if connStatus.Error != "" {
			status = connStatus.Error
		} else {
			status = "Not Connected"
		}
	}

	// Create directories for export and backup
	homeDir, _ := os.UserHomeDir()
	exportDir := filepath.Join(homeDir, ".lmcodenow", "exports")
	backupDir := filepath.Join(homeDir, ".lmcodenow", "backups")
	
	model := &Model{
		conversations:       models.NewConversationHistoryInMemory(), // Use in-memory version for now
		viewport:            vp,
		textarea:            ta,
		modelName:           connStatus.ModelName,
		status:              status,
		connected:           connStatus.Connected,
		connectionErr:       connStatus.Error,
		width:               80,
		height:              24,
		messageProcessor:    nil, // Will be set via SetMessageProcessor
		processingMessages:  make(map[string]bool),
		confirmationDialog:  nil, // Will be created when needed
		shortcutManager:     NewShortcutManager(),
		tooltipManager:      NewTooltipManager(),
		onboardingFlow:      NewOnboardingFlow(),
		themeManager:        createThemeManager(),
		accessibilityMgr:    NewAccessibilityManager(),
		exportImportDialog:  NewExportImportDialog(exportDir, backupDir),
	}
	
	// Initialize viewport content and scroll to show most recent conversations
	model.updateViewportContentAndScrollToBottom()
	
	// Start onboarding for new users (no existing conversations)
	if model.onboardingFlow != nil && model.onboardingFlow.ShouldShowForFirstTime(len(model.conversations.GetBlocks())) {
		model.onboardingFlow.Start()
	}
	
	return model
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width - 4)
		
		// Update viewport size - calculate available space for content
		headerHeight := 2 // Header + separator
		inputHeight := 6  // separator(1) + textarea(3) + newline(1) + instructions(1)
		viewportHeight := msg.Height - headerHeight - inputHeight
		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight
		
		// Update viewport content
		m.updateViewportContent()
		return m, nil

		// Smooth scroll messages removed for reliability

	case tea.KeyMsg:
		// Handle onboarding first if active (highest priority)
		if m.onboardingFlow != nil && m.onboardingFlow.IsActive() {
			onboardingCmd := m.onboardingFlow.HandleKeyMsg(msg)
			return m, onboardingCmd
		}
		
		// Handle confirmation dialog if active
		if m.confirmationDialog != nil && m.confirmationDialog.IsActive() {
			dialog, dialogCmd, result := m.confirmationDialog.Update(msg)
			m.confirmationDialog = dialog
			
			if result != "" {
				return m.handleConfirmationResult(result), tea.Batch(dialogCmd, cmd)
			}
			return m, tea.Batch(dialogCmd, cmd)
		}
		
		// Handle export/import dialog if active
		if m.exportImportDialog != nil && m.exportImportDialog.IsVisible() {
			m.exportImportDialog.Update(msg)
			return m, nil
		}
		
		// Handle tooltip dismissal if any are active
		if m.tooltipManager != nil && m.tooltipManager.HasActiveTooltips() {
			// Any key press dismisses tooltips (except special keys)
			keyStr := msg.String()
			if keyStr != "ctrl+c" && keyStr != "esc" {
				m.tooltipManager.DismissAllTooltips()
			}
		}
		
		// Handle accessibility shortcuts
		if m.accessibilityMgr != nil {
			accessibilityAction := m.accessibilityMgr.HandleAccessibilityKeyMsg(msg)
			if accessibilityAction != "" {
				return m.handleAccessibilityAction(accessibilityAction, msg)
			}
		}
		
		// Use shortcut manager for key handling
		context := m.getCurrentContext()
		action := m.shortcutManager.HandleKeyMsg(msg, context)
		
		if action != "" {
			return m.handleShortcutAction(action, msg)
		}
		
		return m.handleKeyMsg(msg)
	
	case ToolExecutionUpdateMsg:
		// Update the conversation with tool execution progress
		m.handleToolExecutionUpdate(msg)
		
		// Trigger tool execution tooltip on first tool execution
		if m.tooltipManager != nil && msg.Status == "running" {
			m.tooltipManager.TriggerTooltip("first_tool_execution")
		}
		
		m.updateViewportContent()
		return m, nil
		
	case ConversationStatusUpdateMsg:
		// Update conversation status and refresh display
		m.handleConversationStatusUpdate(msg)
		m.updateViewportContent()
		return m, nil
		
	case AnimationTickMsg:
		// Update animation frame and schedule next tick if needed
		if m.hasRunningTools() {
			m.updateViewportContent()
			return m, m.scheduleAnimationTick()
		}
		return m, nil
		
	case StreamingOutputUpdateMsg:
		// Handle streaming output updates
		m.handleStreamingOutputUpdate(msg)
		m.updateViewportContent()
		return m, nil
		
	// Message processing workflow handlers
	case MessageProcessingStartedMsg:
		m.processingMessages[msg.ConversationID] = true
		m.updateViewportContent()
		return m, msg.ProcessorCmd
		
	case MessageProcessingErrorMsg:
		delete(m.processingMessages, msg.ConversationID)
		// Update conversation status to error
		m.conversations.UpdateConversationStatus(msg.ConversationID, "error")
		m.updateViewportContent()
		// TODO: Show error message to user
		return m, nil
		
	case MessageProcessingCompleteMsg:
		delete(m.processingMessages, msg.ConversationID)
		m.updateViewportContent()
		return m, nil
		
	case LLMStreamingStartedMsg:
		return m, msg.ProcessorCmd
		
	case StreamingContentUpdateMsg:
		m.handleStreamingContentUpdate(msg)
		m.updateViewportContent()
		return m, nil
		
	case ToolExecutionStartedMsg:
		return m, msg.ProcessorCmd
		
	case ToolExecutionCompleteMsg:
		m.handleToolExecutionComplete(msg)
		m.updateViewportContent()
		return m, nil
		
	case ToolExecutionErrorMsg:
		m.handleToolExecutionError(msg)
		m.updateViewportContent()
		
		// Check if we need user intervention
		if msg.RequiresIntervention {
			return m.handleUserInterventionRequired(UserInterventionRequiredMsg{
				ConversationID:    msg.ConversationID,
				ToolCallID:        msg.ToolCallID,
				InterventionType:  "retry",
				Message:           msg.Error.Error(),
				Options:           []string{"retry", "skip", "abort"},
			})
		}
		return m, nil
		
	case UserInterventionRequiredMsg:
		return m.handleUserInterventionRequired(msg)
		
	case ConversationStateUpdateMsg:
		m.handleConversationStateUpdate(msg)
		m.updateViewportContent()
		return m, nil
		
	case RetryToolExecutionMsg:
		// Handle tool retry requests
		if m.messageProcessor != nil && m.messageProcessor.toolWorkflow != nil {
			cmd := m.messageProcessor.toolWorkflow.RetryToolCall(
				context.Background(), 
				msg.ConversationID, 
				msg.ToolCallID, 
				msg.NewParameters,
			)
			return m, cmd
		}
		return m, nil
		
	case UserInterventionResponseMsg:
		// Handle user intervention responses
		return m.handleUserInterventionResponse(msg)
		
	case OnboardingCompletedMsg:
		// Onboarding has been completed
		if m.tooltipManager != nil {
			m.tooltipManager.ShowTemporaryMessage(
				"Tutorial Complete! ✨",
				"You can now use lmcodenow effectively. Press ? anytime for help.",
				3*time.Second,
				TooltipTypeSuccess,
			)
		}
		return m, nil
		
	case OnboardingStepTimeoutMsg:
		// Handle onboarding step timeouts
		if m.onboardingFlow != nil {
			cmd := m.onboardingFlow.HandleTimeout(msg)
			return m, cmd
		}
		return m, nil
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Update textarea
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

// updateViewportContent updates the viewport while preserving scroll position
func (m *Model) updateViewportContent() {
	// Save current scroll position as percentage
	currentPercent := m.viewport.ScrollPercent()
	
	renderer := NewRenderer(m.width, m.height)
	content := renderer.RenderConversationsOnly(m)
	m.viewport.SetContent(content)
	
	// Restore relative scroll position using viewport's method
	if currentPercent > 0 {
		totalLines := len(strings.Split(content, "\n"))
		if totalLines > m.viewport.Height {
			targetLine := int(float64(totalLines-m.viewport.Height) * currentPercent)
			for i := 0; i < targetLine && i < totalLines-m.viewport.Height; i++ {
				m.viewport.LineDown(1)
			}
		}
	}
}

// updateViewportContentAndScrollToBottom updates content and scrolls to show latest
func (m *Model) updateViewportContentAndScrollToBottom() {
	// Update the content first
	renderer := NewRenderer(m.width, m.height)
	content := renderer.RenderConversationsOnly(m)
	m.viewport.SetContent(content)
	
	// Use the viewport's proper method to scroll to bottom
	m.viewport.GotoBottom()
}

// ensureSelectedVisible makes sure the selected conversation is visible in viewport
func (m *Model) ensureSelectedVisible() {
	m.updateViewportContent()
	
	selected := m.conversations.GetSelected()
	blocks := m.conversations.GetBlocks()
	
	if len(blocks) == 0 || selected < 0 || selected >= len(blocks) {
		return
	}
	
	// Estimate where the selected block is in the rendered content
	// Each block is roughly 6-8 lines when collapsed, more when expanded
	estimatedLinesPerBlock := 8
	selectedLineStart := selected * estimatedLinesPerBlock
	selectedLineEnd := selectedLineStart + estimatedLinesPerBlock
	
	// Check if selected item is above the viewport (need to scroll up)
	if selectedLineStart < m.viewport.YOffset {
		// Scroll up to show the selected item at the top
		linesToScrollUp := m.viewport.YOffset - selectedLineStart
		for i := 0; i < linesToScrollUp; i++ {
			m.viewport.LineUp(1)
		}
	}
	
	// Check if selected item is below the viewport (need to scroll down)
	viewportBottom := m.viewport.YOffset + m.viewport.Height
	if selectedLineEnd > viewportBottom {
		// Scroll down to show the selected item
		linesToScrollDown := selectedLineEnd - viewportBottom
		for i := 0; i < linesToScrollDown; i++ {
			m.viewport.LineDown(1)
		}
	}
}

// Smooth scroll method removed for simpler, more reliable approach

// View renders the model
func (m *Model) View() string {
	renderer := NewRenderer(m.width, m.height)
	view := renderer.Render(m)
	
	// Handle tooltips first (they should appear behind dialogs)
	if m.tooltipManager != nil && m.tooltipManager.HasActiveTooltips() {
		tooltipsView := m.tooltipManager.RenderTooltips(m.width, m.height)
		if tooltipsView != "" {
			// Simple overlay approach - tooltips appear at bottom
			view = view + "\n" + tooltipsView
		}
	}
	
	// Overlay onboarding if active (highest priority)
	if m.onboardingFlow != nil && m.onboardingFlow.IsActive() {
		onboardingView := m.onboardingFlow.Render(m.width, m.height)
		return onboardingView
	}
	
	// Overlay confirmation dialog if active
	if m.confirmationDialog != nil && m.confirmationDialog.IsActive() {
		m.confirmationDialog.SetSize(m.width, m.height)
		dialogView := m.confirmationDialog.View()
		// Simple overlay - in a real implementation you'd want proper layering
		return dialogView
	}
	
	// Overlay export/import dialog if active
	if m.exportImportDialog != nil && m.exportImportDialog.IsVisible() {
		m.exportImportDialog.SetSize(m.width, m.height)
		dialogView := m.exportImportDialog.View()
		return dialogView
	}
	
	// Overlay help panel if visible
	if m.shortcutManager.IsHelpVisible() {
		helpView := m.shortcutManager.RenderHelp(m.width, m.height)
		return helpView
	}
	
	// Add accessibility indicators if any are active
	if m.accessibilityMgr != nil {
		indicators := m.accessibilityMgr.RenderAccessibilityIndicators()
		if indicators != "" {
			view = indicators + "\n" + view
		}
	}
	
	return view
}

// getCurrentContext determines the current UI context for shortcut handling
func (m *Model) getCurrentContext() string {
	var context string
	if m.confirmationDialog != nil && m.confirmationDialog.IsActive() {
		context = "dialog"
	} else if m.textarea.Focused() {
		context = "input"
	} else {
		// Check if there are conversations
		blocks := m.conversations.GetBlocks()
		if len(blocks) == 0 {
			context = "empty"
		} else {
			context = "conversation"
		}
	}
	
	// Update tooltip manager context
	if m.tooltipManager != nil {
		m.tooltipManager.UpdateContext(context)
	}
	
	// Update accessibility manager focus
	if m.accessibilityMgr != nil {
		m.accessibilityMgr.SetFocus(context)
	}
	
	return context
}

// handleShortcutAction processes shortcut actions
func (m *Model) handleShortcutAction(action string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Announce action for accessibility
	if m.accessibilityMgr != nil {
		context := m.getCurrentContext()
		m.accessibilityMgr.AnnounceAction(action, context)
	}
	
	switch action {
	case "show_help":
		// Help is handled by shortcut manager, just update display
		return m, nil
		
	case "quit":
		return m, tea.Quit
		
	case "force_quit":
		return m, tea.Quit
		
	case "toggle_focus":
		if m.textarea.Focused() {
			m.textarea.Blur()
		} else {
			m.textarea.Focus()
			// Trigger input help tooltip on first focus
			if m.tooltipManager != nil {
				m.tooltipManager.TriggerTooltip("first_input_focus")
			}
		}
		m.updateViewportContent()
		return m, nil
		
	case "delete_conversation":
		// Show confirmation dialog for destructive operation
		selected := m.conversations.GetSelected()
		blocks := m.conversations.GetBlocks()
		if selected >= 0 && selected < len(blocks) {
			m.confirmationDialog = ConfirmDelete(
				"conversation", 
				"This will permanently remove the conversation and all its messages.",
			)
			m.confirmationDialog.Show()
			
			// Trigger confirmation help tooltip
			if m.tooltipManager != nil {
				m.tooltipManager.TriggerTooltip("first_confirmation")
			}
		}
		return m, nil
		
	case "clear_all_conversations":
		// Show confirmation dialog for destructive operation
		m.confirmationDialog = ConfirmClear("conversations")
		m.confirmationDialog.Show()
		return m, nil
		
	// Navigation actions - delegate to original key handler for now
	case "navigate_up", "navigate_down", "navigate_home", "navigate_end":
		return m.handleKeyMsg(msg)
		
	case "send_message", "new_line", "clear_input":
		return m.handleKeyMsg(msg)
		
	case "toggle_expanded":
		m.conversations.ToggleExpanded()
		m.updateViewportContent()
		return m, nil
		
	case "next_theme":
		if m.themeManager != nil {
			availableThemes := m.themeManager.GetAvailableThemes()
			if len(availableThemes) > 1 {
				currentTheme := m.themeManager.GetCurrentTheme()
				currentIndex := 0
				
				// Find current theme index
				for i, themeName := range availableThemes {
					if currentTheme != nil && themeName == currentTheme.Colors.Name {
						currentIndex = i
						break
					}
				}
				
				// Switch to next theme
				nextIndex := (currentIndex + 1) % len(availableThemes)
				nextTheme := availableThemes[nextIndex]
				m.themeManager.SetTheme(nextTheme)
				
				// Show confirmation
				if m.tooltipManager != nil {
					m.tooltipManager.ShowTemporaryMessage(
						"Theme Changed",
						fmt.Sprintf("Switched to '%s' theme", nextTheme),
						2*time.Second,
						TooltipTypeInfo,
					)
				}
			}
		}
		return m, nil
		
	case "prev_theme":
		if m.themeManager != nil {
			availableThemes := m.themeManager.GetAvailableThemes()
			if len(availableThemes) > 1 {
				currentTheme := m.themeManager.GetCurrentTheme()
				currentIndex := 0
				
				// Find current theme index
				for i, themeName := range availableThemes {
					if currentTheme != nil && themeName == currentTheme.Colors.Name {
						currentIndex = i
						break
					}
				}
				
				// Switch to previous theme
				prevIndex := (currentIndex - 1 + len(availableThemes)) % len(availableThemes)
				prevTheme := availableThemes[prevIndex]
				m.themeManager.SetTheme(prevTheme)
				
				// Show confirmation
				if m.tooltipManager != nil {
					m.tooltipManager.ShowTemporaryMessage(
						"Theme Changed",
						fmt.Sprintf("Switched to '%s' theme", prevTheme),
						2*time.Second,
						TooltipTypeInfo,
					)
				}
			}
		}
		return m, nil
		
	// Export/Import actions
	case "export_conversations":
		if m.exportImportDialog != nil {
			m.exportImportDialog.ShowExport()
		}
		return m, nil
		
	case "import_conversations":
		if m.exportImportDialog != nil {
			m.exportImportDialog.ShowImport()
		}
		return m, nil
		
	case "backup_conversations":
		if m.exportImportDialog != nil {
			m.exportImportDialog.ShowBackup()
		}
		return m, nil
		
	case "restore_conversations":
		if m.exportImportDialog != nil {
			m.exportImportDialog.ShowRestore()
		}
		return m, nil
		
	case "filter_conversations":
		if m.exportImportDialog != nil {
			m.exportImportDialog.ShowFilters()
		}
		return m, nil
		
	case "toggle_compact":
		if m.themeManager != nil {
			currentTheme := m.themeManager.GetCurrentTheme()
			if currentTheme != nil {
				// Toggle compact mode
				currentTheme.Layout.CompactMode = !currentTheme.Layout.CompactMode
				
				// Recompute styles with new settings
				m.themeManager.computeStyles(currentTheme)
				
				// Show confirmation
				if m.tooltipManager != nil {
					status := "disabled"
					if currentTheme.Layout.CompactMode {
						status = "enabled"
					}
					m.tooltipManager.ShowTemporaryMessage(
						"Layout Changed",
						fmt.Sprintf("Compact mode %s", status),
						2*time.Second,
						TooltipTypeInfo,
					)
				}
			}
		}
		m.updateViewportContent()
		return m, nil
		
	default:
		// For unhandled actions, fall back to original key handler
		return m.handleKeyMsg(msg)
	}
}

// handleConfirmationResult processes confirmation dialog results
func (m *Model) handleConfirmationResult(result ConfirmationResult) tea.Model {
	if result == ResultConfirm && m.confirmationDialog != nil {
		switch m.confirmationDialog.Type {
		case ConfirmationTypeDelete:
			// Delete the selected conversation
			selected := m.conversations.GetSelected()
			if selected >= 0 {
				blocks := m.conversations.GetBlocks()
				if selected < len(blocks) {
					// Remove the conversation block
					newBlocks := make([]types.ConversationBlock, 0, len(blocks)-1)
					newBlocks = append(newBlocks, blocks[:selected]...)
					newBlocks = append(newBlocks, blocks[selected+1:]...)
					
					// Update selection to stay within bounds
					if selected >= len(newBlocks) && len(newBlocks) > 0 {
						m.conversations.SetSelected(len(newBlocks) - 1)
					}
					
					m.updateViewportContent()
				}
			}
			
		case ConfirmationTypeClear:
			// Clear all conversations
			m.conversations = models.NewConversationHistoryInMemory()
			m.updateViewportContentAndScrollToBottom()
		}
	}
	
	// Hide the dialog
	m.confirmationDialog = nil
	return m
}

// createThemeManager creates a theme manager with default config directory
func createThemeManager() *ThemeManager {
	// Try to get config directory from environment or use default
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "lmcodenow")
	
	// Create config directory if it doesn't exist
	os.MkdirAll(configDir, 0755)
	
	return NewThemeManager(configDir)
}

// handleAccessibilityAction processes accessibility-related actions
func (m *Model) handleAccessibilityAction(action string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch action {
	case "toggle_high_contrast":
		if m.themeManager != nil {
			// Apply high contrast to current theme
			currentTheme := m.themeManager.GetCurrentTheme()
			if currentTheme != nil {
				currentTheme.Accessibility.HighContrast = !currentTheme.Accessibility.HighContrast
				m.themeManager.ApplyAccessibilitySettings(currentTheme.Accessibility)
			}
		}
		return m, nil
		
	case "toggle_reduced_motion":
		if m.themeManager != nil {
			currentTheme := m.themeManager.GetCurrentTheme()
			if currentTheme != nil {
				currentTheme.Accessibility.ReduceMotion = !currentTheme.Accessibility.ReduceMotion
				m.themeManager.ApplyAccessibilitySettings(currentTheme.Accessibility)
			}
		}
		return m, nil
		
	case "toggle_large_text":
		if m.themeManager != nil {
			currentTheme := m.themeManager.GetCurrentTheme()
			if currentTheme != nil {
				currentTheme.Accessibility.LargeFonts = !currentTheme.Accessibility.LargeFonts
				m.themeManager.ApplyAccessibilitySettings(currentTheme.Accessibility)
			}
		}
		return m, nil
		
	case "toggle_screen_reader":
		if m.themeManager != nil {
			currentTheme := m.themeManager.GetCurrentTheme()
			if currentTheme != nil {
				currentTheme.Accessibility.ScreenReaderMode = !currentTheme.Accessibility.ScreenReaderMode
				m.themeManager.ApplyAccessibilitySettings(currentTheme.Accessibility)
			}
		}
		return m, nil
		
	case "announce_context":
		if m.accessibilityMgr != nil {
			context := m.getCurrentContext()
			contextInfo := map[string]interface{}{
				"conversation_count": len(m.conversations.GetBlocks()),
				"selected":          m.conversations.GetSelected(),
				"focused":           m.textarea.Focused(),
			}
			description := m.accessibilityMgr.GetAccessibleDescription(context+"_area", contextInfo)
			m.accessibilityMgr.AnnounceText(description, PriorityMedium)
		}
		return m, nil
		
	case "announce_focus":
		// Already handled in accessibility manager
		return m, nil
		
	default:
		return m, nil
	}
}

// handleToolExecutionUpdate processes tool execution progress updates
func (m *Model) handleToolExecutionUpdate(msg ToolExecutionUpdateMsg) {
	blocks := m.conversations.GetBlocks()
	for i, block := range blocks {
		if block.ID == msg.ConversationID {
			// Update the tool call status
			for _, toolCall := range block.ToolCalls {
				if toolCall.ID == msg.ToolCallID {
					// Find or create the corresponding tool result
					resultFound := false
					for k, result := range block.ToolResults {
						if result.ToolCallID == msg.ToolCallID {
							// Update existing result
							switch msg.Status {
							case "running":
								block.ToolResults[k].Success = false
								block.ToolResults[k].Result = msg.Progress
							case "completed":
								block.ToolResults[k].Success = true
								block.ToolResults[k].Result = msg.Result
							case "failed":
								block.ToolResults[k].Success = false
								block.ToolResults[k].ErrorMessage = msg.Result
							}
							resultFound = true
							break
						}
					}
					
					// Create new tool result if not found
					if !resultFound && msg.Status != "running" {
						newResult := types.ToolResult{
							ID:         "result-" + toolCall.ID,
							ToolCallID: msg.ToolCallID,
							Success:    msg.Status == "completed",
							Result:     msg.Result,
							Timestamp:  time.Now(),
						}
						if msg.Status == "failed" {
							newResult.ErrorMessage = msg.Result
							newResult.Result = ""
						}
						block.ToolResults = append(block.ToolResults, newResult)
					}
					break
				}
			}
			
			// Update the block in the conversation history
			blocks[i] = block
			break
		}
	}
}

// handleConversationStatusUpdate processes conversation status changes
func (m *Model) handleConversationStatusUpdate(msg ConversationStatusUpdateMsg) {
	blocks := m.conversations.GetBlocks()
	for i, block := range blocks {
		if block.ID == msg.ConversationID {
			// Update conversation status based on the message
			switch msg.Status {
			case "pending":
				block.Status = types.StatusPending
			case "executing":
				block.Status = types.StatusExecuting
			case "completed":
				block.Status = types.StatusCompleted
			case "error":
				block.Status = types.StatusError
			}
			
			// Update the block in the conversation history
			blocks[i] = block
			break
		}
	}
}