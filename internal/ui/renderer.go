package ui

import (
	"fmt"
	"strings"
	"time"

	"gocodenow/internal/types"

	"github.com/charmbracelet/lipgloss"
)

// Renderer handles the visual rendering of the TUI
type Renderer struct {
	width  int
	height int
}

// NewRenderer creates a new renderer
func NewRenderer(width, height int) *Renderer {
	return &Renderer{
		width:  width,
		height: height,
	}
}

// Render renders the complete UI using proper Bubble Tea layout patterns
func (r *Renderer) Render(model *Model) string {
	// Render fixed header
	header := r.renderHeader(model)
	
	// Render fixed input area (footer)
	inputArea := r.renderInput(model)
	
	// Use viewport for conversations (viewport handles its own content)
	conversationArea := model.viewport.View()
	
	// Use lipgloss to create the final layout with proper positioning
	return lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		conversationArea,
		inputArea,
	)
}

// RenderConversationsOnly renders just the conversation content for the viewport
func (r *Renderer) RenderConversationsOnly(model *Model) string {
	return r.renderConversations(model, 0) // Height not needed for viewport content
}

// renderHeader renders the fixed header bar that adjusts to screen width
func (r *Renderer) renderHeader(model *Model) string {
	// Choose header colors based on connection status
	var headerStyle lipgloss.Style
	if model.connected {
		// Green/blue for connected
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("4")).
			Padding(0, 1).
			Width(r.width).
			Align(lipgloss.Left)
	} else {
		// Red for connection failure
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("1")). // Red background
			Padding(0, 1).
			Width(r.width).
			Align(lipgloss.Left)
	}

	header := fmt.Sprintf("lmcodenow v1.0 | Model: %s | %s", model.modelName, model.status)
	
	// Add a separator line under the header
	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Width(r.width)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(header),
		separatorStyle.Render(strings.Repeat("─", r.width)),
	)
}

// renderConversations renders all conversation blocks (viewport handles scrolling)
func (r *Renderer) renderConversations(model *Model, maxHeight int) string {
	var s strings.Builder

	blockStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(r.width - 4)

	selectedStyle := blockStyle.Copy().
		BorderForeground(lipgloss.Color("4"))

	allBlocks := model.conversations.GetBlocks()
	selected := model.conversations.GetSelected()

	// Show empty message if no conversations
	if len(allBlocks) == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true).
			Width(r.width).
			Align(lipgloss.Center).
			Render("No conversations yet. Press Tab to focus input and start chatting!")
		s.WriteString(emptyMsg)
		return s.String()
	}

	// Render ALL blocks - let viewport handle what's visible
	for i, conv := range allBlocks {
		if i == selected && !model.textarea.Focused() {
			s.WriteString(selectedStyle.Render(r.renderConversationBlock(conv, i == selected)))
		} else {
			s.WriteString(blockStyle.Render(r.renderConversationBlock(conv, i == selected)))
		}
		s.WriteString("\n\n")
	}

	return s.String()
}

