package recovery

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Test ErrorCategory String method
func TestErrorCategory_String(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected string
	}{
		{CriticalError, "CRITICAL"},
		{RecoverableErrorCategory, "RECOVERABLE"},
		{TransientError, "TRANSIENT"},
		{UserError, "USER"},
		{SystemError, "SYSTEM"},
		{ErrorCategory(999), "UNKNOWN"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			if got := test.category.String(); got != test.expected {
				t.Errorf("ErrorCategory.String() = %v, want %v", got, test.expected)
			}
		})
	}
}

// Test RecoverableError creation and methods
func TestRecoverableError_Creation(t *testing.T) {
	originalErr := errors.New("test error")
	recErr := &RecoverableError{
		OriginalError: originalErr,
		Category:      CriticalError,
		Component:     "test-component",
		Operation:     "test-operation",
		Timestamp:     time.Now(),
		Severity:      8,
		UserMessage:   "Test error occurred",
	}

	// Test Error() method
	errorStr := recErr.Error()
	if errorStr == "" {
		t.Error("Error string should not be empty")
	}

	// Test that error contains expected components
	if !containsAll(errorStr, []string{"CRITICAL", "test-component", "test-operation"}) {
		t.Errorf("Error string missing components: %s", errorStr)
	}
}

// Test ErrorRecoveryManager basic functionality
func TestErrorRecoveryManager_Basic(t *testing.T) {
	manager := NewErrorRecoveryManager(100)
	
	if manager == nil {
		t.Error("Manager should not be nil")
	}
	
	// Test handling an error
	ctx := context.Background()
	originalErr := errors.New("test error")
	
	err := manager.HandleError(ctx, originalErr, "test-component", "test-operation")
	
	// The returned error should be non-nil (it may be the original or a wrapped error)
	if err == nil {
		t.Error("HandleError should return an error")
	}
	
	// Test that metrics are updated
	metrics := manager.GetMetrics()
	if metrics.TotalErrors == 0 {
		t.Error("Total errors should be greater than 0 after handling an error")
	}
}

// Test ErrorAnalytics basic functionality
func TestErrorAnalytics_Basic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "analytics_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	analytics := NewErrorAnalytics(tempDir)
	
	if analytics == nil {
		t.Error("Analytics should not be nil")
	}

	// Record a test error
	testErr := RecoverableError{
		OriginalError: errors.New("analytics test error"),
		Category:      CriticalError,
		Component:     "test-component",
		Operation:     "test-operation",
		Timestamp:     time.Now(),
		Severity:      8,
	}

	analytics.RecordError(testErr)

	// Generate a report
	ctx := context.Background()
	report, err := analytics.GenerateReport(ctx, time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate report: %v", err)
	}

	if report == nil {
		t.Error("Report should not be nil")
	}

	if report.TotalErrors == 0 {
		t.Error("Report should show at least one error")
	}

	// Test health summary
	summary := analytics.GetHealthSummary()
	if summary == nil {
		t.Error("Health summary should not be nil")
	}
}

// Test CrashRecoveryManager basic functionality
func TestCrashRecoveryManager_Basic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "crash_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewCrashRecoveryManager(tempDir)
	
	if manager == nil {
		t.Error("Crash recovery manager should not be nil")
	}

	// Test session lifecycle
	sessionID := "test-session-123"
	session, err := manager.StartSession(sessionID, "1.0.0")
	if err != nil {
		t.Fatalf("Failed to start session: %v", err)
	}

	if session == nil {
		t.Error("Session should not be nil")
	}

	if session.SessionID != sessionID {
		t.Errorf("Session ID = %v, want %v", session.SessionID, sessionID)
	}

	// Test getting current session
	currentSession := manager.GetCurrentSession()
	if currentSession == nil {
		t.Error("Current session should not be nil after starting")
	}

	if currentSession.SessionID != sessionID {
		t.Error("Current session should match started session")
	}

	// Test graceful shutdown
	err = manager.GracefulShutdown()
	if err != nil {
		t.Errorf("Graceful shutdown failed: %v", err)
	}
}

// Test RecoveryAction structure
func TestRecoveryAction_Structure(t *testing.T) {
	action := &RecoveryAction{
		Name:        "test-action",
		Description: "Test recovery action",
		Strategy:    RetryStrategy,
		MaxRetries:  3,
		RetryDelay:  time.Millisecond * 100,
		Timeout:     time.Second,
	}

	if action.Name != "test-action" {
		t.Error("Action name not set correctly")
	}

	if action.Strategy != RetryStrategy {
		t.Error("Action strategy not set correctly")
	}

	if action.MaxRetries != 3 {
		t.Error("Max retries not set correctly")
	}

	if action.Timeout != time.Second {
		t.Error("Timeout not set correctly")
	}
}

// Helper function to check if a string contains all expected substrings
func containsAll(str string, substrings []string) bool {
	for _, substr := range substrings {
		found := false
		for i := 0; i <= len(str)-len(substr); i++ {
			if str[i:i+len(substr)] == substr {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Benchmark tests
func BenchmarkRecoverableError_Error(b *testing.B) {
	recErr := &RecoverableError{
		OriginalError: errors.New("benchmark test error"),
		Category:      CriticalError,
		Component:     "benchmark-component",
		Operation:     "benchmark-operation",
		Timestamp:     time.Now(),
		Severity:      8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = recErr.Error()
	}
}

func BenchmarkErrorRecoveryManager_HandleError(b *testing.B) {
	manager := NewErrorRecoveryManager(1000)
	ctx := context.Background()
	err := errors.New("benchmark test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.HandleError(ctx, err, "benchmark-component", "benchmark-operation")
	}
}

func BenchmarkErrorAnalytics_RecordError(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "bench_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	analytics := NewErrorAnalytics(tempDir)
	
	recErr := RecoverableError{
		OriginalError: errors.New("benchmark test error"),
		Category:      TransientError,
		Component:     "benchmark-component",
		Operation:     "benchmark-operation",
		Timestamp:     time.Now(),
		Severity:      5,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analytics.RecordError(recErr)
	}
}