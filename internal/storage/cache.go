package storage

import (
	"container/list"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"runtime"
	"sync"
	"time"
)

// ConversationCache provides an intelligent caching layer for conversations
type ConversationCache struct {
	// LRU cache for conversations
	conversations     map[string]*CacheEntry
	conversationList  *list.List
	maxConversations  int
	
	// LRU cache for tool executions
	toolExecutions    map[string]*ToolCacheEntry
	toolList          *list.List
	maxToolExecutions int
	
	// Cache statistics
	stats             *CacheStatistics
	
	// Memory management
	memoryLimit       int64 // bytes
	currentMemory     int64 // bytes
	
	// Synchronization
	mutex             sync.RWMutex
	
	// Configuration
	config            *CacheConfig
}

// CacheEntry represents a cached conversation with metadata
type CacheEntry struct {
	ID           string
	Record       *ConversationRecord
	AccessCount  int64
	LastAccessed time.Time
	CreatedAt    time.Time
	Size         int64 // Estimated memory usage
	TTL          time.Duration
	ListElement  *list.Element
}

// ToolCacheEntry represents a cached tool execution
type ToolCacheEntry struct {
	ID           string
	Record       *ToolExecutionRecord
	AccessCount  int64
	LastAccessed time.Time
	Size         int64
	ListElement  *list.Element
}

// CacheConfig defines cache behavior parameters
type CacheConfig struct {
	MaxConversations     int           `json:"max_conversations"`
	MaxToolExecutions    int           `json:"max_tool_executions"`
	MemoryLimitMB        int64         `json:"memory_limit_mb"`
	DefaultTTL           time.Duration `json:"default_ttl"`
	CleanupInterval      time.Duration `json:"cleanup_interval"`
	CompressionEnabled   bool          `json:"compression_enabled"`
	PreloadRecentCount   int           `json:"preload_recent_count"`
	EvictionPolicy       string        `json:"eviction_policy"` // "lru", "lfu", "ttl"
}

// CacheStatistics tracks cache performance metrics
type CacheStatistics struct {
	Hits                 int64         `json:"hits"`
	Misses               int64         `json:"misses"`
	Evictions            int64         `json:"evictions"`
	MemoryUsage          int64         `json:"memory_usage"`
	ConversationCount    int           `json:"conversation_count"`
	ToolExecutionCount   int           `json:"tool_execution_count"`
	AverageAccessTime    time.Duration `json:"average_access_time"`
	LastCleanup          time.Time     `json:"last_cleanup"`
	HitRate              float64       `json:"hit_rate"`
	mutex                sync.RWMutex
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxConversations:     500,
		MaxToolExecutions:    2000,
		MemoryLimitMB:        256,
		DefaultTTL:           24 * time.Hour,
		CleanupInterval:      10 * time.Minute,
		CompressionEnabled:   true,
		PreloadRecentCount:   50,
		EvictionPolicy:       "lru",
	}
}

// NewConversationCache creates a new conversation cache
func NewConversationCache(config *CacheConfig) *ConversationCache {
	if config == nil {
		config = DefaultCacheConfig()
	}
	
	cache := &ConversationCache{
		conversations:     make(map[string]*CacheEntry),
		conversationList:  list.New(),
		maxConversations:  config.MaxConversations,
		toolExecutions:    make(map[string]*ToolCacheEntry),
		toolList:          list.New(),
		maxToolExecutions: config.MaxToolExecutions,
		memoryLimit:       config.MemoryLimitMB * 1024 * 1024,
		stats: &CacheStatistics{
			LastCleanup: time.Now(),
		},
		config: config,
	}
	
	// Start background cleanup goroutine
	go cache.cleanupWorker()
	go cache.memoryMonitor()
	
	return cache
}

