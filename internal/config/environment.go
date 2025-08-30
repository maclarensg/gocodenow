package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Environment represents different deployment environments
type Environment string

const (
	// EnvDevelopment represents a development environment
	EnvDevelopment Environment = "development"
	// EnvTesting represents a testing environment
	EnvTesting Environment = "testing"
	// EnvStaging represents a staging environment
	EnvStaging Environment = "staging"
	// EnvProduction represents a production environment
	EnvProduction Environment = "production"
	// EnvLocal represents a local development environment
	EnvLocal Environment = "local"
)

// EnvironmentConfig holds environment-specific configuration
type EnvironmentConfig struct {
	Environment Environment          `yaml:"environment" json:"environment"`
	Overrides   map[string]interface{} `yaml:"overrides" json:"overrides"`
	Enabled     bool                  `yaml:"enabled" json:"enabled"`
	Priority    int                   `yaml:"priority" json:"priority"`
	Description string                `yaml:"description,omitempty" json:"description,omitempty"`
}

// EnvironmentManager manages environment-specific configurations
type EnvironmentManager struct {
	currentEnv    Environment
	environments  map[Environment]*EnvironmentConfig
	configDir     string
	autoDetect    bool
}

// NewEnvironmentManager creates a new environment manager
func NewEnvironmentManager() *EnvironmentManager {
	configDir := filepath.Join(filepath.Dir(globalConfigPath()), "environments")
	
	em := &EnvironmentManager{
		environments: make(map[Environment]*EnvironmentConfig),
		configDir:    configDir,
		autoDetect:   true,
	}
	
	// Detect current environment
	em.currentEnv = em.detectEnvironment()
	
	return em
}

// detectEnvironment automatically detects the current environment
func (em *EnvironmentManager) detectEnvironment() Environment {
	// Check explicit environment variable
	if env := os.Getenv("GOCODENOW_ENV"); env != "" {
		return Environment(strings.ToLower(env))
	}
	
	// Check common environment indicators
	if os.Getenv("CI") != "" {
		if os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" {
			return EnvTesting
		}
	}
	
	if os.Getenv("PRODUCTION") != "" || os.Getenv("PROD") != "" {
		return EnvProduction
	}
	
	if os.Getenv("STAGING") != "" {
		return EnvStaging
	}
	
	if os.Getenv("TESTING") != "" || os.Getenv("TEST") != "" {
		return EnvTesting
	}
	
	// Check for development indicators
	if isDevEnvironment() {
		return EnvDevelopment
	}
	
	// Default to local
	return EnvLocal
}

// isDevEnvironment checks if we're in a development environment
func isDevEnvironment() bool {
	// Check for common development indicators
	devIndicators := []string{
		".git",
		"go.mod",
		"package.json",
		"Dockerfile",
		"docker-compose.yml",
	}
	
	cwd, _ := os.Getwd()
	for _, indicator := range devIndicators {
		if fileExists(filepath.Join(cwd, indicator)) {
			return true
		}
	}
	
	return false
}

// GetCurrentEnvironment returns the current environment
func (em *EnvironmentManager) GetCurrentEnvironment() Environment {
	return em.currentEnv
}

// SetEnvironment sets the current environment
func (em *EnvironmentManager) SetEnvironment(env Environment) error {
	if !em.isValidEnvironment(env) {
		return fmt.Errorf("invalid environment: %s", env)
	}
	
	em.currentEnv = env
	return nil
}

// isValidEnvironment checks if an environment is valid
func (em *EnvironmentManager) isValidEnvironment(env Environment) bool {
	validEnvs := []Environment{
		EnvDevelopment, EnvTesting, EnvStaging, EnvProduction, EnvLocal,
	}
	
	for _, validEnv := range validEnvs {
		if env == validEnv {
			return true
		}
	}
	
	return false
}

