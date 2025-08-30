package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigFromFile_ValidConfig(t *testing.T) {
	config, err := LoadConfigFromFile("testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	// Test LLM config
	if config.LLM.Endpoint != "http://localhost:1234/v1" {
		t.Errorf("Expected endpoint http://localhost:1234/v1, got %s", config.LLM.Endpoint)
	}
	if config.LLM.Model != "qwen/qwen2.5-coder-14b" {
		t.Errorf("Expected model qwen/qwen2.5-coder-14b, got %s", config.LLM.Model)
	}
	if config.LLM.Token != "test-token" {
		t.Errorf("Expected token test-token, got %s", config.LLM.Token)
	}

	// Test Storage config
	if config.Storage.DatabasePath != "/tmp/test.db" {
		t.Errorf("Expected database path /tmp/test.db, got %s", config.Storage.DatabasePath)
	}
	if config.Storage.CacheSize != 100 {
		t.Errorf("Expected cache size 100, got %d", config.Storage.CacheSize)
	}
	if config.Storage.AutoSaveFreq != 5*time.Minute {
		t.Errorf("Expected auto-save freq 5m, got %v", config.Storage.AutoSaveFreq)
	}
	if !config.Storage.EnableCompression {
		t.Error("Expected compression to be enabled")
	}

	// Test Security config
	expectedAllowedPaths := []string{"."}
	if len(config.Security.FileOperations.AllowedPaths) != len(expectedAllowedPaths) {
		t.Errorf("Expected %d allowed paths, got %d", len(expectedAllowedPaths), len(config.Security.FileOperations.AllowedPaths))
	}
	for i, expected := range expectedAllowedPaths {
		if config.Security.FileOperations.AllowedPaths[i] != expected {
			t.Errorf("Expected allowed path %s at index %d, got %s", expected, i, config.Security.FileOperations.AllowedPaths[i])
		}
	}

	// Test UI config
	if config.UI.Theme != "default" {
		t.Errorf("Expected theme default, got %s", config.UI.Theme)
	}
	if !config.UI.ShowTimestamps {
		t.Error("Expected show timestamps to be true")
	}

	// Test Tools config
	if config.Tools.DefaultTimeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Tools.DefaultTimeout)
	}
	if config.Tools.MaxConcurrent != 5 {
		t.Errorf("Expected max concurrent 5, got %d", config.Tools.MaxConcurrent)
	}
}

func TestLoadConfigFromFile_PartialConfig(t *testing.T) {
	config, err := LoadConfigFromFile("testdata/partial_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load partial config: %v", err)
	}

	// Test overridden values
	if config.LLM.Endpoint != "http://test-server:8080/v1" {
		t.Errorf("Expected endpoint http://test-server:8080/v1, got %s", config.LLM.Endpoint)
	}
	if config.LLM.Model != "custom-model" {
		t.Errorf("Expected model custom-model, got %s", config.LLM.Model)
	}
	if config.Storage.CacheSize != 200 {
		t.Errorf("Expected cache size 200, got %d", config.Storage.CacheSize)
	}
	if config.UI.Theme != "dark" {
		t.Errorf("Expected theme dark, got %s", config.UI.Theme)
	}
	if config.UI.ShowTimestamps {
		t.Error("Expected show timestamps to be false")
	}

	// Test default values are preserved
	if config.LLM.Timeout != 30 {
		t.Errorf("Expected default timeout 30, got %d", config.LLM.Timeout)
	}
	if config.Storage.AutoSaveFreq != 5*time.Minute {
		t.Errorf("Expected default auto-save freq 5m, got %v", config.Storage.AutoSaveFreq)
	}
}

func TestLoadConfigFromFile_EmptyConfig(t *testing.T) {
	config, err := LoadConfigFromFile("testdata/empty_config.yaml")
	if err != nil {
		t.Fatalf("Failed to load empty config: %v", err)
	}

	// Should be all defaults
	defaultConfig := DefaultConfig()
	
	if config.LLM.Endpoint != defaultConfig.LLM.Endpoint {
		t.Errorf("Expected default endpoint %s, got %s", defaultConfig.LLM.Endpoint, config.LLM.Endpoint)
	}
	if config.Storage.CacheSize != defaultConfig.Storage.CacheSize {
		t.Errorf("Expected default cache size %d, got %d", defaultConfig.Storage.CacheSize, config.Storage.CacheSize)
	}
	if config.UI.Theme != defaultConfig.UI.Theme {
		t.Errorf("Expected default theme %s, got %s", defaultConfig.UI.Theme, config.UI.Theme)
	}
}

