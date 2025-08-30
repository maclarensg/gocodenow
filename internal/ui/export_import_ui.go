package ui

import (
	"fmt"
	"gocodenow/internal/types"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ExportImportMode represents different modes for the export/import UI
type ExportImportMode int

const (
	ExportImportModeNone ExportImportMode = iota
	ExportImportModeExport
	ExportImportModeImport
	ExportImportModeBackup
	ExportImportModeRestore
	ExportImportModeFilters
)

// ExportImportDialog represents the export/import dialog UI
type ExportImportDialog struct {
	mode           ExportImportMode
	visible        bool
	currentStep    int
	totalSteps     int
	
	// Managers
	exportMgr     *ExportManager
	importMgr     *ImportManager
	backupMgr     *BackupManager
	filterPresets *PresetFilters
	
	// UI components
	inputs        []textinput.Model
	selectedFormat ExportFormat
	selectedOptions map[string]bool
	filter          *ConversationFilter
	
	// State
	processing    bool
	result        string
	error         string
	width         int
	height        int
}

// NewExportImportDialog creates a new export/import dialog
func NewExportImportDialog(exportDir, backupDir string) *ExportImportDialog {
	dialog := &ExportImportDialog{
		mode:          ExportImportModeNone,
		visible:       false,
		exportMgr:     NewExportManager(exportDir),
		importMgr:     NewImportManager(),
		backupMgr:     NewBackupManager(backupDir, 10),
		filterPresets: NewPresetFilters(),
		selectedOptions: make(map[string]bool),
		filter:        NewConversationFilter(),
		inputs:        make([]textinput.Model, 5),
	}

	// Initialize text inputs
	for i := 0; i < len(dialog.inputs); i++ {
		dialog.inputs[i] = textinput.New()
		dialog.inputs[i].CharLimit = 256
	}

	return dialog
}

// IsVisible returns whether the dialog is visible
func (d *ExportImportDialog) IsVisible() bool {
	return d.visible
}

// ShowExport shows the export dialog
func (d *ExportImportDialog) ShowExport() {
	d.mode = ExportImportModeExport
	d.visible = true
	d.currentStep = 0
	d.totalSteps = 3
	d.setupExportInputs()
	d.resetState()
}

// ShowImport shows the import dialog
func (d *ExportImportDialog) ShowImport() {
	d.mode = ExportImportModeImport
	d.visible = true
	d.currentStep = 0
	d.totalSteps = 2
	d.setupImportInputs()
	d.resetState()
}

// ShowBackup shows the backup dialog
func (d *ExportImportDialog) ShowBackup() {
	d.mode = ExportImportModeBackup
	d.visible = true
	d.currentStep = 0
	d.totalSteps = 1
	d.setupBackupInputs()
	d.resetState()
}

// ShowRestore shows the restore dialog
func (d *ExportImportDialog) ShowRestore() {
	d.mode = ExportImportModeRestore
	d.visible = true
	d.currentStep = 0
	d.totalSteps = 2
	d.setupRestoreInputs()
	d.resetState()
}

// ShowFilters shows the filter dialog
func (d *ExportImportDialog) ShowFilters() {
	d.mode = ExportImportModeFilters
	d.visible = true
	d.currentStep = 0
	d.totalSteps = 1
	d.setupFilterInputs()
	d.resetState()
}

// Hide hides the dialog
func (d *ExportImportDialog) Hide() {
	d.visible = false
	d.mode = ExportImportModeNone
	d.resetState()
}

// resetState resets the dialog state
func (d *ExportImportDialog) resetState() {
	d.processing = false
	d.result = ""
	d.error = ""
	for i := range d.inputs {
		d.inputs[i].SetValue("")
		d.inputs[i].Blur()
	}
	if len(d.inputs) > 0 {
		d.inputs[0].Focus()
	}
}

// SetSize sets the dialog size
func (d *ExportImportDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
	
	// Adjust input widths
	inputWidth := max(30, min(60, width-20))
	for i := range d.inputs {
		d.inputs[i].Width = inputWidth
	}
}

// Update handles dialog updates
func (d *ExportImportDialog) Update(msg interface{}) {
	if !d.visible || d.processing {
		return
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			d.Hide()
		case "enter":
			d.handleEnter()
		case "tab", "shift+tab":
			d.handleTab(msg.String() == "shift+tab")
		case "up", "down":
			d.handleArrowKeys(msg.String() == "up")
		default:
			// Update focused input
			for i := range d.inputs {
				if d.inputs[i].Focused() {
					d.inputs[i], _ = d.inputs[i].Update(msg)
					break
				}
			}
		}
	}
}

