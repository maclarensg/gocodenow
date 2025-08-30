// Demo program showcasing the advanced search and navigation tools
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
	fmt.Println("🔍 Advanced Search & Navigation Tools Demo")
	fmt.Println(strings.Repeat("=", 50))
	
	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	
	// Demo 1: SearchFiles - Find Go files
	fmt.Println("\n📁 File Search Demo - Finding Go files:")
	searchExecutor := tools.NewSearchFilesExecutor()
	params := map[string]interface{}{
		"pattern":    "*.go",
		"path":       wd,
		"max_results": 10,
	}
	
	result, err := searchExecutor.Execute(params)
	if err != nil {
		log.Printf("Search error: %v", err)
	} else if result.Success {
		data := result.Result.(map[string]interface{})
		fmt.Printf("   Found %d Go files\n", data["total_matches"].(int))
		if summary, ok := result.Metadata["summary"]; ok {
			fmt.Printf("   %s\n", summary)
		}
	}
	
	// Demo 2: FindInFiles - Search for "func" in Go files  
	fmt.Println("\n🔎 Content Search Demo - Finding 'func' in Go files:")
	findExecutor := tools.NewFindInFilesExecutor()
	params = map[string]interface{}{
		"pattern":    "func ",
		"path":       wd,
		"file_types": []interface{}{"go"},
		"max_results": 5,
		"context":    1,
	}
	
	result, err = findExecutor.Execute(params)
	if err != nil {
		log.Printf("Find error: %v", err)
	} else if result.Success {
		data := result.Result.(map[string]interface{})
		fmt.Printf("   Found %d matches across %d files\n", 
			data["total_matches"].(int),
			data["files_matched"].(int))
		if summary, ok := result.Metadata["summary"]; ok {
			fmt.Printf("   %s\n", summary)
		}
	}
	
	// Demo 3: Grep - Search with line numbers and context
	fmt.Println("\n📋 Grep Demo - Finding 'package' with context:")
	grepExecutor := tools.NewGrepExecutor()
	params = map[string]interface{}{
		"pattern":        "^package ",
		"path":           wd,
		"regex":          false,
		"line_numbers":   true,
		"context_after":  1,
		"file_types":     []interface{}{"go"},
		"max_results":    3,
		"color":          false, // Disable ANSI colors for demo
	}
	
	result, err = grepExecutor.Execute(params)
	if err != nil {
		log.Printf("Grep error: %v", err)
	} else if result.Success {
		data := result.Result.(map[string]interface{})
		fmt.Printf("   Found %d matches in %d files\n",
			data["total_matches"].(int),
			data["files_matched"].(int))
		if formattedOutput, ok := data["formatted_output"]; ok {
			// Show first few lines of output
			output := formattedOutput.(string)
			lines := strings.Split(output, "\n")
			maxLines := 10
			if len(lines) < maxLines {
				maxLines = len(lines)
			}
			for i := 0; i < maxLines; i++ {
				if lines[i] != "" {
					fmt.Printf("   %s\n", lines[i])
				}
			}
		}
	}
	
	// Demo 4: Tree - Show project structure
	fmt.Println("\n🌳 Tree Demo - Project structure (limited depth):")
	treeExecutor := tools.NewTreeExecutor()
	params = map[string]interface{}{
		"path":       wd,
		"max_depth":  2,
		"show_sizes": false,
		"color":      false,
		"file_types": []interface{}{"go", "md"},
	}
	
	result, err = treeExecutor.Execute(params)
	if err != nil {
		log.Printf("Tree error: %v", err)
	} else if result.Success {
		data := result.Result.(map[string]interface{})
		if visualTree, ok := data["visual_tree"]; ok {
			tree := visualTree.(string)
			// Show first 20 lines of tree
			lines := strings.Split(tree, "\n")
			maxLines := 20
			if len(lines) < maxLines {
				maxLines = len(lines)
			}
			for i := 0; i < maxLines; i++ {
				fmt.Printf("   %s\n", lines[i])
			}
			if len(lines) > maxLines {
				fmt.Printf("   ... (%d more lines)\n", len(lines)-maxLines)
			}
		}
		
		if summary, ok := result.Metadata["summary"]; ok {
			fmt.Printf("\n   %s\n", summary)
		}
	}
	
	// Demo 5: Fuzzy Find - Intelligent file search
	fmt.Println("\n🎯 Fuzzy Find Demo - Finding 'main' files:")
	fuzzyExecutor := tools.NewFuzzyFinderExecutor()
	params = map[string]interface{}{
		"query":          "main",
		"path":           wd,
		"max_results":    5,
		"score_threshold": 0.3,
		"prefer_recent":  true,
	}
	
	result, err = fuzzyExecutor.Execute(params)
	if err != nil {
		log.Printf("Fuzzy error: %v", err)
	} else if result.Success {
		data := result.Result.(map[string]interface{})
		fmt.Printf("   Found %d fuzzy matches\n", data["total_matches"].(int))
		if summary, ok := result.Metadata["summary"]; ok {
			fmt.Printf("   %s\n", summary)
		}
	}
	
	// Save detailed results as JSON
	if len(os.Args) > 1 && os.Args[1] == "--json" {
		allResults := map[string]interface{}{
			"search_demo":    "Advanced Search & Navigation Tools",
			"workspace_path": wd,
			"demos": map[string]interface{}{
				"file_search":  result.Result,
				"find_in_files": result.Result,
				"grep":         result.Result,
				"tree":         result.Result,  
				"fuzzy_find":   result.Result,
			},
		}
		
		jsonData, _ := json.MarshalIndent(allResults, "", "  ")
		outputFile := "search-demo-results.json"
		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			log.Printf("Failed to write JSON: %v", err)
		} else {
			fmt.Printf("\n💾 Detailed results saved to: %s\n", outputFile)
		}
	}
	
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("✅ Search & Navigation Tools Demo Complete!")
	fmt.Println("   Use --json flag to save detailed results")
}