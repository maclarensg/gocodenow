package ui

import (
	"testing"
	"time"

	"gocodenow/internal/types"
)

func TestGetToolResultStatusIcon(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	testCases := []struct {
		name      string
		result    *types.ToolResult
		isRunning bool
		expected  string // We check if we get a non-empty string since icons are unicode
	}{
		{
			name:      "running_tool",
			result:    nil,
			isRunning: true,
			expected:  "not_empty",
		},
		{
			name: "fast_success",
			result: &types.ToolResult{
				Success:  true,
				Duration: 500 * time.Millisecond,
			},
			isRunning: false,
			expected:  "⚡",
		},
		{
			name: "normal_success", 
			result: &types.ToolResult{
				Success:  true,
				Duration: 3 * time.Second,
			},
			isRunning: false,
			expected:  "✅",
		},
		{
			name: "slow_success",
			result: &types.ToolResult{
				Success:  true,
				Duration: 10 * time.Second,
			},
			isRunning: false,
			expected:  "🐌",
		},
		{
			name: "timeout_error",
			result: &types.ToolResult{
				Success:      false,
				ErrorMessage: "operation timed out",
			},
			isRunning: false,
			expected:  "⏰",
		},
		{
			name: "permission_error",
			result: &types.ToolResult{
				Success:      false,
				ErrorMessage: "permission denied",
			},
			isRunning: false,
			expected:  "🔒",
		},
		{
			name: "not_found_error",
			result: &types.ToolResult{
				Success:      false,
				ErrorMessage: "file not found",
			},
			isRunning: false,
			expected:  "🔍",
		},
		{
			name: "streaming_result",
			result: &types.ToolResult{
				IsStreaming: true,
				Success:     false, // Not complete yet
			},
			isRunning: false,
			expected:  "📡",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := renderer.getToolResultStatusIcon(tc.result, tc.isRunning)
			
			if result == "" {
				t.Error("Status icon should not be empty")
			}
			
			if tc.expected != "not_empty" && result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestGetFileOperationIcon(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	testCases := []struct {
		name      string
		operation types.FileOperation
		expected  string
	}{
		{
			name: "successful_read",
			operation: types.FileOperation{
				OperationType: "read",
				Success:       true,
			},
			expected: "📖",
		},
		{
			name: "successful_write",
			operation: types.FileOperation{
				OperationType: "write",
				Success:       true,
			},
			expected: "📝",
		},
		{
			name: "failed_delete",
			operation: types.FileOperation{
				OperationType: "delete",
				Success:       false,
			},
			expected: "❌",
		},
		{
			name: "successful_edit",
			operation: types.FileOperation{
				OperationType: "edit",
				Success:       true,
			},
			expected: "✏️",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := renderer.getFileOperationIcon(tc.operation)
			
			if result == "" {
				t.Error("File operation icon should not be empty")
			}
			
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestGetProgressIndicator(t *testing.T) {
	renderer := NewRenderer(80, 24)
	
	testCases := []struct {
		stage    string
		expected string
	}{
		{"starting", "🚀"},
		{"processing", "⚙️"},
		{"building", "🔨"},
		{"testing", "🧪"},
		{"finalizing", "🏁"},
		{"unknown", "⚡"}, // Default case
	}
	
	for _, tc := range testCases {
		t.Run(tc.stage, func(t *testing.T) {
			result := renderer.getProgressIndicator(tc.stage)
			
			if result == "" {
				t.Error("Progress indicator should not be empty")
			}
			
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}