// renderConversationBlock renders a single conversation block with real-time tool execution status
func (r *Renderer) renderConversationBlock(conv types.ConversationBlock, isSelected bool) string {
	var s strings.Builder

	timeStr := conv.Timestamp.Format("15:04")
	expandIcon := "▶"
	if conv.Expanded {
		expandIcon = "▼"
	}

	titleStyle := lipgloss.NewStyle().Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Get primary input/response content (supporting both new and legacy fields)
	userInput := conv.UserInput
	if userInput == "" {
		userInput = conv.User // Fallback to legacy field
	}
	llmResponse := conv.LLMResponse
	if llmResponse == "" {
		llmResponse = conv.Assistant // Fallback to legacy field
	}

	if conv.Expanded {
		// Expanded view with enhanced tool execution display
		title := fmt.Sprintf("%s User: %s", expandIcon, r.truncateText(userInput, 1))
		statusIcon := r.getStatusIcon(conv.Status)
		
		s.WriteString(titleStyle.Render(title))
		s.WriteString(timeStyle.Render(fmt.Sprintf(" [%s] %s", timeStr, statusIcon)))
		s.WriteString("\n\n")
		
		// REQUEST DETAILS section
		requestBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("6")).
			Padding(0, 1).
			Width(r.width - 12)
		
		requestTitle := lipgloss.NewStyle().Bold(true).Render("┌─ REQUEST DETAILS & PERFORMANCE")
		requestContent := r.buildPerformanceMetrics(conv, userInput)
		
		s.WriteString("   " + requestBoxStyle.Render(requestTitle + "\n" + requestContent))
		s.WriteString("\n\n")

		// TOOL EXECUTION HISTORY section (comprehensive execution timeline)
		if len(conv.ToolCalls) > 0 {
			s.WriteString(r.renderToolExecutionHistorySection(conv, requestBoxStyle))
			s.WriteString("\n\n")
		} else if len(conv.Tools) > 0 {
			// Legacy support for old Tools field
			s.WriteString(r.renderLegacyActionsSection(conv, requestBoxStyle))
			s.WriteString("\n\n")
		}

		// RESPONSE section
		responseTitle := lipgloss.NewStyle().Bold(true).Render("┌─ RESPONSE")
		s.WriteString("   " + requestBoxStyle.Render(responseTitle + "\n" + llmResponse))
	} else {
		// Collapsed view with status indicator
		inputPreview := r.truncateText(userInput, 3)
		statusIcon := r.getStatusIcon(conv.Status)
		title := fmt.Sprintf("%s %s Input: %s", expandIcon, statusIcon, inputPreview)
		s.WriteString(titleStyle.Render(title))
		s.WriteString(timeStyle.Render(fmt.Sprintf(" [%s]", timeStr)))
		s.WriteString("\n")

		// Show truncated response with tool count
		responsePreview := r.truncateText(llmResponse, 2)
		if len(conv.ToolCalls) > 0 {
			toolCount := len(conv.ToolCalls)
			runningCount := r.countRunningTools(conv)
			if runningCount > 0 {
				responsePreview = fmt.Sprintf("[%d tools running] %s", runningCount, responsePreview)
			} else {
				responsePreview = fmt.Sprintf("[%d tools] %s", toolCount, responsePreview)
			}
		}
		
		previewStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Italic(true)

		s.WriteString(previewStyle.Render("   " + responsePreview))
	}

	return s.String()
}

// truncateText truncates text to specified number of lines
func (r *Renderer) truncateText(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	
	if len(lines) <= maxLines {
		return text
	}
	
	// Take first maxLines and add ellipsis
	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + "..."
}

// renderInput renders the input area with proper responsive width and feedback
func (r *Renderer) renderInput(model *Model) string {
	// Create a container for the input area that adjusts to screen width
	inputStyle := lipgloss.NewStyle().
		Width(r.width)

	var s strings.Builder

	// Add a separator line above input
	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Width(r.width)
	s.WriteString(separatorStyle.Render(strings.Repeat("─", r.width)))
	s.WriteString("\n")

	// Message sent feedback removed for simpler UX

	// Render the textarea
	s.WriteString(model.textarea.View())
	s.WriteString("\n")

	// Add instruction line with proper width handling
	promptStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Width(r.width).
		Align(lipgloss.Right)

	// Get contextual help from shortcut manager
	var instructionText string
	if model.shortcutManager != nil {
		context := "global"
		if model.textarea.Focused() {
			context = "input"
		} else {
			context = "conversation"
		}
		instructionText = model.shortcutManager.GetQuickHelp(context)
	} else {
		// Fallback to original instruction text
		instructionText = "[Send: Enter, New line: Ctrl+Enter, Tab: Focus, ↑↓: Navigate, Home/End: Top/Bottom]"
	}
	
	// Truncate instruction text if it's too long for the screen
	if len(instructionText) > r.width {
		instructionText = instructionText[:r.width-3] + "..."
	}
	
	s.WriteString(promptStyle.Render(instructionText))

	// Return the content wrapped in the styled input container
	return inputStyle.Render(s.String())
}

// getStatusIcon returns an appropriate icon for the conversation status
func (r *Renderer) getStatusIcon(status types.ConversationStatus) string {
	switch status {
	case types.StatusPending:
		return "⏳" // Hourglass for pending
	case types.StatusExecuting:
		return r.getAnimatedSpinner() // Animated spinner for executing
	case types.StatusCompleted:
		return "✅" // Green checkmark for completed
	case types.StatusError:
		return "❌" // Red X for error
	default:
		return "❓" // Question mark for unknown
	}
}

