// Package tools provides syntax highlighting capabilities for code display
package tools

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// SyntaxHighlighter provides syntax highlighting for various file types
type SyntaxHighlighter struct {
	style     *chroma.Style
	formatter chroma.Formatter
	theme     string
}

// HighlightOptions configures syntax highlighting behavior
type HighlightOptions struct {
	Language     string `json:"language,omitempty"`      // Explicit language override
	Theme        string `json:"theme,omitempty"`          // Color theme (default: "monokai")
	LineNumbers  bool   `json:"line_numbers"`             // Show line numbers
	StartLine    int    `json:"start_line,omitempty"`     // Starting line number
	HighlightLines []int `json:"highlight_lines,omitempty"` // Lines to highlight
	TabWidth     int    `json:"tab_width,omitempty"`      // Tab width (default: 4)
	WrapLines    bool   `json:"wrap_lines"`               // Wrap long lines
	Format       string `json:"format,omitempty"`         // Output format: "terminal", "terminal256", "terminal16m", "html"
}

// DiffHighlightOptions configures diff highlighting
type DiffHighlightOptions struct {
	Theme        string `json:"theme,omitempty"`     // Color theme
	Context      int    `json:"context,omitempty"`   // Context lines around changes
	ShowLineNumbers bool `json:"show_line_numbers"` // Show line numbers
	SideBySide   bool   `json:"side_by_side"`       // Side-by-side diff display
}

// SupportedThemes lists available syntax highlighting themes
var SupportedThemes = []string{
	"monokai",       // Dark theme with vibrant colors (default)
	"dracula",       // Popular dark theme
	"github",        // GitHub's light theme
	"github-dark",   // GitHub's dark theme
	"solarized-dark", // Solarized dark variant
	"solarized-light", // Solarized light variant
	"nord",          // Nord color scheme
	"onedark",       // One Dark theme
	"vs",            // Visual Studio light
	"vscode-dark",   // VS Code dark theme
	"terminal",      // Terminal-friendly theme
	"native",        // Native terminal colors
	"fruity",        // Fruity dark theme
	"autumn",        // Autumn colors theme
	"emacs",         // Emacs default theme
	"vim",           // Vim default theme
	"tango",         // Tango color scheme
	"paraiso-dark",  // Paraiso dark variant
	"paraiso-light", // Paraiso light variant
}

// NewSyntaxHighlighter creates a new syntax highlighter with the specified theme
func NewSyntaxHighlighter(theme string) *SyntaxHighlighter {
	if theme == "" {
		theme = "monokai"
	}

	style := styles.Get(theme)
	if style == nil {
		style = styles.Fallback
	}

	return &SyntaxHighlighter{
		style:     style,
		formatter: formatters.Get("terminal256"),
		theme:     theme,
	}
}

// HighlightCode applies syntax highlighting to code
func (h *SyntaxHighlighter) HighlightCode(code string, opts HighlightOptions) (string, error) {
	// Detect or get lexer
	var lexer chroma.Lexer
	
	if opts.Language != "" {
		lexer = lexers.Get(opts.Language)
	}
	
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	
	if lexer == nil {
		lexer = lexers.Fallback
	}
	
	lexer = chroma.Coalesce(lexer)

	// Configure style
	style := h.style
	if opts.Theme != "" && opts.Theme != h.theme {
		newStyle := styles.Get(opts.Theme)
		if newStyle != nil {
			style = newStyle
		}
	}

	// Configure formatter
	formatter := h.getFormatter(opts.Format)
	
	// Note: Line numbers and highlighting are formatter-specific features
	// For terminal output, we'll handle this differently
	// The chroma v2 API doesn't have these options directly on formatters

	// Tokenize
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", fmt.Errorf("tokenization failed: %v", err)
	}

	// Format
	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return "", fmt.Errorf("formatting failed: %v", err)
	}

	return buf.String(), nil
}

// HighlightFile highlights a file's content based on its extension
func (h *SyntaxHighlighter) HighlightFile(content, filePath string, opts HighlightOptions) (string, error) {
	// Auto-detect language from file extension if not specified
	if opts.Language == "" {
		opts.Language = h.detectLanguageFromPath(filePath)
	}

	return h.HighlightCode(content, opts)
}

// HighlightDiff applies syntax highlighting to a diff
func (h *SyntaxHighlighter) HighlightDiff(diff string, opts DiffHighlightOptions) (string, error) {
	// Get diff lexer
	lexer := lexers.Get("diff")
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// Configure style
	style := h.style
	if opts.Theme != "" && opts.Theme != h.theme {
		newStyle := styles.Get(opts.Theme)
		if newStyle != nil {
			style = newStyle
		}
	}

	// Use terminal formatter for diffs
	formatter := formatters.Get("terminal256")

	// Tokenize
	iterator, err := lexer.Tokenise(nil, diff)
	if err != nil {
		return "", fmt.Errorf("diff tokenization failed: %v", err)
	}

	// Format
	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return "", fmt.Errorf("diff formatting failed: %v", err)
	}

	return buf.String(), nil
}

