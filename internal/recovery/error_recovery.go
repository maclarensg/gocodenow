package recovery

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

// ErrorCategory represents different categories of errors
type ErrorCategory int

const (
	// CriticalError represents errors that require immediate attention
	CriticalError ErrorCategory = iota
	// RecoverableErrorCategory represents errors that can be recovered from
	RecoverableErrorCategory
	// TransientError represents temporary errors that may resolve themselves
	TransientError
	// UserError represents errors caused by user input
	UserError
	// SystemError represents system-level errors
	SystemError
)

// String returns the string representation of the error category
func (ec ErrorCategory) String() string {
	switch ec {
	case CriticalError:
		return "CRITICAL"
	case RecoverableErrorCategory:
		return "RECOVERABLE"
	case TransientError:
		return "TRANSIENT"
	case UserError:
		return "USER"
	case SystemError:
		return "SYSTEM"
	default:
		return "UNKNOWN"
	}
}

// RecoveryStrategy represents different recovery strategies
type RecoveryStrategy int

const (
	// RetryStrategy attempts to retry the operation
	RetryStrategy RecoveryStrategy = iota
	// FallbackStrategy uses an alternative approach
	FallbackStrategy
	// GracefulDegradationStrategy continues with reduced functionality
	GracefulDegradationStrategy
	// RestartComponentStrategy restarts the affected component
	RestartComponentStrategy
	// UserInterventionStrategy requires user intervention
	UserInterventionStrategy
)

// RecoveryAction represents an action that can be taken for recovery
type RecoveryAction struct {
	Name        string
	Description string
	Strategy    RecoveryStrategy
	Handler     func(ctx context.Context, err *RecoverableError) error
	MaxRetries  int
	RetryDelay  time.Duration
	Timeout     time.Duration
}

// RecoverableError represents an error that includes recovery information
type RecoverableError struct {
	OriginalError    error
	Category         ErrorCategory
	Component        string
	Operation        string
	Timestamp        time.Time
	StackTrace       string
	RecoveryActions  []RecoveryAction
	AttemptedActions []string
	Context          map[string]interface{}
	Severity         int // 1-10, 10 being most severe
	UserMessage      string
	SuggestedFixes   []string
}

// Error implements the error interface
func (re *RecoverableError) Error() string {
	return fmt.Sprintf("[%s] %s in %s: %s", 
		re.categoryString(), re.Operation, re.Component, re.OriginalError.Error())
}

// categoryString returns the string representation of the error category
func (re *RecoverableError) categoryString() string {
	switch re.Category {
	case CriticalError:
		return "CRITICAL"
	case RecoverableErrorCategory:
		return "RECOVERABLE"
	case TransientError:
		return "TRANSIENT"
	case UserError:
		return "USER"
	case SystemError:
		return "SYSTEM"
	default:
		return "UNKNOWN"
	}
}

// ErrorRecoveryManager manages error recovery across the application
type ErrorRecoveryManager struct {
	recoveryActions map[string][]RecoveryAction
	errorHistory    []RecoverableError
	maxHistory      int
	mutex           sync.RWMutex
	metrics         *RecoveryMetrics
	callbacks       []ErrorCallback
}

// ErrorCallback is a function called when errors occur
type ErrorCallback func(err *RecoverableError) error

// RecoveryMetrics tracks recovery statistics
type RecoveryMetrics struct {
	TotalErrors         int64
	RecoveredErrors     int64
	FailedRecoveries    int64
	RecoveryAttempts    int64
	AverageRecoveryTime time.Duration
	ErrorsByCategory    map[ErrorCategory]int64
	ErrorsByComponent   map[string]int64
	mutex               sync.RWMutex
}

// NewErrorRecoveryManager creates a new error recovery manager
func NewErrorRecoveryManager(maxHistory int) *ErrorRecoveryManager {
	return &ErrorRecoveryManager{
		recoveryActions: make(map[string][]RecoveryAction),
		errorHistory:    make([]RecoverableError, 0, maxHistory),
		maxHistory:      maxHistory,
		metrics: &RecoveryMetrics{
			ErrorsByCategory:  make(map[ErrorCategory]int64),
			ErrorsByComponent: make(map[string]int64),
		},
		callbacks: make([]ErrorCallback, 0),
	}
}

// RegisterRecoveryAction registers a recovery action for a specific component/operation
func (erm *ErrorRecoveryManager) RegisterRecoveryAction(component, operation string, action RecoveryAction) {
	erm.mutex.Lock()
	defer erm.mutex.Unlock()
	
	key := fmt.Sprintf("%s:%s", component, operation)
	erm.recoveryActions[key] = append(erm.recoveryActions[key], action)
}

