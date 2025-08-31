package recovery

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// SessionState represents the application session state
type SessionState struct {
	SessionID          string                 `json:"session_id"`
	Timestamp          time.Time              `json:"timestamp"`
	PID                int                    `json:"pid"`
	Version            string                 `json:"version"`
	ConversationStates []ConversationState    `json:"conversation_states"`
	UIState            UIState                `json:"ui_state"`
	ActiveTools        []ActiveTool           `json:"active_tools"`
	Configuration      map[string]interface{} `json:"configuration"`
	MemoryUsage        MemoryInfo             `json:"memory_usage"`
	OpenFiles          []string               `json:"open_files"`
	LastActivity       time.Time              `json:"last_activity"`
}

// ConversationState represents the state of a conversation
type ConversationState struct {
	ID               string            `json:"id"`
	Messages         []MessageState    `json:"messages"`
	CurrentInput     string            `json:"current_input"`
	ScrollPosition   int               `json:"scroll_position"`
	IsExpanded       bool              `json:"is_expanded"`
	LastModified     time.Time         `json:"last_modified"`
	ToolExecutions   []ToolExecution   `json:"tool_executions"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// MessageState represents a message in the conversation
type MessageState struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // user, assistant, system
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// ToolExecution represents a tool execution state
type ToolExecution struct {
	ID          string                 `json:"id"`
	ToolName    string                 `json:"tool_name"`
	Parameters  map[string]interface{} `json:"parameters"`
	Status      string                 `json:"status"` // pending, running, completed, failed
	Output      string                 `json:"output"`
	Error       string                 `json:"error,omitempty"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Duration    time.Duration          `json:"duration"`
}

// UIState represents the state of the user interface
type UIState struct {
	CurrentMode       string `json:"current_mode"`
	ViewportHeight    int    `json:"viewport_height"`
	ViewportWidth     int    `json:"viewport_width"`
	SelectedConversation string `json:"selected_conversation"`
	InputFocused      bool   `json:"input_focused"`
	Theme             string `json:"theme"`
	SidebarOpen       bool   `json:"sidebar_open"`
	ConfigPanelOpen   bool   `json:"config_panel_open"`
}

// ActiveTool represents a tool that's currently running
type ActiveTool struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	PID        int                    `json:"pid"`
	StartTime  time.Time              `json:"start_time"`
	Parameters map[string]interface{} `json:"parameters"`
	Status     string                 `json:"status"`
	Output     []byte                 `json:"output"`
}

// MemoryInfo represents memory usage information
type MemoryInfo struct {
	AllocatedBytes   uint64  `json:"allocated_bytes"`
	TotalAllocations uint64  `json:"total_allocations"`
	SystemMemory     uint64  `json:"system_memory"`
	GCPauses         uint64  `json:"gc_pauses"`
	GoRoutines       int     `json:"go_routines"`
	HeapSize         uint64  `json:"heap_size"`
	StackSize        uint64  `json:"stack_size"`
	UsagePercentage  float64 `json:"usage_percentage"`
}

// CrashRecoveryManager handles crash recovery and session restoration
type CrashRecoveryManager struct {
	sessionDir       string
	currentSession   *SessionState
	backupInterval   time.Duration
	maxSessions      int
	shutdownHandlers []func() error
	mutex            sync.RWMutex
	signalChan       chan os.Signal
	isShuttingDown   bool
}

// NewCrashRecoveryManager creates a new crash recovery manager
func NewCrashRecoveryManager(sessionDir string) *CrashRecoveryManager {
	crm := &CrashRecoveryManager{
		sessionDir:       sessionDir,
		backupInterval:   30 * time.Second,
		maxSessions:      10,
		shutdownHandlers: make([]func() error, 0),
		signalChan:       make(chan os.Signal, 1),
	}
	
	// Setup signal handling for graceful shutdown
	signal.Notify(crm.signalChan, syscall.SIGINT, syscall.SIGTERM)
	go crm.handleShutdownSignals()
	
	return crm
}

