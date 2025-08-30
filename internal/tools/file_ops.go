package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// FileSecurityPolicy defines security constraints for file operations
type FileSecurityPolicy struct {
	// AllowedPaths contains path prefixes that are allowed for file operations
	AllowedPaths []string
	
	// BlockedPaths contains path prefixes that are explicitly blocked
	BlockedPaths []string
	
	// AllowAbsolutePaths determines if absolute paths are allowed
	AllowAbsolutePaths bool
	
	// AllowSymlinks determines if symbolic links are allowed
	AllowSymlinks bool
	
	// MaxFileSize is the maximum file size in bytes for read/write operations
	MaxFileSize int64
	
	// AllowedExtensions contains file extensions that are allowed (empty = all allowed)
	AllowedExtensions []string
	
	// BlockedExtensions contains file extensions that are explicitly blocked
	BlockedExtensions []string
}


// ValidateFilePath validates a file path against the security policy
func (p *FileSecurityPolicy) ValidateFilePath(path string) error {
	// Clean the path to resolve . and .. elements
	cleanPath := filepath.Clean(path)
	
	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") && !p.AllowAbsolutePaths {
		return &ToolValidationError{
			Parameter: "path",
			Message:   "path traversal attempts are not allowed",
			Suggestion: "use relative paths within allowed directories",
		}
	}
	
	// Check if absolute paths are allowed
	if filepath.IsAbs(cleanPath) && !p.AllowAbsolutePaths {
		return &ToolValidationError{
			Parameter: "path",
			Message:   "absolute paths are not allowed",
			Suggestion: "use relative paths starting with ./ or ../",
		}
	}
	
	// Check blocked paths
	for _, blocked := range p.BlockedPaths {
		if strings.HasPrefix(cleanPath, blocked) {
			return &ToolValidationError{
				Parameter: "path",
				Message:   fmt.Sprintf("access to path '%s' is blocked", blocked),
				Suggestion: "use paths outside blocked directories",
			}
		}
	}
	
	// Check allowed paths (if specified)
	if len(p.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range p.AllowedPaths {
			// Handle current directory cases
			if allowedPath == "." || allowedPath == "./" {
				// Allow files in current directory (no path separator)
				if !strings.Contains(cleanPath, "/") || strings.HasPrefix(cleanPath, "./") {
					allowed = true
					break
				}
			} else if strings.HasPrefix(cleanPath, allowedPath) || cleanPath == allowedPath {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ToolValidationError{
				Parameter: "path",
				Message:   "path is not in allowed directories",
				Suggestion: fmt.Sprintf("use paths within: %s", strings.Join(p.AllowedPaths, ", ")),
			}
		}
	}
	
	// Check file extension
	ext := strings.ToLower(filepath.Ext(cleanPath))
	
	// Check blocked extensions
	for _, blocked := range p.BlockedExtensions {
		if ext == strings.ToLower(blocked) {
			return &ToolValidationError{
				Parameter: "path",
				Message:   fmt.Sprintf("file extension '%s' is blocked", ext),
				Suggestion: "use files with allowed extensions",
			}
		}
	}
	
	// Check allowed extensions (if specified)
	if len(p.AllowedExtensions) > 0 {
		allowed := false
		for _, allowedExt := range p.AllowedExtensions {
			if ext == strings.ToLower(allowedExt) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ToolValidationError{
				Parameter: "path",
				Message:   fmt.Sprintf("file extension '%s' is not allowed", ext),
				Suggestion: fmt.Sprintf("use files with allowed extensions: %s", strings.Join(p.AllowedExtensions, ", ")),
			}
		}
	}
	
	return nil
}

// CheckSymlink validates symbolic links according to policy
func (p *FileSecurityPolicy) CheckSymlink(path string) error {
	if !p.AllowSymlinks {
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return &ToolValidationError{
				Parameter: "path",
				Message:   "symbolic links are not allowed",
				Suggestion: "use direct file paths instead of symlinks",
			}
		}
	}
	return nil
}

// ReadFileExecutor implements file reading functionality
type ReadFileExecutor struct {
	*BaseExecutor
	securityPolicy *FileSecurityPolicy
}

// NewReadFileExecutor creates a new file reader executor
func NewReadFileExecutor(policy *FileSecurityPolicy) *ReadFileExecutor {
	if policy == nil {
		policy = NewDefaultFileSecurityPolicy()
	}
	
	base := NewBaseExecutor("read_file", "Reads the contents of a file")
	
	// Define schema
	base.AddRequiredParam("path", "string", "Path to the file to read")
	base.AddOptionalParam("encoding", "string", "Text encoding (utf-8, ascii, latin1)", "utf-8")
	base.AddOptionalParam("max_lines", "integer", "Maximum number of lines to read (0 = all)", 0)
	base.AddOptionalParam("start_line", "integer", "Starting line number (1-based, 0 = from beginning)", 0)
	
	// Add examples
	base.AddExample(ToolParameters{
		"path": "./README.md",
	})
	base.AddExample(ToolParameters{
		"path":      "./src/main.go",
		"max_lines": 100,
		"start_line": 50,
	})
	
	return &ReadFileExecutor{
		BaseExecutor:   base,
		securityPolicy: policy,
	}
}

// Execute reads a file and returns its contents
func (r *ReadFileExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get required parameters
	path, err := GetRequiredStringParam(params, "path")
	if err != nil {
		return nil, err
	}
	
	// Get optional parameters
	encoding, _ := GetStringParam(params, "encoding")
	if encoding == "" {
		encoding = "utf-8"
	}
	
	maxLines, _ := GetIntParam(params, "max_lines")
	startLine, _ := GetIntParam(params, "start_line")
	
	// Validate parameters
	if maxLines < 0 {
		return nil, &ToolValidationError{
			Parameter: "max_lines",
			Message:   "max_lines cannot be negative",
		}
	}
	if startLine < 0 {
		return nil, &ToolValidationError{
			Parameter: "start_line",
			Message:   "start_line cannot be negative",
		}
	}
	
	// Validate file path against security policy
	if err := r.securityPolicy.ValidateFilePath(path); err != nil {
		return nil, err
	}
	
	// Clean path
	cleanPath := filepath.Clean(path)
	
	// Check if file exists and is accessible
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ToolExecutionError{
				ToolName:    r.Name(),
				Message:     fmt.Sprintf("file does not exist: %s", path),
				Recoverable: true,
			}
		}
		return nil, &ToolExecutionError{
			ToolName:    r.Name(),
			Message:     fmt.Sprintf("cannot access file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return nil, &ToolExecutionError{
			ToolName:    r.Name(),
			Message:     fmt.Sprintf("not a regular file: %s", path),
			Recoverable: true,
		}
	}
	
	// Check file size against policy
	if info.Size() > r.securityPolicy.MaxFileSize {
		return nil, &ToolExecutionError{
			ToolName:    r.Name(),
			Message:     fmt.Sprintf("file too large: %d bytes (max: %d)", info.Size(), r.securityPolicy.MaxFileSize),
			Recoverable: true,
		}
	}
	
	// Check for symbolic links
	if err := r.securityPolicy.CheckSymlink(cleanPath); err != nil {
		return nil, err
	}
	
	// Open file
	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolName:    r.Name(),
			Message:     fmt.Sprintf("cannot open file: %s", err.Error()),
			Recoverable: true,
		}
	}
	defer file.Close()
	
	// Read file contents
	var content []byte
	
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil, &ToolExecutionError{
			ToolName:    r.Name(),
			Message:     "file reading was cancelled",
			Cause:       ctx.Err(),
			Recoverable: true,
		}
	default:
	}
	
	content, err = io.ReadAll(file)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolName:    r.Name(),
			Message:     fmt.Sprintf("error reading file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	// Convert to string based on encoding
	var contentStr string
	switch strings.ToLower(encoding) {
	case "utf-8", "utf8":
		if !utf8.Valid(content) {
			return nil, &ToolExecutionError{
				ToolName:    r.Name(),
				Message:     "file contains invalid UTF-8 content",
				Recoverable: true,
			}
		}
		contentStr = string(content)
	case "ascii":
		// Check if all bytes are valid ASCII
		for _, b := range content {
			if b > 127 {
				return nil, &ToolExecutionError{
					ToolName:    r.Name(),
					Message:     "file contains non-ASCII characters",
					Recoverable: true,
				}
			}
		}
		contentStr = string(content)
	case "latin1", "latin-1", "iso-8859-1":
		// Latin1 accepts all byte values
		contentStr = string(content)
	default:
		return nil, &ToolValidationError{
			Parameter:  "encoding",
			Message:    fmt.Sprintf("unsupported encoding: %s", encoding),
			Suggestion: "use utf-8, ascii, or latin1",
		}
	}
	
	// Apply line filtering if requested
	if startLine > 0 || maxLines > 0 {
		lines := strings.Split(contentStr, "\n")
		totalLines := len(lines)
		
		// Adjust start line (convert from 1-based to 0-based)
		start := 0
		if startLine > 0 {
			start = startLine - 1
			if start >= totalLines {
				return &ToolResult{
					Success: true,
					Result:  "",
					Metadata: map[string]interface{}{
						"file_path":    cleanPath,
						"file_size":    info.Size(),
						"total_lines":  totalLines,
						"start_line":   startLine,
						"returned_lines": 0,
						"encoding":     encoding,
					},
				}, nil
			}
		}
		
		// Apply max lines limit
		end := totalLines
		if maxLines > 0 && start+maxLines < totalLines {
			end = start + maxLines
		}
		
		// Extract requested lines
		selectedLines := lines[start:end]
		contentStr = strings.Join(selectedLines, "\n")
		
		return &ToolResult{
			Success: true,
			Result:  contentStr,
			Metadata: map[string]interface{}{
				"file_path":      cleanPath,
				"file_size":      info.Size(),
				"total_lines":    totalLines,
				"start_line":     startLine,
				"returned_lines": len(selectedLines),
				"encoding":       encoding,
			},
		}, nil
	}
	
	// Return full content
	lines := strings.Split(contentStr, "\n")
	return &ToolResult{
		Success: true,
		Result:  contentStr,
		Metadata: map[string]interface{}{
			"file_path":   cleanPath,
			"file_size":   info.Size(),
			"total_lines": len(lines),
			"encoding":    encoding,
		},
	}, nil
}

