# gocodenow Engineering Design Proposal

## Executive Summary

This document outlines the engineering design for evolving gocodenow from a basic TUI chat interface into a fully-featured Claude Code alternative with agentic capabilities, persistent storage, and comprehensive tool integration.

## 1. Dialogue Storage & Memory Management

### Current State
- In-memory FIFO buffer (100 conversations max)
- Ephemeral storage (lost on restart)
- Mock response system for development

### Proposed Architecture

#### 1.1 Hybrid Storage Model
```
┌─ Memory Layer ────────────────────┐
│  Active Session (50 conversations) │
│  - Fast access for UI             │
│  - Real-time updates              │
└───────────────────────────────────┘
           ↕ sync
┌─ Persistence Layer ───────────────┐
│  SQLite Database                  │
│  - conversation_history           │
│  - tool_executions               │
│  - file_operations               │
│  - session_metadata              │
└───────────────────────────────────┘
           ↕ backup
┌─ Logging Layer ───────────────────┐
│  Structured Logs (JSON)          │
│  - Debug/audit trail             │
│  - Error tracking                │
│  - Performance metrics           │
└───────────────────────────────────┘
```

#### 1.2 Storage Implementation
```go
type StorageManager struct {
    memoryCache    *ConversationCache    // 50 most recent
    database      *sql.DB               // SQLite for persistence  
    logger        *structured.Logger    // JSON logs
    maxMemoryMB   int                   // Configurable limit
}

type ConversationRecord struct {
    ID          uuid.UUID
    Timestamp   time.Time
    UserInput   string
    LLMResponse string
    ToolCalls   []ToolExecution
    FileChanges []FileOperation
    ModelUsed   string
    TokenUsage  TokenMetrics
}
```

#### 1.3 Memory Management Strategy
- **Default Memory Limit**: 100MB for conversation data
- **Cleanup Policy**: LRU eviction when memory limit reached
- **Persistence Trigger**: Auto-save every 10 conversations or 5 minutes
- **Session Recovery**: Load last 20 conversations on startup

## 2. Agentic Capabilities Design

### 2.1 Tool Execution Framework

#### Architecture Overview
```
┌─ LLM Response Parser ─────────────┐
│  Parse tool calls from response   │ 
│  Validate tool parameters        │
│  Security checks                 │
└───────────────────────────────────┘
           ↓
┌─ Tool Router ─────────────────────┐
│  Route to appropriate executor    │
│  Handle permissions              │
│  Manage execution context        │
└───────────────────────────────────┘
           ↓
┌─ Tool Executors ──────────────────┐
│  FileOperations  | SystemCommands │
│  SearchTools     | WebRequests    │
│  CodeAnalysis    | TestRunner     │
└───────────────────────────────────┘
           ↓
┌─ Result Aggregator ───────────────┐
│  Collect execution results        │
│  Handle errors gracefully        │
│  Prepare feedback for LLM        │
└───────────────────────────────────┘
```

#### 2.2 Tool Call Format
Following OpenAI function calling standard:
```json
{
  "role": "assistant",
  "content": "I'll help you read that file.",
  "tool_calls": [
    {
      "id": "call_123",
      "type": "function", 
      "function": {
        "name": "read_file",
        "arguments": "{\"path\": \"src/main.go\"}"
      }
    }
  ]
}
```

#### 2.3 Security Framework
```go
type SecurityPolicy struct {
    AllowedCommands    []string          // Whitelist approach
    ForbiddenPaths     []string          // Protect sensitive dirs
    MaxExecutionTime   time.Duration     // Prevent infinite loops
    RequireConfirm     []string          // Destructive operations
    SandboxMode        bool              // Isolated execution
}

type ToolExecutor interface {
    Execute(ctx context.Context, params ToolParams) (*ToolResult, error)
    Validate(params ToolParams) error
    RequiresConfirmation() bool
}
```

### 2.4 Supported Tools (Phase 1)
- **`read_file`**: Read file contents with syntax highlighting
- **`write_file`**: Create/overwrite files with backup
- **`edit_file`**: Apply precise edits with diff preview  
- **`bash_command`**: Execute shell commands with output capture
- **`search_files`**: Search codebase with regex/glob patterns
- **`list_directory`**: Navigate project structure
- **`git_operations`**: Basic git commands (status, diff, commit)

## 3. Claude Code-like Experience

### 3.1 Core Features Mapping

| Claude Code Feature | gocodenow Implementation |
|-------------------|------------------------|
| Conversation Threading | Enhanced conversation blocks with tool execution history |
| File Operations | Tool execution framework with file watchers |
| Project Context | Workspace detection and .gitignore awareness |
| Code Execution | Sandboxed bash tool with timeout protection |
| Multi-turn Interactions | Conversation state persistence with tool results |
| Syntax Highlighting | Integration with chroma/pygments for file display |

### 3.2 Enhanced UI Components

