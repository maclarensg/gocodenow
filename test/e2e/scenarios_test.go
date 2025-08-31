package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gocodenow/internal/app"
	"gocodenow/internal/config"
	"gocodenow/internal/models"
	"gocodenow/internal/storage"
	"gocodenow/internal/tools"
	"gocodenow/internal/types"
)

// E2ETestSuite represents an end-to-end test environment
type E2ETestSuite struct {
	config         *config.Config
	storageManager types.StorageManager
	toolRouter     *tools.DefaultToolRouter
	conversationMgr *models.ConversationHistory
	tempDir        string
}

// SetupE2ETest creates a complete test environment
func SetupE2ETest(t *testing.T) *E2ETestSuite {
	// Create temporary directory for test data
	tempDir, err := os.MkdirTemp("", "gocodenow_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create test configuration
	cfg := &config.Config{
		Storage: config.StorageConfig{
			DatabasePath: filepath.Join(tempDir, "test.db"),
			CacheSize:    10,
			MaxMemoryMB:  50,
		},
		Security: config.SecurityConfig{
			MaxFileSize: 1024 * 1024, // 1MB
		},
		LLM: config.LLMConfig{
			Endpoint: "http://localhost:8080/v1/chat/completions",
			Model:    "test-model",
			Token:    "test-token",
			Timeout:  30,
		},
	}

	// Initialize storage manager
	storageManager, err := storage.NewManager(&cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage manager: %v", err)
	}

	// Initialize tool router
	toolRouter := tools.NewToolRouter()
	
	// Create permissive security policy for testing
	filePolicy := &tools.FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        1024 * 1024,
	}
	
	// Register common tools
	toolRouter.RegisterExecutor("read_file", tools.NewReadFileExecutor(filePolicy))
	toolRouter.RegisterExecutor("write_file", tools.NewWriteFileExecutor(filePolicy))
	toolRouter.RegisterExecutor("edit_file", tools.NewEditFileExecutor())
	toolRouter.RegisterExecutor("bash", tools.NewBashExecutor())
	toolRouter.RegisterExecutor("echo", tools.NewEchoExecutor())

	// Initialize conversation manager
	conversationMgr := models.NewConversationHistory(storageManager)

	return &E2ETestSuite{
		config:         cfg,
		storageManager: storageManager,
		toolRouter:     toolRouter,
		conversationMgr: conversationMgr,
		tempDir:        tempDir,
	}
}

// Cleanup cleans up the test environment
func (suite *E2ETestSuite) Cleanup() {
	if suite.storageManager != nil {
		suite.storageManager.Shutdown()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// TestCompleteConversationWorkflow tests a complete user conversation workflow
func TestCompleteConversationWorkflow(t *testing.T) {
	suite := SetupE2ETest(t)
	defer suite.Cleanup()

	ctx := context.Background()

	// Simulate user asking to create a file
	userInput := "Create a hello world program in Python"
	
	// Create conversation block
	conversation := &types.ConversationBlock{
		ID:          "e2e-test-001",
		UserInput:   userInput,
		LLMResponse: "",
		Timestamp:   time.Now(),
		Status:      types.StatusPending,
	}

	// Add conversation to history
	_, err := suite.conversationMgr.AddConversation(conversation.UserInput, conversation.LLMResponse, conversation.Status)
	if err != nil {
		t.Fatalf("Failed to add conversation: %v", err)
	}

	// Simulate LLM response with tool calls
	toolCall := &tools.ToolCall{
		ID:       "tool-001",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": filepath.Join(suite.tempDir, "hello.py"),
			"content":   "#!/usr/bin/env python3\nprint(\"Hello, World!\")\n",
		},
		Timestamp: time.Now(),
	}

	// Execute the tool
	result, err := suite.toolRouter.Execute(ctx, toolCall)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Tool execution should succeed: %s", result.ErrorMessage)
	}

	// Update conversation with tool results
	conversation.Status = types.StatusExecuting
	conversation.ToolCalls = []types.ToolCall{
		{
			ID:         toolCall.ID,
			ToolName:   toolCall.ToolName,
			Parameters: toolCall.Parameters,
			Timestamp:  toolCall.Timestamp,
		},
	}
	conversation.ToolResults = []types.ToolResult{
		{
			ID:         result.ID,
			ToolCallID: result.ToolCallID,
			ToolName:   result.ToolName,
			Success:    result.Success,
			Result:     result.Result,
			Duration:   result.Duration,
			Timestamp:  result.Timestamp,
		},
	}

	// Simulate LLM final response
	conversation.LLMResponse = "I've created a Python hello world program for you in `hello.py`. The file contains a simple script that prints 'Hello, World!' to the console."
	conversation.Status = types.StatusCompleted
	conversation.ExecutionTime = time.Since(conversation.Timestamp)

	// Update conversation in storage
	err = suite.conversationMgr.UpdateConversation(conversation)
	if err != nil {
		t.Fatalf("Failed to update conversation: %v", err)
	}

	// Verify the file was created
	filePath := filepath.Join(suite.tempDir, "hello.py")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Python file should have been created")
	}

	// Verify file contents
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read created file: %v", err)
	}

	expectedContent := "#!/usr/bin/env python3\nprint(\"Hello, World!\")\n"
	if string(content) != expectedContent {
		t.Errorf("File content mismatch.\nExpected: %q\nGot: %q", expectedContent, string(content))
	}

	// Verify conversation was stored
	conversations := suite.conversationMgr.GetConversations()
	if len(conversations) != 1 {
		t.Errorf("Expected 1 conversation, got %d", len(conversations))
	}

	if conversations[0].Status != types.StatusCompleted {
		t.Errorf("Conversation status should be completed, got %v", conversations[0].Status)
	}
}

