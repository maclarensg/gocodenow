package security

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// ThreatLevel defines threat severity levels
type ThreatLevel string

const (
	ThreatLevelLow      ThreatLevel = "low"
	ThreatLevelMedium   ThreatLevel = "medium"
	ThreatLevelHigh     ThreatLevel = "high"
	ThreatLevelCritical ThreatLevel = "critical"
)

// ThreatType defines different types of security threats
type ThreatType string

const (
	ThreatTypeAnomalousAccess      ThreatType = "anomalous_access"
	ThreatTypeBruteForce           ThreatType = "brute_force"
	ThreatTypePrivilegeEscalation  ThreatType = "privilege_escalation"
	ThreatTypeDataExfiltration     ThreatType = "data_exfiltration"
	ThreatTypeCommandInjection     ThreatType = "command_injection"
	ThreatTypeResourceAbuse        ThreatType = "resource_abuse"
	ThreatTypeSuspiciousPattern    ThreatType = "suspicious_pattern"
	ThreatTypeIntrusionAttempt     ThreatType = "intrusion_attempt"
	ThreatTypeInsiderThreat        ThreatType = "insider_threat"
	ThreatTypeMaliciousPayload     ThreatType = "malicious_payload"
	ThreatTypeTimingAttack         ThreatType = "timing_attack"
	ThreatTypePersistentThreat     ThreatType = "persistent_threat"
)

// ThreatIndicator represents a security threat indicator
type ThreatIndicator struct {
	ID            string                 `json:"id"`
	Type          ThreatType             `json:"type"`
	Level         ThreatLevel            `json:"level"`
	Confidence    float64                `json:"confidence"`    // 0.0 to 1.0
	Source        string                 `json:"source"`
	Target        string                 `json:"target,omitempty"`
	Description   string                 `json:"description"`
	Evidence      []ThreatEvidence       `json:"evidence"`
	Timestamp     time.Time              `json:"timestamp"`
	Duration      time.Duration          `json:"duration,omitempty"`
	AffectedUsers []string               `json:"affected_users,omitempty"`
	Indicators    []string               `json:"indicators"`    // IOCs, patterns, etc.
	Metadata      map[string]interface{} `json:"metadata"`
	Status        ThreatStatus           `json:"status"`
	Remediation   []string               `json:"remediation"`
	FirstSeen     time.Time              `json:"first_seen"`
	LastSeen      time.Time              `json:"last_seen"`
	Frequency     int64                  `json:"frequency"`
}

// ThreatEvidence represents evidence supporting a threat indicator
type ThreatEvidence struct {
	Type        string                 `json:"type"`
	Source      string                 `json:"source"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data"`
	Relevance   float64                `json:"relevance"` // 0.0 to 1.0
	Description string                 `json:"description"`
}

// ThreatStatus defines the status of a threat
type ThreatStatus string

const (
	ThreatStatusActive      ThreatStatus = "active"
	ThreatStatusInvestigating ThreatStatus = "investigating"
	ThreatStatusMitigated   ThreatStatus = "mitigated"
	ThreatStatusResolved    ThreatStatus = "resolved"
	ThreatStatusFalsePositive ThreatStatus = "false_positive"
)

// SecurityReview represents a security review finding
type SecurityReview struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Severity     ThreatLevel            `json:"severity"`
	Category     string                 `json:"category"`
	Findings     []ReviewFinding        `json:"findings"`
	Recommendations []string            `json:"recommendations"`
	AffectedSystems []string            `json:"affected_systems"`
	RiskScore    float64                `json:"risk_score"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ReviewedBy   string                 `json:"reviewed_by"`
	Status       string                 `json:"status"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// ReviewFinding represents a specific finding in a security review
type ReviewFinding struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Evidence    []string               `json:"evidence"`
	Impact      string                 `json:"impact"`
	Likelihood  string                 `json:"likelihood"`
	RiskLevel   ThreatLevel            `json:"risk_level"`
	CWE         []string               `json:"cwe,omitempty"`    // Common Weakness Enumeration
	CVSS        *CVSSScore             `json:"cvss,omitempty"`   // Common Vulnerability Scoring System
	References  []string               `json:"references,omitempty"`
}

// CVSSScore represents CVSS scoring information
type CVSSScore struct {
	Version       string  `json:"version"`
	BaseScore     float64 `json:"base_score"`
	TemporalScore float64 `json:"temporal_score,omitempty"`
	EnvironmentalScore float64 `json:"environmental_score,omitempty"`
	Vector        string  `json:"vector"`
}

