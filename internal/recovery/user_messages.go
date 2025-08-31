package recovery

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// UserMessage represents a user-friendly error message with suggested fixes
type UserMessage struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Severity    MessageSeverity   `json:"severity"`
	Category    MessageCategory   `json:"category"`
	Causes      []string          `json:"causes"`
	Solutions   []Solution        `json:"solutions"`
	Actions     []Action          `json:"actions"`
	References  []Reference       `json:"references"`
	Context     map[string]string `json:"context"`
	Timestamp   time.Time         `json:"timestamp"`
	ID          string            `json:"id"`
	Tags        []string          `json:"tags"`
}

// MessageSeverity represents the severity level of a message
type MessageSeverity int

const (
	SeverityInfo MessageSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// MessageCategory represents the category of a message
type MessageCategory int

const (
	CategoryNetwork MessageCategory = iota
	CategoryFilesystem
	CategoryDatabase
	CategoryConfiguration
	CategoryMemory
	CategoryPermissions
	CategoryDependency
	CategorySecurity
	CategoryPerformance
	CategoryGeneral
)

// Solution represents a suggested solution
type Solution struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
	Difficulty  string `json:"difficulty"` // easy, medium, hard
	Impact      string `json:"impact"`     // low, medium, high
	Risk        string `json:"risk"`       // low, medium, high
	Platform    string `json:"platform"`   // all, windows, linux, macos
}

// Step represents a step in a solution
type Step struct {
	Number      int    `json:"number"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Notes       string `json:"notes,omitempty"`
	Optional    bool   `json:"optional"`
}

// Action represents an actionable item
type Action struct {
	Type        string `json:"type"`        // button, command, link, etc.
	Label       string `json:"label"`
	Command     string `json:"command,omitempty"`
	URL         string `json:"url,omitempty"`
	Dangerous   bool   `json:"dangerous"`
	Description string `json:"description"`
}

// Reference represents a reference link or document
type Reference struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"` // documentation, tutorial, forum, etc.
}

// UserMessageGenerator generates user-friendly messages from errors
type UserMessageGenerator struct {
	patterns []MessagePattern
	context  map[string]string
}

// MessagePattern represents a pattern for matching and generating messages
type MessagePattern struct {
	Name        string
	Regex       *regexp.Regexp
	Category    MessageCategory
	Severity    MessageSeverity
	Generator   func(matches []string, context map[string]string) UserMessage
}

// NewUserMessageGenerator creates a new user message generator
func NewUserMessageGenerator() *UserMessageGenerator {
	umg := &UserMessageGenerator{
		patterns: make([]MessagePattern, 0),
		context:  make(map[string]string),
	}
	
	// Register default patterns
	umg.registerDefaultPatterns()
	
	return umg
}

