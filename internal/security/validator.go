package security

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidationRule defines a validation rule type
type ValidationRule string

const (
	RuleRequired     ValidationRule = "required"
	RuleMinLength    ValidationRule = "min_length"
	RuleMaxLength    ValidationRule = "max_length"
	RuleRegex        ValidationRule = "regex"
	RuleAlphaNumeric ValidationRule = "alphanumeric"
	RuleNumeric      ValidationRule = "numeric"
	RuleEmail        ValidationRule = "email"
	RuleURL          ValidationRule = "url"
	RuleFilePath     ValidationRule = "filepath"
	RuleNoScripts    ValidationRule = "no_scripts"
	RuleNoSQLi       ValidationRule = "no_sqli"
	RuleNoXSS        ValidationRule = "no_xss"
	RuleNoPathTraversal ValidationRule = "no_path_traversal"
	RuleCommand      ValidationRule = "command"
)

// ValidationConfig configures field validation
type ValidationConfig struct {
	Rules      []ValidationRuleConfig `json:"rules"`
	Sanitize   bool                   `json:"sanitize"`
	StripHTML  bool                   `json:"strip_html"`
	StripSQL   bool                   `json:"strip_sql"`
	Normalize  bool                   `json:"normalize"`
	TrimSpaces bool                   `json:"trim_spaces"`
	MaxSize    int                    `json:"max_size"`
}

// ValidationRuleConfig defines a specific validation rule
type ValidationRuleConfig struct {
	Rule      ValidationRule `json:"rule"`
	Parameter string         `json:"parameter,omitempty"`
	Message   string         `json:"message,omitempty"`
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid     bool                       `json:"valid"`
	Errors    []InputValidationError          `json:"errors,omitempty"`
	Warnings  []ValidationWarning        `json:"warnings,omitempty"`
	Sanitized map[string]interface{}     `json:"sanitized,omitempty"`
	Metadata  map[string]interface{}     `json:"metadata,omitempty"`
}

// InputValidationError represents a validation error
type InputValidationError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// InputValidator provides comprehensive input validation and sanitization
type InputValidator struct {
	defaultConfig  ValidationConfig
	fieldConfigs   map[string]ValidationConfig
	customPatterns map[string]*regexp.Regexp
	dangerousCommands []string
	sqlPatterns    []*regexp.Regexp
	xssPatterns    []*regexp.Regexp
	pathPatterns   []*regexp.Regexp
}

// NewInputValidator creates a new input validator with secure defaults
func NewInputValidator() *InputValidator {
	validator := &InputValidator{
		defaultConfig: ValidationConfig{
			Rules: []ValidationRuleConfig{
				{Rule: RuleNoScripts, Message: "Scripts are not allowed"},
				{Rule: RuleNoSQLi, Message: "SQL injection patterns detected"},
				{Rule: RuleNoXSS, Message: "XSS patterns detected"},
				{Rule: RuleNoPathTraversal, Message: "Path traversal patterns detected"},
			},
			Sanitize:   true,
			StripHTML:  true,
			StripSQL:   true,
			Normalize:  true,
			TrimSpaces: true,
			MaxSize:    10485760, // 10MB
		},
		fieldConfigs:   make(map[string]ValidationConfig),
		customPatterns: make(map[string]*regexp.Regexp),
		dangerousCommands: []string{
			"rm", "rmdir", "del", "format", "fdisk", "mkfs",
			"dd", "mount", "umount", "sudo", "su", "chmod",
			"chown", "passwd", "useradd", "userdel", "crontab",
			"systemctl", "service", "reboot", "shutdown", "halt",
			"kill", "killall", "pkill", "wget", "curl", "nc",
			"netcat", "telnet", "ssh", "scp", "rsync", "eval",
			"exec", "sh", "bash", "zsh", "fish", "powershell",
			"cmd", "python", "perl", "ruby", "node", "java",
		},
	}
	
	validator.initializeSecurityPatterns()
	return validator
}

