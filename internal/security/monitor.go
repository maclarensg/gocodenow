package security

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ResourceUsage represents current resource usage
type ResourceUsage struct {
	MemoryUsage      int64         `json:"memory_usage"`
	CPUUsage         float64       `json:"cpu_usage"`
	ExecutionTime    time.Duration `json:"execution_time"`
	ActiveOperations int32         `json:"active_operations"`
	TotalOperations  int64         `json:"total_operations"`
	LastUpdated      time.Time     `json:"last_updated"`
}

// ResourceLimits defines resource usage limits
type ResourceLimits struct {
	MaxMemoryUsage      int64         `json:"max_memory_usage"`
	MaxExecutionTime    time.Duration `json:"max_execution_time"`
	MaxConcurrentOps    int32         `json:"max_concurrent_ops"`
	MaxTotalOperations  int64         `json:"max_total_operations"`
	UpdateInterval      time.Duration `json:"update_interval"`
}

// ResourceMonitor monitors and enforces resource usage limits
type ResourceMonitor struct {
	limits           ResourceLimits
	usage            ResourceUsage
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	alertChan        chan ResourceAlert
	activeOperations map[string]*Operation
	startTime        time.Time
}

// ResourceAlert represents a resource usage alert
type ResourceAlert struct {
	Type      AlertType   `json:"type"`
	Message   string      `json:"message"`
	Usage     ResourceUsage `json:"usage"`
	Limits    ResourceLimits `json:"limits"`
	Timestamp time.Time   `json:"timestamp"`
}

// AlertType defines the type of resource alert
type AlertType int

const (
	AlertMemoryLimit AlertType = iota
	AlertExecutionTimeLimit
	AlertConcurrencyLimit
	AlertOperationLimit
	AlertSystemOverload
)

// String returns the string representation of alert type
func (at AlertType) String() string {
	switch at {
	case AlertMemoryLimit:
		return "MEMORY_LIMIT"
	case AlertExecutionTimeLimit:
		return "EXECUTION_TIME_LIMIT"
	case AlertConcurrencyLimit:
		return "CONCURRENCY_LIMIT"
	case AlertOperationLimit:
		return "OPERATION_LIMIT"
	case AlertSystemOverload:
		return "SYSTEM_OVERLOAD"
	default:
		return "UNKNOWN"
	}
}

