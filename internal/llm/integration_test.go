package llm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gocodenow/internal/config"
)

// Integration test configuration
const (
	// Default test endpoint - can be overridden with environment variable
	DefaultTestEndpoint = "http://192.168.86.20:1234"
	DefaultTestModel    = "qwen2.5-coder-7b-instruct"
	
	// Environment variables for configuration
	EnvTestEndpoint = "LLM_TEST_ENDPOINT"
	EnvTestModel    = "LLM_TEST_MODEL"
	EnvTestToken    = "LLM_TEST_TOKEN"
	EnvRunIntegrationTests = "RUN_LLM_INTEGRATION_TESTS"
)

// getTestConfig returns configuration for integration tests
func getTestConfig() *config.LLMConfig {
	endpoint := os.Getenv(EnvTestEndpoint)
	if endpoint == "" {
		endpoint = DefaultTestEndpoint
	}
	
	// Ensure endpoint has /v1 suffix
	if !strings.HasSuffix(endpoint, "/v1") {
		endpoint = strings.TrimSuffix(endpoint, "/") + "/v1"
	}
	
	model := os.Getenv(EnvTestModel)
	if model == "" {
		model = DefaultTestModel
	}
	
	token := os.Getenv(EnvTestToken) // Optional for local servers
	
	return &config.LLMConfig{
		Endpoint:   endpoint,
		Model:      model,
		Token:      token,
		Timeout:    30, // 30 seconds for integration tests
		MaxRetries: 2,
	}
}

// skipIfNoIntegrationTests skips the test if integration tests are not enabled
func skipIfNoIntegrationTests(t *testing.T) {
	if os.Getenv(EnvRunIntegrationTests) != "true" {
		t.Skipf("Skipping integration test. Set %s=true to run integration tests.", EnvRunIntegrationTests)
	}
}

func TestIntegration_ClientFactory_CreateClient(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	// Verify client configuration
	if client.Provider() == "" {
		t.Error("Client provider should not be empty")
	}
	
	if client.Config().Endpoint != config.Endpoint {
		t.Errorf("Expected endpoint %s, got %s", config.Endpoint, client.Config().Endpoint)
	}
	
	if client.Config().Model != config.Model {
		t.Errorf("Expected model %s, got %s", config.Model, client.Config().Model)
	}
	
	t.Logf("Successfully created client for provider: %s", client.Provider())
}

func TestIntegration_ListModels(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("Failed to list models: %v", err)
	}
	
	if models == nil {
		t.Fatal("Models response should not be nil")
	}
	
	if len(models.Data) == 0 {
		t.Error("Expected at least one model in response")
	}
	
	// Check if our test model is available
	modelFound := false
	for _, model := range models.Data {
		t.Logf("Available model: %s", model.ID)
		if model.ID == config.Model {
			modelFound = true
		}
	}
	
	if !modelFound {
		t.Logf("Warning: Test model %s not found in available models", config.Model)
	}
}

func TestIntegration_GetModel(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	model, err := client.GetModel(ctx, config.Model)
	if err != nil {
		t.Fatalf("Failed to get model info: %v", err)
	}
	
	if model == nil {
		t.Fatal("Model info should not be nil")
	}
	
	if model.ID != config.Model {
		t.Errorf("Expected model ID %s, got %s", config.Model, model.ID)
	}
	
	t.Logf("Model info: ID=%s, Object=%s, OwnedBy=%s", model.ID, model.Object, model.OwnedBy)
}

