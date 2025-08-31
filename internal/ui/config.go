package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gocodenow/internal/config"
)

// ConfigUIMode represents the current mode of the configuration UI
type ConfigUIMode int

const (
	ConfigModeList ConfigUIMode = iota
	ConfigModeEdit
	ConfigModeProfiles
	ConfigModeProfileEdit
)

// ConfigUI represents the configuration user interface
type ConfigUI struct {
	mode            ConfigUIMode
	list            list.Model
	textInput       textinput.Model
	configManager   *config.RuntimeConfigManager
	profileManager  *config.ProfileManager
	currentConfig   *config.Config
	selectedField   string
	editingValue    string
	profileList     []list.Item
	selectedProfile string
	width           int
	height          int
	styles          ConfigStyles
}

// ConfigStyles holds styling for the configuration UI
type ConfigStyles struct {
	Title       lipgloss.Style
	Header      lipgloss.Style
	Selected    lipgloss.Style
	Unselected  lipgloss.Style
	Field       lipgloss.Style
	Value       lipgloss.Style
	Help        lipgloss.Style
	Error       lipgloss.Style
	Success     lipgloss.Style
}

// NewConfigUI creates a new configuration UI
func NewConfigUI(rcm *config.RuntimeConfigManager, pm *config.ProfileManager) *ConfigUI {
	ui := &ConfigUI{
		mode:           ConfigModeList,
		configManager:  rcm,
		profileManager: pm,
		currentConfig:  rcm.GetConfig(),
		styles:         createConfigStyles(),
	}

	ui.initList()
	ui.initTextInput()
	ui.loadProfiles()

	return ui
}

// createConfigStyles creates the styling for the config UI
func createConfigStyles() ConfigStyles {
	return ConfigStyles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true),
		Unselected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		Field: lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Width(30),
		Value: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Width(40),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true),
	}
}

// ConfigItem represents a configuration item in the list
type ConfigItem struct {
	key         string
	title       string
	description string
	value       string
	section     string
}

func (ci ConfigItem) FilterValue() string { return ci.title }
func (ci ConfigItem) Title() string       { return ci.title }
func (ci ConfigItem) Description() string { return ci.description }

// ProfileItem represents a profile item in the list
type ProfileItem struct {
	name        string
	description string
	tags        []string
}

func (pi ProfileItem) FilterValue() string { return pi.name }
func (pi ProfileItem) Title() string       { return pi.name }
func (pi ProfileItem) Description() string { return pi.description }

// initList initializes the configuration list
func (cui *ConfigUI) initList() {
	items := cui.getConfigItems()
	
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(3)

	cui.list = list.New(items, delegate, 80, 20)
	cui.list.Title = "Configuration Settings"
	cui.list.SetShowStatusBar(false)
	cui.list.SetShowHelp(false)
	cui.list.SetFilteringEnabled(true)
}

// initTextInput initializes the text input for editing values
func (cui *ConfigUI) initTextInput() {
	cui.textInput = textinput.New()
	cui.textInput.Focus()
	cui.textInput.CharLimit = 200
}