// getToolResultStatusIcon returns status icons specific to tool results
func (r *Renderer) getToolResultStatusIcon(result *types.ToolResult, isRunning bool) string {
	if result == nil {
		if isRunning {
			return r.getRunningToolIcon() // Animated for running tools
		}
		return "⏸️" // Paused/waiting icon
	}
	
	if result.IsStreaming {
		return "📡" // Streaming data icon
	}
	
	if result.Success {
		// Success with different levels of detail
		if result.Duration > 0 {
			if result.Duration < 1*time.Second {
				return "⚡" // Fast execution
			} else if result.Duration < 5*time.Second {
				return "✅" // Normal completion
			} else {
				return "🐌" // Slow but successful
			}
		}
		return "✅" // Default success
	} else {
		// Error with different types
		if result.ErrorMessage != "" {
			errorMsg := strings.ToLower(result.ErrorMessage)
			if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "timed out") {
				return "⏰" // Timeout error
			} else if strings.Contains(errorMsg, "permission") || strings.Contains(errorMsg, "denied") {
				return "🔒" // Permission error
			} else if strings.Contains(errorMsg, "not found") || strings.Contains(errorMsg, "no such") {
				return "🔍" // Not found error
			} else {
				return "❌" // Generic error
			}
		}
		return "💥" // Unknown failure
	}
}

// getFileOperationIcon returns appropriate icons for file operations
func (r *Renderer) getFileOperationIcon(operation types.FileOperation) string {
	baseIcon := ""
	
	switch strings.ToLower(operation.OperationType) {
	case "read", "get", "fetch":
		baseIcon = "📖" // Reading/fetching
	case "write", "create", "put":
		baseIcon = "📝" // Writing/creating
	case "edit", "update", "modify":
		baseIcon = "✏️" // Editing
	case "delete", "remove", "rm":
		baseIcon = "🗑️" // Deleting
	case "copy", "cp":
		baseIcon = "📋" // Copying
	case "move", "mv", "rename":
		baseIcon = "📁" // Moving/renaming
	case "chmod", "chown", "permission":
		baseIcon = "🔐" // Permission changes
	default:
		baseIcon = "📄" // Generic file operation
	}
	
	// Add success/failure indicator
	if operation.Success {
		return baseIcon // Keep original for success
	} else {
		return "❌" // Error indicator for failures
	}
}

// getProgressIndicator returns progress indicators for different stages
func (r *Renderer) getProgressIndicator(stage string) string {
	switch strings.ToLower(stage) {
	case "starting", "initializing":
		return "🚀" // Starting up
	case "processing", "working":
		return "⚙️" // Processing
	case "downloading", "fetching":
		return "⬇️" // Downloading
	case "uploading", "sending":
		return "⬆️" // Uploading
	case "analyzing", "thinking":
		return "🤔" // Analyzing
	case "building", "compiling":
		return "🔨" // Building
	case "testing", "validating":
		return "🧪" // Testing
	case "finalizing", "completing":
		return "🏁" // Finalizing
	default:
		return "⚡" // Default activity
	}
}

