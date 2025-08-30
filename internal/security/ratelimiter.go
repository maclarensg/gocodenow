package security

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// RateLimitStrategy defines different rate limiting strategies
type RateLimitStrategy string

const (
	StrategyFixedWindow   RateLimitStrategy = "fixed_window"
	StrategySlidingWindow RateLimitStrategy = "sliding_window"
	StrategyTokenBucket   RateLimitStrategy = "token_bucket"
	StrategyLeakyBucket   RateLimitStrategy = "leaky_bucket"
)

// RateLimitScope defines the scope of rate limiting
type RateLimitScope string

const (
	ScopeGlobal    RateLimitScope = "global"
	ScopePerUser   RateLimitScope = "per_user"
	ScopePerTool   RateLimitScope = "per_tool"
	ScopePerIP     RateLimitScope = "per_ip"
	ScopePerUserTool RateLimitScope = "per_user_tool"
)

// RateLimitConfig configures rate limiting behavior
type RateLimitConfig struct {
	Strategy      RateLimitStrategy `json:"strategy"`
	Scope         RateLimitScope    `json:"scope"`
	WindowSize    time.Duration     `json:"window_size"`
	MaxRequests   int64             `json:"max_requests"`
	BurstSize     int64             `json:"burst_size,omitempty"`
	RefillRate    time.Duration     `json:"refill_rate,omitempty"`
	EnableAdaptive bool             `json:"enable_adaptive"`
	BackoffMultiplier float64       `json:"backoff_multiplier"`
	MaxBackoffTime time.Duration    `json:"max_backoff_time"`
	WhitelistUsers []string         `json:"whitelist_users,omitempty"`
	WhitelistIPs   []string         `json:"whitelist_ips,omitempty"`
}

// RateLimitResult represents the result of a rate limit check
type RateLimitResult struct {
	Allowed       bool          `json:"allowed"`
	Remaining     int64         `json:"remaining"`
	ResetTime     time.Time     `json:"reset_time"`
	RetryAfter    time.Duration `json:"retry_after,omitempty"`
	CurrentUsage  int64         `json:"current_usage"`
	WindowStart   time.Time     `json:"window_start"`
	WindowEnd     time.Time     `json:"window_end"`
	BackoffUntil  time.Time     `json:"backoff_until,omitempty"`
	ViolationCount int64        `json:"violation_count"`
}

// RateLimitKey represents a unique key for rate limiting
type RateLimitKey struct {
	Scope      RateLimitScope `json:"scope"`
	Identifier string         `json:"identifier"`
}

// RateLimitEntry tracks rate limit state for a key
type RateLimitEntry struct {
	Key            RateLimitKey    `json:"key"`
	RequestCount   int64           `json:"request_count"`
	WindowStart    time.Time       `json:"window_start"`
	LastRequest    time.Time       `json:"last_request"`
	ViolationCount int64           `json:"violation_count"`
	BackoffUntil   time.Time       `json:"backoff_until"`
	TokenCount     int64           `json:"token_count,omitempty"`
	LastRefill     time.Time       `json:"last_refill,omitempty"`
	SlidingWindow  []time.Time     `json:"sliding_window,omitempty"`
	AdaptiveLimit  int64           `json:"adaptive_limit,omitempty"`
	mu             sync.RWMutex    `json:"-"`
}

// RateLimitMetrics provides rate limiting statistics
type RateLimitMetrics struct {
	TotalRequests      int64                    `json:"total_requests"`
	AllowedRequests    int64                    `json:"allowed_requests"`
	BlockedRequests    int64                    `json:"blocked_requests"`
	ActiveKeys         int64                    `json:"active_keys"`
	ViolationsByScope  map[RateLimitScope]int64 `json:"violations_by_scope"`
	ViolationsByUser   map[string]int64         `json:"violations_by_user"`
	AverageResponseTime time.Duration          `json:"average_response_time"`
	PeakUsageByHour    map[string]int64         `json:"peak_usage_by_hour"`
	LastUpdated        time.Time                `json:"last_updated"`
}

