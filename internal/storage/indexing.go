package storage

import (
	"fmt"
	"strings"
	"time"
)

// DatabaseIndexManager handles database indexing optimization
type DatabaseIndexManager struct {
	database *Database
}

// IndexInfo represents information about a database index
type IndexInfo struct {
	Name       string   `json:"name"`
	Table      string   `json:"table"`
	Columns    []string `json:"columns"`
	IsUnique   bool     `json:"is_unique"`
	CreatedAt  time.Time `json:"created_at"`
}

// QueryAnalysisResult represents query performance analysis
type QueryAnalysisResult struct {
	Query           string        `json:"query"`
	ExecutionTime   time.Duration `json:"execution_time"`
	RowsExamined    int64         `json:"rows_examined"`
	RowsReturned    int64         `json:"rows_returned"`
	UsingIndex      bool          `json:"using_index"`
	IndexesUsed     []string      `json:"indexes_used"`
	Recommendations []string      `json:"recommendations"`
}

// NewDatabaseIndexManager creates a new index manager
func NewDatabaseIndexManager(database *Database) *DatabaseIndexManager {
	return &DatabaseIndexManager{
		database: database,
	}
}

// CreateOptimalIndexes creates performance-critical indexes
func (dim *DatabaseIndexManager) CreateOptimalIndexes() error {
	indexes := []struct {
		name   string
		table  string
		columns []string
		unique bool
	}{
		// Conversations table indexes
		{"idx_conversations_timestamp", "conversations", []string{"timestamp"}, false},
		{"idx_conversations_created_at", "conversations", []string{"created_at"}, false},
		{"idx_conversations_status", "conversations", []string{"status"}, false},
		{"idx_conversations_model", "conversations", []string{"model_name"}, false},
		{"idx_conversations_composite", "conversations", []string{"status", "timestamp"}, false},
		{"idx_conversations_search", "conversations", []string{"timestamp", "status", "model_name"}, false},
		
		// Tool executions table indexes
		{"idx_tool_executions_conversation", "tool_executions", []string{"conversation_id"}, false},
		{"idx_tool_executions_tool", "tool_executions", []string{"tool_name"}, false},
		{"idx_tool_executions_success", "tool_executions", []string{"success"}, false},
		{"idx_tool_executions_created", "tool_executions", []string{"created_at"}, false},
		{"idx_tool_executions_composite", "tool_executions", []string{"conversation_id", "created_at"}, false},
		{"idx_tool_executions_performance", "tool_executions", []string{"tool_name", "success", "duration_ms"}, false},
		
		// File operations table indexes
		{"idx_file_operations_tool", "file_operations", []string{"tool_execution_id"}, false},
		{"idx_file_operations_path", "file_operations", []string{"file_path"}, false},
		{"idx_file_operations_type", "file_operations", []string{"operation_type"}, false},
		{"idx_file_operations_timestamp", "file_operations", []string{"timestamp"}, false},
		{"idx_file_operations_composite", "file_operations", []string{"operation_type", "timestamp"}, false},
	}
	
	for _, idx := range indexes {
		if err := dim.CreateIndex(idx.name, idx.table, idx.columns, idx.unique); err != nil {
			// Log warning but continue with other indexes
			fmt.Printf("Warning: failed to create index %s: %v\n", idx.name, err)
		}
	}
	
	return nil
}

// CreateIndex creates a database index
func (dim *DatabaseIndexManager) CreateIndex(name, table string, columns []string, unique bool) error {
	// Check if index already exists
	exists, err := dim.IndexExists(name)
	if err != nil {
		return fmt.Errorf("failed to check index existence: %w", err)
	}
	
	if exists {
		return nil // Index already exists
	}
	
	uniqueStr := ""
	if unique {
		uniqueStr = "UNIQUE "
	}
	
	columnsStr := strings.Join(columns, ", ")
	query := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)", uniqueStr, name, table, columnsStr)
	
	_, err = dim.database.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", name, err)
	}
	
	return nil
}

// DropIndex drops a database index
func (dim *DatabaseIndexManager) DropIndex(name string) error {
	query := fmt.Sprintf("DROP INDEX IF EXISTS %s", name)
	_, err := dim.database.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to drop index %s: %w", name, err)
	}
	
	return nil
}

// IndexExists checks if an index exists
func (dim *DatabaseIndexManager) IndexExists(name string) (bool, error) {
	query := `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'index' AND name = ?`
		
	var count int
	err := dim.database.db.QueryRow(query, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check index existence: %w", err)
	}
	
	return count > 0, nil
}

