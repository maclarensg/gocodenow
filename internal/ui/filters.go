package ui

import (
	"gocodenow/internal/types"
	"regexp"
	"strings"
	"time"
)

// ConversationFilter provides advanced filtering capabilities for conversations
type ConversationFilter struct {
	// Date filtering
	StartDate *time.Time
	EndDate   *time.Time

	// Status filtering
	Statuses []types.ConversationStatus

	// Model filtering
	Models    []string
	ModelRegex *regexp.Regexp

	// Content filtering
	UserInputContains      []string
	UserInputRegex         *regexp.Regexp
	AssistantResponseContains []string
	AssistantResponseRegex    *regexp.Regexp

	// Token usage filtering
	MinInputTokens  *int
	MaxInputTokens  *int
	MinOutputTokens *int
	MaxOutputTokens *int
	MinTotalTokens  *int
	MaxTotalTokens  *int

	// Execution time filtering
	MinExecutionTime *time.Duration
	MaxExecutionTime *time.Duration

	// Tool and file filtering
	HasToolCalls      *bool
	HasFileOperations *bool
	ToolNames         []string
	FileExtensions    []string
	FilePaths         []string

	// Advanced filtering
	ExpandedOnly   *bool
	ErrorsOnly     *bool
	SuccessOnly    *bool
	RecentOnly     *time.Duration // Only conversations within this duration from now
	
	// Text search
	FullTextSearch string
	CaseSensitive  bool
}

// NewConversationFilter creates a new conversation filter
func NewConversationFilter() *ConversationFilter {
	return &ConversationFilter{}
}

// WithDateRange sets the date range filter
func (f *ConversationFilter) WithDateRange(start, end *time.Time) *ConversationFilter {
	f.StartDate = start
	f.EndDate = end
	return f
}

// WithStatuses sets the status filter
func (f *ConversationFilter) WithStatuses(statuses ...types.ConversationStatus) *ConversationFilter {
	f.Statuses = statuses
	return f
}

// WithModels sets the model filter
func (f *ConversationFilter) WithModels(models ...string) *ConversationFilter {
	f.Models = models
	return f
}

// WithModelRegex sets a regex pattern for model filtering
func (f *ConversationFilter) WithModelRegex(pattern string) (*ConversationFilter, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return f, err
	}
	f.ModelRegex = regex
	return f, nil
}

// WithUserInputContaining sets user input content filter
func (f *ConversationFilter) WithUserInputContaining(terms ...string) *ConversationFilter {
	f.UserInputContains = terms
	return f
}

// WithAssistantResponseContaining sets assistant response content filter
func (f *ConversationFilter) WithAssistantResponseContaining(terms ...string) *ConversationFilter {
	f.AssistantResponseContains = terms
	return f
}

// WithTokenRange sets token usage filters
func (f *ConversationFilter) WithTokenRange(minInput, maxInput, minOutput, maxOutput *int) *ConversationFilter {
	f.MinInputTokens = minInput
	f.MaxInputTokens = maxInput
	f.MinOutputTokens = minOutput
	f.MaxOutputTokens = maxOutput
	return f
}

// WithExecutionTimeRange sets execution time filters
func (f *ConversationFilter) WithExecutionTimeRange(min, max *time.Duration) *ConversationFilter {
	f.MinExecutionTime = min
	f.MaxExecutionTime = max
	return f
}

// WithToolsRequired sets whether conversations must have tool calls
func (f *ConversationFilter) WithToolsRequired(required bool) *ConversationFilter {
	f.HasToolCalls = &required
	return f
}

// WithFilesRequired sets whether conversations must have file operations
func (f *ConversationFilter) WithFilesRequired(required bool) *ConversationFilter {
	f.HasFileOperations = &required
	return f
}

// WithToolNames sets specific tool name filters
func (f *ConversationFilter) WithToolNames(toolNames ...string) *ConversationFilter {
	f.ToolNames = toolNames
	return f
}

// WithFileExtensions sets file extension filters
func (f *ConversationFilter) WithFileExtensions(extensions ...string) *ConversationFilter {
	f.FileExtensions = extensions
	return f
}

// WithErrorsOnly sets filter to only show conversations with errors
func (f *ConversationFilter) WithErrorsOnly(errorsOnly bool) *ConversationFilter {
	f.ErrorsOnly = &errorsOnly
	return f
}