func TestLoadConfigFromFile_NonExistentFile(t *testing.T) {
	_, err := LoadConfigFromFile("testdata/non_existent.yaml")
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
}

func TestLoadConfig_NoFiles(t *testing.T) {
	// Save original environment
	originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalAppData := os.Getenv("APPDATA")
	
	// Set environment to non-existent directories
	os.Setenv("XDG_CONFIG_HOME", "/non/existent/config")
	os.Unsetenv("APPDATA")
	
	// Change to a temporary directory to avoid local config files
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)
	
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config with no files: %v", err)
	}
	
	// Should return defaults
	defaultConfig := DefaultConfig()
	if config.LLM.Endpoint != defaultConfig.LLM.Endpoint {
		t.Errorf("Expected default endpoint %s, got %s", defaultConfig.LLM.Endpoint, config.LLM.Endpoint)
	}
	
	// Restore environment and directory
	if originalXDGConfigHome != "" {
		os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
	} else {
		os.Unsetenv("XDG_CONFIG_HOME")
	}
	if originalAppData != "" {
		os.Setenv("APPDATA", originalAppData)
	}
	os.Chdir(originalDir)
}

func TestApplyEnvVars(t *testing.T) {
	config := DefaultConfig()
	
	// Set environment variables
	os.Setenv("GOCODENOW_LLM_ENDPOINT", "http://env-server:9999/v1")
	os.Setenv("GOCODENOW_LLM_TOKEN", "env-token")
	os.Setenv("GOCODENOW_LLM_MODEL", "env-model")
	os.Setenv("GOCODENOW_STORAGE_DATABASE_PATH", "/tmp/env.db")
	
	defer func() {
		os.Unsetenv("GOCODENOW_LLM_ENDPOINT")
		os.Unsetenv("GOCODENOW_LLM_TOKEN")
		os.Unsetenv("GOCODENOW_LLM_MODEL")
		os.Unsetenv("GOCODENOW_STORAGE_DATABASE_PATH")
	}()
	
	err := applyEnvVars(config)
	if err != nil {
		t.Fatalf("Failed to apply env vars: %v", err)
	}
	
	if config.LLM.Endpoint != "http://env-server:9999/v1" {
		t.Errorf("Expected endpoint from env, got %s", config.LLM.Endpoint)
	}
	if config.LLM.Token != "env-token" {
		t.Errorf("Expected token from env, got %s", config.LLM.Token)
	}
	if config.LLM.Model != "env-model" {
		t.Errorf("Expected model from env, got %s", config.LLM.Model)
	}
	if config.Storage.DatabasePath != "/tmp/env.db" {
		t.Errorf("Expected database path from env, got %s", config.Storage.DatabasePath)
	}
}

func TestApplyCLIFlags(t *testing.T) {
	config := DefaultConfig()
	
	endpoint := "http://cli-server:8888/v1"
	token := "cli-token"
	model := "cli-model"
	
	ApplyCLIFlags(config, &endpoint, &token, &model)
	
	if config.LLM.Endpoint != endpoint {
		t.Errorf("Expected endpoint from CLI, got %s", config.LLM.Endpoint)
	}
	if config.LLM.Token != token {
		t.Errorf("Expected token from CLI, got %s", config.LLM.Token)
	}
	if config.LLM.Model != model {
		t.Errorf("Expected model from CLI, got %s", config.LLM.Model)
	}
}

func TestApplyCLIFlags_NilValues(t *testing.T) {
	config := DefaultConfig()
	originalEndpoint := config.LLM.Endpoint
	originalToken := config.LLM.Token
	originalModel := config.LLM.Model
	
	ApplyCLIFlags(config, nil, nil, nil)
	
	// Values should remain unchanged
	if config.LLM.Endpoint != originalEndpoint {
		t.Errorf("Expected endpoint unchanged, got %s", config.LLM.Endpoint)
	}
	if config.LLM.Token != originalToken {
		t.Errorf("Expected token unchanged, got %s", config.LLM.Token)
	}
	if config.LLM.Model != originalModel {
		t.Errorf("Expected model unchanged, got %s", config.LLM.Model)
	}
}