// buildPerformanceMetrics creates a comprehensive performance metrics display
func (r *Renderer) buildPerformanceMetrics(conv types.ConversationBlock, userInput string) string {
	var metrics strings.Builder
	
	// Basic request info
	metrics.WriteString(fmt.Sprintf("Message: \"%s\"\n", r.truncateText(userInput, 2)))
	metrics.WriteString(fmt.Sprintf("Status: %s %s\n", r.getStatusIcon(conv.Status), conv.Status.String()))
	
	// Execution timing
	if conv.ExecutionTime > 0 {
		perfIcon := r.getPerformanceIcon(conv.ExecutionTime)
		metrics.WriteString(fmt.Sprintf("Duration: %s %v\n", perfIcon, conv.ExecutionTime))
	}
	
	// Token usage analysis
	if conv.TokenUsage.InputTokens > 0 || conv.TokenUsage.OutputTokens > 0 {
		totalTokens := conv.TokenUsage.InputTokens + conv.TokenUsage.OutputTokens
		efficiency := r.calculateTokenEfficiency(conv.TokenUsage, conv.ExecutionTime)
		
		metrics.WriteString(fmt.Sprintf("Tokens: %d in + %d out = %d total\n", 
			conv.TokenUsage.InputTokens, conv.TokenUsage.OutputTokens, totalTokens))
		
		if efficiency != "" {
			metrics.WriteString(fmt.Sprintf("Efficiency: %s\n", efficiency))
		}
	}
	
	// Tool execution performance
	if len(conv.ToolCalls) > 0 {
		toolMetrics := r.calculateToolMetrics(conv)
		metrics.WriteString(fmt.Sprintf("Tools: %d executed", len(conv.ToolCalls)))
		
		if toolMetrics.TotalDuration > 0 {
			metrics.WriteString(fmt.Sprintf(" (%v total)", toolMetrics.TotalDuration))
		}
		if toolMetrics.SuccessRate < 100 {
			metrics.WriteString(fmt.Sprintf(" - %d%% success", int(toolMetrics.SuccessRate)))
		}
		metrics.WriteString("\n")
		
		if toolMetrics.SlowestTool != "" {
			metrics.WriteString(fmt.Sprintf("Slowest: %s (%v)\n", 
				toolMetrics.SlowestTool, toolMetrics.SlowestDuration))
		}
	}
	
	// File operations summary
	if len(conv.FileOperations) > 0 {
		fileStats := r.calculateFileOperationStats(conv.FileOperations)
		metrics.WriteString(fmt.Sprintf("Files: %d ops (%d R, %d W, %d E, %d D)\n", 
			len(conv.FileOperations), 
			fileStats.Reads, fileStats.Writes, fileStats.Edits, fileStats.Deletes))
	}
	
	return metrics.String()
}

// getPerformanceIcon returns icons based on execution performance
func (r *Renderer) getPerformanceIcon(duration time.Duration) string {
	if duration < 500*time.Millisecond {
		return "⚡" // Very fast
	} else if duration < 2*time.Second {
		return "🚀" // Fast
	} else if duration < 10*time.Second {
		return "⏱️" // Normal
	} else if duration < 30*time.Second {
		return "🐌" // Slow
	} else {
		return "🐢" // Very slow
	}
}

// ToolMetrics holds aggregated tool performance metrics
type ToolMetrics struct {
	TotalDuration   time.Duration
	SuccessRate     float64
	SlowestTool     string
	SlowestDuration time.Duration
}

// calculateTokenEfficiency analyzes token usage efficiency
func (r *Renderer) calculateTokenEfficiency(usage types.TokenUsage, duration time.Duration) string {
	if duration == 0 {
		return ""
	}
	
	totalTokens := usage.InputTokens + usage.OutputTokens
	if totalTokens == 0 {
		return ""
	}
	
	tokensPerSecond := float64(totalTokens) / duration.Seconds()
	
	if tokensPerSecond > 1000 {
		return fmt.Sprintf("%.1fk tokens/sec 🚀", tokensPerSecond/1000)
	} else if tokensPerSecond > 100 {
		return fmt.Sprintf("%.0f tokens/sec ⚡", tokensPerSecond)
	} else {
		return fmt.Sprintf("%.1f tokens/sec", tokensPerSecond)
	}
}

// calculateToolMetrics analyzes tool execution performance
func (r *Renderer) calculateToolMetrics(conv types.ConversationBlock) ToolMetrics {
	var metrics ToolMetrics
	var totalDuration time.Duration
	var successCount int
	var slowestDuration time.Duration
	var slowestTool string
	
	for _, toolCall := range conv.ToolCalls {
		// Find corresponding result
		for _, result := range conv.ToolResults {
			if result.ToolCallID == toolCall.ID {
				totalDuration += result.Duration
				
				if result.Success {
					successCount++
				}
				
				if result.Duration > slowestDuration {
					slowestDuration = result.Duration
					slowestTool = toolCall.ToolName
				}
				break
			}
		}
	}
	
	metrics.TotalDuration = totalDuration
	metrics.SlowestTool = slowestTool
	metrics.SlowestDuration = slowestDuration
	
	if len(conv.ToolCalls) > 0 {
		metrics.SuccessRate = float64(successCount) / float64(len(conv.ToolCalls)) * 100
	}
	
	return metrics
}