// RegisterErrorCallback registers a callback for error notifications
func (erm *ErrorRecoveryManager) RegisterErrorCallback(callback ErrorCallback) {
	erm.mutex.Lock()
	defer erm.mutex.Unlock()
	
	erm.callbacks = append(erm.callbacks, callback)
}

// HandleError handles an error and attempts recovery
func (erm *ErrorRecoveryManager) HandleError(ctx context.Context, originalErr error, component, operation string) error {
	// Create recoverable error
	recoverableErr := &RecoverableError{
		OriginalError: originalErr,
		Component:     component,
		Operation:     operation,
		Timestamp:     time.Now(),
		StackTrace:    string(debug.Stack()),
		Context:       make(map[string]interface{}),
	}
	
	// Categorize the error
	erm.categorizeError(recoverableErr)
	
	// Find recovery actions
	erm.findRecoveryActions(recoverableErr)
	
	// Update metrics
	erm.updateMetrics(recoverableErr)
	
	// Add to history
	erm.addToHistory(*recoverableErr)
	
	// Notify callbacks
	erm.notifyCallbacks(recoverableErr)
	
	// Attempt recovery
	return erm.attemptRecovery(ctx, recoverableErr)
}

// categorizeError categorizes the error based on its type and context
func (erm *ErrorRecoveryManager) categorizeError(err *RecoverableError) {
	originalErr := err.OriginalError
	
	// Check for specific error types
	switch {
	case isNetworkError(originalErr):
		err.Category = TransientError
		err.Severity = 5
		err.UserMessage = "Network connection issue detected"
		err.SuggestedFixes = []string{
			"Check your internet connection",
			"Verify the server endpoint is correct",
			"Try again in a few moments",
		}
	case isDatabaseError(originalErr):
		err.Category = RecoverableErrorCategory
		err.Severity = 7
		err.UserMessage = "Database operation failed"
		err.SuggestedFixes = []string{
			"Check database connection",
			"Verify database file permissions",
			"Try restarting the application",
		}
	case isFileSystemError(originalErr):
		err.Category = RecoverableErrorCategory
		err.Severity = 6
		err.UserMessage = "File system operation failed"
		err.SuggestedFixes = []string{
			"Check file permissions",
			"Ensure sufficient disk space",
			"Verify the file path exists",
		}
	case isConfigurationError(originalErr):
		err.Category = UserError
		err.Severity = 4
		err.UserMessage = "Configuration error detected"
		err.SuggestedFixes = []string{
			"Check your configuration file syntax",
			"Verify all required fields are provided",
			"Reset to default configuration if needed",
		}
	case isOutOfMemoryError(originalErr):
		err.Category = SystemError
		err.Severity = 9
		err.UserMessage = "System running low on memory"
		err.SuggestedFixes = []string{
			"Close unnecessary applications",
			"Reduce cache size in configuration",
			"Restart the application",
		}
	default:
		err.Category = RecoverableErrorCategory
		err.Severity = 5
		err.UserMessage = "An unexpected error occurred"
		err.SuggestedFixes = []string{
			"Try the operation again",
			"Check the application logs",
			"Contact support if the issue persists",
		}
	}
}

// findRecoveryActions finds appropriate recovery actions for the error
func (erm *ErrorRecoveryManager) findRecoveryActions(err *RecoverableError) {
	erm.mutex.RLock()
	defer erm.mutex.RUnlock()
	
	key := fmt.Sprintf("%s:%s", err.Component, err.Operation)
	if actions, exists := erm.recoveryActions[key]; exists {
		err.RecoveryActions = actions
		return
	}
	
	// Fall back to component-level actions
	componentKey := fmt.Sprintf("%s:*", err.Component)
	if actions, exists := erm.recoveryActions[componentKey]; exists {
		err.RecoveryActions = actions
		return
	}
	
	// Use default recovery actions based on category
	err.RecoveryActions = erm.getDefaultRecoveryActions(err.Category)
}