// handleEnter processes enter key based on current step and mode
func (d *ExportImportDialog) handleEnter() {
	switch d.mode {
	case ExportImportModeExport:
		if d.currentStep < d.totalSteps-1 {
			d.currentStep++
		} else {
			d.executeExport()
		}
	case ExportImportModeImport:
		if d.currentStep < d.totalSteps-1 {
			d.currentStep++
		} else {
			d.executeImport()
		}
	case ExportImportModeBackup:
		d.executeBackup()
	case ExportImportModeRestore:
		if d.currentStep < d.totalSteps-1 {
			d.currentStep++
		} else {
			d.executeRestore()
		}
	case ExportImportModeFilters:
		d.applyFilters()
	}
}

// handleTab handles tab navigation between inputs
func (d *ExportImportDialog) handleTab(reverse bool) {
	focusedIndex := -1
	for i := range d.inputs {
		if d.inputs[i].Focused() {
			focusedIndex = i
			d.inputs[i].Blur()
			break
		}
	}

	if focusedIndex == -1 {
		focusedIndex = 0
	} else {
		if reverse {
			focusedIndex--
			if focusedIndex < 0 {
				focusedIndex = len(d.inputs) - 1
			}
		} else {
			focusedIndex++
			if focusedIndex >= len(d.inputs) {
				focusedIndex = 0
			}
		}
	}

	d.inputs[focusedIndex].Focus()
}

// handleArrowKeys handles up/down arrow keys for option selection
func (d *ExportImportDialog) handleArrowKeys(up bool) {
	// Handle format selection for export
	if d.mode == ExportImportModeExport && d.currentStep == 1 {
		formats := d.exportMgr.GetSupportedFormats()
		currentIndex := int(d.selectedFormat)
		
		if up {
			currentIndex--
			if currentIndex < 0 {
				currentIndex = len(formats) - 1
			}
		} else {
			currentIndex++
			if currentIndex >= len(formats) {
				currentIndex = 0
			}
		}
		
		d.selectedFormat = formats[currentIndex]
	}
}

// setupExportInputs configures inputs for export mode
func (d *ExportImportDialog) setupExportInputs() {
	d.inputs[0].Placeholder = "Export file path (leave empty for default)"
	d.inputs[1].Placeholder = "Start date (YYYY-MM-DD) - optional"
	d.inputs[2].Placeholder = "End date (YYYY-MM-DD) - optional"
	d.inputs[3].Placeholder = "Model filter (comma-separated) - optional"
	d.inputs[4].Placeholder = "Description - optional"
	
	d.selectedFormat = FormatJSON
	d.selectedOptions["include_tools"] = true
	d.selectedOptions["include_files"] = true
	d.selectedOptions["pretty_print"] = true
}

// setupImportInputs configures inputs for import mode
func (d *ExportImportDialog) setupImportInputs() {
	d.inputs[0].Placeholder = "Import file path"
	d.inputs[1].Placeholder = "Options: validate,overwrite,skip_duplicates (comma-separated)"
}

// setupBackupInputs configures inputs for backup mode
func (d *ExportImportDialog) setupBackupInputs() {
	d.inputs[0].Placeholder = "Backup description (optional)"
}

