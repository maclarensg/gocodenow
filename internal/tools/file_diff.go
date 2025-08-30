package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// DiffPreviewExecutor provides diff preview functionality before applying file changes
type DiffPreviewExecutor struct {
	highlighter *SyntaxHighlighter
}

// NewDiffPreviewExecutor creates a new diff preview executor
func NewDiffPreviewExecutor() *DiffPreviewExecutor {
	return &DiffPreviewExecutor{
		highlighter: NewSyntaxHighlighter("monokai"),
	}
}

// Execute generates a diff preview for proposed file changes
func (d *DiffPreviewExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	filePath, ok := params["file_path"].(string)
	if !ok || filePath == "" {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "file_path parameter is required",
		}, nil
	}

	newContent, ok := params["new_content"].(string)
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "new_content parameter is required",
		}, nil
	}

	// Optional parameters
	contextLines := 3
	if cl, ok := params["context_lines"].(float64); ok {
		contextLines = int(cl)
	}

	showSyntaxHighlighting := true
	if sh, ok := params["syntax_highlighting"].(bool); ok {
		showSyntaxHighlighting = sh
	}

	theme := "monokai"
	if t, ok := params["theme"].(string); ok && t != "" {
		theme = t
	}

	// Read current file content
	currentContent := ""
	if _, err := os.Stat(filePath); err == nil {
		contentBytes, err := os.ReadFile(filePath)
		if err != nil {
			return &ToolResult{
				Success: false,
				ErrorMessage:   fmt.Sprintf("Failed to read current file: %v", err),
			}, nil
		}
		currentContent = string(contentBytes)
	}

	// Generate unified diff
	diff, err := d.generateUnifiedDiff(filePath, currentContent, newContent, contextLines)
	if err != nil {
		return &ToolResult{
			Success: false,
			ErrorMessage:   fmt.Sprintf("Failed to generate diff: %v", err),
		}, nil
	}

	// Apply syntax highlighting if requested
	var finalDiff string
	if showSyntaxHighlighting {
		d.highlighter.SetTheme(theme)
		highlighted, err := d.highlighter.HighlightDiff(diff, DiffHighlightOptions{
			ShowLineNumbers: false,
		})
		if err != nil {
			// Fall back to plain diff if highlighting fails
			finalDiff = diff
		} else {
			finalDiff = highlighted
		}
	} else {
		finalDiff = diff
	}

	// Calculate diff statistics
	stats := d.calculateDiffStats(diff)

	return &ToolResult{
		Success: true,
		Result:  finalDiff,
		Metadata: map[string]interface{}{
			"file_path":           filePath,
			"context_lines":       contextLines,
			"syntax_highlighted":  showSyntaxHighlighting,
			"theme":              theme,
			"lines_added":        stats.LinesAdded,
			"lines_removed":      stats.LinesRemoved,
			"lines_modified":     stats.LinesModified,
			"hunks":              stats.Hunks,
			"generated_at":       time.Now().Format(time.RFC3339),
		},
	}, nil
}

// DiffStats represents statistics about a diff
type DiffStats struct {
	LinesAdded    int
	LinesRemoved  int
	LinesModified int
	Hunks         int
}

// generateUnifiedDiff creates a unified diff between old and new content
func (d *DiffPreviewExecutor) generateUnifiedDiff(filePath, oldContent, newContent string, contextLines int) (string, error) {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Simple diff implementation using Myers algorithm approximation
	diff := d.computeDiff(oldLines, newLines)
	
	// Generate unified diff format
	var builder strings.Builder
	
	builder.WriteString(fmt.Sprintf("--- a%s\n", filePath))
	builder.WriteString(fmt.Sprintf("+++ b%s\n", filePath))
	
	// Group changes into hunks
	hunks := d.groupIntoHunks(diff, contextLines)
	
	for _, hunk := range hunks {
		builder.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", 
			hunk.OldStart, hunk.OldLines, hunk.NewStart, hunk.NewLines))
		
		for _, line := range hunk.Lines {
			builder.WriteString(line + "\n")
		}
	}
	
	return builder.String(), nil
}

// DiffOperation represents a single diff operation
type DiffOperation struct {
	Type    string // "equal", "delete", "insert"
	OldLine int
	NewLine int
	Text    string
}

