package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// PerformanceMetricsCollector collects and analyzes system performance metrics
type PerformanceMetricsCollector struct {
	// Configuration
	config           *MetricsConfig
	
	// Data storage
	metrics          map[string]*MetricTimeSeries
	aggregatedMetrics map[string]*AggregatedMetric
	
	// Components to monitor
	database         *Database
	cache            *ConversationCache
	lazyLoader       *LazyConversationLoader
	persistence      *BackgroundPersistenceManager
	memoryMonitor    *MemoryMonitor
	indexManager     *DatabaseIndexManager
	
	// Collection state
	running          bool
	collectInterval  time.Duration
	
	// Event tracking
	operations       []OperationMetric
	queryLog         []QueryMetric
	
	// Synchronization
	mutex            sync.RWMutex
	shutdownCh       chan struct{}
	workerWg         sync.WaitGroup
	
	// Reporting
	reportGenerator  *PerformanceReportGenerator
}

// MetricsConfig defines performance metrics collection configuration
type MetricsConfig struct {
	CollectionInterval    time.Duration `json:"collection_interval"`
	RetentionPeriod      time.Duration `json:"retention_period"`
	MaxDataPoints        int           `json:"max_data_points"`
	EnableQueryLogging   bool          `json:"enable_query_logging"`
	EnableOperationTrace bool          `json:"enable_operation_trace"`
	MetricsStoragePath   string        `json:"metrics_storage_path"`
	EnableReporting      bool          `json:"enable_reporting"`
	ReportInterval       time.Duration `json:"report_interval"`
	AlertThresholds      map[string]float64 `json:"alert_thresholds"`
}

// MetricTimeSeries represents a time series of metric values
type MetricTimeSeries struct {
	Name        string         `json:"name"`
	Unit        string         `json:"unit"`
	DataPoints  []MetricPoint  `json:"data_points"`
	LastUpdated time.Time      `json:"last_updated"`
	mutex       sync.RWMutex
}

// MetricPoint represents a single metric measurement
type MetricPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     float64     `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// AggregatedMetric represents aggregated metric statistics
type AggregatedMetric struct {
	Name       string    `json:"name"`
	Min        float64   `json:"min"`
	Max        float64   `json:"max"`
	Avg        float64   `json:"avg"`
	P50        float64   `json:"p50"`
	P95        float64   `json:"p95"`
	P99        float64   `json:"p99"`
	Count      int64     `json:"count"`
	Sum        float64   `json:"sum"`
	LastUpdate time.Time `json:"last_update"`
}

// OperationMetric tracks individual operation performance
type OperationMetric struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time"`
	Duration    time.Duration     `json:"duration"`
	Success     bool              `json:"success"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// QueryMetric tracks database query performance
type QueryMetric struct {
	Query         string        `json:"query"`
	Duration      time.Duration `json:"duration"`
	RowsAffected  int64         `json:"rows_affected"`
	Success       bool          `json:"success"`
	Error         string        `json:"error,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
	ExecutionPlan string        `json:"execution_plan,omitempty"`
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		CollectionInterval:    30 * time.Second,
		RetentionPeriod:      24 * time.Hour,
		MaxDataPoints:        2880, // 24 hours at 30-second intervals
		EnableQueryLogging:   true,
		EnableOperationTrace: true,
		MetricsStoragePath:   "./metrics",
		EnableReporting:      true,
		ReportInterval:       1 * time.Hour,
		AlertThresholds: map[string]float64{
			"response_time_p95":    1000, // 1 second
			"error_rate":           5,    // 5%
			"memory_usage_percent": 80,   // 80%
			"cache_hit_rate":       90,   // 90%
		},
	}
}

// NewPerformanceMetricsCollector creates a new metrics collector
func NewPerformanceMetricsCollector(config *MetricsConfig) *PerformanceMetricsCollector {
	if config == nil {
		config = DefaultMetricsConfig()
	}
	
	collector := &PerformanceMetricsCollector{
		config:            config,
		metrics:           make(map[string]*MetricTimeSeries),
		aggregatedMetrics: make(map[string]*AggregatedMetric),
		collectInterval:   config.CollectionInterval,
		shutdownCh:        make(chan struct{}),
		operations:        make([]OperationMetric, 0),
		queryLog:          make([]QueryMetric, 0),
	}
	
	// Initialize report generator
	if config.EnableReporting {
		collector.reportGenerator = NewPerformanceReportGenerator(config)
	}
	
	// Create metrics storage directory
	if config.MetricsStoragePath != "" {
		os.MkdirAll(config.MetricsStoragePath, 0755)
	}
	
	return collector
}

