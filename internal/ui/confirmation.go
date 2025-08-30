package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmationType defines different types of confirmations
type ConfirmationType string

const (
	ConfirmationTypeDelete    ConfirmationType = "delete"
	ConfirmationTypeOverwrite ConfirmationType = "overwrite"
	ConfirmationTypeExecute   ConfirmationType = "execute"
	ConfirmationTypeClear     ConfirmationType = "clear"
	ConfirmationTypeReset     ConfirmationType = "reset"
	ConfirmationTypeCancel    ConfirmationType = "cancel"
)

// ConfirmationResult represents the user's choice
type ConfirmationResult string

const (
	ResultConfirm ConfirmationResult = "confirm"
	ResultCancel  ConfirmationResult = "cancel"
	ResultDismiss ConfirmationResult = "dismiss"
)

// ConfirmationDialog represents a confirmation dialog
type ConfirmationDialog struct {
	Type           ConfirmationType
	Title          string
	Message        string
	Details        []string // Additional details to show
	ConfirmText    string   // Custom text for confirm button (default: "Confirm")
	CancelText     string   // Custom text for cancel button (default: "Cancel")
	DefaultToCancel bool    // Whether cancel is the default action
	DangerLevel    int     // 0=info, 1=warning, 2=danger, 3=critical
	Width          int
	Height         int
	active         bool
	selected       int // 0=confirm, 1=cancel
	keyMap         ConfirmationKeyMap
}

// ConfirmationKeyMap defines key bindings for confirmation dialog
type ConfirmationKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
	Left    key.Binding
	Right   key.Binding
	Tab     key.Binding
	Enter   key.Binding
	Escape  key.Binding
}

// DefaultConfirmationKeyMap returns default key bindings
func DefaultConfirmationKeyMap() ConfirmationKeyMap {
	return ConfirmationKeyMap{
		Confirm: key.NewBinding(
			key.WithKeys("y", "Y"),
			key.WithHelp("y", "confirm"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("n", "N"),
			key.WithHelp("n", "cancel"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "cancel"),
		),
	}
}

// NewConfirmationDialog creates a new confirmation dialog
func NewConfirmationDialog(confirmationType ConfirmationType, title, message string) *ConfirmationDialog {
	dialog := &ConfirmationDialog{
		Type:            confirmationType,
		Title:           title,
		Message:         message,
		ConfirmText:     "Confirm",
		CancelText:      "Cancel",
		DefaultToCancel: true,
		DangerLevel:     1,
		Width:           60,
		Height:          12,
		active:          false,
		selected:        1, // Default to cancel
		keyMap:          DefaultConfirmationKeyMap(),
	}

	// Set defaults based on confirmation type
	switch confirmationType {
	case ConfirmationTypeDelete:
		dialog.DangerLevel = 3
		dialog.ConfirmText = "Delete"
		dialog.DefaultToCancel = true
	case ConfirmationTypeOverwrite:
		dialog.DangerLevel = 2
		dialog.ConfirmText = "Overwrite"
		dialog.DefaultToCancel = true
	case ConfirmationTypeExecute:
		dialog.DangerLevel = 1
		dialog.ConfirmText = "Execute"
		dialog.DefaultToCancel = false
	case ConfirmationTypeClear:
		dialog.DangerLevel = 2
		dialog.ConfirmText = "Clear"
		dialog.DefaultToCancel = true
	case ConfirmationTypeReset:
		dialog.DangerLevel = 2
		dialog.ConfirmText = "Reset"
		dialog.DefaultToCancel = true
	case ConfirmationTypeCancel:
		dialog.DangerLevel = 1
		dialog.ConfirmText = "Yes"
		dialog.CancelText = "No"
		dialog.DefaultToCancel = false
	}

	if !dialog.DefaultToCancel {
		dialog.selected = 0 // Default to confirm
	}

	return dialog
}

// Show activates the confirmation dialog
func (cd *ConfirmationDialog) Show() {
	cd.active = true
}

// Hide deactivates the confirmation dialog
func (cd *ConfirmationDialog) Hide() {
	cd.active = false
}

// IsActive returns whether the dialog is currently active
func (cd *ConfirmationDialog) IsActive() bool {
	return cd.active
}

// SetSize sets the dialog dimensions
func (cd *ConfirmationDialog) SetSize(width, height int) {
	cd.Width = width
	cd.Height = height
}

// AddDetails adds additional details to display
func (cd *ConfirmationDialog) AddDetails(details ...string) {
	cd.Details = append(cd.Details, details...)
}

// SetDangerLevel sets the danger level (affects styling)
func (cd *ConfirmationDialog) SetDangerLevel(level int) {
	cd.DangerLevel = level
}

// SetCustomButtons sets custom button text
func (cd *ConfirmationDialog) SetCustomButtons(confirmText, cancelText string) {
	cd.ConfirmText = confirmText
	cd.CancelText = cancelText
}

// Update handles input messages for the confirmation dialog
func (cd *ConfirmationDialog) Update(msg tea.Msg) (*ConfirmationDialog, tea.Cmd, ConfirmationResult) {
	if !cd.active {
		return cd, nil, ""
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, cd.keyMap.Confirm):
			cd.active = false
			return cd, nil, ResultConfirm
		case key.Matches(msg, cd.keyMap.Cancel):
			cd.active = false
			return cd, nil, ResultCancel
		case key.Matches(msg, cd.keyMap.Escape):
			cd.active = false
			return cd, nil, ResultCancel
		case key.Matches(msg, cd.keyMap.Enter):
			cd.active = false
			if cd.selected == 0 {
				return cd, nil, ResultConfirm
			}
			return cd, nil, ResultCancel
		case key.Matches(msg, cd.keyMap.Left):
			cd.selected = 0
		case key.Matches(msg, cd.keyMap.Right):
			cd.selected = 1
		case key.Matches(msg, cd.keyMap.Tab):
			cd.selected = (cd.selected + 1) % 2
		}
	}

	return cd, nil, ""
}

