package ui

import (
	"context"
	"fmt"
	"time"

	"gocodenow/internal/llm"
	"gocodenow/internal/models"
	"gocodenow/internal/security"
	"gocodenow/internal/tools"
	"gocodenow/internal/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// MessageProcessor handles the complete workflow of processing user messages
// including LLM interaction and tool execution
type MessageProcessor struct {
	llmClient      llm.Client
	toolRouter     tools.ToolRouter
	conversations  *models.ConversationHistory
	toolWorkflow   *ToolExecutionWorkflow
}

// NewMessageProcessor creates a new message processor
func NewMessageProcessor(
	llmClient llm.Client,
	toolRouter tools.ToolRouter,
	conversations *models.ConversationHistory,
	securityService *security.ConfirmationService,
	resourceMonitor *security.ResourceMonitor,
) *MessageProcessor {
	toolWorkflow := NewToolExecutionWorkflow(
		toolRouter,
		conversations,
		securityService,
		resourceMonitor,
	)
	
	return &MessageProcessor{
		llmClient:     llmClient,
		toolRouter:    toolRouter,
		conversations: conversations,
		toolWorkflow:  toolWorkflow,
	}
}

// ProcessMessage handles the complete message processing workflow
func (mp *MessageProcessor) ProcessMessage(ctx context.Context, userInput string, modelName string) tea.Cmd {
	return func() tea.Msg {
		// Start processing the message
		return mp.processUserMessage(ctx, userInput, modelName)
	}
}

// processUserMessage handles the complete message processing workflow
func (mp *MessageProcessor) processUserMessage(ctx context.Context, userInput string, modelName string) tea.Msg {
	// Create initial conversation block
	conversationID := uuid.New().String()
	
	// Add to conversation history
	if err := mp.conversations.AddConversation(userInput, ""); err != nil {
		return MessageProcessingErrorMsg{
			ConversationID: conversationID,
			Error:          fmt.Errorf("failed to add conversation: %w", err),
		}
	}
	
	// Update status to executing
	mp.conversations.UpdateConversationStatus(conversationID, "executing")
	
	// Start streaming LLM response
	return MessageProcessingStartedMsg{
		ConversationID: conversationID,
		ProcessorCmd:   mp.startLLMStreaming(ctx, conversationID, userInput, modelName),
	}
}

// startLLMStreaming initiates the LLM streaming process
func (mp *MessageProcessor) startLLMStreaming(ctx context.Context, conversationID, userInput, modelName string) tea.Cmd {
	return func() tea.Msg {
		// Get available tools
		toolSchemas := mp.getAllToolSchemas()
		toolDefinitions := mp.convertSchemasToLLMTools(toolSchemas)
		
		// Create chat request
		req := &llm.ChatRequest{
			Model: modelName,
			Messages: []llm.ChatMessage{
				{
					Role:    "user",
					Content: userInput,
				},
			},
			Tools:       toolDefinitions,
			Stream:      true,
			Temperature: floatPtr(0.7),
			MaxTokens:   intPtr(4000),
		}
		
		// Start streaming
		streamChan, err := mp.llmClient.StreamChat(ctx, req)
		if err != nil {
			return MessageProcessingErrorMsg{
				ConversationID: conversationID,
				Error:          fmt.Errorf("failed to start LLM streaming: %w", err),
			}
		}
		
		return LLMStreamingStartedMsg{
			ConversationID: conversationID,
			StreamChannel:  streamChan,
			ProcessorCmd:   mp.handleLLMStreaming(ctx, conversationID, streamChan),
		}
	}
}

// handleLLMStreaming processes the streaming LLM response
func (mp *MessageProcessor) handleLLMStreaming(ctx context.Context, conversationID string, streamChan <-chan *llm.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		var completeResponse string
		var toolCalls []llm.ToolCall
		var tokenUsage llm.TokenUsage
		
		for {
			select {
			case <-ctx.Done():
				return MessageProcessingErrorMsg{
					ConversationID: conversationID,
					Error:          ctx.Err(),
				}
				
			case event, ok := <-streamChan:
				if !ok {
					// Stream finished - process complete response
					return mp.handleLLMResponseComplete(ctx, conversationID, completeResponse, toolCalls, tokenUsage)
				}
				
				switch event.Type {
				case "message":
					// Handle streaming message content
					if streamResp, ok := event.Data.(*llm.StreamResponse); ok {
						for _, choice := range streamResp.Choices {
							if choice.Delta.Content != "" {
								completeResponse += choice.Delta.Content
								// Send streaming update
								tea.Sequence(
									func() tea.Msg {
										return StreamingContentUpdateMsg{
											ConversationID: conversationID,
											Content:        choice.Delta.Content,
											Complete:       false,
										}
									},
								)()
							}
							
							// Handle tool calls in streaming response
							for _, toolCall := range choice.Delta.ToolCalls {
								toolCalls = append(toolCalls, toolCall)
							}
						}
					}
					
				case "error":
					return MessageProcessingErrorMsg{
						ConversationID: conversationID,
						Error:          event.Error,
					}
					
				case "done":
					// Extract final token usage if available
					if finalResp, ok := event.Data.(*llm.ChatResponse); ok {
						tokenUsage = finalResp.Usage
					}
					
					return mp.handleLLMResponseComplete(ctx, conversationID, completeResponse, toolCalls, tokenUsage)
				}
			}
		}
	}
}

