package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConfigError represents a single configuration validation error
type ConfigError struct {
	Field   string
	Value   interface{}
	Message string
}

// ValidationError represents multiple configuration validation errors
type ValidationError struct {
	Errors []ConfigError
}

// Error implements the error interface for ValidationError
func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "configuration validation failed: no specific errors"
	}
	
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("%s: %s (got %v)", err.Field, err.Message, err.Value))
	}
	return "configuration validation failed:\n" + strings.Join(msgs, "\n")
}

// ValidateConfig validates the entire configuration and returns any errors
func ValidateConfig(config *Config) error {
	var errors []ConfigError

	// Validate LLM configuration
	if errs := validateLLMConfig(&config.LLM); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate Storage configuration
	if errs := validateStorageConfig(&config.Storage); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate Security configuration
	if errs := validateSecurityConfig(&config.Security); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate UI configuration
	if errs := validateUIConfig(&config.UI); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate Tools configuration
	if errs := validateToolsConfig(&config.Tools); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	if len(errors) > 0 {
		return &ValidationError{Errors: errors}
	}

	return nil
}

// validateLLMConfig validates LLM configuration
func validateLLMConfig(config *LLMConfig) []ConfigError {
	var errors []ConfigError

	// Validate endpoint URL
	if config.Endpoint == "" {
		errors = append(errors, ConfigError{
			Field:   "llm.endpoint",
			Value:   config.Endpoint,
			Message: "endpoint cannot be empty",
		})
	} else {
		if parsedURL, err := url.Parse(config.Endpoint); err != nil {
			errors = append(errors, ConfigError{
				Field:   "llm.endpoint",
				Value:   config.Endpoint,
				Message: fmt.Sprintf("invalid URL format: %v", err),
			})
		} else if parsedURL.Scheme == "" || parsedURL.Host == "" {
			errors = append(errors, ConfigError{
				Field:   "llm.endpoint",
				Value:   config.Endpoint,
				Message: "endpoint must be a complete URL with scheme and host",
			})
		}
	}

	// Validate model name
	if config.Model == "" {
		errors = append(errors, ConfigError{
			Field:   "llm.model",
			Value:   config.Model,
			Message: "model name cannot be empty",
		})
	}

	// Validate timeout
	if config.Timeout <= 0 {
		errors = append(errors, ConfigError{
			Field:   "llm.timeout",
			Value:   config.Timeout,
			Message: "timeout must be positive",
		})
	} else if config.Timeout > 300 { // 5 minutes max
		errors = append(errors, ConfigError{
			Field:   "llm.timeout",
			Value:   config.Timeout,
			Message: "timeout should not exceed 300 seconds",
		})
	}

	// Validate max retries
	if config.MaxRetries < 0 {
		errors = append(errors, ConfigError{
			Field:   "llm.max_retries",
			Value:   config.MaxRetries,
			Message: "max_retries cannot be negative",
		})
	} else if config.MaxRetries > 10 {
		errors = append(errors, ConfigError{
			Field:   "llm.max_retries",
			Value:   config.MaxRetries,
			Message: "max_retries should not exceed 10",
		})
	}

	// Validate rate limit
	if config.RateLimitRPM < 0 {
		errors = append(errors, ConfigError{
			Field:   "llm.rate_limit_rpm",
			Value:   config.RateLimitRPM,
			Message: "rate_limit_rpm cannot be negative",
		})
	}

	return errors
}

// validateStorageConfig validates storage configuration
func validateStorageConfig(config *StorageConfig) []ConfigError {
	var errors []ConfigError

	// Validate database path
	if config.DatabasePath == "" {
		errors = append(errors, ConfigError{
			Field:   "storage.database_path",
			Value:   config.DatabasePath,
			Message: "database_path cannot be empty",
		})
	} else {
		// Check if the directory exists or can be created
		dbDir := filepath.Dir(config.DatabasePath)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			errors = append(errors, ConfigError{
				Field:   "storage.database_path",
				Value:   config.DatabasePath,
				Message: fmt.Sprintf("cannot create database directory: %v", err),
			})
		}
	}

	// Validate cache size
	if config.CacheSize <= 0 {
		errors = append(errors, ConfigError{
			Field:   "storage.cache_size",
			Value:   config.CacheSize,
			Message: "cache_size must be positive",
		})
	} else if config.CacheSize > 10000 {
		errors = append(errors, ConfigError{
			Field:   "storage.cache_size",
			Value:   config.CacheSize,
			Message: "cache_size should not exceed 10000",
		})
	}

	// Validate auto-save frequency
	if config.AutoSaveFreq <= 0 {
		errors = append(errors, ConfigError{
			Field:   "storage.auto_save_freq",
			Value:   config.AutoSaveFreq,
			Message: "auto_save_freq must be positive",
		})
	} else if config.AutoSaveFreq < 10*time.Second {
		errors = append(errors, ConfigError{
			Field:   "storage.auto_save_freq",
			Value:   config.AutoSaveFreq,
			Message: "auto_save_freq should be at least 10 seconds",
		})
	}

	// Validate memory limit
	if config.MaxMemoryMB <= 0 {
		errors = append(errors, ConfigError{
			Field:   "storage.max_memory_mb",
			Value:   config.MaxMemoryMB,
			Message: "max_memory_mb must be positive",
		})
	} else if config.MaxMemoryMB > 10240 { // 10GB
		errors = append(errors, ConfigError{
			Field:   "storage.max_memory_mb",
			Value:   config.MaxMemoryMB,
			Message: "max_memory_mb should not exceed 10240 MB (10GB)",
		})
	}

	// Validate backup interval
	if config.BackupInterval <= 0 {
		errors = append(errors, ConfigError{
			Field:   "storage.backup_interval",
			Value:   config.BackupInterval,
			Message: "backup_interval must be positive",
		})
	}

	// Validate cleanup interval
	if config.CleanupInterval <= 0 {
		errors = append(errors, ConfigError{
			Field:   "storage.cleanup_interval",
			Value:   config.CleanupInterval,
			Message: "cleanup_interval must be positive",
		})
	}

	return errors
}

