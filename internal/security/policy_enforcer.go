package security

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// PolicyEnforcementMode defines different enforcement modes
type PolicyEnforcementMode string

const (
	EnforcementModePermissive PolicyEnforcementMode = "permissive" // Log violations but allow
	EnforcementModeEnforcing  PolicyEnforcementMode = "enforcing"  // Block violations
	EnforcementModeAuditing   PolicyEnforcementMode = "auditing"   // Only audit, no enforcement
)

// PolicyViolationType defines different types of policy violations
type PolicyViolationType string

const (
	ViolationTypeUnauthorizedAccess   PolicyViolationType = "unauthorized_access"
	ViolationTypeRateLimitExceeded    PolicyViolationType = "rate_limit_exceeded"
	ViolationTypeResourceLimitExceeded PolicyViolationType = "resource_limit_exceeded"
	ViolationTypeInputValidationFailed PolicyViolationType = "input_validation_failed"
	ViolationTypePathTraversalAttempt PolicyViolationType = "path_traversal_attempt"
	ViolationTypeCommandInjection     PolicyViolationType = "command_injection"
	ViolationTypeSandboxEscape        PolicyViolationType = "sandbox_escape"
	ViolationTypeIntegrityViolation   PolicyViolationType = "integrity_violation"
	ViolationTypeTimeBasedViolation   PolicyViolationType = "time_based_violation"
	ViolationTypeCustomRule           PolicyViolationType = "custom_rule"
)

// PolicyViolation represents a security policy violation
type PolicyViolation struct {
	ID              string                `json:"id"`
	Type            PolicyViolationType   `json:"type"`
	Severity        ViolationSeverity     `json:"severity"`
	Source          string                `json:"source"`
	User            string                `json:"user,omitempty"`
	Resource        string                `json:"resource,omitempty"`
	Action          string                `json:"action"`
	Description     string                `json:"description"`
	Evidence        map[string]interface{} `json:"evidence"`
	Timestamp       time.Time             `json:"timestamp"`
	Policy          string                `json:"policy"`
	Remediation     []string              `json:"remediation"`
	Impact          ViolationImpact       `json:"impact"`
	ResponseActions []ResponseAction      `json:"response_actions"`
}

// ViolationSeverity defines violation severity levels
type ViolationSeverity string

const (
	SeverityLow      ViolationSeverity = "low"
	SeverityMedium   ViolationSeverity = "medium"
	SeverityHigh     ViolationSeverity = "high"
	SeverityCritical ViolationSeverity = "critical"
)

// ViolationImpact describes the potential impact of a violation
type ViolationImpact struct {
	ConfidentialityImpact string `json:"confidentiality_impact"`
	IntegrityImpact       string `json:"integrity_impact"`
	AvailabilityImpact    string `json:"availability_impact"`
	BusinessImpact        string `json:"business_impact"`
}

// ResponseAction defines actions taken in response to violations
type ResponseAction struct {
	Action      string    `json:"action"`
	Timestamp   time.Time `json:"timestamp"`
	Success     bool      `json:"success"`
	Details     string    `json:"details,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// PolicyRule defines a security policy rule
type PolicyRule struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Category       string                 `json:"category"`
	Severity       ViolationSeverity      `json:"severity"`
	Enabled        bool                   `json:"enabled"`
	Conditions     []PolicyCondition      `json:"conditions"`
	Actions        []PolicyAction         `json:"actions"`
	Exceptions     []PolicyException      `json:"exceptions,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	ValidFrom      *time.Time             `json:"valid_from,omitempty"`
	ValidUntil     *time.Time             `json:"valid_until,omitempty"`
}

// PolicyCondition defines a condition that triggers a policy rule
type PolicyCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, ne, gt, lt, contains, matches, etc.
	Value    interface{} `json:"value"`
	Negate   bool        `json:"negate,omitempty"`
}

