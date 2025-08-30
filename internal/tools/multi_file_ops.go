package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// BatchEditExecutor handles batch editing operations across multiple files
type BatchEditExecutor struct {
	backupManager *BackupManager
}

// NewBatchEditExecutor creates a new batch edit executor
func NewBatchEditExecutor() *BatchEditExecutor {
	manager, _ := NewBackupManager("", 100)
	return &BatchEditExecutor{
		backupManager: manager,
	}
}

// Execute performs batch editing operations on multiple files
func (b *BatchEditExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	files, ok := params["files"].([]interface{})
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "files parameter must be an array of file paths",
		}, nil
	}

	operation, ok := params["operation"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "operation parameter is required (find_replace, line_edit, etc.)",
		}, nil
	}

	// Convert files to string slice
	filePaths := make([]string, len(files))
	for i, file := range files {
		if path, ok := file.(string); ok {
			filePaths[i] = path
		} else {
			return &ToolResult{
				Success: false,
				ErrorMessage:   fmt.Sprintf("File %d must be a string path", i),
			}, nil
		}
	}

	// Create backups before making changes
	createBackups, _ := params["create_backup"].(bool)
	if createBackups {
		for _, filePath := range filePaths {
			_, err := b.backupManager.CreateBackup(filePath, BackupOptions{})
			if err != nil {
				return &ToolResult{
					Success: false,
					ErrorMessage:   fmt.Sprintf("Failed to create backup for %s: %v", filePath, err),
				}, nil
			}
		}
	}

	var results []map[string]interface{}
	successCount := 0

	switch operation {
	case "find_replace":
		results, successCount = b.executeFindReplace(filePaths, params)
	case "line_edit":
		results, successCount = b.executeLineEdit(filePaths, params)
	case "prepend":
		results, successCount = b.executePrepend(filePaths, params)
	case "append":
		results, successCount = b.executeAppend(filePaths, params)
	case "insert_at_line":
		results, successCount = b.executeInsertAtLine(filePaths, params)
	default:
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Unknown operation: %s", operation),
		}, nil
	}

	return &ToolResult{
		Success: successCount > 0,
		Result:  results,
		Metadata: map[string]interface{}{
			"operation":      operation,
			"total_files":    len(filePaths),
			"success_count":  successCount,
			"failure_count":  len(filePaths) - successCount,
			"backup_created": createBackups,
			"processed_at":   time.Now().Format(time.RFC3339),
		},
	}, nil
}

// executeFindReplace performs find and replace operations across files
func (b *BatchEditExecutor) executeFindReplace(files []string, params ToolParameters) ([]map[string]interface{}, int) {
	find, _ := params["find"].(string)
	replace, _ := params["replace"].(string)
	useRegex, _ := params["use_regex"].(bool)
	caseSensitive, _ := params["case_sensitive"].(bool)

	var results []map[string]interface{}
	successCount := 0

	for _, filePath := range files {
		result := map[string]interface{}{
			"file_path": filePath,
			"success":   false,
		}

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to read file: %v", err)
			results = append(results, result)
			continue
		}

		originalContent := string(content)
		var newContent string
		var replacements int

		if useRegex {
			flags := ""
			if !caseSensitive {
				flags = "(?i)"
			}
			pattern := flags + find
			
			re, err := regexp.Compile(pattern)
			if err != nil {
				result["error"] = fmt.Sprintf("Invalid regex pattern: %v", err)
				results = append(results, result)
				continue
			}

			matches := re.FindAllString(originalContent, -1)
			replacements = len(matches)
			newContent = re.ReplaceAllString(originalContent, replace)
		} else {
			if caseSensitive {
				replacements = strings.Count(originalContent, find)
				newContent = strings.ReplaceAll(originalContent, find, replace)
			} else {
				// Case-insensitive replacement
				lowerContent := strings.ToLower(originalContent)
				lowerFind := strings.ToLower(find)
				replacements = strings.Count(lowerContent, lowerFind)
				
				// Manual case-insensitive replacement
				newContent = originalContent
				start := 0
				for {
					index := strings.Index(strings.ToLower(newContent[start:]), lowerFind)
					if index == -1 {
						break
					}
					actualIndex := start + index
					newContent = newContent[:actualIndex] + replace + newContent[actualIndex+len(find):]
					start = actualIndex + len(replace)
				}
			}
		}

		// Write the modified content back to file
		if replacements > 0 {
			err = os.WriteFile(filePath, []byte(newContent), 0644)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to write file: %v", err)
				results = append(results, result)
				continue
			}
		}

		result["success"] = true
		result["replacements"] = replacements
		result["modified"] = replacements > 0
		results = append(results, result)
		successCount++
	}

	return results, successCount
}

