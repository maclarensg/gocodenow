// Package tools provides file backup and rollback system
package tools

import (
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupManager manages file backups and rollback operations
type BackupManager struct {
	backupDir     string
	maxBackups    int
	compressOld   bool
	retentionDays int
	metadata      *BackupMetadata
}

// BackupMetadata tracks backup information
type BackupMetadata struct {
	backupDir string
	backups   map[string][]*BackupInfo
}

// BackupInfo contains information about a single backup
type BackupInfo struct {
	ID           string            `json:"id"`
	OriginalPath string            `json:"original_path"`
	BackupPath   string            `json:"backup_path"`
	Timestamp    time.Time         `json:"timestamp"`
	Size         int64             `json:"size"`
	Checksum     string            `json:"checksum"`
	Description  string            `json:"description,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Version      int               `json:"version"`
	Compressed   bool              `json:"compressed"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	FileMode     os.FileMode       `json:"file_mode"`
	ModTime      time.Time         `json:"mod_time"`
}

// BackupOptions configures backup behavior
type BackupOptions struct {
	Description   string            `json:"description,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Compress      bool              `json:"compress"`
	CreateDiff    bool              `json:"create_diff"`
	PreserveMode  bool              `json:"preserve_mode"`
	PreserveTime  bool              `json:"preserve_time"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// RestoreOptions configures restore behavior
type RestoreOptions struct {
	BackupID      string `json:"backup_id,omitempty"`
	Version       int    `json:"version,omitempty"`      // Restore to specific version
	Timestamp     string `json:"timestamp,omitempty"`    // Restore to specific time
	CreateBackup  bool   `json:"create_backup"`          // Create backup before restore
	PreserveMode  bool   `json:"preserve_mode"`
	PreserveTime  bool   `json:"preserve_time"`
	VerifyChecksum bool  `json:"verify_checksum"`
}

// BackupStats provides statistics about backups
type BackupStats struct {
	TotalFiles        int               `json:"total_files"`
	TotalBackups      int               `json:"total_backups"`
	TotalSize         int64             `json:"total_size"`
	CompressedSize    int64             `json:"compressed_size,omitempty"`
	OldestBackup      *time.Time        `json:"oldest_backup,omitempty"`
	NewestBackup      *time.Time        `json:"newest_backup,omitempty"`
	TopFiles          []string          `json:"top_files,omitempty"`        // Most backed up files
	BackupsByDay      map[string]int    `json:"backups_by_day,omitempty"`
	CompressionRatio  float64           `json:"compression_ratio,omitempty"`
}

// NewBackupManager creates a new backup manager
func NewBackupManager(backupDir string, maxBackups int) (*BackupManager, error) {
	if backupDir == "" {
		// Default to .gocodenow-backups in user's home directory
		homeDir, _ := os.UserHomeDir()
		backupDir = filepath.Join(homeDir, ".gocodenow-backups")
	}

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %v", err)
	}

	if maxBackups <= 0 {
		maxBackups = 100 // Default max backups per file
	}

	bm := &BackupManager{
		backupDir:     backupDir,
		maxBackups:    maxBackups,
		compressOld:   true,
		retentionDays: 30,
		metadata: &BackupMetadata{
			backupDir: backupDir,
			backups:   make(map[string][]*BackupInfo),
		},
	}

	// Load existing metadata
	if err := bm.loadMetadata(); err != nil {
		return nil, fmt.Errorf("failed to load metadata: %v", err)
	}

	return bm, nil
}

// CreateBackup creates a backup of the specified file
func (bm *BackupManager) CreateBackup(filePath string, opts BackupOptions) (*BackupInfo, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %v", err)
	}

	// Check if file exists
	sourceInfo, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("source file does not exist: %v", err)
	}

	// Calculate checksum
	checksum, err := bm.calculateChecksum(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %v", err)
	}

	// Generate backup ID
	backupID := bm.generateBackupID(absPath)
	
	// Create backup directory for this file
	fileBackupDir := filepath.Join(bm.backupDir, bm.sanitizePath(absPath))
	if err := os.MkdirAll(fileBackupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create file backup directory: %v", err)
	}

	// Generate backup filename
	timestamp := time.Now()
	version := bm.getNextVersion(absPath)
	backupFilename := fmt.Sprintf("%s_v%d_%s", filepath.Base(absPath), version, timestamp.Format("20060102_150405"))
	
	if opts.Compress {
		backupFilename += ".gz"
	}
	
	backupPath := filepath.Join(fileBackupDir, backupFilename)

	// Create backup
	if err := bm.copyFile(absPath, backupPath, opts); err != nil {
		return nil, fmt.Errorf("failed to create backup: %v", err)
	}

	// Get backup file info
	_, err = os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %v", err)
	}

	// Create backup info
	backup := &BackupInfo{
		ID:           backupID,
		OriginalPath: absPath,
		BackupPath:   backupPath,
		Timestamp:    timestamp,
		Size:         sourceInfo.Size(),
		Checksum:     checksum,
		Description:  opts.Description,
		Tags:         opts.Tags,
		Version:      version,
		Compressed:   opts.Compress,
		Metadata:     opts.Metadata,
		FileMode:     sourceInfo.Mode(),
		ModTime:      sourceInfo.ModTime(),
	}

	// Add to metadata
	bm.metadata.backups[absPath] = append(bm.metadata.backups[absPath], backup)

	// Sort backups by timestamp (newest first)
	sort.Slice(bm.metadata.backups[absPath], func(i, j int) bool {
		return bm.metadata.backups[absPath][i].Timestamp.After(bm.metadata.backups[absPath][j].Timestamp)
	})

	// Cleanup old backups
	if err := bm.cleanupOldBackups(absPath); err != nil {
		return nil, fmt.Errorf("failed to cleanup old backups: %v", err)
	}

	// Save metadata
	if err := bm.saveMetadata(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %v", err)
	}

	return backup, nil
}

