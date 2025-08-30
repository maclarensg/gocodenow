package storage

import (
	"context"
	"fmt"
	"gocodenow/internal/types"
	"sync"
	"time"
)

// LazyConversationLoader manages lazy loading of conversation data
type LazyConversationLoader struct {
	database     *Database
	cache        *ConversationCache
	loadMutex    sync.RWMutex
	loadedData   map[string]*LazyConversationData
	loadingQueue chan *LazyLoadRequest
	metrics      *LazyLoadMetrics
}

// LazyConversationData represents a conversation with lazy-loaded content
type LazyConversationData struct {
	// Always loaded metadata
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Status    types.ConversationStatus `json:"status"`
	ModelName string    `json:"model_name"`
	TokenUsageInput  int `json:"token_usage_input"`
	TokenUsageOutput int `json:"token_usage_output"`
	ExecutionTimeMs  int `json:"execution_time_ms"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	// Lazy-loaded content
	UserInput       *string                `json:"user_input,omitempty"`
	LLMResponse     *string                `json:"llm_response,omitempty"`
	ToolExecutions  []ToolExecutionRecord  `json:"tool_executions,omitempty"`
	
	// Loading state
	isContentLoaded bool
	isLoading       bool
	loadError       error
	lastAccessed    time.Time
	mutex           sync.RWMutex
}

// LazyLoadRequest represents a request to load conversation content
type LazyLoadRequest struct {
	ConversationID string
	Priority       LoadPriority
	Callback       func(*LazyConversationData, error)
	RequestTime    time.Time
}

// LoadPriority defines the priority of lazy load requests
type LoadPriority int

const (
	LoadPriorityLow LoadPriority = iota
	LoadPriorityNormal
	LoadPriorityHigh
	LoadPriorityImmediate
)

// LazyLoadMetrics tracks performance metrics for lazy loading
type LazyLoadMetrics struct {
	TotalRequests     int64         `json:"total_requests"`
	CacheHits         int64         `json:"cache_hits"`
	CacheMisses       int64         `json:"cache_misses"`
	AverageLoadTime   time.Duration `json:"average_load_time"`
	PendingRequests   int           `json:"pending_requests"`
	ActiveLoads       int           `json:"active_loads"`
	ErrorCount        int64         `json:"error_count"`
	LastUpdated       time.Time     `json:"last_updated"`
	mutex             sync.RWMutex
}

// NewLazyConversationLoader creates a new lazy conversation loader
func NewLazyConversationLoader(database *Database, cache *ConversationCache) *LazyConversationLoader {
	loader := &LazyConversationLoader{
		database:     database,
		cache:        cache,
		loadedData:   make(map[string]*LazyConversationData),
		loadingQueue: make(chan *LazyLoadRequest, 1000),
		metrics: &LazyLoadMetrics{
			LastUpdated: time.Now(),
		},
	}
	
	// Start background workers
	go loader.processLoadingQueue()
	go loader.cleanupWorker()
	
	return loader
}

// LoadConversationMetadata loads only the conversation metadata (lightweight)
func (l *LazyConversationLoader) LoadConversationMetadata(conversationID string) (*LazyConversationData, error) {
	l.loadMutex.RLock()
	if data, exists := l.loadedData[conversationID]; exists {
		data.lastAccessed = time.Now()
		l.loadMutex.RUnlock()
		return data, nil
	}
	l.loadMutex.RUnlock()

	// Load metadata only from database
	query := `
		SELECT id, timestamp, status, model_name, token_usage_input, token_usage_output,
		       execution_time_ms, created_at, updated_at
		FROM conversations WHERE id = ?`
		
	row := l.database.db.QueryRow(query, conversationID)
	
	data := &LazyConversationData{
		lastAccessed: time.Now(),
	}
	
	err := row.Scan(
		&data.ID, &data.Timestamp, &data.Status, &data.ModelName,
		&data.TokenUsageInput, &data.TokenUsageOutput, &data.ExecutionTimeMs,
		&data.CreatedAt, &data.UpdatedAt,
	)
	
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation metadata: %w", err)
	}
	
	// Cache the metadata
	l.loadMutex.Lock()
	l.loadedData[conversationID] = data
	l.loadMutex.Unlock()
	
	return data, nil
}

// LoadConversationContent loads the full conversation content (heavy operation)
func (l *LazyConversationData) LoadContent(loader *LazyConversationLoader) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	
	if l.isContentLoaded {
		return nil
	}
	
	if l.isLoading {
		// Wait for ongoing load
		for l.isLoading && l.loadError == nil {
			l.mutex.Unlock()
			time.Sleep(10 * time.Millisecond)
			l.mutex.Lock()
		}
		return l.loadError
	}
	
	l.isLoading = true
	startTime := time.Now()
	
	// Load full conversation from database
	// TODO: Implement LoadConversation method in Database
	_ = l.ID // Avoid unused variable  
	var record *ConversationRecord
	err := fmt.Errorf("LoadConversation method not implemented")
	if err != nil {
		l.loadError = err
		l.isLoading = false
		loader.updateMetrics(false, time.Since(startTime))
		return err
	}
	
	// Populate lazy-loaded fields
	l.UserInput = &record.UserInput
	l.LLMResponse = &record.LLMResponse
	l.ToolExecutions = record.ToolExecutions
	
	l.isContentLoaded = true
	l.isLoading = false
	l.lastAccessed = time.Now()
	
	loader.updateMetrics(true, time.Since(startTime))
	return nil
}

// LoadContentAsync loads conversation content asynchronously
func (l *LazyConversationLoader) LoadContentAsync(conversationID string, priority LoadPriority, callback func(*LazyConversationData, error)) {
	request := &LazyLoadRequest{
		ConversationID: conversationID,
		Priority:       priority,
		Callback:       callback,
		RequestTime:    time.Now(),
	}
	
	select {
	case l.loadingQueue <- request:
		l.metrics.mutex.Lock()
		l.metrics.PendingRequests++
		l.metrics.mutex.Unlock()
	default:
		// Queue is full, execute callback with error
		if callback != nil {
			go callback(nil, fmt.Errorf("loading queue is full"))
		}
	}
}

// GetConversation retrieves a conversation with optional content loading
func (l *LazyConversationLoader) GetConversation(conversationID string, loadContent bool) (*LazyConversationData, error) {
	// First get metadata
	data, err := l.LoadConversationMetadata(conversationID)
	if err != nil {
		return nil, err
	}
	
	if loadContent {
		err = data.LoadContent(l)
		if err != nil {
			return nil, err
		}
	}
	
	return data, nil
}

// LoadConversationsPage loads a page of conversations with lazy content
func (l *LazyConversationLoader) LoadConversationsPage(offset, limit int, loadContent bool) ([]*LazyConversationData, error) {
	// Load conversation IDs and metadata efficiently
	query := `
		SELECT id, timestamp, status, model_name, token_usage_input, token_usage_output,
		       execution_time_ms, created_at, updated_at
		FROM conversations 
		ORDER BY timestamp DESC 
		LIMIT ? OFFSET ?`
		
	rows, err := l.database.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to load conversation page: %w", err)
	}
	defer rows.Close()
	
	var conversations []*LazyConversationData
	
	for rows.Next() {
		data := &LazyConversationData{
			lastAccessed: time.Now(),
		}
		
		err := rows.Scan(
			&data.ID, &data.Timestamp, &data.Status, &data.ModelName,
			&data.TokenUsageInput, &data.TokenUsageOutput, &data.ExecutionTimeMs,
			&data.CreatedAt, &data.UpdatedAt,
		)
		
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		
		// Cache metadata
		l.loadMutex.Lock()
		l.loadedData[data.ID] = data
		l.loadMutex.Unlock()
		
		// Optionally load content
		if loadContent {
			if err := data.LoadContent(l); err != nil {
				// Log error but continue with other conversations
				fmt.Printf("Warning: failed to load content for conversation %s: %v\n", data.ID, err)
			}
		}
		
		conversations = append(conversations, data)
	}
	
	return conversations, nil
}

// processLoadingQueue processes async loading requests with priority ordering
func (l *LazyConversationLoader) processLoadingQueue() {
	for request := range l.loadingQueue {
		l.metrics.mutex.Lock()
		l.metrics.PendingRequests--
		l.metrics.ActiveLoads++
		l.metrics.mutex.Unlock()
		
		// Load conversation data
		data, err := l.GetConversation(request.ConversationID, true)
		
		l.metrics.mutex.Lock()
		l.metrics.ActiveLoads--
		l.metrics.TotalRequests++
		if err != nil {
			l.metrics.ErrorCount++
		}
		l.metrics.mutex.Unlock()
		
		// Execute callback
		if request.Callback != nil {
			go request.Callback(data, err)
		}
	}
}

// cleanupWorker periodically cleans up old cached data
func (l *LazyConversationLoader) cleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		l.cleanupOldData()
	}
}

// cleanupOldData removes old conversations from memory
func (l *LazyConversationLoader) cleanupOldData() {
	l.loadMutex.Lock()
	defer l.loadMutex.Unlock()
	
	cutoff := time.Now().Add(-30 * time.Minute) // Remove data not accessed in 30 minutes
	
	for id, data := range l.loadedData {
		data.mutex.RLock()
		lastAccessed := data.lastAccessed
		data.mutex.RUnlock()
		
		if lastAccessed.Before(cutoff) {
			delete(l.loadedData, id)
		}
	}
}

// updateMetrics updates loading performance metrics
func (l *LazyConversationLoader) updateMetrics(success bool, duration time.Duration) {
	l.metrics.mutex.Lock()
	defer l.metrics.mutex.Unlock()
	
	if success {
		l.metrics.CacheHits++
	} else {
		l.metrics.CacheMisses++
	}
	
	// Update average load time using exponential moving average
	if l.metrics.AverageLoadTime == 0 {
		l.metrics.AverageLoadTime = duration
	} else {
		alpha := 0.1 // Smoothing factor
		l.metrics.AverageLoadTime = time.Duration(
			float64(l.metrics.AverageLoadTime)*(1-alpha) + float64(duration)*alpha,
		)
	}
	
	l.metrics.LastUpdated = time.Now()
}

// GetMetrics returns current lazy loading metrics
func (l *LazyConversationLoader) GetMetrics() LazyLoadMetrics {
	l.metrics.mutex.RLock()
	defer l.metrics.mutex.RUnlock()
	
	metrics := *l.metrics // Copy struct
	return metrics
}

// IsContentLoaded checks if conversation content is loaded
func (l *LazyConversationData) IsContentLoaded() bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.isContentLoaded
}

// GetUserInput safely retrieves user input (loads content if needed)
func (l *LazyConversationData) GetUserInput(loader *LazyConversationLoader) (string, error) {
	if !l.IsContentLoaded() {
		if err := l.LoadContent(loader); err != nil {
			return "", err
		}
	}
	
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	
	if l.UserInput != nil {
		return *l.UserInput, nil
	}
	return "", nil
}

// GetLLMResponse safely retrieves LLM response (loads content if needed)
func (l *LazyConversationData) GetLLMResponse(loader *LazyConversationLoader) (string, error) {
	if !l.IsContentLoaded() {
		if err := l.LoadContent(loader); err != nil {
			return "", err
		}
	}
	
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	
	if l.LLMResponse != nil {
		return *l.LLMResponse, nil
	}
	return "", nil
}

// GetToolExecutions safely retrieves tool executions (loads content if needed)
func (l *LazyConversationData) GetToolExecutions(loader *LazyConversationLoader) ([]ToolExecutionRecord, error) {
	if !l.IsContentLoaded() {
		if err := l.LoadContent(loader); err != nil {
			return nil, err
		}
	}
	
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	
	return l.ToolExecutions, nil
}

// Shutdown gracefully shuts down the lazy loader
func (l *LazyConversationLoader) Shutdown(ctx context.Context) error {
	close(l.loadingQueue)
	
	// Wait for pending operations to complete
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for lazy loader shutdown")
		case <-ticker.C:
			l.metrics.mutex.RLock()
			activeLoads := l.metrics.ActiveLoads
			pendingRequests := l.metrics.PendingRequests
			l.metrics.mutex.RUnlock()
			
			if activeLoads == 0 && pendingRequests == 0 {
				return nil
			}
		}
	}
}