package ui

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// ColorPalette defines a set of colors for the theme
type ColorPalette struct {
	Primary     string `json:"primary"`     // Main accent color
	Secondary   string `json:"secondary"`   // Secondary accent color
	Background  string `json:"background"`  // Background color
	Surface     string `json:"surface"`     // Surface color (dialogs, panels)
	OnPrimary   string `json:"onPrimary"`   // Text on primary color
	OnSecondary string `json:"onSecondary"` // Text on secondary color
	OnSurface   string `json:"onSurface"`   // Text on surface
	Text        string `json:"text"`        // Regular text color
	TextMuted   string `json:"textMuted"`   // Muted text color
	Success     string `json:"success"`     // Success color
	Warning     string `json:"warning"`     // Warning color
	Error       string `json:"error"`       // Error color
	Info        string `json:"info"`        // Info color
	Border      string `json:"border"`      // Border color
}

// ThemeColors represents the complete color scheme
type ThemeColors struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Author      string       `json:"author"`
	Version     string       `json:"version"`
	Colors      ColorPalette `json:"colors"`
}

// LayoutSettings defines layout preferences
type LayoutSettings struct {
	CompactMode      bool `json:"compactMode"`      // Reduce spacing and padding
	ShowIcons        bool `json:"showIcons"`        // Show emoji/Unicode icons
	ShowProgressBars bool `json:"showProgressBars"` // Show progress indicators
	ShowTimestamps   bool `json:"showTimestamps"`   // Show message timestamps
	MaxContentWidth  int  `json:"maxContentWidth"`  // Maximum width for content
	SidebarWidth     int  `json:"sidebarWidth"`     // Width of any sidebar
}

// AnimationSettings controls UI animations
type AnimationSettings struct {
	EnableAnimations bool `json:"enableAnimations"` // Enable all animations
	SpinnerStyle     int  `json:"spinnerStyle"`     // Spinner animation style (0-4)
	TransitionSpeed  int  `json:"transitionSpeed"`  // Speed of transitions (1-10)
	BlinkCursor      bool `json:"blinkCursor"`      // Enable cursor blinking
}

// AccessibilitySettings for better accessibility
type AccessibilitySettings struct {
	HighContrast     bool `json:"highContrast"`     // Use high contrast colors
	ReduceMotion     bool `json:"reduceMotion"`     // Reduce animations for motion sensitivity
	LargeFonts       bool `json:"largeFonts"`       // Use larger font indicators
	ScreenReaderMode bool `json:"screenReaderMode"` // Optimize for screen readers
	SoundIndicators  bool `json:"soundIndicators"`  // Enable sound feedback (future)
}

// Theme represents a complete theme configuration
type Theme struct {
	Colors        ThemeColors            `json:"colors"`
	Layout        LayoutSettings         `json:"layout"`
	Animations    AnimationSettings      `json:"animations"`
	Accessibility AccessibilitySettings  `json:"accessibility"`
	Styles        map[string]lipgloss.Style `json:"-"` // Computed styles (not serialized)
}

// ThemeManager manages themes and their application
type ThemeManager struct {
	currentTheme    *Theme
	availableThemes map[string]*Theme
	configDir       string
	themesDir       string
}

// NewThemeManager creates a new theme manager
func NewThemeManager(configDir string) *ThemeManager {
	themesDir := filepath.Join(configDir, "themes")
	
	tm := &ThemeManager{
		availableThemes: make(map[string]*Theme),
		configDir:       configDir,
		themesDir:       themesDir,
	}
	
	// Initialize with default themes
	tm.initializeDefaultThemes()
	
	// Load custom themes from disk
	tm.loadCustomThemes()
	
	// Set default theme
	tm.currentTheme = tm.availableThemes["default"]
	
	return tm
}

