// Demo program showcasing the Git integration tools
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"gocodenow/internal/tools"
)

func main() {
	fmt.Println("📍 Git Integration Tools Demo")
	fmt.Println(strings.Repeat("=", 50))
	
	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	
	// Demo 1: Git Status - Show repository status
	fmt.Println("\n🔍 Git Status Demo - Repository status:")
	statusExecutor := tools.NewGitStatusExecutor()
	params := map[string]interface{}{
		"path":          wd,
		"show_branch":   true,
		"show_remote":   true,
		"include_untracked": true,
	}
	
	result, err := statusExecutor.Execute(params)
	if err != nil {
		log.Printf("Status error: %v", err)
	} else if result.Success {
		data := result.Result.(*tools.GitStatusResult)
		fmt.Printf("   Branch: %s\n", data.Branch)
		if data.Ahead > 0 || data.Behind > 0 {
			fmt.Printf("   Tracking: ahead %d, behind %d\n", data.Ahead, data.Behind)
		}
		fmt.Printf("   Changes: %d total\n", data.TotalChanges)
		if len(data.Modified) > 0 {
			fmt.Printf("     Modified: %d files\n", len(data.Modified))
		}
		if len(data.Added) > 0 {
			fmt.Printf("     Added: %d files\n", len(data.Added))
		}
		if len(data.Untracked) > 0 {
			fmt.Printf("     Untracked: %d files\n", len(data.Untracked))
		}
		if summary, ok := result.Metadata["summary"]; ok {
			fmt.Printf("   %s\n", summary)
		}
	}
	
	// Demo 2: Git Log - Show commit history
	fmt.Println("\n📜 Git Log Demo - Recent commit history:")
	logExecutor := tools.NewGitLogExecutor()
	params = map[string]interface{}{
		"path":        wd,
		"max_commits": 5,
		"show_stats":  false,
	}
	
	result, err = logExecutor.Execute(params)
	if err != nil {
		log.Printf("Log error: %v", err)
	} else if result.Success {
		data := result.Result.(*tools.GitLogResult)
		fmt.Printf("   Found %d recent commits\n", len(data.Commits))
		fmt.Printf("   Authors: %d contributors\n", len(data.Authors))
		
		// Show most recent commits
		maxShow := 3
		if len(data.Commits) < maxShow {
			maxShow = len(data.Commits)
		}
		for i := 0; i < maxShow; i++ {
			commit := data.Commits[i]
			fmt.Printf("     %s - %s (%s)\n", 
				commit.ShortHash, 
				commit.Subject, 
				commit.Author)
		}
		
		if summary, ok := result.Metadata["summary"]; ok {
			fmt.Printf("   %s\n", summary)
		}
	}
	
	// Demo 3: Git Diff - Show current changes
	fmt.Println("\n📋 Git Diff Demo - Working tree changes:")
	diffExecutor := tools.NewGitDiffExecutor()
	params = map[string]interface{}{
		"path":      wd,
		"context":   3,
		"max_lines": 50,
	}
	
	result, err = diffExecutor.Execute(params)
	if err != nil {
		log.Printf("Diff error: %v", err)
	} else if result.Success {
		data := result.Result.(*tools.GitDiffResult)
		fmt.Printf("   Changes in %d files\n", data.TotalFiles)
		if data.Additions > 0 || data.Deletions > 0 {
			fmt.Printf("   Lines: +%d -%d\n", data.Additions, data.Deletions)
		}
		
		// Show changed files
		maxShow := 5
		if len(data.Files) < maxShow {
			maxShow = len(data.Files)
		}
		for i := 0; i < maxShow; i++ {
			file := data.Files[i]
			fmt.Printf("     %s (+%d -%d) [%s]\n", 
				file.Path, 
				file.Additions, 
				file.Deletions,
				file.Language)
		}
		
		if summary, ok := result.Metadata["summary"]; ok {
			fmt.Printf("   %s\n", summary)
		}
	}
	
	// Demo 4: Git Blame - Show authorship for a specific file
	fmt.Println("\n👤 Git Blame Demo - File authorship analysis:")
	blameExecutor := tools.NewGitBlameExecutor()
	
	// Find a suitable file to blame
	testFiles := []string{
		"README.md",
		"go.mod", 
		"main.go",
		"cmd/gocodenow/main.go",
		"internal/app/app.go",
	}
	
	blameFile := ""
	for _, file := range testFiles {
		fullPath := strings.TrimSpace(wd + "/" + file)
		if _, err := os.Stat(fullPath); err == nil {
			blameFile = fullPath
			break
		}
	}
	
	if blameFile != "" {
		params = map[string]interface{}{
			"file_path":  blameFile,
			"show_email": false,
			"show_date":  true,
		}
		
		result, err = blameExecutor.Execute(params)
		if err != nil {
			log.Printf("Blame error: %v", err)
		} else if result.Success {
			data := result.Result.(*tools.GitBlameResult)
			fmt.Printf("   File: %s\n", strings.TrimPrefix(data.FilePath, wd+"/"))
			fmt.Printf("   Lines: %d total\n", data.TotalLines)
			fmt.Printf("   Authors: %d contributors\n", len(data.Authors))
			
			// Show top contributors
			type authorStat struct {
				name  string
				lines int
			}
			
			var authors []authorStat
			for name, count := range data.Authors {
				authors = append(authors, authorStat{name, count})
			}
			
			// Sort by line count (simple bubble sort for small data)
			for i := 0; i < len(authors); i++ {
				for j := i + 1; j < len(authors); j++ {
					if authors[j].lines > authors[i].lines {
						authors[i], authors[j] = authors[j], authors[i]
					}
				}
			}
			
			maxShow := 3
			if len(authors) < maxShow {
				maxShow = len(authors)
			}
			for i := 0; i < maxShow; i++ {
				author := authors[i]
				percentage := float64(author.lines) / float64(data.TotalLines) * 100
				fmt.Printf("     %s: %d lines (%.1f%%)\n", 
					author.name, author.lines, percentage)
			}
			
			if summary, ok := result.Metadata["summary"]; ok {
				fmt.Printf("   %s\n", summary)
			}
		}
	} else {
		fmt.Println("   No suitable files found for blame demo")
	}
	
	// Save detailed results as JSON if requested
	if len(os.Args) > 1 && os.Args[1] == "--json" {
		allResults := map[string]interface{}{
			"git_demo":       "Git Integration Tools",
			"workspace_path": wd,
			"demos": map[string]interface{}{
				"git_status": result.Result,
				"git_log":    result.Result,
				"git_diff":   result.Result,
				"git_blame":  result.Result,
			},
		}
		
		jsonData, _ := json.MarshalIndent(allResults, "", "  ")
		outputFile := "git-demo-results.json"
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			log.Printf("Failed to write JSON: %v", err)
		} else {
			fmt.Printf("\n💾 Detailed results saved to: %s\n", outputFile)
		}
	}
	
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("✅ Git Integration Tools Demo Complete!")
	fmt.Println("   Use --json flag to save detailed results")
}