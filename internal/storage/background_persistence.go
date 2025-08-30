package storage

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BackgroundPersistenceManager handles asynchronous database operations
type BackgroundPersistenceManager struct {
	database         *Database
	cache            *ConversationCache
	
	// Operation queues
	saveQueue        chan *SaveOperation
	deleteQueue      chan *DeleteOperation
	updateQueue      chan *UpdateOperation
	
	// Batch operations
	batchSaveQueue   chan []*SaveOperation
	batchSize        int
	batchTimeout     time.Duration
	
	// Configuration
	config           *PersistenceConfig
	
	// State management
	running          bool
	workers          int
	workerWg         sync.WaitGroup
	shutdownCh       chan struct{}
	
	// Metrics
	metrics          *PersistenceMetrics
	
	// Synchronization
	mutex            sync.RWMutex
}

// SaveOperation represents an asynchronous save operation
type SaveOperation struct {
	Type         string                 `json:"type"` // "conversation", "tool_execution", "file_operation"
	Data         interface{}            `json:"data"`
	Priority     OperationPriority      `json:"priority"`
	Callback     func(error)            `json:"-"`
	Timeout      time.Duration          `json:"timeout"`
	CreatedAt    time.Time             `json:"created_at"`
	AttemptCount int                   `json:"attempt_count"`
}

// DeleteOperation represents an asynchronous delete operation
type DeleteOperation struct {
	Type      string            `json:"type"`
	ID        string            `json:"id"`
	Priority  OperationPriority `json:"priority"`
	Callback  func(error)       `json:"-"`
	CreatedAt time.Time         `json:"created_at"`
}

// UpdateOperation represents an asynchronous update operation
type UpdateOperation struct {
	Type      string            `json:"type"`
	ID        string            `json:"id"`
	Data      interface{}       `json:"data"`
	Priority  OperationPriority `json:"priority"`
	Callback  func(error)       `json:"-"`
	CreatedAt time.Time         `json:"created_at"`
}

// OperationPriority defines the priority of background operations
type OperationPriority int

const (
	PriorityLow OperationPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityImmediate
)

// PersistenceConfig configures background persistence behavior
type PersistenceConfig struct {
	QueueSize        int           `json:"queue_size"`
	WorkerCount      int           `json:"worker_count"`
	BatchSize        int           `json:"batch_size"`
	BatchTimeout     time.Duration `json:"batch_timeout"`
	MaxRetries       int           `json:"max_retries"`
	RetryDelay       time.Duration `json:"retry_delay"`
	FlushInterval    time.Duration `json:"flush_interval"`
	EnableBatching   bool          `json:"enable_batching"`
	EnableCompression bool         `json:"enable_compression"`
}

// PersistenceMetrics tracks background persistence performance
type PersistenceMetrics struct {
	TotalOperations     int64         `json:"total_operations"`
	SuccessfulOps       int64         `json:"successful_ops"`
	FailedOps           int64         `json:"failed_ops"`
	QueueLength         int           `json:"queue_length"`
	AverageProcessTime  time.Duration `json:"average_process_time"`
	BatchOperations     int64         `json:"batch_operations"`
	RetryOperations     int64         `json:"retry_operations"`
	LastFlush           time.Time     `json:"last_flush"`
	mutex               sync.RWMutex
}

// DefaultPersistenceConfig returns default configuration
func DefaultPersistenceConfig() *PersistenceConfig {
	return &PersistenceConfig{
		QueueSize:         10000,
		WorkerCount:       4,
		BatchSize:         50,
		BatchTimeout:      5 * time.Second,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		FlushInterval:     30 * time.Second,
		EnableBatching:    true,
		EnableCompression: false,
	}
}

