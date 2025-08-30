package security

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// SecurityLevel defines the security enforcement level
type SecurityLevel int

const (
	// Permissive allows most operations with basic safety checks
	Permissive SecurityLevel = iota
	// Standard enforces common security policies
	Standard
	// Strict applies maximum security restrictions
	Strict
	// Sandbox runs in complete isolation
	Sandbox
)

// SecurityPolicy defines security rules and constraints for tool execution
type SecurityPolicy struct {
	// Security level enforcement
	Level SecurityLevel
	
	// Command execution policies
	AllowedCommands     []string          // Whitelist of allowed commands
	BlockedCommands     []string          // Blacklist of forbidden commands
	BlockedPatterns     []*regexp.Regexp  // Regex patterns for command blocking
	RequireConfirmation []string          // Commands requiring user confirmation
	
	// Path and file access policies
	AllowedPaths        []string          // Whitelist of allowed paths
	BlockedPaths        []string          // Blacklist of forbidden paths
	BlockedExtensions   []string          // Forbidden file extensions
	AllowedExtensions   []string          // Allowed file extensions (if set, only these allowed)
	MaxFileSize         int64             // Maximum file size for operations
	
	// Execution limits
	MaxExecutionTime    time.Duration     // Maximum execution time for commands
	MaxConcurrentOps    int               // Maximum concurrent operations
	MaxMemoryUsage      int64             // Maximum memory usage (bytes)
	MaxOutputSize       int64             // Maximum output size (bytes)
	
	// Network and system policies
	AllowNetworkAccess  bool              // Allow network operations
	AllowSystemModify   bool              // Allow system modification operations
	AllowProcessSpawn   bool              // Allow spawning new processes
	
	// Sandbox configuration
	SandboxEnabled      bool              // Enable sandbox mode
	SandboxRoot         string            // Root directory for sandbox
	TempDirectory       string            // Temporary directory for operations
}

// ValidationError represents a security validation error
type ValidationError struct {
	Type    string
	Message string
	Details map[string]interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("Security validation failed [%s]: %s", e.Type, e.Message)
}

// NewSecurityPolicy creates a new security policy with the specified level
func NewSecurityPolicy(level SecurityLevel) *SecurityPolicy {
	policy := &SecurityPolicy{
		Level:            level,
		MaxExecutionTime: 30 * time.Second,
		MaxConcurrentOps: 3,
		MaxMemoryUsage:   100 * 1024 * 1024, // 100MB
		MaxOutputSize:    10 * 1024 * 1024,  // 10MB
		MaxFileSize:      50 * 1024 * 1024,  // 50MB
		TempDirectory:    os.TempDir(),
	}
	
	switch level {
	case Permissive:
		policy.configurePermissive()
	case Standard:
		policy.configureStandard()
	case Strict:
		policy.configureStrict()
	case Sandbox:
		policy.configureSandbox()
	}
	
	return policy
}

// configurePermissive sets up permissive security settings
func (p *SecurityPolicy) configurePermissive() {
	p.AllowNetworkAccess = true
	p.AllowSystemModify = true
	p.AllowProcessSpawn = true
	
	// Basic dangerous commands still blocked
	p.BlockedCommands = []string{
		"rm -rf /", "dd if=/dev/zero", ":(){ :|:& };:", // Fork bombs and destructive
	}
	
	// Basic dangerous patterns
	p.BlockedPatterns = []*regexp.Regexp{
		regexp.MustCompile(`rm\s+.*-rf\s+/\s*$`), // rm -rf / variations
	}
}

