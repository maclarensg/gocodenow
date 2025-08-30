package llm

import (
	"context"
	"testing"
	"time"

	"gocodenow/internal/config"
)

func TestClientFactory_NewClientFactory(t *testing.T) {
	factory := NewClientFactory()
	
	if factory == nil {
		t.Fatal("Factory should not be nil")
	}
	
	// Check that default providers are registered
	providers := factory.ListProviders()
	if len(providers) == 0 {
		t.Fatal("Factory should have default providers registered")
	}
	
	expectedProviders := map[string]bool{
		"openai":    false,
		"anthropic": false, 
		"local":     false,
	}
	
	for _, providerID := range providers {
		if _, expected := expectedProviders[providerID]; expected {
			expectedProviders[providerID] = true
		}
	}
	
	for providerID, found := range expectedProviders {
		if !found {
			t.Errorf("Expected provider %s not found in factory", providerID)
		}
	}
}

func TestClientFactory_RegisterProvider(t *testing.T) {
	factory := NewClientFactory()
	
	// Test registering nil provider
	err := factory.RegisterProvider(nil)
	if err == nil {
		t.Error("Expected error when registering nil provider")
	}
	
	// Create a mock provider for testing
	mockProvider := &MockProvider{
		id:   "test-provider",
		name: "Test Provider",
	}
	
	// Test successful registration
	err = factory.RegisterProvider(mockProvider)
	if err != nil {
		t.Fatalf("Failed to register valid provider: %v", err)
	}
	
	// Test duplicate registration
	err = factory.RegisterProvider(mockProvider)
	if err == nil {
		t.Error("Expected error when registering duplicate provider")
	}
	
	// Verify provider was registered
	provider, exists := factory.GetProvider("test-provider")
	if !exists {
		t.Error("Registered provider should exist")
	}
	if provider != mockProvider {
		t.Error("Retrieved provider should be the same instance")
	}
}

func TestClientFactory_CreateClient(t *testing.T) {
	factory := NewClientFactory()
	
	// Test with nil config
	_, err := factory.CreateClient(nil)
	if err == nil {
		t.Error("Expected error when creating client with nil config")
	}
	
	// Test with valid config for local provider
	config := &config.LLMConfig{
		Endpoint: "http://localhost:8080",
		Model:    "test-model",
		Token:    "test-token",
		Timeout:  30,
	}
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Fatalf("Failed to create client with valid config: %v", err)
	}
	
	if client == nil {
		t.Fatal("Client should not be nil")
	}
	
	// Clean up
	client.Close()
}

func TestClientFactory_CreateClientForProvider(t *testing.T) {
	factory := NewClientFactory()
	
	config := &config.LLMConfig{
		Endpoint: "http://localhost:8080",
		Model:    "test-model",
		Token:    "test-token",
		Timeout:  30,
	}
	
	// Test with unknown provider
	_, err := factory.CreateClientForProvider("unknown-provider", config)
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
	
	// Test with known provider
	client, err := factory.CreateClientForProvider("local", config)
	if err != nil {
		t.Fatalf("Failed to create client for known provider: %v", err)
	}
	
	if client == nil {
		t.Fatal("Client should not be nil")
	}
	
	if client.Provider() != "openai" { // Local provider uses OpenAI client
		t.Errorf("Expected provider 'openai', got '%s'", client.Provider())
	}
	
	// Clean up
	client.Close()
}

func TestEndpointDetector_DetectProvider(t *testing.T) {
	detector := NewEndpointDetector()
	
	testCases := []struct {
		endpoint string
		expected string
	}{
		{"https://api.openai.com", "openai"},
		{"https://api.openai.com/v1", "openai"},
		{"https://openai.com", "openai"},
		{"https://api.anthropic.com", "anthropic"},
		{"https://api.anthropic.com/v1", "anthropic"},
		{"https://anthropic.com", "anthropic"},
		{"http://localhost:8080", ""},
		{"https://unknown-provider.com", ""},
		{"", ""},
	}
	
	for _, tc := range testCases {
		result := detector.DetectProvider(tc.endpoint)
		if result != tc.expected {
			t.Errorf("DetectProvider(%s) = %s, expected %s", tc.endpoint, result, tc.expected)
		}
	}
}

func TestContainsPattern(t *testing.T) {
	testCases := []struct {
		endpoint string
		pattern  string
		expected bool
	}{
		{"https://api.openai.com", "openai.com", true},
		{"https://api.openai.com/v1", "openai.com", true},
		{"https://example.com", "openai.com", false},
		{"", "openai.com", false},
		{"https://api.openai.com", "", false},
		{"openai.com", "openai.com", true},
		{"test-openai.com", "openai.com", true},
	}
	
	for _, tc := range testCases {
		result := containsPattern(tc.endpoint, tc.pattern)
		if result != tc.expected {
			t.Errorf("containsPattern(%s, %s) = %t, expected %t", 
				tc.endpoint, tc.pattern, result, tc.expected)
		}
	}
}

func TestIndexSubstring(t *testing.T) {
	testCases := []struct {
		s        string
		substr   string
		expected int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello world", "xyz", -1},
		{"", "hello", -1},
		{"hello", "", 0},
		{"hello", "hello", 0},
		{"hello", "hell", 0},
		{"hello", "ello", 1},
	}
	
	for _, tc := range testCases {
		result := indexSubstring(tc.s, tc.substr)
		if result != tc.expected {
			t.Errorf("indexSubstring(%s, %s) = %d, expected %d", 
				tc.s, tc.substr, result, tc.expected)
		}
	}
}

// MockProvider for testing
type MockProvider struct {
	id   string
	name string
}

func (p *MockProvider) Name() string {
	return p.name
}

func (p *MockProvider) ID() string {
	return p.id
}

func (p *MockProvider) CreateClient(config *config.LLMConfig) (Client, error) {
	return &MockClient{config: config}, nil
}

func (p *MockProvider) ValidateConfig(config *config.LLMConfig) error {
	if config == nil {
		return NewValidationError("config cannot be nil")
	}
	return nil
}

func (p *MockProvider) SupportedModels() []string {
	return []string{"mock-model-1", "mock-model-2"}
}

func (p *MockProvider) DetectFromEndpoint(endpoint string) bool {
	return false
}

func (p *MockProvider) GetDefaultModel() string {
	return "mock-model-1"
}

// MockClient for testing
type MockClient struct {
	config *config.LLMConfig
}

func (c *MockClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	return &ChatResponse{
		ID:      "mock-response-id",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []ChatChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: "Mock response",
				},
				FinishReason: "stop",
			},
		},
		Usage: TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}, nil
}

func (c *MockClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error) {
	eventChan := make(chan *StreamEvent, 1)
	go func() {
		defer close(eventChan)
		eventChan <- &StreamEvent{
			Type:      "message",
			Data:      "Mock stream response",
			Timestamp: time.Now(),
		}
	}()
	return eventChan, nil
}

func (c *MockClient) ListModels(ctx context.Context) (*ModelsResponse, error) {
	return &ModelsResponse{
		Object: "list",
		Data: []ModelInfo{
			{
				ID:      "mock-model-1",
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "mock",
			},
		},
	}, nil
}

func (c *MockClient) GetModel(ctx context.Context, modelID string) (*ModelInfo, error) {
	return &ModelInfo{
		ID:      modelID,
		Object:  "model",
		Created: time.Now().Unix(),
		OwnedBy: "mock",
	}, nil
}

func (c *MockClient) Close() error {
	return nil
}

func (c *MockClient) Provider() string {
	return "mock"
}

func (c *MockClient) Config() *config.LLMConfig {
	return c.config
}

