package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// KeyBinding represents a keyboard shortcut
type KeyBinding struct {
	Keys        string
	Description string
	Action      string
	Context     string // "global", "conversation", "input", "dialog"
	Category    string // "navigation", "editing", "tools", "system"
}

// ShortcutManager manages keyboard shortcuts and help display
type ShortcutManager struct {
	bindings    []KeyBinding
	helpVisible bool
	keyMap      GlobalKeyMap
	helpScroll  int // Current scroll position in help dialog
}

// GlobalKeyMap defines all key bindings for the application
type GlobalKeyMap struct {
	// Navigation
	NavigateUp     key.Binding
	NavigateDown   key.Binding
	NavigateHome   key.Binding
	NavigateEnd    key.Binding
	ToggleFocus    key.Binding
	
	// Editing
	SendMessage    key.Binding
	NewLine        key.Binding
	ClearInput     key.Binding
	
	// Conversation Management
	ToggleExpanded key.Binding
	DeleteConv     key.Binding
	ClearAll       key.Binding
	ExportConv     key.Binding
	ImportConv     key.Binding
	BackupConv     key.Binding
	RestoreConv    key.Binding
	FilterConv     key.Binding
	
	// Tools and Actions
	RetryLast      key.Binding
	CancelExec     key.Binding
	QuickSave      key.Binding
	
	// System
	ShowHelp       key.Binding
	ShowSettings   key.Binding
	Quit           key.Binding
	ForceQuit      key.Binding
	
	// Themes
	NextTheme      key.Binding
	PrevTheme      key.Binding
	ToggleCompact  key.Binding
	
	// Quick Actions (F-keys)
	F1Help         key.Binding
	F2Rename       key.Binding
	F3Search       key.Binding
	F4Settings     key.Binding
	F5Refresh      key.Binding
	F12Debug       key.Binding
}

// DefaultGlobalKeyMap returns the default key bindings
func DefaultGlobalKeyMap() GlobalKeyMap {
	return GlobalKeyMap{
		// Navigation
		NavigateUp: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "navigate up"),
		),
		NavigateDown: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "navigate down"),
		),
		NavigateHome: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("home/g", "go to top"),
		),
		NavigateEnd: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("end/G", "go to bottom"),
		),
		ToggleFocus: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch focus"),
		),
		
		// Editing
		SendMessage: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "send message"),
		),
		NewLine: key.NewBinding(
			key.WithKeys("ctrl+j", "ctrl+enter"),
			key.WithHelp("ctrl+j", "new line"),
		),
		ClearInput: key.NewBinding(
			key.WithKeys("ctrl+u"),
			key.WithHelp("ctrl+u", "clear input"),
		),
		
		// Conversation Management
		ToggleExpanded: key.NewBinding(
			key.WithKeys("space", "enter"),
			key.WithHelp("space", "toggle expanded"),
		),
		DeleteConv: key.NewBinding(
			key.WithKeys("delete", "d"),
			key.WithHelp("del/d", "delete conversation"),
		),
		ClearAll: key.NewBinding(
			key.WithKeys("ctrl+delete", "ctrl+d"),
			key.WithHelp("ctrl+del", "clear all conversations"),
		),
		ExportConv: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("ctrl+e", "export conversation"),
		),
		ImportConv: key.NewBinding(
			key.WithKeys("ctrl+i"),
			key.WithHelp("ctrl+i", "import conversations"),
		),
		BackupConv: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("ctrl+b", "backup conversations"),
		),
		RestoreConv: key.NewBinding(
			key.WithKeys("ctrl+shift+b"),
			key.WithHelp("ctrl+shift+b", "restore from backup"),
		),
		FilterConv: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("ctrl+f", "filter conversations"),
		),
		
		// Tools and Actions
		RetryLast: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "retry last action"),
		),
		CancelExec: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "cancel/quit"),
		),
		QuickSave: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "quick save"),
		),
		
		// System
		ShowHelp: key.NewBinding(
			key.WithKeys(), // No keys - only F1 should toggle help
			key.WithHelp("F1", "show help"),
		),
		ShowSettings: key.NewBinding(
			key.WithKeys("ctrl+comma", "s"),
			key.WithHelp("ctrl+,", "settings"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+q"),
			key.WithHelp("ctrl+q", "force quit"),
		),
		
		// Themes
		NextTheme: key.NewBinding(
			key.WithKeys("ctrl+]"),
			key.WithHelp("ctrl+]", "next theme"),
		),
		PrevTheme: key.NewBinding(
			key.WithKeys("ctrl+["),
			key.WithHelp("ctrl+[", "previous theme"),
		),
		ToggleCompact: key.NewBinding(
			key.WithKeys("ctrl+shift+c"),
			key.WithHelp("ctrl+shift+c", "toggle compact mode"),
		),
		
		// Function Keys
		F1Help: key.NewBinding(
			key.WithKeys("f1"),
			key.WithHelp("F1", "help"),
		),
		F2Rename: key.NewBinding(
			key.WithKeys("f2"),
			key.WithHelp("F2", "rename"),
		),
		F3Search: key.NewBinding(
			key.WithKeys("f3"),
			key.WithHelp("F3", "search"),
		),
		F4Settings: key.NewBinding(
			key.WithKeys("f4"),
			key.WithHelp("F4", "settings"),
		),
		F5Refresh: key.NewBinding(
			key.WithKeys("f5"),
			key.WithHelp("F5", "refresh"),
		),
		F12Debug: key.NewBinding(
			key.WithKeys("f12"),
			key.WithHelp("F12", "debug info"),
		),
	}
}

