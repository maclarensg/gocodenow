package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// AccessibilityMode defines different accessibility modes
type AccessibilityMode string

const (
	AccessibilityModeNormal       AccessibilityMode = "normal"
	AccessibilityModeHighContrast AccessibilityMode = "high_contrast"
	AccessibilityModeScreenReader AccessibilityMode = "screen_reader"
	AccessibilityModeReducedMotion AccessibilityMode = "reduced_motion"
	AccessibilityModeLargeText    AccessibilityMode = "large_text"
)

// AccessibilityManager manages accessibility features and adaptations
type AccessibilityManager struct {
	mode                   AccessibilityMode
	highContrastEnabled    bool
	screenReaderMode       bool
	reducedMotionEnabled   bool
	largeTextMode          bool
	soundFeedbackEnabled   bool
	verboseDescriptions    bool
	keyboardNavigationOnly bool
	announcements          []AccessibilityAnnouncement
	focusHistory           []string
	currentFocus           string
}

// AccessibilityAnnouncement represents a screen reader announcement
type AccessibilityAnnouncement struct {
	ID        string
	Message   string
	Priority  AnnouncementPriority
	Timestamp time.Time
	Announced bool
}

// AnnouncementPriority defines the urgency of accessibility announcements
type AnnouncementPriority string

const (
	PriorityLow    AnnouncementPriority = "polite"     // Non-urgent, wait for silence
	PriorityMedium AnnouncementPriority = "assertive"  // Important, interrupt current speech
	PriorityHigh   AnnouncementPriority = "off"        // Don't announce (contradictory name for compatibility)
)

// NewAccessibilityManager creates a new accessibility manager
func NewAccessibilityManager() *AccessibilityManager {
	return &AccessibilityManager{
		mode:                   AccessibilityModeNormal,
		highContrastEnabled:    false,
		screenReaderMode:       false,
		reducedMotionEnabled:   false,
		largeTextMode:          false,
		soundFeedbackEnabled:   false,
		verboseDescriptions:    false,
		keyboardNavigationOnly: false,
		announcements:          make([]AccessibilityAnnouncement, 0),
		focusHistory:           make([]string, 0),
		currentFocus:           "",
	}
}

// EnableHighContrast enables high contrast mode
func (am *AccessibilityManager) EnableHighContrast() {
	am.highContrastEnabled = true
	am.announcements = append(am.announcements, AccessibilityAnnouncement{
		ID:        fmt.Sprintf("high_contrast_%d", time.Now().UnixNano()),
		Message:   "High contrast mode enabled",
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Announced: false,
	})
}

// DisableHighContrast disables high contrast mode
func (am *AccessibilityManager) DisableHighContrast() {
	am.highContrastEnabled = false
	am.announcements = append(am.announcements, AccessibilityAnnouncement{
		ID:        fmt.Sprintf("high_contrast_off_%d", time.Now().UnixNano()),
		Message:   "High contrast mode disabled",
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Announced: false,
	})
}

// EnableScreenReaderMode enables screen reader optimizations
func (am *AccessibilityManager) EnableScreenReaderMode() {
	am.screenReaderMode = true
	am.verboseDescriptions = true
	am.keyboardNavigationOnly = true
	am.announcements = append(am.announcements, AccessibilityAnnouncement{
		ID:        fmt.Sprintf("screen_reader_%d", time.Now().UnixNano()),
		Message:   "Screen reader mode enabled. Interface optimized for assistive technology.",
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Announced: false,
	})
}

// EnableReducedMotion enables reduced motion mode
func (am *AccessibilityManager) EnableReducedMotion() {
	am.reducedMotionEnabled = true
	am.announcements = append(am.announcements, AccessibilityAnnouncement{
		ID:        fmt.Sprintf("reduced_motion_%d", time.Now().UnixNano()),
		Message:   "Reduced motion enabled. Animations disabled.",
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Announced: false,
	})
}

// EnableLargeText enables large text mode
func (am *AccessibilityManager) EnableLargeText() {
	am.largeTextMode = true
	am.announcements = append(am.announcements, AccessibilityAnnouncement{
		ID:        fmt.Sprintf("large_text_%d", time.Now().UnixNano()),
		Message:   "Large text mode enabled",
		Priority:  PriorityMedium,
		Timestamp: time.Now(),
		Announced: false,
	})
}

// SetFocus updates the current focus and announces focus changes
func (am *AccessibilityManager) SetFocus(element string) {
	if am.currentFocus != element {
		am.focusHistory = append(am.focusHistory, am.currentFocus)
		am.currentFocus = element
		
		if am.screenReaderMode {
			am.AnnounceText(fmt.Sprintf("Focus changed to %s", element), PriorityMedium)
		}
	}
}

