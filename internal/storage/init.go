package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// InitializeDatabase creates and migrates the database
func InitializeDatabase(dbPath string) (*Database, error) {
	log.Printf("Initializing database at: %s", dbPath)

	// Create database connection
	db, err := NewDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection: %w", err)
	}

	// Test connection
	if err := db.Health(); err != nil {
		db.Close()
		return nil, fmt.Errorf("database health check failed: %w", err)
	}

	// Run migrations
	migrationRunner := NewMigrationRunner(db)
	if err := migrationRunner.RunMigrations(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	version, err := db.GetVersion()
	if err != nil {
		log.Printf("Warning: Could not determine database version: %v", err)
	} else {
		log.Printf("Database initialized successfully, version: %d", version)
	}

	return db, nil
}

// GetDefaultDatabasePath returns the default database file path
func GetDefaultDatabasePath() (string, error) {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Create path: ~/.local/share/gocodenow/conversations.db
	dataDir := filepath.Join(homeDir, ".local", "share", "lmcodenow")
	dbPath := filepath.Join(dataDir, "conversations.db")

	return dbPath, nil
}

// CleanupDatabase performs database cleanup and optimization
func CleanupDatabase(db *Database) error {
	log.Println("Performing database cleanup...")

	// Run VACUUM to reclaim space and optimize
	if _, err := db.db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	// Analyze tables for query optimization
	if _, err := db.db.Exec("ANALYZE"); err != nil {
		return fmt.Errorf("failed to analyze database: %w", err)
	}

	log.Println("Database cleanup completed")
	return nil
}

// BackupDatabase creates a backup of the database
func BackupDatabase(srcPath, destPath string) error {
	log.Printf("Backing up database from %s to %s", srcPath, destPath)

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Read source file
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source database: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(destPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup database: %w", err)
	}

	log.Printf("Database backup completed: %s", destPath)
	return nil
}

// DatabaseExists checks if a database file exists at the given path
func DatabaseExists(dbPath string) bool {
	_, err := os.Stat(dbPath)
	return err == nil
}

// GetDatabaseSize returns the size of the database file in bytes
func GetDatabaseSize(dbPath string) (int64, error) {
	info, err := os.Stat(dbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get database file info: %w", err)
	}
	return info.Size(), nil
}