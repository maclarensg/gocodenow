# gocodenow User Guide

## Overview

gocodenow is a terminal-based user interface (TUI) that provides an open-source alternative to Claude Code, designed to work with multiple LLM providers via OpenAI-compatible APIs. It offers conversation-based workflows similar to Claude Code while supporting local models and custom endpoints.

## Installation

### Prerequisites

- Go 1.21 or later
- Git

### From Source

```bash
git clone https://github.com/your-org/gocodenow
cd gocodenow
go build -o gocodenow ./cmd/gocodenow
```

### Using devbox (Recommended for Development)

```bash
devbox shell
go run ./cmd/gocodenow
```

## Quick Start

1. **Start gocodenow**:
   ```bash
   ./gocodenow
   ```

2. **Basic Navigation**:
   - Use **Tab** to switch between conversation history and input
   - Use **↑↓** arrow keys to navigate conversations
   - Press **Enter** to send messages or expand conversation details
   - Press **Ctrl+C** to exit

3. **Write your first message**:
   Type in the input area and press Enter to send.

## Configuration

gocodenow supports various LLM providers through OpenAI-compatible APIs:

### Configuration File

Create a `config.yaml` file in your working directory:

```yaml
llm:
  endpoint: "http://localhost:8080/v1/chat/completions"  # Your API endpoint
  token: "your-api-key-here"                            # API key
  model: "llama-2-7b-chat"                             # Model name
  timeout: 30                                          # Request timeout in seconds

storage:
  database_path: "./conversations.db"                   # SQLite database path
  cache_size: 100                                      # Number of conversations to cache
  max_memory_mb: 256                                   # Memory limit for cache

security:
  max_file_size: 10485760                             # Max file size (10MB)
  allowed_file_types: [".txt", ".md", ".py", ".js"]   # Allowed file extensions
```

### Environment Variables

You can also use environment variables:

```bash
export LLM_ENDPOINT="http://localhost:8080/v1/chat/completions"
export LLM_TOKEN="your-api-key"
export LLM_MODEL="gpt-3.5-turbo"
```

### Popular LLM Provider Examples

#### OpenAI
```yaml
llm:
  endpoint: "https://api.openai.com/v1/chat/completions"
  token: "sk-your-openai-key"
  model: "gpt-4"
```

#### Anthropic (Claude via OpenAI API)
```yaml
llm:
  endpoint: "https://api.anthropic.com/v1/messages"
  token: "sk-ant-your-anthropic-key" 
  model: "claude-3-sonnet-20240229"
```

#### Local Ollama
```yaml
llm:
  endpoint: "http://localhost:11434/v1/chat/completions"
  token: ""  # Usually not needed for local models
  model: "llama2:7b"
```

#### LM Studio
```yaml
llm:
  endpoint: "http://localhost:1234/v1/chat/completions"
  token: ""
  model: "local-model"
```

## User Interface

### Layout

gocodenow uses a three-panel layout:

1. **Header** (Top): Shows current model, status, and connection info
2. **Conversation History** (Middle): Scrollable list of previous conversations
3. **Input Area** (Bottom): Text input for new messages

### Navigation Controls

| Key | Action |
|-----|---------|
| **Tab** | Toggle between conversation history and input focus |
| **↑ ↓** | Navigate up/down in conversation history |
| **Enter** | Send message (in input) / Expand conversation (in history) |
| **Ctrl+Enter** | Insert new line in input |
| **Home** | Jump to first conversation |
| **End** | Jump to last conversation |
| **Ctrl+C** | Exit application |

### Conversation Management

#### Viewing Conversations

- Conversations appear in chronological order (newest at bottom)
- Each conversation shows:
  - Timestamp
  - User input (truncated if long)
  - LLM response status
  - Tool usage indicators

#### Expanding Conversations

Press **Enter** on a selected conversation to view:
- Full user input
- Complete LLM response
- Tool calls and results
- File operations performed
- Execution time and token usage

#### Conversation Storage

- Conversations are automatically saved to SQLite database
- Up to 100 conversations kept in memory cache
- Older conversations loaded on-demand
- Conversation history persists between sessions

## Tool Usage

gocodenow supports various tools that the LLM can use to interact with your system:

### File Operations

#### Reading Files
```
Read the contents of package.json
```

The LLM can read files from your current directory and subdirectories.

#### Writing Files
```
Create a Python script that prints "Hello, World!"
```

The LLM can create new files or overwrite existing ones.

#### Editing Files
```
Add error handling to the main function in server.py
```

The LLM can modify existing files with surgical precision.

### System Operations

#### Running Commands
```
Run the test suite and show me the results
```

The LLM can execute shell commands and return the output.

#### Git Operations
```
Show me the git status and commit the changes
```

Built-in git integration for version control operations.

### Security Considerations

- File operations are restricted to the current working directory tree
- File size limits prevent excessive memory usage
- Command execution uses secure subprocess handling
- All operations are logged for audit purposes

## Advanced Usage

### Multi-Step Workflows

gocodenow excels at multi-step tasks:

```
1. Create a new React component for user profiles
2. Add proper TypeScript types
3. Write unit tests for the component
4. Update the main App component to use it
```

The LLM will break this down into individual tool calls and execute them in sequence.

### Error Recovery

If a tool operation fails:
- Error details are shown in the conversation
- The LLM can suggest fixes or alternative approaches
- Conversation continues normally after errors

### Conversation Export

Export conversations for documentation or sharing:

1. Navigate to the conversation you want to export
2. Press **Ctrl+E** to export (implementation planned)
3. Choose format: Markdown, JSON, or plain text

## Troubleshooting

### Common Issues

#### "Connection refused" errors
- Check that your LLM provider endpoint is correct and accessible
- Verify your API key is valid and has sufficient credits/usage
- Test the endpoint manually with curl

#### File operation failures
- Ensure you have read/write permissions in the current directory
- Check that file paths don't contain invalid characters
- Verify files aren't locked by other processes

#### Slow performance
- Reduce cache size in configuration
- Use a local LLM provider for faster responses
- Check available memory and disk space

#### Conversation history not loading
- Check that the database file is writable
- Ensure sufficient disk space for the database
- Try deleting the database file to start fresh (loses history)

### Debug Mode

Enable debug logging:

```bash
export DEBUG=1
./gocodenow
```

This provides detailed logging of:
- API requests and responses
- Tool execution details
- Database operations
- Memory usage statistics

### Performance Optimization

#### For Local Models
- Use models sized appropriately for your hardware
- Adjust context window size based on available RAM
- Consider using quantized models for faster inference

#### For Remote APIs
- Implement request batching for multiple operations
- Use streaming responses where supported
- Cache frequently accessed data locally

## Support and Contributing

### Getting Help

- Check the [GitHub Issues](https://github.com/your-org/gocodenow/issues) for known problems
- Search existing discussions for similar questions
- Create a new issue with detailed information about your problem

### Contributing

We welcome contributions! See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

### License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.

---

*Last updated: $(date)*