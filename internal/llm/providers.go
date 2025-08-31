package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"gocodenow/internal/config"
	"github.com/sirupsen/logrus"
)

// Debug logging for LLM operations
var llmLogger *logrus.Logger

func init() {
	// Initialize logrus logger
	llmLogger = logrus.New()
	
	// Check if debug logging is enabled
	if os.Getenv("GOCODENOW_DEBUG") == "1" || os.Getenv("GOCODENOW_LOGTRACE") == "1" {
		llmLogger.SetLevel(logrus.DebugLevel)
		
		// Create log file in current working directory
		wd, _ := os.Getwd()
		logPath := filepath.Join(wd, "gocodenow-llm.log")
		
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open LLM log file: %v\n", err)
			llmLogger.SetOutput(os.Stderr)
		} else {
			llmLogger.SetOutput(logFile)
		}
		
		// Set formatter for structured logging
		llmLogger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
			TimestampFormat: "2006-01-02 15:04:05.000",
		})
		
		llmLogger.Debug("LLM debug logging enabled")
	} else {
		// Disable logging if not enabled
		llmLogger.SetLevel(logrus.PanicLevel)
		llmLogger.SetOutput(io.Discard)
	}
}

// OpenAIProvider implements the Provider interface for OpenAI API
type OpenAIProvider struct{}

// Name returns the human-readable name
func (p *OpenAIProvider) Name() string {
	return "OpenAI"
}

// ID returns the unique identifier
func (p *OpenAIProvider) ID() string {
	return "openai"
}

// CreateClient creates a new OpenAI client
func (p *OpenAIProvider) CreateClient(config *config.LLMConfig) (Client, error) {
	client := &OpenAIClient{
		config:     config,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		baseURL:    strings.TrimSuffix(config.Endpoint, "/"),
		headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "gocodenow/1.0",
		},
	}
	
	// Add authorization header if token is provided
	if config.Token != "" {
		client.headers["Authorization"] = "Bearer " + config.Token
	}
	
	// Ensure base URL ends with /v1
	if !strings.HasSuffix(client.baseURL, "/v1") {
		client.baseURL += "/v1"
	}
	
	return client, nil
}

// ValidateConfig validates OpenAI-specific configuration
func (p *OpenAIProvider) ValidateConfig(config *config.LLMConfig) error {
	if config.Endpoint == "" {
		return NewValidationError("endpoint is required for OpenAI provider")
	}
	
	if config.Model == "" {
		return NewValidationError("model is required for OpenAI provider")
	}
	
	// Token is optional for some local OpenAI-compatible APIs
	
	return nil
}

// SupportedModels returns OpenAI supported models
func (p *OpenAIProvider) SupportedModels() []string {
	return []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-preview",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-16k",
	}
}

// DetectFromEndpoint detects if endpoint is OpenAI
func (p *OpenAIProvider) DetectFromEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "openai.com") ||
		   strings.Contains(endpoint, "api.openai.com")
}

// GetDefaultModel returns the default model
func (p *OpenAIProvider) GetDefaultModel() string {
	return "gpt-4"
}

// OpenAIClient implements the Client interface for OpenAI
type OpenAIClient struct {
	config     *config.LLMConfig
	httpClient *http.Client
	baseURL    string
	headers    map[string]string
}

// Chat sends a chat completion request
func (c *OpenAIClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	url := c.baseURL + "/chat/completions"
	
	// Build request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, NewParsingError("failed to marshal request", err)
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, NewLLMErrorWithCause(ErrorTypeInvalidRequest, "failed to create HTTP request", err)
	}
	
	// Add headers
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}
	
	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewNetworkError("failed to send request", err)
	}
	defer resp.Body.Close()
	
	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewNetworkError("failed to read response", err)
	}
	
	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrorResponse(resp.StatusCode, respBody)
	}
	
	// Parse successful response
	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, NewParsingError("failed to parse response", err)
	}
	
	return &chatResp, nil
}

