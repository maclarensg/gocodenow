package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DiagnosticLevel represents the level of diagnostic information
type DiagnosticLevel int

const (
	DiagnosticLevelBasic DiagnosticLevel = iota
	DiagnosticLevelDetailed
	DiagnosticLevelVerbose
	DiagnosticLevelDebug
)

// DiagnosticReport contains comprehensive diagnostic information
type DiagnosticReport struct {
	GeneratedAt      time.Time                    `json:"generated_at"`
	ReportID         string                       `json:"report_id"`
	Level            DiagnosticLevel              `json:"level"`
	SystemInfo       SystemInfo                   `json:"system_info"`
	ApplicationInfo  ApplicationInfo              `json:"application_info"`
	PerformanceInfo  PerformanceInfo              `json:"performance_info"`
	ErrorSummary     ErrorSummary                 `json:"error_summary"`
	ConfigurationInfo ConfigurationInfo           `json:"configuration_info"`
	DependencyInfo   DependencyInfo               `json:"dependency_info"`
	LogSummary       LogSummary                   `json:"log_summary"`
	NetworkInfo      NetworkInfo                  `json:"network_info"`
	FileSystemInfo   FileSystemInfo               `json:"file_system_info"`
	Recommendations  []Recommendation             `json:"recommendations"`
	HealthScore      int                          `json:"health_score"` // 0-100
	Metadata         map[string]interface{}       `json:"metadata"`
}

// SystemInfo contains system-level diagnostic information
type SystemInfo struct {
	OS              string            `json:"os"`
	Architecture    string            `json:"architecture"`
	GoVersion       string            `json:"go_version"`
	NumCPU          int               `json:"num_cpu"`
	TotalMemory     uint64            `json:"total_memory"`
	AvailableMemory uint64            `json:"available_memory"`
	DiskSpace       map[string]uint64 `json:"disk_space"`
	Environment     map[string]string `json:"environment"`
	Hostname        string            `json:"hostname"`
	Username        string            `json:"username"`
	ProcessID       int               `json:"process_id"`
	ParentPID       int               `json:"parent_pid"`
	WorkingDir      string            `json:"working_dir"`
	ExecutablePath  string            `json:"executable_path"`
}

// ApplicationInfo contains application-specific diagnostic information
type ApplicationInfo struct {
	Version         string        `json:"version"`
	BuildTime       time.Time     `json:"build_time"`
	StartTime       time.Time     `json:"start_time"`
	Uptime          time.Duration `json:"uptime"`
	ConfigPath      string        `json:"config_path"`
	DatabasePath    string        `json:"database_path"`
	SessionID       string        `json:"session_id"`
	ActiveFeatures  []string      `json:"active_features"`
	LoadedModules   []string      `json:"loaded_modules"`
	CommandLineArgs []string      `json:"command_line_args"`
}

// PerformanceInfo contains performance-related diagnostic information
type PerformanceInfo struct {
	MemoryStats     runtime.MemStats              `json:"memory_stats"`
	GoroutineCount  int                           `json:"goroutine_count"`
	CGOCalls        int64                         `json:"cgo_calls"`
	GCStats         GCStats                       `json:"gc_stats"`
	CPUUsage        float64                       `json:"cpu_usage"`
	DiskIO          DiskIOStats                   `json:"disk_io"`
	NetworkIO       NetworkIOStats                `json:"network_io"`
	ResponseTimes   map[string]time.Duration      `json:"response_times"`
	ThroughputStats map[string]float64            `json:"throughput_stats"`
	ResourceLimits  ResourceLimits                `json:"resource_limits"`
}

// GCStats contains garbage collection statistics
type GCStats struct {
	NumGC       uint32        `json:"num_gc"`
	TotalPause  time.Duration `json:"total_pause"`
	LastPause   time.Duration `json:"last_pause"`
	AvgPause    time.Duration `json:"avg_pause"`
	MaxPause    time.Duration `json:"max_pause"`
	GCCPUFraction float64      `json:"gc_cpu_fraction"`
}

// DiskIOStats contains disk I/O statistics
type DiskIOStats struct {
	ReadBytes    uint64 `json:"read_bytes"`
	WriteBytes   uint64 `json:"write_bytes"`
	ReadOps      uint64 `json:"read_ops"`
	WriteOps     uint64 `json:"write_ops"`
	AvgReadTime  time.Duration `json:"avg_read_time"`
	AvgWriteTime time.Duration `json:"avg_write_time"`
}