// LoadEnvironmentConfigs loads all environment-specific configurations
func (em *EnvironmentManager) LoadEnvironmentConfigs() error {
	// Create environments directory if it doesn't exist
	if err := os.MkdirAll(em.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create environments directory: %w", err)
	}
	
	// Load built-in environment configurations
	em.loadBuiltinEnvironments()
	
	// Load custom environment configurations from files
	entries, err := os.ReadDir(em.configDir)
	if err != nil {
		return fmt.Errorf("failed to read environments directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			envPath := filepath.Join(em.configDir, entry.Name())
			if err := em.loadEnvironmentFromFile(envPath); err != nil {
				// Log warning but continue
				fmt.Printf("Warning: failed to load environment config %s: %v\n", 
					entry.Name(), err)
			}
		}
	}
	
	return nil
}

// loadBuiltinEnvironments creates built-in environment configurations
func (em *EnvironmentManager) loadBuiltinEnvironments() {
	environments := []*EnvironmentConfig{
		em.createDevelopmentEnv(),
		em.createTestingEnv(),
		em.createStagingEnv(),
		em.createProductionEnv(),
		em.createLocalEnv(),
	}
	
	for _, env := range environments {
		em.environments[env.Environment] = env
	}
}

// createDevelopmentEnv creates development environment configuration
func (em *EnvironmentManager) createDevelopmentEnv() *EnvironmentConfig {
	return &EnvironmentConfig{
		Environment: EnvDevelopment,
		Enabled:     true,
		Priority:    100,
		Description: "Development environment with relaxed security and debug features",
		Overrides: map[string]interface{}{
			"llm.timeout":                      60,
			"llm.max_retries":                  1,
			"storage.auto_save_freq":           "1m",
			"storage.enable_compression":       false,
			"security.enable_sandbox":          false,
			"security.file_operations.allow_absolute_paths": true,
			"ui.default_expanded":              true,
			"ui.show_timestamps":               true,
			"tools.default_timeout":            "60s",
			"tools.max_concurrent":             10,
		},
	}
}

// createTestingEnv creates testing environment configuration
func (em *EnvironmentManager) createTestingEnv() *EnvironmentConfig {
	return &EnvironmentConfig{
		Environment: EnvTesting,
		Enabled:     true,
		Priority:    90,
		Description: "Testing environment with fast execution and minimal logging",
		Overrides: map[string]interface{}{
			"llm.timeout":                15,
			"llm.max_retries":            1,
			"storage.cache_size":         20,
			"storage.auto_save_freq":     "30s",
			"storage.max_memory_mb":      50,
			"security.enable_sandbox":    true,
			"ui.show_timestamps":         false,
			"ui.max_history_size":        50,
			"tools.default_timeout":      "15s",
			"tools.max_concurrent":       3,
		},
	}
}

// createStagingEnv creates staging environment configuration
func (em *EnvironmentManager) createStagingEnv() *EnvironmentConfig {
	return &EnvironmentConfig{
		Environment: EnvStaging,
		Enabled:     true,
		Priority:    80,
		Description: "Staging environment mirroring production with additional monitoring",
		Overrides: map[string]interface{}{
			"llm.timeout":                25,
			"llm.max_retries":            2,
			"storage.cache_size":         100,
			"storage.enable_compression": true,
			"storage.backup_interval":    "3h",
			"security.enable_sandbox":    true,
			"security.file_operations.allow_absolute_paths": false,
			"ui.max_history_size":        300,
			"tools.default_timeout":      "25s",
			"tools.max_concurrent":       5,
		},
	}
}