// WithRecentOnly sets filter to only show recent conversations
func (f *ConversationFilter) WithRecentOnly(duration time.Duration) *ConversationFilter {
	f.RecentOnly = &duration
	return f
}

// WithFullTextSearch sets full text search across all conversation content
func (f *ConversationFilter) WithFullTextSearch(query string, caseSensitive bool) *ConversationFilter {
	f.FullTextSearch = query
	f.CaseSensitive = caseSensitive
	return f
}

// Apply applies the filter to a list of conversations
func (f *ConversationFilter) Apply(conversations []types.ConversationBlock) []types.ConversationBlock {
	var filtered []types.ConversationBlock

	for _, conv := range conversations {
		if f.matches(conv) {
			filtered = append(filtered, conv)
		}
	}

	return filtered
}

// matches checks if a conversation matches the filter criteria
func (f *ConversationFilter) matches(conv types.ConversationBlock) bool {
	// Date range filter
	if f.StartDate != nil && conv.Timestamp.Before(*f.StartDate) {
		return false
	}
	if f.EndDate != nil && conv.Timestamp.After(*f.EndDate) {
		return false
	}

	// Recent only filter
	if f.RecentOnly != nil {
		cutoff := time.Now().Add(-*f.RecentOnly)
		if conv.Timestamp.Before(cutoff) {
			return false
		}
	}

	// Status filter
	if len(f.Statuses) > 0 {
		statusMatch := false
		for _, status := range f.Statuses {
			if conv.Status == status {
				statusMatch = true
				break
			}
		}
		if !statusMatch {
			return false
		}
	}

	// Model filter
	if len(f.Models) > 0 {
		modelMatch := false
		for _, model := range f.Models {
			if strings.Contains(strings.ToLower(conv.ModelName), strings.ToLower(model)) {
				modelMatch = true
				break
			}
		}
		if !modelMatch {
			return false
		}
	}

	// Model regex filter
	if f.ModelRegex != nil && !f.ModelRegex.MatchString(conv.ModelName) {
		return false
	}

	// User input content filter
	if len(f.UserInputContains) > 0 {
		contentMatch := false
		userInput := conv.UserInput
		if !f.CaseSensitive {
			userInput = strings.ToLower(userInput)
		}
		
		for _, term := range f.UserInputContains {
			searchTerm := term
			if !f.CaseSensitive {
				searchTerm = strings.ToLower(searchTerm)
			}
			if strings.Contains(userInput, searchTerm) {
				contentMatch = true
				break
			}
		}
		if !contentMatch {
			return false
		}
	}

	// User input regex filter
	if f.UserInputRegex != nil && !f.UserInputRegex.MatchString(conv.UserInput) {
		return false
	}

	// Assistant response content filter
	if len(f.AssistantResponseContains) > 0 {
		contentMatch := false
		response := conv.LLMResponse
		if !f.CaseSensitive {
			response = strings.ToLower(response)
		}
		
		for _, term := range f.AssistantResponseContains {
			searchTerm := term
			if !f.CaseSensitive {
				searchTerm = strings.ToLower(searchTerm)
			}
			if strings.Contains(response, searchTerm) {
				contentMatch = true
				break
			}
		}
		if !contentMatch {
			return false
		}
	}

	// Assistant response regex filter
	if f.AssistantResponseRegex != nil && !f.AssistantResponseRegex.MatchString(conv.LLMResponse) {
		return false
	}

	// Token usage filters
	if f.MinInputTokens != nil && conv.TokenUsage.InputTokens < *f.MinInputTokens {
		return false
	}
	if f.MaxInputTokens != nil && conv.TokenUsage.InputTokens > *f.MaxInputTokens {
		return false
	}
	if f.MinOutputTokens != nil && conv.TokenUsage.OutputTokens < *f.MinOutputTokens {
		return false
	}
	if f.MaxOutputTokens != nil && conv.TokenUsage.OutputTokens > *f.MaxOutputTokens {
		return false
	}

	totalTokens := conv.TokenUsage.InputTokens + conv.TokenUsage.OutputTokens
	if f.MinTotalTokens != nil && totalTokens < *f.MinTotalTokens {
		return false
	}
	if f.MaxTotalTokens != nil && totalTokens > *f.MaxTotalTokens {
		return false
	}

	// Execution time filters
	if f.MinExecutionTime != nil && conv.ExecutionTime < *f.MinExecutionTime {
		return false
	}
	if f.MaxExecutionTime != nil && conv.ExecutionTime > *f.MaxExecutionTime {
		return false
	}

	// Tool calls filter
	if f.HasToolCalls != nil {
		hasTools := len(conv.ToolCalls) > 0
		if hasTools != *f.HasToolCalls {
			return false
		}
	}

	// File operations filter
	if f.HasFileOperations != nil {
		hasFiles := len(conv.FileOperations) > 0
		if hasFiles != *f.HasFileOperations {
			return false
		}
	}

	// Specific tool names filter
	if len(f.ToolNames) > 0 {
		toolMatch := false
		for _, toolCall := range conv.ToolCalls {
			for _, toolName := range f.ToolNames {
				if strings.Contains(strings.ToLower(toolCall.ToolName), strings.ToLower(toolName)) {
					toolMatch = true
					break
				}
			}
			if toolMatch {
				break
			}
		}
		if !toolMatch {
			return false
		}
	}

	// File extensions filter
	if len(f.FileExtensions) > 0 {
		extMatch := false
		for _, fileOp := range conv.FileOperations {
			for _, ext := range f.FileExtensions {
				if strings.HasSuffix(strings.ToLower(fileOp.FilePath), strings.ToLower(ext)) {
					extMatch = true
					break
				}
			}
			if extMatch {
				break
			}
		}
		if !extMatch {
			return false
		}
	}

	// File paths filter
	if len(f.FilePaths) > 0 {
		pathMatch := false
		for _, fileOp := range conv.FileOperations {
			for _, path := range f.FilePaths {
				if strings.Contains(strings.ToLower(fileOp.FilePath), strings.ToLower(path)) {
					pathMatch = true
					break
				}
			}
			if pathMatch {
				break
			}
		}
		if !pathMatch {
			return false
		}
	}

	// Expanded only filter
	if f.ExpandedOnly != nil && conv.Expanded != *f.ExpandedOnly {
		return false
	}

	// Errors only filter
	if f.ErrorsOnly != nil && *f.ErrorsOnly {
		hasErrors := conv.Status == types.StatusError
		if !hasErrors {
			// Check for tool execution errors
			for _, result := range conv.ToolResults {
				if !result.Success {
					hasErrors = true
					break
				}
			}
		}
		if !hasErrors {
			// Check for file operation errors
			for _, fileOp := range conv.FileOperations {
				if !fileOp.Success {
					hasErrors = true
					break
				}
			}
		}
		if !hasErrors {
			return false
		}
	}

	// Success only filter
	if f.SuccessOnly != nil && *f.SuccessOnly {
		if conv.Status == types.StatusError {
			return false
		}
		// Check for tool execution errors
		for _, result := range conv.ToolResults {
			if !result.Success {
				return false
			}
		}
		// Check for file operation errors
		for _, fileOp := range conv.FileOperations {
			if !fileOp.Success {
				return false
			}
		}
	}

	// Full text search
	if f.FullTextSearch != "" {
		searchText := f.FullTextSearch
		if !f.CaseSensitive {
			searchText = strings.ToLower(searchText)
		}

		// Search in user input
		userInput := conv.UserInput
		if !f.CaseSensitive {
			userInput = strings.ToLower(userInput)
		}
		if strings.Contains(userInput, searchText) {
			return true
		}

		// Search in assistant response
		response := conv.LLMResponse
		if !f.CaseSensitive {
			response = strings.ToLower(response)
		}
		if strings.Contains(response, searchText) {
			return true
		}

		// Search in model name
		modelName := conv.ModelName
		if !f.CaseSensitive {
			modelName = strings.ToLower(modelName)
		}
		if strings.Contains(modelName, searchText) {
			return true
		}

		// Search in tool names
		for _, toolCall := range conv.ToolCalls {
			toolName := toolCall.ToolName
			if !f.CaseSensitive {
				toolName = strings.ToLower(toolName)
			}
			if strings.Contains(toolName, searchText) {
				return true
			}
		}

		// Search in file paths
		for _, fileOp := range conv.FileOperations {
			filePath := fileOp.FilePath
			if !f.CaseSensitive {
				filePath = strings.ToLower(filePath)
			}
			if strings.Contains(filePath, searchText) {
				return true
			}
		}

		// If we reach here and we're doing full text search, no match was found
		return false
	}

	return true
}

