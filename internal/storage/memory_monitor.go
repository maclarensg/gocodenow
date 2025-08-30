package storage

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// MemoryMonitor provides comprehensive memory usage monitoring and cleanup
type MemoryMonitor struct {
	// Configuration
	config           *MemoryConfig
	
	// Components to monitor
	cache            *ConversationCache
	lazyLoader       *LazyConversationLoader
	persistence      *BackgroundPersistenceManager
	
	// Monitoring state
	running          bool
	metrics          *MemoryMetrics
	alerts           []MemoryAlert
	
	// Cleanup strategies
	cleanupStrategies []CleanupStrategy
	
	// Event handling
	alertCallbacks   []func(MemoryAlert)
	
	// Synchronization
	mutex            sync.RWMutex
	shutdownCh       chan struct{}
	workerWg         sync.WaitGroup
}

// MemoryConfig defines memory monitoring configuration
type MemoryConfig struct {
	MonitorInterval      time.Duration `json:"monitor_interval"`
	AlertThresholds      AlertThresholds `json:"alert_thresholds"`
	CleanupThresholds    CleanupThresholds `json:"cleanup_thresholds"`
	GCTriggerPercent     int          `json:"gc_trigger_percent"`
	EnableAutoCleanup    bool         `json:"enable_auto_cleanup"`
	EnableGCTuning       bool         `json:"enable_gc_tuning"`
	MemoryLimitMB        int64        `json:"memory_limit_mb"`
	EnableProfiling      bool         `json:"enable_profiling"`
}

// AlertThresholds defines when to trigger memory alerts
type AlertThresholds struct {
	WarningPercent   int `json:"warning_percent"`   // 70%
	CriticalPercent  int `json:"critical_percent"`  // 85%
	EmergencyPercent int `json:"emergency_percent"` // 95%
}

// CleanupThresholds defines when to trigger cleanup operations
type CleanupThresholds struct {
	CacheCleanup     int `json:"cache_cleanup"`     // 75%
	ForceGC          int `json:"force_gc"`          // 80%
	AggressiveCleanup int `json:"aggressive_cleanup"` // 90%
}

// MemoryMetrics tracks detailed memory usage statistics
type MemoryMetrics struct {
	// Current memory usage
	HeapInUse        int64 `json:"heap_in_use"`
	HeapIdle         int64 `json:"heap_idle"`
	HeapSys          int64 `json:"heap_sys"`
	StackInUse       int64 `json:"stack_in_use"`
	TotalAlloc       int64 `json:"total_alloc"`
	
	// System memory
	SystemMemory     int64 `json:"system_memory"`
	AvailableMemory  int64 `json:"available_memory"`
	
	// Garbage collection
	GCCount          int64 `json:"gc_count"`
	LastGCTime       time.Time `json:"last_gc_time"`
	GCPauseTotal     time.Duration `json:"gc_pause_total"`
	
	// Component memory usage
	CacheMemory      int64 `json:"cache_memory"`
	LazyLoaderMemory int64 `json:"lazy_loader_memory"`
	
	// Performance metrics
	MemoryPressure   float64 `json:"memory_pressure"`   // 0-100%
	AllocationRate   float64 `json:"allocation_rate"`   // MB/s
	GCEfficiency     float64 `json:"gc_efficiency"`     // % memory reclaimed
	
	// Monitoring metadata
	LastUpdated      time.Time `json:"last_updated"`
	mutex            sync.RWMutex
}

// MemoryAlert represents a memory usage alert
type MemoryAlert struct {
	Level       AlertLevel `json:"level"`
	Message     string     `json:"message"`
	Metric      string     `json:"metric"`
	Value       int64      `json:"value"`
	Threshold   int64      `json:"threshold"`
	Timestamp   time.Time  `json:"timestamp"`
	Resolved    bool       `json:"resolved"`
}

// AlertLevel defines the severity of memory alerts
type AlertLevel int