// AnnounceText creates an announcement for screen readers
func (am *AccessibilityManager) AnnounceText(message string, priority AnnouncementPriority) {
	announcement := AccessibilityAnnouncement{
		ID:        fmt.Sprintf("announce_%d", time.Now().UnixNano()),
		Message:   message,
		Priority:  priority,
		Timestamp: time.Now(),
		Announced: false,
	}
	
	am.announcements = append(am.announcements, announcement)
}

// AnnounceAction announces when an action is performed
func (am *AccessibilityManager) AnnounceAction(action, context string) {
	if !am.screenReaderMode {
		return
	}
	
	var message string
	switch action {
	case "send_message":
		message = "Message sent to AI assistant"
	case "delete_conversation":
		message = "Conversation deleted"
	case "clear_all_conversations":
		message = "All conversations cleared"
	case "toggle_focus":
		if context == "input" {
			message = "Focus moved to input area"
		} else {
			message = "Focus moved to conversation area"
		}
	case "navigate_up":
		message = "Moved to previous conversation"
	case "navigate_down":
		message = "Moved to next conversation"
	case "toggle_expanded":
		message = "Conversation view toggled"
	case "show_help":
		message = "Help panel opened"
	case "next_theme":
		message = "Theme changed to next option"
	case "prev_theme":
		message = "Theme changed to previous option"
	case "export_conversations":
		message = "Export dialog opened"
	case "import_conversations":
		message = "Import dialog opened"
	case "backup_conversations":
		message = "Backup dialog opened"
	case "restore_conversations":
		message = "Restore dialog opened"
	case "filter_conversations":
		message = "Filter dialog opened"
	default:
		message = fmt.Sprintf("Action performed: %s", action)
	}
	
	am.AnnounceText(message, PriorityMedium)
}

// AnnounceStatus announces status changes
func (am *AccessibilityManager) AnnounceStatus(status, details string) {
	if !am.screenReaderMode {
		return
	}
	
	message := status
	if details != "" {
		message = fmt.Sprintf("%s. %s", status, details)
	}
	
	am.AnnounceText(message, PriorityMedium)
}

// GetPendingAnnouncements returns announcements that haven't been processed
func (am *AccessibilityManager) GetPendingAnnouncements() []AccessibilityAnnouncement {
	var pending []AccessibilityAnnouncement
	for i := range am.announcements {
		if !am.announcements[i].Announced {
			pending = append(pending, am.announcements[i])
			am.announcements[i].Announced = true
		}
	}
	return pending
}

// GetAccessibleDescription returns a detailed description for screen readers
func (am *AccessibilityManager) GetAccessibleDescription(element string, context map[string]interface{}) string {
	if !am.screenReaderMode {
		return ""
	}
	
	switch element {
	case "header":
		modelName := "unknown"
		status := "disconnected"
		if ctx, ok := context["model_name"].(string); ok {
			modelName = ctx
		}
		if ctx, ok := context["status"].(string); ok {
			status = ctx
		}
		return fmt.Sprintf("Header: gocodenow version 1.0, connected to %s model, status: %s", modelName, status)
		
	case "conversation_area":
		count := 0
		if ctx, ok := context["conversation_count"].(int); ok {
			count = ctx
		}
		selected := 0
		if ctx, ok := context["selected"].(int); ok {
			selected = ctx + 1 // Convert to 1-based for users
		}
		
		if count == 0 {
			return "Conversation area: No conversations yet. Start by typing a message in the input area."
		}
		return fmt.Sprintf("Conversation area: %d conversations total, currently viewing conversation %d", count, selected)
		
	case "input_area":
		focused := false
		if ctx, ok := context["focused"].(bool); ok {
			focused = ctx
		}
		
		focusState := "not focused"
		if focused {
			focusState = "focused and ready for input"
		}
		
		return fmt.Sprintf("Input area: Multi-line text input %s. Press Enter to send message, Ctrl+J for new line, Tab to switch focus", focusState)
		
	case "conversation_block":
		expanded := false
		if ctx, ok := context["expanded"].(bool); ok {
			expanded = ctx
		}
		
		toolCount := 0
		if ctx, ok := context["tool_count"].(int); ok {
			toolCount = ctx
		}
		
		timestamp := ""
		if ctx, ok := context["timestamp"].(string); ok {
			timestamp = ctx
		}
		
		expandState := "collapsed"
		if expanded {
			expandState = "expanded"
		}
		
		description := fmt.Sprintf("Conversation block %s, created at %s", expandState, timestamp)
		if toolCount > 0 {
			description += fmt.Sprintf(", %d tools executed", toolCount)
		}
		
		return description
		
	case "confirmation_dialog":
		title := "Confirmation required"
		if ctx, ok := context["title"].(string); ok {
			title = ctx
		}
		
		message := ""
		if ctx, ok := context["message"].(string); ok {
			message = ctx
		}
		
		dangerLevel := 0
		if ctx, ok := context["danger_level"].(int); ok {
			dangerLevel = ctx
		}
		
		urgency := "normal"
		if dangerLevel >= 3 {
			urgency = "critical"
		} else if dangerLevel >= 2 {
			urgency = "high priority"
		}
		
		return fmt.Sprintf("Confirmation dialog: %s. %s. This is a %s action. Use Y for yes, N for no, or arrow keys to navigate", title, message, urgency)
		
	case "help_panel":
		shortcutCount := 0
		if ctx, ok := context["shortcut_count"].(int); ok {
			shortcutCount = ctx
		}
		
		return fmt.Sprintf("Help panel: Displaying %d keyboard shortcuts organized by category. Use arrow keys to navigate, Escape to close", shortcutCount)
		
	case "onboarding_step":
		stepNumber := 1
		totalSteps := 1
		if ctx, ok := context["step_number"].(int); ok {
			stepNumber = ctx
		}
		if ctx, ok := context["total_steps"].(int); ok {
			totalSteps = ctx
		}
		
		title := "Tutorial step"
		if ctx, ok := context["title"].(string); ok {
			title = ctx
		}
		
		return fmt.Sprintf("Tutorial: Step %d of %d. %s. Follow the instructions to continue, or press Escape to skip", stepNumber, totalSteps, title)
		
	default:
		return fmt.Sprintf("Interface element: %s", element)
	}
}

