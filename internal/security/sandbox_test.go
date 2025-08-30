package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSandbox(t *testing.T) {
	policy := NewSecurityPolicy(Sandbox)
	
	t.Run("with_temp_root", func(t *testing.T) {
		config := SandboxConfig{}
		
		sandbox, err := NewSandbox(policy, config)
		if err != nil {
			t.Fatalf("Failed to create sandbox: %v", err)
		}
		defer sandbox.Cleanup()
		
		if sandbox.GetRootDir() == "" {
			t.Error("Root directory should be set")
		}
		
		if sandbox.GetTempDir() == "" {
			t.Error("Temp directory should be set")
		}
		
		// Check that directories exist
		if _, err := os.Stat(sandbox.GetRootDir()); os.IsNotExist(err) {
			t.Error("Root directory should exist")
		}
		
		if _, err := os.Stat(sandbox.GetTempDir()); os.IsNotExist(err) {
			t.Error("Temp directory should exist")
		}
	})
	
	t.Run("with_custom_root", func(t *testing.T) {
		tempDir := os.TempDir()
		customRoot := filepath.Join(tempDir, "test-sandbox")
		
		config := SandboxConfig{
			RootDir: customRoot,
		}
		
		sandbox, err := NewSandbox(policy, config)
		if err != nil {
			t.Fatalf("Failed to create sandbox: %v", err)
		}
		defer func() {
			sandbox.Cleanup()
			os.RemoveAll(customRoot) // Clean up custom directory
		}()
		
		if !strings.Contains(sandbox.GetRootDir(), "test-sandbox") {
			t.Error("Should use custom root directory")
		}
	})
	
	t.Run("sandbox_disabled_policy", func(t *testing.T) {
		disabledPolicy := NewSecurityPolicy(Standard)
		config := SandboxConfig{}
		
		_, err := NewSandbox(disabledPolicy, config)
		if err == nil {
			t.Error("Should fail when sandbox is disabled in policy")
		}
	})
}

func TestSandbox_ResolvePath(t *testing.T) {
	policy := NewSecurityPolicy(Sandbox)
	config := SandboxConfig{}
	
	sandbox, err := NewSandbox(policy, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()
	
	testCases := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "relative_path",
			path:        "data/file.txt",
			expectError: false,
		},
		{
			name:        "absolute_path_converted",
			path:        "/data/file.txt",
			expectError: false,
		},
		{
			name:        "current_directory",
			path:        ".",
			expectError: false,
		},
		{
			name:        "parent_directory_escape",
			path:        "../escape.txt",
			expectError: true,
		},
		{
			name:        "deep_escape_attempt",
			path:        "../../../../../../etc/passwd",
			expectError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolvedPath, err := sandbox.ResolvePath(tc.path)
			
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error for path: %s", tc.path)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for path %s: %v", tc.path, err)
				} else {
					// Both paths should be absolute for comparison
					absRoot, err := filepath.Abs(sandbox.GetRootDir())
					if err != nil {
						t.Fatalf("Failed to get absolute sandbox root: %v", err)
					}
					
					absResolved, err := filepath.Abs(resolvedPath)
					if err != nil {
						t.Fatalf("Failed to get absolute resolved path: %v", err)
					}
					
					// Resolved path should be within sandbox
					if !strings.HasPrefix(absResolved, absRoot) {
						t.Errorf("Resolved path %s should be within sandbox root %s", 
							absResolved, absRoot)
					}
				}
			}
		})
	}
}