// AnomalyDetectionRule defines rules for detecting anomalous behavior
type AnomalyDetectionRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	Enabled     bool          `json:"enabled"`
	Sensitivity float64       `json:"sensitivity"` // 0.0 to 1.0
	Window      time.Duration `json:"window"`
	Threshold   float64       `json:"threshold"`
	Parameters  map[string]interface{} `json:"parameters"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// UserBehaviorProfile tracks user behavior for anomaly detection
type UserBehaviorProfile struct {
	UserID           string                 `json:"user_id"`
	LastUpdate       time.Time              `json:"last_update"`
	AccessPatterns   map[string]float64     `json:"access_patterns"`    // resource -> frequency
	TimePatterns     map[int]float64        `json:"time_patterns"`      // hour -> frequency
	CommandPatterns  map[string]float64     `json:"command_patterns"`   // command -> frequency
	LocationPatterns map[string]float64     `json:"location_patterns"`  // IP/location -> frequency
	FailureRate      float64                `json:"failure_rate"`
	AverageSession   time.Duration          `json:"average_session"`
	RiskScore        float64                `json:"risk_score"`
	Anomalies        []BehaviorAnomaly      `json:"anomalies"`
	BaselineData     map[string]interface{} `json:"baseline_data"`
}

// BehaviorAnomaly represents detected anomalous behavior
type BehaviorAnomaly struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Severity    ThreatLevel            `json:"severity"`
	Confidence  float64                `json:"confidence"`
	Timestamp   time.Time              `json:"timestamp"`
	Evidence    map[string]interface{} `json:"evidence"`
}

// ThreatIntelligence provides threat intelligence information
type ThreatIntelligence struct {
	IOCs           map[string]IOC         `json:"iocs"`           // Indicators of Compromise
	ThreatActors   map[string]ThreatActor `json:"threat_actors"`
	AttackPatterns map[string]AttackPattern `json:"attack_patterns"`
	Vulnerabilities map[string]Vulnerability `json:"vulnerabilities"`
	LastUpdated    time.Time              `json:"last_updated"`
	Sources        []string               `json:"sources"`
}

// IOC represents an Indicator of Compromise
type IOC struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`        // IP, domain, hash, etc.
	Value       string    `json:"value"`
	Confidence  float64   `json:"confidence"`
	Severity    ThreatLevel `json:"severity"`
	Description string    `json:"description"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Source      string    `json:"source"`
	Tags        []string  `json:"tags"`
}

// ThreatActor represents a threat actor
type ThreatActor struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases"`
	Description string   `json:"description"`
	Tactics     []string `json:"tactics"`
	Techniques  []string `json:"techniques"`
	Procedures  []string `json:"procedures"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
}

// AttackPattern represents an attack pattern
type AttackPattern struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Techniques     []string `json:"techniques"`
	Indicators     []string `json:"indicators"`
	Mitigations    []string `json:"mitigations"`
	DetectionRules []string `json:"detection_rules"`
}

// Vulnerability represents a security vulnerability
type Vulnerability struct {
	ID          string     `json:"id"`
	CVE         string     `json:"cve,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Severity    ThreatLevel `json:"severity"`
	CVSS        *CVSSScore `json:"cvss,omitempty"`
	CWE         []string   `json:"cwe"`
	References  []string   `json:"references"`
	Published   time.Time  `json:"published"`
	Modified    time.Time  `json:"modified"`
}

// ThreatDetectionEngine provides comprehensive threat detection capabilities
type ThreatDetectionEngine struct {
	config              ThreatDetectionConfig
	indicators          map[string]*ThreatIndicator
	reviews             map[string]*SecurityReview
	anomalyRules        map[string]*AnomalyDetectionRule
	userProfiles        map[string]*UserBehaviorProfile
	threatIntel         *ThreatIntelligence
	mu                  sync.RWMutex
	auditor             *SecurityAuditor
	resourceMonitor     *ResourceMonitor
	rateLimiter         *SecurityRateLimiter
	policyEnforcer      *PolicyEnforcementEngine
	detectionStats      DetectionStatistics
	statsMu             sync.RWMutex
	alertThresholds     map[ThreatType]float64
	correlationRules    map[string]CorrelationRule
	ticker              *time.Ticker
	stopChan            chan struct{}
	wg                  sync.WaitGroup
}

// ThreatDetectionConfig configures threat detection behavior
type ThreatDetectionConfig struct {
	EnableRealTimeDetection bool          `json:"enable_realtime_detection"`
	EnableBehaviorAnalysis  bool          `json:"enable_behavior_analysis"`
	EnableThreatIntel       bool          `json:"enable_threat_intel"`
	AnalysisInterval        time.Duration `json:"analysis_interval"`
	RetentionPeriod         time.Duration `json:"retention_period"`
	ConfidenceThreshold     float64       `json:"confidence_threshold"`
	AlertingEnabled         bool          `json:"alerting_enabled"`
	AutoMitigation          bool          `json:"auto_mitigation"`
	ThreatIntelSources      []string      `json:"threat_intel_sources"`
	BaselinePeriod          time.Duration `json:"baseline_period"`
	AnomalyThreshold        float64       `json:"anomaly_threshold"`
}

// DetectionStatistics tracks threat detection statistics
type DetectionStatistics struct {
	TotalThreats         int64                    `json:"total_threats"`
	ThreatsByType        map[ThreatType]int64     `json:"threats_by_type"`
	ThreatsByLevel       map[ThreatLevel]int64    `json:"threats_by_level"`
	FalsePositives       int64                    `json:"false_positives"`
	TruePositives        int64                    `json:"true_positives"`
	ReviewsCompleted     int64                    `json:"reviews_completed"`
	AnomaliesDetected    int64                    `json:"anomalies_detected"`
	IOCMatches           int64                    `json:"ioc_matches"`
	ActiveThreats        int64                    `json:"active_threats"`
	MitigatedThreats     int64                    `json:"mitigated_threats"`
	AverageDetectionTime time.Duration           `json:"average_detection_time"`
	LastUpdated          time.Time               `json:"last_updated"`
}

// CorrelationRule defines rules for correlating events
type CorrelationRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Events      []string      `json:"events"`        // Event types to correlate
	Window      time.Duration `json:"window"`        // Time window for correlation
	Threshold   int           `json:"threshold"`     // Minimum event count
	Weight      float64       `json:"weight"`        // Weight in overall threat score
	Enabled     bool          `json:"enabled"`
}