func TestIntegration_SimpleChatCompletion(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Simple chat request without tools
	request := &ChatRequest{
		Model: config.Model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful coding assistant. Respond concisely.",
			},
			{
				Role:    "user",
				Content: "What is 2 + 2? Please respond with just the number.",
			},
		},
		MaxTokens:   &[]int{50}[0],
		Temperature: &[]float64{0.1}[0],
	}
	
	response, err := client.Chat(ctx, request)
	if err != nil {
		t.Fatalf("Failed to get chat completion: %v", err)
	}
	
	if response == nil {
		t.Fatal("Response should not be nil")
	}
	
	if len(response.Choices) == 0 {
		t.Fatal("Response should have at least one choice")
	}
	
	choice := response.Choices[0]
	if choice.Message.Role != "assistant" {
		t.Errorf("Expected assistant role, got %s", choice.Message.Role)
	}
	
	if choice.Message.Content == "" {
		t.Error("Response content should not be empty")
	}
	
	// Check if response contains "4" (the answer to 2+2)
	if !strings.Contains(choice.Message.Content, "4") {
		t.Logf("Warning: Expected response to contain '4', got: %s", choice.Message.Content)
	}
	
	// Verify token usage
	if response.Usage.TotalTokens == 0 {
		t.Error("Total tokens should be greater than 0")
	}
	
	t.Logf("Chat completion successful!")
	t.Logf("Response: %s", choice.Message.Content)
	t.Logf("Token usage: %d prompt + %d completion = %d total", 
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
}

func TestIntegration_ChatCompletionWithTools(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Define a simple calculator tool
	calculatorTool := ToolDefinition{
		Type: "function",
		Function: FunctionSchema{
			Name:        "calculate",
			Description: "Perform basic arithmetic operations",
			Parameters: FunctionParameters{
				Type: "object",
				Properties: map[string]PropertySchema{
					"operation": {
						Type:        "string",
						Description: "The operation to perform (add, subtract, multiply, divide)",
						Enum:        []interface{}{"add", "subtract", "multiply", "divide"},
					},
					"a": {
						Type:        "number",
						Description: "The first number",
					},
					"b": {
						Type:        "number",
						Description: "The second number",
					},
				},
				Required: []string{"operation", "a", "b"},
			},
		},
	}
	
	// Chat request with tools
	request := &ChatRequest{
		Model: config.Model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant with access to a calculator tool. Use the calculate tool when asked to perform arithmetic.",
			},
			{
				Role:    "user",
				Content: "Please calculate 15 + 27 using the calculator tool.",
			},
		},
		Tools:       []ToolDefinition{calculatorTool},
		ToolChoice:  "auto",
		MaxTokens:   &[]int{200}[0],
		Temperature: &[]float64{0.1}[0],
	}
	
	response, err := client.Chat(ctx, request)
	if err != nil {
		t.Fatalf("Failed to get chat completion with tools: %v", err)
	}
	
	if response == nil {
		t.Fatal("Response should not be nil")
	}
	
	if len(response.Choices) == 0 {
		t.Fatal("Response should have at least one choice")
	}
	
	choice := response.Choices[0]
	
	// Check if the model made a tool call
	if len(choice.Message.ToolCalls) > 0 {
		t.Logf("Model made %d tool call(s)", len(choice.Message.ToolCalls))
		
		for i, toolCall := range choice.Message.ToolCalls {
			t.Logf("Tool call %d: ID=%s, Type=%s, Function=%s", 
				i+1, toolCall.ID, toolCall.Type, toolCall.Function.Name)
			
			if toolCall.Function.Name != "calculate" {
				t.Errorf("Expected tool call to 'calculate', got '%s'", toolCall.Function.Name)
			}
			
			// Try to parse the arguments
			args, err := toolCall.Function.ParseArguments()
			if err != nil {
				t.Errorf("Failed to parse tool call arguments: %v", err)
			} else {
				t.Logf("Tool call arguments: %+v", args)
				
				// Verify the arguments make sense for addition
				if operation, ok := args["operation"].(string); ok {
					if operation != "add" {
						t.Logf("Warning: Expected operation 'add', got '%s'", operation)
					}
				}
			}
		}
	} else {
		t.Log("Model did not make any tool calls (this might be expected depending on the model's capabilities)")
	}
	
	t.Logf("Response content: %s", choice.Message.Content)
	t.Logf("Token usage: %d prompt + %d completion = %d total", 
		response.Usage.PromptTokens, response.Usage.CompletionTokens, response.Usage.TotalTokens)
}

