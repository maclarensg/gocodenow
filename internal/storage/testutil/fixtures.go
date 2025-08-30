package testutil

import (
	"database/sql"
	"fmt"
	"gocodenow/internal/types"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

// TestConversation represents a test conversation for fixtures
type TestConversation struct {
	ID        string
	UserInput string
	LLMResponse string
	ModelName string
	Timestamp time.Time
	Status    string
	TokenUsageInput  int
	TokenUsageOutput int
	ExecutionTimeMs  int
}

// TestToolExecution represents a test tool execution for fixtures
type TestToolExecution struct {
	ID             string
	ConversationID string
	ToolName       string
	Parameters     string
	Result         string
	Success        bool
	DurationMs     int
	ErrorMessage   string
}

// TestFileOperation represents a test file operation for fixtures
type TestFileOperation struct {
	ID               string
	ToolExecutionID  string
	OperationType    string
	FilePath         string
	OldContent       string
	NewContent       string
	Success          bool
	ErrorMessage     string
}

// GenerateTestConversations creates a set of test conversations with realistic data
func GenerateTestConversations(count int) []TestConversation {
	conversations := make([]TestConversation, count)
	
	userInputs := []string{
		"Help me implement a function to parse JSON data",
		"Fix the bug in my authentication middleware",
		"Create a REST API for user management",
		"Optimize this database query for better performance",
		"Write unit tests for the payment processing module",
		"Debug the memory leak in the background worker",
		"Implement caching for the search functionality",
		"Add logging to track user activities",
		"Create a migration script for the new schema",
		"Set up CI/CD pipeline for automated deployment",
	}
	
	llmResponses := []string{
		"I'll help you implement the JSON parsing function. Here's a solution that handles errors gracefully...",
		"I found the issue in your authentication middleware. The problem is with token validation...",
		"I'll create a comprehensive REST API for user management with proper validation and error handling...",
		"Let me analyze your query and suggest optimizations. I can see several areas for improvement...",
		"I'll write comprehensive unit tests for your payment processing module, including edge cases...",
		"I've identified the memory leak source. The issue is in the goroutine management...",
		"I'll implement a multi-level caching strategy for your search functionality...",
		"I'll add structured logging throughout your application with proper correlation IDs...",
		"I'll create a safe migration script with rollback capabilities...",
		"I'll set up a complete CI/CD pipeline with testing, building, and deployment stages...",
	}
	
	models := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-sonnet", "llama-3.1-70b"}
	statuses := []string{"completed", "pending", "executing", "error"}
	
	for i := 0; i < count; i++ {
		timestamp := time.Now().Add(-time.Duration(rand.Intn(72)) * time.Hour)
		
		conversations[i] = TestConversation{
			ID:               fmt.Sprintf("test-conversation-%d", i+1),
			UserInput:        userInputs[rand.Intn(len(userInputs))],
			LLMResponse:      llmResponses[rand.Intn(len(llmResponses))],
			ModelName:        models[rand.Intn(len(models))],
			Timestamp:        timestamp,
			Status:           statuses[rand.Intn(len(statuses))],
			TokenUsageInput:  rand.Intn(1000) + 50,
			TokenUsageOutput: rand.Intn(2000) + 100,
			ExecutionTimeMs:  rand.Intn(5000) + 500,
		}
	}
	
	return conversations
}

// GenerateTestToolExecutions creates test tool executions for given conversation IDs
func GenerateTestToolExecutions(conversationIDs []string, executionsPerConv int) []TestToolExecution {
	var executions []TestToolExecution
	
	toolNames := []string{"read_file", "write_file", "edit_file", "bash_command", "search_files", "list_directory"}
	parameters := []string{
		`{"path": "/src/main.go"}`,
		`{"path": "/src/config.json", "content": "{}"}`,
		`{"path": "/src/utils.js", "changes": [{"line": 10, "content": "fixed"}]}`,
		`{"command": "npm test"}`,
		`{"pattern": "*.js", "query": "function"}`,
		`{"path": "/src"}`,
	}
	
	results := []string{
		"File read successfully",
		"File written successfully",
		"File edited successfully", 
		"Command executed with exit code 0",
		"Found 15 matching files",
		"Listed 23 items",
	}
	
	errors := []string{
		"",
		"Permission denied",
		"File not found",
		"Invalid syntax",
		"Command timeout",
	}
	
	execID := 1
	for _, convID := range conversationIDs {
		for i := 0; i < executionsPerConv; i++ {
			isSuccess := rand.Float32() > 0.2 // 80% success rate
			errorMsg := ""
			if !isSuccess {
				errorMsg = errors[rand.Intn(len(errors)-1)+1] // Exclude empty string
			}
			
			execution := TestToolExecution{
				ID:             fmt.Sprintf("test-tool-exec-%d", execID),
				ConversationID: convID,
				ToolName:       toolNames[rand.Intn(len(toolNames))],
				Parameters:     parameters[rand.Intn(len(parameters))],
				Result:         results[rand.Intn(len(results))],
				Success:        isSuccess,
				DurationMs:     rand.Intn(3000) + 100,
				ErrorMessage:   errorMsg,
			}
			
			executions = append(executions, execution)
			execID++
		}
	}
	
	return executions
}

// GenerateTestFileOperations creates test file operations for given tool execution IDs
func GenerateTestFileOperations(toolExecIDs []string, opsPerExec int) []TestFileOperation {
	var operations []TestFileOperation
	
	operationTypes := []string{"read", "write", "edit", "delete", "create", "move"}
	filePaths := []string{
		"/src/main.go",
		"/src/utils.js", 
		"/src/config.json",
		"/tests/unit.test.js",
		"/docs/README.md",
		"/scripts/deploy.sh",
	}
	
	opID := 1
	for _, execID := range toolExecIDs {
		for i := 0; i < opsPerExec; i++ {
			isSuccess := rand.Float32() > 0.15 // 85% success rate
			errorMsg := ""
			if !isSuccess {
				errorMsg = "Operation failed"
			}
			
			operation := TestFileOperation{
				ID:              fmt.Sprintf("test-file-op-%d", opID),
				ToolExecutionID: execID,
				OperationType:   operationTypes[rand.Intn(len(operationTypes))],
				FilePath:        filePaths[rand.Intn(len(filePaths))],
				OldContent:      "Original content...",
				NewContent:      "Modified content...",
				Success:         isSuccess,
				ErrorMessage:    errorMsg,
			}
			
			operations = append(operations, operation)
			opID++
		}
	}
	
	return operations
}

// InsertTestConversations inserts test conversations into the database
func InsertTestConversations(db *sql.DB, conversations []TestConversation) error {
	query := `INSERT INTO conversations 
		(id, user_input, llm_response, model_name, timestamp, status, token_usage_input, token_usage_output, execution_time_ms) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	for _, conv := range conversations {
		_, err := db.Exec(query, conv.ID, conv.UserInput, conv.LLMResponse, conv.ModelName,
			conv.Timestamp, conv.Status, conv.TokenUsageInput, conv.TokenUsageOutput, conv.ExecutionTimeMs)
		if err != nil {
			return fmt.Errorf("failed to insert conversation %s: %w", conv.ID, err)
		}
	}
	
	return nil
}

// InsertTestToolExecutions inserts test tool executions into the database
func InsertTestToolExecutions(db *sql.DB, executions []TestToolExecution) error {
	query := `INSERT INTO tool_executions 
		(id, conversation_id, tool_name, parameters, result, success, duration_ms, error_message) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	
	for _, exec := range executions {
		_, err := db.Exec(query, exec.ID, exec.ConversationID, exec.ToolName, exec.Parameters,
			exec.Result, exec.Success, exec.DurationMs, exec.ErrorMessage)
		if err != nil {
			return fmt.Errorf("failed to insert tool execution %s: %w", exec.ID, err)
		}
	}
	
	return nil
}

// InsertTestFileOperations inserts test file operations into the database
func InsertTestFileOperations(db *sql.DB, operations []TestFileOperation) error {
	query := `INSERT INTO file_operations 
		(id, tool_execution_id, operation_type, file_path, old_content, new_content, success, error_message) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	
	for _, op := range operations {
		_, err := db.Exec(query, op.ID, op.ToolExecutionID, op.OperationType, op.FilePath,
			op.OldContent, op.NewContent, op.Success, op.ErrorMessage)
		if err != nil {
			return fmt.Errorf("failed to insert file operation %s: %w", op.ID, err)
		}
	}
	
	return nil
}

// CreateTestDataset creates a complete test dataset with conversations, tool executions, and file operations
func CreateTestDataset(db *sql.DB, numConversations int) error {
	// Generate conversations
	conversations := GenerateTestConversations(numConversations)
	if err := InsertTestConversations(db, conversations); err != nil {
		return fmt.Errorf("failed to insert test conversations: %w", err)
	}
	
	// Extract conversation IDs
	var conversationIDs []string
	for _, conv := range conversations {
		conversationIDs = append(conversationIDs, conv.ID)
	}
	
	// Generate tool executions (1-3 per conversation)
	toolExecutions := GenerateTestToolExecutions(conversationIDs, 2)
	if err := InsertTestToolExecutions(db, toolExecutions); err != nil {
		return fmt.Errorf("failed to insert test tool executions: %w", err)
	}
	
	// Extract tool execution IDs
	var toolExecIDs []string
	for _, exec := range toolExecutions {
		toolExecIDs = append(toolExecIDs, exec.ID)
	}
	
	// Generate file operations (0-2 per tool execution)
	fileOperations := GenerateTestFileOperations(toolExecIDs, 1)
	if err := InsertTestFileOperations(db, fileOperations); err != nil {
		return fmt.Errorf("failed to insert test file operations: %w", err)
	}
	
	return nil
}

// CleanupTestData removes all test data from the database
func CleanupTestData(db *sql.DB) error {
	tables := []string{"file_operations", "tool_executions", "conversations"}
	
	for _, table := range tables {
		query := fmt.Sprintf("DELETE FROM %s WHERE id LIKE 'test-%%'", table)
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to cleanup %s: %w", table, err)
		}
	}
	
	return nil
}

// GenerateConversation creates a single test conversation block for testing
func GenerateConversation(id string) *types.ConversationBlock {
	userInputs := []string{
		"Help me implement a function to parse JSON data",
		"Fix the bug in my authentication middleware", 
		"Create a REST API for user management",
		"Optimize this database query for better performance",
		"Write unit tests for the payment processing module",
	}
	
	llmResponses := []string{
		"I'll help you implement the JSON parsing function. Here's a solution that handles errors gracefully...",
		"I found the issue in your authentication middleware. The problem is with token validation...",
		"I'll create a comprehensive REST API for user management with proper validation...",
		"Let me analyze your query and suggest optimizations...",
		"I'll write comprehensive unit tests for your payment processing module...",
	}
	
	modelNames := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-sonnet", "llama-3.1-70b"}
	
	// Use provided ID or generate one if empty
	conversationID := id
	if conversationID == "" {
		conversationID = uuid.New().String()
	}
	
	userInput := userInputs[rand.Intn(len(userInputs))]
	llmResponse := llmResponses[rand.Intn(len(llmResponses))]
	
	return &types.ConversationBlock{
		ID:          conversationID,
		UserInput:   userInput,
		LLMResponse: llmResponse,
		ModelName:   modelNames[rand.Intn(len(modelNames))],
		Timestamp:   time.Now().Add(-time.Duration(rand.Intn(72)) * time.Hour),
		Status:      types.StatusCompleted,
		TokenUsage: types.TokenUsage{
			InputTokens:  rand.Intn(1000) + 50,
			OutputTokens: rand.Intn(2000) + 100,
		},
		ExecutionTime: time.Duration(rand.Intn(5000)+500) * time.Millisecond,
		ToolCalls:     []types.ToolCall{},
		ToolResults:   []types.ToolResult{},
		FileOperations: []types.FileOperation{},
		Expanded:      false,
		
		// Legacy fields for backward compatibility
		User:      userInput,
		Assistant: llmResponse,
		Tools:     []string{},
		Files:     []string{},
	}
}


