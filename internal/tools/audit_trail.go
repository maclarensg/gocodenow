package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AuditEntry represents a single entry in the audit trail
type AuditEntry struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Operation   string                 `json:"operation"`
	FilePath    string                 `json:"file_path"`
	Status      string                 `json:"status"` // success, failure, pending
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	ChecksumBefore string              `json:"checksum_before,omitempty"`
	ChecksumAfter  string              `json:"checksum_after,omitempty"`
	SizeBefore     int64               `json:"size_before,omitempty"`
	SizeAfter      int64               `json:"size_after,omitempty"`
	BackupPath     string              `json:"backup_path,omitempty"`
	RelatedFiles   []string            `json:"related_files,omitempty"`
}

// AuditTrail manages the audit trail for file operations
type AuditTrail struct {
	logPath     string
	entries     []AuditEntry
	maxEntries  int
	autoFlush   bool
	sessionID   string
	userID      string
}

// NewAuditTrail creates a new audit trail
func NewAuditTrail(logPath string, maxEntries int, autoFlush bool) *AuditTrail {
	return &AuditTrail{
		logPath:    logPath,
		maxEntries: maxEntries,
		autoFlush:  autoFlush,
		sessionID:  generateSessionID(),
		userID:     getCurrentUserID(),
	}
}

// LogOperation logs a file operation to the audit trail
func (a *AuditTrail) LogOperation(operation, filePath, status string, details map[string]interface{}) error {
	entry := AuditEntry{
		ID:        generateEntryID(),
		Timestamp: time.Now(),
		Operation: operation,
		FilePath:  filePath,
		Status:    status,
		UserID:    a.userID,
		SessionID: a.sessionID,
		Details:   details,
	}

	// Collect file metadata before/after if applicable
	if operation != "delete" && status == "success" {
		if info, err := os.Stat(filePath); err == nil {
			entry.SizeAfter = info.Size()
		}
		if checksum, err := calculateFileSHA256(filePath); err == nil {
			entry.ChecksumAfter = checksum
		}
	}

	a.entries = append(a.entries, entry)

	// Maintain maximum entries limit
	if len(a.entries) > a.maxEntries {
		a.entries = a.entries[1:]
	}

	if a.autoFlush {
		return a.Flush()
	}

	return nil
}

// StartOperation begins tracking an operation and returns an entry ID
func (a *AuditTrail) StartOperation(operation, filePath string, details map[string]interface{}) (string, error) {
	entryID := generateEntryID()
	
	entry := AuditEntry{
		ID:        entryID,
		Timestamp: time.Now(),
		Operation: operation,
		FilePath:  filePath,
		Status:    "pending",
		UserID:    a.userID,
		SessionID: a.sessionID,
		Details:   details,
	}

	// Collect "before" metadata
	if operation != "create" {
		if info, err := os.Stat(filePath); err == nil {
			entry.SizeBefore = info.Size()
		}
		if checksum, err := calculateFileSHA256(filePath); err == nil {
			entry.ChecksumBefore = checksum
		}
	}

	a.entries = append(a.entries, entry)

	if a.autoFlush {
		a.Flush()
	}

	return entryID, nil
}

// CompleteOperation completes a tracked operation
func (a *AuditTrail) CompleteOperation(entryID, status string, errorMsg string, additionalDetails map[string]interface{}) error {
	// Find the entry
	for i, entry := range a.entries {
		if entry.ID == entryID {
			startTime := entry.Timestamp
			
			a.entries[i].Status = status
			a.entries[i].Duration = time.Since(startTime)
			
			if errorMsg != "" {
				a.entries[i].Error = errorMsg
			}

			// Merge additional details
			if additionalDetails != nil {
				if a.entries[i].Details == nil {
					a.entries[i].Details = make(map[string]interface{})
				}
				for k, v := range additionalDetails {
					a.entries[i].Details[k] = v
				}
			}

			// Collect "after" metadata on success
			if status == "success" && a.entries[i].Operation != "delete" {
				if info, err := os.Stat(a.entries[i].FilePath); err == nil {
					a.entries[i].SizeAfter = info.Size()
				}
				if checksum, err := calculateFileSHA256(a.entries[i].FilePath); err == nil {
					a.entries[i].ChecksumAfter = checksum
				}
			}

			if a.autoFlush {
				return a.Flush()
			}
			return nil
		}
	}

	return fmt.Errorf("entry with ID %s not found", entryID)
}