// SetComponents sets the components to monitor
func (pmc *PerformanceMetricsCollector) SetComponents(
	database *Database,
	cache *ConversationCache,
	lazyLoader *LazyConversationLoader,
	persistence *BackgroundPersistenceManager,
	memoryMonitor *MemoryMonitor,
	indexManager *DatabaseIndexManager,
) {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	
	pmc.database = database
	pmc.cache = cache
	pmc.lazyLoader = lazyLoader
	pmc.persistence = persistence
	pmc.memoryMonitor = memoryMonitor
	pmc.indexManager = indexManager
}

// Start begins performance metrics collection
func (pmc *PerformanceMetricsCollector) Start() error {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	
	if pmc.running {
		return fmt.Errorf("performance metrics collector is already running")
	}
	
	pmc.running = true
	
	// Start collection worker
	pmc.workerWg.Add(1)
	go pmc.collectionWorker()
	
	// Start aggregation worker
	pmc.workerWg.Add(1)
	go pmc.aggregationWorker()
	
	// Start reporting worker if enabled
	if pmc.config.EnableReporting {
		pmc.workerWg.Add(1)
		go pmc.reportingWorker()
	}
	
	// Start persistence worker
	pmc.workerWg.Add(1)
	go pmc.persistenceWorker()
	
	return nil
}

// Stop stops performance metrics collection
func (pmc *PerformanceMetricsCollector) Stop(ctx context.Context) error {
	pmc.mutex.Lock()
	if !pmc.running {
		pmc.mutex.Unlock()
		return nil
	}
	pmc.running = false
	pmc.mutex.Unlock()
	
	// Signal shutdown
	close(pmc.shutdownCh)
	
	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		pmc.workerWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// collectionWorker periodically collects metrics
func (pmc *PerformanceMetricsCollector) collectionWorker() {
	defer pmc.workerWg.Done()
	
	ticker := time.NewTicker(pmc.collectInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-pmc.shutdownCh:
			return
		case <-ticker.C:
			pmc.collectAllMetrics()
		}
	}
}

// aggregationWorker periodically aggregates metrics
func (pmc *PerformanceMetricsCollector) aggregationWorker() {
	defer pmc.workerWg.Done()
	
	ticker := time.NewTicker(5 * time.Minute) // Aggregate every 5 minutes
	defer ticker.Stop()
	
	for {
		select {
		case <-pmc.shutdownCh:
			return
		case <-ticker.C:
			pmc.aggregateMetrics()
		}
	}
}

// reportingWorker generates periodic performance reports
func (pmc *PerformanceMetricsCollector) reportingWorker() {
	defer pmc.workerWg.Done()
	
	ticker := time.NewTicker(pmc.config.ReportInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-pmc.shutdownCh:
			return
		case <-ticker.C:
			if pmc.reportGenerator != nil {
				pmc.generatePerformanceReport()
			}
		}
	}
}

// persistenceWorker periodically saves metrics to disk
func (pmc *PerformanceMetricsCollector) persistenceWorker() {
	defer pmc.workerWg.Done()
	
	ticker := time.NewTicker(10 * time.Minute) // Save every 10 minutes
	defer ticker.Stop()
	
	for {
		select {
		case <-pmc.shutdownCh:
			// Final save before shutdown
			pmc.saveMetricsToDisk()
			return
		case <-ticker.C:
			pmc.saveMetricsToDisk()
		}
	}
}

// collectAllMetrics collects metrics from all monitored components
func (pmc *PerformanceMetricsCollector) collectAllMetrics() {
	now := time.Now()
	
	// Collect database metrics
	if pmc.database != nil {
		pmc.collectDatabaseMetrics(now)
	}
	
	// Collect cache metrics
	if pmc.cache != nil {
		pmc.collectCacheMetrics(now)
	}
	
	// Collect lazy loader metrics
	if pmc.lazyLoader != nil {
		pmc.collectLazyLoaderMetrics(now)
	}
	
	// Collect persistence metrics
	if pmc.persistence != nil {
		pmc.collectPersistenceMetrics(now)
	}
	
	// Collect memory metrics
	if pmc.memoryMonitor != nil {
		pmc.collectMemoryMetrics(now)
	}
	
	// Collect system metrics
	pmc.collectSystemMetrics(now)
}

