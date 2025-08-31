package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TooltipPosition defines where tooltips should appear
type TooltipPosition string

const (
	TooltipPositionTop    TooltipPosition = "top"
	TooltipPositionBottom TooltipPosition = "bottom"
	TooltipPositionLeft   TooltipPosition = "left"
	TooltipPositionRight  TooltipPosition = "right"
)

// TooltipType categorizes different types of tooltips
type TooltipType string

const (
	TooltipTypeInfo    TooltipType = "info"
	TooltipTypeWarning TooltipType = "warning"
	TooltipTypeError   TooltipType = "error"
	TooltipTypeSuccess TooltipType = "success"
	TooltipTypeHint    TooltipType = "hint"
)

// Tooltip represents a single tooltip
type Tooltip struct {
	ID         string
	Title      string
	Content    string
	Type       TooltipType
	Position   TooltipPosition
	Duration   time.Duration // 0 means persistent until dismissed
	ShowOnce   bool          // Whether to show only once per session
	Context    string        // Context where this tooltip applies
	Trigger    string        // What triggers this tooltip
	Shown      bool          // Whether this tooltip has been shown
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// TooltipManager manages tooltips and contextual help
type TooltipManager struct {
	tooltips       []Tooltip
	activeTooltips []string // IDs of currently active tooltips
	shownTooltips  map[string]bool // Track tooltips shown this session
	helpHints      map[string][]string // Context-specific help hints
	currentContext string
}

// NewTooltipManager creates a new tooltip manager
func NewTooltipManager() *TooltipManager {
	tm := &TooltipManager{
		tooltips:       make([]Tooltip, 0),
		activeTooltips: make([]string, 0),
		shownTooltips:  make(map[string]bool),
		helpHints:      make(map[string][]string),
		currentContext: "global",
	}
	
	tm.initializeDefaultTooltips()
	tm.initializeHelpHints()
	return tm
}

// initializeDefaultTooltips sets up default helpful tooltips for the application
func (tm *TooltipManager) initializeDefaultTooltips() {
	defaultTooltips := []Tooltip{
		{
			ID:       "welcome",
			Title:    "Welcome to gocodenow! 🚀",
			Content:  "This is your AI coding assistant. Type your questions or requests in the input area below and press Enter to start a conversation.",
			Type:     TooltipTypeInfo,
			Position: TooltipPositionTop,
			Duration: 10 * time.Second,
			Context:  "first_run",
			Trigger:  "startup",
		},
		{
			ID:       "navigation_help",
			Title:    "Navigation Tips",
			Content:  "Use ↑↓ arrows to browse conversations, Tab to switch focus, Space to expand/collapse conversations. Press ? or F1 for full help.",
			Type:     TooltipTypeHint,
			Position: TooltipPositionBottom,
			Duration: 8 * time.Second,
			Context:  "conversation",
			Trigger:  "first_navigation",
			ShowOnce: true,
		},
		{
			ID:       "input_help",
			Title:    "Input Tips",
			Content:  "Press Enter to send your message, Ctrl+J for new lines, Ctrl+U to clear input. You can paste multi-line code and text directly.",
			Type:     TooltipTypeHint,
			Position: TooltipPositionTop,
			Duration: 6 * time.Second,
			Context:  "input",
			Trigger:  "first_input_focus",
			ShowOnce: true,
		},
		{
			ID:       "confirmation_help",
			Title:    "Confirmation Dialog",
			Content:  "Use y/n for quick choices, arrow keys to navigate, Enter to confirm, Esc to cancel. Red borders indicate destructive actions.",
			Type:     TooltipTypeWarning,
			Position: TooltipPositionTop,
			Duration: 5 * time.Second,
			Context:  "dialog",
			Trigger:  "first_confirmation",
			ShowOnce: true,
		},
		{
			ID:       "help_available",
			Title:    "Need Help?",
			Content:  "Press ? or F1 to see all available keyboard shortcuts and commands. Help is context-sensitive.",
			Type:     TooltipTypeInfo,
			Position: TooltipPositionBottom,
			Duration: 0, // Persistent
			Context:  "global",
			Trigger:  "idle_5_minutes",
			ShowOnce: true,
		},
		{
			ID:       "tool_execution",
			Title:    "Tool Execution",
			Content:  "The AI is running tools to help with your request. You can see real-time progress and results in the expanded conversation view.",
			Type:     TooltipTypeInfo,
			Position: TooltipPositionTop,
			Duration: 4 * time.Second,
			Context:  "conversation",
			Trigger:  "first_tool_execution",
			ShowOnce: true,
		},
	}
	
	for _, tooltip := range defaultTooltips {
		tooltip.CreatedAt = time.Now()
		if tooltip.Duration > 0 {
			tooltip.ExpiresAt = tooltip.CreatedAt.Add(tooltip.Duration)
		}
		tm.tooltips = append(tm.tooltips, tooltip)
	}
}

// initializeHelpHints sets up context-specific help hints
func (tm *TooltipManager) initializeHelpHints() {
	tm.helpHints = map[string][]string{
		"global": {
			"Press ? or F1 to show help panel",
			"Use Tab to switch between conversation and input areas",
			"Home/End keys jump to first/last conversation",
			"Ctrl+C to quit the application",
		},
		"conversation": {
			"↑↓ arrows navigate through conversations",
			"Space or Enter toggles conversation expansion",
			"Del or 'd' deletes the selected conversation",
			"Ctrl+Del clears all conversations",
		},
		"input": {
			"Enter sends your message to the AI",
			"Ctrl+J inserts a new line without sending",
			"Ctrl+U clears the entire input area",
			"You can paste code and multi-line text directly",
		},
		"dialog": {
			"y/n keys for quick yes/no responses",
			"←/→ arrows or Tab to navigate between buttons",
			"Enter confirms the selected option",
			"Esc always cancels and closes the dialog",
		},
		"tool_execution": {
			"Tools are running in the background",
			"Expand conversations to see detailed progress",
			"Some tools may take time to complete",
			"You can continue typing while tools execute",
		},
		"error": {
			"Check the error message for details",
			"Try rephrasing your request if needed",
			"Some operations may require confirmation",
			"Press Ctrl+R to retry the last action",
		},
		"empty": {
			"Start by typing a message or question",
			"Ask about coding problems, file operations, or general help",
			"The AI can read, write, and modify files",
			"Try: 'Help me debug this code' or 'Create a simple web server'",
		},
	}
}

// ShowTooltip activates a tooltip by ID
func (tm *TooltipManager) ShowTooltip(id string) {
	// Check if already shown and set to show once
	if tm.shownTooltips[id] {
		for _, tooltip := range tm.tooltips {
			if tooltip.ID == id && tooltip.ShowOnce {
				return
			}
		}
	}
	
	// Find the tooltip
	for i, tooltip := range tm.tooltips {
		if tooltip.ID == id {
			// Mark as active
			tm.activeTooltips = append(tm.activeTooltips, id)
			tm.shownTooltips[id] = true
			tm.tooltips[i].Shown = true
			
			// Set expiration time if not persistent
			if tooltip.Duration > 0 {
				tm.tooltips[i].ExpiresAt = time.Now().Add(tooltip.Duration)
			}
			break
		}
	}
}

// HideTooltip removes a tooltip from active display
func (tm *TooltipManager) HideTooltip(id string) {
	for i, activeID := range tm.activeTooltips {
		if activeID == id {
			// Remove from active tooltips
			tm.activeTooltips = append(tm.activeTooltips[:i], tm.activeTooltips[i+1:]...)
			break
		}
	}
}

// UpdateContext changes the current context and triggers relevant tooltips
func (tm *TooltipManager) UpdateContext(newContext string) {
	if tm.currentContext == newContext {
		return
	}
	
	oldContext := tm.currentContext
	tm.currentContext = newContext
	
	// Trigger context-specific tooltips
	tm.triggerContextTooltips(newContext, oldContext)
}

// triggerContextTooltips shows appropriate tooltips for context changes
func (tm *TooltipManager) triggerContextTooltips(newContext, oldContext string) {
	contextTriggers := map[string]string{
		"input":        "first_input_focus",
		"conversation": "first_navigation",
		"dialog":       "first_confirmation",
	}
	
	if trigger, exists := contextTriggers[newContext]; exists {
		tm.TriggerTooltip(trigger)
	}
}

// TriggerTooltip triggers tooltips based on an event
func (tm *TooltipManager) TriggerTooltip(trigger string) {
	for _, tooltip := range tm.tooltips {
		if tooltip.Trigger == trigger && tooltip.Context == tm.currentContext {
			tm.ShowTooltip(tooltip.ID)
		}
	}
}

// CleanupExpiredTooltips removes expired tooltips
func (tm *TooltipManager) CleanupExpiredTooltips() {
	now := time.Now()
	
	for i := len(tm.activeTooltips) - 1; i >= 0; i-- {
		tooltipID := tm.activeTooltips[i]
		
		// Find the tooltip
		for _, tooltip := range tm.tooltips {
			if tooltip.ID == tooltipID {
				// Remove if expired (and not persistent)
				if tooltip.Duration > 0 && now.After(tooltip.ExpiresAt) {
					tm.HideTooltip(tooltipID)
				}
				break
			}
		}
	}
}

// GetActiveTooltips returns currently active tooltips
func (tm *TooltipManager) GetActiveTooltips() []Tooltip {
	var active []Tooltip
	
	for _, tooltipID := range tm.activeTooltips {
		for _, tooltip := range tm.tooltips {
			if tooltip.ID == tooltipID {
				active = append(active, tooltip)
				break
			}
		}
	}
	
	return active
}

// GetContextualHints returns help hints for the current context
func (tm *TooltipManager) GetContextualHints() []string {
	if hints, exists := tm.helpHints[tm.currentContext]; exists {
		return hints
	}
	return tm.helpHints["global"]
}

// RenderTooltips renders all active tooltips
func (tm *TooltipManager) RenderTooltips(width, height int) string {
	tm.CleanupExpiredTooltips()
	
	activeTooltips := tm.GetActiveTooltips()
	if len(activeTooltips) == 0 {
		return ""
	}
	
	var rendered strings.Builder
	
	// Render each active tooltip
	for i, tooltip := range activeTooltips {
		tooltipView := tm.renderSingleTooltip(tooltip, width)
		
		// Position the tooltip based on its settings
		positioned := tm.positionTooltip(tooltipView, tooltip.Position, width, height, i)
		rendered.WriteString(positioned)
		
		if i < len(activeTooltips)-1 {
			rendered.WriteString("\n")
		}
	}
	
	return rendered.String()
}

// renderSingleTooltip renders a single tooltip with appropriate styling
func (tm *TooltipManager) renderSingleTooltip(tooltip Tooltip, maxWidth int) string {
	// Define styles based on tooltip type
	var (
		borderColor   string
		backgroundColor string
		titleColor     string
		contentColor   string
	)
	
	switch tooltip.Type {
	case TooltipTypeInfo:
		borderColor = "#00D7FF"
		backgroundColor = "#001122"
		titleColor = "#00D7FF"
		contentColor = "#FFFFFF"
	case TooltipTypeWarning:
		borderColor = "#FFFF00"
		backgroundColor = "#221100"
		titleColor = "#FFFF00"
		contentColor = "#FFFFFF"
	case TooltipTypeError:
		borderColor = "#FF0000"
		backgroundColor = "#220000"
		titleColor = "#FF0000"
		contentColor = "#FFFFFF"
	case TooltipTypeSuccess:
		borderColor = "#00FF00"
		backgroundColor = "#002200"
		titleColor = "#00FF00"
		contentColor = "#FFFFFF"
	case TooltipTypeHint:
		borderColor = "#AA00FF"
		backgroundColor = "#110022"
		titleColor = "#AA00FF"
		contentColor = "#CCCCCC"
	default:
		borderColor = "#666666"
		backgroundColor = "#111111"
		titleColor = "#CCCCCC"
		contentColor = "#FFFFFF"
	}
	
	// Calculate tooltip width (max 60 characters or screen width - 10)
	tooltipWidth := min(60, maxWidth-10)
	
	// Create styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Background(lipgloss.Color(backgroundColor)).
		Padding(1).
		Width(tooltipWidth).
		Align(lipgloss.Left)
	
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(titleColor)).
		Bold(true)
	
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(contentColor))
	
	// Build tooltip content
	var content strings.Builder
	
	if tooltip.Title != "" {
		content.WriteString(titleStyle.Render(tooltip.Title))
		content.WriteString("\n\n")
	}
	
	// Wrap content text
	wrappedContent := tm.wrapText(tooltip.Content, tooltipWidth-4)
	content.WriteString(contentStyle.Render(wrappedContent))
	
	// Add dismiss hint for persistent tooltips
	if tooltip.Duration == 0 {
		content.WriteString("\n\n")
		dismissStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
		content.WriteString(dismissStyle.Render("Press any key to dismiss"))
	}
	
	return borderStyle.Render(content.String())
}