// GetEntries returns all audit entries, optionally filtered
func (a *AuditTrail) GetEntries(filters map[string]interface{}) []AuditEntry {
	var filtered []AuditEntry

	for _, entry := range a.entries {
		match := true

		if operation, ok := filters["operation"].(string); ok && entry.Operation != operation {
			match = false
		}

		if status, ok := filters["status"].(string); ok && entry.Status != status {
			match = false
		}

		if filePath, ok := filters["file_path"].(string); ok && !strings.Contains(entry.FilePath, filePath) {
			match = false
		}

		if sessionID, ok := filters["session_id"].(string); ok && entry.SessionID != sessionID {
			match = false
		}

		if after, ok := filters["after"].(time.Time); ok && entry.Timestamp.Before(after) {
			match = false
		}

		if before, ok := filters["before"].(time.Time); ok && entry.Timestamp.After(before) {
			match = false
		}

		if match {
			filtered = append(filtered, entry)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	return filtered
}

// Flush writes the audit trail to disk
func (a *AuditTrail) Flush() error {
	if a.logPath == "" {
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(a.logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Load existing entries if file exists
	var allEntries []AuditEntry
	if data, err := os.ReadFile(a.logPath); err == nil {
		json.Unmarshal(data, &allEntries)
	}

	// Merge with current entries (avoid duplicates)
	entryMap := make(map[string]AuditEntry)
	for _, entry := range allEntries {
		entryMap[entry.ID] = entry
	}
	for _, entry := range a.entries {
		entryMap[entry.ID] = entry
	}

	// Convert back to slice
	allEntries = make([]AuditEntry, 0, len(entryMap))
	for _, entry := range entryMap {
		allEntries = append(allEntries, entry)
	}

	// Sort by timestamp
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Maintain size limit
	if len(allEntries) > a.maxEntries {
		allEntries = allEntries[len(allEntries)-a.maxEntries:]
	}

	// Write to file
	data, err := json.MarshalIndent(allEntries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal audit entries: %w", err)
	}

	return os.WriteFile(a.logPath, data, 0644)
}

// Load reads audit trail from disk
func (a *AuditTrail) Load() error {
	if a.logPath == "" {
		return nil
	}

	data, err := os.ReadFile(a.logPath)
	if os.IsNotExist(err) {
		return nil // No existing log file
	}
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	return json.Unmarshal(data, &a.entries)
}

// GetStatistics returns audit trail statistics
func (a *AuditTrail) GetStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"total_entries":      len(a.entries),
		"operations":        make(map[string]int),
		"statuses":          make(map[string]int),
		"files_affected":    make(map[string]int),
		"sessions":          make(map[string]int),
		"users":            make(map[string]int),
		"average_duration":  time.Duration(0),
		"total_duration":    time.Duration(0),
		"success_rate":      0.0,
	}

	if len(a.entries) == 0 {
		return stats
	}

	operations := stats["operations"].(map[string]int)
	statuses := stats["statuses"].(map[string]int)
	filesAffected := stats["files_affected"].(map[string]int)
	sessions := stats["sessions"].(map[string]int)
	users := stats["users"].(map[string]int)

	var totalDuration time.Duration
	successCount := 0

	for _, entry := range a.entries {
		operations[entry.Operation]++
		statuses[entry.Status]++
		filesAffected[entry.FilePath]++
		sessions[entry.SessionID]++
		users[entry.UserID]++

		totalDuration += entry.Duration
		if entry.Status == "success" {
			successCount++
		}
	}

	stats["average_duration"] = totalDuration / time.Duration(len(a.entries))
	stats["total_duration"] = totalDuration
	stats["success_rate"] = float64(successCount) / float64(len(a.entries)) * 100

	return stats
}

// AuditTrailExecutor provides audit trail functionality as a tool
type AuditTrailExecutor struct {
	auditTrail *AuditTrail
}

// NewAuditTrailExecutor creates a new audit trail executor
func NewAuditTrailExecutor(logPath string) *AuditTrailExecutor {
	trail := NewAuditTrail(logPath, 10000, true)
	trail.Load() // Load existing entries
	
	return &AuditTrailExecutor{
		auditTrail: trail,
	}
}

// Execute handles audit trail operations
func (a *AuditTrailExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "operation parameter is required",
		}, nil
	}

	switch operation {
	case "query":
		return a.queryAuditTrail(params)
	case "statistics":
		return a.getStatistics(params)
	case "export":
		return a.exportAuditTrail(params)
	case "cleanup":
		return a.cleanupAuditTrail(params)
	default:
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Unknown operation: %s", operation),
		}, nil
	}
}

