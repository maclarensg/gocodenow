package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test LLM defaults
	if config.LLM.Endpoint == "" {
		t.Error("Expected default endpoint to be non-empty")
	}
	if config.LLM.Model == "" {
		t.Error("Expected default model to be non-empty")
	}
	if config.LLM.Timeout <= 0 {
		t.Error("Expected default timeout to be positive")
	}

	// Test Storage defaults
	if config.Storage.DatabasePath == "" {
		t.Error("Expected default database path to be non-empty")
	}
	if config.Storage.CacheSize <= 0 {
		t.Error("Expected default cache size to be positive")
	}
	if config.Storage.AutoSaveFreq <= 0 {
		t.Error("Expected default auto-save frequency to be positive")
	}

	// Test Security defaults
	if len(config.Security.FileOperations.AllowedPaths) == 0 {
		t.Error("Expected default allowed paths to be non-empty")
	}
	if len(config.Security.FileOperations.BlockedPaths) == 0 {
		t.Error("Expected default blocked paths to be non-empty")
	}

	// Test UI defaults
	if config.UI.Theme == "" {
		t.Error("Expected default theme to be non-empty")
	}
	if config.UI.MaxHistorySize <= 0 {
		t.Error("Expected default max history size to be positive")
	}

	// Test Tools defaults
	if config.Tools.DefaultTimeout <= 0 {
		t.Error("Expected default timeout to be positive")
	}
	if config.Tools.MaxConcurrent <= 0 {
		t.Error("Expected default max concurrent to be positive")
	}
}

func TestDefaultLLMConfig(t *testing.T) {
	config := DefaultLLMConfig()

	expectedEndpoint := "http://localhost:1234/v1"
	if config.Endpoint != expectedEndpoint {
		t.Errorf("Expected endpoint %s, got %s", expectedEndpoint, config.Endpoint)
	}

	expectedModel := "qwen/qwen2.5-coder-14b"
	if config.Model != expectedModel {
		t.Errorf("Expected model %s, got %s", expectedModel, config.Model)
	}

	if config.Timeout != 30 {
		t.Errorf("Expected timeout 30, got %d", config.Timeout)
	}

	if config.MaxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", config.MaxRetries)
	}

	if config.RateLimitRPM != 60 {
		t.Errorf("Expected rate limit 60, got %d", config.RateLimitRPM)
	}
}

func TestDefaultStorageConfig(t *testing.T) {
	config := DefaultStorageConfig()

	if config.CacheSize != 50 {
		t.Errorf("Expected cache size 50, got %d", config.CacheSize)
	}

	if config.AutoSaveFreq != 5*time.Minute {
		t.Errorf("Expected auto-save frequency 5m, got %v", config.AutoSaveFreq)
	}

	if config.MaxMemoryMB != 100 {
		t.Errorf("Expected max memory 100, got %d", config.MaxMemoryMB)
	}

	if config.BackupInterval != 24*time.Hour {
		t.Errorf("Expected backup interval 24h, got %v", config.BackupInterval)
	}

	if config.CleanupInterval != 7*24*time.Hour {
		t.Errorf("Expected cleanup interval 168h, got %v", config.CleanupInterval)
	}

	if config.EnableCompression != false {
		t.Errorf("Expected compression false, got %v", config.EnableCompression)
	}
}

func TestDefaultSecurityConfig(t *testing.T) {
	config := DefaultSecurityConfig()

	// Test file security defaults
	if len(config.FileOperations.AllowedPaths) == 0 {
		t.Error("Expected allowed paths to be non-empty")
	}

	if len(config.FileOperations.BlockedPaths) == 0 {
		t.Error("Expected blocked paths to be non-empty")
	}

	if config.FileOperations.AllowAbsolutePaths != false {
		t.Error("Expected allow absolute paths to be false by default")
	}

	if config.FileOperations.AllowSymlinks != false {
		t.Error("Expected allow symlinks to be false by default")
	}

	if len(config.FileOperations.BlockedExtensions) == 0 {
		t.Error("Expected blocked extensions to be non-empty")
	}

	// Test other security defaults
	if config.MaxFileSize != 10*1024*1024 {
		t.Errorf("Expected max file size 10MB, got %d", config.MaxFileSize)
	}

	if config.EnableSandbox != false {
		t.Error("Expected sandbox to be disabled by default")
	}
}

func TestDefaultUIConfig(t *testing.T) {
	config := DefaultUIConfig()

	if config.Theme != "default" {
		t.Errorf("Expected theme 'default', got %s", config.Theme)
	}

	if config.DefaultExpanded != false {
		t.Error("Expected default expanded to be false")
	}

	if config.ShowTimestamps != true {
		t.Error("Expected show timestamps to be true")
	}

	if config.AutoScroll != true {
		t.Error("Expected auto scroll to be true")
	}

	if config.MaxHistorySize != 1000 {
		t.Errorf("Expected max history size 1000, got %d", config.MaxHistorySize)
	}
}

func TestDefaultToolsConfig(t *testing.T) {
	config := DefaultToolsConfig()

	if config.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.DefaultTimeout)
	}

	if config.MaxConcurrent != 5 {
		t.Errorf("Expected max concurrent 5, got %d", config.MaxConcurrent)
	}

	if config.EnableBuiltins != true {
		t.Error("Expected enable builtins to be true")
	}

	if len(config.CustomToolsPaths) != 0 {
		t.Error("Expected custom tools paths to be empty by default")
	}
}

func TestGetUserDataDir(t *testing.T) {
	// Save original environment
	originalXDGDataHome := os.Getenv("XDG_DATA_HOME")
	originalAppData := os.Getenv("APPDATA")

	// Test XDG_DATA_HOME
	os.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	dataDir := getUserDataDir()
	if dataDir != "/tmp/xdg" {
		t.Errorf("Expected /tmp/xdg, got %s", dataDir)
	}

	// Test APPDATA (simulate Windows)
	os.Unsetenv("XDG_DATA_HOME")
	os.Setenv("APPDATA", "/tmp/appdata")
	dataDir = getUserDataDir()
	if dataDir != "/tmp/appdata" {
		t.Errorf("Expected /tmp/appdata, got %s", dataDir)
	}

	// Restore original environment
	if originalXDGDataHome != "" {
		os.Setenv("XDG_DATA_HOME", originalXDGDataHome)
	} else {
		os.Unsetenv("XDG_DATA_HOME")
	}
	if originalAppData != "" {
		os.Setenv("APPDATA", originalAppData)
	} else {
		os.Unsetenv("APPDATA")
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-file-exists-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test existing file
	if !fileExists(tmpFile.Name()) {
		t.Error("Expected temp file to exist")
	}

	// Test non-existing file
	if fileExists("/non/existent/file") {
		t.Error("Expected non-existent file to not exist")
	}

	// Test directory
	tmpDir := t.TempDir()
	if !fileExists(tmpDir) {
		t.Error("Expected temp directory to exist")
	}
}