// getConfigItems returns all configuration items for the list
func (cui *ConfigUI) getConfigItems() []list.Item {
	items := make([]list.Item, 0)

	// LLM Configuration
	items = append(items, ConfigItem{
		key:         "llm.endpoint",
		title:       "LLM Endpoint",
		description: "API endpoint URL for the LLM service",
		value:       cui.currentConfig.LLM.Endpoint,
		section:     "LLM",
	})
	items = append(items, ConfigItem{
		key:         "llm.token",
		title:       "LLM Token",
		description: "Authentication token for the LLM API",
		value:       maskSensitiveValue(cui.currentConfig.LLM.Token),
		section:     "LLM",
	})
	items = append(items, ConfigItem{
		key:         "llm.model",
		title:       "LLM Model",
		description: "Model name to use for completions",
		value:       cui.currentConfig.LLM.Model,
		section:     "LLM",
	})
	items = append(items, ConfigItem{
		key:         "llm.timeout",
		title:       "LLM Timeout",
		description: "Request timeout in seconds",
		value:       fmt.Sprintf("%d", cui.currentConfig.LLM.Timeout),
		section:     "LLM",
	})

	// Storage Configuration
	items = append(items, ConfigItem{
		key:         "storage.database_path",
		title:       "Database Path",
		description: "Path to the SQLite database file",
		value:       cui.currentConfig.Storage.DatabasePath,
		section:     "Storage",
	})
	items = append(items, ConfigItem{
		key:         "storage.cache_size",
		title:       "Cache Size",
		description: "Number of conversations to keep in memory",
		value:       fmt.Sprintf("%d", cui.currentConfig.Storage.CacheSize),
		section:     "Storage",
	})
	items = append(items, ConfigItem{
		key:         "storage.max_memory_mb",
		title:       "Max Memory (MB)",
		description: "Maximum memory usage in megabytes",
		value:       fmt.Sprintf("%d", cui.currentConfig.Storage.MaxMemoryMB),
		section:     "Storage",
	})

	// Security Configuration
	items = append(items, ConfigItem{
		key:         "security.enable_sandbox",
		title:       "Enable Sandbox",
		description: "Run tools in sandboxed environment",
		value:       fmt.Sprintf("%t", cui.currentConfig.Security.EnableSandbox),
		section:     "Security",
	})
	items = append(items, ConfigItem{
		key:         "security.max_file_size",
		title:       "Max File Size",
		description: "Maximum file size for operations (bytes)",
		value:       fmt.Sprintf("%d", cui.currentConfig.Security.MaxFileSize),
		section:     "Security",
	})

	// UI Configuration
	items = append(items, ConfigItem{
		key:         "ui.theme",
		title:       "UI Theme",
		description: "Color theme for the interface",
		value:       cui.currentConfig.UI.Theme,
		section:     "UI",
	})
	items = append(items, ConfigItem{
		key:         "ui.show_timestamps",
		title:       "Show Timestamps",
		description: "Display timestamps in conversations",
		value:       fmt.Sprintf("%t", cui.currentConfig.UI.ShowTimestamps),
		section:     "UI",
	})
	items = append(items, ConfigItem{
		key:         "ui.max_history_size",
		title:       "Max History Size",
		description: "Maximum number of conversations to keep",
		value:       fmt.Sprintf("%d", cui.currentConfig.UI.MaxHistorySize),
		section:     "UI",
	})

	// Tools Configuration
	items = append(items, ConfigItem{
		key:         "tools.max_concurrent",
		title:       "Max Concurrent Tools",
		description: "Maximum number of tools running simultaneously",
		value:       fmt.Sprintf("%d", cui.currentConfig.Tools.MaxConcurrent),
		section:     "Tools",
	})
	items = append(items, ConfigItem{
		key:         "tools.enable_builtins",
		title:       "Enable Built-in Tools",
		description: "Enable built-in tool executors",
		value:       fmt.Sprintf("%t", cui.currentConfig.Tools.EnableBuiltins),
		section:     "Tools",
	})

	return items
}

// loadProfiles loads available profiles into the list
func (cui *ConfigUI) loadProfiles() {
	if cui.profileManager == nil {
		return
	}

	profiles := cui.profileManager.ListProfiles()
	cui.profileList = make([]list.Item, 0, len(profiles))

	for _, profile := range profiles {
		cui.profileList = append(cui.profileList, ProfileItem{
			name:        profile.Name,
			description: profile.Description,
			tags:        profile.Tags,
		})
	}
}

// Update handles Bubble Tea updates
func (cui *ConfigUI) Update(msg tea.Msg) (*ConfigUI, tea.Cmd) {
	var cmd tea.Cmd

	switch cui.mode {
	case ConfigModeList:
		return cui.updateList(msg)
	case ConfigModeEdit:
		return cui.updateEdit(msg)
	case ConfigModeProfiles:
		return cui.updateProfiles(msg)
	case ConfigModeProfileEdit:
		return cui.updateProfileEdit(msg)
	}

	return cui, cmd
}