// configureStandard sets up standard security settings (default)
func (p *SecurityPolicy) configureStandard() {
	p.AllowNetworkAccess = false
	p.AllowSystemModify = false
	p.AllowProcessSpawn = true
	
	// Standard command whitelist
	p.AllowedCommands = []string{
		"ls", "cat", "echo", "pwd", "whoami", "id", "date", "uname",
		"grep", "find", "head", "tail", "wc", "sort", "uniq", "awk", "sed",
		"ps", "top", "df", "du", "free", "uptime", "sleep",
		"git", "node", "npm", "go", "python", "java", "cargo", "make",
	}
	
	// Dangerous commands blacklist
	p.BlockedCommands = []string{
		"rm", "rmdir", "dd", "mkfs", "fdisk", "mount", "umount",
		"sudo", "su", "passwd", "chown", "chmod", "chroot",
		"iptables", "systemctl", "service", "init",
		"reboot", "shutdown", "halt", "poweroff",
		"kill", "killall", "pkill",
	}
	
	// Dangerous patterns
	p.BlockedPatterns = []*regexp.Regexp{
		regexp.MustCompile(`rm\s+.*-r.*f`),      // rm -rf patterns
		regexp.MustCompile(`>\s*/dev/`),         // Redirect to device files
		regexp.MustCompile(`curl.*\|\s*sh`),     // Pipe to shell
		regexp.MustCompile(`wget.*\|\s*sh`),     // Pipe to shell
		regexp.MustCompile(`;\s*rm\s`),          // Command chaining with rm
		regexp.MustCompile(`&&\s*rm\s`),         // Command chaining with rm
		regexp.MustCompile(`\$\([^)]+\)`),       // Command substitution
		regexp.MustCompile("`[^`]+`"),           // Backtick command substitution
	}
	
	// Commands requiring confirmation
	p.RequireConfirmation = []string{
		"git push", "git reset --hard", "git clean -fd",
		"npm publish", "cargo publish",
	}
	
	// Blocked paths
	p.BlockedPaths = []string{
		"/etc", "/sys", "/proc", "/dev", "/boot",
		"/usr/bin", "/usr/sbin", "/bin", "/sbin",
		"/var/log", "/var/run", "/var/lib",
	}
	
	// Blocked extensions
	p.BlockedExtensions = []string{
		".exe", ".dll", ".so", ".dylib", ".app",
		".deb", ".rpm", ".msi", ".dmg",
	}
}

// configureStrict sets up strict security settings
func (p *SecurityPolicy) configureStrict() {
	p.configureStandard()
	
	// More restrictive limits
	p.MaxExecutionTime = 10 * time.Second
	p.MaxConcurrentOps = 1
	p.MaxMemoryUsage = 50 * 1024 * 1024  // 50MB
	p.MaxOutputSize = 5 * 1024 * 1024    // 5MB
	p.MaxFileSize = 10 * 1024 * 1024     // 10MB
	
	// Very limited command set
	p.AllowedCommands = []string{
		"ls", "cat", "echo", "pwd", "grep", "head", "tail", "wc",
		"find", "sort", "uniq", "date",
	}
	
	// All modification commands require confirmation
	p.RequireConfirmation = append(p.RequireConfirmation,
		"git add", "git commit", "git merge", "git checkout",
		"npm install", "go mod tidy", "make", "cargo build",
	)
	
	// Only allow specific extensions
	p.AllowedExtensions = []string{
		".txt", ".md", ".json", ".yaml", ".yml", ".toml",
		".go", ".js", ".ts", ".py", ".java", ".rs", ".c", ".h",
		".html", ".css", ".xml", ".csv",
	}
	
	// More blocked paths
	p.BlockedPaths = append(p.BlockedPaths,
		"~/.ssh", "~/.aws", "~/.kube",
		"/var/tmp",
	)
}

// configureSandbox sets up sandbox mode
func (p *SecurityPolicy) configureSandbox() {
	p.configureStrict()
	
	p.SandboxEnabled = true
	p.AllowNetworkAccess = false
	p.AllowSystemModify = false
	p.AllowProcessSpawn = false
	
	// Minimal command set
	p.AllowedCommands = []string{
		"echo", "cat", "ls", "pwd", "grep", "head", "tail", "wc",
	}
	
	// Very restrictive limits
	p.MaxExecutionTime = 5 * time.Second
	p.MaxConcurrentOps = 1
	p.MaxMemoryUsage = 20 * 1024 * 1024  // 20MB
	p.MaxOutputSize = 1024 * 1024        // 1MB
	p.MaxFileSize = 1024 * 1024          // 1MB
}