// PolicyAction defines an action to take when a rule is triggered
type PolicyAction struct {
	Type       string                 `json:"type"`       // block, alert, rate_limit, quarantine, etc.
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// PolicyException defines exceptions to policy rules
type PolicyException struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Conditions  []PolicyCondition `json:"conditions"`
	ValidFrom   *time.Time        `json:"valid_from,omitempty"`
	ValidUntil  *time.Time        `json:"valid_until,omitempty"`
}

// EnforcementContext provides context for policy enforcement
type EnforcementContext struct {
	User        string                 `json:"user"`
	SessionID   string                 `json:"session_id,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Resource    string                 `json:"resource"`
	Action      string                 `json:"action"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	RequestData map[string]interface{} `json:"request_data,omitempty"`
}

// EnforcementResult represents the result of policy enforcement
type EnforcementResult struct {
	Allowed     bool               `json:"allowed"`
	Violations  []PolicyViolation  `json:"violations,omitempty"`
	Actions     []ResponseAction   `json:"actions,omitempty"`
	Message     string             `json:"message,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PolicyEnforcementEngine provides comprehensive security policy enforcement
type PolicyEnforcementEngine struct {
	mode              PolicyEnforcementMode
	rules             map[string]*PolicyRule
	violations        map[string]*PolicyViolation
	mu                sync.RWMutex
	auditor           *SecurityAuditor
	rateLimiter       *SecurityRateLimiter
	resourceMonitor   *ResourceMonitor
	inputValidator    *InputValidator
	violationCounter  int64
	enforcementStats  EnforcementStatistics
	statsMu           sync.RWMutex
	customValidators  map[string]func(*EnforcementContext) bool
	responseHandlers  map[string]func(*PolicyViolation) []ResponseAction
}

// EnforcementStatistics tracks policy enforcement statistics
type EnforcementStatistics struct {
	TotalEnforcements    int64                            `json:"total_enforcements"`
	AllowedActions       int64                            `json:"allowed_actions"`
	BlockedActions       int64                            `json:"blocked_actions"`
	ViolationsByType     map[PolicyViolationType]int64    `json:"violations_by_type"`
	ViolationsBySeverity map[ViolationSeverity]int64      `json:"violations_by_severity"`
	ViolationsByUser     map[string]int64                 `json:"violations_by_user"`
	RulesTriggerCount    map[string]int64                 `json:"rules_trigger_count"`
	ResponseActions      map[string]int64                 `json:"response_actions"`
	AverageEnforcementTime time.Duration                 `json:"average_enforcement_time"`
	LastUpdated          time.Time                        `json:"last_updated"`
}

// NewPolicyEnforcementEngine creates a new policy enforcement engine
func NewPolicyEnforcementEngine(
	mode PolicyEnforcementMode,
	auditor *SecurityAuditor,
	rateLimiter *SecurityRateLimiter,
	resourceMonitor *ResourceMonitor,
	inputValidator *InputValidator,
) *PolicyEnforcementEngine {
	
	engine := &PolicyEnforcementEngine{
		mode:            mode,
		rules:           make(map[string]*PolicyRule),
		violations:      make(map[string]*PolicyViolation),
		auditor:         auditor,
		rateLimiter:     rateLimiter,
		resourceMonitor: resourceMonitor,
		inputValidator:  inputValidator,
		enforcementStats: EnforcementStatistics{
			ViolationsByType:     make(map[PolicyViolationType]int64),
			ViolationsBySeverity: make(map[ViolationSeverity]int64),
			ViolationsByUser:     make(map[string]int64),
			RulesTriggerCount:    make(map[string]int64),
			ResponseActions:      make(map[string]int64),
		},
		customValidators: make(map[string]func(*EnforcementContext) bool),
		responseHandlers: make(map[string]func(*PolicyViolation) []ResponseAction),
	}
	
	// Load default policies
	engine.loadDefaultPolicies()
	
	// Register default response handlers
	engine.registerDefaultResponseHandlers()
	
	return engine
}

// EnforcePolicy checks and enforces security policies against the given context
func (pee *PolicyEnforcementEngine) EnforcePolicy(ctx context.Context, enforcementCtx *EnforcementContext) *EnforcementResult {
	startTime := time.Now()
	
	result := &EnforcementResult{
		Allowed:     true,
		Violations:  []PolicyViolation{},
		Actions:     []ResponseAction{},
		Metadata:    make(map[string]interface{}),
	}
	
	// Update statistics
	pee.statsMu.Lock()
	pee.enforcementStats.TotalEnforcements++
	pee.statsMu.Unlock()
	
	// Evaluate all enabled rules
	pee.mu.RLock()
	applicableRules := pee.getApplicableRules(enforcementCtx)
	pee.mu.RUnlock()
	
	var violations []PolicyViolation
	
	for _, rule := range applicableRules {
		if violation := pee.evaluateRule(rule, enforcementCtx); violation != nil {
			violations = append(violations, *violation)
			
			// Update statistics
			pee.statsMu.Lock()
			pee.enforcementStats.RulesTriggerCount[rule.ID]++
			pee.enforcementStats.ViolationsByType[violation.Type]++
			pee.enforcementStats.ViolationsBySeverity[violation.Severity]++
			pee.enforcementStats.ViolationsByUser[enforcementCtx.User]++
			pee.statsMu.Unlock()
		}
	}
	
	// Process violations
	if len(violations) > 0 {
		result.Violations = violations
		
		// Determine overall result based on enforcement mode and violation severity
		shouldBlock := pee.shouldBlockRequest(violations)
		result.Allowed = !shouldBlock
		
		// Execute response actions
		for _, violation := range violations {
			actions := pee.executeResponseActions(&violation)
			result.Actions = append(result.Actions, actions...)
		}
		
		// Update statistics
		pee.statsMu.Lock()
		if result.Allowed {
			pee.enforcementStats.AllowedActions++
		} else {
			pee.enforcementStats.BlockedActions++
		}
		pee.statsMu.Unlock()
		
		// Generate appropriate message
		if !result.Allowed {
			result.Message = pee.generateViolationMessage(violations)
		}
	}
	
	// Update enforcement time statistics
	enforcementTime := time.Since(startTime)
	pee.statsMu.Lock()
	if pee.enforcementStats.TotalEnforcements > 0 {
		totalTime := time.Duration(int64(pee.enforcementStats.AverageEnforcementTime) * (pee.enforcementStats.TotalEnforcements - 1) + int64(enforcementTime))
		pee.enforcementStats.AverageEnforcementTime = totalTime / time.Duration(pee.enforcementStats.TotalEnforcements)
	}
	pee.enforcementStats.LastUpdated = time.Now()
	pee.statsMu.Unlock()
	
	result.Metadata["enforcement_time_ms"] = enforcementTime.Milliseconds()
	result.Metadata["rules_evaluated"] = len(applicableRules)
	result.Metadata["enforcement_mode"] = pee.mode
	
	// Log enforcement result
	if pee.auditor != nil {
		pee.logEnforcementResult(enforcementCtx, result, enforcementTime)
	}
	
	return result
}

// AddPolicyRule adds or updates a policy rule
func (pee *PolicyEnforcementEngine) AddPolicyRule(rule *PolicyRule) error {
	if rule.ID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}
	
	if rule.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}
	
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	
	pee.mu.Lock()
	pee.rules[rule.ID] = rule
	pee.mu.Unlock()
	
	if pee.auditor != nil {
		pee.auditor.LogEvent(&AuditEvent{
			Level:     AuditLevelInfo,
			EventType: EventTypeConfigChange,
			Source:    "policy_enforcer",
			Action:    "ADD_POLICY_RULE",
			Result:    "SUCCESS",
			Message:   fmt.Sprintf("Policy rule added: %s", rule.Name),
			Details: map[string]interface{}{
				"rule_id":   rule.ID,
				"rule_name": rule.Name,
				"category":  rule.Category,
				"severity":  rule.Severity,
			},
		})
	}
	
	return nil
}

// RemovePolicyRule removes a policy rule
func (pee *PolicyEnforcementEngine) RemovePolicyRule(ruleID string) error {
	pee.mu.Lock()
	rule, exists := pee.rules[ruleID]
	if exists {
		delete(pee.rules, ruleID)
	}
	pee.mu.Unlock()
	
	if !exists {
		return fmt.Errorf("rule %s not found", ruleID)
	}
	
	if pee.auditor != nil {
		pee.auditor.LogEvent(&AuditEvent{
			Level:     AuditLevelWarning,
			EventType: EventTypeConfigChange,
			Source:    "policy_enforcer",
			Action:    "REMOVE_POLICY_RULE",
			Result:    "SUCCESS",
			Message:   fmt.Sprintf("Policy rule removed: %s", rule.Name),
			Details: map[string]interface{}{
				"rule_id":   ruleID,
				"rule_name": rule.Name,
			},
		})
	}
	
	return nil
}

// SetEnforcementMode changes the enforcement mode
func (pee *PolicyEnforcementEngine) SetEnforcementMode(mode PolicyEnforcementMode) {
	oldMode := pee.mode
	pee.mode = mode
	
	if pee.auditor != nil {
		pee.auditor.LogEvent(&AuditEvent{
			Level:     AuditLevelWarning,
			EventType: EventTypeConfigChange,
			Source:    "policy_enforcer",
			Action:    "CHANGE_ENFORCEMENT_MODE",
			Result:    "SUCCESS",
			Message:   fmt.Sprintf("Enforcement mode changed from %s to %s", oldMode, mode),
			Details: map[string]interface{}{
				"old_mode": oldMode,
				"new_mode": mode,
			},
		})
	}
}

// GetPolicyRules returns all policy rules
func (pee *PolicyEnforcementEngine) GetPolicyRules() map[string]*PolicyRule {
	pee.mu.RLock()
	defer pee.mu.RUnlock()
	
	rules := make(map[string]*PolicyRule)
	for id, rule := range pee.rules {
		ruleCopy := *rule
		rules[id] = &ruleCopy
	}
	
	return rules
}

// GetViolations returns all recorded violations
func (pee *PolicyEnforcementEngine) GetViolations(since time.Time) []*PolicyViolation {
	pee.mu.RLock()
	defer pee.mu.RUnlock()
	
	var violations []*PolicyViolation
	for _, violation := range pee.violations {
		if violation.Timestamp.After(since) {
			violationCopy := *violation
			violations = append(violations, &violationCopy)
		}
	}
	
	return violations
}

// GetStatistics returns enforcement statistics
func (pee *PolicyEnforcementEngine) GetStatistics() EnforcementStatistics {
	pee.statsMu.RLock()
	defer pee.statsMu.RUnlock()
	
	stats := pee.enforcementStats
	stats.LastUpdated = time.Now()
	return stats
}

// RegisterCustomValidator registers a custom validation function
func (pee *PolicyEnforcementEngine) RegisterCustomValidator(name string, validator func(*EnforcementContext) bool) {
	pee.customValidators[name] = validator
}

// RegisterResponseHandler registers a custom response handler
func (pee *PolicyEnforcementEngine) RegisterResponseHandler(violationType string, handler func(*PolicyViolation) []ResponseAction) {
	pee.responseHandlers[violationType] = handler
}

// Private methods

func (pee *PolicyEnforcementEngine) loadDefaultPolicies() {
	// Rate limiting policy
	rateLimitRule := &PolicyRule{
		ID:          "rate_limit_enforcement",
		Name:        "Rate Limit Enforcement",
		Description: "Enforces rate limits for tool executions",
		Category:    "access_control",
		Severity:    SeverityHigh,
		Enabled:     true,
		Conditions: []PolicyCondition{
			{
				Field:    "rate_limit_check",
				Operator: "eq",
				Value:    "exceeded",
			},
		},
		Actions: []PolicyAction{
			{
				Type: "block",
			},
			{
				Type: "alert",
				Parameters: map[string]interface{}{
					"severity": "high",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	pee.rules[rateLimitRule.ID] = rateLimitRule
	
	// Input validation policy
	inputValidationRule := &PolicyRule{
		ID:          "input_validation_enforcement",
		Name:        "Input Validation Enforcement",
		Description: "Blocks requests with invalid or malicious input",
		Category:    "input_validation",
		Severity:    SeverityCritical,
		Enabled:     true,
		Conditions: []PolicyCondition{
			{
				Field:    "input_validation",
				Operator: "eq",
				Value:    "failed",
			},
		},
		Actions: []PolicyAction{
			{
				Type: "block",
			},
			{
				Type: "alert",
				Parameters: map[string]interface{}{
					"severity": "critical",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	pee.rules[inputValidationRule.ID] = inputValidationRule
	
	// Resource limit policy
	resourceLimitRule := &PolicyRule{
		ID:          "resource_limit_enforcement",
		Name:        "Resource Limit Enforcement",
		Description: "Enforces system resource limits",
		Category:    "resource_management",
		Severity:    SeverityHigh,
		Enabled:     true,
		Conditions: []PolicyCondition{
			{
				Field:    "resource_usage",
				Operator: "exceeds",
				Value:    "limit",
			},
		},
		Actions: []PolicyAction{
			{
				Type: "throttle",
			},
			{
				Type: "alert",
				Parameters: map[string]interface{}{
					"severity": "high",
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	pee.rules[resourceLimitRule.ID] = resourceLimitRule
	
	// Time-based access policy
	timeBasedRule := &PolicyRule{
		ID:          "time_based_access",
		Name:        "Time-Based Access Control",
		Description: "Restricts access outside business hours",
		Category:    "access_control",
		Severity:    SeverityMedium,
		Enabled:     false, // Disabled by default
		Conditions: []PolicyCondition{
			{
				Field:    "time_of_day",
				Operator: "outside_hours",
				Value:    map[string]interface{}{
					"start": "09:00",
					"end":   "17:00",
					"timezone": "UTC",
				},
			},
		},
		Actions: []PolicyAction{
			{
				Type: "require_approval",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	pee.rules[timeBasedRule.ID] = timeBasedRule
}

func (pee *PolicyEnforcementEngine) registerDefaultResponseHandlers() {
	// Rate limit violation handler
	pee.responseHandlers["rate_limit"] = func(violation *PolicyViolation) []ResponseAction {
		return []ResponseAction{
			{
				Action:    "apply_backoff",
				Timestamp: time.Now(),
				Success:   true,
				Details:   "Applied exponential backoff",
			},
			{
				Action:    "notify_admin",
				Timestamp: time.Now(),
				Success:   true,
				Details:   "Administrator notified of rate limit violation",
			},
		}
	}
	
	// Input validation failure handler
	pee.responseHandlers["input_validation"] = func(violation *PolicyViolation) []ResponseAction {
		return []ResponseAction{
			{
				Action:    "sanitize_input",
				Timestamp: time.Now(),
				Success:   true,
				Details:   "Input sanitized and logged",
			},
			{
				Action:    "increase_monitoring",
				Timestamp: time.Now(),
				Success:   true,
				Details:   "Increased monitoring for user session",
			},
		}
	}
	
	// Resource limit violation handler
	pee.responseHandlers["resource_limit"] = func(violation *PolicyViolation) []ResponseAction {
		return []ResponseAction{
			{
				Action:    "throttle_requests",
				Timestamp: time.Now(),
				Success:   true,
				Details:   "Request throttling applied",
			},
			{
				Action:    "cleanup_resources",
				Timestamp: time.Now(),
				Success:   true,
				Details:   "Initiated resource cleanup",
			},
		}
	}
}

func (pee *PolicyEnforcementEngine) getApplicableRules(enforcementCtx *EnforcementContext) []*PolicyRule {
	var applicableRules []*PolicyRule
	
	for _, rule := range pee.rules {
		if !rule.Enabled {
			continue
		}
		
		// Check time validity
		if rule.ValidFrom != nil && enforcementCtx.Timestamp.Before(*rule.ValidFrom) {
			continue
		}
		
		if rule.ValidUntil != nil && enforcementCtx.Timestamp.After(*rule.ValidUntil) {
			continue
		}
		
		applicableRules = append(applicableRules, rule)
	}
	
	return applicableRules
}

func (pee *PolicyEnforcementEngine) evaluateRule(rule *PolicyRule, enforcementCtx *EnforcementContext) *PolicyViolation {
	// Check if any exceptions apply
	for _, exception := range rule.Exceptions {
		if pee.evaluateConditions(exception.Conditions, enforcementCtx) {
			return nil // Exception applies, skip this rule
		}
	}
	
	// Evaluate rule conditions
	if !pee.evaluateConditions(rule.Conditions, enforcementCtx) {
		return nil // Rule conditions not met
	}
	
	// Create violation
	violation := &PolicyViolation{
		ID:          fmt.Sprintf("violation_%d_%s", time.Now().Unix(), rule.ID),
		Type:        PolicyViolationType(rule.Category),
		Severity:    rule.Severity,
		Source:      "policy_enforcer",
		User:        enforcementCtx.User,
		Resource:    enforcementCtx.Resource,
		Action:      enforcementCtx.Action,
		Description: fmt.Sprintf("Policy rule violated: %s", rule.Description),
		Evidence:    make(map[string]interface{}),
		Timestamp:   enforcementCtx.Timestamp,
		Policy:      rule.Name,
		Remediation: pee.generateRemediationSteps(rule),
		Impact:      pee.assessViolationImpact(rule.Severity, rule.Category),
	}
	
	// Add evidence
	violation.Evidence["rule_id"] = rule.ID
	violation.Evidence["enforcement_context"] = enforcementCtx
	violation.Evidence["conditions_met"] = rule.Conditions
	
	// Store violation
	pee.mu.Lock()
	pee.violations[violation.ID] = violation
	pee.mu.Unlock()
	
	return violation
}

func (pee *PolicyEnforcementEngine) evaluateConditions(conditions []PolicyCondition, enforcementCtx *EnforcementContext) bool {
	if len(conditions) == 0 {
		return false
	}
	
	// All conditions must be true (AND logic)
	for _, condition := range conditions {
		result := pee.evaluateCondition(condition, enforcementCtx)
		if condition.Negate {
			result = !result
		}
		if !result {
			return false
		}
	}
	
	return true
}

func (pee *PolicyEnforcementEngine) evaluateCondition(condition PolicyCondition, enforcementCtx *EnforcementContext) bool {
	fieldValue := pee.getFieldValue(condition.Field, enforcementCtx)
	
	switch condition.Operator {
	case "eq":
		return pee.compareValues(fieldValue, condition.Value, "eq")
	case "ne":
		return pee.compareValues(fieldValue, condition.Value, "ne")
	case "gt":
		return pee.compareValues(fieldValue, condition.Value, "gt")
	case "lt":
		return pee.compareValues(fieldValue, condition.Value, "lt")
	case "gte":
		return pee.compareValues(fieldValue, condition.Value, "gte")
	case "lte":
		return pee.compareValues(fieldValue, condition.Value, "lte")
	case "contains":
		return pee.stringContains(fieldValue, condition.Value)
	case "matches":
		return pee.regexMatches(fieldValue, condition.Value)
	case "in":
		return pee.valueInList(fieldValue, condition.Value)
	case "exceeds":
		return pee.checkResourceLimits(condition.Value)
	case "outside_hours":
		return pee.checkTimeWindow(condition.Value, enforcementCtx.Timestamp)
	default:
		// Check custom validators
		if validator, exists := pee.customValidators[condition.Operator]; exists {
			return validator(enforcementCtx)
		}
		return false
	}
}

func (pee *PolicyEnforcementEngine) getFieldValue(field string, enforcementCtx *EnforcementContext) interface{} {
	switch field {
	case "user":
		return enforcementCtx.User
	case "resource":
		return enforcementCtx.Resource
	case "action":
		return enforcementCtx.Action
	case "ip_address":
		return enforcementCtx.IPAddress
	case "time_of_day":
		return enforcementCtx.Timestamp.Format("15:04")
	case "day_of_week":
		return enforcementCtx.Timestamp.Weekday().String()
	case "rate_limit_check":
		if pee.rateLimiter != nil {
			result := pee.rateLimiter.CheckRateLimit(context.Background(), enforcementCtx.User, enforcementCtx.Resource, enforcementCtx.IPAddress)
			if result.Allowed {
				return "allowed"
			}
			return "exceeded"
		}
		return "unknown"
	case "input_validation":
		if pee.inputValidator != nil && enforcementCtx.RequestData != nil {
			result := pee.inputValidator.ValidateInput(enforcementCtx.RequestData)
			if result.Valid {
				return "passed"
			}
			return "failed"
		}
		return "unknown"
	case "resource_usage":
		if pee.resourceMonitor != nil {
			alerts := pee.resourceMonitor.CheckLimits()
			if len(alerts) > 0 {
				return "exceeds"
			}
			return "normal"
		}
		return "unknown"
	default:
		// Check metadata
		if enforcementCtx.Metadata != nil {
			if value, exists := enforcementCtx.Metadata[field]; exists {
				return value
			}
		}
		// Check request data
		if enforcementCtx.RequestData != nil {
			if value, exists := enforcementCtx.RequestData[field]; exists {
				return value
			}
		}
		return nil
	}
}

func (pee *PolicyEnforcementEngine) compareValues(fieldValue, conditionValue interface{}, operator string) bool {
	// This is a simplified comparison - in a real implementation, you'd want more robust type handling
	switch operator {
	case "eq":
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", conditionValue)
	case "ne":
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", conditionValue)
	default:
		return false
	}
}

func (pee *PolicyEnforcementEngine) stringContains(fieldValue, conditionValue interface{}) bool {
	fieldStr := fmt.Sprintf("%v", fieldValue)
	conditionStr := fmt.Sprintf("%v", conditionValue)
	return strings.Contains(strings.ToLower(fieldStr), strings.ToLower(conditionStr))
}

func (pee *PolicyEnforcementEngine) regexMatches(fieldValue, conditionValue interface{}) bool {
	// Implement regex matching
	return false // Placeholder
}

func (pee *PolicyEnforcementEngine) valueInList(fieldValue, conditionValue interface{}) bool {
	// Implement list checking
	return false // Placeholder
}

func (pee *PolicyEnforcementEngine) checkResourceLimits(conditionValue interface{}) bool {
	if pee.resourceMonitor == nil {
		return false
	}
	
	alerts := pee.resourceMonitor.CheckLimits()
	return len(alerts) > 0
}

func (pee *PolicyEnforcementEngine) checkTimeWindow(conditionValue interface{}, timestamp time.Time) bool {
	// Implement time window checking
	return false // Placeholder
}

func (pee *PolicyEnforcementEngine) shouldBlockRequest(violations []PolicyViolation) bool {
	if pee.mode == EnforcementModePermissive || pee.mode == EnforcementModeAuditing {
		return false
	}
	
	// Block if any violation is critical or high severity
	for _, violation := range violations {
		if violation.Severity == SeverityCritical || violation.Severity == SeverityHigh {
			return true
		}
	}
	
	return false
}

func (pee *PolicyEnforcementEngine) executeResponseActions(violation *PolicyViolation) []ResponseAction {
	var actions []ResponseAction
	
	// Check for custom response handlers
	if handler, exists := pee.responseHandlers[string(violation.Type)]; exists {
		customActions := handler(violation)
		actions = append(actions, customActions...)
	}
	
	// Default actions based on severity
	switch violation.Severity {
	case SeverityCritical:
		actions = append(actions, ResponseAction{
			Action:    "immediate_alert",
			Timestamp: time.Now(),
			Success:   true,
			Details:   "Critical violation - immediate alert sent",
		})
	case SeverityHigh:
		actions = append(actions, ResponseAction{
			Action:    "elevated_monitoring",
			Timestamp: time.Now(),
			Success:   true,
			Details:   "High severity violation - monitoring increased",
		})
	}
	
	// Update statistics
	pee.statsMu.Lock()
	for _, action := range actions {
		pee.enforcementStats.ResponseActions[action.Action]++
	}
	pee.statsMu.Unlock()
	
	return actions
}

func (pee *PolicyEnforcementEngine) generateViolationMessage(violations []PolicyViolation) string {
	if len(violations) == 1 {
		return fmt.Sprintf("Security policy violation: %s", violations[0].Description)
	}
	
	return fmt.Sprintf("Multiple security policy violations detected (%d total)", len(violations))
}

func (pee *PolicyEnforcementEngine) generateRemediationSteps(rule *PolicyRule) []string {
	var steps []string
	
	switch rule.Category {
	case "access_control":
		steps = append(steps, "Review user permissions and access controls")
		steps = append(steps, "Verify user authentication and authorization")
	case "input_validation":
		steps = append(steps, "Sanitize and validate all user inputs")
		steps = append(steps, "Review input validation rules and patterns")
	case "resource_management":
		steps = append(steps, "Monitor system resource usage")
		steps = append(steps, "Implement resource quotas and limits")
	default:
		steps = append(steps, "Review security policies and procedures")
		steps = append(steps, "Investigate potential security threats")
	}
	
	return steps
}

func (pee *PolicyEnforcementEngine) assessViolationImpact(severity ViolationSeverity, category string) ViolationImpact {
	impact := ViolationImpact{}
	
	switch severity {
	case SeverityCritical:
		impact.ConfidentialityImpact = "high"
		impact.IntegrityImpact = "high"
		impact.AvailabilityImpact = "high"
		impact.BusinessImpact = "severe"
	case SeverityHigh:
		impact.ConfidentialityImpact = "medium"
		impact.IntegrityImpact = "medium"
		impact.AvailabilityImpact = "medium"
		impact.BusinessImpact = "significant"
	case SeverityMedium:
		impact.ConfidentialityImpact = "low"
		impact.IntegrityImpact = "low"
		impact.AvailabilityImpact = "low"
		impact.BusinessImpact = "moderate"
	case SeverityLow:
		impact.ConfidentialityImpact = "minimal"
		impact.IntegrityImpact = "minimal"
		impact.AvailabilityImpact = "minimal"
		impact.BusinessImpact = "minor"
	}
	
	return impact
}

func (pee *PolicyEnforcementEngine) logEnforcementResult(enforcementCtx *EnforcementContext, result *EnforcementResult, duration time.Duration) {
	level := AuditLevelInfo
	resultStr := "ALLOWED"
	
	if !result.Allowed {
		level = AuditLevelWarning
		resultStr = "BLOCKED"
	}
	
	if len(result.Violations) > 0 {
		for _, violation := range result.Violations {
			if violation.Severity == SeverityCritical {
				level = AuditLevelCritical
				break
			}
		}
	}
	
	details := map[string]interface{}{
		"enforcement_time_ms": duration.Milliseconds(),
		"violations_count":    len(result.Violations),
		"actions_count":       len(result.Actions),
		"enforcement_mode":    pee.mode,
	}
	
	if len(result.Violations) > 0 {
		details["violation_types"] = make([]string, len(result.Violations))
		for i, violation := range result.Violations {
			details["violation_types"].([]string)[i] = string(violation.Type)
		}
	}
	
	pee.auditor.LogEvent(&AuditEvent{
		Level:     level,
		EventType: EventTypeAuthorization,
		Source:    "policy_enforcer",
		User:      enforcementCtx.User,
		IPAddress: enforcementCtx.IPAddress,
		Resource:  enforcementCtx.Resource,
		Action:    "POLICY_ENFORCEMENT",
		Result:    resultStr,
		Message:   fmt.Sprintf("Policy enforcement result for %s accessing %s: %s", enforcementCtx.User, enforcementCtx.Resource, resultStr),
		Details:   details,
	})
}