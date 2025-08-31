package config

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigTemplate represents a configuration template
type ConfigTemplate struct {
	Name        string                 `yaml:"name" json:"name"`
	Description string                 `yaml:"description" json:"description"`
	Version     string                 `yaml:"version" json:"version"`
	Author      string                 `yaml:"author,omitempty" json:"author,omitempty"`
	Tags        []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
	Variables   map[string]TemplateVar `yaml:"variables,omitempty" json:"variables,omitempty"`
	Template    string                 `yaml:"template" json:"template"`
	Created     time.Time              `yaml:"created" json:"created"`
	Modified    time.Time              `yaml:"modified" json:"modified"`
	Checksum    string                 `yaml:"checksum,omitempty" json:"checksum,omitempty"`
}

// TemplateVar represents a template variable with metadata
type TemplateVar struct {
	Name         string      `yaml:"name" json:"name"`
	Description  string      `yaml:"description,omitempty" json:"description,omitempty"`
	Type         string      `yaml:"type" json:"type"` // string, int, bool, duration
	Required     bool        `yaml:"required" json:"required"`
	Default      interface{} `yaml:"default,omitempty" json:"default,omitempty"`
	Options      []string    `yaml:"options,omitempty" json:"options,omitempty"`
	Validation   string      `yaml:"validation,omitempty" json:"validation,omitempty"`
	Example      string      `yaml:"example,omitempty" json:"example,omitempty"`
}

// SharedConfig represents a shareable configuration
type SharedConfig struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Version     string            `yaml:"version" json:"version"`
	Author      string            `yaml:"author,omitempty" json:"author,omitempty"`
	Tags        []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Config      Config            `yaml:"config" json:"config"`
	Metadata    SharedConfigMeta  `yaml:"metadata" json:"metadata"`
	Checksum    string            `yaml:"checksum,omitempty" json:"checksum,omitempty"`
}

