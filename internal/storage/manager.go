package storage

import (
	"container/list"
	"context"
	"fmt"
	"gocodenow/internal/types"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ConversationRecord represents a conversation with all its related data
type ConversationRecord struct {
	ID               string                `json:"id"`
	Timestamp        time.Time             `json:"timestamp"`
	UserInput        string                `json:"user_input"`
	LLMResponse      string                `json:"llm_response"`
	ModelName        string                `json:"model_name"`
	TokenUsageInput  int                   `json:"token_usage_input"`
	TokenUsageOutput int                   `json:"token_usage_output"`
	ExecutionTimeMs  int                   `json:"execution_time_ms"`
	Status           types.ConversationStatus `json:"status"`
	ToolExecutions   []ToolExecutionRecord `json:"tool_executions,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// ToolExecutionRecord represents a tool execution with its results
type ToolExecutionRecord struct {
	ID             string                `json:"id"`
	ConversationID string                `json:"conversation_id"`
	ToolName       string                `json:"tool_name"`
	Parameters     string                `json:"parameters"`
	Result         string                `json:"result"`
	Success        bool                  `json:"success"`
	DurationMs     int                   `json:"duration_ms"`
	ErrorMessage   string                `json:"error_message,omitempty"`
	FileOperations []FileOperationRecord `json:"file_operations,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
}

// FileOperationRecord represents a file operation performed by a tool
type FileOperationRecord struct {
	ID              string    `json:"id"`
	ToolExecutionID string    `json:"tool_execution_id"`
	OperationType   string    `json:"operation_type"`
	FilePath        string    `json:"file_path"`
	OldContent      string    `json:"old_content,omitempty"`
	NewContent      string    `json:"new_content,omitempty"`
	Success         bool      `json:"success"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// StorageManager manages conversation persistence with memory caching and auto-save
// Implements the types.StorageManager interface
type StorageManager struct {
	db           *Database
	cache        *LRUCache
	autoSaveFreq time.Duration
	autoSaveChan chan struct{}
	stopChan     chan struct{}
	mu           sync.RWMutex
	metrics      StorageMetrics
	ctx          context.Context
	cancel       context.CancelFunc
	closed       bool
}

// StorageMetrics tracks storage performance and statistics
type StorageMetrics struct {
	CacheHits          int64     `json:"cache_hits"`
	CacheMisses        int64     `json:"cache_misses"`
	DatabaseReads      int64     `json:"database_reads"`
	DatabaseWrites     int64     `json:"database_writes"`
	AutoSaves          int64     `json:"auto_saves"`
	LastAutoSave       time.Time `json:"last_auto_save"`
	TotalConversations int64     `json:"total_conversations"`
	
	// Operation counts
	Saves   int64 `json:"saves"`
	Loads   int64 `json:"loads"`
	Deletes int64 `json:"deletes"`
}

// LRUCache implements a thread-safe LRU cache for conversations
type LRUCache struct {
	maxSize  int
	items    map[string]*list.Element
	evictList *list.List
	mu       sync.RWMutex
}

// cacheItem represents an item in the LRU cache
type cacheItem struct {
	key   string
	value *ConversationRecord
}

// StorageConfig holds configuration for the StorageManager
type StorageConfig struct {
	CacheSize       int           `json:"cache_size"`
	AutoSaveFreq    time.Duration `json:"auto_save_frequency"`
	MaxMemoryMB     int           `json:"max_memory_mb"`
	BackupInterval  time.Duration `json:"backup_interval"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// DefaultStorageConfig returns a default storage configuration
func DefaultStorageConfig() StorageConfig {
	return StorageConfig{
		CacheSize:       50,
		AutoSaveFreq:    5 * time.Minute,
		MaxMemoryMB:     100,
		BackupInterval:  24 * time.Hour,
		CleanupInterval: 7 * 24 * time.Hour,
	}
}

// NewStorageManager creates a new storage manager with the given database and configuration
func NewStorageManager(db *Database, config StorageConfig) (*StorageManager, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}

	// Validate configuration
	if config.CacheSize <= 0 {
		return nil, fmt.Errorf("cache size must be greater than 0")
	}
	if config.AutoSaveFreq <= 0 {
		return nil, fmt.Errorf("auto-save frequency must be greater than 0")
	}

	ctx, cancel := context.WithCancel(context.Background())

	sm := &StorageManager{
		db:           db,
		cache:        NewLRUCache(config.CacheSize),
		autoSaveFreq: config.AutoSaveFreq,
		autoSaveChan: make(chan struct{}, 1),
		stopChan:     make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start background processes
	go sm.autoSaveWorker()
	
	log.Printf("StorageManager initialized with cache size %d, auto-save frequency %v", 
		config.CacheSize, config.AutoSaveFreq)

	return sm, nil
}

// NewLRUCache creates a new LRU cache with the specified size
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		maxSize:   maxSize,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

// Get retrieves a value from the LRU cache
func (c *LRUCache) Get(key string) (*ConversationRecord, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if elem, exists := c.items[key]; exists {
		// Move to front (most recently used)
		c.evictList.MoveToFront(elem)
		return elem.Value.(*cacheItem).value, true
	}
	return nil, false
}

// Put adds or updates a value in the LRU cache
func (c *LRUCache) Put(key string, value *ConversationRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If key already exists, update it and move to front
	if elem, exists := c.items[key]; exists {
		c.evictList.MoveToFront(elem)
		elem.Value.(*cacheItem).value = value
		return
	}

	// Add new item
	item := &cacheItem{key: key, value: value}
	elem := c.evictList.PushFront(item)
	c.items[key] = elem

	// Check if we need to evict
	if c.evictList.Len() > c.maxSize {
		c.evictOldest()
	}
}

// evictOldest removes the least recently used item
func (c *LRUCache) evictOldest() {
	if elem := c.evictList.Back(); elem != nil {
		c.removeElement(elem)
	}
}

// removeElement removes a specific element from the cache
func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	item := elem.Value.(*cacheItem)
	delete(c.items, item.key)
}

// Remove removes a key from the cache
func (c *LRUCache) Remove(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.items[key]; exists {
		c.removeElement(elem)
		return true
	}
	return false
}

// Size returns the current number of items in the cache
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[string]*list.Element)
	c.evictList.Init()
}

