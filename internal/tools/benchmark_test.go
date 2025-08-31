package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// BenchmarkToolExecution benchmarks tool execution performance
func BenchmarkToolExecution(b *testing.B) {
	router := NewToolRouter()
	router.RegisterExecutor("echo", NewEchoExecutor())
	
	ctx := context.Background()
	call := &ToolCall{
		ID:       "benchmark-001",
		ToolName: "echo",
		Parameters: map[string]interface{}{
			"message": "benchmark test message",
		},
		Timestamp: time.Now(),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := router.Execute(ctx, call)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConcurrentToolExecution benchmarks concurrent tool execution
func BenchmarkConcurrentToolExecution(b *testing.B) {
	router := NewToolRouter()
	router.RegisterExecutor("echo", NewEchoExecutor())
	
	ctx := context.Background()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			call := &ToolCall{
				ID:       fmt.Sprintf("bench-concurrent-%d", time.Now().UnixNano()),
				ToolName: "echo",
				Parameters: map[string]interface{}{
					"message": "concurrent benchmark message",
				},
				Timestamp: time.Now(),
			}
			
			_, err := router.Execute(ctx, call)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkFileOperations benchmarks file read/write operations
func BenchmarkFileOperations(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "gocodenow_benchmark_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	router := NewToolRouter()
	policy := &FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        1024 * 1024,
	}
	
	router.RegisterExecutor("write_file", NewWriteFileExecutor(policy))
	router.RegisterExecutor("read_file", NewReadFileExecutor(policy))
	
	ctx := context.Background()
	testContent := "This is benchmark test content for file operations."
	
	b.Run("WriteFile", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			call := &ToolCall{
				ID:       fmt.Sprintf("write-bench-%d", i),
				ToolName: "write_file",
				Parameters: map[string]interface{}{
					"file_path": filepath.Join(tempDir, fmt.Sprintf("bench_%d.txt", i)),
					"content":   testContent,
				},
				Timestamp: time.Now(),
			}
			
			result, err := router.Execute(ctx, call)
			if err != nil || !result.Success {
				b.Fatal(err)
			}
		}
	})
	
	// Create a test file for read benchmarking
	testFilePath := filepath.Join(tempDir, "read_benchmark.txt")
	writeCall := &ToolCall{
		ID:       "setup-read-bench",
		ToolName: "write_file",
		Parameters: map[string]interface{}{
			"file_path": testFilePath,
			"content":   testContent,
		},
		Timestamp: time.Now(),
	}
	
	_, err = router.Execute(ctx, writeCall)
	if err != nil {
		b.Fatal(err)
	}
	
	b.Run("ReadFile", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			call := &ToolCall{
				ID:       fmt.Sprintf("read-bench-%d", i),
				ToolName: "read_file",
				Parameters: map[string]interface{}{
					"path": testFilePath,
				},
				Timestamp: time.Now(),
			}
			
			result, err := router.Execute(ctx, call)
			if err != nil || !result.Success {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkLargeFileOperations benchmarks operations with larger files
func BenchmarkLargeFileOperations(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "gocodenow_large_benchmark_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	
	router := NewToolRouter()
	policy := &FileSecurityPolicy{
		AllowedPaths:       []string{tempDir},
		AllowAbsolutePaths: true,
		MaxFileSize:        10 * 1024 * 1024, // 10MB
	}
	
	router.RegisterExecutor("write_file", NewWriteFileExecutor(policy))
	router.RegisterExecutor("read_file", NewReadFileExecutor(policy))
	
	ctx := context.Background()
	
	// Generate different sized content for benchmarking
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}
	
	for _, size := range sizes {
		content := make([]byte, size.size)
		for i := range content {
			content[i] = byte('A' + (i % 26))
		}
		
		b.Run(fmt.Sprintf("Write_%s", size.name), func(b *testing.B) {
			b.SetBytes(int64(size.size))
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				call := &ToolCall{
					ID:       fmt.Sprintf("large-write-%d", i),
					ToolName: "write_file",
					Parameters: map[string]interface{}{
						"file_path": filepath.Join(tempDir, fmt.Sprintf("large_%s_%d.txt", size.name, i)),
						"content":   string(content),
					},
					Timestamp: time.Now(),
				}
				
				result, err := router.Execute(ctx, call)
				if err != nil || !result.Success {
					b.Fatal(err)
				}
			}
		})
		
		// Create test file for read benchmark
		testFilePath := filepath.Join(tempDir, fmt.Sprintf("read_large_%s.txt", size.name))
		writeCall := &ToolCall{
			ID:       fmt.Sprintf("setup-large-read-%s", size.name),
			ToolName: "write_file",
			Parameters: map[string]interface{}{
				"file_path": testFilePath,
				"content":   string(content),
			},
			Timestamp: time.Now(),
		}
		
		_, err = router.Execute(ctx, writeCall)
		if err != nil {
			b.Fatal(err)
		}
		
		b.Run(fmt.Sprintf("Read_%s", size.name), func(b *testing.B) {
			b.SetBytes(int64(size.size))
			b.ResetTimer()
			
			for i := 0; i < b.N; i++ {
				call := &ToolCall{
					ID:       fmt.Sprintf("large-read-%d", i),
					ToolName: "read_file",
					Parameters: map[string]interface{}{
						"path": testFilePath,
					},
					Timestamp: time.Now(),
				}
				
				result, err := router.Execute(ctx, call)
				if err != nil || !result.Success {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkToolRouter benchmarks the tool router itself
func BenchmarkToolRouter(b *testing.B) {
	router := NewToolRouter()
	router.RegisterExecutor("echo", NewEchoExecutor())
	router.RegisterExecutor("sleep", NewSleepExecutor())
	
	b.Run("GetSchema", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := router.GetSchema("echo")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("ListTools", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tools := router.ListTools()
			if len(tools) == 0 {
				b.Fatal("No tools found")
			}
		}
	})
}

// BenchmarkErrorHandling benchmarks error handling performance
func BenchmarkErrorHandling(b *testing.B) {
	router := NewToolRouter()
	router.RegisterExecutor("error", NewErrorExecutor())
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		call := &ToolCall{
			ID:       fmt.Sprintf("error-bench-%d", i),
			ToolName: "error",
			Parameters: map[string]interface{}{
				"message": "benchmark error message",
			},
			Timestamp: time.Now(),
		}
		
		result, err := router.Execute(ctx, call)
		// Expect result (not execution error) with failure status
		if err != nil {
			b.Fatal(err)
		}
		if result.Success {
			b.Fatal("Error tool should not succeed")
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	router := NewToolRouter()
	router.RegisterExecutor("echo", NewEchoExecutor())
	
	ctx := context.Background()
	
	b.Run("SmallMessages", func(b *testing.B) {
		message := "small message"
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			call := &ToolCall{
				ID:       fmt.Sprintf("small-%d", i),
				ToolName: "echo",
				Parameters: map[string]interface{}{
					"message": message,
				},
				Timestamp: time.Now(),
			}
			
			_, err := router.Execute(ctx, call)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("LargeMessages", func(b *testing.B) {
		// Create a 10KB message
		largeMessage := make([]byte, 10*1024)
		for i := range largeMessage {
			largeMessage[i] = byte('A' + (i % 26))
		}
		
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			call := &ToolCall{
				ID:       fmt.Sprintf("large-%d", i),
				ToolName: "echo",
				Parameters: map[string]interface{}{
					"message": string(largeMessage),
				},
				Timestamp: time.Now(),
			}
			
			_, err := router.Execute(ctx, call)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}