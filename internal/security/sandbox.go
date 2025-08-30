package security

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/process"
)

// ExecutionConfig configures process execution within sandbox
type ExecutionConfig struct {
	MaxMemoryMB       int           `json:"max_memory_mb"`
	MaxCPUPercent     float64       `json:"max_cpu_percent"`
	MaxExecutionTime  time.Duration `json:"max_execution_time"`
	AllowNetworking   bool          `json:"allow_networking"`
	AllowFileWrite    bool          `json:"allow_file_write"`
	EnableLogging     bool          `json:"enable_logging"`
	ContainerImage    string        `json:"container_image"`
	UseNamespaces     bool          `json:"use_namespaces"`
	ReadOnlyRootFS    bool          `json:"readonly_rootfs"`
	DropCapabilities  []string      `json:"drop_capabilities"`
	SecurityProfile   string        `json:"security_profile"`
}

// ExecutionResult contains the result of sandbox execution
type ExecutionResult struct {
	Stdout       string        `json:"stdout"`
	Stderr       string        `json:"stderr"`
	ExitCode     int           `json:"exit_code"`
	ExecutionTime time.Duration `json:"execution_time"`
	MemoryUsed   uint64        `json:"memory_used"`
	CPUUsed      float64       `json:"cpu_used"`
	Violations   []string      `json:"violations"`
	Sandboxed    bool          `json:"sandboxed"`
	ProcessID    int           `json:"process_id"`
}

// ProcessMonitor monitors resource usage of sandbox processes
type ProcessMonitor struct {
	pid          int32
	process      *process.Process
	maxMemory    uint64
	maxCPU       float64
	violations   []string
	mu           sync.RWMutex
	stopChan     chan bool
	violationsCh chan string
}

// ExecutionMetrics tracks sandbox execution statistics
type ExecutionMetrics struct {
	TotalExecutions    int64         `json:"total_executions"`
	SuccessfulRuns     int64         `json:"successful_runs"`
	FailedRuns         int64         `json:"failed_runs"`
	SecurityViolations int64         `json:"security_violations"`
	AverageExecTime    time.Duration `json:"average_exec_time"`
	PeakMemoryUsage    uint64        `json:"peak_memory_usage"`
	mu                 sync.RWMutex
}

// SecurityLogger logs security events
type SecurityLogger struct {
	logFile    *os.File
	mu         sync.Mutex
	enableLogs bool
}

// SandboxEnvironment provides isolated execution environment
type SandboxEnvironment struct {
	policy           *SecurityPolicy
	rootDir          string
	tempDir          string
	cleanupFn        func() error
	execConfig       ExecutionConfig
	activeProcesses  map[int32]*ProcessMonitor
	mu               sync.RWMutex
	logger           *SecurityLogger
	metrics          *ExecutionMetrics
}

// SandboxConfig configures sandbox creation
type SandboxConfig struct {
	RootDir          string          // Root directory for sandbox (if empty, creates temp)
	TempDir          string          // Temporary directory for operations
	Permissions      os.FileMode     // Directory permissions
	Timeout          time.Duration   // Sandbox lifetime timeout
	ExecutionConfig  ExecutionConfig // Configuration for process execution
}

// NewSecurityLogger creates a new security logger
func NewSecurityLogger(logPath string) (*SecurityLogger, error) {
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	return &SecurityLogger{
		logFile:    file,
		enableLogs: true,
	}, nil
}

// Log writes a message to the security log
func (sl *SecurityLogger) Log(message string) {
	if !sl.enableLogs || sl.logFile == nil {
		return
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] %s\n", timestamp, message)
	sl.logFile.WriteString(logEntry)
	sl.logFile.Sync()
}

// Close closes the security logger
func (sl *SecurityLogger) Close() error {
	if sl.logFile != nil {
		return sl.logFile.Close()
	}
	return nil
}

