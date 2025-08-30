package security

import (
	"testing"
	"time"
)

func TestNewSecurityManager(t *testing.T) {
	t.Run("with_default_config", func(t *testing.T) {
		config := SecurityManagerConfig{}
		manager := NewSecurityManager(config)
		defer manager.Stop()
		
		// Should have default policy
		if manager.GetPolicy() == nil {
			t.Error("Should have default policy")
		}
		
		if manager.GetPolicy().Level != Standard {
			t.Error("Default policy should be Standard level")
		}
		
		// Should have resource monitor
		if manager.GetResourceMonitor() == nil {
			t.Error("Should have resource monitor")
		}
	})
	
	t.Run("with_custom_config", func(t *testing.T) {
		policy := NewSecurityPolicy(Strict)
		confirmProvider := NewConsoleConfirmationProvider()
		limits := StrictResourceLimits()
		
		config := SecurityManagerConfig{
			Policy:          policy,
			ConfirmProvider: confirmProvider,
			ResourceLimits:  limits,
			EnableSandbox:   true,
		}
		
		manager := NewSecurityManager(config)
		defer manager.Stop()
		
		if manager.GetPolicy().Level != Strict {
			t.Error("Should use custom policy")
		}
		
		if manager.GetConfirmationService() == nil {
			t.Error("Should have confirmation service")
		}
		
		if manager.GetSandboxManager() == nil {
			t.Error("Should have sandbox manager when enabled")
		}
	})
}

func TestSecurityManager_ValidateCommand(t *testing.T) {
	manager := DefaultSecurityManager()
	defer manager.Stop()
	
	testCases := []struct {
		name        string
		command     string
		expectError bool
	}{
		{
			name:        "safe_command",
			command:     "ls -la",
			expectError: false,
		},
		{
			name:        "dangerous_command",
			command:     "rm -rf /",
			expectError: true,
		},
		{
			name:        "blocked_command",
			command:     "sudo reboot",
			expectError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.ValidateCommand(tc.command)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error for command: %s", tc.command)
			}
			
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for command %s: %v", tc.command, err)
			}
		})
	}
}

func TestSecurityManager_ValidatePath(t *testing.T) {
	manager := DefaultSecurityManager()
	defer manager.Stop()
	
	testCases := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "safe_path",
			path:        "/home/user/project/file.txt",
			expectError: false,
		},
		{
			name:        "blocked_path",
			path:        "/etc/passwd",
			expectError: true,
		},
		{
			name:        "path_traversal",
			path:        "../../../secret.txt",
			expectError: true,
		},
		{
			name:        "blocked_extension",
			path:        "malware.exe",
			expectError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.ValidatePath(tc.path)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error for path: %s", tc.path)
			}
			
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for path %s: %v", tc.path, err)
			}
		})
	}
}

func TestSecurityManager_ValidateFileOperation(t *testing.T) {
	manager := DefaultSecurityManager()
	defer manager.Stop()
	
	testCases := []struct {
		name        string
		operation   string
		path        string
		size        int64
		expectError bool
	}{
		{
			name:        "safe_read",
			operation:   "read",
			path:        "/home/user/data.txt",
			size:        1024,
			expectError: false,
		},
		{
			name:        "safe_write",
			operation:   "write",
			path:        "/home/user/output.txt",
			size:        2048,
			expectError: false,
		},
		{
			name:        "blocked_path",
			operation:   "read",
			path:        "/etc/passwd",
			size:        1024,
			expectError: true,
		},
		{
			name:        "file_too_large",
			operation:   "write",
			path:        "/home/user/large.txt",
			size:        100 * 1024 * 1024, // 100MB (exceeds default limit)
			expectError: true,
		},
		{
			name:        "blocked_extension",
			operation:   "write",
			path:        "/home/user/malware.exe",
			size:        1024,
			expectError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.ValidateFileOperation(tc.operation, tc.path, tc.size)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error for operation %s on %s", tc.operation, tc.path)
			}
			
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for operation %s on %s: %v", tc.operation, tc.path, err)
			}
		})
	}
}

