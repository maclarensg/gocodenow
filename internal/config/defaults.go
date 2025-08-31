package config

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultConfig returns the default application configuration
func DefaultConfig() *Config {
	return &Config{
		LLM:      DefaultLLMConfig(),
		Storage:  DefaultStorageConfig(),
		Security: DefaultSecurityConfig(),
		UI:       DefaultUIConfig(),
		Tools:    DefaultToolsConfig(),
	}
}

// DefaultLLMConfig returns default LLM configuration
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		Endpoint:     "http://localhost:1234/v1",
		Token:        "",
		Model:        "qwen/qwen2.5-coder-14b",
		Timeout:      30,
		MaxRetries:   3,
		RateLimitRPM: 60,
	}
}

// DefaultStorageConfig returns default storage configuration
func DefaultStorageConfig() StorageConfig {
	// Get user's data directory
	userDataDir := getUserDataDir()
	
	return StorageConfig{
		DatabasePath:      filepath.Join(userDataDir, "gocodenow", "conversations.db"),
		CacheSize:         50,
		AutoSaveFreq:      5 * time.Minute,
		MaxMemoryMB:       100,
		BackupInterval:    24 * time.Hour,
		CleanupInterval:   7 * 24 * time.Hour,
		EnableCompression: false,
	}
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() SecurityConfig {
	// Get current working directory for path validation
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	
	return SecurityConfig{
		FileOperations: FileSecurityConfig{
			AllowedPaths:       []string{".", "./", cwd},
			BlockedPaths:       []string{"/etc", "/proc", "/sys", "/dev", "/root", "/usr/bin", "/usr/sbin"},
			AllowAbsolutePaths: false,
			AllowSymlinks:      false,
			AllowedExtensions:  []string{}, // Empty means all extensions allowed
			BlockedExtensions: []string{
				".exe", ".bat", ".cmd", ".com", ".scr", ".pif", ".msi", ".msp", 
				".dll", ".so", ".dylib", ".deb", ".rpm", ".dmg", ".pkg",
			},
		},
		MaxFileSize:   10 * 1024 * 1024, // 10MB
		AllowedHosts:  []string{},        // Empty means all hosts allowed
		EnableSandbox: false,
	}
}

// DefaultUIConfig returns default UI configuration
func DefaultUIConfig() UIConfig {
	return UIConfig{
		Theme:           "default",
		DefaultExpanded: false,
		ShowTimestamps:  true,
		AutoScroll:      true,
		MaxHistorySize:  1000,
	}
}

// DefaultToolsConfig returns default tools configuration
func DefaultToolsConfig() ToolsConfig {
	return ToolsConfig{
		DefaultTimeout:   30 * time.Second,
		MaxConcurrent:    5,
		EnableBuiltins:   true,
		CustomToolsPaths: []string{},
	}
}

// getUserDataDir returns the appropriate user data directory for the current OS
func getUserDataDir() string {
	// Check XDG_DATA_HOME first (Linux)
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return dataHome
	}
	
	// Check APPDATA (Windows)
	if appData := os.Getenv("APPDATA"); appData != "" {
		return appData
	}
	
	// Default to home directory with appropriate subdirectory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "." // Fallback to current directory
	}
	
	// Use platform-appropriate data directory
	switch {
	case os.Getenv("XDG_DATA_HOME") != "" || fileExists("/etc/os-release"):
		// Linux: ~/.local/share
		return filepath.Join(homeDir, ".local", "share")
	case os.Getenv("APPDATA") != "":
		// Windows: already handled above
		return os.Getenv("APPDATA")
	default:
		// macOS and others: ~/Library/Application Support or ~/.config
		macOSDir := filepath.Join(homeDir, "Library", "Application Support")
		if fileExists(macOSDir) {
			return macOSDir
		}
		return filepath.Join(homeDir, ".config")
	}
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}