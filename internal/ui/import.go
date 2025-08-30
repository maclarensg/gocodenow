package ui

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"gocodenow/internal/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ImportResult represents the result of an import operation
type ImportResult struct {
	TotalRecords    int                      `json:"total_records"`
	ImportedRecords int                      `json:"imported_records"`
	SkippedRecords  int                      `json:"skipped_records"`
	ErrorRecords    int                      `json:"error_records"`
	Errors          []ImportError            `json:"errors"`
	ImportedConvs   []types.ConversationBlock `json:"imported_conversations"`
	Duration        time.Duration            `json:"duration"`
}

// ImportError represents an error during import
type ImportError struct {
	RecordIndex int    `json:"record_index"`
	FieldName   string `json:"field_name,omitempty"`
	Error       string `json:"error"`
	Data        string `json:"data,omitempty"`
}

// ImportOptions defines options for importing conversations
type ImportOptions struct {
	FilePath           string
	Format             ExportFormat
	OverwriteExisting  bool
	ValidateData       bool
	SkipDuplicates     bool
	UpdateExistingIDs  bool
	PreserveTimestamps bool
	BatchSize          int
}

// ImportManager handles conversation import operations
type ImportManager struct {
	validator *DataValidator
}

// NewImportManager creates a new import manager
func NewImportManager() *ImportManager {
	return &ImportManager{
		validator: NewDataValidator(),
	}
}

// ImportConversations imports conversations from a file
func (im *ImportManager) ImportConversations(options ImportOptions) (*ImportResult, error) {
	startTime := time.Now()
	
	result := &ImportResult{
		Errors:        []ImportError{},
		ImportedConvs: []types.ConversationBlock{},
	}

	// Validate file exists
	if _, err := os.Stat(options.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("import file does not exist: %s", options.FilePath)
	}

	// Detect format if not specified
	if options.Format == 0 {
		detectedFormat, err := im.detectFormat(options.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to detect file format: %w", err)
		}
		options.Format = detectedFormat
	}

	// Set default batch size
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}

	// Import based on format
	var conversations []types.ConversationBlock
	var err error

	switch options.Format {
	case FormatJSON:
		conversations, err = im.importFromJSON(options.FilePath)
	case FormatMarkdown:
		conversations, err = im.importFromMarkdown(options.FilePath)
	case FormatCSV:
		conversations, err = im.importFromCSV(options.FilePath)
	default:
		return nil, fmt.Errorf("unsupported import format: %s", options.Format.String())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read import file: %w", err)
	}

	result.TotalRecords = len(conversations)

	// Process conversations
	for i, conv := range conversations {
		// Validate data if requested
		if options.ValidateData {
			if validationErr := im.validator.ValidateConversation(conv); validationErr != nil {
				result.ErrorRecords++
				result.Errors = append(result.Errors, ImportError{
					RecordIndex: i,
					Error:       validationErr.Error(),
					Data:        conv.ID,
				})
				continue
			}
		}

		// Process the conversation
		processedConv, err := im.processConversation(conv, options)
		if err != nil {
			result.ErrorRecords++
			result.Errors = append(result.Errors, ImportError{
				RecordIndex: i,
				Error:       err.Error(),
				Data:        conv.ID,
			})
			continue
		}

		result.ImportedConvs = append(result.ImportedConvs, processedConv)
		result.ImportedRecords++
	}

	result.SkippedRecords = result.TotalRecords - result.ImportedRecords - result.ErrorRecords
	result.Duration = time.Since(startTime)

	return result, nil
}

// detectFormat attempts to detect the file format from the file extension and content
func (im *ImportManager) detectFormat(filePath string) (ExportFormat, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".json":
		return FormatJSON, nil
	case ".md", ".markdown":
		return FormatMarkdown, nil
	case ".csv":
		return FormatCSV, nil
	case ".html", ".htm":
		return FormatHTML, nil
	default:
		// Try to detect from content
		return im.detectFormatFromContent(filePath)
	}
}

