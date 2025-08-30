package storage

import (
	"fmt"
	"gocodenow/internal/storage/testutil"
	"gocodenow/internal/types"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStorageManager_Initialization(t *testing.T) {
	tests := []struct {
		name          string
		dbPath        string
		cacheCapacity int
		autoSaveFreq  time.Duration
		wantErr       bool
		expectErr     string
	}{
		{
			name:          "valid initialization",
			dbPath:        filepath.Join(t.TempDir(), "test.db"),
			cacheCapacity: 50,
			autoSaveFreq:  5 * time.Minute,
			wantErr:       false,
		},
		{
			name:          "zero cache capacity",
			dbPath:        filepath.Join(t.TempDir(), "test.db"),
			cacheCapacity: 0,
			autoSaveFreq:  5 * time.Minute,
			wantErr:       true,
			expectErr:     "cache size must be greater than 0",
		},
		{
			name:          "negative cache capacity",
			dbPath:        filepath.Join(t.TempDir(), "test.db"),
			cacheCapacity: -1,
			autoSaveFreq:  5 * time.Minute,
			wantErr:       true,
			expectErr:     "cache size must be greater than 0",
		},
		{
			name:          "zero auto-save frequency",
			dbPath:        filepath.Join(t.TempDir(), "test.db"),
			cacheCapacity: 50,
			autoSaveFreq:  0,
			wantErr:       true,
			expectErr:     "auto-save frequency must be greater than 0",
		},
		{
			name:          "invalid database path",
			dbPath:        "/invalid/nonexistent/path/test.db",
			cacheCapacity: 50,
			autoSaveFreq:  5 * time.Minute,
			wantErr:       true,
			expectErr:     "failed to create database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create database
		db, dbErr := NewDatabase(tt.dbPath)
		if dbErr != nil && !tt.wantErr {
			t.Errorf("Unexpected database creation error: %v", dbErr)
			return
		}
		if dbErr != nil && tt.wantErr && containsError(dbErr.Error(), tt.expectErr) {
			return // Expected error occurred at database creation
		}
		if dbErr != nil {
			return // Some other error
		}
		
		config := StorageConfig{
			CacheSize:    tt.cacheCapacity,
			AutoSaveFreq: tt.autoSaveFreq,
		}
		manager, err := NewStorageManager(db, config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewStorageManager() expected error but got none")
					return
				}
				if tt.expectErr != "" && !containsError(err.Error(), tt.expectErr) {
					t.Errorf("NewStorageManager() error = %v, expected to contain %v", err, tt.expectErr)
				}
				return
			}

			if err != nil {
				t.Errorf("NewStorageManager() unexpected error = %v", err)
				return
			}

			if manager == nil {
				t.Error("NewStorageManager() returned nil manager")
				return
			}

			// Verify initialization
			if manager.db == nil {
				t.Error("StorageManager database is nil")
			}
			if manager.cache == nil {
				t.Error("StorageManager cache is nil")
			}
			if manager.cache.maxSize != tt.cacheCapacity {
				t.Errorf("StorageManager cache capacity = %d, expected %d", manager.cache.maxSize, tt.cacheCapacity)
			}
			if manager.autoSaveFreq != tt.autoSaveFreq {
				t.Errorf("StorageManager auto-save frequency = %v, expected %v", manager.autoSaveFreq, tt.autoSaveFreq)
			}

			// Verify context and channels are set up
			if manager.ctx == nil {
				t.Error("StorageManager context is nil")
			}
			if manager.cancel == nil {
				t.Error("StorageManager cancel function is nil")
			}
			if manager.autoSaveChan == nil {
				t.Error("StorageManager auto-save channel is nil")
			}
			if manager.stopChan == nil {
				t.Error("StorageManager stop channel is nil")
			}

			// Clean up
			manager.Close()
		})
	}
}