// StreamChat sends a streaming chat completion request
func (c *OpenAIClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error) {
	url := c.baseURL + "/chat/completions"
	
	// Enable streaming
	req.Stream = true
	
	// Build request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, NewParsingError("failed to marshal request", err)
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, NewLLMErrorWithCause(ErrorTypeInvalidRequest, "failed to create HTTP request", err)
	}
	
	// Add headers
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	
	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewNetworkError("failed to send request", err)
	}
	
	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, c.parseErrorResponse(resp.StatusCode, respBody)
	}
	
	// Create event channel
	eventChan := make(chan *StreamEvent)
	
	go c.handleStreamResponse(ctx, resp.Body, eventChan)
	
	return eventChan, nil
}

// handleStreamResponse processes the streaming response
func (c *OpenAIClient) handleStreamResponse(ctx context.Context, body io.ReadCloser, eventChan chan<- *StreamEvent) {
	defer close(eventChan)
	defer body.Close()
	
	llmLogger.Debug("Starting to read LLM stream response")
	scanner := bufio.NewScanner(body)
	var position int64
	lineCount := 0
	
	for scanner.Scan() {
		line := scanner.Text()
		position += int64(len(line))
		lineCount++
		
		llmLogger.WithFields(logrus.Fields{
			"line_number": lineCount,
			"line_content": line,
			"position": position,
		}).Debug("Stream line received")
		
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			llmLogger.WithField("line_number", lineCount).Debug("Skipping empty line")
			continue
		}
		
		// Parse SSE event
		event := c.parseSSELine(line, position)
		llmLogger.WithField("event_type", event.Type).Debug("Parsed SSE event")
		
		select {
		case eventChan <- event:
			llmLogger.WithField("event_type", event.Type).Debug("Sent event to channel")
		case <-ctx.Done():
			llmLogger.Debug("Context cancelled, stopping stream")
			return
		}
		
		// Check if stream is complete
		if event.Type == "done" {
			llmLogger.Debug("Stream completed with 'done' event")
			return
		}
	}
	
	llmLogger.WithField("total_lines", lineCount).Debug("Stream scanner finished")
	
	if err := scanner.Err(); err != nil {
		select {
		case eventChan <- &StreamEvent{
			Type:      "error",
			Error:     NewStreamError("failed to read stream", position, "", err),
			Timestamp: time.Now(),
		}:
		case <-ctx.Done():
		}
	}
}

// parseSSELine parses a Server-Sent Events line
func (c *OpenAIClient) parseSSELine(line string, position int64) *StreamEvent {
	event := &StreamEvent{
		Timestamp: time.Now(),
		Raw:       line,
	}
	
	// Handle "data: " prefix
	if strings.HasPrefix(line, "data: ") {
		dataStr := line[6:] // Remove "data: " prefix
		llmLogger.WithField("sse_data", dataStr).Debug("Found SSE data")
		
		// Check for stream end
		if dataStr == "[DONE]" {
			llmLogger.Debug("Found stream end marker [DONE]")
			event.Type = "done"
			return event
		}
		
		// Parse JSON data
		var streamResp StreamResponse
		if err := json.Unmarshal([]byte(dataStr), &streamResp); err != nil {
			llmLogger.WithFields(logrus.Fields{
				"error": err.Error(),
				"json_data": dataStr,
			}).Error("Failed to parse SSE JSON data")
			event.Type = "error"
			event.Error = NewStreamError("failed to parse stream data", position, line, err)
			return event
		}
		
		llmLogger.Debug("Successfully parsed JSON stream response")
		event.Type = "message"
		event.Data = &streamResp
		return event
	}
	
	// Handle other SSE fields (event:, id:, retry:)
	if strings.HasPrefix(line, "event: ") {
		eventType := line[7:]
		llmLogger.WithField("event_type", eventType).Debug("Found SSE event header")
		event.Type = eventType
		return event
	}
	
	// Unknown line format
	llmLogger.WithField("line", line).Debug("Unknown SSE line format")
	event.Type = "unknown"
	return event
}

