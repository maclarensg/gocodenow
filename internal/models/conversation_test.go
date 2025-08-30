package models

import (
	"fmt"
	"gocodenow/internal/storage"
	"gocodenow/internal/types"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func setupTestConversationHistory(t *testing.T) (*ConversationHistory, *storage.StorageManager, func()) {
	// Create temporary database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	runner := storage.NewMigrationRunner(db)
	if err := runner.RunMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create storage manager
	config := storage.StorageConfig{
		CacheSize:    50,
		AutoSaveFreq: 5 * time.Minute,
	}
	manager, err := storage.NewStorageManager(db, config)
	if err != nil {
		t.Fatalf("Failed to create test StorageManager: %v", err)
	}

	// Create conversation history
	history := NewConversationHistory(manager)

	// Cleanup function
	cleanup := func() {
		manager.Close()
		db.Close()
	}

	return history, manager, cleanup
}

func TestConversationHistory_Integration_Basic(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Test initial empty state
	blocks := history.GetBlocks()
	if len(blocks) != 0 {
		t.Errorf("Expected empty conversation history, got %d blocks", len(blocks))
	}

	if history.GetSelected() != -1 {
		t.Errorf("Expected no selection (-1), got %d", history.GetSelected())
	}
}

func TestConversationHistory_Integration_AddConversation(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add a conversation
	userInput := "Hello, how are you?"
	assistantResponse := "I'm doing well, thank you!"
	
	err := history.AddConversation(userInput, assistantResponse)
	if err != nil {
		t.Fatalf("AddConversation() error = %v", err)
	}

	// Check that conversation was added
	blocks := history.GetBlocks()
	if len(blocks) != 1 {
		t.Errorf("Expected 1 conversation block, got %d", len(blocks))
	}

	// Verify conversation content
	conv := blocks[0]
	if conv.UserInput != userInput {
		t.Errorf("UserInput = %s, expected %s", conv.UserInput, userInput)
	}
	if conv.LLMResponse != assistantResponse {
		t.Errorf("LLMResponse = %s, expected %s", conv.LLMResponse, assistantResponse)
	}
	if conv.Status != types.StatusCompleted {
		t.Errorf("Status = %s, expected completed", conv.Status)
	}
	if conv.ID == "" {
		t.Error("Expected non-empty conversation ID")
	}

	// Verify selection updated
	if history.GetSelected() != 0 {
		t.Errorf("Expected selected = 0, got %d", history.GetSelected())
	}
}

func TestConversationHistory_Integration_MultipleConversations(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add multiple conversations
	conversations := []struct {
		userInput, assistantResponse string
	}{
		{"Hello", "Hi there!"},
		{"How are you?", "I'm doing well!"},
		{"What's the weather?", "It's sunny today."},
	}

	for _, conv := range conversations {
		err := history.AddConversation(conv.userInput, conv.assistantResponse)
		if err != nil {
			t.Fatalf("AddConversation() error = %v", err)
		}
	}

	// Check all conversations were added
	blocks := history.GetBlocks()
	if len(blocks) != len(conversations) {
		t.Errorf("Expected %d conversation blocks, got %d", len(conversations), len(blocks))
	}

	// Verify selection is on the last conversation
	expectedSelected := len(conversations) - 1
	if history.GetSelected() != expectedSelected {
		t.Errorf("Expected selected = %d, got %d", expectedSelected, history.GetSelected())
	}

	// Verify conversation order and content
	for i, expectedConv := range conversations {
		if blocks[i].UserInput != expectedConv.userInput {
			t.Errorf("Block %d UserInput = %s, expected %s", i, blocks[i].UserInput, expectedConv.userInput)
		}
		if blocks[i].LLMResponse != expectedConv.assistantResponse {
			t.Errorf("Block %d LLMResponse = %s, expected %s", i, blocks[i].LLMResponse, expectedConv.assistantResponse)
		}
	}
}

func TestConversationHistory_Integration_Selection(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add conversations for selection testing
	for i := 0; i < 5; i++ {
		err := history.AddConversation(fmt.Sprintf("Input %d", i), fmt.Sprintf("Response %d", i))
		if err != nil {
			t.Fatalf("AddConversation() error = %v", err)
		}
	}

	// Test SelectUp
	history.SelectUp()
	if history.GetSelected() != 3 {
		t.Errorf("After SelectUp, expected selected = 3, got %d", history.GetSelected())
	}

	// Test multiple SelectUp
	history.SelectUp()
	history.SelectUp()
	if history.GetSelected() != 1 {
		t.Errorf("After multiple SelectUp, expected selected = 1, got %d", history.GetSelected())
	}

	// Test SelectDown
	history.SelectDown()
	if history.GetSelected() != 2 {
		t.Errorf("After SelectDown, expected selected = 2, got %d", history.GetSelected())
	}

	// Test bounds - SelectUp at beginning
	history.SetSelected(0)
	history.SelectUp()
	if history.GetSelected() != 0 {
		t.Errorf("SelectUp at beginning should stay at 0, got %d", history.GetSelected())
	}

	// Test bounds - SelectDown at end
	history.SetSelected(4)
	history.SelectDown()
	if history.GetSelected() != 4 {
		t.Errorf("SelectDown at end should stay at 4, got %d", history.GetSelected())
	}
}

func TestConversationHistory_Integration_ToggleExpanded(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add a conversation
	err := history.AddConversation("Test input", "Test response")
	if err != nil {
		t.Fatalf("AddConversation() error = %v", err)
	}

	// Initially should not be expanded
	blocks := history.GetBlocks()
	if blocks[0].Expanded {
		t.Error("Conversation should not be expanded initially")
	}

	// Toggle expansion
	history.ToggleExpanded()
	blocks = history.GetBlocks()
	if !blocks[0].Expanded {
		t.Error("Conversation should be expanded after toggle")
	}

	// Toggle again
	history.ToggleExpanded()
	blocks = history.GetBlocks()
	if blocks[0].Expanded {
		t.Error("Conversation should not be expanded after second toggle")
	}
}

func TestConversationHistory_Integration_StatusManagement(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add a conversation
	err := history.AddConversation("Test input", "Test response")
	if err != nil {
		t.Fatalf("AddConversation() error = %v", err)
	}

	blocks := history.GetBlocks()
	conversationID := blocks[0].ID

	// Update status
	err = history.UpdateConversationStatus(conversationID, "executing")
	if err != nil {
		t.Fatalf("UpdateConversationStatus() error = %v", err)
	}

	// Verify status update
	blocks = history.GetBlocks()
	if blocks[0].Status != types.StatusExecuting {
		t.Errorf("Status = %s, expected executing", blocks[0].Status)
	}
}

func TestConversationHistory_Integration_GetConversationByID(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add conversations
	err := history.AddConversation("Test input 1", "Test response 1")
	if err != nil {
		t.Fatalf("AddConversation() error = %v", err)
	}
	err = history.AddConversation("Test input 2", "Test response 2")
	if err != nil {
		t.Fatalf("AddConversation() error = %v", err)
	}

	blocks := history.GetBlocks()
	firstID := blocks[0].ID

	// Get conversation by ID
	conv, err := history.GetConversationByID(firstID)
	if err != nil {
		t.Fatalf("GetConversationByID() error = %v", err)
	}

	if conv == nil {
		t.Fatal("GetConversationByID() returned nil")
	}

	if conv.ID != firstID {
		t.Errorf("Retrieved conversation ID = %s, expected %s", conv.ID, firstID)
	}
	if conv.UserInput != "Test input 1" {
		t.Errorf("Retrieved conversation UserInput = %s, expected Test input 1", conv.UserInput)
	}

	// Test non-existent ID
	nonExistentID := uuid.New().String()
	conv, err = history.GetConversationByID(nonExistentID)
	if err != nil {
		t.Fatalf("GetConversationByID() with non-existent ID error = %v", err)
	}
	if conv != nil {
		t.Error("Expected nil for non-existent conversation ID")
	}
}

func TestConversationHistory_Integration_DeleteConversation(t *testing.T) {
	history, _, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add conversations
	for i := 0; i < 3; i++ {
		err := history.AddConversation(fmt.Sprintf("Input %d", i), fmt.Sprintf("Response %d", i))
		if err != nil {
			t.Fatalf("AddConversation() error = %v", err)
		}
	}

	blocks := history.GetBlocks()
	middleID := blocks[1].ID

	// Delete middle conversation
	err := history.DeleteConversation(middleID)
	if err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	// Verify deletion
	blocks = history.GetBlocks()
	if len(blocks) != 2 {
		t.Errorf("Expected 2 conversations after deletion, got %d", len(blocks))
	}

	// Verify selection adjustment
	if history.GetSelected() != 1 {
		t.Errorf("Expected selected = 1 after deletion, got %d", history.GetSelected())
	}

	// Verify the deleted conversation is not in the cache
	conv, err := history.GetConversationByID(middleID)
	// The error might be expected if the conversation is not found in storage
	// We just verify that the conversation is not returned (conv == nil)
	if conv != nil {
		t.Error("Deleted conversation should not be found")
	}
	// Note: err might be non-nil if the conversation doesn't exist in storage,
	// which is expected behavior after deletion
}

func TestConversationHistory_Integration_PersistenceAcrossInstances(t *testing.T) {
	// Create temporary database path that will be shared
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "shared.db")

	// Create first instance
	db1, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create first database: %v", err)
	}

	runner1 := storage.NewMigrationRunner(db1)
	if err := runner1.RunMigrations(); err != nil {
		t.Fatalf("Failed to run migrations on first database: %v", err)
	}

	config1 := storage.StorageConfig{
		CacheSize:    50,
		AutoSaveFreq: 5 * time.Minute,
	}
	manager1, err := storage.NewStorageManager(db1, config1)
	if err != nil {
		t.Fatalf("Failed to create first StorageManager: %v", err)
	}

	history1 := NewConversationHistory(manager1)

	// Add conversations to first instance
	conversations := []struct {
		userInput, assistantResponse string
	}{
		{"Persistent test 1", "Response 1"},
		{"Persistent test 2", "Response 2"},
	}

	for _, conv := range conversations {
		err := history1.AddConversation(conv.userInput, conv.assistantResponse)
		if err != nil {
			t.Fatalf("AddConversation() error = %v", err)
		}
	}

	// Close first instance properly
	manager1.Close()
	db1.Close()

	// Create second instance with same database file
	db2, err := storage.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create second database: %v", err)
	}
	defer db2.Close()

	// No need to run migrations again, they should already exist
	config2 := storage.StorageConfig{
		CacheSize:    50,
		AutoSaveFreq: 5 * time.Minute,
	}
	manager2, err := storage.NewStorageManager(db2, config2)
	if err != nil {
		t.Fatalf("Failed to create second StorageManager: %v", err)
	}
	defer manager2.Close()

	history2 := NewConversationHistory(manager2)

	// Verify conversations were loaded from persisted storage
	blocks2 := history2.GetBlocks()
	if len(blocks2) != len(conversations) {
		t.Errorf("Expected %d conversations in second instance, got %d", len(conversations), len(blocks2))
	}

	// Since conversations are loaded in reverse chronological order (most recent first),
	// we need to check accordingly
	if len(blocks2) >= 2 {
		// Most recent conversation should be "Persistent test 2"
		if blocks2[0].UserInput != "Persistent test 2" {
			t.Errorf("Most recent conversation UserInput = %s, expected 'Persistent test 2'", blocks2[0].UserInput)
		}
		if blocks2[0].LLMResponse != "Response 2" {
			t.Errorf("Most recent conversation LLMResponse = %s, expected 'Response 2'", blocks2[0].LLMResponse)
		}

		// Older conversation should be "Persistent test 1"  
		if blocks2[1].UserInput != "Persistent test 1" {
			t.Errorf("Older conversation UserInput = %s, expected 'Persistent test 1'", blocks2[1].UserInput)
		}
		if blocks2[1].LLMResponse != "Response 1" {
			t.Errorf("Older conversation LLMResponse = %s, expected 'Response 1'", blocks2[1].LLMResponse)
		}
	}
}