// NewSandbox creates a new sandbox environment
func NewSandbox(policy *SecurityPolicy, config SandboxConfig) (*SandboxEnvironment, error) {
	if !policy.SandboxEnabled {
		return nil, fmt.Errorf("sandbox mode is not enabled in security policy")
	}
	
	var rootDir string
	var cleanupFn func() error
	
	if config.RootDir != "" {
		// Use provided root directory
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sandbox root path: %w", err)
		}
		
		// Ensure directory exists
		if err := os.MkdirAll(absRoot, 0755); err != nil {
			return nil, fmt.Errorf("failed to create sandbox root directory: %w", err)
		}
		
		rootDir = absRoot
		cleanupFn = func() error { return nil } // Don't cleanup user-provided directory
	} else {
		// Create temporary sandbox directory in project's tmp folder
		projectTmp := "./tmp"
		if err := os.MkdirAll(projectTmp, 0755); err != nil {
			return nil, fmt.Errorf("failed to create project tmp directory: %w", err)
		}
		
		tempRoot, err := os.MkdirTemp(projectTmp, "gocodenow-sandbox-")
		if err != nil {
			return nil, fmt.Errorf("failed to create sandbox temp directory: %w", err)
		}
		
		rootDir = tempRoot
		cleanupFn = func() error {
			return os.RemoveAll(tempRoot)
		}
	}
	
	// Create temp directory within sandbox
	tempDir := filepath.Join(rootDir, "tmp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		if cleanupFn != nil {
			cleanupFn()
		}
		return nil, fmt.Errorf("failed to create sandbox temp directory: %w", err)
	}
	
	// Update policy to use sandbox directories
	sandboxPolicy := policy.Clone()
	sandboxPolicy.SandboxRoot = rootDir
	sandboxPolicy.TempDirectory = tempDir
	
	// For sandbox, we allow access to the sandbox root and remove blocking restrictions
	// The ResolvePath function itself ensures sandbox boundary enforcement
	sandboxPolicy.AllowedPaths = []string{rootDir}
	sandboxPolicy.BlockedPaths = []string{} // Remove path blocks within sandbox
	
	// Initialize logger if enabled
	var logger *SecurityLogger
	if config.ExecutionConfig.EnableLogging {
		var err error
		logger, err = NewSecurityLogger(filepath.Join(tempDir, "sandbox.log"))
		if err != nil {
			if cleanupFn != nil {
				cleanupFn()
			}
			return nil, fmt.Errorf("failed to create security logger: %w", err)
		}
	}
	
	sandbox := &SandboxEnvironment{
		policy:          sandboxPolicy,
		rootDir:         rootDir,
		tempDir:         tempDir,
		cleanupFn:       cleanupFn,
		execConfig:      config.ExecutionConfig,
		activeProcesses: make(map[int32]*ProcessMonitor),
		logger:          logger,
		metrics:         &ExecutionMetrics{},
	}
	
	// Setup timeout cleanup if specified
	if config.Timeout > 0 {
		go func() {
			time.Sleep(config.Timeout)
			sandbox.Cleanup()
		}()
	}
	
	return sandbox, nil
}

// GetPolicy returns the sandbox security policy
func (s *SandboxEnvironment) GetPolicy() *SecurityPolicy {
	return s.policy
}

// GetRootDir returns the sandbox root directory
func (s *SandboxEnvironment) GetRootDir() string {
	return s.rootDir
}

// GetTempDir returns the sandbox temporary directory
func (s *SandboxEnvironment) GetTempDir() string {
	return s.tempDir
}

