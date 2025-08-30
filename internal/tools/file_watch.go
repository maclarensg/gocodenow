// Package tools provides file watching and monitoring capabilities
package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher provides real-time file monitoring capabilities
type FileWatcher struct {
	watcher     *fsnotify.Watcher
	watches     map[string]*WatchInfo
	mu          sync.RWMutex
	eventBuffer chan FileEvent
	stopChan    chan struct{}
	callbacks   map[string][]FileEventCallback
	running     bool
}

// WatchInfo contains information about a watched path
type WatchInfo struct {
	Path        string            `json:"path"`
	IsRecursive bool              `json:"is_recursive"`
	Filters     []string          `json:"filters,omitempty"`     // File extensions to watch
	ExcludePatterns []string      `json:"exclude_patterns,omitempty"`
	AddedAt     time.Time         `json:"added_at"`
	LastEvent   *time.Time        `json:"last_event,omitempty"`
	EventCount  int               `json:"event_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// FileEvent represents a file system event
type FileEvent struct {
	Type      EventType         `json:"type"`
	Path      string            `json:"path"`
	OldPath   string            `json:"old_path,omitempty"` // For rename events
	Timestamp time.Time         `json:"timestamp"`
	Size      int64             `json:"size,omitempty"`
	Mode      os.FileMode       `json:"mode,omitempty"`
	IsDir     bool              `json:"is_dir"`
	Error     error             `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// EventType represents the type of file event
type EventType string

const (
	EventCreate EventType = "create"
	EventWrite  EventType = "write"
	EventRemove EventType = "remove"
	EventRename EventType = "rename"
	EventChmod  EventType = "chmod"
	EventError  EventType = "error"
)

// FileEventCallback is a function that handles file events
type FileEventCallback func(event FileEvent)

// WatchOptions configures file watching behavior
type WatchOptions struct {
	Recursive       bool              `json:"recursive"`
	Filters         []string          `json:"filters,omitempty"`
	ExcludePatterns []string          `json:"exclude_patterns,omitempty"`
	BufferSize      int               `json:"buffer_size"`
	Debounce        time.Duration     `json:"debounce"`
	FollowSymlinks  bool              `json:"follow_symlinks"`
	InitialScan     bool              `json:"initial_scan"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(bufferSize int) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %v", err)
	}

	if bufferSize <= 0 {
		bufferSize = 100
	}

	return &FileWatcher{
		watcher:     watcher,
		watches:     make(map[string]*WatchInfo),
		eventBuffer: make(chan FileEvent, bufferSize),
		stopChan:    make(chan struct{}),
		callbacks:   make(map[string][]FileEventCallback),
		running:     false,
	}, nil
}

// Start begins watching for file events
func (fw *FileWatcher) Start(ctx context.Context) error {
	fw.mu.Lock()
	if fw.running {
		fw.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	fw.running = true
	fw.mu.Unlock()

	// Start event processor
	go fw.processEvents(ctx)

	// Start watcher loop
	go fw.watchLoop(ctx)

	return nil
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if !fw.running {
		return fmt.Errorf("watcher not running")
	}

	close(fw.stopChan)
	fw.running = false

	return fw.watcher.Close()
}

// AddWatch adds a path to watch
func (fw *FileWatcher) AddWatch(path string, opts WatchOptions) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %v", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %v", err)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Check if already watching
	if _, exists := fw.watches[absPath]; exists {
		return fmt.Errorf("already watching path: %s", absPath)
	}

	// Add to watcher
	if err := fw.watcher.Add(absPath); err != nil {
		return fmt.Errorf("failed to add watch: %v", err)
	}

	// Store watch info
	watchInfo := &WatchInfo{
		Path:            absPath,
		IsRecursive:     opts.Recursive,
		Filters:         opts.Filters,
		ExcludePatterns: opts.ExcludePatterns,
		AddedAt:         time.Now(),
		EventCount:      0,
		Metadata:        opts.Metadata,
	}
	fw.watches[absPath] = watchInfo

	// If recursive and directory, add subdirectories
	if opts.Recursive && info.IsDir() {
		if err := fw.addRecursiveWatches(absPath, opts); err != nil {
			return fmt.Errorf("failed to add recursive watches: %v", err)
		}
	}

	// Perform initial scan if requested
	if opts.InitialScan {
		go fw.performInitialScan(absPath, info.IsDir(), opts)
	}

	return nil
}

// RemoveWatch removes a watch
func (fw *FileWatcher) RemoveWatch(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %v", err)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()

	watchInfo, exists := fw.watches[absPath]
	if !exists {
		return fmt.Errorf("not watching path: %s", absPath)
	}

	// Remove from watcher
	if err := fw.watcher.Remove(absPath); err != nil {
		return fmt.Errorf("failed to remove watch: %v", err)
	}

	// Remove recursive watches if applicable
	if watchInfo.IsRecursive {
		fw.removeRecursiveWatches(absPath)
	}

	delete(fw.watches, absPath)
	return nil
}

// RegisterCallback registers a callback for specific path patterns
func (fw *FileWatcher) RegisterCallback(pattern string, callback FileEventCallback) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.callbacks[pattern] == nil {
		fw.callbacks[pattern] = make([]FileEventCallback, 0)
	}
	fw.callbacks[pattern] = append(fw.callbacks[pattern], callback)
}

// GetWatches returns all active watches
func (fw *FileWatcher) GetWatches() map[string]*WatchInfo {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	watches := make(map[string]*WatchInfo)
	for k, v := range fw.watches {
		watches[k] = v
	}
	return watches
}

// GetEvents returns the event channel for consuming events
func (fw *FileWatcher) GetEvents() <-chan FileEvent {
	return fw.eventBuffer
}

// addRecursiveWatches adds watches for all subdirectories
func (fw *FileWatcher) addRecursiveWatches(root string, opts WatchOptions) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() && path != root {
			// Check if should exclude
			if fw.shouldExclude(path, opts.ExcludePatterns) {
				return filepath.SkipDir
			}

			// Add watch
			if err := fw.watcher.Add(path); err == nil {
				fw.watches[path] = &WatchInfo{
					Path:            path,
					IsRecursive:     false, // Sub-watches are not marked as recursive
					Filters:         opts.Filters,
					ExcludePatterns: opts.ExcludePatterns,
					AddedAt:         time.Now(),
					EventCount:      0,
				}
			}
		}
		return nil
	})
}

// removeRecursiveWatches removes watches for subdirectories
func (fw *FileWatcher) removeRecursiveWatches(root string) {
	for path := range fw.watches {
		if path != root && filepath.HasPrefix(path, root) {
			fw.watcher.Remove(path)
			delete(fw.watches, path)
		}
	}
}

// watchLoop processes fsnotify events
func (fw *FileWatcher) watchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-fw.stopChan:
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleFsNotifyEvent(event)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.handleError(err)
		}
	}
}

// handleFsNotifyEvent converts fsnotify events to FileEvents
func (fw *FileWatcher) handleFsNotifyEvent(event fsnotify.Event) {
	// Determine event type
	var eventType EventType
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = EventCreate
	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = EventWrite
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = EventRemove
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = EventRename
	case event.Op&fsnotify.Chmod == fsnotify.Chmod:
		eventType = EventChmod
	default:
		return // Unknown event
	}

	// Get file info
	var size int64
	var mode os.FileMode
	var isDir bool
	
	if info, err := os.Stat(event.Name); err == nil {
		size = info.Size()
		mode = info.Mode()
		isDir = info.IsDir()
	}

	// Create file event
	fileEvent := FileEvent{
		Type:      eventType,
		Path:      event.Name,
		Timestamp: time.Now(),
		Size:      size,
		Mode:      mode,
		IsDir:     isDir,
	}

	// Check if should filter
	fw.mu.RLock()
	shouldSend := fw.shouldSendEvent(event.Name)
	fw.mu.RUnlock()

	if shouldSend {
		// Update watch info
		fw.updateWatchInfo(event.Name)
		
		// Send event
		select {
		case fw.eventBuffer <- fileEvent:
		default:
			// Buffer full, drop event
		}
	}

	// Handle new directories for recursive watches
	if eventType == EventCreate && isDir {
		fw.handleNewDirectory(event.Name)
	}
}

// handleError handles watcher errors
func (fw *FileWatcher) handleError(err error) {
	fileEvent := FileEvent{
		Type:      EventError,
		Timestamp: time.Now(),
		Error:     err,
	}

	select {
	case fw.eventBuffer <- fileEvent:
	default:
		// Buffer full, drop event
	}
}

// shouldSendEvent checks if an event should be sent based on filters
func (fw *FileWatcher) shouldSendEvent(path string) bool {
	// Find applicable watch
	var watchInfo *WatchInfo
	for watchPath, info := range fw.watches {
		if path == watchPath || filepath.HasPrefix(path, watchPath) {
			watchInfo = info
			break
		}
	}

	if watchInfo == nil {
		return false
	}

	// Check exclude patterns
	if fw.shouldExclude(path, watchInfo.ExcludePatterns) {
		return false
	}

	// Check filters
	if len(watchInfo.Filters) > 0 {
		ext := filepath.Ext(path)
		found := false
		for _, filter := range watchInfo.Filters {
			if ext == filter || ext == "."+filter {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// shouldExclude checks if a path should be excluded
func (fw *FileWatcher) shouldExclude(path string, patterns []string) bool {
	base := filepath.Base(path)
	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, base)
		if matched {
			return true
		}
	}
	return false
}

// updateWatchInfo updates watch statistics
func (fw *FileWatcher) updateWatchInfo(path string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	for watchPath, info := range fw.watches {
		if path == watchPath || filepath.HasPrefix(path, watchPath) {
			now := time.Now()
			info.LastEvent = &now
			info.EventCount++
			break
		}
	}
}

// handleNewDirectory handles new directory creation for recursive watches
func (fw *FileWatcher) handleNewDirectory(path string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Find parent watch
	var parentWatch *WatchInfo
	for watchPath, info := range fw.watches {
		if filepath.HasPrefix(path, watchPath) && info.IsRecursive {
			parentWatch = info
			break
		}
	}

	if parentWatch != nil {
		// Add watch for new directory
		if err := fw.watcher.Add(path); err == nil {
			fw.watches[path] = &WatchInfo{
				Path:            path,
				IsRecursive:     false,
				Filters:         parentWatch.Filters,
				ExcludePatterns: parentWatch.ExcludePatterns,
				AddedAt:         time.Now(),
				EventCount:      0,
			}
		}
	}
}

// processEvents processes and dispatches events to callbacks
func (fw *FileWatcher) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-fw.stopChan:
			return
		case event := <-fw.eventBuffer:
			fw.dispatchEvent(event)
		}
	}
}

// dispatchEvent dispatches an event to registered callbacks
func (fw *FileWatcher) dispatchEvent(event FileEvent) {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	for pattern, callbacks := range fw.callbacks {
		matched, _ := filepath.Match(pattern, event.Path)
		if matched || pattern == "*" {
			for _, callback := range callbacks {
				go callback(event) // Execute callbacks asynchronously
			}
		}
	}
}

// performInitialScan performs an initial scan of the watched path
func (fw *FileWatcher) performInitialScan(path string, isDir bool, opts WatchOptions) {
	if !isDir {
		// Single file
		if info, err := os.Stat(path); err == nil {
			event := FileEvent{
				Type:      EventCreate,
				Path:      path,
				Timestamp: time.Now(),
				Size:      info.Size(),
				Mode:      info.Mode(),
				IsDir:     false,
				Metadata:  map[string]interface{}{"initial_scan": true},
			}
			fw.eventBuffer <- event
		}
		return
	}

	// Directory scan
	filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if fw.shouldExclude(filePath, opts.ExcludePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		event := FileEvent{
			Type:      EventCreate,
			Path:      filePath,
			Timestamp: time.Now(),
			Size:      info.Size(),
			Mode:      info.Mode(),
			IsDir:     info.IsDir(),
			Metadata:  map[string]interface{}{"initial_scan": true},
		}

		select {
		case fw.eventBuffer <- event:
		default:
			// Buffer full
		}

		return nil
	})
}

// FileWatchExecutor provides file watching as a tool executor
type FileWatchExecutor struct {
	watchers map[string]*FileWatcher
	mu       sync.RWMutex
}

// NewFileWatchExecutor creates a new file watch executor
func NewFileWatchExecutor() *FileWatchExecutor {
	return &FileWatchExecutor{
		watchers: make(map[string]*FileWatcher),
	}
}

// Execute starts or manages file watching
func (e *FileWatchExecutor) Execute(params map[string]interface{}) (*ToolResult, error) {
	action, _ := params["action"].(string)
	if action == "" {
		action = "start"
	}

	switch action {
	case "start":
		return e.startWatch(params)
	case "stop":
		return e.stopWatch(params)
	case "list":
		return e.listWatches(params)
	case "events":
		return e.getEvents(params)
	default:
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("unknown action: %s", action),
		}, fmt.Errorf("unknown action: %s", action)
	}
}

// startWatch starts watching a path
func (e *FileWatchExecutor) startWatch(params map[string]interface{}) (*ToolResult, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "path parameter required",
		}, fmt.Errorf("path parameter required")
	}

	watcherId, _ := params["watcher_id"].(string)
	if watcherId == "" {
		watcherId = fmt.Sprintf("watch_%d", time.Now().Unix())
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Check if watcher already exists
	if _, exists := e.watchers[watcherId]; exists {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("watcher %s already exists", watcherId),
		}, fmt.Errorf("watcher already exists")
	}

	// Create watcher
	watcher, err := NewFileWatcher(100)
	if err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to create watcher: %v", err),
		}, err
	}

	// Configure options
	opts := WatchOptions{
		Recursive:   true,
		BufferSize:  100,
		InitialScan: false,
	}

	if recursive, ok := params["recursive"].(bool); ok {
		opts.Recursive = recursive
	}

	if filters, ok := params["filters"].([]interface{}); ok {
		opts.Filters = make([]string, len(filters))
		for i, f := range filters {
			opts.Filters[i] = f.(string)
		}
	}

	// Add watch
	if err := watcher.AddWatch(path, opts); err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to add watch: %v", err),
		}, err
	}

	// Start watcher
	if err := watcher.Start(context.Background()); err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to start watcher: %v", err),
		}, err
	}

	e.watchers[watcherId] = watcher

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"watcher_id": watcherId,
			"path":       path,
			"status":     "watching",
		},
		Metadata: map[string]interface{}{
			"action": "start",
		},
	}, nil
}

// stopWatch stops a watcher
func (e *FileWatchExecutor) stopWatch(params map[string]interface{}) (*ToolResult, error) {
	watcherId, _ := params["watcher_id"].(string)
	if watcherId == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "watcher_id parameter required",
		}, fmt.Errorf("watcher_id parameter required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	watcher, exists := e.watchers[watcherId]
	if !exists {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("watcher %s not found", watcherId),
		}, fmt.Errorf("watcher not found")
	}

	if err := watcher.Stop(); err != nil {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to stop watcher: %v", err),
		}, err
	}

	delete(e.watchers, watcherId)

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"watcher_id": watcherId,
			"status":     "stopped",
		},
		Metadata: map[string]interface{}{
			"action": "stop",
		},
	}, nil
}

// listWatches lists all active watchers
func (e *FileWatchExecutor) listWatches(params map[string]interface{}) (*ToolResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	watchers := make([]map[string]interface{}, 0)
	for id, watcher := range e.watchers {
		watches := watcher.GetWatches()
		watcherInfo := map[string]interface{}{
			"id":      id,
			"watches": len(watches),
			"paths":   make([]string, 0, len(watches)),
		}
		
		for path := range watches {
			watcherInfo["paths"] = append(watcherInfo["paths"].([]string), path)
		}
		
		watchers = append(watchers, watcherInfo)
	}

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"watchers": watchers,
			"count":    len(watchers),
		},
		Metadata: map[string]interface{}{
			"action": "list",
		},
	}, nil
}

// getEvents retrieves recent events from a watcher
func (e *FileWatchExecutor) getEvents(params map[string]interface{}) (*ToolResult, error) {
	watcherId, _ := params["watcher_id"].(string)
	if watcherId == "" {
		return &ToolResult{
			Success:      false,
			ErrorMessage: "watcher_id parameter required",
		}, fmt.Errorf("watcher_id parameter required")
	}

	maxEvents := 10
	if max, ok := params["max_events"].(int); ok {
		maxEvents = max
	}

	e.mu.RLock()
	watcher, exists := e.watchers[watcherId]
	e.mu.RUnlock()

	if !exists {
		return &ToolResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("watcher %s not found", watcherId),
		}, fmt.Errorf("watcher not found")
	}

	events := make([]FileEvent, 0, maxEvents)
	eventChan := watcher.GetEvents()
	
	// Collect events (non-blocking)
	timeout := time.After(100 * time.Millisecond)
	for i := 0; i < maxEvents; i++ {
		select {
		case event := <-eventChan:
			events = append(events, event)
		case <-timeout:
			break
		}
	}

	return &ToolResult{
		Success: true,
		Result: map[string]interface{}{
			"events": events,
			"count":  len(events),
		},
		Metadata: map[string]interface{}{
			"action":     "events",
			"watcher_id": watcherId,
		},
	}, nil
}

// Name returns the name of the executor
func (e *FileWatchExecutor) Name() string {
	return "file_watch"
}

// Description returns the description of the executor
func (e *FileWatchExecutor) Description() string {
	return "Watch files and directories for changes in real-time"
}

// FileChangeDebouncer provides debouncing for file change events
type FileChangeDebouncer struct {
	duration time.Duration
	timers   map[string]*time.Timer
	mu       sync.Mutex
	callback func(path string)
}

// NewFileChangeDebouncer creates a new debouncer
func NewFileChangeDebouncer(duration time.Duration, callback func(path string)) *FileChangeDebouncer {
	return &FileChangeDebouncer{
		duration: duration,
		timers:   make(map[string]*time.Timer),
		callback: callback,
	}
}

// Trigger triggers a debounced event
func (d *FileChangeDebouncer) Trigger(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel existing timer
	if timer, exists := d.timers[path]; exists {
		timer.Stop()
	}

	// Create new timer
	d.timers[path] = time.AfterFunc(d.duration, func() {
		d.mu.Lock()
		delete(d.timers, path)
		d.mu.Unlock()
		d.callback(path)
	})
}

// Stop stops all pending timers
func (d *FileChangeDebouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, timer := range d.timers {
		timer.Stop()
	}
	d.timers = make(map[string]*time.Timer)
}

// WatcherPool manages a pool of file watchers
type WatcherPool struct {
	watchers  []*FileWatcher
	maxSize   int
	current   int
	mu        sync.Mutex
}

// NewWatcherPool creates a new watcher pool
func NewWatcherPool(maxSize int) *WatcherPool {
	return &WatcherPool{
		watchers: make([]*FileWatcher, 0, maxSize),
		maxSize:  maxSize,
		current:  0,
	}
}

// Get gets a watcher from the pool
func (p *WatcherPool) Get() (*FileWatcher, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.watchers) > 0 {
		watcher := p.watchers[len(p.watchers)-1]
		p.watchers = p.watchers[:len(p.watchers)-1]
		return watcher, nil
	}

	if p.current >= p.maxSize {
		return nil, fmt.Errorf("watcher pool exhausted")
	}

	watcher, err := NewFileWatcher(100)
	if err != nil {
		return nil, err
	}

	p.current++
	return watcher, nil
}

// Put returns a watcher to the pool
func (p *WatcherPool) Put(watcher *FileWatcher) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.watchers) < p.maxSize {
		p.watchers = append(p.watchers, watcher)
	} else {
		watcher.Stop()
		p.current--
	}
}

// StreamingFileReader provides streaming file reading with watch capability
type StreamingFileReader struct {
	path     string
	file     *os.File
	watcher  *FileWatcher
	position int64
	mu       sync.Mutex
}

// NewStreamingFileReader creates a new streaming file reader
func NewStreamingFileReader(path string) (*StreamingFileReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &StreamingFileReader{
		path:     path,
		file:     file,
		position: 0,
	}, nil
}

// ReadNew reads new content since last read
func (r *StreamingFileReader) ReadNew() ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Seek to last position
	if _, err := r.file.Seek(r.position, 0); err != nil {
		return nil, err
	}

	// Read new content
	data, err := io.ReadAll(r.file)
	if err != nil {
		return nil, err
	}

	// Update position
	if newPos, err := r.file.Seek(0, io.SeekCurrent); err == nil {
		r.position = newPos
	}

	return data, nil
}

// Close closes the reader
func (r *StreamingFileReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.watcher != nil {
		r.watcher.Stop()
	}
	return r.file.Close()
}