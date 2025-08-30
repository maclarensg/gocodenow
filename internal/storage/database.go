package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Database wraps the SQL database connection
type Database struct {
	db   *sql.DB
	path string
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure SQLite settings for better performance
	if err := configureSQLite(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure SQLite: %w", err)
	}

	database := &Database{
		db:   db,
		path: dbPath,
	}

	return database, nil
}

// configureSQLite sets up SQLite pragmas for optimal performance
func configureSQLite(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",           // Enable foreign key constraints
		"PRAGMA journal_mode = WAL",         // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous = NORMAL",       // Good balance of safety and performance
		"PRAGMA cache_size = 1000",          // Cache size in pages
		"PRAGMA temp_store = memory",        // Store temp tables in memory
		"PRAGMA mmap_size = 268435456",      // 256MB memory map
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute pragma %s: %w", pragma, err)
		}
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// DB returns the underlying sql.DB for queries
func (d *Database) DB() *sql.DB {
	return d.db
}

// Path returns the database file path
func (d *Database) Path() string {
	return d.path
}

// Health checks if the database connection is healthy
func (d *Database) Health() error {
	return d.db.Ping()
}

// GetVersion returns the current database schema version
func (d *Database) GetVersion() (int, error) {
	var version int
	query := `SELECT version FROM schema_version ORDER BY id DESC LIMIT 1`
	err := d.db.QueryRow(query).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil // No version found, assume version 0
	}
	if err != nil {
		// Check if the error is due to missing table (after rollback to version 0)
		if strings.Contains(err.Error(), "no such table") {
			return 0, nil // Table doesn't exist, assume version 0
		}
		return 0, fmt.Errorf("failed to get database version: %w", err)
	}
	return version, nil
}

// SetVersion sets the current database schema version
func (d *Database) SetVersion(version int) error {
	query := `INSERT INTO schema_version (version, applied_at) VALUES (?, ?)`
	_, err := d.db.Exec(query, version, time.Now())
	if err != nil {
		return fmt.Errorf("failed to set database version: %w", err)
	}
	return nil
}

// BeginTransaction starts a new database transaction
func (d *Database) BeginTransaction() (*sql.Tx, error) {
	return d.db.Begin()
}

// Schema definitions
const (
	// SchemaVersionTable tracks database migrations
	SchemaVersionTable = `
		CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version INTEGER NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`

	// ConversationsTable stores conversation history
	ConversationsTable = `
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			user_input TEXT NOT NULL,
			llm_response TEXT NOT NULL,
			model_name TEXT NOT NULL,
			token_usage_input INTEGER DEFAULT 0,
			token_usage_output INTEGER DEFAULT 0,
			execution_time_ms INTEGER DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'completed',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`

	// ToolExecutionsTable stores tool execution records
	ToolExecutionsTable = `
		CREATE TABLE IF NOT EXISTS tool_executions (
			id TEXT PRIMARY KEY,
			conversation_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			parameters TEXT NOT NULL,
			result TEXT,
			success BOOLEAN NOT NULL DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		)
	`

	// FileOperationsTable stores file operation records
	FileOperationsTable = `
		CREATE TABLE IF NOT EXISTS file_operations (
			id TEXT PRIMARY KEY,
			tool_execution_id TEXT NOT NULL,
			operation_type TEXT NOT NULL,
			file_path TEXT NOT NULL,
			old_content TEXT,
			new_content TEXT,
			success BOOLEAN NOT NULL DEFAULT 0,
			error_message TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (tool_execution_id) REFERENCES tool_executions(id) ON DELETE CASCADE
		)
	`

	// Indexes for better query performance
	ConversationIndexes = `
		CREATE INDEX IF NOT EXISTS idx_conversations_timestamp ON conversations(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_conversations_status ON conversations(status);
		CREATE INDEX IF NOT EXISTS idx_conversations_model ON conversations(model_name);
	`

	ToolExecutionIndexes = `
		CREATE INDEX IF NOT EXISTS idx_tool_executions_conversation ON tool_executions(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_tool_executions_tool_name ON tool_executions(tool_name);
		CREATE INDEX IF NOT EXISTS idx_tool_executions_success ON tool_executions(success);
	`

	FileOperationIndexes = `
		CREATE INDEX IF NOT EXISTS idx_file_operations_tool_execution ON file_operations(tool_execution_id);
		CREATE INDEX IF NOT EXISTS idx_file_operations_type ON file_operations(operation_type);
		CREATE INDEX IF NOT EXISTS idx_file_operations_path ON file_operations(file_path);
	`
)

// AllTables returns all table creation statements
func AllTables() []string {
	return []string{
		SchemaVersionTable,
		ConversationsTable,
		ToolExecutionsTable,
		FileOperationsTable,
	}
}

// AllIndexes returns all index creation statements
func AllIndexes() []string {
	return []string{
		ConversationIndexes,
		ToolExecutionIndexes,
		FileOperationIndexes,
	}
}