// NewBackgroundPersistenceManager creates a new background persistence manager
func NewBackgroundPersistenceManager(database *Database, cache *ConversationCache, config *PersistenceConfig) *BackgroundPersistenceManager {
	if config == nil {
		config = DefaultPersistenceConfig()
	}
	
	manager := &BackgroundPersistenceManager{
		database:       database,
		cache:          cache,
		saveQueue:      make(chan *SaveOperation, config.QueueSize),
		deleteQueue:    make(chan *DeleteOperation, config.QueueSize/2),
		updateQueue:    make(chan *UpdateOperation, config.QueueSize/2),
		batchSaveQueue: make(chan []*SaveOperation, 100),
		batchSize:      config.BatchSize,
		batchTimeout:   config.BatchTimeout,
		config:         config,
		running:        false,
		workers:        config.WorkerCount,
		shutdownCh:     make(chan struct{}),
		metrics: &PersistenceMetrics{
			LastFlush: time.Now(),
		},
	}
	
	return manager
}

// Start begins background persistence operations
func (bpm *BackgroundPersistenceManager) Start() error {
	bpm.mutex.Lock()
	defer bpm.mutex.Unlock()
	
	if bpm.running {
		return fmt.Errorf("background persistence manager is already running")
	}
	
	bpm.running = true
	
	// Start worker goroutines
	for i := 0; i < bpm.workers; i++ {
		bpm.workerWg.Add(1)
		go bpm.worker(i)
	}
	
	// Start batch processor if batching is enabled
	if bpm.config.EnableBatching {
		bpm.workerWg.Add(1)
		go bpm.batchProcessor()
	}
	
	// Start periodic flush worker
	bpm.workerWg.Add(1)
	go bpm.flushWorker()
	
	// Start metrics collector
	bpm.workerWg.Add(1)
	go bpm.metricsCollector()
	
	return nil
}

