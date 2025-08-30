package context

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultLanguageDetector implements language detection based on file extensions and content analysis
type DefaultLanguageDetector struct {
	languages map[string]*Language
	patterns  map[string][]*regexp.Regexp
}

// NewLanguageDetector creates a new language detector with built-in language definitions
func NewLanguageDetector() *DefaultLanguageDetector {
	detector := &DefaultLanguageDetector{
		languages: make(map[string]*Language),
		patterns:  make(map[string][]*regexp.Regexp),
	}
	
	detector.initializeLanguages()
	return detector
}

// DetectLanguages analyzes files in the workspace and returns detected languages
func (d *DefaultLanguageDetector) DetectLanguages(workspacePath string) ([]Language, error) {
	langStats := make(map[string]*Language)
	totalFiles := 0
	
	err := filepath.WalkDir(workspacePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if entry.IsDir() {
			return nil
		}
		
		// Skip hidden files and common non-source files
		if d.shouldSkipFile(entry.Name()) {
			return nil
		}
		
		totalFiles++
		
		// Analyze the file
		if lang, err := d.AnalyzeFile(path); err == nil && lang != nil {
			if existing, exists := langStats[lang.Name]; exists {
				existing.FileCount++
				existing.LineCount += lang.LineCount
			} else {
				langCopy := *lang
				langCopy.FileCount = 1
				langStats[lang.Name] = &langCopy
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Convert map to slice and calculate confidence
	var languages []Language
	for _, lang := range langStats {
		// Calculate confidence based on file count and total files
		confidence := float64(lang.FileCount) / float64(totalFiles)
		if confidence > 1.0 {
			confidence = 1.0
		}
		
		lang.Confidence = confidence
		
		// Mark primary language (highest file count)
		if len(languages) == 0 || lang.FileCount > languages[0].FileCount {
			// Unmark previous primary
			for i := range languages {
				languages[i].Primary = false
			}
			lang.Primary = true
		}
		
		languages = append(languages, *lang)
	}
	
	// Sort by file count (descending)
	for i := 0; i < len(languages)-1; i++ {
		for j := i + 1; j < len(languages); j++ {
			if languages[j].FileCount > languages[i].FileCount {
				languages[i], languages[j] = languages[j], languages[i]
			}
		}
	}
	
	return languages, nil
}

// AnalyzeFile determines the language of a specific file
func (d *DefaultLanguageDetector) AnalyzeFile(filePath string) (*Language, error) {
	// First try extension-based detection
	ext := strings.ToLower(filepath.Ext(filePath))
	if lang, exists := d.GetLanguageByExtension(ext); exists {
		// Count lines if it's a source file
		if lineCount, err := d.countLines(filePath); err == nil {
			langCopy := *lang
			langCopy.LineCount = lineCount
			return &langCopy, nil
		}
		return lang, nil
	}
	
	// Try content-based detection for files without clear extensions
	if ext == "" || ext == ".txt" {
		return d.detectByContent(filePath)
	}
	
	return nil, nil
}

// GetLanguageByExtension returns language info for a file extension
func (d *DefaultLanguageDetector) GetLanguageByExtension(extension string) (*Language, bool) {
	extension = strings.ToLower(extension)
	if strings.HasPrefix(extension, ".") {
		extension = extension[1:]
	}
	
	for _, lang := range d.languages {
		for _, ext := range lang.Extensions {
			if ext == extension {
				return lang, true
			}
		}
	}
	
	return nil, false
}

// initializeLanguages sets up the built-in language definitions
func (d *DefaultLanguageDetector) initializeLanguages() {
	languages := []Language{
		{
			Name:       "Go",
			Extensions: []string{"go"},
		},
		{
			Name:       "JavaScript",
			Extensions: []string{"js", "mjs", "cjs"},
		},
		{
			Name:       "TypeScript",
			Extensions: []string{"ts", "tsx"},
		},
		{
			Name:       "Python",
			Extensions: []string{"py", "pyx", "pyw", "pyi"},
		},
		{
			Name:       "Java",
			Extensions: []string{"java"},
		},
		{
			Name:       "C",
			Extensions: []string{"c", "h"},
		},
		{
			Name:       "C++",
			Extensions: []string{"cpp", "cxx", "cc", "hpp", "hxx", "hh"},
		},
		{
			Name:       "C#",
			Extensions: []string{"cs", "csx"},
		},
		{
			Name:       "Rust",
			Extensions: []string{"rs"},
		},
		{
			Name:       "Ruby",
			Extensions: []string{"rb", "rbw"},
		},
		{
			Name:       "PHP",
			Extensions: []string{"php", "php3", "php4", "php5", "phtml"},
		},
		{
			Name:       "Swift",
			Extensions: []string{"swift"},
		},
		{
			Name:       "Kotlin",
			Extensions: []string{"kt", "kts"},
		},
		{
			Name:       "Scala",
			Extensions: []string{"scala", "sc"},
		},
		{
			Name:       "R",
			Extensions: []string{"r", "R"},
		},
		{
			Name:       "Dart",
			Extensions: []string{"dart"},
		},
		{
			Name:       "Lua",
			Extensions: []string{"lua"},
		},
		{
			Name:       "Perl",
			Extensions: []string{"pl", "pm", "perl"},
		},
		{
			Name:       "Shell",
			Extensions: []string{"sh", "bash", "zsh", "fish"},
		},
		{
			Name:       "PowerShell",
			Extensions: []string{"ps1", "psm1", "psd1"},
		},
		{
			Name:       "HTML",
			Extensions: []string{"html", "htm", "xhtml"},
		},
		{
			Name:       "CSS",
			Extensions: []string{"css", "scss", "sass", "less"},
		},
		{
			Name:       "SQL",
			Extensions: []string{"sql", "mysql", "pgsql"},
		},
		{
			Name:       "JSON",
			Extensions: []string{"json", "jsonc"},
		},
		{
			Name:       "YAML",
			Extensions: []string{"yaml", "yml"},
		},
		{
			Name:       "XML",
			Extensions: []string{"xml", "xsd", "xsl", "xslt"},
		},
		{
			Name:       "Markdown",
			Extensions: []string{"md", "markdown", "mdown", "mkd"},
		},
		{
			Name:       "Docker",
			Extensions: []string{"dockerfile"},
		},
		{
			Name:       "Vim",
			Extensions: []string{"vim", "vimrc"},
		},
	}
	
	// Store languages by name for quick lookup
	for i := range languages {
		d.languages[languages[i].Name] = &languages[i]
	}
	
	// Initialize content detection patterns
	d.initializePatterns()
}

// initializePatterns sets up regex patterns for content-based detection
func (d *DefaultLanguageDetector) initializePatterns() {
	patterns := map[string][]string{
		"Shell": {
			`^#!/bin/(ba)?sh`,
			`^#!/usr/bin/(ba)?sh`,
			`^#!/bin/zsh`,
			`^#!/usr/bin/env (ba)?sh`,
		},
		"Python": {
			`^#!/usr/bin/env python`,
			`^#!/usr/bin/python`,
			`^# -\*- coding: utf-8 -\*-`,
			`import \w+`,
			`from \w+ import`,
			`def \w+\(.*\):`,
		},
		"JavaScript": {
			`^#!/usr/bin/env node`,
			`require\(['"].*['"]\)`,
			`module\.exports`,
			`function\s+\w+\s*\(`,
		},
		"Go": {
			`package \w+`,
			`import \(`,
			`func \w+\(.*\)`,
		},
	}
	
	// Compile patterns
	for lang, patternStrs := range patterns {
		var compiledPatterns []*regexp.Regexp
		for _, pattern := range patternStrs {
			if compiled, err := regexp.Compile(pattern); err == nil {
				compiledPatterns = append(compiledPatterns, compiled)
			}
		}
		d.patterns[lang] = compiledPatterns
	}
}

// detectByContent analyzes file content to determine language
func (d *DefaultLanguageDetector) detectByContent(filePath string) (*Language, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineCount := 0
	
	// Read first few lines for pattern matching
	var lines []string
	for scanner.Scan() && len(lines) < 10 {
		lines = append(lines, scanner.Text())
		lineCount++
	}
	
	// Continue counting remaining lines
	for scanner.Scan() {
		lineCount++
	}
	
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	
	// Test patterns against content
	for langName, patterns := range d.patterns {
		score := 0
		for _, pattern := range patterns {
			for _, line := range lines {
				if pattern.MatchString(line) {
					score++
				}
			}
		}
		
		// If we found matches, return this language
		if score > 0 {
			if lang, exists := d.languages[langName]; exists {
				langCopy := *lang
				langCopy.LineCount = lineCount
				langCopy.Confidence = float64(score) / float64(len(patterns))
				return &langCopy, nil
			}
		}
	}
	
	return nil, nil
}

// countLines counts the number of lines in a file
func (d *DefaultLanguageDetector) countLines(filePath string) (int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineCount := 0
	
	for scanner.Scan() {
		lineCount++
	}
	
	return lineCount, scanner.Err()
}

// shouldSkipFile determines if a file should be skipped during analysis
func (d *DefaultLanguageDetector) shouldSkipFile(filename string) bool {
	// Skip hidden files
	if strings.HasPrefix(filename, ".") {
		return true
	}
	
	// Skip common binary and generated files
	skipPatterns := []string{
		".exe", ".dll", ".so", ".dylib", ".a", ".lib",
		".bin", ".obj", ".o", ".class", ".jar", ".war",
		".zip", ".tar", ".gz", ".rar", ".7z",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".ico",
		".mp3", ".mp4", ".avi", ".mov", ".wmv", ".flv",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".min.js", ".min.css", ".bundle.js", ".bundle.css",
		"node_modules", ".git", ".svn", ".hg",
		"vendor", "target", "build", "dist", "out",
	}
	
	lowerFilename := strings.ToLower(filename)
	for _, pattern := range skipPatterns {
		if strings.Contains(lowerFilename, pattern) {
			return true
		}
	}
	
	return false
}