// NetworkIOStats contains network I/O statistics
type NetworkIOStats struct {
	BytesReceived uint64 `json:"bytes_received"`
	BytesSent     uint64 `json:"bytes_sent"`
	PacketsReceived uint64 `json:"packets_received"`
	PacketsSent   uint64 `json:"packets_sent"`
	Connections   int    `json:"connections"`
	FailedConnections int `json:"failed_connections"`
}

// ResourceLimits contains resource limit information
type ResourceLimits struct {
	MaxMemory    uint64        `json:"max_memory"`
	MaxCPU       float64       `json:"max_cpu"`
	MaxFileHandles int         `json:"max_file_handles"`
	MaxConnections int         `json:"max_connections"`
	TimeoutLimits map[string]time.Duration `json:"timeout_limits"`
}

// ErrorSummary contains error analysis information
type ErrorSummary struct {
	TotalErrors       int64                    `json:"total_errors"`
	ErrorsByCategory  map[string]int64         `json:"errors_by_category"`
	ErrorsByComponent map[string]int64         `json:"errors_by_component"`
	RecentErrors      []ErrorInfo              `json:"recent_errors"`
	CriticalErrors    []ErrorInfo              `json:"critical_errors"`
	RecoveryRate      float64                  `json:"recovery_rate"`
	FailurePatterns   []FailurePattern         `json:"failure_patterns"`
	ErrorTrends       map[string][]int64       `json:"error_trends"`
}

// ErrorInfo contains detailed information about an error
type ErrorInfo struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	Message      string                 `json:"message"`
	Category     string                 `json:"category"`
	Component    string                 `json:"component"`
	Operation    string                 `json:"operation"`
	Severity     int                    `json:"severity"`
	StackTrace   string                 `json:"stack_trace"`
	Context      map[string]interface{} `json:"context"`
	RecoveryAttempts int                `json:"recovery_attempts"`
	Resolved     bool                   `json:"resolved"`
}

// FailurePattern represents a detected failure pattern
type FailurePattern struct {
	Pattern     string    `json:"pattern"`
	Frequency   int       `json:"frequency"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Confidence  float64   `json:"confidence"`
	Impact      string    `json:"impact"`
	Mitigation  string    `json:"mitigation"`
}

// ConfigurationInfo contains configuration diagnostic information
type ConfigurationInfo struct {
	ConfigValid       bool                   `json:"config_valid"`
	ConfigVersion     int                    `json:"config_version"`
	ConfigPath        string                 `json:"config_path"`
	ConfigSize        int64                  `json:"config_size"`
	LastModified      time.Time              `json:"last_modified"`
	ValidationErrors  []string               `json:"validation_errors"`
	MissingKeys       []string               `json:"missing_keys"`
	DeprecatedKeys    []string               `json:"deprecated_keys"`
	ConfigSnapshot    map[string]interface{} `json:"config_snapshot"`
	ProfilesAvailable []string               `json:"profiles_available"`
	ActiveProfile     string                 `json:"active_profile"`
}

// DependencyInfo contains dependency diagnostic information
type DependencyInfo struct {
	GoModules       []ModuleInfo      `json:"go_modules"`
	SystemLibraries []LibraryInfo     `json:"system_libraries"`
	ExternalTools   []ToolInfo        `json:"external_tools"`
	APIEndpoints    []EndpointInfo    `json:"api_endpoints"`
	MissingDeps     []string          `json:"missing_deps"`
	OutdatedDeps    []string          `json:"outdated_deps"`
	VulnerableDeps  []VulnerabilityInfo `json:"vulnerable_deps"`
}

// ModuleInfo contains Go module information
type ModuleInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Path     string `json:"path"`
	Required bool   `json:"required"`
	Indirect bool   `json:"indirect"`
}

// LibraryInfo contains system library information
type LibraryInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

// ToolInfo contains external tool information
type ToolInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	Available bool   `json:"available"`
	Required  bool   `json:"required"`
}

// EndpointInfo contains API endpoint information
type EndpointInfo struct {
	URL         string        `json:"url"`
	Status      string        `json:"status"`
	ResponseTime time.Duration `json:"response_time"`
	LastChecked  time.Time     `json:"last_checked"`
	Error       string        `json:"error,omitempty"`
}

// VulnerabilityInfo contains vulnerability information
type VulnerabilityInfo struct {
	Module      string `json:"module"`
	Version     string `json:"version"`
	CVEID       string `json:"cve_id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	FixedIn     string `json:"fixed_in"`
}