// Stop gracefully shuts down background persistence
func (bpm *BackgroundPersistenceManager) Stop(ctx context.Context) error {
	bpm.mutex.Lock()
	if !bpm.running {
		bpm.mutex.Unlock()
		return nil
	}
	bpm.running = false
	bpm.mutex.Unlock()
	
	// Signal shutdown
	close(bpm.shutdownCh)
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		bpm.workerWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SaveAsync queues a save operation for background processing
func (bpm *BackgroundPersistenceManager) SaveAsync(operationType string, data interface{}, priority OperationPriority, callback func(error)) error {
	operation := &SaveOperation{
		Type:      operationType,
		Data:      data,
		Priority:  priority,
		Callback:  callback,
		Timeout:   30 * time.Second,
		CreatedAt: time.Now(),
	}
	
	select {
	case bpm.saveQueue <- operation:
		return nil
	default:
		return fmt.Errorf("save queue is full")
	}
}

// DeleteAsync queues a delete operation for background processing
func (bpm *BackgroundPersistenceManager) DeleteAsync(operationType, id string, priority OperationPriority, callback func(error)) error {
	operation := &DeleteOperation{
		Type:      operationType,
		ID:        id,
		Priority:  priority,
		Callback:  callback,
		CreatedAt: time.Now(),
	}
	
	select {
	case bpm.deleteQueue <- operation:
		return nil
	default:
		return fmt.Errorf("delete queue is full")
	}
}

// UpdateAsync queues an update operation for background processing
func (bpm *BackgroundPersistenceManager) UpdateAsync(operationType, id string, data interface{}, priority OperationPriority, callback func(error)) error {
	operation := &UpdateOperation{
		Type:      operationType,
		ID:        id,
		Data:      data,
		Priority:  priority,
		Callback:  callback,
		CreatedAt: time.Now(),
	}
	
	select {
	case bpm.updateQueue <- operation:
		return nil
	default:
		return fmt.Errorf("update queue is full")
	}
}

// SaveConversationAsync saves a conversation asynchronously
func (bpm *BackgroundPersistenceManager) SaveConversationAsync(record *ConversationRecord, priority OperationPriority, callback func(error)) error {
	// Update cache immediately for fast retrieval
	if bpm.cache != nil {
		bpm.cache.Put(record)
	}
	
	return bpm.SaveAsync("conversation", record, priority, callback)
}

// SaveToolExecutionAsync saves a tool execution asynchronously
func (bpm *BackgroundPersistenceManager) SaveToolExecutionAsync(record *ToolExecutionRecord, priority OperationPriority, callback func(error)) error {
	// Update cache immediately
	if bpm.cache != nil {
		bpm.cache.PutTool(record)
	}
	
	return bpm.SaveAsync("tool_execution", record, priority, callback)
}

// BatchSave saves multiple operations as a batch
func (bpm *BackgroundPersistenceManager) BatchSave(operations []*SaveOperation) error {
	if !bpm.config.EnableBatching {
		// Process individually if batching is disabled
		for _, op := range operations {
			if err := bpm.SaveAsync(op.Type, op.Data, op.Priority, op.Callback); err != nil {
				return err
			}
		}
		return nil
	}
	
	select {
	case bpm.batchSaveQueue <- operations:
		return nil
	default:
		return fmt.Errorf("batch save queue is full")
	}
}

// worker processes operations from queues
func (bpm *BackgroundPersistenceManager) worker(workerID int) {
	defer bpm.workerWg.Done()
	
	for {
		select {
		case <-bpm.shutdownCh:
			return
			
		case saveOp := <-bpm.saveQueue:
			bpm.processSaveOperation(saveOp)
			
		case deleteOp := <-bpm.deleteQueue:
			bpm.processDeleteOperation(deleteOp)
			
		case updateOp := <-bpm.updateQueue:
			bpm.processUpdateOperation(updateOp)
		}
	}
}

// batchProcessor handles batch operations
func (bpm *BackgroundPersistenceManager) batchProcessor() {
	defer bpm.workerWg.Done()
	
	ticker := time.NewTicker(bpm.batchTimeout)
	defer ticker.Stop()
	
	var pendingBatch []*SaveOperation
	
	for {
		select {
		case <-bpm.shutdownCh:
			// Process any remaining batch before shutdown
			if len(pendingBatch) > 0 {
				bpm.processBatch(pendingBatch)
			}
			return
			
		case batch := <-bpm.batchSaveQueue:
			bpm.processBatch(batch)
			
		case saveOp := <-bpm.saveQueue:
			pendingBatch = append(pendingBatch, saveOp)
			
			// Process batch when it reaches target size
			if len(pendingBatch) >= bpm.batchSize {
				bpm.processBatch(pendingBatch)
				pendingBatch = nil
			}
			
		case <-ticker.C:
			// Process batch on timeout
			if len(pendingBatch) > 0 {
				bpm.processBatch(pendingBatch)
				pendingBatch = nil
			}
		}
	}
}

// processSaveOperation processes a single save operation
func (bpm *BackgroundPersistenceManager) processSaveOperation(op *SaveOperation) {
	startTime := time.Now()
	var err error
	
	defer func() {
		duration := time.Since(startTime)
		bpm.updateMetrics(err == nil, duration, false)
		
		if op.Callback != nil {
			go op.Callback(err)
		}
	}()
	
	switch op.Type {
	case "conversation":
		if record, ok := op.Data.(*ConversationRecord); ok {
			// TODO: Implement SaveConversation method in Database
			_ = record // Avoid unused variable
			err = nil  // Temporary - no-op
		} else {
			err = fmt.Errorf("invalid conversation data type")
		}
		
	case "tool_execution":
		if record, ok := op.Data.(*ToolExecutionRecord); ok {
			// TODO: Implement SaveToolExecution method in Database
			_ = record // Avoid unused variable
			err = nil  // Temporary - no-op
		} else {
			err = fmt.Errorf("invalid tool execution data type")
		}
		
	case "file_operation":
		if record, ok := op.Data.(*FileOperationRecord); ok {
			// TODO: Implement SaveFileOperation method in Database
			_ = record // Avoid unused variable
			err = nil  // Temporary - no-op
		} else {
			err = fmt.Errorf("invalid file operation data type")
		}
		
	default:
		err = fmt.Errorf("unknown operation type: %s", op.Type)
	}
	
	// Retry on failure
	if err != nil && op.AttemptCount < bpm.config.MaxRetries {
		op.AttemptCount++
		time.Sleep(bpm.config.RetryDelay)
		
		select {
		case bpm.saveQueue <- op:
			bpm.metrics.mutex.Lock()
			bpm.metrics.RetryOperations++
			bpm.metrics.mutex.Unlock()
		default:
			// Queue full, give up
		}
	}
}

// processDeleteOperation processes a delete operation
func (bpm *BackgroundPersistenceManager) processDeleteOperation(op *DeleteOperation) {
	startTime := time.Now()
	var err error
	
	defer func() {
		duration := time.Since(startTime)
		bpm.updateMetrics(err == nil, duration, false)
		
		if op.Callback != nil {
			go op.Callback(err)
		}
	}()
	
	switch op.Type {
	case "conversation":
		// TODO: Implement DeleteConversation method in Database
		_ = op.ID // Avoid unused variable
		err = nil // Temporary - no-op
		if err == nil && bpm.cache != nil {
			bpm.cache.Remove(op.ID)
		}
		
	default:
		err = fmt.Errorf("unknown delete operation type: %s", op.Type)
	}
}

// processUpdateOperation processes an update operation
func (bpm *BackgroundPersistenceManager) processUpdateOperation(op *UpdateOperation) {
	startTime := time.Now()
	var err error
	
	defer func() {
		duration := time.Since(startTime)
		bpm.updateMetrics(err == nil, duration, false)
		
		if op.Callback != nil {
			go op.Callback(err)
		}
	}()
	
	switch op.Type {
	case "conversation":
		if record, ok := op.Data.(*ConversationRecord); ok {
			record.ID = op.ID
			// TODO: Implement UpdateConversation method in Database
			_ = record // Avoid unused variable
			err = nil  // Temporary - no-op
			if err == nil && bpm.cache != nil {
				bpm.cache.Put(record)
			}
		} else {
			err = fmt.Errorf("invalid conversation data type")
		}
		
	default:
		err = fmt.Errorf("unknown update operation type: %s", op.Type)
	}
}

// processBatch processes a batch of save operations
func (bpm *BackgroundPersistenceManager) processBatch(batch []*SaveOperation) {
	if len(batch) == 0 {
		return
	}
	
	startTime := time.Now()
	
	// Begin transaction for batch
	tx, err := bpm.database.db.Begin()
	if err != nil {
		// Process individually on transaction failure
		for _, op := range batch {
			bpm.processSaveOperation(op)
		}
		return
	}
	
	var successCount, failureCount int
	
	// Process each operation in the batch
	for _, op := range batch {
		var opErr error
		
		switch op.Type {
		case "conversation":
			if record, ok := op.Data.(*ConversationRecord); ok {
				opErr = bpm.saveConversationInTx(tx, record)
			}
		case "tool_execution":
			if record, ok := op.Data.(*ToolExecutionRecord); ok {
				opErr = bpm.saveToolExecutionInTx(tx, record)
			}
		case "file_operation":
			if record, ok := op.Data.(*FileOperationRecord); ok {
				opErr = bpm.saveFileOperationInTx(tx, record)
			}
		}
		
		if opErr != nil {
			failureCount++
		} else {
			successCount++
		}
		
		// Call individual callbacks
		if op.Callback != nil {
			go op.Callback(opErr)
		}
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		// Rollback and process individually
		tx.Rollback()
		for _, op := range batch {
			bpm.processSaveOperation(op)
		}
		return
	}
	
	duration := time.Since(startTime)
	bpm.updateMetrics(successCount > failureCount, duration, true)
	
	bpm.metrics.mutex.Lock()
	bpm.metrics.BatchOperations++
	bpm.metrics.mutex.Unlock()
}

// Helper functions for batch processing (simplified versions)
func (bpm *BackgroundPersistenceManager) saveConversationInTx(tx interface{}, record *ConversationRecord) error {
	// Implementation would use the transaction to save the conversation
	// This is a simplified placeholder
	return nil
}

func (bpm *BackgroundPersistenceManager) saveToolExecutionInTx(tx interface{}, record *ToolExecutionRecord) error {
	// Implementation would use the transaction to save the tool execution
	return nil
}

func (bpm *BackgroundPersistenceManager) saveFileOperationInTx(tx interface{}, record *FileOperationRecord) error {
	// Implementation would use the transaction to save the file operation
	return nil
}

// flushWorker periodically flushes pending operations
func (bpm *BackgroundPersistenceManager) flushWorker() {
	defer bpm.workerWg.Done()
	
	ticker := time.NewTicker(bpm.config.FlushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-bpm.shutdownCh:
			return
		case <-ticker.C:
			bpm.flush()
		}
	}
}

