package storage

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// Migration represents a single database migration
type Migration struct {
	Version     int
	Description string
	SQL         []string
}

// MigrationRunner handles database migrations
type MigrationRunner struct {
	db *Database
}

// NewMigrationRunner creates a new migration runner
func NewMigrationRunner(db *Database) *MigrationRunner {
	return &MigrationRunner{db: db}
}

// GetMigrations returns all available migrations in order
func GetMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Initial schema creation",
			SQL: []string{
				SchemaVersionTable,
				ConversationsTable,
				ToolExecutionsTable,
				FileOperationsTable,
			},
		},
		{
			Version:     2,
			Description: "Add database indexes for performance",
			SQL: []string{
				ConversationIndexes,
				ToolExecutionIndexes,
				FileOperationIndexes,
			},
		},
	}
}

// RunMigrations applies all pending migrations
func (mr *MigrationRunner) RunMigrations() error {
	// Ensure schema_version table exists
	if err := mr.createSchemaVersionTable(); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	// Get current database version
	currentVersion, err := mr.db.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get current database version: %w", err)
	}

	log.Printf("Current database version: %d", currentVersion)

	// Get all migrations
	migrations := GetMigrations()

	// Apply pending migrations
	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue // Skip already applied migrations
		}

		log.Printf("Applying migration %d: %s", migration.Version, migration.Description)

		if err := mr.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
		}

		log.Printf("Successfully applied migration %d", migration.Version)
	}

	return nil
}

// createSchemaVersionTable creates the schema version tracking table
func (mr *MigrationRunner) createSchemaVersionTable() error {
	_, err := mr.db.db.Exec(SchemaVersionTable)
	return err
}

// applyMigration applies a single migration within a transaction
func (mr *MigrationRunner) applyMigration(migration Migration) error {
	// Start transaction
	tx, err := mr.db.BeginTransaction()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Rollback on error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Execute all SQL statements in the migration
	for i, sqlStatement := range migration.SQL {
		// Handle multi-statement SQL (like indexes)
		statements := strings.Split(sqlStatement, ";")
		
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			log.Printf("  Executing SQL statement %d/%d", i+1, len(migration.SQL))
			
			if _, execErr := tx.Exec(stmt); execErr != nil {
				err = fmt.Errorf("failed to execute SQL statement: %s, error: %w", stmt, execErr)
				return err
			}
		}
	}

	// Update schema version
	if err = mr.setVersionInTransaction(tx, migration.Version); err != nil {
		return fmt.Errorf("failed to update schema version: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	return nil
}

// setVersionInTransaction updates the schema version within a transaction
func (mr *MigrationRunner) setVersionInTransaction(tx *sql.Tx, version int) error {
	query := `INSERT INTO schema_version (version, applied_at) VALUES (?, CURRENT_TIMESTAMP)`
	_, err := tx.Exec(query, version)
	return err
}

// RollbackToVersion rolls back the database to a specific version (destructive!)
func (mr *MigrationRunner) RollbackToVersion(targetVersion int) error {
	// This is a destructive operation - in a production system you'd want
	// proper rollback migrations. For now, we'll implement a simple approach.
	
	currentVersion, err := mr.db.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if targetVersion >= currentVersion {
		return fmt.Errorf("target version %d is not lower than current version %d", targetVersion, currentVersion)
	}

	log.Printf("WARNING: Rolling back database from version %d to %d", currentVersion, targetVersion)
	log.Printf("This will DROP ALL TABLES and recreate them. All data will be lost!")

	// Drop all tables
	tables := []string{"file_operations", "tool_executions", "conversations", "schema_version"}
	
	tx, err := mr.db.BeginTransaction()
	if err != nil {
		return fmt.Errorf("failed to start rollback transaction: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Drop tables in reverse dependency order
	for _, table := range tables {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
		if _, err = tx.Exec(dropSQL); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	log.Printf("All tables dropped. Re-running migrations to version %d", targetVersion)

	// If target version is 0, don't re-apply any migrations
	if targetVersion == 0 {
		return nil
	}

	// Re-run migrations up to target version
	migrations := GetMigrations()
	for _, migration := range migrations {
		if migration.Version > targetVersion {
			break
		}

		if err := mr.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to re-apply migration %d during rollback: %w", migration.Version, err)
		}
	}

	return nil
}

// GetAppliedMigrations returns a list of all applied migrations
func (mr *MigrationRunner) GetAppliedMigrations() ([]int, error) {
	query := `SELECT version FROM schema_version ORDER BY version`
	rows, err := mr.db.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		versions = append(versions, version)
	}

	return versions, nil
}