// SecurityRateLimiter provides comprehensive rate limiting for security enforcement
type SecurityRateLimiter struct {
	config     map[RateLimitScope]RateLimitConfig
	entries    map[string]*RateLimitEntry
	mu         sync.RWMutex
	metrics    RateLimitMetrics
	metricsMu  sync.RWMutex
	auditor    *SecurityAuditor
	cleanupTicker *time.Ticker
	stopChan   chan struct{}
}

// NewSecurityRateLimiter creates a new security rate limiter
func NewSecurityRateLimiter(auditor *SecurityAuditor) *SecurityRateLimiter {
	limiter := &SecurityRateLimiter{
		config:   make(map[RateLimitScope]RateLimitConfig),
		entries:  make(map[string]*RateLimitEntry),
		auditor:  auditor,
		stopChan: make(chan struct{}),
		metrics: RateLimitMetrics{
			ViolationsByScope: make(map[RateLimitScope]int64),
			ViolationsByUser:  make(map[string]int64),
			PeakUsageByHour:   make(map[string]int64),
			LastUpdated:       time.Now(),
		},
	}

	// Set default configurations
	limiter.setDefaultConfigs()

	// Start cleanup goroutine
	limiter.cleanupTicker = time.NewTicker(5 * time.Minute)
	go limiter.cleanupRoutine()

	return limiter
}

// setDefaultConfigs sets default rate limiting configurations
func (srl *SecurityRateLimiter) setDefaultConfigs() {
	// Global rate limit - very generous for normal operations
	srl.config[ScopeGlobal] = RateLimitConfig{
		Strategy:          StrategyTokenBucket,
		Scope:            ScopeGlobal,
		WindowSize:       time.Hour,
		MaxRequests:      10000,
		BurstSize:        100,
		RefillRate:       time.Second,
		EnableAdaptive:   false,
		BackoffMultiplier: 2.0,
		MaxBackoffTime:   30 * time.Minute,
	}

	// Per-user rate limit - reasonable for individual users
	srl.config[ScopePerUser] = RateLimitConfig{
		Strategy:          StrategySlidingWindow,
		Scope:            ScopePerUser,
		WindowSize:       time.Hour,
		MaxRequests:      1000,
		EnableAdaptive:   true,
		BackoffMultiplier: 1.5,
		MaxBackoffTime:   10 * time.Minute,
	}

	// Per-tool rate limit - stricter for potentially dangerous tools
	srl.config[ScopePerTool] = RateLimitConfig{
		Strategy:          StrategyFixedWindow,
		Scope:            ScopePerTool,
		WindowSize:       10 * time.Minute,
		MaxRequests:      50,
		EnableAdaptive:   false,
		BackoffMultiplier: 2.0,
		MaxBackoffTime:   20 * time.Minute,
	}

	// Per-IP rate limit - protection against automated attacks
	srl.config[ScopePerIP] = RateLimitConfig{
		Strategy:          StrategyLeakyBucket,
		Scope:            ScopePerIP,
		WindowSize:       time.Minute,
		MaxRequests:      100,
		RefillRate:       time.Second,
		EnableAdaptive:   true,
		BackoffMultiplier: 3.0,
		MaxBackoffTime:   60 * time.Minute,
	}

	// Per-user-tool rate limit - finest granularity control
	srl.config[ScopePerUserTool] = RateLimitConfig{
		Strategy:          StrategySlidingWindow,
		Scope:            ScopePerUserTool,
		WindowSize:       5 * time.Minute,
		MaxRequests:      20,
		EnableAdaptive:   true,
		BackoffMultiplier: 2.0,
		MaxBackoffTime:   15 * time.Minute,
	}
}

// SetConfig sets or updates rate limiting configuration for a scope
func (srl *SecurityRateLimiter) SetConfig(scope RateLimitScope, config RateLimitConfig) {
	srl.mu.Lock()
	defer srl.mu.Unlock()
	
	config.Scope = scope
	srl.config[scope] = config
}