// setupRestoreInputs configures inputs for restore mode
func (d *ExportImportDialog) setupRestoreInputs() {
	d.inputs[0].Placeholder = "Backup file path"
	d.inputs[1].Placeholder = "Options: validate,overwrite (comma-separated)"
}

// setupFilterInputs configures inputs for filter mode
func (d *ExportImportDialog) setupFilterInputs() {
	d.inputs[0].Placeholder = "Full text search"
	d.inputs[1].Placeholder = "Start date (YYYY-MM-DD)"
	d.inputs[2].Placeholder = "End date (YYYY-MM-DD)"
	d.inputs[3].Placeholder = "Model filter"
	d.inputs[4].Placeholder = "Status filter (pending,executing,completed,error)"
}

// View renders the dialog
func (d *ExportImportDialog) View() string {
	if !d.visible {
		return ""
	}

	var content strings.Builder
	
	// Dialog title
	title := d.getDialogTitle()
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Align(lipgloss.Center).
		Width(d.width - 4)
	
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// Step indicator
	if d.totalSteps > 1 {
		stepIndicator := fmt.Sprintf("Step %d of %d", d.currentStep+1, d.totalSteps)
		stepStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Align(lipgloss.Center).
			Width(d.width - 4)
		content.WriteString(stepStyle.Render(stepIndicator))
		content.WriteString("\n\n")
	}

	// Content based on mode and step
	switch d.mode {
	case ExportImportModeExport:
		content.WriteString(d.renderExportStep())
	case ExportImportModeImport:
		content.WriteString(d.renderImportStep())
	case ExportImportModeBackup:
		content.WriteString(d.renderBackupStep())
	case ExportImportModeRestore:
		content.WriteString(d.renderRestoreStep())
	case ExportImportModeFilters:
		content.WriteString(d.renderFilterStep())
	}

	// Show result or error
	if d.result != "" {
		resultStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("10")).
			Padding(1).
			Margin(1, 0)
		content.WriteString("\n")
		content.WriteString(resultStyle.Render(d.result))
	}

	if d.error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("9")).
			Padding(1).
			Margin(1, 0)
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("Error: "+d.error))
	}

	// Processing indicator
	if d.processing {
		processingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Align(lipgloss.Center).
			Width(d.width - 4)
		content.WriteString("\n")
		content.WriteString(processingStyle.Render("Processing... Please wait"))
	}

	// Instructions
	if !d.processing {
		instructions := d.getInstructions()
		instructStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Align(lipgloss.Center).
			Width(d.width - 4).
			Margin(1, 0)
		content.WriteString("\n")
		content.WriteString(instructStyle.Render(instructions))
	}

	// Border and background
	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("12")).
		Padding(2).
		Width(d.width).
		Height(d.height)

	return dialogStyle.Render(content.String())
}

// getDialogTitle returns the appropriate title for the current mode
func (d *ExportImportDialog) getDialogTitle() string {
	switch d.mode {
	case ExportImportModeExport:
		return "Export Conversations"
	case ExportImportModeImport:
		return "Import Conversations"
	case ExportImportModeBackup:
		return "Create Backup"
	case ExportImportModeRestore:
		return "Restore from Backup"
	case ExportImportModeFilters:
		return "Filter Conversations"
	default:
		return "Export/Import"
	}
}

// getInstructions returns instructions for the current state
func (d *ExportImportDialog) getInstructions() string {
	if d.result != "" || d.error != "" {
		return "Press ESC to close"
	}
	
	switch d.mode {
	case ExportImportModeExport, ExportImportModeImport, ExportImportModeRestore:
		if d.currentStep < d.totalSteps-1 {
			return "Press Enter to continue • Press ESC to cancel"
		}
		return "Press Enter to execute • Press ESC to cancel"
	case ExportImportModeBackup, ExportImportModeFilters:
		return "Press Enter to execute • Press ESC to cancel"
	default:
		return "Press ESC to cancel"
	}
}