// Count returns the number of conversations that match the filter
func (f *ConversationFilter) Count(conversations []types.ConversationBlock) int {
	count := 0
	for _, conv := range conversations {
		if f.matches(conv) {
			count++
		}
	}
	return count
}

// GetStats returns statistics about the filtered conversations
func (f *ConversationFilter) GetStats(conversations []types.ConversationBlock) FilterStats {
	filtered := f.Apply(conversations)
	
	stats := FilterStats{
		TotalFiltered: len(filtered),
		TotalOriginal: len(conversations),
	}

	if len(filtered) == 0 {
		return stats
	}

	// Calculate statistics
	var totalInputTokens, totalOutputTokens int
	var totalExecutionTime time.Duration
	statusCounts := make(map[types.ConversationStatus]int)
	modelCounts := make(map[string]int)
	toolCounts := make(map[string]int)

	for _, conv := range filtered {
		totalInputTokens += conv.TokenUsage.InputTokens
		totalOutputTokens += conv.TokenUsage.OutputTokens
		totalExecutionTime += conv.ExecutionTime
		
		statusCounts[conv.Status]++
		modelCounts[conv.ModelName]++

		for _, toolCall := range conv.ToolCalls {
			toolCounts[toolCall.ToolName]++
		}
	}

	stats.TotalInputTokens = totalInputTokens
	stats.TotalOutputTokens = totalOutputTokens
	stats.TotalExecutionTime = totalExecutionTime
	stats.AverageInputTokens = float64(totalInputTokens) / float64(len(filtered))
	stats.AverageOutputTokens = float64(totalOutputTokens) / float64(len(filtered))
	stats.AverageExecutionTime = totalExecutionTime / time.Duration(len(filtered))
	stats.StatusCounts = statusCounts
	stats.ModelCounts = modelCounts
	stats.ToolCounts = toolCounts

	return stats
}