// ListIndexes returns all indexes in the database
func (dim *DatabaseIndexManager) ListIndexes() ([]IndexInfo, error) {
	query := `
		SELECT name, tbl_name, sql
		FROM sqlite_master
		WHERE type = 'index' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`
		
	rows, err := dim.database.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer rows.Close()
	
	var indexes []IndexInfo
	
	for rows.Next() {
		var name, table, sql string
		if err := rows.Scan(&name, &table, &sql); err != nil {
			continue
		}
		
		index := IndexInfo{
			Name:      name,
			Table:     table,
			IsUnique:  strings.Contains(strings.ToUpper(sql), "UNIQUE"),
			CreatedAt: time.Now(), // SQLite doesn't track creation time
		}
		
		// Extract columns from SQL (basic parsing)
		index.Columns = dim.extractColumnsFromSQL(sql)
		
		indexes = append(indexes, index)
	}
	
	return indexes, nil
}

// extractColumnsFromSQL extracts column names from CREATE INDEX SQL
func (dim *DatabaseIndexManager) extractColumnsFromSQL(sql string) []string {
	// Simple parsing - look for content between parentheses
	start := strings.Index(sql, "(")
	end := strings.LastIndex(sql, ")")
	
	if start == -1 || end == -1 || start >= end {
		return []string{}
	}
	
	columnsPart := sql[start+1 : end]
	columns := strings.Split(columnsPart, ",")
	
	// Clean up column names
	for i, col := range columns {
		columns[i] = strings.TrimSpace(col)
		// Remove any ordering keywords like ASC, DESC
		if idx := strings.Index(columns[i], " "); idx != -1 {
			columns[i] = columns[i][:idx]
		}
	}
	
	return columns
}

// AnalyzeQuery analyzes query performance
func (dim *DatabaseIndexManager) AnalyzeQuery(query string, params ...interface{}) (*QueryAnalysisResult, error) {
	startTime := time.Now()
	
	// Execute EXPLAIN QUERY PLAN
	explainQuery := "EXPLAIN QUERY PLAN " + query
	rows, err := dim.database.db.Query(explainQuery, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}
	defer rows.Close()
	
	var explanations []string
	var usingIndex bool
	var indexesUsed []string
	
	// SQLite EXPLAIN QUERY PLAN format: id, parent, notused, detail
	for rows.Next() {
		var id, parent, notused int
		var detail string
		
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			continue
		}
		
		explanations = append(explanations, detail)
		
		// Check if using index
		detailLower := strings.ToLower(detail)
		if strings.Contains(detailLower, "using index") {
			usingIndex = true
		}
		
		// Extract index names
		if strings.Contains(detailLower, "index") {
			// Simple extraction of index names
			parts := strings.Fields(detail)
			for i, part := range parts {
				if strings.ToLower(part) == "index" && i+1 < len(parts) {
					indexName := parts[i+1]
					indexesUsed = append(indexesUsed, indexName)
				}
			}
		}
	}
	
	// Execute actual query to measure performance
	actualStart := time.Now()
	actualRows, err := dim.database.db.Query(query, params...)
	if err != nil {
		return &QueryAnalysisResult{
			Query:         query,
			ExecutionTime: time.Since(startTime),
		}, fmt.Errorf("failed to execute query for analysis: %w", err)
	}
	defer actualRows.Close()
	
	var rowCount int64
	for actualRows.Next() {
		rowCount++
		// Don't actually process rows, just count them
		actualRows.Scan()
	}
	
	executionTime := time.Since(actualStart)
	totalTime := time.Since(startTime)
	
	// Generate recommendations
	recommendations := dim.generateRecommendations(query, usingIndex, executionTime, explanations)
	
	return &QueryAnalysisResult{
		Query:           query,
		ExecutionTime:   totalTime,
		RowsReturned:    rowCount,
		UsingIndex:      usingIndex,
		IndexesUsed:     indexesUsed,
		Recommendations: recommendations,
	}, nil
}