// ListModels lists available models
func (c *OpenAIClient) ListModels(ctx context.Context) (*ModelsResponse, error) {
	url := c.baseURL + "/models"
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, NewLLMErrorWithCause(ErrorTypeInvalidRequest, "failed to create HTTP request", err)
	}
	
	// Add headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	
	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError("failed to send request", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewNetworkError("failed to read response", err)
	}
	
	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrorResponse(resp.StatusCode, body)
	}
	
	// Parse response
	var modelsResp ModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, NewParsingError("failed to parse models response", err)
	}
	
	return &modelsResp, nil
}

// GetModel gets information about a specific model
func (c *OpenAIClient) GetModel(ctx context.Context, modelID string) (*ModelInfo, error) {
	url := c.baseURL + "/models/" + modelID
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, NewLLMErrorWithCause(ErrorTypeInvalidRequest, "failed to create HTTP request", err)
	}
	
	// Add headers
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}
	
	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError("failed to send request", err)
	}
	defer resp.Body.Close()
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewNetworkError("failed to read response", err)
	}
	
	// Handle error responses
	if resp.StatusCode == http.StatusNotFound {
		return nil, NewModelNotFoundError(modelID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseErrorResponse(resp.StatusCode, body)
	}
	
	// Parse response
	var modelInfo ModelInfo
	if err := json.Unmarshal(body, &modelInfo); err != nil {
		return nil, NewParsingError("failed to parse model response", err)
	}
	
	return &modelInfo, nil
}

// Close closes the client and cleans up resources
func (c *OpenAIClient) Close() error {
	// Close HTTP client if needed
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

// Provider returns the provider name
func (c *OpenAIClient) Provider() string {
	return "openai"
}

// Config returns the client configuration
func (c *OpenAIClient) Config() *config.LLMConfig {
	return c.config
}

// parseErrorResponse parses an error response from the API
func (c *OpenAIClient) parseErrorResponse(statusCode int, body []byte) error {
	// Try to parse as structured error response
	var errorResp ErrorResponse
	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
		return c.mapAPIError(statusCode, &errorResp.Error)
	}
	
	// Fallback to generic error
	return NewServerError(fmt.Sprintf("API request failed with status %d: %s", 
		statusCode, string(body)), statusCode)
}

// mapAPIError maps OpenAI API errors to internal error types
func (c *OpenAIClient) mapAPIError(statusCode int, apiError *APIError) error {
	var errType ErrorType
	
	switch statusCode {
	case http.StatusUnauthorized:
		errType = ErrorTypeAuthentication
	case http.StatusTooManyRequests:
		errType = ErrorTypeRateLimit
	case http.StatusBadRequest:
		errType = ErrorTypeInvalidRequest
	case http.StatusNotFound:
		if strings.Contains(apiError.Message, "model") {
			errType = ErrorTypeModelNotFound
		} else {
			errType = ErrorTypeInvalidRequest
		}
	case http.StatusRequestEntityTooLarge:
		errType = ErrorTypeTokenLimit
	default:
		if statusCode >= 500 {
			errType = ErrorTypeServerError
		} else {
			errType = ErrorTypeUnknown
		}
	}
	
	err := NewLLMError(errType, apiError.Message)
	err.Provider = "openai"
	err.WithContext("api_error_type", apiError.Type)
	err.WithContext("api_error_code", apiError.Code)
	err.WithContext("http_status", statusCode)
	
	return err
}

// AnthropicProvider implements the Provider interface for Anthropic API
type AnthropicProvider struct{}

// Name returns the human-readable name
func (p *AnthropicProvider) Name() string {
	return "Anthropic"
}

// ID returns the unique identifier
func (p *AnthropicProvider) ID() string {
	return "anthropic"
}

// CreateClient creates a new Anthropic client
func (p *AnthropicProvider) CreateClient(config *config.LLMConfig) (Client, error) {
	client := &AnthropicClient{
		config:     config,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		baseURL:    strings.TrimSuffix(config.Endpoint, "/"),
		headers: map[string]string{
			"Content-Type":      "application/json",
			"User-Agent":        "gocodenow/1.0",
			"anthropic-version": "2023-06-01",
		},
	}
	
	// Add API key header if token is provided
	if config.Token != "" {
		client.headers["x-api-key"] = config.Token
	}
	
	return client, nil
}