func TestSecurityManager_SandboxOperations(t *testing.T) {
	manager := SandboxSecurityManager()
	defer manager.Stop()
	
	sandboxID := "test-sandbox"
	config := SandboxConfig{}
	
	t.Run("create_sandbox", func(t *testing.T) {
		err := manager.CreateSandbox(sandboxID, config)
		if err != nil {
			t.Fatalf("Failed to create sandbox: %v", err)
		}
		
		// Verify sandbox exists
		sandboxMgr := manager.GetSandboxManager()
		if sandboxMgr == nil {
			t.Fatal("Should have sandbox manager")
		}
		
		_, exists := sandboxMgr.GetSandbox(sandboxID)
		if !exists {
			t.Error("Sandbox should exist after creation")
		}
	})
	
	t.Run("destroy_sandbox", func(t *testing.T) {
		err := manager.DestroySandbox(sandboxID)
		if err != nil {
			t.Fatalf("Failed to destroy sandbox: %v", err)
		}
		
		// Verify sandbox is gone
		sandboxMgr := manager.GetSandboxManager()
		_, exists := sandboxMgr.GetSandbox(sandboxID)
		if exists {
			t.Error("Sandbox should not exist after destruction")
		}
	})
}

func TestSecurityManager_GetResourceUsage(t *testing.T) {
	manager := DefaultSecurityManager()
	defer manager.Stop()
	
	// Initial usage
	usage := manager.GetResourceUsage()
	if usage.ActiveOperations != 0 {
		t.Error("Should start with no active operations")
	}
	
	// Start some operations through the monitor
	monitor := manager.GetResourceMonitor()
	metadata := map[string]interface{}{"test": "resource_usage"}
	
	op, err := monitor.StartOperation("test-op", "test", metadata)
	if err != nil {
		t.Fatalf("Failed to start operation: %v", err)
	}
	
	// Check updated usage
	usage = manager.GetResourceUsage()
	if usage.ActiveOperations != 1 {
		t.Error("Should have 1 active operation")
	}
	
	// End operation
	monitor.EndOperation("test-op")
	
	// Verify context is cancelled
	select {
	case <-op.Context.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("Operation context should be cancelled")
	}
	
	usage = manager.GetResourceUsage()
	if usage.ActiveOperations != 0 {
		t.Error("Should have 0 active operations after end")
	}
}

func TestDefaultSecurityManager(t *testing.T) {
	manager := DefaultSecurityManager()
	defer manager.Stop()
	
	if manager.GetPolicy().Level != Standard {
		t.Error("Default manager should use Standard security level")
	}
	
	if manager.GetConfirmationService() == nil {
		t.Error("Default manager should have confirmation service")
	}
	
	if manager.GetResourceMonitor() == nil {
		t.Error("Default manager should have resource monitor")
	}
	
	// Sandbox should not be enabled by default
	if manager.GetSandboxManager() != nil {
		t.Error("Default manager should not have sandbox manager")
	}
}

func TestStrictSecurityManager(t *testing.T) {
	manager := StrictSecurityManager()
	defer manager.Stop()
	
	if manager.GetPolicy().Level != Strict {
		t.Error("Strict manager should use Strict security level")
	}
	
	if manager.GetSandboxManager() == nil {
		t.Error("Strict manager should have sandbox manager")
	}
}

func TestSandboxSecurityManager(t *testing.T) {
	manager := SandboxSecurityManager()
	defer manager.Stop()
	
	if manager.GetPolicy().Level != Sandbox {
		t.Error("Sandbox manager should use Sandbox security level")
	}
	
	if !manager.GetPolicy().SandboxEnabled {
		t.Error("Sandbox manager should have sandbox enabled")
	}
	
	if manager.GetSandboxManager() == nil {
		t.Error("Sandbox manager should have sandbox manager")
	}
}

func TestSecurityManager_Stop(t *testing.T) {
	manager := DefaultSecurityManager()
	
	// Start some operations
	monitor := manager.GetResourceMonitor()
	metadata := map[string]interface{}{"test": "stop_test"}
	
	op1, _ := monitor.StartOperation("op1", "test", metadata)
	op2, _ := monitor.StartOperation("op2", "test", metadata)
	
	// Stop manager
	err := manager.Stop()
	if err != nil {
		t.Errorf("Stop should not return error: %v", err)
	}
	
	// Verify operations are cancelled
	operations := []*Operation{op1, op2}
	for i, op := range operations {
		if op != nil {
			select {
			case <-op.Context.Done():
				// Good
			case <-time.After(100 * time.Millisecond):
				t.Errorf("Operation %d context should be cancelled after Stop", i+1)
			}
		}
	}
}