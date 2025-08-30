package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// FileMetadata represents comprehensive file metadata information
type FileMetadata struct {
	FilePath     string      `json:"file_path"`
	Name         string      `json:"name"`
	Size         int64       `json:"size"`
	Mode         os.FileMode `json:"mode"`
	ModeString   string      `json:"mode_string"`
	IsDirectory  bool        `json:"is_directory"`
	IsRegular    bool        `json:"is_regular"`
	IsSymlink    bool        `json:"is_symlink"`
	ModTime      time.Time   `json:"mod_time"`
	AccessTime   time.Time   `json:"access_time"`
	ChangeTime   time.Time   `json:"change_time"`
	CreateTime   time.Time   `json:"create_time,omitempty"`
	Owner        string      `json:"owner"`
	Group        string      `json:"group"`
	UID          uint32      `json:"uid"`
	GID          uint32      `json:"gid"`
	DeviceID     uint64      `json:"device_id"`
	InodeID      uint64      `json:"inode_id"`
	HardLinks    uint64      `json:"hard_links"`
	BlockSize    int64       `json:"block_size,omitempty"`
	Blocks       int64       `json:"blocks,omitempty"`
	Checksum     string      `json:"checksum,omitempty"`
	MimeType     string      `json:"mime_type,omitempty"`
	Extension    string      `json:"extension"`
	BaseName     string      `json:"base_name"`
	DirName      string      `json:"dir_name"`
	CollectedAt  time.Time   `json:"collected_at"`
}

// FileMetadataTracker handles file metadata collection and tracking
type FileMetadataTracker struct {
	includeChecksums bool
	includeExtended  bool
}

// NewFileMetadataTracker creates a new file metadata tracker
func NewFileMetadataTracker(includeChecksums, includeExtended bool) *FileMetadataTracker {
	return &FileMetadataTracker{
		includeChecksums: includeChecksums,
		includeExtended:  includeExtended,
	}
}

// CollectMetadata collects comprehensive metadata for a file
func (f *FileMetadataTracker) CollectMetadata(filePath string) (*FileMetadata, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	metadata := &FileMetadata{
		FilePath:    filePath,
		Name:        info.Name(),
		Size:        info.Size(),
		Mode:        info.Mode(),
		ModeString:  info.Mode().String(),
		IsDirectory: info.IsDir(),
		IsRegular:   info.Mode().IsRegular(),
		IsSymlink:   info.Mode()&os.ModeSymlink != 0,
		ModTime:     info.ModTime(),
		Extension:   filepath.Ext(filePath),
		BaseName:    filepath.Base(filePath),
		DirName:     filepath.Dir(filePath),
		CollectedAt: time.Now(),
	}

	// Get system-specific metadata
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		metadata.UID = stat.Uid
		metadata.GID = stat.Gid
		metadata.DeviceID = stat.Dev
		metadata.InodeID = stat.Ino
		metadata.HardLinks = uint64(stat.Nlink)
		metadata.BlockSize = int64(stat.Blksize)
		metadata.Blocks = stat.Blocks

		// Convert timestamps
		metadata.AccessTime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
		metadata.ChangeTime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
		
		// Birth time (creation time) - Linux doesn't have reliable birth time
		// We'll use change time as approximation
		metadata.CreateTime = metadata.ChangeTime
	}

	// Get owner and group names
	if f.includeExtended {
		metadata.Owner, metadata.Group = f.getOwnerGroup(metadata.UID, metadata.GID)
		metadata.MimeType = f.detectMimeType(filePath)
	}

	// Calculate checksum if requested
	if f.includeChecksums && metadata.IsRegular {
		checksum, err := f.calculateChecksum(filePath)
		if err == nil {
			metadata.Checksum = checksum
		}
	}

	return metadata, nil
}

// getOwnerGroup gets owner and group names from UID/GID (Unix-specific)
func (f *FileMetadataTracker) getOwnerGroup(uid, gid uint32) (string, string) {
	// This is a simplified version - in practice you'd use user.LookupId and group.LookupId
	// but those require cgo which complicates the build
	return fmt.Sprintf("uid-%d", uid), fmt.Sprintf("gid-%d", gid)
}

// detectMimeType detects the MIME type of a file
func (f *FileMetadataTracker) detectMimeType(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".txt", ".md":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".java":
		return "text/x-java-source"
	case ".c", ".h":
		return "text/x-c"
	case ".cpp", ".hpp":
		return "text/x-c++"
	case ".rs":
		return "text/x-rust"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	default:
		return "application/octet-stream"
	}
}

// calculateChecksum calculates SHA256 checksum of a file
func (f *FileMetadataTracker) calculateChecksum(filePath string) (string, error) {
	return calculateFileSHA256(filePath)
}