// ValidateParameters validates the parameters for file reading
func (r *ReadFileExecutor) ValidateParameters(params ToolParameters) error {
	// Call base validation first
	if err := r.BaseExecutor.ValidateParameters(params); err != nil {
		return err
	}
	
	// Additional validation for file-specific parameters
	if encoding, exists := GetStringParam(params, "encoding"); exists {
		validEncodings := []string{"utf-8", "utf8", "ascii", "latin1", "latin-1", "iso-8859-1"}
		valid := false
		for _, validEnc := range validEncodings {
			if strings.ToLower(encoding) == validEnc {
				valid = true
				break
			}
		}
		if !valid {
			return &ToolValidationError{
				Parameter:  "encoding",
				Message:    fmt.Sprintf("unsupported encoding: %s", encoding),
				Suggestion: "use utf-8, ascii, or latin1",
			}
		}
	}
	
	return nil
}
// WriteFileExecutor implements file writing functionality with backup support
type WriteFileExecutor struct {
	*BaseExecutor
	securityPolicy *FileSecurityPolicy
}

// NewWriteFileExecutor creates a new file writer executor
func NewWriteFileExecutor(policy *FileSecurityPolicy) *WriteFileExecutor {
	if policy == nil {
		policy = NewDefaultFileSecurityPolicy()
	}
	
	base := NewBaseExecutor("write_file", "Writes content to a file with optional backup")
	
	// Define schema
	base.AddRequiredParam("file_path", "string", "Path to the file to write")
	base.AddRequiredParam("content", "string", "Content to write to the file")
	base.AddOptionalParam("encoding", "string", "Text encoding (utf-8, ascii, latin1)", "utf-8")
	base.AddOptionalParam("create_backup", "boolean", "Whether to create a backup of existing file", true)
	base.AddOptionalParam("append", "boolean", "Whether to append to existing file instead of overwriting", false)
	base.AddOptionalParam("create_dirs", "boolean", "Whether to create parent directories if they don't exist", false)
	
	// Add examples
	base.AddExample(ToolParameters{
		"file_path": "./output.txt",
		"content":   "Hello, World!",
	})
	base.AddExample(ToolParameters{
		"file_path":     "./config.json",
		"content":       `{"setting": "value"}`,
		"create_backup": true,
		"create_dirs":   true,
	})
	
	return &WriteFileExecutor{
		BaseExecutor:   base,
		securityPolicy: policy,
	}
}

