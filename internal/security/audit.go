package security

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AuditLevel defines the severity level of audit events
type AuditLevel string

const (
	AuditLevelInfo     AuditLevel = "INFO"
	AuditLevelWarning  AuditLevel = "WARNING"
	AuditLevelError    AuditLevel = "ERROR"
	AuditLevelCritical AuditLevel = "CRITICAL"
	AuditLevelDebug    AuditLevel = "DEBUG"
)

// AuditEventType defines the type of audit event
type AuditEventType string

const (
	EventTypeAuthentication   AuditEventType = "AUTHENTICATION"
	EventTypeAuthorization    AuditEventType = "AUTHORIZATION"
	EventTypeDataAccess       AuditEventType = "DATA_ACCESS"
	EventTypeDataModification AuditEventType = "DATA_MODIFICATION"
	EventTypeSystemAccess     AuditEventType = "SYSTEM_ACCESS"
	EventTypeSecurityViolation AuditEventType = "SECURITY_VIOLATION"
	EventTypeToolExecution    AuditEventType = "TOOL_EXECUTION"
	EventTypeFileOperation    AuditEventType = "FILE_OPERATION"
	EventTypeNetworkAccess    AuditEventType = "NETWORK_ACCESS"
	EventTypeConfigChange     AuditEventType = "CONFIG_CHANGE"
	EventTypeError            AuditEventType = "ERROR"
	EventTypePerformance      AuditEventType = "PERFORMANCE"
)