// FileOperationStats holds file operation statistics
type FileOperationStats struct {
	Reads   int
	Writes  int
	Edits   int
	Deletes int
}

// calculateFileOperationStats analyzes file operation patterns
func (r *Renderer) calculateFileOperationStats(fileOps []types.FileOperation) FileOperationStats {
	var stats FileOperationStats
	
	for _, op := range fileOps {
		switch strings.ToLower(op.OperationType) {
		case "read", "get", "fetch":
			stats.Reads++
		case "write", "create", "put":
			stats.Writes++
		case "edit", "update", "modify":
			stats.Edits++
		case "delete", "remove", "rm":
			stats.Deletes++
		}
	}
	
	return stats
}

// getAnimatedSpinner returns different spinner frames for animation effect
func (r *Renderer) getAnimatedSpinner() string {
	// Use different spinner characters to create animation effect
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	// Cycle through spinners based on current time for smooth animation
	now := time.Now().UnixMilli()
	frame := (now / 100) % int64(len(spinners)) // Change every 100ms
	return spinners[frame]
}

// getRunningToolIcon returns animated icon for running tools
func (r *Renderer) getRunningToolIcon() string {
	// Different running indicators for variety
	indicators := []string{"⚡", "🔄", "⚙️", "🔨", "⭐"}
	// Cycle through indicators based on time (slower than spinner)
	now := time.Now().UnixMilli()
	frame := (now / 500) % int64(len(indicators)) // Change every 500ms
	return indicators[frame]
}

// countRunningTools counts how many tools are currently running
func (r *Renderer) countRunningTools(conv types.ConversationBlock) int {
	runningCount := 0
	for _, toolCall := range conv.ToolCalls {
		// Check if this tool call has a corresponding result
		hasResult := false
		for _, result := range conv.ToolResults {
			if result.ToolCallID == toolCall.ID {
				hasResult = true
				break
			}
		}
		if !hasResult {
			runningCount++
		}
	}
	return runningCount
}

// renderToolExecutionHistorySection renders comprehensive tool execution history
func (r *Renderer) renderToolExecutionHistorySection(conv types.ConversationBlock, boxStyle lipgloss.Style) string {
	var s strings.Builder
	
	toolsTitle := lipgloss.NewStyle().Bold(true).Render("┌─ TOOL EXECUTION HISTORY")
	var toolsContent strings.Builder
	
	// Create a timeline of all tool executions with detailed history
	for i, toolCall := range conv.ToolCalls {
		// Find corresponding result
		var result *types.ToolResult
		for j := range conv.ToolResults {
			if conv.ToolResults[j].ToolCallID == toolCall.ID {
				result = &conv.ToolResults[j]
				break
			}
		}
		
		// Create timeline entry header
		timelineIcon := "├"
		if i == len(conv.ToolCalls)-1 {
			timelineIcon = "└"
		}
		
		startTime := toolCall.Timestamp.Format("15:04:05")
		toolsContent.WriteString(fmt.Sprintf("%s─ [%s] %s\n", timelineIcon, startTime, toolCall.ToolName))
		
		// Show tool parameters
		if len(toolCall.Parameters) > 0 {
			paramsStr := r.formatToolParameters(toolCall.Parameters)
			toolsContent.WriteString(fmt.Sprintf("│   Parameters: %s\n", paramsStr))
		}
		
		// Show execution status and timeline
		if result == nil {
			// Tool still running
			toolsContent.WriteString("│   Status: ⚡ Running...\n")
			toolsContent.WriteString("│   Started: " + startTime + "\n")
		} else {
			// Tool completed - show full history
			r.renderToolExecutionDetails(&toolsContent, toolCall, result)
		}
		
		// Show file operations related to this tool
		fileOps := r.getFileOperationsForTool(conv, result)
		if len(fileOps) > 0 {
			toolsContent.WriteString("│   File Operations:\n")
			for _, fileOp := range fileOps {
				opIcon := r.getFileOperationIcon(fileOp)
				toolsContent.WriteString(fmt.Sprintf("│     %s %s: %s\n", 
					opIcon, fileOp.OperationType, fileOp.FilePath))
			}
		}
		
		if i < len(conv.ToolCalls)-1 {
			toolsContent.WriteString("│\n")
		}
	}
	
	s.WriteString("   " + boxStyle.Render(toolsTitle + "\n" + toolsContent.String()))
	return s.String()
}