// NewShortcutManager creates a new shortcut manager
func NewShortcutManager() *ShortcutManager {
	sm := &ShortcutManager{
		helpVisible: false,
		keyMap:      DefaultGlobalKeyMap(),
		bindings:    make([]KeyBinding, 0),
	}
	
	sm.initializeDefaultBindings()
	return sm
}

// initializeDefaultBindings sets up the default key bindings
func (sm *ShortcutManager) initializeDefaultBindings() {
	sm.bindings = []KeyBinding{
		// Navigation
		{"↑, k", "Navigate up", "navigate_up", "conversation", "navigation"},
		{"↓, j", "Navigate down", "navigate_down", "conversation", "navigation"},
		{"Home, g", "Go to top", "navigate_home", "conversation", "navigation"},
		{"End, G", "Go to bottom", "navigate_end", "conversation", "navigation"},
		{"Tab", "Switch focus", "toggle_focus", "global", "navigation"},
		
		// Editing
		{"Enter", "Send message / Toggle expand", "send_or_toggle", "input", "editing"},
		{"Ctrl+J", "Insert new line", "new_line", "input", "editing"},
		{"Ctrl+U", "Clear input", "clear_input", "input", "editing"},
		
		// Conversation Management
		{"Space", "Toggle conversation expanded", "toggle_expanded", "conversation", "editing"},
		{"Del, d", "Delete conversation", "delete_conversation", "conversation", "editing"},
		{"Ctrl+Del", "Clear all conversations", "clear_all", "conversation", "editing"},
		{"Ctrl+E", "Export conversations", "export_conversations", "conversation", "tools"},
		{"Ctrl+I", "Import conversations", "import_conversations", "global", "tools"},
		{"Ctrl+B", "Backup conversations", "backup_conversations", "global", "tools"},
		{"Ctrl+Shift+B", "Restore from backup", "restore_conversations", "global", "tools"},
		{"Ctrl+F", "Filter conversations", "filter_conversations", "conversation", "tools"},
		
		// Tools and Actions
		{"Ctrl+R", "Retry last action", "retry_last", "global", "tools"},
		{"Ctrl+S", "Quick save", "quick_save", "global", "tools"},
		{"Ctrl+Z", "Undo last action", "undo", "global", "tools"},
		
		// System
		{"F1", "Show/hide help", "show_help", "global", "system"},
		{"Ctrl+,", "Open settings", "show_settings", "global", "system"},
		{"q", "Quit application", "quit", "global", "system"},
		{"Ctrl+Q", "Force quit", "force_quit", "global", "system"},
		
		// Themes
		{"Ctrl+]", "Next theme", "next_theme", "global", "system"},
		{"Ctrl+[", "Previous theme", "prev_theme", "global", "system"},
		{"Ctrl+Shift+C", "Toggle compact mode", "toggle_compact", "global", "system"},
		
		// Function Keys
		{"F1", "Help", "help", "global", "system"},
		{"F2", "Rename conversation", "rename", "conversation", "editing"},
		{"F3", "Search conversations", "search", "global", "tools"},
		{"F4", "Settings", "settings", "global", "system"},
		{"F5", "Refresh/Reload", "refresh", "global", "system"},
		{"F12", "Debug information", "debug", "global", "system"},
	}
}

// ToggleHelp toggles the help visibility
func (sm *ShortcutManager) ToggleHelp() {
	sm.helpVisible = !sm.helpVisible
}

// IsHelpVisible returns whether help is currently visible
func (sm *ShortcutManager) IsHelpVisible() bool {
	return sm.helpVisible
}

// ShowHelp shows the help panel
func (sm *ShortcutManager) ShowHelp() {
	sm.helpVisible = true
}

// HideHelp hides the help panel
func (sm *ShortcutManager) HideHelp() {
	sm.helpVisible = false
	sm.helpScroll = 0 // Reset scroll when hiding
}

