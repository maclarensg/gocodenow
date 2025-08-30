package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// BashExecutor executes bash commands with security controls and real-time output streaming
type BashExecutor struct {
	// Security configuration
	allowedCommands   []string
	blockedCommands   []string
	blockedPatterns   []*regexp.Regexp
	allowDangerous    bool
	maxExecutionTime  time.Duration
	
	// Environment configuration
	workingDir        string
	env               []string
	inheritEnv        bool
	
	// Output configuration
	streamOutput      bool
	captureOutput     bool
	maxOutputSize     int64
	
	// Internal state
	mu                sync.RWMutex
	runningCommands   map[string]*exec.Cmd
}

// NewBashExecutor creates a new bash command executor with default security settings
func NewBashExecutor(options ...BashExecutorOption) *BashExecutor {
	executor := &BashExecutor{
		// Default security settings (very restrictive)
		allowedCommands:  []string{
			"ls", "cat", "echo", "pwd", "whoami", "id", "date", "uname",
			"grep", "find", "head", "tail", "wc", "sort", "uniq",
			"ps", "top", "df", "du", "free", "uptime", "sleep",
		},
		blockedCommands: []string{
			"rm", "rmdir", "dd", "mkfs", "fdisk", "mount", "umount",
			"sudo", "su", "passwd", "chown", "chmod", "chroot",
			"iptables", "systemctl", "service", "init",
			"reboot", "shutdown", "halt", "poweroff",
			"kill", "killall", "pkill",
		},
		blockedPatterns: []*regexp.Regexp{
			regexp.MustCompile(`rm\s+.*-r.*f`),      // rm -rf patterns
			regexp.MustCompile(`>\s*/dev/`),         // Redirect to device files
			regexp.MustCompile(`curl.*\|\s*sh`),     // Pipe to shell
			regexp.MustCompile(`wget.*\|\s*sh`),     // Pipe to shell
			regexp.MustCompile(`;\s*rm\s`),          // Command chaining with rm
			regexp.MustCompile(`&&\s*rm\s`),         // Command chaining with rm
			regexp.MustCompile(`\$\([^)]+\)`),       // Command substitution
			regexp.MustCompile("`[^`]+`"),           // Backtick command substitution
		},
		
		allowDangerous:   false,
		maxExecutionTime: 30 * time.Second,
		
		// Default environment
		workingDir:       "",  // Will use current directory
		env:             []string{},
		inheritEnv:      true,
		
		// Default output settings
		streamOutput:    true,
		captureOutput:   true,
		maxOutputSize:   1024 * 1024, // 1MB max output
		
		runningCommands: make(map[string]*exec.Cmd),
	}
	
	// Apply options
	for _, option := range options {
		option(executor)
	}
	
	return executor
}

// BashExecutorOption configures the bash executor
type BashExecutorOption func(*BashExecutor)

// WithAllowedCommands sets the list of allowed commands
func WithAllowedCommands(commands []string) BashExecutorOption {
	return func(e *BashExecutor) {
		e.allowedCommands = commands
	}
}

// WithBlockedCommands sets the list of blocked commands
func WithBlockedCommands(commands []string) BashExecutorOption {
	return func(e *BashExecutor) {
		e.blockedCommands = commands
	}
}

// WithDangerousCommands allows dangerous commands (USE WITH EXTREME CAUTION)
func WithDangerousCommands(allow bool) BashExecutorOption {
	return func(e *BashExecutor) {
		e.allowDangerous = allow
	}
}

// WithMaxExecutionTime sets the maximum execution time for commands
func WithMaxExecutionTime(timeout time.Duration) BashExecutorOption {
	return func(e *BashExecutor) {
		e.maxExecutionTime = timeout
	}
}

// WithWorkingDirectory sets the working directory for commands
func WithWorkingDirectory(dir string) BashExecutorOption {
	return func(e *BashExecutor) {
		e.workingDir = dir
	}
}