// ResolvePath resolves a path within the sandbox
func (s *SandboxEnvironment) ResolvePath(path string) (string, error) {
	// Handle absolute paths by making them relative to sandbox
	if filepath.IsAbs(path) {
		// Strip leading slash and make relative to sandbox root
		path = strings.TrimPrefix(path, "/")
		if path == "" {
			path = "."
		}
	}
	
	// Resolve relative to sandbox root
	resolvedPath := filepath.Join(s.rootDir, path)
	
	// Clean the path to resolve any . or .. components
	cleanedPath := filepath.Clean(resolvedPath)
	
	// Ensure path is within sandbox (compare absolute paths)
	absRoot, err := filepath.Abs(s.rootDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve sandbox root: %w", err)
	}
	
	absResolved, err := filepath.Abs(cleanedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	
	if !strings.HasPrefix(absResolved, absRoot) {
		return "", fmt.Errorf("path %s escapes sandbox boundary", path)
	}
	
	// Validate path against security policy
	if err := s.policy.ValidatePath(cleanedPath); err != nil {
		return "", fmt.Errorf("sandbox path validation failed: %w", err)
	}
	
	return cleanedPath, nil
}

// CreateFile creates a file within the sandbox
func (s *SandboxEnvironment) CreateFile(path string, content []byte) error {
	resolvedPath, err := s.ResolvePath(path)
	if err != nil {
		return err
	}
	
	// Validate file size
	if err := s.policy.ValidateFileSize(int64(len(content))); err != nil {
		return err
	}
	
	// Ensure parent directory exists
	parentDir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	
	// Write file
	if err := os.WriteFile(resolvedPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}

// ReadFile reads a file from within the sandbox
func (s *SandboxEnvironment) ReadFile(path string) ([]byte, error) {
	resolvedPath, err := s.ResolvePath(path)
	if err != nil {
		return nil, err
	}
	
	// Check if file exists
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	
	// Validate file size
	if err := s.policy.ValidateFileSize(info.Size()); err != nil {
		return nil, err
	}
	
	// Read file
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	return content, nil
}

// ListFiles lists files in a sandbox directory
func (s *SandboxEnvironment) ListFiles(path string) ([]os.FileInfo, error) {
	resolvedPath, err := s.ResolvePath(path)
	if err != nil {
		return nil, err
	}
	
	// Read directory
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory: %w", err)
	}
	
	// Convert to FileInfo slice
	var fileInfos []os.FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip files with stat errors
		}
		fileInfos = append(fileInfos, info)
	}
	
	return fileInfos, nil
}

// CopyFromHost copies a file from the host system into the sandbox
func (s *SandboxEnvironment) CopyFromHost(hostPath, sandboxPath string) error {
	// Validate host path exists and is readable
	hostInfo, err := os.Stat(hostPath)
	if err != nil {
		return fmt.Errorf("host file not accessible: %w", err)
	}
	
	// Validate file size
	if err := s.policy.ValidateFileSize(hostInfo.Size()); err != nil {
		return err
	}
	
	// Read host file
	content, err := os.ReadFile(hostPath)
	if err != nil {
		return fmt.Errorf("failed to read host file: %w", err)
	}
	
	// Create file in sandbox
	return s.CreateFile(sandboxPath, content)
}

// CopyToHost copies a file from the sandbox to the host system
func (s *SandboxEnvironment) CopyToHost(sandboxPath, hostPath string) error {
	// Read sandbox file
	content, err := s.ReadFile(sandboxPath)
	if err != nil {
		return err
	}
	
	// Ensure host parent directory exists
	parentDir := filepath.Dir(hostPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create host parent directory: %w", err)
	}
	
	// Write to host
	if err := os.WriteFile(hostPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write host file: %w", err)
	}
	
	return nil
}

