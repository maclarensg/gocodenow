// Package llm provides LLM API integration with support for multiple providers
// including OpenAI, Anthropic, and local/generic OpenAI-compatible APIs.
//
// This package handles:
// - Chat completion requests and responses
// - Tool call parsing and execution
// - Streaming responses
// - Provider-specific authentication and formatting
// - Token usage tracking and rate limiting
//
// Example usage:
//
//	// Create a client factory
//	factory := NewClientFactory()
//	
//	// Create a client for OpenAI
//	client, err := factory.CreateClient(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//	
//	// Send a chat request
//	response, err := client.Chat(ctx, &ChatRequest{
//		Model: "gpt-4",
//		Messages: []ChatMessage{{
//			Role: "user", 
//			Content: "Hello, world!",
//		}},
//	})
//
package llm

import (
	"context"
	"fmt"
	
	"gocodenow/internal/config"
)

// Client represents an LLM API client capable of sending requests and receiving responses
type Client interface {
	// Chat sends a chat completion request and returns the response
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	
	// StreamChat sends a streaming chat completion request and returns a channel of events
	StreamChat(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error)
	
	// ListModels returns the list of available models for this client
	ListModels(ctx context.Context) (*ModelsResponse, error)
	
	// GetModel returns information about a specific model
	GetModel(ctx context.Context, modelID string) (*ModelInfo, error)
	
	// Close cleanly shuts down the client and releases resources
	Close() error
	
	// Provider returns the provider name for this client
	Provider() string
	
	// Config returns the configuration used by this client
	Config() *config.LLMConfig
}

// Provider represents different LLM API providers (OpenAI, Anthropic, Local, etc.)
type Provider interface {
	// Name returns the human-readable name of this provider
	Name() string
	
	// ID returns a unique identifier for this provider
	ID() string
	
	// CreateClient creates a new client instance for this provider
	CreateClient(config *config.LLMConfig) (Client, error)
	
	// ValidateConfig validates provider-specific configuration
	ValidateConfig(config *config.LLMConfig) error
	
	// SupportedModels returns a list of models supported by this provider
	SupportedModels() []string
	
	// DetectFromEndpoint attempts to detect if this provider matches the given endpoint
	DetectFromEndpoint(endpoint string) bool
	
	// GetDefaultModel returns the default model for this provider
	GetDefaultModel() string
}

// ClientFactory manages provider registration and client creation
type ClientFactory struct {
	providers map[string]Provider
	detector  *EndpointDetector
}

// NewClientFactory creates a new client factory with default providers registered
func NewClientFactory() *ClientFactory {
	factory := &ClientFactory{
		providers: make(map[string]Provider),
		detector:  NewEndpointDetector(),
	}
	
	// Register default providers
	factory.RegisterProvider(&OpenAIProvider{})
	factory.RegisterProvider(&AnthropicProvider{})
	factory.RegisterProvider(&LocalProvider{})
	
	return factory
}

// RegisterProvider registers a new provider with the factory
func (f *ClientFactory) RegisterProvider(provider Provider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}
	
	id := provider.ID()
	if id == "" {
		return fmt.Errorf("provider ID cannot be empty")
	}
	
	if _, exists := f.providers[id]; exists {
		return fmt.Errorf("provider with ID %s already registered", id)
	}
	
	f.providers[id] = provider
	return nil
}

// CreateClient creates a client using automatic provider detection
func (f *ClientFactory) CreateClient(config *config.LLMConfig) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	
	// Try to detect provider from endpoint
	providerID := f.detector.DetectProvider(config.Endpoint)
	if providerID == "" {
		// Default to local provider for generic endpoints
		providerID = "local"
	}
	
	return f.CreateClientForProvider(providerID, config)
}

// CreateClientForProvider creates a client for a specific provider
func (f *ClientFactory) CreateClientForProvider(providerID string, config *config.LLMConfig) (Client, error) {
	provider, exists := f.providers[providerID]
	if !exists {
		return nil, fmt.Errorf("unknown provider: %s", providerID)
	}
	
	// Validate configuration for this provider
	if err := provider.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config for provider %s: %w", providerID, err)
	}
	
	return provider.CreateClient(config)
}

// ListProviders returns the IDs of all registered providers
func (f *ClientFactory) ListProviders() []string {
	var providers []string
	for id := range f.providers {
		providers = append(providers, id)
	}
	return providers
}

// GetProvider returns a provider by ID
func (f *ClientFactory) GetProvider(id string) (Provider, bool) {
	provider, exists := f.providers[id]
	return provider, exists
}

// EndpointDetector detects LLM providers based on endpoint URLs
type EndpointDetector struct {
	patterns map[string][]string
}

// NewEndpointDetector creates a new endpoint detector with default patterns
func NewEndpointDetector() *EndpointDetector {
	return &EndpointDetector{
		patterns: map[string][]string{
			"openai": {
				"api.openai.com",
				"openai.com",
			},
			"anthropic": {
				"api.anthropic.com",
				"anthropic.com",
			},
			// Local providers are detected as fallback
		},
	}
}

// DetectProvider detects the provider based on an endpoint URL
func (d *EndpointDetector) DetectProvider(endpoint string) string {
	for providerID, patterns := range d.patterns {
		for _, pattern := range patterns {
			if containsPattern(endpoint, pattern) {
				return providerID
			}
		}
	}
	return "" // Unknown provider
}

// containsPattern checks if an endpoint contains a specific pattern
func containsPattern(endpoint, pattern string) bool {
	// Simple substring check - could be enhanced with regex if needed
	return len(endpoint) > 0 && len(pattern) > 0 && 
		   (endpoint == pattern || 
		    contains(endpoint, pattern))
}

// contains is a simple string contains helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexSubstring(s, substr) >= 0)
}

// indexSubstring finds the index of substr in s
func indexSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}