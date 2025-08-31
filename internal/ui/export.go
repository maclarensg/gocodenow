package ui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"gocodenow/internal/types"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportFormat represents different export formats
type ExportFormat int

const (
	FormatJSON ExportFormat = iota
	FormatMarkdown
	FormatCSV
	FormatHTML
)

// String returns the string representation of ExportFormat
func (f ExportFormat) String() string {
	switch f {
	case FormatJSON:
		return "JSON"
	case FormatMarkdown:
		return "Markdown"
	case FormatCSV:
		return "CSV"
	case FormatHTML:
		return "HTML"
	default:
		return "Unknown"
	}
}

// FileExtension returns the file extension for the format
func (f ExportFormat) FileExtension() string {
	switch f {
	case FormatJSON:
		return ".json"
	case FormatMarkdown:
		return ".md"
	case FormatCSV:
		return ".csv"
	case FormatHTML:
		return ".html"
	default:
		return ".txt"
	}
}

// ExportOptions defines options for exporting conversations
type ExportOptions struct {
	Format        ExportFormat
	FilePath      string
	StartDate     *time.Time
	EndDate       *time.Time
	Status        []types.ConversationStatus
	ModelFilter   []string
	IncludeTools  bool
	IncludeFiles  bool
	PrettyPrint   bool
	MetadataOnly  bool
}

// ExportManager handles conversation export and import operations
type ExportManager struct {
	defaultExportDir string
	formats          []ExportFormat
}

// NewExportManager creates a new export manager
func NewExportManager(defaultExportDir string) *ExportManager {
	// Create export directory if it doesn't exist
	if err := os.MkdirAll(defaultExportDir, 0755); err != nil {
		defaultExportDir = "." // Fallback to current directory
	}

	return &ExportManager{
		defaultExportDir: defaultExportDir,
		formats: []ExportFormat{
			FormatJSON,
			FormatMarkdown,
			FormatCSV,
			FormatHTML,
		},
	}
}

// GetSupportedFormats returns all supported export formats
func (em *ExportManager) GetSupportedFormats() []ExportFormat {
	return em.formats
}

// ExportConversations exports conversations based on the provided options
func (em *ExportManager) ExportConversations(conversations []types.ConversationBlock, options ExportOptions) error {
	// Filter conversations based on options
	filtered := em.filterConversations(conversations, options)
	
	if len(filtered) == 0 {
		return fmt.Errorf("no conversations match the export criteria")
	}

	// Generate filename if not provided
	if options.FilePath == "" {
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		filename := fmt.Sprintf("gocodenow_export_%s%s", timestamp, options.Format.FileExtension())
		options.FilePath = filepath.Join(em.defaultExportDir, filename)
	}

	// Export based on format
	switch options.Format {
	case FormatJSON:
		return em.exportToJSON(filtered, options)
	case FormatMarkdown:
		return em.exportToMarkdown(filtered, options)
	case FormatCSV:
		return em.exportToCSV(filtered, options)
	case FormatHTML:
		return em.exportToHTML(filtered, options)
	default:
		return fmt.Errorf("unsupported export format: %s", options.Format.String())
	}
}

