package recovery

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// FeatureLevel represents the level of feature availability
type FeatureLevel int

const (
	// FeatureLevelFull represents full functionality
	FeatureLevelFull FeatureLevel = iota
	// FeatureLevelReduced represents reduced functionality
	FeatureLevelReduced
	// FeatureLevelBasic represents basic functionality only
	FeatureLevelBasic
	// FeatureLevelMinimal represents minimal functionality
	FeatureLevelMinimal
	// FeatureLevelDisabled represents disabled functionality
	FeatureLevelDisabled
)

// DependencyType represents the type of dependency
type DependencyType int

const (
	// SystemDependency represents system-level dependencies (executables, libraries)
	SystemDependency DependencyType = iota
	// ServiceDependency represents service dependencies (APIs, databases)
	ServiceDependency
	// ConfigDependency represents configuration dependencies
	ConfigDependency
	// ResourceDependency represents resource dependencies (memory, disk, network)
	ResourceDependency
	// OptionalDependency represents optional dependencies
	OptionalDependency
)

// DependencyStatus represents the status of a dependency
type DependencyStatus int

const (
	// StatusAvailable indicates the dependency is available
	StatusAvailable DependencyStatus = iota
	// StatusUnavailable indicates the dependency is unavailable
	StatusUnavailable
	// StatusDegraded indicates the dependency is partially available
	StatusDegraded
	// StatusUnknown indicates the dependency status is unknown
	StatusUnknown
	// StatusChecking indicates the dependency is being checked
	StatusChecking
)

// Dependency represents a system dependency
type Dependency struct {
	Name            string                              `json:"name"`
	Type            DependencyType                      `json:"type"`
	Required        bool                                `json:"required"`
	Description     string                              `json:"description"`
	CheckFunction   func(context.Context) (DependencyStatus, error) `json:"-"`
	FallbackHandler func(context.Context) error       `json:"-"`
	Status          DependencyStatus                    `json:"status"`
	LastChecked     time.Time                           `json:"last_checked"`
	Error           string                              `json:"error,omitempty"`
	Version         string                              `json:"version,omitempty"`
	Metadata        map[string]interface{}              `json:"metadata"`
	CheckInterval   time.Duration                       `json:"check_interval"`
	MaxRetries      int                                 `json:"max_retries"`
	RetryCount      int                                 `json:"retry_count"`
}

