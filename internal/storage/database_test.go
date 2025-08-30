package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestDatabaseInitialization tests basic database creation and initialization
func TestDatabaseInitialization(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_init.db")

	// Test successful initialization
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test health check
	if err := db.Health(); err != nil {
		t.Errorf("Database health check failed: %v", err)
	}

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Test database path
	if db.Path() != dbPath {
		t.Errorf("Expected path %s, got %s", dbPath, db.Path())
	}
}

// TestDatabaseInitializationErrors tests error conditions during initialization
func TestDatabaseInitializationErrors(t *testing.T) {
	// Test invalid path
	invalidPath := "/root/cannot_write/test.db" // Assuming no write permission
	_, err := NewDatabase(invalidPath)
	if err == nil {
		t.Error("Expected error for invalid database path, got nil")
	}

	// Test directory creation with invalid parent
	if os.Getuid() != 0 { // Skip if running as root
		restrictedPath := "/etc/gocodenow/test.db"
		_, err := NewDatabase(restrictedPath)
		if err == nil {
			t.Error("Expected error for restricted directory, got nil")
		}
	}
}

// TestDatabaseConnection tests connection handling and reconnection
func TestDatabaseConnection(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_connection.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test multiple health checks
	for i := 0; i < 5; i++ {
		if err := db.Health(); err != nil {
			t.Errorf("Health check %d failed: %v", i, err)
		}
	}

	// Test accessing underlying DB
	sqlDB := db.DB()
	if sqlDB == nil {
		t.Error("Expected non-nil SQL DB")
	}

	// Test basic query
	var count int
	err = sqlDB.QueryRow("SELECT COUNT(*) FROM sqlite_master").Scan(&count)
	if err != nil {
		t.Errorf("Failed to execute basic query: %v", err)
	}
}

// TestDatabaseMigrations tests the migration system
func TestDatabaseMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_migrations.db")

	// Initialize database (should run migrations)
	db, err := InitializeDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check that migrations were applied
	version, err := db.GetVersion()
	if err != nil {
		t.Fatalf("Failed to get database version: %v", err)
	}

	expectedVersion := 2 // Based on our current migrations
	if version != expectedVersion {
		t.Errorf("Expected version %d, got %d", expectedVersion, version)
	}

	// Verify tables exist
	tables := []string{"schema_version", "conversations", "tool_executions", "file_operations"}
	for _, table := range tables {
		var exists int
		query := `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`
		err = db.db.QueryRow(query, table).Scan(&exists)
		if err != nil {
			t.Errorf("Failed to check table %s: %v", table, err)
		}
		if exists != 1 {
			t.Errorf("Table %s does not exist", table)
		}
	}
}

// TestMigrationRollback tests migration rollback functionality
func TestMigrationRollback(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_rollback.db")

	// Initialize database
	db, err := InitializeDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Get initial version
	initialVersion, err := db.GetVersion()
	if err != nil {
		t.Fatalf("Failed to get initial version: %v", err)
	}

	db.Close()

	// Test rollback
	db2, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer db2.Close()

	migrationRunner := NewMigrationRunner(db2)

	// Test rollback to version 0 (this will drop all tables)
	if err := migrationRunner.RollbackToVersion(0); err != nil {
		t.Fatalf("Failed to rollback to version 0: %v", err)
	}

	// After rollback to version 0, schema_version table doesn't exist
	// so GetVersion should return 0 (default when no table exists)
	version, err := db2.GetVersion()
	if err != nil {
		t.Fatalf("Failed to get version after rollback: %v", err)
	}

	if version != 0 {
		t.Errorf("Expected version 0 after rollback, got %d", version)
	}

	// Verify tables don't exist (except schema_version)
	tables := []string{"conversations", "tool_executions", "file_operations"}
	for _, table := range tables {
		var exists int
		query := `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`
		err = db2.db.QueryRow(query, table).Scan(&exists)
		if err != nil {
			t.Errorf("Failed to check table %s after rollback: %v", table, err)
		}
		if exists != 0 {
			t.Errorf("Table %s still exists after rollback", table)
		}
	}

	// Test that we can re-apply migrations
	if err := migrationRunner.RunMigrations(); err != nil {
		t.Errorf("Failed to re-apply migrations after rollback: %v", err)
	}

	finalVersion, err := db2.GetVersion()
	if err != nil {
		t.Fatalf("Failed to get final version: %v", err)
	}

	if finalVersion != initialVersion {
		t.Errorf("Expected final version %d, got %d", initialVersion, finalVersion)
	}
}

