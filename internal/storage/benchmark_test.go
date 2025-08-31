package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// BenchmarkDatabaseOperations benchmarks basic database operations
func BenchmarkDatabaseOperations(b *testing.B) {
	// Create temporary database for benchmarking
	tmpFile, err := os.CreateTemp("", "gocodenow_benchmark_*.db")
	if err != nil {
		b.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	db, err := NewDatabase(tmpFile.Name())
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	
	// Initialize database schema
	migrationRunner := NewMigrationRunner(db)
	if err := migrationRunner.RunMigrations(); err != nil {
		b.Fatal(err)
	}
	
	b.Run("StoreConversation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			record := &ConversationRecord{
				ID:               fmt.Sprintf("bench-conv-%d", i),
				Timestamp:        time.Now(),
				UserInput:        fmt.Sprintf("User input %d", i),
				LLMResponse:      fmt.Sprintf("LLM response %d", i),
				ModelName:        "benchmark-model",
				TokenUsageInput:  50,
				TokenUsageOutput: 100,
				ExecutionTimeMs:  250,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			
			if err := db.StoreConversation(record); err != nil {
				b.Fatal(err)
			}
		}
	})
	
	// Store some conversations for read benchmarks
	for i := 0; i < 100; i++ {
		record := &ConversationRecord{
			ID:               fmt.Sprintf("read-bench-conv-%d", i),
			Timestamp:        time.Now(),
			UserInput:        fmt.Sprintf("Read benchmark input %d", i),
			LLMResponse:      fmt.Sprintf("Read benchmark response %d", i),
			ModelName:        "benchmark-model",
			TokenUsageInput:  50,
			TokenUsageOutput: 100,
			ExecutionTimeMs:  250,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		
		if err := db.StoreConversation(record); err != nil {
			b.Fatal(err)
		}
	}
	
	b.Run("GetRecentConversations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := db.GetRecentConversations(50)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("GetConversation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convID := fmt.Sprintf("read-bench-conv-%d", i%100)
			_, err := db.GetConversation(convID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkCacheOperations benchmarks cache performance
func BenchmarkCacheOperations(b *testing.B) {
	config := &CacheConfig{
		MaxSize:          1000,
		TTL:              time.Hour,
		CleanupInterval:  time.Minute,
		EnableMetrics:    true,
	}
	
	cache := NewConversationCache(config)
	defer cache.Shutdown()
	
	b.Run("Put", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			record := &ConversationRecord{
				ID:               fmt.Sprintf("cache-bench-%d", i),
				Timestamp:        time.Now(),
				UserInput:        fmt.Sprintf("Cache benchmark input %d", i),
				LLMResponse:      fmt.Sprintf("Cache benchmark response %d", i),
				ModelName:        "benchmark-model",
				TokenUsageInput:  50,
				TokenUsageOutput: 100,
				ExecutionTimeMs:  250,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			
			cache.Put(record.ID, record)
		}
	})
	
	// Add some entries for read benchmarks
	for i := 0; i < 1000; i++ {
		record := &ConversationRecord{
			ID:               fmt.Sprintf("cache-read-bench-%d", i),
			Timestamp:        time.Now(),
			UserInput:        fmt.Sprintf("Cache read input %d", i),
			LLMResponse:      fmt.Sprintf("Cache read response %d", i),
			ModelName:        "benchmark-model",
			TokenUsageInput:  50,
			TokenUsageOutput: 100,
			ExecutionTimeMs:  250,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		cache.Put(record.ID, record)
	}
	
	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("cache-read-bench-%d", i%1000)
			_, found := cache.Get(key)
			if !found {
				b.Fatal("Cache entry should be found")
			}
		}
	})
	
	b.Run("GetStats", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = cache.GetStats()
		}
	})
}