// Keys returns all keys in the cache (most recent first)
func (c *LRUCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for elem := c.evictList.Front(); elem != nil; elem = elem.Next() {
		keys = append(keys, elem.Value.(*cacheItem).key)
	}
	return keys
}

// Delete removes an item from the cache (alias for Remove)
func (c *LRUCache) Delete(key string) {
	c.Remove(key)
}

// GetAll returns all conversation records in the cache (most recent first)
func (c *LRUCache) GetAll() []*ConversationRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	records := make([]*ConversationRecord, 0, len(c.items))
	for elem := c.evictList.Front(); elem != nil; elem = elem.Next() {
		records = append(records, elem.Value.(*cacheItem).value)
	}
	return records
}

// EstimateMemoryUsage returns an estimate of memory usage in bytes
func (c *LRUCache) EstimateMemoryUsage() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var totalBytes int64
	for elem := c.evictList.Front(); elem != nil; elem = elem.Next() {
		conv := elem.Value.(*cacheItem).value
		
		// Estimate size of conversation record
		totalBytes += int64(len(conv.ID))
		totalBytes += int64(len(conv.UserInput))
		totalBytes += int64(len(conv.LLMResponse))
		totalBytes += int64(len(conv.ModelName))
		totalBytes += 64 // Approximate size for timestamps and other fields
		
		// Add tool execution sizes
		for _, toolExec := range conv.ToolExecutions {
			totalBytes += int64(len(toolExec.ID))
			totalBytes += int64(len(toolExec.Parameters))
			totalBytes += int64(len(toolExec.Result))
			totalBytes += int64(len(toolExec.ErrorMessage))
			totalBytes += 32 // Other fields
		}
	}
	
	return totalBytes
}