// FilterStats contains statistics about filtered conversations
type FilterStats struct {
	TotalFiltered        int                              `json:"total_filtered"`
	TotalOriginal        int                              `json:"total_original"`
	TotalInputTokens     int                              `json:"total_input_tokens"`
	TotalOutputTokens    int                              `json:"total_output_tokens"`
	TotalExecutionTime   time.Duration                    `json:"total_execution_time"`
	AverageInputTokens   float64                          `json:"average_input_tokens"`
	AverageOutputTokens  float64                          `json:"average_output_tokens"`
	AverageExecutionTime time.Duration                    `json:"average_execution_time"`
	StatusCounts         map[types.ConversationStatus]int `json:"status_counts"`
	ModelCounts          map[string]int                   `json:"model_counts"`
	ToolCounts           map[string]int                   `json:"tool_counts"`
}

// PresetFilters provides common filter presets
type PresetFilters struct{}

// NewPresetFilters creates preset filters
func NewPresetFilters() *PresetFilters {
	return &PresetFilters{}
}

// ErrorConversations returns a filter for conversations with errors
func (p *PresetFilters) ErrorConversations() *ConversationFilter {
	return NewConversationFilter().WithErrorsOnly(true)
}

// RecentConversations returns a filter for recent conversations
func (p *PresetFilters) RecentConversations(duration time.Duration) *ConversationFilter {
	return NewConversationFilter().WithRecentOnly(duration)
}

// HighTokenUsage returns a filter for conversations with high token usage
func (p *PresetFilters) HighTokenUsage(minTokens int) *ConversationFilter {
	return NewConversationFilter().WithTokenRange(nil, nil, nil, nil).
		WithTokenRange(nil, nil, nil, nil) // Will be set properly
}

// WithToolsOnly returns a filter for conversations that used tools
func (p *PresetFilters) WithToolsOnly() *ConversationFilter {
	return NewConversationFilter().WithToolsRequired(true)
}

// WithFilesOnly returns a filter for conversations that modified files
func (p *PresetFilters) WithFilesOnly() *ConversationFilter {
	return NewConversationFilter().WithFilesRequired(true)
}

// CodeRelated returns a filter for code-related conversations
func (p *PresetFilters) CodeRelated() *ConversationFilter {
	return NewConversationFilter().
		WithUserInputContaining("code", "function", "class", "bug", "error", "implement", "fix").
		WithFileExtensions(".go", ".js", ".py", ".java", ".cpp", ".c", ".rs", ".ts", ".jsx", ".tsx")
}