// collectDatabaseMetrics collects database performance metrics
func (pmc *PerformanceMetricsCollector) collectDatabaseMetrics(timestamp time.Time) {
	if pmc.indexManager != nil {
		// Get database size
		if size, err := pmc.indexManager.GetDatabaseSize(); err == nil {
			pmc.recordMetric("database_size_bytes", float64(size), timestamp, nil)
		}
		
		// Get table statistics
		if tableStats, err := pmc.indexManager.GetTableStats(); err == nil {
			for tableName, stats := range tableStats {
				labels := map[string]string{"table": tableName}
				pmc.recordMetric("table_row_count", float64(stats.RowCount), timestamp, labels)
				pmc.recordMetric("table_index_count", float64(stats.IndexCount), timestamp, labels)
			}
		}
	}
}

// collectCacheMetrics collects cache performance metrics
func (pmc *PerformanceMetricsCollector) collectCacheMetrics(timestamp time.Time) {
	stats := pmc.cache.GetStats()
	
	pmc.recordMetric("cache_hits", float64(stats.Hits), timestamp, nil)
	pmc.recordMetric("cache_misses", float64(stats.Misses), timestamp, nil)
	pmc.recordMetric("cache_evictions", float64(stats.Evictions), timestamp, nil)
	pmc.recordMetric("cache_memory_usage", float64(stats.MemoryUsage), timestamp, nil)
	pmc.recordMetric("cache_hit_rate", stats.HitRate*100, timestamp, nil)
	pmc.recordMetric("cache_conversation_count", float64(stats.ConversationCount), timestamp, nil)
	
	if stats.AverageAccessTime > 0 {
		pmc.recordMetric("cache_access_time_ms", float64(stats.AverageAccessTime.Nanoseconds())/1e6, timestamp, nil)
	}
}

// collectLazyLoaderMetrics collects lazy loader performance metrics
func (pmc *PerformanceMetricsCollector) collectLazyLoaderMetrics(timestamp time.Time) {
	metrics := pmc.lazyLoader.GetMetrics()
	
	pmc.recordMetric("lazy_loader_total_requests", float64(metrics.TotalRequests), timestamp, nil)
	pmc.recordMetric("lazy_loader_cache_hits", float64(metrics.CacheHits), timestamp, nil)
	pmc.recordMetric("lazy_loader_cache_misses", float64(metrics.CacheMisses), timestamp, nil)
	pmc.recordMetric("lazy_loader_pending_requests", float64(metrics.PendingRequests), timestamp, nil)
	pmc.recordMetric("lazy_loader_active_loads", float64(metrics.ActiveLoads), timestamp, nil)
	pmc.recordMetric("lazy_loader_error_count", float64(metrics.ErrorCount), timestamp, nil)
	
	if metrics.AverageLoadTime > 0 {
		pmc.recordMetric("lazy_loader_avg_load_time_ms", float64(metrics.AverageLoadTime.Nanoseconds())/1e6, timestamp, nil)
	}
}

// collectPersistenceMetrics collects background persistence metrics
func (pmc *PerformanceMetricsCollector) collectPersistenceMetrics(timestamp time.Time) {
	metrics := pmc.persistence.GetMetrics()
	
	pmc.recordMetric("persistence_total_operations", float64(metrics.TotalOperations), timestamp, nil)
	pmc.recordMetric("persistence_successful_ops", float64(metrics.SuccessfulOps), timestamp, nil)
	pmc.recordMetric("persistence_failed_ops", float64(metrics.FailedOps), timestamp, nil)
	pmc.recordMetric("persistence_queue_length", float64(metrics.QueueLength), timestamp, nil)
	pmc.recordMetric("persistence_batch_operations", float64(metrics.BatchOperations), timestamp, nil)
	pmc.recordMetric("persistence_retry_operations", float64(metrics.RetryOperations), timestamp, nil)
	
	if metrics.AverageProcessTime > 0 {
		pmc.recordMetric("persistence_avg_process_time_ms", float64(metrics.AverageProcessTime.Nanoseconds())/1e6, timestamp, nil)
	}
	
	// Calculate success rate
	if metrics.TotalOperations > 0 {
		successRate := float64(metrics.SuccessfulOps) / float64(metrics.TotalOperations) * 100
		pmc.recordMetric("persistence_success_rate", successRate, timestamp, nil)
	}
}