// Execute runs a command within the sandbox with full process isolation
func (s *SandboxEnvironment) Execute(ctx context.Context, command string, args []string, workDir string) (*ExecutionResult, error) {
	startTime := time.Now()
	
	s.metrics.mu.Lock()
	s.metrics.TotalExecutions++
	s.metrics.mu.Unlock()

	if s.logger != nil {
		s.logger.Log(fmt.Sprintf("Executing command: %s %v in directory: %s", command, args, workDir))
	}

	if err := s.validateCommand(command, args); err != nil {
		s.metrics.mu.Lock()
		s.metrics.FailedRuns++
		s.metrics.SecurityViolations++
		s.metrics.mu.Unlock()
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Resolve work directory within sandbox
	resolvedWorkDir, err := s.ResolvePath(workDir)
	if err != nil {
		s.metrics.mu.Lock()
		s.metrics.FailedRuns++
		s.metrics.SecurityViolations++
		s.metrics.mu.Unlock()
		return nil, fmt.Errorf("work directory validation failed: %w", err)
	}

	execCtx, cancel := context.WithTimeout(ctx, s.execConfig.MaxExecutionTime)
	defer cancel()

	var result *ExecutionResult

	if s.execConfig.ContainerImage != "" {
		result, err = s.executeInContainer(execCtx, command, args, resolvedWorkDir)
	} else if s.execConfig.UseNamespaces {
		result, err = s.executeWithNamespaces(execCtx, command, args, resolvedWorkDir)
	} else {
		result, err = s.executeWithLimits(execCtx, command, args, resolvedWorkDir)
	}

	executionTime := time.Since(startTime)
	
	if result != nil {
		result.ExecutionTime = executionTime
	}

	s.metrics.mu.Lock()
	if err != nil {
		s.metrics.FailedRuns++
	} else {
		s.metrics.SuccessfulRuns++
	}
	
	if s.metrics.TotalExecutions > 0 {
		totalTime := time.Duration(int64(s.metrics.AverageExecTime) * (s.metrics.TotalExecutions - 1) + int64(executionTime))
		s.metrics.AverageExecTime = totalTime / time.Duration(s.metrics.TotalExecutions)
	}
	
	if result != nil && result.MemoryUsed > s.metrics.PeakMemoryUsage {
		s.metrics.PeakMemoryUsage = result.MemoryUsed
	}
	s.metrics.mu.Unlock()

	if s.logger != nil {
		if err != nil {
			s.logger.Log(fmt.Sprintf("Command execution failed: %v", err))
		} else {
			s.logger.Log(fmt.Sprintf("Command executed successfully in %v", executionTime))
		}
	}

	return result, err
}

func (s *SandboxEnvironment) executeInContainer(ctx context.Context, command string, args []string, workDir string) (*ExecutionResult, error) {
	containerArgs := []string{
		"run", "--rm",
		"--memory", fmt.Sprintf("%dm", s.execConfig.MaxMemoryMB),
		"--cpus", fmt.Sprintf("%.2f", s.execConfig.MaxCPUPercent/100.0),
		"--workdir", "/workspace",
		"--volume", fmt.Sprintf("%s:/workspace", workDir),
	}

	if s.execConfig.ReadOnlyRootFS {
		containerArgs = append(containerArgs, "--read-only")
	}

	if !s.execConfig.AllowNetworking {
		containerArgs = append(containerArgs, "--network", "none")
	}

	for _, cap := range s.execConfig.DropCapabilities {
		containerArgs = append(containerArgs, "--cap-drop", cap)
	}

	if s.execConfig.SecurityProfile != "" {
		containerArgs = append(containerArgs, "--security-opt", s.execConfig.SecurityProfile)
	}

	containerArgs = append(containerArgs, s.execConfig.ContainerImage, command)
	containerArgs = append(containerArgs, args...)

	cmd := exec.CommandContext(ctx, "docker", containerArgs...)
	return s.runCommandWithMonitoring(ctx, cmd)
}

func (s *SandboxEnvironment) executeWithNamespaces(ctx context.Context, command string, args []string, workDir string) (*ExecutionResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if !s.execConfig.AllowNetworking {
		cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
	}

	return s.runCommandWithMonitoring(ctx, cmd)
}

func (s *SandboxEnvironment) executeWithLimits(ctx context.Context, command string, args []string, workDir string) (*ExecutionResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	return s.runCommandWithMonitoring(ctx, cmd)
}

func (s *SandboxEnvironment) runCommandWithMonitoring(ctx context.Context, cmd *exec.Cmd) (*ExecutionResult, error) {
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	pid := int32(cmd.Process.Pid)
	monitor := s.startProcessMonitoring(pid)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		cmd.Process.Kill()
		err = ctx.Err()
	case violation := <-monitor.violationsCh:
		cmd.Process.Kill()
		err = fmt.Errorf("security violation: %s", violation)
	}

	s.stopProcessMonitoring(pid)

	result := &ExecutionResult{
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		ExitCode:  cmd.ProcessState.ExitCode(),
		Sandboxed: true,
		ProcessID: int(pid),
	}

	monitor.mu.RLock()
	result.Violations = make([]string, len(monitor.violations))
	copy(result.Violations, monitor.violations)
	monitor.mu.RUnlock()

	if monitor.process != nil {
		if memInfo, err := monitor.process.MemoryInfo(); err == nil {
			result.MemoryUsed = memInfo.RSS
		}
		if cpuPercent, err := monitor.process.CPUPercent(); err == nil {
			result.CPUUsed = cpuPercent
		}
	}

	return result, err
}

