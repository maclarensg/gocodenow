package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// ConfigChangeCallback is a function called when configuration changes
type ConfigChangeCallback func(oldConfig, newConfig *Config) error

// RuntimeConfigManager handles runtime configuration updates without restart
type RuntimeConfigManager struct {
	config    *Config
	mutex     sync.RWMutex
	callbacks []ConfigChangeCallback
	watcher   *fsnotify.Watcher
	stopCh    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewRuntimeConfigManager creates a new runtime configuration manager
func NewRuntimeConfigManager(initialConfig *Config) *RuntimeConfigManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &RuntimeConfigManager{
		config:    initialConfig,
		callbacks: make([]ConfigChangeCallback, 0),
		stopCh:    make(chan struct{}),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// GetConfig returns the current configuration (thread-safe)
func (rcm *RuntimeConfigManager) GetConfig() *Config {
	rcm.mutex.RLock()
	defer rcm.mutex.RUnlock()
	
	// Return a deep copy to prevent external modifications
	return rcm.copyConfig(rcm.config)
}

// UpdateConfig updates the configuration and notifies callbacks
func (rcm *RuntimeConfigManager) UpdateConfig(newConfig *Config) error {
	// Validate the new configuration first
	if err := ValidateConfig(newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	rcm.mutex.Lock()
	oldConfig := rcm.copyConfig(rcm.config)
	rcm.config = rcm.copyConfig(newConfig)
	rcm.mutex.Unlock()

	// Notify all callbacks about the configuration change
	for _, callback := range rcm.callbacks {
		if err := callback(oldConfig, newConfig); err != nil {
			// Log the error but don't revert the configuration
			log.Printf("Configuration change callback failed: %v", err)
		}
	}

	return nil
}

// RegisterCallback registers a function to be called when configuration changes
func (rcm *RuntimeConfigManager) RegisterCallback(callback ConfigChangeCallback) {
	rcm.callbacks = append(rcm.callbacks, callback)
}

// StartFileWatching starts watching configuration files for changes
func (rcm *RuntimeConfigManager) StartFileWatching() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	
	rcm.watcher = watcher

	// Watch global and local config files
	globalPath := globalConfigPath()
	localPath := localConfigPath()
	
	// Add global config file to watcher if it exists
	if fileExists(globalPath) {
		if err := watcher.Add(globalPath); err != nil {
			log.Printf("Failed to watch global config file %s: %v", globalPath, err)
		}
	}
	
	// Add local config file to watcher if it exists
	if fileExists(localPath) {
		if err := watcher.Add(localPath); err != nil {
			log.Printf("Failed to watch local config file %s: %v", localPath, err)
		}
	}

	go rcm.watchFiles()
	
	return nil
}

// watchFiles monitors file system events for configuration changes
func (rcm *RuntimeConfigManager) watchFiles() {
	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}
	
	for {
		select {
		case <-rcm.ctx.Done():
			return
		case event, ok := <-rcm.watcher.Events:
			if !ok {
				return
			}
			
			// Only process write events
			if event.Op&fsnotify.Write == fsnotify.Write {
				// Debounce file events to avoid multiple rapid reloads
				debounceTimer.Reset(500 * time.Millisecond)
			}
			
		case err, ok := <-rcm.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
			
		case <-debounceTimer.C:
			// Reload configuration from files
			if err := rcm.reloadFromFiles(); err != nil {
				log.Printf("Failed to reload configuration: %v", err)
			}
		}
	}
}

// reloadFromFiles reloads configuration from files
func (rcm *RuntimeConfigManager) reloadFromFiles() error {
	newConfig, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	return rcm.UpdateConfig(newConfig)
}

// Stop stops the runtime configuration manager
func (rcm *RuntimeConfigManager) Stop() error {
	if rcm.cancel != nil {
		rcm.cancel()
	}
	
	if rcm.watcher != nil {
		return rcm.watcher.Close()
	}
	
	return nil
}

// SaveConfig saves the current configuration to the local config file
func (rcm *RuntimeConfigManager) SaveConfig() error {
	rcm.mutex.RLock()
	config := rcm.copyConfig(rcm.config)
	rcm.mutex.RUnlock()
	
	return SaveConfigToFile(config, localConfigPath())
}

// SaveConfigToFile saves configuration to a specific file
func SaveConfigToFile(config *Config, path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Marshal configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}
	
	return nil
}

// copyConfig creates a deep copy of the configuration
func (rcm *RuntimeConfigManager) copyConfig(config *Config) *Config {
	if config == nil {
		return nil
	}
	
	// Create a new config instance
	newConfig := &Config{
		LLM:      config.LLM,
		Storage:  config.Storage,
		Security: copySecurityConfig(config.Security),
		UI:       config.UI,
		Tools:    copyToolsConfig(config.Tools),
	}
	
	return newConfig
}