// executeLineEdit performs line-based editing operations
func (b *BatchEditExecutor) executeLineEdit(files []string, params ToolParameters) ([]map[string]interface{}, int) {
	lineNumber, _ := params["line_number"].(float64)
	newContent, _ := params["line_content"].(string)
	operation, _ := params["line_operation"].(string) // replace, delete

	var results []map[string]interface{}
	successCount := 0

	for _, filePath := range files {
		result := map[string]interface{}{
			"file_path": filePath,
			"success":   false,
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to read file: %v", err)
			results = append(results, result)
			continue
		}

		lines := strings.Split(string(content), "\n")
		lineIdx := int(lineNumber) - 1 // Convert to 0-based index

		if lineIdx < 0 || lineIdx >= len(lines) {
			result["error"] = fmt.Sprintf("Line number %d is out of range (1-%d)", int(lineNumber), len(lines))
			results = append(results, result)
			continue
		}

		originalLine := lines[lineIdx]

		switch operation {
		case "replace":
			lines[lineIdx] = newContent
		case "delete":
			lines = append(lines[:lineIdx], lines[lineIdx+1:]...)
		default:
			result["error"] = fmt.Sprintf("Unknown line operation: %s", operation)
			results = append(results, result)
			continue
		}

		newFileContent := strings.Join(lines, "\n")
		err = os.WriteFile(filePath, []byte(newFileContent), 0644)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to write file: %v", err)
			results = append(results, result)
			continue
		}

		result["success"] = true
		result["original_line"] = originalLine
		result["operation"] = operation
		if operation == "replace" {
			result["new_line"] = newContent
		}
		results = append(results, result)
		successCount++
	}

	return results, successCount
}

// executePrepend adds content to the beginning of files
func (b *BatchEditExecutor) executePrepend(files []string, params ToolParameters) ([]map[string]interface{}, int) {
	content, _ := params["content"].(string)
	addNewline, _ := params["add_newline"].(bool)

	var results []map[string]interface{}
	successCount := 0

	for _, filePath := range files {
		result := map[string]interface{}{
			"file_path": filePath,
			"success":   false,
		}

		originalContent, err := os.ReadFile(filePath)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to read file: %v", err)
			results = append(results, result)
			continue
		}

		var newContent string
		if addNewline && len(originalContent) > 0 {
			newContent = content + "\n" + string(originalContent)
		} else {
			newContent = content + string(originalContent)
		}

		err = os.WriteFile(filePath, []byte(newContent), 0644)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to write file: %v", err)
			results = append(results, result)
			continue
		}

		result["success"] = true
		result["content_added"] = content
		results = append(results, result)
		successCount++
	}

	return results, successCount
}

// executeAppend adds content to the end of files
func (b *BatchEditExecutor) executeAppend(files []string, params ToolParameters) ([]map[string]interface{}, int) {
	content, _ := params["content"].(string)
	addNewline, _ := params["add_newline"].(bool)

	var results []map[string]interface{}
	successCount := 0

	for _, filePath := range files {
		result := map[string]interface{}{
			"file_path": filePath,
			"success":   false,
		}

		originalContent, err := os.ReadFile(filePath)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to read file: %v", err)
			results = append(results, result)
			continue
		}

		var newContent string
		if addNewline && len(originalContent) > 0 {
			newContent = string(originalContent) + "\n" + content
		} else {
			newContent = string(originalContent) + content
		}

		err = os.WriteFile(filePath, []byte(newContent), 0644)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to write file: %v", err)
			results = append(results, result)
			continue
		}

		result["success"] = true
		result["content_added"] = content
		results = append(results, result)
		successCount++
	}

	return results, successCount
}

