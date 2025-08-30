package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	
	"gocodenow/internal/tools"
)

// Parser handles parsing and validation of LLM tool calls
type Parser struct {
	toolRouter tools.ToolRouter
	validator  *Validator
}

// NewParser creates a new tool call parser
func NewParser(toolRouter tools.ToolRouter) *Parser {
	return &Parser{
		toolRouter: toolRouter,
		validator:  NewValidator(toolRouter),
	}
}

// ParseToolCalls parses tool calls from an LLM response and converts them to internal format
func (p *Parser) ParseToolCalls(response *ChatResponse) ([]*tools.ToolCall, error) {
	if response == nil {
		return nil, NewValidationError("response cannot be nil")
	}
	
	var toolCalls []*tools.ToolCall
	
	// Iterate through all choices in the response
	for choiceIdx, choice := range response.Choices {
		if len(choice.Message.ToolCalls) == 0 {
			continue
		}
		
		// Parse each tool call in this choice
		for _, llmToolCall := range choice.Message.ToolCalls {
			// Convert LLM tool call to internal format
			internalTC, err := p.convertToolCall(&llmToolCall, choiceIdx)
			if err != nil {
				return nil, fmt.Errorf("converting tool call %s: %w", llmToolCall.ID, err)
			}
			
			// Validate the tool call
			if err := p.validator.ValidateToolCall(internalTC); err != nil {
				return nil, fmt.Errorf("validating tool call %s: %w", llmToolCall.ID, err)
			}
			
			toolCalls = append(toolCalls, internalTC)
		}
	}
	
	return toolCalls, nil
}

// convertToolCall converts an LLM ToolCall to internal tools.ToolCall format
func (p *Parser) convertToolCall(llmTC *ToolCall, choiceIdx int) (*tools.ToolCall, error) {
	// Parse arguments from JSON string
	params, err := p.parseArguments(llmTC.Function.Arguments)
	if err != nil {
		return nil, fmt.Errorf("parsing arguments: %w", err)
	}
	
	// Create internal tool call
	internalTC := &tools.ToolCall{
		ID:         llmTC.ID,
		ToolName:   llmTC.Function.Name,
		Parameters: params,
		Timestamp:  time.Now(),
	}
	
	return internalTC, nil
}

// parseArguments parses JSON arguments string into a map
func (p *Parser) parseArguments(argsJSON string) (map[string]interface{}, error) {
	if argsJSON == "" {
		return make(map[string]interface{}), nil
	}
	
	// Remove any surrounding whitespace
	argsJSON = strings.TrimSpace(argsJSON)
	
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, NewParsingError("invalid JSON arguments", err).
			WithContext("raw_json", argsJSON)
	}
	
	return args, nil
}