// createProductionEnv creates production environment configuration
func (em *EnvironmentManager) createProductionEnv() *EnvironmentConfig {
	return &EnvironmentConfig{
		Environment: EnvProduction,
		Enabled:     true,
		Priority:    70,
		Description: "Production environment with maximum security and stability",
		Overrides: map[string]interface{}{
			"llm.timeout":                30,
			"llm.max_retries":            3,
			"llm.rate_limit_rpm":         60,
			"storage.cache_size":         200,
			"storage.enable_compression": true,
			"storage.backup_interval":    "6h",
			"storage.cleanup_interval":   "168h", // 7 days
			"security.enable_sandbox":    true,
			"security.max_file_size":     5242880, // 5MB
			"security.file_operations.allow_absolute_paths": false,
			"security.file_operations.allow_symlinks":       false,
			"ui.default_expanded":        false,
			"ui.max_history_size":        500,
			"tools.default_timeout":      "30s",
			"tools.max_concurrent":       5,
		},
	}
}

// createLocalEnv creates local development environment configuration
func (em *EnvironmentManager) createLocalEnv() *EnvironmentConfig {
	return &EnvironmentConfig{
		Environment: EnvLocal,
		Enabled:     true,
		Priority:    110,
		Description: "Local development environment with maximum flexibility",
		Overrides: map[string]interface{}{
			"llm.endpoint":               "http://localhost:1234/v1",
			"llm.timeout":                120,
			"storage.auto_save_freq":     "2m",
			"storage.enable_compression": false,
			"security.enable_sandbox":    false,
			"security.file_operations.allow_absolute_paths": true,
			"security.file_operations.allow_symlinks":       true,
			"ui.default_expanded":        true,
			"ui.show_timestamps":         true,
			"tools.default_timeout":      "120s",
			"tools.max_concurrent":       15,
		},
	}
}

// ApplyEnvironmentOverrides applies environment-specific overrides to a configuration
func (em *EnvironmentManager) ApplyEnvironmentOverrides(config *Config) error {
	// Get environment configuration for current environment
	envConfig, exists := em.environments[em.currentEnv]
	if !exists || !envConfig.Enabled {
		// No overrides for this environment
		return nil
	}
	
	// Apply overrides
	for path, value := range envConfig.Overrides {
		if err := em.applyOverride(config, path, value); err != nil {
			return fmt.Errorf("failed to apply override %s: %w", path, err)
		}
	}
	
	return nil
}

// applyOverride applies a single configuration override
func (em *EnvironmentManager) applyOverride(config *Config, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return fmt.Errorf("invalid override path: %s", path)
	}
	
	section := parts[0]
	fieldPath := strings.Join(parts[1:], ".")
	
	switch section {
	case "llm":
		return em.applyLLMOverride(&config.LLM, fieldPath, value)
	case "storage":
		return em.applyStorageOverride(&config.Storage, fieldPath, value)
	case "security":
		return em.applySecurityOverride(&config.Security, fieldPath, value)
	case "ui":
		return em.applyUIOverride(&config.UI, fieldPath, value)
	case "tools":
		return em.applyToolsOverride(&config.Tools, fieldPath, value)
	default:
		return fmt.Errorf("unknown configuration section: %s", section)
	}
}

// Helper methods for applying overrides to specific sections

func (em *EnvironmentManager) applyLLMOverride(llm *LLMConfig, field string, value interface{}) error {
	switch field {
	case "endpoint":
		if str, ok := value.(string); ok {
			llm.Endpoint = str
		} else {
			return fmt.Errorf("endpoint must be a string")
		}
	case "token":
		if str, ok := value.(string); ok {
			llm.Token = str
		} else {
			return fmt.Errorf("token must be a string")
		}
	case "model":
		if str, ok := value.(string); ok {
			llm.Model = str
		} else {
			return fmt.Errorf("model must be a string")
		}
	case "timeout":
		if val, ok := em.convertToInt(value); ok {
			llm.Timeout = val
		} else {
			return fmt.Errorf("timeout must be an integer")
		}
	case "max_retries":
		if val, ok := em.convertToInt(value); ok {
			llm.MaxRetries = val
		} else {
			return fmt.Errorf("max_retries must be an integer")
		}
	case "rate_limit_rpm":
		if val, ok := em.convertToInt(value); ok {
			llm.RateLimitRPM = val
		} else {
			return fmt.Errorf("rate_limit_rpm must be an integer")
		}
	default:
		return fmt.Errorf("unknown LLM field: %s", field)
	}
	return nil
}