// ValidateConfig validates Anthropic-specific configuration
func (p *AnthropicProvider) ValidateConfig(config *config.LLMConfig) error {
	if config.Endpoint == "" {
		return NewValidationError("endpoint is required for Anthropic provider")
	}
	
	if config.Model == "" {
		return NewValidationError("model is required for Anthropic provider")
	}
	
	if config.Token == "" {
		return NewValidationError("API key is required for Anthropic provider")
	}
	
	return nil
}

// SupportedModels returns Anthropic supported models
func (p *AnthropicProvider) SupportedModels() []string {
	return []string{
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
		"claude-2.1",
		"claude-2.0",
		"claude-instant-1.2",
	}
}

// DetectFromEndpoint detects if endpoint is Anthropic
func (p *AnthropicProvider) DetectFromEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "anthropic.com") ||
		   strings.Contains(endpoint, "api.anthropic.com")
}

// GetDefaultModel returns the default model
func (p *AnthropicProvider) GetDefaultModel() string {
	return "claude-3-sonnet-20240229"
}

// AnthropicClient implements the Client interface for Anthropic
type AnthropicClient struct {
	config     *config.LLMConfig
	httpClient *http.Client
	baseURL    string
	headers    map[string]string
}

// Chat sends a chat completion request
func (c *AnthropicClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	// Convert OpenAI format to Anthropic format
	anthropicReq := c.convertChatRequest(req)
	
	// Prepare request
	requestBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, NewLLMError(ErrorTypeParsing, "failed to marshal request").WithContext("error", err.Error())
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(requestBody))
	if err != nil {
		return nil, NewNetworkError("failed to create request", err)
	}
	
	// Add headers
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}
	
	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewNetworkError("request failed", err)
	}
	defer resp.Body.Close()
	
	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewNetworkError("failed to read response", err)
	}
	
	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, c.handleAnthropicError(resp.StatusCode, body)
	}
	
	// Parse Anthropic response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, NewParsingError("failed to unmarshal response", err).WithContext("body", string(body))
	}
	
	// Convert to OpenAI format
	return c.convertAnthropicResponse(&anthropicResp), nil
}

// StreamChat sends a streaming chat completion request
func (c *AnthropicClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan *StreamEvent, error) {
	// Convert OpenAI format to Anthropic format with streaming enabled
	anthropicReq := c.convertChatRequest(req)
	anthropicReq.Stream = true
	
	// Prepare request
	requestBody, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, NewLLMError(ErrorTypeParsing, "failed to marshal request").WithContext("error", err.Error())
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(requestBody))
	if err != nil {
		return nil, NewNetworkError("failed to create request", err)
	}
	
	// Add headers
	for key, value := range c.headers {
		httpReq.Header.Set(key, value)
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")
	
	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, NewNetworkError("request failed", err)
	}
	
	// Handle non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, c.handleAnthropicError(resp.StatusCode, body)
	}
	
	// Create event channel
	eventChan := make(chan *StreamEvent, 10)
	
	// Start streaming goroutine
	go c.handleAnthropicStream(ctx, resp.Body, eventChan)
	
	return eventChan, nil
}

// ListModels lists available models
func (c *AnthropicClient) ListModels(ctx context.Context) (*ModelsResponse, error) {
	// Anthropic doesn't have a models endpoint, return supported models
	provider := &AnthropicProvider{}
	models := provider.SupportedModels()
	
	var modelData []ModelInfo
	for _, modelID := range models {
		modelData = append(modelData, ModelInfo{
			ID:      modelID,
			Object:  "model",
			Created: time.Now().Unix(),
			OwnedBy: "anthropic",
		})
	}
	
	return &ModelsResponse{
		Object: "list",
		Data:   modelData,
	}, nil
}

