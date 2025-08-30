package storage

import (
	"fmt"
	"gocodenow/internal/types"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// Helper function to create test ConversationRecord for LRU cache testing
func generateTestConversationRecord(id string) *ConversationRecord {
	userInputs := []string{
		"Help me implement a function to parse JSON data",
		"Fix the bug in my authentication middleware", 
		"Create a REST API for user management",
		"Optimize this database query for better performance",
		"Write unit tests for the payment processing module",
	}
	
	llmResponses := []string{
		"I'll help you implement the JSON parsing function. Here's a solution that handles errors gracefully...",
		"I found the issue in your authentication middleware. The problem is with token validation...",
		"I'll create a comprehensive REST API for user management with proper validation...",
		"Let me analyze your query and suggest optimizations...",
		"I'll write comprehensive unit tests for your payment processing module...",
	}
	
	modelNames := []string{"gpt-4", "gpt-3.5-turbo", "claude-3-sonnet", "llama-3.1-70b"}
	
	userInput := userInputs[rand.Intn(len(userInputs))]
	llmResponse := llmResponses[rand.Intn(len(llmResponses))]
	timestamp := time.Now().Add(-time.Duration(rand.Intn(72)) * time.Hour)
	
	return &ConversationRecord{
		ID:               id,
		Timestamp:        timestamp,
		UserInput:        userInput,
		LLMResponse:      llmResponse,
		ModelName:        modelNames[rand.Intn(len(modelNames))],
		TokenUsageInput:  rand.Intn(1000) + 50,
		TokenUsageOutput: rand.Intn(2000) + 100,
		ExecutionTimeMs:  rand.Intn(5000) + 500,
		Status:           types.StatusCompleted,
		ToolExecutions:   []ToolExecutionRecord{},
		CreatedAt:        timestamp,
		UpdatedAt:        timestamp,
	}
}

func TestLRUCache_BasicOperations(t *testing.T) {
	cache := NewLRUCache(3)

	// Test Put and Get
	conv1 := generateTestConversationRecord("lru-test-1")
	cache.Put(conv1.ID, conv1)

	retrieved, found := cache.Get(conv1.ID)
	if !found {
		t.Error("Expected to find conversation in cache")
	}
	if retrieved.ID != conv1.ID {
		t.Errorf("Retrieved conversation ID = %s, expected %s", retrieved.ID, conv1.ID)
	}

	// Test non-existent key
	_, found = cache.Get("non-existent")
	if found {
		t.Error("Expected not to find non-existent key")
	}
}

func TestLRUCache_CapacityLimit(t *testing.T) {
	capacity := 3
	cache := NewLRUCache(capacity)

	// Add conversations up to capacity
	conversations := make([]*ConversationRecord, 5)
	for i := 0; i < 5; i++ {
		conv := generateTestConversationRecord(fmt.Sprintf("capacity-test-%d", i))
		conversations[i] = conv
		cache.Put(conv.ID, conv)
	}

	// Cache should only contain last 3 conversations
	if cache.Size() != capacity {
		t.Errorf("Cache size = %d, expected %d", cache.Size(), capacity)
	}

	// First two conversations should be evicted
	_, found := cache.Get(conversations[0].ID)
	if found {
		t.Error("Expected first conversation to be evicted")
	}
	_, found = cache.Get(conversations[1].ID)
	if found {
		t.Error("Expected second conversation to be evicted")
	}

	// Last three conversations should still be in cache
	for i := 2; i < 5; i++ {
		_, found := cache.Get(conversations[i].ID)
		if !found {
			t.Errorf("Expected conversation %d to be in cache", i)
		}
	}
}

func TestLRUCache_LeastRecentlyUsedEviction(t *testing.T) {
	cache := NewLRUCache(3)

	// Add 3 conversations
	conv1 := generateTestConversationRecord("lru-eviction-1")
	conv2 := generateTestConversationRecord("lru-eviction-2")
	conv3 := generateTestConversationRecord("lru-eviction-3")

	cache.Put(conv1.ID, conv1)
	cache.Put(conv2.ID, conv2)
	cache.Put(conv3.ID, conv3)

	// Access conv1 to make it recently used
	cache.Get(conv1.ID)

	// Add a new conversation - conv2 should be evicted (least recently used)
	conv4 := generateTestConversationRecord("lru-eviction-4")
	cache.Put(conv4.ID, conv4)

	// conv2 should be evicted
	_, found := cache.Get(conv2.ID)
	if found {
		t.Error("Expected conv2 to be evicted as least recently used")
	}

	// conv1, conv3, conv4 should still be in cache
	_, found = cache.Get(conv1.ID)
	if !found {
		t.Error("Expected conv1 to still be in cache")
	}
	_, found = cache.Get(conv3.ID)
	if !found {
		t.Error("Expected conv3 to still be in cache")
	}
	_, found = cache.Get(conv4.ID)
	if !found {
		t.Error("Expected conv4 to be in cache")
	}
}

func TestLRUCache_UpdateExistingKey(t *testing.T) {
	cache := NewLRUCache(3)

	// Add conversation
	conv1 := generateTestConversationRecord("update-test-1")
	cache.Put(conv1.ID, conv1)

	// Update same conversation
	updatedConv := generateTestConversationRecord("update-test-1")
	updatedConv.LLMResponse = "Updated response"
	cache.Put(conv1.ID, updatedConv)

	// Cache size should remain 1
	if cache.Size() != 1 {
		t.Errorf("Cache size = %d, expected 1 after update", cache.Size())
	}

	// Retrieved conversation should have updated content
	retrieved, found := cache.Get(conv1.ID)
	if !found {
		t.Error("Expected to find updated conversation")
	}
	if retrieved.LLMResponse != "Updated response" {
		t.Errorf("Retrieved LLMResponse = %s, expected 'Updated response'", retrieved.LLMResponse)
	}
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(3)

	// Add conversations
	conv1 := generateTestConversationRecord("delete-test-1")
	conv2 := generateTestConversationRecord("delete-test-2")
	cache.Put(conv1.ID, conv1)
	cache.Put(conv2.ID, conv2)

	// Delete conv1
	cache.Delete(conv1.ID)

	// conv1 should be gone
	_, found := cache.Get(conv1.ID)
	if found {
		t.Error("Expected conv1 to be deleted")
	}

	// conv2 should still be there
	_, found = cache.Get(conv2.ID)
	if !found {
		t.Error("Expected conv2 to still be in cache")
	}

	// Cache size should be 1
	if cache.Size() != 1 {
		t.Errorf("Cache size = %d, expected 1 after delete", cache.Size())
	}

	// Delete non-existent key should not cause error
	cache.Delete("non-existent")
	if cache.Size() != 1 {
		t.Errorf("Cache size changed after deleting non-existent key")
	}
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(3)

	// Add conversations
	for i := 0; i < 3; i++ {
		conv := generateTestConversationRecord(fmt.Sprintf("clear-test-%d", i))
		cache.Put(conv.ID, conv)
	}

	// Clear cache
	cache.Clear()

	// Cache should be empty
	if cache.Size() != 0 {
		t.Errorf("Cache size = %d, expected 0 after clear", cache.Size())
	}

	// No conversations should be retrievable
	_, found := cache.Get("clear-test-0")
	if found {
		t.Error("Expected no conversations after clear")
	}
}

func TestLRUCache_GetAll(t *testing.T) {
	cache := NewLRUCache(5)

	// Add conversations
	conversations := make([]*ConversationRecord, 3)
	for i := 0; i < 3; i++ {
		conv := generateTestConversationRecord(fmt.Sprintf("getall-test-%d", i))
		conversations[i] = conv
		cache.Put(conv.ID, conv)
	}

	// Get all conversations
	allConversations := cache.GetAll()

	if len(allConversations) != 3 {
		t.Errorf("GetAll() returned %d conversations, expected 3", len(allConversations))
	}

	// Verify all conversations are present
	for _, originalConv := range conversations {
		found := false
		for _, retrievedConv := range allConversations {
			if retrievedConv.ID == originalConv.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Conversation %s not found in GetAll() result", originalConv.ID)
		}
	}
}

func TestLRUCache_MemoryLimit(t *testing.T) {
	cache := NewLRUCache(1000) // Large capacity

	// Add conversations with large content to test memory usage
	largeContent := make([]byte, 10*1024) // 10KB
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}

	conversations := make([]*ConversationRecord, 100)
	for i := 0; i < 100; i++ {
		conv := generateTestConversationRecord(fmt.Sprintf("memory-test-%d", i))
		conv.LLMResponse = string(largeContent) // Add large content
		conversations[i] = conv
		cache.Put(conv.ID, conv)
	}

	// Verify all conversations are in cache (within capacity limit)
	if cache.Size() != 100 {
		t.Errorf("Cache size = %d, expected 100", cache.Size())
	}

	// Check memory estimation (approximate)
	totalMemory := cache.EstimateMemoryUsage()
	expectedMinMemory := 100 * 10 * 1024 // At least 100 * 10KB
	if totalMemory < int64(expectedMinMemory) {
		t.Errorf("Estimated memory usage %d bytes is too low, expected at least %d bytes", totalMemory, expectedMinMemory)
	}
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	cache := NewLRUCache(100)
	numGoroutines := 50
	numOpsPerGoroutine := 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent put and get operations
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOpsPerGoroutine; j++ {
				conv := generateTestConversationRecord(fmt.Sprintf("concurrent-%d-%d", goroutineID, j))
				
				// Put conversation
				cache.Put(conv.ID, conv)
				
				// Get conversation
				retrieved, found := cache.Get(conv.ID)
				if !found {
					t.Errorf("Failed to retrieve conversation %s immediately after put", conv.ID)
					continue
				}
				
				if retrieved.ID != conv.ID {
					t.Errorf("Retrieved conversation ID mismatch: got %s, expected %s", retrieved.ID, conv.ID)
				}
			}
		}(i)
	}

	wg.Wait()

	// Cache should not exceed capacity
	if cache.Size() > 100 {
		t.Errorf("Cache size %d exceeds capacity 100", cache.Size())
	}

	// Cache should contain some conversations (may be less than total due to eviction)
	if cache.Size() == 0 {
		t.Error("Cache is empty after concurrent operations")
	}
}

