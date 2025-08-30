package ui

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"gocodenow/internal/types"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupMetadata contains information about a backup
type BackupMetadata struct {
	CreatedAt     time.Time `json:"created_at"`
	Version       string    `json:"version"`
	TotalConvs    int       `json:"total_conversations"`
	Checksum      string    `json:"checksum"`
	Description   string    `json:"description"`
	Source        string    `json:"source"`
	CompressionType string  `json:"compression_type"`
}

// BackupManager handles backup and restore operations
type BackupManager struct {
	backupDir    string
	exportMgr    *ExportManager
	importMgr    *ImportManager
	maxBackups   int
}

// NewBackupManager creates a new backup manager
func NewBackupManager(backupDir string, maxBackups int) *BackupManager {
	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		backupDir = "." // Fallback to current directory
	}

	if maxBackups <= 0 {
		maxBackups = 10 // Default to keeping 10 backups
	}

	return &BackupManager{
		backupDir:  backupDir,
		exportMgr:  NewExportManager(backupDir),
		importMgr:  NewImportManager(),
		maxBackups: maxBackups,
	}
}

// CreateBackup creates a compressed backup of conversations
func (bm *BackupManager) CreateBackup(conversations []types.ConversationBlock, description string) (string, error) {
	if len(conversations) == 0 {
		return "", fmt.Errorf("no conversations to backup")
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupName := fmt.Sprintf("lmcodenow_backup_%s.tar.gz", timestamp)
	backupPath := filepath.Join(bm.backupDir, backupName)

	// Create backup file
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer backupFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(backupFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Create temporary JSON export
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("lmcodenow_backup_%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Export conversations to JSON
	jsonPath := filepath.Join(tempDir, "conversations.json")
	exportOptions := ExportOptions{
		Format:       FormatJSON,
		FilePath:     jsonPath,
		IncludeTools: true,
		IncludeFiles: true,
		PrettyPrint:  true,
	}

	if err := bm.exportMgr.ExportConversations(conversations, exportOptions); err != nil {
		return "", fmt.Errorf("failed to export conversations: %w", err)
	}

	// Calculate checksum of JSON file
	checksum, err := bm.calculateFileChecksum(jsonPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Create metadata
	metadata := BackupMetadata{
		CreatedAt:       time.Now(),
		Version:         "1.0",
		TotalConvs:      len(conversations),
		Checksum:        checksum,
		Description:     description,
		Source:          "lmcodenow",
		CompressionType: "gzip",
	}

	// Add metadata to archive
	metadataPath := filepath.Join(tempDir, "metadata.json")
	if err := bm.writeMetadata(metadata, metadataPath); err != nil {
		return "", fmt.Errorf("failed to write metadata: %w", err)
	}

	// Add files to tar archive
	files := []string{"conversations.json", "metadata.json"}
	for _, filename := range files {
		filePath := filepath.Join(tempDir, filename)
		if err := bm.addFileToTar(tarWriter, filePath, filename); err != nil {
			return "", fmt.Errorf("failed to add %s to archive: %w", filename, err)
		}
	}

	// Cleanup old backups
	if err := bm.cleanupOldBackups(); err != nil {
		// Log warning but don't fail the backup
		fmt.Printf("Warning: failed to cleanup old backups: %v\n", err)
	}

	return backupPath, nil
}

// RestoreBackup restores conversations from a backup file
func (bm *BackupManager) RestoreBackup(backupPath string, validateData bool) (*ImportResult, error) {
	// Validate backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	// Create temporary directory for extraction
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("lmcodenow_restore_%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract backup
	if err := bm.extractBackup(backupPath, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract backup: %w", err)
	}

	// Read and validate metadata
	metadataPath := filepath.Join(tempDir, "metadata.json")
	metadata, err := bm.readMetadata(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup metadata: %w", err)
	}

	// Validate checksum
	conversationsPath := filepath.Join(tempDir, "conversations.json")
	checksum, err := bm.calculateFileChecksum(conversationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if checksum != metadata.Checksum {
		return nil, fmt.Errorf("backup integrity check failed: checksum mismatch")
	}

	// Import conversations
	importOptions := ImportOptions{
		FilePath:           conversationsPath,
		Format:             FormatJSON,
		ValidateData:       validateData,
		PreserveTimestamps: true,
		BatchSize:          100,
	}

	result, err := bm.importMgr.ImportConversations(importOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to import conversations: %w", err)
	}

	return result, nil
}

// ListBackups returns information about available backups
func (bm *BackupManager) ListBackups() ([]BackupInfo, error) {
	files, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".gz" &&
			(strings.HasPrefix(file.Name(), "lmcodenow_backup_") || strings.HasSuffix(file.Name(), ".tar.gz")) {
			
			info, err := file.Info()
			if err != nil {
				continue
			}

			backup := BackupInfo{
				Name:     file.Name(),
				Path:     filepath.Join(bm.backupDir, file.Name()),
				Size:     info.Size(),
				ModTime:  info.ModTime(),
			}

			// Try to read metadata if possible
			if metadata, err := bm.getBackupMetadata(backup.Path); err == nil {
				backup.Metadata = metadata
			}

			backups = append(backups, backup)
		}
	}

	return backups, nil
}

// BackupInfo contains information about a backup file
type BackupInfo struct {
	Name     string          `json:"name"`
	Path     string          `json:"path"`
	Size     int64           `json:"size"`
	ModTime  time.Time       `json:"mod_time"`
	Metadata *BackupMetadata `json:"metadata,omitempty"`
}

// getBackupMetadata attempts to read metadata from a backup file
func (bm *BackupManager) getBackupMetadata(backupPath string) (*BackupMetadata, error) {
	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("lmcodenow_meta_%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	// Extract only metadata file
	if err := bm.extractFileFromBackup(backupPath, "metadata.json", tempDir); err != nil {
		return nil, err
	}

	// Read metadata
	return bm.readMetadata(filepath.Join(tempDir, "metadata.json"))
}

// extractFileFromBackup extracts a single file from a backup archive
func (bm *BackupManager) extractFileFromBackup(backupPath, filename, destDir string) error {
	file, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Name == filename {
			destPath := filepath.Join(destDir, filename)
			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, tarReader)
			return err
		}
	}

	return fmt.Errorf("file %s not found in backup", filename)
}

// DeleteBackup deletes a backup file
func (bm *BackupManager) DeleteBackup(backupPath string) error {
	if !filepath.IsAbs(backupPath) {
		backupPath = filepath.Join(bm.backupDir, backupPath)
	}

	// Verify it's in the backup directory for safety
	if !strings.HasPrefix(backupPath, bm.backupDir) {
		return fmt.Errorf("backup file must be in the backup directory")
	}

	return os.Remove(backupPath)
}

// calculateFileChecksum calculates MD5 checksum of a file
func (bm *BackupManager) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// writeMetadata writes metadata to a JSON file
func (bm *BackupManager) writeMetadata(metadata BackupMetadata, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(metadata)
}

// readMetadata reads metadata from a JSON file
func (bm *BackupManager) readMetadata(filePath string) (*BackupMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var metadata BackupMetadata
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// addFileToTar adds a file to a tar archive
func (bm *BackupManager) addFileToTar(tarWriter *tar.Writer, filePath, archiveName string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    archiveName,
		Mode:    int64(info.Mode()),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	return err
}

// extractBackup extracts a tar.gz backup file
func (bm *BackupManager) extractBackup(backupPath, destDir string) error {
	file, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, header.Name)
		
		// Security check: ensure the file is within destDir
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeReg:
			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()

			if _, err := io.Copy(destFile, tarReader); err != nil {
				return err
			}

			if err := os.Chmod(destPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}

	return nil
}

// cleanupOldBackups removes old backup files to maintain the limit
func (bm *BackupManager) cleanupOldBackups() error {
	backups, err := bm.ListBackups()
	if err != nil {
		return err
	}

	if len(backups) <= bm.maxBackups {
		return nil // No cleanup needed
	}

	// Sort backups by modification time (oldest first)
	for i := 0; i < len(backups)-1; i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].ModTime.After(backups[j].ModTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	// Delete oldest backups
	toDelete := len(backups) - bm.maxBackups
	for i := 0; i < toDelete; i++ {
		if err := os.Remove(backups[i].Path); err != nil {
			return fmt.Errorf("failed to delete old backup %s: %w", backups[i].Name, err)
		}
	}

	return nil
}

// GetBackupStats returns statistics about backups
func (bm *BackupManager) GetBackupStats() (*BackupStats, error) {
	backups, err := bm.ListBackups()
	if err != nil {
		return nil, err
	}

	stats := &BackupStats{
		TotalBackups: len(backups),
		BackupDir:    bm.backupDir,
		MaxBackups:   bm.maxBackups,
	}

	var totalSize int64
	var oldestTime, newestTime time.Time

	for i, backup := range backups {
		totalSize += backup.Size

		if i == 0 {
			oldestTime = backup.ModTime
			newestTime = backup.ModTime
		} else {
			if backup.ModTime.Before(oldestTime) {
				oldestTime = backup.ModTime
			}
			if backup.ModTime.After(newestTime) {
				newestTime = backup.ModTime
			}
		}

		if backup.Metadata != nil {
			stats.TotalConversations += backup.Metadata.TotalConvs
		}
	}

	stats.TotalSize = totalSize
	if !oldestTime.IsZero() {
		stats.OldestBackup = &oldestTime
		stats.NewestBackup = &newestTime
	}

	return stats, nil
}

// BackupStats contains statistics about backups
type BackupStats struct {
	TotalBackups       int        `json:"total_backups"`
	TotalSize          int64      `json:"total_size"`
	TotalConversations int        `json:"total_conversations"`
	BackupDir          string     `json:"backup_dir"`
	MaxBackups         int        `json:"max_backups"`
	OldestBackup       *time.Time `json:"oldest_backup,omitempty"`
	NewestBackup       *time.Time `json:"newest_backup,omitempty"`
}

// VerifyBackup verifies the integrity of a backup file
func (bm *BackupManager) VerifyBackup(backupPath string) error {
	metadata, err := bm.getBackupMetadata(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup metadata: %w", err)
	}

	// Create temporary directory for extraction
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("lmcodenow_verify_%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract conversations file
	if err := bm.extractFileFromBackup(backupPath, "conversations.json", tempDir); err != nil {
		return fmt.Errorf("failed to extract conversations: %w", err)
	}

	// Verify checksum
	conversationsPath := filepath.Join(tempDir, "conversations.json")
	checksum, err := bm.calculateFileChecksum(conversationsPath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if checksum != metadata.Checksum {
		return fmt.Errorf("backup integrity check failed: checksum mismatch")
	}

	// Try to parse the JSON to ensure it's valid
	importOptions := ImportOptions{
		FilePath:     conversationsPath,
		Format:       FormatJSON,
		ValidateData: true,
	}

	_, err = bm.importMgr.ImportConversations(importOptions)
	if err != nil {
		return fmt.Errorf("backup validation failed: %w", err)
	}

	return nil
}