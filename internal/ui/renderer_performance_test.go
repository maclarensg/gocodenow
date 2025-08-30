package ui

import (
	"strings"
	"testing"
	"time"

	"gocodenow/internal/types"
)

func TestBuildPerformanceMetrics(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	conv := types.ConversationBlock{
		UserInput:     "Test message for performance metrics",
		Status:        types.StatusCompleted,
		ExecutionTime: 2 * time.Second,
		TokenUsage: types.TokenUsage{
			InputTokens:  100,
			OutputTokens: 200,
		},
		ToolCalls: []types.ToolCall{
			{
				ID:       "tool1",
				ToolName: "test_tool",
				Timestamp: time.Now(),
			},
		},
		ToolResults: []types.ToolResult{
			{
				ID:         "result1",
				ToolCallID: "tool1",
				Success:    true,
				Duration:   500 * time.Millisecond,
				Timestamp:  time.Now(),
			},
		},
		FileOperations: []types.FileOperation{
			{
				OperationType: "read",
				Success:       true,
			},
			{
				OperationType: "write",
				Success:       true,
			},
		},
	}
	
	result := renderer.buildPerformanceMetrics(conv, conv.UserInput)
	
	// Check that all expected sections are present
	expectedSections := []string{
		"Message:",
		"Status:",
		"Duration:",
		"Tokens:",
		"Efficiency:",
		"Tools:",
		"Files:",
	}
	
	for _, section := range expectedSections {
		if !strings.Contains(result, section) {
			t.Errorf("Expected performance metrics to contain '%s', but it was missing", section)
		}
	}
	
	// Verify specific content
	if !strings.Contains(result, "300 total") { // Total tokens
		t.Error("Expected total tokens to be displayed")
	}
	
	if !strings.Contains(result, "1 executed") { // Tool count
		t.Error("Expected tool execution count to be displayed")
	}
	
	if !strings.Contains(result, "2 ops") { // File operations
		t.Error("Expected file operations count to be displayed")
	}
}

func TestGetPerformanceIcon(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	testCases := []struct {
		duration time.Duration
		expected string
	}{
		{100 * time.Millisecond, "⚡"}, // Very fast
		{1 * time.Second, "🚀"},        // Fast
		{5 * time.Second, "⏱️"},         // Normal
		{20 * time.Second, "🐌"},       // Slow
		{60 * time.Second, "🐢"},       // Very slow
	}
	
	for _, tc := range testCases {
		result := renderer.getPerformanceIcon(tc.duration)
		if result != tc.expected {
			t.Errorf("For duration %v, expected %s, got %s", tc.duration, tc.expected, result)
		}
	}
}

func TestCalculateTokenEfficiency(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	testCases := []struct {
		name     string
		usage    types.TokenUsage
		duration time.Duration
		expected string
	}{
		{
			name: "high_efficiency",
			usage: types.TokenUsage{
				InputTokens:  500,
				OutputTokens: 1500,
			},
			duration: time.Second,
			expected: "2.0k tokens/sec 🚀",
		},
		{
			name: "medium_efficiency",
			usage: types.TokenUsage{
				InputTokens:  50,
				OutputTokens: 150,
			},
			duration: time.Second,
			expected: "200 tokens/sec ⚡",
		},
		{
			name: "low_efficiency",
			usage: types.TokenUsage{
				InputTokens:  10,
				OutputTokens: 40,
			},
			duration: time.Second,
			expected: "50.0 tokens/sec",
		},
		{
			name:     "zero_duration",
			usage:    types.TokenUsage{InputTokens: 100, OutputTokens: 100},
			duration: 0,
			expected: "",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := renderer.calculateTokenEfficiency(tc.usage, tc.duration)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestCalculateToolMetrics(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	conv := types.ConversationBlock{
		ToolCalls: []types.ToolCall{
			{
				ID:       "tool1",
				ToolName: "fast_tool",
			},
			{
				ID:       "tool2",
				ToolName: "slow_tool",
			},
			{
				ID:       "tool3",
				ToolName: "failed_tool",
			},
		},
		ToolResults: []types.ToolResult{
			{
				ToolCallID: "tool1",
				Success:    true,
				Duration:   100 * time.Millisecond,
			},
			{
				ToolCallID: "tool2",
				Success:    true,
				Duration:   2 * time.Second,
			},
			{
				ToolCallID: "tool3",
				Success:    false,
				Duration:   500 * time.Millisecond,
			},
		},
	}
	
	metrics := renderer.calculateToolMetrics(conv)
	
	// Check total duration
	expectedTotal := 100*time.Millisecond + 2*time.Second + 500*time.Millisecond
	if metrics.TotalDuration != expectedTotal {
		t.Errorf("Expected total duration %v, got %v", expectedTotal, metrics.TotalDuration)
	}
	
	// Check success rate (2 out of 3 successful)
	expectedRate := 66.66666666666667 // 2/3 * 100
	if abs(metrics.SuccessRate-expectedRate) > 0.1 {
		t.Errorf("Expected success rate %.2f%%, got %.2f%%", expectedRate, metrics.SuccessRate)
	}
	
	// Check slowest tool
	if metrics.SlowestTool != "slow_tool" {
		t.Errorf("Expected slowest tool 'slow_tool', got '%s'", metrics.SlowestTool)
	}
	
	if metrics.SlowestDuration != 2*time.Second {
		t.Errorf("Expected slowest duration 2s, got %v", metrics.SlowestDuration)
	}
}

func TestCalculateFileOperationStats(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	fileOps := []types.FileOperation{
		{OperationType: "read"},
		{OperationType: "read"},
		{OperationType: "write"},
		{OperationType: "edit"},
		{OperationType: "delete"},
		{OperationType: "unknown"}, // Should not be counted
	}
	
	stats := renderer.calculateFileOperationStats(fileOps)
	
	if stats.Reads != 2 {
		t.Errorf("Expected 2 reads, got %d", stats.Reads)
	}
	
	if stats.Writes != 1 {
		t.Errorf("Expected 1 write, got %d", stats.Writes)
	}
	
	if stats.Edits != 1 {
		t.Errorf("Expected 1 edit, got %d", stats.Edits)
	}
	
	if stats.Deletes != 1 {
		t.Errorf("Expected 1 delete, got %d", stats.Deletes)
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}