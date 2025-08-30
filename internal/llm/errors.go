package llm

import (
	"fmt"
	"time"
)

// LLMError represents a general LLM-related error
type LLMError struct {
	// Type of error
	Type ErrorType `json:"type"`
	
	// Error message
	Message string `json:"message"`
	
	// Underlying cause
	Cause error `json:"cause,omitempty"`
	
	// Provider that generated the error
	Provider string `json:"provider,omitempty"`
	
	// Model being used when error occurred
	Model string `json:"model,omitempty"`
	
	// Whether this error is retryable
	Retryable bool `json:"retryable"`
	
	// Timestamp when error occurred
	Timestamp time.Time `json:"timestamp"`
	
	// Additional error context
	Context map[string]interface{} `json:"context,omitempty"`
}

// ErrorType represents different categories of LLM errors
type ErrorType int

const (
	// ErrorTypeUnknown represents an unknown error
	ErrorTypeUnknown ErrorType = iota
	
	// ErrorTypeNetwork represents network connectivity errors
	ErrorTypeNetwork
	
	// ErrorTypeAuthentication represents authentication/authorization errors
	ErrorTypeAuthentication
	
	// ErrorTypeRateLimit represents rate limiting errors
	ErrorTypeRateLimit
	
	// ErrorTypeInvalidRequest represents client request errors
	ErrorTypeInvalidRequest
	
	// ErrorTypeModelNotFound represents model not available errors
	ErrorTypeModelNotFound
	
	// ErrorTypeTokenLimit represents token limit exceeded errors
	ErrorTypeTokenLimit
	
	// ErrorTypeServerError represents server-side errors
	ErrorTypeServerError
	
	// ErrorTypeTimeout represents timeout errors
	ErrorTypeTimeout
	
	// ErrorTypeParsing represents response parsing errors
	ErrorTypeParsing
	
	// ErrorTypeToolCall represents tool call related errors
	ErrorTypeToolCall
	
	// ErrorTypeValidation represents validation errors
	ErrorTypeValidation
)

// String returns a string representation of the error type
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeUnknown:
		return "unknown"
	case ErrorTypeNetwork:
		return "network"
	case ErrorTypeAuthentication:
		return "authentication"
	case ErrorTypeRateLimit:
		return "rate_limit"
	case ErrorTypeInvalidRequest:
		return "invalid_request"
	case ErrorTypeModelNotFound:
		return "model_not_found"
	case ErrorTypeTokenLimit:
		return "token_limit"
	case ErrorTypeServerError:
		return "server_error"
	case ErrorTypeTimeout:
		return "timeout"
	case ErrorTypeParsing:
		return "parsing"
	case ErrorTypeToolCall:
		return "tool_call"
	case ErrorTypeValidation:
		return "validation"
	default:
		return "unknown"
	}
}

// Error implements the error interface
func (e *LLMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type.String(), e.Message)
}