// GetConfig gets rate limiting configuration for a scope
func (srl *SecurityRateLimiter) GetConfig(scope RateLimitScope) (RateLimitConfig, bool) {
	srl.mu.RLock()
	defer srl.mu.RUnlock()
	
	config, exists := srl.config[scope]
	return config, exists
}

// CheckRateLimit checks if a request is allowed under rate limits
func (srl *SecurityRateLimiter) CheckRateLimit(ctx context.Context, user, tool, ipAddress string) *RateLimitResult {
	now := time.Now()
	overallResult := &RateLimitResult{
		Allowed:      true,
		Remaining:    0,
		CurrentUsage: 0,
	}

	// Check all applicable scopes
	scopes := srl.getApplicableScopes(user, tool, ipAddress)
	
	for _, scopeInfo := range scopes {
		result := srl.checkScopeRateLimit(scopeInfo.scope, scopeInfo.key, now)
		
		// Update overall result based on most restrictive outcome
		if !result.Allowed {
			overallResult.Allowed = false
			if result.RetryAfter > overallResult.RetryAfter {
				overallResult.RetryAfter = result.RetryAfter
			}
			if result.BackoffUntil.After(overallResult.BackoffUntil) {
				overallResult.BackoffUntil = result.BackoffUntil
			}
		}
		
		// Use the most restrictive remaining count
		if overallResult.Remaining == 0 || (result.Remaining < overallResult.Remaining && result.Remaining >= 0) {
			overallResult.Remaining = result.Remaining
			overallResult.ResetTime = result.ResetTime
			overallResult.WindowStart = result.WindowStart
			overallResult.WindowEnd = result.WindowEnd
		}
		
		overallResult.CurrentUsage += result.CurrentUsage
		overallResult.ViolationCount += result.ViolationCount
	}

	// Update metrics
	srl.updateMetrics(overallResult.Allowed, user, scopes)

	// Log rate limit events
	if srl.auditor != nil {
		srl.logRateLimitEvent(user, tool, ipAddress, overallResult)
	}

	return overallResult
}

// AllowRequest checks and consumes a rate limit token if allowed
func (srl *SecurityRateLimiter) AllowRequest(ctx context.Context, user, tool, ipAddress string) *RateLimitResult {
	result := srl.CheckRateLimit(ctx, user, tool, ipAddress)
	
	if result.Allowed {
		// Consume tokens for all applicable scopes
		scopes := srl.getApplicableScopes(user, tool, ipAddress)
		for _, scopeInfo := range scopes {
			srl.consumeToken(scopeInfo.scope, scopeInfo.key, time.Now())
		}
	}
	
	return result
}

// ResetLimits resets rate limits for a specific user or scope
func (srl *SecurityRateLimiter) ResetLimits(scope RateLimitScope, identifier string) error {
	srl.mu.Lock()
	defer srl.mu.Unlock()
	
	key := srl.generateKey(scope, identifier)
	delete(srl.entries, key)
	
	if srl.auditor != nil {
		srl.auditor.LogEvent(&AuditEvent{
			Level:     AuditLevelWarning,
			EventType: EventTypeSystemAccess,
			Source:    "rate_limiter",
			Action:    "RESET_LIMITS",
			Result:    "SUCCESS",
			Message:   fmt.Sprintf("Rate limits reset for %s: %s", scope, identifier),
			Details: map[string]interface{}{
				"scope":      scope,
				"identifier": identifier,
			},
		})
	}
	
	return nil
}

// GetCurrentUsage returns current usage statistics for monitoring
func (srl *SecurityRateLimiter) GetCurrentUsage(scope RateLimitScope, identifier string) (*RateLimitEntry, error) {
	srl.mu.RLock()
	defer srl.mu.RUnlock()
	
	key := srl.generateKey(scope, identifier)
	entry, exists := srl.entries[key]
	if !exists {
		return nil, fmt.Errorf("no rate limit entry found for %s", key)
	}
	
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	entryCopy := *entry
	entryCopy.mu = sync.RWMutex{}
	
	return &entryCopy, nil
}