func (s *SandboxEnvironment) startProcessMonitoring(pid int32) *ProcessMonitor {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return &ProcessMonitor{
			pid:          pid,
			maxMemory:    uint64(s.execConfig.MaxMemoryMB) * 1024 * 1024,
			maxCPU:       s.execConfig.MaxCPUPercent,
			stopChan:     make(chan bool),
			violationsCh: make(chan string, 10),
		}
	}

	monitor := &ProcessMonitor{
		pid:          pid,
		process:      proc,
		maxMemory:    uint64(s.execConfig.MaxMemoryMB) * 1024 * 1024,
		maxCPU:       s.execConfig.MaxCPUPercent,
		stopChan:     make(chan bool),
		violationsCh: make(chan string, 10),
	}

	s.mu.Lock()
	s.activeProcesses[pid] = monitor
	s.mu.Unlock()

	go s.monitorProcess(monitor)
	return monitor
}

func (s *SandboxEnvironment) monitorProcess(monitor *ProcessMonitor) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-monitor.stopChan:
			return
		case <-ticker.C:
			if monitor.process == nil {
				continue
			}

			if memInfo, err := monitor.process.MemoryInfo(); err == nil {
				if memInfo.RSS > monitor.maxMemory {
					violation := fmt.Sprintf("Memory limit exceeded: %d MB > %d MB", 
						memInfo.RSS/(1024*1024), monitor.maxMemory/(1024*1024))
					monitor.mu.Lock()
					monitor.violations = append(monitor.violations, violation)
					monitor.mu.Unlock()
					
					select {
					case monitor.violationsCh <- violation:
					default:
					}
				}
			}

			if cpuPercent, err := monitor.process.CPUPercent(); err == nil {
				if cpuPercent > monitor.maxCPU {
					violation := fmt.Sprintf("CPU limit exceeded: %.2f%% > %.2f%%", cpuPercent, monitor.maxCPU)
					monitor.mu.Lock()
					monitor.violations = append(monitor.violations, violation)
					monitor.mu.Unlock()
					
					select {
					case monitor.violationsCh <- violation:
					default:
					}
				}
			}
		}
	}
}

func (s *SandboxEnvironment) stopProcessMonitoring(pid int32) {
	s.mu.Lock()
	monitor, exists := s.activeProcesses[pid]
	if exists {
		delete(s.activeProcesses, pid)
	}
	s.mu.Unlock()

	if exists {
		close(monitor.stopChan)
	}
}

