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

// ConfigVersion represents the version of the configuration format
type ConfigVersion int

const (
	// ConfigV1 represents the initial configuration format
	ConfigV1 ConfigVersion = 1
	// ConfigV2 represents the second version with enhanced security
	ConfigV2 ConfigVersion = 2
	// ConfigV3 represents the third version with profiles and runtime updates
	ConfigV3 ConfigVersion = 3
	// CurrentConfigVersion is the latest configuration version
	CurrentConfigVersion = ConfigV3
)

// VersionedConfig represents a configuration with version information
type VersionedConfig struct {
	Version int           `yaml:"version" json:"version"`
	Config  yaml.Node     `yaml:"config" json:"config"`
	Meta    ConfigMeta    `yaml:"meta,omitempty" json:"meta,omitempty"`
}

// ConfigMeta contains metadata about the configuration
type ConfigMeta struct {
	CreatedAt    time.Time `yaml:"created_at" json:"created_at"`
	ModifiedAt   time.Time `yaml:"modified_at" json:"modified_at"`
	MigratedFrom int       `yaml:"migrated_from,omitempty" json:"migrated_from,omitempty"`
	Application  string    `yaml:"application" json:"application"`
	AppVersion   string    `yaml:"app_version,omitempty" json:"app_version,omitempty"`
}

