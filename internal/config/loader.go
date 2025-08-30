package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from files, environment variables, and applies defaults
// Loading precedence (lowest to highest):
// 1. Default values (hardcoded)
// 2. Global config file (~/.config/gocodenow/config.yaml)
// 3. Local config file (./gocodenow.yaml)
// 4. Environment variables (GOCODENOW_*)
// 5. CLI flags (applied separately by caller)
func LoadConfig() (*Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// Load global config file
	globalPath := globalConfigPath()
	if err := loadConfigFile(config, globalPath); err != nil {
		// Only return error if file exists but has issues
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading global config from %s: %w", globalPath, err)
		}
	}

	// Load local config file
	localPath := localConfigPath()
	if err := loadConfigFile(config, localPath); err != nil {
		// Only return error if file exists but has issues
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("loading local config from %s: %w", localPath, err)
		}
	}

	// Apply environment variables
	if err := applyEnvVars(config); err != nil {
		return nil, fmt.Errorf("applying environment variables: %w", err)
	}

	return config, nil
}

// LoadConfigFromFile loads configuration from a specific file path
func LoadConfigFromFile(path string) (*Config, error) {
	config := DefaultConfig()
	
	if err := loadConfigFile(config, path); err != nil {
		return nil, fmt.Errorf("loading config from %s: %w", path, err)
	}

	// Still apply environment variables
	if err := applyEnvVars(config); err != nil {
		return nil, fmt.Errorf("applying environment variables: %w", err)
	}

	return config, nil
}