func (s *SandboxEnvironment) validateCommand(command string, args []string) error {
	if strings.Contains(command, "..") {
		return fmt.Errorf("command contains path traversal")
	}

	dangerousCommands := []string{
		"rm", "rmdir", "del", "format", "fdisk", "mkfs",
		"dd", "mount", "umount", "sudo", "su", "chmod",
		"chown", "passwd", "useradd", "userdel", "crontab",
		"systemctl", "service", "reboot", "shutdown", "halt",
		"kill", "killall", "pkill", "wget", "curl", "nc",
		"netcat", "telnet", "ssh", "scp", "rsync",
	}

	cmdName := filepath.Base(command)
	for _, dangerous := range dangerousCommands {
		if cmdName == dangerous {
			return fmt.Errorf("command '%s' is not allowed", dangerous)
		}
	}

	for _, arg := range args {
		if strings.Contains(arg, "..") {
			return fmt.Errorf("argument contains path traversal: %s", arg)
		}
		
		if strings.HasPrefix(arg, "/etc") || strings.HasPrefix(arg, "/proc") || 
		   strings.HasPrefix(arg, "/sys") || strings.HasPrefix(arg, "/dev") {
			return fmt.Errorf("access to system directory not allowed: %s", arg)
		}
	}

	return nil
}

// GetMetrics returns execution metrics
func (s *SandboxEnvironment) GetMetrics() ExecutionMetrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()
	
	return ExecutionMetrics{
		TotalExecutions:    s.metrics.TotalExecutions,
		SuccessfulRuns:     s.metrics.SuccessfulRuns,
		FailedRuns:         s.metrics.FailedRuns,
		SecurityViolations: s.metrics.SecurityViolations,
		AverageExecTime:    s.metrics.AverageExecTime,
		PeakMemoryUsage:    s.metrics.PeakMemoryUsage,
	}
}

// GetActiveProcessCount returns the number of active processes
func (s *SandboxEnvironment) GetActiveProcessCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.activeProcesses)
}

// KillAllProcesses terminates all active processes
func (s *SandboxEnvironment) KillAllProcesses() error {
	s.mu.RLock()
	processes := make(map[int32]*ProcessMonitor)
	for pid, monitor := range s.activeProcesses {
		processes[pid] = monitor
	}
	s.mu.RUnlock()

	for pid, monitor := range processes {
		if monitor.process != nil {
			if err := monitor.process.Kill(); err != nil {
				if s.logger != nil {
					s.logger.Log(fmt.Sprintf("Failed to kill process %d: %v", pid, err))
				}
			}
		}
		s.stopProcessMonitoring(pid)
	}

	return nil
}

// GetDiskUsage returns the current disk usage of the sandbox
func (s *SandboxEnvironment) GetDiskUsage() (int64, error) {
	var totalSize int64
	
	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() {
			totalSize += info.Size()
		}
		
		return nil
	})
	
	if err != nil {
		return 0, fmt.Errorf("failed to calculate disk usage: %w", err)
	}
	
	return totalSize, nil
}

// Cleanup removes the sandbox and all its contents
func (s *SandboxEnvironment) Cleanup() error {
	// Kill all active processes first
	s.KillAllProcesses()
	
	// Close logger
	if s.logger != nil {
		s.logger.Close()
	}
	
	// Run cleanup function
	if s.cleanupFn != nil {
		return s.cleanupFn()
	}
	return nil
}

// SandboxManager manages multiple sandbox instances
type SandboxManager struct {
	sandboxes map[string]*SandboxEnvironment
	policy    *SecurityPolicy
}

// NewSandboxManager creates a new sandbox manager
func NewSandboxManager(policy *SecurityPolicy) *SandboxManager {
	return &SandboxManager{
		sandboxes: make(map[string]*SandboxEnvironment),
		policy:    policy,
	}
}

// CreateSandbox creates a new sandbox with the given ID
func (sm *SandboxManager) CreateSandbox(id string, config SandboxConfig) (*SandboxEnvironment, error) {
	if _, exists := sm.sandboxes[id]; exists {
		return nil, fmt.Errorf("sandbox with id %s already exists", id)
	}
	
	sandbox, err := NewSandbox(sm.policy, config)
	if err != nil {
		return nil, err
	}
	
	sm.sandboxes[id] = sandbox
	return sandbox, nil
}