func (em *EnvironmentManager) applyStorageOverride(storage *StorageConfig, field string, value interface{}) error {
	switch field {
	case "database_path":
		if str, ok := value.(string); ok {
			storage.DatabasePath = str
		} else {
			return fmt.Errorf("database_path must be a string")
		}
	case "cache_size":
		if val, ok := em.convertToInt(value); ok {
			storage.CacheSize = val
		} else {
			return fmt.Errorf("cache_size must be an integer")
		}
	case "auto_save_freq":
		if dur, ok := em.convertToDuration(value); ok {
			storage.AutoSaveFreq = dur
		} else {
			return fmt.Errorf("auto_save_freq must be a duration")
		}
	case "max_memory_mb":
		if val, ok := em.convertToInt(value); ok {
			storage.MaxMemoryMB = val
		} else {
			return fmt.Errorf("max_memory_mb must be an integer")
		}
	case "backup_interval":
		if dur, ok := em.convertToDuration(value); ok {
			storage.BackupInterval = dur
		} else {
			return fmt.Errorf("backup_interval must be a duration")
		}
	case "cleanup_interval":
		if dur, ok := em.convertToDuration(value); ok {
			storage.CleanupInterval = dur
		} else {
			return fmt.Errorf("cleanup_interval must be a duration")
		}
	case "enable_compression":
		if val, ok := value.(bool); ok {
			storage.EnableCompression = val
		} else {
			return fmt.Errorf("enable_compression must be a boolean")
		}
	default:
		return fmt.Errorf("unknown storage field: %s", field)
	}
	return nil
}

func (em *EnvironmentManager) applySecurityOverride(security *SecurityConfig, field string, value interface{}) error {
	parts := strings.Split(field, ".")
	
	if len(parts) > 1 && parts[0] == "file_operations" {
		return em.applyFileSecurityOverride(&security.FileOperations, parts[1], value)
	}
	
	switch field {
	case "max_file_size":
		if val, ok := em.convertToInt64(value); ok {
			security.MaxFileSize = val
		} else {
			return fmt.Errorf("max_file_size must be an int64")
		}
	case "enable_sandbox":
		if val, ok := value.(bool); ok {
			security.EnableSandbox = val
		} else {
			return fmt.Errorf("enable_sandbox must be a boolean")
		}
	default:
		return fmt.Errorf("unknown security field: %s", field)
	}
	return nil
}

func (em *EnvironmentManager) applyFileSecurityOverride(fileSec *FileSecurityConfig, field string, value interface{}) error {
	switch field {
	case "allow_absolute_paths":
		if val, ok := value.(bool); ok {
			fileSec.AllowAbsolutePaths = val
		} else {
			return fmt.Errorf("allow_absolute_paths must be a boolean")
		}
	case "allow_symlinks":
		if val, ok := value.(bool); ok {
			fileSec.AllowSymlinks = val
		} else {
			return fmt.Errorf("allow_symlinks must be a boolean")
		}
	default:
		return fmt.Errorf("unknown file security field: %s", field)
	}
	return nil
}

func (em *EnvironmentManager) applyUIOverride(ui *UIConfig, field string, value interface{}) error {
	switch field {
	case "theme":
		if str, ok := value.(string); ok {
			ui.Theme = str
		} else {
			return fmt.Errorf("theme must be a string")
		}
	case "default_expanded":
		if val, ok := value.(bool); ok {
			ui.DefaultExpanded = val
		} else {
			return fmt.Errorf("default_expanded must be a boolean")
		}
	case "show_timestamps":
		if val, ok := value.(bool); ok {
			ui.ShowTimestamps = val
		} else {
			return fmt.Errorf("show_timestamps must be a boolean")
		}
	case "auto_scroll":
		if val, ok := value.(bool); ok {
			ui.AutoScroll = val
		} else {
			return fmt.Errorf("auto_scroll must be a boolean")
		}
	case "max_history_size":
		if val, ok := em.convertToInt(value); ok {
			ui.MaxHistorySize = val
		} else {
			return fmt.Errorf("max_history_size must be an integer")
		}
	default:
		return fmt.Errorf("unknown UI field: %s", field)
	}
	return nil
}