// GetMetrics returns comprehensive rate limiting metrics
func (srl *SecurityRateLimiter) GetMetrics() RateLimitMetrics {
	srl.metricsMu.RLock()
	defer srl.metricsMu.RUnlock()
	
	// Update active keys count
	srl.mu.RLock()
	srl.metrics.ActiveKeys = int64(len(srl.entries))
	srl.mu.RUnlock()
	
	srl.metrics.LastUpdated = time.Now()
	return srl.metrics
}

// Close shuts down the rate limiter
func (srl *SecurityRateLimiter) Close() error {
	close(srl.stopChan)
	if srl.cleanupTicker != nil {
		srl.cleanupTicker.Stop()
	}
	return nil
}

// Private helper methods

type scopeInfo struct {
	scope RateLimitScope
	key   string
}

func (srl *SecurityRateLimiter) getApplicableScopes(user, tool, ipAddress string) []scopeInfo {
	var scopes []scopeInfo
	
	// Global scope always applies
	scopes = append(scopes, scopeInfo{
		scope: ScopeGlobal,
		key:   srl.generateKey(ScopeGlobal, "global"),
	})
	
	// Per-user scope
	if user != "" {
		scopes = append(scopes, scopeInfo{
			scope: ScopePerUser,
			key:   srl.generateKey(ScopePerUser, user),
		})
	}
	
	// Per-tool scope
	if tool != "" {
		scopes = append(scopes, scopeInfo{
			scope: ScopePerTool,
			key:   srl.generateKey(ScopePerTool, tool),
		})
	}
	
	// Per-IP scope
	if ipAddress != "" {
		scopes = append(scopes, scopeInfo{
			scope: ScopePerIP,
			key:   srl.generateKey(ScopePerIP, ipAddress),
		})
	}
	
	// Per-user-tool scope
	if user != "" && tool != "" {
		scopes = append(scopes, scopeInfo{
			scope: ScopePerUserTool,
			key:   srl.generateKey(ScopePerUserTool, fmt.Sprintf("%s:%s", user, tool)),
		})
	}
	
	return scopes
}

func (srl *SecurityRateLimiter) checkScopeRateLimit(scope RateLimitScope, key string, now time.Time) *RateLimitResult {
	config, exists := srl.config[scope]
	if !exists {
		return &RateLimitResult{Allowed: true}
	}
	
	// Check whitelists
	if srl.isWhitelisted(scope, key, config) {
		return &RateLimitResult{
			Allowed:   true,
			Remaining: config.MaxRequests,
		}
	}
	
	entry := srl.getOrCreateEntry(scope, key, config, now)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	
	// Check if in backoff period
	if now.Before(entry.BackoffUntil) {
		return &RateLimitResult{
			Allowed:        false,
			Remaining:      0,
			RetryAfter:     entry.BackoffUntil.Sub(now),
			BackoffUntil:   entry.BackoffUntil,
			ViolationCount: entry.ViolationCount,
		}
	}
	
	switch config.Strategy {
	case StrategyFixedWindow:
		return srl.checkFixedWindow(entry, config, now)
	case StrategySlidingWindow:
		return srl.checkSlidingWindow(entry, config, now)
	case StrategyTokenBucket:
		return srl.checkTokenBucket(entry, config, now)
	case StrategyLeakyBucket:
		return srl.checkLeakyBucket(entry, config, now)
	default:
		return srl.checkFixedWindow(entry, config, now)
	}
}