// TestMultiStepWorkflow tests a workflow with multiple tool executions
func TestMultiStepWorkflow(t *testing.T) {
	suite := SetupE2ETest(t)
	defer suite.Cleanup()

	ctx := context.Background()

	// Create conversation for multi-step workflow
	conversation := &types.ConversationBlock{
		ID:        "e2e-multi-001",
		UserInput: "Create a config file and then read it back to verify",
		Timestamp: time.Now(),
		Status:    types.StatusPending,
	}

	_, err := suite.conversationMgr.AddConversation(conversation.UserInput, conversation.LLMResponse, conversation.Status)
	if err != nil {
		t.Fatalf("Failed to add conversation: %v", err)
	}

	configContent := `{
  "app_name": "gocodenow",
  "version": "1.0.0",
  "debug": true
}`

	// Step 1: Write config file
	writeCall := &tools.ToolCall{
		ID:       "write-config",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": filepath.Join(suite.tempDir, "config.json"),
			"content":   configContent,
		},
		Timestamp: time.Now(),
	}

	writeResult, err := suite.toolRouter.Execute(ctx, writeCall)
	if err != nil || !writeResult.Success {
		t.Fatalf("Write operation failed: %v, %s", err, writeResult.ErrorMessage)
	}

	// Step 2: Read config file back
	readCall := &tools.ToolCall{
		ID:       "read-config",
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			"path": filepath.Join(suite.tempDir, "config.json"),
		},
		Timestamp: time.Now(),
	}

	readResult, err := suite.toolRouter.Execute(ctx, readCall)
	if err != nil || !readResult.Success {
		t.Fatalf("Read operation failed: %v, %s", err, readResult.ErrorMessage)
	}

	// Verify content matches
	if readResult.Result.(string) != configContent {
		t.Errorf("Read content doesn't match written content")
	}

	// Update conversation with all tool calls and results
	conversation.ToolCalls = []types.ToolCall{
		{
			ID:         writeCall.ID,
			ToolName:   writeCall.ToolName,
			Parameters: writeCall.Parameters,
			Timestamp:  writeCall.Timestamp,
		},
		{
			ID:         readCall.ID,
			ToolName:   readCall.ToolName,
			Parameters: readCall.Parameters,
			Timestamp:  readCall.Timestamp,
		},
	}

	conversation.ToolResults = []types.ToolResult{
		{
			ID:         writeResult.ID,
			ToolCallID: writeResult.ToolCallID,
			ToolName:   writeResult.ToolName,
			Success:    writeResult.Success,
			Result:     writeResult.Result,
			Duration:   writeResult.Duration,
			Timestamp:  writeResult.Timestamp,
		},
		{
			ID:         readResult.ID,
			ToolCallID: readResult.ToolCallID,
			ToolName:   readResult.ToolName,
			Success:    readResult.Success,
			Result:     readResult.Result,
			Duration:   readResult.Duration,
			Timestamp:  readResult.Timestamp,
		},
	}

	conversation.LLMResponse = "I've successfully created the config.json file and verified its contents. The file contains the JSON configuration with app_name, version, and debug settings as requested."
	conversation.Status = types.StatusCompleted

	err = suite.conversationMgr.UpdateConversation(conversation)
	if err != nil {
		t.Fatalf("Failed to update conversation: %v", err)
	}

	// Verify both operations are recorded
	if len(conversation.ToolCalls) != 2 {
		t.Errorf("Expected 2 tool calls, got %d", len(conversation.ToolCalls))
	}

	if len(conversation.ToolResults) != 2 {
		t.Errorf("Expected 2 tool results, got %d", len(conversation.ToolResults))
	}
}