// DiffHunk represents a hunk in a unified diff
type DiffHunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []string
}

// computeDiff computes the diff operations between old and new lines
func (d *DiffPreviewExecutor) computeDiff(oldLines, newLines []string) []DiffOperation {
	var operations []DiffOperation
	
	// Simple line-by-line comparison (can be enhanced with proper diff algorithm)
	oldIdx, newIdx := 0, 0
	
	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if oldIdx >= len(oldLines) {
			// Remaining lines are insertions
			operations = append(operations, DiffOperation{
				Type:    "insert",
				OldLine: oldIdx,
				NewLine: newIdx,
				Text:    newLines[newIdx],
			})
			newIdx++
		} else if newIdx >= len(newLines) {
			// Remaining lines are deletions
			operations = append(operations, DiffOperation{
				Type:    "delete",
				OldLine: oldIdx,
				NewLine: newIdx,
				Text:    oldLines[oldIdx],
			})
			oldIdx++
		} else if oldLines[oldIdx] == newLines[newIdx] {
			// Lines are equal
			operations = append(operations, DiffOperation{
				Type:    "equal",
				OldLine: oldIdx,
				NewLine: newIdx,
				Text:    oldLines[oldIdx],
			})
			oldIdx++
			newIdx++
		} else {
			// Look ahead to see if this is a modification or separate insert/delete
			found := false
			
			// Look for the old line in upcoming new lines (within reasonable distance)
			for i := newIdx + 1; i < min(newIdx+5, len(newLines)); i++ {
				if oldLines[oldIdx] == newLines[i] {
					// Insert the lines before the match
					for j := newIdx; j < i; j++ {
						operations = append(operations, DiffOperation{
							Type:    "insert",
							OldLine: oldIdx,
							NewLine: j,
							Text:    newLines[j],
						})
					}
					newIdx = i
					found = true
					break
				}
			}
			
			if !found {
				// Look for the new line in upcoming old lines
				for i := oldIdx + 1; i < min(oldIdx+5, len(oldLines)); i++ {
					if newLines[newIdx] == oldLines[i] {
						// Delete the lines before the match
						for j := oldIdx; j < i; j++ {
							operations = append(operations, DiffOperation{
								Type:    "delete",
								OldLine: j,
								NewLine: newIdx,
								Text:    oldLines[j],
							})
						}
						oldIdx = i
						found = true
						break
					}
				}
			}
			
			if !found {
				// Treat as a modification (delete + insert)
				operations = append(operations, DiffOperation{
					Type:    "delete",
					OldLine: oldIdx,
					NewLine: newIdx,
					Text:    oldLines[oldIdx],
				})
				operations = append(operations, DiffOperation{
					Type:    "insert",
					OldLine: oldIdx + 1,
					NewLine: newIdx,
					Text:    newLines[newIdx],
				})
				oldIdx++
				newIdx++
			}
		}
	}
	
	return operations
}

// groupIntoHunks groups diff operations into hunks with context
func (d *DiffPreviewExecutor) groupIntoHunks(operations []DiffOperation, contextLines int) []DiffHunk {
	if len(operations) == 0 {
		return nil
	}
	
	var hunks []DiffHunk
	var currentHunk *DiffHunk
	
	for i, op := range operations {
		if op.Type != "equal" {
			// This is a change, start a new hunk if needed
			if currentHunk == nil {
				currentHunk = &DiffHunk{
					OldStart: max(1, op.OldLine-contextLines+1),
					NewStart: max(1, op.NewLine-contextLines+1),
				}
				
				// Add context lines before the change
				for j := max(0, op.OldLine-contextLines); j < op.OldLine && j < len(operations); j++ {
					if j < len(operations) && operations[j].Type == "equal" {
						currentHunk.Lines = append(currentHunk.Lines, " "+operations[j].Text)
						currentHunk.OldLines++
						currentHunk.NewLines++
					}
				}
			}
			
			// Add the change line
			switch op.Type {
			case "delete":
				currentHunk.Lines = append(currentHunk.Lines, "-"+op.Text)
				currentHunk.OldLines++
			case "insert":
				currentHunk.Lines = append(currentHunk.Lines, "+"+op.Text)
				currentHunk.NewLines++
			}
			
			// Look ahead for context lines or end of changes
			contextAdded := 0
			for j := i + 1; j < len(operations) && contextAdded < contextLines; j++ {
				if operations[j].Type == "equal" {
					currentHunk.Lines = append(currentHunk.Lines, " "+operations[j].Text)
					currentHunk.OldLines++
					currentHunk.NewLines++
					contextAdded++
				} else {
					break
				}
			}
			
			// Check if we should close this hunk
			shouldClose := true
			for j := i + 1; j < min(i+1+contextLines*2, len(operations)); j++ {
				if operations[j].Type != "equal" {
					shouldClose = false
					break
				}
			}
			
			if shouldClose || i == len(operations)-1 {
				hunks = append(hunks, *currentHunk)
				currentHunk = nil
			}
		}
	}
	
	return hunks
}