// GetNavigationHelp returns navigation instructions for the current context
func (am *AccessibilityManager) GetNavigationHelp(context string) string {
	if !am.screenReaderMode {
		return ""
	}
	
	switch context {
	case "global":
		return "Navigation: Tab switches focus areas, F1 opens help, Ctrl+C exits application"
	case "conversation":
		return "Conversation navigation: Up/Down arrows move between conversations, Space expands details, Enter also expands, Del deletes conversation"
	case "input":
		return "Input area: Type your message, Enter sends, Ctrl+J adds new line, Ctrl+U clears input, Tab switches to conversation area"
	case "dialog":
		return "Dialog navigation: Y for yes, N for no, Tab or arrow keys move between options, Enter confirms, Escape cancels"
	case "help":
		return "Help panel: Arrow keys navigate shortcuts, Escape closes help, shortcuts grouped by category"
	case "onboarding":
		return "Tutorial: Follow on-screen instructions, press indicated keys to continue, Escape skips tutorial"
	default:
		return "Use Tab to navigate between areas, F1 for help, arrow keys within areas"
	}
}

// ShouldUseHighContrast returns whether high contrast should be applied
func (am *AccessibilityManager) ShouldUseHighContrast() bool {
	return am.highContrastEnabled
}

// ShouldReduceMotion returns whether motion should be reduced
func (am *AccessibilityManager) ShouldReduceMotion() bool {
	return am.reducedMotionEnabled
}

// ShouldUseLargeText returns whether large text indicators should be used
func (am *AccessibilityManager) ShouldUseLargeText() bool {
	return am.largeTextMode
}

// IsScreenReaderMode returns whether screen reader mode is enabled
func (am *AccessibilityManager) IsScreenReaderMode() bool {
	return am.screenReaderMode
}

// GetCurrentFocus returns the currently focused element
func (am *AccessibilityManager) GetCurrentFocus() string {
	return am.currentFocus
}

// GetAccessibilityStatus returns current accessibility settings status
func (am *AccessibilityManager) GetAccessibilityStatus() map[string]bool {
	return map[string]bool{
		"high_contrast":     am.highContrastEnabled,
		"screen_reader":     am.screenReaderMode,
		"reduced_motion":    am.reducedMotionEnabled,
		"large_text":        am.largeTextMode,
		"sound_feedback":    am.soundFeedbackEnabled,
		"verbose_mode":      am.verboseDescriptions,
		"keyboard_only":     am.keyboardNavigationOnly,
	}
}

