package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigProfile represents a named configuration profile
type ConfigProfile struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Config      Config `yaml:"config" json:"config"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Created     time.Time `yaml:"created" json:"created"`
	Modified    time.Time `yaml:"modified" json:"modified"`
}

// ProfileManager manages configuration profiles
type ProfileManager struct {
	profilesDir string
	profiles    map[string]*ConfigProfile
}

// NewProfileManager creates a new profile manager
func NewProfileManager() *ProfileManager {
	profilesDir := filepath.Join(filepath.Dir(globalConfigPath()), "profiles")
	
	return &ProfileManager{
		profilesDir: profilesDir,
		profiles:    make(map[string]*ConfigProfile),
	}
}

// LoadProfiles loads all available profiles from disk
func (pm *ProfileManager) LoadProfiles() error {
	// Create profiles directory if it doesn't exist
	if err := os.MkdirAll(pm.profilesDir, 0755); err != nil {
		return fmt.Errorf("failed to create profiles directory: %w", err)
	}

	// Load built-in profiles first
	if err := pm.loadBuiltinProfiles(); err != nil {
		return fmt.Errorf("failed to load built-in profiles: %w", err)
	}

	// Load user profiles from disk
	entries, err := os.ReadDir(pm.profilesDir)
	if err != nil {
		return fmt.Errorf("failed to read profiles directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			profilePath := filepath.Join(pm.profilesDir, entry.Name())
			if err := pm.loadProfileFromFile(profilePath); err != nil {
				// Log error but continue loading other profiles
				fmt.Printf("Warning: failed to load profile %s: %v\n", entry.Name(), err)
			}
		}
	}

	return nil
}

// loadBuiltinProfiles creates and loads built-in configuration profiles
func (pm *ProfileManager) loadBuiltinProfiles() error {
	profiles := []*ConfigProfile{
		pm.createDevelopmentProfile(),
		pm.createProductionProfile(),
		pm.createSecureProfile(),
		pm.createPerformanceProfile(),
		pm.createDebugProfile(),
		pm.createMinimalProfile(),
	}

	for _, profile := range profiles {
		pm.profiles[profile.Name] = profile
	}

	return nil
}

// createDevelopmentProfile creates a development-optimized profile
func (pm *ProfileManager) createDevelopmentProfile() *ConfigProfile {
	config := DefaultConfig()
	
	// Development-friendly settings
	config.LLM.Endpoint = "http://localhost:1234/v1"
	config.LLM.Timeout = 60
	config.LLM.MaxRetries = 1
	config.Storage.CacheSize = 100
	config.Storage.AutoSaveFreq = 1 * time.Minute
	config.Storage.EnableCompression = false
	config.Security.EnableSandbox = false
	config.Security.FileOperations.AllowAbsolutePaths = true
	config.UI.DefaultExpanded = true
	config.UI.ShowTimestamps = true
	config.Tools.DefaultTimeout = 60 * time.Second
	config.Tools.MaxConcurrent = 10

	return &ConfigProfile{
		Name:        "development",
		Description: "Development-optimized settings with relaxed security and extended timeouts",
		Config:      *config,
		Tags:        []string{"dev", "local", "debug"},
		Created:     time.Now(),
		Modified:    time.Now(),
	}
}

// createProductionProfile creates a production-ready profile
func (pm *ProfileManager) createProductionProfile() *ConfigProfile {
	config := DefaultConfig()
	
	// Production settings
	config.LLM.Timeout = 30
	config.LLM.MaxRetries = 3
	config.Storage.CacheSize = 200
	config.Storage.AutoSaveFreq = 5 * time.Minute
	config.Storage.EnableCompression = true
	config.Storage.BackupInterval = 6 * time.Hour
	config.Security.EnableSandbox = true
	config.Security.FileOperations.AllowAbsolutePaths = false
	config.UI.DefaultExpanded = false
	config.UI.MaxHistorySize = 500
	config.Tools.DefaultTimeout = 30 * time.Second
	config.Tools.MaxConcurrent = 5

	return &ConfigProfile{
		Name:        "production",
		Description: "Production-ready settings with enhanced security and conservative resource usage",
		Config:      *config,
		Tags:        []string{"prod", "secure", "stable"},
		Created:     time.Now(),
		Modified:    time.Now(),
	}
}

// createSecureProfile creates a security-focused profile
func (pm *ProfileManager) createSecureProfile() *ConfigProfile {
	config := DefaultConfig()
	
	// Security-focused settings
	config.LLM.Timeout = 20
	config.LLM.MaxRetries = 2
	config.Storage.EnableCompression = true
	config.Security.EnableSandbox = true
	config.Security.MaxFileSize = 5 * 1024 * 1024 // 5MB
	config.Security.FileOperations.AllowAbsolutePaths = false
	config.Security.FileOperations.AllowSymlinks = false
	config.Security.FileOperations.BlockedPaths = append(config.Security.FileOperations.BlockedPaths,
		"/home", "/Users", "/tmp", "/var", "/opt")
	config.Tools.DefaultTimeout = 20 * time.Second
	config.Tools.MaxConcurrent = 3

	return &ConfigProfile{
		Name:        "secure",
		Description: "Maximum security settings with restricted file access and sandboxing",
		Config:      *config,
		Tags:        []string{"secure", "restricted", "sandbox"},
		Created:     time.Now(),
		Modified:    time.Now(),
	}
}

// createPerformanceProfile creates a performance-optimized profile
func (pm *ProfileManager) createPerformanceProfile() *ConfigProfile {
	config := DefaultConfig()
	
	// Performance-optimized settings
	config.LLM.Timeout = 45
	config.LLM.MaxRetries = 5
	config.LLM.RateLimitRPM = 120
	config.Storage.CacheSize = 500
	config.Storage.AutoSaveFreq = 30 * time.Second
	config.Storage.MaxMemoryMB = 500
	config.Storage.EnableCompression = true
	config.UI.MaxHistorySize = 2000
	config.Tools.DefaultTimeout = 45 * time.Second
	config.Tools.MaxConcurrent = 15

	return &ConfigProfile{
		Name:        "performance",
		Description: "High-performance settings with large caches and increased concurrency",
		Config:      *config,
		Tags:        []string{"performance", "fast", "high-memory"},
		Created:     time.Now(),
		Modified:    time.Now(),
	}
}

// createDebugProfile creates a debugging-focused profile
func (pm *ProfileManager) createDebugProfile() *ConfigProfile {
	config := DefaultConfig()
	
	// Debug-friendly settings
	config.LLM.Timeout = 120
	config.LLM.MaxRetries = 1
	config.Storage.AutoSaveFreq = 10 * time.Second
	config.Storage.EnableCompression = false
	config.Security.EnableSandbox = false
	config.Security.FileOperations.AllowAbsolutePaths = true
	config.UI.DefaultExpanded = true
	config.UI.ShowTimestamps = true
	config.Tools.DefaultTimeout = 120 * time.Second
	config.Tools.MaxConcurrent = 1 // Sequential execution for debugging

	return &ConfigProfile{
		Name:        "debug",
		Description: "Debug-friendly settings with extended timeouts and detailed logging",
		Config:      *config,
		Tags:        []string{"debug", "verbose", "sequential"},
		Created:     time.Now(),
		Modified:    time.Now(),
	}
}

// createMinimalProfile creates a minimal resource usage profile
func (pm *ProfileManager) createMinimalProfile() *ConfigProfile {
	config := DefaultConfig()
	
	// Minimal resource settings
	config.LLM.Timeout = 15
	config.LLM.MaxRetries = 1
	config.LLM.RateLimitRPM = 30
	config.Storage.CacheSize = 20
	config.Storage.AutoSaveFreq = 10 * time.Minute
	config.Storage.MaxMemoryMB = 50
	config.Storage.EnableCompression = true
	config.UI.MaxHistorySize = 100
	config.Tools.DefaultTimeout = 15 * time.Second
	config.Tools.MaxConcurrent = 2

	return &ConfigProfile{
		Name:        "minimal",
		Description: "Minimal resource usage settings for low-end systems or limited environments",
		Config:      *config,
		Tags:        []string{"minimal", "low-memory", "constrained"},
		Created:     time.Now(),
		Modified:    time.Now(),
	}
}

// GetProfile retrieves a profile by name
func (pm *ProfileManager) GetProfile(name string) (*ConfigProfile, error) {
	profile, exists := pm.profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}
	
	return profile, nil
}

// ListProfiles returns all available profiles
func (pm *ProfileManager) ListProfiles() []*ConfigProfile {
	profiles := make([]*ConfigProfile, 0, len(pm.profiles))
	for _, profile := range pm.profiles {
		profiles = append(profiles, profile)
	}
	return profiles
}

// CreateProfile creates a new custom profile
func (pm *ProfileManager) CreateProfile(name, description string, config *Config, tags []string) error {
	if _, exists := pm.profiles[name]; exists {
		return fmt.Errorf("profile '%s' already exists", name)
	}

	profile := &ConfigProfile{
		Name:        name,
		Description: description,
		Config:      *config,
		Tags:        tags,
		Created:     time.Now(),
		Modified:    time.Now(),
	}

	pm.profiles[name] = profile

	// Save to disk
	return pm.SaveProfile(profile)
}

// UpdateProfile updates an existing profile
func (pm *ProfileManager) UpdateProfile(name string, config *Config, description string, tags []string) error {
	profile, exists := pm.profiles[name]
	if !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	profile.Config = *config
	if description != "" {
		profile.Description = description
	}
	if tags != nil {
		profile.Tags = tags
	}
	profile.Modified = time.Now()

	// Save to disk (don't save built-in profiles)
	if !pm.isBuiltinProfile(name) {
		return pm.SaveProfile(profile)
	}

	return nil
}

// DeleteProfile deletes a profile
func (pm *ProfileManager) DeleteProfile(name string) error {
	if pm.isBuiltinProfile(name) {
		return fmt.Errorf("cannot delete built-in profile '%s'", name)
	}

	if _, exists := pm.profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(pm.profiles, name)

	// Remove file from disk
	profilePath := filepath.Join(pm.profilesDir, name+".yaml")
	if err := os.Remove(profilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete profile file: %w", err)
	}

	return nil
}

// SaveProfile saves a profile to disk
func (pm *ProfileManager) SaveProfile(profile *ConfigProfile) error {
	if pm.isBuiltinProfile(profile.Name) {
		// Don't save built-in profiles to disk
		return nil
	}

	profilePath := filepath.Join(pm.profilesDir, profile.Name+".yaml")
	
	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(profilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}

	return nil
}

// loadProfileFromFile loads a profile from a YAML file
func (pm *ProfileManager) loadProfileFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read profile file: %w", err)
	}

	var profile ConfigProfile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return fmt.Errorf("failed to unmarshal profile: %w", err)
	}

	pm.profiles[profile.Name] = &profile
	return nil
}

// isBuiltinProfile checks if a profile is built-in
func (pm *ProfileManager) isBuiltinProfile(name string) bool {
	builtinProfiles := []string{"development", "production", "secure", "performance", "debug", "minimal"}
	for _, builtin := range builtinProfiles {
		if builtin == name {
			return true
		}
	}
	return false
}

// ApplyProfile applies a profile to the runtime configuration manager
func (pm *ProfileManager) ApplyProfile(name string, rcm *RuntimeConfigManager) error {
	profile, err := pm.GetProfile(name)
	if err != nil {
		return err
	}

	return rcm.UpdateConfig(&profile.Config)
}

// GetProfilesByTag returns profiles that match any of the given tags
func (pm *ProfileManager) GetProfilesByTag(tags ...string) []*ConfigProfile {
	var matching []*ConfigProfile

	for _, profile := range pm.profiles {
		for _, profileTag := range profile.Tags {
			for _, searchTag := range tags {
				if profileTag == searchTag {
					matching = append(matching, profile)
					goto nextProfile
				}
			}
		}
	nextProfile:
	}

	return matching
}

// ExportProfile exports a profile to a file
func (pm *ProfileManager) ExportProfile(name, path string) error {
	profile, err := pm.GetProfile(name)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write profile file: %w", err)
	}

	return nil
}

// ImportProfile imports a profile from a file
func (pm *ProfileManager) ImportProfile(path string) error {
	return pm.loadProfileFromFile(path)
}

// CloneProfile creates a copy of an existing profile with a new name
func (pm *ProfileManager) CloneProfile(sourceName, newName, description string, tags []string) error {
	sourceProfile, err := pm.GetProfile(sourceName)
	if err != nil {
		return err
	}

	if description == "" {
		description = fmt.Sprintf("Cloned from '%s': %s", sourceName, sourceProfile.Description)
	}

	if tags == nil {
		tags = append([]string{"cloned"}, sourceProfile.Tags...)
	}

	return pm.CreateProfile(newName, description, &sourceProfile.Config, tags)
}