// calculateDiffStats calculates statistics about the diff
func (d *DiffPreviewExecutor) calculateDiffStats(diff string) DiffStats {
	stats := DiffStats{}
	lines := strings.Split(diff, "\n")
	
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			stats.LinesAdded++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			stats.LinesRemoved++
		} else if strings.HasPrefix(line, "@@") {
			stats.Hunks++
		}
	}
	
	// Estimate modified lines (pairs of delete+insert)
	stats.LinesModified = min(stats.LinesAdded, stats.LinesRemoved)
	
	return stats
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// InteractiveDiffPreviewExecutor provides interactive diff preview with approval workflow
type InteractiveDiffPreviewExecutor struct {
	diffPreview *DiffPreviewExecutor
}

// NewInteractiveDiffPreviewExecutor creates a new interactive diff preview executor
func NewInteractiveDiffPreviewExecutor() *InteractiveDiffPreviewExecutor {
	return &InteractiveDiffPreviewExecutor{
		diffPreview: NewDiffPreviewExecutor(),
	}
}

// Execute provides interactive diff preview with user approval workflow
func (i *InteractiveDiffPreviewExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Generate the diff preview first
	result, err := i.diffPreview.Execute(ctx, params)
	if err != nil {
		return result, err
	}
	
	if !result.Success {
		return result, nil
	}
	
	// Check if auto-approve is enabled
	autoApprove, _ := params["auto_approve"].(bool)
	if autoApprove {
		result.Metadata["approval_status"] = "auto_approved"
		result.Metadata["user_action_required"] = false
		return result, nil
	}
	
	// Add interactive elements to the result
	result.Metadata["approval_status"] = "pending_approval"
	result.Metadata["user_action_required"] = true
	result.Metadata["approval_options"] = map[string]string{
		"approve": "Apply the changes shown in the diff",
		"reject":  "Reject the changes and keep the current file",
		"edit":    "Make additional modifications before applying",
	}
	
	return result, nil
}

// BatchDiffPreviewExecutor handles diff previews for multiple files
type BatchDiffPreviewExecutor struct {
	diffPreview *DiffPreviewExecutor
}

// NewBatchDiffPreviewExecutor creates a new batch diff preview executor
func NewBatchDiffPreviewExecutor() *BatchDiffPreviewExecutor {
	return &BatchDiffPreviewExecutor{
		diffPreview: NewDiffPreviewExecutor(),
	}
}

// Execute generates diff previews for multiple files
func (b *BatchDiffPreviewExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	changes, ok := params["changes"].([]interface{})
	if !ok {
		return &ToolResult{
			Success: false,
			ErrorMessage:   "changes parameter must be an array of file changes",
		}, nil
	}
	
	var results []map[string]interface{}
	totalStats := DiffStats{}
	
	for i, change := range changes {
		changeMap, ok := change.(map[string]interface{})
		if !ok {
			return &ToolResult{
				Success: false,
				ErrorMessage:   fmt.Sprintf("Change %d must be an object with file_path and new_content", i),
			}, nil
		}
		
		// Execute diff preview for this file
		result, err := b.diffPreview.Execute(ctx, ToolParameters(changeMap))
		if err != nil {
			return &ToolResult{
				Success: false,
				ErrorMessage:   fmt.Sprintf("Failed to generate diff for change %d: %v", i, err),
			}, nil
		}
		
		if !result.Success {
			return &ToolResult{
				Success: false,
				ErrorMessage:   fmt.Sprintf("Diff preview failed for change %d: %s", i, result.ErrorMessage),
			}, nil
		}
		
		// Accumulate statistics
		if stats := result.Metadata; stats != nil {
			if added, ok := stats["lines_added"].(int); ok {
				totalStats.LinesAdded += added
			}
			if removed, ok := stats["lines_removed"].(int); ok {
				totalStats.LinesRemoved += removed
			}
			if modified, ok := stats["lines_modified"].(int); ok {
				totalStats.LinesModified += modified
			}
			if hunks, ok := stats["hunks"].(int); ok {
				totalStats.Hunks += hunks
			}
		}
		
		results = append(results, map[string]interface{}{
			"file_path": changeMap["file_path"],
			"diff":      result.Result,
			"metadata":  result.Metadata,
		})
	}
	
	return &ToolResult{
		Success: true,
		Result:  results,
		Metadata: map[string]interface{}{
			"total_files":        len(results),
			"total_lines_added":  totalStats.LinesAdded,
			"total_lines_removed": totalStats.LinesRemoved,
			"total_lines_modified": totalStats.LinesModified,
			"total_hunks":        totalStats.Hunks,
			"generated_at":       time.Now().Format(time.RFC3339),
		},
	}, nil
}