// Get retrieves a conversation from cache
func (c *ConversationCache) Get(conversationID string) (*ConversationRecord, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	startTime := time.Now()
	defer func() {
		c.updateAccessTime(time.Since(startTime))
	}()
	
	entry, exists := c.conversations[conversationID]
	if !exists {
		c.stats.mutex.Lock()
		c.stats.Misses++
		c.stats.mutex.Unlock()
		return nil, false
	}
	
	// Check TTL
	if time.Since(entry.CreatedAt) > entry.TTL {
		c.removeConversationUnsafe(conversationID)
		c.stats.mutex.Lock()
		c.stats.Misses++
		c.stats.mutex.Unlock()
		return nil, false
	}
	
	// Update access information
	entry.AccessCount++
	entry.LastAccessed = time.Now()
	
	// Move to front of LRU list
	c.conversationList.MoveToFront(entry.ListElement)
	
	c.stats.mutex.Lock()
	c.stats.Hits++
	c.stats.mutex.Unlock()
	
	return entry.Record, true
}

// Put stores a conversation in cache
func (c *ConversationCache) Put(record *ConversationRecord) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Check if already exists
	if existingEntry, exists := c.conversations[record.ID]; exists {
		// Update existing entry
		existingEntry.Record = record
		existingEntry.LastAccessed = time.Now()
		existingEntry.Size = c.estimateSize(record)
		c.conversationList.MoveToFront(existingEntry.ListElement)
		c.updateMemoryUsage()
		return
	}
	
	// Create new entry
	entry := &CacheEntry{
		ID:           record.ID,
		Record:       record,
		AccessCount:  1,
		LastAccessed: time.Now(),
		CreatedAt:    time.Now(),
		Size:         c.estimateSize(record),
		TTL:          c.config.DefaultTTL,
	}
	
	// Add to front of LRU list
	entry.ListElement = c.conversationList.PushFront(entry)
	c.conversations[record.ID] = entry
	
	// Update memory usage
	c.currentMemory += entry.Size
	
	// Check if we need to evict
	c.evictIfNecessary()
}

// PutTool stores a tool execution in cache
func (c *ConversationCache) PutTool(record *ToolExecutionRecord) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Check if already exists
	if existingEntry, exists := c.toolExecutions[record.ID]; exists {
		existingEntry.Record = record
		existingEntry.LastAccessed = time.Now()
		c.toolList.MoveToFront(existingEntry.ListElement)
		return
	}
	
	// Create new entry
	entry := &ToolCacheEntry{
		ID:           record.ID,
		Record:       record,
		AccessCount:  1,
		LastAccessed: time.Now(),
		Size:         c.estimateToolSize(record),
	}
	
	// Add to front of LRU list
	entry.ListElement = c.toolList.PushFront(entry)
	c.toolExecutions[record.ID] = entry
	c.currentMemory += entry.Size
	
	// Check if we need to evict tools
	c.evictToolsIfNecessary()
}

// GetTool retrieves a tool execution from cache
func (c *ConversationCache) GetTool(toolExecutionID string) (*ToolExecutionRecord, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	entry, exists := c.toolExecutions[toolExecutionID]
	if !exists {
		return nil, false
	}
	
	// Update access information
	entry.AccessCount++
	entry.LastAccessed = time.Now()
	
	return entry.Record, true
}

// Remove removes a conversation from cache
func (c *ConversationCache) Remove(conversationID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.removeConversationUnsafe(conversationID)
}

// removeConversationUnsafe removes a conversation without locking (internal use)
func (c *ConversationCache) removeConversationUnsafe(conversationID string) {
	entry, exists := c.conversations[conversationID]
	if !exists {
		return
	}
	
	// Remove from list and map
	c.conversationList.Remove(entry.ListElement)
	delete(c.conversations, conversationID)
	c.currentMemory -= entry.Size
}

// Clear removes all entries from cache
func (c *ConversationCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.conversations = make(map[string]*CacheEntry)
	c.conversationList = list.New()
	c.toolExecutions = make(map[string]*ToolCacheEntry)
	c.toolList = list.New()
	c.currentMemory = 0
}