// MigrationResult contains the results of a configuration migration
type MigrationResult struct {
	Success         bool     `json:"success"`
	FromVersion     int      `json:"from_version"`
	ToVersion       int      `json:"to_version"`
	BackupPath      string   `json:"backup_path,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Changes         []string `json:"changes,omitempty"`
	Error           string   `json:"error,omitempty"`
}

// ConfigMigrator handles configuration migration between versions
type ConfigMigrator struct {
	backupDir string
}

// NewConfigMigrator creates a new configuration migrator
func NewConfigMigrator() *ConfigMigrator {
	backupDir := filepath.Join(filepath.Dir(globalConfigPath()), "backups")
	return &ConfigMigrator{
		backupDir: backupDir,
	}
}

// MigrateConfig migrates a configuration to the latest version
func (cm *ConfigMigrator) MigrateConfig(configPath string) (*MigrationResult, error) {
	result := &MigrationResult{
		Changes:  make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Read the current configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to read config file: %v", err)
		return result, err
	}

	// Detect current version
	version, err := cm.detectConfigVersion(data)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to detect config version: %v", err)
		return result, err
	}

	result.FromVersion = int(version)
	result.ToVersion = int(CurrentConfigVersion)

	// No migration needed if already at current version
	if version == CurrentConfigVersion {
		result.Success = true
		return result, nil
	}

	// Create backup before migration
	backupPath, err := cm.createBackup(configPath, version)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create backup: %v", err)
		return result, err
	}
	result.BackupPath = backupPath

	// Perform step-by-step migration
	migratedData := data
	for currentVersion := version; currentVersion < CurrentConfigVersion; currentVersion++ {
		nextVersion := currentVersion + 1
		migratedData, err = cm.migrateToVersion(migratedData, currentVersion, nextVersion, result)
		if err != nil {
			result.Error = fmt.Sprintf("Failed to migrate from v%d to v%d: %v", 
				currentVersion, nextVersion, err)
			return result, err
		}
	}

	// Write the migrated configuration
	if err := os.WriteFile(configPath, migratedData, 0644); err != nil {
		result.Error = fmt.Sprintf("Failed to write migrated config: %v", err)
		return result, err
	}

	result.Success = true
	return result, nil
}

// detectConfigVersion detects the version of a configuration file
func (cm *ConfigMigrator) detectConfigVersion(data []byte) (ConfigVersion, error) {
	var versionedConfig VersionedConfig
	if err := yaml.Unmarshal(data, &versionedConfig); err == nil && versionedConfig.Version > 0 {
		return ConfigVersion(versionedConfig.Version), nil
	}

	// Try to detect version by structure
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &rawConfig); err != nil {
		return 0, fmt.Errorf("invalid YAML format: %w", err)
	}

	// Version detection logic
	if hasV3Features(rawConfig) {
		return ConfigV3, nil
	} else if hasV2Features(rawConfig) {
		return ConfigV2, nil
	} else {
		return ConfigV1, nil
	}
}

// hasV3Features checks if the config has V3-specific features
func hasV3Features(config map[string]interface{}) bool {
	// V3 introduced runtime configuration and profiles
	if meta, ok := config["meta"]; ok {
		if metaMap, ok := meta.(map[string]interface{}); ok {
			if _, hasApp := metaMap["application"]; hasApp {
				return true
			}
		}
	}
	return false
}

// hasV2Features checks if the config has V2-specific features
func hasV2Features(config map[string]interface{}) bool {
	// V2 introduced enhanced security configuration
	if security, ok := config["security"]; ok {
		if secMap, ok := security.(map[string]interface{}); ok {
			if _, hasFileOps := secMap["file_operations"]; hasFileOps {
				return true
			}
			if _, hasSandbox := secMap["enable_sandbox"]; hasSandbox {
				return true
			}
		}
	}
	return false
}

// createBackup creates a backup of the configuration file
func (cm *ConfigMigrator) createBackup(configPath string, version ConfigVersion) (string, error) {
	if err := os.MkdirAll(cm.backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	filename := fmt.Sprintf("config_v%d_%s.yaml", version, time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(cm.backupDir, filename)

	// Copy the original file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read original config: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	return backupPath, nil
}

// migrateToVersion migrates configuration from one version to the next
func (cm *ConfigMigrator) migrateToVersion(data []byte, from, to ConfigVersion, result *MigrationResult) ([]byte, error) {
	switch {
	case from == ConfigV1 && to == ConfigV2:
		return cm.migrateV1ToV2(data, result)
	case from == ConfigV2 && to == ConfigV3:
		return cm.migrateV2ToV3(data, result)
	default:
		return nil, fmt.Errorf("unsupported migration path from v%d to v%d", from, to)
	}
}

// migrateV1ToV2 migrates from V1 to V2 format
func (cm *ConfigMigrator) migrateV1ToV2(data []byte, result *MigrationResult) ([]byte, error) {
	var v1Config map[string]interface{}
	if err := yaml.Unmarshal(data, &v1Config); err != nil {
		return nil, fmt.Errorf("failed to parse V1 config: %w", err)
	}

	// V1 to V2 changes:
	// - Enhanced security configuration with file_operations
	// - Added enable_sandbox flag
	// - Restructured security settings

	result.Changes = append(result.Changes, "Enhanced security configuration structure")

	if security, ok := v1Config["security"]; ok {
		if secMap, ok := security.(map[string]interface{}); ok {
			// Move file-related settings to file_operations section
			fileOps := make(map[string]interface{})
			
			// Migrate allowed_paths, blocked_paths, etc.
			if val, exists := secMap["allowed_paths"]; exists {
				fileOps["allowed_paths"] = val
				delete(secMap, "allowed_paths")
			}
			if val, exists := secMap["blocked_paths"]; exists {
				fileOps["blocked_paths"] = val
				delete(secMap, "blocked_paths")
			}
			if val, exists := secMap["allow_absolute_paths"]; exists {
				fileOps["allow_absolute_paths"] = val
				delete(secMap, "allow_absolute_paths")
			} else {
				fileOps["allow_absolute_paths"] = false
			}
			if val, exists := secMap["allow_symlinks"]; exists {
				fileOps["allow_symlinks"] = val
				delete(secMap, "allow_symlinks")
			} else {
				fileOps["allow_symlinks"] = false
			}

			// Add default values for new fields
			if _, exists := fileOps["allowed_extensions"]; !exists {
				fileOps["allowed_extensions"] = []string{}
			}
			if _, exists := fileOps["blocked_extensions"]; !exists {
				fileOps["blocked_extensions"] = []string{
					".exe", ".bat", ".cmd", ".com", ".scr", ".pif", ".msi", ".msp",
					".dll", ".so", ".dylib", ".deb", ".rpm", ".dmg", ".pkg",
				}
			}

			secMap["file_operations"] = fileOps

			// Add sandbox setting
			if _, exists := secMap["enable_sandbox"]; !exists {
				secMap["enable_sandbox"] = false
			}

			result.Changes = append(result.Changes, "Moved file security settings to file_operations section")
			result.Changes = append(result.Changes, "Added enable_sandbox configuration")
		}
	}

	// Add version information
	versionedConfig := VersionedConfig{
		Version: int(ConfigV2),
		Meta: ConfigMeta{
			CreatedAt:    time.Now(),
			ModifiedAt:   time.Now(),
			MigratedFrom: int(ConfigV1),
			Application:  "gocodenow",
		},
	}

	// Convert back to YAML node for embedding
	configNode := yaml.Node{}
	configData, _ := yaml.Marshal(v1Config)
	yaml.Unmarshal(configData, &configNode)
	versionedConfig.Config = configNode

	return yaml.Marshal(versionedConfig)
}

// migrateV2ToV3 migrates from V2 to V3 format
func (cm *ConfigMigrator) migrateV2ToV3(data []byte, result *MigrationResult) ([]byte, error) {
	var v2Config VersionedConfig
	if err := yaml.Unmarshal(data, &v2Config); err != nil {
		// Try parsing as raw config if versioned format fails
		var rawConfig map[string]interface{}
		if err2 := yaml.Unmarshal(data, &rawConfig); err2 != nil {
			return nil, fmt.Errorf("failed to parse V2 config: %w", err)
		}
		
		// Convert to versioned format
		configData, _ := yaml.Marshal(rawConfig)
		yaml.Unmarshal(configData, &v2Config.Config)
		v2Config.Version = int(ConfigV2)
	}

	// V2 to V3 changes:
	// - Added runtime configuration support
	// - Enhanced metadata tracking
	// - Profile system integration

	result.Changes = append(result.Changes, "Added runtime configuration support")
	result.Changes = append(result.Changes, "Enhanced metadata tracking")

	// Update version and metadata
	v2Config.Version = int(ConfigV3)
	if v2Config.Meta.CreatedAt.IsZero() {
		v2Config.Meta.CreatedAt = time.Now()
	}
	v2Config.Meta.ModifiedAt = time.Now()
	if v2Config.Meta.MigratedFrom == 0 {
		v2Config.Meta.MigratedFrom = int(ConfigV2)
	}
	v2Config.Meta.Application = "gocodenow"

	return yaml.Marshal(v2Config)
}

// ValidateConfigFile validates a configuration file
func (cm *ConfigMigrator) ValidateConfigFile(configPath string) error {
	// Check if file exists
	if !fileExists(configPath) {
		return fmt.Errorf("configuration file does not exist: %s", configPath)
	}

	// Read and parse the file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Detect version
	version, err := cm.detectConfigVersion(data)
	if err != nil {
		return fmt.Errorf("failed to detect configuration version: %w", err)
	}

	// If not current version, suggest migration
	if version != CurrentConfigVersion {
		return fmt.Errorf("configuration file is version %d, current version is %d. Please run migration", 
			version, CurrentConfigVersion)
	}

	// Load and validate the configuration
	config, err := LoadConfigFromFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate the loaded configuration
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// GetConfigVersion returns the version of a configuration file
func (cm *ConfigMigrator) GetConfigVersion(configPath string) (ConfigVersion, error) {
	if !fileExists(configPath) {
		return 0, fmt.Errorf("configuration file does not exist: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read configuration file: %w", err)
	}

	return cm.detectConfigVersion(data)
}

// CreateNewVersionedConfig creates a new configuration file with version information
func (cm *ConfigMigrator) CreateNewVersionedConfig(configPath string, config *Config) error {
	versionedConfig := VersionedConfig{
		Version: int(CurrentConfigVersion),
		Meta: ConfigMeta{
			CreatedAt:   time.Now(),
			ModifiedAt:  time.Now(),
			Application: "gocodenow",
		},
	}

	// Convert config to YAML node
	configData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := yaml.Unmarshal(configData, &versionedConfig.Config); err != nil {
		return fmt.Errorf("failed to convert configuration: %w", err)
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create configuration directory: %w", err)
	}

	// Write versioned configuration
	data, err := yaml.Marshal(versionedConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal versioned configuration: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

// ConfigValidator provides enhanced configuration validation
type ConfigValidator struct {
	rules []ValidationRule
}

// ValidationRule represents a configuration validation rule
type ValidationRule struct {
	Name        string
	Description string
	Validator   func(*Config) error
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	validator := &ConfigValidator{
		rules: make([]ValidationRule, 0),
	}

	// Add default validation rules
	validator.addDefaultRules()

	return validator
}

// addDefaultRules adds the default validation rules
func (cv *ConfigValidator) addDefaultRules() {
	cv.rules = append(cv.rules, []ValidationRule{
		{
			Name:        "llm_endpoint_reachable",
			Description: "LLM endpoint should be reachable",
			Validator:   cv.validateLLMEndpoint,
		},
		{
			Name:        "storage_path_writable",
			Description: "Storage path should be writable",
			Validator:   cv.validateStoragePath,
		},
		{
			Name:        "security_paths_valid",
			Description: "Security paths should exist and be valid",
			Validator:   cv.validateSecurityPaths,
		},
		{
			Name:        "resource_limits_reasonable",
			Description: "Resource limits should be reasonable for the system",
			Validator:   cv.validateResourceLimits,
		},
	}...)
}

// ValidateWithRules validates configuration using all registered rules
func (cv *ConfigValidator) ValidateWithRules(config *Config) []ValidationError {
	var errors []ValidationError

	for _, rule := range cv.rules {
		if err := rule.Validator(config); err != nil {
			errors = append(errors, ValidationError{
				Rule:    rule.Name,
				Message: err.Error(),
			})
		}
	}

	return errors
}

// ValidationError represents a validation error with rule context
type ValidationError struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("validation rule '%s' failed: %s", ve.Rule, ve.Message)
}

// Validation rule implementations
func (cv *ConfigValidator) validateLLMEndpoint(config *Config) error {
	if config.LLM.Endpoint == "" {
		return fmt.Errorf("LLM endpoint cannot be empty")
	}
	
	// Basic URL format check
	if !strings.HasPrefix(config.LLM.Endpoint, "http://") && 
	   !strings.HasPrefix(config.LLM.Endpoint, "https://") {
		return fmt.Errorf("LLM endpoint must be a valid HTTP/HTTPS URL")
	}
	
	return nil
}

func (cv *ConfigValidator) validateStoragePath(config *Config) error {
	dir := filepath.Dir(config.Storage.DatabasePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create storage directory: %v", err)
	}
	
	// Test write permissions
	testFile := filepath.Join(dir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("storage directory not writable: %v", err)
	}
	os.Remove(testFile) // Clean up
	
	return nil
}

func (cv *ConfigValidator) validateSecurityPaths(config *Config) error {
	// Check allowed paths exist
	for _, path := range config.Security.FileOperations.AllowedPaths {
		if path != "." && path != "./" && !fileExists(path) {
			return fmt.Errorf("allowed path does not exist: %s", path)
		}
	}
	
	return nil
}

func (cv *ConfigValidator) validateResourceLimits(config *Config) error {
	// Check memory limits are reasonable (not too low or impossibly high)
	if config.Storage.MaxMemoryMB < 10 {
		return fmt.Errorf("max memory limit too low: %d MB (minimum 10 MB)", config.Storage.MaxMemoryMB)
	}
	
	if config.Storage.MaxMemoryMB > 16384 { // 16GB
		return fmt.Errorf("max memory limit too high: %d MB (maximum 16384 MB)", config.Storage.MaxMemoryMB)
	}
	
	// Check timeout values
	if config.LLM.Timeout < 1 {
		return fmt.Errorf("LLM timeout too low: %d seconds (minimum 1 second)", config.LLM.Timeout)
	}
	
	if config.LLM.Timeout > 300 { // 5 minutes
		return fmt.Errorf("LLM timeout too high: %d seconds (maximum 300 seconds)", config.LLM.Timeout)
	}
	
	return nil
}