// AuditEvent represents a security audit event
type AuditEvent struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	Level         AuditLevel             `json:"level"`
	EventType     AuditEventType         `json:"event_type"`
	Source        string                 `json:"source"`
	User          string                 `json:"user,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	IPAddress     string                 `json:"ip_address,omitempty"`
	UserAgent     string                 `json:"user_agent,omitempty"`
	Resource      string                 `json:"resource,omitempty"`
	Action        string                 `json:"action"`
	Result        string                 `json:"result"`
	Message       string                 `json:"message"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Duration      time.Duration          `json:"duration,omitempty"`
	ErrorCode     string                 `json:"error_code,omitempty"`
	Stack         string                 `json:"stack,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Checksum      string                 `json:"checksum"`
}

// AuditConfig configures the audit logging system
type AuditConfig struct {
	LogDirectory     string        `json:"log_directory"`
	MaxFileSize      int64         `json:"max_file_size"`      // bytes
	MaxFiles         int           `json:"max_files"`
	RotationInterval time.Duration `json:"rotation_interval"`
	CompressionEnabled bool        `json:"compression_enabled"`
	EncryptionEnabled bool         `json:"encryption_enabled"`
	RemoteLogging     bool          `json:"remote_logging"`
	RemoteEndpoint    string        `json:"remote_endpoint,omitempty"`
	BufferSize       int           `json:"buffer_size"`
	FlushInterval    time.Duration `json:"flush_interval"`
	MinLevel         AuditLevel    `json:"min_level"`
	EnableIntegrity  bool          `json:"enable_integrity"`
	IncludeStackTrace bool         `json:"include_stack_trace"`
}

// AuditFilter defines filtering criteria for audit events
type AuditFilter struct {
	StartTime  *time.Time       `json:"start_time,omitempty"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	Levels     []AuditLevel     `json:"levels,omitempty"`
	EventTypes []AuditEventType `json:"event_types,omitempty"`
	Sources    []string         `json:"sources,omitempty"`
	Users      []string         `json:"users,omitempty"`
	Resources  []string         `json:"resources,omitempty"`
	Actions    []string         `json:"actions,omitempty"`
	Results    []string         `json:"results,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
	Limit      int              `json:"limit,omitempty"`
	Offset     int              `json:"offset,omitempty"`
}

// AuditStatistics provides audit log statistics
type AuditStatistics struct {
	TotalEvents       int64                    `json:"total_events"`
	EventsByLevel     map[AuditLevel]int64     `json:"events_by_level"`
	EventsByType      map[AuditEventType]int64 `json:"events_by_type"`
	EventsBySource    map[string]int64         `json:"events_by_source"`
	EventsByUser      map[string]int64         `json:"events_by_user"`
	EventsByHour      map[string]int64         `json:"events_by_hour"`
	EventsByDay       map[string]int64         `json:"events_by_day"`
	SecurityViolations int64                    `json:"security_violations"`
	FailedOperations  int64                    `json:"failed_operations"`
	AverageEventSize  int64                    `json:"average_event_size"`
	LogFilesCount     int                      `json:"log_files_count"`
	TotalLogSize      int64                    `json:"total_log_size"`
	GeneratedAt       time.Time                `json:"generated_at"`
}

// SecurityAuditor provides comprehensive audit logging functionality
type SecurityAuditor struct {
	config       AuditConfig
	currentFile  *os.File
	currentSize  int64
	buffer       chan *AuditEvent
	mu           sync.RWMutex
	wg           sync.WaitGroup
	stopChan     chan struct{}
	eventCounter int64
	statsCache   *AuditStatistics
	statsMu      sync.RWMutex
	lastStatsUpdate time.Time
}

// NewSecurityAuditor creates a new security auditor with the given configuration
func NewSecurityAuditor(config AuditConfig) (*SecurityAuditor, error) {
	if config.LogDirectory == "" {
		config.LogDirectory = "./logs/audit"
	}
	
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = 100 * 1024 * 1024 // 100MB
	}
	
	if config.MaxFiles <= 0 {
		config.MaxFiles = 10
	}
	
	if config.RotationInterval <= 0 {
		config.RotationInterval = 24 * time.Hour
	}
	
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}
	
	if config.MinLevel == "" {
		config.MinLevel = AuditLevelInfo
	}

	// Create log directory
	if err := os.MkdirAll(config.LogDirectory, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	auditor := &SecurityAuditor{
		config:   config,
		buffer:   make(chan *AuditEvent, config.BufferSize),
		stopChan: make(chan struct{}),
	}

	// Open initial log file
	if err := auditor.rotateLogFile(); err != nil {
		return nil, fmt.Errorf("failed to open initial log file: %w", err)
	}

	// Start background workers
	auditor.wg.Add(2)
	go auditor.eventWriter()
	go auditor.periodicRotation()

	return auditor, nil
}

// LogEvent logs a security audit event
func (sa *SecurityAuditor) LogEvent(event *AuditEvent) {
	// Check if event level meets minimum threshold
	if !sa.shouldLogLevel(event.Level) {
		return
	}

	// Set event metadata
	if event.ID == "" {
		sa.mu.Lock()
		sa.eventCounter++
		event.ID = fmt.Sprintf("%d-%d", time.Now().Unix(), sa.eventCounter)
		sa.mu.Unlock()
	}
	
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Calculate event checksum for integrity
	if sa.config.EnableIntegrity {
		event.Checksum = sa.calculateChecksum(event)
	}

	// Try to send to buffer (non-blocking)
	select {
	case sa.buffer <- event:
	default:
		// Buffer is full, log this issue
		fmt.Printf("Audit buffer full, dropping event: %s\n", event.ID)
	}
}

// LogAuthentication logs authentication events
func (sa *SecurityAuditor) LogAuthentication(user, sessionID, ipAddress string, success bool, details map[string]interface{}) {
	result := "SUCCESS"
	level := AuditLevelInfo
	if !success {
		result = "FAILURE"
		level = AuditLevelWarning
	}

	event := &AuditEvent{
		Level:     level,
		EventType: EventTypeAuthentication,
		Source:    "authentication_system",
		User:      user,
		SessionID: sessionID,
		IPAddress: ipAddress,
		Action:    "LOGIN_ATTEMPT",
		Result:    result,
		Message:   fmt.Sprintf("User %s authentication %s", user, strings.ToLower(result)),
		Details:   details,
	}

	sa.LogEvent(event)
}

// LogAuthorization logs authorization events
func (sa *SecurityAuditor) LogAuthorization(user, resource, action string, allowed bool, details map[string]interface{}) {
	result := "ALLOWED"
	level := AuditLevelInfo
	if !allowed {
		result = "DENIED"
		level = AuditLevelWarning
	}

	event := &AuditEvent{
		Level:     level,
		EventType: EventTypeAuthorization,
		Source:    "authorization_system",
		User:      user,
		Resource:  resource,
		Action:    action,
		Result:    result,
		Message:   fmt.Sprintf("Authorization %s for user %s accessing %s", strings.ToLower(result), user, resource),
		Details:   details,
	}

	sa.LogEvent(event)
}

// LogSecurityViolation logs security violation events
func (sa *SecurityAuditor) LogSecurityViolation(violationType, source, details string, metadata map[string]interface{}) {
	event := &AuditEvent{
		Level:     AuditLevelCritical,
		EventType: EventTypeSecurityViolation,
		Source:    source,
		Action:    violationType,
		Result:    "VIOLATION_DETECTED",
		Message:   details,
		Details:   metadata,
		Tags:      []string{"security_violation", violationType},
	}

	sa.LogEvent(event)
}

// LogToolExecution logs tool execution events
func (sa *SecurityAuditor) LogToolExecution(tool, user, command string, duration time.Duration, success bool, output string, metadata map[string]interface{}) {
	result := "SUCCESS"
	level := AuditLevelInfo
	if !success {
		result = "FAILURE"
		level = AuditLevelError
	}

	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["command"] = command
	metadata["output_length"] = len(output)
	metadata["execution_time_ms"] = duration.Milliseconds()

	event := &AuditEvent{
		Level:     level,
		EventType: EventTypeToolExecution,
		Source:    "tool_executor",
		User:      user,
		Resource:  tool,
		Action:    "EXECUTE_TOOL",
		Result:    result,
		Duration:  duration,
		Message:   fmt.Sprintf("Tool %s executed by %s: %s", tool, user, result),
		Details:   metadata,
		Tags:      []string{"tool_execution", tool},
	}

	sa.LogEvent(event)
}

// LogFileOperation logs file operation events
func (sa *SecurityAuditor) LogFileOperation(operation, filePath, user string, success bool, metadata map[string]interface{}) {
	result := "SUCCESS"
	level := AuditLevelInfo
	if !success {
		result = "FAILURE"
		level = AuditLevelWarning
	}

	event := &AuditEvent{
		Level:     level,
		EventType: EventTypeFileOperation,
		Source:    "file_system",
		User:      user,
		Resource:  filePath,
		Action:    operation,
		Result:    result,
		Message:   fmt.Sprintf("File operation %s on %s by %s: %s", operation, filePath, user, result),
		Details:   metadata,
		Tags:      []string{"file_operation", operation},
	}

	sa.LogEvent(event)
}

// SearchEvents searches audit events based on filter criteria
func (sa *SecurityAuditor) SearchEvents(filter AuditFilter) ([]*AuditEvent, error) {
	var events []*AuditEvent
	
	// Get list of log files
	files, err := sa.getLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %w", err)
	}

	// Search through log files
	for _, file := range files {
		fileEvents, err := sa.searchInFile(file, filter)
		if err != nil {
			continue // Skip files with errors
		}
		events = append(events, fileEvents...)
	}

	// Sort by timestamp (newest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(events) {
		events = events[filter.Offset:]
	}
	
	if filter.Limit > 0 && filter.Limit < len(events) {
		events = events[:filter.Limit]
	}

	return events, nil
}

// GetStatistics returns audit log statistics
func (sa *SecurityAuditor) GetStatistics(refresh bool) (*AuditStatistics, error) {
	sa.statsMu.RLock()
	if !refresh && sa.statsCache != nil && time.Since(sa.lastStatsUpdate) < 5*time.Minute {
		defer sa.statsMu.RUnlock()
		return sa.statsCache, nil
	}
	sa.statsMu.RUnlock()

	sa.statsMu.Lock()
	defer sa.statsMu.Unlock()

	stats := &AuditStatistics{
		EventsByLevel:  make(map[AuditLevel]int64),
		EventsByType:   make(map[AuditEventType]int64),
		EventsBySource: make(map[string]int64),
		EventsByUser:   make(map[string]int64),
		EventsByHour:   make(map[string]int64),
		EventsByDay:    make(map[string]int64),
		GeneratedAt:    time.Now(),
	}

	// Get log files
	files, err := sa.getLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %w", err)
	}

	stats.LogFilesCount = len(files)
	var totalSize int64

	// Process each log file
	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			continue
		}
		
		totalSize += fileInfo.Size()
		
		if err := sa.processFileForStats(file, stats); err != nil {
			continue
		}
	}

	stats.TotalLogSize = totalSize
	if stats.TotalEvents > 0 {
		stats.AverageEventSize = totalSize / stats.TotalEvents
	}

	sa.statsCache = stats
	sa.lastStatsUpdate = time.Now()

	return stats, nil
}

// ExportEvents exports audit events to a specified format
func (sa *SecurityAuditor) ExportEvents(filter AuditFilter, format string, output io.Writer) error {
	events, err := sa.SearchEvents(filter)
	if err != nil {
		return fmt.Errorf("failed to search events: %w", err)
	}

	switch strings.ToUpper(format) {
	case "JSON":
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(events)
		
	case "CSV":
		return sa.exportToCSV(events, output)
		
	case "TEXT":
		return sa.exportToText(events, output)
		
	default:
		return fmt.Errorf("unsupported export format: %s", format)
	}
}

// ValidateIntegrity validates the integrity of audit logs
func (sa *SecurityAuditor) ValidateIntegrity() (*map[string]bool, error) {
	if !sa.config.EnableIntegrity {
		return nil, fmt.Errorf("integrity checking is not enabled")
	}

	results := make(map[string]bool)
	
	files, err := sa.getLogFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to get log files: %w", err)
	}

	for _, file := range files {
		valid, err := sa.validateFileIntegrity(file)
		if err != nil {
			results[file] = false
		} else {
			results[file] = valid
		}
	}

	return &results, nil
}

// Close gracefully shuts down the auditor
func (sa *SecurityAuditor) Close() error {
	close(sa.stopChan)
	sa.wg.Wait()
	
	sa.mu.Lock()
	defer sa.mu.Unlock()
	
	if sa.currentFile != nil {
		return sa.currentFile.Close()
	}
	
	return nil
}

// Private helper methods

func (sa *SecurityAuditor) eventWriter() {
	defer sa.wg.Done()
	
	ticker := time.NewTicker(sa.config.FlushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case event := <-sa.buffer:
			sa.writeEvent(event)
			
		case <-ticker.C:
			sa.flushBuffer()
			
		case <-sa.stopChan:
			sa.flushBuffer()
			return
		}
	}
}

func (sa *SecurityAuditor) periodicRotation() {
	defer sa.wg.Done()
	
	ticker := time.NewTicker(sa.config.RotationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			sa.checkAndRotate()
			
		case <-sa.stopChan:
			return
		}
	}
}

func (sa *SecurityAuditor) writeEvent(event *AuditEvent) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if sa.currentFile == nil {
		return
	}

	// Serialize event
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	data = append(data, '\n')

	// Write to file
	n, err := sa.currentFile.Write(data)
	if err != nil {
		return
	}

	sa.currentSize += int64(n)

	// Check if rotation is needed
	if sa.currentSize >= sa.config.MaxFileSize {
		sa.rotateLogFile()
	}
}

func (sa *SecurityAuditor) flushBuffer() {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	
	if sa.currentFile != nil {
		sa.currentFile.Sync()
	}
}

func (sa *SecurityAuditor) rotateLogFile() error {
	// Close current file
	if sa.currentFile != nil {
		sa.currentFile.Close()
	}

	// Generate new filename
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(sa.config.LogDirectory, fmt.Sprintf("audit_%s.log", timestamp))

	// Open new file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}

	sa.currentFile = file
	sa.currentSize = 0

	// Cleanup old files
	go sa.cleanupOldFiles()

	return nil
}

func (sa *SecurityAuditor) checkAndRotate() {
	sa.mu.RLock()
	needsRotation := sa.currentSize >= sa.config.MaxFileSize
	sa.mu.RUnlock()

	if needsRotation {
		sa.mu.Lock()
		sa.rotateLogFile()
		sa.mu.Unlock()
	}
}

func (sa *SecurityAuditor) cleanupOldFiles() {
	files, err := sa.getLogFiles()
	if err != nil {
		return
	}

	if len(files) <= sa.config.MaxFiles {
		return
	}

	// Sort files by modification time
	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var fileInfos []fileInfo
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, fileInfo{
			path:    file,
			modTime: info.ModTime(),
		})
	}

	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].modTime.Before(fileInfos[j].modTime)
	})

	// Remove oldest files
	for i := 0; i < len(fileInfos)-sa.config.MaxFiles; i++ {
		os.Remove(fileInfos[i].path)
		
		// Also remove compressed version if it exists
		if sa.config.CompressionEnabled {
			os.Remove(fileInfos[i].path + ".gz")
		}
	}
}

func (sa *SecurityAuditor) getLogFiles() ([]string, error) {
	pattern := filepath.Join(sa.config.LogDirectory, "audit_*.log")
	return filepath.Glob(pattern)
}

func (sa *SecurityAuditor) searchInFile(filename string, filter AuditFilter) ([]*AuditEvent, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reader io.Reader = file

	// Check if file is compressed
	if strings.HasSuffix(filename, ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var events []*AuditEvent
	scanner := bufio.NewScanner(reader)
	
	for scanner.Scan() {
		var event AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}

		if sa.matchesFilter(&event, filter) {
			events = append(events, &event)
		}
	}

	return events, nil
}

func (sa *SecurityAuditor) matchesFilter(event *AuditEvent, filter AuditFilter) bool {
	// Time range check
	if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
		return false
	}

	// Level check
	if len(filter.Levels) > 0 {
		found := false
		for _, level := range filter.Levels {
			if event.Level == level {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Event type check
	if len(filter.EventTypes) > 0 {
		found := false
		for _, eventType := range filter.EventTypes {
			if event.EventType == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Source check
	if len(filter.Sources) > 0 {
		found := false
		for _, source := range filter.Sources {
			if event.Source == source {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// User check
	if len(filter.Users) > 0 {
		found := false
		for _, user := range filter.Users {
			if event.User == user {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Resource check
	if len(filter.Resources) > 0 {
		found := false
		for _, resource := range filter.Resources {
			if strings.Contains(event.Resource, resource) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Action check
	if len(filter.Actions) > 0 {
		found := false
		for _, action := range filter.Actions {
			if event.Action == action {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Result check
	if len(filter.Results) > 0 {
		found := false
		for _, result := range filter.Results {
			if event.Result == result {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Tags check
	if len(filter.Tags) > 0 {
		found := false
		for _, filterTag := range filter.Tags {
			for _, eventTag := range event.Tags {
				if eventTag == filterTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (sa *SecurityAuditor) shouldLogLevel(level AuditLevel) bool {
	levelPriority := map[AuditLevel]int{
		AuditLevelDebug:    0,
		AuditLevelInfo:     1,
		AuditLevelWarning:  2,
		AuditLevelError:    3,
		AuditLevelCritical: 4,
	}

	eventPriority := levelPriority[level]
	minPriority := levelPriority[sa.config.MinLevel]

	return eventPriority >= minPriority
}

func (sa *SecurityAuditor) calculateChecksum(event *AuditEvent) string {
	// Create a copy without checksum for calculation
	eventCopy := *event
	eventCopy.Checksum = ""

	data, _ := json.Marshal(eventCopy)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (sa *SecurityAuditor) processFileForStats(filename string, stats *AuditStatistics) error {
	events, err := sa.searchInFile(filename, AuditFilter{})
	if err != nil {
		return err
	}

	for _, event := range events {
		stats.TotalEvents++
		stats.EventsByLevel[event.Level]++
		stats.EventsByType[event.EventType]++
		stats.EventsBySource[event.Source]++
		
		if event.User != "" {
			stats.EventsByUser[event.User]++
		}

		hourKey := event.Timestamp.Format("2006-01-02T15")
		dayKey := event.Timestamp.Format("2006-01-02")
		stats.EventsByHour[hourKey]++
		stats.EventsByDay[dayKey]++

		if event.EventType == EventTypeSecurityViolation {
			stats.SecurityViolations++
		}

		if event.Result == "FAILURE" || event.Result == "ERROR" {
			stats.FailedOperations++
		}
	}

	return nil
}

func (sa *SecurityAuditor) validateFileIntegrity(filename string) (bool, error) {
	events, err := sa.searchInFile(filename, AuditFilter{})
	if err != nil {
		return false, err
	}

	for _, event := range events {
		if event.Checksum == "" {
			continue
		}

		expectedChecksum := sa.calculateChecksum(event)
		if event.Checksum != expectedChecksum {
			return false, nil
		}
	}

	return true, nil
}

func (sa *SecurityAuditor) exportToCSV(events []*AuditEvent, output io.Writer) error {
	// Write CSV header
	header := "ID,Timestamp,Level,EventType,Source,User,Resource,Action,Result,Message\n"
	if _, err := output.Write([]byte(header)); err != nil {
		return err
	}

	// Write events
	for _, event := range events {
		row := fmt.Sprintf("%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\n",
			event.ID,
			event.Timestamp.Format(time.RFC3339),
			event.Level,
			event.EventType,
			event.Source,
			event.User,
			event.Resource,
			event.Action,
			event.Result,
			strconv.Quote(event.Message),
		)
		if _, err := output.Write([]byte(row)); err != nil {
			return err
		}
	}

	return nil
}

func (sa *SecurityAuditor) exportToText(events []*AuditEvent, output io.Writer) error {
	for _, event := range events {
		text := fmt.Sprintf("[%s] %s %s/%s: %s (%s)\n",
			event.Timestamp.Format(time.RFC3339),
			event.Level,
			event.Source,
			event.EventType,
			event.Message,
			event.Result,
		)
		if _, err := output.Write([]byte(text)); err != nil {
			return err
		}
	}

	return nil
}

// NewDefaultAuditConfig returns a secure default audit configuration
func NewDefaultAuditConfig() AuditConfig {
	return AuditConfig{
		LogDirectory:       "./logs/audit",
		MaxFileSize:        100 * 1024 * 1024, // 100MB
		MaxFiles:           10,
		RotationInterval:   24 * time.Hour,
		CompressionEnabled: true,
		EncryptionEnabled:  false,
		RemoteLogging:      false,
		BufferSize:         1000,
		FlushInterval:      5 * time.Second,
		MinLevel:           AuditLevelInfo,
		EnableIntegrity:    true,
		IncludeStackTrace:  false,
	}
}