// queryAuditTrail queries the audit trail with filters
func (a *AuditTrailExecutor) queryAuditTrail(params ToolParameters) (*ToolResult, error) {
	filters := make(map[string]interface{})

	// Extract filter parameters
	if op, ok := params["filter_operation"].(string); ok {
		filters["operation"] = op
	}
	if status, ok := params["filter_status"].(string); ok {
		filters["status"] = status
	}
	if filePath, ok := params["filter_file_path"].(string); ok {
		filters["file_path"] = filePath
	}
	if sessionID, ok := params["filter_session_id"].(string); ok {
		filters["session_id"] = sessionID
	}

	// Time range filters
	if afterStr, ok := params["after"].(string); ok {
		if after, err := time.Parse(time.RFC3339, afterStr); err == nil {
			filters["after"] = after
		}
	}
	if beforeStr, ok := params["before"].(string); ok {
		if before, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			filters["before"] = before
		}
	}

	entries := a.auditTrail.GetEntries(filters)

	// Limit results
	limit := 100
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}

	// Format results
	format, _ := params["format"].(string)
	if format == "" {
		format = "detailed"
	}

	var result interface{}
	switch format {
	case "summary":
		result = a.formatSummary(entries)
	case "detailed":
		result = entries
	case "timeline":
		result = a.formatTimeline(entries)
	default:
		result = entries
	}

	return &ToolResult{
		Success: true,
		Result:  result,
		Metadata: map[string]interface{}{
			"total_entries": len(entries),
			"filters_applied": filters,
			"format":        format,
			"queried_at":    time.Now().Format(time.RFC3339),
		},
	}, nil
}

// getStatistics returns audit trail statistics
func (a *AuditTrailExecutor) getStatistics(params ToolParameters) (*ToolResult, error) {
	stats := a.auditTrail.GetStatistics()

	return &ToolResult{
		Success: true,
		Result:  stats,
		Metadata: map[string]interface{}{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// exportAuditTrail exports audit trail to different formats
func (a *AuditTrailExecutor) exportAuditTrail(params ToolParameters) (*ToolResult, error) {
	format, _ := params["format"].(string)
	if format == "" {
		format = "json"
	}

	outputFile, _ := params["output_file"].(string)
	if outputFile == "" {
		outputFile = fmt.Sprintf("audit_export_%s.%s", time.Now().Format("20060102_150405"), format)
	}

	entries := a.auditTrail.GetEntries(make(map[string]interface{}))

	var data []byte
	var err error

	switch format {
	case "json":
		data, err = json.MarshalIndent(entries, "", "  ")
	case "csv":
		data, err = a.exportToCSV(entries)
	default:
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Unsupported export format: %s", format),
		}, nil
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to format data: %v", err),
		}, nil
	}

	err = os.WriteFile(outputFile, data, 0644)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to write export file: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Result:  fmt.Sprintf("Exported %d entries to %s", len(entries), outputFile),
		Metadata: map[string]interface{}{
			"output_file":   outputFile,
			"format":       format,
			"entries_count": len(entries),
			"exported_at":  time.Now().Format(time.RFC3339),
		},
	}, nil
}

// cleanupAuditTrail performs cleanup operations
func (a *AuditTrailExecutor) cleanupAuditTrail(params ToolParameters) (*ToolResult, error) {
	olderThanDays, _ := params["older_than_days"].(float64)
	if olderThanDays <= 0 {
		olderThanDays = 30
	}

	cutoffTime := time.Now().AddDate(0, 0, -int(olderThanDays))
	
	originalCount := len(a.auditTrail.entries)
	var keptEntries []AuditEntry

	for _, entry := range a.auditTrail.entries {
		if entry.Timestamp.After(cutoffTime) {
			keptEntries = append(keptEntries, entry)
		}
	}

	a.auditTrail.entries = keptEntries
	removedCount := originalCount - len(keptEntries)

	// Flush to persist changes
	err := a.auditTrail.Flush()
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to persist cleanup: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Result:  fmt.Sprintf("Removed %d entries older than %d days", removedCount, int(olderThanDays)),
		Metadata: map[string]interface{}{
			"original_count":   originalCount,
			"remaining_count":  len(keptEntries),
			"removed_count":    removedCount,
			"cutoff_date":      cutoffTime.Format(time.RFC3339),
			"cleaned_at":       time.Now().Format(time.RFC3339),
		},
	}, nil
}