// RestoreBackup restores a file from backup
func (bm *BackupManager) RestoreBackup(filePath string, opts RestoreOptions) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %v", err)
	}

	// Find the backup to restore
	backup, err := bm.findBackup(absPath, opts)
	if err != nil {
		return fmt.Errorf("failed to find backup: %v", err)
	}

	// Create backup of current file if requested
	if opts.CreateBackup {
		if _, err := os.Stat(absPath); err == nil {
			_, err := bm.CreateBackup(absPath, BackupOptions{
				Description: "Pre-restore backup",
				Tags:        []string{"auto", "pre-restore"},
			})
			if err != nil {
				return fmt.Errorf("failed to create pre-restore backup: %v", err)
			}
		}
	}

	// Restore the file
	if err := bm.restoreFile(backup, absPath, opts); err != nil {
		return fmt.Errorf("failed to restore file: %v", err)
	}

	// Verify checksum if requested
	if opts.VerifyChecksum {
		checksum, err := bm.calculateChecksum(absPath)
		if err != nil {
			return fmt.Errorf("failed to verify checksum: %v", err)
		}
		if checksum != backup.Checksum {
			return fmt.Errorf("checksum mismatch after restore")
		}
	}

	return nil
}

// ListBackups lists all backups for a file
func (bm *BackupManager) ListBackups(filePath string) ([]*BackupInfo, error) {
	if filePath == "" {
		// List all backups
		var allBackups []*BackupInfo
		for _, backups := range bm.metadata.backups {
			allBackups = append(allBackups, backups...)
		}
		
		// Sort by timestamp
		sort.Slice(allBackups, func(i, j int) bool {
			return allBackups[i].Timestamp.After(allBackups[j].Timestamp)
		})
		
		return allBackups, nil
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %v", err)
	}

	backups, exists := bm.metadata.backups[absPath]
	if !exists {
		return []*BackupInfo{}, nil
	}

	return backups, nil
}

// DeleteBackup deletes a specific backup
func (bm *BackupManager) DeleteBackup(backupID string) error {
	// Find the backup
	var backup *BackupInfo
	
	for path, backups := range bm.metadata.backups {
		for i, b := range backups {
			if b.ID == backupID {
				backup = b
				
				// Remove from slice
				bm.metadata.backups[path] = append(backups[:i], backups[i+1:]...)
				break
			}
		}
		if backup != nil {
			break
		}
	}

	if backup == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	// Delete backup file
	if err := os.Remove(backup.BackupPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete backup file: %v", err)
	}

	// Clean up empty directories
	bm.cleanupEmptyDirs(filepath.Dir(backup.BackupPath))

	// Save metadata
	return bm.saveMetadata()
}