// StartSession starts a new session or recovers from a previous crash
func (crm *CrashRecoveryManager) StartSession(sessionID, version string) (*SessionState, error) {
	// Create session directory if it doesn't exist
	if err := os.MkdirAll(crm.sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}
	
	// Check for previous crash
	crashedSessions, err := crm.findCrashedSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to check for crashed sessions: %w", err)
	}
	
	var recoveredSession *SessionState
	if len(crashedSessions) > 0 {
		// Attempt to recover from the most recent crashed session
		latest := crm.findLatestSession(crashedSessions)
		recoveredSession, err = crm.recoverSession(latest)
		if err != nil {
			fmt.Printf("Warning: failed to recover session %s: %v\n", latest.SessionID, err)
		}
	}
	
	// Create new session state
	crm.currentSession = &SessionState{
		SessionID:     sessionID,
		Timestamp:     time.Now(),
		PID:           os.Getpid(),
		Version:       version,
		LastActivity:  time.Now(),
		Configuration: make(map[string]interface{}),
		MemoryUsage:   crm.collectMemoryInfo(),
	}
	
	// If we recovered a session, merge the states
	if recoveredSession != nil {
		crm.mergeSessionStates(recoveredSession, crm.currentSession)
	}
	
	// Start periodic backup
	go crm.startPeriodicBackup()
	
	return crm.currentSession, nil
}

// findCrashedSessions finds sessions that didn't shut down cleanly
func (crm *CrashRecoveryManager) findCrashedSessions() ([]*SessionState, error) {
	files, err := os.ReadDir(crm.sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}
	
	var crashedSessions []*SessionState
	
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".session" {
			sessionPath := filepath.Join(crm.sessionDir, file.Name())
			session, err := crm.loadSession(sessionPath)
			if err != nil {
				continue // Skip corrupted sessions
			}
			
			// Check if the process is still running
			if !crm.isProcessRunning(session.PID) {
				// Process is not running, this is a crashed session
				crashedSessions = append(crashedSessions, session)
			}
		}
	}
	
	return crashedSessions, nil
}

// isProcessRunning checks if a process is still running
func (crm *CrashRecoveryManager) isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	
	// On Unix systems, we can check by sending signal 0
	if runtime.GOOS != "windows" {
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
	
	// On Windows, this is more complex, for now assume not running
	return false
}

// findLatestSession finds the most recent session from a list
func (crm *CrashRecoveryManager) findLatestSession(sessions []*SessionState) *SessionState {
	if len(sessions) == 0 {
		return nil
	}
	
	latest := sessions[0]
	for _, session := range sessions[1:] {
		if session.LastActivity.After(latest.LastActivity) {
			latest = session
		}
	}
	
	return latest
}

// recoverSession attempts to recover a crashed session
func (crm *CrashRecoveryManager) recoverSession(session *SessionState) (*SessionState, error) {
	if session == nil {
		return nil, fmt.Errorf("session is nil")
	}
	
	// Validate session integrity
	if err := crm.validateSession(session); err != nil {
		return nil, fmt.Errorf("session validation failed: %w", err)
	}
	
	// Clean up any incomplete operations
	crm.cleanupIncompleteOperations(session)
	
	fmt.Printf("Successfully recovered session %s from %v\n", 
		session.SessionID, session.LastActivity)
	
	return session, nil
}

// validateSession validates the integrity of a session
func (crm *CrashRecoveryManager) validateSession(session *SessionState) error {
	if session.SessionID == "" {
		return fmt.Errorf("session ID is empty")
	}
	
	if session.Timestamp.IsZero() {
		return fmt.Errorf("session timestamp is zero")
	}
	
	// Check if session is too old (more than 7 days)
	if time.Since(session.LastActivity) > 7*24*time.Hour {
		return fmt.Errorf("session is too old: %v", session.LastActivity)
	}
	
	// Validate conversation states
	for i, conv := range session.ConversationStates {
		if conv.ID == "" {
			return fmt.Errorf("conversation %d has empty ID", i)
		}
	}
	
	return nil
}