// formatSummary formats entries as summary view
func (a *AuditTrailExecutor) formatSummary(entries []AuditEntry) []map[string]interface{} {
	var summary []map[string]interface{}

	for _, entry := range entries {
		item := map[string]interface{}{
			"timestamp": entry.Timestamp.Format("2006-01-02 15:04:05"),
			"operation": entry.Operation,
			"file":      filepath.Base(entry.FilePath),
			"status":    entry.Status,
		}

		if entry.Duration > 0 {
			item["duration"] = entry.Duration.String()
		}

		if entry.Error != "" {
			item["error"] = entry.Error
		}

		summary = append(summary, item)
	}

	return summary
}

// formatTimeline formats entries as timeline view
func (a *AuditTrailExecutor) formatTimeline(entries []AuditEntry) map[string][]map[string]interface{} {
	timeline := make(map[string][]map[string]interface{})

	for _, entry := range entries {
		date := entry.Timestamp.Format("2006-01-02")
		
		item := map[string]interface{}{
			"time":      entry.Timestamp.Format("15:04:05"),
			"operation": entry.Operation,
			"file":      entry.FilePath,
			"status":    entry.Status,
		}

		timeline[date] = append(timeline[date], item)
	}

	return timeline
}

// exportToCSV exports entries to CSV format
func (a *AuditTrailExecutor) exportToCSV(entries []AuditEntry) ([]byte, error) {
	var lines []string
	
	// Header
	header := "Timestamp,Operation,FilePath,Status,UserID,SessionID,Duration,Error,SizeBefore,SizeAfter"
	lines = append(lines, header)

	// Data rows
	for _, entry := range entries {
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%d,%d",
			entry.Timestamp.Format(time.RFC3339),
			entry.Operation,
			entry.FilePath,
			entry.Status,
			entry.UserID,
			entry.SessionID,
			entry.Duration.String(),
			entry.Error,
			entry.SizeBefore,
			entry.SizeAfter,
		)
		lines = append(lines, line)
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// Helper functions

// generateSessionID generates a unique session ID
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// generateEntryID generates a unique entry ID
func generateEntryID() string {
	return fmt.Sprintf("entry_%d", time.Now().UnixNano())
}

// getCurrentUserID gets the current user ID (simplified version)
func getCurrentUserID() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}

// AuditingFileExecutor wraps other file executors to provide auditing
type AuditingFileExecutor struct {
	wrappedExecutor ToolExecutor
	auditTrail     *AuditTrail
	operationName  string
}

// NewAuditingFileExecutor creates a wrapper that adds auditing to any file executor
func NewAuditingFileExecutor(wrapped ToolExecutor, auditTrail *AuditTrail, operationName string) *AuditingFileExecutor {
	return &AuditingFileExecutor{
		wrappedExecutor: wrapped,
		auditTrail:     auditTrail,
		operationName:  operationName,
	}
}

// Execute wraps the underlying executor with audit logging
func (a *AuditingFileExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Extract file path from parameters (common patterns)
	var filePath string
	if path, ok := params["file_path"].(string); ok {
		filePath = path
	} else if path, ok := params["path"].(string); ok {
		filePath = path
	} else if files, ok := params["files"].([]interface{}); ok && len(files) > 0 {
		if path, ok := files[0].(string); ok {
			filePath = path
		}
	}

	// Start audit entry
	entryID, err := a.auditTrail.StartOperation(a.operationName, filePath, map[string]interface{}{
		"parameters": params,
	})
	if err != nil {
		// Continue even if audit fails
	}

	// Execute the wrapped operation
	result, err := a.wrappedExecutor.Execute(ctx, params)

	// Complete audit entry
	status := "success"
	errorMsg := ""
	if err != nil {
		status = "failure"
		errorMsg = err.Error()
	} else if result != nil && !result.Success {
		status = "failure"
		errorMsg = result.ErrorMessage
	}

	additionalDetails := map[string]interface{}{
		"result_success": result != nil && result.Success,
	}
	if result != nil && result.Metadata != nil {
		additionalDetails["result_metadata"] = result.Metadata
	}

	a.auditTrail.CompleteOperation(entryID, status, errorMsg, additionalDetails)

	return result, err
}

// calculateFileSHA256 calculates the SHA256 checksum of a file
func calculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}