// GetStats returns backup statistics
func (bm *BackupManager) GetStats() *BackupStats {
	stats := &BackupStats{
		BackupsByDay: make(map[string]int),
	}

	fileCounts := make(map[string]int)
	var oldest, newest *time.Time

	for filePath, backups := range bm.metadata.backups {
		stats.TotalFiles++
		stats.TotalBackups += len(backups)
		fileCounts[filePath] = len(backups)

		for _, backup := range backups {
			stats.TotalSize += backup.Size

			// Track oldest/newest
			if oldest == nil || backup.Timestamp.Before(*oldest) {
				oldest = &backup.Timestamp
			}
			if newest == nil || backup.Timestamp.After(*newest) {
				newest = &backup.Timestamp
			}

			// Count by day
			day := backup.Timestamp.Format("2006-01-02")
			stats.BackupsByDay[day]++
		}
	}

	stats.OldestBackup = oldest
	stats.NewestBackup = newest

	// Find top files by backup count
	type fileCount struct {
		path  string
		count int
	}
	var fileCounts2 []fileCount
	for path, count := range fileCounts {
		fileCounts2 = append(fileCounts2, fileCount{path, count})
	}
	sort.Slice(fileCounts2, func(i, j int) bool {
		return fileCounts2[i].count > fileCounts2[j].count
	})

	maxTop := 10
	if len(fileCounts2) < maxTop {
		maxTop = len(fileCounts2)
	}
	
	for i := 0; i < maxTop; i++ {
		stats.TopFiles = append(stats.TopFiles, fileCounts2[i].path)
	}

	return stats
}

// CleanupExpired removes expired backups based on retention policy
func (bm *BackupManager) CleanupExpired() error {
	cutoff := time.Now().AddDate(0, 0, -bm.retentionDays)
	
	for filePath, backups := range bm.metadata.backups {
		var keepBackups []*BackupInfo
		
		for _, backup := range backups {
			if backup.Timestamp.Before(cutoff) {
				// Delete backup file
				os.Remove(backup.BackupPath)
			} else {
				keepBackups = append(keepBackups, backup)
			}
		}
		
		bm.metadata.backups[filePath] = keepBackups
		
		// Remove entry if no backups left
		if len(keepBackups) == 0 {
			delete(bm.metadata.backups, filePath)
		}
	}

	return bm.saveMetadata()
}

// Helper methods

func (bm *BackupManager) copyFile(src, dst string, opts BackupOptions) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if opts.Compress {
		gzWriter := gzip.NewWriter(destFile)
		defer gzWriter.Close()
		_, err = io.Copy(gzWriter, sourceFile)
	} else {
		_, err = io.Copy(destFile, sourceFile)
	}

	if err != nil {
		return err
	}

	// Preserve file mode if requested
	if opts.PreserveMode {
		sourceInfo, _ := os.Stat(src)
		destFile.Chmod(sourceInfo.Mode())
	}

	return nil
}