// TestConcurrentDatabaseAccess tests concurrent database operations
func TestConcurrentDatabaseAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_concurrent.db")

	// Initialize database
	db, err := InitializeDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	const numGoroutines = 10
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	// Test concurrent inserts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				conversationID := fmt.Sprintf("test-conversation-%d-%d", routineID, j)
				query := `INSERT INTO conversations (id, user_input, llm_response, model_name) VALUES (?, ?, ?, ?)`
				
				_, err := db.db.Exec(query, conversationID, "Test input", "Test response", "test-model")
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("Had %d concurrent operation errors", errorCount)
	}

	// Verify all records were inserted
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM conversations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count conversations: %v", err)
	}

	expectedCount := numGoroutines * operationsPerGoroutine
	if count != expectedCount {
		t.Errorf("Expected %d conversations, got %d", expectedCount, count)
	}
}

// TestTransactionHandling tests transaction functionality
func TestTransactionHandling(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_transactions.db")

	db, err := InitializeDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test successful transaction
	tx, err := db.BeginTransaction()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Insert data in transaction
	query := `INSERT INTO conversations (id, user_input, llm_response, model_name) VALUES (?, ?, ?, ?)`
	_, err = tx.Exec(query, "tx-test-1", "Test input 1", "Test response 1", "test-model")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to execute insert in transaction: %v", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify data was committed
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM conversations WHERE id = ?", "tx-test-1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count committed records: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 committed record, got %d", count)
	}

	// Test transaction rollback
	tx2, err := db.BeginTransaction()
	if err != nil {
		t.Fatalf("Failed to begin second transaction: %v", err)
	}

	_, err = tx2.Exec(query, "tx-test-2", "Test input 2", "Test response 2", "test-model")
	if err != nil {
		tx2.Rollback()
		t.Fatalf("Failed to execute insert in second transaction: %v", err)
	}

	// Rollback instead of commit
	if err = tx2.Rollback(); err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}

	// Verify data was not committed
	err = db.db.QueryRow("SELECT COUNT(*) FROM conversations WHERE id = ?", "tx-test-2").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count rolled back records: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 rolled back records, got %d", count)
	}
}

// TestForeignKeyConstraints tests foreign key relationships
func TestForeignKeyConstraints(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_foreign_keys.db")

	db, err := InitializeDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Insert a conversation first
	conversationID := "fk-test-conversation"
	query := `INSERT INTO conversations (id, user_input, llm_response, model_name) VALUES (?, ?, ?, ?)`
	_, err = db.db.Exec(query, conversationID, "Test input", "Test response", "test-model")
	if err != nil {
		t.Fatalf("Failed to insert conversation: %v", err)
	}

	// Insert a tool execution that references the conversation
	toolExecID := "fk-test-tool-exec"
	toolQuery := `INSERT INTO tool_executions (id, conversation_id, tool_name, parameters, success) VALUES (?, ?, ?, ?, ?)`
	_, err = db.db.Exec(toolQuery, toolExecID, conversationID, "test_tool", "{}", true)
	if err != nil {
		t.Fatalf("Failed to insert tool execution: %v", err)
	}

	// Try to insert tool execution with invalid conversation_id (should fail)
	_, err = db.db.Exec(toolQuery, "invalid-tool-exec", "non-existent-conversation", "test_tool", "{}", true)
	if err == nil {
		t.Error("Expected foreign key constraint violation, but insert succeeded")
	}

	// Insert a file operation that references the tool execution
	fileOpQuery := `INSERT INTO file_operations (id, tool_execution_id, operation_type, file_path, success) VALUES (?, ?, ?, ?, ?)`
	_, err = db.db.Exec(fileOpQuery, "fk-test-file-op", toolExecID, "read", "/test/path", true)
	if err != nil {
		t.Fatalf("Failed to insert file operation: %v", err)
	}

	// Try to insert file operation with invalid tool_execution_id (should fail)
	_, err = db.db.Exec(fileOpQuery, "invalid-file-op", "non-existent-tool-exec", "read", "/test/path", true)
	if err == nil {
		t.Error("Expected foreign key constraint violation for file operation, but insert succeeded")
	}

	// Test cascade delete: delete conversation should cascade to related records
	_, err = db.db.Exec("DELETE FROM conversations WHERE id = ?", conversationID)
	if err != nil {
		t.Fatalf("Failed to delete conversation: %v", err)
	}

	// Verify tool execution was cascade deleted
	var toolCount int
	err = db.db.QueryRow("SELECT COUNT(*) FROM tool_executions WHERE id = ?", toolExecID).Scan(&toolCount)
	if err != nil {
		t.Fatalf("Failed to count tool executions: %v", err)
	}
	if toolCount != 0 {
		t.Error("Tool execution was not cascade deleted")
	}

	// Verify file operation was cascade deleted
	var fileCount int
	err = db.db.QueryRow("SELECT COUNT(*) FROM file_operations WHERE id = ?", "fk-test-file-op").Scan(&fileCount)
	if err != nil {
		t.Fatalf("Failed to count file operations: %v", err)
	}
	if fileCount != 0 {
		t.Error("File operation was not cascade deleted")
	}
}