// positionTooltip positions a tooltip based on its position preference
func (tm *TooltipManager) positionTooltip(tooltipView string, position TooltipPosition, width, height, index int) string {
	// For simplicity, we'll use basic positioning
	// In a more advanced implementation, you'd calculate exact positions
	
	// Add some spacing between multiple tooltips
	if index > 0 {
		return "\n" + tooltipView
	}
	
	// For now, just return the tooltip as-is
	// The calling View method will handle overall positioning
	return tooltipView
}

// wrapText wraps text to fit within specified width
func (tm *TooltipManager) wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	
	var lines []string
	var currentLine strings.Builder
	
	for _, word := range words {
		// If adding this word would exceed the width, start a new line
		if currentLine.Len() > 0 && currentLine.Len()+len(word)+1 > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}
		
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}
	
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}
	
	return strings.Join(lines, "\n")
}

// AddCustomTooltip adds a custom tooltip to the manager
func (tm *TooltipManager) AddCustomTooltip(tooltip Tooltip) {
	tooltip.CreatedAt = time.Now()
	if tooltip.Duration > 0 {
		tooltip.ExpiresAt = tooltip.CreatedAt.Add(tooltip.Duration)
	}
	tm.tooltips = append(tm.tooltips, tooltip)
}

// ShowTemporaryMessage shows a temporary message tooltip
func (tm *TooltipManager) ShowTemporaryMessage(title, message string, duration time.Duration, tooltipType TooltipType) {
	tooltip := Tooltip{
		ID:       "temp_" + time.Now().Format("20060102150405"),
		Title:    title,
		Content:  message,
		Type:     tooltipType,
		Position: TooltipPositionTop,
		Duration: duration,
		Context:  tm.currentContext,
		Trigger:  "manual",
	}
	
	tm.AddCustomTooltip(tooltip)
	tm.ShowTooltip(tooltip.ID)
}

// GetCurrentContext returns the current context
func (tm *TooltipManager) GetCurrentContext() string {
	return tm.currentContext
}

// HasActiveTooltips returns whether there are any active tooltips
func (tm *TooltipManager) HasActiveTooltips() bool {
	tm.CleanupExpiredTooltips()
	return len(tm.activeTooltips) > 0
}

// DismissAllTooltips hides all currently active tooltips
func (tm *TooltipManager) DismissAllTooltips() {
	tm.activeTooltips = make([]string, 0)
}