// Execute writes content to a file
func (w *WriteFileExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get required parameters
	path, err := GetRequiredStringParam(params, "file_path")
	if err != nil {
		return nil, err
	}
	
	content, err := GetRequiredStringParam(params, "content")
	if err != nil {
		return nil, err
	}
	
	// Get optional parameters
	encoding, _ := GetStringParam(params, "encoding")
	if encoding == "" {
		encoding = "utf-8"
	}
	
	createBackup, exists := GetBoolParam(params, "create_backup")
	if !exists {
		createBackup = true
	}
	
	appendMode, _ := GetBoolParam(params, "append")
	createDirs, _ := GetBoolParam(params, "create_dirs")
	
	// Validate file path against security policy
	if err := w.securityPolicy.ValidateFilePath(path); err != nil {
		return nil, err
	}
	
	// Clean path
	cleanPath := filepath.Clean(path)
	
	// Check content size
	contentBytes := []byte(content)
	if int64(len(contentBytes)) > w.securityPolicy.MaxFileSize {
		return nil, &ToolExecutionError{
			ToolName:    w.Name(),
			Message:     fmt.Sprintf("content too large: %d bytes (max: %d)", len(contentBytes), w.securityPolicy.MaxFileSize),
			Recoverable: true,
		}
	}
	
	// Validate encoding and convert content if needed
	switch strings.ToLower(encoding) {
	case "utf-8", "utf8":
		if !utf8.ValidString(content) {
			return nil, &ToolValidationError{
				Parameter: "content",
				Message:   "content contains invalid UTF-8 characters",
			}
		}
	case "ascii":
		for _, r := range content {
			if r > 127 {
				return nil, &ToolValidationError{
					Parameter: "content",
					Message:   "content contains non-ASCII characters",
				}
			}
		}
	case "latin1", "latin-1", "iso-8859-1":
		// Latin1 accepts most characters, but check for valid range
		for _, r := range content {
			if r > 255 {
				return nil, &ToolValidationError{
					Parameter: "content",
					Message:   "content contains characters not valid in Latin1 encoding",
				}
			}
		}
	default:
		return nil, &ToolValidationError{
			Parameter:  "encoding",
			Message:    fmt.Sprintf("unsupported encoding: %s", encoding),
			Suggestion: "use utf-8, ascii, or latin1",
		}
	}
	
	// Create parent directories if requested and needed
	parentDir := filepath.Dir(cleanPath)
	if createDirs {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return nil, &ToolExecutionError{
				ToolName:    w.Name(),
				Message:     fmt.Sprintf("cannot create parent directories: %s", err.Error()),
				Recoverable: true,
			}
		}
	}
	
	// Check if file exists
	var existingInfo os.FileInfo
	var fileExists bool
	if info, err := os.Stat(cleanPath); err == nil {
		existingInfo = info
		fileExists = true
		
		// Check if it's a regular file
		if !info.Mode().IsRegular() {
			return nil, &ToolExecutionError{
				ToolName:    w.Name(),
				Message:     fmt.Sprintf("not a regular file: %s", path),
				Recoverable: true,
			}
		}
		
		// Check for symbolic links
		if err := w.securityPolicy.CheckSymlink(cleanPath); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, &ToolExecutionError{
			ToolName:    w.Name(),
			Message:     fmt.Sprintf("cannot access file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	var backupPath string
	var backupCreated bool
	
	// Create backup if requested and file exists
	if createBackup && fileExists && !appendMode {
		backupPath = cleanPath + ".bak." + time.Now().Format("20060102-150405")
		
		// Check context for cancellation
		select {
		case <-ctx.Done():
			return nil, &ToolExecutionError{
				ToolName:    w.Name(),
				Message:     "file writing was cancelled",
				Cause:       ctx.Err(),
				Recoverable: true,
			}
		default:
		}
		
		// Copy original file to backup
		if err := copyFile(cleanPath, backupPath); err != nil {
			return nil, &ToolExecutionError{
				ToolName:    w.Name(),
				Message:     fmt.Sprintf("cannot create backup: %s", err.Error()),
				Recoverable: true,
			}
		}
		backupCreated = true
	}
	
	// Write content to file
	var flags int
	if appendMode {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	} else {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	
	file, err := os.OpenFile(cleanPath, flags, 0644)
	if err != nil {
		// If backup was created, try to restore it
		if backupCreated {
			if restoreErr := copyFile(backupPath, cleanPath); restoreErr != nil {
				return nil, &ToolExecutionError{
					ToolName:    w.Name(),
					Message:     fmt.Sprintf("cannot open file for writing and failed to restore backup: %s (original error: %s)", restoreErr.Error(), err.Error()),
					Recoverable: false,
				}
			}
		}
		return nil, &ToolExecutionError{
			ToolName:    w.Name(),
			Message:     fmt.Sprintf("cannot open file for writing: %s", err.Error()),
			Recoverable: true,
		}
	}
	defer file.Close()
	
	// Write content
	bytesWritten, err := file.WriteString(content)
	if err != nil {
		// If backup was created, try to restore it
		if backupCreated {
			file.Close()
			if restoreErr := copyFile(backupPath, cleanPath); restoreErr != nil {
				return nil, &ToolExecutionError{
					ToolName:    w.Name(),
					Message:     fmt.Sprintf("write failed and cannot restore backup: %s (original error: %s)", restoreErr.Error(), err.Error()),
					Recoverable: false,
				}
			}
		}
		return nil, &ToolExecutionError{
			ToolName:    w.Name(),
			Message:     fmt.Sprintf("error writing to file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	// Sync to ensure data is written to disk
	if err := file.Sync(); err != nil {
		return nil, &ToolExecutionError{
			ToolName:    w.Name(),
			Message:     fmt.Sprintf("error syncing file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	// Get final file info
	finalInfo, err := os.Stat(cleanPath)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolName:    w.Name(),
			Message:     fmt.Sprintf("cannot get final file info: %s", err.Error()),
			Recoverable: false,
		}
	}
	
	// Prepare result metadata
	metadata := map[string]interface{}{
		"file_path":      cleanPath,
		"bytes_written":  bytesWritten,
		"file_size":      finalInfo.Size(),
		"encoding":       encoding,
		"append_mode":    appendMode,
		"backup_created": backupCreated,
	}
	
	if backupCreated {
		metadata["backup_path"] = backupPath
	}
	
	if fileExists {
		metadata["file_existed"] = true
		metadata["original_size"] = existingInfo.Size()
	} else {
		metadata["file_existed"] = false
	}
	
	return &ToolResult{
		Success:  true,
		Result:   fmt.Sprintf("Successfully wrote %d bytes to %s", bytesWritten, path),
		Metadata: metadata,
	}, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
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
	
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}
	
	// Copy file permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	
	return os.Chmod(dst, sourceInfo.Mode())
}

// NewDefaultFileSecurityPolicy creates a default file security policy
func NewDefaultFileSecurityPolicy() *FileSecurityPolicy {
	// Get current working directory for path validation
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	
	return &FileSecurityPolicy{
		AllowedPaths:      []string{".", "./", cwd},
		BlockedPaths:      []string{"/etc", "/proc", "/sys", "/dev"},
		AllowAbsolutePaths: false,
		AllowSymlinks:     false,
		MaxFileSize:       100 * 1024 * 1024, // 100MB
		AllowedExtensions: []string{}, // Empty means all allowed by default
		BlockedExtensions: []string{".exe", ".bat", ".cmd", ".sh"},
	}
}

// CheckPath validates a file path against the security policy
func (policy *FileSecurityPolicy) CheckPath(path string) error {
	// Convert to absolute path for checking
	absPath, err := filepath.Abs(path)
	if err != nil {
		return &ToolValidationError{
			Parameter: "path",
			Message:   fmt.Sprintf("cannot resolve path: %s", err.Error()),
		}
	}
	
	// Check if absolute paths are allowed
	if !policy.AllowAbsolutePaths && filepath.IsAbs(path) {
		return &ToolValidationError{
			Parameter: "path",
			Message:   "absolute paths are not allowed",
			Suggestion: "use relative paths starting from current directory",
		}
	}
	
	// Check blocked paths
	for _, blocked := range policy.BlockedPaths {
		if strings.HasPrefix(absPath, blocked) {
			return &ToolValidationError{
				Parameter: "path",
				Message:   fmt.Sprintf("access to path '%s' is blocked", blocked),
			}
		}
	}
	
	// Check allowed paths (if specified)
	if len(policy.AllowedPaths) > 0 {
		allowed := false
		for _, allowedPath := range policy.AllowedPaths {
			allowedAbs, err := filepath.Abs(allowedPath)
			if err != nil {
				continue
			}
			if strings.HasPrefix(absPath, allowedAbs) {
				allowed = true
				break
			}
		}
		if !allowed {
			return &ToolValidationError{
				Parameter: "path",
				Message:   "path is not in allowed paths list",
				Suggestion: "use paths within allowed directories: " + strings.Join(policy.AllowedPaths, ", "),
			}
		}
	}
	
	// Check file extension
	ext := filepath.Ext(path)
	if ext != "" {
		// Check blocked extensions
		for _, blocked := range policy.BlockedExtensions {
			if strings.EqualFold(ext, blocked) {
				return &ToolValidationError{
					Parameter: "path",
					Message:   fmt.Sprintf("file extension '%s' is blocked", ext),
				}
			}
		}
		
		// Check allowed extensions (if specified)
		if len(policy.AllowedExtensions) > 0 {
			allowed := false
			for _, allowedExt := range policy.AllowedExtensions {
				if strings.EqualFold(ext, allowedExt) {
					allowed = true
					break
				}
			}
			if !allowed {
				return &ToolValidationError{
					Parameter: "path",
					Message:   fmt.Sprintf("file extension '%s' is not allowed", ext),
					Suggestion: "use allowed extensions: " + strings.Join(policy.AllowedExtensions, ", "),
				}
			}
		}
	}
	
	return nil
}


// EditFileExecutor provides advanced file editing capabilities with diff preview
type EditFileExecutor struct {
	*BaseExecutor
	securityPolicy *FileSecurityPolicy
}

// NewEditFileExecutor creates a new edit file tool executor
func NewEditFileExecutor() *EditFileExecutor {
	base := NewBaseExecutor("edit_file", "Edit a file with find-and-replace operations and diff preview")
	
	// Define schema
	base.AddRequiredParam("file_path", "string", "Path to the file to edit")
	base.AddRequiredParam("operation", "string", "Edit operation: replace, insert_line, delete_line, append, prepend")
	base.AddOptionalParam("find", "string", "Text to find (for replace operation)", "")
	base.AddOptionalParam("replace", "string", "Replacement text (for replace operation)", "")
	base.AddOptionalParam("text", "string", "Text to insert/append/prepend", "")
	base.AddOptionalParam("line_number", "integer", "Line number for insert/delete operations (1-based)", 0)
	base.AddOptionalParam("preview", "boolean", "Show diff preview instead of applying changes", false)
	base.AddOptionalParam("create_backup", "boolean", "Create backup before editing", true)
	base.AddOptionalParam("encoding", "string", "File encoding (utf-8, ascii, latin1)", "utf-8")
	base.AddOptionalParam("case_sensitive", "boolean", "Case-sensitive find/replace", true)
	base.AddOptionalParam("whole_words", "boolean", "Match whole words only in find/replace", false)
	base.AddOptionalParam("max_replacements", "integer", "Maximum number of replacements (0 = unlimited)", 0)
	
	// Add examples
	base.AddExample(ToolParameters{
		"file_path":  "config.txt",
		"operation":  "replace",
		"find":       "old_value",
		"replace":    "new_value",
		"preview":    true,
	})
	base.AddExample(ToolParameters{
		"file_path":   "script.sh",
		"operation":   "insert_line",
		"line_number": 5,
		"text":        "echo 'New line'",
	})
	base.AddExample(ToolParameters{
		"file_path": "notes.txt",
		"operation": "append",
		"text":      "Additional notes",
	})
	
	return &EditFileExecutor{
		BaseExecutor:   base,
		securityPolicy: NewDefaultFileSecurityPolicy(),
	}
}

// Execute performs file editing operations
func (e *EditFileExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get required parameters
	path, err := GetRequiredStringParam(params, "file_path")
	if err != nil {
		return nil, err
	}
	
	operation, err := GetRequiredStringParam(params, "operation")
	if err != nil {
		return nil, err
	}
	
	// Get optional parameters
	find, _ := GetStringParam(params, "find")
	replace, _ := GetStringParam(params, "replace")
	text, _ := GetStringParam(params, "text")
	lineNumber, _ := GetIntParam(params, "line_number")
	preview, _ := GetBoolParam(params, "preview")
	createBackup, exists := GetBoolParam(params, "create_backup")
	if !exists {
		createBackup = true
	}
	encoding, exists := GetStringParam(params, "encoding")
	if !exists {
		encoding = "utf-8"
	}
	caseSensitive, exists := GetBoolParam(params, "case_sensitive")
	if !exists {
		caseSensitive = true
	}
	wholeWords, _ := GetBoolParam(params, "whole_words")
	maxReplacements, _ := GetIntParam(params, "max_replacements")
	
	// Validate parameters based on operation
	switch strings.ToLower(operation) {
	case "replace":
		if find == "" {
			return nil, &ToolValidationError{
				Parameter: "find",
				Message:   "find parameter is required for replace operation",
			}
		}
	case "insert_line", "delete_line":
		if lineNumber < 1 {
			return nil, &ToolValidationError{
				Parameter: "line_number",
				Message:   "line_number must be >= 1 for insert_line/delete_line operations",
			}
		}
		if operation == "insert_line" && text == "" {
			return nil, &ToolValidationError{
				Parameter: "text",
				Message:   "text parameter is required for insert_line operation",
			}
		}
	case "append", "prepend":
		if text == "" {
			return nil, &ToolValidationError{
				Parameter: "text",
				Message:   "text parameter is required for append/prepend operations",
			}
		}
	default:
		return nil, &ToolValidationError{
			Parameter:  "operation",
			Message:    fmt.Sprintf("unsupported operation: %s", operation),
			Suggestion: "use replace, insert_line, delete_line, append, or prepend",
		}
	}
	
	// Check file path security
	if err := e.securityPolicy.CheckPath(path); err != nil {
		return nil, err
	}
	
	// Clean path
	cleanPath := filepath.Clean(path)
	
	// Check if file exists
	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			return nil, &ToolExecutionError{
				ToolName:    e.Name(),
				Message:     fmt.Sprintf("file does not exist: %s", path),
				Recoverable: true,
			}
		}
		return nil, &ToolExecutionError{
			ToolName:    e.Name(),
			Message:     fmt.Sprintf("cannot access file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	// Read original content
	originalContent, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, &ToolExecutionError{
			ToolName:    e.Name(),
			Message:     fmt.Sprintf("cannot read file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	// Convert to string with encoding validation
	originalText := string(originalContent)
	if !e.validateEncoding(originalText, encoding) {
		return nil, &ToolValidationError{
			Parameter: "encoding",
			Message:   fmt.Sprintf("file content is not valid %s", encoding),
		}
	}
	
	// Apply the edit operation
	editedText, changes, err := e.applyEdit(originalText, operation, find, replace, text, lineNumber, caseSensitive, wholeWords, maxReplacements)
	if err != nil {
		return nil, err
	}
	
	// Generate diff
	diffText := e.generateDiff(originalText, editedText, path)
	
	metadata := map[string]interface{}{
		"file_path":        cleanPath,
		"operation":        operation,
		"encoding":         encoding,
		"changes_made":     changes,
		"original_size":    len(originalContent),
		"edited_size":      len(editedText),
		"preview_mode":     preview,
	}
	
	// If preview mode, return diff without making changes
	if preview {
		metadata["diff"] = diffText
		return &ToolResult{
			Success:  true,
			Result:   fmt.Sprintf("Preview of changes to %s:\n\n%s", path, diffText),
			Metadata: metadata,
		}, nil
	}
	
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil, &ToolExecutionError{
			ToolName:    e.Name(),
			Message:     "edit operation was cancelled",
			Cause:       ctx.Err(),
			Recoverable: true,
		}
	default:
	}
	
	var backupPath string
	var backupCreated bool
	
	// Create backup if requested
	if createBackup {
		backupPath = cleanPath + ".bak." + time.Now().Format("20060102-150405")
		if err := copyFile(cleanPath, backupPath); err != nil {
			return nil, &ToolExecutionError{
				ToolName:    e.Name(),
				Message:     fmt.Sprintf("cannot create backup: %s", err.Error()),
				Recoverable: true,
			}
		}
		backupCreated = true
		metadata["backup_created"] = true
		metadata["backup_path"] = backupPath
	}
	
	// Write edited content
	if err := os.WriteFile(cleanPath, []byte(editedText), 0644); err != nil {
		// Try to restore backup if write fails
		if backupCreated {
			if restoreErr := copyFile(backupPath, cleanPath); restoreErr != nil {
				return nil, &ToolExecutionError{
					ToolName:    e.Name(),
					Message:     fmt.Sprintf("write failed and cannot restore backup: %s (original error: %s)", restoreErr.Error(), err.Error()),
					Recoverable: false,
				}
			}
		}
		return nil, &ToolExecutionError{
			ToolName:    e.Name(),
			Message:     fmt.Sprintf("cannot write edited file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	metadata["diff"] = diffText
	
	return &ToolResult{
		Success:  true,
		Result:   fmt.Sprintf("Successfully edited %s (%d changes made)", path, changes),
		Metadata: metadata,
	}, nil
}

// validateEncoding checks if text is valid for the specified encoding
func (e *EditFileExecutor) validateEncoding(text, encoding string) bool {
	switch strings.ToLower(encoding) {
	case "utf-8", "utf8":
		return utf8.ValidString(text)
	case "ascii":
		for _, r := range text {
			if r > 127 {
				return false
			}
		}
		return true
	case "latin1", "latin-1", "iso-8859-1":
		for _, r := range text {
			if r > 255 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// applyEdit applies the specified edit operation to the text
func (e *EditFileExecutor) applyEdit(originalText, operation, find, replace, text string, lineNumber int, caseSensitive, wholeWords bool, maxReplacements int) (string, int, error) {
	lines := strings.Split(originalText, "\n")
	
	switch strings.ToLower(operation) {
	case "replace":
		return e.performReplace(originalText, find, replace, caseSensitive, wholeWords, maxReplacements)
		
	case "insert_line":
		if lineNumber > len(lines)+1 {
			return "", 0, &ToolValidationError{
				Parameter: "line_number",
				Message:   fmt.Sprintf("line_number %d exceeds file length (%d lines)", lineNumber, len(lines)),
			}
		}
		// Insert at specified line (1-based)
		insertPos := lineNumber - 1
		if insertPos < 0 {
			insertPos = 0
		}
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertPos]...)
		newLines = append(newLines, text)
		newLines = append(newLines, lines[insertPos:]...)
		return strings.Join(newLines, "\n"), 1, nil
		
	case "delete_line":
		if lineNumber > len(lines) || lineNumber < 1 {
			return "", 0, &ToolValidationError{
				Parameter: "line_number",
				Message:   fmt.Sprintf("line_number %d is out of range (1-%d)", lineNumber, len(lines)),
			}
		}
		// Delete specified line (1-based)
		deletePos := lineNumber - 1
		newLines := make([]string, 0, len(lines)-1)
		newLines = append(newLines, lines[:deletePos]...)
		newLines = append(newLines, lines[deletePos+1:]...)
		return strings.Join(newLines, "\n"), 1, nil
		
	case "append":
		if originalText == "" || strings.HasSuffix(originalText, "\n") {
			return originalText + text + "\n", 1, nil
		}
		return originalText + "\n" + text + "\n", 1, nil
		
	case "prepend":
		return text + "\n" + originalText, 1, nil
		
	default:
		return "", 0, &ToolValidationError{
			Parameter: "operation",
			Message:   fmt.Sprintf("unsupported operation: %s", operation),
		}
	}
}

// performReplace performs find and replace operations
func (e *EditFileExecutor) performReplace(text, find, replace string, caseSensitive, wholeWords bool, maxReplacements int) (string, int, error) {
	if find == "" {
		return text, 0, nil
	}
	
	searchText := text
	searchFind := find
	
	// Handle case sensitivity
	if !caseSensitive {
		searchText = strings.ToLower(text)
		searchFind = strings.ToLower(find)
	}
	
	var result strings.Builder
	var changes int
	pos := 0
	
	for {
		// Check if we've hit max replacements
		if maxReplacements > 0 && changes >= maxReplacements {
			break
		}
		
		// Find next occurrence
		index := strings.Index(searchText[pos:], searchFind)
		if index == -1 {
			break
		}
		
		actualIndex := pos + index
		
		// Check for whole word matching
		if wholeWords {
			// Check character before match
			if actualIndex > 0 {
				prevChar := rune(text[actualIndex-1])
				if isAlphaNumeric(prevChar) {
					pos = actualIndex + 1
					continue
				}
			}
			
			// Check character after match
			endPos := actualIndex + len(find)
			if endPos < len(text) {
				nextChar := rune(text[endPos])
				if isAlphaNumeric(nextChar) {
					pos = actualIndex + 1
					continue
				}
			}
		}
		
		// Add text before the match
		result.WriteString(text[pos:actualIndex])
		
		// Add replacement text
		result.WriteString(replace)
		
		// Move position past the match
		pos = actualIndex + len(find)
		changes++
	}
	
	// Add remaining text
	result.WriteString(text[pos:])
	
	return result.String(), changes, nil
}

// isAlphaNumeric checks if a character is alphanumeric
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// generateDiff creates a unified diff between original and edited text
func (e *EditFileExecutor) generateDiff(original, edited, filename string) string {
	if original == edited {
		return "No changes made"
	}
	
	originalLines := strings.Split(original, "\n")
	editedLines := strings.Split(edited, "\n")
	
	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- %s (original)\n", filename))
	diff.WriteString(fmt.Sprintf("+++ %s (edited)\n", filename))
	
	// Simple diff implementation - show context around changes
	maxLines := len(originalLines)
	if len(editedLines) > maxLines {
		maxLines = len(editedLines)
	}
	
	i, j := 0, 0
	for i < len(originalLines) || j < len(editedLines) {
		if i < len(originalLines) && j < len(editedLines) {
			if originalLines[i] == editedLines[j] {
				// Lines match
				i++
				j++
				continue
			}
		}
		
		// Found a difference, show context
		contextStart := i - 2
		if contextStart < 0 {
			contextStart = 0
		}
		
		// Show hunk header
		diff.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", i+1, min(5, len(originalLines)-i), j+1, min(5, len(editedLines)-j)))
		
		// Show removed lines
		for k := i; k < len(originalLines) && k < i+5; k++ {
			diff.WriteString("- " + originalLines[k] + "\n")
		}
		
		// Show added lines
		for k := j; k < len(editedLines) && k < j+5; k++ {
			diff.WriteString("+ " + editedLines[k] + "\n")
		}
		
		// Move forward
		i += min(5, len(originalLines)-i)
		j += min(5, len(editedLines)-j)
		
		if i >= len(originalLines) {
			i = len(originalLines)
		}
		if j >= len(editedLines) {
			j = len(editedLines)
		}
		
		break // Simple implementation - just show first difference
	}
	
	return diff.String()
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ListDirectoryExecutor provides directory listing and navigation capabilities
type ListDirectoryExecutor struct {
	*BaseExecutor
	securityPolicy *FileSecurityPolicy
}

// NewListDirectoryExecutor creates a new list directory tool executor
func NewListDirectoryExecutor() *ListDirectoryExecutor {
	base := NewBaseExecutor("list_directory", "List files and directories with detailed information")
	
	// Define schema
	base.AddRequiredParam("path", "string", "Directory path to list")
	base.AddOptionalParam("recursive", "boolean", "Recursively list subdirectories", false)
	base.AddOptionalParam("show_hidden", "boolean", "Show hidden files (starting with .)", false)
	base.AddOptionalParam("show_details", "boolean", "Show detailed file information", true)
	base.AddOptionalParam("filter_pattern", "string", "Filter files by pattern (glob)", "")
	base.AddOptionalParam("sort_by", "string", "Sort by: name, size, time, type", "name")
	base.AddOptionalParam("sort_desc", "boolean", "Sort in descending order", false)
	base.AddOptionalParam("max_depth", "integer", "Maximum recursion depth", 10)
	base.AddOptionalParam("max_files", "integer", "Maximum number of files to return", 1000)
	
	// Add examples
	base.AddExample(ToolParameters{
		"path": ".",
	})
	base.AddExample(ToolParameters{
		"path":         "/project/src",
		"recursive":    true,
		"max_depth":    3,
		"filter_pattern": "*.go",
	})
	base.AddExample(ToolParameters{
		"path":        "logs/",
		"show_hidden": true,
		"sort_by":     "time",
		"sort_desc":   true,
	})
	
	return &ListDirectoryExecutor{
		BaseExecutor:   base,
		securityPolicy: NewDefaultFileSecurityPolicy(),
	}
}

// FileInfo represents detailed information about a file or directory
type FileInfo struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Size         int64             `json:"size"`
	IsDirectory  bool              `json:"is_directory"`
	IsSymlink    bool              `json:"is_symlink"`
	Mode         string            `json:"mode"`
	ModTime      time.Time         `json:"mod_time"`
	Permissions  string            `json:"permissions"`
	Owner        string            `json:"owner,omitempty"`
	Group        string            `json:"group,omitempty"`
	Hidden       bool              `json:"hidden"`
	Extension    string            `json:"extension,omitempty"`
	MimeType     string            `json:"mime_type,omitempty"`
	Children     int               `json:"children,omitempty"` // For directories
	LinkTarget   string            `json:"link_target,omitempty"` // For symlinks
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Execute performs directory listing
func (l *ListDirectoryExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get required parameters
	path, err := GetRequiredStringParam(params, "path")
	if err != nil {
		return nil, err
	}
	
	// Get optional parameters
	recursive, _ := GetBoolParam(params, "recursive")
	showHidden, _ := GetBoolParam(params, "show_hidden")
	showDetails, exists := GetBoolParam(params, "show_details")
	if !exists {
		showDetails = true
	}
	filterPattern, _ := GetStringParam(params, "filter_pattern")
	sortBy, exists := GetStringParam(params, "sort_by")
	if !exists {
		sortBy = "name"
	}
	sortDesc, _ := GetBoolParam(params, "sort_desc")
	maxDepth, exists := GetIntParam(params, "max_depth")
	if !exists {
		maxDepth = 10
	}
	maxFiles, exists := GetIntParam(params, "max_files")
	if !exists {
		maxFiles = 1000
	}
	
	// Validate parameters
	if maxDepth < 1 || maxDepth > 50 {
		return nil, &ToolValidationError{
			Parameter: "max_depth",
			Message:   "max_depth must be between 1 and 50",
		}
	}
	
	if maxFiles < 1 || maxFiles > 10000 {
		return nil, &ToolValidationError{
			Parameter: "max_files",
			Message:   "max_files must be between 1 and 10000",
		}
	}
	
	sortBy = strings.ToLower(sortBy)
	if sortBy != "name" && sortBy != "size" && sortBy != "time" && sortBy != "type" {
		return nil, &ToolValidationError{
			Parameter:  "sort_by",
			Message:    fmt.Sprintf("invalid sort_by value: %s", sortBy),
			Suggestion: "use name, size, time, or type",
		}
	}
	
	// Check file path security
	if err := l.securityPolicy.CheckPath(path); err != nil {
		return nil, err
	}
	
	// Clean and validate path
	cleanPath := filepath.Clean(path)
	
	// Check if directory exists
	dirInfo, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ToolExecutionError{
				ToolName:    l.Name(),
				Message:     fmt.Sprintf("directory does not exist: %s", path),
				Recoverable: true,
			}
		}
		return nil, &ToolExecutionError{
			ToolName:    l.Name(),
			Message:     fmt.Sprintf("cannot access directory: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	if !dirInfo.IsDir() {
		return nil, &ToolExecutionError{
			ToolName:    l.Name(),
			Message:     fmt.Sprintf("path is not a directory: %s", path),
			Recoverable: true,
		}
	}
	
	// List files
	var files []FileInfo
	var totalFiles int
	var skippedFiles int
	
	err = l.listFiles(ctx, cleanPath, "", 0, maxDepth, recursive, showHidden, showDetails, filterPattern, &files, &totalFiles, &skippedFiles, maxFiles)
	if err != nil {
		return nil, err
	}
	
	// Sort files
	l.sortFiles(files, sortBy, sortDesc)
	
	// Prepare metadata
	metadata := map[string]interface{}{
		"directory_path":  cleanPath,
		"total_files":     totalFiles,
		"returned_files":  len(files),
		"skipped_files":   skippedFiles,
		"recursive":       recursive,
		"show_hidden":     showHidden,
		"show_details":    showDetails,
		"sort_by":         sortBy,
		"sort_desc":       sortDesc,
		"max_depth":       maxDepth,
		"max_files":       maxFiles,
	}
	
	if filterPattern != "" {
		metadata["filter_pattern"] = filterPattern
	}
	
	// Format result
	var resultText strings.Builder
	resultText.WriteString(fmt.Sprintf("Directory listing for %s\n", cleanPath))
	resultText.WriteString(fmt.Sprintf("Found %d files", totalFiles))
	if skippedFiles > 0 {
		resultText.WriteString(fmt.Sprintf(" (%d shown, %d skipped due to max_files limit)", len(files), skippedFiles))
	}
	resultText.WriteString("\n\n")
	
	if showDetails {
		l.formatDetailedListing(&resultText, files)
	} else {
		l.formatSimpleListing(&resultText, files)
	}
	
	return &ToolResult{
		Success:  true,
		Result:   resultText.String(),
		Metadata: metadata,
	}, nil
}

// listFiles recursively lists files in a directory
func (l *ListDirectoryExecutor) listFiles(ctx context.Context, basePath, relativePath string, currentDepth, maxDepth int, recursive, showHidden, showDetails bool, filterPattern string, files *[]FileInfo, totalFiles *int, skippedFiles *int, maxFiles int) error {
	// Check context for cancellation
	select {
	case <-ctx.Done():
		return &ToolExecutionError{
			ToolName:    l.Name(),
			Message:     "directory listing was cancelled",
			Cause:       ctx.Err(),
			Recoverable: true,
		}
	default:
	}
	
	// Check if we've hit max files
	if *totalFiles >= maxFiles {
		return nil
	}
	
	fullPath := filepath.Join(basePath, relativePath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return &ToolExecutionError{
			ToolName:    l.Name(),
			Message:     fmt.Sprintf("cannot read directory %s: %s", fullPath, err.Error()),
			Recoverable: true,
		}
	}
	
	for _, entry := range entries {
		// Check if we've hit max files
		if *totalFiles >= maxFiles {
			*skippedFiles = *skippedFiles + (len(entries) - *totalFiles + len(*files))
			break
		}
		
		name := entry.Name()
		entryPath := filepath.Join(fullPath, name)
		entryRelativePath := filepath.Join(relativePath, name)
		
		// Skip hidden files if not requested
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		
		// Get file info
		fileInfo, err := l.getFileInfo(entryPath, entryRelativePath, showDetails)
		if err != nil {
			// Skip files we can't access
			continue
		}
		
		// Apply filter pattern
		if filterPattern != "" {
			matched, err := filepath.Match(filterPattern, name)
			if err != nil {
				return &ToolValidationError{
					Parameter: "filter_pattern",
					Message:   fmt.Sprintf("invalid glob pattern: %s", err.Error()),
				}
			}
			if !matched {
				// For directories, still recurse but don't include in results
				if fileInfo.IsDirectory && recursive && currentDepth < maxDepth {
					err = l.listFiles(ctx, basePath, entryRelativePath, currentDepth+1, maxDepth, recursive, showHidden, showDetails, filterPattern, files, totalFiles, skippedFiles, maxFiles)
					if err != nil {
						return err
					}
				}
				continue
			}
		}
		
		// Add to results
		*files = append(*files, fileInfo)
		*totalFiles++
		
		// Recurse into subdirectories
		if fileInfo.IsDirectory && recursive && currentDepth < maxDepth {
			err = l.listFiles(ctx, basePath, entryRelativePath, currentDepth+1, maxDepth, recursive, showHidden, showDetails, filterPattern, files, totalFiles, skippedFiles, maxFiles)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

// getFileInfo retrieves detailed information about a file
func (l *ListDirectoryExecutor) getFileInfo(fullPath, relativePath string, showDetails bool) (FileInfo, error) {
	info, err := os.Lstat(fullPath) // Use Lstat to not follow symlinks
	if err != nil {
		return FileInfo{}, err
	}
	
	name := filepath.Base(fullPath)
	fileInfo := FileInfo{
		Name:        name,
		Path:        relativePath,
		Size:        info.Size(),
		IsDirectory: info.IsDir(),
		IsSymlink:   info.Mode()&os.ModeSymlink != 0,
		ModTime:     info.ModTime(),
		Hidden:      strings.HasPrefix(name, "."),
		Metadata:    make(map[string]interface{}),
	}
	
	if showDetails {
		// Get permissions
		fileInfo.Mode = info.Mode().String()
		fileInfo.Permissions = l.formatPermissions(info.Mode())
		
		// Get extension for files
		if !fileInfo.IsDirectory {
			ext := filepath.Ext(name)
			if ext != "" {
				fileInfo.Extension = ext[1:] // Remove the dot
			}
			
			// Simple MIME type detection
			fileInfo.MimeType = l.getMimeType(ext)
		}
		
		// Handle symlinks
		if fileInfo.IsSymlink {
			target, err := os.Readlink(fullPath)
			if err == nil {
				fileInfo.LinkTarget = target
			}
		}
		
		// Count children for directories
		if fileInfo.IsDirectory {
			if entries, err := os.ReadDir(fullPath); err == nil {
				fileInfo.Children = len(entries)
			}
		}
		
		// Add file system specific metadata
		fileInfo.Metadata["mode_octal"] = fmt.Sprintf("%04o", info.Mode().Perm())
		fileInfo.Metadata["size_human"] = l.formatSize(info.Size())
		fileInfo.Metadata["mod_time_unix"] = info.ModTime().Unix()
	}
	
	return fileInfo, nil
}

// formatPermissions converts file mode to rwx string
func (l *ListDirectoryExecutor) formatPermissions(mode os.FileMode) string {
	perms := make([]rune, 9)
	
	// Owner permissions
	if mode&0400 != 0 {
		perms[0] = 'r'
	} else {
		perms[0] = '-'
	}
	if mode&0200 != 0 {
		perms[1] = 'w'
	} else {
		perms[1] = '-'
	}
	if mode&0100 != 0 {
		perms[2] = 'x'
	} else {
		perms[2] = '-'
	}
	
	// Group permissions
	if mode&0040 != 0 {
		perms[3] = 'r'
	} else {
		perms[3] = '-'
	}
	if mode&0020 != 0 {
		perms[4] = 'w'
	} else {
		perms[4] = '-'
	}
	if mode&0010 != 0 {
		perms[5] = 'x'
	} else {
		perms[5] = '-'
	}
	
	// Other permissions
	if mode&0004 != 0 {
		perms[6] = 'r'
	} else {
		perms[6] = '-'
	}
	if mode&0002 != 0 {
		perms[7] = 'w'
	} else {
		perms[7] = '-'
	}
	if mode&0001 != 0 {
		perms[8] = 'x'
	} else {
		perms[8] = '-'
	}
	
	return string(perms)
}

// getMimeType returns a simple MIME type based on file extension
func (l *ListDirectoryExecutor) getMimeType(ext string) string {
	switch strings.ToLower(ext) {
	case ".txt", ".text":
		return "text/plain"
	case ".md", ".markdown":
		return "text/markdown"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".go":
		return "text/x-go"
	case ".py":
		return "text/x-python"
	case ".java":
		return "text/x-java"
	case ".c":
		return "text/x-c"
	case ".cpp", ".cxx", ".cc":
		return "text/x-c++"
	case ".h", ".hpp":
		return "text/x-c-header"
	case ".sh", ".bash":
		return "application/x-shellscript"
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

// formatSize converts bytes to human readable format
func (l *ListDirectoryExecutor) formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// sortFiles sorts the file list according to the specified criteria
func (l *ListDirectoryExecutor) sortFiles(files []FileInfo, sortBy string, descending bool) {
	switch sortBy {
	case "name":
		if descending {
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name > files[j].Name
			})
		} else {
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name < files[j].Name
			})
		}
	case "size":
		if descending {
			sort.Slice(files, func(i, j int) bool {
				return files[i].Size > files[j].Size
			})
		} else {
			sort.Slice(files, func(i, j int) bool {
				return files[i].Size < files[j].Size
			})
		}
	case "time":
		if descending {
			sort.Slice(files, func(i, j int) bool {
				return files[i].ModTime.After(files[j].ModTime)
			})
		} else {
			sort.Slice(files, func(i, j int) bool {
				return files[i].ModTime.Before(files[j].ModTime)
			})
		}
	case "type":
		if descending {
			sort.Slice(files, func(i, j int) bool {
				// Directories first, then by extension
				if files[i].IsDirectory && !files[j].IsDirectory {
					return true
				}
				if !files[i].IsDirectory && files[j].IsDirectory {
					return false
				}
				return files[i].Extension > files[j].Extension
			})
		} else {
			sort.Slice(files, func(i, j int) bool {
				// Directories first, then by extension
				if files[i].IsDirectory && !files[j].IsDirectory {
					return true
				}
				if !files[i].IsDirectory && files[j].IsDirectory {
					return false
				}
				return files[i].Extension < files[j].Extension
			})
		}
	}
}

// formatDetailedListing formats files with detailed information
func (l *ListDirectoryExecutor) formatDetailedListing(result *strings.Builder, files []FileInfo) {
	result.WriteString("Permissions  Size     Modified             Name\n")
	result.WriteString("-------------------------------------------------------\n")
	
	for _, file := range files {
		var prefix string
		if file.IsDirectory {
			prefix = "d"
		} else if file.IsSymlink {
			prefix = "l"
		} else {
			prefix = "-"
		}
		
		sizeStr := l.formatSize(file.Size)
		timeStr := file.ModTime.Format("2006-01-02 15:04:05")
		
		result.WriteString(fmt.Sprintf("%s%s %8s %s %s",
			prefix, file.Permissions, sizeStr, timeStr, file.Path))
		
		if file.IsSymlink && file.LinkTarget != "" {
			result.WriteString(" -> " + file.LinkTarget)
		}
		
		result.WriteString("\n")
	}
}

// formatSimpleListing formats files with names only
func (l *ListDirectoryExecutor) formatSimpleListing(result *strings.Builder, files []FileInfo) {
	for _, file := range files {
		if file.IsDirectory {
			result.WriteString(file.Path + "/\n")
		} else {
			result.WriteString(file.Path + "\n")
		}
	}
}

// FilePermissionsExecutor provides file permissions management capabilities
type FilePermissionsExecutor struct {
	*BaseExecutor
	securityPolicy *FileSecurityPolicy
}

// NewFilePermissionsExecutor creates a new file permissions tool executor
func NewFilePermissionsExecutor() *FilePermissionsExecutor {
	base := NewBaseExecutor("file_permissions", "Manage file and directory permissions")
	
	// Define schema
	base.AddRequiredParam("file_path", "string", "Path to the file or directory")
	base.AddRequiredParam("operation", "string", "Operation: get, set, add, remove")
	base.AddOptionalParam("permissions", "string", "Permission string (octal like 644, or symbolic like rwxr-xr-x)", "")
	base.AddOptionalParam("owner", "string", "Owner user (for chown operation)", "")
	base.AddOptionalParam("group", "string", "Group (for chgrp operation)", "")
	base.AddOptionalParam("recursive", "boolean", "Apply permissions recursively to directories", false)
	base.AddOptionalParam("force", "boolean", "Force operation even if risky", false)
	
	// Add examples
	base.AddExample(ToolParameters{
		"file_path":   "script.sh",
		"operation":   "set",
		"permissions": "755",
	})
	base.AddExample(ToolParameters{
		"file_path": "config.txt",
		"operation": "get",
	})
	base.AddExample(ToolParameters{
		"file_path":   "logs/",
		"operation":   "set",
		"permissions": "644",
		"recursive":   true,
	})
	
	return &FilePermissionsExecutor{
		BaseExecutor:   base,
		securityPolicy: NewDefaultFileSecurityPolicy(),
	}
}

// PermissionInfo represents detailed permission information
type PermissionInfo struct {
	Path        string `json:"path"`
	Octal       string `json:"octal"`
	Symbolic    string `json:"symbolic"`
	Owner       string `json:"owner,omitempty"`
	Group       string `json:"group,omitempty"`
	IsDirectory bool   `json:"is_directory"`
	CanRead     bool   `json:"can_read"`
	CanWrite    bool   `json:"can_write"`
	CanExecute  bool   `json:"can_execute"`
}

// Execute performs file permission operations
func (f *FilePermissionsExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Get required parameters
	path, err := GetRequiredStringParam(params, "file_path")
	if err != nil {
		return nil, err
	}
	
	operation, err := GetRequiredStringParam(params, "operation")
	if err != nil {
		return nil, err
	}
	
	// Get optional parameters
	permissions, _ := GetStringParam(params, "permissions")
	_, _ = GetStringParam(params, "owner")     // Reserved for future use
	_, _ = GetStringParam(params, "group")     // Reserved for future use
	recursive, _ := GetBoolParam(params, "recursive")
	force, _ := GetBoolParam(params, "force")
	
	// Validate operation
	operation = strings.ToLower(operation)
	if operation != "get" && operation != "set" && operation != "add" && operation != "remove" {
		return nil, &ToolValidationError{
			Parameter:  "operation",
			Message:    fmt.Sprintf("invalid operation: %s", operation),
			Suggestion: "use get, set, add, or remove",
		}
	}
	
	// Check file path security
	if err := f.securityPolicy.CheckPath(path); err != nil {
		return nil, err
	}
	
	// Clean path
	cleanPath := filepath.Clean(path)
	
	// Check if file/directory exists
	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &ToolExecutionError{
				ToolName:    f.Name(),
				Message:     fmt.Sprintf("file does not exist: %s", path),
				Recoverable: true,
			}
		}
		return nil, &ToolExecutionError{
			ToolName:    f.Name(),
			Message:     fmt.Sprintf("cannot access file: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	switch operation {
	case "get":
		return f.getPermissions(cleanPath, info)
	case "set":
		if permissions == "" {
			return nil, &ToolValidationError{
				Parameter: "permissions",
				Message:   "permissions parameter is required for set operation",
			}
		}
		return f.setPermissions(ctx, cleanPath, info, permissions, recursive, force)
	case "add":
		if permissions == "" {
			return nil, &ToolValidationError{
				Parameter: "permissions",
				Message:   "permissions parameter is required for add operation",
			}
		}
		return f.addPermissions(ctx, cleanPath, info, permissions, recursive)
	case "remove":
		if permissions == "" {
			return nil, &ToolValidationError{
				Parameter: "permissions",
				Message:   "permissions parameter is required for remove operation",
			}
		}
		return f.removePermissions(ctx, cleanPath, info, permissions, recursive)
	default:
		return nil, &ToolValidationError{
			Parameter: "operation",
			Message:   fmt.Sprintf("unsupported operation: %s", operation),
		}
	}
}

// getPermissions retrieves current file permissions
func (f *FilePermissionsExecutor) getPermissions(path string, info os.FileInfo) (*ToolResult, error) {
	permInfo := f.analyzePermissions(path, info)
	
	result := fmt.Sprintf("Permissions for %s:\n", path)
	result += fmt.Sprintf("  Octal: %s\n", permInfo.Octal)
	result += fmt.Sprintf("  Symbolic: %s\n", permInfo.Symbolic)
	result += fmt.Sprintf("  Type: ")
	if permInfo.IsDirectory {
		result += "Directory\n"
	} else {
		result += "File\n"
	}
	result += fmt.Sprintf("  Access: ")
	var access []string
	if permInfo.CanRead {
		access = append(access, "read")
	}
	if permInfo.CanWrite {
		access = append(access, "write")
	}
	if permInfo.CanExecute {
		access = append(access, "execute")
	}
	result += strings.Join(access, ", ")
	
	metadata := map[string]interface{}{
		"file_path":    path,
		"permissions":  permInfo,
		"operation":    "get",
	}
	
	return &ToolResult{
		Success:  true,
		Result:   result,
		Metadata: metadata,
	}, nil
}

// setPermissions sets file permissions
func (f *FilePermissionsExecutor) setPermissions(ctx context.Context, path string, info os.FileInfo, permissions string, recursive, force bool) (*ToolResult, error) {
	// Parse permissions
	mode, err := f.parsePermissions(permissions)
	if err != nil {
		return nil, &ToolValidationError{
			Parameter: "permissions",
			Message:   fmt.Sprintf("invalid permissions format: %s", err.Error()),
			Suggestion: "use octal format (e.g., 644, 755) or symbolic format (e.g., rwxr-xr-x)",
		}
	}
	
	// Safety check for dangerous permissions
	if !force && f.isDangerousPermission(mode, info.IsDir()) {
		return nil, &ToolExecutionError{
			ToolName:    f.Name(),
			Message:     fmt.Sprintf("refusing to set potentially dangerous permissions %o. Use force=true to override", mode),
			Recoverable: true,
		}
	}
	
	var changedFiles int
	var errors []string
	
	if recursive && info.IsDir() {
		err = f.setPermissionsRecursive(ctx, path, mode, &changedFiles, &errors)
	} else {
		err = os.Chmod(path, mode)
		if err == nil {
			changedFiles = 1
		} else {
			errors = append(errors, fmt.Sprintf("%s: %s", path, err.Error()))
		}
	}
	
	if err != nil && !recursive {
		return nil, &ToolExecutionError{
			ToolName:    f.Name(),
			Message:     fmt.Sprintf("cannot set permissions: %s", err.Error()),
			Recoverable: true,
		}
	}
	
	// Prepare result
	result := fmt.Sprintf("Set permissions %s on %s", permissions, path)
	if recursive {
		result += fmt.Sprintf(" (recursive: %d files changed)", changedFiles)
	}
	if len(errors) > 0 {
		result += fmt.Sprintf("\nErrors:\n%s", strings.Join(errors, "\n"))
	}
	
	metadata := map[string]interface{}{
		"file_path":     path,
		"permissions":   permissions,
		"operation":     "set",
		"recursive":     recursive,
		"changed_files": changedFiles,
		"error_count":   len(errors),
	}
	
	if len(errors) > 0 {
		metadata["errors"] = errors
	}
	
	success := len(errors) == 0 || changedFiles > 0
	
	return &ToolResult{
		Success:  success,
		Result:   result,
		Metadata: metadata,
	}, nil
}

// addPermissions adds permissions (bitwise OR)
func (f *FilePermissionsExecutor) addPermissions(ctx context.Context, path string, info os.FileInfo, permissions string, recursive bool) (*ToolResult, error) {
	addMode, err := f.parsePermissions(permissions)
	if err != nil {
		return nil, &ToolValidationError{
			Parameter: "permissions",
			Message:   fmt.Sprintf("invalid permissions format: %s", err.Error()),
		}
	}
	
	currentMode := info.Mode().Perm()
	newMode := currentMode | addMode
	
	var changedFiles int
	var errors []string
	
	if recursive && info.IsDir() {
		err = f.addPermissionsRecursive(ctx, path, addMode, &changedFiles, &errors)
	} else {
		err = os.Chmod(path, newMode)
		if err == nil {
			changedFiles = 1
		} else {
			errors = append(errors, fmt.Sprintf("%s: %s", path, err.Error()))
		}
	}
	
	result := fmt.Sprintf("Added permissions %s to %s", permissions, path)
	if recursive {
		result += fmt.Sprintf(" (recursive: %d files changed)", changedFiles)
	}
	if len(errors) > 0 {
		result += fmt.Sprintf("\nErrors:\n%s", strings.Join(errors, "\n"))
	}
	
	metadata := map[string]interface{}{
		"file_path":     path,
		"permissions":   permissions,
		"operation":     "add",
		"recursive":     recursive,
		"changed_files": changedFiles,
		"error_count":   len(errors),
	}
	
	success := len(errors) == 0 || changedFiles > 0
	
	return &ToolResult{
		Success:  success,
		Result:   result,
		Metadata: metadata,
	}, nil
}

// removePermissions removes permissions (bitwise AND NOT)
func (f *FilePermissionsExecutor) removePermissions(ctx context.Context, path string, info os.FileInfo, permissions string, recursive bool) (*ToolResult, error) {
	removeMode, err := f.parsePermissions(permissions)
	if err != nil {
		return nil, &ToolValidationError{
			Parameter: "permissions",
			Message:   fmt.Sprintf("invalid permissions format: %s", err.Error()),
		}
	}
	
	currentMode := info.Mode().Perm()
	newMode := currentMode &^ removeMode // AND NOT
	
	var changedFiles int
	var errors []string
	
	if recursive && info.IsDir() {
		err = f.removePermissionsRecursive(ctx, path, removeMode, &changedFiles, &errors)
	} else {
		err = os.Chmod(path, newMode)
		if err == nil {
			changedFiles = 1
		} else {
			errors = append(errors, fmt.Sprintf("%s: %s", path, err.Error()))
		}
	}
	
	result := fmt.Sprintf("Removed permissions %s from %s", permissions, path)
	if recursive {
		result += fmt.Sprintf(" (recursive: %d files changed)", changedFiles)
	}
	if len(errors) > 0 {
		result += fmt.Sprintf("\nErrors:\n%s", strings.Join(errors, "\n"))
	}
	
	metadata := map[string]interface{}{
		"file_path":     path,
		"permissions":   permissions,
		"operation":     "remove",
		"recursive":     recursive,
		"changed_files": changedFiles,
		"error_count":   len(errors),
	}
	
	success := len(errors) == 0 || changedFiles > 0
	
	return &ToolResult{
		Success:  success,
		Result:   result,
		Metadata: metadata,
	}, nil
}

// analyzePermissions analyzes current file permissions
func (f *FilePermissionsExecutor) analyzePermissions(path string, info os.FileInfo) PermissionInfo {
	mode := info.Mode()
	perm := mode.Perm()
	
	return PermissionInfo{
		Path:        path,
		Octal:       fmt.Sprintf("%03o", perm),
		Symbolic:    f.modeToSymbolic(mode),
		IsDirectory: info.IsDir(),
		CanRead:     perm&0400 != 0, // Owner read
		CanWrite:    perm&0200 != 0, // Owner write
		CanExecute:  perm&0100 != 0, // Owner execute
	}
}

// parsePermissions parses permission string (octal or symbolic)
func (f *FilePermissionsExecutor) parsePermissions(permissions string) (os.FileMode, error) {
	// Try octal first
	if len(permissions) == 3 || len(permissions) == 4 {
		var mode uint32
		for _, digit := range permissions {
			if digit < '0' || digit > '7' {
				break
			}
			mode = mode*8 + uint32(digit-'0')
		}
		if mode <= 0o777 {
			return os.FileMode(mode), nil
		}
	}
	
	// Try symbolic format (rwxrwxrwx)
	if len(permissions) == 9 {
		var mode uint32
		for i, char := range permissions {
			bit := uint32(8 - i)
			switch char {
			case 'r':
				if i%3 == 0 {
					mode |= 1 << bit
				} else {
					return 0, fmt.Errorf("invalid symbolic permission: 'r' in wrong position")
				}
			case 'w':
				if i%3 == 1 {
					mode |= 1 << bit
				} else {
					return 0, fmt.Errorf("invalid symbolic permission: 'w' in wrong position")
				}
			case 'x':
				if i%3 == 2 {
					mode |= 1 << bit
				} else {
					return 0, fmt.Errorf("invalid symbolic permission: 'x' in wrong position")
				}
			case '-':
				// No permission set for this bit
			default:
				return 0, fmt.Errorf("invalid character in symbolic permission: %c", char)
			}
		}
		return os.FileMode(mode), nil
	}
	
	return 0, fmt.Errorf("invalid permission format: must be 3-4 digit octal (e.g., 644) or 9-character symbolic (e.g., rwxr-xr-x)")
}

// modeToSymbolic converts file mode to symbolic string
func (f *FilePermissionsExecutor) modeToSymbolic(mode os.FileMode) string {
	perm := mode.Perm()
	symbolic := make([]byte, 9)
	
	// Owner
	if perm&0400 != 0 { symbolic[0] = 'r' } else { symbolic[0] = '-' }
	if perm&0200 != 0 { symbolic[1] = 'w' } else { symbolic[1] = '-' }
	if perm&0100 != 0 { symbolic[2] = 'x' } else { symbolic[2] = '-' }
	
	// Group
	if perm&0040 != 0 { symbolic[3] = 'r' } else { symbolic[3] = '-' }
	if perm&0020 != 0 { symbolic[4] = 'w' } else { symbolic[4] = '-' }
	if perm&0010 != 0 { symbolic[5] = 'x' } else { symbolic[5] = '-' }
	
	// Other
	if perm&0004 != 0 { symbolic[6] = 'r' } else { symbolic[6] = '-' }
	if perm&0002 != 0 { symbolic[7] = 'w' } else { symbolic[7] = '-' }
	if perm&0001 != 0 { symbolic[8] = 'x' } else { symbolic[8] = '-' }
	
	return string(symbolic)
}

// isDangerousPermission checks if a permission is potentially dangerous
func (f *FilePermissionsExecutor) isDangerousPermission(mode os.FileMode, isDir bool) bool {
	// World writable
	if mode&0002 != 0 {
		return true
	}
	
	// Executable for everyone on non-directories
	if !isDir && mode&0111 == 0111 {
		return true
	}
	
	// Too permissive (777)
	if mode&0777 == 0777 {
		return true
	}
	
	return false
}

// setPermissionsRecursive sets permissions recursively
func (f *FilePermissionsExecutor) setPermissionsRecursive(ctx context.Context, root string, mode os.FileMode, changedFiles *int, errors *[]string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			return &ToolExecutionError{
				ToolName:    f.Name(),
				Message:     "permission setting was cancelled",
				Cause:       ctx.Err(),
				Recoverable: true,
			}
		default:
		}
		
		if err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
			return nil // Continue walking
		}
		
		if err := os.Chmod(path, mode); err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
		} else {
			*changedFiles++
		}
		
		return nil
	})
}

// addPermissionsRecursive adds permissions recursively
func (f *FilePermissionsExecutor) addPermissionsRecursive(ctx context.Context, root string, addMode os.FileMode, changedFiles *int, errors *[]string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
			return nil
		}
		
		info, err := os.Stat(path)
		if err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
			return nil
		}
		
		newMode := info.Mode().Perm() | addMode
		if err := os.Chmod(path, newMode); err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
		} else {
			*changedFiles++
		}
		
		return nil
	})
}

// removePermissionsRecursive removes permissions recursively
func (f *FilePermissionsExecutor) removePermissionsRecursive(ctx context.Context, root string, removeMode os.FileMode, changedFiles *int, errors *[]string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
			return nil
		}
		
		info, err := os.Stat(path)
		if err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
			return nil
		}
		
		newMode := info.Mode().Perm() &^ removeMode
		if err := os.Chmod(path, newMode); err != nil {
			*errors = append(*errors, fmt.Sprintf("%s: %s", path, err.Error()))
		} else {
			*changedFiles++
		}
		
		return nil
	})
}