// FileMetadataExecutor provides file metadata collection as a tool
type FileMetadataExecutor struct {
	tracker *FileMetadataTracker
}

// NewFileMetadataExecutor creates a new file metadata executor
func NewFileMetadataExecutor() *FileMetadataExecutor {
	return &FileMetadataExecutor{
		tracker: NewFileMetadataTracker(false, true),
	}
}

// Execute collects metadata for specified files
func (f *FileMetadataExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	files, ok := params["files"].([]interface{})
	if !ok {
		// Single file mode
		if filePath, ok := params["file_path"].(string); ok {
			files = []interface{}{filePath}
		} else {
			return &ToolResult{
				Success: false,
				ErrorMessage:   "files or file_path parameter is required",
			}, nil
		}
	}

	includeChecksums, _ := params["include_checksums"].(bool)
	includeExtended, _ := params["include_extended"].(bool)
	format, _ := params["format"].(string)
	if format == "" {
		format = "json"
	}

	// Update tracker settings
	f.tracker.includeChecksums = includeChecksums
	f.tracker.includeExtended = includeExtended

	var results []FileMetadata
	var errors []string

	for _, file := range files {
		filePath, ok := file.(string)
		if !ok {
			continue
		}

		metadata, err := f.tracker.CollectMetadata(filePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		results = append(results, *metadata)
	}

	// Format output
	var output interface{}
	switch format {
	case "json":
		output = results
	case "summary":
		output = f.createSummary(results)
	case "table":
		output = f.createTable(results)
	default:
		output = results
	}

	return &ToolResult{
		Success: len(results) > 0,
		Result:  output,
		Metadata: map[string]interface{}{
			"total_files":       len(files),
			"successful_files":  len(results),
			"failed_files":      len(errors),
			"errors":           errors,
			"include_checksums": includeChecksums,
			"include_extended":  includeExtended,
			"format":           format,
			"collected_at":     time.Now().Format(time.RFC3339),
		},
	}, nil
}

// createSummary creates a summary view of metadata
func (f *FileMetadataExecutor) createSummary(metadata []FileMetadata) map[string]interface{} {
	summary := map[string]interface{}{
		"total_files": len(metadata),
		"total_size":  int64(0),
		"file_types":  make(map[string]int),
		"directories": 0,
		"regular_files": 0,
		"symlinks":    0,
		"permissions": make(map[string]int),
	}

	for _, meta := range metadata {
		summary["total_size"] = summary["total_size"].(int64) + meta.Size
		
		if meta.IsDirectory {
			summary["directories"] = summary["directories"].(int) + 1
		} else if meta.IsRegular {
			summary["regular_files"] = summary["regular_files"].(int) + 1
		} else if meta.IsSymlink {
			summary["symlinks"] = summary["symlinks"].(int) + 1
		}

		// Count file types by extension
		fileTypes := summary["file_types"].(map[string]int)
		ext := meta.Extension
		if ext == "" {
			ext = "no_extension"
		}
		fileTypes[ext]++

		// Count permissions
		permissions := summary["permissions"].(map[string]int)
		permissions[meta.ModeString]++
	}

	return summary
}

// createTable creates a table view of metadata
func (f *FileMetadataExecutor) createTable(metadata []FileMetadata) []map[string]interface{} {
	var table []map[string]interface{}

	for _, meta := range metadata {
		row := map[string]interface{}{
			"name":        meta.Name,
			"size":        meta.Size,
			"permissions": meta.ModeString,
			"modified":    meta.ModTime.Format("2006-01-02 15:04:05"),
			"type":        f.getFileType(meta),
		}

		if meta.Extension != "" {
			row["extension"] = meta.Extension
		}

		if meta.Checksum != "" {
			row["checksum"] = meta.Checksum[:16] + "..." // Truncated for display
		}

		table = append(table, row)
	}

	return table
}

// getFileType returns a human-readable file type
func (f *FileMetadataExecutor) getFileType(meta FileMetadata) string {
	if meta.IsDirectory {
		return "directory"
	}
	if meta.IsSymlink {
		return "symlink"
	}
	if meta.MimeType != "" {
		return meta.MimeType
	}
	return "file"
}

// FileMetadataPreservationExecutor handles preserving and restoring file metadata
type FileMetadataPreservationExecutor struct {
	tracker *FileMetadataTracker
}

// NewFileMetadataPreservationExecutor creates a new metadata preservation executor
func NewFileMetadataPreservationExecutor() *FileMetadataPreservationExecutor {
	return &FileMetadataPreservationExecutor{
		tracker: NewFileMetadataTracker(false, true),
	}
}

// Execute preserves or restores file metadata
func (f *FileMetadataPreservationExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "operation parameter is required (preserve, restore)",
		}, nil
	}

	switch operation {
	case "preserve":
		return f.preserveMetadata(params)
	case "restore":
		return f.restoreMetadata(params)
	default:
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Unknown operation: %s", operation),
		}, nil
	}
}