// GetSandbox retrieves a sandbox by ID
func (sm *SandboxManager) GetSandbox(id string) (*SandboxEnvironment, bool) {
	sandbox, exists := sm.sandboxes[id]
	return sandbox, exists
}

// DestroySandbox removes and cleans up a sandbox
func (sm *SandboxManager) DestroySandbox(id string) error {
	sandbox, exists := sm.sandboxes[id]
	if !exists {
		return fmt.Errorf("sandbox with id %s does not exist", id)
	}
	
	if err := sandbox.Cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup sandbox: %w", err)
	}
	
	delete(sm.sandboxes, id)
	return nil
}

// ListSandboxes returns all active sandbox IDs
func (sm *SandboxManager) ListSandboxes() []string {
	var ids []string
	for id := range sm.sandboxes {
		ids = append(ids, id)
	}
	return ids
}

// CleanupAll destroys all sandboxes
func (sm *SandboxManager) CleanupAll() error {
	var errors []string
	
	for id, sandbox := range sm.sandboxes {
		if err := sandbox.Cleanup(); err != nil {
			errors = append(errors, fmt.Sprintf("sandbox %s: %v", id, err))
		}
	}
	
	sm.sandboxes = make(map[string]*SandboxEnvironment)
	
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// NewDefaultExecutionConfig returns a secure default execution configuration
func NewDefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		MaxMemoryMB:      512,
		MaxCPUPercent:    50.0,
		MaxExecutionTime: 30 * time.Second,
		AllowNetworking:  false,
		AllowFileWrite:   true,
		EnableLogging:    true,
		UseNamespaces:    true,
		ReadOnlyRootFS:   false,
		DropCapabilities: []string{"NET_RAW", "SYS_ADMIN", "SYS_PTRACE"},
		SecurityProfile:  "seccomp:default",
	}
}

// NewContainerExecutionConfig returns configuration for container-based execution
func NewContainerExecutionConfig(image string) ExecutionConfig {
	config := NewDefaultExecutionConfig()
	config.ContainerImage = image
	config.UseNamespaces = false  // Containers provide their own namespace isolation
	config.ReadOnlyRootFS = true
	return config
}

// NewHighSecurityConfig returns a high-security execution configuration
func NewHighSecurityConfig() ExecutionConfig {
	return ExecutionConfig{
		MaxMemoryMB:      256,
		MaxCPUPercent:    25.0,
		MaxExecutionTime: 10 * time.Second,
		AllowNetworking:  false,
		AllowFileWrite:   false,
		EnableLogging:    true,
		UseNamespaces:    true,
		ReadOnlyRootFS:   true,
		DropCapabilities: []string{
			"AUDIT_CONTROL", "AUDIT_READ", "AUDIT_WRITE",
			"BLOCK_SUSPEND", "CHOWN", "DAC_OVERRIDE",
			"DAC_READ_SEARCH", "FOWNER", "FSETID", "IPC_LOCK",
			"IPC_OWNER", "KILL", "LEASE", "LINUX_IMMUTABLE",
			"MAC_ADMIN", "MAC_OVERRIDE", "MKNOD", "NET_ADMIN",
			"NET_BIND_SERVICE", "NET_BROADCAST", "NET_RAW",
			"SETGID", "SETFCAP", "SETPCAP", "SETUID",
			"SYS_ADMIN", "SYS_BOOT", "SYS_CHROOT", "SYS_MODULE",
			"SYS_NICE", "SYS_PACCT", "SYS_PTRACE", "SYS_RAWIO",
			"SYS_RESOURCE", "SYS_TIME", "SYS_TTY_CONFIG", "SYSLOG",
			"WAKE_ALARM",
		},
		SecurityProfile: "seccomp:unconfined",
	}
}

func init() {
	if unsafe.Sizeof(uintptr(0)) == 4 {
		fmt.Fprintf(os.Stderr, "Warning: Running on 32-bit system, some security features may be limited\n")
	}
}