// SaveConversation saves a conversation to the database and cache
func (sm *StorageManager) SaveConversation(conv *ConversationRecord) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.closed {
		return fmt.Errorf("storage manager is closed")
	}

	// Generate ID if not provided
	if conv.ID == "" {
		conv.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	if conv.CreatedAt.IsZero() {
		conv.CreatedAt = now
	}
	conv.UpdatedAt = now

	// Save to database
	if err := sm.saveConversationToDatabase(conv); err != nil {
		return fmt.Errorf("failed to save conversation to database: %w", err)
	}

	// Update cache
	sm.cache.Put(conv.ID, conv)

	// Update metrics
	sm.metrics.DatabaseWrites++
	sm.metrics.TotalConversations++
	sm.metrics.Saves++

	// Trigger auto-save check
	select {
	case sm.autoSaveChan <- struct{}{}:
	default:
	}

	return nil
}

// LoadConversation loads a conversation by ID, checking cache first
func (sm *StorageManager) LoadConversation(id string) (*ConversationRecord, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Check cache first
	if conv, exists := sm.cache.Get(id); exists {
		sm.metrics.CacheHits++
		sm.metrics.Loads++
		return conv, nil
	}

	sm.metrics.CacheMisses++

	// Load from database
	conv, err := sm.loadConversationFromDatabase(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation from database: %w", err)
	}

	// Update cache
	sm.cache.Put(id, conv)
	sm.metrics.DatabaseReads++
	sm.metrics.Loads++

	return conv, nil
}

// LoadRecentConversations loads the most recent conversations
func (sm *StorageManager) LoadRecentConversations(limit int) ([]*ConversationRecord, error) {
	if limit <= 0 {
		limit = 20 // Default session recovery size
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	conversations, err := sm.loadRecentConversationsFromDatabase(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to load recent conversations: %w", err)
	}

	// Update cache with loaded conversations
	for _, conv := range conversations {
		sm.cache.Put(conv.ID, conv)
	}

	sm.metrics.DatabaseReads++
	return conversations, nil
}

// DeleteConversation removes a conversation from both cache and database
func (sm *StorageManager) DeleteConversation(id string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Remove from cache
	sm.cache.Remove(id)

	// Remove from database
	if err := sm.deleteConversationFromDatabase(id); err != nil {
		return fmt.Errorf("failed to delete conversation from database: %w", err)
	}

	sm.metrics.DatabaseWrites++
	sm.metrics.Deletes++
	return nil
}

// ListConversations returns conversations with pagination
func (sm *StorageManager) ListConversations(offset, limit int) ([]*ConversationRecord, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// For now, use the LoadRecentConversations method with pagination logic
	// In a full implementation, this would be more sophisticated
	allConversations, err := sm.loadRecentConversationsFromDatabase(offset + limit)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversations: %w", err)
	}

	// Apply offset and limit
	start := offset
	if start > len(allConversations) {
		return []*ConversationRecord{}, nil
	}

	end := start + limit
	if end > len(allConversations) {
		end = len(allConversations)
	}

	return allConversations[start:end], nil
}

// GetMetrics returns current storage metrics
func (sm *StorageManager) GetMetrics() StorageMetrics {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.metrics
}

// Close gracefully shuts down the storage manager
func (sm *StorageManager) Close() error {
	log.Println("Shutting down StorageManager...")

	// Cancel context to stop background workers
	sm.cancel()

	// Wait for auto-save worker to finish
	select {
	case <-sm.stopChan:
	case <-time.After(5 * time.Second):
		log.Println("Warning: Auto-save worker did not shut down gracefully")
	}

	// Perform final save
	if err := sm.flushCache(); err != nil {
		log.Printf("Warning: Failed to flush cache during shutdown: %v", err)
	}

	// Mark as closed
	sm.mu.Lock()
	sm.closed = true
	sm.mu.Unlock()

	// Clear cache
	sm.cache.Clear()

	log.Println("StorageManager shutdown complete")
	return nil
}

// Shutdown implements the types.StorageManager interface
func (sm *StorageManager) Shutdown() error {
	return sm.Close()
}

// autoSaveWorker runs in the background and performs periodic saves
func (sm *StorageManager) autoSaveWorker() {
	defer close(sm.stopChan)

	ticker := time.NewTicker(sm.autoSaveFreq)
	defer ticker.Stop()

	conversationCount := 0
	
	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			// Periodic auto-save
			if err := sm.performAutoSave(); err != nil {
				log.Printf("Auto-save failed: %v", err)
			}
		case <-sm.autoSaveChan:
			// Conversation count based auto-save
			conversationCount++
			if conversationCount >= 10 {
				if err := sm.performAutoSave(); err != nil {
					log.Printf("Auto-save failed: %v", err)
				}
				conversationCount = 0
			}
		}
	}
}

