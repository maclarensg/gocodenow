// Package security provides comprehensive security framework for lmcodenow
//
// This package implements security policies, sandboxing, user confirmation systems,
// and resource monitoring to ensure safe execution of tools and commands.
//
// Key Components:
//
// - SecurityPolicy: Configurable security rules and constraints
// - Sandbox: Isolated execution environments
// - ConfirmationService: User confirmation for risky operations
// - ResourceMonitor: Resource usage monitoring and limits enforcement
//
// Usage:
//
//	// Create a security policy
//	policy := security.NewSecurityPolicy(security.Standard)
//	
//	// Create confirmation service
//	confirmProvider := security.NewConsoleConfirmationProvider()
//	confirmService := security.NewConfirmationService(confirmProvider, policy)
//	
//	// Create resource monitor
//	monitor := security.NewResourceMonitor(security.DefaultResourceLimits())
//	
//	// Create security manager
//	manager := security.NewSecurityManager(policy, confirmService, monitor)
//	
//	// Execute a command securely
//	err := manager.ExecuteCommand("ls -la", nil)
//
package security

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SecurityManager coordinates all security components
type SecurityManager struct {
	policy        *SecurityPolicy
	confirmation  *ConfirmationService
	monitor       *ResourceMonitor
	sandboxMgr    *SandboxManager
}

// SecurityManagerConfig configures the security manager
type SecurityManagerConfig struct {
	Policy           *SecurityPolicy
	ConfirmProvider  ConfirmationProvider
	ResourceLimits   ResourceLimits
	EnableSandbox    bool
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config SecurityManagerConfig) *SecurityManager {
	if config.Policy == nil {
		config.Policy = NewSecurityPolicy(Standard)
	}
	
	var confirmation *ConfirmationService
	if config.ConfirmProvider != nil {
		confirmation = NewConfirmationService(config.ConfirmProvider, config.Policy)
	}
	
	monitor := NewResourceMonitor(config.ResourceLimits)
	
	var sandboxMgr *SandboxManager
	if config.EnableSandbox || config.Policy.SandboxEnabled {
		sandboxMgr = NewSandboxManager(config.Policy)
	}
	
	return &SecurityManager{
		policy:       config.Policy,
		confirmation: confirmation,
		monitor:      monitor,
		sandboxMgr:   sandboxMgr,
	}
}

// GetPolicy returns the security policy
func (sm *SecurityManager) GetPolicy() *SecurityPolicy {
	return sm.policy
}

// GetConfirmationService returns the confirmation service
func (sm *SecurityManager) GetConfirmationService() *ConfirmationService {
	return sm.confirmation
}

// GetResourceMonitor returns the resource monitor
func (sm *SecurityManager) GetResourceMonitor() *ResourceMonitor {
	return sm.monitor
}

// GetSandboxManager returns the sandbox manager
func (sm *SecurityManager) GetSandboxManager() *SandboxManager {
	return sm.sandboxMgr
}

// ValidateCommand validates a command against security policy
func (sm *SecurityManager) ValidateCommand(command string) error {
	return sm.policy.ValidateCommand(command)
}

// ValidatePath validates a file path against security policy
func (sm *SecurityManager) ValidatePath(path string) error {
	return sm.policy.ValidatePath(path)
}

// ExecuteCommand executes a command with full security enforcement
func (sm *SecurityManager) ExecuteCommand(command string, metadata map[string]interface{}) error {
	// Validate command
	if err := sm.policy.ValidateCommand(command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}
	
	// Check if confirmation is required
	if sm.confirmation != nil && sm.policy.RequiresConfirmation(command) {
		confirmed, err := sm.confirmation.RequestCommandConfirmation(command)
		if err != nil {
			return fmt.Errorf("confirmation failed: %w", err)
		}
		if !confirmed {
			return fmt.Errorf("command execution denied by user")
		}
	}
	
	// Create resource tracker
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["command"] = command
	metadata["security_level"] = sm.policy.Level
	
	tracker, err := NewResourceTracker(sm.monitor, "command_execution", metadata)
	if err != nil {
		return fmt.Errorf("resource tracking failed: %w", err)
	}
	defer tracker.Close()
	
	// Execute command
	return sm.executeCommandWithTimeout(tracker.GetContext(), command)
}