func TestStorageManager_SaveAndLoadConversation(t *testing.T) {
	manager := setupTestStorageManager(t)
	defer manager.Close()

	// Create test conversation
	convModel := testutil.GenerateConversation("test-conversation-1")
	conv := conversationToRecord(convModel)

	// Test Save
	err := manager.SaveConversation(conv)
	if err != nil {
		t.Fatalf("SaveConversation() error = %v", err)
	}

	// Verify in cache
	cachedConv, found := manager.cache.Get(conv.ID)
	if !found {
		t.Error("Conversation not found in cache after save")
	}
	if cachedConv.ID != conv.ID {
		t.Errorf("Cached conversation ID = %s, expected %s", cachedConv.ID, conv.ID)
	}

	// Test Load
	loadedConv, err := manager.LoadConversation(conv.ID)
	if err != nil {
		t.Fatalf("LoadConversation() error = %v", err)
	}

	if loadedConv.ID != conv.ID {
		t.Errorf("LoadConversation() ID = %s, expected %s", loadedConv.ID, conv.ID)
	}
	if loadedConv.UserInput != conv.UserInput {
		t.Errorf("LoadConversation() UserInput = %s, expected %s", loadedConv.UserInput, conv.UserInput)
	}
	if loadedConv.LLMResponse != conv.LLMResponse {
		t.Errorf("LoadConversation() LLMResponse = %s, expected %s", loadedConv.LLMResponse, conv.LLMResponse)
	}
}

func TestStorageManager_DeleteConversation(t *testing.T) {
	manager := setupTestStorageManager(t)
	defer manager.Close()

	// Create and save test conversation
	convModel := testutil.GenerateConversation("test-conversation-delete")
	conv := conversationToRecord(convModel)
	err := manager.SaveConversation(conv)
	if err != nil {
		t.Fatalf("SaveConversation() error = %v", err)
	}

	// Verify it exists
	_, err = manager.LoadConversation(conv.ID)
	if err != nil {
		t.Fatalf("Conversation should exist before delete, error = %v", err)
	}

	// Delete conversation
	err = manager.DeleteConversation(conv.ID)
	if err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	// Verify it's removed from cache
	_, found := manager.cache.Get(conv.ID)
	if found {
		t.Error("Conversation still found in cache after delete")
	}

	// Verify it's removed from database
	_, err = manager.LoadConversation(conv.ID)
	if err == nil {
		t.Error("LoadConversation() should return error for deleted conversation")
	}
}

func TestStorageManager_ListConversations(t *testing.T) {
	manager := setupTestStorageManager(t)
	defer manager.Close()

	// Create multiple test conversations
	conversations := make([]*ConversationRecord, 5)
	for i := 0; i < 5; i++ {
		convModel := testutil.GenerateConversation(fmt.Sprintf("test-conversation-%d", i))
		conv := conversationToRecord(convModel)
		conversations[i] = conv
		err := manager.SaveConversation(conv)
		if err != nil {
			t.Fatalf("SaveConversation() error = %v", err)
		}
	}

	// Test list all
	listed, err := manager.ListConversations(0, 10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}

	if len(listed) != 5 {
		t.Errorf("ListConversations() returned %d conversations, expected 5", len(listed))
	}

	// Test pagination
	listed, err = manager.ListConversations(0, 3)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("ListConversations() with limit 3 returned %d conversations, expected 3", len(listed))
	}

	// Test offset
	listed, err = manager.ListConversations(2, 10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("ListConversations() with offset 2 returned %d conversations, expected 3", len(listed))
	}
}

func TestStorageManager_AutoSave(t *testing.T) {
	// Use shorter auto-save frequency for testing
	manager := setupTestStorageManagerWithFreq(t, 100*time.Millisecond)
	defer manager.Close()

	// Create conversations but don't save them manually
	conversations := make([]*ConversationRecord, 12)
	for i := 0; i < 12; i++ {
		convModel := testutil.GenerateConversation(fmt.Sprintf("auto-save-test-%d", i))
	conv := conversationToRecord(convModel)
		conversations[i] = conv
		// Add to cache without saving to database
		manager.cache.Put(conv.ID, conv)
	}

	// Wait for auto-save to trigger (should trigger after 10 conversations)
	time.Sleep(200 * time.Millisecond)

	// Check that conversations were auto-saved to database
	for i := 0; i < 10; i++ { // First 10 should be saved
		_, err := manager.LoadConversation(conversations[i].ID)
		if err != nil {
			t.Errorf("Conversation %s not found in database after auto-save, error = %v", conversations[i].ID, err)
		}
	}

	// Check metrics
	manager.mu.RLock()
	autoSaves := manager.metrics.AutoSaves
	manager.mu.RUnlock()

	if autoSaves == 0 {
		t.Error("Auto-save metrics not updated")
	}
}