// updateList handles updates in list mode
func (cui *ConfigUI) updateList(msg tea.Msg) (*ConfigUI, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if selected := cui.list.SelectedItem(); selected != nil {
				if item, ok := selected.(ConfigItem); ok {
					cui.selectedField = item.key
					cui.editingValue = cui.getCurrentValue(item.key)
					cui.textInput.SetValue(cui.editingValue)
					cui.mode = ConfigModeEdit
					return cui, nil
				}
			}
		case "p":
			cui.mode = ConfigModeProfiles
			cui.setupProfileList()
			return cui, nil
		case "s":
			if err := cui.configManager.SaveConfig(); err != nil {
				// Handle error - could show in status
			}
			return cui, nil
		case "r":
			cui.refreshConfig()
			return cui, nil
		}
	case tea.WindowSizeMsg:
		cui.width = msg.Width
		cui.height = msg.Height
		cui.list.SetWidth(msg.Width - 4)
		cui.list.SetHeight(msg.Height - 8)
	}

	var cmd tea.Cmd
	cui.list, cmd = cui.list.Update(msg)
	return cui, cmd
}

// updateEdit handles updates in edit mode
func (cui *ConfigUI) updateEdit(msg tea.Msg) (*ConfigUI, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if err := cui.saveFieldValue(); err != nil {
				// Handle error - could show in status
			}
			cui.mode = ConfigModeList
			cui.refreshList()
			return cui, nil
		case "esc":
			cui.mode = ConfigModeList
			return cui, nil
		}
	}

	var cmd tea.Cmd
	cui.textInput, cmd = cui.textInput.Update(msg)
	cui.editingValue = cui.textInput.Value()
	return cui, cmd
}

// updateProfiles handles updates in profiles mode
func (cui *ConfigUI) updateProfiles(msg tea.Msg) (*ConfigUI, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if selected := cui.list.SelectedItem(); selected != nil {
				if item, ok := selected.(ProfileItem); ok {
					cui.applyProfile(item.name)
				}
			}
			cui.mode = ConfigModeList
			cui.refreshList()
			return cui, nil
		case "esc":
			cui.mode = ConfigModeList
			cui.setupConfigList()
			return cui, nil
		}
	}

	var cmd tea.Cmd
	cui.list, cmd = cui.list.Update(msg)
	return cui, cmd
}

// updateProfileEdit handles updates in profile edit mode
func (cui *ConfigUI) updateProfileEdit(msg tea.Msg) (*ConfigUI, tea.Cmd) {
	// Placeholder for profile editing functionality
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			cui.mode = ConfigModeProfiles
			return cui, nil
		}
	}
	return cui, nil
}

// View renders the configuration UI
func (cui *ConfigUI) View() string {
	switch cui.mode {
	case ConfigModeList:
		return cui.viewList()
	case ConfigModeEdit:
		return cui.viewEdit()
	case ConfigModeProfiles:
		return cui.viewProfiles()
	case ConfigModeProfileEdit:
		return cui.viewProfileEdit()
	}
	return ""
}

// viewList renders the configuration list view
func (cui *ConfigUI) viewList() string {
	header := cui.styles.Title.Render("Configuration Settings")
	
	help := cui.styles.Help.Render(
		"↑/↓: navigate • enter: edit • p: profiles • s: save • r: refresh • q: quit",
	)
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		cui.list.View(),
		help,
	)
	
	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(content)
}

// viewEdit renders the edit field view
func (cui *ConfigUI) viewEdit() string {
	header := cui.styles.Title.Render("Edit Configuration")
	
	fieldInfo := fmt.Sprintf("Editing: %s", cui.selectedField)
	fieldHeader := cui.styles.Header.Render(fieldInfo)
	
	input := cui.textInput.View()
	
	help := cui.styles.Help.Render(
		"enter: save • esc: cancel",
	)
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		fieldHeader,
		"",
		input,
		"",
		help,
	)
	
	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(content)
}

// viewProfiles renders the profiles view
func (cui *ConfigUI) viewProfiles() string {
	header := cui.styles.Title.Render("Configuration Profiles")
	
	help := cui.styles.Help.Render(
		"↑/↓: navigate • enter: apply profile • esc: back",
	)
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		cui.list.View(),
		help,
	)
	
	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(content)
}