func (srl *SecurityRateLimiter) checkFixedWindow(entry *RateLimitEntry, config RateLimitConfig, now time.Time) *RateLimitResult {
	windowStart := now.Truncate(config.WindowSize)
	
	// Reset window if it's a new window
	if windowStart.After(entry.WindowStart) {
		entry.WindowStart = windowStart
		entry.RequestCount = 0
	}
	
	limit := config.MaxRequests
	if config.EnableAdaptive && entry.AdaptiveLimit > 0 {
		limit = entry.AdaptiveLimit
	}
	
	remaining := limit - entry.RequestCount
	allowed := remaining > 0
	
	if !allowed {
		entry.ViolationCount++
		srl.applyBackoff(entry, config)
	}
	
	return &RateLimitResult{
		Allowed:        allowed,
		Remaining:      remaining,
		ResetTime:      windowStart.Add(config.WindowSize),
		CurrentUsage:   entry.RequestCount,
		WindowStart:    windowStart,
		WindowEnd:      windowStart.Add(config.WindowSize),
		ViolationCount: entry.ViolationCount,
	}
}

func (srl *SecurityRateLimiter) checkSlidingWindow(entry *RateLimitEntry, config RateLimitConfig, now time.Time) *RateLimitResult {
	cutoff := now.Add(-config.WindowSize)
	
	// Remove old entries
	validRequests := make([]time.Time, 0, len(entry.SlidingWindow))
	for _, timestamp := range entry.SlidingWindow {
		if timestamp.After(cutoff) {
			validRequests = append(validRequests, timestamp)
		}
	}
	entry.SlidingWindow = validRequests
	entry.RequestCount = int64(len(validRequests))
	
	limit := config.MaxRequests
	if config.EnableAdaptive && entry.AdaptiveLimit > 0 {
		limit = entry.AdaptiveLimit
	}
	
	remaining := limit - entry.RequestCount
	allowed := remaining > 0
	
	if !allowed {
		entry.ViolationCount++
		srl.applyBackoff(entry, config)
	}
	
	return &RateLimitResult{
		Allowed:        allowed,
		Remaining:      remaining,
		ResetTime:      now.Add(config.WindowSize),
		CurrentUsage:   entry.RequestCount,
		WindowStart:    cutoff,
		WindowEnd:      now,
		ViolationCount: entry.ViolationCount,
	}
}

func (srl *SecurityRateLimiter) checkTokenBucket(entry *RateLimitEntry, config RateLimitConfig, now time.Time) *RateLimitResult {
	// Initialize if first request
	if entry.LastRefill.IsZero() {
		entry.LastRefill = now
		entry.TokenCount = config.BurstSize
	}
	
	// Calculate tokens to add based on time elapsed
	elapsed := now.Sub(entry.LastRefill)
	tokensToAdd := elapsed / config.RefillRate
	
	if tokensToAdd > 0 {
		entry.TokenCount += int64(tokensToAdd)
		if entry.TokenCount > config.BurstSize {
			entry.TokenCount = config.BurstSize
		}
		entry.LastRefill = now
	}
	
	allowed := entry.TokenCount > 0
	
	if !allowed {
		entry.ViolationCount++
		srl.applyBackoff(entry, config)
	}
	
	nextRefill := entry.LastRefill.Add(config.RefillRate)
	
	return &RateLimitResult{
		Allowed:        allowed,
		Remaining:      entry.TokenCount,
		ResetTime:      nextRefill,
		CurrentUsage:   config.BurstSize - entry.TokenCount,
		ViolationCount: entry.ViolationCount,
	}
}

func (srl *SecurityRateLimiter) checkLeakyBucket(entry *RateLimitEntry, config RateLimitConfig, now time.Time) *RateLimitResult {
	// Similar to token bucket but with steady leak rate
	if entry.LastRequest.IsZero() {
		entry.LastRequest = now
		entry.RequestCount = 0
	}
	
	// Calculate leak since last request
	elapsed := now.Sub(entry.LastRequest)
	leaked := elapsed / config.RefillRate
	
	if leaked > 0 {
		entry.RequestCount -= int64(leaked)
		if entry.RequestCount < 0 {
			entry.RequestCount = 0
		}
		entry.LastRequest = now
	}
	
	allowed := entry.RequestCount < config.MaxRequests
	
	if !allowed {
		entry.ViolationCount++
		srl.applyBackoff(entry, config)
	}
	
	remaining := config.MaxRequests - entry.RequestCount
	
	return &RateLimitResult{
		Allowed:        allowed,
		Remaining:      remaining,
		CurrentUsage:   entry.RequestCount,
		ViolationCount: entry.ViolationCount,
	}
}