// LogSummary contains log analysis information
type LogSummary struct {
	LogFiles      []LogFileInfo            `json:"log_files"`
	TotalEntries  int64                    `json:"total_entries"`
	ErrorEntries  int64                    `json:"error_entries"`
	WarnEntries   int64                    `json:"warn_entries"`
	RecentEntries []LogEntry               `json:"recent_entries"`
	LogPatterns   []LogPattern             `json:"log_patterns"`
	LogHealth     string                   `json:"log_health"`
}

// LogFileInfo contains log file information
type LogFileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	LineCount    int64     `json:"line_count"`
	Readable     bool      `json:"readable"`
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
	Context   map[string]interface{} `json:"context"`
}

// LogPattern represents a detected log pattern
type LogPattern struct {
	Pattern   string `json:"pattern"`
	Count     int64  `json:"count"`
	Level     string `json:"level"`
	Component string `json:"component"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// NetworkInfo contains network diagnostic information
type NetworkInfo struct {
	Interfaces    []NetworkInterface `json:"interfaces"`
	DNSServers    []string          `json:"dns_servers"`
	Connectivity  []ConnectivityTest `json:"connectivity"`
	Latency       map[string]time.Duration `json:"latency"`
	Bandwidth     map[string]float64 `json:"bandwidth"`
	ActiveConnections int            `json:"active_connections"`
}

// NetworkInterface contains network interface information
type NetworkInterface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	MTU       int      `json:"mtu"`
	Up        bool     `json:"up"`
	Loopback  bool     `json:"loopback"`
}

// ConnectivityTest contains connectivity test results
type ConnectivityTest struct {
	Target      string        `json:"target"`
	Success     bool          `json:"success"`
	Latency     time.Duration `json:"latency"`
	Error       string        `json:"error,omitempty"`
	TestedAt    time.Time     `json:"tested_at"`
}

// FileSystemInfo contains file system diagnostic information
type FileSystemInfo struct {
	Mounts        []MountInfo  `json:"mounts"`
	DiskUsage     []DiskUsage  `json:"disk_usage"`
	FileHandles   FileHandleInfo `json:"file_handles"`
	Permissions   []PermissionInfo `json:"permissions"`
	TempDirs      []string     `json:"temp_dirs"`
	CacheSize     int64        `json:"cache_size"`
}

// MountInfo contains mount point information
type MountInfo struct {
	Device     string `json:"device"`
	MountPoint string `json:"mount_point"`
	FileSystem string `json:"file_system"`
	Options    string `json:"options"`
}

// DiskUsage contains disk usage information
type DiskUsage struct {
	Path        string  `json:"path"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	UsagePercent float64 `json:"usage_percent"`
}

// FileHandleInfo contains file handle information
type FileHandleInfo struct {
	Open      int `json:"open"`
	Max       int `json:"max"`
	Available int `json:"available"`
}

// PermissionInfo contains file permission information
type PermissionInfo struct {
	Path        string `json:"path"`
	Readable    bool   `json:"readable"`
	Writable    bool   `json:"writable"`
	Executable  bool   `json:"executable"`
	Owner       string `json:"owner"`
	Group       string `json:"group"`
}

// Recommendation contains a system recommendation
type Recommendation struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // performance, security, maintenance, etc.
	Priority    string `json:"priority"` // high, medium, low
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"`
}

// DiagnosticCollector collects diagnostic information
type DiagnosticCollector struct {
	level           DiagnosticLevel
	errorManager    *ErrorRecoveryManager
	crashManager    *CrashRecoveryManager
	collectors      map[string]func(context.Context) (interface{}, error)
	mutex           sync.RWMutex
}

// NewDiagnosticCollector creates a new diagnostic collector
func NewDiagnosticCollector(level DiagnosticLevel) *DiagnosticCollector {
	dc := &DiagnosticCollector{
		level:      level,
		collectors: make(map[string]func(context.Context) (interface{}, error)),
	}
	
	// Register default collectors
	dc.registerDefaultCollectors()
	
	return dc
}