// loadConfigFile loads configuration from a YAML file and merges it with existing config
func loadConfigFile(config *Config, path string) error {
	if path == "" {
		return nil
	}

	// Expand home directory if present
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("expanding home directory in path %s: %w", path, err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return err // Return the not-exist error so caller can decide how to handle
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	// Create a temporary config to unmarshal into
	var fileConfig Config
	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	// Merge file config into the existing config
	mergeConfigs(config, &fileConfig)

	return nil
}

// mergeConfigs merges the source config into the destination config
// Only non-zero values from source are applied to destination
func mergeConfigs(dst, src *Config) {
	// Merge LLM config
	mergeLLMConfig(&dst.LLM, &src.LLM)
	
	// Merge Storage config
	mergeStorageConfig(&dst.Storage, &src.Storage)
	
	// Merge Security config
	mergeSecurityConfig(&dst.Security, &src.Security)
	
	// Merge UI config
	mergeUIConfig(&dst.UI, &src.UI)
	
	// Merge Tools config
	mergeToolsConfig(&dst.Tools, &src.Tools)
}

func mergeLLMConfig(dst, src *LLMConfig) {
	if src.Endpoint != "" {
		dst.Endpoint = src.Endpoint
	}
	if src.Token != "" {
		dst.Token = src.Token
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Timeout != 0 {
		dst.Timeout = src.Timeout
	}
	if src.MaxRetries != 0 {
		dst.MaxRetries = src.MaxRetries
	}
	if src.RateLimitRPM != 0 {
		dst.RateLimitRPM = src.RateLimitRPM
	}
}

func mergeStorageConfig(dst, src *StorageConfig) {
	if src.DatabasePath != "" {
		dst.DatabasePath = src.DatabasePath
	}
	if src.CacheSize != 0 {
		dst.CacheSize = src.CacheSize
	}
	if src.AutoSaveFreq != 0 {
		dst.AutoSaveFreq = src.AutoSaveFreq
	}
	if src.MaxMemoryMB != 0 {
		dst.MaxMemoryMB = src.MaxMemoryMB
	}
	if src.BackupInterval != 0 {
		dst.BackupInterval = src.BackupInterval
	}
	if src.CleanupInterval != 0 {
		dst.CleanupInterval = src.CleanupInterval
	}
	// Boolean field - always merge
	dst.EnableCompression = src.EnableCompression
}

func mergeSecurityConfig(dst, src *SecurityConfig) {
	mergeFileSecurityConfig(&dst.FileOperations, &src.FileOperations)
	
	if src.MaxFileSize != 0 {
		dst.MaxFileSize = src.MaxFileSize
	}
	if len(src.AllowedHosts) > 0 {
		dst.AllowedHosts = src.AllowedHosts
	}
	// Boolean field - always merge
	dst.EnableSandbox = src.EnableSandbox
}

func mergeFileSecurityConfig(dst, src *FileSecurityConfig) {
	if len(src.AllowedPaths) > 0 {
		dst.AllowedPaths = src.AllowedPaths
	}
	if len(src.BlockedPaths) > 0 {
		dst.BlockedPaths = src.BlockedPaths
	}
	if len(src.AllowedExtensions) > 0 {
		dst.AllowedExtensions = src.AllowedExtensions
	}
	if len(src.BlockedExtensions) > 0 {
		dst.BlockedExtensions = src.BlockedExtensions
	}
	// Boolean fields - always merge
	dst.AllowAbsolutePaths = src.AllowAbsolutePaths
	dst.AllowSymlinks = src.AllowSymlinks
}

func mergeUIConfig(dst, src *UIConfig) {
	if src.Theme != "" {
		dst.Theme = src.Theme
	}
	if src.MaxHistorySize != 0 {
		dst.MaxHistorySize = src.MaxHistorySize
	}
	// Boolean fields - always merge
	dst.DefaultExpanded = src.DefaultExpanded
	dst.ShowTimestamps = src.ShowTimestamps
	dst.AutoScroll = src.AutoScroll
}

func mergeToolsConfig(dst, src *ToolsConfig) {
	if src.DefaultTimeout != 0 {
		dst.DefaultTimeout = src.DefaultTimeout
	}
	if src.MaxConcurrent != 0 {
		dst.MaxConcurrent = src.MaxConcurrent
	}
	if len(src.CustomToolsPaths) > 0 {
		dst.CustomToolsPaths = src.CustomToolsPaths
	}
	// Boolean field - always merge
	dst.EnableBuiltins = src.EnableBuiltins
}

// applyEnvVars applies environment variables to the configuration
// Environment variables use the format: GOCODENOW_SECTION_FIELD
// For example: GOCODENOW_LLM_ENDPOINT, GOCODENOW_STORAGE_CACHE_SIZE
func applyEnvVars(config *Config) error {
	// Map of environment variable names to config field setters
	envVars := map[string]func(string) error{
		"GOCODENOW_LLM_ENDPOINT":      func(v string) error { config.LLM.Endpoint = v; return nil },
		"GOCODENOW_LLM_TOKEN":         func(v string) error { config.LLM.Token = v; return nil },
		"GOCODENOW_LLM_MODEL":         func(v string) error { config.LLM.Model = v; return nil },
		"GOCODENOW_STORAGE_DATABASE_PATH": func(v string) error { config.Storage.DatabasePath = v; return nil },
	}

	// Apply environment variables that are set
	for envVar, setter := range envVars {
		if value := os.Getenv(envVar); value != "" {
			if err := setter(value); err != nil {
				return fmt.Errorf("applying environment variable %s: %w", envVar, err)
			}
		}
	}

	return nil
}

// globalConfigPath returns the path to the global configuration file
func globalConfigPath() string {
	// Check XDG_CONFIG_HOME first
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "gocodenow", "config.yaml")
	}

	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "" // No home directory available
	}

	// Platform-specific config directories
	switch {
	case fileExists("/etc/os-release"): // Linux
		return filepath.Join(homeDir, ".config", "gocodenow", "config.yaml")
	case os.Getenv("APPDATA") != "": // Windows
		return filepath.Join(os.Getenv("APPDATA"), "gocodenow", "config.yaml")
	default: // macOS and others
		macOSDir := filepath.Join(homeDir, "Library", "Application Support", "gocodenow", "config.yaml")
		if fileExists(filepath.Dir(macOSDir)) {
			return macOSDir
		}
		return filepath.Join(homeDir, ".config", "gocodenow", "config.yaml")
	}
}

// localConfigPath returns the path to the local (project-specific) configuration file
func localConfigPath() string {
	return "gocodenow.yaml"
}

// ApplyCLIFlags applies CLI flag overrides to the configuration
func ApplyCLIFlags(config *Config, endpoint, token, model *string) {
	if endpoint != nil && *endpoint != "" {
		config.LLM.Endpoint = *endpoint
	}
	if token != nil && *token != "" {
		config.LLM.Token = *token
	}
	if model != nil && *model != "" {
		config.LLM.Model = *model
	}
}