func TestApplyCLIFlags_EmptyValues(t *testing.T) {
	config := DefaultConfig()
	originalEndpoint := config.LLM.Endpoint
	originalToken := config.LLM.Token
	originalModel := config.LLM.Model
	
	empty := ""
	ApplyCLIFlags(config, &empty, &empty, &empty)
	
	// Values should remain unchanged for empty strings
	if config.LLM.Endpoint != originalEndpoint {
		t.Errorf("Expected endpoint unchanged, got %s", config.LLM.Endpoint)
	}
	if config.LLM.Token != originalToken {
		t.Errorf("Expected token unchanged, got %s", config.LLM.Token)
	}
	if config.LLM.Model != originalModel {
		t.Errorf("Expected model unchanged, got %s", config.LLM.Model)
	}
}

func TestGlobalConfigPath(t *testing.T) {
	// Save original environment
	originalXDGConfigHome := os.Getenv("XDG_CONFIG_HOME")
	originalAppData := os.Getenv("APPDATA")
	
	// Test XDG_CONFIG_HOME
	os.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	os.Unsetenv("APPDATA")
	
	path := globalConfigPath()
	expected := "/tmp/xdg-config/gocodenow/config.yaml"
	if path != expected {
		t.Errorf("Expected XDG path %s, got %s", expected, path)
	}
	
	// Restore environment
	if originalXDGConfigHome != "" {
		os.Setenv("XDG_CONFIG_HOME", originalXDGConfigHome)
	} else {
		os.Unsetenv("XDG_CONFIG_HOME")
	}
	if originalAppData != "" {
		os.Setenv("APPDATA", originalAppData)
	}
}

func TestLocalConfigPath(t *testing.T) {
	path := localConfigPath()
	expected := "gocodenow.yaml"
	if path != expected {
		t.Errorf("Expected local config path %s, got %s", expected, path)
	}
}

func TestMergeConfigs(t *testing.T) {
	dst := DefaultConfig()
	src := &Config{
		LLM: LLMConfig{
			Endpoint: "http://override:1234/v1",
			Model:    "override-model",
			// Token left empty - should not override
		},
		Storage: StorageConfig{
			CacheSize: 200,
			// Other fields left empty - should not override
		},
		UI: UIConfig{
			Theme: "dark",
			// Boolean fields will always merge
			ShowTimestamps: false,
		},
	}
	
	originalToken := dst.LLM.Token
	originalAutoSaveFreq := dst.Storage.AutoSaveFreq
	originalMaxConcurrent := dst.Tools.MaxConcurrent
	
	mergeConfigs(dst, src)
	
	// Test overridden values
	if dst.LLM.Endpoint != "http://override:1234/v1" {
		t.Errorf("Expected merged endpoint, got %s", dst.LLM.Endpoint)
	}
	if dst.LLM.Model != "override-model" {
		t.Errorf("Expected merged model, got %s", dst.LLM.Model)
	}
	if dst.Storage.CacheSize != 200 {
		t.Errorf("Expected merged cache size, got %d", dst.Storage.CacheSize)
	}
	if dst.UI.Theme != "dark" {
		t.Errorf("Expected merged theme, got %s", dst.UI.Theme)
	}
	if dst.UI.ShowTimestamps {
		t.Error("Expected merged show timestamps to be false")
	}
	
	// Test preserved values
	if dst.LLM.Token != originalToken {
		t.Errorf("Expected original token preserved, got %s", dst.LLM.Token)
	}
	if dst.Storage.AutoSaveFreq != originalAutoSaveFreq {
		t.Errorf("Expected original auto-save freq preserved, got %v", dst.Storage.AutoSaveFreq)
	}
	if dst.Tools.MaxConcurrent != originalMaxConcurrent {
		t.Errorf("Expected original max concurrent preserved, got %d", dst.Tools.MaxConcurrent)
	}
}

func TestLoadConfigFile_HomeDirectoryExpansion(t *testing.T) {
	// Create a temporary config in a subdirectory to simulate home directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "lmcodenow")
	os.MkdirAll(configDir, 0755)
	
	configFile := filepath.Join(configDir, "test.yaml")
	configContent := `
llm:
  endpoint: "http://home-test:1234/v1"
  model: "home-test-model"
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	// Test with tilde expansion (simulated)
	config := DefaultConfig()
	relativeConfigPath := filepath.Join(configDir, "test.yaml")
	err = loadConfigFile(config, relativeConfigPath)
	if err != nil {
		t.Fatalf("Failed to load config with path expansion: %v", err)
	}
	
	if config.LLM.Endpoint != "http://home-test:1234/v1" {
		t.Errorf("Expected home-test endpoint, got %s", config.LLM.Endpoint)
	}
}