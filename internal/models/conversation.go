package models

import (
	"database/sql"
	"gocodenow/internal/types"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ConversationHistory manages the conversation state using StorageManager
type ConversationHistory struct {
	storageManager types.StorageManager
	cachedBlocks   []types.ConversationBlock
	selected       int
	dirty          bool // tracks if cache needs refresh
}

// NewConversationHistory creates a new conversation history with StorageManager
func NewConversationHistory(storageManager types.StorageManager) *ConversationHistory {
	ch := &ConversationHistory{
		storageManager: storageManager,
		cachedBlocks:   []types.ConversationBlock{},
		selected:       -1,
		dirty:          true,
	}
	
	// Load initial conversations from storage
	ch.refreshCache()
	return ch
}

// NewConversationHistoryInMemory creates a new in-memory conversation history for testing
func NewConversationHistoryInMemory() *ConversationHistory {
	return &ConversationHistory{
		storageManager: nil, // Will use in-memory cache only
		cachedBlocks:   []types.ConversationBlock{},
		selected:       -1,
		dirty:          false,
	}
}

// GetBlocks returns all conversation blocks
func (ch *ConversationHistory) GetBlocks() []types.ConversationBlock {
	ch.ensureCacheUpdated()
	return ch.cachedBlocks
}

// GetVisibleBlocks returns all blocks (viewport handles visibility now)
func (ch *ConversationHistory) GetVisibleBlocks(viewportHeight int) []types.ConversationBlock {
	ch.ensureCacheUpdated()
	return ch.cachedBlocks
}

// GetSelected returns the currently selected block index
func (ch *ConversationHistory) GetSelected() int {
	return ch.selected
}

// SetSelected sets the selected block index
func (ch *ConversationHistory) SetSelected(index int) {
	ch.ensureCacheUpdated()
	if index >= 0 && index < len(ch.cachedBlocks) {
		ch.selected = index
	}
}

// Scrolling is now handled by viewport component - these methods removed

// SelectUp moves selection up
func (ch *ConversationHistory) SelectUp() {
	ch.ensureCacheUpdated()
	if ch.selected > 0 {
		ch.selected--
	}
}

// SelectDown moves selection down
func (ch *ConversationHistory) SelectDown() {
	ch.ensureCacheUpdated()
	if ch.selected < len(ch.cachedBlocks)-1 {
		ch.selected++
	}
}

// ToggleExpanded toggles the expanded state of the selected block
func (ch *ConversationHistory) ToggleExpanded() {
	ch.ensureCacheUpdated()
	if ch.selected >= 0 && ch.selected < len(ch.cachedBlocks) {
		ch.cachedBlocks[ch.selected].Expanded = !ch.cachedBlocks[ch.selected].Expanded
		
		// If we have storage, persist the expanded state change
		if ch.storageManager != nil {
			ch.storageManager.Save(&ch.cachedBlocks[ch.selected])
		}
	}
}

// AddConversation adds a new conversation block and persists it
func (ch *ConversationHistory) AddConversation(userInput string, assistantResponse string) error {
	// Generate a unique ID for the conversation
	conversationID := uuid.New().String()
	
	newConv := types.ConversationBlock{
		ID:          conversationID,
		UserInput:   userInput,
		LLMResponse: assistantResponse,
		ModelName:   "", // Will be set when sending to LLM
		Timestamp:   time.Now(),
		Status:      types.StatusCompleted,
		TokenUsage:  types.TokenUsage{InputTokens: 0, OutputTokens: 0},
		ExecutionTime: 0,
		ToolCalls:     []types.ToolCall{},
		ToolResults:   []types.ToolResult{},
		FileOperations: []types.FileOperation{},
		Expanded:      false,
		
		// Legacy fields for backward compatibility
		User:      userInput,
		Assistant: assistantResponse,
		Tools:     []string{},
		Files:     []string{},
	}
	
	// Save to storage if available
	if ch.storageManager != nil {
		if err := ch.storageManager.Save(&newConv); err != nil {
			return err
		}
	}
	
	// Add to cache
	ch.cachedBlocks = append(ch.cachedBlocks, newConv)
	
	// Select the newest conversation
	ch.selected = len(ch.cachedBlocks) - 1
	
	return nil
}

// ensureCacheUpdated refreshes the cache if it's dirty
func (ch *ConversationHistory) ensureCacheUpdated() {
	if ch.dirty {
		ch.refreshCache()
	}
}

// refreshCache loads conversations from storage into the cache
func (ch *ConversationHistory) refreshCache() {
	if ch.storageManager == nil {
		// In-memory mode, no refresh needed
		ch.dirty = false
		return
	}
	
	// Load recent conversations from storage (last 50)
	conversations, err := ch.storageManager.LoadRecent(50)
	if err != nil {
		// If storage load fails, keep empty cache
		ch.cachedBlocks = []types.ConversationBlock{}
		ch.dirty = false
		return
	}
	ch.cachedBlocks = make([]types.ConversationBlock, len(conversations))
	
	for i, conv := range conversations {
		ch.cachedBlocks[i] = *conv
	}
	
	// Adjust selection if it's out of bounds
	if ch.selected >= len(ch.cachedBlocks) {
		if len(ch.cachedBlocks) > 0 {
			ch.selected = len(ch.cachedBlocks) - 1
		} else {
			ch.selected = -1
		}
	}
	
	ch.dirty = false
}


// Status management methods

// UpdateConversationStatus updates the status of a conversation
func (ch *ConversationHistory) UpdateConversationStatus(conversationID string, status string) error {
	ch.ensureCacheUpdated()
	
	// Find and update in cache
	for i := range ch.cachedBlocks {
		if ch.cachedBlocks[i].ID == conversationID {
			ch.cachedBlocks[i].Status = types.ParseConversationStatus(status)
			
			// Persist to storage if available
			if ch.storageManager != nil {
				if err := ch.storageManager.Save(&ch.cachedBlocks[i]); err != nil {
					return err
				}
			}
			return nil
		}
	}
	
	return nil // Conversation not found in cache
}

// GetConversationByID retrieves a specific conversation by ID
func (ch *ConversationHistory) GetConversationByID(conversationID string) (*types.ConversationBlock, error) {
	ch.ensureCacheUpdated()
	
	// First check cache
	for i := range ch.cachedBlocks {
		if ch.cachedBlocks[i].ID == conversationID {
			return &ch.cachedBlocks[i], nil
		}
	}
	
	// If not in cache and we have storage, try loading from storage
	if ch.storageManager != nil {
		convRecord, err := ch.storageManager.Load(conversationID)
		if err != nil {
				// Check for "not found" errors - handle sql.ErrNoRows or related errors
			if err == sql.ErrNoRows || strings.Contains(err.Error(), "no rows in result set") ||
				strings.Contains(err.Error(), "conversation not found") {
				return nil, nil // Not found, but not an error
			}
			return nil, err
		}
		if convRecord != nil {
			return convRecord, nil
		}
	}
	
	return nil, nil // Not found
}

// DeleteConversation removes a conversation
func (ch *ConversationHistory) DeleteConversation(conversationID string) error {
	// Remove from storage if available
	if ch.storageManager != nil {
		if err := ch.storageManager.Delete(conversationID); err != nil {
			return err
		}
	}
	
	// Remove from cache
	for i, block := range ch.cachedBlocks {
		if block.ID == conversationID {
			ch.cachedBlocks = append(ch.cachedBlocks[:i], ch.cachedBlocks[i+1:]...)
			
			// Adjust selection if needed
			if ch.selected >= i {
				if ch.selected > 0 {
					ch.selected--
				} else if len(ch.cachedBlocks) == 0 {
					ch.selected = -1
				}
			}
			break
		}
	}
	
	return nil
}

// RefreshFromStorage forces a refresh of the conversation cache from storage
func (ch *ConversationHistory) RefreshFromStorage() {
	ch.dirty = true
	ch.ensureCacheUpdated()
}

// Close cleanly shuts down the conversation history
func (ch *ConversationHistory) Close() error {
	if ch.storageManager != nil {
		return ch.storageManager.Shutdown()
	}
	return nil
}

// Utility functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}