// ScrollHelpUp scrolls the help panel up
func (sm *ShortcutManager) ScrollHelpUp() {
	if sm.helpScroll > 0 {
		sm.helpScroll--
	}
}

// ScrollHelpDown scrolls the help panel down
func (sm *ShortcutManager) ScrollHelpDown() {
	sm.helpScroll++
}

// HandleHelpKeyMsg handles keys when help is visible
func (sm *ShortcutManager) HandleHelpKeyMsg(msg tea.KeyMsg) bool {
	if !sm.helpVisible {
		return false
	}
	
	switch msg.String() {
	case "f1":
		sm.ToggleHelp()
		return true
	case "up", "k":
		sm.ScrollHelpUp()
		return true
	case "down", "j":
		sm.ScrollHelpDown()
		return true
	case "home":
		sm.helpScroll = 0
		return true
	default:
		return false
	}
}

// HandleKeyMsg processes key messages and returns the corresponding action
func (sm *ShortcutManager) HandleKeyMsg(msg tea.KeyMsg, context string) string {
	keyStr := msg.String()
	
	// Special handling for function keys
	switch keyStr {
	case "f1":
		sm.ToggleHelp()
		return "show_help"
	case "f2":
		return "rename_conversation"
	case "f3":
		return "search_conversations"
	case "f4":
		return "show_settings"
	case "f5":
		return "refresh"
	case "f12":
		return "show_debug"
	}
	
	// Handle other shortcuts based on context
	switch context {
	case "global":
		return sm.handleGlobalShortcuts(msg)
	case "conversation":
		return sm.handleConversationShortcuts(msg)
	case "input":
		return sm.handleInputShortcuts(msg)
	}
	
	return ""
}

// handleGlobalShortcuts handles shortcuts available in any context
func (sm *ShortcutManager) handleGlobalShortcuts(msg tea.KeyMsg) string {
	switch {
	case key.Matches(msg, sm.keyMap.ShowHelp):
		sm.ToggleHelp()
		return "show_help"
	case key.Matches(msg, sm.keyMap.ShowSettings):
		return "show_settings"
	case key.Matches(msg, sm.keyMap.Quit):
		return "quit"
	case key.Matches(msg, sm.keyMap.ForceQuit):
		return "force_quit"
	case key.Matches(msg, sm.keyMap.RetryLast):
		return "retry_last"
	case key.Matches(msg, sm.keyMap.QuickSave):
		return "quick_save"
	case key.Matches(msg, sm.keyMap.NextTheme):
		return "next_theme"
	case key.Matches(msg, sm.keyMap.PrevTheme):
		return "prev_theme"
	case key.Matches(msg, sm.keyMap.ToggleCompact):
		return "toggle_compact"
	case key.Matches(msg, sm.keyMap.ImportConv):
		return "import_conversations"
	case key.Matches(msg, sm.keyMap.BackupConv):
		return "backup_conversations"
	case key.Matches(msg, sm.keyMap.RestoreConv):
		return "restore_conversations"
	}
	return ""
}

// handleConversationShortcuts handles shortcuts when in conversation context
func (sm *ShortcutManager) handleConversationShortcuts(msg tea.KeyMsg) string {
	switch {
	case key.Matches(msg, sm.keyMap.NavigateUp):
		return "navigate_up"
	case key.Matches(msg, sm.keyMap.NavigateDown):
		return "navigate_down"
	case key.Matches(msg, sm.keyMap.NavigateHome):
		return "navigate_home"
	case key.Matches(msg, sm.keyMap.NavigateEnd):
		return "navigate_end"
	case key.Matches(msg, sm.keyMap.ToggleExpanded):
		return "toggle_expanded"
	case key.Matches(msg, sm.keyMap.DeleteConv):
		return "delete_conversation"
	case key.Matches(msg, sm.keyMap.ClearAll):
		return "clear_all_conversations"
	case key.Matches(msg, sm.keyMap.ExportConv):
		return "export_conversations"
	case key.Matches(msg, sm.keyMap.FilterConv):
		return "filter_conversations"
	}
	return ""
}

// handleInputShortcuts handles shortcuts when in input context
func (sm *ShortcutManager) handleInputShortcuts(msg tea.KeyMsg) string {
	switch {
	case key.Matches(msg, sm.keyMap.SendMessage):
		return "send_message"
	case key.Matches(msg, sm.keyMap.NewLine):
		return "new_line"
	case key.Matches(msg, sm.keyMap.ClearInput):
		return "clear_input"
	}
	return ""
}