// FileDiffOptions represents options for file diff operations
type FileDiffOptions struct {
	ContextLines        int    `json:"context_lines"`
	SyntaxHighlighting  bool   `json:"syntax_highlighting"`
	Theme               string `json:"theme"`
	IgnoreWhitespace    bool   `json:"ignore_whitespace"`
	IgnoreCase          bool   `json:"ignore_case"`
	ShowWordDiff        bool   `json:"show_word_diff"`
}

// AdvancedDiffPreviewExecutor provides advanced diff preview with enhanced options
type AdvancedDiffPreviewExecutor struct {
	diffPreview *DiffPreviewExecutor
}

// NewAdvancedDiffPreviewExecutor creates a new advanced diff preview executor
func NewAdvancedDiffPreviewExecutor() *AdvancedDiffPreviewExecutor {
	return &AdvancedDiffPreviewExecutor{
		diffPreview: NewDiffPreviewExecutor(),
	}
}

// Execute provides advanced diff preview with enhanced options
func (a *AdvancedDiffPreviewExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	// Extract advanced options
	options := FileDiffOptions{
		ContextLines:       3,
		SyntaxHighlighting: true,
		Theme:             "monokai",
		IgnoreWhitespace:  false,
		IgnoreCase:        false,
		ShowWordDiff:      false,
	}
	
	if opts, ok := params["options"].(map[string]interface{}); ok {
		if cl, ok := opts["context_lines"].(float64); ok {
			options.ContextLines = int(cl)
		}
		if sh, ok := opts["syntax_highlighting"].(bool); ok {
			options.SyntaxHighlighting = sh
		}
		if theme, ok := opts["theme"].(string); ok {
			options.Theme = theme
		}
		if iw, ok := opts["ignore_whitespace"].(bool); ok {
			options.IgnoreWhitespace = iw
		}
		if ic, ok := opts["ignore_case"].(bool); ok {
			options.IgnoreCase = ic
		}
		if wd, ok := opts["show_word_diff"].(bool); ok {
			options.ShowWordDiff = wd
		}
	}
	
	// Prepare parameters for base diff preview
	diffParams := make(ToolParameters)
	for k, v := range params {
		diffParams[k] = v
	}
	diffParams["context_lines"] = float64(options.ContextLines)
	diffParams["syntax_highlighting"] = options.SyntaxHighlighting
	diffParams["theme"] = options.Theme
	
	// Apply preprocessing if needed
	if options.IgnoreWhitespace || options.IgnoreCase {
		newContent := params["new_content"].(string)
		
		if options.IgnoreWhitespace {
			newContent = strings.ReplaceAll(newContent, " ", "")
			newContent = strings.ReplaceAll(newContent, "\t", "")
		}
		
		if options.IgnoreCase {
			newContent = strings.ToLower(newContent)
		}
		
		diffParams["new_content"] = newContent
	}
	
	result, err := a.diffPreview.Execute(ctx, diffParams)
	if err != nil {
		return result, err
	}
	
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	
	result.Metadata["advanced_options"] = options
	result.Metadata["preprocessing_applied"] = options.IgnoreWhitespace || options.IgnoreCase
	
	return result, nil
}