// renderToolExecutionSection renders the enhanced tool execution section
func (r *Renderer) renderToolExecutionSection(conv types.ConversationBlock, boxStyle lipgloss.Style) string {
	var s strings.Builder
	
	toolsTitle := lipgloss.NewStyle().Bold(true).Render("┌─ TOOL EXECUTION")
	var toolsContent strings.Builder
	
	for i, toolCall := range conv.ToolCalls {
		// Find corresponding result
		var result *types.ToolResult
		for j := range conv.ToolResults {
			if conv.ToolResults[j].ToolCallID == toolCall.ID {
				result = &conv.ToolResults[j]
				break
			}
		}
		
		// Determine tool status with enhanced progress indicators
		var statusIcon, statusText string
		isRunning := result == nil
		statusIcon = r.getToolResultStatusIcon(result, isRunning)
		
		if result == nil {
			// Tool still running
			statusText = "Running..."
		} else if result.Success {
			statusText = fmt.Sprintf("Completed (%v)", result.Duration)
		} else {
			statusText = fmt.Sprintf("Failed: %s", result.ErrorMessage)
		}
		
		// Format tool parameters
		paramsStr := r.formatToolParameters(toolCall.Parameters)
		
		toolsContent.WriteString(fmt.Sprintf("[%d] %s %s %s\n", 
			i+1, statusIcon, toolCall.ToolName, statusText))
		
		if paramsStr != "" {
			toolsContent.WriteString(fmt.Sprintf("    Parameters: %s\n", paramsStr))
		}
		
		// Show result preview or streaming output
		if result != nil {
			if result.IsStreaming || len(result.StreamingOutput) > 0 {
				// Show streaming output
				r.renderStreamingOutput(&toolsContent, result)
			} else if result.Success && result.Result != "" {
				// Show final result
				preview := r.truncateText(result.Result, 2)
				toolsContent.WriteString(fmt.Sprintf("    Result: %s\n", preview))
			}
		}
		
		// Show file operations for this tool
		fileOpsCount := r.countFileOperations(conv, result)
		if fileOpsCount > 0 {
			toolsContent.WriteString(fmt.Sprintf("    Files: %d operations\n", fileOpsCount))
		}
		
		if i < len(conv.ToolCalls)-1 {
			toolsContent.WriteString("\n")
		}
	}
	
	s.WriteString("   " + boxStyle.Render(toolsTitle + "\n" + toolsContent.String()))
	return s.String()
}

// renderLegacyActionsSection renders the legacy actions section for backward compatibility
func (r *Renderer) renderLegacyActionsSection(conv types.ConversationBlock, boxStyle lipgloss.Style) string {
	var s strings.Builder
	
	actionsTitle := lipgloss.NewStyle().Bold(true).Render("┌─ ASSISTANT ACTIONS")
	var actionsContent strings.Builder
	
	for i, tool := range conv.Tools {
		actionsContent.WriteString(fmt.Sprintf("[%d] %s\n", i+1, tool))
	}
	if len(conv.Files) > 0 {
		actionsContent.WriteString("Files Modified: " + strings.Join(conv.Files, ", "))
	}
	
	s.WriteString("   " + boxStyle.Render(actionsTitle + "\n" + actionsContent.String()))
	return s.String()
}

// formatToolParameters formats tool parameters for display
func (r *Renderer) formatToolParameters(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}
	
	var parts []string
	for key, value := range params {
		// Truncate long parameter values
		valueStr := fmt.Sprintf("%v", value)
		if len(valueStr) > 50 {
			valueStr = valueStr[:47] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, valueStr))
	}
	
	return strings.Join(parts, ", ")
}