// WithEnvironment sets custom environment variables
func WithEnvironment(env []string, inherit bool) BashExecutorOption {
	return func(e *BashExecutor) {
		e.env = env
		e.inheritEnv = inherit
	}
}

// WithOutputSettings configures output handling
func WithOutputSettings(stream, capture bool, maxSize int64) BashExecutorOption {
	return func(e *BashExecutor) {
		e.streamOutput = stream
		e.captureOutput = capture
		e.maxOutputSize = maxSize
	}
}

// Name returns the name of this tool executor
func (e *BashExecutor) Name() string {
	return "bash_command"
}

// Description returns a description of what this executor does
func (e *BashExecutor) Description() string {
	return "Execute bash commands with security controls and real-time output streaming"
}

// ValidateParameters validates the parameters for bash command execution
func (e *BashExecutor) ValidateParameters(params ToolParameters) error {
	// Check required parameters
	command, exists := params["command"]
	if !exists {
		return fmt.Errorf("required parameter 'command' is missing")
	}
	
	commandStr, ok := command.(string)
	if !ok {
		return fmt.Errorf("parameter 'command' must be a string")
	}
	
	if strings.TrimSpace(commandStr) == "" {
		return fmt.Errorf("parameter 'command' cannot be empty")
	}
	
	// Validate optional parameters
	if workDir, exists := params["working_directory"]; exists {
		if workDirStr, ok := workDir.(string); ok {
			if !filepath.IsAbs(workDirStr) {
				return fmt.Errorf("working_directory must be an absolute path")
			}
		} else {
			return fmt.Errorf("parameter 'working_directory' must be a string")
		}
	}
	
	if timeout, exists := params["timeout"]; exists {
		switch v := timeout.(type) {
		case float64:
			if v <= 0 || v > 300 { // Max 5 minutes
				return fmt.Errorf("timeout must be between 1 and 300 seconds")
			}
		case int:
			if v <= 0 || v > 300 {
				return fmt.Errorf("timeout must be between 1 and 300 seconds")
			}
		default:
			return fmt.Errorf("parameter 'timeout' must be a number")
		}
	}
	
	return nil
}

// Schema returns the JSON schema for this tool's parameters
func (e *BashExecutor) Schema() ToolSchema {
	return ToolSchema{
		Name:        e.Name(),
		Description: e.Description(),
		Parameters: map[string]interface{}{
			"command": map[string]interface{}{
				"type":        "string",
				"description": "The bash command to execute",
			},
			"working_directory": map[string]interface{}{
				"type":        "string",
				"description": "Working directory for command execution (optional)",
			},
			"timeout": map[string]interface{}{
				"type":        "number",
				"description": "Maximum execution time in seconds (optional, default: 30)",
				"minimum":     1,
				"maximum":     300,
			},
			"env": map[string]interface{}{
				"type":        "object",
				"description": "Environment variables for the command (optional)",
			},
		},
		Required: []string{"command"},
		Examples: []ToolParameters{
			{
				"command": "ls -la",
			},
			{
				"command":           "find /tmp -name '*.log'",
				"working_directory": "/tmp",
				"timeout":          10,
			},
			{
				"command": "echo $MY_VAR",
				"env":     map[string]string{"MY_VAR": "hello world"},
			},
		},
	}
}