// collectMemoryMetrics collects memory usage metrics
func (pmc *PerformanceMetricsCollector) collectMemoryMetrics(timestamp time.Time) {
	metrics := pmc.memoryMonitor.GetMetrics()
	
	pmc.recordMetric("memory_heap_in_use", float64(metrics.HeapInUse), timestamp, nil)
	pmc.recordMetric("memory_heap_idle", float64(metrics.HeapIdle), timestamp, nil)
	pmc.recordMetric("memory_stack_in_use", float64(metrics.StackInUse), timestamp, nil)
	pmc.recordMetric("memory_total_alloc", float64(metrics.TotalAlloc), timestamp, nil)
	pmc.recordMetric("memory_gc_count", float64(metrics.GCCount), timestamp, nil)
	pmc.recordMetric("memory_pressure", metrics.MemoryPressure, timestamp, nil)
	pmc.recordMetric("memory_gc_efficiency", metrics.GCEfficiency, timestamp, nil)
	pmc.recordMetric("memory_cache_usage", float64(metrics.CacheMemory), timestamp, nil)
	
	if metrics.GCPauseTotal > 0 && metrics.GCCount > 0 {
		avgPause := float64(metrics.GCPauseTotal.Nanoseconds()) / float64(metrics.GCCount) / 1e6
		pmc.recordMetric("memory_avg_gc_pause_ms", avgPause, timestamp, nil)
	}
}

// collectSystemMetrics collects system-level performance metrics
func (pmc *PerformanceMetricsCollector) collectSystemMetrics(timestamp time.Time) {
	// Collect goroutine count
	pmc.recordMetric("system_goroutines", float64(runtime.NumGoroutine()), timestamp, nil)
	
	// Collect CPU count
	pmc.recordMetric("system_cpu_count", float64(runtime.NumCPU()), timestamp, nil)
}

// recordMetric records a metric value with timestamp
func (pmc *PerformanceMetricsCollector) recordMetric(name string, value float64, timestamp time.Time, labels map[string]string) {
	pmc.mutex.Lock()
	defer pmc.mutex.Unlock()
	
	// Get or create metric time series
	series, exists := pmc.metrics[name]
	if !exists {
		series = &MetricTimeSeries{
			Name:       name,
			DataPoints: make([]MetricPoint, 0),
		}
		pmc.metrics[name] = series
	}
	
	series.mutex.Lock()
	defer series.mutex.Unlock()
	
	// Add new data point
	point := MetricPoint{
		Timestamp: timestamp,
		Value:     value,
		Labels:    labels,
	}
	
	series.DataPoints = append(series.DataPoints, point)
	series.LastUpdated = timestamp
	
	// Maintain max data points limit
	if len(series.DataPoints) > pmc.config.MaxDataPoints {
		series.DataPoints = series.DataPoints[1:]
	}
}

// aggregateMetrics calculates aggregated statistics for all metrics
func (pmc *PerformanceMetricsCollector) aggregateMetrics() {
	pmc.mutex.RLock()
	defer pmc.mutex.RUnlock()
	
	for name, series := range pmc.metrics {
		pmc.aggregateMetricSeries(name, series)
	}
}