// RenderHelp renders the scrollable help panel
func (sm *ShortcutManager) RenderHelp(width, height int) string {
	if !sm.helpVisible {
		return ""
	}
	
	// Define styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D7FF")).
		Bold(true).
		Align(lipgloss.Center).
		Width(width - 6)
		
	categoryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00")).
		Bold(true).
		MarginTop(1)
		
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FF00")).
		Bold(true).
		Width(15)
		
	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))
		
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00D7FF")).
		Padding(1).
		Width(width - 4).
		Height(height - 4)
	
	// Build all content lines first
	var allLines []string
	
	// Title
	allLines = append(allLines, titleStyle.Render("🚀 gocodenow - Keyboard Shortcuts"))
	allLines = append(allLines, "")
	
	// Group bindings by category
	categories := map[string][]KeyBinding{
		"navigation": {},
		"editing":    {},
		"tools":      {},
		"system":     {},
	}
	
	for _, binding := range sm.bindings {
		categories[binding.Category] = append(categories[binding.Category], binding)
	}
	
	// Render each category
	categoryOrder := []string{"navigation", "editing", "tools", "system"}
	categoryTitles := map[string]string{
		"navigation": "🧭 Navigation",
		"editing":    "✏️  Editing & Conversation",
		"tools":      "🛠️  Tools & Actions",
		"system":     "⚙️  System",
	}
	
	for _, catName := range categoryOrder {
		if len(categories[catName]) == 0 {
			continue
		}
		
		allLines = append(allLines, categoryStyle.Render(categoryTitles[catName]))
		allLines = append(allLines, "")
		
		for _, binding := range categories[catName] {
			keyText := keyStyle.Render(binding.Keys)
			descText := descStyle.Render(binding.Description)
			allLines = append(allLines, keyText + " " + descText)
		}
		allLines = append(allLines, "")
	}
	
	// Footer instructions
	allLines = append(allLines, "")
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true).
		Align(lipgloss.Center).
		Width(width - 6)
	allLines = append(allLines, footerStyle.Render("↑↓/jk: Scroll • Home: Top • F1: Close"))
	
	// Calculate available content height (subtract border and padding)
	availableHeight := height - 6 // border(2) + padding(2) + some margin(2)
	
	// Apply scrolling - determine which lines to show
	startLine := sm.helpScroll
	endLine := startLine + availableHeight
	
	// Clamp scroll position to valid range
	if startLine < 0 {
		startLine = 0
		sm.helpScroll = 0
	}
	if startLine >= len(allLines) {
		startLine = len(allLines) - 1
		if startLine < 0 {
			startLine = 0
		}
		sm.helpScroll = startLine
	}
	
	// Get visible lines
	var visibleLines []string
	for i := startLine; i < endLine && i < len(allLines); i++ {
		visibleLines = append(visibleLines, allLines[i])
	}
	
	// Add scroll indicators if needed
	if startLine > 0 {
		scrollIndicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true).
			Align(lipgloss.Center).
			Width(width - 6)
		visibleLines[0] = scrollIndicator.Render("... (scroll up for more) ...")
	}
	if endLine < len(allLines) {
		scrollIndicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true).
			Align(lipgloss.Center).
			Width(width - 6)
		if len(visibleLines) > 0 {
			visibleLines[len(visibleLines)-1] = scrollIndicator.Render("... (scroll down for more) ...")
		}
	}
	
	// Join visible content
	content := strings.Join(visibleLines, "\n")
	
	// Apply border
	helpPanel := borderStyle.Render(content)
	
	// Center the help panel
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, helpPanel)
}

// GetAvailableActions returns available actions for a given context
func (sm *ShortcutManager) GetAvailableActions(context string) []KeyBinding {
	var actions []KeyBinding
	for _, binding := range sm.bindings {
		if binding.Context == context || binding.Context == "global" {
			actions = append(actions, binding)
		}
	}
	return actions
}

// GetQuickHelp returns a brief help string for the current context
func (sm *ShortcutManager) GetQuickHelp(context string) string {
	quickHelp := map[string]string{
		"global":       "F1:Help • Tab:Focus • Ctrl+C:Quit",
		"conversation": "↑↓:Navigate • Space:Expand • Del:Delete • ?:Help",
		"input":        "Enter:Send • Ctrl+J:NewLine • Ctrl+U:Clear • Tab:Focus",
		"dialog":       "Tab:Navigate • Enter:Select • Esc:Cancel",
	}
	
	if help, exists := quickHelp[context]; exists {
		return help
	}
	return quickHelp["global"]
}

// ShortcutAction represents an action triggered by a keyboard shortcut
type ShortcutAction struct {
	Action  string
	Context string
	Data    interface{}
}

// ShortcutActionMsg is the message sent when a shortcut action is triggered
type ShortcutActionMsg ShortcutAction