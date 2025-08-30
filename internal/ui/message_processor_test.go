package ui

import (
	"context"
	"testing"
	"time"

	"gocodenow/internal/config"
	"gocodenow/internal/llm"
	"gocodenow/internal/models"
	"gocodenow/internal/tools"
)

// Mock implementations for testing

// MockLLMClient implements the llm.Client interface for testing
type MockLLMClient struct {
	responses       []llm.ChatResponse
	streamResponses [][]llm.StreamEvent
	responseIndex   int
}

func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		responses:       []llm.ChatResponse{},
		streamResponses: [][]llm.StreamEvent{},
		responseIndex:   0,
	}
}

func (m *MockLLMClient) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.responseIndex < len(m.responses) {
		resp := m.responses[m.responseIndex]
		m.responseIndex++
		return &resp, nil
	}
	
	// Default response
	return &llm.ChatResponse{
		ID:      "test-response-id",
		Model:   req.Model,
		Choices: []llm.ChatChoice{
			{
				Message: llm.ChatMessage{
					Role:    "assistant",
					Content: "Test response",
				},
			},
		},
	}, nil
}

func (m *MockLLMClient) StreamChat(ctx context.Context, req *llm.ChatRequest) (<-chan *llm.StreamEvent, error) {
	ch := make(chan *llm.StreamEvent, 10)
	
	go func() {
		defer close(ch)
		
		// Send a simple streaming response
		ch <- &llm.StreamEvent{
			Type: "message",
			Data: &llm.StreamResponse{
				ID:    "stream-1",
				Model: req.Model,
				Choices: []llm.StreamChoice{
					{
						Delta: llm.ChatMessage{
							Content: "Hello ",
						},
					},
				},
			},
			Timestamp: time.Now(),
		}
		
		ch <- &llm.StreamEvent{
			Type: "message",
			Data: &llm.StreamResponse{
				ID:    "stream-2",
				Model: req.Model,
				Choices: []llm.StreamChoice{
					{
						Delta: llm.ChatMessage{
							Content: "world!",
						},
					},
				},
			},
			Timestamp: time.Now(),
		}
		
		ch <- &llm.StreamEvent{
			Type: "done",
			Data: &llm.ChatResponse{
				Usage: llm.TokenUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			},
			Timestamp: time.Now(),
		}
	}()
	
	return ch, nil
}

func (m *MockLLMClient) ListModels(ctx context.Context) (*llm.ModelsResponse, error) {
	return &llm.ModelsResponse{
		Data: []llm.ModelInfo{
			{ID: "test-model", Object: "model"},
		},
	}, nil
}

func (m *MockLLMClient) GetModel(ctx context.Context, modelID string) (*llm.ModelInfo, error) {
	return &llm.ModelInfo{
		ID:     modelID,
		Object: "model",
	}, nil
}

func (m *MockLLMClient) Close() error {
	return nil
}

func (m *MockLLMClient) Provider() string {
	return "mock"
}

func (m *MockLLMClient) Config() *config.LLMConfig {
	return nil
}

// MockToolRouter implements a simple tool router for testing
type MockToolRouter struct {
	tools map[string]tools.ToolExecutor
}

func NewMockToolRouter() *MockToolRouter {
	return &MockToolRouter{
		tools: make(map[string]tools.ToolExecutor),
	}
}

func (r *MockToolRouter) RegisterExecutor(toolName string, executor tools.ToolExecutor) error {
	r.tools[toolName] = executor
	return nil
}

func (r *MockToolRouter) UnregisterExecutor(toolName string) error {
	delete(r.tools, toolName)
	return nil
}

func (r *MockToolRouter) Execute(ctx context.Context, toolCall *tools.ToolCall) (*tools.ToolResult, error) {
	executor, exists := r.tools[toolCall.ToolName]
	if !exists {
		return &tools.ToolResult{
			ID:           "test-result-id",
			ToolCallID:   toolCall.ID,
			ToolName:     toolCall.ToolName,
			Success:      false,
			ErrorMessage: "Tool not found",
			Duration:     time.Millisecond,
			Timestamp:    time.Now(),
		}, nil
	}
	
	result, err := executor.Execute(ctx, toolCall.Parameters)
	if err != nil {
		return &tools.ToolResult{
			ID:           "test-result-id",
			ToolCallID:   toolCall.ID,
			ToolName:     toolCall.ToolName,
			Success:      false,
			ErrorMessage: err.Error(),
			Duration:     time.Millisecond,
			Timestamp:    time.Now(),
		}, err
	}
	
	return result, nil
}

func (r *MockToolRouter) ListTools() []string {
	var toolNames []string
	for name := range r.tools {
		toolNames = append(toolNames, name)
	}
	return toolNames
}

func (r *MockToolRouter) GetExecutor(toolName string) (tools.ToolExecutor, bool) {
	executor, exists := r.tools[toolName]
	return executor, exists
}

func (r *MockToolRouter) GetSchema(toolName string) (*tools.ToolSchema, error) {
	executor, exists := r.tools[toolName]
	if !exists {
		return nil, &tools.ToolValidationError{
			ToolName: toolName,
			Message:  "tool not found",
		}
	}
	
	schema := executor.Schema()
	return &schema, nil
}