// aggregateMetricSeries calculates aggregated statistics for a metric series
func (pmc *PerformanceMetricsCollector) aggregateMetricSeries(name string, series *MetricTimeSeries) {
	series.mutex.RLock()
	points := make([]MetricPoint, len(series.DataPoints))
	copy(points, series.DataPoints)
	series.mutex.RUnlock()
	
	if len(points) == 0 {
		return
	}
	
	// Calculate basic statistics
	var sum, min, max float64
	values := make([]float64, len(points))
	
	for i, point := range points {
		values[i] = point.Value
		sum += point.Value
		
		if i == 0 || point.Value < min {
			min = point.Value
		}
		if i == 0 || point.Value > max {
			max = point.Value
		}
	}
	
	avg := sum / float64(len(points))
	
	// Calculate percentiles
	sort.Float64s(values)
	p50 := percentile(values, 0.5)
	p95 := percentile(values, 0.95)
	p99 := percentile(values, 0.99)
	
	// Store aggregated metrics
	pmc.aggregatedMetrics[name] = &AggregatedMetric{
		Name:       name,
		Min:        min,
		Max:        max,
		Avg:        avg,
		P50:        p50,
		P95:        p95,
		P99:        p99,
		Count:      int64(len(points)),
		Sum:        sum,
		LastUpdate: time.Now(),
	}
}

// percentile calculates the percentile value from sorted values
func percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	
	if p <= 0 {
		return sortedValues[0]
	}
	if p >= 1 {
		return sortedValues[len(sortedValues)-1]
	}
	
	index := p * float64(len(sortedValues)-1)
	lower := int(index)
	upper := lower + 1
	
	if upper >= len(sortedValues) {
		return sortedValues[lower]
	}
	
	weight := index - float64(lower)
	return sortedValues[lower]*(1-weight) + sortedValues[upper]*weight
}

// RecordOperation records the start of an operation for tracking
func (pmc *PerformanceMetricsCollector) RecordOperation(operationType string, metadata map[string]interface{}, tags map[string]string) *OperationTracker {
	if !pmc.config.EnableOperationTrace {
		return &OperationTracker{} // Return dummy tracker
	}
	
	operation := OperationMetric{
		ID:        fmt.Sprintf("%s_%d", operationType, time.Now().UnixNano()),
		Type:      operationType,
		StartTime: time.Now(),
		Success:   false,
		Metadata:  metadata,
		Tags:      tags,
	}
	
	return &OperationTracker{
		collector: pmc,
		operation: operation,
	}
}

// OperationTracker tracks individual operation performance
type OperationTracker struct {
	collector *PerformanceMetricsCollector
	operation OperationMetric
}

// Finish marks an operation as completed
func (ot *OperationTracker) Finish(success bool, err error) {
	if ot.collector == nil {
		return
	}
	
	ot.operation.EndTime = time.Now()
	ot.operation.Duration = ot.operation.EndTime.Sub(ot.operation.StartTime)
	ot.operation.Success = success
	
	if err != nil {
		ot.operation.Error = err.Error()
	}
	
	ot.collector.mutex.Lock()
	ot.collector.operations = append(ot.collector.operations, ot.operation)
	
	// Maintain operation log size
	if len(ot.collector.operations) > 10000 {
		ot.collector.operations = ot.collector.operations[1000:] // Keep last 9000
	}
	ot.collector.mutex.Unlock()
	
	// Record operation metrics
	labels := map[string]string{
		"operation_type": ot.operation.Type,
		"success":        fmt.Sprintf("%t", success),
	}
	
	for k, v := range ot.operation.Tags {
		labels[k] = v
	}
	
	ot.collector.recordMetric("operation_duration_ms", float64(ot.operation.Duration.Nanoseconds())/1e6, ot.operation.EndTime, labels)
	ot.collector.recordMetric("operation_count", 1, ot.operation.EndTime, labels)
}

// RecordQuery records a database query for performance tracking
func (pmc *PerformanceMetricsCollector) RecordQuery(query string, duration time.Duration, rowsAffected int64, success bool, err error) {
	if !pmc.config.EnableQueryLogging {
		return
	}
	
	queryMetric := QueryMetric{
		Query:        query,
		Duration:     duration,
		RowsAffected: rowsAffected,
		Success:      success,
		Timestamp:    time.Now(),
	}
	
	if err != nil {
		queryMetric.Error = err.Error()
	}
	
	pmc.mutex.Lock()
	pmc.queryLog = append(pmc.queryLog, queryMetric)
	
	// Maintain query log size
	if len(pmc.queryLog) > 5000 {
		pmc.queryLog = pmc.queryLog[1000:] // Keep last 4000
	}
	pmc.mutex.Unlock()
	
	// Record query metrics
	labels := map[string]string{
		"success": fmt.Sprintf("%t", success),
	}
	
	pmc.recordMetric("query_duration_ms", float64(duration.Nanoseconds())/1e6, time.Now(), labels)
	pmc.recordMetric("query_rows_affected", float64(rowsAffected), time.Now(), labels)
}