// ValidateCommand checks if a command is allowed by the security policy
func (p *SecurityPolicy) ValidateCommand(command string) error {
	// Extract base command
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) == 0 {
		return &ValidationError{
			Type:    "empty_command",
			Message: "command cannot be empty",
		}
	}
	
	baseCommand := parts[0]
	
	// Check blocked commands first
	for _, blocked := range p.BlockedCommands {
		if baseCommand == blocked {
			return &ValidationError{
				Type:    "blocked_command",
				Message: fmt.Sprintf("command '%s' is blocked by security policy", baseCommand),
				Details: map[string]interface{}{
					"command": baseCommand,
					"level":   p.Level,
				},
			}
		}
	}
	
	// Check blocked patterns
	for _, pattern := range p.BlockedPatterns {
		if pattern.MatchString(command) {
			return &ValidationError{
				Type:    "blocked_pattern",
				Message: fmt.Sprintf("command matches blocked pattern: %s", pattern.String()),
				Details: map[string]interface{}{
					"command": command,
					"pattern": pattern.String(),
				},
			}
		}
	}
	
	// Check allowed commands (if whitelist is defined)
	if len(p.AllowedCommands) > 0 {
		allowed := false
		for _, allowedCmd := range p.AllowedCommands {
			if baseCommand == allowedCmd {
				allowed = true
				break
			}
		}
		
		if !allowed {
			return &ValidationError{
				Type:    "command_not_allowed",
				Message: fmt.Sprintf("command '%s' is not in allowed list", baseCommand),
				Details: map[string]interface{}{
					"command":        baseCommand,
					"allowed_commands": p.AllowedCommands,
				},
			}
		}
	}
	
	return nil
}

// ValidatePath checks if a file path is allowed by the security policy
func (p *SecurityPolicy) ValidatePath(path string) error {
	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return &ValidationError{
			Type:    "invalid_path",
			Message: fmt.Sprintf("cannot resolve path: %v", err),
		}
	}
	
	// Check for path traversal
	if strings.Contains(path, "..") {
		return &ValidationError{
			Type:    "path_traversal",
			Message: "path traversal detected in path",
			Details: map[string]interface{}{
				"path":         path,
				"resolved_path": absPath,
			},
		}
	}
	
	// Check blocked paths
	for _, blocked := range p.BlockedPaths {
		blockedAbs, _ := filepath.Abs(blocked)
		if strings.HasPrefix(absPath, blockedAbs) {
			return &ValidationError{
				Type:    "blocked_path",
				Message: fmt.Sprintf("path '%s' is blocked by security policy", absPath),
				Details: map[string]interface{}{
					"path":        absPath,
					"blocked_path": blockedAbs,
				},
			}
		}
	}
	
	// Check allowed paths (if whitelist is defined)
	if len(p.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range p.AllowedPaths {
			allowedAbs, _ := filepath.Abs(allowedPath)
			if strings.HasPrefix(absPath, allowedAbs) {
				allowed = true
				break
			}
		}
		
		if !allowed {
			return &ValidationError{
				Type:    "path_not_allowed",
				Message: fmt.Sprintf("path '%s' is not in allowed paths", absPath),
				Details: map[string]interface{}{
					"path":          absPath,
					"allowed_paths": p.AllowedPaths,
				},
			}
		}
	}
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	
	// Check blocked extensions
	for _, blockedExt := range p.BlockedExtensions {
		if ext == strings.ToLower(blockedExt) {
			return &ValidationError{
				Type:    "blocked_extension",
				Message: fmt.Sprintf("file extension '%s' is blocked", ext),
				Details: map[string]interface{}{
					"path":      absPath,
					"extension": ext,
				},
			}
		}
	}
	
	// Check allowed extensions (if whitelist is defined)
	if len(p.AllowedExtensions) > 0 && ext != "" {
		allowed := false
		for _, allowedExt := range p.AllowedExtensions {
			if ext == strings.ToLower(allowedExt) {
				allowed = true
				break
			}
		}
		
		if !allowed {
			return &ValidationError{
				Type:    "extension_not_allowed",
				Message: fmt.Sprintf("file extension '%s' is not allowed", ext),
				Details: map[string]interface{}{
					"path":               absPath,
					"extension":          ext,
					"allowed_extensions": p.AllowedExtensions,
				},
			}
		}
	}
	
	return nil
}