// cleanupIncompleteOperations cleans up any incomplete operations from a crashed session
func (crm *CrashRecoveryManager) cleanupIncompleteOperations(session *SessionState) {
	// Clean up running tools
	for i := range session.ActiveTools {
		tool := &session.ActiveTools[i]
		if tool.Status == "running" {
			tool.Status = "failed"
			// Try to kill the process if it's still running
			if crm.isProcessRunning(tool.PID) {
				if process, err := os.FindProcess(tool.PID); err == nil {
					process.Kill()
				}
			}
		}
	}
	
	// Clean up incomplete tool executions in conversations
	for convIdx := range session.ConversationStates {
		conv := &session.ConversationStates[convIdx]
		for toolIdx := range conv.ToolExecutions {
			exec := &conv.ToolExecutions[toolIdx]
			if exec.Status == "running" || exec.Status == "pending" {
				exec.Status = "failed"
				exec.Error = "Interrupted by application crash"
				if exec.EndTime == nil {
					now := time.Now()
					exec.EndTime = &now
				}
			}
		}
	}
}

// mergeSessionStates merges a recovered session into the current session
func (crm *CrashRecoveryManager) mergeSessionStates(recovered, current *SessionState) {
	// Merge conversation states
	current.ConversationStates = recovered.ConversationStates
	
	// Merge UI state
	current.UIState = recovered.UIState
	
	// Don't merge active tools as they're likely no longer valid
	// Don't merge configuration as it might have changed
	
	fmt.Printf("Merged %d conversations from recovered session\n", 
		len(recovered.ConversationStates))
}

// UpdateSession updates the current session state
func (crm *CrashRecoveryManager) UpdateSession(updateFunc func(*SessionState)) error {
	crm.mutex.Lock()
	defer crm.mutex.Unlock()
	
	if crm.currentSession == nil {
		return fmt.Errorf("no active session")
	}
	
	updateFunc(crm.currentSession)
	crm.currentSession.LastActivity = time.Now()
	crm.currentSession.MemoryUsage = crm.collectMemoryInfo()
	
	return nil
}

// startPeriodicBackup starts periodic session backup
func (crm *CrashRecoveryManager) startPeriodicBackup() {
	ticker := time.NewTicker(crm.backupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := crm.BackupSession(); err != nil {
				fmt.Printf("Session backup failed: %v\n", err)
			}
		case <-crm.signalChan:
			// Shutdown signal received, exit backup loop
			return
		}
	}
}

// BackupSession backs up the current session state
func (crm *CrashRecoveryManager) BackupSession() error {
	crm.mutex.RLock()
	defer crm.mutex.RUnlock()
	
	if crm.currentSession == nil {
		return fmt.Errorf("no active session to backup")
	}
	
	filename := fmt.Sprintf("%s.session", crm.currentSession.SessionID)
	sessionPath := filepath.Join(crm.sessionDir, filename)
	
	// Update memory usage before backup
	crm.currentSession.MemoryUsage = crm.collectMemoryInfo()
	
	data, err := json.MarshalIndent(crm.currentSession, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	
	// Write to temporary file first, then rename for atomic operation
	tempPath := sessionPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}
	
	if err := os.Rename(tempPath, sessionPath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename session file: %w", err)
	}
	
	return nil
}

// loadSession loads a session from a file
func (crm *CrashRecoveryManager) loadSession(sessionPath string) (*SessionState, error) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}
	
	var session SessionState
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	
	return &session, nil
}

// collectMemoryInfo collects current memory usage information
func (crm *CrashRecoveryManager) collectMemoryInfo() MemoryInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return MemoryInfo{
		AllocatedBytes:   m.Alloc,
		TotalAllocations: m.TotalAlloc,
		SystemMemory:     m.Sys,
		GCPauses:         uint64(m.NumGC),
		GoRoutines:       runtime.NumGoroutine(),
		HeapSize:         m.HeapAlloc,
		StackSize:        m.StackInuse,
		UsagePercentage:  float64(m.Alloc) / float64(m.Sys) * 100,
	}
}