// validateSecurityConfig validates security configuration
func validateSecurityConfig(config *SecurityConfig) []ConfigError {
	var errors []ConfigError

	// Validate file security config
	if errs := validateFileSecurityConfig(&config.FileOperations); len(errs) > 0 {
		errors = append(errors, errs...)
	}

	// Validate max file size
	if config.MaxFileSize <= 0 {
		errors = append(errors, ConfigError{
			Field:   "security.max_file_size",
			Value:   config.MaxFileSize,
			Message: "max_file_size must be positive",
		})
	} else if config.MaxFileSize > 1024*1024*1024 { // 1GB
		errors = append(errors, ConfigError{
			Field:   "security.max_file_size",
			Value:   config.MaxFileSize,
			Message: "max_file_size should not exceed 1GB",
		})
	}

	// Validate allowed hosts (if specified)
	for i, host := range config.AllowedHosts {
		if host == "" {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("security.allowed_hosts[%d]", i),
				Value:   host,
				Message: "host cannot be empty",
			})
		}
	}

	return errors
}

// validateFileSecurityConfig validates file security configuration
func validateFileSecurityConfig(config *FileSecurityConfig) []ConfigError {
	var errors []ConfigError

	// Validate allowed paths
	for i, path := range config.AllowedPaths {
		if path == "" {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("security.file_operations.allowed_paths[%d]", i),
				Value:   path,
				Message: "path cannot be empty",
			})
			continue
		}

		// Check if path exists (for validation purposes)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("security.file_operations.allowed_paths[%d]", i),
				Value:   path,
				Message: "path does not exist",
			})
		}
	}

	// Validate blocked paths
	for i, path := range config.BlockedPaths {
		if path == "" {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("security.file_operations.blocked_paths[%d]", i),
				Value:   path,
				Message: "path cannot be empty",
			})
		}
	}

	// Validate extensions
	for i, ext := range config.AllowedExtensions {
		if !strings.HasPrefix(ext, ".") {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("security.file_operations.allowed_extensions[%d]", i),
				Value:   ext,
				Message: "extension must start with '.'",
			})
		}
	}

	for i, ext := range config.BlockedExtensions {
		if !strings.HasPrefix(ext, ".") {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("security.file_operations.blocked_extensions[%d]", i),
				Value:   ext,
				Message: "extension must start with '.'",
			})
		}
	}

	return errors
}

// validateUIConfig validates UI configuration
func validateUIConfig(config *UIConfig) []ConfigError {
	var errors []ConfigError

	// Validate theme
	validThemes := []string{"default", "dark", "light"}
	if config.Theme == "" {
		errors = append(errors, ConfigError{
			Field:   "ui.theme",
			Value:   config.Theme,
			Message: "theme cannot be empty",
		})
	} else {
		valid := false
		for _, theme := range validThemes {
			if config.Theme == theme {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, ConfigError{
				Field:   "ui.theme",
				Value:   config.Theme,
				Message: fmt.Sprintf("theme must be one of: %s", strings.Join(validThemes, ", ")),
			})
		}
	}

	// Validate max history size
	if config.MaxHistorySize <= 0 {
		errors = append(errors, ConfigError{
			Field:   "ui.max_history_size",
			Value:   config.MaxHistorySize,
			Message: "max_history_size must be positive",
		})
	} else if config.MaxHistorySize > 100000 {
		errors = append(errors, ConfigError{
			Field:   "ui.max_history_size",
			Value:   config.MaxHistorySize,
			Message: "max_history_size should not exceed 100000",
		})
	}

	return errors
}

// validateToolsConfig validates tools configuration
func validateToolsConfig(config *ToolsConfig) []ConfigError {
	var errors []ConfigError

	// Validate default timeout
	if config.DefaultTimeout <= 0 {
		errors = append(errors, ConfigError{
			Field:   "tools.default_timeout",
			Value:   config.DefaultTimeout,
			Message: "default_timeout must be positive",
		})
	} else if config.DefaultTimeout > 10*time.Minute {
		errors = append(errors, ConfigError{
			Field:   "tools.default_timeout",
			Value:   config.DefaultTimeout,
			Message: "default_timeout should not exceed 10 minutes",
		})
	}

	// Validate max concurrent
	if config.MaxConcurrent <= 0 {
		errors = append(errors, ConfigError{
			Field:   "tools.max_concurrent",
			Value:   config.MaxConcurrent,
			Message: "max_concurrent must be positive",
		})
	} else if config.MaxConcurrent > 100 {
		errors = append(errors, ConfigError{
			Field:   "tools.max_concurrent",
			Value:   config.MaxConcurrent,
			Message: "max_concurrent should not exceed 100",
		})
	}

	// Validate custom tools paths
	for i, path := range config.CustomToolsPaths {
		if path == "" {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("tools.custom_tools_paths[%d]", i),
				Value:   path,
				Message: "path cannot be empty",
			})
			continue
		}

		// Check if path exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			errors = append(errors, ConfigError{
				Field:   fmt.Sprintf("tools.custom_tools_paths[%d]", i),
				Value:   path,
				Message: "path does not exist",
			})
		}
	}

	return errors
}