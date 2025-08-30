package security

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// ConfirmationRequest represents a request for user confirmation
type ConfirmationRequest struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Message     string                 `json:"message"`
	Command     string                 `json:"command,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	RiskLevel   RiskLevel              `json:"risk_level"`
	Timeout     time.Duration          `json:"timeout"`
	Options     []string               `json:"options"`
	DefaultOpt  string                 `json:"default_option"`
	Timestamp   time.Time              `json:"timestamp"`
}

// ConfirmationResponse represents the user's response
type ConfirmationResponse struct {
	ID        string    `json:"id"`
	Approved  bool      `json:"approved"`
	Option    string    `json:"option"`
	Timestamp time.Time `json:"timestamp"`
}

// RiskLevel defines the risk level of an operation
type RiskLevel int

const (
	// Low risk operations with minimal impact
	RiskLow RiskLevel = iota
	// Medium risk operations that could affect files/data
	RiskMedium
	// High risk operations that could cause system damage
	RiskHigh
	// Critical risk operations that could cause irreversible damage
	RiskCritical
)

// String returns the string representation of risk level
func (r RiskLevel) String() string {
	switch r {
	case RiskLow:
		return "LOW"
	case RiskMedium:
		return "MEDIUM"
	case RiskHigh:
		return "HIGH"
	case RiskCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ConfirmationProvider defines the interface for user confirmation
type ConfirmationProvider interface {
	RequestConfirmation(req ConfirmationRequest) (ConfirmationResponse, error)
	IsAvailable() bool
}

// ConsoleConfirmationProvider provides confirmation through console input
type ConsoleConfirmationProvider struct {
	reader *bufio.Reader
}

// NewConsoleConfirmationProvider creates a new console confirmation provider
func NewConsoleConfirmationProvider() *ConsoleConfirmationProvider {
	return &ConsoleConfirmationProvider{
		reader: bufio.NewReader(os.Stdin),
	}
}

// RequestConfirmation requests confirmation from user via console
func (c *ConsoleConfirmationProvider) RequestConfirmation(req ConfirmationRequest) (ConfirmationResponse, error) {
	response := ConfirmationResponse{
		ID:        req.ID,
		Timestamp: time.Now(),
	}
	
	// Display confirmation request
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("🛡️  SECURITY CONFIRMATION REQUIRED [%s]\n", req.RiskLevel.String())
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Title: %s\n", req.Title)
	fmt.Printf("Message: %s\n", req.Message)
	
	if req.Command != "" {
		fmt.Printf("Command: %s\n", req.Command)
	}
	
	// Display details
	if len(req.Details) > 0 {
		fmt.Println("\nDetails:")
		for key, value := range req.Details {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}
	
	// Display options
	fmt.Println("\nOptions:")
	for i, option := range req.Options {
		prefix := " "
		if option == req.DefaultOpt {
			prefix = "*"
		}
		fmt.Printf(" %s %d. %s\n", prefix, i+1, option)
	}
	
	if req.DefaultOpt != "" {
		fmt.Printf("\n* Default option (press Enter): %s\n", req.DefaultOpt)
	}
	
	if req.Timeout > 0 {
		fmt.Printf("⏱️  Timeout: %v\n", req.Timeout)
	}
	
	fmt.Printf("\nYour choice (1-%d): ", len(req.Options))
	
	// Handle timeout
	inputChan := make(chan string, 1)
	go func() {
		input, err := c.reader.ReadString('\n')
		if err != nil {
			inputChan <- ""
		} else {
			inputChan <- strings.TrimSpace(input)
		}
	}()
	
	var input string
	if req.Timeout > 0 {
		select {
		case input = <-inputChan:
		case <-time.After(req.Timeout):
			fmt.Println("\n⏱️  Timeout reached. Using default option.")
			input = ""
		}
	} else {
		input = <-inputChan
	}
	
	// Parse input
	if input == "" && req.DefaultOpt != "" {
		response.Option = req.DefaultOpt
		response.Approved = !strings.Contains(strings.ToLower(req.DefaultOpt), "deny") &&
			!strings.Contains(strings.ToLower(req.DefaultOpt), "cancel")
	} else if input != "" {
		// Try to parse as number
		var optionIndex int
		if _, err := fmt.Sscanf(input, "%d", &optionIndex); err == nil && 
			optionIndex >= 1 && optionIndex <= len(req.Options) {
			response.Option = req.Options[optionIndex-1]
			response.Approved = !strings.Contains(strings.ToLower(response.Option), "deny") &&
				!strings.Contains(strings.ToLower(response.Option), "cancel")
		} else {
			// Try to match input with options
			inputLower := strings.ToLower(input)
			found := false
			for _, option := range req.Options {
				if strings.Contains(strings.ToLower(option), inputLower) {
					response.Option = option
					response.Approved = !strings.Contains(strings.ToLower(option), "deny") &&
						!strings.Contains(strings.ToLower(option), "cancel")
					found = true
					break
				}
			}
			
			if !found {
				return response, fmt.Errorf("invalid option: %s", input)
			}
		}
	} else {
		return response, fmt.Errorf("no option selected and no default available")
	}
	
	// Display result
	if response.Approved {
		fmt.Printf("✅ Approved: %s\n", response.Option)
	} else {
		fmt.Printf("❌ Denied: %s\n", response.Option)
	}
	
	fmt.Println(strings.Repeat("=", 60) + "\n")
	
	return response, nil
}

// IsAvailable checks if console confirmation is available
func (c *ConsoleConfirmationProvider) IsAvailable() bool {
	// Check if we have a terminal
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// ConfirmationService manages user confirmations for security operations
type ConfirmationService struct {
	provider ConfirmationProvider
	policy   *SecurityPolicy
}

// NewConfirmationService creates a new confirmation service
func NewConfirmationService(provider ConfirmationProvider, policy *SecurityPolicy) *ConfirmationService {
	return &ConfirmationService{
		provider: provider,
		policy:   policy,
	}
}

// RequestCommandConfirmation requests confirmation for a command execution
func (cs *ConfirmationService) RequestCommandConfirmation(command string) (bool, error) {
	if !cs.policy.RequiresConfirmation(command) {
		return true, nil // No confirmation needed
	}
	
	// Determine risk level
	riskLevel := cs.assessCommandRisk(command)
	
	req := ConfirmationRequest{
		ID:      fmt.Sprintf("cmd-%d", time.Now().UnixNano()),
		Title:   "Command Execution Confirmation",
		Message: fmt.Sprintf("The following command requires confirmation before execution:"),
		Command: command,
		Details: map[string]interface{}{
			"risk_level": riskLevel.String(),
			"timestamp": time.Now().Format(time.RFC3339),
		},
		RiskLevel:  riskLevel,
		Timeout:    30 * time.Second,
		Options:    []string{"Allow", "Deny"},
		DefaultOpt: "Deny",
		Timestamp:  time.Now(),
	}
	
	// Customize options based on risk level
	switch riskLevel {
	case RiskHigh, RiskCritical:
		req.Options = []string{"Allow (I understand the risks)", "Deny"}
		req.Timeout = 60 * time.Second
	case RiskMedium:
		req.Options = []string{"Allow", "Deny", "Review Details"}
	}
	
	response, err := cs.provider.RequestConfirmation(req)
	if err != nil {
		return false, fmt.Errorf("confirmation failed: %w", err)
	}
	
	return response.Approved, nil
}

// RequestFileOperationConfirmation requests confirmation for file operations
func (cs *ConfirmationService) RequestFileOperationConfirmation(operation, path string, details map[string]interface{}) (bool, error) {
	riskLevel := cs.assessFileOperationRisk(operation, path)
	
	// Auto-approve low risk operations
	if riskLevel == RiskLow {
		return true, nil
	}
	
	req := ConfirmationRequest{
		ID:      fmt.Sprintf("file-%d", time.Now().UnixNano()),
		Title:   "File Operation Confirmation",
		Message: fmt.Sprintf("The following file operation requires confirmation:"),
		Details: map[string]interface{}{
			"operation":  operation,
			"path":       path,
			"risk_level": riskLevel.String(),
		},
		RiskLevel:  riskLevel,
		Timeout:    15 * time.Second,
		Options:    []string{"Allow", "Deny"},
		DefaultOpt: "Allow",
		Timestamp:  time.Now(),
	}
	
	// Add custom details
	for k, v := range details {
		req.Details[k] = v
	}
	
	// Adjust based on risk level
	if riskLevel >= RiskHigh {
		req.DefaultOpt = "Deny"
		req.Timeout = 30 * time.Second
	}
	
	response, err := cs.provider.RequestConfirmation(req)
	if err != nil {
		return false, fmt.Errorf("confirmation failed: %w", err)
	}
	
	return response.Approved, nil
}

// assessCommandRisk determines the risk level of a command
func (cs *ConfirmationService) assessCommandRisk(command string) RiskLevel {
	cmdLower := strings.ToLower(command)
	
	// Critical risk commands
	criticalPatterns := []string{
		"rm -rf", "format", "fdisk", "mkfs", "dd if=", ">(", "| sh", "| bash",
	}
	for _, pattern := range criticalPatterns {
		if strings.Contains(cmdLower, pattern) {
			return RiskCritical
		}
	}
	
	// High risk commands (check these first - more specific patterns)
	highRiskCommands := []string{
		"git reset --hard", "git clean -fd", "npm publish", "cargo publish",
		"pip install", "chmod 777",
	}
	for _, cmd := range highRiskCommands {
		if strings.Contains(cmdLower, cmd) {
			return RiskHigh
		}
	}
	
	// Medium risk commands 
	mediumRiskCommands := []string{
		"git push", "git merge", "npm install", "go mod", "cargo build",
		"make install", "make", "chmod", "chown",
	}
	for _, cmd := range mediumRiskCommands {
		if strings.Contains(cmdLower, cmd) {
			return RiskMedium
		}
	}
	
	return RiskLow
}

// assessFileOperationRisk determines the risk level of a file operation
func (cs *ConfirmationService) assessFileOperationRisk(operation, path string) RiskLevel {
	pathLower := strings.ToLower(path)
	opLower := strings.ToLower(operation)
	
	// Critical paths
	criticalPaths := []string{"/etc", "/sys", "/proc", "/dev", "/boot"}
	for _, critPath := range criticalPaths {
		if strings.HasPrefix(pathLower, critPath) {
			return RiskCritical
		}
	}
	
	// High risk operations
	if opLower == "delete" || opLower == "remove" {
		return RiskHigh
	}
	
	// Medium risk for system directories
	systemPaths := []string{"/usr", "/var", "/opt"}
	for _, sysPath := range systemPaths {
		if strings.HasPrefix(pathLower, sysPath) {
			return RiskMedium
		}
	}
	
	return RiskLow
}