// initializeDefaultThemes creates the built-in themes
func (tm *ThemeManager) initializeDefaultThemes() {
	// Default theme (similar to current styling)
	defaultTheme := &Theme{
		Colors: ThemeColors{
			Name:        "default",
			Description: "Default lmcodenow theme with blue accents",
			Author:      "lmcodenow",
			Version:     "1.0.0",
			Colors: ColorPalette{
				Primary:     "#00D7FF", // Bright cyan
				Secondary:   "#0066FF", // Blue
				Background:  "#000000", // Black
				Surface:     "#111111", // Dark gray
				OnPrimary:   "#000000", // Black text on cyan
				OnSecondary: "#FFFFFF", // White text on blue
				OnSurface:   "#FFFFFF", // White text on surface
				Text:        "#FFFFFF", // White text
				TextMuted:   "#888888", // Gray text
				Success:     "#00FF00", // Green
				Warning:     "#FFFF00", // Yellow
				Error:       "#FF0000", // Red
				Info:        "#00D7FF", // Cyan
				Border:      "#444444", // Dark gray border
			},
		},
		Layout: LayoutSettings{
			CompactMode:      false,
			ShowIcons:        true,
			ShowProgressBars: true,
			ShowTimestamps:   true,
			MaxContentWidth:  120,
			SidebarWidth:     30,
		},
		Animations: AnimationSettings{
			EnableAnimations: true,
			SpinnerStyle:     0,
			TransitionSpeed:  5,
			BlinkCursor:      true,
		},
		Accessibility: AccessibilitySettings{
			HighContrast:     false,
			ReduceMotion:     false,
			LargeFonts:       false,
			ScreenReaderMode: false,
			SoundIndicators:  false,
		},
	}
	
	// Dark theme (darker variant)
	darkTheme := &Theme{
		Colors: ThemeColors{
			Name:        "dark",
			Description: "Dark theme with purple accents",
			Author:      "lmcodenow",
			Version:     "1.0.0",
			Colors: ColorPalette{
				Primary:     "#AA00FF", // Purple
				Secondary:   "#6600CC", // Dark purple
				Background:  "#000000", // Black
				Surface:     "#0A0A0A", // Very dark gray
				OnPrimary:   "#FFFFFF", // White text on purple
				OnSecondary: "#FFFFFF", // White text on dark purple
				OnSurface:   "#CCCCCC", // Light gray text on surface
				Text:        "#DDDDDD", // Light gray text
				TextMuted:   "#666666", // Darker gray text
				Success:     "#00AA00", // Darker green
				Warning:     "#CCCC00", // Darker yellow
				Error:       "#CC0000", // Darker red
				Info:        "#AA00FF", // Purple
				Border:      "#333333", // Darker border
			},
		},
		Layout:        defaultTheme.Layout,
		Animations:    defaultTheme.Animations,
		Accessibility: defaultTheme.Accessibility,
	}
	
	// Light theme
	lightTheme := &Theme{
		Colors: ThemeColors{
			Name:        "light",
			Description: "Light theme with blue accents",
			Author:      "lmcodenow",
			Version:     "1.0.0",
			Colors: ColorPalette{
				Primary:     "#0066FF", // Blue
				Secondary:   "#0044CC", // Darker blue
				Background:  "#FFFFFF", // White
				Surface:     "#F8F8F8", // Light gray
				OnPrimary:   "#FFFFFF", // White text on blue
				OnSecondary: "#FFFFFF", // White text on darker blue
				OnSurface:   "#333333", // Dark text on light surface
				Text:        "#000000", // Black text
				TextMuted:   "#666666", // Gray text
				Success:     "#00AA00", // Green
				Warning:     "#FF8800", // Orange
				Error:       "#CC0000", // Red
				Info:        "#0066FF", // Blue
				Border:      "#CCCCCC", // Light gray border
			},
		},
		Layout:        defaultTheme.Layout,
		Animations:    defaultTheme.Animations,
		Accessibility: defaultTheme.Accessibility,
	}
	
	// High contrast theme for accessibility
	highContrastTheme := &Theme{
		Colors: ThemeColors{
			Name:        "high-contrast",
			Description: "High contrast theme for accessibility",
			Author:      "lmcodenow",
			Version:     "1.0.0",
			Colors: ColorPalette{
				Primary:     "#FFFF00", // Bright yellow
				Secondary:   "#00FFFF", // Bright cyan
				Background:  "#000000", // Black
				Surface:     "#000000", // Black
				OnPrimary:   "#000000", // Black text on yellow
				OnSecondary: "#000000", // Black text on cyan
				OnSurface:   "#FFFFFF", // White text on black
				Text:        "#FFFFFF", // White text
				TextMuted:   "#CCCCCC", // Light gray text
				Success:     "#00FF00", // Bright green
				Warning:     "#FFFF00", // Bright yellow
				Error:       "#FF0000", // Bright red
				Info:        "#00FFFF", // Bright cyan
				Border:      "#FFFFFF", // White border
			},
		},
		Layout: LayoutSettings{
			CompactMode:      false,
			ShowIcons:        false, // Icons can be confusing in high contrast
			ShowProgressBars: true,
			ShowTimestamps:   true,
			MaxContentWidth:  100,
			SidebarWidth:     25,
		},
		Animations: AnimationSettings{
			EnableAnimations: false, // Reduce distractions
			SpinnerStyle:     0,
			TransitionSpeed:  1,
			BlinkCursor:      true,
		},
		Accessibility: AccessibilitySettings{
			HighContrast:     true,
			ReduceMotion:     true,
			LargeFonts:       true,
			ScreenReaderMode: false,
			SoundIndicators:  false,
		},
	}
	
	// Compact theme for smaller terminals
	compactTheme := &Theme{
		Colors:        defaultTheme.Colors,
		Animations:    defaultTheme.Animations,
		Accessibility: defaultTheme.Accessibility,
		Layout: LayoutSettings{
			CompactMode:      true,
			ShowIcons:        false,
			ShowProgressBars: false,
			ShowTimestamps:   false,
			MaxContentWidth:  80,
			SidebarWidth:     20,
		},
	}
	compactTheme.Colors.Name = "compact"
	compactTheme.Colors.Description = "Compact theme for smaller terminals"
	
	// Register all themes
	tm.availableThemes["default"] = defaultTheme
	tm.availableThemes["dark"] = darkTheme
	tm.availableThemes["light"] = lightTheme
	tm.availableThemes["high-contrast"] = highContrastTheme
	tm.availableThemes["compact"] = compactTheme
	
	// Compute styles for all themes
	for _, theme := range tm.availableThemes {
		tm.computeStyles(theme)
	}
}