// GetStats returns current cache statistics
func (c *ConversationCache) GetStats() CacheStatistics {
	c.stats.mutex.RLock()
	defer c.stats.mutex.RUnlock()
	
	stats := *c.stats // Copy struct
	
	// Calculate hit rate
	if stats.Hits+stats.Misses > 0 {
		stats.HitRate = float64(stats.Hits) / float64(stats.Hits+stats.Misses)
	}
	
	c.mutex.RLock()
	stats.MemoryUsage = c.currentMemory
	stats.ConversationCount = len(c.conversations)
	stats.ToolExecutionCount = len(c.toolExecutions)
	c.mutex.RUnlock()
	
	return stats
}

// evictIfNecessary removes old entries if cache limits are exceeded
func (c *ConversationCache) evictIfNecessary() {
	// Check conversation count limit
	for len(c.conversations) > c.maxConversations {
		c.evictOldestConversation()
	}
	
	// Check memory limit
	for c.currentMemory > c.memoryLimit {
		if !c.evictByPolicy() {
			break // No more entries to evict
		}
	}
}

// evictToolsIfNecessary removes old tool executions if limit is exceeded
func (c *ConversationCache) evictToolsIfNecessary() {
	for len(c.toolExecutions) > c.maxToolExecutions {
		c.evictOldestTool()
	}
}

// evictOldestConversation removes the least recently used conversation
func (c *ConversationCache) evictOldestConversation() {
	if c.conversationList.Len() == 0 {
		return
	}
	
	// Get oldest entry (back of list)
	oldest := c.conversationList.Back()
	if oldest == nil {
		return
	}
	
	entry := oldest.Value.(*CacheEntry)
	c.removeConversationUnsafe(entry.ID)
	
	c.stats.mutex.Lock()
	c.stats.Evictions++
	c.stats.mutex.Unlock()
}

// evictOldestTool removes the least recently used tool execution
func (c *ConversationCache) evictOldestTool() {
	if c.toolList.Len() == 0 {
		return
	}
	
	oldest := c.toolList.Back()
	if oldest == nil {
		return
	}
	
	entry := oldest.Value.(*ToolCacheEntry)
	c.toolList.Remove(entry.ListElement)
	delete(c.toolExecutions, entry.ID)
	c.currentMemory -= entry.Size
}

// evictByPolicy evicts entries based on the configured eviction policy
func (c *ConversationCache) evictByPolicy() bool {
	switch c.config.EvictionPolicy {
	case "lfu": // Least Frequently Used
		return c.evictLFU()
	case "ttl": // Time To Live
		return c.evictExpired()
	default: // Default to LRU
		c.evictOldestConversation()
		return len(c.conversations) > 0
	}
}

// evictLFU removes the least frequently used conversation
func (c *ConversationCache) evictLFU() bool {
	if len(c.conversations) == 0 {
		return false
	}
	
	var minAccessCount int64 = -1
	var targetID string
	
	for id, entry := range c.conversations {
		if minAccessCount == -1 || entry.AccessCount < minAccessCount {
			minAccessCount = entry.AccessCount
			targetID = id
		}
	}
	
	if targetID != "" {
		c.removeConversationUnsafe(targetID)
		return true
	}
	
	return false
}

// evictExpired removes expired conversations
func (c *ConversationCache) evictExpired() bool {
	now := time.Now()
	evicted := false
	
	for id, entry := range c.conversations {
		if now.Sub(entry.CreatedAt) > entry.TTL {
			c.removeConversationUnsafe(id)
			evicted = true
		}
	}
	
	return evicted
}

// estimateSize estimates the memory usage of a conversation record
func (c *ConversationCache) estimateSize(record *ConversationRecord) int64 {
	if record == nil {
		return 0
	}
	
	size := int64(len(record.UserInput) + len(record.LLMResponse) + len(record.ModelName))
	size += int64(len(record.ID) + len(record.ToolExecutions)*200) // Rough estimate for tool executions
	size += 200 // Base struct size
	
	return size
}

// estimateToolSize estimates the memory usage of a tool execution record
func (c *ConversationCache) estimateToolSize(record *ToolExecutionRecord) int64 {
	if record == nil {
		return 0
	}
	
	size := int64(len(record.Parameters) + len(record.Result) + len(record.ErrorMessage))
	size += int64(len(record.ID) + len(record.ConversationID) + len(record.ToolName))
	size += int64(len(record.FileOperations) * 100) // Rough estimate for file operations
	size += 150 // Base struct size
	
	return size
}