// generateRecommendations generates performance recommendations
func (dim *DatabaseIndexManager) generateRecommendations(query string, usingIndex bool, execTime time.Duration, explanations []string) []string {
	var recommendations []string
	queryLower := strings.ToLower(query)
	
	// Check if query is slow
	if execTime > 100*time.Millisecond {
		recommendations = append(recommendations, "Query execution time is high (>100ms)")
	}
	
	// Check if not using index
	if !usingIndex {
		recommendations = append(recommendations, "Query is not using any indexes")
		
		// Suggest specific indexes based on query patterns
		if strings.Contains(queryLower, "where") {
			if strings.Contains(queryLower, "timestamp") {
				recommendations = append(recommendations, "Consider adding index on timestamp column")
			}
			if strings.Contains(queryLower, "status") {
				recommendations = append(recommendations, "Consider adding index on status column")
			}
			if strings.Contains(queryLower, "conversation_id") {
				recommendations = append(recommendations, "Consider adding index on conversation_id column")
			}
		}
		
		if strings.Contains(queryLower, "order by") {
			recommendations = append(recommendations, "Consider adding index on ORDER BY columns")
		}
		
		if strings.Contains(queryLower, "join") {
			recommendations = append(recommendations, "Consider adding indexes on JOIN columns")
		}
	}
	
	// Check for table scans in explanations
	for _, explanation := range explanations {
		if strings.Contains(strings.ToLower(explanation), "scan table") {
			recommendations = append(recommendations, "Query is performing full table scan")
			break
		}
	}
	
	// Check for complex WHERE conditions
	whereCount := strings.Count(queryLower, "where")
	andCount := strings.Count(queryLower, "and")
	orCount := strings.Count(queryLower, "or")
	
	if whereCount > 0 && (andCount > 2 || orCount > 1) {
		recommendations = append(recommendations, "Complex WHERE conditions may benefit from composite indexes")
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "Query performance appears optimal")
	}
	
	return recommendations
}

// OptimizeDatabase performs comprehensive database optimization
func (dim *DatabaseIndexManager) OptimizeDatabase() error {
	// Create optimal indexes
	if err := dim.CreateOptimalIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}
	
	// Analyze database statistics
	if err := dim.UpdateStatistics(); err != nil {
		return fmt.Errorf("failed to update statistics: %w", err)
	}
	
	// Vacuum database to reclaim space and optimize storage
	if err := dim.VacuumDatabase(); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	
	return nil
}

// UpdateStatistics updates SQLite statistics for query optimization
func (dim *DatabaseIndexManager) UpdateStatistics() error {
	// SQLite automatically maintains statistics, but we can trigger analysis
	_, err := dim.database.db.Exec("ANALYZE")
	if err != nil {
		return fmt.Errorf("failed to analyze database: %w", err)
	}
	
	return nil
}

// VacuumDatabase optimizes database storage
func (dim *DatabaseIndexManager) VacuumDatabase() error {
	_, err := dim.database.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	
	return nil
}

// GetDatabaseSize returns the current database size in bytes
func (dim *DatabaseIndexManager) GetDatabaseSize() (int64, error) {
	var pageCount, pageSize int64
	
	// Get page count
	err := dim.database.db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get page count: %w", err)
	}
	
	// Get page size
	err = dim.database.db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get page size: %w", err)
	}
	
	return pageCount * pageSize, nil
}

// GetTableStats returns statistics for all tables
func (dim *DatabaseIndexManager) GetTableStats() (map[string]TableStats, error) {
	tables := []string{"conversations", "tool_executions", "file_operations"}
	stats := make(map[string]TableStats)
	
	for _, table := range tables {
		stat, err := dim.getTableStats(table)
		if err != nil {
			continue // Skip tables that might not exist
		}
		stats[table] = stat
	}
	
	return stats, nil
}

// TableStats represents statistics for a database table
type TableStats struct {
	RowCount    int64 `json:"row_count"`
	TableSize   int64 `json:"table_size"`
	IndexCount  int   `json:"index_count"`
	LastUpdated time.Time `json:"last_updated"`
}

// getTableStats returns statistics for a specific table
func (dim *DatabaseIndexManager) getTableStats(tableName string) (TableStats, error) {
	stats := TableStats{
		LastUpdated: time.Now(),
	}
	
	// Get row count
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := dim.database.db.QueryRow(query).Scan(&stats.RowCount)
	if err != nil {
		return stats, fmt.Errorf("failed to get row count for table %s: %w", tableName, err)
	}
	
	// Get index count
	query = `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'index' AND tbl_name = ? AND name NOT LIKE 'sqlite_%'`
		
	err = dim.database.db.QueryRow(query, tableName).Scan(&stats.IndexCount)
	if err != nil {
		return stats, fmt.Errorf("failed to get index count for table %s: %w", tableName, err)
	}
	
	return stats, nil
}

// RecommendIndexes analyzes query patterns and recommends new indexes
func (dim *DatabaseIndexManager) RecommendIndexes(queryLog []string) ([]IndexRecommendation, error) {
	var recommendations []IndexRecommendation
	
	// Analyze common query patterns
	patterns := dim.analyzeQueryPatterns(queryLog)
	
	// Generate index recommendations based on patterns
	for pattern, frequency := range patterns {
		if frequency >= 5 { // Only recommend for frequently used patterns
			recommendation := IndexRecommendation{
				Pattern:     pattern,
				Frequency:   frequency,
				Priority:    dim.calculatePriority(pattern, frequency),
				IndexName:   dim.generateIndexName(pattern),
				Columns:     dim.extractColumnsFromPattern(pattern),
				EstimatedBenefit: dim.estimateBenefit(pattern, frequency),
			}
			
			recommendations = append(recommendations, recommendation)
		}
	}
	
	return recommendations, nil
}