const (
	AlertInfo AlertLevel = iota
	AlertWarning
	AlertCritical
	AlertEmergency
)

// CleanupStrategy defines a memory cleanup strategy
type CleanupStrategy struct {
	Name        string
	Priority    int
	Threshold   int // Memory usage percentage to trigger
	Execute     func() error
	Description string
}

// DefaultMemoryConfig returns default memory monitoring configuration
func DefaultMemoryConfig() *MemoryConfig {
	return &MemoryConfig{
		MonitorInterval: 10 * time.Second,
		AlertThresholds: AlertThresholds{
			WarningPercent:   70,
			CriticalPercent:  85,
			EmergencyPercent: 95,
		},
		CleanupThresholds: CleanupThresholds{
			CacheCleanup:     75,
			ForceGC:          80,
			AggressiveCleanup: 90,
		},
		GCTriggerPercent:  50,
		EnableAutoCleanup: true,
		EnableGCTuning:    true,
		MemoryLimitMB:     1024, // 1GB default
		EnableProfiling:   false,
	}
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(config *MemoryConfig) *MemoryMonitor {
	if config == nil {
		config = DefaultMemoryConfig()
	}
	
	monitor := &MemoryMonitor{
		config:     config,
		running:    false,
		metrics:    &MemoryMetrics{},
		shutdownCh: make(chan struct{}),
	}
	
	// Initialize cleanup strategies
	monitor.initCleanupStrategies()
	
	// Configure garbage collector if tuning is enabled
	if config.EnableGCTuning {
		monitor.configureGC()
	}
	
	return monitor
}

// SetComponents sets the components to monitor
func (mm *MemoryMonitor) SetComponents(cache *ConversationCache, lazyLoader *LazyConversationLoader, persistence *BackgroundPersistenceManager) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	
	mm.cache = cache
	mm.lazyLoader = lazyLoader
	mm.persistence = persistence
}

// Start begins memory monitoring
func (mm *MemoryMonitor) Start() error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	
	if mm.running {
		return fmt.Errorf("memory monitor is already running")
	}
	
	mm.running = true
	
	// Start monitoring worker
	mm.workerWg.Add(1)
	go mm.monitorWorker()
	
	// Start cleanup worker if auto-cleanup is enabled
	if mm.config.EnableAutoCleanup {
		mm.workerWg.Add(1)
		go mm.cleanupWorker()
	}
	
	return nil
}

// Stop stops memory monitoring
func (mm *MemoryMonitor) Stop(ctx context.Context) error {
	mm.mutex.Lock()
	if !mm.running {
		mm.mutex.Unlock()
		return nil
	}
	mm.running = false
	mm.mutex.Unlock()
	
	// Signal shutdown
	close(mm.shutdownCh)
	
	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		mm.workerWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// monitorWorker continuously monitors memory usage
func (mm *MemoryMonitor) monitorWorker() {
	defer mm.workerWg.Done()
	
	ticker := time.NewTicker(mm.config.MonitorInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-mm.shutdownCh:
			return
		case <-ticker.C:
			mm.collectMetrics()
			mm.checkAlerts()
		}
	}
}

// cleanupWorker performs automatic cleanup when needed
func (mm *MemoryMonitor) cleanupWorker() {
	defer mm.workerWg.Done()
	
	ticker := time.NewTicker(mm.config.MonitorInterval * 2)
	defer ticker.Stop()
	
	for {
		select {
		case <-mm.shutdownCh:
			return
		case <-ticker.C:
			mm.performAutoCleanup()
		}
	}
}