// ValidateFileSize checks if a file size is within policy limits
func (p *SecurityPolicy) ValidateFileSize(size int64) error {
	if p.MaxFileSize > 0 && size > p.MaxFileSize {
		return &ValidationError{
			Type:    "file_too_large",
			Message: fmt.Sprintf("file size %d bytes exceeds limit of %d bytes", size, p.MaxFileSize),
			Details: map[string]interface{}{
				"size":      size,
				"max_size":  p.MaxFileSize,
			},
		}
	}
	return nil
}

// RequiresConfirmation checks if a command requires user confirmation
func (p *SecurityPolicy) RequiresConfirmation(command string) bool {
	for _, confirmCmd := range p.RequireConfirmation {
		if strings.Contains(strings.ToLower(command), strings.ToLower(confirmCmd)) {
			return true
		}
	}
	return false
}

// GetSandboxRoot returns the sandbox root directory
func (p *SecurityPolicy) GetSandboxRoot() string {
	if p.SandboxEnabled && p.SandboxRoot != "" {
		return p.SandboxRoot
	}
	return p.TempDirectory
}

// Clone creates a copy of the security policy
func (p *SecurityPolicy) Clone() *SecurityPolicy {
	clone := &SecurityPolicy{
		Level:               p.Level,
		AllowedCommands:     make([]string, len(p.AllowedCommands)),
		BlockedCommands:     make([]string, len(p.BlockedCommands)),
		BlockedPatterns:     make([]*regexp.Regexp, len(p.BlockedPatterns)),
		RequireConfirmation: make([]string, len(p.RequireConfirmation)),
		AllowedPaths:        make([]string, len(p.AllowedPaths)),
		BlockedPaths:        make([]string, len(p.BlockedPaths)),
		BlockedExtensions:   make([]string, len(p.BlockedExtensions)),
		AllowedExtensions:   make([]string, len(p.AllowedExtensions)),
		MaxFileSize:         p.MaxFileSize,
		MaxExecutionTime:    p.MaxExecutionTime,
		MaxConcurrentOps:    p.MaxConcurrentOps,
		MaxMemoryUsage:      p.MaxMemoryUsage,
		MaxOutputSize:       p.MaxOutputSize,
		AllowNetworkAccess:  p.AllowNetworkAccess,
		AllowSystemModify:   p.AllowSystemModify,
		AllowProcessSpawn:   p.AllowProcessSpawn,
		SandboxEnabled:      p.SandboxEnabled,
		SandboxRoot:         p.SandboxRoot,
		TempDirectory:       p.TempDirectory,
	}
	
	copy(clone.AllowedCommands, p.AllowedCommands)
	copy(clone.BlockedCommands, p.BlockedCommands)
	copy(clone.BlockedPatterns, p.BlockedPatterns)
	copy(clone.RequireConfirmation, p.RequireConfirmation)
	copy(clone.AllowedPaths, p.AllowedPaths)
	copy(clone.BlockedPaths, p.BlockedPaths)
	copy(clone.BlockedExtensions, p.BlockedExtensions)
	copy(clone.AllowedExtensions, p.AllowedExtensions)
	
	return clone
}