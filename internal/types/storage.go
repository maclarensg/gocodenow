package types

// ConversationRepository defines the interface for conversation persistence operations
type ConversationRepository interface {
	// Save stores a conversation in the repository
	Save(conv *ConversationBlock) error
	
	// Load retrieves a conversation by ID
	Load(id string) (*ConversationBlock, error)
	
	// Delete removes a conversation by ID
	Delete(id string) error
	
	// LoadRecent retrieves the most recent conversations up to the specified limit
	LoadRecent(limit int) ([]*ConversationBlock, error)
	
	// LoadAll retrieves all conversations
	LoadAll() ([]*ConversationBlock, error)
	
	// Update updates an existing conversation
	Update(conv *ConversationBlock) error
}

// StorageManager defines the interface for managing conversation storage with caching
type StorageManager interface {
	ConversationRepository
	
	// Shutdown gracefully shuts down the storage manager
	Shutdown() error
	
	// RefreshCache refreshes the in-memory cache from the persistent storage
	RefreshCache() error
	
	// GetCacheSize returns the current number of conversations in the cache
	GetCacheSize() int
	
	// FlushCache forces writing all cached data to persistent storage
	FlushCache() error
}