func TestLRUCache_EvictionOrder(t *testing.T) {
	cache := NewLRUCache(3)

	// Add conversations in order
	conv1 := generateTestConversationRecord("eviction-order-1")
	conv2 := generateTestConversationRecord("eviction-order-2")
	conv3 := generateTestConversationRecord("eviction-order-3")
	conv4 := generateTestConversationRecord("eviction-order-4")

	cache.Put(conv1.ID, conv1)
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	cache.Put(conv2.ID, conv2)
	time.Sleep(1 * time.Millisecond)
	cache.Put(conv3.ID, conv3)
	time.Sleep(1 * time.Millisecond)

	// Access conv1 and conv2 to make them more recently used
	cache.Get(conv1.ID)
	cache.Get(conv2.ID)

	// Add conv4 - conv3 should be evicted as least recently used
	cache.Put(conv4.ID, conv4)

	// conv3 should be evicted
	_, found := cache.Get(conv3.ID)
	if found {
		t.Error("Expected conv3 to be evicted")
	}

	// conv1, conv2, conv4 should still be in cache
	expectedIDs := []string{conv1.ID, conv2.ID, conv4.ID}
	for _, id := range expectedIDs {
		_, found := cache.Get(id)
		if !found {
			t.Errorf("Expected conversation %s to be in cache", id)
		}
	}
}