// BenchmarkStorageManager benchmarks the complete storage manager
func BenchmarkStorageManager(b *testing.B) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "gocodenow_storage_benchmark_*.db")
	if err != nil {
		b.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	db, err := NewDatabase(tmpFile.Name())
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	
	// Initialize database schema
	migrationRunner := NewMigrationRunner(db)
	if err := migrationRunner.RunMigrations(); err != nil {
		b.Fatal(err)
	}
	
	config := StorageConfig{
		CacheSize:    1000,
		MaxMemoryMB:  100,
		EnableCache:  true,
		CacheTTL:     time.Hour,
	}
	
	manager, err := NewStorageManager(db, config)
	if err != nil {
		b.Fatal(err)
	}
	defer manager.Shutdown()
	
	ctx := context.Background()
	
	b.Run("StoreConversation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			record := &ConversationRecord{
				ID:               fmt.Sprintf("mgr-bench-%d", i),
				Timestamp:        time.Now(),
				UserInput:        fmt.Sprintf("Manager benchmark input %d", i),
				LLMResponse:      fmt.Sprintf("Manager benchmark response %d", i),
				ModelName:        "benchmark-model",
				TokenUsageInput:  50,
				TokenUsageOutput: 100,
				ExecutionTimeMs:  250,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			
			if err := manager.StoreConversation(ctx, record); err != nil {
				b.Fatal(err)
			}
		}
	})
	
	// Store conversations for read benchmarks
	for i := 0; i < 500; i++ {
		record := &ConversationRecord{
			ID:               fmt.Sprintf("mgr-read-bench-%d", i),
			Timestamp:        time.Now(),
			UserInput:        fmt.Sprintf("Manager read input %d", i),
			LLMResponse:      fmt.Sprintf("Manager read response %d", i),
			ModelName:        "benchmark-model",
			TokenUsageInput:  50,
			TokenUsageOutput: 100,
			ExecutionTimeMs:  250,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		
		if err := manager.StoreConversation(ctx, record); err != nil {
			b.Fatal(err)
		}
	}
	
	b.Run("GetRecentConversations", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := manager.GetRecentConversations(ctx, 50)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("GetConversation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convID := fmt.Sprintf("mgr-read-bench-%d", i%500)
			_, err := manager.GetConversation(ctx, convID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkLazyLoading benchmarks lazy loading performance
func BenchmarkLazyLoading(b *testing.B) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "gocodenow_lazy_benchmark_*.db")
	if err != nil {
		b.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	db, err := NewDatabase(tmpFile.Name())
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	
	// Initialize database schema
	migrationRunner := NewMigrationRunner(db)
	if err := migrationRunner.RunMigrations(); err != nil {
		b.Fatal(err)
	}
	
	config := &CacheConfig{
		MaxSize:          100,
		TTL:              time.Hour,
		CleanupInterval:  time.Minute,
		EnableMetrics:    true,
	}
	
	cache := NewConversationCache(config)
	defer cache.Shutdown()
	
	loader := NewLazyConversationLoader(db, cache)
	
	// Store many conversations in database
	for i := 0; i < 1000; i++ {
		record := &ConversationRecord{
			ID:               fmt.Sprintf("lazy-bench-%d", i),
			Timestamp:        time.Now(),
			UserInput:        fmt.Sprintf("Lazy loading input %d", i),
			LLMResponse:      fmt.Sprintf("Lazy loading response %d", i),
			ModelName:        "benchmark-model",
			TokenUsageInput:  50,
			TokenUsageOutput: 100,
			ExecutionTimeMs:  250,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		
		if err := db.StoreConversation(record); err != nil {
			b.Fatal(err)
		}
	}
	
	b.Run("LoadConversation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convID := fmt.Sprintf("lazy-bench-%d", i%1000)
			_, err := loader.LoadConversation(convID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("LoadBatch", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var ids []string
			start := i * 10 % 1000
			for j := 0; j < 10; j++ {
				ids = append(ids, fmt.Sprintf("lazy-bench-%d", (start+j)%1000))
			}
			
			_, err := loader.LoadBatch(ids)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkConcurrentAccess benchmarks concurrent database access
func BenchmarkConcurrentAccess(b *testing.B) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "gocodenow_concurrent_benchmark_*.db")
	if err != nil {
		b.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	db, err := NewDatabase(tmpFile.Name())
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	
	// Initialize database schema
	migrationRunner := NewMigrationRunner(db)
	if err := migrationRunner.RunMigrations(); err != nil {
		b.Fatal(err)
	}
	
	config := StorageConfig{
		CacheSize:    1000,
		MaxMemoryMB:  100,
		EnableCache:  true,
		CacheTTL:     time.Hour,
	}
	
	manager, err := NewStorageManager(db, config)
	if err != nil {
		b.Fatal(err)
	}
	defer manager.Shutdown()
	
	ctx := context.Background()
	
	b.Run("ConcurrentStore", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				record := &ConversationRecord{
					ID:               fmt.Sprintf("concurrent-store-%d-%d", time.Now().UnixNano(), i),
					Timestamp:        time.Now(),
					UserInput:        fmt.Sprintf("Concurrent store input %d", i),
					LLMResponse:      fmt.Sprintf("Concurrent store response %d", i),
					ModelName:        "benchmark-model",
					TokenUsageInput:  50,
					TokenUsageOutput: 100,
					ExecutionTimeMs:  250,
					CreatedAt:        time.Now(),
					UpdatedAt:        time.Now(),
				}
				
				if err := manager.StoreConversation(ctx, record); err != nil {
					b.Fatal(err)
				}
				i++
			}
		})
	})
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	config := &CacheConfig{
		MaxSize:          10000,
		TTL:              time.Hour,
		CleanupInterval:  time.Minute,
		EnableMetrics:    true,
	}
	
	cache := NewConversationCache(config)
	defer cache.Shutdown()
	
	b.Run("MemoryGrowth", func(b *testing.B) {
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			// Create conversations of varying sizes
			var response string
			switch i % 4 {
			case 0:
				response = "Small response"
			case 1:
				response = fmt.Sprintf("Medium response %s", generateString(1000))
			case 2:
				response = fmt.Sprintf("Large response %s", generateString(10000))
			case 3:
				response = fmt.Sprintf("Very large response %s", generateString(100000))
			}
			
			record := &ConversationRecord{
				ID:               fmt.Sprintf("memory-bench-%d", i),
				Timestamp:        time.Now(),
				UserInput:        fmt.Sprintf("Memory benchmark input %d", i),
				LLMResponse:      response,
				ModelName:        "benchmark-model",
				TokenUsageInput:  50,
				TokenUsageOutput: len(response),
				ExecutionTimeMs:  250,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}
			
			cache.Put(record.ID, record)
		}
	})
}

// Helper function to generate strings of specific length
func generateString(length int) string {
	result := make([]byte, length)
	for i := range result {
		result[i] = byte('A' + (i % 26))
	}
	return string(result)
}