// viewProfileEdit renders the profile edit view
func (cui *ConfigUI) viewProfileEdit() string {
	header := cui.styles.Title.Render("Edit Profile")
	
	help := cui.styles.Help.Render(
		"esc: back",
	)
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"Profile editing not yet implemented",
		help,
	)
	
	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(content)
}

// Helper methods

// getCurrentValue gets the current value for a configuration field
func (cui *ConfigUI) getCurrentValue(key string) string {
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return ""
	}

	section, field := parts[0], parts[1]
	_ = section // Mark as used to avoid compiler warning
	config := cui.currentConfig

	switch section {
	case "llm":
		switch field {
		case "endpoint":
			return config.LLM.Endpoint
		case "token":
			return config.LLM.Token
		case "model":
			return config.LLM.Model
		case "timeout":
			return fmt.Sprintf("%d", config.LLM.Timeout)
		}
	case "storage":
		switch field {
		case "database_path":
			return config.Storage.DatabasePath
		case "cache_size":
			return fmt.Sprintf("%d", config.Storage.CacheSize)
		case "max_memory_mb":
			return fmt.Sprintf("%d", config.Storage.MaxMemoryMB)
		}
	case "security":
		switch field {
		case "enable_sandbox":
			return fmt.Sprintf("%t", config.Security.EnableSandbox)
		case "max_file_size":
			return fmt.Sprintf("%d", config.Security.MaxFileSize)
		}
	case "ui":
		switch field {
		case "theme":
			return config.UI.Theme
		case "show_timestamps":
			return fmt.Sprintf("%t", config.UI.ShowTimestamps)
		case "max_history_size":
			return fmt.Sprintf("%d", config.UI.MaxHistorySize)
		}
	case "tools":
		switch field {
		case "max_concurrent":
			return fmt.Sprintf("%d", config.Tools.MaxConcurrent)
		case "enable_builtins":
			return fmt.Sprintf("%t", config.Tools.EnableBuiltins)
		}
	}

	return ""
}

// saveFieldValue saves the edited field value
func (cui *ConfigUI) saveFieldValue() error {
	value := cui.parseValue(cui.selectedField, cui.editingValue)
	
	field := config.ConfigField{
		Path:  cui.selectedField,
		Value: value,
	}
	
	return cui.configManager.UpdateConfigField(field)
}

// parseValue parses a string value to the appropriate type
func (cui *ConfigUI) parseValue(key, value string) interface{} {
	parts := strings.Split(key, ".")
	if len(parts) != 2 {
		return value
	}

	section, field := parts[0], parts[1]
	_ = section // Mark as used to avoid compiler warning

	// Type-specific parsing based on field
	switch {
	case strings.HasSuffix(field, "timeout") || 
		 strings.HasSuffix(field, "size") || 
		 strings.HasSuffix(field, "concurrent") ||
		 strings.HasSuffix(field, "retries"):
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	case strings.HasPrefix(field, "enable_") || 
		 strings.HasPrefix(field, "show_") ||
		 strings.HasPrefix(field, "allow_"):
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	case field == "max_file_size":
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}

	return value
}

// refreshConfig refreshes the current configuration
func (cui *ConfigUI) refreshConfig() {
	cui.currentConfig = cui.configManager.GetConfig()
	cui.refreshList()
}

// refreshList refreshes the configuration list
func (cui *ConfigUI) refreshList() {
	items := cui.getConfigItems()
	cui.list.SetItems(items)
}

// setupConfigList sets up the configuration list
func (cui *ConfigUI) setupConfigList() {
	items := cui.getConfigItems()
	cui.list.SetItems(items)
	cui.list.Title = "Configuration Settings"
}

// setupProfileList sets up the profiles list
func (cui *ConfigUI) setupProfileList() {
	cui.loadProfiles()
	cui.list.SetItems(cui.profileList)
	cui.list.Title = "Configuration Profiles"
}

// applyProfile applies a selected profile
func (cui *ConfigUI) applyProfile(profileName string) error {
	if cui.profileManager == nil {
		return fmt.Errorf("profile manager not available")
	}
	
	err := cui.profileManager.ApplyProfile(profileName, cui.configManager)
	if err == nil {
		cui.refreshConfig()
	}
	return err
}

// maskSensitiveValue masks sensitive configuration values
func maskSensitiveValue(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}