// filterConversations filters conversations based on export options
func (em *ExportManager) filterConversations(conversations []types.ConversationBlock, options ExportOptions) []types.ConversationBlock {
	var filtered []types.ConversationBlock

	for _, conv := range conversations {
		// Date range filter
		if options.StartDate != nil && conv.Timestamp.Before(*options.StartDate) {
			continue
		}
		if options.EndDate != nil && conv.Timestamp.After(*options.EndDate) {
			continue
		}

		// Status filter
		if len(options.Status) > 0 {
			statusMatch := false
			for _, status := range options.Status {
				if conv.Status == status {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				continue
			}
		}

		// Model filter
		if len(options.ModelFilter) > 0 {
			modelMatch := false
			for _, model := range options.ModelFilter {
				if strings.Contains(strings.ToLower(conv.ModelName), strings.ToLower(model)) {
					modelMatch = true
					break
				}
			}
			if !modelMatch {
				continue
			}
		}

		filtered = append(filtered, conv)
	}

	return filtered
}

// exportToJSON exports conversations to JSON format
func (em *ExportManager) exportToJSON(conversations []types.ConversationBlock, options ExportOptions) error {
	var data interface{}
	
	if options.MetadataOnly {
		// Export only metadata
		metadata := make([]map[string]interface{}, len(conversations))
		for i, conv := range conversations {
			metadata[i] = map[string]interface{}{
				"id":             conv.ID,
				"timestamp":      conv.Timestamp,
				"status":         conv.Status.String(),
				"model_name":     conv.ModelName,
				"token_usage":    conv.TokenUsage,
				"execution_time": conv.ExecutionTime,
				"tool_count":     len(conv.ToolCalls),
				"file_count":     len(conv.FileOperations),
			}
		}
		data = metadata
	} else {
		// Export full conversations
		exportData := struct {
			ExportedAt     time.Time                    `json:"exported_at"`
			Version        string                       `json:"version"`
			Format         string                       `json:"format"`
			Count          int                          `json:"count"`
			Conversations  []types.ConversationBlock    `json:"conversations"`
			Options        ExportOptions                `json:"options"`
		}{
			ExportedAt:    time.Now(),
			Version:       "1.0",
			Format:        "gocodenow-json",
			Count:         len(conversations),
			Conversations: conversations,
			Options:       options,
		}
		data = exportData
	}

	file, err := os.Create(options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if options.PrettyPrint {
		encoder.SetIndent("", "  ")
	}

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// exportToMarkdown exports conversations to Markdown format
func (em *ExportManager) exportToMarkdown(conversations []types.ConversationBlock, options ExportOptions) error {
	file, err := os.Create(options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "# GoCodeNow Conversation Export\n\n")
	fmt.Fprintf(file, "**Exported:** %s  \n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "**Count:** %d conversations  \n\n", len(conversations))

	// Write conversations
	for i, conv := range conversations {
		fmt.Fprintf(file, "## Conversation %d\n\n", i+1)
		fmt.Fprintf(file, "**ID:** `%s`  \n", conv.ID)
		fmt.Fprintf(file, "**Timestamp:** %s  \n", conv.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(file, "**Model:** %s  \n", conv.ModelName)
		fmt.Fprintf(file, "**Status:** %s  \n", conv.Status.String())
		
		if conv.TokenUsage.InputTokens > 0 || conv.TokenUsage.OutputTokens > 0 {
			fmt.Fprintf(file, "**Token Usage:** %d input, %d output  \n", 
				conv.TokenUsage.InputTokens, conv.TokenUsage.OutputTokens)
		}
		
		if conv.ExecutionTime > 0 {
			fmt.Fprintf(file, "**Execution Time:** %v  \n", conv.ExecutionTime)
		}
		
		fmt.Fprintf(file, "\n### User Input\n\n")
		fmt.Fprintf(file, "```\n%s\n```\n\n", conv.UserInput)
		
		fmt.Fprintf(file, "### Assistant Response\n\n")
		fmt.Fprintf(file, "%s\n\n", conv.LLMResponse)

		// Include tools if requested
		if options.IncludeTools && len(conv.ToolCalls) > 0 {
			fmt.Fprintf(file, "### Tool Calls\n\n")
			for _, tool := range conv.ToolCalls {
				fmt.Fprintf(file, "- **%s** (ID: `%s`) at %s\n", 
					tool.ToolName, tool.ID, tool.Timestamp.Format("15:04:05"))
			}
			fmt.Fprintf(file, "\n")
		}

		// Include files if requested
		if options.IncludeFiles && len(conv.FileOperations) > 0 {
			fmt.Fprintf(file, "### File Operations\n\n")
			for _, fileOp := range conv.FileOperations {
				status := "✅"
				if !fileOp.Success {
					status = "❌"
				}
				fmt.Fprintf(file, "- %s **%s**: `%s` at %s\n", 
					status, fileOp.OperationType, fileOp.FilePath, fileOp.Timestamp.Format("15:04:05"))
			}
			fmt.Fprintf(file, "\n")
		}

		fmt.Fprintf(file, "---\n\n")
	}

	return nil
}

// exportToCSV exports conversations to CSV format
func (em *ExportManager) exportToCSV(conversations []types.ConversationBlock, options ExportOptions) error {
	file, err := os.Create(options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	headers := []string{
		"ID", "Timestamp", "Model", "Status", "InputTokens", "OutputTokens", 
		"ExecutionTime", "UserInput", "AssistantResponse",
	}
	
	if options.IncludeTools {
		headers = append(headers, "ToolCalls", "ToolResults")
	}
	
	if options.IncludeFiles {
		headers = append(headers, "FileOperations")
	}

	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data
	for _, conv := range conversations {
		record := []string{
			conv.ID,
			conv.Timestamp.Format("2006-01-02 15:04:05"),
			conv.ModelName,
			conv.Status.String(),
			fmt.Sprintf("%d", conv.TokenUsage.InputTokens),
			fmt.Sprintf("%d", conv.TokenUsage.OutputTokens),
			conv.ExecutionTime.String(),
			strings.ReplaceAll(conv.UserInput, "\n", "\\n"),
			strings.ReplaceAll(conv.LLMResponse, "\n", "\\n"),
		}

		if options.IncludeTools {
			toolCalls := make([]string, len(conv.ToolCalls))
			for i, tool := range conv.ToolCalls {
				toolCalls[i] = tool.ToolName
			}
			record = append(record, strings.Join(toolCalls, "; "))

			toolResults := make([]string, len(conv.ToolResults))
			for i, result := range conv.ToolResults {
				status := "success"
				if !result.Success {
					status = "error"
				}
				toolResults[i] = status
			}
			record = append(record, strings.Join(toolResults, "; "))
		}

		if options.IncludeFiles {
			fileOps := make([]string, len(conv.FileOperations))
			for i, fileOp := range conv.FileOperations {
				fileOps[i] = fmt.Sprintf("%s:%s", fileOp.OperationType, fileOp.FilePath)
			}
			record = append(record, strings.Join(fileOps, "; "))
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// exportToHTML exports conversations to HTML format
func (em *ExportManager) exportToHTML(conversations []types.ConversationBlock, options ExportOptions) error {
	file, err := os.Create(options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	// Write HTML header
	fmt.Fprintf(file, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GoCodeNow Conversation Export</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 40px; line-height: 1.6; }
        .header { border-bottom: 2px solid #eee; padding-bottom: 20px; margin-bottom: 30px; }
        .conversation { border: 1px solid #ddd; border-radius: 8px; margin-bottom: 20px; padding: 20px; }
        .metadata { background: #f8f9fa; padding: 10px; border-radius: 4px; margin-bottom: 15px; font-size: 0.9em; }
        .user-input { background: #e3f2fd; padding: 15px; border-radius: 4px; margin: 10px 0; }
        .assistant-response { background: #f1f8e9; padding: 15px; border-radius: 4px; margin: 10px 0; }
        .tools, .files { background: #fff3e0; padding: 10px; border-radius: 4px; margin: 10px 0; }
        .timestamp { color: #666; font-size: 0.8em; }
        pre { background: #f5f5f5; padding: 10px; border-radius: 4px; overflow-x: auto; }
        code { background: #f5f5f5; padding: 2px 4px; border-radius: 2px; }
    </style>
</head>
<body>`)

	// Write header
	fmt.Fprintf(file, `<div class="header">
        <h1>GoCodeNow Conversation Export</h1>
        <p><strong>Exported:</strong> %s</p>
        <p><strong>Count:</strong> %d conversations</p>
    </div>`, time.Now().Format("2006-01-02 15:04:05"), len(conversations))

	// Write conversations
	for i, conv := range conversations {
		fmt.Fprintf(file, `<div class="conversation">
            <h2>Conversation %d</h2>
            <div class="metadata">
                <strong>ID:</strong> <code>%s</code><br>
                <strong>Timestamp:</strong> %s<br>
                <strong>Model:</strong> %s<br>
                <strong>Status:</strong> %s`, 
			i+1, conv.ID, conv.Timestamp.Format("2006-01-02 15:04:05"), 
			conv.ModelName, conv.Status.String())

		if conv.TokenUsage.InputTokens > 0 || conv.TokenUsage.OutputTokens > 0 {
			fmt.Fprintf(file, `<br><strong>Token Usage:</strong> %d input, %d output`, 
				conv.TokenUsage.InputTokens, conv.TokenUsage.OutputTokens)
		}

		if conv.ExecutionTime > 0 {
			fmt.Fprintf(file, `<br><strong>Execution Time:</strong> %v`, conv.ExecutionTime)
		}

		fmt.Fprintf(file, `</div>`)

		fmt.Fprintf(file, `<div class="user-input">
                <h3>User Input</h3>
                <pre>%s</pre>
            </div>`, conv.UserInput)

		fmt.Fprintf(file, `<div class="assistant-response">
                <h3>Assistant Response</h3>
                <div>%s</div>
            </div>`, strings.ReplaceAll(conv.LLMResponse, "\n", "<br>"))

		// Include tools if requested
		if options.IncludeTools && len(conv.ToolCalls) > 0 {
			fmt.Fprintf(file, `<div class="tools">
                <h3>Tool Calls</h3>
                <ul>`)
			for _, tool := range conv.ToolCalls {
				fmt.Fprintf(file, `<li><strong>%s</strong> (ID: <code>%s</code>) <span class="timestamp">%s</span></li>`, 
					tool.ToolName, tool.ID, tool.Timestamp.Format("15:04:05"))
			}
			fmt.Fprintf(file, `</ul></div>`)
		}

		// Include files if requested
		if options.IncludeFiles && len(conv.FileOperations) > 0 {
			fmt.Fprintf(file, `<div class="files">
                <h3>File Operations</h3>
                <ul>`)
			for _, fileOp := range conv.FileOperations {
				status := "✅"
				if !fileOp.Success {
					status = "❌"
				}
				fmt.Fprintf(file, `<li>%s <strong>%s</strong>: <code>%s</code> <span class="timestamp">%s</span></li>`, 
					status, fileOp.OperationType, fileOp.FilePath, fileOp.Timestamp.Format("15:04:05"))
			}
			fmt.Fprintf(file, `</ul></div>`)
		}

		fmt.Fprintf(file, `</div>`)
	}

	// Write HTML footer
	fmt.Fprintf(file, `</body>
</html>`)

	return nil
}

// GetDefaultExportPath generates a default export path
func (em *ExportManager) GetDefaultExportPath(format ExportFormat) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("gocodenow_export_%s%s", timestamp, format.FileExtension())
	return filepath.Join(em.defaultExportDir, filename)
}

// ValidateExportOptions validates export options
func (em *ExportManager) ValidateExportOptions(options ExportOptions) error {
	if options.FilePath == "" {
		return fmt.Errorf("file path is required")
	}

	// Check if directory exists
	dir := filepath.Dir(options.FilePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	// Validate date range
	if options.StartDate != nil && options.EndDate != nil {
		if options.StartDate.After(*options.EndDate) {
			return fmt.Errorf("start date must be before end date")
		}
	}

	return nil
}