// handleLLMResponseComplete processes the complete LLM response and executes tools if needed
func (mp *MessageProcessor) handleLLMResponseComplete(
	ctx context.Context,
	conversationID string,
	response string,
	toolCalls []llm.ToolCall,
	tokenUsage llm.TokenUsage,
) tea.Msg {
	// Update conversation with LLM response
	conv, err := mp.conversations.GetConversationByID(conversationID)
	if err != nil {
		return MessageProcessingErrorMsg{
			ConversationID: conversationID,
			Error:          fmt.Errorf("failed to get conversation: %w", err),
		}
	}
	
	if conv != nil {
		conv.LLMResponse = response
		conv.TokenUsage = types.TokenUsage{
			InputTokens:  tokenUsage.PromptTokens,
			OutputTokens: tokenUsage.CompletionTokens,
		}
	}
	
	// If no tool calls, mark as completed
	if len(toolCalls) == 0 {
		mp.conversations.UpdateConversationStatus(conversationID, "completed")
		return MessageProcessingCompleteMsg{
			ConversationID: conversationID,
			FinalResponse:  response,
		}
	}
	
	// Convert LLM tool calls to internal format and start execution
	internalToolCalls := mp.convertLLMToolCalls(toolCalls)
	
	// Update conversation with tool calls
	if conv != nil {
		conv.ToolCalls = internalToolCalls
	}
	
	return ToolExecutionStartedMsg{
		ConversationID: conversationID,
		ToolCalls:      internalToolCalls,
		ProcessorCmd:   mp.toolWorkflow.ExecuteToolCalls(ctx, conversationID, internalToolCalls),
	}
}


// Helper methods

// getAllToolSchemas gets schemas for all registered tools
func (mp *MessageProcessor) getAllToolSchemas() map[string]*tools.ToolSchema {
	schemas := make(map[string]*tools.ToolSchema)
	
	// Get all tool names
	toolNames := mp.toolRouter.ListTools()
	
	// Get schema for each tool
	for _, toolName := range toolNames {
		schema, err := mp.toolRouter.GetSchema(toolName)
		if err == nil && schema != nil {
			schemas[toolName] = schema
		}
	}
	
	return schemas
}

func (mp *MessageProcessor) convertSchemasToLLMTools(schemas map[string]*tools.ToolSchema) []llm.ToolDefinition {
	var definitions []llm.ToolDefinition
	
	for _, schema := range schemas {
		def := llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionSchema{
				Name:        schema.Name,
				Description: schema.Description,
				Parameters: llm.FunctionParameters{
					Type:       "object",
					Properties: mp.convertProperties(schema.Parameters),
					Required:   schema.Required,
				},
			},
		}
		definitions = append(definitions, def)
	}
	
	return definitions
}

func (mp *MessageProcessor) convertProperties(params map[string]interface{}) map[string]llm.PropertySchema {
	props := make(map[string]llm.PropertySchema)
	
	for name, param := range params {
		// Convert interface{} parameter schema to PropertySchema
		// The tools.ToolSchema.Parameters is map[string]interface{} containing JSON schema
		if paramMap, ok := param.(map[string]interface{}); ok {
			propSchema := llm.PropertySchema{}
			
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
			
			props[name] = propSchema
		} else {
			// Fallback for simple string parameters
			props[name] = llm.PropertySchema{
				Type:        "string",
				Description: fmt.Sprintf("Parameter %s", name),
			}
		}
	}
	
	return props
}

func (mp *MessageProcessor) convertLLMToolCalls(llmToolCalls []llm.ToolCall) []types.ToolCall {
	var toolCalls []types.ToolCall
	
	for _, tc := range llmToolCalls {
		args, _ := tc.Function.ParseArguments()
		
		toolCall := types.ToolCall{
			ID:         tc.ID,
			ToolName:   tc.Function.Name,
			Parameters: args,
			Timestamp:  time.Now(),
		}
		
		toolCalls = append(toolCalls, toolCall)
	}
	
	return toolCalls
}

// Utility functions
func floatPtr(f float64) *float64 { return &f }
func intPtr(i int) *int { return &i }