// ApplyAccessibilityStyles applies accessibility modifications to styles
func (am *AccessibilityManager) ApplyAccessibilityStyles(baseStyle lipgloss.Style, element string) lipgloss.Style {
	style := baseStyle
	
	// High contrast modifications
	if am.highContrastEnabled {
		switch element {
		case "text":
			style = style.Foreground(lipgloss.Color("#FFFFFF"))
		case "background":
			style = style.Background(lipgloss.Color("#000000"))
		case "border":
			style = style.BorderForeground(lipgloss.Color("#FFFFFF"))
		case "selected":
			style = style.
				Background(lipgloss.Color("#FFFF00")).
				Foreground(lipgloss.Color("#000000")).
				Bold(true)
		case "error":
			style = style.
				Foreground(lipgloss.Color("#FF0000")).
				Background(lipgloss.Color("#000000")).
				Bold(true)
		case "success":
			style = style.
				Foreground(lipgloss.Color("#00FF00")).
				Background(lipgloss.Color("#000000")).
				Bold(true)
		}
	}
	
	// Large text modifications
	if am.largeTextMode {
		// In terminal context, we use visual indicators instead of actual font size
		switch element {
		case "title":
			style = style.Bold(true).Underline(true)
		case "important":
			style = style.Bold(true).Italic(true)
		}
	}
	
	// Screen reader optimizations
	if am.screenReaderMode {
		// Remove decorative elements that might confuse screen readers
		switch element {
		case "decorative":
			return lipgloss.NewStyle() // Return empty style
		case "icon":
			return lipgloss.NewStyle() // Remove icons in screen reader mode
		}
	}
	
	return style
}

// HandleAccessibilityKeyMsg processes accessibility-related key messages
func (am *AccessibilityManager) HandleAccessibilityKeyMsg(msg tea.KeyMsg) string {
	keyStr := msg.String()
	
	// Accessibility shortcuts
	switch keyStr {
	case "alt+h": // Toggle high contrast
		if am.highContrastEnabled {
			am.DisableHighContrast()
		} else {
			am.EnableHighContrast()
		}
		return "toggle_high_contrast"
		
	case "alt+r": // Toggle reduced motion
		if am.reducedMotionEnabled {
			am.reducedMotionEnabled = false
			am.AnnounceText("Reduced motion disabled", PriorityMedium)
		} else {
			am.EnableReducedMotion()
		}
		return "toggle_reduced_motion"
		
	case "alt+l": // Toggle large text
		if am.largeTextMode {
			am.largeTextMode = false
			am.AnnounceText("Large text mode disabled", PriorityMedium)
		} else {
			am.EnableLargeText()
		}
		return "toggle_large_text"
		
	case "alt+s": // Toggle screen reader mode
		if am.screenReaderMode {
			am.screenReaderMode = false
			am.verboseDescriptions = false
			am.keyboardNavigationOnly = false
			am.AnnounceText("Screen reader mode disabled", PriorityMedium)
		} else {
			am.EnableScreenReaderMode()
		}
		return "toggle_screen_reader"
		
	case "alt+a": // Announce current context
		return "announce_context"
		
	case "alt+f": // Announce current focus
		if am.currentFocus != "" {
			am.AnnounceText(fmt.Sprintf("Currently focused on: %s", am.currentFocus), PriorityMedium)
		} else {
			am.AnnounceText("No element currently focused", PriorityMedium)
		}
		return "announce_focus"
	}
	
	return ""
}

// RenderAccessibilityIndicators renders indicators for active accessibility features
func (am *AccessibilityManager) RenderAccessibilityIndicators() string {
	if !am.highContrastEnabled && !am.screenReaderMode && !am.reducedMotionEnabled && !am.largeTextMode {
		return ""
	}
	
	var indicators []string
	
	if am.highContrastEnabled {
		indicators = append(indicators, "HC")
	}
	if am.screenReaderMode {
		indicators = append(indicators, "SR")
	}
	if am.reducedMotionEnabled {
		indicators = append(indicators, "RM")
	}
	if am.largeTextMode {
		indicators = append(indicators, "LT")
	}
	
	indicatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00")).
		Background(lipgloss.Color("#0000AA")).
		Bold(true).
		Padding(0, 1)
	
	return indicatorStyle.Render("[A11Y: " + strings.Join(indicators, ",") + "]")
}

// CleanupOldAnnouncements removes old announcements to prevent memory buildup
func (am *AccessibilityManager) CleanupOldAnnouncements() {
	cutoff := time.Now().Add(-5 * time.Minute)
	
	var filtered []AccessibilityAnnouncement
	for _, announcement := range am.announcements {
		if announcement.Timestamp.After(cutoff) {
			filtered = append(filtered, announcement)
		}
	}
	
	am.announcements = filtered
}