// detectFormatFromContent detects format by examining file content
func (im *ImportManager) detectFormatFromContent(filePath string) (ExportFormat, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	firstLines := make([]string, 0, 5)
	
	for i := 0; i < 5 && scanner.Scan(); i++ {
		firstLines = append(firstLines, strings.TrimSpace(scanner.Text()))
	}

	content := strings.Join(firstLines, " ")

	// Check for JSON
	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		return FormatJSON, nil
	}

	// Check for HTML
	if strings.Contains(strings.ToLower(content), "<html") || strings.Contains(strings.ToLower(content), "<!doctype") {
		return FormatHTML, nil
	}

	// Check for Markdown headers
	if strings.Contains(content, "# ") || strings.Contains(content, "## ") {
		return FormatMarkdown, nil
	}

	// Check for CSV (commas and quoted fields)
	if strings.Contains(content, ",") && (strings.Contains(content, "\"") || len(strings.Split(content, ",")) > 3) {
		return FormatCSV, nil
	}

	return 0, fmt.Errorf("unable to detect file format")
}

// importFromJSON imports conversations from JSON format
func (im *ImportManager) importFromJSON(filePath string) ([]types.ConversationBlock, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var conversations []types.ConversationBlock

	// Try to decode as lmcodenow export format first
	var exportData struct {
		Conversations []types.ConversationBlock `json:"conversations"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&exportData); err == nil && len(exportData.Conversations) > 0 {
		return exportData.Conversations, nil
	}

	// Reset file position
	file.Seek(0, 0)

	// Try to decode as direct array of conversations
	decoder = json.NewDecoder(file)
	if err := decoder.Decode(&conversations); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return conversations, nil
}

// importFromMarkdown imports conversations from Markdown format
func (im *ImportManager) importFromMarkdown(filePath string) ([]types.ConversationBlock, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var conversations []types.ConversationBlock
	scanner := bufio.NewScanner(file)
	
	var currentConv types.ConversationBlock
	var currentSection string
	var contentBuilder strings.Builder
	conversationStarted := false

	// Regex patterns
	convHeaderRegex := regexp.MustCompile(`^## Conversation \d+`)
	idRegex := regexp.MustCompile(`\*\*ID:\*\* ` + "`" + `([^` + "`" + `]+)` + "`")
	timestampRegex := regexp.MustCompile(`\*\*Timestamp:\*\* (.+)`)
	modelRegex := regexp.MustCompile(`\*\*Model:\*\* (.+)`)
	statusRegex := regexp.MustCompile(`\*\*Status:\*\* (.+)`)

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		// Check for new conversation
		if convHeaderRegex.MatchString(trimmedLine) {
			// Save previous conversation if exists
			if conversationStarted {
				im.finalizeMarkdownConversation(&currentConv, currentSection, contentBuilder.String())
				conversations = append(conversations, currentConv)
			}

			// Start new conversation
			currentConv = types.ConversationBlock{
				ID:        uuid.New().String(),
				Timestamp: time.Now(),
				Status:    types.StatusCompleted,
			}
			conversationStarted = true
			currentSection = ""
			contentBuilder.Reset()
			continue
		}

		if !conversationStarted {
			continue
		}

		// Parse metadata
		if matches := idRegex.FindStringSubmatch(line); len(matches) > 1 {
			currentConv.ID = matches[1]
			continue
		}
		if matches := timestampRegex.FindStringSubmatch(line); len(matches) > 1 {
			if timestamp, err := time.Parse("2006-01-02 15:04:05", matches[1]); err == nil {
				currentConv.Timestamp = timestamp
			}
			continue
		}
		if matches := modelRegex.FindStringSubmatch(line); len(matches) > 1 {
			currentConv.ModelName = strings.TrimSpace(matches[1])
			continue
		}
		if matches := statusRegex.FindStringSubmatch(line); len(matches) > 1 {
			currentConv.Status = types.ParseConversationStatus(strings.ToLower(strings.TrimSpace(matches[1])))
			continue
		}

		// Check for section headers
		if trimmedLine == "### User Input" {
			im.finalizeMarkdownConversation(&currentConv, currentSection, contentBuilder.String())
			currentSection = "user"
			contentBuilder.Reset()
			continue
		}
		if trimmedLine == "### Assistant Response" {
			im.finalizeMarkdownConversation(&currentConv, currentSection, contentBuilder.String())
			currentSection = "assistant"
			contentBuilder.Reset()
			continue
		}

		// Skip horizontal rules and empty lines in content
		if trimmedLine == "---" || (trimmedLine == "" && contentBuilder.Len() == 0) {
			continue
		}

		// Skip code fences for user input
		if currentSection == "user" && (trimmedLine == "```" || strings.HasPrefix(trimmedLine, "```")) {
			continue
		}

		// Accumulate content
		if currentSection != "" {
			if contentBuilder.Len() > 0 {
				contentBuilder.WriteString("\n")
			}
			contentBuilder.WriteString(line)
		}
	}

	// Save last conversation
	if conversationStarted {
		im.finalizeMarkdownConversation(&currentConv, currentSection, contentBuilder.String())
		conversations = append(conversations, currentConv)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading markdown file: %w", err)
	}

	return conversations, nil
}