// registerDefaultPatterns registers built-in error patterns
func (umg *UserMessageGenerator) registerDefaultPatterns() {
	// Network connection errors
	umg.RegisterPattern(MessagePattern{
		Name:     "connection_refused",
		Regex:    regexp.MustCompile(`connection refused|connection reset|network is unreachable`),
		Category: CategoryNetwork,
		Severity: SeverityError,
		Generator: func(matches []string, context map[string]string) UserMessage {
			return UserMessage{
				Title:       "Network Connection Failed",
				Description: "Unable to establish a network connection to the remote service.",
				Severity:    SeverityError,
				Category:    CategoryNetwork,
				Causes: []string{
					"The remote server is not running",
					"Network connectivity issues",
					"Firewall blocking the connection",
					"Incorrect server address or port",
					"DNS resolution problems",
				},
				Solutions: []Solution{
					{
						Title:       "Check Server Status",
						Description: "Verify that the remote server is running and accessible",
						Difficulty:  "easy",
						Impact:      "high",
						Risk:        "low",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Verify the server address and port are correct"},
							{Number: 2, Description: "Check if the server is running", Command: "ping <server-address>"},
							{Number: 3, Description: "Test connectivity with telnet", Command: "telnet <server-address> <port>"},
						},
					},
					{
						Title:       "Check Network Configuration",
						Description: "Verify your network settings and connectivity",
						Difficulty:  "medium",
						Impact:      "medium",
						Risk:        "low",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Check your internet connection"},
							{Number: 2, Description: "Verify DNS resolution", Command: "nslookup <server-address>"},
							{Number: 3, Description: "Check firewall settings"},
							{Number: 4, Description: "Try connecting from a different network", Optional: true},
						},
					},
				},
				Actions: []Action{
					{Type: "button", Label: "Retry Connection", Description: "Attempt to reconnect"},
					{Type: "button", Label: "Check Network", Description: "Run network diagnostics"},
				},
				References: []Reference{
					{Title: "Network Troubleshooting Guide", URL: "https://docs.gocodenow.dev/troubleshooting/network", Type: "documentation"},
				},
				Tags: []string{"network", "connection", "server"},
			}
		},
	})

	// File permission errors
	umg.RegisterPattern(MessagePattern{
		Name:     "permission_denied",
		Regex:    regexp.MustCompile(`permission denied|access denied|operation not permitted`),
		Category: CategoryPermissions,
		Severity: SeverityError,
		Generator: func(matches []string, context map[string]string) UserMessage {
			filePath := context["file_path"]
			if filePath == "" {
				filePath = "the requested file or directory"
			}
			
			return UserMessage{
				Title:       "Permission Denied",
				Description: fmt.Sprintf("Insufficient permissions to access %s.", filePath),
				Severity:    SeverityError,
				Category:    CategoryPermissions,
				Causes: []string{
					"File or directory has restrictive permissions",
					"Current user lacks necessary privileges",
					"File is owned by another user or system",
					"File system is read-only",
				},
				Solutions: []Solution{
					{
						Title:       "Fix File Permissions",
						Description: "Adjust file permissions to allow access",
						Difficulty:  "medium",
						Impact:      "high",
						Risk:        "medium",
						Platform:    "linux,macos",
						Steps: []Step{
							{Number: 1, Description: "Check current permissions", Command: "ls -la " + filePath},
							{Number: 2, Description: "Make file readable/writable", Command: "chmod u+rw " + filePath},
							{Number: 3, Description: "If it's a directory, add execute permission", Command: "chmod u+x " + filePath, Optional: true},
						},
					},
					{
						Title:       "Run with Elevated Privileges",
						Description: "Use administrator privileges to access the file",
						Difficulty:  "easy",
						Impact:      "high",
						Risk:        "high",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Run the application as administrator/root"},
							{Number: 2, Description: "On Linux/macOS, use sudo", Command: "sudo gocodenow"},
							{Number: 3, Description: "On Windows, run as administrator", Notes: "Right-click and select 'Run as administrator'"},
						},
					},
				},
				Actions: []Action{
					{Type: "command", Label: "Check Permissions", Command: "ls -la " + filePath, Description: "View current file permissions"},
					{Type: "button", Label: "Retry with Sudo", Command: "sudo", Description: "Retry operation with elevated privileges", Dangerous: true},
				},
				References: []Reference{
					{Title: "File Permissions Guide", URL: "https://docs.gocodenow.dev/troubleshooting/permissions", Type: "documentation"},
				},
				Tags: []string{"permissions", "access", "file"},
			}
		},
	})

	// Database errors
	umg.RegisterPattern(MessagePattern{
		Name:     "database_locked",
		Regex:    regexp.MustCompile(`database is locked|database lock|sqlite.*busy`),
		Category: CategoryDatabase,
		Severity: SeverityWarning,
		Generator: func(matches []string, context map[string]string) UserMessage {
			return UserMessage{
				Title:       "Database Locked",
				Description: "The database is currently locked by another process.",
				Severity:    SeverityWarning,
				Category:    CategoryDatabase,
				Causes: []string{
					"Another instance of the application is running",
					"Previous application crash left database locked",
					"Database file is being accessed by another program",
					"File system issues preventing proper locking",
				},
				Solutions: []Solution{
					{
						Title:       "Close Other Instances",
						Description: "Ensure only one instance of the application is running",
						Difficulty:  "easy",
						Impact:      "high",
						Risk:        "low",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Close all other instances of gocodenow"},
							{Number: 2, Description: "Check for background processes", Command: "ps aux | grep gocodenow"},
							{Number: 3, Description: "Kill any remaining processes", Command: "killall gocodenow", Optional: true},
						},
					},
					{
						Title:       "Remove Lock File",
						Description: "Manually remove database lock files",
						Difficulty:  "medium",
						Impact:      "high",
						Risk:        "medium",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Find database directory"},
							{Number: 2, Description: "Look for .db-wal or .db-shm files"},
							{Number: 3, Description: "Remove lock files", Command: "rm *.db-wal *.db-shm"},
							{Number: 4, Description: "Restart the application"},
						},
					},
				},
				Actions: []Action{
					{Type: "button", Label: "Retry Operation", Description: "Try the database operation again"},
					{Type: "button", Label: "Check Processes", Description: "Show running gocodenow processes"},
				},
				Tags: []string{"database", "lock", "sqlite"},
			}
		},
	})

	// Configuration errors
	umg.RegisterPattern(MessagePattern{
		Name:     "config_parse_error",
		Regex:    regexp.MustCompile(`yaml.*error|json.*error|config.*parse|unmarshal.*error`),
		Category: CategoryConfiguration,
		Severity: SeverityError,
		Generator: func(matches []string, context map[string]string) UserMessage {
			configFile := context["config_file"]
			if configFile == "" {
				configFile = "configuration file"
			}
			
			return UserMessage{
				Title:       "Configuration Parse Error",
				Description: fmt.Sprintf("Failed to parse the %s.", configFile),
				Severity:    SeverityError,
				Category:    CategoryConfiguration,
				Causes: []string{
					"Invalid YAML or JSON syntax",
					"Missing required configuration fields",
					"Incorrect data types in configuration",
					"Corrupted configuration file",
				},
				Solutions: []Solution{
					{
						Title:       "Validate Configuration Syntax",
						Description: "Check and fix configuration file syntax",
						Difficulty:  "medium",
						Impact:      "high",
						Risk:        "low",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Open the configuration file in a text editor"},
							{Number: 2, Description: "Check for syntax errors (missing quotes, brackets, etc.)"},
							{Number: 3, Description: "Validate YAML syntax online", Notes: "Use a YAML validator tool"},
							{Number: 4, Description: "Compare with example configuration"},
						},
					},
					{
						Title:       "Reset to Default Configuration",
						Description: "Replace current configuration with default settings",
						Difficulty:  "easy",
						Impact:      "medium",
						Risk:        "medium",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Backup current configuration", Command: "cp " + configFile + " " + configFile + ".backup"},
							{Number: 2, Description: "Remove current configuration", Command: "rm " + configFile},
							{Number: 3, Description: "Restart application to generate default config"},
							{Number: 4, Description: "Reconfigure settings as needed"},
						},
					},
				},
				Actions: []Action{
					{Type: "button", Label: "Open Config File", Description: "Open configuration file in editor"},
					{Type: "button", Label: "Reset to Default", Description: "Reset to default configuration", Dangerous: true},
					{Type: "link", Label: "Configuration Guide", URL: "https://docs.gocodenow.dev/configuration", Description: "View configuration documentation"},
				},
				References: []Reference{
					{Title: "Configuration Documentation", URL: "https://docs.gocodenow.dev/configuration", Type: "documentation"},
					{Title: "YAML Syntax Guide", URL: "https://yaml.org/spec/", Type: "documentation"},
				},
				Tags: []string{"configuration", "yaml", "parse", "syntax"},
			}
		},
	})

	// Memory errors
	umg.RegisterPattern(MessagePattern{
		Name:     "out_of_memory",
		Regex:    regexp.MustCompile(`out of memory|memory allocation|cannot allocate`),
		Category: CategoryMemory,
		Severity: SeverityCritical,
		Generator: func(matches []string, context map[string]string) UserMessage {
			return UserMessage{
				Title:       "Out of Memory",
				Description: "The application has run out of available memory.",
				Severity:    SeverityCritical,
				Category:    CategoryMemory,
				Causes: []string{
					"Insufficient system memory",
					"Memory leak in the application",
					"Large dataset processing",
					"Memory limits set too low",
				},
				Solutions: []Solution{
					{
						Title:       "Increase Memory Limits",
						Description: "Adjust memory settings in configuration",
						Difficulty:  "easy",
						Impact:      "high",
						Risk:        "low",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Open configuration file"},
							{Number: 2, Description: "Increase max_memory_mb setting"},
							{Number: 3, Description: "Reduce cache_size if necessary"},
							{Number: 4, Description: "Restart the application"},
						},
					},
					{
						Title:       "Clear System Memory",
						Description: "Free up system memory",
						Difficulty:  "easy",
						Impact:      "medium",
						Risk:        "low",
						Platform:    "all",
						Steps: []Step{
							{Number: 1, Description: "Close unnecessary applications"},
							{Number: 2, Description: "Clear browser cache and tabs"},
							{Number: 3, Description: "Restart the system if needed", Optional: true},
						},
					},
				},
				Actions: []Action{
					{Type: "button", Label: "View Memory Usage", Description: "Show current memory statistics"},
					{Type: "button", Label: "Restart Application", Description: "Restart to free memory", Dangerous: true},
				},
				Tags: []string{"memory", "allocation", "system"},
			}
		},
	})
}