func (bm *BackupManager) restoreFile(backup *BackupInfo, destPath string, opts RestoreOptions) error {
	sourceFile, err := os.Open(backup.BackupPath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	var reader io.Reader = sourceFile
	if backup.Compressed {
		gzReader, err := gzip.NewReader(sourceFile)
		if err != nil {
			return err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	_, err = io.Copy(destFile, reader)
	if err != nil {
		return err
	}

	// Restore file mode if requested
	if opts.PreserveMode {
		destFile.Chmod(backup.FileMode)
	}

	// Restore modification time if requested
	if opts.PreserveTime {
		os.Chtimes(destPath, backup.ModTime, backup.ModTime)
	}

	return nil
}

func (bm *BackupManager) findBackup(filePath string, opts RestoreOptions) (*BackupInfo, error) {
	backups, exists := bm.metadata.backups[filePath]
	if !exists || len(backups) == 0 {
		return nil, fmt.Errorf("no backups found for file: %s", filePath)
	}

	// Find by backup ID
	if opts.BackupID != "" {
		for _, backup := range backups {
			if backup.ID == opts.BackupID {
				return backup, nil
			}
		}
		return nil, fmt.Errorf("backup not found: %s", opts.BackupID)
	}

	// Find by version
	if opts.Version > 0 {
		for _, backup := range backups {
			if backup.Version == opts.Version {
				return backup, nil
			}
		}
		return nil, fmt.Errorf("version not found: %d", opts.Version)
	}

	// Find by timestamp
	if opts.Timestamp != "" {
		targetTime, err := time.Parse("2006-01-02 15:04:05", opts.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp format: %v", err)
		}

		// Find closest backup before or at target time
		var closestBackup *BackupInfo
		for _, backup := range backups {
			if backup.Timestamp.Before(targetTime) || backup.Timestamp.Equal(targetTime) {
				if closestBackup == nil || backup.Timestamp.After(closestBackup.Timestamp) {
					closestBackup = backup
				}
			}
		}

		if closestBackup == nil {
			return nil, fmt.Errorf("no backup found before timestamp: %s", opts.Timestamp)
		}
		return closestBackup, nil
	}

	// Return most recent backup
	return backups[0], nil
}

func (bm *BackupManager) calculateChecksum(filePath string) (string, error) {
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

func (bm *BackupManager) generateBackupID(filePath string) string {
	return fmt.Sprintf("backup_%d_%s", time.Now().Unix(), bm.sanitizePath(filePath))
}

func (bm *BackupManager) sanitizePath(path string) string {
	// Replace path separators and other problematic characters
	sanitized := strings.ReplaceAll(path, string(os.PathSeparator), "_")
	sanitized = strings.ReplaceAll(sanitized, ":", "_")
	sanitized = strings.ReplaceAll(sanitized, " ", "_")
	return sanitized
}

func (bm *BackupManager) getNextVersion(filePath string) int {
	backups, exists := bm.metadata.backups[filePath]
	if !exists || len(backups) == 0 {
		return 1
	}

	maxVersion := 0
	for _, backup := range backups {
		if backup.Version > maxVersion {
			maxVersion = backup.Version
		}
	}

	return maxVersion + 1
}

func (bm *BackupManager) cleanupOldBackups(filePath string) error {
	backups, exists := bm.metadata.backups[filePath]
	if !exists || len(backups) <= bm.maxBackups {
		return nil
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	// Keep only maxBackups
	toDelete := backups[bm.maxBackups:]
	keepBackups := backups[:bm.maxBackups]

	// Delete old backup files
	for _, backup := range toDelete {
		os.Remove(backup.BackupPath)
	}

	// Update metadata
	bm.metadata.backups[filePath] = keepBackups

	return nil
}

func (bm *BackupManager) cleanupEmptyDirs(dirPath string) {
	if dirPath == bm.backupDir {
		return // Don't delete the root backup directory
	}

	if entries, err := os.ReadDir(dirPath); err == nil && len(entries) == 0 {
		os.Remove(dirPath)
		// Recursively clean up parent directories
		bm.cleanupEmptyDirs(filepath.Dir(dirPath))
	}
}

func (bm *BackupManager) loadMetadata() error {
	metadataPath := filepath.Join(bm.backupDir, "metadata.json")
	
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		// Metadata doesn't exist, start fresh
		return nil
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, bm.metadata)
}

func (bm *BackupManager) saveMetadata() error {
	metadataPath := filepath.Join(bm.backupDir, "metadata.json")
	
	data, err := json.MarshalIndent(bm.metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// FileBackupExecutor provides backup operations as a tool executor
type FileBackupExecutor struct {
	manager *BackupManager
}

// NewFileBackupExecutor creates a new file backup executor
func NewFileBackupExecutor(backupDir string, maxBackups int) (*FileBackupExecutor, error) {
	manager, err := NewBackupManager(backupDir, maxBackups)
	if err != nil {
		return nil, err
	}

	return &FileBackupExecutor{
		manager: manager,
	}, nil
}

// Execute performs backup operations
func (e *FileBackupExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		action = "create"
	}

	switch action {
	case "create":
		return e.createBackup(params)
	case "restore":
		return e.restoreBackup(params)
	case "list":
		return e.listBackups(params)
	case "delete":
		return e.deleteBackup(params)
	case "stats":
		return e.getStats(params)
	case "cleanup":
		return e.cleanup(params)
	default:
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("unknown action: %s", action),
		}, fmt.Errorf("unknown action: %s", action)
	}
}

func (e *FileBackupExecutor) createBackup(params map[string]interface{}) (*ToolResult, error) {
	filePath, _ := params["file_path"].(string)
	if filePath == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "file_path parameter required",
		}, fmt.Errorf("file_path parameter required")
	}

	opts := BackupOptions{
		Compress:     true,
		PreserveMode: true,
		PreserveTime: true,
	}

	if desc, ok := params["description"].(string); ok {
		opts.Description = desc
	}

	if compress, ok := params["compress"].(bool); ok {
		opts.Compress = compress
	}

	backup, err := e.manager.CreateBackup(filePath, opts)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to create backup: %v", err),
		}, err
	}

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"backup_id":     backup.ID,
			"backup_path":   backup.BackupPath,
			"version":       backup.Version,
			"size":          backup.Size,
			"timestamp":     backup.Timestamp,
			"checksum":      backup.Checksum,
		},
		Metadata: map[string]interface{}{
			"action": "create",
		},
	}, nil
}