// collectMetrics collects current memory usage metrics
func (mm *MemoryMonitor) collectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	mm.metrics.mutex.Lock()
	defer mm.metrics.mutex.Unlock()
	
	// Update runtime metrics
	mm.metrics.HeapInUse = int64(m.HeapInuse)
	mm.metrics.HeapIdle = int64(m.HeapIdle)
	mm.metrics.HeapSys = int64(m.HeapSys)
	mm.metrics.StackInUse = int64(m.StackInuse)
	mm.metrics.TotalAlloc = int64(m.TotalAlloc)
	
	// GC metrics
	mm.metrics.GCCount = int64(m.NumGC)
	if m.NumGC > 0 {
		mm.metrics.LastGCTime = time.Unix(0, int64(m.LastGC))
		mm.metrics.GCPauseTotal = time.Duration(m.PauseTotalNs)
	}
	
	// Component memory usage
	if mm.cache != nil {
		mm.metrics.CacheMemory = mm.cache.GetMemoryUsage()
	}
	
	// Calculate derived metrics
	mm.calculateDerivedMetrics(&m)
	
	mm.metrics.LastUpdated = time.Now()
}

// calculateDerivedMetrics calculates derived performance metrics
func (mm *MemoryMonitor) calculateDerivedMetrics(m *runtime.MemStats) {
	memoryLimit := mm.config.MemoryLimitMB * 1024 * 1024
	
	// Memory pressure (0-100%)
	if memoryLimit > 0 {
		mm.metrics.MemoryPressure = float64(mm.metrics.HeapInUse) / float64(memoryLimit) * 100
	} else {
		// Use system memory if no limit set
		mm.metrics.MemoryPressure = float64(mm.metrics.HeapInUse) / float64(mm.metrics.SystemMemory) * 100
	}
	
	// GC efficiency (simplified calculation)
	if mm.metrics.GCCount > 0 {
		avgGCPause := mm.metrics.GCPauseTotal / time.Duration(mm.metrics.GCCount)
		// Lower pause time = higher efficiency
		mm.metrics.GCEfficiency = 100.0 - (float64(avgGCPause.Nanoseconds()) / 1000000.0) // Convert to ms and invert
		if mm.metrics.GCEfficiency < 0 {
			mm.metrics.GCEfficiency = 0
		}
	}
}

// checkAlerts checks for memory usage alerts
func (mm *MemoryMonitor) checkAlerts() {
	pressure := mm.GetMemoryPressure()
	
	// Check alert thresholds
	if pressure >= float64(mm.config.AlertThresholds.EmergencyPercent) {
		mm.triggerAlert(AlertEmergency, "Emergency memory usage", "memory_pressure", int64(pressure), int64(mm.config.AlertThresholds.EmergencyPercent))
	} else if pressure >= float64(mm.config.AlertThresholds.CriticalPercent) {
		mm.triggerAlert(AlertCritical, "Critical memory usage", "memory_pressure", int64(pressure), int64(mm.config.AlertThresholds.CriticalPercent))
	} else if pressure >= float64(mm.config.AlertThresholds.WarningPercent) {
		mm.triggerAlert(AlertWarning, "High memory usage", "memory_pressure", int64(pressure), int64(mm.config.AlertThresholds.WarningPercent))
	}
	
	// Check heap growth rate
	mm.metrics.mutex.RLock()
	heapInUse := mm.metrics.HeapInUse
	mm.metrics.mutex.RUnlock()
	
	if heapInUse > 500*1024*1024 { // 500MB
		mm.triggerAlert(AlertWarning, "High heap usage", "heap_in_use", heapInUse, 500*1024*1024)
	}
}

// triggerAlert creates and processes a memory alert
func (mm *MemoryMonitor) triggerAlert(level AlertLevel, message, metric string, value, threshold int64) {
	alert := MemoryAlert{
		Level:     level,
		Message:   message,
		Metric:    metric,
		Value:     value,
		Threshold: threshold,
		Timestamp: time.Now(),
		Resolved:  false,
	}
	
	mm.mutex.Lock()
	mm.alerts = append(mm.alerts, alert)
	
	// Keep only recent alerts (last 100)
	if len(mm.alerts) > 100 {
		mm.alerts = mm.alerts[1:]
	}
	mm.mutex.Unlock()
	
	// Notify callbacks
	for _, callback := range mm.alertCallbacks {
		go callback(alert)
	}
}