func (srl *SecurityRateLimiter) consumeToken(scope RateLimitScope, key string, now time.Time) {
	config, exists := srl.config[scope]
	if !exists {
		return
	}
	
	entry := srl.getOrCreateEntry(scope, key, config, now)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	
	switch config.Strategy {
	case StrategyFixedWindow, StrategyLeakyBucket:
		entry.RequestCount++
	case StrategySlidingWindow:
		entry.SlidingWindow = append(entry.SlidingWindow, now)
		entry.RequestCount++
	case StrategyTokenBucket:
		if entry.TokenCount > 0 {
			entry.TokenCount--
		}
	}
	
	entry.LastRequest = now
}

func (srl *SecurityRateLimiter) getOrCreateEntry(scope RateLimitScope, key string, config RateLimitConfig, now time.Time) *RateLimitEntry {
	srl.mu.RLock()
	entry, exists := srl.entries[key]
	srl.mu.RUnlock()
	
	if exists {
		return entry
	}
	
	srl.mu.Lock()
	defer srl.mu.Unlock()
	
	// Double-check after acquiring write lock
	entry, exists = srl.entries[key]
	if exists {
		return entry
	}
	
	entry = &RateLimitEntry{
		Key: RateLimitKey{
			Scope:      scope,
			Identifier: key,
		},
		WindowStart:    now,
		LastRequest:    now,
		AdaptiveLimit:  config.MaxRequests,
		SlidingWindow:  make([]time.Time, 0, config.MaxRequests),
	}
	
	// Initialize based on strategy
	if config.Strategy == StrategyTokenBucket {
		entry.TokenCount = config.BurstSize
		entry.LastRefill = now
	}
	
	srl.entries[key] = entry
	return entry
}

func (srl *SecurityRateLimiter) generateKey(scope RateLimitScope, identifier string) string {
	return fmt.Sprintf("%s:%s", scope, identifier)
}

func (srl *SecurityRateLimiter) isWhitelisted(scope RateLimitScope, key string, config RateLimitConfig) bool {
	// Extract identifier from key
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return false
	}
	identifier := parts[1]
	
	// Check user whitelist
	for _, whitelistUser := range config.WhitelistUsers {
		if identifier == whitelistUser || strings.Contains(identifier, whitelistUser) {
			return true
		}
	}
	
	// Check IP whitelist
	for _, whitelistIP := range config.WhitelistIPs {
		if identifier == whitelistIP {
			return true
		}
	}
	
	return false
}

func (srl *SecurityRateLimiter) applyBackoff(entry *RateLimitEntry, config RateLimitConfig) {
	if config.BackoffMultiplier <= 0 {
		return
	}
	
	backoffDuration := time.Duration(float64(config.WindowSize) * config.BackoffMultiplier)
	
	// Apply exponential backoff based on violation count
	for i := int64(1); i < entry.ViolationCount && i < 10; i++ {
		backoffDuration = time.Duration(float64(backoffDuration) * config.BackoffMultiplier)
	}
	
	if config.MaxBackoffTime > 0 && backoffDuration > config.MaxBackoffTime {
		backoffDuration = config.MaxBackoffTime
	}
	
	entry.BackoffUntil = time.Now().Add(backoffDuration)
}

func (srl *SecurityRateLimiter) updateMetrics(allowed bool, user string, scopes []scopeInfo) {
	srl.metricsMu.Lock()
	defer srl.metricsMu.Unlock()
	
	srl.metrics.TotalRequests++
	
	if allowed {
		srl.metrics.AllowedRequests++
	} else {
		srl.metrics.BlockedRequests++
		
		// Update violation counts
		for _, scopeInfo := range scopes {
			srl.metrics.ViolationsByScope[scopeInfo.scope]++
		}
		
		if user != "" {
			srl.metrics.ViolationsByUser[user]++
		}
	}
	
	// Update peak usage by hour
	hourKey := time.Now().Format("2006-01-02T15")
	srl.metrics.PeakUsageByHour[hourKey]++
}

