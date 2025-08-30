package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// CreateTempDatabase creates a temporary database for testing
func CreateTempDatabase(t *testing.T) (string, func()) {
	t.Helper()
	
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	cleanup := func() {
		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			t.Logf("Failed to cleanup test database %s: %v", dbPath, err)
		}
	}
	
	return dbPath, cleanup
}

// WaitForCondition waits for a condition to be true or times out
func WaitForCondition(condition func() bool, timeout time.Duration, message string) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	return false
}

// AssertEventuallyTrue waits for a condition to become true within a timeout
func AssertEventuallyTrue(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	t.Helper()
	
	if !WaitForCondition(condition, timeout, message) {
		t.Fatalf("Condition did not become true within %v: %s", timeout, message)
	}
}

// RunWithTimeout runs a function with a timeout
func RunWithTimeout(t *testing.T, timeout time.Duration, fn func()) {
	t.Helper()
	
	done := make(chan struct{})
	
	go func() {
		defer close(done)
		fn()
	}()
	
	select {
	case <-done:
		// Function completed successfully
	case <-time.After(timeout):
		t.Fatalf("Function did not complete within %v", timeout)
	}
}

// TestDatabaseConfig holds configuration for test databases
type TestDatabaseConfig struct {
	InMemory     bool
	WALMode      bool
	ForeignKeys  bool
	Synchronous  string
	CacheSize    int
}

// DefaultTestConfig returns a default test database configuration
func DefaultTestConfig() TestDatabaseConfig {
	return TestDatabaseConfig{
		InMemory:    true,
		WALMode:     false, // Disable WAL for in-memory databases
		ForeignKeys: true,
		Synchronous: "OFF", // Faster for tests
		CacheSize:   1000,
	}
}

// FastTestConfig returns a configuration optimized for test speed
func FastTestConfig() TestDatabaseConfig {
	return TestDatabaseConfig{
		InMemory:    true,
		WALMode:     false,
		ForeignKeys: false, // Disable for faster inserts
		Synchronous: "OFF",
		CacheSize:   10000,
	}
}

// ProductionLikeConfig returns a configuration similar to production
func ProductionLikeConfig() TestDatabaseConfig {
	return TestDatabaseConfig{
		InMemory:    false,
		WALMode:     true,
		ForeignKeys: true,
		Synchronous: "NORMAL",
		CacheSize:   1000,
	}
}

// BenchmarkHelper provides utilities for benchmark tests
type BenchmarkHelper struct {
	DataSizes []int
	Iterations []int
}

// NewBenchmarkHelper creates a new benchmark helper
func NewBenchmarkHelper() *BenchmarkHelper {
	return &BenchmarkHelper{
		DataSizes:  []int{10, 100, 1000, 10000},
		Iterations: []int{1, 10, 100, 1000},
	}
}

// RunScalabilityTest runs a test function with different data sizes
func (bh *BenchmarkHelper) RunScalabilityTest(t *testing.T, testFunc func(t *testing.T, dataSize int)) {
	for _, size := range bh.DataSizes {
		t.Run(fmt.Sprintf("DataSize_%d", size), func(t *testing.T) {
			testFunc(t, size)
		})
	}
}

// MemoryUsage provides utilities to measure memory usage during tests
type MemoryUsage struct {
	StartTime time.Time
	PeakRSS   int64
}

// StartMemoryMonitoring begins monitoring memory usage
func StartMemoryMonitoring() *MemoryUsage {
	return &MemoryUsage{
		StartTime: time.Now(),
	}
}

// LogMemoryUsage logs current memory usage
func (mu *MemoryUsage) LogMemoryUsage(t *testing.T, label string) {
	// This is a placeholder - in a real implementation, you'd use
	// runtime.MemStats or external tools to measure actual memory usage
	t.Logf("Memory usage at %s: %s elapsed", label, time.Since(mu.StartTime))
}

// ErrorRecorder helps capture and analyze errors during testing
type ErrorRecorder struct {
	errors []error
}

// NewErrorRecorder creates a new error recorder
func NewErrorRecorder() *ErrorRecorder {
	return &ErrorRecorder{
		errors: make([]error, 0),
	}
}

// Record records an error
func (er *ErrorRecorder) Record(err error) {
	if err != nil {
		er.errors = append(er.errors, err)
	}
}

// HasErrors returns true if any errors were recorded
func (er *ErrorRecorder) HasErrors() bool {
	return len(er.errors) > 0
}

// GetErrors returns all recorded errors
func (er *ErrorRecorder) GetErrors() []error {
	return er.errors
}

// GetErrorCount returns the number of recorded errors
func (er *ErrorRecorder) GetErrorCount() int {
	return len(er.errors)
}

// AssertNoErrors fails the test if any errors were recorded
func (er *ErrorRecorder) AssertNoErrors(t *testing.T) {
	t.Helper()
	
	if len(er.errors) > 0 {
		t.Fatalf("Expected no errors, but got %d errors: %v", len(er.errors), er.errors)
	}
}

// ConcurrentTestRunner helps run tests concurrently with proper synchronization
type ConcurrentTestRunner struct {
	NumWorkers   int
	OperationsPerWorker int
	Timeout      time.Duration
}

// NewConcurrentTestRunner creates a new concurrent test runner
func NewConcurrentTestRunner(workers, opsPerWorker int, timeout time.Duration) *ConcurrentTestRunner {
	return &ConcurrentTestRunner{
		NumWorkers:          workers,
		OperationsPerWorker: opsPerWorker,
		Timeout:            timeout,
	}
}

// Run executes the given function concurrently
func (ctr *ConcurrentTestRunner) Run(t *testing.T, workFunc func(workerID int, opID int) error) {
	t.Helper()
	
	var wg sync.WaitGroup
	errorRecorder := NewErrorRecorder()
	errorChan := make(chan error, ctr.NumWorkers*ctr.OperationsPerWorker)
	
	// Start workers
	for i := 0; i < ctr.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < ctr.OperationsPerWorker; j++ {
				if err := workFunc(workerID, j); err != nil {
					select {
					case errorChan <- err:
					default:
						// Channel is full, skip this error
					}
				}
			}
		}(i)
	}
	
	// Wait for completion with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All workers completed
	case <-time.After(ctr.Timeout):
		t.Fatalf("Concurrent test did not complete within %v", ctr.Timeout)
	}
	
	// Collect errors
	close(errorChan)
	for err := range errorChan {
		errorRecorder.Record(err)
	}
	
	// Assert no errors occurred
	errorRecorder.AssertNoErrors(t)
	
	t.Logf("Concurrent test completed successfully: %d workers × %d operations = %d total operations",
		ctr.NumWorkers, ctr.OperationsPerWorker, ctr.NumWorkers*ctr.OperationsPerWorker)
}