// ExecuteInSandbox executes a command in a sandbox environment
func (sm *SecurityManager) ExecuteInSandbox(sandboxID, command string, metadata map[string]interface{}) error {
	if sm.sandboxMgr == nil {
		return fmt.Errorf("sandbox manager not available")
	}
	
	sandbox, exists := sm.sandboxMgr.GetSandbox(sandboxID)
	if !exists {
		return fmt.Errorf("sandbox %s not found", sandboxID)
	}
	
	// Use sandbox policy for validation
	sandboxPolicy := sandbox.GetPolicy()
	if err := sandboxPolicy.ValidateCommand(command); err != nil {
		return fmt.Errorf("sandbox command validation failed: %w", err)
	}
	
	// Create resource tracker
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["command"] = command
	metadata["sandbox_id"] = sandboxID
	metadata["security_level"] = sandboxPolicy.Level
	
	tracker, err := NewResourceTracker(sm.monitor, "sandbox_execution", metadata)
	if err != nil {
		return fmt.Errorf("resource tracking failed: %w", err)
	}
	defer tracker.Close()
	
	// Execute in sandbox context
	return sm.executeCommandInSandbox(tracker.GetContext(), sandbox, command)
}

// CreateSandbox creates a new sandbox environment
func (sm *SecurityManager) CreateSandbox(id string, config SandboxConfig) error {
	if sm.sandboxMgr == nil {
		return fmt.Errorf("sandbox manager not available")
	}
	
	_, err := sm.sandboxMgr.CreateSandbox(id, config)
	return err
}

// DestroySandbox removes a sandbox environment
func (sm *SecurityManager) DestroySandbox(id string) error {
	if sm.sandboxMgr == nil {
		return fmt.Errorf("sandbox manager not available")
	}
	
	return sm.sandboxMgr.DestroySandbox(id)
}

// ValidateFileOperation validates a file operation
func (sm *SecurityManager) ValidateFileOperation(operation, path string, size int64) error {
	// Validate path
	if err := sm.policy.ValidatePath(path); err != nil {
		return err
	}
	
	// Validate file size
	if err := sm.policy.ValidateFileSize(size); err != nil {
		return err
	}
	
	// Check if confirmation is required for this operation
	if sm.confirmation != nil {
		riskLevel := sm.confirmation.assessFileOperationRisk(operation, path)
		if riskLevel >= RiskMedium {
			details := map[string]interface{}{
				"size": size,
				"risk_level": riskLevel.String(),
			}
			
			confirmed, err := sm.confirmation.RequestFileOperationConfirmation(operation, path, details)
			if err != nil {
				return fmt.Errorf("file operation confirmation failed: %w", err)
			}
			if !confirmed {
				return fmt.Errorf("file operation denied by user")
			}
		}
	}
	
	return nil
}

// GetResourceUsage returns current resource usage
func (sm *SecurityManager) GetResourceUsage() ResourceUsage {
	return sm.monitor.GetUsage()
}

// Stop gracefully stops the security manager
func (sm *SecurityManager) Stop() error {
	var errors []string
	
	// Stop resource monitor
	if sm.monitor != nil {
		sm.monitor.Stop()
	}
	
	// Cleanup all sandboxes
	if sm.sandboxMgr != nil {
		if err := sm.sandboxMgr.CleanupAll(); err != nil {
			errors = append(errors, fmt.Sprintf("sandbox cleanup: %v", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// executeCommandWithTimeout executes a command with timeout
func (sm *SecurityManager) executeCommandWithTimeout(ctx context.Context, command string) error {
	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}
	
	// Create command
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	
	// Set working directory if specified in policy
	if sm.policy.SandboxEnabled && sm.policy.SandboxRoot != "" {
		cmd.Dir = sm.policy.SandboxRoot
	}
	
	// Execute command
	return cmd.Run()
}

// executeCommandInSandbox executes a command within a sandbox
func (sm *SecurityManager) executeCommandInSandbox(ctx context.Context, sandbox *SandboxEnvironment, command string) error {
	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}
	
	// Create command with sandbox context
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = sandbox.GetRootDir()
	
	// Execute command
	return cmd.Run()
}

// DefaultSecurityManager creates a security manager with sensible defaults
func DefaultSecurityManager() *SecurityManager {
	config := SecurityManagerConfig{
		Policy:          NewSecurityPolicy(Standard),
		ConfirmProvider: NewConsoleConfirmationProvider(),
		ResourceLimits:  DefaultResourceLimits(),
		EnableSandbox:   false,
	}
	
	return NewSecurityManager(config)
}

// StrictSecurityManager creates a security manager with strict security settings
func StrictSecurityManager() *SecurityManager {
	config := SecurityManagerConfig{
		Policy:          NewSecurityPolicy(Strict),
		ConfirmProvider: NewConsoleConfirmationProvider(),
		ResourceLimits:  StrictResourceLimits(),
		EnableSandbox:   true,
	}
	
	return NewSecurityManager(config)
}

// SandboxSecurityManager creates a security manager with full sandboxing
func SandboxSecurityManager() *SecurityManager {
	config := SecurityManagerConfig{
		Policy:          NewSecurityPolicy(Sandbox),
		ConfirmProvider: NewConsoleConfirmationProvider(),
		ResourceLimits:  StrictResourceLimits(),
		EnableSandbox:   true,
	}
	
	return NewSecurityManager(config)
}