// ParseStreamingToolCall parses a tool call from streaming events
func (p *Parser) ParseStreamingToolCall(events []*StreamEvent) ([]*tools.ToolCall, error) {
	// Aggregate streaming events into complete tool calls
	toolCallMap := make(map[string]*StreamingToolCall)
	
	for _, event := range events {
		if event.Type != "tool_call" && event.Type != "tool_call_chunk" {
			continue
		}
		
		// Parse streaming tool call data
		var streamData struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
			Delta struct {
				Function struct {
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"delta,omitempty"`
		}
		
		eventBytes, err := json.Marshal(event.Data)
		if err != nil {
			continue
		}
		
		if err := json.Unmarshal(eventBytes, &streamData); err != nil {
			continue
		}
		
		// Initialize or update streaming tool call
		if streamTC, exists := toolCallMap[streamData.ID]; exists {
			// Append delta arguments
			streamTC.Arguments += streamData.Delta.Function.Arguments
		} else {
			// Create new streaming tool call
			toolCallMap[streamData.ID] = &StreamingToolCall{
				ID:        streamData.ID,
				Type:      streamData.Type,
				Name:      streamData.Function.Name,
				Arguments: streamData.Function.Arguments + streamData.Delta.Function.Arguments,
			}
		}
	}
	
	// Convert completed streaming tool calls to internal format
	var toolCalls []*tools.ToolCall
	for _, streamTC := range toolCallMap {
		// Create LLM tool call for conversion
		llmTC := &ToolCall{
			ID:   streamTC.ID,
			Type: streamTC.Type,
			Function: ToolCallFunction{
				Name:      streamTC.Name,
				Arguments: streamTC.Arguments,
			},
		}
		
		// Convert to internal format
		internalTC, err := p.convertToolCall(llmTC, 0)
		if err != nil {
			return nil, fmt.Errorf("converting streaming tool call %s: %w", streamTC.ID, err)
		}
		
		// Validate the tool call
		if err := p.validator.ValidateToolCall(internalTC); err != nil {
			return nil, fmt.Errorf("validating streaming tool call %s: %w", streamTC.ID, err)
		}
		
		toolCalls = append(toolCalls, internalTC)
	}
	
	return toolCalls, nil
}

// StreamingToolCall represents a tool call being built from streaming events
type StreamingToolCall struct {
	ID        string
	Type      string
	Name      string
	Arguments string
}

// Validator handles validation of tool calls against the available tools
type Validator struct {
	toolRouter tools.ToolRouter
}

// NewValidator creates a new tool call validator
func NewValidator(toolRouter tools.ToolRouter) *Validator {
	return &Validator{
		toolRouter: toolRouter,
	}
}

// ValidateToolCall validates a tool call against available tools and schemas
func (v *Validator) ValidateToolCall(toolCall *tools.ToolCall) error {
	if toolCall == nil {
		return NewValidationError("tool call cannot be nil")
	}
	
	// Check if tool name is provided
	if toolCall.ToolName == "" {
		return NewValidationError("tool name is required")
	}
	
	// Check if tool exists in router
	tools := v.toolRouter.ListTools()
	toolExists := false
	for _, t := range tools {
		if t == toolCall.ToolName {
			toolExists = true
			break
		}
	}
	if !toolExists {
		return NewValidationError(fmt.Sprintf("unknown tool: %s", toolCall.ToolName))
	}
	
	// Get tool executor for validation
	executor, exists := v.toolRouter.GetExecutor(toolCall.ToolName)
	if !exists {
		return NewValidationError(fmt.Sprintf("failed to get tool executor for %s", toolCall.ToolName))
	}
	
	// Validate required parameters
	if err := v.validateRequiredParams(toolCall, executor); err != nil {
		return err
	}
	
	// Validate parameter types
	if err := v.validateParameterTypes(toolCall, executor); err != nil {
		return err
	}
	
	// Security validation
	if err := v.validateSecurity(toolCall); err != nil {
		return err
	}
	
	return nil
}

// validateRequiredParams checks that all required parameters are present
func (v *Validator) validateRequiredParams(toolCall *tools.ToolCall, executor tools.ToolExecutor) error {
	// Use the tool executor's validation
	if err := executor.ValidateParameters(toolCall.Parameters); err != nil {
		return NewValidationError(fmt.Sprintf("parameter validation failed: %v", err))
	}
	
	if toolCall.Parameters == nil {
		toolCall.Parameters = make(map[string]interface{})
	}
	
	return nil
}

// validateParameterTypes validates parameter types against the tool schema
func (v *Validator) validateParameterTypes(toolCall *tools.ToolCall, executor tools.ToolExecutor) error {
	// This would validate types based on the tool's parameter schema
	// For now, we'll do basic type checking
	
	for key, value := range toolCall.Parameters {
		// Basic type validation
		if value == nil {
			continue
		}
		
		// Check for potentially dangerous values
		if str, ok := value.(string); ok {
			if v.containsSuspiciousContent(str) {
				return NewValidationError(fmt.Sprintf("suspicious content in parameter %s", key))
			}
		}
	}
	
	return nil
}

// validateSecurity performs security validation on tool calls
func (v *Validator) validateSecurity(toolCall *tools.ToolCall) error {
	// Check for path traversal attempts
	for key, value := range toolCall.Parameters {
		if str, ok := value.(string); ok {
			if v.hasPathTraversal(str) {
				return NewValidationError(fmt.Sprintf("path traversal detected in parameter %s", key))
			}
		}
	}
	
	return nil
}

// containsSuspiciousContent checks for potentially dangerous content
func (v *Validator) containsSuspiciousContent(content string) bool {
	suspicious := []string{
		"../",
		"..\\",
		"/etc/passwd",
		"/bin/sh",
		"cmd.exe",
		"powershell",
		"<script>",
		"javascript:",
		"data:",
	}
	
	lower := strings.ToLower(content)
	for _, pattern := range suspicious {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	
	return false
}

// hasPathTraversal checks for path traversal patterns
func (v *Validator) hasPathTraversal(path string) bool {
	return strings.Contains(path, "../") || strings.Contains(path, "..\\")
}

// FormatToolResults formats tool execution results for LLM consumption
func (p *Parser) FormatToolResults(results []*tools.ToolResult) []ChatMessage {
	var messages []ChatMessage
	
	for _, result := range results {
		var content string
		
		if !result.Success {
			// Format error result
			content = fmt.Sprintf("Tool execution failed: %s", result.ErrorMessage)
		} else {
			// Format success result
			if result.Result != nil {
				if str, ok := result.Result.(string); ok {
					content = str
				} else {
					// Convert to JSON for complex outputs
					if jsonBytes, err := json.MarshalIndent(result.Result, "", "  "); err == nil {
						content = string(jsonBytes)
					} else {
						content = fmt.Sprintf("%v", result.Result)
					}
				}
			} else {
				content = "Tool executed successfully"
			}
		}
		
		message := ChatMessage{
			Role:       "tool",
			Content:    content,
			ToolCallID: &result.ToolCallID,
		}
		
		messages = append(messages, message)
	}
	
	return messages
}

// GenerateToolDefinitions converts internal tools to LLM tool definitions
func (p *Parser) GenerateToolDefinitions(toolNames []string) ([]ToolDefinition, error) {
	var definitions []ToolDefinition
	
	availableTools := p.toolRouter.ListTools()
	
	for _, name := range toolNames {
		// Check if tool exists
		toolExists := false
		for _, t := range availableTools {
			if t == name {
				toolExists = true
				break
			}
		}
		if !toolExists {
			return nil, NewValidationError(fmt.Sprintf("unknown tool: %s", name))
		}
		
		executor, exists := p.toolRouter.GetExecutor(name)
		if !exists {
			return nil, fmt.Errorf("failed to get tool executor %s", name)
		}
		
		// Convert internal tool to LLM format
		llmTool := ToolDefinition{
			Type: "function",
			Function: FunctionSchema{
				Name:        name,
				Description: executor.Description(),
				Parameters:  p.generateParametersSchema(executor),
			},
		}
		
		definitions = append(definitions, llmTool)
	}
	
	return definitions, nil
}

// generateParametersSchema generates parameter schema from tool executor
func (p *Parser) generateParametersSchema(executor tools.ToolExecutor) FunctionParameters {
	// Get the tool's schema
	schema := executor.Schema()
	
	// Convert from internal schema to LLM parameter schema
	properties := make(map[string]PropertySchema)
	
	// Since schema.Parameters is map[string]interface{}, we need to convert it
	// This is a simplified implementation - in practice, you'd parse the JSON schema
	for name, paramDef := range schema.Parameters {
		propSchema := PropertySchema{
			Type:        "string", // Default type
			Description: fmt.Sprintf("Parameter %s", name),
		}
		
		// Try to extract type and description if it's a map
		if paramMap, ok := paramDef.(map[string]interface{}); ok {
			if typeVal, exists := paramMap["type"]; exists {
				if typeStr, ok := typeVal.(string); ok {
					propSchema.Type = typeStr
				}
			}
			if descVal, exists := paramMap["description"]; exists {
				if descStr, ok := descVal.(string); ok {
					propSchema.Description = descStr
				}
			}
		}
		
		properties[name] = propSchema
	}
	
	return FunctionParameters{
		Type:                 "object",
		Properties:           properties,
		Required:             schema.Required,
		AdditionalProperties: false,
	}
}