// RegisterPattern registers a new message pattern
func (umg *UserMessageGenerator) RegisterPattern(pattern MessagePattern) {
	umg.patterns = append(umg.patterns, pattern)
}

// SetContext sets context information for message generation
func (umg *UserMessageGenerator) SetContext(key, value string) {
	umg.context[key] = value
}

// GenerateMessage generates a user-friendly message from an error
func (umg *UserMessageGenerator) GenerateMessage(err error, component, operation string) UserMessage {
	errorText := err.Error()
	
	// Try to match against registered patterns
	for _, pattern := range umg.patterns {
		if matches := pattern.Regex.FindStringSubmatch(errorText); len(matches) > 0 {
			// Add component and operation to context
			context := make(map[string]string)
			for k, v := range umg.context {
				context[k] = v
			}
			context["component"] = component
			context["operation"] = operation
			context["error"] = errorText
			
			message := pattern.Generator(matches, context)
			message.Timestamp = time.Now()
			message.ID = fmt.Sprintf("%s_%d", pattern.Name, time.Now().Unix())
			
			return message
		}
	}
	
	// No pattern matched, generate generic message
	return umg.generateGenericMessage(err, component, operation)
}

// generateGenericMessage generates a generic user message
func (umg *UserMessageGenerator) generateGenericMessage(err error, component, operation string) UserMessage {
	return UserMessage{
		Title:       "An Error Occurred",
		Description: fmt.Sprintf("An error occurred in %s while performing %s.", component, operation),
		Severity:    SeverityError,
		Category:    CategoryGeneral,
		Causes: []string{
			"Unexpected system condition",
			"Software bug or edge case",
			"Invalid input or state",
		},
		Solutions: []Solution{
			{
				Title:       "Retry the Operation",
				Description: "Try performing the operation again",
				Difficulty:  "easy",
				Impact:      "medium",
				Risk:        "low",
				Platform:    "all",
				Steps: []Step{
					{Number: 1, Description: "Wait a moment and try again"},
					{Number: 2, Description: "Check if the issue persists"},
					{Number: 3, Description: "Restart the application if needed", Optional: true},
				},
			},
			{
				Title:       "Check Application Logs",
				Description: "Review logs for more detailed information",
				Difficulty:  "medium",
				Impact:      "low",
				Risk:        "low",
				Platform:    "all",
				Steps: []Step{
					{Number: 1, Description: "Open the application logs"},
					{Number: 2, Description: "Look for related error messages"},
					{Number: 3, Description: "Note any patterns or recurring issues"},
				},
			},
		},
		Actions: []Action{
			{Type: "button", Label: "Retry", Description: "Retry the failed operation"},
			{Type: "button", Label: "Report Issue", Description: "Report this issue to support"},
		},
		References: []Reference{
			{Title: "Troubleshooting Guide", URL: "https://docs.gocodenow.dev/troubleshooting", Type: "documentation"},
			{Title: "Support Forum", URL: "https://github.com/gocodenow/gocodenow/discussions", Type: "forum"},
		},
		Context: map[string]string{
			"component": component,
			"operation": operation,
			"error":     err.Error(),
		},
		Timestamp: time.Now(),
		ID:        fmt.Sprintf("generic_error_%d", time.Now().Unix()),
		Tags:      []string{"error", "generic", component},
	}
}