// GetModel gets information about a specific model
func (c *AnthropicClient) GetModel(ctx context.Context, modelID string) (*ModelInfo, error) {
	// Anthropic doesn't have a specific model info endpoint
	provider := &AnthropicProvider{}
	supportedModels := provider.SupportedModels()
	
	// Check if model is supported
	for _, model := range supportedModels {
		if model == modelID {
			return &ModelInfo{
				ID:      modelID,
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "anthropic",
			}, nil
		}
	}
	
	return nil, NewModelNotFoundError(modelID)
}

// Close closes the client and cleans up resources
func (c *AnthropicClient) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
	return nil
}

// Provider returns the provider name
func (c *AnthropicClient) Provider() string {
	return "anthropic"
}

// Config returns the client configuration
func (c *AnthropicClient) Config() *config.LLMConfig {
	return c.config
}

// convertChatRequest converts OpenAI ChatRequest to Anthropic format
func (c *AnthropicClient) convertChatRequest(req *ChatRequest) *AnthropicRequest {
	anthropicReq := &AnthropicRequest{
		Model:      req.Model,
		MaxTokens:  1000, // Default value
		Messages:   []AnthropicMessage{},
		Tools:      []AnthropicTool{},
		Stream:     req.Stream,
	}
	
	// Set max tokens if specified
	if req.MaxTokens != nil {
		anthropicReq.MaxTokens = *req.MaxTokens
	}
	
	// Set temperature if specified
	if req.Temperature != nil {
		anthropicReq.Temperature = req.Temperature
	}
	
	// Convert messages
	var systemContent string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Anthropic handles system messages differently
			systemContent = msg.Content
		} else {
			anthropicMsg := AnthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}
			anthropicReq.Messages = append(anthropicReq.Messages, anthropicMsg)
		}
	}
	
	// Set system content if present
	if systemContent != "" {
		anthropicReq.System = systemContent
	}
	
	// Convert tools if present
	if len(req.Tools) > 0 {
		for _, tool := range req.Tools {
			anthropicTool := AnthropicTool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: tool.Function.Parameters,
			}
			anthropicReq.Tools = append(anthropicReq.Tools, anthropicTool)
		}
	}
	
	return anthropicReq
}

// convertAnthropicResponse converts Anthropic response to OpenAI format
func (c *AnthropicClient) convertAnthropicResponse(resp *AnthropicResponse) *ChatResponse {
	openaiResp := &ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: []ChatChoice{},
		Usage: TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
	
	// Convert content to OpenAI format
	var content strings.Builder
	for _, contentBlock := range resp.Content {
		if contentBlock.Type == "text" {
			content.WriteString(contentBlock.Text)
		}
	}
	
	// Create choice
	choice := ChatChoice{
		Index: 0,
		Message: ChatMessage{
			Role:    "assistant",
			Content: content.String(),
		},
		FinishReason: resp.StopReason,
	}
	
	// Handle tool calls if present
	if len(resp.Content) > 0 {
		for _, contentBlock := range resp.Content {
			if contentBlock.Type == "tool_use" {
				toolCall := ToolCall{
					ID:   contentBlock.ID,
					Type: "function",
					Function: ToolCallFunction{
						Name:      contentBlock.Name,
						Arguments: string(contentBlock.Input),
					},
				}
				choice.Message.ToolCalls = append(choice.Message.ToolCalls, toolCall)
			}
		}
	}
	
	openaiResp.Choices = append(openaiResp.Choices, choice)
	
	return openaiResp
}

// handleAnthropicError handles Anthropic API errors
func (c *AnthropicClient) handleAnthropicError(statusCode int, body []byte) error {
	// Try to parse Anthropic error format
	var anthropicError struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	
	if err := json.Unmarshal(body, &anthropicError); err != nil {
		// If parsing fails, return generic error
		return NewServerError(fmt.Sprintf("HTTP %d: %s", statusCode, string(body)), statusCode)
	}
	
	// Map status codes to error types
	var errType ErrorType
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		errType = ErrorTypeAuthentication
	case http.StatusTooManyRequests:
		errType = ErrorTypeRateLimit
	case http.StatusBadRequest:
		errType = ErrorTypeInvalidRequest
	case http.StatusNotFound:
		errType = ErrorTypeModelNotFound
	case http.StatusRequestEntityTooLarge:
		errType = ErrorTypeTokenLimit
	default:
		if statusCode >= 500 {
			errType = ErrorTypeServerError
		} else {
			errType = ErrorTypeUnknown
		}
	}
	
	err := NewLLMError(errType, anthropicError.Message)
	err.Provider = "anthropic"
	err.WithContext("anthropic_error_type", anthropicError.Type)
	err.WithContext("http_status", statusCode)
	
	return err
}