func (r *MockToolRouter) ValidateCall(toolCall *tools.ToolCall) error {
	executor, exists := r.tools[toolCall.ToolName]
	if !exists {
		return &tools.ToolValidationError{
			ToolName: toolCall.ToolName,
			Message:  "tool not found",
		}
	}
	
	return executor.ValidateParameters(toolCall.Parameters)
}

func (r *MockToolRouter) BatchExecute(ctx context.Context, toolCalls []*tools.ToolCall) ([]*tools.ToolResult, error) {
	var results []*tools.ToolResult
	for _, toolCall := range toolCalls {
		result, _ := r.Execute(ctx, toolCall)
		results = append(results, result)
	}
	return results, nil
}

func (r *MockToolRouter) ExecuteConcurrent(ctx context.Context, toolCalls []*tools.ToolCall) ([]*tools.ToolResult, error) {
	return r.BatchExecute(ctx, toolCalls)
}

// Test cases

func TestNewMessageProcessor(t *testing.T) {
	llmClient := NewMockLLMClient()
	toolRouter := NewMockToolRouter()
	conversations := models.NewConversationHistoryInMemory()
	
	processor := NewMessageProcessor(llmClient, toolRouter, conversations, nil, nil)
	
	if processor.llmClient != llmClient {
		t.Error("LLM client not set correctly")
	}
	
	if processor.toolRouter != toolRouter {
		t.Error("Tool router not set correctly")
	}
	
	if processor.conversations != conversations {
		t.Error("Conversations not set correctly")
	}
}

func TestMessageProcessor_ProcessMessage(t *testing.T) {
	llmClient := NewMockLLMClient()
	toolRouter := NewMockToolRouter()
	conversations := models.NewConversationHistoryInMemory()
	
	processor := NewMessageProcessor(llmClient, toolRouter, conversations, nil, nil)
	
	// Test processing a simple message
	ctx := context.Background()
	userInput := "Hello, world!"
	modelName := "test-model"
	
	cmd := processor.ProcessMessage(ctx, userInput, modelName)
	
	// Execute the command to get the message
	msg := cmd()
	
	// Should return a MessageProcessingStartedMsg
	if startedMsg, ok := msg.(MessageProcessingStartedMsg); ok {
		if startedMsg.ConversationID == "" {
			t.Error("Expected conversation ID to be set")
		}
		
		if startedMsg.ProcessorCmd == nil {
			t.Error("Expected processor command to be set")
		}
	} else {
		t.Errorf("Expected MessageProcessingStartedMsg, got %T", msg)
	}
}

func TestMessageProcessor_ConvertSchemasToLLMTools(t *testing.T) {
	llmClient := NewMockLLMClient()
	toolRouter := NewMockToolRouter()
	conversations := models.NewConversationHistoryInMemory()
	
	processor := NewMessageProcessor(llmClient, toolRouter, conversations, nil, nil)
	
	// Create test schemas
	schemas := map[string]*tools.ToolSchema{
		"test_tool": {
			Name:        "test_tool",
			Description: "A test tool",
			Parameters: map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input parameter",
				},
			},
			Required: []string{"input"},
		},
	}
	
	definitions := processor.convertSchemasToLLMTools(schemas)
	
	if len(definitions) != 1 {
		t.Errorf("Expected 1 tool definition, got %d", len(definitions))
	}
	
	def := definitions[0]
	if def.Function.Name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", def.Function.Name)
	}
	
	if def.Function.Description != "A test tool" {
		t.Errorf("Expected description 'A test tool', got '%s'", def.Function.Description)
	}
	
	if len(def.Function.Parameters.Properties) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(def.Function.Parameters.Properties))
	}
	
	if len(def.Function.Parameters.Required) != 1 {
		t.Errorf("Expected 1 required parameter, got %d", len(def.Function.Parameters.Required))
	}
}

func TestMessageProcessor_ConvertLLMToolCalls(t *testing.T) {
	llmClient := NewMockLLMClient()
	toolRouter := NewMockToolRouter()
	conversations := models.NewConversationHistoryInMemory()
	
	processor := NewMessageProcessor(llmClient, toolRouter, conversations, nil, nil)
	
	// Create test LLM tool calls
	llmToolCalls := []llm.ToolCall{
		{
			ID:   "test-call-1",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "test_tool",
				Arguments: `{"input": "test value"}`,
			},
		},
	}
	
	toolCalls := processor.convertLLMToolCalls(llmToolCalls)
	
	if len(toolCalls) != 1 {
		t.Errorf("Expected 1 tool call, got %d", len(toolCalls))
	}
	
	toolCall := toolCalls[0]
	if toolCall.ID != "test-call-1" {
		t.Errorf("Expected ID 'test-call-1', got '%s'", toolCall.ID)
	}
	
	if toolCall.ToolName != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", toolCall.ToolName)
	}
	
	if inputVal, exists := toolCall.Parameters["input"]; !exists {
		t.Error("Expected 'input' parameter to exist")
	} else if inputStr, ok := inputVal.(string); !ok {
		t.Errorf("Expected input to be string, got %T", inputVal)
	} else if inputStr != "test value" {
		t.Errorf("Expected input 'test value', got '%s'", inputStr)
	}
}