// IndexRecommendation represents a recommended index
type IndexRecommendation struct {
	Pattern          string    `json:"pattern"`
	Frequency        int       `json:"frequency"`
	Priority         int       `json:"priority"`
	IndexName        string    `json:"index_name"`
	Columns          []string  `json:"columns"`
	EstimatedBenefit string    `json:"estimated_benefit"`
}

// analyzeQueryPatterns analyzes query patterns from logs
func (dim *DatabaseIndexManager) analyzeQueryPatterns(queryLog []string) map[string]int {
	patterns := make(map[string]int)
	
	for _, query := range queryLog {
		queryLower := strings.ToLower(query)
		
		// Extract WHERE clause patterns
		if whereIdx := strings.Index(queryLower, "where"); whereIdx != -1 {
			whereClause := queryLower[whereIdx:]
			// Normalize the WHERE clause
			normalized := dim.normalizeWhereClause(whereClause)
			patterns[normalized]++
		}
		
		// Extract ORDER BY patterns
		if orderIdx := strings.Index(queryLower, "order by"); orderIdx != -1 {
			orderClause := queryLower[orderIdx:]
			if limitIdx := strings.Index(orderClause, "limit"); limitIdx != -1 {
				orderClause = orderClause[:limitIdx]
			}
			patterns[strings.TrimSpace(orderClause)]++
		}
	}
	
	return patterns
}

// normalizeWhereClause normalizes WHERE clauses for pattern matching
func (dim *DatabaseIndexManager) normalizeWhereClause(whereClause string) string {
	// Replace parameter placeholders with generic markers
	normalized := whereClause
	normalized = strings.ReplaceAll(normalized, "?", "PARAM")
	normalized = strings.ReplaceAll(normalized, "'", "")
	
	// Remove specific values, keep only structure
	words := strings.Fields(normalized)
	var result []string
	
	for i, word := range words {
		if word == "=" || word == ">" || word == "<" || word == ">=" || word == "<=" || word == "!=" {
			result = append(result, word)
			if i+1 < len(words) {
				result = append(result, "PARAM")
			}
		} else if word != "PARAM" {
			result = append(result, word)
		}
	}
	
	return strings.Join(result, " ")
}

// calculatePriority calculates the priority of an index recommendation
func (dim *DatabaseIndexManager) calculatePriority(pattern string, frequency int) int {
	priority := frequency
	
	// Boost priority for certain patterns
	if strings.Contains(pattern, "timestamp") {
		priority += 10
	}
	if strings.Contains(pattern, "status") {
		priority += 8
	}
	if strings.Contains(pattern, "conversation_id") {
		priority += 6
	}
	
	return priority
}

// generateIndexName generates a descriptive index name
func (dim *DatabaseIndexManager) generateIndexName(pattern string) string {
	// Extract table name and key columns to generate meaningful name
	words := strings.Fields(pattern)
	
	var tableName, columns string
	for i, word := range words {
		if word == "from" && i+1 < len(words) {
			tableName = words[i+1]
		}
		if word == "where" && i+1 < len(words) {
			columns = words[i+1]
		}
	}
	
	if tableName == "" {
		tableName = "table"
	}
	if columns == "" {
		columns = "column"
	}
	
	return fmt.Sprintf("idx_%s_%s_optimized", tableName, columns)
}

// extractColumnsFromPattern extracts column names from a query pattern
func (dim *DatabaseIndexManager) extractColumnsFromPattern(pattern string) []string {
	var columns []string
	words := strings.Fields(pattern)
	
	// Look for column names in WHERE and ORDER BY clauses
	for i, word := range words {
		if (word == "where" || word == "and" || word == "or") && i+1 < len(words) {
			nextWord := words[i+1]
			if nextWord != "(" && nextWord != ")" {
				columns = append(columns, nextWord)
			}
		}
		if word == "by" && i+1 < len(words) {
			nextWord := words[i+1]
			if nextWord != "(" && nextWord != ")" {
				columns = append(columns, nextWord)
			}
		}
	}
	
	return dim.removeDuplicates(columns)
}

// removeDuplicates removes duplicate strings from a slice
func (dim *DatabaseIndexManager) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// estimateBenefit estimates the performance benefit of an index
func (dim *DatabaseIndexManager) estimateBenefit(pattern string, frequency int) string {
	if frequency > 50 {
		return "High - Frequently used query pattern"
	} else if frequency > 20 {
		return "Medium - Moderately used query pattern"
	} else if frequency > 5 {
		return "Low - Occasionally used query pattern"
	}
	
	return "Minimal - Rarely used query pattern"
}