// flush forces processing of any pending operations
func (bpm *BackgroundPersistenceManager) flush() {
	// Update metrics
	bpm.metrics.mutex.Lock()
	bpm.metrics.LastFlush = time.Now()
	bpm.metrics.QueueLength = len(bpm.saveQueue) + len(bpm.deleteQueue) + len(bpm.updateQueue)
	bpm.metrics.mutex.Unlock()
}

// metricsCollector collects and updates metrics
func (bpm *BackgroundPersistenceManager) metricsCollector() {
	defer bpm.workerWg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-bpm.shutdownCh:
			return
		case <-ticker.C:
			bpm.collectMetrics()
		}
	}
}

// collectMetrics collects current metrics
func (bpm *BackgroundPersistenceManager) collectMetrics() {
	bpm.metrics.mutex.Lock()
	defer bpm.metrics.mutex.Unlock()
	
	bpm.metrics.QueueLength = len(bpm.saveQueue) + len(bpm.deleteQueue) + len(bpm.updateQueue)
}

// updateMetrics updates performance metrics
func (bpm *BackgroundPersistenceManager) updateMetrics(success bool, duration time.Duration, isBatch bool) {
	bpm.metrics.mutex.Lock()
	defer bpm.metrics.mutex.Unlock()
	
	bpm.metrics.TotalOperations++
	
	if success {
		bpm.metrics.SuccessfulOps++
	} else {
		bpm.metrics.FailedOps++
	}
	
	// Update average process time using exponential moving average
	if bpm.metrics.AverageProcessTime == 0 {
		bpm.metrics.AverageProcessTime = duration
	} else {
		alpha := 0.1
		bpm.metrics.AverageProcessTime = time.Duration(
			float64(bpm.metrics.AverageProcessTime)*(1-alpha) + float64(duration)*alpha,
		)
	}
}

// GetMetrics returns current persistence metrics
func (bpm *BackgroundPersistenceManager) GetMetrics() PersistenceMetrics {
	bpm.metrics.mutex.RLock()
	defer bpm.metrics.mutex.RUnlock()
	
	metrics := *bpm.metrics
	return metrics
}

// GetQueueLength returns the current queue length
func (bpm *BackgroundPersistenceManager) GetQueueLength() int {
	return len(bpm.saveQueue) + len(bpm.deleteQueue) + len(bpm.updateQueue)
}

// IsHealthy checks if the persistence manager is healthy
func (bpm *BackgroundPersistenceManager) IsHealthy() bool {
	bpm.mutex.RLock()
	running := bpm.running
	bpm.mutex.RUnlock()
	
	if !running {
		return false
	}
	
	// Check queue lengths
	queueLength := bpm.GetQueueLength()
	if queueLength > bpm.config.QueueSize*80/100 { // 80% capacity
		return false
	}
	
	// Check success rate
	metrics := bpm.GetMetrics()
	if metrics.TotalOperations > 0 {
		successRate := float64(metrics.SuccessfulOps) / float64(metrics.TotalOperations)
		if successRate < 0.95 { // Less than 95% success rate
			return false
		}
	}
	
	return true
}