// HighlightCodeBlock highlights a code block with language hint
func (h *SyntaxHighlighter) HighlightCodeBlock(code, language string) (string, error) {
	opts := HighlightOptions{
		Language:    language,
		LineNumbers: false,
		Format:      "terminal256",
	}
	
	return h.HighlightCode(code, opts)
}

// GetAvailableLanguages returns a list of supported languages
func (h *SyntaxHighlighter) GetAvailableLanguages() []string {
	// Get all lexer names from the registry
	return lexers.Names(true)
}

// GetAvailableThemes returns a list of available themes
func (h *SyntaxHighlighter) GetAvailableThemes() []string {
	return SupportedThemes
}

// SetTheme changes the active theme
func (h *SyntaxHighlighter) SetTheme(theme string) error {
	// Check if theme is in our supported list
	found := false
	for _, supported := range SupportedThemes {
		if supported == theme {
			found = true
			break
		}
	}
	
	if !found {
		return fmt.Errorf("theme '%s' not found", theme)
	}
	
	style := styles.Get(theme)
	if style == nil {
		style = styles.Fallback
	}
	
	h.style = style
	h.theme = theme
	return nil
}

// detectLanguageFromPath detects language from file path
func (h *SyntaxHighlighter) detectLanguageFromPath(filePath string) string {
	// Try to get lexer by filename
	lexer := lexers.Match(filePath)
	if lexer != nil {
		config := lexer.Config()
		if config != nil {
			return config.Name
		}
	}

	// Fallback to extension-based detection
	ext := strings.TrimPrefix(filepath.Ext(filePath), ".")
	
	// Common mappings
	langMap := map[string]string{
		"go":     "Go",
		"js":     "JavaScript",
		"jsx":    "JavaScript",
		"ts":     "TypeScript", 
		"tsx":    "TypeScript",
		"py":     "Python",
		"java":   "Java",
		"cpp":    "C++",
		"cc":     "C++",
		"cxx":    "C++",
		"c":      "C",
		"h":      "C",
		"hpp":    "C++",
		"rs":     "Rust",
		"rb":     "Ruby",
		"php":    "PHP",
		"swift":  "Swift",
		"kt":     "Kotlin",
		"scala":  "Scala",
		"sh":     "Bash",
		"bash":   "Bash",
		"zsh":    "Zsh",
		"fish":   "Fish",
		"ps1":    "PowerShell",
		"sql":    "SQL",
		"html":   "HTML",
		"css":    "CSS",
		"scss":   "SCSS",
		"sass":   "Sass",
		"less":   "Less",
		"xml":    "XML",
		"json":   "JSON",
		"yaml":   "YAML",
		"yml":    "YAML",
		"toml":   "TOML",
		"ini":    "INI",
		"md":     "Markdown",
		"tex":    "TeX",
		"r":      "R",
		"R":      "R",
		"m":      "Objective-C",
		"mm":     "Objective-C++",
		"pl":     "Perl",
		"pm":     "Perl",
		"lua":    "Lua",
		"vim":    "VimL",
		"hs":     "Haskell",
		"clj":    "Clojure",
		"ex":     "Elixir",
		"exs":    "Elixir",
		"erl":    "Erlang",
		"dart":   "Dart",
		"vue":    "Vue",
		"svelte": "Svelte",
	}
	
	if lang, exists := langMap[ext]; exists {
		return lang
	}
	
	return "" // Let chroma auto-detect
}

// getFormatter returns the appropriate formatter based on format string
func (h *SyntaxHighlighter) getFormatter(format string) chroma.Formatter {
	switch format {
	case "terminal":
		return formatters.Get("terminal")
	case "terminal256":
		return formatters.Get("terminal256")
	case "terminal16m":
		return formatters.Get("terminal16m")
	case "html":
		return formatters.Get("html")
	case "svg":
		return formatters.Get("svg")
	default:
		return h.formatter
	}
}

// CreateDiffPreview creates a highlighted diff preview for file changes
func (h *SyntaxHighlighter) CreateDiffPreview(oldContent, newContent string, filePath string) (string, error) {
	// Generate unified diff
	diff := h.generateUnifiedDiff(oldContent, newContent, filePath)
	
	// Highlight the diff
	opts := DiffHighlightOptions{
		ShowLineNumbers: true,
		Context:        3,
	}
	
	return h.HighlightDiff(diff, opts)
}