// View renders the confirmation dialog
func (cd *ConfirmationDialog) View() string {
	if !cd.active {
		return ""
	}

	// Define styles based on danger level
	var (
		titleStyle   lipgloss.Style
		messageStyle lipgloss.Style
		buttonStyle  lipgloss.Style
		borderStyle  lipgloss.Style
	)

	switch cd.DangerLevel {
	case 0: // Info
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00D7FF")).Bold(true)
		messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#00D7FF"))
	case 1: // Warning
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00")).Bold(true)
		messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FFFF00"))
	case 2: // Danger
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF8C00")).Bold(true)
		messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FF8C00"))
	case 3: // Critical
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
		messageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		borderStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FF0000"))
	}

	buttonStyle = lipgloss.NewStyle().
		Padding(0, 2).
		Margin(0, 1)

	// Build dialog content
	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render(cd.Title))
	content.WriteString("\n\n")

	// Message
	content.WriteString(messageStyle.Render(cd.Message))
	content.WriteString("\n")

	// Details
	if len(cd.Details) > 0 {
		content.WriteString("\n")
		detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true)
		for _, detail := range cd.Details {
			content.WriteString(detailStyle.Render("• " + detail))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")

	// Buttons
	confirmButton := cd.ConfirmText
	cancelButton := cd.CancelText

	if cd.selected == 0 {
		// Confirm button selected
		confirmStyle := buttonStyle.Copy().
			Background(lipgloss.Color("#0066FF")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		confirmButton = confirmStyle.Render("▶ " + confirmButton + " ◀")
		cancelButton = buttonStyle.Copy().
			Foreground(lipgloss.Color("#888888")).
			Render(cancelButton)
	} else {
		// Cancel button selected
		cancelStyle := buttonStyle.Copy().
			Background(lipgloss.Color("#666666")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)
		cancelButton = cancelStyle.Render("▶ " + cancelButton + " ◀")
		confirmButton = buttonStyle.Copy().
			Foreground(lipgloss.Color("#888888")).
			Render(confirmButton)
	}

	buttonsLine := lipgloss.JoinHorizontal(lipgloss.Center, confirmButton, cancelButton)
	content.WriteString(lipgloss.NewStyle().Width(cd.Width-4).Align(lipgloss.Center).Render(buttonsLine))

	// Help text
	content.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Italic(true)
	helpText := "y/n: quick choice • ←/→/tab: navigate • enter: select • esc: cancel"
	content.WriteString(helpStyle.Render(helpText))

	// Apply border and padding
	dialog := borderStyle.
		Width(cd.Width).
		Height(cd.Height).
		Padding(1).
		Align(lipgloss.Center).
		Render(content.String())

	// Center the dialog
	return lipgloss.Place(80, 24, lipgloss.Center, lipgloss.Center, dialog)
}

// ConfirmationDialogMsg represents messages related to confirmation dialogs
type ConfirmationDialogMsg struct {
	Type   ConfirmationType
	Result ConfirmationResult
	Data   interface{} // Additional data associated with the confirmation
}

// Helper functions for common confirmation dialogs

// ConfirmDelete creates a confirmation dialog for delete operations
func ConfirmDelete(itemName string, details ...string) *ConfirmationDialog {
	dialog := NewConfirmationDialog(
		ConfirmationTypeDelete,
		"Confirm Deletion",
		"Are you sure you want to delete \""+itemName+"\"?",
	)
	dialog.AddDetails(details...)
	return dialog
}

// ConfirmOverwrite creates a confirmation dialog for overwrite operations
func ConfirmOverwrite(fileName string) *ConfirmationDialog {
	dialog := NewConfirmationDialog(
		ConfirmationTypeOverwrite,
		"File Exists",
		"\""+fileName+"\" already exists. Do you want to overwrite it?",
	)
	return dialog
}

// ConfirmExecute creates a confirmation dialog for potentially dangerous command execution
func ConfirmExecute(command string, warnings ...string) *ConfirmationDialog {
	dialog := NewConfirmationDialog(
		ConfirmationTypeExecute,
		"Confirm Command Execution",
		"Execute command: "+command,
	)
	dialog.AddDetails(warnings...)
	return dialog
}

// ConfirmClear creates a confirmation dialog for clearing data
func ConfirmClear(dataType string) *ConfirmationDialog {
	dialog := NewConfirmationDialog(
		ConfirmationTypeClear,
		"Confirm Clear",
		"Are you sure you want to clear all "+dataType+"?",
	)
	dialog.AddDetails("This action cannot be undone.")
	return dialog
}

// ConfirmReset creates a confirmation dialog for reset operations
func ConfirmReset(context string) *ConfirmationDialog {
	dialog := NewConfirmationDialog(
		ConfirmationTypeReset,
		"Confirm Reset",
		"Reset "+context+" to default settings?",
	)
	dialog.AddDetails("Current configuration will be lost.")
	return dialog
}

// ConfirmCancel creates a confirmation dialog for canceling operations
func ConfirmCancel(operation string) *ConfirmationDialog {
	dialog := NewConfirmationDialog(
		ConfirmationTypeCancel,
		"Confirm Cancel",
		"Cancel "+operation+"?",
	)
	return dialog
}