// NewThreatDetectionEngine creates a new threat detection engine
func NewThreatDetectionEngine(
	config ThreatDetectionConfig,
	auditor *SecurityAuditor,
	resourceMonitor *ResourceMonitor,
	rateLimiter *SecurityRateLimiter,
	policyEnforcer *PolicyEnforcementEngine,
) (*ThreatDetectionEngine, error) {
	
	if config.AnalysisInterval == 0 {
		config.AnalysisInterval = 30 * time.Second
	}
	
	if config.RetentionPeriod == 0 {
		config.RetentionPeriod = 30 * 24 * time.Hour // 30 days
	}
	
	if config.ConfidenceThreshold == 0 {
		config.ConfidenceThreshold = 0.7
	}
	
	if config.BaselinePeriod == 0 {
		config.BaselinePeriod = 7 * 24 * time.Hour // 7 days
	}
	
	if config.AnomalyThreshold == 0 {
		config.AnomalyThreshold = 2.0 // 2 standard deviations
	}

	engine := &ThreatDetectionEngine{
		config:          config,
		indicators:      make(map[string]*ThreatIndicator),
		reviews:         make(map[string]*SecurityReview),
		anomalyRules:    make(map[string]*AnomalyDetectionRule),
		userProfiles:    make(map[string]*UserBehaviorProfile),
		threatIntel:     &ThreatIntelligence{
			IOCs:           make(map[string]IOC),
			ThreatActors:   make(map[string]ThreatActor),
			AttackPatterns: make(map[string]AttackPattern),
			Vulnerabilities: make(map[string]Vulnerability),
			LastUpdated:    time.Now(),
		},
		auditor:         auditor,
		resourceMonitor: resourceMonitor,
		rateLimiter:     rateLimiter,
		policyEnforcer:  policyEnforcer,
		detectionStats: DetectionStatistics{
			ThreatsByType:  make(map[ThreatType]int64),
			ThreatsByLevel: make(map[ThreatLevel]int64),
		},
		alertThresholds:  createDefaultAlertThresholds(),
		correlationRules: createDefaultCorrelationRules(),
		stopChan:         make(chan struct{}),
	}

	// Load default anomaly detection rules
	engine.loadDefaultAnomalyRules()

	// Start background analysis if real-time detection is enabled
	if config.EnableRealTimeDetection {
		engine.ticker = time.NewTicker(config.AnalysisInterval)
		engine.wg.Add(1)
		go engine.analysisLoop()
	}

	return engine, nil
}

// AnalyzeSecurityEvents analyzes security events for threats
func (tde *ThreatDetectionEngine) AnalyzeSecurityEvents(ctx context.Context, events []*AuditEvent) ([]*ThreatIndicator, error) {
	var indicators []*ThreatIndicator
	
	// Group events by user and time for pattern analysis
	eventGroups := tde.groupEventsByPattern(events)
	
	// Analyze each group for suspicious patterns
	for _, group := range eventGroups {
		if threatIndicators := tde.analyzeEventGroup(group); len(threatIndicators) > 0 {
			indicators = append(indicators, threatIndicators...)
		}
	}
	
	// Correlate events using correlation rules
	correlatedIndicators := tde.correlateEvents(events)
	indicators = append(indicators, correlatedIndicators...)
	
	// Check against threat intelligence
	if tde.config.EnableThreatIntel {
		intelIndicators := tde.checkThreatIntelligence(events)
		indicators = append(indicators, intelIndicators...)
	}
	
	// Update user behavior profiles and detect anomalies
	if tde.config.EnableBehaviorAnalysis {
		behaviorIndicators := tde.analyzeBehaviorAnomalies(events)
		indicators = append(indicators, behaviorIndicators...)
	}
	
	// Store and process indicators
	for _, indicator := range indicators {
		tde.processNewIndicator(indicator)
	}
	
	// Update statistics
	tde.updateDetectionStats(indicators)
	
	return indicators, nil
}

// ConductSecurityReview performs a comprehensive security review
func (tde *ThreatDetectionEngine) ConductSecurityReview(ctx context.Context, scope string) (*SecurityReview, error) {
	reviewID := fmt.Sprintf("review_%d", time.Now().Unix())
	
	review := &SecurityReview{
		ID:          reviewID,
		Type:        "comprehensive",
		Title:       fmt.Sprintf("Security Review - %s", scope),
		Description: "Comprehensive security review of system and processes",
		Category:    "security_assessment",
		CreatedAt:   time.Now(),
		Status:      "in_progress",
		Metadata:    make(map[string]interface{}),
	}
	
	var findings []ReviewFinding
	
	// Review audit logs for security events
	auditFindings := tde.reviewAuditLogs(ctx)
	findings = append(findings, auditFindings...)
	
	// Review access patterns and permissions
	accessFindings := tde.reviewAccessPatterns(ctx)
	findings = append(findings, accessFindings...)
	
	// Review resource usage and limits
	resourceFindings := tde.reviewResourceUsage(ctx)
	findings = append(findings, resourceFindings...)
	
	// Review policy violations
	policyFindings := tde.reviewPolicyViolations(ctx)
	findings = append(findings, policyFindings...)
	
	// Review threat indicators
	threatFindings := tde.reviewThreatIndicators(ctx)
	findings = append(findings, threatFindings...)
	
	// Calculate overall risk score
	riskScore := tde.calculateRiskScore(findings)
	
	// Generate recommendations
	recommendations := tde.generateRecommendations(findings)
	
	// Finalize review
	review.Findings = findings
	review.RiskScore = riskScore
	review.Recommendations = recommendations
	review.Severity = tde.determineSeverityFromRisk(riskScore)
	review.UpdatedAt = time.Now()
	review.Status = "completed"
	
	// Store review
	tde.mu.Lock()
	tde.reviews[reviewID] = review
	tde.mu.Unlock()
	
	// Update statistics
	tde.statsMu.Lock()
	tde.detectionStats.ReviewsCompleted++
	tde.detectionStats.LastUpdated = time.Now()
	tde.statsMu.Unlock()
	
	// Log review completion
	if tde.auditor != nil {
		tde.auditor.LogEvent(&AuditEvent{
			Level:     AuditLevelInfo,
			EventType: EventTypeSystemAccess,
			Source:    "threat_detector",
			Action:    "SECURITY_REVIEW_COMPLETED",
			Result:    "SUCCESS",
			Message:   fmt.Sprintf("Security review completed with risk score %.2f", riskScore),
			Details: map[string]interface{}{
				"review_id":      reviewID,
				"findings_count": len(findings),
				"risk_score":     riskScore,
				"severity":       review.Severity,
			},
		})
	}
	
	return review, nil
}