// preserveMetadata saves current metadata to a file
func (f *FileMetadataPreservationExecutor) preserveMetadata(params ToolParameters) (*ToolResult, error) {
	files, ok := params["files"].([]interface{})
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "files parameter is required",
		}, nil
	}

	outputFile, ok := params["output_file"].(string)
	if !ok {
		outputFile = "metadata_backup.json"
	}

	var metadata []FileMetadata
	var errors []string

	for _, file := range files {
		filePath, ok := file.(string)
		if !ok {
			continue
		}

		meta, err := f.tracker.CollectMetadata(filePath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		metadata = append(metadata, *meta)
	}

	// Save metadata to file
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to marshal metadata: %v", err),
		}, nil
	}

	err = os.WriteFile(outputFile, data, 0644)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to write metadata file: %v", err),
		}, nil
	}

	return &ToolResult{
		Success: true,
		Result:  fmt.Sprintf("Preserved metadata for %d files to %s", len(metadata), outputFile),
		Metadata: map[string]interface{}{
			"output_file":      outputFile,
			"files_processed":  len(metadata),
			"errors":          errors,
			"preserved_at":    time.Now().Format(time.RFC3339),
		},
	}, nil
}

// restoreMetadata restores metadata from a saved file
func (f *FileMetadataPreservationExecutor) restoreMetadata(params ToolParameters) (*ToolResult, error) {
	inputFile, ok := params["input_file"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "input_file parameter is required",
		}, nil
	}

	restorePermissions, _ := params["restore_permissions"].(bool)
	restoreTimestamps, _ := params["restore_timestamps"].(bool)
	dryRun, _ := params["dry_run"].(bool)

	// Read metadata file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to read metadata file: %v", err),
		}, nil
	}

	var metadata []FileMetadata
	err = json.Unmarshal(data, &metadata)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to unmarshal metadata: %v", err),
		}, nil
	}

	var results []map[string]interface{}
	successCount := 0

	for _, meta := range metadata {
		result := map[string]interface{}{
			"file_path": meta.FilePath,
			"success":   false,
		}

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
			result["would_restore"] = []string{}
			
			if restorePermissions {
				result["would_restore"] = append(result["would_restore"].([]string), "permissions")
			}
			if restoreTimestamps {
				result["would_restore"] = append(result["would_restore"].([]string), "timestamps")
			}
			
			results = append(results, result)
			successCount++
			continue
		}

		// Check if file exists
		if _, err := os.Stat(meta.FilePath); os.IsNotExist(err) {
			result["error"] = "File does not exist"
			results = append(results, result)
			continue
		}

		restored := []string{}

		// Restore permissions
		if restorePermissions {
			err := os.Chmod(meta.FilePath, meta.Mode)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to restore permissions: %v", err)
				results = append(results, result)
				continue
			}
			restored = append(restored, "permissions")
		}

		// Restore timestamps
		if restoreTimestamps {
			err := os.Chtimes(meta.FilePath, meta.AccessTime, meta.ModTime)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to restore timestamps: %v", err)
				results = append(results, result)
				continue
			}
			restored = append(restored, "timestamps")
		}

		result["success"] = true
		result["restored"] = restored
		results = append(results, result)
		successCount++
	}

	return &ToolResult{
		Success: successCount > 0 || dryRun,
		Result:  results,
		Metadata: map[string]interface{}{
			"input_file":         inputFile,
			"total_files":        len(metadata),
			"success_count":      successCount,
			"failure_count":      len(metadata) - successCount,
			"restore_permissions": restorePermissions,
			"restore_timestamps": restoreTimestamps,
			"dry_run":           dryRun,
			"restored_at":       time.Now().Format(time.RFC3339),
		},
	}, nil
}

// FileMetadataComparisonExecutor compares metadata between files or against saved metadata
type FileMetadataComparisonExecutor struct {
	tracker *FileMetadataTracker
}

// NewFileMetadataComparisonExecutor creates a new metadata comparison executor
func NewFileMetadataComparisonExecutor() *FileMetadataComparisonExecutor {
	return &FileMetadataComparisonExecutor{
		tracker: NewFileMetadataTracker(true, true),
	}
}