// renderExportStep renders the current export step
func (d *ExportImportDialog) renderExportStep() string {
	switch d.currentStep {
	case 0:
		return d.renderExportFileStep()
	case 1:
		return d.renderExportFormatStep()
	case 2:
		return d.renderExportOptionsStep()
	default:
		return ""
	}
}

// renderExportFileStep renders file path input step
func (d *ExportImportDialog) renderExportFileStep() string {
	var content strings.Builder
	content.WriteString("Choose export file path:\n\n")
	content.WriteString(d.inputs[0].View())
	content.WriteString("\n\nLeave empty to use default location with timestamp")
	return content.String()
}

// renderExportFormatStep renders format selection step
func (d *ExportImportDialog) renderExportFormatStep() string {
	var content strings.Builder
	content.WriteString("Select export format:\n\n")
	
	formats := d.exportMgr.GetSupportedFormats()
	for _, format := range formats {
		prefix := "  "
		if format == d.selectedFormat {
			prefix = "▶ "
		}
		content.WriteString(fmt.Sprintf("%s%s\n", prefix, format.String()))
	}
	
	content.WriteString("\nUse ↑/↓ arrows to select")
	return content.String()
}

// renderExportOptionsStep renders options configuration step
func (d *ExportImportDialog) renderExportOptionsStep() string {
	var content strings.Builder
	content.WriteString("Configure export options:\n\n")
	
	// Date range
	content.WriteString("Date range (optional):\n")
	content.WriteString("Start: " + d.inputs[1].View() + "\n")
	content.WriteString("End: " + d.inputs[2].View() + "\n\n")
	
	// Model filter
	content.WriteString("Model filter:\n")
	content.WriteString(d.inputs[3].View())
	content.WriteString("\n\n")
	
	// Options checkboxes (simplified)
	options := []string{"Include Tools", "Include Files", "Pretty Print"}
	keys := []string{"include_tools", "include_files", "pretty_print"}
	
	content.WriteString("Options:\n")
	for i, option := range options {
		checkbox := "☐"
		if d.selectedOptions[keys[i]] {
			checkbox = "☑"
		}
		content.WriteString(fmt.Sprintf("%s %s\n", checkbox, option))
	}
	
	return content.String()
}

// renderImportStep renders the current import step
func (d *ExportImportDialog) renderImportStep() string {
	switch d.currentStep {
	case 0:
		return d.renderImportFileStep()
	case 1:
		return d.renderImportOptionsStep()
	default:
		return ""
	}
}

// renderImportFileStep renders import file selection
func (d *ExportImportDialog) renderImportFileStep() string {
	var content strings.Builder
	content.WriteString("Select file to import:\n\n")
	content.WriteString(d.inputs[0].View())
	content.WriteString("\n\nSupported formats: JSON, Markdown, CSV")
	return content.String()
}

// renderImportOptionsStep renders import options
func (d *ExportImportDialog) renderImportOptionsStep() string {
	var content strings.Builder
	content.WriteString("Configure import options:\n\n")
	content.WriteString(d.inputs[1].View())
	content.WriteString("\n\nAvailable options:")
	content.WriteString("\n• validate - Validate data before import")
	content.WriteString("\n• overwrite - Overwrite existing conversations")
	content.WriteString("\n• skip_duplicates - Skip duplicate conversations")
	return content.String()
}

// renderBackupStep renders backup configuration
func (d *ExportImportDialog) renderBackupStep() string {
	var content strings.Builder
	content.WriteString("Create backup of all conversations:\n\n")
	content.WriteString("Description:\n")
	content.WriteString(d.inputs[0].View())
	content.WriteString("\n\nBackup will be saved as compressed archive (.tar.gz)")
	return content.String()
}

// renderRestoreStep renders restore configuration
func (d *ExportImportDialog) renderRestoreStep() string {
	switch d.currentStep {
	case 0:
		return d.renderRestoreFileStep()
	case 1:
		return d.renderRestoreOptionsStep()
	default:
		return ""
	}
}