// SetErrorManager sets the error recovery manager
func (dc *DiagnosticCollector) SetErrorManager(erm *ErrorRecoveryManager) {
	dc.errorManager = erm
}

// SetCrashManager sets the crash recovery manager
func (dc *DiagnosticCollector) SetCrashManager(crm *CrashRecoveryManager) {
	dc.crashManager = crm
}

// RegisterCollector registers a custom diagnostic collector
func (dc *DiagnosticCollector) RegisterCollector(name string, collector func(context.Context) (interface{}, error)) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()
	
	dc.collectors[name] = collector
}

// registerDefaultCollectors registers the built-in collectors
func (dc *DiagnosticCollector) registerDefaultCollectors() {
	dc.collectors["system_info"] = dc.collectSystemInfo
	dc.collectors["performance_info"] = dc.collectPerformanceInfo
	dc.collectors["error_summary"] = dc.collectErrorSummary
	dc.collectors["configuration_info"] = dc.collectConfigurationInfo
	dc.collectors["dependency_info"] = dc.collectDependencyInfo
	dc.collectors["log_summary"] = dc.collectLogSummary
	dc.collectors["network_info"] = dc.collectNetworkInfo
	dc.collectors["file_system_info"] = dc.collectFileSystemInfo
}

// GenerateReport generates a comprehensive diagnostic report
func (dc *DiagnosticCollector) GenerateReport(ctx context.Context) (*DiagnosticReport, error) {
	report := &DiagnosticReport{
		GeneratedAt: time.Now(),
		ReportID:    fmt.Sprintf("diag_%d", time.Now().Unix()),
		Level:       dc.level,
		Metadata:    make(map[string]interface{}),
	}
	
	// Collect all diagnostic information
	for name, collector := range dc.collectors {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		data, err := collector(ctx)
		if err != nil {
			report.Metadata[name+"_error"] = err.Error()
			continue
		}
		
		// Store collected data in appropriate field
		switch name {
		case "system_info":
			if info, ok := data.(SystemInfo); ok {
				report.SystemInfo = info
			}
		case "performance_info":
			if info, ok := data.(PerformanceInfo); ok {
				report.PerformanceInfo = info
			}
		case "error_summary":
			if info, ok := data.(ErrorSummary); ok {
				report.ErrorSummary = info
			}
		case "configuration_info":
			if info, ok := data.(ConfigurationInfo); ok {
				report.ConfigurationInfo = info
			}
		case "dependency_info":
			if info, ok := data.(DependencyInfo); ok {
				report.DependencyInfo = info
			}
		case "log_summary":
			if info, ok := data.(LogSummary); ok {
				report.LogSummary = info
			}
		case "network_info":
			if info, ok := data.(NetworkInfo); ok {
				report.NetworkInfo = info
			}
		case "file_system_info":
			if info, ok := data.(FileSystemInfo); ok {
				report.FileSystemInfo = info
			}
		default:
			report.Metadata[name] = data
		}
	}
	
	// Generate recommendations
	report.Recommendations = dc.generateRecommendations(report)
	
	// Calculate health score
	report.HealthScore = dc.calculateHealthScore(report)
	
	return report, nil
}

// collectSystemInfo collects system information
func (dc *DiagnosticCollector) collectSystemInfo(ctx context.Context) (interface{}, error) {
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	wd, _ := os.Getwd()
	execPath, _ := os.Executable()
	
	env := make(map[string]string)
	if dc.level >= DiagnosticLevelDetailed {
		for _, envVar := range os.Environ() {
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) == 2 {
				// Don't include sensitive environment variables
				if !isSensitiveEnvVar(parts[0]) {
					env[parts[0]] = parts[1]
				}
			}
		}
	}
	
	return SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		Environment:  env,
		Hostname:     hostname,
		Username:     username,
		ProcessID:    os.Getpid(),
		ParentPID:    os.Getppid(),
		WorkingDir:   wd,
		ExecutablePath: execPath,
	}, nil
}

