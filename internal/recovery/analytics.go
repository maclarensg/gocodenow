package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ErrorFrequency struct {
	ErrorType string `json:"error_type"`
	Count     int    `json:"count"`
	LastSeen  time.Time `json:"last_seen"`
}

type ErrorPattern struct {
	Pattern     string    `json:"pattern"`
	Frequency   int       `json:"frequency"`
	Severity    int       `json:"severity"`
	Component   string    `json:"component"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Resolution  string    `json:"resolution,omitempty"`
}

type ComponentHealth struct {
	Component       string    `json:"component"`
	ErrorRate       float64   `json:"error_rate"`
	HealthScore     int       `json:"health_score"`
	LastHealthy     time.Time `json:"last_healthy"`
	CriticalErrors  int       `json:"critical_errors"`
	WarningErrors   int       `json:"warning_errors"`
}

type AnalyticsReport struct {
	GeneratedAt       time.Time         `json:"generated_at"`
	TimeRange         string            `json:"time_range"`
	TotalErrors       int               `json:"total_errors"`
	CriticalErrors    int               `json:"critical_errors"`
	ErrorFrequencies  []ErrorFrequency  `json:"error_frequencies"`
	ErrorPatterns     []ErrorPattern    `json:"error_patterns"`
	ComponentHealth   []ComponentHealth `json:"component_health"`
	TopErrorsByHour   map[string]int    `json:"top_errors_by_hour"`
	RecoverySuccessRate float64         `json:"recovery_success_rate"`
	AverageRecoveryTime time.Duration   `json:"average_recovery_time"`
	Recommendations   []string          `json:"recommendations"`
}

type ErrorAnalytics struct {
	dataDir           string
	errors            []RecoverableError
	mutex             sync.RWMutex
	reportingEnabled  bool
	retentionDays     int
	maxErrors         int
	lastCleanup       time.Time
}

func NewErrorAnalytics(dataDir string) *ErrorAnalytics {
	analytics := &ErrorAnalytics{
		dataDir:          dataDir,
		errors:           make([]RecoverableError, 0),
		reportingEnabled: true,
		retentionDays:    30,
		maxErrors:        10000,
		lastCleanup:      time.Now(),
	}
	
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		analytics.reportingEnabled = false
	}
	
	analytics.loadPersistedErrors()
	return analytics
}

func (ea *ErrorAnalytics) RecordError(err RecoverableError) {
	if !ea.reportingEnabled {
		return
	}
	
	ea.mutex.Lock()
	defer ea.mutex.Unlock()
	
	ea.errors = append(ea.errors, err)
	
	if len(ea.errors) > ea.maxErrors {
		ea.errors = ea.errors[len(ea.errors)-ea.maxErrors:]
	}
	
	if time.Since(ea.lastCleanup) > 24*time.Hour {
		ea.cleanupOldErrors()
		ea.lastCleanup = time.Now()
	}
	
	ea.persistError(err)
}

func (ea *ErrorAnalytics) GenerateReport(ctx context.Context, timeRange time.Duration) (*AnalyticsReport, error) {
	ea.mutex.RLock()
	defer ea.mutex.RUnlock()
	
	cutoff := time.Now().Add(-timeRange)
	recentErrors := make([]RecoverableError, 0)
	
	for _, err := range ea.errors {
		if err.Timestamp.After(cutoff) {
			recentErrors = append(recentErrors, err)
		}
	}
	
	report := &AnalyticsReport{
		GeneratedAt:     time.Now(),
		TimeRange:       timeRange.String(),
		TotalErrors:     len(recentErrors),
		TopErrorsByHour: make(map[string]int),
	}
	
	ea.calculateErrorFrequencies(recentErrors, report)
	ea.analyzeErrorPatterns(recentErrors, report)
	ea.assessComponentHealth(recentErrors, report)
	ea.calculateHourlyDistribution(recentErrors, report)
	ea.calculateRecoveryMetrics(recentErrors, report)
	ea.generateRecommendations(report)
	
	return report, nil
}

func (ea *ErrorAnalytics) calculateErrorFrequencies(errors []RecoverableError, report *AnalyticsReport) {
	freqMap := make(map[string]*ErrorFrequency)
	
	for _, err := range errors {
		key := fmt.Sprintf("%s:%s", err.Component, err.Category)
		
		if freq, exists := freqMap[key]; exists {
			freq.Count++
			if err.Timestamp.After(freq.LastSeen) {
				freq.LastSeen = err.Timestamp
			}
		} else {
			freqMap[key] = &ErrorFrequency{
				ErrorType: key,
				Count:     1,
				LastSeen:  err.Timestamp,
			}
		}
	}
	
	frequencies := make([]ErrorFrequency, 0, len(freqMap))
	for _, freq := range freqMap {
		frequencies = append(frequencies, *freq)
	}
	
	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].Count > frequencies[j].Count
	})
	
	report.ErrorFrequencies = frequencies
}

func (ea *ErrorAnalytics) analyzeErrorPatterns(errors []RecoverableError, report *AnalyticsReport) {
	patternMap := make(map[string]*ErrorPattern)
	
	for _, err := range errors {
		pattern := ea.extractErrorPattern(err)
		
		if existing, exists := patternMap[pattern]; exists {
			existing.Frequency++
			if err.Timestamp.After(existing.LastSeen) {
				existing.LastSeen = err.Timestamp
			}
			if err.Severity > existing.Severity {
				existing.Severity = err.Severity
			}
		} else {
			patternMap[pattern] = &ErrorPattern{
				Pattern:   pattern,
				Frequency: 1,
				Severity:  err.Severity,
				Component: err.Component,
				FirstSeen: err.Timestamp,
				LastSeen:  err.Timestamp,
			}
		}
	}
	
	patterns := make([]ErrorPattern, 0, len(patternMap))
	for _, pattern := range patternMap {
		patterns = append(patterns, *pattern)
	}
	
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Severity != patterns[j].Severity {
			return patterns[i].Severity > patterns[j].Severity
		}
		return patterns[i].Frequency > patterns[j].Frequency
	})
	
	report.ErrorPatterns = patterns
}

func (ea *ErrorAnalytics) assessComponentHealth(errors []RecoverableError, report *AnalyticsReport) {
	componentMap := make(map[string]*ComponentHealth)
	componentErrorCounts := make(map[string]int)
	
	for _, err := range errors {
		componentErrorCounts[err.Component]++
		
		if health, exists := componentMap[err.Component]; exists {
			if err.Severity >= 8 {
				health.CriticalErrors++
			} else if err.Severity >= 5 {
				health.WarningErrors++
			}
		} else {
			health := &ComponentHealth{
				Component:   err.Component,
				LastHealthy: time.Now().Add(-24 * time.Hour),
			}
			
			if err.Severity >= 8 {
				health.CriticalErrors = 1
			} else if err.Severity >= 5 {
				health.WarningErrors = 1
			}
			
			componentMap[err.Component] = health
		}
	}
	
	totalOperations := len(errors) * 10
	
	for component, health := range componentMap {
		errorCount := componentErrorCounts[component]
		health.ErrorRate = float64(errorCount) / float64(totalOperations) * 100
		
		health.HealthScore = 100
		if health.CriticalErrors > 0 {
			health.HealthScore -= health.CriticalErrors * 20
		}
		if health.WarningErrors > 0 {
			health.HealthScore -= health.WarningErrors * 5
		}
		if health.ErrorRate > 10 {
			health.HealthScore -= int(health.ErrorRate)
		}
		
		if health.HealthScore < 0 {
			health.HealthScore = 0
		}
	}
	
	healthList := make([]ComponentHealth, 0, len(componentMap))
	for _, health := range componentMap {
		healthList = append(healthList, *health)
	}
	
	sort.Slice(healthList, func(i, j int) bool {
		return healthList[i].HealthScore < healthList[j].HealthScore
	})
	
	report.ComponentHealth = healthList
}

func (ea *ErrorAnalytics) calculateHourlyDistribution(errors []RecoverableError, report *AnalyticsReport) {
	for _, err := range errors {
		hour := err.Timestamp.Format("2006-01-02 15:00")
		report.TopErrorsByHour[hour]++
	}
}

func (ea *ErrorAnalytics) calculateRecoveryMetrics(errors []RecoverableError, report *AnalyticsReport) {
	recoveryAttempts := 0
	successfulRecoveries := 0
	totalRecoveryTime := time.Duration(0)
	
	for _, err := range errors {
		if len(err.AttemptedActions) > 0 {
			recoveryAttempts++
			
			// Check if any recovery action was attempted
			if len(err.RecoveryActions) > 0 {
				successfulRecoveries++
				// Use timeout as approximation for recovery time
				if len(err.RecoveryActions) > 0 && err.RecoveryActions[0].Timeout > 0 {
					totalRecoveryTime += err.RecoveryActions[0].Timeout
				}
			}
		}
	}
	
	if recoveryAttempts > 0 {
		report.RecoverySuccessRate = float64(successfulRecoveries) / float64(recoveryAttempts) * 100
	}
	
	if successfulRecoveries > 0 {
		report.AverageRecoveryTime = totalRecoveryTime / time.Duration(successfulRecoveries)
	}
}

func (ea *ErrorAnalytics) generateRecommendations(report *AnalyticsReport) {
	recommendations := make([]string, 0)
	
	if report.CriticalErrors > 10 {
		recommendations = append(recommendations, "High number of critical errors detected. Consider implementing additional error prevention measures.")
	}
	
	if report.RecoverySuccessRate < 70 {
		recommendations = append(recommendations, "Low recovery success rate. Review and improve error recovery strategies.")
	}
	
	if report.AverageRecoveryTime > 5*time.Second {
		recommendations = append(recommendations, "Recovery times are high. Consider optimizing recovery procedures.")
	}
	
	for _, health := range report.ComponentHealth {
		if health.HealthScore < 60 {
			recommendations = append(recommendations, fmt.Sprintf("Component '%s' has poor health (score: %d). Requires immediate attention.", health.Component, health.HealthScore))
		}
	}
	
	if len(report.ErrorPatterns) > 0 && report.ErrorPatterns[0].Frequency > 20 {
		recommendations = append(recommendations, fmt.Sprintf("Frequent error pattern detected: '%s'. Consider implementing specific prevention measures.", report.ErrorPatterns[0].Pattern))
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "System health appears stable. Continue monitoring for any emerging patterns.")
	}
	
	report.Recommendations = recommendations
}

func (ea *ErrorAnalytics) extractErrorPattern(err RecoverableError) string {
	if err.OriginalError != nil {
		errorMsg := err.OriginalError.Error()
		if len(errorMsg) > 50 {
			return errorMsg[:50] + "..."
		}
		return errorMsg
	}
	
	return fmt.Sprintf("%s:%s", err.Component, err.Operation)
}

func (ea *ErrorAnalytics) ExportReport(report *AnalyticsReport, format string) (string, error) {
	switch format {
	case "json":
		return ea.exportJSON(report)
	case "csv":
		return ea.exportCSV(report)
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

func (ea *ErrorAnalytics) exportJSON(report *AnalyticsReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}
	
	filename := filepath.Join(ea.dataDir, fmt.Sprintf("error_report_%s.json", time.Now().Format("2006-01-02_15-04-05")))
	
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write report file: %w", err)
	}
	
	return filename, nil
}

func (ea *ErrorAnalytics) exportCSV(report *AnalyticsReport) (string, error) {
	filename := filepath.Join(ea.dataDir, fmt.Sprintf("error_report_%s.csv", time.Now().Format("2006-01-02_15-04-05")))
	
	file, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()
	
	file.WriteString("Component,Error Type,Frequency,Severity,First Seen,Last Seen\n")
	
	for _, pattern := range report.ErrorPatterns {
		file.WriteString(fmt.Sprintf("%s,%s,%d,%d,%s,%s\n",
			pattern.Component,
			pattern.Pattern,
			pattern.Frequency,
			pattern.Severity,
			pattern.FirstSeen.Format("2006-01-02 15:04:05"),
			pattern.LastSeen.Format("2006-01-02 15:04:05"),
		))
	}
	
	return filename, nil
}

func (ea *ErrorAnalytics) persistError(err RecoverableError) {
	if !ea.reportingEnabled {
		return
	}
	
	filename := filepath.Join(ea.dataDir, fmt.Sprintf("errors_%s.json", time.Now().Format("2006-01-02")))
	
	file, openErr := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		return
	}
	defer file.Close()
	
	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		return
	}
	
	file.Write(data)
	file.WriteString("\n")
}

func (ea *ErrorAnalytics) loadPersistedErrors() {
	if !ea.reportingEnabled {
		return
	}
	
	files, err := filepath.Glob(filepath.Join(ea.dataDir, "errors_*.json"))
	if err != nil {
		return
	}
	
	for _, filename := range files {
		ea.loadErrorsFromFile(filename)
	}
	
	sort.Slice(ea.errors, func(i, j int) bool {
		return ea.errors[i].Timestamp.Before(ea.errors[j].Timestamp)
	})
}

func (ea *ErrorAnalytics) loadErrorsFromFile(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}
	
	lines := string(data)
	for _, line := range strings.Split(lines, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		var err RecoverableError
		if json.Unmarshal([]byte(line), &err) == nil {
			ea.errors = append(ea.errors, err)
		}
	}
}

func (ea *ErrorAnalytics) cleanupOldErrors() {
	cutoff := time.Now().AddDate(0, 0, -ea.retentionDays)
	
	newErrors := make([]RecoverableError, 0)
	for _, err := range ea.errors {
		if err.Timestamp.After(cutoff) {
			newErrors = append(newErrors, err)
		}
	}
	
	ea.errors = newErrors
	
	files, globErr := filepath.Glob(filepath.Join(ea.dataDir, "errors_*.json"))
	if globErr != nil {
		return
	}
	
	for _, filename := range files {
		info, statErr := os.Stat(filename)
		if statErr != nil {
			continue
		}
		
		if info.ModTime().Before(cutoff) {
			os.Remove(filename)
		}
	}
}

func (ea *ErrorAnalytics) GetHealthSummary() map[string]interface{} {
	ea.mutex.RLock()
	defer ea.mutex.RUnlock()
	
	recentErrors := 0
	criticalErrors := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	
	for _, err := range ea.errors {
		if err.Timestamp.After(cutoff) {
			recentErrors++
			if err.Severity >= 8 {
				criticalErrors++
			}
		}
	}
	
	return map[string]interface{}{
		"total_errors":      len(ea.errors),
		"recent_errors_24h": recentErrors,
		"critical_errors":   criticalErrors,
		"data_retention":    ea.retentionDays,
		"reporting_enabled": ea.reportingEnabled,
	}
}