// GetMetrics returns current metric values
func (pmc *PerformanceMetricsCollector) GetMetrics() map[string]*MetricTimeSeries {
	pmc.mutex.RLock()
	defer pmc.mutex.RUnlock()
	
	// Create a deep copy to avoid data races
	result := make(map[string]*MetricTimeSeries)
	for name, series := range pmc.metrics {
		series.mutex.RLock()
		result[name] = &MetricTimeSeries{
			Name:        series.Name,
			Unit:        series.Unit,
			DataPoints:  make([]MetricPoint, len(series.DataPoints)),
			LastUpdated: series.LastUpdated,
		}
		copy(result[name].DataPoints, series.DataPoints)
		series.mutex.RUnlock()
	}
	
	return result
}

// GetAggregatedMetrics returns aggregated metric statistics
func (pmc *PerformanceMetricsCollector) GetAggregatedMetrics() map[string]*AggregatedMetric {
	pmc.mutex.RLock()
	defer pmc.mutex.RUnlock()
	
	// Create a copy
	result := make(map[string]*AggregatedMetric)
	for name, metric := range pmc.aggregatedMetrics {
		copy := *metric
		result[name] = &copy
	}
	
	return result
}

// saveMetricsToDisk persists metrics to disk for persistence
func (pmc *PerformanceMetricsCollector) saveMetricsToDisk() {
	if pmc.config.MetricsStoragePath == "" {
		return
	}
	
	// Save current metrics
	metricsFile := filepath.Join(pmc.config.MetricsStoragePath, fmt.Sprintf("metrics_%s.json", time.Now().Format("2006-01-02")))
	metrics := pmc.GetMetrics()
	
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return
	}
	
	os.WriteFile(metricsFile, data, 0644)
	
	// Save aggregated metrics
	aggFile := filepath.Join(pmc.config.MetricsStoragePath, fmt.Sprintf("aggregated_%s.json", time.Now().Format("2006-01-02")))
	aggMetrics := pmc.GetAggregatedMetrics()
	
	data, err = json.MarshalIndent(aggMetrics, "", "  ")
	if err != nil {
		return
	}
	
	os.WriteFile(aggFile, data, 0644)
}

// generatePerformanceReport generates a comprehensive performance report
func (pmc *PerformanceMetricsCollector) generatePerformanceReport() {
	if pmc.reportGenerator == nil {
		return
	}
	
	report := pmc.reportGenerator.GenerateReport(pmc)
	
	// Save report to file
	if pmc.config.MetricsStoragePath != "" {
		reportFile := filepath.Join(pmc.config.MetricsStoragePath, fmt.Sprintf("performance_report_%s.json", time.Now().Format("2006-01-02_15-04-05")))
		
		data, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			os.WriteFile(reportFile, data, 0644)
		}
	}
}

// CheckAlerts checks current metrics against alert thresholds
func (pmc *PerformanceMetricsCollector) CheckAlerts() []PerformanceAlert {
	var alerts []PerformanceAlert
	
	aggregated := pmc.GetAggregatedMetrics()
	
	for metricName, threshold := range pmc.config.AlertThresholds {
		if metric, exists := aggregated[metricName]; exists {
			if metric.P95 > threshold {
				alerts = append(alerts, PerformanceAlert{
					MetricName:  metricName,
					Value:       metric.P95,
					Threshold:   threshold,
					Severity:    "warning",
					Description: fmt.Sprintf("%s P95 (%.2f) exceeds threshold (%.2f)", metricName, metric.P95, threshold),
					Timestamp:   time.Now(),
				})
			}
		}
	}
	
	return alerts
}