// executeInsertAtLine inserts content at a specific line in files
func (b *BatchEditExecutor) executeInsertAtLine(files []string, params ToolParameters) ([]map[string]interface{}, int) {
	lineNumber, _ := params["line_number"].(float64)
	content, _ := params["content"].(string)

	var results []map[string]interface{}
	successCount := 0

	for _, filePath := range files {
		result := map[string]interface{}{
			"file_path": filePath,
			"success":   false,
		}

		originalContent, err := os.ReadFile(filePath)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to read file: %v", err)
			results = append(results, result)
			continue
		}

		lines := strings.Split(string(originalContent), "\n")
		lineIdx := int(lineNumber) - 1 // Convert to 0-based index

		if lineIdx < 0 || lineIdx > len(lines) {
			result["error"] = fmt.Sprintf("Line number %d is out of range (1-%d)", int(lineNumber), len(lines)+1)
			results = append(results, result)
			continue
		}

		// Insert the content at the specified line
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:lineIdx]...)
		newLines = append(newLines, content)
		newLines = append(newLines, lines[lineIdx:]...)

		newContent := strings.Join(newLines, "\n")
		err = os.WriteFile(filePath, []byte(newContent), 0644)
		if err != nil {
			result["error"] = fmt.Sprintf("Failed to write file: %v", err)
			results = append(results, result)
			continue
		}

		result["success"] = true
		result["content_inserted"] = content
		result["at_line"] = int(lineNumber)
		results = append(results, result)
		successCount++
	}

	return results, successCount
}

// BatchRenameExecutor handles batch file and directory renaming operations
type BatchRenameExecutor struct {
	backupManager *BackupManager
}

// NewBatchRenameExecutor creates a new batch rename executor
func NewBatchRenameExecutor() *BatchRenameExecutor {
	manager, _ := NewBackupManager("", 100)
	return &BatchRenameExecutor{
		backupManager: manager,
	}
}

// Execute performs batch renaming operations
func (b *BatchRenameExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	pattern, ok := params["pattern"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "pattern parameter is required",
		}, nil
	}

	directory, ok := params["directory"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "directory parameter is required",
		}, nil
	}

	operation, _ := params["operation"].(string)
	if operation == "" {
		operation = "rename"
	}

	recursive, _ := params["recursive"].(bool)
	dryRun, _ := params["dry_run"].(bool)

	var files []string
	var err error

	// Find files matching the pattern
	if recursive {
		err = filepath.WalkDir(directory, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			
			matched, matchErr := filepath.Match(pattern, filepath.Base(path))
			if matchErr != nil {
				return matchErr
			}
			
			if matched {
				files = append(files, path)
			}
			return nil
		})
	} else {
		matches, err := filepath.Glob(filepath.Join(directory, pattern))
		if err == nil {
			files = matches
		}
	}

	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to find files: %v", err),
		}, nil
	}

	var results []map[string]interface{}
	successCount := 0

	switch operation {
	case "rename":
		results, successCount = b.executeRename(files, params, dryRun)
	case "add_prefix":
		results, successCount = b.executeAddPrefix(files, params, dryRun)
	case "add_suffix":
		results, successCount = b.executeAddSuffix(files, params, dryRun)
	case "change_extension":
		results, successCount = b.executeChangeExtension(files, params, dryRun)
	case "remove_extension":
		results, successCount = b.executeRemoveExtension(files, dryRun)
	default:
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Unknown operation: %s", operation),
		}, nil
	}

	return &ToolResult{
		Success: successCount > 0 || dryRun,
		Result:  results,
		Metadata: map[string]interface{}{
			"operation":     operation,
			"pattern":       pattern,
			"directory":     directory,
			"recursive":     recursive,
			"dry_run":       dryRun,
			"total_files":   len(files),
			"success_count": successCount,
			"failure_count": len(files) - successCount,
			"processed_at":  time.Now().Format(time.RFC3339),
		},
	}, nil
}

// executeRename performs file renaming with find/replace
func (b *BatchRenameExecutor) executeRename(files []string, params ToolParameters, dryRun bool) ([]map[string]interface{}, int) {
	find, _ := params["find"].(string)
	replace, _ := params["replace"].(string)
	useRegex, _ := params["use_regex"].(bool)

	var results []map[string]interface{}
	successCount := 0

	for _, oldPath := range files {
		result := map[string]interface{}{
			"old_path": oldPath,
			"success":  false,
		}

		oldName := filepath.Base(oldPath)
		var newName string

		if useRegex {
			re, err := regexp.Compile(find)
			if err != nil {
				result["error"] = fmt.Sprintf("Invalid regex: %v", err)
				results = append(results, result)
				continue
			}
			newName = re.ReplaceAllString(oldName, replace)
		} else {
			newName = strings.ReplaceAll(oldName, find, replace)
		}

		if newName == oldName {
			result["error"] = "No changes needed"
			results = append(results, result)
			continue
		}

		newPath := filepath.Join(filepath.Dir(oldPath), newName)
		result["new_path"] = newPath

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
		} else {
			err := os.Rename(oldPath, newPath)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to rename: %v", err)
			} else {
				result["success"] = true
			}
		}

		results = append(results, result)
		if result["success"].(bool) {
			successCount++
		}
	}

	return results, successCount
}