// renderStreamingOutput renders streaming output for a tool result
func (r *Renderer) renderStreamingOutput(content *strings.Builder, result *types.ToolResult) {
	if len(result.StreamingOutput) == 0 {
		if result.IsStreaming {
			content.WriteString("    Output: [Waiting for output...]\n")
		}
		return
	}
	
	// Show the latest streaming chunks (limit to avoid overwhelming the UI)
	maxChunks := 3
	startIndex := 0
	if len(result.StreamingOutput) > maxChunks {
		startIndex = len(result.StreamingOutput) - maxChunks
	}
	
	content.WriteString("    Output:\n")
	for i := startIndex; i < len(result.StreamingOutput); i++ {
		chunk := result.StreamingOutput[i]
		
		// Style the output based on type
		var prefix string
		switch chunk.Type {
		case "stdout":
			prefix = "    │ "
		case "stderr":
			prefix = "    │ [ERR] "
		case "info":
			prefix = "    │ [INFO] "
		case "error":
			prefix = "    │ [ERROR] "
		default:
			prefix = "    │ "
		}
		
		// Truncate very long lines
		data := chunk.Data
		if len(data) > 80 {
			data = data[:77] + "..."
		}
		
		content.WriteString(fmt.Sprintf("%s%s\n", prefix, data))
	}
	
	// Show streaming indicator if still active
	if result.IsStreaming {
		content.WriteString("    │ [streaming...] ⠋\n")
	} else if len(result.StreamingOutput) > maxChunks {
		content.WriteString(fmt.Sprintf("    │ ... (%d more lines)\n", 
			len(result.StreamingOutput)-maxChunks))
	}
}

// renderToolExecutionDetails renders detailed execution information for a completed tool
func (r *Renderer) renderToolExecutionDetails(content *strings.Builder, toolCall types.ToolCall, result *types.ToolResult) {
	// Show execution status with enhanced icons
	statusIcon := r.getToolResultStatusIcon(result, false)
	statusText := "Completed"
	if !result.Success {
		statusText = "Failed"
	}
	
	content.WriteString(fmt.Sprintf("│   Status: %s %s\n", statusIcon, statusText))
	
	// Show timing information
	startTime := toolCall.Timestamp.Format("15:04:05")
	endTime := result.Timestamp.Format("15:04:05")
	content.WriteString(fmt.Sprintf("│   Timeline: %s → %s (%v)\n", startTime, endTime, result.Duration))
	
	// Show execution result or error
	if result.Success && result.Result != "" {
		preview := r.truncateText(result.Result, 2)
		content.WriteString(fmt.Sprintf("│   Result: %s\n", preview))
	} else if !result.Success && result.ErrorMessage != "" {
		content.WriteString(fmt.Sprintf("│   Error: %s\n", result.ErrorMessage))
	}
	
	// Show streaming output summary if available
	if len(result.StreamingOutput) > 0 {
		content.WriteString(fmt.Sprintf("│   Streaming Output: %d chunks captured\n", len(result.StreamingOutput)))
		
		// Show last few chunks as preview
		maxPreview := 2
		startIdx := len(result.StreamingOutput) - maxPreview
		if startIdx < 0 {
			startIdx = 0
		}
		
		for i := startIdx; i < len(result.StreamingOutput); i++ {
			chunk := result.StreamingOutput[i]
			timestamp := chunk.Timestamp.Format("15:04:05.000")
			data := chunk.Data
			if len(data) > 50 {
				data = data[:47] + "..."
			}
			content.WriteString(fmt.Sprintf("│     [%s] %s\n", timestamp, data))
		}
	}
}

// getFileOperationsForTool gets file operations related to a specific tool result
func (r *Renderer) getFileOperationsForTool(conv types.ConversationBlock, result *types.ToolResult) []types.FileOperation {
	if result == nil {
		return nil
	}
	
	var fileOps []types.FileOperation
	for _, fileOp := range conv.FileOperations {
		if fileOp.ToolResultID == result.ID {
			fileOps = append(fileOps, fileOp)
		}
	}
	return fileOps
}

// countFileOperations counts file operations related to a tool result
func (r *Renderer) countFileOperations(conv types.ConversationBlock, result *types.ToolResult) int {
	if result == nil {
		return 0
	}
	
	count := 0
	for _, fileOp := range conv.FileOperations {
		if fileOp.ToolResultID == result.ID {
			count++
		}
	}
	return count
}

// Utility functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}