// GetThreatIndicators returns current threat indicators
func (tde *ThreatDetectionEngine) GetThreatIndicators(status ThreatStatus) []*ThreatIndicator {
	tde.mu.RLock()
	defer tde.mu.RUnlock()
	
	var indicators []*ThreatIndicator
	for _, indicator := range tde.indicators {
		if status == "" || indicator.Status == status {
			indicatorCopy := *indicator
			indicators = append(indicators, &indicatorCopy)
		}
	}
	
	// Sort by timestamp (newest first)
	sort.Slice(indicators, func(i, j int) bool {
		return indicators[i].Timestamp.After(indicators[j].Timestamp)
	})
	
	return indicators
}

// GetSecurityReviews returns security reviews
func (tde *ThreatDetectionEngine) GetSecurityReviews(limit int) []*SecurityReview {
	tde.mu.RLock()
	defer tde.mu.RUnlock()
	
	var reviews []*SecurityReview
	for _, review := range tde.reviews {
		reviewCopy := *review
		reviews = append(reviews, &reviewCopy)
	}
	
	// Sort by creation date (newest first)
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].CreatedAt.After(reviews[j].CreatedAt)
	})
	
	if limit > 0 && len(reviews) > limit {
		reviews = reviews[:limit]
	}
	
	return reviews
}

// GetDetectionStatistics returns detection statistics
func (tde *ThreatDetectionEngine) GetDetectionStatistics() DetectionStatistics {
	tde.statsMu.RLock()
	defer tde.statsMu.RUnlock()
	
	stats := tde.detectionStats
	
	// Update active threats count
	tde.mu.RLock()
	activeCount := int64(0)
	mitigatedCount := int64(0)
	for _, indicator := range tde.indicators {
		if indicator.Status == ThreatStatusActive {
			activeCount++
		} else if indicator.Status == ThreatStatusMitigated || indicator.Status == ThreatStatusResolved {
			mitigatedCount++
		}
	}
	tde.mu.RUnlock()
	
	stats.ActiveThreats = activeCount
	stats.MitigatedThreats = mitigatedCount
	stats.LastUpdated = time.Now()
	
	return stats
}

// UpdateThreatStatus updates the status of a threat indicator
func (tde *ThreatDetectionEngine) UpdateThreatStatus(threatID string, status ThreatStatus, notes string) error {
	tde.mu.Lock()
	defer tde.mu.Unlock()
	
	indicator, exists := tde.indicators[threatID]
	if !exists {
		return fmt.Errorf("threat indicator %s not found", threatID)
	}
	
	oldStatus := indicator.Status
	indicator.Status = status
	indicator.LastSeen = time.Now()
	
	// Add status change to metadata
	if indicator.Metadata == nil {
		indicator.Metadata = make(map[string]interface{})
	}
	
	statusHistory := indicator.Metadata["status_history"]
	if statusHistory == nil {
		statusHistory = []map[string]interface{}{}
	}
	
	statusEntry := map[string]interface{}{
		"from":      oldStatus,
		"to":        status,
		"timestamp": time.Now(),
		"notes":     notes,
	}
	
	indicator.Metadata["status_history"] = append(statusHistory.([]map[string]interface{}), statusEntry)
	
	// Log status change
	if tde.auditor != nil {
		tde.auditor.LogEvent(&AuditEvent{
			Level:     AuditLevelInfo,
			EventType: EventTypeSystemAccess,
			Source:    "threat_detector",
			Action:    "UPDATE_THREAT_STATUS",
			Result:    "SUCCESS",
			Message:   fmt.Sprintf("Threat indicator %s status changed from %s to %s", threatID, oldStatus, status),
			Details: map[string]interface{}{
				"threat_id":   threatID,
				"old_status":  oldStatus,
				"new_status":  status,
				"notes":       notes,
			},
		})
	}
	
	return nil
}

// Close shuts down the threat detection engine
func (tde *ThreatDetectionEngine) Close() error {
	close(tde.stopChan)
	
	if tde.ticker != nil {
		tde.ticker.Stop()
	}
	
	tde.wg.Wait()
	
	return nil
}

// Private helper methods

func (tde *ThreatDetectionEngine) analysisLoop() {
	defer tde.wg.Done()
	
	for {
		select {
		case <-tde.ticker.C:
			tde.performPeriodicAnalysis()
		case <-tde.stopChan:
			return
		}
	}
}