// RegisterShutdownHandler registers a function to be called during shutdown
func (crm *CrashRecoveryManager) RegisterShutdownHandler(handler func() error) {
	crm.mutex.Lock()
	defer crm.mutex.Unlock()
	
	crm.shutdownHandlers = append(crm.shutdownHandlers, handler)
}

// handleShutdownSignals handles shutdown signals for graceful shutdown
func (crm *CrashRecoveryManager) handleShutdownSignals() {
	<-crm.signalChan
	
	fmt.Println("\nReceived shutdown signal, performing graceful shutdown...")
	crm.GracefulShutdown()
	
	os.Exit(0)
}

// GracefulShutdown performs a graceful shutdown
func (crm *CrashRecoveryManager) GracefulShutdown() error {
	crm.mutex.Lock()
	crm.isShuttingDown = true
	crm.mutex.Unlock()
	
	fmt.Println("Starting graceful shutdown...")
	
	// Execute shutdown handlers
	for i, handler := range crm.shutdownHandlers {
		fmt.Printf("Executing shutdown handler %d...\n", i+1)
		if err := handler(); err != nil {
			fmt.Printf("Shutdown handler %d failed: %v\n", i+1, err)
		}
	}
	
	// Final session backup
	if err := crm.BackupSession(); err != nil {
		fmt.Printf("Final session backup failed: %v\n", err)
	}
	
	// Clean up old sessions
	if err := crm.cleanupOldSessions(); err != nil {
		fmt.Printf("Session cleanup failed: %v\n", err)
	}
	
	// Mark session as cleanly shut down
	if crm.currentSession != nil {
		sessionPath := filepath.Join(crm.sessionDir, 
			fmt.Sprintf("%s.session", crm.currentSession.SessionID))
		os.Remove(sessionPath)
	}
	
	fmt.Println("Graceful shutdown completed")
	return nil
}

// cleanupOldSessions removes old session files
func (crm *CrashRecoveryManager) cleanupOldSessions() error {
	files, err := os.ReadDir(crm.sessionDir)
	if err != nil {
		return fmt.Errorf("failed to read session directory: %w", err)
	}
	
	var sessions []*SessionState
	var filePaths []string
	
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".session" {
			sessionPath := filepath.Join(crm.sessionDir, file.Name())
			session, err := crm.loadSession(sessionPath)
			if err != nil {
				// Remove corrupted session files
				os.Remove(sessionPath)
				continue
			}
			sessions = append(sessions, session)
			filePaths = append(filePaths, sessionPath)
		}
	}
	
	// Keep only the most recent sessions
	if len(sessions) > crm.maxSessions {
		// Sort by last activity (oldest first)
		for i := 0; i < len(sessions)-1; i++ {
			for j := i + 1; j < len(sessions); j++ {
				if sessions[i].LastActivity.After(sessions[j].LastActivity) {
					sessions[i], sessions[j] = sessions[j], sessions[i]
					filePaths[i], filePaths[j] = filePaths[j], filePaths[i]
				}
			}
		}
		
		// Remove oldest sessions
		toRemove := len(sessions) - crm.maxSessions
		for i := 0; i < toRemove; i++ {
			os.Remove(filePaths[i])
		}
	}
	
	return nil
}

// GetCurrentSession returns the current session state
func (crm *CrashRecoveryManager) GetCurrentSession() *SessionState {
	crm.mutex.RLock()
	defer crm.mutex.RUnlock()
	
	return crm.currentSession
}

// IsShuttingDown returns whether the application is shutting down
func (crm *CrashRecoveryManager) IsShuttingDown() bool {
	crm.mutex.RLock()
	defer crm.mutex.RUnlock()
	
	return crm.isShuttingDown
}

// GetSessionHistory returns a list of all available sessions
func (crm *CrashRecoveryManager) GetSessionHistory() ([]*SessionState, error) {
	files, err := os.ReadDir(crm.sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}
	
	var sessions []*SessionState
	
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".session" {
			sessionPath := filepath.Join(crm.sessionDir, file.Name())
			session, err := crm.loadSession(sessionPath)
			if err != nil {
				continue // Skip corrupted sessions
			}
			sessions = append(sessions, session)
		}
	}
	
	return sessions, nil
}