func (e *FileBackupExecutor) restoreBackup(params map[string]interface{}) (*ToolResult, error) {
	filePath, _ := params["file_path"].(string)
	if filePath == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "file_path parameter required",
		}, fmt.Errorf("file_path parameter required")
	}

	opts := RestoreOptions{
		CreateBackup:   true,
		VerifyChecksum: true,
		PreserveMode:   true,
		PreserveTime:   true,
	}

	if backupID, ok := params["backup_id"].(string); ok {
		opts.BackupID = backupID
	}

	if version, ok := params["version"].(int); ok {
		opts.Version = version
	}

	if timestamp, ok := params["timestamp"].(string); ok {
		opts.Timestamp = timestamp
	}

	err := e.manager.RestoreBackup(filePath, opts)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to restore backup: %v", err),
		}, err
	}

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"restored": true,
			"file_path": filePath,
		},
		Metadata: map[string]interface{}{
			"action": "restore",
		},
	}, nil
}

func (e *FileBackupExecutor) listBackups(params map[string]interface{}) (*ToolResult, error) {
	filePath, _ := params["file_path"].(string)

	backups, err := e.manager.ListBackups(filePath)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to list backups: %v", err),
		}, err
	}

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"backups": backups,
			"count":   len(backups),
		},
		Metadata: map[string]interface{}{
			"action": "list",
		},
	}, nil
}

func (e *FileBackupExecutor) deleteBackup(params map[string]interface{}) (*ToolResult, error) {
	backupID, _ := params["backup_id"].(string)
	if backupID == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "backup_id parameter required",
		}, fmt.Errorf("backup_id parameter required")
	}

	err := e.manager.DeleteBackup(backupID)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to delete backup: %v", err),
		}, err
	}

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"deleted":   true,
			"backup_id": backupID,
		},
		Metadata: map[string]interface{}{
			"action": "delete",
		},
	}, nil
}

func (e *FileBackupExecutor) getStats(params map[string]interface{}) (*ToolResult, error) {
	stats := e.manager.GetStats()

	return &ToolResult{
		Success: true,
		Result:  stats,
		Metadata: map[string]interface{}{
			"action": "stats",
		},
	}, nil
}

func (e *FileBackupExecutor) cleanup(params map[string]interface{}) (*ToolResult, error) {
	err := e.manager.CleanupExpired()
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to cleanup: %v", err),
		}, err
	}

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"cleanup_completed": true,
		},
		Metadata: map[string]interface{}{
			"action": "cleanup",
		},
	}, nil
}

// Name returns the name of the executor
func (e *FileBackupExecutor) Name() string {
	return "file_backup"
}

// Description returns the description of the executor
func (e *FileBackupExecutor) Description() string {
	return "Create and manage file backups with rollback capabilities"
}