// performAutoCleanup performs automatic cleanup based on thresholds
func (mm *MemoryMonitor) performAutoCleanup() {
	pressure := mm.GetMemoryPressure()
	
	// Execute cleanup strategies based on pressure level
	for _, strategy := range mm.cleanupStrategies {
		if pressure >= float64(strategy.Threshold) {
			if err := strategy.Execute(); err != nil {
				mm.triggerAlert(AlertWarning, fmt.Sprintf("Cleanup strategy failed: %s", strategy.Name), "cleanup_error", 0, 0)
			} else {
				mm.triggerAlert(AlertInfo, fmt.Sprintf("Cleanup strategy executed: %s", strategy.Name), "cleanup_success", 0, 0)
			}
		}
	}
}

// initCleanupStrategies initializes available cleanup strategies
func (mm *MemoryMonitor) initCleanupStrategies() {
	mm.cleanupStrategies = []CleanupStrategy{
		{
			Name:        "Cache Cleanup",
			Priority:    1,
			Threshold:   mm.config.CleanupThresholds.CacheCleanup,
			Execute:     mm.cleanupCache,
			Description: "Cleans up least recently used cache entries",
		},
		{
			Name:        "Force Garbage Collection",
			Priority:    2,
			Threshold:   mm.config.CleanupThresholds.ForceGC,
			Execute:     mm.forceGC,
			Description: "Forces garbage collection to free unused memory",
		},
		{
			Name:        "Aggressive Cache Cleanup",
			Priority:    3,
			Threshold:   mm.config.CleanupThresholds.AggressiveCleanup,
			Execute:     mm.aggressiveCacheCleanup,
			Description: "Aggressively cleans cache and loaded data",
		},
		{
			Name:        "Emergency Memory Cleanup",
			Priority:    4,
			Threshold:   95,
			Execute:     mm.emergencyCleanup,
			Description: "Emergency cleanup of all non-essential data",
		},
	}
}

// cleanupCache performs cache cleanup
func (mm *MemoryMonitor) cleanupCache() error {
	if mm.cache == nil {
		return nil
	}
	
	// Reduce cache size by 25%
	newLimit := mm.config.MemoryLimitMB * 75 / 100
	mm.cache.SetMemoryLimit(newLimit)
	
	// Trigger cache cleanup
	runtime.GC()
	
	return nil
}

// forceGC performs forced garbage collection
func (mm *MemoryMonitor) forceGC() error {
	runtime.GC()
	debug.FreeOSMemory()
	return nil
}

// aggressiveCacheCleanup performs aggressive cache cleanup
func (mm *MemoryMonitor) aggressiveCacheCleanup() error {
	if mm.cache != nil {
		// Clear 50% of cache
		newLimit := mm.config.MemoryLimitMB * 50 / 100
		mm.cache.SetMemoryLimit(newLimit)
	}
	
	if mm.lazyLoader != nil {
		// Cleanup old lazy loaded data
		// This would need implementation in the lazy loader
	}
	
	runtime.GC()
	debug.FreeOSMemory()
	
	return nil
}

// emergencyCleanup performs emergency memory cleanup
func (mm *MemoryMonitor) emergencyCleanup() error {
	if mm.cache != nil {
		// Clear most of the cache
		mm.cache.Clear()
	}
	
	// Force multiple GC cycles
	for i := 0; i < 3; i++ {
		runtime.GC()
	}
	
	debug.FreeOSMemory()
	
	return nil
}

// configureGC configures garbage collector for optimal performance
func (mm *MemoryMonitor) configureGC() {
	// Set GC target percentage
	debug.SetGCPercent(mm.config.GCTriggerPercent)
	
	// Set memory limit if specified
	if mm.config.MemoryLimitMB > 0 {
		debug.SetMemoryLimit(mm.config.MemoryLimitMB * 1024 * 1024)
	}
}