func TestIntegration_StreamingChatCompletion(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	
	// Streaming chat request
	request := &ChatRequest{
		Model: config.Model,
		Messages: []ChatMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant. Keep responses concise.",
			},
			{
				Role:    "user",
				Content: "Count from 1 to 5, with each number on a new line.",
			},
		},
		MaxTokens:   &[]int{100}[0],
		Temperature: &[]float64{0.1}[0],
		Stream:      true,
	}
	
	eventChan, err := client.StreamChat(ctx, request)
	if err != nil {
		t.Fatalf("Failed to start streaming chat: %v", err)
	}
	
	eventCount := 0
	var receivedContent strings.Builder
	
	for event := range eventChan {
		eventCount++
		
		if event.Error != nil {
			t.Errorf("Stream event error: %v", event.Error)
			continue
		}
		
		t.Logf("Stream event %d: Type=%s", eventCount, event.Type)
		
		// Try to parse the event data based on type
		switch event.Type {
		case "content", "message":
			if event.Data != nil {
				if str, ok := event.Data.(string); ok {
					receivedContent.WriteString(str)
				}
			}
		case "done":
			t.Log("Stream completed")
		case "error":
			t.Errorf("Stream error event: %v", event.Data)
		}
		
		// Log raw event data for debugging
		if event.Raw != "" && len(event.Raw) < 200 {
			t.Logf("Raw event data: %s", event.Raw)
		}
	}
	
	if eventCount == 0 {
		t.Error("Expected to receive at least one stream event")
	}
	
	content := receivedContent.String()
	t.Logf("Total events received: %d", eventCount)
	t.Logf("Accumulated content: %s", content)
	
	// The content might be empty if the streaming format is different than expected
	// This is informational rather than a hard failure
	if content == "" {
		t.Log("No content accumulated from stream (this might be expected depending on the streaming format)")
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	// Test with invalid model
	factory := NewClientFactory()
	config := getTestConfig()
	invalidConfig := *config
	invalidConfig.Model = "nonexistent-model-12345"
	
	client, err := factory.CreateClient(&invalidConfig)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	request := &ChatRequest{
		Model: invalidConfig.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
		MaxTokens: &[]int{10}[0],
	}
	
	_, err = client.Chat(ctx, request)
	if err == nil {
		t.Log("Expected error for invalid model, but request succeeded (server might be very permissive)")
	} else {
		t.Logf("Got expected error for invalid model: %v", err)
		
		// Check if it's an LLM error with proper context
		if llmErr, ok := err.(*LLMError); ok {
			t.Logf("LLM Error type: %s, retryable: %t", llmErr.Type.String(), llmErr.Retryable)
		}
	}
}

func TestIntegration_ConnectionTimeout(t *testing.T) {
	skipIfNoIntegrationTests(t)
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	// Set a very short timeout
	timeoutConfig := *config
	timeoutConfig.Timeout = 1 // 1 second
	
	client, err := factory.CreateClient(&timeoutConfig)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	// Use an even shorter context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	request := &ChatRequest{
		Model: config.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: "This request should timeout quickly.",
			},
		},
		MaxTokens: &[]int{1000}[0], // Request many tokens to make it slower
	}
	
	_, err = client.Chat(ctx, request)
	if err == nil {
		t.Log("Expected timeout error, but request succeeded (server might be very fast)")
	} else {
		t.Logf("Got expected timeout error: %v", err)
		
		// Check if it's a timeout-related error
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "context deadline exceeded") {
			t.Log("Error correctly identified as timeout-related")
		}
	}
}

// Benchmark test for performance measurement
func BenchmarkIntegration_SimpleChatCompletion(b *testing.B) {
	if os.Getenv(EnvRunIntegrationTests) != "true" {
		b.Skip("Skipping integration benchmark. Set RUN_LLM_INTEGRATION_TESTS=true to run.")
	}
	
	factory := NewClientFactory()
	config := getTestConfig()
	
	client, err := factory.CreateClient(config)
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()
	
	request := &ChatRequest{
		Model: config.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: "Say 'Hello' once.",
			},
		},
		MaxTokens:   &[]int{10}[0],
		Temperature: &[]float64{0.0}[0],
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := client.Chat(ctx, request)
		cancel()
		
		if err != nil {
			b.Fatalf("Chat completion failed: %v", err)
		}
	}
}