func (srl *SecurityRateLimiter) logRateLimitEvent(user, tool, ipAddress string, result *RateLimitResult) {
	if srl.auditor == nil {
		return
	}
	
	level := AuditLevelInfo
	action := "RATE_CHECK"
	resultStr := "ALLOWED"
	
	if !result.Allowed {
		level = AuditLevelWarning
		action = "RATE_LIMIT_EXCEEDED"
		resultStr = "BLOCKED"
	}
	
	details := map[string]interface{}{
		"remaining":       result.Remaining,
		"current_usage":   result.CurrentUsage,
		"violation_count": result.ViolationCount,
		"window_start":    result.WindowStart,
		"window_end":      result.WindowEnd,
	}
	
	if !result.BackoffUntil.IsZero() {
		details["backoff_until"] = result.BackoffUntil
	}
	
	if result.RetryAfter > 0 {
		details["retry_after_seconds"] = result.RetryAfter.Seconds()
	}
	
	srl.auditor.LogEvent(&AuditEvent{
		Level:     level,
		EventType: EventTypeSecurityViolation,
		Source:    "rate_limiter",
		User:      user,
		IPAddress: ipAddress,
		Resource:  tool,
		Action:    action,
		Result:    resultStr,
		Message:   fmt.Sprintf("Rate limit check for user %s using tool %s: %s", user, tool, resultStr),
		Details:   details,
		Tags:      []string{"rate_limiting", action},
	})
}

func (srl *SecurityRateLimiter) cleanupRoutine() {
	for {
		select {
		case <-srl.cleanupTicker.C:
			srl.cleanupExpiredEntries()
		case <-srl.stopChan:
			return
		}
	}
}

func (srl *SecurityRateLimiter) cleanupExpiredEntries() {
	now := time.Now()
	expiredKeys := make([]string, 0)
	
	srl.mu.RLock()
	for key, entry := range srl.entries {
		entry.mu.RLock()
		// Consider entry expired if no requests in the last 2 hours and not in backoff
		isExpired := now.Sub(entry.LastRequest) > 2*time.Hour && now.After(entry.BackoffUntil)
		entry.mu.RUnlock()
		
		if isExpired {
			expiredKeys = append(expiredKeys, key)
		}
	}
	srl.mu.RUnlock()
	
	if len(expiredKeys) > 0 {
		srl.mu.Lock()
		for _, key := range expiredKeys {
			delete(srl.entries, key)
		}
		srl.mu.Unlock()
	}
}

// CreateHighSecurityConfig creates a restrictive rate limit configuration
func CreateHighSecurityConfig() map[RateLimitScope]RateLimitConfig {
	return map[RateLimitScope]RateLimitConfig{
		ScopeGlobal: {
			Strategy:          StrategyTokenBucket,
			Scope:            ScopeGlobal,
			WindowSize:       time.Hour,
			MaxRequests:      1000,
			BurstSize:        10,
			RefillRate:       10 * time.Second,
			BackoffMultiplier: 3.0,
			MaxBackoffTime:   60 * time.Minute,
		},
		ScopePerUser: {
			Strategy:          StrategySlidingWindow,
			Scope:            ScopePerUser,
			WindowSize:       10 * time.Minute,
			MaxRequests:      50,
			EnableAdaptive:   true,
			BackoffMultiplier: 2.0,
			MaxBackoffTime:   30 * time.Minute,
		},
		ScopePerTool: {
			Strategy:          StrategyFixedWindow,
			Scope:            ScopePerTool,
			WindowSize:       5 * time.Minute,
			MaxRequests:      10,
			BackoffMultiplier: 4.0,
			MaxBackoffTime:   120 * time.Minute,
		},
	}
}