// loadCustomThemes loads custom themes from the themes directory
func (tm *ThemeManager) loadCustomThemes() {
	if _, err := os.Stat(tm.themesDir); os.IsNotExist(err) {
		return // No themes directory, skip
	}
	
	files, err := ioutil.ReadDir(tm.themesDir)
	if err != nil {
		return // Can't read directory, skip
	}
	
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			themePath := filepath.Join(tm.themesDir, file.Name())
			if theme := tm.loadThemeFromFile(themePath); theme != nil {
				tm.computeStyles(theme)
				tm.availableThemes[theme.Colors.Name] = theme
			}
		}
	}
}

// loadThemeFromFile loads a theme from a JSON file
func (tm *ThemeManager) loadThemeFromFile(filePath string) *Theme {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil
	}
	
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return nil
	}
	
	return &theme
}

// computeStyles pre-computes lipgloss styles for a theme
func (tm *ThemeManager) computeStyles(theme *Theme) {
	colors := theme.Colors.Colors
	layout := theme.Layout
	
	theme.Styles = make(map[string]lipgloss.Style)
	
	// Header styles
	theme.Styles["header"] = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colors.OnPrimary)).
		Background(lipgloss.Color(colors.Primary))
	
	if !layout.CompactMode {
		theme.Styles["header"] = theme.Styles["header"].Padding(0, 1)
	}
	
	// Text styles
	theme.Styles["text"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Text))
	
	theme.Styles["text_muted"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.TextMuted))
	
	theme.Styles["text_bold"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Text)).
		Bold(true)
	
	// Status styles
	theme.Styles["success"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Success)).
		Bold(true)
	
	theme.Styles["warning"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Warning)).
		Bold(true)
	
	theme.Styles["error"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Error)).
		Bold(true)
	
	theme.Styles["info"] = lipgloss.NewStyle().
		Foreground(lipgloss.Color(colors.Info)).
		Bold(true)
	
	// Border styles
	theme.Styles["border"] = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colors.Border))
	
	theme.Styles["border_primary"] = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colors.Primary))
	
	theme.Styles["border_selected"] = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colors.Secondary))
	
	// Surface styles
	theme.Styles["surface"] = lipgloss.NewStyle().
		Background(lipgloss.Color(colors.Surface)).
		Foreground(lipgloss.Color(colors.OnSurface))
	
	// Button styles
	theme.Styles["button_primary"] = lipgloss.NewStyle().
		Background(lipgloss.Color(colors.Primary)).
		Foreground(lipgloss.Color(colors.OnPrimary)).
		Bold(true)
	
	theme.Styles["button_secondary"] = lipgloss.NewStyle().
		Background(lipgloss.Color(colors.Secondary)).
		Foreground(lipgloss.Color(colors.OnSecondary)).
		Bold(true)
	
	if !layout.CompactMode {
		theme.Styles["button_primary"] = theme.Styles["button_primary"].Padding(0, 2).Margin(0, 1)
		theme.Styles["button_secondary"] = theme.Styles["button_secondary"].Padding(0, 2).Margin(0, 1)
	}
}

// GetCurrentTheme returns the currently active theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.currentTheme
}

// SetTheme changes the active theme
func (tm *ThemeManager) SetTheme(themeName string) error {
	if theme, exists := tm.availableThemes[themeName]; exists {
		tm.currentTheme = theme
		return nil
	}
	return fmt.Errorf("theme '%s' not found", themeName)
}

// GetAvailableThemes returns a list of available theme names
func (tm *ThemeManager) GetAvailableThemes() []string {
	var themes []string
	for name := range tm.availableThemes {
		themes = append(themes, name)
	}
	return themes
}

// GetThemeInfo returns information about a specific theme
func (tm *ThemeManager) GetThemeInfo(themeName string) (*ThemeColors, error) {
	if theme, exists := tm.availableThemes[themeName]; exists {
		return &theme.Colors, nil
	}
	return nil, fmt.Errorf("theme '%s' not found", themeName)
}