// Execute runs the bash command with all security checks and controls
func (e *BashExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	startTime := time.Now()
	
	// Validate parameters
	if err := e.ValidateParameters(params); err != nil {
		return &ToolResult{
			ToolCallID:   "",
			ToolName:     e.Name(),
			Success:      false,
			Result:       nil,
			ErrorMessage: fmt.Sprintf("Parameter validation failed: %v", err),
			Duration:     time.Since(startTime),
			Timestamp:    startTime,
		}, nil
	}
	
	// Extract parameters
	command := params["command"].(string)
	
	// Security validation
	if err := e.validateCommandSecurity(command); err != nil {
		return &ToolResult{
			ToolCallID:   "",
			ToolName:     e.Name(),
			Success:      false,
			Result:       nil,
			ErrorMessage: fmt.Sprintf("Security validation failed: %v", err),
			Duration:     time.Since(startTime),
			Timestamp:    startTime,
		}, nil
	}
	
	// Prepare execution context
	execCtx := ctx
	if timeout := e.getTimeout(params); timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	
	// Execute the command
	output, err := e.executeCommand(execCtx, command, params)
	
	// Prepare result
	result := &ToolResult{
		ToolCallID: "",
		ToolName:   e.Name(),
		Success:    err == nil,
		Duration:   time.Since(startTime),
		Timestamp:  startTime,
		Metadata: map[string]interface{}{
			"command":        command,
			"execution_time": time.Since(startTime).String(),
		},
	}
	
	if err != nil {
		result.ErrorMessage = err.Error()
		result.Result = map[string]interface{}{
			"error":  err.Error(),
			"output": output,
		}
	} else {
		result.Result = map[string]interface{}{
			"output":     output,
			"exit_code":  0,
		}
	}
	
	return result, nil
}

// validateCommandSecurity performs comprehensive security validation
func (e *BashExecutor) validateCommandSecurity(command string) error {
	if !e.allowDangerous {
		// Check blocked commands
		words := strings.Fields(command)
		if len(words) == 0 {
			return fmt.Errorf("empty command")
		}
		
		baseCommand := filepath.Base(words[0])
		
		// Check if command is explicitly blocked
		for _, blocked := range e.blockedCommands {
			if baseCommand == blocked {
				return fmt.Errorf("command '%s' is not allowed for security reasons", blocked)
			}
		}
		
		// Check blocked patterns
		for _, pattern := range e.blockedPatterns {
			if pattern.MatchString(command) {
				return fmt.Errorf("command matches blocked pattern for security reasons")
			}
		}
		
		// Check if command is in allowed list (if allowlist is defined and not empty)
		if len(e.allowedCommands) > 0 {
			allowed := false
			for _, allowedCmd := range e.allowedCommands {
				if baseCommand == allowedCmd {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("command '%s' is not in the allowed commands list", baseCommand)
			}
		}
		
		// Additional security checks
		if strings.Contains(command, "..") {
			return fmt.Errorf("path traversal detected in command")
		}
		
		if strings.Contains(command, "/dev/") {
			return fmt.Errorf("device file access not allowed")
		}
		
		if strings.Contains(command, "/proc/") {
			return fmt.Errorf("proc filesystem access not allowed")
		}
	}
	
	return nil
}

// executeCommand executes the command with proper isolation and output handling
func (e *BashExecutor) executeCommand(ctx context.Context, command string, params ToolParameters) (string, error) {
	// Create the command
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	
	// Set working directory
	workDir := e.getWorkingDirectory(params)
	if workDir != "" {
		cmd.Dir = workDir
	}
	
	// Set environment
	cmd.Env = e.buildEnvironment(params)
	
	// Create pipes for output capture
	var outputBuffer strings.Builder
	var errorBuffer strings.Builder
	
	if e.captureOutput {
		// Create pipes
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return "", fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return "", fmt.Errorf("failed to create stderr pipe: %w", err)
		}
		
		// Start the command
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("failed to start command: %w", err)
		}
		
		// Track running command
		commandID := fmt.Sprintf("%d", cmd.Process.Pid)
		e.mu.Lock()
		e.runningCommands[commandID] = cmd
		e.mu.Unlock()
		
		// Clean up when done
		defer func() {
			e.mu.Lock()
			delete(e.runningCommands, commandID)
			e.mu.Unlock()
		}()
		
		// Stream output
		var wg sync.WaitGroup
		wg.Add(2)
		
		// Handle stdout
		go func() {
			defer wg.Done()
			e.streamReader(stdoutPipe, &outputBuffer, "stdout")
		}()
		
		// Handle stderr
		go func() {
			defer wg.Done()
			e.streamReader(stderrPipe, &errorBuffer, "stderr")
		}()
		
		// Wait for command completion
		wg.Wait()
		err = cmd.Wait()
		
		// Prepare output
		output := outputBuffer.String()
		if errorBuffer.Len() > 0 {
			if output != "" {
				output += "\n--- STDERR ---\n"
			}
			output += errorBuffer.String()
		}
		
		if err != nil {
			// Check for timeout
			if ctx.Err() == context.DeadlineExceeded {
				return output, fmt.Errorf("command timed out")
			}
			
			// Check exit code
			if exitError, ok := err.(*exec.ExitError); ok {
				if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
					return output, fmt.Errorf("command exited with code %d", status.ExitStatus())
				}
			}
			
			return output, fmt.Errorf("command execution failed: %w", err)
		}
		
		return output, nil
	}
	
	// Simple execution without streaming
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return string(output), fmt.Errorf("command timed out")
		}
		return string(output), fmt.Errorf("command execution failed: %w", err)
	}
	
	return string(output), nil
}