// Unwrap returns the underlying cause error
func (e *LLMError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether this error is retryable
func (e *LLMError) IsRetryable() bool {
	return e.Retryable
}

// WithContext adds context to the error
func (e *LLMError) WithContext(key string, value interface{}) *LLMError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// NewLLMError creates a new LLM error
func NewLLMError(errType ErrorType, message string) *LLMError {
	return &LLMError{
		Type:      errType,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: isRetryableErrorType(errType),
	}
}

// NewLLMErrorWithCause creates a new LLM error with an underlying cause
func NewLLMErrorWithCause(errType ErrorType, message string, cause error) *LLMError {
	return &LLMError{
		Type:      errType,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryableErrorType(errType),
	}
}

// NewNetworkError creates a network-related error
func NewNetworkError(message string, cause error) *LLMError {
	return NewLLMErrorWithCause(ErrorTypeNetwork, message, cause)
}

// NewAuthenticationError creates an authentication error
func NewAuthenticationError(message string) *LLMError {
	return NewLLMError(ErrorTypeAuthentication, message)
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(message string, resetTime *time.Time) *LLMError {
	err := NewLLMError(ErrorTypeRateLimit, message)
	if resetTime != nil {
		err.WithContext("reset_time", *resetTime)
	}
	return err
}

// NewInvalidRequestError creates an invalid request error
func NewInvalidRequestError(message string) *LLMError {
	return &LLMError{
		Type:      ErrorTypeInvalidRequest,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: false, // Invalid requests are typically not retryable
	}
}

// NewModelNotFoundError creates a model not found error
func NewModelNotFoundError(model string) *LLMError {
	return NewLLMError(ErrorTypeModelNotFound, fmt.Sprintf("model not found: %s", model)).
		WithContext("model", model)
}

// NewTokenLimitError creates a token limit exceeded error
func NewTokenLimitError(message string, tokensUsed, tokenLimit int) *LLMError {
	return NewLLMError(ErrorTypeTokenLimit, message).
		WithContext("tokens_used", tokensUsed).
		WithContext("token_limit", tokenLimit)
}

// NewServerError creates a server error
func NewServerError(message string, statusCode int) *LLMError {
	return NewLLMError(ErrorTypeServerError, message).
		WithContext("status_code", statusCode)
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, timeout time.Duration) *LLMError {
	return NewLLMError(ErrorTypeTimeout, message).
		WithContext("timeout", timeout)
}

// NewParsingError creates a parsing error
func NewParsingError(message string, cause error) *LLMError {
	return &LLMError{
		Type:      ErrorTypeParsing,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: false, // Parsing errors are typically not retryable
	}
}

// NewToolCallError creates a tool call error
func NewToolCallError(message string, toolName string, cause error) *LLMError {
	return NewLLMErrorWithCause(ErrorTypeToolCall, message, cause).
		WithContext("tool_name", toolName)
}

// NewValidationError creates a validation error
func NewValidationError(message string) *LLMError {
	return &LLMError{
		Type:      ErrorTypeValidation,
		Message:   message,
		Timestamp: time.Now(),
		Retryable: false, // Validation errors are not retryable
	}
}

// isRetryableErrorType determines if an error type is generally retryable
func isRetryableErrorType(errType ErrorType) bool {
	switch errType {
	case ErrorTypeNetwork, ErrorTypeRateLimit, ErrorTypeServerError, ErrorTypeTimeout:
		return true
	case ErrorTypeAuthentication, ErrorTypeInvalidRequest, ErrorTypeModelNotFound, 
		 ErrorTypeTokenLimit, ErrorTypeParsing, ErrorTypeValidation:
		return false
	default:
		return false
	}
}

// ProviderError represents a provider-specific error
type ProviderError struct {
	*LLMError
	
	// Provider-specific error code
	ProviderCode string `json:"provider_code,omitempty"`
	
	// HTTP status code (if applicable)
	HTTPStatusCode int `json:"http_status_code,omitempty"`
	
	// Provider-specific error details
	ProviderDetails map[string]interface{} `json:"provider_details,omitempty"`
}

// NewProviderError creates a new provider-specific error
func NewProviderError(provider, message string, statusCode int) *ProviderError {
	return &ProviderError{
		LLMError: &LLMError{
			Type:      ErrorTypeServerError,
			Message:   message,
			Provider:  provider,
			Timestamp: time.Now(),
			Retryable: statusCode >= 500 && statusCode < 600, // 5xx errors are retryable
		},
		HTTPStatusCode: statusCode,
	}
}

// WithProviderCode adds a provider-specific error code
func (e *ProviderError) WithProviderCode(code string) *ProviderError {
	e.ProviderCode = code
	return e
}

// WithProviderDetails adds provider-specific details
func (e *ProviderError) WithProviderDetails(details map[string]interface{}) *ProviderError {
	e.ProviderDetails = details
	return e
}

// StreamError represents errors that occur during streaming
type StreamError struct {
	*LLMError
	
	// Stream position where error occurred
	StreamPosition int64 `json:"stream_position,omitempty"`
	
	// Raw event data that caused the error
	RawEvent string `json:"raw_event,omitempty"`
}

// NewStreamError creates a new streaming error
func NewStreamError(message string, position int64, rawEvent string, cause error) *StreamError {
	return &StreamError{
		LLMError: &LLMError{
			Type:      ErrorTypeParsing,
			Message:   message,
			Cause:     cause,
			Timestamp: time.Now(),
			Retryable: false, // Stream parsing errors are not retryable
		},
		StreamPosition: position,
		RawEvent:       rawEvent,
	}
}