// TestErrorHandlingWorkflow tests error scenarios in end-to-end workflows
func TestErrorHandlingWorkflow(t *testing.T) {
	suite := SetupE2ETest(t)
	defer suite.Cleanup()

	ctx := context.Background()

	// Create conversation that will encounter an error
	conversation := &types.ConversationBlock{
		ID:        "e2e-error-001",
		UserInput: "Try to read a file that doesn't exist",
		Timestamp: time.Now(),
		Status:    types.StatusPending,
	}

	_, err := suite.conversationMgr.AddConversation(conversation.UserInput, conversation.LLMResponse, conversation.Status)
	if err != nil {
		t.Fatalf("Failed to add conversation: %v", err)
	}

	// Attempt to read non-existent file
	readCall := &tools.ToolCall{
		ID:       "read-nonexistent",
		ToolName: "read_file",
		Parameters: map[string]interface{}{
			"path": filepath.Join(suite.tempDir, "nonexistent.txt"),
		},
		Timestamp: time.Now(),
	}

	result, err := suite.toolRouter.Execute(ctx, readCall)
	if err != nil {
		t.Fatalf("Tool execution should not error: %v", err)
	}

	// Should get a failed result, not an error
	if result.Success {
		t.Error("Reading non-existent file should fail")
	}

	if result.ErrorMessage == "" {
		t.Error("Failed operation should have error message")
	}

	// Update conversation with error result
	conversation.ToolCalls = []types.ToolCall{
		{
			ID:         readCall.ID,
			ToolName:   readCall.ToolName,
			Parameters: readCall.Parameters,
			Timestamp:  readCall.Timestamp,
		},
	}

	conversation.ToolResults = []types.ToolResult{
		{
			ID:           result.ID,
			ToolCallID:   result.ToolCallID,
			ToolName:     result.ToolName,
			Success:      result.Success,
			Result:       result.Result,
			ErrorMessage: result.ErrorMessage,
			Duration:     result.Duration,
			Timestamp:    result.Timestamp,
		},
	}

	conversation.LLMResponse = "I tried to read the file, but it doesn't exist. The error was: " + result.ErrorMessage
	conversation.Status = types.StatusCompleted

	err = suite.conversationMgr.UpdateConversation(conversation)
	if err != nil {
		t.Fatalf("Failed to update conversation: %v", err)
	}

	// Verify error was properly handled and recorded
	if len(conversation.ToolResults) != 1 {
		t.Error("Should have one tool result recorded")
	}

	if conversation.ToolResults[0].Success {
		t.Error("Tool result should indicate failure")
	}

	if conversation.Status != types.StatusCompleted {
		t.Error("Conversation should still be completed despite tool failure")
	}
}

// TestSessionPersistence tests that conversations persist across sessions
func TestSessionPersistence(t *testing.T) {
	suite := SetupE2ETest(t)
	
	// Create and save a conversation
	conversation := &types.ConversationBlock{
		ID:          "persistence-001",
		UserInput:   "Test persistence",
		LLMResponse: "This conversation should persist",
		Timestamp:   time.Now(),
		Status:      types.StatusCompleted,
	}

	_, err := suite.conversationMgr.AddConversation(conversation.UserInput, conversation.LLMResponse, conversation.Status)
	if err != nil {
		t.Fatalf("Failed to add conversation: %v", err)
	}

	// Close current storage manager
	suite.storageManager.Shutdown()

	// Create new storage manager with same database
	newStorageManager, err := storage.NewManager(&suite.config.Storage)
	if err != nil {
		t.Fatalf("Failed to create new storage manager: %v", err)
	}
	defer newStorageManager.Shutdown()

	// Create new conversation manager
	newConversationMgr := models.NewConversationHistory(newStorageManager)

	// Verify conversation persisted
	conversations := newConversationMgr.GetConversations()
	if len(conversations) == 0 {
		t.Error("Conversation should have persisted across sessions")
	}

	found := false
	for _, conv := range conversations {
		if conv.ID == "persistence-001" {
			found = true
			if conv.UserInput != "Test persistence" {
				t.Error("Persisted conversation data is incorrect")
			}
			break
		}
	}

	if !found {
		t.Error("Specific conversation was not found in persisted data")
	}

	// Cleanup
	suite.tempDir = ""  // Prevent double cleanup
	os.RemoveAll(suite.tempDir)
}

// TestConcurrentOperations tests multiple operations happening concurrently
func TestConcurrentOperations(t *testing.T) {
	suite := SetupE2ETest(t)
	defer suite.Cleanup()

	ctx := context.Background()
	numOperations := 5

	// Create multiple conversations concurrently
	results := make(chan error, numOperations)

	for i := 0; i < numOperations; i++ {
		go func(id int) {
			conversation := &types.ConversationBlock{
				ID:          fmt.Sprintf("concurrent-%d", id),
				UserInput:   fmt.Sprintf("Concurrent operation %d", id),
				LLMResponse: fmt.Sprintf("Response %d", id),
				Timestamp:   time.Now(),
				Status:      types.StatusCompleted,
			}

			_, err := suite.conversationMgr.AddConversation(conversation.UserInput, conversation.LLMResponse, conversation.Status)
			results <- err
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numOperations; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent operation %d failed: %v", i, err)
		}
	}

	// Verify all conversations were saved
	conversations := suite.conversationMgr.GetConversations()
	if len(conversations) < numOperations {
		t.Errorf("Expected at least %d conversations, got %d", numOperations, len(conversations))
	}
}

// Helper function to simulate real application startup
func TestApplicationLifecycle(t *testing.T) {
	suite := SetupE2ETest(t)
	defer suite.Cleanup()

	// Simulate creating an app instance
	appInstance := app.New(suite.config)
	if appInstance == nil {
		t.Error("App instance should not be nil")
	}

	// The app initialization should work without errors
	// Note: We can't test the full TUI without a terminal, but we can test initialization
	t.Log("Application lifecycle test completed successfully")
}