func TestSandbox_FileOperations(t *testing.T) {
	policy := NewSecurityPolicy(Sandbox)
	config := SandboxConfig{}
	
	sandbox, err := NewSandbox(policy, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()
	
	t.Run("create_and_read_file", func(t *testing.T) {
		testContent := []byte("Hello, sandbox world!")
		testPath := "test/file.txt"
		
		// Create file
		err := sandbox.CreateFile(testPath, testContent)
		if err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
		
		// Read file
		readContent, err := sandbox.ReadFile(testPath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		
		if string(readContent) != string(testContent) {
			t.Errorf("Expected content %s, got %s", testContent, readContent)
		}
	})
	
	t.Run("create_file_too_large", func(t *testing.T) {
		largeContent := make([]byte, policy.MaxFileSize+1)
		testPath := "large_file.txt"
		
		err := sandbox.CreateFile(testPath, largeContent)
		if err == nil {
			t.Error("Should fail to create file exceeding size limit")
		}
	})
	
	t.Run("list_files", func(t *testing.T) {
		// Create some test files
		testFiles := map[string][]byte{
			"file1.txt": []byte("content1"),
			"file2.txt": []byte("content2"),
			"dir/file3.txt": []byte("content3"),
		}
		
		for path, content := range testFiles {
			err := sandbox.CreateFile(path, content)
			if err != nil {
				t.Fatalf("Failed to create test file %s: %v", path, err)
			}
		}
		
		// List files in root
		files, err := sandbox.ListFiles(".")
		if err != nil {
			t.Fatalf("Failed to list files: %v", err)
		}
		
		if len(files) == 0 {
			t.Error("Should find some files in sandbox")
		}
		
		// Check that we can find our created files
		foundFile1 := false
		for _, file := range files {
			if file.Name() == "file1.txt" {
				foundFile1 = true
			}
		}
		
		if !foundFile1 {
			t.Error("Should find file1.txt in sandbox listing")
		}
	})
}

func TestSandbox_CopyOperations(t *testing.T) {
	policy := NewSecurityPolicy(Sandbox)
	config := SandboxConfig{}
	
	sandbox, err := NewSandbox(policy, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()
	
	t.Run("copy_from_host", func(t *testing.T) {
		// Create temporary host file
		hostFile, err := os.CreateTemp("", "host-test-")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(hostFile.Name())
		
		testContent := []byte("host content")
		if _, err := hostFile.Write(testContent); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		hostFile.Close()
		
		// Copy to sandbox
		sandboxPath := "copied_from_host.txt"
		err = sandbox.CopyFromHost(hostFile.Name(), sandboxPath)
		if err != nil {
			t.Fatalf("Failed to copy from host: %v", err)
		}
		
		// Verify content
		readContent, err := sandbox.ReadFile(sandboxPath)
		if err != nil {
			t.Fatalf("Failed to read copied file: %v", err)
		}
		
		if string(readContent) != string(testContent) {
			t.Errorf("Expected content %s, got %s", testContent, readContent)
		}
	})
	
	t.Run("copy_to_host", func(t *testing.T) {
		// Create file in sandbox
		testContent := []byte("sandbox content")
		sandboxPath := "to_copy_out.txt"
		
		err := sandbox.CreateFile(sandboxPath, testContent)
		if err != nil {
			t.Fatalf("Failed to create sandbox file: %v", err)
		}
		
		// Create temp directory for host destination
		tempDir, err := os.MkdirTemp("", "host-dest-")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)
		
		hostPath := filepath.Join(tempDir, "copied_to_host.txt")
		
		// Copy to host
		err = sandbox.CopyToHost(sandboxPath, hostPath)
		if err != nil {
			t.Fatalf("Failed to copy to host: %v", err)
		}
		
		// Verify content on host
		readContent, err := os.ReadFile(hostPath)
		if err != nil {
			t.Fatalf("Failed to read host file: %v", err)
		}
		
		if string(readContent) != string(testContent) {
			t.Errorf("Expected content %s, got %s", testContent, readContent)
		}
	})
}

func TestSandbox_GetDiskUsage(t *testing.T) {
	policy := NewSecurityPolicy(Sandbox)
	config := SandboxConfig{}
	
	sandbox, err := NewSandbox(policy, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	defer sandbox.Cleanup()
	
	// Initial usage should be minimal
	initialUsage, err := sandbox.GetDiskUsage()
	if err != nil {
		t.Fatalf("Failed to get disk usage: %v", err)
	}
	
	// Create some files
	testContent := make([]byte, 1024) // 1KB
	for i := 0; i < 5; i++ {
		path := filepath.Join("test", fmt.Sprintf("file%d.txt", i))
		err := sandbox.CreateFile(path, testContent)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}
	
	// Usage should have increased
	finalUsage, err := sandbox.GetDiskUsage()
	if err != nil {
		t.Fatalf("Failed to get final disk usage: %v", err)
	}
	
	if finalUsage <= initialUsage {
		t.Errorf("Expected disk usage to increase from %d to %d", initialUsage, finalUsage)
	}
	
	expectedIncrease := int64(5 * 1024) // 5KB
	actualIncrease := finalUsage - initialUsage
	
	// Allow some tolerance for filesystem overhead
	if actualIncrease < expectedIncrease {
		t.Errorf("Expected at least %d bytes increase, got %d", expectedIncrease, actualIncrease)
	}
}

func TestSandboxManager(t *testing.T) {
	policy := NewSecurityPolicy(Sandbox)
	manager := NewSandboxManager(policy)
	
	t.Run("create_and_get_sandbox", func(t *testing.T) {
		config := SandboxConfig{}
		sandboxID := "test-sandbox-1"
		
		sandbox, err := manager.CreateSandbox(sandboxID, config)
		if err != nil {
			t.Fatalf("Failed to create sandbox: %v", err)
		}
		
		retrievedSandbox, exists := manager.GetSandbox(sandboxID)
		if !exists {
			t.Error("Sandbox should exist after creation")
		}
		
		if retrievedSandbox != sandbox {
			t.Error("Retrieved sandbox should be the same instance")
		}
	})
	
	t.Run("duplicate_sandbox_id", func(t *testing.T) {
		config := SandboxConfig{}
		sandboxID := "duplicate-test"
		
		// Create first sandbox
		_, err := manager.CreateSandbox(sandboxID, config)
		if err != nil {
			t.Fatalf("Failed to create first sandbox: %v", err)
		}
		
		// Try to create another with same ID
		_, err = manager.CreateSandbox(sandboxID, config)
		if err == nil {
			t.Error("Should fail to create duplicate sandbox ID")
		}
	})
	
	t.Run("list_sandboxes", func(t *testing.T) {
		// Create multiple sandboxes
		sandboxIDs := []string{"list-test-1", "list-test-2", "list-test-3"}
		config := SandboxConfig{}
		
		for _, id := range sandboxIDs {
			_, err := manager.CreateSandbox(id, config)
			if err != nil {
				t.Fatalf("Failed to create sandbox %s: %v", id, err)
			}
		}
		
		listedIDs := manager.ListSandboxes()
		
		// Check that all our sandboxes are listed
		for _, expectedID := range sandboxIDs {
			found := false
			for _, listedID := range listedIDs {
				if listedID == expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Sandbox %s not found in listing", expectedID)
			}
		}
	})
	
	t.Run("destroy_sandbox", func(t *testing.T) {
		config := SandboxConfig{}
		sandboxID := "destroy-test"
		
		// Create sandbox
		_, err := manager.CreateSandbox(sandboxID, config)
		if err != nil {
			t.Fatalf("Failed to create sandbox: %v", err)
		}
		
		// Verify it exists
		_, exists := manager.GetSandbox(sandboxID)
		if !exists {
			t.Error("Sandbox should exist before destruction")
		}
		
		// Destroy it
		err = manager.DestroySandbox(sandboxID)
		if err != nil {
			t.Fatalf("Failed to destroy sandbox: %v", err)
		}
		
		// Verify it's gone
		_, exists = manager.GetSandbox(sandboxID)
		if exists {
			t.Error("Sandbox should not exist after destruction")
		}
	})
	
	t.Run("cleanup_all", func(t *testing.T) {
		// Create multiple sandboxes
		config := SandboxConfig{}
		for i := 0; i < 3; i++ {
			sandboxID := fmt.Sprintf("cleanup-test-%d", i)
			_, err := manager.CreateSandbox(sandboxID, config)
			if err != nil {
				t.Fatalf("Failed to create sandbox %s: %v", sandboxID, err)
			}
		}
		
		// Verify they exist
		listedIDs := manager.ListSandboxes()
		if len(listedIDs) == 0 {
			t.Error("Should have sandboxes before cleanup")
		}
		
		// Cleanup all
		err := manager.CleanupAll()
		if err != nil {
			t.Fatalf("Failed to cleanup all sandboxes: %v", err)
		}
		
		// Verify they're all gone
		listedIDs = manager.ListSandboxes()
		if len(listedIDs) != 0 {
			t.Errorf("Should have no sandboxes after cleanup, found: %v", listedIDs)
		}
	})
}

func TestSandbox_WithTimeout(t *testing.T) {
	policy := NewSecurityPolicy(Sandbox)
	
	config := SandboxConfig{
		Timeout: 100 * time.Millisecond, // Very short timeout
	}
	
	sandbox, err := NewSandbox(policy, config)
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}
	
	// Wait for timeout to trigger cleanup
	time.Sleep(200 * time.Millisecond)
	
	// Try to use sandbox - should still work as cleanup is async
	// But the cleanup goroutine should have run
	testContent := []byte("test after timeout")
	err = sandbox.CreateFile("timeout_test.txt", testContent)
	
	// The behavior here depends on implementation - the sandbox might still work
	// or might be cleaned up. This test mainly ensures no panic occurs.
	_ = err // Don't fail on error as cleanup timing is not guaranteed
}