// streamReader handles real-time output streaming
func (e *BashExecutor) streamReader(reader io.Reader, buffer *strings.Builder, streamType string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		
		// Write to buffer if capturing
		if e.captureOutput {
			buffer.WriteString(line)
			buffer.WriteString("\n")
		}
		
		// Stream to output if enabled (could be extended to send to UI)
		if e.streamOutput {
			// For now, we'll just capture. In future, this could stream to UI
			// fmt.Fprintf(os.Stderr, "[%s] %s\n", streamType, line)
		}
		
		// Check output size limit
		if buffer.Len() > int(e.maxOutputSize) {
			buffer.WriteString(fmt.Sprintf("\n[OUTPUT TRUNCATED - EXCEEDED %d BYTES]\n", e.maxOutputSize))
			break
		}
	}
}

// getTimeout extracts timeout from parameters or returns default
func (e *BashExecutor) getTimeout(params ToolParameters) time.Duration {
	if timeout, exists := params["timeout"]; exists {
		switch v := timeout.(type) {
		case float64:
			return time.Duration(v * float64(time.Second))
		case int:
			return time.Duration(v) * time.Second
		}
	}
	return e.maxExecutionTime
}

// getWorkingDirectory extracts working directory from parameters
func (e *BashExecutor) getWorkingDirectory(params ToolParameters) string {
	if workDir, exists := params["working_directory"]; exists {
		if workDirStr, ok := workDir.(string); ok {
			return workDirStr
		}
	}
	return e.workingDir
}

// buildEnvironment builds the environment variables for command execution
func (e *BashExecutor) buildEnvironment(params ToolParameters) []string {
	env := []string{}
	
	// Inherit system environment if configured
	if e.inheritEnv {
		env = append(env, os.Environ()...)
	}
	
	// Add executor-level environment variables
	env = append(env, e.env...)
	
	// Add parameter-level environment variables
	if envParam, exists := params["env"]; exists {
		if envMap, ok := envParam.(map[string]interface{}); ok {
			for key, value := range envMap {
				env = append(env, fmt.Sprintf("%s=%v", key, value))
			}
		}
	}
	
	return env
}

// InterruptAll interrupts all running commands (useful for cleanup)
func (e *BashExecutor) InterruptAll() error {
	e.mu.RLock()
	commands := make([]*exec.Cmd, 0, len(e.runningCommands))
	for _, cmd := range e.runningCommands {
		commands = append(commands, cmd)
	}
	e.mu.RUnlock()
	
	var errors []string
	for _, cmd := range commands {
		if cmd.Process != nil {
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to interrupt some commands: %s", strings.Join(errors, "; "))
	}
	
	return nil
}