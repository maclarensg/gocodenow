package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TestDatabase creates a temporary database for testing
func TestDatabase() error {
	// Create temporary database path
	tmpDir := os.TempDir()
	dbPath := filepath.Join(tmpDir, fmt.Sprintf("gocodenow_test_%d.db", time.Now().UnixNano()))
	
	fmt.Printf("Creating test database at: %s\n", dbPath)

	// Initialize database
	db, err := InitializeDatabase(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize test database: %w", err)
	}
	defer func() {
		db.Close()
		os.Remove(dbPath) // Cleanup
	}()

	// Test database health
	if err := db.Health(); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	// Test getting version
	version, err := db.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get database version: %w", err)
	}

	fmt.Printf("Database version: %d\n", version)

	// Test inserting a conversation
	testConversationID := "test-conversation-001"
	query := `INSERT INTO conversations (id, user_input, llm_response, model_name) VALUES (?, ?, ?, ?)`
	
	_, err = db.db.Exec(query, testConversationID, "Hello, world!", "This is a test response.", "test-model")
	if err != nil {
		return fmt.Errorf("failed to insert test conversation: %w", err)
	}

	// Test retrieving the conversation
	var id, userInput, llmResponse, modelName string
	var timestamp time.Time
	
	selectQuery := `SELECT id, user_input, llm_response, model_name, timestamp FROM conversations WHERE id = ?`
	err = db.db.QueryRow(selectQuery, testConversationID).Scan(&id, &userInput, &llmResponse, &modelName, &timestamp)
	if err != nil {
		return fmt.Errorf("failed to retrieve test conversation: %w", err)
	}

	fmt.Printf("Retrieved conversation:\n")
	fmt.Printf("  ID: %s\n", id)
	fmt.Printf("  User: %s\n", userInput)
	fmt.Printf("  Assistant: %s\n", llmResponse)
	fmt.Printf("  Model: %s\n", modelName)
	fmt.Printf("  Timestamp: %s\n", timestamp.Format(time.RFC3339))

	fmt.Println("✅ Database test completed successfully!")
	return nil
}