// GetPlatformSpecificSolutions filters solutions for the current platform
func (um *UserMessage) GetPlatformSpecificSolutions() []Solution {
	currentPlatform := runtime.GOOS
	var filtered []Solution
	
	for _, solution := range um.Solutions {
		if solution.Platform == "all" || 
		   solution.Platform == currentPlatform || 
		   strings.Contains(solution.Platform, currentPlatform) {
			filtered = append(filtered, solution)
		}
	}
	
	return filtered
}

// FormatForDisplay formats the message for display in the UI
func (um *UserMessage) FormatForDisplay() string {
	var builder strings.Builder
	
	// Title and severity
	severityStr := um.severityString()
	builder.WriteString(fmt.Sprintf("[%s] %s\n\n", severityStr, um.Title))
	
	// Description
	builder.WriteString(fmt.Sprintf("%s\n\n", um.Description))
	
	// Possible causes
	if len(um.Causes) > 0 {
		builder.WriteString("Possible causes:\n")
		for _, cause := range um.Causes {
			builder.WriteString(fmt.Sprintf("• %s\n", cause))
		}
		builder.WriteString("\n")
	}
	
	// Solutions
	if len(um.Solutions) > 0 {
		builder.WriteString("Suggested solutions:\n\n")
		solutions := um.GetPlatformSpecificSolutions()
		for i, solution := range solutions {
			builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, solution.Title))
			builder.WriteString(fmt.Sprintf("   %s\n", solution.Description))
			builder.WriteString(fmt.Sprintf("   Difficulty: %s | Impact: %s | Risk: %s\n\n", 
				solution.Difficulty, solution.Impact, solution.Risk))
		}
	}
	
	return builder.String()
}

// severityString returns the string representation of message severity
func (um *UserMessage) severityString() string {
	switch um.Severity {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// categoryString returns the string representation of message category
func (um *UserMessage) categoryString() string {
	switch um.Category {
	case CategoryNetwork:
		return "Network"
	case CategoryFilesystem:
		return "Filesystem"
	case CategoryDatabase:
		return "Database"
	case CategoryConfiguration:
		return "Configuration"
	case CategoryMemory:
		return "Memory"
	case CategoryPermissions:
		return "Permissions"
	case CategoryDependency:
		return "Dependency"
	case CategorySecurity:
		return "Security"
	case CategoryPerformance:
		return "Performance"
	case CategoryGeneral:
		return "General"
	default:
		return "Unknown"
	}
}