// collectPerformanceInfo collects performance information
func (dc *DiagnosticCollector) collectPerformanceInfo(ctx context.Context) (interface{}, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	gcStats := GCStats{
		NumGC:        m.NumGC,
		TotalPause:   time.Duration(m.PauseTotalNs),
		GCCPUFraction: m.GCCPUFraction,
	}
	
	if m.NumGC > 0 {
		gcStats.LastPause = time.Duration(m.PauseNs[(m.NumGC+255)%256])
		gcStats.AvgPause = gcStats.TotalPause / time.Duration(m.NumGC)
		
		// Find max pause
		var maxPause uint64
		for i := uint32(0); i < m.NumGC && i < 256; i++ {
			pause := m.PauseNs[i]
			if pause > maxPause {
				maxPause = pause
			}
		}
		gcStats.MaxPause = time.Duration(maxPause)
	}
	
	return PerformanceInfo{
		MemoryStats:    m,
		GoroutineCount: runtime.NumGoroutine(),
		CGOCalls:       runtime.NumCgoCall(),
		GCStats:        gcStats,
	}, nil
}

// collectErrorSummary collects error summary information
func (dc *DiagnosticCollector) collectErrorSummary(ctx context.Context) (interface{}, error) {
	if dc.errorManager == nil {
		return ErrorSummary{}, nil
	}
	
	metrics := dc.errorManager.GetMetrics()
	history := dc.errorManager.GetErrorHistory()
	
	// Convert error history to ErrorInfo
	recentErrors := make([]ErrorInfo, 0, len(history))
	criticalErrors := make([]ErrorInfo, 0)
	
	for _, err := range history {
		errorInfo := ErrorInfo{
			ID:        fmt.Sprintf("err_%d", err.Timestamp.Unix()),
			Timestamp: err.Timestamp,
			Message:   err.OriginalError.Error(),
			Category:  err.categoryString(),
			Component: err.Component,
			Operation: err.Operation,
			Severity:  err.Severity,
			Context:   err.Context,
			RecoveryAttempts: len(err.AttemptedActions),
		}
		
		recentErrors = append(recentErrors, errorInfo)
		
		if err.Severity >= 8 {
			criticalErrors = append(criticalErrors, errorInfo)
		}
	}
	
	// Calculate recovery rate
	recoveryRate := float64(0)
	if metrics.TotalErrors > 0 {
		recoveryRate = float64(metrics.RecoveredErrors) / float64(metrics.TotalErrors) * 100
	}
	
	// Convert category map
	errorsByCategory := make(map[string]int64)
	for category, count := range metrics.ErrorsByCategory {
		switch category {
		case CriticalError:
			errorsByCategory["critical"] = count
		case RecoverableErrorCategory:
			errorsByCategory["recoverable"] = count
		case TransientError:
			errorsByCategory["transient"] = count
		case UserError:
			errorsByCategory["user"] = count
		case SystemError:
			errorsByCategory["system"] = count
		}
	}
	
	return ErrorSummary{
		TotalErrors:       metrics.TotalErrors,
		ErrorsByCategory:  errorsByCategory,
		ErrorsByComponent: metrics.ErrorsByComponent,
		RecentErrors:      recentErrors,
		CriticalErrors:    criticalErrors,
		RecoveryRate:      recoveryRate,
	}, nil
}

// collectConfigurationInfo collects configuration information
func (dc *DiagnosticCollector) collectConfigurationInfo(ctx context.Context) (interface{}, error) {
	// This would integrate with the configuration system
	// For now, return basic information
	return ConfigurationInfo{
		ConfigValid: true,
		ConfigVersion: 3,
	}, nil
}

// collectDependencyInfo collects dependency information
func (dc *DiagnosticCollector) collectDependencyInfo(ctx context.Context) (interface{}, error) {
	// This would analyze Go modules and system dependencies
	// For now, return basic information
	return DependencyInfo{
		GoModules: []ModuleInfo{},
		SystemLibraries: []LibraryInfo{},
		ExternalTools: []ToolInfo{},
		APIEndpoints: []EndpointInfo{},
	}, nil
}

// collectLogSummary collects log summary information
func (dc *DiagnosticCollector) collectLogSummary(ctx context.Context) (interface{}, error) {
	// This would analyze log files
	// For now, return basic information
	return LogSummary{
		LogFiles: []LogFileInfo{},
		TotalEntries: 0,
		ErrorEntries: 0,
		WarnEntries: 0,
		RecentEntries: []LogEntry{},
		LogPatterns: []LogPattern{},
		LogHealth: "unknown",
	}, nil
}