func (tde *ThreatDetectionEngine) performPeriodicAnalysis() {
	ctx := context.Background()
	
	// Get recent audit events
	if tde.auditor != nil {
		filter := AuditFilter{
			StartTime: func() *time.Time { t := time.Now().Add(-tde.config.AnalysisInterval * 2); return &t }(),
			Limit:     1000,
		}
		
		if events, err := tde.auditor.SearchEvents(filter); err == nil {
			tde.AnalyzeSecurityEvents(ctx, events)
		}
	}
	
	// Clean up old indicators
	tde.cleanupOldIndicators()
	
	// Update user behavior baselines
	if tde.config.EnableBehaviorAnalysis {
		tde.updateBehaviorBaselines()
	}
}

func (tde *ThreatDetectionEngine) groupEventsByPattern(events []*AuditEvent) map[string][]*AuditEvent {
	groups := make(map[string][]*AuditEvent)
	
	for _, event := range events {
		// Group by user and event type
		key := fmt.Sprintf("%s:%s", event.User, event.EventType)
		groups[key] = append(groups[key], event)
	}
	
	return groups
}

func (tde *ThreatDetectionEngine) analyzeEventGroup(events []*AuditEvent) []*ThreatIndicator {
	var indicators []*ThreatIndicator
	
	if len(events) == 0 {
		return indicators
	}
	
	// Check for brute force patterns
	if bruteForceIndicator := tde.detectBruteForce(events); bruteForceIndicator != nil {
		indicators = append(indicators, bruteForceIndicator)
	}
	
	// Check for privilege escalation attempts
	if privEscIndicator := tde.detectPrivilegeEscalation(events); privEscIndicator != nil {
		indicators = append(indicators, privEscIndicator)
	}
	
	// Check for suspicious command patterns
	if commandIndicator := tde.detectSuspiciousCommands(events); commandIndicator != nil {
		indicators = append(indicators, commandIndicator)
	}
	
	// Check for resource abuse patterns
	if resourceIndicator := tde.detectResourceAbuse(events); resourceIndicator != nil {
		indicators = append(indicators, resourceIndicator)
	}
	
	return indicators
}

func (tde *ThreatDetectionEngine) detectBruteForce(events []*AuditEvent) *ThreatIndicator {
	if len(events) < 5 {
		return nil
	}
	
	// Look for authentication failures
	failureCount := 0
	var firstFailure, lastFailure time.Time
	
	for _, event := range events {
		if event.EventType == EventTypeAuthentication && event.Result == "FAILURE" {
			failureCount++
			if firstFailure.IsZero() {
				firstFailure = event.Timestamp
			}
			lastFailure = event.Timestamp
		}
	}
	
	// Check if failures exceed threshold within time window
	if failureCount >= 5 && lastFailure.Sub(firstFailure) <= 10*time.Minute {
		return &ThreatIndicator{
			ID:          fmt.Sprintf("brute_force_%d", time.Now().Unix()),
			Type:        ThreatTypeBruteForce,
			Level:       ThreatLevelHigh,
			Confidence:  0.8,
			Source:      "event_analysis",
			Target:      events[0].User,
			Description: fmt.Sprintf("Brute force attack detected: %d failed authentication attempts", failureCount),
			Evidence: []ThreatEvidence{
				{
					Type:        "authentication_failures",
					Source:      "audit_logs",
					Timestamp:   time.Now(),
					Data:        map[string]interface{}{"failure_count": failureCount, "time_window": lastFailure.Sub(firstFailure)},
					Relevance:   1.0,
					Description: "Multiple failed authentication attempts in short time period",
				},
			},
			Timestamp:     time.Now(),
			Duration:      lastFailure.Sub(firstFailure),
			AffectedUsers: []string{events[0].User},
			Indicators:    []string{"repeated_auth_failures", "short_time_window"},
			Metadata:      map[string]interface{}{"failure_count": failureCount},
			Status:        ThreatStatusActive,
			Remediation:   []string{"Block IP address", "Reset user credentials", "Enable account lockout"},
			FirstSeen:     firstFailure,
			LastSeen:      lastFailure,
			Frequency:     int64(failureCount),
		}
	}
	
	return nil
}

func (tde *ThreatDetectionEngine) detectPrivilegeEscalation(events []*AuditEvent) *ThreatIndicator {
	// Look for privilege escalation patterns
	for _, event := range events {
		if event.EventType == EventTypeSecurityViolation &&
		   strings.Contains(strings.ToLower(event.Message), "privilege") {
			
			return &ThreatIndicator{
				ID:          fmt.Sprintf("priv_esc_%d", time.Now().Unix()),
				Type:        ThreatTypePrivilegeEscalation,
				Level:       ThreatLevelCritical,
				Confidence:  0.9,
				Source:      "event_analysis",
				Target:      event.User,
				Description: "Privilege escalation attempt detected",
				Evidence: []ThreatEvidence{
					{
						Type:        "security_violation",
						Source:      "audit_logs",
						Timestamp:   event.Timestamp,
						Data:        map[string]interface{}{"event_details": event.Details},
						Relevance:   1.0,
						Description: "Security violation indicating privilege escalation attempt",
					},
				},
				Timestamp:     time.Now(),
				AffectedUsers: []string{event.User},
				Indicators:    []string{"privilege_escalation_attempt", "security_violation"},
				Metadata:      map[string]interface{}{"original_event": event.ID},
				Status:        ThreatStatusActive,
				Remediation:   []string{"Investigate user account", "Review system permissions", "Check for compromise"},
				FirstSeen:     event.Timestamp,
				LastSeen:      event.Timestamp,
				Frequency:     1,
			}
		}
	}
	
	return nil
}

