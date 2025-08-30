// Simple program to test the context detection system on the current project
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gocodenow/internal/context"
)

func main() {
	// Get the current working directory (should be the project root)
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	
	// Create a context detector with default options
	detector := context.NewDefaultContextDetector()
	
	fmt.Printf("Analyzing project at: %s\n", wd)
	fmt.Println("=" + strings.Repeat("=", 50))
	
	// Detect the project context
	ctx, err := detector.DetectContext(wd)
	if err != nil {
		log.Fatalf("Failed to detect context: %v", err)
	}
	
	// Display the results
	fmt.Printf("Project Name: %s\n", ctx.ProjectName)
	fmt.Printf("Primary Language: %s\n", ctx.PrimaryLanguage)
	fmt.Printf("Workspace Root: %s\n", ctx.WorkspaceRoot)
	fmt.Printf("Is Git Repository: %t\n", ctx.Git.IsRepo)
	if ctx.Git.IsRepo {
		fmt.Printf("Current Branch: %s\n", ctx.Git.CurrentBranch)
		fmt.Printf("Has Uncommitted Changes: %t\n", ctx.Git.HasUncommitted)
		fmt.Printf("Remote URL: %s\n", ctx.Git.RemoteURL)
	}
	
	fmt.Println("\nDetected Languages:")
	for _, lang := range ctx.Languages {
		fmt.Printf("  - %s: %d files (%.1f%% confidence)%s\n", 
			lang.Name, 
			lang.FileCount, 
			lang.Confidence*100,
			func() string {
				if lang.Primary {
					return " [PRIMARY]"
				}
				return ""
			}(),
		)
	}
	
	fmt.Println("\nBuild Systems:")
	for _, bs := range ctx.BuildSystems {
		fmt.Printf("  - %s (%s)\n", bs.Type, bs.ConfigFile)
		if bs.Version != "" {
			fmt.Printf("    Version: %s\n", bs.Version)
		}
		if len(bs.Dependencies) > 0 {
			fmt.Printf("    Dependencies (%d): %v\n", len(bs.Dependencies), bs.Dependencies[:min(3, len(bs.Dependencies))])
		}
		if len(bs.Scripts) > 0 {
			fmt.Printf("    Available Scripts: %v\n", getKeys(bs.Scripts))
		}
	}
	
	fmt.Println("\nFile Statistics:")
	fmt.Printf("  Total Files: %d\n", ctx.Stats.TotalFiles)
	fmt.Printf("  Total Directories: %d\n", ctx.Stats.DirectoryCount)
	fmt.Printf("  Language Breakdown:\n")
	for lang, count := range ctx.Stats.LanguageBreakdown {
		fmt.Printf("    %s: %d files\n", lang, count)
	}
	
	if len(ctx.Stats.LargestFiles) > 0 {
		fmt.Println("\n  Largest Files:")
		for i, file := range ctx.Stats.LargestFiles {
			if i >= 5 { // Show only top 5
				break
			}
			fmt.Printf("    %s (%d bytes)\n", file.RelativePath, file.Size)
		}
	}
	
	fmt.Printf("\nIgnore Patterns (%d):\n", len(ctx.IgnorePatterns))
	for i, pattern := range ctx.IgnorePatterns {
		if i >= 10 { // Show only first 10
			fmt.Printf("  ... and %d more\n", len(ctx.IgnorePatterns)-10)
			break
		}
		fmt.Printf("  %s\n", pattern)
	}
	
	fmt.Printf("\nCache Information:\n")
	fmt.Printf("  Cache Version: %s\n", ctx.CacheVersion)
	fmt.Printf("  Last Updated: %s\n", ctx.LastUpdated.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Detection Time: %s\n", ctx.DetectedAt.Format("2006-01-02 15:04:05"))
	
	// Save full context as JSON for inspection
	if len(os.Args) > 1 && os.Args[1] == "--json" {
		jsonData, err := json.MarshalIndent(ctx, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal context to JSON: %v", err)
		}
		
		outputFile := filepath.Join(wd, "project-context.json")
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			log.Fatalf("Failed to write JSON output: %v", err)
		}
		
		fmt.Printf("\nFull context saved to: %s\n", outputFile)
	}
	
	fmt.Println("\n" + strings.Repeat("=", 51))
	fmt.Println("Context detection completed successfully!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