// Operation represents a monitored operation
type Operation struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	StartTime   time.Time         `json:"start_time"`
	Context     context.Context   `json:"-"`
	Cancel      context.CancelFunc `json:"-"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(limits ResourceLimits) *ResourceMonitor {
	if limits.UpdateInterval == 0 {
		limits.UpdateInterval = 1 * time.Second
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	monitor := &ResourceMonitor{
		limits:           limits,
		ctx:              ctx,
		cancel:           cancel,
		alertChan:        make(chan ResourceAlert, 100),
		activeOperations: make(map[string]*Operation),
		startTime:        time.Now(),
	}
	
	// Start monitoring goroutine
	go monitor.monitorLoop()
	
	return monitor
}

// StartOperation starts monitoring a new operation
func (rm *ResourceMonitor) StartOperation(id, name string, metadata map[string]interface{}) (*Operation, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// Check if we're at the concurrent operation limit
	if rm.limits.MaxConcurrentOps > 0 && int32(len(rm.activeOperations)) >= rm.limits.MaxConcurrentOps {
		return nil, fmt.Errorf("concurrent operation limit reached: %d", rm.limits.MaxConcurrentOps)
	}
	
	// Check total operations limit
	if rm.limits.MaxTotalOperations > 0 && rm.usage.TotalOperations >= rm.limits.MaxTotalOperations {
		return nil, fmt.Errorf("total operation limit reached: %d", rm.limits.MaxTotalOperations)
	}
	
	// Create operation context with timeout
	ctx := rm.ctx
	var cancel context.CancelFunc
	if rm.limits.MaxExecutionTime > 0 {
		ctx, cancel = context.WithTimeout(rm.ctx, rm.limits.MaxExecutionTime)
	} else {
		ctx, cancel = context.WithCancel(rm.ctx)
	}
	
	operation := &Operation{
		ID:        id,
		Name:      name,
		StartTime: time.Now(),
		Context:   ctx,
		Cancel:    cancel,
		Metadata:  metadata,
	}
	
	rm.activeOperations[id] = operation
	atomic.AddInt32(&rm.usage.ActiveOperations, 1)
	atomic.AddInt64(&rm.usage.TotalOperations, 1)
	
	return operation, nil
}

// EndOperation ends monitoring of an operation
func (rm *ResourceMonitor) EndOperation(id string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if op, exists := rm.activeOperations[id]; exists {
		op.Cancel()
		delete(rm.activeOperations, id)
		atomic.AddInt32(&rm.usage.ActiveOperations, -1)
	}
}

// GetUsage returns current resource usage
func (rm *ResourceMonitor) GetUsage() ResourceUsage {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	return rm.usage
}

// GetLimits returns resource limits
func (rm *ResourceMonitor) GetLimits() ResourceLimits {
	return rm.limits
}

// GetActiveOperations returns currently active operations
func (rm *ResourceMonitor) GetActiveOperations() map[string]*Operation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	operations := make(map[string]*Operation)
	for id, op := range rm.activeOperations {
		operations[id] = op
	}
	
	return operations
}

// GetAlerts returns the alert channel
func (rm *ResourceMonitor) GetAlerts() <-chan ResourceAlert {
	return rm.alertChan
}

// CheckLimits checks if current usage exceeds limits
func (rm *ResourceMonitor) CheckLimits() []ResourceAlert {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	var alerts []ResourceAlert
	now := time.Now()
	
	// Check memory limit
	if rm.limits.MaxMemoryUsage > 0 && rm.usage.MemoryUsage > rm.limits.MaxMemoryUsage {
		alerts = append(alerts, ResourceAlert{
			Type:    AlertMemoryLimit,
			Message: fmt.Sprintf("Memory usage %d MB exceeds limit %d MB", 
				rm.usage.MemoryUsage/(1024*1024), rm.limits.MaxMemoryUsage/(1024*1024)),
			Usage:     rm.usage,
			Limits:    rm.limits,
			Timestamp: now,
		})
	}
	
	// Check concurrent operations limit
	if rm.limits.MaxConcurrentOps > 0 && rm.usage.ActiveOperations > rm.limits.MaxConcurrentOps {
		alerts = append(alerts, ResourceAlert{
			Type:    AlertConcurrencyLimit,
			Message: fmt.Sprintf("Active operations %d exceeds limit %d", 
				rm.usage.ActiveOperations, rm.limits.MaxConcurrentOps),
			Usage:     rm.usage,
			Limits:    rm.limits,
			Timestamp: now,
		})
	}
	
	// Check execution time for individual operations
	for id, op := range rm.activeOperations {
		if rm.limits.MaxExecutionTime > 0 {
			elapsed := now.Sub(op.StartTime)
			if elapsed > rm.limits.MaxExecutionTime {
				alerts = append(alerts, ResourceAlert{
					Type:    AlertExecutionTimeLimit,
					Message: fmt.Sprintf("Operation %s (%s) execution time %v exceeds limit %v", 
						id, op.Name, elapsed, rm.limits.MaxExecutionTime),
					Usage:     rm.usage,
					Limits:    rm.limits,
					Timestamp: now,
				})
			}
		}
	}
	
	return alerts
}

// Stop stops the resource monitor
func (rm *ResourceMonitor) Stop() {
	rm.cancel()
	
	// Cancel all active operations
	rm.mu.Lock()
	for _, op := range rm.activeOperations {
		op.Cancel()
	}
	rm.mu.Unlock()
	
	close(rm.alertChan)
}

// monitorLoop runs the main monitoring loop
func (rm *ResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(rm.limits.UpdateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.updateUsage()
			rm.checkAndSendAlerts()
		}
	}
}

// updateUsage updates current resource usage
func (rm *ResourceMonitor) updateUsage() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	rm.mu.Lock()
	rm.usage.MemoryUsage = int64(memStats.Alloc)
	rm.usage.ExecutionTime = time.Since(rm.startTime)
	rm.usage.LastUpdated = time.Now()
	rm.mu.Unlock()
}

// checkAndSendAlerts checks for limit violations and sends alerts
func (rm *ResourceMonitor) checkAndSendAlerts() {
	alerts := rm.CheckLimits()
	
	for _, alert := range alerts {
		select {
		case rm.alertChan <- alert:
		default:
			// Alert channel is full, skip this alert
		}
	}
}

// ResourceTracker provides simple resource tracking for operations
type ResourceTracker struct {
	monitor   *ResourceMonitor
	operation *Operation
}

// NewResourceTracker creates a new resource tracker for an operation
func NewResourceTracker(monitor *ResourceMonitor, operationName string, metadata map[string]interface{}) (*ResourceTracker, error) {
	id := fmt.Sprintf("%s-%d", operationName, time.Now().UnixNano())
	
	operation, err := monitor.StartOperation(id, operationName, metadata)
	if err != nil {
		return nil, err
	}
	
	return &ResourceTracker{
		monitor:   monitor,
		operation: operation,
	}, nil
}

// GetContext returns the operation context (for timeout/cancellation)
func (rt *ResourceTracker) GetContext() context.Context {
	return rt.operation.Context
}

// GetOperation returns the tracked operation
func (rt *ResourceTracker) GetOperation() *Operation {
	return rt.operation
}

// Close ends the resource tracking
func (rt *ResourceTracker) Close() {
	rt.monitor.EndOperation(rt.operation.ID)
}

// WithResourceTracking executes a function with resource tracking
func WithResourceTracking(monitor *ResourceMonitor, operationName string, 
	metadata map[string]interface{}, fn func(context.Context) error) error {
	
	tracker, err := NewResourceTracker(monitor, operationName, metadata)
	if err != nil {
		return err
	}
	defer tracker.Close()
	
	return fn(tracker.GetContext())
}

// DefaultResourceLimits returns sensible default resource limits
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxMemoryUsage:     100 * 1024 * 1024, // 100MB
		MaxExecutionTime:   30 * time.Second,
		MaxConcurrentOps:   3,
		MaxTotalOperations: 1000,
		UpdateInterval:     1 * time.Second,
	}
}

// StrictResourceLimits returns strict resource limits for high-security environments
func StrictResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxMemoryUsage:     50 * 1024 * 1024,  // 50MB
		MaxExecutionTime:   10 * time.Second,
		MaxConcurrentOps:   1,
		MaxTotalOperations: 100,
		UpdateInterval:     500 * time.Millisecond,
	}
}