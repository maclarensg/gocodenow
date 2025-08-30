package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateConfig_Valid(t *testing.T) {
	config, err := LoadConfigFromFile("testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	err = ValidateConfig(config)
	if err != nil {
		t.Errorf("Expected valid config to pass validation, got: %v", err)
	}
}

func TestValidateConfig_Invalid(t *testing.T) {
	config, err := LoadConfigFromFile("testdata/invalid_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load invalid config: %v", err)
	}

	err = ValidateConfig(config)
	if err == nil {
		t.Error("Expected invalid config to fail validation")
	}

	// Check that it's a ValidationError
	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Errorf("Expected ValidationError, got %T", err)
	}

	// Check that multiple errors were found
	if len(validationErr.Errors) == 0 {
		t.Error("Expected validation errors to be found")
	}

	// Check for specific errors that are actually produced
	errorMessages := err.Error()
	expectedErrors := []string{
		"llm.timeout: timeout must be positive",
		"llm.max_retries: max_retries should not exceed 10", 
		"storage.cache_size: cache_size must be positive",
		"storage.auto_save_freq: auto_save_freq should be at least 10 seconds",
		"ui.theme: theme must be one of: default, dark, light",
		"ui.max_history_size: max_history_size must be positive",
		"tools.max_concurrent: max_concurrent must be positive",
	}

	for _, expectedError := range expectedErrors {
		if !strings.Contains(errorMessages, expectedError) {
			t.Errorf("Expected error message to contain '%s', got: %s", expectedError, errorMessages)
		}
	}
}

func TestValidateLLMConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      LLMConfig
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: LLMConfig{
				Endpoint:     "http://localhost:1234/v1",
				Model:        "test-model",
				Timeout:      30,
				MaxRetries:   3,
				RateLimitRPM: 60,
			},
			expectError: false,
		},
		{
			name: "empty endpoint",
			config: LLMConfig{
				Endpoint: "",
				Model:    "test-model",
				Timeout:  30,
			},
			expectError: true,
			errorField:  "llm.endpoint",
		},
		{
			name: "invalid endpoint URL",
			config: LLMConfig{
				Endpoint: "not-a-url",
				Model:    "test-model",
				Timeout:  30,
			},
			expectError: true,
			errorField:  "llm.endpoint",
		},
		{
			name: "empty model",
			config: LLMConfig{
				Endpoint: "http://localhost:1234/v1",
				Model:    "",
				Timeout:  30,
			},
			expectError: true,
			errorField:  "llm.model",
		},
		{
			name: "negative timeout",
			config: LLMConfig{
				Endpoint: "http://localhost:1234/v1",
				Model:    "test-model",
				Timeout:  -1,
			},
			expectError: true,
			errorField:  "llm.timeout",
		},
		{
			name: "timeout too large",
			config: LLMConfig{
				Endpoint: "http://localhost:1234/v1",
				Model:    "test-model",
				Timeout:  400, // > 300 seconds
			},
			expectError: true,
			errorField:  "llm.timeout",
		},
		{
			name: "negative max retries",
			config: LLMConfig{
				Endpoint:   "http://localhost:1234/v1",
				Model:      "test-model",
				Timeout:    30,
				MaxRetries: -1,
			},
			expectError: true,
			errorField:  "llm.max_retries",
		},
		{
			name: "max retries too large",
			config: LLMConfig{
				Endpoint:   "http://localhost:1234/v1",
				Model:      "test-model", 
				Timeout:    30,
				MaxRetries: 15, // > 10
			},
			expectError: true,
			errorField:  "llm.max_retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateLLMConfig(&tt.config)
			
			if tt.expectError {
				if len(errors) == 0 {
					t.Error("Expected validation error but got none")
				} else {
					found := false
					for _, err := range errors {
						if strings.Contains(err.Field, tt.errorField) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error for field %s, got errors: %v", tt.errorField, errors)
					}
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no validation errors but got: %v", errors)
				}
			}
		})
	}
}

func TestValidateStorageConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	tests := []struct {
		name        string
		config      StorageConfig
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: StorageConfig{
				DatabasePath:    tmpDir + "/test.db",
				CacheSize:       100,
				AutoSaveFreq:    5 * time.Minute,
				MaxMemoryMB:     256,
				BackupInterval:  24 * time.Hour,
				CleanupInterval: 168 * time.Hour,
			},
			expectError: false,
		},
		{
			name: "empty database path",
			config: StorageConfig{
				DatabasePath: "",
				CacheSize:    100,
				AutoSaveFreq: 5 * time.Minute,
			},
			expectError: true,
			errorField:  "storage.database_path",
		},
		{
			name: "negative cache size",
			config: StorageConfig{
				DatabasePath: tmpDir + "/test.db",
				CacheSize:    -1,
				AutoSaveFreq: 5 * time.Minute,
			},
			expectError: true,
			errorField:  "storage.cache_size",
		},
		{
			name: "cache size too large",
			config: StorageConfig{
				DatabasePath: tmpDir + "/test.db",
				CacheSize:    20000, // > 10000
				AutoSaveFreq: 5 * time.Minute,
			},
			expectError: true,
			errorField:  "storage.cache_size",
		},
		{
			name: "auto save freq too short",
			config: StorageConfig{
				DatabasePath: tmpDir + "/test.db",
				CacheSize:    100,
				AutoSaveFreq: 1 * time.Second, // < 10 seconds
			},
			expectError: true,
			errorField:  "storage.auto_save_freq",
		},
		{
			name: "memory limit too large",
			config: StorageConfig{
				DatabasePath: tmpDir + "/test.db",
				CacheSize:    100,
				AutoSaveFreq: 5 * time.Minute,
				MaxMemoryMB:  20000, // > 10240
			},
			expectError: true,
			errorField:  "storage.max_memory_mb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateStorageConfig(&tt.config)
			
			if tt.expectError {
				if len(errors) == 0 {
					t.Error("Expected validation error but got none")
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no validation errors but got: %v", errors)
				}
			}
		})
	}
}

func TestValidateSecurityConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	tests := []struct {
		name        string
		config      SecurityConfig
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: SecurityConfig{
				FileOperations: FileSecurityConfig{
					AllowedPaths:       []string{tmpDir},
					BlockedPaths:       []string{"/etc", "/proc"},
					AllowedExtensions:  []string{".txt", ".md"},
					BlockedExtensions:  []string{".exe", ".bat"},
					AllowAbsolutePaths: false,
					AllowSymlinks:      false,
				},
				MaxFileSize:   10 * 1024 * 1024,
				AllowedHosts:  []string{"localhost"},
				EnableSandbox: false,
			},
			expectError: false,
		},
		{
			name: "negative max file size",
			config: SecurityConfig{
				FileOperations: FileSecurityConfig{
					AllowedPaths: []string{tmpDir},
				},
				MaxFileSize: -1,
			},
			expectError: true,
			errorField:  "security.max_file_size",
		},
		{
			name: "max file size too large",
			config: SecurityConfig{
				FileOperations: FileSecurityConfig{
					AllowedPaths: []string{tmpDir},
				},
				MaxFileSize: 2 * 1024 * 1024 * 1024, // 2GB > 1GB limit
			},
			expectError: true,
			errorField:  "security.max_file_size",
		},
		{
			name: "empty allowed host",
			config: SecurityConfig{
				FileOperations: FileSecurityConfig{
					AllowedPaths: []string{tmpDir},
				},
				AllowedHosts: []string{""},
			},
			expectError: true,
			errorField:  "security.allowed_hosts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateSecurityConfig(&tt.config)
			
			if tt.expectError {
				if len(errors) == 0 {
					t.Error("Expected validation error but got none")
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no validation errors but got: %v", errors)
				}
			}
		})
	}
}

func TestValidateFileSecurityConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	tests := []struct {
		name        string
		config      FileSecurityConfig
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: FileSecurityConfig{
				AllowedPaths:       []string{tmpDir},
				BlockedPaths:       []string{"/etc", "/proc"},
				AllowedExtensions:  []string{".txt", ".md"},
				BlockedExtensions:  []string{".exe", ".bat"},
				AllowAbsolutePaths: false,
				AllowSymlinks:      false,
			},
			expectError: false,
		},
		{
			name: "empty allowed path",
			config: FileSecurityConfig{
				AllowedPaths: []string{""},
			},
			expectError: true,
			errorField:  "security.file_operations.allowed_paths",
		},
		{
			name: "non-existent allowed path",
			config: FileSecurityConfig{
				AllowedPaths: []string{"/non/existent/path"},
			},
			expectError: true,
			errorField:  "security.file_operations.allowed_paths",
		},
		{
			name: "empty blocked path",
			config: FileSecurityConfig{
				AllowedPaths: []string{tmpDir},
				BlockedPaths: []string{""},
			},
			expectError: true,
			errorField:  "security.file_operations.blocked_paths",
		},
		{
			name: "invalid allowed extension",
			config: FileSecurityConfig{
				AllowedPaths:      []string{tmpDir},
				AllowedExtensions: []string{"txt"}, // Missing dot
			},
			expectError: true,
			errorField:  "security.file_operations.allowed_extensions",
		},
		{
			name: "invalid blocked extension",
			config: FileSecurityConfig{
				AllowedPaths:      []string{tmpDir},
				BlockedExtensions: []string{"exe"}, // Missing dot
			},
			expectError: true,
			errorField:  "security.file_operations.blocked_extensions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateFileSecurityConfig(&tt.config)
			
			if tt.expectError {
				if len(errors) == 0 {
					t.Error("Expected validation error but got none")
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no validation errors but got: %v", errors)
				}
			}
		})
	}
}

func TestValidateUIConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      UIConfig
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: UIConfig{
				Theme:           "default",
				DefaultExpanded: false,
				ShowTimestamps:  true,
				AutoScroll:      true,
				MaxHistorySize:  1000,
			},
			expectError: false,
		},
		{
			name: "empty theme",
			config: UIConfig{
				Theme:          "",
				MaxHistorySize: 1000,
			},
			expectError: true,
			errorField:  "ui.theme",
		},
		{
			name: "invalid theme",
			config: UIConfig{
				Theme:          "invalid_theme",
				MaxHistorySize: 1000,
			},
			expectError: true,
			errorField:  "ui.theme",
		},
		{
			name: "negative history size",
			config: UIConfig{
				Theme:          "default",
				MaxHistorySize: -1,
			},
			expectError: true,
			errorField:  "ui.max_history_size",
		},
		{
			name: "history size too large",
			config: UIConfig{
				Theme:          "default",
				MaxHistorySize: 200000, // > 100000
			},
			expectError: true,
			errorField:  "ui.max_history_size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateUIConfig(&tt.config)
			
			if tt.expectError {
				if len(errors) == 0 {
					t.Error("Expected validation error but got none")
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no validation errors but got: %v", errors)
				}
			}
		})
	}
}

func TestValidateToolsConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	tests := []struct {
		name        string
		config      ToolsConfig
		expectError bool
		errorField  string
	}{
		{
			name: "valid config",
			config: ToolsConfig{
				DefaultTimeout:   30 * time.Second,
				MaxConcurrent:    5,
				EnableBuiltins:   true,
				CustomToolsPaths: []string{},
			},
			expectError: false,
		},
		{
			name: "negative timeout",
			config: ToolsConfig{
				DefaultTimeout: -1 * time.Second,
				MaxConcurrent:  5,
			},
			expectError: true,
			errorField:  "tools.default_timeout",
		},
		{
			name: "timeout too large",
			config: ToolsConfig{
				DefaultTimeout: 15 * time.Minute, // > 10 minutes
				MaxConcurrent:  5,
			},
			expectError: true,
			errorField:  "tools.default_timeout",
		},
		{
			name: "negative concurrency",
			config: ToolsConfig{
				DefaultTimeout: 30 * time.Second,
				MaxConcurrent:  -1,
			},
			expectError: true,
			errorField:  "tools.max_concurrent",
		},
		{
			name: "concurrency too large",
			config: ToolsConfig{
				DefaultTimeout: 30 * time.Second,
				MaxConcurrent:  200, // > 100
			},
			expectError: true,
			errorField:  "tools.max_concurrent",
		},
		{
			name: "empty custom tools path",
			config: ToolsConfig{
				DefaultTimeout:   30 * time.Second,
				MaxConcurrent:    5,
				CustomToolsPaths: []string{""},
			},
			expectError: true,
			errorField:  "tools.custom_tools_paths",
		},
		{
			name: "non-existent custom tools path",
			config: ToolsConfig{
				DefaultTimeout:   30 * time.Second,
				MaxConcurrent:    5,
				CustomToolsPaths: []string{"/non/existent/path"},
			},
			expectError: true,
			errorField:  "tools.custom_tools_paths",
		},
		{
			name: "valid custom tools path",
			config: ToolsConfig{
				DefaultTimeout:   30 * time.Second,
				MaxConcurrent:    5,
				CustomToolsPaths: []string{tmpDir},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validateToolsConfig(&tt.config)
			
			if tt.expectError {
				if len(errors) == 0 {
					t.Error("Expected validation error but got none")
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected no validation errors but got: %v", errors)
				}
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Errors: []ConfigError{
			{
				Field:   "test.field1",
				Value:   "invalid_value",
				Message: "must be valid",
			},
			{
				Field:   "test.field2",
				Value:   -1,
				Message: "must be positive",
			},
		},
	}

	errorMsg := err.Error()
	
	if !strings.Contains(errorMsg, "configuration validation failed") {
		t.Error("Expected error message to contain validation failed message")
	}
	if !strings.Contains(errorMsg, "test.field1: must be valid (got invalid_value)") {
		t.Error("Expected error message to contain field1 error")
	}
	if !strings.Contains(errorMsg, "test.field2: must be positive (got -1)") {
		t.Error("Expected error message to contain field2 error")
	}
}

func TestValidationError_Error_Empty(t *testing.T) {
	err := &ValidationError{
		Errors: []ConfigError{},
	}

	errorMsg := err.Error()
	expected := "configuration validation failed: no specific errors"
	if errorMsg != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, errorMsg)
	}
}