// finalizeMarkdownConversation sets the content for the current section
func (im *ImportManager) finalizeMarkdownConversation(conv *types.ConversationBlock, section, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	switch section {
	case "user":
		conv.UserInput = content
		conv.User = content // Backward compatibility
	case "assistant":
		conv.LLMResponse = content
		conv.Assistant = content // Backward compatibility
	}
}

// importFromCSV imports conversations from CSV format
func (im *ImportManager) importFromCSV(filePath string) ([]types.ConversationBlock, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have at least a header and one data row")
	}

	// Parse header to determine column positions
	headers := records[0]
	columnMap := make(map[string]int)
	for i, header := range headers {
		columnMap[strings.ToLower(strings.TrimSpace(header))] = i
	}

	var conversations []types.ConversationBlock

	// Process data rows
	for i, record := range records[1:] {
		conv := types.ConversationBlock{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			Status:    types.StatusCompleted,
		}

		// Map CSV fields to conversation struct
		if col, exists := columnMap["id"]; exists && col < len(record) {
			if record[col] != "" {
				conv.ID = record[col]
			}
		}

		if col, exists := columnMap["timestamp"]; exists && col < len(record) {
			if timestamp, err := time.Parse("2006-01-02 15:04:05", record[col]); err == nil {
				conv.Timestamp = timestamp
			}
		}

		if col, exists := columnMap["model"]; exists && col < len(record) {
			conv.ModelName = record[col]
		}

		if col, exists := columnMap["status"]; exists && col < len(record) {
			conv.Status = types.ParseConversationStatus(strings.ToLower(record[col]))
		}

		if col, exists := columnMap["inputtokens"]; exists && col < len(record) {
			if tokens, err := strconv.Atoi(record[col]); err == nil {
				conv.TokenUsage.InputTokens = tokens
			}
		}

		if col, exists := columnMap["outputtokens"]; exists && col < len(record) {
			if tokens, err := strconv.Atoi(record[col]); err == nil {
				conv.TokenUsage.OutputTokens = tokens
			}
		}

		if col, exists := columnMap["executiontime"]; exists && col < len(record) {
			if duration, err := time.ParseDuration(record[col]); err == nil {
				conv.ExecutionTime = duration
			}
		}

		if col, exists := columnMap["userinput"]; exists && col < len(record) {
			conv.UserInput = strings.ReplaceAll(record[col], "\\n", "\n")
			conv.User = conv.UserInput // Backward compatibility
		}

		if col, exists := columnMap["assistantresponse"]; exists && col < len(record) {
			conv.LLMResponse = strings.ReplaceAll(record[col], "\\n", "\n")
			conv.Assistant = conv.LLMResponse // Backward compatibility
		}

		// Validate required fields
		if conv.UserInput == "" && conv.LLMResponse == "" {
			return nil, fmt.Errorf("row %d: missing both user input and assistant response", i+2)
		}

		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// processConversation processes a conversation according to import options
func (im *ImportManager) processConversation(conv types.ConversationBlock, options ImportOptions) (types.ConversationBlock, error) {
	// Update ID if requested
	if options.UpdateExistingIDs {
		conv.ID = uuid.New().String()
	}

	// Update timestamps if not preserving them
	if !options.PreserveTimestamps {
		conv.Timestamp = time.Now()
	}

	// Ensure backward compatibility fields are set
	if conv.User == "" && conv.UserInput != "" {
		conv.User = conv.UserInput
	}
	if conv.Assistant == "" && conv.LLMResponse != "" {
		conv.Assistant = conv.LLMResponse
	}
	if conv.UserInput == "" && conv.User != "" {
		conv.UserInput = conv.User
	}
	if conv.LLMResponse == "" && conv.Assistant != "" {
		conv.LLMResponse = conv.Assistant
	}

	return conv, nil
}

// DataValidator validates conversation data
type DataValidator struct{}

// NewDataValidator creates a new data validator
func NewDataValidator() *DataValidator {
	return &DataValidator{}
}

// ValidateConversation validates a conversation record
func (v *DataValidator) ValidateConversation(conv types.ConversationBlock) error {
	// ID validation
	if conv.ID == "" {
		return fmt.Errorf("conversation ID is required")
	}

	// Content validation
	if conv.UserInput == "" && conv.User == "" {
		return fmt.Errorf("user input is required")
	}

	if conv.LLMResponse == "" && conv.Assistant == "" {
		return fmt.Errorf("assistant response is required")
	}

	// Timestamp validation
	if conv.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}

	// Token usage validation
	if conv.TokenUsage.InputTokens < 0 || conv.TokenUsage.OutputTokens < 0 {
		return fmt.Errorf("token usage cannot be negative")
	}

	// Tool calls validation
	for i, toolCall := range conv.ToolCalls {
		if toolCall.ID == "" {
			return fmt.Errorf("tool call %d: ID is required", i)
		}
		if toolCall.ToolName == "" {
			return fmt.Errorf("tool call %d: tool name is required", i)
		}
	}

	// Tool results validation
	for i, toolResult := range conv.ToolResults {
		if toolResult.ID == "" {
			return fmt.Errorf("tool result %d: ID is required", i)
		}
		if toolResult.ToolCallID == "" {
			return fmt.Errorf("tool result %d: tool call ID is required", i)
		}
	}

	// File operations validation
	for i, fileOp := range conv.FileOperations {
		if fileOp.ID == "" {
			return fmt.Errorf("file operation %d: ID is required", i)
		}
		if fileOp.FilePath == "" {
			return fmt.Errorf("file operation %d: file path is required", i)
		}
		if fileOp.OperationType == "" {
			return fmt.Errorf("file operation %d: operation type is required", i)
		}
	}

	return nil
}

// ValidateImportFile validates an import file before processing
func (v *DataValidator) ValidateImportFile(filePath string) error {
	// Check file exists
	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}
	if err != nil {
		return fmt.Errorf("error accessing file: %w", err)
	}

	// Check file size (warn if > 50MB)
	if fileInfo.Size() > 50*1024*1024 {
		return fmt.Errorf("file is very large (%d MB), import may be slow", fileInfo.Size()/(1024*1024))
	}

	// Check file is readable
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("file is not readable: %w", err)
	}
	file.Close()

	return nil
}