// updateMemoryUsage recalculates current memory usage
func (c *ConversationCache) updateMemoryUsage() {
	c.currentMemory = 0
	
	for _, entry := range c.conversations {
		c.currentMemory += entry.Size
	}
	
	for _, entry := range c.toolExecutions {
		c.currentMemory += entry.Size
	}
}

// updateAccessTime updates average access time statistics
func (c *ConversationCache) updateAccessTime(duration time.Duration) {
	c.stats.mutex.Lock()
	defer c.stats.mutex.Unlock()
	
	if c.stats.AverageAccessTime == 0 {
		c.stats.AverageAccessTime = duration
	} else {
		// Exponential moving average
		alpha := 0.1
		c.stats.AverageAccessTime = time.Duration(
			float64(c.stats.AverageAccessTime)*(1-alpha) + float64(duration)*alpha,
		)
	}
}

// cleanupWorker runs periodic cache cleanup
func (c *ConversationCache) cleanupWorker() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		c.cleanup()
	}
}

// cleanup performs cache maintenance tasks
func (c *ConversationCache) cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Remove expired entries
	c.evictExpired()
	
	// Update memory usage calculation
	c.updateMemoryUsage()
	
	// Update statistics
	c.stats.mutex.Lock()
	c.stats.LastCleanup = time.Now()
	c.stats.mutex.Unlock()
	
	// Force garbage collection if memory usage is high
	if c.currentMemory > c.memoryLimit*80/100 {
		runtime.GC()
	}
}

// memoryMonitor monitors system memory usage
func (c *ConversationCache) memoryMonitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		// If system memory usage is high, be more aggressive with eviction
		if m.HeapInuse > 512*1024*1024 { // 512MB
			c.mutex.Lock()
			
			// Reduce cache size temporarily
			targetSize := c.maxConversations / 2
			for len(c.conversations) > targetSize {
				c.evictOldestConversation()
			}
			
			c.mutex.Unlock()
			runtime.GC()
		}
	}
}

// PreloadRecent preloads recent conversations into cache
func (c *ConversationCache) PreloadRecent(database *Database) error {
	if c.config.PreloadRecentCount <= 0 {
		return nil
	}
	
	// Load recent conversations from database
	query := `
		SELECT id, timestamp, user_input, llm_response, model_name,
		       token_usage_input, token_usage_output, execution_time_ms,
		       status, created_at, updated_at
		FROM conversations
		ORDER BY timestamp DESC
		LIMIT ?`
		
	rows, err := database.db.Query(query, c.config.PreloadRecentCount)
	if err != nil {
		return fmt.Errorf("failed to preload conversations: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		record := &ConversationRecord{}
		
		err := rows.Scan(
			&record.ID, &record.Timestamp, &record.UserInput, &record.LLMResponse,
			&record.ModelName, &record.TokenUsageInput, &record.TokenUsageOutput,
			&record.ExecutionTimeMs, &record.Status, &record.CreatedAt, &record.UpdatedAt,
		)
		
		if err != nil {
			continue // Skip corrupted records
		}
		
		c.Put(record)
	}
	
	return nil
}

// GetMemoryUsage returns current memory usage in bytes
func (c *ConversationCache) GetMemoryUsage() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.currentMemory
}

// SetMemoryLimit updates the cache memory limit
func (c *ConversationCache) SetMemoryLimit(limitMB int64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.memoryLimit = limitMB * 1024 * 1024
	c.config.MemoryLimitMB = limitMB
	
	// Trigger eviction if over new limit
	c.evictIfNecessary()
}

// GetCacheKey generates a cache key for custom content
func GetCacheKey(data interface{}) string {
	h := fnv.New64a()
	
	// Serialize data to JSON for consistent hashing
	jsonData, err := json.Marshal(data)
	if err != nil {
		// Fallback to string representation
		h.Write([]byte(fmt.Sprintf("%v", data)))
	} else {
		h.Write(jsonData)
	}
	
	return fmt.Sprintf("%x", h.Sum64())
}