// executeAddPrefix adds a prefix to filenames
func (b *BatchRenameExecutor) executeAddPrefix(files []string, params ToolParameters, dryRun bool) ([]map[string]interface{}, int) {
	prefix, _ := params["prefix"].(string)

	var results []map[string]interface{}
	successCount := 0

	for _, oldPath := range files {
		result := map[string]interface{}{
			"old_path": oldPath,
			"success":  false,
		}

		oldName := filepath.Base(oldPath)
		newName := prefix + oldName
		newPath := filepath.Join(filepath.Dir(oldPath), newName)
		result["new_path"] = newPath

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
		} else {
			err := os.Rename(oldPath, newPath)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to rename: %v", err)
			} else {
				result["success"] = true
			}
		}

		results = append(results, result)
		if result["success"].(bool) {
			successCount++
		}
	}

	return results, successCount
}

// executeAddSuffix adds a suffix to filenames (before extension)
func (b *BatchRenameExecutor) executeAddSuffix(files []string, params ToolParameters, dryRun bool) ([]map[string]interface{}, int) {
	suffix, _ := params["suffix"].(string)

	var results []map[string]interface{}
	successCount := 0

	for _, oldPath := range files {
		result := map[string]interface{}{
			"old_path": oldPath,
			"success":  false,
		}

		oldName := filepath.Base(oldPath)
		ext := filepath.Ext(oldName)
		nameWithoutExt := strings.TrimSuffix(oldName, ext)
		newName := nameWithoutExt + suffix + ext
		newPath := filepath.Join(filepath.Dir(oldPath), newName)
		result["new_path"] = newPath

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
		} else {
			err := os.Rename(oldPath, newPath)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to rename: %v", err)
			} else {
				result["success"] = true
			}
		}

		results = append(results, result)
		if result["success"].(bool) {
			successCount++
		}
	}

	return results, successCount
}

// executeChangeExtension changes file extensions
func (b *BatchRenameExecutor) executeChangeExtension(files []string, params ToolParameters, dryRun bool) ([]map[string]interface{}, int) {
	newExt, _ := params["new_extension"].(string)
	if newExt != "" && !strings.HasPrefix(newExt, ".") {
		newExt = "." + newExt
	}

	var results []map[string]interface{}
	successCount := 0

	for _, oldPath := range files {
		result := map[string]interface{}{
			"old_path": oldPath,
			"success":  false,
		}

		oldName := filepath.Base(oldPath)
		nameWithoutExt := strings.TrimSuffix(oldName, filepath.Ext(oldName))
		newName := nameWithoutExt + newExt
		newPath := filepath.Join(filepath.Dir(oldPath), newName)
		result["new_path"] = newPath

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
		} else {
			err := os.Rename(oldPath, newPath)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to rename: %v", err)
			} else {
				result["success"] = true
			}
		}

		results = append(results, result)
		if result["success"].(bool) {
			successCount++
		}
	}

	return results, successCount
}

// executeRemoveExtension removes file extensions
func (b *BatchRenameExecutor) executeRemoveExtension(files []string, dryRun bool) ([]map[string]interface{}, int) {
	var results []map[string]interface{}
	successCount := 0

	for _, oldPath := range files {
		result := map[string]interface{}{
			"old_path": oldPath,
			"success":  false,
		}

		oldName := filepath.Base(oldPath)
		newName := strings.TrimSuffix(oldName, filepath.Ext(oldName))
		
		if newName == oldName {
			result["error"] = "File has no extension to remove"
			results = append(results, result)
			continue
		}

		newPath := filepath.Join(filepath.Dir(oldPath), newName)
		result["new_path"] = newPath

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
		} else {
			err := os.Rename(oldPath, newPath)
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to rename: %v", err)
			} else {
				result["success"] = true
			}
		}

		results = append(results, result)
		if result["success"].(bool) {
			successCount++
		}
	}

	return results, successCount
}

// BatchCopyExecutor handles batch file copying operations
type BatchCopyExecutor struct{}

// NewBatchCopyExecutor creates a new batch copy executor
func NewBatchCopyExecutor() *BatchCopyExecutor {
	return &BatchCopyExecutor{}
}