// collectNetworkInfo collects network information
func (dc *DiagnosticCollector) collectNetworkInfo(ctx context.Context) (interface{}, error) {
	// This would collect network interface and connectivity information
	// For now, return basic information
	return NetworkInfo{
		Interfaces: []NetworkInterface{},
		DNSServers: []string{},
		Connectivity: []ConnectivityTest{},
		ActiveConnections: 0,
	}, nil
}

// collectFileSystemInfo collects file system information
func (dc *DiagnosticCollector) collectFileSystemInfo(ctx context.Context) (interface{}, error) {
	// This would collect file system information
	// For now, return basic information
	return FileSystemInfo{
		Mounts: []MountInfo{},
		DiskUsage: []DiskUsage{},
		FileHandles: FileHandleInfo{},
		Permissions: []PermissionInfo{},
		TempDirs: []string{},
		CacheSize: 0,
	}, nil
}

// generateRecommendations generates recommendations based on the diagnostic data
func (dc *DiagnosticCollector) generateRecommendations(report *DiagnosticReport) []Recommendation {
	var recommendations []Recommendation
	
	// Memory usage recommendations
	if report.PerformanceInfo.MemoryStats.Alloc > 100*1024*1024 { // > 100MB
		recommendations = append(recommendations, Recommendation{
			ID:          "high_memory_usage",
			Type:        "performance",
			Priority:    "medium",
			Title:       "High Memory Usage Detected",
			Description: "Application is using more than 100MB of memory",
			Action:      "Consider reducing cache size or optimizing memory usage",
			Impact:      "May cause performance degradation or system instability",
			Effort:      "medium",
		})
	}
	
	// Error rate recommendations
	if report.ErrorSummary.RecoveryRate < 80 {
		recommendations = append(recommendations, Recommendation{
			ID:          "low_recovery_rate",
			Type:        "reliability",
			Priority:    "high",
			Title:       "Low Error Recovery Rate",
			Description: fmt.Sprintf("Error recovery rate is %.1f%%, which is below optimal", report.ErrorSummary.RecoveryRate),
			Action:      "Review error handling and recovery mechanisms",
			Impact:      "Reduced application stability and user experience",
			Effort:      "high",
		})
	}
	
	// Critical error recommendations
	if len(report.ErrorSummary.CriticalErrors) > 0 {
		recommendations = append(recommendations, Recommendation{
			ID:          "critical_errors",
			Type:        "reliability",
			Priority:    "high",
			Title:       "Critical Errors Detected",
			Description: fmt.Sprintf("Found %d critical errors that require immediate attention", len(report.ErrorSummary.CriticalErrors)),
			Action:      "Investigate and resolve critical errors immediately",
			Impact:      "Application may crash or lose data",
			Effort:      "high",
		})
	}
	
	return recommendations
}

// calculateHealthScore calculates an overall health score
func (dc *DiagnosticCollector) calculateHealthScore(report *DiagnosticReport) int {
	score := 100
	
	// Deduct points for critical errors
	if len(report.ErrorSummary.CriticalErrors) > 0 {
		score -= len(report.ErrorSummary.CriticalErrors) * 10
	}
	
	// Deduct points for low recovery rate
	if report.ErrorSummary.RecoveryRate < 80 {
		score -= int(80 - report.ErrorSummary.RecoveryRate)
	}
	
	// Deduct points for high memory usage
	memUsageGB := float64(report.PerformanceInfo.MemoryStats.Alloc) / (1024 * 1024 * 1024)
	if memUsageGB > 1 {
		score -= int(memUsageGB * 5)
	}
	
	// Ensure score is between 0 and 100
	if score < 0 {
		score = 0
	}
	
	return score
}

// SaveReport saves a diagnostic report to a file
func (dc *DiagnosticCollector) SaveReport(report *DiagnosticReport, filePath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}
	
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}
	
	return nil
}

// isSensitiveEnvVar checks if an environment variable contains sensitive information
func isSensitiveEnvVar(name string) bool {
	sensitiveVars := []string{
		"PASSWORD", "SECRET", "TOKEN", "KEY", "API_KEY",
		"PRIVATE", "CREDENTIAL", "AUTH", "CERT", "SSL",
	}
	
	upperName := strings.ToUpper(name)
	for _, sensitive := range sensitiveVars {
		if strings.Contains(upperName, sensitive) {
			return true
		}
	}
	
	return false
}