// GetMemoryPressure returns current memory pressure (0-100%)
func (mm *MemoryMonitor) GetMemoryPressure() float64 {
	mm.metrics.mutex.RLock()
	defer mm.metrics.mutex.RUnlock()
	return mm.metrics.MemoryPressure
}

// GetMetrics returns current memory metrics
func (mm *MemoryMonitor) GetMetrics() MemoryMetrics {
	mm.metrics.mutex.RLock()
	defer mm.metrics.mutex.RUnlock()
	
	metrics := *mm.metrics // Copy struct
	return metrics
}

// GetAlerts returns recent memory alerts
func (mm *MemoryMonitor) GetAlerts() []MemoryAlert {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	
	alerts := make([]MemoryAlert, len(mm.alerts))
	copy(alerts, mm.alerts)
	return alerts
}

// AddAlertCallback adds a callback for memory alerts
func (mm *MemoryMonitor) AddAlertCallback(callback func(MemoryAlert)) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	
	mm.alertCallbacks = append(mm.alertCallbacks, callback)
}

// IsHealthy checks if memory usage is within acceptable limits
func (mm *MemoryMonitor) IsHealthy() bool {
	pressure := mm.GetMemoryPressure()
	return pressure < float64(mm.config.AlertThresholds.CriticalPercent)
}

// GetMemoryUsageReport generates a detailed memory usage report
func (mm *MemoryMonitor) GetMemoryUsageReport() MemoryUsageReport {
	metrics := mm.GetMetrics()
	alerts := mm.GetAlerts()
	
	// Count alerts by level
	var warningCount, criticalCount, emergencyCount int
	for _, alert := range alerts {
		if alert.Timestamp.After(time.Now().Add(-time.Hour)) { // Last hour only
			switch alert.Level {
			case AlertWarning:
				warningCount++
			case AlertCritical:
				criticalCount++
			case AlertEmergency:
				emergencyCount++
			}
		}
	}
	
	return MemoryUsageReport{
		Metrics:         metrics,
		RecentAlerts:    alerts,
		WarningCount:    warningCount,
		CriticalCount:   criticalCount,
		EmergencyCount:  emergencyCount,
		IsHealthy:       mm.IsHealthy(),
		Recommendations: mm.generateRecommendations(metrics),
		GeneratedAt:     time.Now(),
	}
}

// MemoryUsageReport represents a comprehensive memory usage report
type MemoryUsageReport struct {
	Metrics         MemoryMetrics `json:"metrics"`
	RecentAlerts    []MemoryAlert `json:"recent_alerts"`
	WarningCount    int           `json:"warning_count"`
	CriticalCount   int           `json:"critical_count"`
	EmergencyCount  int           `json:"emergency_count"`
	IsHealthy       bool          `json:"is_healthy"`
	Recommendations []string      `json:"recommendations"`
	GeneratedAt     time.Time     `json:"generated_at"`
}

// generateRecommendations generates memory optimization recommendations
func (mm *MemoryMonitor) generateRecommendations(metrics MemoryMetrics) []string {
	var recommendations []string
	
	if metrics.MemoryPressure > 80 {
		recommendations = append(recommendations, "Memory pressure is high. Consider increasing memory limit or enabling more aggressive cleanup.")
	}
	
	if metrics.GCEfficiency < 70 {
		recommendations = append(recommendations, "Garbage collection efficiency is low. Consider tuning GC parameters.")
	}
	
	if metrics.CacheMemory > metrics.HeapInUse/2 {
		recommendations = append(recommendations, "Cache is using significant memory. Consider reducing cache size limits.")
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Memory usage appears optimal.")
	}
	
	return recommendations
}

// String returns a string representation of alert level
func (al AlertLevel) String() string {
	switch al {
	case AlertInfo:
		return "INFO"
	case AlertWarning:
		return "WARNING"
	case AlertCritical:
		return "CRITICAL"
	case AlertEmergency:
		return "EMERGENCY"
	default:
		return "UNKNOWN"
	}
}