// SharedConfigMeta contains metadata about shared configurations
type SharedConfigMeta struct {
	Created      time.Time `yaml:"created" json:"created"`
	Modified     time.Time `yaml:"modified" json:"modified"`
	Downloads    int       `yaml:"downloads,omitempty" json:"downloads,omitempty"`
	Rating       float64   `yaml:"rating,omitempty" json:"rating,omitempty"`
	License      string    `yaml:"license,omitempty" json:"license,omitempty"`
	Repository   string    `yaml:"repository,omitempty" json:"repository,omitempty"`
	Issues       string    `yaml:"issues,omitempty" json:"issues,omitempty"`
	Dependencies []string  `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
}

// TemplateManager manages configuration templates and sharing
type TemplateManager struct {
	templatesDir   string
	sharedDir      string
	templates      map[string]*ConfigTemplate
	sharedConfigs  map[string]*SharedConfig
	registries     []string
}

// NewTemplateManager creates a new template manager
func NewTemplateManager() *TemplateManager {
	configDir := filepath.Dir(globalConfigPath())
	
	tm := &TemplateManager{
		templatesDir:  filepath.Join(configDir, "templates"),
		sharedDir:     filepath.Join(configDir, "shared"),
		templates:     make(map[string]*ConfigTemplate),
		sharedConfigs: make(map[string]*SharedConfig),
		registries: []string{
			"https://raw.githubusercontent.com/gocodenow/config-templates/main/registry.json",
		},
	}
	
	return tm
}

// LoadTemplates loads all available configuration templates
func (tm *TemplateManager) LoadTemplates() error {
	// Create templates directory if it doesn't exist
	if err := os.MkdirAll(tm.templatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}
	
	// Load built-in templates
	tm.loadBuiltinTemplates()
	
	// Load user templates from files
	entries, err := os.ReadDir(tm.templatesDir)
	if err != nil {
		return fmt.Errorf("failed to read templates directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			templatePath := filepath.Join(tm.templatesDir, entry.Name())
			if err := tm.loadTemplateFromFile(templatePath); err != nil {
				fmt.Printf("Warning: failed to load template %s: %v\n", entry.Name(), err)
			}
		}
	}
	
	return nil
}

// loadBuiltinTemplates creates built-in configuration templates
func (tm *TemplateManager) loadBuiltinTemplates() {
	templates := []*ConfigTemplate{
		tm.createMinimalTemplate(),
		tm.createDeveloperTemplate(),
		tm.createTeamTemplate(),
		tm.createProductionTemplate(),
		tm.createLocalAITemplate(),
		tm.createCloudTemplate(),
	}
	
	for _, tmpl := range templates {
		tm.templates[tmpl.Name] = tmpl
	}
}

// createMinimalTemplate creates a minimal configuration template
func (tm *TemplateManager) createMinimalTemplate() *ConfigTemplate {
	templateStr := `
version: 3
config:
  llm:
    endpoint: "{{.endpoint}}"
    model: "{{.model}}"
    timeout: {{.timeout}}
    max_retries: 1
    rate_limit_rpm: 30
  storage:
    cache_size: 20
    max_memory_mb: 50
    enable_compression: true
  security:
    enable_sandbox: {{.sandbox}}
  ui:
    theme: "{{.theme}}"
    max_history_size: 100
  tools:
    default_timeout: 15s
    max_concurrent: 2
`

	return &ConfigTemplate{
		Name:        "minimal",
		Description: "Minimal configuration for resource-constrained environments",
		Version:     "1.0.0",
		Author:      "gocodenow",
		Tags:        []string{"minimal", "lightweight", "constrained"},
		Variables: map[string]TemplateVar{
			"endpoint": {
				Name:        "endpoint",
				Description: "LLM API endpoint URL",
				Type:        "string",
				Required:    true,
				Default:     "http://localhost:1234/v1",
				Example:     "http://localhost:1234/v1",
			},
			"model": {
				Name:        "model",
				Description: "LLM model to use",
				Type:        "string",
				Required:    true,
				Default:     "llama3",
				Example:     "llama3",
			},
			"timeout": {
				Name:        "timeout",
				Description: "Request timeout in seconds",
				Type:        "int",
				Required:    false,
				Default:     15,
				Example:     "15",
			},
			"sandbox": {
				Name:        "sandbox",
				Description: "Enable sandboxed execution",
				Type:        "bool",
				Required:    false,
				Default:     true,
				Example:     "true",
			},
			"theme": {
				Name:        "theme",
				Description: "UI color theme",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Options:     []string{"default", "dark", "light", "minimal"},
				Example:     "default",
			},
		},
		Template: strings.TrimSpace(templateStr),
		Created:  time.Now(),
		Modified: time.Now(),
	}
}

// createDeveloperTemplate creates a developer-focused configuration template
func (tm *TemplateManager) createDeveloperTemplate() *ConfigTemplate {
	templateStr := `
version: 3
config:
  llm:
    endpoint: "{{.endpoint}}"
    model: "{{.model}}"
    timeout: {{.timeout}}
    max_retries: {{.retries}}
  storage:
    database_path: "{{.db_path}}"
    cache_size: {{.cache_size}}
    auto_save_freq: "{{.save_freq}}"
    enable_compression: false
  security:
    enable_sandbox: false
    file_operations:
      allow_absolute_paths: true
      allow_symlinks: true
  ui:
    theme: "{{.theme}}"
    default_expanded: true
    show_timestamps: true
    max_history_size: {{.history_size}}
  tools:
    default_timeout: "{{.tool_timeout}}"
    max_concurrent: {{.max_concurrent}}
    enable_builtins: true
`

	return &ConfigTemplate{
		Name:        "developer",
		Description: "Developer-optimized configuration with debugging features and relaxed security",
		Version:     "1.0.0",
		Author:      "gocodenow",
		Tags:        []string{"development", "debug", "flexible"},
		Variables: map[string]TemplateVar{
			"endpoint": {
				Name:        "endpoint",
				Description: "LLM API endpoint URL",
				Type:        "string",
				Required:    true,
				Default:     "http://localhost:1234/v1",
			},
			"model": {
				Name:        "model",
				Description: "LLM model for development",
				Type:        "string",
				Required:    true,
				Default:     "qwen2.5-coder-7b-instruct",
			},
			"timeout": {
				Name:        "timeout",
				Description: "Request timeout in seconds",
				Type:        "int",
				Required:    false,
				Default:     60,
			},
			"retries": {
				Name:        "retries",
				Description: "Maximum number of retries",
				Type:        "int",
				Required:    false,
				Default:     3,
			},
			"cache_size": {
				Name:        "cache_size",
				Description: "Number of conversations to cache",
				Type:        "int",
				Required:    false,
				Default:     100,
			},
			"save_freq": {
				Name:        "save_freq",
				Description: "Auto-save frequency",
				Type:        "duration",
				Required:    false,
				Default:     "1m",
			},
			"theme": {
				Name:        "theme",
				Description: "UI color theme",
				Type:        "string",
				Required:    false,
				Default:     "dark",
				Options:     []string{"default", "dark", "light", "monokai", "dracula"},
			},
			"history_size": {
				Name:        "history_size",
				Description: "Maximum conversation history size",
				Type:        "int",
				Required:    false,
				Default:     1000,
			},
			"tool_timeout": {
				Name:        "tool_timeout",
				Description: "Default tool execution timeout",
				Type:        "duration",
				Required:    false,
				Default:     "60s",
			},
			"max_concurrent": {
				Name:        "max_concurrent",
				Description: "Maximum concurrent tool executions",
				Type:        "int",
				Required:    false,
				Default:     10,
			},
			"db_path": {
				Name:        "db_path",
				Description: "Database file path",
				Type:        "string",
				Required:    false,
				Default:     "./dev_conversations.db",
			},
		},
		Template: strings.TrimSpace(templateStr),
		Created:  time.Now(),
		Modified: time.Now(),
	}
}

// createTeamTemplate creates a team collaboration template
func (tm *TemplateManager) createTeamTemplate() *ConfigTemplate {
	templateStr := `
version: 3
config:
  llm:
    endpoint: "{{.endpoint}}"
    token: "{{.token}}"
    model: "{{.model}}"
    timeout: {{.timeout}}
    max_retries: 3
    rate_limit_rpm: {{.rate_limit}}
  storage:
    cache_size: {{.cache_size}}
    auto_save_freq: "{{.save_freq}}"
    backup_interval: "{{.backup_interval}}"
    enable_compression: true
  security:
    enable_sandbox: true
    max_file_size: {{.max_file_size}}
    file_operations:
      allow_absolute_paths: false
      allow_symlinks: false
  ui:
    theme: "{{.theme}}"
    show_timestamps: true
    max_history_size: {{.history_size}}
  tools:
    default_timeout: "{{.tool_timeout}}"
    max_concurrent: {{.max_concurrent}}
    enable_builtins: true
`

	return &ConfigTemplate{
		Name:        "team",
		Description: "Team collaboration configuration with balanced security and functionality",
		Version:     "1.0.0",
		Author:      "gocodenow",
		Tags:        []string{"team", "collaboration", "balanced"},
		Variables: map[string]TemplateVar{
			"endpoint": {
				Name:        "endpoint",
				Description: "Shared LLM API endpoint",
				Type:        "string",
				Required:    true,
				Example:     "https://api.anthropic.com/v1",
			},
			"token": {
				Name:        "token",
				Description: "Team API token",
				Type:        "string",
				Required:    true,
				Example:     "sk-ant-...",
			},
			"model": {
				Name:        "model",
				Description: "Team's preferred model",
				Type:        "string",
				Required:    true,
				Default:     "claude-3-sonnet-20240229",
			},
			"timeout": {
				Name:        "timeout",
				Description: "Request timeout in seconds",
				Type:        "int",
				Required:    false,
				Default:     30,
			},
			"rate_limit": {
				Name:        "rate_limit",
				Description: "Rate limit in requests per minute",
				Type:        "int",
				Required:    false,
				Default:     100,
			},
			"cache_size": {
				Name:        "cache_size",
				Description: "Conversation cache size",
				Type:        "int",
				Required:    false,
				Default:     150,
			},
			"save_freq": {
				Name:        "save_freq",
				Description: "Auto-save frequency",
				Type:        "duration",
				Required:    false,
				Default:     "3m",
			},
			"backup_interval": {
				Name:        "backup_interval",
				Description: "Backup interval",
				Type:        "duration",
				Required:    false,
				Default:     "12h",
			},
			"max_file_size": {
				Name:        "max_file_size",
				Description: "Maximum file size for operations",
				Type:        "int",
				Required:    false,
				Default:     10485760, // 10MB
			},
			"theme": {
				Name:        "theme",
				Description: "Team UI theme",
				Type:        "string",
				Required:    false,
				Default:     "default",
				Options:     []string{"default", "dark", "light", "team-blue", "team-green"},
			},
			"history_size": {
				Name:        "history_size",
				Description: "Maximum conversation history",
				Type:        "int",
				Required:    false,
				Default:     750,
			},
			"tool_timeout": {
				Name:        "tool_timeout",
				Description: "Tool execution timeout",
				Type:        "duration",
				Required:    false,
				Default:     "45s",
			},
			"max_concurrent": {
				Name:        "max_concurrent",
				Description: "Max concurrent tool executions",
				Type:        "int",
				Required:    false,
				Default:     7,
			},
		},
		Template: strings.TrimSpace(templateStr),
		Created:  time.Now(),
		Modified: time.Now(),
	}
}

// createProductionTemplate creates a production-ready template
func (tm *TemplateManager) createProductionTemplate() *ConfigTemplate {
	// Similar to team template but with production-hardened settings
	// Implementation would be similar to above templates
	return &ConfigTemplate{
		Name:        "production",
		Description: "Production-hardened configuration with maximum security and stability",
		Version:     "1.0.0",
		Author:      "gocodenow",
		Tags:        []string{"production", "secure", "stable"},
		// ... variables and template would be defined here
		Created:  time.Now(),
		Modified: time.Now(),
	}
}

// createLocalAITemplate creates a local AI setup template
func (tm *TemplateManager) createLocalAITemplate() *ConfigTemplate {
	// Template for local AI setups (Ollama, LocalAI, etc.)
	return &ConfigTemplate{
		Name:        "local-ai",
		Description: "Optimized configuration for local AI models (Ollama, LocalAI, etc.)",
		Version:     "1.0.0",
		Author:      "gocodenow",
		Tags:        []string{"local", "ai", "offline", "privacy"},
		// ... variables and template would be defined here
		Created:  time.Now(),
		Modified: time.Now(),
	}
}

// createCloudTemplate creates a cloud deployment template
func (tm *TemplateManager) createCloudTemplate() *ConfigTemplate {
	// Template for cloud deployments
	return &ConfigTemplate{
		Name:        "cloud",
		Description: "Cloud-optimized configuration for scalable deployments",
		Version:     "1.0.0",
		Author:      "gocodenow",
		Tags:        []string{"cloud", "scalable", "distributed"},
		// ... variables and template would be defined here
		Created:  time.Now(),
		Modified: time.Now(),
	}
}

// GenerateConfig generates a configuration from a template
func (tm *TemplateManager) GenerateConfig(templateName string, variables map[string]interface{}) (*Config, error) {
	tmpl, exists := tm.templates[templateName]
	if !exists {
		return nil, fmt.Errorf("template '%s' not found", templateName)
	}
	
	// Validate required variables
	if err := tm.validateTemplateVariables(tmpl, variables); err != nil {
		return nil, fmt.Errorf("template variable validation failed: %w", err)
	}
	
	// Apply defaults for missing optional variables
	variables = tm.applyTemplateDefaults(tmpl, variables)
	
	// Parse and execute template
	t, err := template.New(templateName).Parse(tmpl.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	
	var result strings.Builder
	if err := t.Execute(&result, variables); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	
	// Parse the generated YAML into a Config
	var versionedConfig VersionedConfig
	if err := yaml.Unmarshal([]byte(result.String()), &versionedConfig); err != nil {
		return nil, fmt.Errorf("failed to parse generated config: %w", err)
	}
	
	// Extract the config from the versioned wrapper
	var config Config
	configData, err := yaml.Marshal(versionedConfig.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	return &config, nil
}

// validateTemplateVariables validates that all required variables are provided
func (tm *TemplateManager) validateTemplateVariables(tmpl *ConfigTemplate, variables map[string]interface{}) error {
	for _, tmplVar := range tmpl.Variables {
		if tmplVar.Required {
			if _, exists := variables[tmplVar.Name]; !exists {
				return fmt.Errorf("required variable '%s' not provided", tmplVar.Name)
			}
		}
	}
	return nil
}

// applyTemplateDefaults applies default values for missing optional variables
func (tm *TemplateManager) applyTemplateDefaults(tmpl *ConfigTemplate, variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy provided variables
	for k, v := range variables {
		result[k] = v
	}
	
	// Apply defaults for missing variables
	for _, tmplVar := range tmpl.Variables {
		if _, exists := result[tmplVar.Name]; !exists && tmplVar.Default != nil {
			result[tmplVar.Name] = tmplVar.Default
		}
	}
	
	return result
}

// SaveTemplate saves a template to disk
func (tm *TemplateManager) SaveTemplate(tmpl *ConfigTemplate) error {
	// Calculate checksum
	tmpl.Checksum = tm.calculateTemplateChecksum(tmpl)
	tmpl.Modified = time.Now()
	
	filename := fmt.Sprintf("%s.yaml", tmpl.Name)
	path := filepath.Join(tm.templatesDir, filename)
	
	data, err := yaml.Marshal(tmpl)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}
	
	// Add to memory
	tm.templates[tmpl.Name] = tmpl
	
	return nil
}

// LoadSharedConfigs loads shared configurations
func (tm *TemplateManager) LoadSharedConfigs() error {
	// Create shared directory if it doesn't exist
	if err := os.MkdirAll(tm.sharedDir, 0755); err != nil {
		return fmt.Errorf("failed to create shared directory: %w", err)
	}
	
	// Load shared configs from files
	entries, err := os.ReadDir(tm.sharedDir)
	if err != nil {
		return fmt.Errorf("failed to read shared directory: %w", err)
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			sharedPath := filepath.Join(tm.sharedDir, entry.Name())
			if err := tm.loadSharedConfigFromFile(sharedPath); err != nil {
				fmt.Printf("Warning: failed to load shared config %s: %v\n", entry.Name(), err)
			}
		}
	}
	
	return nil
}

// ShareConfiguration creates a shareable configuration package
func (tm *TemplateManager) ShareConfiguration(name, description, author string, config *Config, tags []string) (*SharedConfig, error) {
	shared := &SharedConfig{
		Name:        name,
		Description: description,
		Version:     "1.0.0",
		Author:      author,
		Tags:        tags,
		Config:      *config,
		Metadata: SharedConfigMeta{
			Created:  time.Now(),
			Modified: time.Now(),
			License:  "MIT",
		},
	}
	
	// Calculate checksum
	shared.Checksum = tm.calculateSharedConfigChecksum(shared)
	
	// Save to memory
	tm.sharedConfigs[shared.Name] = shared
	
	return shared, nil
}

// ExportSharedConfig exports a shared configuration to a file
func (tm *TemplateManager) ExportSharedConfig(name, path string) error {
	shared, exists := tm.sharedConfigs[name]
	if !exists {
		return fmt.Errorf("shared configuration '%s' not found", name)
	}
	
	data, err := yaml.Marshal(shared)
	if err != nil {
		return fmt.Errorf("failed to marshal shared config: %w", err)
	}
	
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write shared config file: %w", err)
	}
	
	return nil
}

// ImportSharedConfig imports a shared configuration from a file
func (tm *TemplateManager) ImportSharedConfig(path string) error {
	return tm.loadSharedConfigFromFile(path)
}

// DownloadTemplate downloads a template from a registry
func (tm *TemplateManager) DownloadTemplate(templateURL string) error {
	// Download template from URL
	resp, err := http.Get(templateURL)
	if err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download template: HTTP %d", resp.StatusCode)
	}
	
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read template data: %w", err)
	}
	
	// Parse template
	var tmpl ConfigTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}
	
	// Save template
	return tm.SaveTemplate(&tmpl)
}

// Helper methods

func (tm *TemplateManager) loadTemplateFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}
	
	var tmpl ConfigTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return fmt.Errorf("failed to unmarshal template: %w", err)
	}
	
	tm.templates[tmpl.Name] = &tmpl
	return nil
}

func (tm *TemplateManager) loadSharedConfigFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read shared config file: %w", err)
	}
	
	var shared SharedConfig
	if err := yaml.Unmarshal(data, &shared); err != nil {
		return fmt.Errorf("failed to unmarshal shared config: %w", err)
	}
	
	tm.sharedConfigs[shared.Name] = &shared
	return nil
}

func (tm *TemplateManager) calculateTemplateChecksum(tmpl *ConfigTemplate) string {
	// Create a copy without checksum and timestamps for consistent hashing
	hashData := struct {
		Name        string
		Description string
		Version     string
		Author      string
		Tags        []string
		Variables   map[string]TemplateVar
		Template    string
	}{
		Name:        tmpl.Name,
		Description: tmpl.Description,
		Version:     tmpl.Version,
		Author:      tmpl.Author,
		Tags:        tmpl.Tags,
		Variables:   tmpl.Variables,
		Template:    tmpl.Template,
	}
	
	data, _ := json.Marshal(hashData)
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

func (tm *TemplateManager) calculateSharedConfigChecksum(shared *SharedConfig) string {
	// Similar to template checksum but for shared configs
	hashData := struct {
		Name        string
		Description string
		Version     string
		Author      string
		Tags        []string
		Config      Config
	}{
		Name:        shared.Name,
		Description: shared.Description,
		Version:     shared.Version,
		Author:      shared.Author,
		Tags:        shared.Tags,
		Config:      shared.Config,
	}
	
	data, _ := json.Marshal(hashData)
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

// GetTemplate returns a template by name
func (tm *TemplateManager) GetTemplate(name string) (*ConfigTemplate, error) {
	if tmpl, exists := tm.templates[name]; exists {
		return tmpl, nil
	}
	return nil, fmt.Errorf("template '%s' not found", name)
}

// ListTemplates returns all available templates
func (tm *TemplateManager) ListTemplates() []*ConfigTemplate {
	templates := make([]*ConfigTemplate, 0, len(tm.templates))
	for _, tmpl := range tm.templates {
		templates = append(templates, tmpl)
	}
	return templates
}

// GetSharedConfig returns a shared configuration by name
func (tm *TemplateManager) GetSharedConfig(name string) (*SharedConfig, error) {
	if shared, exists := tm.sharedConfigs[name]; exists {
		return shared, nil
	}
	return nil, fmt.Errorf("shared configuration '%s' not found", name)
}

// ListSharedConfigs returns all available shared configurations
func (tm *TemplateManager) ListSharedConfigs() []*SharedConfig {
	configs := make([]*SharedConfig, 0, len(tm.sharedConfigs))
	for _, shared := range tm.sharedConfigs {
		configs = append(configs, shared)
	}
	return configs
}