// TestDatabaseIndexes tests that indexes are working correctly
func TestDatabaseIndexes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_indexes.db")

	db, err := InitializeDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Insert test data with timestamps
	conversations := []struct {
		id    string
		model string
		timestamp time.Time
	}{
		{"conv-1", "gpt-4", time.Now().Add(-1 * time.Hour)},
		{"conv-2", "gpt-3.5", time.Now().Add(-2 * time.Hour)},
		{"conv-3", "gpt-4", time.Now().Add(-30 * time.Minute)},
	}

	for _, conv := range conversations {
		query := `INSERT INTO conversations (id, user_input, llm_response, model_name, timestamp) VALUES (?, ?, ?, ?, ?)`
		_, err = db.db.Exec(query, conv.id, "Test input", "Test response", conv.model, conv.timestamp)
		if err != nil {
			t.Fatalf("Failed to insert test conversation %s: %v", conv.id, err)
		}
	}

	// Test timestamp index (ORDER BY timestamp DESC should be fast)
	rows, err := db.db.Query("SELECT id FROM conversations ORDER BY timestamp DESC")
	if err != nil {
		t.Fatalf("Failed to query by timestamp: %v", err)
	}
	
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatalf("Failed to scan conversation ID: %v", err)
		}
		ids = append(ids, id)
	}
	rows.Close()

	// Should be ordered by timestamp DESC (most recent first)
	expectedOrder := []string{"conv-3", "conv-1", "conv-2"}
	for i, expectedID := range expectedOrder {
		if i >= len(ids) || ids[i] != expectedID {
			t.Errorf("Expected conversation order %v, got %v", expectedOrder, ids)
			break
		}
	}

	// Test model name index
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM conversations WHERE model_name = ?", "gpt-4").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query by model name: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 gpt-4 conversations, got %d", count)
	}
}

// TestDatabaseCleanup tests cleanup and optimization functions
func TestDatabaseCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_cleanup.db")

	db, err := InitializeDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Insert and delete some data to create fragmentation
	for i := 0; i < 100; i++ {
		conversationID := fmt.Sprintf("cleanup-test-%d", i)
		query := `INSERT INTO conversations (id, user_input, llm_response, model_name) VALUES (?, ?, ?, ?)`
		_, err = db.db.Exec(query, conversationID, "Test input", "Test response", "test-model")
		if err != nil {
			t.Fatalf("Failed to insert test conversation %d: %v", i, err)
		}
	}

	// Delete half the records
	_, err = db.db.Exec("DELETE FROM conversations WHERE id LIKE 'cleanup-test-%' AND CAST(SUBSTR(id, 14) AS INTEGER) % 2 = 0")
	if err != nil {
		t.Fatalf("Failed to delete test conversations: %v", err)
	}

	// Test cleanup
	if err := CleanupDatabase(db); err != nil {
		t.Errorf("Database cleanup failed: %v", err)
	}

	// Verify database is still functional after cleanup
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM conversations WHERE id LIKE 'cleanup-test-%'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count conversations after cleanup: %v", err)
	}
	
	// Should have 50 records left (odd numbered ones)
	if count != 50 {
		t.Errorf("Expected 50 conversations after cleanup, got %d", count)
	}
}

// Benchmark tests
func BenchmarkDatabaseInsert(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_insert.db")

	db, err := InitializeDatabase(dbPath)
	if err != nil {
		b.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conversationID := fmt.Sprintf("bench-conversation-%d", i)
		query := `INSERT INTO conversations (id, user_input, llm_response, model_name) VALUES (?, ?, ?, ?)`
		_, err = db.db.Exec(query, conversationID, "Benchmark input", "Benchmark response", "bench-model")
		if err != nil {
			b.Fatalf("Failed to insert benchmark conversation %d: %v", i, err)
		}
	}
}

func BenchmarkDatabaseQuery(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_query.db")

	db, err := InitializeDatabase(dbPath)
	if err != nil {
		b.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Insert test data
	for i := 0; i < 1000; i++ {
		conversationID := fmt.Sprintf("bench-query-%d", i)
		query := `INSERT INTO conversations (id, user_input, llm_response, model_name) VALUES (?, ?, ?, ?)`
		_, err = db.db.Exec(query, conversationID, "Benchmark input", "Benchmark response", "bench-model")
		if err != nil {
			b.Fatalf("Failed to insert test data %d: %v", i, err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rows, err := db.db.Query("SELECT id, user_input, llm_response FROM conversations ORDER BY timestamp DESC LIMIT 20")
		if err != nil {
			b.Fatalf("Failed to query conversations: %v", err)
		}
		
		// Consume the rows
		for rows.Next() {
			var id, input, response string
			rows.Scan(&id, &input, &response)
		}
		rows.Close()
	}
}