// getDefaultRecoveryActions returns default recovery actions for each category
func (erm *ErrorRecoveryManager) getDefaultRecoveryActions(category ErrorCategory) []RecoveryAction {
	switch category {
	case TransientError:
		return []RecoveryAction{
			{
				Name:        "Retry with backoff",
				Description: "Retry the operation with exponential backoff",
				Strategy:    RetryStrategy,
				MaxRetries:  3,
				RetryDelay:  time.Second,
				Timeout:     30 * time.Second,
				Handler:     erm.defaultRetryHandler,
			},
		}
	case RecoverableErrorCategory:
		return []RecoveryAction{
			{
				Name:        "Graceful retry",
				Description: "Attempt operation with error handling",
				Strategy:    RetryStrategy,
				MaxRetries:  2,
				RetryDelay:  2 * time.Second,
				Timeout:     60 * time.Second,
				Handler:     erm.defaultRetryHandler,
			},
			{
				Name:        "Fallback operation",
				Description: "Use alternative approach",
				Strategy:    FallbackStrategy,
				MaxRetries:  1,
				Timeout:     30 * time.Second,
				Handler:     erm.defaultFallbackHandler,
			},
		}
	case UserError:
		return []RecoveryAction{
			{
				Name:        "User intervention",
				Description: "Request user to fix the issue",
				Strategy:    UserInterventionStrategy,
				MaxRetries:  1,
				Handler:     erm.defaultUserInterventionHandler,
			},
		}
	case SystemError:
		return []RecoveryAction{
			{
				Name:        "Component restart",
				Description: "Restart the affected component",
				Strategy:    RestartComponentStrategy,
				MaxRetries:  1,
				Timeout:     60 * time.Second,
				Handler:     erm.defaultRestartHandler,
			},
		}
	default:
		return []RecoveryAction{}
	}
}

// attemptRecovery attempts to recover from the error using available actions
func (erm *ErrorRecoveryManager) attemptRecovery(ctx context.Context, err *RecoverableError) error {
	startTime := time.Now()
	
	for _, action := range err.RecoveryActions {
		log.Printf("Attempting recovery action: %s for %s", action.Name, err.Component)
		
		// Create context with timeout
		actionCtx, cancel := context.WithTimeout(ctx, action.Timeout)
		
		// Attempt recovery with retries
		var lastErr error
		for attempt := 0; attempt <= action.MaxRetries; attempt++ {
			if attempt > 0 {
				// Wait before retry
				select {
				case <-time.After(action.RetryDelay * time.Duration(attempt)):
				case <-actionCtx.Done():
					cancel()
					return actionCtx.Err()
				}
			}
			
			erm.metrics.mutex.Lock()
			erm.metrics.RecoveryAttempts++
			erm.metrics.mutex.Unlock()
			
			lastErr = action.Handler(actionCtx, err)
			if lastErr == nil {
				// Recovery successful
				cancel()
				
				// Update metrics
				recoveryTime := time.Since(startTime)
				erm.metrics.mutex.Lock()
				erm.metrics.RecoveredErrors++
				erm.updateAverageRecoveryTime(recoveryTime)
				erm.metrics.mutex.Unlock()
				
				log.Printf("Recovery successful for %s using %s", err.Component, action.Name)
				err.AttemptedActions = append(err.AttemptedActions, 
					fmt.Sprintf("%s (successful)", action.Name))
				return nil
			}
			
			log.Printf("Recovery attempt %d failed for %s: %v", attempt+1, action.Name, lastErr)
		}
		
		cancel()
		err.AttemptedActions = append(err.AttemptedActions, 
			fmt.Sprintf("%s (failed after %d attempts)", action.Name, action.MaxRetries+1))
	}
	
	// All recovery actions failed
	erm.metrics.mutex.Lock()
	erm.metrics.FailedRecoveries++
	erm.metrics.mutex.Unlock()
	
	return fmt.Errorf("all recovery actions failed for %s: %w", err.Component, err.OriginalError)
}

// Default recovery handlers

func (erm *ErrorRecoveryManager) defaultRetryHandler(ctx context.Context, err *RecoverableError) error {
	// This would be implemented by the specific component
	// For now, return an error indicating retry is needed
	return fmt.Errorf("retry handler not implemented for %s", err.Component)
}

func (erm *ErrorRecoveryManager) defaultFallbackHandler(ctx context.Context, err *RecoverableError) error {
	// This would implement fallback logic
	return fmt.Errorf("fallback handler not implemented for %s", err.Component)
}

func (erm *ErrorRecoveryManager) defaultUserInterventionHandler(ctx context.Context, err *RecoverableError) error {
	// This would trigger user notification
	return fmt.Errorf("user intervention required for %s", err.Component)
}

func (erm *ErrorRecoveryManager) defaultRestartHandler(ctx context.Context, err *RecoverableError) error {
	// This would restart the component
	return fmt.Errorf("component restart not implemented for %s", err.Component)
}

// Helper functions for error categorization