#### Conversation Block Evolution
```go
type ConversationBlock struct {
    // Existing fields
    ID        uuid.UUID
    User      string  
    Assistant string
    Timestamp time.Time
    Expanded  bool
    
    // New agentic fields
    ToolCalls      []ToolCall
    ToolResults    []ToolResult
    FileOperations []FileChange
    ExecutionTime  time.Duration
    TokenUsage     TokenMetrics
    Status         ConversationStatus // pending, executing, completed, error
}

type ToolResult struct {
    ToolID      string
    Success     bool
    Output      string
    Error       string
    FilesModified []string
    Duration    time.Duration
}
```

#### Real-time Tool Execution Display
```
┌─ Conversation Block ──────────────────────────────────────┐
│ 🤖 Assistant: I'll help you implement that feature.       │
│                                                           │
│ ┌─ TOOL EXECUTIONS ─────────────────────────────────────┐ │
│ │ [1] read_file(src/main.go) ✓ 125 lines               │ │
│ │ [2] edit_file(src/main.go:45-50) ✓ Applied changes   │ │  
│ │ [3] bash_command(go build) ⏳ Running...              │ │
│ └───────────────────────────────────────────────────────┘ │
│                                                           │
│ I've updated the main function and I'm now building...   │
└───────────────────────────────────────────────────────────┘
```

### 3.3 Project Context Awareness

#### Workspace Detection
```go
type ProjectContext struct {
    RootPath        string
    Language        string        // Auto-detect from files
    BuildSystem     BuildType     // go.mod, package.json, etc.
    GitRepository   *GitInfo      // Branch, status, etc.
    ConfigFiles     []string      // .env, config files
    IgnorePatterns  []string      // From .gitignore, .clignore
}

func DetectProject(path string) (*ProjectContext, error) {
    // Walk up directory tree looking for indicators
    // Parse .gitignore, go.mod, package.json, etc.
    // Return rich context for LLM prompts
}
```

## 4. Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)
- [ ] SQLite persistence layer
- [ ] Basic tool execution framework  
- [ ] File operations (read, write, edit)
- [ ] Enhanced conversation storage

### Phase 2: Agentic Core (Weeks 3-4)  
- [ ] LLM response parser for tool calls
- [ ] Bash command executor with sandboxing
- [ ] Real-time tool execution UI updates
- [ ] Error handling and recovery

### Phase 3: Advanced Features (Weeks 5-6)
- [ ] Project context detection
- [ ] Git integration 
- [ ] Syntax highlighting for file display
- [ ] Search and navigation tools

### Phase 4: Polish & Security (Weeks 7-8)
- [ ] Comprehensive security policies
- [ ] Performance optimization
- [ ] User confirmation flows
- [ ] Export/import functionality

## 5. Technical Considerations

### 5.1 Performance
- **Lazy Loading**: Load conversation details on demand
- **Streaming**: Show tool output in real-time
- **Caching**: Cache file contents and search results
- **Debouncing**: Batch UI updates during rapid tool execution

### 5.2 Security
- **Sandboxing**: Execute tools in restricted environment
- **Path Validation**: Prevent directory traversal attacks
- **Command Filtering**: Whitelist safe commands
- **User Consent**: Confirm destructive operations

### 5.3 Error Handling
- **Graceful Degradation**: Continue on tool failures
- **Retry Logic**: Auto-retry transient failures
- **User Feedback**: Clear error messages and recovery options
- **Rollback**: Undo file changes on errors

## 6. Data Architecture

### 6.1 SQLite Schema
```sql
CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    timestamp DATETIME,
    user_input TEXT,
    llm_response TEXT,
    model_name TEXT,
    token_usage_input INTEGER,
    token_usage_output INTEGER,
    status TEXT
);

CREATE TABLE tool_executions (
    id TEXT PRIMARY KEY,
    conversation_id TEXT,
    tool_name TEXT,
    parameters TEXT,
    result TEXT,
    success BOOLEAN,
    duration_ms INTEGER,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id)
);

CREATE TABLE file_operations (
    id TEXT PRIMARY KEY,
    tool_execution_id TEXT,
    operation_type TEXT, -- read, write, edit, delete
    file_path TEXT,
    old_content TEXT,
    new_content TEXT,
    FOREIGN KEY (tool_execution_id) REFERENCES tool_executions(id)
);
```

### 6.2 Configuration Management
```yaml
# ~/.config/gocodenow/config.yaml
storage:
  max_memory_mb: 100
  sqlite_path: "~/.local/share/gocodenow/conversations.db"
  backup_interval_minutes: 5

security:
  sandbox_mode: true
  allowed_commands: ["ls", "cat", "grep", "find", "git"]
  forbidden_paths: ["/etc", "/usr", "~/.ssh"]
  require_confirmation: ["rm", "mv", "sudo"]

ui:
  syntax_highlighting: true
  real_time_updates: true
  max_tool_output_lines: 100
```

## Conclusion

This design transforms gocodenow from a simple chat interface into a powerful agentic development tool that rivals Claude Code's capabilities while maintaining the advantages of local execution and customization. The hybrid storage model ensures both performance and persistence, while the comprehensive tool framework enables sophisticated multi-turn interactions with full system integration.

The modular architecture allows for incremental implementation and future extensibility, making gocodenow a robust foundation for AI-assisted development workflows.