// initializeSecurityPatterns initializes regex patterns for security validation
func (v *InputValidator) initializeSecurityPatterns() {
	// SQL injection patterns
	v.sqlPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\b(ALTER|CREATE|DELETE|DROP|EXEC(UTE)?|INSERT|MERGE|SELECT|UNION|UPDATE)\b)`),
		regexp.MustCompile(`(?i)(\b(AND|OR)\b.*?(\b(SELECT|INSERT|UPDATE|DELETE|UNION|ALTER|CREATE|DROP)\b))`),
		regexp.MustCompile(`(?i)(\b(UNION)\b.*?\b(SELECT)\b)`),
		regexp.MustCompile(`(?i)(\b(SELECT)\b.*?\b(FROM)\b)`),
		regexp.MustCompile(`(?i)(--|#|\*\/|\/\*)`),
		regexp.MustCompile(`(?i)(\bxp_cmdshell\b|\bsp_password\b|\bsp_configure\b)`),
		regexp.MustCompile(`(?i)(\b1=1\b|\b1=0\b|\b'='\b|\b"="\b)`),
		regexp.MustCompile(`(?i)((\s|%20)*(UNION|SELECT|INSERT|UPDATE|DELETE)(\s|%20)*)`),
	}

	// XSS patterns
	v.xssPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<\s*script[^>]*>.*?<\s*/\s*script\s*>`),
		regexp.MustCompile(`(?i)on\w+\s*=\s*['"]*[^'"]*['"]*`),
		regexp.MustCompile(`(?i)javascript\s*:`),
		regexp.MustCompile(`(?i)vbscript\s*:`),
		regexp.MustCompile(`(?i)expression\s*\(`),
		regexp.MustCompile(`(?i)<\s*iframe[^>]*>.*?<\s*/\s*iframe\s*>`),
		regexp.MustCompile(`(?i)<\s*object[^>]*>.*?<\s*/\s*object\s*>`),
		regexp.MustCompile(`(?i)<\s*embed[^>]*>`),
		regexp.MustCompile(`(?i)<\s*meta[^>]*http-equiv[^>]*>`),
		regexp.MustCompile(`(?i)eval\s*\(`),
		regexp.MustCompile(`(?i)setTimeout\s*\(`),
		regexp.MustCompile(`(?i)setInterval\s*\(`),
	}

	// Path traversal patterns
	v.pathPatterns = []*regexp.Regexp{
		regexp.MustCompile(`\.\.(/|\\)`),
		regexp.MustCompile(`%2e%2e(/|\\|%2f|%5c)`),
		regexp.MustCompile(`\.\.%2f`),
		regexp.MustCompile(`%2e%2e%2f`),
		regexp.MustCompile(`\.\.%5c`),
		regexp.MustCompile(`%2e%2e%5c`),
		regexp.MustCompile(`\.\./`),
		regexp.MustCompile(`\.\.\\`),
	}
}

// SetFieldConfig sets validation configuration for a specific field
func (v *InputValidator) SetFieldConfig(fieldName string, config ValidationConfig) {
	v.fieldConfigs[fieldName] = config
}

// AddCustomPattern adds a custom regex pattern for validation
func (v *InputValidator) AddCustomPattern(name string, pattern string) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	v.customPatterns[name] = regex
	return nil
}

// ValidateInput validates and sanitizes input data
func (v *InputValidator) ValidateInput(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		Valid:     true,
		Errors:    []InputValidationError{},
		Warnings:  []ValidationWarning{},
		Sanitized: make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
	}

	for fieldName, value := range data {
		config := v.getFieldConfig(fieldName)
		fieldResult := v.validateField(fieldName, value, config)
		
		if !fieldResult.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, fieldResult.Errors...)
		}
		
		result.Warnings = append(result.Warnings, fieldResult.Warnings...)
		
		if fieldResult.Sanitized != nil {
			for k, v := range fieldResult.Sanitized {
				result.Sanitized[k] = v
			}
		}
	}

	// Add metadata
	result.Metadata["validation_time"] = fmt.Sprintf("%d fields processed", len(data))
	result.Metadata["sanitization_applied"] = len(result.Sanitized) > 0

	return result
}

// validateField validates a single field
func (v *InputValidator) validateField(fieldName string, value interface{}, config ValidationConfig) *ValidationResult {
	result := &ValidationResult{
		Valid:     true,
		Errors:    []InputValidationError{},
		Warnings:  []ValidationWarning{},
		Sanitized: make(map[string]interface{}),
	}

	// Convert value to string for validation
	strValue := v.convertToString(value)
	
	// Check size limits
	if config.MaxSize > 0 && len(strValue) > config.MaxSize {
		result.Valid = false
		result.Errors = append(result.Errors, InputValidationError{
			Field:   fieldName,
			Rule:    "max_size",
			Message: fmt.Sprintf("Value exceeds maximum size of %d bytes", config.MaxSize),
			Value:   v.truncateForDisplay(strValue, 100),
		})
		return result
	}

	// Apply sanitization if enabled
	if config.Sanitize {
		sanitized := v.sanitizeValue(strValue, config)
		if sanitized != strValue {
			result.Sanitized[fieldName] = sanitized
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   fieldName,
				Message: "Value was sanitized",
				Value:   v.truncateForDisplay(strValue, 50),
			})
		}
		strValue = sanitized
	}

	// Apply validation rules
	for _, ruleConfig := range config.Rules {
		if err := v.validateRule(fieldName, strValue, ruleConfig); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, *err)
		}
	}

	return result
}

// validateRule validates a specific rule against a value
func (v *InputValidator) validateRule(fieldName, value string, ruleConfig ValidationRuleConfig) *InputValidationError {
	switch ruleConfig.Rule {
	case RuleRequired:
		if strings.TrimSpace(value) == "" {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Field is required"),
				Value:   value,
			}
		}

	case RuleMinLength:
		minLen, _ := strconv.Atoi(ruleConfig.Parameter)
		if len(value) < minLen {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, fmt.Sprintf("Minimum length is %d characters", minLen)),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleMaxLength:
		maxLen, _ := strconv.Atoi(ruleConfig.Parameter)
		if len(value) > maxLen {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, fmt.Sprintf("Maximum length is %d characters", maxLen)),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleRegex:
		if ruleConfig.Parameter != "" {
			if regex, exists := v.customPatterns[ruleConfig.Parameter]; exists {
				if !regex.MatchString(value) {
					return &InputValidationError{
						Field:   fieldName,
						Rule:    string(ruleConfig.Rule),
						Message: v.getErrorMessage(ruleConfig, "Value does not match required pattern"),
						Value:   v.truncateForDisplay(value, 50),
					}
				}
			} else {
				// Try to compile inline regex
				if regex, err := regexp.Compile(ruleConfig.Parameter); err == nil {
					if !regex.MatchString(value) {
						return &InputValidationError{
							Field:   fieldName,
							Rule:    string(ruleConfig.Rule),
							Message: v.getErrorMessage(ruleConfig, "Value does not match required pattern"),
							Value:   v.truncateForDisplay(value, 50),
						}
					}
				}
			}
		}

	case RuleAlphaNumeric:
		if !v.isAlphaNumeric(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Value must contain only alphanumeric characters"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleNumeric:
		if !v.isNumeric(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Value must be numeric"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleEmail:
		if !v.isValidEmail(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Invalid email format"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleURL:
		if !v.isValidURL(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Invalid URL format"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleFilePath:
		if !v.isValidFilePath(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Invalid file path"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleNoScripts:
		if v.containsScripts(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Script content detected"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleNoSQLi:
		if v.containsSQLInjection(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "SQL injection pattern detected"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleNoXSS:
		if v.containsXSS(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "XSS pattern detected"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleNoPathTraversal:
		if v.containsPathTraversal(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Path traversal pattern detected"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}

	case RuleCommand:
		if v.isDangerousCommand(value) {
			return &InputValidationError{
				Field:   fieldName,
				Rule:    string(ruleConfig.Rule),
				Message: v.getErrorMessage(ruleConfig, "Dangerous command detected"),
				Value:   v.truncateForDisplay(value, 50),
			}
		}
	}

	return nil
}

// sanitizeValue sanitizes a value based on configuration
func (v *InputValidator) sanitizeValue(value string, config ValidationConfig) string {
	sanitized := value

	if config.TrimSpaces {
		sanitized = strings.TrimSpace(sanitized)
	}

	if config.Normalize {
		sanitized = v.normalizeString(sanitized)
	}

	if config.StripHTML {
		sanitized = v.stripHTML(sanitized)
	}

	if config.StripSQL {
		sanitized = v.stripSQLPatterns(sanitized)
	}

	return sanitized
}

// Helper methods for validation
func (v *InputValidator) convertToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}

func (v *InputValidator) getFieldConfig(fieldName string) ValidationConfig {
	if config, exists := v.fieldConfigs[fieldName]; exists {
		return config
	}
	return v.defaultConfig
}

func (v *InputValidator) getErrorMessage(ruleConfig ValidationRuleConfig, defaultMsg string) string {
	if ruleConfig.Message != "" {
		return ruleConfig.Message
	}
	return defaultMsg
}

func (v *InputValidator) truncateForDisplay(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}

func (v *InputValidator) isAlphaNumeric(value string) bool {
	for _, r := range value {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func (v *InputValidator) isNumeric(value string) bool {
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func (v *InputValidator) isValidEmail(value string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(value)
}

func (v *InputValidator) isValidURL(value string) bool {
	_, err := url.ParseRequestURI(value)
	return err == nil
}

func (v *InputValidator) isValidFilePath(value string) bool {
	cleaned := filepath.Clean(value)
	return cleaned == value && !strings.Contains(value, "..")
}

func (v *InputValidator) containsScripts(value string) bool {
	lower := strings.ToLower(value)
	scriptPatterns := []string{
		"<script", "</script>", "javascript:", "vbscript:",
		"onload=", "onclick=", "onerror=", "onmouseover=",
		"eval(", "setTimeout(", "setInterval(",
	}
	
	for _, pattern := range scriptPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	
	return false
}

func (v *InputValidator) containsSQLInjection(value string) bool {
	for _, pattern := range v.sqlPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func (v *InputValidator) containsXSS(value string) bool {
	for _, pattern := range v.xssPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func (v *InputValidator) containsPathTraversal(value string) bool {
	for _, pattern := range v.pathPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func (v *InputValidator) isDangerousCommand(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	
	// Check for dangerous commands at the start of the value
	for _, cmd := range v.dangerousCommands {
		if strings.HasPrefix(lower, cmd+" ") || lower == cmd {
			return true
		}
	}
	
	// Check for command injection patterns
	dangerousPatterns := []string{
		"|", "&&", "||", ";", "`", "$(", "${",
	}
	
	for _, pattern := range dangerousPatterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	
	return false
}

func (v *InputValidator) normalizeString(value string) string {
	// Normalize unicode characters
	normalized := strings.Map(func(r rune) rune {
		if !utf8.ValidRune(r) {
			return -1
		}
		return r
	}, value)
	
	// Remove excessive whitespace
	whitespaceRegex := regexp.MustCompile(`\s+`)
	normalized = whitespaceRegex.ReplaceAllString(normalized, " ")
	
	return strings.TrimSpace(normalized)
}

func (v *InputValidator) stripHTML(value string) string {
	// Remove HTML tags
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	stripped := htmlRegex.ReplaceAllString(value, "")
	
	// Decode HTML entities
	stripped = html.UnescapeString(stripped)
	
	return stripped
}

func (v *InputValidator) stripSQLPatterns(value string) string {
	stripped := value
	
	// Remove SQL comments
	sqlCommentRegex := regexp.MustCompile(`(?i)(--|#|\/\*.*?\*\/)`)
	stripped = sqlCommentRegex.ReplaceAllString(stripped, "")
	
	// Remove common SQL injection markers
	injectionMarkers := []string{
		"1=1", "1=0", "'='", `"="`, " OR ", " AND ",
	}
	
	for _, marker := range injectionMarkers {
		stripped = strings.ReplaceAll(strings.ToUpper(stripped), marker, "")
	}
	
	return stripped
}

// ValidateJSON validates JSON input with schema validation
func (v *InputValidator) ValidateJSON(jsonData []byte, schema map[string]ValidationConfig) *ValidationResult {
	var data map[string]interface{}
	
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []InputValidationError{
				{
					Field:   "json",
					Rule:    "format",
					Message: "Invalid JSON format: " + err.Error(),
				},
			},
		}
	}
	
	// Apply schema validation
	originalConfigs := v.fieldConfigs
	v.fieldConfigs = schema
	
	result := v.ValidateInput(data)
	
	// Restore original configs
	v.fieldConfigs = originalConfigs
	
	return result
}

// GetSecurityReport generates a security validation report
func (v *InputValidator) GetSecurityReport(data map[string]interface{}) map[string]interface{} {
	report := map[string]interface{}{
		"total_fields": len(data),
		"security_checks": map[string]int{
			"sql_injection":   0,
			"xss_attempts":    0,
			"path_traversal":  0,
			"script_injection": 0,
			"command_injection": 0,
		},
		"field_analysis": make(map[string]map[string]interface{}),
	}
	
	for fieldName, value := range data {
		strValue := v.convertToString(value)
		analysis := map[string]interface{}{
			"length": len(strValue),
			"threats": []string{},
		}
		
		if v.containsSQLInjection(strValue) {
			analysis["threats"] = append(analysis["threats"].([]string), "SQL Injection")
			report["security_checks"].(map[string]int)["sql_injection"]++
		}
		
		if v.containsXSS(strValue) {
			analysis["threats"] = append(analysis["threats"].([]string), "XSS")
			report["security_checks"].(map[string]int)["xss_attempts"]++
		}
		
		if v.containsPathTraversal(strValue) {
			analysis["threats"] = append(analysis["threats"].([]string), "Path Traversal")
			report["security_checks"].(map[string]int)["path_traversal"]++
		}
		
		if v.containsScripts(strValue) {
			analysis["threats"] = append(analysis["threats"].([]string), "Script Injection")
			report["security_checks"].(map[string]int)["script_injection"]++
		}
		
		if v.isDangerousCommand(strValue) {
			analysis["threats"] = append(analysis["threats"].([]string), "Command Injection")
			report["security_checks"].(map[string]int)["command_injection"]++
		}
		
		report["field_analysis"].(map[string]map[string]interface{})[fieldName] = analysis
	}
	
	return report
}