// Package tools provides integration of syntax highlighting with file operations
package tools

import (
	"context"
	"fmt"
	"strings"
	
	"github.com/alecthomas/chroma/v2/lexers"
)

// ReadFileWithSyntaxExecutor extends ReadFileExecutor with syntax highlighting
type ReadFileWithSyntaxExecutor struct {
	*ReadFileExecutor
	highlighter *SyntaxHighlighter
}

// EditFileWithSyntaxExecutor extends EditFileExecutor with diff highlighting
type EditFileWithSyntaxExecutor struct {
	*EditFileExecutor
	highlighter *SyntaxHighlighter
}

// NewReadFileWithSyntaxExecutor creates a new file reader with syntax highlighting
func NewReadFileWithSyntaxExecutor() *ReadFileWithSyntaxExecutor {
	return &ReadFileWithSyntaxExecutor{
		ReadFileExecutor: NewReadFileExecutor(nil), // Uses default security policy
		highlighter:      NewSyntaxHighlighter("monokai"),
	}
}

// NewEditFileWithSyntaxExecutor creates a new file editor with diff highlighting
func NewEditFileWithSyntaxExecutor() *EditFileWithSyntaxExecutor {
	return &EditFileWithSyntaxExecutor{
		EditFileExecutor: NewEditFileExecutor(),
		highlighter:      NewSyntaxHighlighter("monokai"),
	}
}

// Execute reads a file and applies syntax highlighting
func (e *ReadFileWithSyntaxExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// First, execute the base read operation
	result, err := e.ReadFileExecutor.Execute(ctx, params)
	if err != nil {
		return result, err
	}
	
	// Check if syntax highlighting is requested
	enableHighlight := true
	if highlight, ok := params["syntax_highlight"].(bool); ok {
		enableHighlight = highlight
	}
	
	// Get theme preference
	theme := "monokai"
	if themeParam, ok := params["theme"].(string); ok {
		theme = themeParam
	}
	
	// Get line numbers preference
	showLineNumbers := true
	if lineNums, ok := params["show_line_numbers"].(bool); ok {
		showLineNumbers = lineNums
	}
	
	if !enableHighlight {
		return result, nil
	}
	
	// Extract file path from metadata
	filePath := ""
	if metadata, ok := result.Metadata["file_path"].(string); ok {
		filePath = metadata
	}
	
	// Get the content
	content := ""
	if contentStr, ok := result.Result.(string); ok {
		content = contentStr
	}
	
	// Apply syntax highlighting
	if content != "" && filePath != "" {
		opts := HighlightOptions{
			Theme:       theme,
			LineNumbers: showLineNumbers,
			TabWidth:    4,
			Format:      "terminal256",
		}
		
		// Check if specific lines should be highlighted
		if highlightLines, ok := params["highlight_lines"].([]interface{}); ok {
			lines := make([]int, 0, len(highlightLines))
			for _, line := range highlightLines {
				if lineNum, ok := line.(int); ok {
					lines = append(lines, lineNum)
				}
			}
			opts.HighlightLines = lines
		}
		
		// Check for start line
		if startLine, ok := params["start_line"].(int); ok {
			opts.StartLine = startLine
		}
		
		highlighted, err := e.highlighter.HighlightFile(content, filePath, opts)
		if err == nil {
			result.Result = highlighted
			result.Metadata["syntax_highlighted"] = true
			result.Metadata["theme"] = theme
		}
	}
	
	return result, nil
}

// Execute edits a file and provides highlighted diff preview
func (e *EditFileWithSyntaxExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Check if this is a preview request
	preview := false
	if previewParam, ok := params["preview"].(bool); ok {
		preview = previewParam
	}
	
	// Get the original content for diff
	filePath := ""
	if pathParam, ok := params["path"].(string); ok {
		filePath = pathParam
	}
	
	// Note: originalContent could be used for advanced diff generation
	// For now, we rely on the EditFileExecutor's built-in diff generation
	
	// Execute the base edit operation
	result, err := e.EditFileExecutor.Execute(ctx, params)
	if err != nil {
		return result, err
	}
	
	// If preview mode and we have a diff, highlight it
	if preview && result.Success {
		if diffText, ok := result.Metadata["diff"].(string); ok {
			// Apply diff highlighting
			opts := DiffHighlightOptions{
				Theme:           "monokai",
				ShowLineNumbers: true,
				Context:         3,
			}
			
			// Get theme preference
			if themeParam, ok := params["theme"].(string); ok {
				opts.Theme = themeParam
			}
			
			highlightedDiff, err := e.highlighter.HighlightDiff(diffText, opts)
			if err == nil {
				result.Metadata["diff"] = highlightedDiff
				result.Metadata["diff_highlighted"] = true
				
				// Update the result message to include highlighted diff
				result.Result = fmt.Sprintf("Preview of changes to %s:\n\n%s", filePath, highlightedDiff)
			}
		}
	}
	
	return result, nil
}

// ConversationSyntaxHighlighter provides syntax highlighting for conversation display
type ConversationSyntaxHighlighter struct {
	highlighter *SyntaxHighlighter
	theme       string
}

// NewConversationSyntaxHighlighter creates a highlighter for conversation display
func NewConversationSyntaxHighlighter(theme string) *ConversationSyntaxHighlighter {
	if theme == "" {
		theme = "monokai"
	}
	
	return &ConversationSyntaxHighlighter{
		highlighter: NewSyntaxHighlighter(theme),
		theme:       theme,
	}
}