// handleAnthropicStream handles streaming responses from Anthropic
func (c *AnthropicClient) handleAnthropicStream(ctx context.Context, body io.ReadCloser, eventChan chan<- *StreamEvent) {
	defer close(eventChan)
	defer body.Close()
	
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()
		
		// Check for context cancellation
		select {
		case <-ctx.Done():
			eventChan <- &StreamEvent{Type: "error", Error: ctx.Err(), Timestamp: time.Now()}
			return
		default:
		}
		
		// Parse SSE line
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			
			// Handle end of stream
			if data == "[DONE]" {
				eventChan <- &StreamEvent{Type: "done", Timestamp: time.Now()}
				return
			}
			
			// Parse JSON event
			var anthropicStreamEvent struct {
				Type string      `json:"type"`
				Data interface{} `json:"data,omitempty"`
			}
			
			if err := json.Unmarshal([]byte(data), &anthropicStreamEvent); err != nil {
				eventChan <- &StreamEvent{
					Type:      "error",
					Error:     NewParsingError("failed to parse stream event", err),
					Raw:       data,
					Timestamp: time.Now(),
				}
				continue
			}
			
			// Send event
			eventChan <- &StreamEvent{
				Type:      anthropicStreamEvent.Type,
				Data:      anthropicStreamEvent.Data,
				Raw:       data,
				Timestamp: time.Now(),
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		eventChan <- &StreamEvent{
			Type:      "error",
			Error:     NewNetworkError("stream reading failed", err),
			Timestamp: time.Now(),
		}
	}
}

// LocalProvider implements the Provider interface for local/generic OpenAI-compatible APIs
type LocalProvider struct{}

// Name returns the human-readable name
func (p *LocalProvider) Name() string {
	return "Local/Generic"
}

// ID returns the unique identifier
func (p *LocalProvider) ID() string {
	return "local"
}

// CreateClient creates a new local client (reuses OpenAI client)
func (p *LocalProvider) CreateClient(config *config.LLMConfig) (Client, error) {
	// Local providers use the same API format as OpenAI
	client := &OpenAIClient{
		config:     config,
		httpClient: &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		baseURL:    strings.TrimSuffix(config.Endpoint, "/"),
		headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "gocodenow/1.0",
		},
	}
	
	// Add authorization header if token is provided (optional for local)
	if config.Token != "" {
		client.headers["Authorization"] = "Bearer " + config.Token
	}
	
	// Ensure base URL ends with /v1 for OpenAI compatibility
	if !strings.HasSuffix(client.baseURL, "/v1") {
		client.baseURL += "/v1"
	}
	
	return client, nil
}

// ValidateConfig validates local provider configuration
func (p *LocalProvider) ValidateConfig(config *config.LLMConfig) error {
	if config.Endpoint == "" {
		return NewValidationError("endpoint is required for local provider")
	}
	
	if config.Model == "" {
		return NewValidationError("model is required for local provider")
	}
	
	// Token is optional for local providers
	
	return nil
}

// SupportedModels returns generic model names (since local providers vary)
func (p *LocalProvider) SupportedModels() []string {
	return []string{
		"default",
		"local-model",
	}
}

// DetectFromEndpoint detects if endpoint might be a local provider
func (p *LocalProvider) DetectFromEndpoint(endpoint string) bool {
	// Local provider is the fallback, so always return true
	// but this is typically used when no other provider matches
	return true
}

// GetDefaultModel returns the default model
func (p *LocalProvider) GetDefaultModel() string {
	return "default"
}