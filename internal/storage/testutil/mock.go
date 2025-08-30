package testutil

import (
	"fmt"
	"sync"
	"time"
)

// MockDatabase provides a mock implementation of Database for testing
type MockDatabase struct {
	mu           sync.RWMutex
	conversations map[string]MockConversation
	toolExecutions map[string]MockToolExecution
	fileOperations map[string]MockFileOperation
	version      int
	closed       bool
	shouldError  bool
	errorMessage string
}

// MockConversation represents a conversation in the mock database
type MockConversation struct {
	ID               string
	UserInput        string
	LLMResponse      string
	ModelName        string
	Timestamp        time.Time
	Status           string
	TokenUsageInput  int
	TokenUsageOutput int
	ExecutionTimeMs  int
}

// MockToolExecution represents a tool execution in the mock database
type MockToolExecution struct {
	ID             string
	ConversationID string
	ToolName       string
	Parameters     string
	Result         string
	Success        bool
	DurationMs     int
	ErrorMessage   string
}

// MockFileOperation represents a file operation in the mock database
type MockFileOperation struct {
	ID              string
	ToolExecutionID string
	OperationType   string
	FilePath        string
	OldContent      string
	NewContent      string
	Success         bool
	ErrorMessage    string
}

// NewMockDatabase creates a new mock database
func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		conversations:  make(map[string]MockConversation),
		toolExecutions: make(map[string]MockToolExecution),
		fileOperations: make(map[string]MockFileOperation),
		version:        2, // Default to current schema version
	}
}

// SetError configures the mock to return errors
func (m *MockDatabase) SetError(shouldError bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.shouldError = shouldError
	m.errorMessage = message
}

// Health simulates database health check
func (m *MockDatabase) Health() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.shouldError {
		return fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	if m.closed {
		return fmt.Errorf("database connection closed")
	}
	
	return nil
}

// Close simulates closing the database connection
func (m *MockDatabase) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.closed = true
	return nil
}