func TestLRUCache_AccessPatterns(t *testing.T) {
	cache := NewLRUCache(3)

	// Add 3 conversations
	conv1 := generateTestConversationRecord("access-pattern-1")
	conv2 := generateTestConversationRecord("access-pattern-2")
	conv3 := generateTestConversationRecord("access-pattern-3")

	cache.Put(conv1.ID, conv1)
	cache.Put(conv2.ID, conv2)
	cache.Put(conv3.ID, conv3)

	// Pattern: frequently access conv1
	for i := 0; i < 5; i++ {
		cache.Get(conv1.ID)
		time.Sleep(1 * time.Millisecond)
	}

	// Add new conversations - conv1 should be kept longer due to frequent access
	conv4 := generateTestConversationRecord("access-pattern-4")
	conv5 := generateTestConversationRecord("access-pattern-5")

	cache.Put(conv4.ID, conv4) // Should evict conv2 (less recently used)
	cache.Put(conv5.ID, conv5) // Should evict conv3 (less recently used)

	// conv1 should still be in cache due to frequent access
	_, found := cache.Get(conv1.ID)
	if !found {
		t.Error("Expected frequently accessed conv1 to remain in cache")
	}

	// conv2 and conv3 should be evicted
	_, found = cache.Get(conv2.ID)
	if found {
		t.Error("Expected conv2 to be evicted")
	}
	_, found = cache.Get(conv3.ID)
	if found {
		t.Error("Expected conv3 to be evicted")
	}

	// conv4 and conv5 should be in cache
	_, found = cache.Get(conv4.ID)
	if !found {
		t.Error("Expected conv4 to be in cache")
	}
	_, found = cache.Get(conv5.ID)
	if !found {
		t.Error("Expected conv5 to be in cache")
	}
}

// Benchmarks

func BenchmarkLRUCache_Put(b *testing.B) {
	cache := NewLRUCache(1000)
	conversations := make([]*ConversationRecord, b.N)
	
	for i := 0; i < b.N; i++ {
		conversations[i] = generateTestConversationRecord(fmt.Sprintf("bench-put-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(conversations[i].ID, conversations[i])
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(1000)
	
	// Pre-populate cache
	conversations := make([]*ConversationRecord, b.N)
	for i := 0; i < b.N; i++ {
		conv := generateTestConversationRecord(fmt.Sprintf("bench-get-%d", i))
		conversations[i] = conv
		cache.Put(conv.ID, conv)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(conversations[i%len(conversations)].ID)
	}
}

func BenchmarkLRUCache_ConcurrentAccess(b *testing.B) {
	cache := NewLRUCache(1000)
	
	// Pre-populate cache
	for i := 0; i < 500; i++ {
		conv := generateTestConversationRecord(fmt.Sprintf("bench-concurrent-%d", i))
		cache.Put(conv.ID, conv)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			conv := generateTestConversationRecord(fmt.Sprintf("bench-parallel-%d", i))
			cache.Put(conv.ID, conv)
			cache.Get(conv.ID)
			i++
		}
	})
}

func BenchmarkLRUCache_EvictionHeavy(b *testing.B) {
	cache := NewLRUCache(100) // Small cache to force evictions
	
	conversations := make([]*ConversationRecord, b.N)
	for i := 0; i < b.N; i++ {
		conversations[i] = generateTestConversationRecord(fmt.Sprintf("bench-eviction-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Put(conversations[i].ID, conversations[i])
	}
}