func TestStorageManager_ConcurrentAccess(t *testing.T) {
	manager := setupTestStorageManager(t)
	defer manager.Close()

	numGoroutines := 50
	numOpsPerGoroutine := 20
	var wg sync.WaitGroup

	// Concurrent save operations
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				convModel := testutil.GenerateConversation(fmt.Sprintf("concurrent-%d-%d", goroutineID, j))
	conv := conversationToRecord(convModel)
				err := manager.SaveConversation(conv)
				if err != nil {
					t.Errorf("Concurrent SaveConversation() error = %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all conversations were saved
	conversations, err := manager.ListConversations(0, numGoroutines*numOpsPerGoroutine+10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}

	expected := numGoroutines * numOpsPerGoroutine
	if len(conversations) != expected {
		t.Errorf("Expected %d conversations, got %d", expected, len(conversations))
	}
}

func TestStorageManager_SessionRecovery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session-recovery.db")

	// Create first manager and save conversations
	manager1 := setupTestStorageManagerWithPath(t, dbPath)
	
	conversations := make([]*ConversationRecord, 25)
	for i := 0; i < 25; i++ {
		convModel := testutil.GenerateConversation(fmt.Sprintf("session-test-%d", i))
	conv := conversationToRecord(convModel)
		conversations[i] = conv
		err := manager1.SaveConversation(conv)
		if err != nil {
			t.Fatalf("SaveConversation() error = %v", err)
		}
	}
	manager1.Close()

	// Create second manager with same database
	manager2 := setupTestStorageManagerWithPath(t, dbPath)
	defer manager2.Close()

	// Test session recovery (should load last 20 conversations)
	recovered, err := manager2.LoadRecentConversations(20)
	if err != nil {
		t.Fatalf("LoadRecentConversations() error = %v", err)
	}

	if len(recovered) != 20 {
		t.Errorf("Session recovery loaded %d conversations, expected 20", len(recovered))
	}

	// Verify conversations are in cache
	for _, conv := range recovered {
		_, found := manager2.cache.Get(conv.ID)
		if !found {
			t.Errorf("Recovered conversation %s not found in cache", conv.ID)
		}
	}
}

func TestStorageManager_Metrics(t *testing.T) {
	manager := setupTestStorageManager(t)
	defer manager.Close()

	// Perform various operations
	convModel1 := testutil.GenerateConversation("metrics-test-1")
	conv1 := conversationToRecord(convModel1)
	convModel2 := testutil.GenerateConversation("metrics-test-2")
	conv2 := conversationToRecord(convModel2)

	err := manager.SaveConversation(conv1)
	if err != nil {
		t.Fatalf("SaveConversation() error = %v", err)
	}

	err = manager.SaveConversation(conv2)
	if err != nil {
		t.Fatalf("SaveConversation() error = %v", err)
	}

	_, err = manager.LoadConversation(conv1.ID)
	if err != nil {
		t.Fatalf("LoadConversation() error = %v", err)
	}

	err = manager.DeleteConversation(conv2.ID)
	if err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	// Check metrics
	metrics := manager.GetMetrics()

	if metrics.Saves < 2 {
		t.Errorf("Expected at least 2 saves, got %d", metrics.Saves)
	}
	if metrics.Loads < 1 {
		t.Errorf("Expected at least 1 load, got %d", metrics.Loads)
	}
	if metrics.Deletes < 1 {
		t.Errorf("Expected at least 1 delete, got %d", metrics.Deletes)
	}
	if metrics.CacheHits == 0 && metrics.CacheMisses == 0 {
		t.Error("Expected some cache activity")
	}
}

func TestStorageManager_Close(t *testing.T) {
	manager := setupTestStorageManager(t)

	// Add some conversations to cache
	convModel := testutil.GenerateConversation("close-test")
	conv := conversationToRecord(convModel)
	manager.cache.Put(conv.ID, conv)

	// Close manager
	err := manager.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify context is cancelled
	select {
	case <-manager.ctx.Done():
		// Context should be cancelled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context not cancelled after Close()")
	}

	// Verify operations fail after close
	err = manager.SaveConversation(conv)
	if err == nil {
		t.Error("SaveConversation() should fail after Close()")
	}
}

// Helper functions

func setupTestStorageManager(t *testing.T) *StorageManager {
	return setupTestStorageManagerWithPath(t, filepath.Join(t.TempDir(), "test.db"))
}

func setupTestStorageManagerWithPath(t *testing.T, dbPath string) *StorageManager {
	return setupTestStorageManagerWithFreq(t, 5*time.Minute, dbPath)
}

func setupTestStorageManagerWithFreq(t *testing.T, freq time.Duration, paths ...string) *StorageManager {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if len(paths) > 0 {
		dbPath = paths[0]
	}

	// Create database
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Run migrations
	runner := NewMigrationRunner(db)
	if err := runner.RunMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create storage manager
	config := StorageConfig{
		CacheSize:    50,
		AutoSaveFreq: freq,
	}
	
	manager, err := NewStorageManager(db, config)
	if err != nil {
		t.Fatalf("Failed to create test StorageManager: %v", err)
	}
	return manager
}

func containsError(got, want string) bool {
	return len(got) >= len(want) && 
		   (got == want || 
		    len(got) > len(want) && 
		    (got[:len(want)] == want || got[len(got)-len(want):] == want ||
		     containsSubstring(got, want)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Helper function to convert types.ConversationBlock to ConversationRecord
func conversationToRecord(conv *types.ConversationBlock) *ConversationRecord {
	return &ConversationRecord{
		ID:               conv.ID,
		Timestamp:        conv.Timestamp,
		UserInput:        conv.UserInput,
		LLMResponse:      conv.LLMResponse,
		ModelName:        conv.ModelName,
		TokenUsageInput:  conv.TokenUsage.InputTokens,
		TokenUsageOutput: conv.TokenUsage.OutputTokens,
		ExecutionTimeMs:  int(conv.ExecutionTime.Milliseconds()),
		Status:           conv.Status,
		ToolExecutions:   []ToolExecutionRecord{},
		CreatedAt:        conv.Timestamp,
		UpdatedAt:        conv.Timestamp,
	}
}

// Benchmarks

func BenchmarkStorageManager_SaveConversation(b *testing.B) {
	manager := setupBenchStorageManager(b)
	defer manager.Close()

	conversations := make([]*ConversationRecord, b.N)
	for i := 0; i < b.N; i++ {
		convModel := testutil.GenerateConversation(fmt.Sprintf("bench-save-%d", i))
		conversations[i] = conversationToRecord(convModel)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := manager.SaveConversation(conversations[i])
		if err != nil {
			b.Fatalf("SaveConversation() error = %v", err)
		}
	}
}

func BenchmarkStorageManager_LoadConversation(b *testing.B) {
	manager := setupBenchStorageManager(b)
	defer manager.Close()

	// Pre-populate with conversations
	conversations := make([]*ConversationRecord, b.N)
	for i := 0; i < b.N; i++ {
		convModel := testutil.GenerateConversation(fmt.Sprintf("bench-load-%d", i))
	conv := conversationToRecord(convModel)
		conversations[i] = conv
		err := manager.SaveConversation(conv)
		if err != nil {
			b.Fatalf("SaveConversation() error = %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.LoadConversation(conversations[i].ID)
		if err != nil {
			b.Fatalf("LoadConversation() error = %v", err)
		}
	}
}

func BenchmarkStorageManager_ConcurrentOperations(b *testing.B) {
	manager := setupBenchStorageManager(b)
	defer manager.Close()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			convModel := testutil.GenerateConversation(fmt.Sprintf("bench-concurrent-%d", i))
	conv := conversationToRecord(convModel)
			err := manager.SaveConversation(conv)
			if err != nil {
				b.Fatalf("SaveConversation() error = %v", err)
			}

			_, err = manager.LoadConversation(conv.ID)
			if err != nil {
				b.Fatalf("LoadConversation() error = %v", err)
			}
			i++
		}
	})
}

func setupBenchStorageManager(b *testing.B) *StorageManager {
	tmpDir, err := os.MkdirTemp("", "bench-storage-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "bench.db")
	
	// Create database
	db, err := NewDatabase(dbPath)
	if err != nil {
		b.Fatalf("Failed to create bench database: %v", err)
	}

	// Run migrations
	runner := NewMigrationRunner(db)
	if err := runner.RunMigrations(); err != nil {
		b.Fatalf("Failed to run bench migrations: %v", err)
	}

	// Create storage manager
	config := StorageConfig{
		CacheSize:    1000, // Larger cache for benchmarks
		AutoSaveFreq: 10 * time.Minute,
	}
	
	manager, err := NewStorageManager(db, config)
	if err != nil {
		b.Fatalf("Failed to create bench StorageManager: %v", err)
	}

	b.Cleanup(func() {
		manager.Close()
		os.RemoveAll(tmpDir)
	})

	return manager
}