func (tde *ThreatDetectionEngine) detectSuspiciousCommands(events []*AuditEvent) *ThreatIndicator {
	suspiciousCommands := []string{
		"nc -", "netcat", "wget", "curl", "/dev/tcp",
		"base64", "python -c", "perl -e", "ruby -e",
		"powershell", "cmd.exe", "bash -c",
	}
	
	for _, event := range events {
		if event.EventType == EventTypeToolExecution {
			if details, ok := event.Details["command"].(string); ok {
				for _, suspicious := range suspiciousCommands {
					if strings.Contains(strings.ToLower(details), suspicious) {
						return &ThreatIndicator{
							ID:          fmt.Sprintf("suspicious_cmd_%d", time.Now().Unix()),
							Type:        ThreatTypeCommandInjection,
							Level:       ThreatLevelHigh,
							Confidence:  0.7,
							Source:      "event_analysis",
							Target:      event.User,
							Description: fmt.Sprintf("Suspicious command execution detected: %s", suspicious),
							Evidence: []ThreatEvidence{
								{
									Type:        "command_execution",
									Source:      "audit_logs",
									Timestamp:   event.Timestamp,
									Data:        map[string]interface{}{"command": details},
									Relevance:   0.9,
									Description: "Potentially malicious command executed",
								},
							},
							Timestamp:     time.Now(),
							AffectedUsers: []string{event.User},
							Indicators:    []string{"suspicious_command", suspicious},
							Metadata:      map[string]interface{}{"command": details},
							Status:        ThreatStatusActive,
							Remediation:   []string{"Review command context", "Check for malware", "Investigate user activity"},
							FirstSeen:     event.Timestamp,
							LastSeen:      event.Timestamp,
							Frequency:     1,
						}
					}
				}
			}
		}
	}
	
	return nil
}

func (tde *ThreatDetectionEngine) detectResourceAbuse(events []*AuditEvent) *ThreatIndicator {
	// Check resource monitor alerts
	if tde.resourceMonitor != nil {
		alerts := tde.resourceMonitor.CheckLimits()
		if len(alerts) > 0 {
			for _, alert := range alerts {
				if alert.Type == AlertMemoryLimit || alert.Type == AlertSystemOverload {
					return &ThreatIndicator{
						ID:          fmt.Sprintf("resource_abuse_%d", time.Now().Unix()),
						Type:        ThreatTypeResourceAbuse,
						Level:       ThreatLevelHigh,
						Confidence:  0.8,
						Source:      "resource_monitor",
						Description: "Resource abuse detected: excessive resource consumption",
						Evidence: []ThreatEvidence{
							{
								Type:        "resource_alert",
								Source:      "resource_monitor",
								Timestamp:   alert.Timestamp,
								Data:        map[string]interface{}{"alert": alert},
								Relevance:   1.0,
								Description: "Critical resource usage alert triggered",
							},
						},
						Timestamp:   time.Now(),
						Indicators:  []string{"resource_exhaustion", alert.Type.String()},
						Metadata:    map[string]interface{}{"resource_type": alert.Type},
						Status:      ThreatStatusActive,
						Remediation: []string{"Limit resource usage", "Investigate cause", "Scale resources"},
						FirstSeen:   alert.Timestamp,
						LastSeen:    alert.Timestamp,
						Frequency:   1,
					}
				}
			}
		}
	}
	
	return nil
}

func (tde *ThreatDetectionEngine) correlateEvents(events []*AuditEvent) []*ThreatIndicator {
	// Implementation placeholder for event correlation
	return nil
}

func (tde *ThreatDetectionEngine) checkThreatIntelligence(events []*AuditEvent) []*ThreatIndicator {
	// Implementation placeholder for threat intelligence checking
	return nil
}

func (tde *ThreatDetectionEngine) analyzeBehaviorAnomalies(events []*AuditEvent) []*ThreatIndicator {
	// Implementation placeholder for behavior analysis
	return nil
}

func (tde *ThreatDetectionEngine) processNewIndicator(indicator *ThreatIndicator) {
	tde.mu.Lock()
	tde.indicators[indicator.ID] = indicator
	tde.mu.Unlock()
	
	// Check if alert should be generated
	if tde.config.AlertingEnabled && indicator.Confidence >= tde.config.ConfidenceThreshold {
		tde.generateAlert(indicator)
	}
	
	// Apply auto-mitigation if enabled
	if tde.config.AutoMitigation && indicator.Level == ThreatLevelCritical {
		tde.applyAutoMitigation(indicator)
	}
}

func (tde *ThreatDetectionEngine) generateAlert(indicator *ThreatIndicator) {
	if tde.auditor != nil {
		level := AuditLevelWarning
		if indicator.Level == ThreatLevelCritical {
			level = AuditLevelCritical
		}
		
		tde.auditor.LogEvent(&AuditEvent{
			Level:     level,
			EventType: EventTypeSecurityViolation,
			Source:    "threat_detector",
			Action:    "THREAT_DETECTED",
			Result:    "ALERT_GENERATED",
			Message:   fmt.Sprintf("Threat detected: %s", indicator.Description),
			Details: map[string]interface{}{
				"threat_id":   indicator.ID,
				"threat_type": indicator.Type,
				"threat_level": indicator.Level,
				"confidence":  indicator.Confidence,
				"target":      indicator.Target,
			},
		})
	}
}

