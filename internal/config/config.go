package config

import (
	"time"
)

// Config represents the complete application configuration
type Config struct {
	// LLM API Configuration
	LLM LLMConfig `yaml:"llm" json:"llm"`
	
	// Storage Configuration  
	Storage StorageConfig `yaml:"storage" json:"storage"`
	
	// Security Configuration
	Security SecurityConfig `yaml:"security" json:"security"`
	
	// UI Configuration
	UI UIConfig `yaml:"ui" json:"ui"`
	
	// Tool execution configuration
	Tools ToolsConfig `yaml:"tools" json:"tools"`
}

// LLMConfig holds LLM API related configuration
type LLMConfig struct {
	Endpoint     string `yaml:"endpoint" json:"endpoint"`
	Token        string `yaml:"token" json:"token"`
	Model        string `yaml:"model" json:"model"`
	Timeout      int    `yaml:"timeout" json:"timeout"`
	MaxRetries   int    `yaml:"max_retries" json:"max_retries"`
	RateLimitRPM int    `yaml:"rate_limit_rpm" json:"rate_limit_rpm"`
}

// StorageConfig holds storage and database related configuration
type StorageConfig struct {
	DatabasePath      string        `yaml:"database_path" json:"database_path"`
	CacheSize         int           `yaml:"cache_size" json:"cache_size"`
	AutoSaveFreq      time.Duration `yaml:"auto_save_freq" json:"auto_save_freq"`
	MaxMemoryMB       int           `yaml:"max_memory_mb" json:"max_memory_mb"`
	BackupInterval    time.Duration `yaml:"backup_interval" json:"backup_interval"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`
	EnableCompression bool          `yaml:"enable_compression" json:"enable_compression"`
}

// SecurityConfig holds security and safety related configuration
type SecurityConfig struct {
	FileOperations FileSecurityConfig `yaml:"file_operations" json:"file_operations"`
	MaxFileSize    int64              `yaml:"max_file_size" json:"max_file_size"`
	AllowedHosts   []string           `yaml:"allowed_hosts" json:"allowed_hosts"`
	EnableSandbox  bool               `yaml:"enable_sandbox" json:"enable_sandbox"`
}

// FileSecurityConfig holds file operation security settings
type FileSecurityConfig struct {
	AllowedPaths       []string `yaml:"allowed_paths" json:"allowed_paths"`
	BlockedPaths       []string `yaml:"blocked_paths" json:"blocked_paths"`
	AllowAbsolutePaths bool     `yaml:"allow_absolute_paths" json:"allow_absolute_paths"`
	AllowSymlinks      bool     `yaml:"allow_symlinks" json:"allow_symlinks"`
	AllowedExtensions  []string `yaml:"allowed_extensions" json:"allowed_extensions"`
	BlockedExtensions  []string `yaml:"blocked_extensions" json:"blocked_extensions"`
}

// UIConfig holds user interface related configuration
type UIConfig struct {
	Theme           string `yaml:"theme" json:"theme"`
	DefaultExpanded bool   `yaml:"default_expanded" json:"default_expanded"`
	ShowTimestamps  bool   `yaml:"show_timestamps" json:"show_timestamps"`
	AutoScroll      bool   `yaml:"auto_scroll" json:"auto_scroll"`
	MaxHistorySize  int    `yaml:"max_history_size" json:"max_history_size"`
}

// ToolsConfig holds tool execution configuration
type ToolsConfig struct {
	DefaultTimeout   time.Duration `yaml:"default_timeout" json:"default_timeout"`
	MaxConcurrent    int           `yaml:"max_concurrent" json:"max_concurrent"`
	EnableBuiltins   bool          `yaml:"enable_builtins" json:"enable_builtins"`
	CustomToolsPaths []string      `yaml:"custom_tools_paths" json:"custom_tools_paths"`
}