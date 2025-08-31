# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gocodenow is a terminal-based user interface (TUI) that serves as an open-source alternative to Claude Code, designed to work with multiple LLM providers via OpenAI-compatible APIs. The project aims to replicate Claude Code's conversation-based workflow and user experience while supporting local models and custom endpoints.

## Build & Run Commands

### Building
```bash
go build -o gocodenow ./cmd/gocodenow
```

### Running
```bash
./gocodenow
# or
go run ./cmd/gocodenow
```

### Development Environment
The project uses devbox for environment management:
```bash
devbox shell
```

## Architecture Overview

The codebase follows a clean layered architecture pattern:

### Core Components

- **`cmd/gocodenow/main.go`**: Entry point that initializes and runs the application
- **`internal/app/`**: Application layer that orchestrates the TUI using Bubble Tea framework
- **`internal/ui/`**: TUI implementation with Bubble Tea models, views, and event handling
  - `model.go`: Main TUI model with viewport and textarea components
  - `keys.go`: Keyboard event handling and navigation logic
  - `renderer.go`: UI rendering with fixed header/footer layout using lipgloss
- **`internal/models/`**: Domain models for conversation history management
  - `conversation.go`: ConversationHistory with FIFO buffer (100 blocks max) and mock response system

### Key Architectural Patterns

1. **Bubble Tea Framework**: Built on Charm's Bubble Tea for TUI functionality with:
   - Model-View-Update (MVU) pattern
   - Message-based state updates
   - Component isolation using Bubbles library (textarea, viewport)

2. **Conversation-Centric Design**: The UI revolves around conversation blocks that can be:
   - Expanded/collapsed for detailed view
   - Navigated with keyboard shortcuts (selection always stays visible)
   - Display tool usage and file modifications
   - FIFO buffer management (100 conversation limit)

3. **Fixed Layout System**: Three-panel layout using lipgloss.JoinVertical:
   - **Header**: Fixed top bar with model info and status
   - **Viewport**: Scrollable conversation history with proper content management
   - **Footer**: Fixed input area with textarea and instructions

### TUI Interaction Model

- **Tab**: Toggle between conversation history and input focus
- **↑↓ Arrow Keys**: Navigate conversations (selection always stays visible)
- **Enter**: Send message (in input) or expand/collapse conversation block
- **Ctrl+Enter**: Insert new line in input
- **Home/End**: Jump to first/last conversation
- **Ctrl+C**: Exit application

### Current Implementation Status

✅ **COMPLETED FEATURES:**
- Fixed header/footer layout with responsive sizing
- Textarea component for multi-line input with paste support
- Viewport component for conversation scrolling
- Conversation navigation with selection always visible on screen
- Expand/collapse functionality for conversation details
- FIFO buffer (100 conversations max)
- Mock response system for development/testing
- Proper keyboard event handling

✅ **CRITICAL BUGS FIXED:**
- Selection cursor no longer goes off-screen
- Scroll-to-bottom functionality works properly
- Tab highlighting appears immediately
- Viewport scrolling state management corrected
- Height calculations match actual rendered content

🔄 **NEXT PRIORITIES:**
- Real LLM provider integration (OpenAI API, local models)
- File system operations and tool calls
- Configuration management for API keys and endpoints
- Enhanced conversation export/import functionality

### State Management

- **ConversationHistory**: Manages conversation blocks with expansion state, tool tracking, and file modification history using FIFO buffer
- **Viewport/Textarea Components**: Handle scrolling and input state using Bubble Tea's built-in components
- **MockResponse System**: Currently implements mock AI responses based on keyword detection for development/testing

## Development Patterns

### Adding New Features

1. Domain logic goes in `internal/models/`
2. UI components go in `internal/ui/`
3. External integrations go in `pkg/`
4. Follow the existing message-passing patterns for state updates

### Key Dependencies

- `github.com/charmbracelet/bubbletea`: Core TUI framework
- `github.com/charmbracelet/bubbles`: UI components (textarea, viewport)
- `github.com/charmbracelet/lipgloss`: Styling and layout

### Quality Assurance Notes

The TUI has been thoroughly tested for:
- Responsive layout across different terminal sizes
- Proper viewport scrolling without state conflicts
- Selection visibility and navigation reliability
- Multi-line input handling with paste support
- Memory management with FIFO buffer limits

The project is designed to be extensible for real LLM provider integration while maintaining the established UI/UX patterns that mirror Claude Code's functionality.

## Naming Convention

**IMPORTANT**: The project name is **"gocodenow"** (all lowercase, no spaces). Use this consistently in:
- File paths and directory names
- Configuration paths (`.gocodenow`, `.config/gocodenow`)  
- Database paths (`~/.local/share/gocodenow/`)
- Backup and temporary file naming (`gocodenow_backup_`, `gocodenow_test_`)
- User-facing text and UI elements
- Comments and documentation
- Export file naming (`gocodenow_export_`)
- Cache directories and prefixes

Never use the old name "lmcodenow" - it has been completely replaced with "gocodenow".

## Development Workflow Preferences

**IMPORTANT**: When adding new functionality:
- **Use Taskfile.yml**: Add new commands and workflows to the existing Taskfile.yml instead of creating standalone scripts
- **No separate scripts**: Avoid creating shell scripts in `scripts/` directory - integrate everything into the task runner
- **Consistent task naming**: Follow the existing pattern (`task:subtask` format)
- **Use existing patterns**: Follow the established structure for integration tests, benchmarks, etc.

Examples:
- ✅ Add `task benchmark:tools` to Taskfile.yml
- ❌ Create `scripts/run-benchmarks.sh`

## Documentation Standards for TUI Applications

**IMPORTANT**: gocodenow is a **Terminal User Interface (TUI) application**, not an API service. Documentation should reflect this:

**Required Documentation:**
- ✅ **User Guide**: How to install, configure, and use the TUI
- ✅ **Developer/Contributor Guide**: Architecture, development setup, contribution workflow
- ✅ **Performance Benchmarks**: Measure and optimize TUI performance, file operations, storage

**NOT Required for TUI Apps:**
- ❌ **API Reference**: gocodenow doesn't expose APIs - it's a standalone terminal application
- ❌ **REST API Docs**: Not applicable for terminal applications
- ❌ **SDK Documentation**: Users interact directly with the TUI, not through SDKs

When planning documentation tasks, focus on:
1. User experience with the terminal interface
2. Development patterns for TUI components (Bubble Tea)  
3. Performance of file operations and storage systems
4. Tool execution and security frameworks

Avoid generic "API documentation" tasks that don't apply to terminal applications.