// SaveTheme saves a theme to disk
func (tm *ThemeManager) SaveTheme(theme *Theme) error {
	// Ensure themes directory exists
	if err := os.MkdirAll(tm.themesDir, 0755); err != nil {
		return fmt.Errorf("failed to create themes directory: %w", err)
	}
	
	// Serialize theme to JSON
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize theme: %w", err)
	}
	
	// Write to file
	filename := fmt.Sprintf("%s.json", theme.Colors.Name)
	filePath := filepath.Join(tm.themesDir, filename)
	
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write theme file: %w", err)
	}
	
	// Add to available themes and compute styles
	tm.computeStyles(theme)
	tm.availableThemes[theme.Colors.Name] = theme
	
	return nil
}

// CreateCustomTheme creates a new theme based on an existing one
func (tm *ThemeManager) CreateCustomTheme(baseName, newName, description string) (*Theme, error) {
	baseTheme, exists := tm.availableThemes[baseName]
	if !exists {
		return nil, fmt.Errorf("base theme '%s' not found", baseName)
	}
	
	// Create a deep copy of the base theme
	customTheme := *baseTheme
	customTheme.Colors.Name = newName
	customTheme.Colors.Description = description
	customTheme.Colors.Author = "Custom"
	customTheme.Colors.Version = "1.0.0"
	
	return &customTheme, nil
}

// ApplyAccessibilitySettings applies accessibility overrides to the current theme
func (tm *ThemeManager) ApplyAccessibilitySettings(settings AccessibilitySettings) {
	if tm.currentTheme == nil {
		return
	}
	
	// Update accessibility settings
	tm.currentTheme.Accessibility = settings
	
	// Apply accessibility overrides if high contrast is enabled
	if settings.HighContrast {
		colors := &tm.currentTheme.Colors.Colors
		colors.Background = "#000000"
		colors.Text = "#FFFFFF"
		colors.TextMuted = "#CCCCCC"
		colors.Border = "#FFFFFF"
		colors.Primary = "#FFFF00"
		colors.Secondary = "#00FFFF"
	}
	
	// Disable animations if motion should be reduced
	if settings.ReduceMotion {
		tm.currentTheme.Animations.EnableAnimations = false
		tm.currentTheme.Animations.TransitionSpeed = 1
	}
	
	// Recompute styles with new settings
	tm.computeStyles(tm.currentTheme)
}

// GetStyle returns a pre-computed style by name
func (tm *ThemeManager) GetStyle(styleName string) lipgloss.Style {
	if tm.currentTheme == nil || tm.currentTheme.Styles == nil {
		return lipgloss.NewStyle() // Return empty style as fallback
	}
	
	if style, exists := tm.currentTheme.Styles[styleName]; exists {
		return style
	}
	
	return lipgloss.NewStyle() // Return empty style as fallback
}

// GetColor returns a color from the current theme
func (tm *ThemeManager) GetColor(colorName string) string {
	if tm.currentTheme == nil {
		return "#FFFFFF" // Fallback to white
	}
	
	colors := tm.currentTheme.Colors.Colors
	
	switch colorName {
	case "primary":
		return colors.Primary
	case "secondary":
		return colors.Secondary
	case "background":
		return colors.Background
	case "surface":
		return colors.Surface
	case "text":
		return colors.Text
	case "text_muted":
		return colors.TextMuted
	case "success":
		return colors.Success
	case "warning":
		return colors.Warning
	case "error":
		return colors.Error
	case "info":
		return colors.Info
	case "border":
		return colors.Border
	default:
		return "#FFFFFF" // Fallback
	}
}

// IsCompactMode returns whether compact mode is enabled
func (tm *ThemeManager) IsCompactMode() bool {
	if tm.currentTheme == nil {
		return false
	}
	return tm.currentTheme.Layout.CompactMode
}

// IsAnimationEnabled returns whether animations are enabled
func (tm *ThemeManager) IsAnimationEnabled() bool {
	if tm.currentTheme == nil {
		return true
	}
	return tm.currentTheme.Animations.EnableAnimations
}

// GetSpinnerStyle returns the current spinner style
func (tm *ThemeManager) GetSpinnerStyle() int {
	if tm.currentTheme == nil {
		return 0
	}
	return tm.currentTheme.Animations.SpinnerStyle
}

// ShouldShowIcons returns whether icons should be displayed
func (tm *ThemeManager) ShouldShowIcons() bool {
	if tm.currentTheme == nil {
		return true
	}
	return tm.currentTheme.Layout.ShowIcons
}

// ShouldShowTimestamps returns whether timestamps should be displayed
func (tm *ThemeManager) ShouldShowTimestamps() bool {
	if tm.currentTheme == nil {
		return true
	}
	return tm.currentTheme.Layout.ShowTimestamps
}