// GetVersion returns the mock database version
func (m *MockDatabase) GetVersion() (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.shouldError {
		return 0, fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	return m.version, nil
}

// SetVersion sets the mock database version
func (m *MockDatabase) SetVersion(version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.shouldError {
		return fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	m.version = version
	return nil
}

// AddConversation adds a conversation to the mock database
func (m *MockDatabase) AddConversation(conv MockConversation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.shouldError {
		return fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	m.conversations[conv.ID] = conv
	return nil
}

// GetConversation retrieves a conversation from the mock database
func (m *MockDatabase) GetConversation(id string) (MockConversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.shouldError {
		return MockConversation{}, fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	conv, exists := m.conversations[id]
	if !exists {
		return MockConversation{}, fmt.Errorf("conversation not found: %s", id)
	}
	
	return conv, nil
}

// GetAllConversations retrieves all conversations from the mock database
func (m *MockDatabase) GetAllConversations() ([]MockConversation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.shouldError {
		return nil, fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	var conversations []MockConversation
	for _, conv := range m.conversations {
		conversations = append(conversations, conv)
	}
	
	return conversations, nil
}

// DeleteConversation removes a conversation from the mock database
func (m *MockDatabase) DeleteConversation(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.shouldError {
		return fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	// Simulate cascade delete - remove related tool executions and file operations
	var toolExecIDs []string
	for _, exec := range m.toolExecutions {
		if exec.ConversationID == id {
			toolExecIDs = append(toolExecIDs, exec.ID)
		}
	}
	
	// Remove file operations for these tool executions
	for fileOpID, fileOp := range m.fileOperations {
		for _, execID := range toolExecIDs {
			if fileOp.ToolExecutionID == execID {
				delete(m.fileOperations, fileOpID)
			}
		}
	}
	
	// Remove tool executions
	for _, execID := range toolExecIDs {
		delete(m.toolExecutions, execID)
	}
	
	// Remove conversation
	delete(m.conversations, id)
	return nil
}

// AddToolExecution adds a tool execution to the mock database
func (m *MockDatabase) AddToolExecution(exec MockToolExecution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.shouldError {
		return fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	// Check if conversation exists (foreign key constraint)
	if _, exists := m.conversations[exec.ConversationID]; !exists {
		return fmt.Errorf("foreign key constraint violation: conversation %s not found", exec.ConversationID)
	}
	
	m.toolExecutions[exec.ID] = exec
	return nil
}

// GetToolExecutionsForConversation retrieves tool executions for a conversation
func (m *MockDatabase) GetToolExecutionsForConversation(conversationID string) ([]MockToolExecution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.shouldError {
		return nil, fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	var executions []MockToolExecution
	for _, exec := range m.toolExecutions {
		if exec.ConversationID == conversationID {
			executions = append(executions, exec)
		}
	}
	
	return executions, nil
}

// AddFileOperation adds a file operation to the mock database
func (m *MockDatabase) AddFileOperation(op MockFileOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.shouldError {
		return fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	// Check if tool execution exists (foreign key constraint)
	if _, exists := m.toolExecutions[op.ToolExecutionID]; !exists {
		return fmt.Errorf("foreign key constraint violation: tool execution %s not found", op.ToolExecutionID)
	}
	
	m.fileOperations[op.ID] = op
	return nil
}

// GetFileOperationsForToolExecution retrieves file operations for a tool execution
func (m *MockDatabase) GetFileOperationsForToolExecution(toolExecutionID string) ([]MockFileOperation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if m.shouldError {
		return nil, fmt.Errorf("mock database error: %s", m.errorMessage)
	}
	
	var operations []MockFileOperation
	for _, op := range m.fileOperations {
		if op.ToolExecutionID == toolExecutionID {
			operations = append(operations, op)
		}
	}
	
	return operations, nil
}

// GetStats returns statistics about the mock database
func (m *MockDatabase) GetStats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return map[string]int{
		"conversations":   len(m.conversations),
		"tool_executions": len(m.toolExecutions),
		"file_operations": len(m.fileOperations),
	}
}

// Reset clears all data from the mock database
func (m *MockDatabase) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.conversations = make(map[string]MockConversation)
	m.toolExecutions = make(map[string]MockToolExecution)
	m.fileOperations = make(map[string]MockFileOperation)
	m.version = 2
	m.closed = false
	m.shouldError = false
	m.errorMessage = ""
}

// MockMigrationRunner provides a mock implementation of migration functionality
type MockMigrationRunner struct {
	db            *MockDatabase
	shouldError   bool
	errorMessage  string
	migrations    []int
}

// NewMockMigrationRunner creates a new mock migration runner
func NewMockMigrationRunner(db *MockDatabase) *MockMigrationRunner {
	return &MockMigrationRunner{
		db:         db,
		migrations: []int{1, 2}, // Default applied migrations
	}
}

// SetError configures the mock to return errors
func (m *MockMigrationRunner) SetError(shouldError bool, message string) {
	m.shouldError = shouldError
	m.errorMessage = message
}

// RunMigrations simulates running migrations
func (m *MockMigrationRunner) RunMigrations() error {
	if m.shouldError {
		return fmt.Errorf("mock migration error: %s", m.errorMessage)
	}
	
	// Simulate applying migrations
	currentVersion, _ := m.db.GetVersion()
	if currentVersion < 2 {
		m.db.SetVersion(2)
	}
	
	return nil
}

// RollbackToVersion simulates rolling back to a specific version
func (m *MockMigrationRunner) RollbackToVersion(targetVersion int) error {
	if m.shouldError {
		return fmt.Errorf("mock migration rollback error: %s", m.errorMessage)
	}
	
	if targetVersion < 0 || targetVersion > 2 {
		return fmt.Errorf("invalid target version: %d", targetVersion)
	}
	
	// Simulate rollback by clearing data and setting version
	m.db.Reset()
	m.db.SetVersion(targetVersion)
	
	return nil
}

// GetAppliedMigrations returns the list of applied migrations
func (m *MockMigrationRunner) GetAppliedMigrations() ([]int, error) {
	if m.shouldError {
		return nil, fmt.Errorf("mock migration query error: %s", m.errorMessage)
	}
	
	return m.migrations, nil
}