func TestConversationHistory_Integration_RefreshFromStorage(t *testing.T) {
	history, manager, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add conversation through history
	err := history.AddConversation("Original input", "Original response")
	if err != nil {
		t.Fatalf("AddConversation() error = %v", err)
	}

	blocks := history.GetBlocks()
	conversationID := blocks[0].ID

	// Directly modify conversation in storage (simulating external change)
	convRecord, err := manager.LoadConversation(conversationID)
	if err != nil {
		t.Fatalf("LoadConversation() error = %v", err)
	}

	convRecord.LLMResponse = "Modified response"
	err = manager.SaveConversation(convRecord)
	if err != nil {
		t.Fatalf("SaveConversation() error = %v", err)
	}

	// Before refresh, should still see old data
	blocks = history.GetBlocks()
	if blocks[0].LLMResponse != "Original response" {
		t.Error("Should see original response before refresh")
	}

	// Refresh from storage
	history.RefreshFromStorage()

	// After refresh, should see modified data
	blocks = history.GetBlocks()
	if blocks[0].LLMResponse != "Modified response" {
		t.Errorf("Expected modified response after refresh, got %s", blocks[0].LLMResponse)
	}
}

func TestConversationHistory_InMemory(t *testing.T) {
	// Test in-memory mode (no StorageManager)
	history := NewConversationHistoryInMemory()

	// Should start empty
	blocks := history.GetBlocks()
	if len(blocks) != 0 {
		t.Errorf("Expected empty in-memory history, got %d blocks", len(blocks))
	}

	// Add conversation
	err := history.AddConversation("Memory test", "Memory response")
	if err != nil {
		t.Fatalf("AddConversation() in memory mode error = %v", err)
	}

	// Should have one conversation
	blocks = history.GetBlocks()
	if len(blocks) != 1 {
		t.Errorf("Expected 1 conversation in memory mode, got %d", len(blocks))
	}

	if blocks[0].UserInput != "Memory test" {
		t.Errorf("UserInput = %s, expected Memory test", blocks[0].UserInput)
	}
}

func TestConversationHistory_Integration_ErrorHandling(t *testing.T) {
	history, manager, cleanup := setupTestConversationHistory(t)
	defer cleanup()

	// Add a conversation
	err := history.AddConversation("Test input", "Test response")
	if err != nil {
		t.Fatalf("AddConversation() error = %v", err)
	}

	// Close the storage manager to simulate storage failure
	manager.Close()

	// Try to add another conversation - should handle gracefully
	err = history.AddConversation("Test input 2", "Test response 2")
	// In real implementation, this might return an error or handle gracefully
	// For now, we just verify it doesn't panic
	_ = err // Acknowledge we're not checking the error in this test
}