// Feature represents a feature that depends on certain dependencies
type Feature struct {
	Name         string            `json:"name"`
	Dependencies []string          `json:"dependencies"`
	Level        FeatureLevel      `json:"level"`
	Description  string            `json:"description"`
	Fallbacks    []FallbackOption  `json:"fallbacks"`
	Enabled      bool              `json:"enabled"`
	LastUpdate   time.Time         `json:"last_update"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// FallbackOption represents a fallback option for a feature
type FallbackOption struct {
	Name        string       `json:"name"`
	Level       FeatureLevel `json:"level"`
	Description string       `json:"description"`
	Handler     func(context.Context) error `json:"-"`
	Enabled     bool         `json:"enabled"`
}

// DegradationManager manages graceful degradation of features
type DegradationManager struct {
	dependencies     map[string]*Dependency
	features         map[string]*Feature
	checkInterval    time.Duration
	mutex            sync.RWMutex
	checkChan        chan string
	stopChan         chan struct{}
	notifications    []DegradationNotifier
	healthChecker    *HealthChecker
	statusHistory    []StatusSnapshot
	maxHistorySize   int
}

// DegradationNotifier is called when feature levels change
type DegradationNotifier func(feature string, oldLevel, newLevel FeatureLevel, reason string)

// StatusSnapshot represents a point-in-time status snapshot
type StatusSnapshot struct {
	Timestamp    time.Time                    `json:"timestamp"`
	Dependencies map[string]DependencyStatus  `json:"dependencies"`
	Features     map[string]FeatureLevel      `json:"features"`
	OverallHealth string                      `json:"overall_health"`
}

// HealthChecker performs periodic health checks
type HealthChecker struct {
	checks   map[string]func(context.Context) error
	interval time.Duration
	timeout  time.Duration
}

// NewDegradationManager creates a new degradation manager
func NewDegradationManager() *DegradationManager {
	return &DegradationManager{
		dependencies:   make(map[string]*Dependency),
		features:       make(map[string]*Feature),
		checkInterval:  30 * time.Second,
		checkChan:      make(chan string, 100),
		stopChan:       make(chan struct{}),
		notifications:  make([]DegradationNotifier, 0),
		statusHistory:  make([]StatusSnapshot, 0),
		maxHistorySize: 100,
		healthChecker:  &HealthChecker{
			checks:   make(map[string]func(context.Context) error),
			interval: 60 * time.Second,
			timeout:  10 * time.Second,
		},
	}
}

// RegisterDependency registers a dependency for monitoring
func (dm *DegradationManager) RegisterDependency(dep *Dependency) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	if dep.CheckInterval == 0 {
		dep.CheckInterval = dm.checkInterval
	}
	if dep.Metadata == nil {
		dep.Metadata = make(map[string]interface{})
	}
	
	dm.dependencies[dep.Name] = dep
}

// RegisterFeature registers a feature that depends on certain dependencies
func (dm *DegradationManager) RegisterFeature(feature *Feature) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	if feature.Metadata == nil {
		feature.Metadata = make(map[string]interface{})
	}
	
	dm.features[feature.Name] = feature
}

// RegisterNotifier registers a degradation notifier
func (dm *DegradationManager) RegisterNotifier(notifier DegradationNotifier) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	dm.notifications = append(dm.notifications, notifier)
}

// Start starts the degradation manager
func (dm *DegradationManager) Start(ctx context.Context) error {
	// Perform initial dependency checks
	if err := dm.CheckAllDependencies(ctx); err != nil {
		log.Printf("Initial dependency check failed: %v", err)
	}
	
	// Update feature levels based on initial checks
	dm.updateAllFeatureLevels()
	
	// Start periodic monitoring
	go dm.monitorDependencies(ctx)
	go dm.healthChecker.start(ctx)
	
	return nil
}

// Stop stops the degradation manager
func (dm *DegradationManager) Stop() error {
	close(dm.stopChan)
	return nil
}

// CheckAllDependencies checks all registered dependencies
func (dm *DegradationManager) CheckAllDependencies(ctx context.Context) error {
	dm.mutex.RLock()
	deps := make([]*Dependency, 0, len(dm.dependencies))
	for _, dep := range dm.dependencies {
		deps = append(deps, dep)
	}
	dm.mutex.RUnlock()
	
	// Check dependencies concurrently
	errChan := make(chan error, len(deps))
	var wg sync.WaitGroup
	
	for _, dep := range deps {
		wg.Add(1)
		go func(d *Dependency) {
			defer wg.Done()
			if err := dm.checkDependency(ctx, d); err != nil {
				errChan <- fmt.Errorf("dependency %s: %w", d.Name, err)
			}
		}(dep)
	}
	
	wg.Wait()
	close(errChan)
	
	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("dependency check failures: %v", errors)
	}
	
	return nil
}

// checkDependency checks a single dependency
func (dm *DegradationManager) checkDependency(ctx context.Context, dep *Dependency) error {
	dm.mutex.Lock()
	dep.Status = StatusChecking
	dep.LastChecked = time.Now()
	dm.mutex.Unlock()
	
	// Create timeout context for the check
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	status, err := dep.CheckFunction(checkCtx)
	
	dm.mutex.Lock()
	dep.Status = status
	dep.LastChecked = time.Now()
	if err != nil {
		dep.Error = err.Error()
		dep.RetryCount++
	} else {
		dep.Error = ""
		dep.RetryCount = 0
	}
	dm.mutex.Unlock()
	
	// If dependency is unavailable and has a fallback, activate it
	if status == StatusUnavailable && dep.FallbackHandler != nil {
		if fallbackErr := dep.FallbackHandler(ctx); fallbackErr != nil {
			log.Printf("Fallback handler failed for %s: %v", dep.Name, fallbackErr)
		}
	}
	
	return err
}

// updateAllFeatureLevels updates all feature levels based on dependency status
func (dm *DegradationManager) updateAllFeatureLevels() {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	for _, feature := range dm.features {
		oldLevel := feature.Level
		newLevel := dm.calculateFeatureLevel(feature)
		
		if oldLevel != newLevel {
			feature.Level = newLevel
			feature.LastUpdate = time.Now()
			
			// Notify about level change
			reason := fmt.Sprintf("Dependency status change affecting feature %s", feature.Name)
			dm.notifyLevelChange(feature.Name, oldLevel, newLevel, reason)
		}
	}
}

// calculateFeatureLevel calculates the appropriate level for a feature
func (dm *DegradationManager) calculateFeatureLevel(feature *Feature) FeatureLevel {
	// Check if all required dependencies are available
	allAvailable := true
	someAvailable := false
	
	for _, depName := range feature.Dependencies {
		if dep, exists := dm.dependencies[depName]; exists {
			switch dep.Status {
			case StatusAvailable:
				someAvailable = true
			case StatusDegraded:
				someAvailable = true
				allAvailable = false
			case StatusUnavailable:
				allAvailable = false
				if dep.Required {
					return FeatureLevelDisabled
				}
			}
		}
	}
	
	// Determine feature level based on dependency status
	if allAvailable {
		return FeatureLevelFull
	} else if someAvailable {
		return FeatureLevelReduced
	} else {
		return FeatureLevelMinimal
	}
}

// monitorDependencies monitors dependencies continuously
func (dm *DegradationManager) monitorDependencies(ctx context.Context) {
	ticker := time.NewTicker(dm.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-dm.stopChan:
			return
		case <-ticker.C:
			// Check all dependencies
			if err := dm.CheckAllDependencies(ctx); err != nil {
				log.Printf("Periodic dependency check failed: %v", err)
			}
			dm.updateAllFeatureLevels()
			dm.takeStatusSnapshot()
		case depName := <-dm.checkChan:
			// Check specific dependency
			if dep, exists := dm.dependencies[depName]; exists {
				if err := dm.checkDependency(ctx, dep); err != nil {
					log.Printf("On-demand check for %s failed: %v", depName, err)
				}
				dm.updateAllFeatureLevels()
			}
		}
	}
}

// takeStatusSnapshot takes a snapshot of current status
func (dm *DegradationManager) takeStatusSnapshot() {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	snapshot := StatusSnapshot{
		Timestamp:    time.Now(),
		Dependencies: make(map[string]DependencyStatus),
		Features:     make(map[string]FeatureLevel),
	}
	
	for name, dep := range dm.dependencies {
		snapshot.Dependencies[name] = dep.Status
	}
	
	for name, feature := range dm.features {
		snapshot.Features[name] = feature.Level
	}
	
	// Calculate overall health
	snapshot.OverallHealth = dm.calculateOverallHealth()
	
	// Add to history
	dm.statusHistory = append(dm.statusHistory, snapshot)
	
	// Maintain history size
	if len(dm.statusHistory) > dm.maxHistorySize {
		dm.statusHistory = dm.statusHistory[1:]
	}
}

// calculateOverallHealth calculates overall system health
func (dm *DegradationManager) calculateOverallHealth() string {
	totalDeps := len(dm.dependencies)
	if totalDeps == 0 {
		return "unknown"
	}
	
	available := 0
	degraded := 0
	unavailable := 0
	
	for _, dep := range dm.dependencies {
		switch dep.Status {
		case StatusAvailable:
			available++
		case StatusDegraded:
			degraded++
		case StatusUnavailable:
			if dep.Required {
				unavailable++
			}
		}
	}
	
	if unavailable > 0 {
		return "critical"
	} else if degraded > totalDeps/2 {
		return "degraded"
	} else if available > totalDeps*3/4 {
		return "healthy"
	} else {
		return "warning"
	}
}

// notifyLevelChange notifies about feature level changes
func (dm *DegradationManager) notifyLevelChange(feature string, oldLevel, newLevel FeatureLevel, reason string) {
	for _, notifier := range dm.notifications {
		go func(n DegradationNotifier) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Degradation notifier panic: %v", r)
				}
			}()
			n(feature, oldLevel, newLevel, reason)
		}(notifier)
	}
}

// GetFeatureLevel returns the current level of a feature
func (dm *DegradationManager) GetFeatureLevel(featureName string) FeatureLevel {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	
	if feature, exists := dm.features[featureName]; exists {
		return feature.Level
	}
	
	return FeatureLevelDisabled
}

// IsFeatureAvailable checks if a feature is available at a given level
func (dm *DegradationManager) IsFeatureAvailable(featureName string, minLevel FeatureLevel) bool {
	currentLevel := dm.GetFeatureLevel(featureName)
	return currentLevel >= minLevel && currentLevel != FeatureLevelDisabled
}

// RequestDependencyCheck requests an immediate check of a dependency
func (dm *DegradationManager) RequestDependencyCheck(depName string) {
	select {
	case dm.checkChan <- depName:
	default:
		// Channel is full, ignore request
	}
}

// GetDependencyStatus returns the current status of a dependency
func (dm *DegradationManager) GetDependencyStatus(depName string) (DependencyStatus, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	
	if dep, exists := dm.dependencies[depName]; exists {
		return dep.Status, nil
	}
	
	return StatusUnknown, fmt.Errorf("dependency %s not found", depName)
}

// GetStatusSnapshot returns the latest status snapshot
func (dm *DegradationManager) GetStatusSnapshot() StatusSnapshot {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	
	if len(dm.statusHistory) > 0 {
		return dm.statusHistory[len(dm.statusHistory)-1]
	}
	
	return StatusSnapshot{
		Timestamp: time.Now(),
		Dependencies: make(map[string]DependencyStatus),
		Features: make(map[string]FeatureLevel),
		OverallHealth: "unknown",
	}
}

// Built-in dependency checkers

// CreateExecutableChecker creates a checker for executable dependencies
func CreateExecutableChecker(execName string) func(context.Context) (DependencyStatus, error) {
	return func(ctx context.Context) (DependencyStatus, error) {
		_, err := exec.LookPath(execName)
		if err != nil {
			return StatusUnavailable, fmt.Errorf("executable %s not found: %w", execName, err)
		}
		return StatusAvailable, nil
	}
}

// CreateServiceChecker creates a checker for service dependencies
func CreateServiceChecker(serviceURL string) func(context.Context) (DependencyStatus, error) {
	return func(ctx context.Context) (DependencyStatus, error) {
		// This would implement HTTP/TCP connectivity check
		// For now, return available
		return StatusAvailable, nil
	}
}

// CreateFileChecker creates a checker for file dependencies
func CreateFileChecker(filePath string) func(context.Context) (DependencyStatus, error) {
	return func(ctx context.Context) (DependencyStatus, error) {
		if _, err := os.Stat(filePath); err != nil {
			if os.IsNotExist(err) {
				return StatusUnavailable, fmt.Errorf("file %s does not exist", filePath)
			}
			return StatusDegraded, fmt.Errorf("file %s access error: %w", filePath, err)
		}
		return StatusAvailable, nil
	}
}

// CreateMemoryChecker creates a checker for memory availability
func CreateMemoryChecker(minMemoryMB uint64) func(context.Context) (DependencyStatus, error) {
	return func(ctx context.Context) (DependencyStatus, error) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		availableMemoryMB := (m.Sys - m.Alloc) / (1024 * 1024)
		
		if availableMemoryMB >= minMemoryMB {
			return StatusAvailable, nil
		} else if availableMemoryMB >= minMemoryMB/2 {
			return StatusDegraded, fmt.Errorf("low memory: %dMB available, %dMB required", 
				availableMemoryMB, minMemoryMB)
		} else {
			return StatusUnavailable, fmt.Errorf("insufficient memory: %dMB available, %dMB required", 
				availableMemoryMB, minMemoryMB)
		}
	}
}

// Health checker methods

func (hc *HealthChecker) start(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hc.runChecks(ctx)
		}
	}
}

func (hc *HealthChecker) runChecks(ctx context.Context) {
	for name, check := range hc.checks {
		go func(checkName string, checkFunc func(context.Context) error) {
			checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
			defer cancel()
			
			if err := checkFunc(checkCtx); err != nil {
				log.Printf("Health check %s failed: %v", checkName, err)
			}
		}(name, check)
	}
}

func (hc *HealthChecker) RegisterCheck(name string, check func(context.Context) error) {
	hc.checks[name] = check
}