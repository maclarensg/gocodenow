package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gocodenow/internal/llm"
	"gocodenow/internal/models"
	"gocodenow/internal/security"
	"gocodenow/internal/tools"
	"gocodenow/internal/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sirupsen/logrus"
)

// Debug logger for message processor (separate from LLM logger)
var mpLogger *logrus.Logger

func init() {
	mpLogger = logrus.New()
	
	// Check if debug logging is enabled
	if os.Getenv("GOCODENOW_DEBUG") == "1" || os.Getenv("GOCODENOW_LOGTRACE") == "1" {
		mpLogger.SetLevel(logrus.DebugLevel)
		
		// Create log file in current working directory
		wd, _ := os.Getwd()
		logPath := filepath.Join(wd, "gocodenow-message-processor.log")
		
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open MP log file: %v\n", err)
			mpLogger.SetOutput(os.Stderr)
		} else {
			mpLogger.SetOutput(logFile)
		}
		
		mpLogger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			TimestampFormat: "2006-01-02 15:04:05.000",
		})
		
		mpLogger.Debug("Message processor debug logging enabled")
	} else {
		mpLogger.SetLevel(logrus.PanicLevel)
		mpLogger.SetOutput(io.Discard)
	}
}

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
	// Add to conversation history and get the actual conversation ID
	conversationID, err := mp.conversations.AddConversation(userInput, "", types.StatusExecuting)
	if err != nil {
		return MessageProcessingErrorMsg{
			ConversationID: "",
			Error:          fmt.Errorf("failed to add conversation: %w", err),
		}
	}
	
	// Status is already set to executing by AddConversation, no need to update it again
	
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
	return tea.Batch(mp.processStreamEvents(ctx, conversationID, streamChan))
}

// processStreamEvents handles the actual streaming and returns a command that sends updates
func (mp *MessageProcessor) processStreamEvents(ctx context.Context, conversationID string, streamChan <-chan *llm.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		// Start the streaming loop
		return mp.streamProcessor(ctx, conversationID, streamChan)
	}
}

// streamProcessor handles the streaming events and sends updates
func (mp *MessageProcessor) streamProcessor(ctx context.Context, conversationID string, streamChan <-chan *llm.StreamEvent) tea.Msg {
	var completeResponse string
	var toolCalls []llm.ToolCall
	var tokenUsage llm.TokenUsage
	
	mpLogger.WithField("conversation_id", conversationID).Debug("Starting stream processing")
	
	for {
		select {
		case <-ctx.Done():
			mpLogger.WithFields(logrus.Fields{
				"conversation_id": conversationID,
				"error": ctx.Err(),
				"response_length": len(completeResponse),
			}).Error("Stream cancelled by context - this indicates a timeout or user cancellation")
			return MessageProcessingErrorMsg{
				ConversationID: conversationID,
				Error:          fmt.Errorf("streaming interrupted: %w", ctx.Err()),
			}
			
		case event, ok := <-streamChan:
			if !ok {
				// Stream finished - process complete response
				mpLogger.WithFields(logrus.Fields{
					"conversation_id": conversationID,
					"complete_response_length": len(completeResponse),
					"tool_calls_count": len(toolCalls),
					"input_tokens": tokenUsage.PromptTokens,
					"output_tokens": tokenUsage.CompletionTokens,
				}).Info("Stream channel closed normally, processing completion")
				
				if len(completeResponse) == 0 && len(toolCalls) == 0 {
					mpLogger.WithField("conversation_id", conversationID).Warn("Stream completed but no response content received")
					return MessageProcessingErrorMsg{
						ConversationID: conversationID,
						Error:          fmt.Errorf("no response received from LLM - connection may have been terminated"),
					}
				}
				
				return mp.handleLLMResponseComplete(ctx, conversationID, completeResponse, toolCalls, tokenUsage)
			}
			
			switch event.Type {
			case "message":
				// Handle streaming message content
				if streamResp, ok := event.Data.(*llm.StreamResponse); ok {
					for _, choice := range streamResp.Choices {
						if choice.Delta.Content != "" {
							completeResponse += choice.Delta.Content
							// Send streaming update to UI for visual feedback
							// TODO: Send intermediate update here for live streaming
						}
						
						// Handle tool calls in streaming response
						for _, toolCall := range choice.Delta.ToolCalls {
							toolCalls = append(toolCalls, toolCall)
						}
					}
				}
				
			case "error":
				mpLogger.WithError(event.Error).Debug("Stream error received")
				return MessageProcessingErrorMsg{
					ConversationID: conversationID,
					Error:          event.Error,
				}
				
			case "done":
				// Extract final token usage if available
				if finalResp, ok := event.Data.(*llm.ChatResponse); ok {
					tokenUsage = finalResp.Usage
				}
				
				mpLogger.WithFields(logrus.Fields{
					"conversation_id": conversationID,
					"complete_response": completeResponse,
					"tool_calls_count": len(toolCalls),
				}).Debug("Processing stream completion")
				
				result := mp.handleLLMResponseComplete(ctx, conversationID, completeResponse, toolCalls, tokenUsage)
				mpLogger.WithField("result_type", fmt.Sprintf("%T", result)).Debug("Stream completion result")
				return result
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
	mpLogger.WithFields(logrus.Fields{
		"conversation_id": conversationID,
		"response": response,
		"tool_calls_count": len(toolCalls),
	}).Debug("handleLLMResponseComplete called")
	
	// Update conversation with LLM response
	tokenUsageTyped := types.TokenUsage{
		InputTokens:  tokenUsage.PromptTokens,
		OutputTokens: tokenUsage.CompletionTokens,
	}
	
	if err := mp.conversations.UpdateConversationResponse(conversationID, response, tokenUsageTyped); err != nil {
		mpLogger.WithError(err).Error("Failed to update conversation with LLM response")
		return MessageProcessingErrorMsg{
			ConversationID: conversationID,
			Error:          fmt.Errorf("failed to update conversation: %w", err),
		}
	}
	
	mpLogger.WithField("conversation_id", conversationID).Debug("Successfully updated conversation with LLM response")
	
	// If no tool calls, mark as completed
	if len(toolCalls) == 0 {
		mpLogger.WithField("conversation_id", conversationID).Debug("Updating conversation status to completed")
		mp.conversations.UpdateConversationStatus(conversationID, "completed")
		
		completionMsg := MessageProcessingCompleteMsg{
			ConversationID: conversationID,
			FinalResponse:  response,
		}
		mpLogger.WithField("conversation_id", conversationID).Debug("Returning MessageProcessingCompleteMsg")
		return completionMsg
	}
	
	// Convert LLM tool calls to internal format and start execution
	internalToolCalls := mp.convertLLMToolCalls(toolCalls)
	
	// TODO: Update conversation with tool calls if needed (for now, tool execution will handle this)
	
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