// Execute performs batch file copying operations
func (b *BatchCopyExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	sources, ok := params["sources"].([]interface{})
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "sources parameter must be an array of file paths",
		}, nil
	}

	destination, ok := params["destination"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "destination parameter is required",
		}, nil
	}

	overwrite, _ := params["overwrite"].(bool)
	preserveStructure, _ := params["preserve_structure"].(bool)
	dryRun, _ := params["dry_run"].(bool)

	var results []map[string]interface{}
	successCount := 0

	for _, source := range sources {
		sourcePath, ok := source.(string)
		if !ok {
			continue
		}

		result := map[string]interface{}{
			"source": sourcePath,
			"success": false,
		}

		// Determine destination path
		var destPath string
		if preserveStructure {
			destPath = filepath.Join(destination, sourcePath)
		} else {
			destPath = filepath.Join(destination, filepath.Base(sourcePath))
		}
		result["destination"] = destPath

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
			results = append(results, result)
			successCount++
			continue
		}

		// Check if destination exists
		if _, err := os.Stat(destPath); err == nil && !overwrite {
			result["error"] = "Destination exists and overwrite is false"
			results = append(results, result)
			continue
		}

		// Ensure destination directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			result["error"] = fmt.Sprintf("Failed to create destination directory: %v", err)
			results = append(results, result)
			continue
		}

		// Copy the file
		if err := copyFile(sourcePath, destPath); err != nil {
			result["error"] = fmt.Sprintf("Failed to copy file: %v", err)
		} else {
			result["success"] = true
			successCount++
		}

		results = append(results, result)
	}

	return &ToolResult{
		Success: successCount > 0 || dryRun,
		Result:  results,
		Metadata: map[string]interface{}{
			"total_files":        len(sources),
			"success_count":      successCount,
			"failure_count":      len(sources) - successCount,
			"destination":        destination,
			"preserve_structure": preserveStructure,
			"overwrite":         overwrite,
			"dry_run":           dryRun,
			"processed_at":      time.Now().Format(time.RFC3339),
		},
	}, nil
}


// BatchDeleteExecutor handles batch file deletion operations
type BatchDeleteExecutor struct {
	backupManager *BackupManager
}

// NewBatchDeleteExecutor creates a new batch delete executor
func NewBatchDeleteExecutor() *BatchDeleteExecutor {
	manager, _ := NewBackupManager("", 100)
	return &BatchDeleteExecutor{
		backupManager: manager,
	}
}

// Execute performs batch file deletion operations
func (b *BatchDeleteExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	files, ok := params["files"].([]interface{})
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "files parameter must be an array of file paths",
		}, nil
	}

	createBackup, _ := params["create_backup"].(bool)
	dryRun, _ := params["dry_run"].(bool)
	force, _ := params["force"].(bool)

	var results []map[string]interface{}
	successCount := 0

	for _, file := range files {
		filePath, ok := file.(string)
		if !ok {
			continue
		}

		result := map[string]interface{}{
			"file_path": filePath,
			"success":   false,
		}

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			result["error"] = "File does not exist"
			results = append(results, result)
			continue
		}

		if dryRun {
			result["success"] = true
			result["dry_run"] = true
			results = append(results, result)
			successCount++
			continue
		}

		// Create backup if requested
		if createBackup {
			backupInfo, err := b.backupManager.CreateBackup(filePath, BackupOptions{})
			if err != nil {
				result["error"] = fmt.Sprintf("Failed to create backup: %v", err)
				results = append(results, result)
				continue
			}
			result["backup_created"] = backupInfo.BackupPath
		}

		// Delete the file
		err := os.Remove(filePath)
		if err != nil {
			if !force {
				result["error"] = fmt.Sprintf("Failed to delete file: %v", err)
			} else {
				// Try to force delete by changing permissions first
				os.Chmod(filePath, 0777)
				err = os.Remove(filePath)
				if err != nil {
					result["error"] = fmt.Sprintf("Failed to force delete file: %v", err)
				} else {
					result["success"] = true
					successCount++
				}
			}
		} else {
			result["success"] = true
			successCount++
		}

		results = append(results, result)
	}

	return &ToolResult{
		Success: successCount > 0 || dryRun,
		Result:  results,
		Metadata: map[string]interface{}{
			"total_files":    len(files),
			"success_count":  successCount,
			"failure_count":  len(files) - successCount,
			"backup_created": createBackup,
			"dry_run":        dryRun,
			"force":          force,
			"processed_at":   time.Now().Format(time.RFC3339),
		},
	}, nil
}