// performAutoSave executes an auto-save operation
func (sm *StorageManager) performAutoSave() error {
	log.Println("Performing auto-save...")
	
	if err := sm.flushCache(); err != nil {
		return fmt.Errorf("failed to flush cache: %w", err)
	}

	sm.mu.Lock()
	sm.metrics.AutoSaves++
	sm.metrics.LastAutoSave = time.Now()
	sm.mu.Unlock()

	log.Println("Auto-save completed")
	return nil
}

// flushCache saves all cached conversations that have been modified
func (sm *StorageManager) flushCache() error {
	// For now, we assume all cached items are already saved to database
	// In a more sophisticated implementation, we would track dirty state
	return nil
}

// Database operation methods (these will interact with the Database)
func (sm *StorageManager) saveConversationToDatabase(conv *ConversationRecord) error {
	// Save main conversation record
	query := `INSERT OR REPLACE INTO conversations 
		(id, timestamp, user_input, llm_response, model_name, token_usage_input, 
		 token_usage_output, execution_time_ms, status, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := sm.db.DB().Exec(query, conv.ID, conv.Timestamp, conv.UserInput, 
		conv.LLMResponse, conv.ModelName, conv.TokenUsageInput, conv.TokenUsageOutput,
		conv.ExecutionTimeMs, conv.Status.String(), conv.CreatedAt, conv.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to save conversation: %w", err)
	}

	// Save tool executions
	for _, toolExec := range conv.ToolExecutions {
		if err := sm.saveToolExecutionToDatabase(&toolExec); err != nil {
			return fmt.Errorf("failed to save tool execution: %w", err)
		}
	}

	return nil
}

func (sm *StorageManager) saveToolExecutionToDatabase(toolExec *ToolExecutionRecord) error {
	query := `INSERT OR REPLACE INTO tool_executions 
		(id, conversation_id, tool_name, parameters, result, success, duration_ms, error_message, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := sm.db.DB().Exec(query, toolExec.ID, toolExec.ConversationID, toolExec.ToolName,
		toolExec.Parameters, toolExec.Result, toolExec.Success, toolExec.DurationMs,
		toolExec.ErrorMessage, toolExec.CreatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to save tool execution: %w", err)
	}

	// Save file operations
	for _, fileOp := range toolExec.FileOperations {
		if err := sm.saveFileOperationToDatabase(&fileOp); err != nil {
			return fmt.Errorf("failed to save file operation: %w", err)
		}
	}

	return nil
}

func (sm *StorageManager) saveFileOperationToDatabase(fileOp *FileOperationRecord) error {
	query := `INSERT OR REPLACE INTO file_operations 
		(id, tool_execution_id, operation_type, file_path, old_content, new_content, success, error_message, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	_, err := sm.db.DB().Exec(query, fileOp.ID, fileOp.ToolExecutionID, fileOp.OperationType,
		fileOp.FilePath, fileOp.OldContent, fileOp.NewContent, fileOp.Success,
		fileOp.ErrorMessage, fileOp.CreatedAt)
	
	return err
}

func (sm *StorageManager) loadConversationFromDatabase(id string) (*ConversationRecord, error) {
	// Load main conversation
	conv := &ConversationRecord{}
	var statusStr string
	
	query := `SELECT id, timestamp, user_input, llm_response, model_name, token_usage_input, 
		token_usage_output, execution_time_ms, status, created_at, updated_at 
		FROM conversations WHERE id = ?`
	
	err := sm.db.DB().QueryRow(query, id).Scan(&conv.ID, &conv.Timestamp, &conv.UserInput,
		&conv.LLMResponse, &conv.ModelName, &conv.TokenUsageInput, &conv.TokenUsageOutput,
		&conv.ExecutionTimeMs, &statusStr, &conv.CreatedAt, &conv.UpdatedAt)
	
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	
	conv.Status = types.ParseConversationStatus(statusStr)

	// Load tool executions
	toolExecs, err := sm.loadToolExecutionsFromDatabase(id)
	if err != nil {
		return nil, fmt.Errorf("failed to load tool executions: %w", err)
	}
	conv.ToolExecutions = toolExecs

	return conv, nil
}

func (sm *StorageManager) loadToolExecutionsFromDatabase(conversationID string) ([]ToolExecutionRecord, error) {
	query := `SELECT id, conversation_id, tool_name, parameters, result, success, duration_ms, error_message, created_at 
		FROM tool_executions WHERE conversation_id = ? ORDER BY created_at`
	
	rows, err := sm.db.DB().Query(query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var toolExecs []ToolExecutionRecord
	for rows.Next() {
		var toolExec ToolExecutionRecord
		err := rows.Scan(&toolExec.ID, &toolExec.ConversationID, &toolExec.ToolName,
			&toolExec.Parameters, &toolExec.Result, &toolExec.Success, &toolExec.DurationMs,
			&toolExec.ErrorMessage, &toolExec.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Load file operations for this tool execution
		fileOps, err := sm.loadFileOperationsFromDatabase(toolExec.ID)
		if err != nil {
			return nil, err
		}
		toolExec.FileOperations = fileOps
		
		toolExecs = append(toolExecs, toolExec)
	}

	return toolExecs, nil
}

func (sm *StorageManager) loadFileOperationsFromDatabase(toolExecutionID string) ([]FileOperationRecord, error) {
	query := `SELECT id, tool_execution_id, operation_type, file_path, old_content, new_content, success, error_message, created_at 
		FROM file_operations WHERE tool_execution_id = ? ORDER BY created_at`
	
	rows, err := sm.db.DB().Query(query, toolExecutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fileOps []FileOperationRecord
	for rows.Next() {
		var fileOp FileOperationRecord
		err := rows.Scan(&fileOp.ID, &fileOp.ToolExecutionID, &fileOp.OperationType,
			&fileOp.FilePath, &fileOp.OldContent, &fileOp.NewContent, &fileOp.Success,
			&fileOp.ErrorMessage, &fileOp.CreatedAt)
		if err != nil {
			return nil, err
		}
		fileOps = append(fileOps, fileOp)
	}

	return fileOps, nil
}

func (sm *StorageManager) loadRecentConversationsFromDatabase(limit int) ([]*ConversationRecord, error) {
	query := `SELECT id, timestamp, user_input, llm_response, model_name, token_usage_input, 
		token_usage_output, execution_time_ms, status, created_at, updated_at 
		FROM conversations ORDER BY timestamp DESC LIMIT ?`
	
	rows, err := sm.db.DB().Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []*ConversationRecord
	for rows.Next() {
		conv := &ConversationRecord{}
		var statusStr string
		
		err := rows.Scan(&conv.ID, &conv.Timestamp, &conv.UserInput, &conv.LLMResponse,
			&conv.ModelName, &conv.TokenUsageInput, &conv.TokenUsageOutput,
			&conv.ExecutionTimeMs, &statusStr, &conv.CreatedAt, &conv.UpdatedAt)
		if err != nil {
			return nil, err
		}
		
		conv.Status = types.ParseConversationStatus(statusStr)

		// Load tool executions for each conversation
		toolExecs, err := sm.loadToolExecutionsFromDatabase(conv.ID)
		if err != nil {
			return nil, err
		}
		conv.ToolExecutions = toolExecs
		
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

func (sm *StorageManager) deleteConversationFromDatabase(id string) error {
	// Foreign key constraints will cascade delete tool_executions and file_operations
	query := `DELETE FROM conversations WHERE id = ?`
	_, err := sm.db.DB().Exec(query, id)
	return err
}

// Interface implementation methods for types.StorageManager
// These convert between types.ConversationBlock and internal ConversationRecord

// Save implements types.ConversationRepository.Save
func (sm *StorageManager) Save(conv *types.ConversationBlock) error {
	record := sm.conversationBlockToRecord(conv)
	return sm.SaveConversation(record)
}

// Load implements types.ConversationRepository.Load
func (sm *StorageManager) Load(id string) (*types.ConversationBlock, error) {
	record, err := sm.LoadConversation(id)
	if err != nil {
		return nil, err
	}
	block := sm.conversationRecordToBlock(record)
	return &block, nil
}

// Delete implements types.ConversationRepository.Delete
func (sm *StorageManager) Delete(id string) error {
	return sm.DeleteConversation(id)
}

// LoadRecent implements types.ConversationRepository.LoadRecent
func (sm *StorageManager) LoadRecent(limit int) ([]*types.ConversationBlock, error) {
	records, err := sm.LoadRecentConversations(limit)
	if err != nil {
		return nil, err
	}
	
	blocks := make([]*types.ConversationBlock, len(records))
	for i, record := range records {
		block := sm.conversationRecordToBlock(record)
		blocks[i] = &block
	}
	return blocks, nil
}

// LoadAll implements types.ConversationRepository.LoadAll
func (sm *StorageManager) LoadAll() ([]*types.ConversationBlock, error) {
	// Use LoadRecentConversations with a large limit to get all conversations
	records, err := sm.LoadRecentConversations(10000) // Large enough limit for all conversations
	if err != nil {
		return nil, err
	}
	
	blocks := make([]*types.ConversationBlock, len(records))
	for i, record := range records {
		block := sm.conversationRecordToBlock(record)
		blocks[i] = &block
	}
	return blocks, nil
}

// Update implements types.ConversationRepository.Update
func (sm *StorageManager) Update(conv *types.ConversationBlock) error {
	record := sm.conversationBlockToRecord(conv)
	// Use SaveConversation which does INSERT OR REPLACE (acts as update)
	return sm.SaveConversation(record)
}

// RefreshCache implements types.StorageManager.RefreshCache
func (sm *StorageManager) RefreshCache() error {
	// Load recent conversations into cache
	records, err := sm.LoadRecentConversations(sm.cache.maxSize)
	if err != nil {
		return err
	}
	
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Clear existing cache
	sm.cache = NewLRUCache(sm.cache.maxSize)
	
	// Add conversations to cache
	for _, record := range records {
		sm.cache.Put(record.ID, record)
	}
	
	return nil
}

// GetCacheSize implements types.StorageManager.GetCacheSize
func (sm *StorageManager) GetCacheSize() int {
	return sm.cache.Size()
}

// FlushCache implements types.StorageManager.FlushCache
func (sm *StorageManager) FlushCache() error {
	return sm.flushCache()
}

// Conversion functions between types.ConversationBlock and ConversationRecord

// conversationBlockToRecord converts types.ConversationBlock to ConversationRecord
func (sm *StorageManager) conversationBlockToRecord(block *types.ConversationBlock) *ConversationRecord {
	return &ConversationRecord{
		ID:               block.ID,
		Timestamp:        block.Timestamp,
		UserInput:        block.UserInput,
		LLMResponse:      block.LLMResponse,
		ModelName:        block.ModelName,
		TokenUsageInput:  block.TokenUsage.InputTokens,
		TokenUsageOutput: block.TokenUsage.OutputTokens,
		ExecutionTimeMs:  int(block.ExecutionTime.Milliseconds()),
		Status:           block.Status,
		ToolExecutions:   []ToolExecutionRecord{}, // TODO: Convert from block.ToolCalls/ToolResults
		CreatedAt:        block.Timestamp,
		UpdatedAt:        time.Now(),
	}
}

// conversationRecordToBlock converts ConversationRecord to types.ConversationBlock
func (sm *StorageManager) conversationRecordToBlock(record *ConversationRecord) types.ConversationBlock {
	return types.ConversationBlock{
		ID:          record.ID,
		UserInput:   record.UserInput,
		LLMResponse: record.LLMResponse,
		ModelName:   record.ModelName,
		Timestamp:   record.Timestamp,
		Status:      record.Status,
		TokenUsage: types.TokenUsage{
			InputTokens:  record.TokenUsageInput,
			OutputTokens: record.TokenUsageOutput,
		},
		ExecutionTime:  time.Duration(record.ExecutionTimeMs) * time.Millisecond,
		ToolCalls:      []types.ToolCall{},      // TODO: Convert from record.ToolExecutions
		ToolResults:    []types.ToolResult{},    // TODO: Convert from record.ToolExecutions
		FileOperations: []types.FileOperation{}, // TODO: Convert from record.ToolExecutions
		Expanded:       false,                   // Default value for UI state
	}
}