// copySecurityConfig creates a deep copy of security config
func copySecurityConfig(security SecurityConfig) SecurityConfig {
	return SecurityConfig{
		FileOperations: FileSecurityConfig{
			AllowedPaths:       copyStringSlice(security.FileOperations.AllowedPaths),
			BlockedPaths:       copyStringSlice(security.FileOperations.BlockedPaths),
			AllowAbsolutePaths: security.FileOperations.AllowAbsolutePaths,
			AllowSymlinks:      security.FileOperations.AllowSymlinks,
			AllowedExtensions:  copyStringSlice(security.FileOperations.AllowedExtensions),
			BlockedExtensions:  copyStringSlice(security.FileOperations.BlockedExtensions),
		},
		MaxFileSize:   security.MaxFileSize,
		AllowedHosts:  copyStringSlice(security.AllowedHosts),
		EnableSandbox: security.EnableSandbox,
	}
}

// copyToolsConfig creates a deep copy of tools config
func copyToolsConfig(tools ToolsConfig) ToolsConfig {
	return ToolsConfig{
		DefaultTimeout:   tools.DefaultTimeout,
		MaxConcurrent:    tools.MaxConcurrent,
		EnableBuiltins:   tools.EnableBuiltins,
		CustomToolsPaths: copyStringSlice(tools.CustomToolsPaths),
	}
}

// copyStringSlice creates a copy of a string slice
func copyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// ConfigField represents a configuration field that can be updated
type ConfigField struct {
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// UpdateConfigField updates a specific configuration field
func (rcm *RuntimeConfigManager) UpdateConfigField(field ConfigField) error {
	rcm.mutex.Lock()
	defer rcm.mutex.Unlock()
	
	config := rcm.copyConfig(rcm.config)
	
	if err := setConfigField(config, field.Path, field.Value); err != nil {
		return fmt.Errorf("failed to update config field %s: %w", field.Path, err)
	}
	
	// Validate the updated configuration
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed after field update: %w", err)
	}
	
	oldConfig := rcm.config
	rcm.config = config
	
	// Notify callbacks
	for _, callback := range rcm.callbacks {
		if err := callback(oldConfig, config); err != nil {
			log.Printf("Configuration change callback failed: %v", err)
		}
	}
	
	return nil
}

// setConfigField sets a configuration field using a dot-separated path
func setConfigField(config *Config, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("invalid field path: %s", path)
	}
	
	switch parts[0] {
	case "llm":
		return setLLMField(&config.LLM, parts[1:], value)
	case "storage":
		return setStorageField(&config.Storage, parts[1:], value)
	case "security":
		return setSecurityField(&config.Security, parts[1:], value)
	case "ui":
		return setUIField(&config.UI, parts[1:], value)
	case "tools":
		return setToolsField(&config.Tools, parts[1:], value)
	default:
		return fmt.Errorf("unknown config section: %s", parts[0])
	}
}

// Helper functions to set specific config fields
func setLLMField(llm *LLMConfig, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty LLM field path")
	}
	
	switch path[0] {
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
		if val, ok := value.(int); ok {
			llm.Timeout = val
		} else {
			return fmt.Errorf("timeout must be an integer")
		}
	case "max_retries":
		if val, ok := value.(int); ok {
			llm.MaxRetries = val
		} else {
			return fmt.Errorf("max_retries must be an integer")
		}
	case "rate_limit_rpm":
		if val, ok := value.(int); ok {
			llm.RateLimitRPM = val
		} else {
			return fmt.Errorf("rate_limit_rpm must be an integer")
		}
	default:
		return fmt.Errorf("unknown LLM field: %s", path[0])
	}
	
	return nil
}

func setStorageField(storage *StorageConfig, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty storage field path")
	}
	
	switch path[0] {
	case "database_path":
		if str, ok := value.(string); ok {
			storage.DatabasePath = str
		} else {
			return fmt.Errorf("database_path must be a string")
		}
	case "cache_size":
		if val, ok := value.(int); ok {
			storage.CacheSize = val
		} else {
			return fmt.Errorf("cache_size must be an integer")
		}
	case "max_memory_mb":
		if val, ok := value.(int); ok {
			storage.MaxMemoryMB = val
		} else {
			return fmt.Errorf("max_memory_mb must be an integer")
		}
	case "enable_compression":
		if val, ok := value.(bool); ok {
			storage.EnableCompression = val
		} else {
			return fmt.Errorf("enable_compression must be a boolean")
		}
	default:
		return fmt.Errorf("unknown storage field: %s", path[0])
	}
	
	return nil
}

func setSecurityField(security *SecurityConfig, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty security field path")
	}
	
	switch path[0] {
	case "max_file_size":
		if val, ok := value.(int64); ok {
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
		return fmt.Errorf("unknown security field: %s", path[0])
	}
	
	return nil
}

func setUIField(ui *UIConfig, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty UI field path")
	}
	
	switch path[0] {
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
		if val, ok := value.(int); ok {
			ui.MaxHistorySize = val
		} else {
			return fmt.Errorf("max_history_size must be an integer")
		}
	default:
		return fmt.Errorf("unknown UI field: %s", path[0])
	}
	
	return nil
}

func setToolsField(tools *ToolsConfig, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty tools field path")
	}
	
	switch path[0] {
	case "max_concurrent":
		if val, ok := value.(int); ok {
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
		return fmt.Errorf("unknown tools field: %s", path[0])
	}
	
	return nil
}