// Execute compares file metadata
func (f *FileMetadataComparisonExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	mode, ok := params["mode"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "mode parameter is required (files, snapshot)",
		}, nil
	}

	switch mode {
	case "files":
		return f.compareFiles(params)
	case "snapshot":
		return f.compareWithSnapshot(params)
	default:
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Unknown comparison mode: %s", mode),
		}, nil
	}
}

// compareFiles compares metadata between two files
func (f *FileMetadataComparisonExecutor) compareFiles(params ToolParameters) (*ToolResult, error) {
	file1, ok1 := params["file1"].(string)
	file2, ok2 := params["file2"].(string)
	
	if !ok1 || !ok2 {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "file1 and file2 parameters are required",
		}, nil
	}

	meta1, err := f.tracker.CollectMetadata(file1)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to get metadata for %s: %v", file1, err),
		}, nil
	}

	meta2, err := f.tracker.CollectMetadata(file2)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to get metadata for %s: %v", file2, err),
		}, nil
	}

	comparison := f.compareMetadata(*meta1, *meta2)

	return &ToolResult{
		Success: true,
		Result:  comparison,
		Metadata: map[string]interface{}{
			"file1":       file1,
			"file2":       file2,
			"compared_at": time.Now().Format(time.RFC3339),
		},
	}, nil
}

// compareWithSnapshot compares current file metadata with a saved snapshot
func (f *FileMetadataComparisonExecutor) compareWithSnapshot(params ToolParameters) (*ToolResult, error) {
	snapshotFile, ok := params["snapshot_file"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "snapshot_file parameter is required",
		}, nil
	}

	// Read snapshot
	data, err := os.ReadFile(snapshotFile)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to read snapshot file: %v", err),
		}, nil
	}

	var snapshotMetadata []FileMetadata
	err = json.Unmarshal(data, &snapshotMetadata)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to unmarshal snapshot: %v", err),
		}, nil
	}

	var results []map[string]interface{}
	changedCount := 0
	newCount := 0
	deletedCount := 0

	// Create map of current metadata for quick lookup
	currentMap := make(map[string]FileMetadata)
	for _, meta := range snapshotMetadata {
		if currentMeta, err := f.tracker.CollectMetadata(meta.FilePath); err == nil {
			currentMap[meta.FilePath] = *currentMeta
		}
	}

	// Compare each file in snapshot with current state
	for _, snapshotMeta := range snapshotMetadata {
		result := map[string]interface{}{
			"file_path": snapshotMeta.FilePath,
		}

		currentMeta, exists := currentMap[snapshotMeta.FilePath]
		if !exists {
			result["status"] = "deleted"
			deletedCount++
		} else {
			comparison := f.compareMetadata(snapshotMeta, currentMeta)
			if len(comparison["differences"].([]map[string]interface{})) > 0 {
				result["status"] = "changed"
				result["changes"] = comparison["differences"]
				changedCount++
			} else {
				result["status"] = "unchanged"
			}
		}

		results = append(results, result)
	}

	return &ToolResult{
		Success: true,
		Result:  results,
		Metadata: map[string]interface{}{
			"snapshot_file":  snapshotFile,
			"total_files":    len(snapshotMetadata),
			"changed_files":  changedCount,
			"deleted_files":  deletedCount,
			"new_files":      newCount,
			"compared_at":    time.Now().Format(time.RFC3339),
		},
	}, nil
}

// compareMetadata compares two FileMetadata objects and returns differences
func (f *FileMetadataComparisonExecutor) compareMetadata(meta1, meta2 FileMetadata) map[string]interface{} {
	var differences []map[string]interface{}

	// Compare key fields
	if meta1.Size != meta2.Size {
		differences = append(differences, map[string]interface{}{
			"field": "size",
			"old":   meta1.Size,
			"new":   meta2.Size,
		})
	}

	if meta1.Mode != meta2.Mode {
		differences = append(differences, map[string]interface{}{
			"field": "permissions",
			"old":   meta1.ModeString,
			"new":   meta2.ModeString,
		})
	}

	if !meta1.ModTime.Equal(meta2.ModTime) {
		differences = append(differences, map[string]interface{}{
			"field": "mod_time",
			"old":   meta1.ModTime.Format(time.RFC3339),
			"new":   meta2.ModTime.Format(time.RFC3339),
		})
	}

	if meta1.Checksum != "" && meta2.Checksum != "" && meta1.Checksum != meta2.Checksum {
		differences = append(differences, map[string]interface{}{
			"field": "checksum",
			"old":   meta1.Checksum,
			"new":   meta2.Checksum,
		})
	}

	return map[string]interface{}{
		"identical":   len(differences) == 0,
		"differences": differences,
		"file1":       meta1.FilePath,
		"file2":       meta2.FilePath,
	}
}