// generateUnifiedDiff creates a unified diff from old and new content
func (h *SyntaxHighlighter) generateUnifiedDiff(oldContent, newContent, filePath string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")
	
	var diff strings.Builder
	
	// Add diff header
	diff.WriteString(fmt.Sprintf("--- a/%s\n", filePath))
	diff.WriteString(fmt.Sprintf("+++ b/%s\n", filePath))
	
	// Simple diff generation (for demonstration)
	// In production, use a proper diff algorithm
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}
	
	diff.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))
	
	for i := 0; i < maxLines; i++ {
		if i < len(oldLines) && i < len(newLines) {
			if oldLines[i] != newLines[i] {
				diff.WriteString(fmt.Sprintf("-%s\n", oldLines[i]))
				diff.WriteString(fmt.Sprintf("+%s\n", newLines[i]))
			} else {
				diff.WriteString(fmt.Sprintf(" %s\n", oldLines[i]))
			}
		} else if i < len(oldLines) {
			diff.WriteString(fmt.Sprintf("-%s\n", oldLines[i]))
		} else if i < len(newLines) {
			diff.WriteString(fmt.Sprintf("+%s\n", newLines[i]))
		}
	}
	
	return diff.String()
}

// StreamHighlight provides streaming syntax highlighting for large files
func (h *SyntaxHighlighter) StreamHighlight(reader io.Reader, writer io.Writer, opts HighlightOptions) error {
	// Read content
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return fmt.Errorf("failed to read content: %v", err)
	}
	
	// Highlight
	highlighted, err := h.HighlightCode(buf.String(), opts)
	if err != nil {
		return err
	}
	
	// Write highlighted content
	_, err = writer.Write([]byte(highlighted))
	return err
}

// ExtractCodeBlocks extracts and highlights code blocks from markdown or other text
func (h *SyntaxHighlighter) ExtractCodeBlocks(text string) ([]CodeBlock, error) {
	var blocks []CodeBlock
	
	// Pattern for markdown code blocks
	pattern := "```([a-zA-Z0-9_+-]*)\n([^`]+)\n```"
	re := regexp.MustCompile(pattern)
	
	matches := re.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			language := match[1]
			code := match[2]
			
			// Highlight the code block
			highlighted, err := h.HighlightCodeBlock(code, language)
			if err != nil {
				// If highlighting fails, use original
				highlighted = code
			}
			
			blocks = append(blocks, CodeBlock{
				Language:    language,
				Code:        code,
				Highlighted: highlighted,
			})
		}
	}
	
	return blocks, nil
}

// CodeBlock represents an extracted code block
type CodeBlock struct {
	Language    string `json:"language"`
	Code        string `json:"code"`
	Highlighted string `json:"highlighted"`
}

// ThemeInfo provides information about a syntax highlighting theme
type ThemeInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Dark        bool     `json:"dark"`
	Light       bool     `json:"light"`
	Colors      []string `json:"colors,omitempty"`
}

// GetThemeInfo returns information about a specific theme
func (h *SyntaxHighlighter) GetThemeInfo(themeName string) *ThemeInfo {
	themeDescriptions := map[string]*ThemeInfo{
		"monokai": {
			Name:        "Monokai",
			Description: "Dark theme with vibrant colors, popular in Sublime Text",
			Dark:        true,
			Light:       false,
		},
		"dracula": {
			Name:        "Dracula",
			Description: "A dark theme with a purple background",
			Dark:        true,
			Light:       false,
		},
		"github": {
			Name:        "GitHub",
			Description: "GitHub's default light theme",
			Dark:        false,
			Light:       true,
		},
		"github-dark": {
			Name:        "GitHub Dark",
			Description: "GitHub's dark theme",
			Dark:        true,
			Light:       false,
		},
		"solarized-dark": {
			Name:        "Solarized Dark",
			Description: "Solarized color scheme, dark variant",
			Dark:        true,
			Light:       false,
		},
		"solarized-light": {
			Name:        "Solarized Light",
			Description: "Solarized color scheme, light variant",
			Dark:        false,
			Light:       true,
		},
		"nord": {
			Name:        "Nord",
			Description: "An arctic, north-bluish color palette",
			Dark:        true,
			Light:       false,
		},
		"onedark": {
			Name:        "One Dark",
			Description: "Atom's One Dark theme",
			Dark:        true,
			Light:       false,
		},
		"vs": {
			Name:        "Visual Studio",
			Description: "Visual Studio's default light theme",
			Dark:        false,
			Light:       true,
		},
		"vscode-dark": {
			Name:        "VS Code Dark",
			Description: "Visual Studio Code's dark theme",
			Dark:        true,
			Light:       false,
		},
	}
	
	if info, exists := themeDescriptions[themeName]; exists {
		return info
	}
	
	// Default info for unknown themes
	return &ThemeInfo{
		Name:        themeName,
		Description: "Syntax highlighting theme",
		Dark:        false,
		Light:       false,
	}
}