// renderRestoreFileStep renders restore file selection
func (d *ExportImportDialog) renderRestoreFileStep() string {
	var content strings.Builder
	content.WriteString("Select backup file to restore:\n\n")
	content.WriteString(d.inputs[0].View())
	content.WriteString("\n\nSelect a .tar.gz backup file")
	return content.String()
}

// renderRestoreOptionsStep renders restore options
func (d *ExportImportDialog) renderRestoreOptionsStep() string {
	var content strings.Builder
	content.WriteString("Configure restore options:\n\n")
	content.WriteString(d.inputs[1].View())
	content.WriteString("\n\nAvailable options:")
	content.WriteString("\n• validate - Validate backup integrity")
	content.WriteString("\n• overwrite - Overwrite existing conversations")
	return content.String()
}

// renderFilterStep renders filter configuration
func (d *ExportImportDialog) renderFilterStep() string {
	var content strings.Builder
	content.WriteString("Configure conversation filters:\n\n")
	
	labels := []string{"Full text search:", "Start date:", "End date:", "Model filter:", "Status filter:"}
	for idx, label := range labels {
		if idx < len(d.inputs) {
			content.WriteString(label + "\n")
			content.WriteString(d.inputs[idx].View() + "\n\n")
		}
	}
	
	return content.String()
}

// executeExport executes the export operation
func (d *ExportImportDialog) executeExport() {
	d.processing = true
	// Implementation would go here - this is a placeholder
	d.result = "Export functionality ready to implement"
	d.processing = false
}

// executeImport executes the import operation
func (d *ExportImportDialog) executeImport() {
	d.processing = true
	// Implementation would go here - this is a placeholder
	d.result = "Import functionality ready to implement"
	d.processing = false
}

// executeBackup executes the backup operation
func (d *ExportImportDialog) executeBackup() {
	d.processing = true
	// Implementation would go here - this is a placeholder
	d.result = "Backup functionality ready to implement"
	d.processing = false
}

// executeRestore executes the restore operation
func (d *ExportImportDialog) executeRestore() {
	d.processing = true
	// Implementation would go here - this is a placeholder
	d.result = "Restore functionality ready to implement"
	d.processing = false
}

// applyFilters applies the configured filters
func (d *ExportImportDialog) applyFilters() {
	d.processing = true
	
	// Parse filter inputs
	filter := NewConversationFilter()
	
	// Full text search
	if d.inputs[0].Value() != "" {
		filter.WithFullTextSearch(d.inputs[0].Value(), false)
	}
	
	// Date range
	if d.inputs[1].Value() != "" {
		if startDate, err := time.Parse("2006-01-02", d.inputs[1].Value()); err == nil {
			filter.WithDateRange(&startDate, nil)
		}
	}
	if d.inputs[2].Value() != "" {
		if endDate, err := time.Parse("2006-01-02", d.inputs[2].Value()); err == nil {
			filter.WithDateRange(nil, &endDate)
		}
	}
	
	// Model filter
	if d.inputs[3].Value() != "" {
		models := strings.Split(d.inputs[3].Value(), ",")
		for i, model := range models {
			models[i] = strings.TrimSpace(model)
		}
		filter.WithModels(models...)
	}
	
	// Status filter
	if d.inputs[4].Value() != "" {
		statusStrs := strings.Split(d.inputs[4].Value(), ",")
		var statuses []types.ConversationStatus
		for _, statusStr := range statusStrs {
			status := types.ParseConversationStatus(strings.ToLower(strings.TrimSpace(statusStr)))
			statuses = append(statuses, status)
		}
		filter.WithStatuses(statuses...)
	}
	
	d.filter = filter
	d.result = "Filters applied successfully"
	d.processing = false
}

// GetCurrentFilter returns the currently configured filter
func (d *ExportImportDialog) GetCurrentFilter() *ConversationFilter {
	return d.filter
}