func (em *EnvironmentManager) applyToolsOverride(tools *ToolsConfig, field string, value interface{}) error {
	switch field {
	case "default_timeout":
		if dur, ok := em.convertToDuration(value); ok {
			tools.DefaultTimeout = dur
		} else {
			return fmt.Errorf("default_timeout must be a duration")
		}
	case "max_concurrent":
		if val, ok := em.convertToInt(value); ok {
			tools.MaxConcurrent = val
		} else {
			return fmt.Errorf("max_concurrent must be an integer")
		}
	case "enable_builtins":
		if val, ok := value.(bool); ok {
			tools.EnableBuiltins = val
		} else {
			return fmt.Errorf("enable_builtins must be a boolean")
		}
	default:
		return fmt.Errorf("unknown tools field: %s", field)
	}
	return nil
}

// Type conversion helpers

func (em *EnvironmentManager) convertToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
	}
	return 0, false
}

func (em *EnvironmentManager) convertToInt64(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float32:
		return int64(v), true
	case float64:
		return int64(v), true
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}

func (em *EnvironmentManager) convertToDuration(value interface{}) (time.Duration, bool) {
	switch v := value.(type) {
	case string:
		if d, err := time.ParseDuration(v); err == nil {
			return d, true
		}
	case int:
		return time.Duration(v) * time.Second, true
	case int64:
		return time.Duration(v) * time.Second, true
	case float64:
		return time.Duration(v) * time.Second, true
	}
	return 0, false
}

// File operations

func (em *EnvironmentManager) loadEnvironmentFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read environment file: %w", err)
	}
	
	var envConfig EnvironmentConfig
	if err := yaml.Unmarshal(data, &envConfig); err != nil {
		return fmt.Errorf("failed to unmarshal environment config: %w", err)
	}
	
	em.environments[envConfig.Environment] = &envConfig
	return nil
}

// SaveEnvironmentConfig saves an environment configuration to disk
func (em *EnvironmentManager) SaveEnvironmentConfig(env Environment) error {
	envConfig, exists := em.environments[env]
	if !exists {
		return fmt.Errorf("environment %s not found", env)
	}
	
	// Don't save built-in environments
	if em.isBuiltinEnvironment(env) {
		return nil
	}
	
	filename := fmt.Sprintf("%s.yaml", env)
	path := filepath.Join(em.configDir, filename)
	
	data, err := yaml.Marshal(envConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal environment config: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write environment config: %w", err)
	}
	
	return nil
}

// isBuiltinEnvironment checks if an environment is built-in
func (em *EnvironmentManager) isBuiltinEnvironment(env Environment) bool {
	builtinEnvs := []Environment{
		EnvDevelopment, EnvTesting, EnvStaging, EnvProduction, EnvLocal,
	}
	
	for _, builtin := range builtinEnvs {
		if builtin == env {
			return true
		}
	}
	
	return false
}

// GetEnvironmentConfig returns the configuration for a specific environment
func (em *EnvironmentManager) GetEnvironmentConfig(env Environment) (*EnvironmentConfig, error) {
	if envConfig, exists := em.environments[env]; exists {
		return envConfig, nil
	}
	
	return nil, fmt.Errorf("environment %s not found", env)
}

// ListEnvironments returns all available environments
func (em *EnvironmentManager) ListEnvironments() []Environment {
	envs := make([]Environment, 0, len(em.environments))
	for env := range em.environments {
		envs = append(envs, env)
	}
	return envs
}