// HighlightConversationContent processes conversation content and highlights code blocks
func (c *ConversationSyntaxHighlighter) HighlightConversationContent(content string) (string, error) {
	// Extract and highlight code blocks
	blocks, err := c.highlighter.ExtractCodeBlocks(content)
	if err != nil {
		return content, err
	}
	
	// Replace code blocks with highlighted versions
	result := content
	for _, block := range blocks {
		// Create the original block pattern
		original := fmt.Sprintf("```%s\n%s\n```", block.Language, block.Code)
		
		// Create the highlighted replacement
		replacement := fmt.Sprintf("```%s\n%s\n```", block.Language, block.Highlighted)
		
		// Replace in the content
		result = strings.Replace(result, original, replacement, 1)
	}
	
	return result, nil
}

// HighlightToolOutput highlights tool output based on the tool type
func (c *ConversationSyntaxHighlighter) HighlightToolOutput(toolName string, output interface{}) (string, error) {
	outputStr := fmt.Sprintf("%v", output)
	
	// Determine highlighting based on tool name
	switch toolName {
	case "read_file", "ReadFile":
		// Attempt to highlight as code
		highlighted, err := c.highlighter.HighlightCode(outputStr, HighlightOptions{
			Theme:       c.theme,
			LineNumbers: false,
			Format:      "terminal256",
		})
		if err == nil {
			return highlighted, nil
		}
		
	case "bash_command", "BashCommand":
		// Highlight as shell output
		highlighted, err := c.highlighter.HighlightCode(outputStr, HighlightOptions{
			Language:    "bash",
			Theme:       c.theme,
			LineNumbers: false,
			Format:      "terminal256",
		})
		if err == nil {
			return highlighted, nil
		}
		
	case "git_diff", "GitDiff":
		// Highlight as diff
		opts := DiffHighlightOptions{
			Theme:           c.theme,
			ShowLineNumbers: false,
		}
		highlighted, err := c.highlighter.HighlightDiff(outputStr, opts)
		if err == nil {
			return highlighted, nil
		}
	}
	
	// Return original if highlighting fails
	return outputStr, nil
}

// SetTheme changes the active theme for conversation highlighting
func (c *ConversationSyntaxHighlighter) SetTheme(theme string) error {
	err := c.highlighter.SetTheme(theme)
	if err != nil {
		return err
	}
	c.theme = theme
	return nil
}

// ThemeManager manages syntax highlighting themes across the application
type ThemeManager struct {
	currentTheme string
	highlighters map[string]*SyntaxHighlighter
}

// NewThemeManager creates a new theme manager
func NewThemeManager(defaultTheme string) *ThemeManager {
	if defaultTheme == "" {
		defaultTheme = "monokai"
	}
	
	return &ThemeManager{
		currentTheme: defaultTheme,
		highlighters: make(map[string]*SyntaxHighlighter),
	}
}

// GetHighlighter returns a highlighter for the current theme
func (m *ThemeManager) GetHighlighter() *SyntaxHighlighter {
	if highlighter, exists := m.highlighters[m.currentTheme]; exists {
		return highlighter
	}
	
	// Create new highlighter for theme
	highlighter := NewSyntaxHighlighter(m.currentTheme)
	m.highlighters[m.currentTheme] = highlighter
	return highlighter
}

// SetTheme changes the active theme
func (m *ThemeManager) SetTheme(theme string) error {
	// Validate theme exists
	testHighlighter := NewSyntaxHighlighter(theme)
	if testHighlighter == nil {
		return fmt.Errorf("theme '%s' not found", theme)
	}
	
	m.currentTheme = theme
	return nil
}

// GetCurrentTheme returns the current active theme
func (m *ThemeManager) GetCurrentTheme() string {
	return m.currentTheme
}

// GetAvailableThemes returns all available themes
func (m *ThemeManager) GetAvailableThemes() []string {
	return SupportedThemes
}

// GetThemeInfo returns information about a specific theme
func (m *ThemeManager) GetThemeInfo(theme string) *ThemeInfo {
	highlighter := NewSyntaxHighlighter(theme)
	return highlighter.GetThemeInfo(theme)
}

// DetectOptimalTheme detects the optimal theme based on terminal capabilities
func (m *ThemeManager) DetectOptimalTheme(isDarkMode bool) string {
	if isDarkMode {
		// Dark themes
		return "monokai"
	} else {
		// Light themes
		return "github"
	}
}

// LanguageDetector provides advanced language detection for syntax highlighting
type LanguageDetector struct {
	highlighter *SyntaxHighlighter
}

// NewLanguageDetector creates a new language detector
func NewLanguageDetector() *LanguageDetector {
	return &LanguageDetector{
		highlighter: NewSyntaxHighlighter("native"),
	}
}

// DetectLanguage detects the programming language of the given code
func (d *LanguageDetector) DetectLanguage(code string) string {
	lexer := lexers.Analyse(code)
	if lexer != nil {
		config := lexer.Config()
		if config != nil {
			return config.Name
		}
	}
	return ""
}

// DetectLanguageFromFile detects language from file path and content
func (d *LanguageDetector) DetectLanguageFromFile(filePath, content string) string {
	// First try by file path
	lexer := lexers.Match(filePath)
	if lexer != nil {
		config := lexer.Config()
		if config != nil {
			return config.Name
		}
	}
	
	// Then try by content
	return d.DetectLanguage(content)
}

// GetLanguageInfo returns information about a programming language
func (d *LanguageDetector) GetLanguageInfo(language string) map[string]interface{} {
	lexer := lexers.Get(language)
	if lexer == nil {
		return nil
	}
	
	config := lexer.Config()
	if config == nil {
		return nil
	}
	
	return map[string]interface{}{
		"name":        config.Name,
		"aliases":     config.Aliases,
		"filenames":   config.Filenames,
		"mime_types":  config.MimeTypes,
	}
}

// IsSupported checks if a language is supported for syntax highlighting
func (d *LanguageDetector) IsSupported(language string) bool {
	lexer := lexers.Get(language)
	return lexer != nil
}