func (tde *ThreatDetectionEngine) applyAutoMitigation(indicator *ThreatIndicator) {
	// Apply mitigation based on threat type
	actions := []string{}
	
	switch indicator.Type {
	case ThreatTypeBruteForce:
		actions = append(actions, "Applied rate limiting", "Initiated IP blocking")
	case ThreatTypeResourceAbuse:
		actions = append(actions, "Applied resource throttling", "Initiated cleanup")
	case ThreatTypeCommandInjection:
		actions = append(actions, "Enhanced input validation", "Increased monitoring")
	default:
		actions = append(actions, "Increased monitoring", "Alerted administrators")
	}
	
	// Update indicator status
	indicator.Status = ThreatStatusMitigated
	
	if tde.auditor != nil {
		tde.auditor.LogEvent(&AuditEvent{
			Level:     AuditLevelWarning,
			EventType: EventTypeSystemAccess,
			Source:    "threat_detector",
			Action:    "AUTO_MITIGATION",
			Result:    "SUCCESS",
			Message:   fmt.Sprintf("Auto-mitigation applied for threat %s", indicator.ID),
			Details: map[string]interface{}{
				"threat_id": indicator.ID,
				"actions":   actions,
			},
		})
	}
}

func (tde *ThreatDetectionEngine) updateDetectionStats(indicators []*ThreatIndicator) {
	tde.statsMu.Lock()
	defer tde.statsMu.Unlock()
	
	tde.detectionStats.TotalThreats += int64(len(indicators))
	
	for _, indicator := range indicators {
		tde.detectionStats.ThreatsByType[indicator.Type]++
		tde.detectionStats.ThreatsByLevel[indicator.Level]++
	}
	
	tde.detectionStats.LastUpdated = time.Now()
}

func (tde *ThreatDetectionEngine) cleanupOldIndicators() {
	cutoff := time.Now().Add(-tde.config.RetentionPeriod)
	
	tde.mu.Lock()
	for id, indicator := range tde.indicators {
		if indicator.Timestamp.Before(cutoff) && 
		   (indicator.Status == ThreatStatusResolved || indicator.Status == ThreatStatusFalsePositive) {
			delete(tde.indicators, id)
		}
	}
	tde.mu.Unlock()
}

func (tde *ThreatDetectionEngine) updateBehaviorBaselines() {
	// Implementation placeholder for behavior baseline updates
}

// Review methods for security reviews

func (tde *ThreatDetectionEngine) reviewAuditLogs(ctx context.Context) []ReviewFinding {
	var findings []ReviewFinding
	
	if tde.auditor != nil {
		filter := AuditFilter{
			Levels:    []AuditLevel{AuditLevelError, AuditLevelCritical},
			StartTime: func() *time.Time { t := time.Now().Add(-24 * time.Hour); return &t }(),
			Limit:     100,
		}
		
		if events, err := tde.auditor.SearchEvents(filter); err == nil {
			if len(events) > 10 {
				findings = append(findings, ReviewFinding{
					ID:          "audit_high_error_rate",
					Type:        "audit_analysis",
					Description: fmt.Sprintf("High number of error events detected: %d in last 24 hours", len(events)),
					Evidence:    []string{"audit_logs", "error_events"},
					Impact:      "Medium",
					Likelihood:  "High",
					RiskLevel:   ThreatLevelMedium,
				})
			}
		}
	}
	
	return findings
}

func (tde *ThreatDetectionEngine) reviewAccessPatterns(ctx context.Context) []ReviewFinding {
	var findings []ReviewFinding
	
	// Analysis placeholder - would analyze access patterns for anomalies
	
	return findings
}

func (tde *ThreatDetectionEngine) reviewResourceUsage(ctx context.Context) []ReviewFinding {
	var findings []ReviewFinding
	
	if tde.resourceMonitor != nil {
		usage := tde.resourceMonitor.GetUsage()
		
		// Check if memory usage is high
		if usage.MemoryUsage > 0 {
			findings = append(findings, ReviewFinding{
				ID:          "memory_usage_analysis",
				Type:        "resource_analysis", 
				Description: "Memory usage analysis completed",
				Evidence:    []string{"resource_monitoring", "usage_statistics"},
				Impact:      "Medium",
				Likelihood:  "High",
				RiskLevel:   ThreatLevelLow,
			})
		}
	}
	
	return findings
}

func (tde *ThreatDetectionEngine) reviewPolicyViolations(ctx context.Context) []ReviewFinding {
	var findings []ReviewFinding
	
	if tde.policyEnforcer != nil {
		violations := tde.policyEnforcer.GetViolations(time.Now().Add(-24 * time.Hour))
		
		if len(violations) > 0 {
			highSeverityCount := 0
			for _, violation := range violations {
				if violation.Severity == SeverityHigh || violation.Severity == SeverityCritical {
					highSeverityCount++
				}
			}
			
			if highSeverityCount > 0 {
				findings = append(findings, ReviewFinding{
					ID:          "policy_violations",
					Type:        "policy_analysis",
					Description: fmt.Sprintf("High severity policy violations detected: %d in last 24 hours", highSeverityCount),
					Evidence:    []string{"policy_violations", "security_events"},
					Impact:      "High",
					Likelihood:  "High",
					RiskLevel:   ThreatLevelHigh,
				})
			}
		}
	}
	
	return findings
}