func isNetworkError(err error) bool {
	errStr := err.Error()
	networkKeywords := []string{
		"connection refused", "network", "timeout", "dns", "host", 
		"unreachable", "no route", "connection reset",
	}
	
	for _, keyword := range networkKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	return false
}

func isDatabaseError(err error) bool {
	errStr := err.Error()
	dbKeywords := []string{
		"database", "sql", "sqlite", "constraint", "table", 
		"column", "syntax error", "locked",
	}
	
	for _, keyword := range dbKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	return false
}

func isFileSystemError(err error) bool {
	errStr := err.Error()
	fsKeywords := []string{
		"no such file", "permission denied", "file exists", 
		"not a directory", "directory not empty", "disk", "space",
	}
	
	for _, keyword := range fsKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	return false
}

func isConfigurationError(err error) bool {
	errStr := err.Error()
	configKeywords := []string{
		"configuration", "config", "invalid", "missing", 
		"yaml", "json", "parse", "unmarshal",
	}
	
	for _, keyword := range configKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	return false
}

func isOutOfMemoryError(err error) bool {
	errStr := err.Error()
	memoryKeywords := []string{
		"out of memory", "cannot allocate", "memory", "oom",
	}
	
	for _, keyword := range memoryKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    len(s) > len(substr) && 
		    (s[:len(substr)+1] == substr+" " || 
		     s[len(s)-len(substr)-1:] == " "+substr ||
		     fmt.Sprintf(" %s ", substr) != "" && 
		     fmt.Sprintf("%s ", substr) != "" && 
		     fmt.Sprintf(" %s", substr) != ""))
}

// Utility methods

func (erm *ErrorRecoveryManager) updateMetrics(err *RecoverableError) {
	erm.metrics.mutex.Lock()
	defer erm.metrics.mutex.Unlock()
	
	erm.metrics.TotalErrors++
	erm.metrics.ErrorsByCategory[err.Category]++
	erm.metrics.ErrorsByComponent[err.Component]++
}

func (erm *ErrorRecoveryManager) addToHistory(err RecoverableError) {
	erm.mutex.Lock()
	defer erm.mutex.Unlock()
	
	erm.errorHistory = append(erm.errorHistory, err)
	
	// Maintain history size limit
	if len(erm.errorHistory) > erm.maxHistory {
		erm.errorHistory = erm.errorHistory[1:]
	}
}

func (erm *ErrorRecoveryManager) notifyCallbacks(err *RecoverableError) {
	for _, callback := range erm.callbacks {
		go func(cb ErrorCallback) {
			if cbErr := cb(err); cbErr != nil {
				log.Printf("Error callback failed: %v", cbErr)
			}
		}(callback)
	}
}

func (erm *ErrorRecoveryManager) updateAverageRecoveryTime(recoveryTime time.Duration) {
	// Simple moving average calculation
	if erm.metrics.RecoveredErrors == 1 {
		erm.metrics.AverageRecoveryTime = recoveryTime
	} else {
		// Weighted average with more weight on recent times
		erm.metrics.AverageRecoveryTime = 
			(erm.metrics.AverageRecoveryTime*7 + recoveryTime*3) / 10
	}
}

// GetMetrics returns the current recovery metrics
func (erm *ErrorRecoveryManager) GetMetrics() RecoveryMetrics {
	erm.metrics.mutex.RLock()
	defer erm.metrics.mutex.RUnlock()
	
	// Create a copy to avoid race conditions
	return RecoveryMetrics{
		TotalErrors:         erm.metrics.TotalErrors,
		RecoveredErrors:     erm.metrics.RecoveredErrors,
		FailedRecoveries:    erm.metrics.FailedRecoveries,
		RecoveryAttempts:    erm.metrics.RecoveryAttempts,
		AverageRecoveryTime: erm.metrics.AverageRecoveryTime,
		ErrorsByCategory:    erm.copyIntMap(erm.metrics.ErrorsByCategory),
		ErrorsByComponent:   erm.copyStringMap(erm.metrics.ErrorsByComponent),
	}
}

func (erm *ErrorRecoveryManager) copyIntMap(src map[ErrorCategory]int64) map[ErrorCategory]int64 {
	dst := make(map[ErrorCategory]int64)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (erm *ErrorRecoveryManager) copyStringMap(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// GetErrorHistory returns the error history
func (erm *ErrorRecoveryManager) GetErrorHistory() []RecoverableError {
	erm.mutex.RLock()
	defer erm.mutex.RUnlock()
	
	// Return a copy to prevent external modifications
	history := make([]RecoverableError, len(erm.errorHistory))
	copy(history, erm.errorHistory)
	return history
}