// PerformanceAlert represents a performance alert
type PerformanceAlert struct {
	MetricName  string    `json:"metric_name"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

// PerformanceReportGenerator generates comprehensive performance reports
type PerformanceReportGenerator struct {
	config *MetricsConfig
}

// NewPerformanceReportGenerator creates a new report generator
func NewPerformanceReportGenerator(config *MetricsConfig) *PerformanceReportGenerator {
	return &PerformanceReportGenerator{
		config: config,
	}
}

// GenerateReport generates a comprehensive performance report
func (prg *PerformanceReportGenerator) GenerateReport(collector *PerformanceMetricsCollector) *PerformanceReport {
	aggregated := collector.GetAggregatedMetrics()
	alerts := collector.CheckAlerts()
	
	report := &PerformanceReport{
		GeneratedAt:        time.Now(),
		ReportPeriod:       prg.config.RetentionPeriod,
		AggregatedMetrics:  aggregated,
		Alerts:            alerts,
		Summary:           prg.generateSummary(aggregated, alerts),
		Recommendations:   prg.generateRecommendations(aggregated, alerts),
	}
	
	return report
}

// PerformanceReport represents a comprehensive performance report
type PerformanceReport struct {
	GeneratedAt        time.Time                    `json:"generated_at"`
	ReportPeriod       time.Duration                `json:"report_period"`
	AggregatedMetrics  map[string]*AggregatedMetric `json:"aggregated_metrics"`
	Alerts            []PerformanceAlert           `json:"alerts"`
	Summary           ReportSummary                `json:"summary"`
	Recommendations   []string                     `json:"recommendations"`
}

// ReportSummary contains high-level performance summary
type ReportSummary struct {
	OverallHealthScore float64 `json:"overall_health_score"`
	CriticalIssues     int     `json:"critical_issues"`
	Warnings           int     `json:"warnings"`
	TopBottlenecks     []string `json:"top_bottlenecks"`
}

// generateSummary generates a high-level performance summary
func (prg *PerformanceReportGenerator) generateSummary(metrics map[string]*AggregatedMetric, alerts []PerformanceAlert) ReportSummary {
	var criticalIssues, warnings int
	var bottlenecks []string
	
	for _, alert := range alerts {
		switch alert.Severity {
		case "critical":
			criticalIssues++
		case "warning":
			warnings++
		}
	}
	
	// Calculate overall health score (0-100)
	healthScore := 100.0
	healthScore -= float64(criticalIssues * 20) // -20 points per critical issue
	healthScore -= float64(warnings * 5)       // -5 points per warning
	
	if healthScore < 0 {
		healthScore = 0
	}
	
	// Identify bottlenecks
	if metric, exists := metrics["persistence_avg_process_time_ms"]; exists && metric.P95 > 1000 {
		bottlenecks = append(bottlenecks, "Slow persistence operations")
	}
	if metric, exists := metrics["cache_hit_rate"]; exists && metric.Avg < 80 {
		bottlenecks = append(bottlenecks, "Low cache hit rate")
	}
	if metric, exists := metrics["memory_pressure"]; exists && metric.Avg > 80 {
		bottlenecks = append(bottlenecks, "High memory pressure")
	}
	
	return ReportSummary{
		OverallHealthScore: healthScore,
		CriticalIssues:     criticalIssues,
		Warnings:          warnings,
		TopBottlenecks:    bottlenecks,
	}
}

// generateRecommendations generates performance improvement recommendations
func (prg *PerformanceReportGenerator) generateRecommendations(metrics map[string]*AggregatedMetric, alerts []PerformanceAlert) []string {
	var recommendations []string
	
	// Analyze cache performance
	if metric, exists := metrics["cache_hit_rate"]; exists && metric.Avg < 90 {
		recommendations = append(recommendations, "Consider increasing cache size or improving cache key strategies to improve hit rate")
	}
	
	// Analyze memory usage
	if metric, exists := metrics["memory_pressure"]; exists && metric.P95 > 85 {
		recommendations = append(recommendations, "High memory pressure detected. Consider increasing memory limits or enabling more aggressive cleanup")
	}
	
	// Analyze persistence performance
	if metric, exists := metrics["persistence_queue_length"]; exists && metric.P95 > 1000 {
		recommendations = append(recommendations, "High persistence queue length. Consider increasing worker count or batch size")
	}
	
	// Analyze query performance
	if metric, exists := metrics["query_duration_ms"]; exists && metric.P95 > 100 {
		recommendations = append(recommendations, "Slow database queries detected. Consider adding indexes or optimizing query patterns")
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "System performance appears optimal")
	}
	
	return recommendations
}