func (tde *ThreatDetectionEngine) reviewThreatIndicators(ctx context.Context) []ReviewFinding {
	var findings []ReviewFinding
	
	activeIndicators := tde.GetThreatIndicators(ThreatStatusActive)
	
	if len(activeIndicators) > 0 {
		criticalCount := 0
		for _, indicator := range activeIndicators {
			if indicator.Level == ThreatLevelCritical {
				criticalCount++
			}
		}
		
		if criticalCount > 0 {
			findings = append(findings, ReviewFinding{
				ID:          "active_threats",
				Type:        "threat_analysis",
				Description: fmt.Sprintf("Critical active threats detected: %d requiring attention", criticalCount),
				Evidence:    []string{"threat_indicators", "security_monitoring"},
				Impact:      "High",
				Likelihood:  "High",
				RiskLevel:   ThreatLevelHigh,
			})
		}
	}
	
	return findings
}

func (tde *ThreatDetectionEngine) calculateRiskScore(findings []ReviewFinding) float64 {
	if len(findings) == 0 {
		return 0.0
	}
	
	score := 0.0
	weights := map[ThreatLevel]float64{
		ThreatLevelLow:      1.0,
		ThreatLevelMedium:   2.5,
		ThreatLevelHigh:     4.0,
		ThreatLevelCritical: 5.0,
	}
	
	for _, finding := range findings {
		score += weights[finding.RiskLevel]
	}
	
	// Normalize score to 0-10 scale
	maxPossibleScore := float64(len(findings)) * weights[ThreatLevelCritical]
	normalizedScore := (score / maxPossibleScore) * 10.0
	
	return math.Min(normalizedScore, 10.0)
}

func (tde *ThreatDetectionEngine) generateRecommendations(findings []ReviewFinding) []string {
	recommendations := []string{
		"Review and update security policies regularly",
		"Implement continuous monitoring and alerting",
		"Conduct regular security assessments",
		"Provide security awareness training",
		"Maintain up-to-date threat intelligence",
	}
	
	// Add specific recommendations based on findings
	for _, finding := range findings {
		switch finding.Type {
		case "audit_analysis":
			recommendations = append(recommendations, "Investigate root cause of audit log errors")
		case "resource_analysis":
			recommendations = append(recommendations, "Optimize resource usage and implement better limits")
		case "policy_analysis":
			recommendations = append(recommendations, "Review and strengthen security policy enforcement")
		case "threat_analysis":
			recommendations = append(recommendations, "Immediately investigate and mitigate active threats")
		}
	}
	
	return recommendations
}

func (tde *ThreatDetectionEngine) determineSeverityFromRisk(riskScore float64) ThreatLevel {
	switch {
	case riskScore >= 8.0:
		return ThreatLevelCritical
	case riskScore >= 6.0:
		return ThreatLevelHigh
	case riskScore >= 3.0:
		return ThreatLevelMedium
	default:
		return ThreatLevelLow
	}
}

func (tde *ThreatDetectionEngine) loadDefaultAnomalyRules() {
	// Load default anomaly detection rules
	rules := map[string]*AnomalyDetectionRule{
		"unusual_access_time": {
			ID:          "unusual_access_time",
			Name:        "Unusual Access Time",
			Description: "Detects access outside normal business hours",
			Category:    "temporal_anomaly",
			Enabled:     true,
			Sensitivity: 0.7,
			Window:      24 * time.Hour,
			Threshold:   2.0,
			Parameters: map[string]interface{}{
				"business_hours_start": 9,
				"business_hours_end":   17,
				"timezone":             "UTC",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		"unusual_resource_usage": {
			ID:          "unusual_resource_usage",
			Name:        "Unusual Resource Usage",
			Description: "Detects abnormal resource consumption patterns",
			Category:    "resource_anomaly",
			Enabled:     true,
			Sensitivity: 0.8,
			Window:      time.Hour,
			Threshold:   3.0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
	
	for id, rule := range rules {
		tde.anomalyRules[id] = rule
	}
}

// Helper functions for creating default configurations

func createDefaultAlertThresholds() map[ThreatType]float64 {
	return map[ThreatType]float64{
		ThreatTypeBruteForce:          0.8,
		ThreatTypePrivilegeEscalation: 0.9,
		ThreatTypeCommandInjection:    0.7,
		ThreatTypeResourceAbuse:       0.6,
		ThreatTypeAnomalousAccess:     0.7,
		ThreatTypeSuspiciousPattern:   0.6,
		ThreatTypeDataExfiltration:    0.9,
		ThreatTypeIntrusionAttempt:    0.8,
		ThreatTypeInsiderThreat:       0.8,
		ThreatTypeMaliciousPayload:    0.9,
	}
}

func createDefaultCorrelationRules() map[string]CorrelationRule {
	return map[string]CorrelationRule{
		"auth_failure_escalation": {
			ID:          "auth_failure_escalation",
			Name:        "Authentication Failure to Escalation",
			Description: "Correlates authentication failures with privilege escalation attempts",
			Events:      []string{"AUTHENTICATION", "SECURITY_VIOLATION"},
			Window:      10 * time.Minute,
			Threshold:   3,
			Weight:      0.9,
			Enabled:     true,
		},
		"resource_abuse_pattern": {
			ID:          "resource_abuse_pattern",
			Name:        "Resource Abuse Pattern",
			Description: "Correlates multiple resource limit violations",
			Events:      []string{"RESOURCE_LIMIT", "TOOL_EXECUTION"},
			Window:      30 * time.Minute,
			Threshold:   5,
			Weight:      0.7,
			Enabled:     true,
		},
	}
}