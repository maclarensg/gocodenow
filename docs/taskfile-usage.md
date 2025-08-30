# Taskfile Usage Guide

This project uses [Task](https://taskfile.dev) for build automation and testing.

## Installation

If you don't have Task installed:
```bash
# Install Task (choose one method)
go install github.com/go-task/task/v3/cmd/task@latest
# or
curl -sL https://taskfile.dev/install.sh | sh
```

## Available Tasks

```bash
# List all tasks
task

# Run all Go tests  
task test

# Build the application
task build

# Run integration test suite
task itest
```

## Integration Testing

### Basic Usage

```bash
# Run all integration tests (uses defaults)
task itest
```

**Default Configuration:**
- Endpoint: `http://192.168.86.20:1234`
- Model: `qwen2.5-coder-7b-instruct`
- Token: (not set)

### Custom Configuration

```bash
# Use different endpoint and model
ENDPOINT="http://localhost:8080" MODEL="llama2" task itest

# With authentication token
ENDPOINT="https://api.openai.com" MODEL="gpt-4" TOKEN="sk-..." task itest

# Multiple variables
ENDPOINT="http://192.168.1.100:1234" MODEL="dolphin-llama3" TOKEN="abc123" task itest
```

### Environment Variables

You can also set these as environment variables:

```bash
export LLM_TEST_ENDPOINT="http://192.168.86.20:1234"
export LLM_TEST_MODEL="qwen2.5-coder-7b-instruct"  
export LLM_TEST_TOKEN="optional-token"

# Then just run
task itest
```

## Examples

### Development Workflow

```bash
# 1. Build and test
task build
task test

# 2. Run integration tests if you have LLM server running
task itest

# 3. Check everything works
./gocodenow
```

### CI/CD Usage

```bash
# In your CI pipeline
task test                    # Unit tests
task build                  # Build binary

# Integration tests (only if LLM server available)
if [[ "$RUN_INTEGRATION_TESTS" == "true" ]]; then
  ENDPOINT="$CI_LLM_ENDPOINT" MODEL="$CI_LLM_MODEL" task itest
fi
```

### Testing Different LLM Providers

```bash
# Test with OpenAI
ENDPOINT="https://api.openai.com" MODEL="gpt-3.5-turbo" TOKEN="sk-..." task itest

# Test with Anthropic  
ENDPOINT="https://api.anthropic.com" MODEL="claude-3-sonnet-20240229" TOKEN="sk-ant-..." task itest

# Test with local server
ENDPOINT="http://localhost:1234" MODEL="local-model" task itest
```

## Troubleshooting

### Server Not Reachable
```
❌ LLM server is not reachable at http://192.168.86.20:1234/v1
```
**Solutions:**
1. Check if your LLM server is running
2. Verify the endpoint URL is correct
3. Check network connectivity/firewall

### Integration Tests Skip
If you see tests being skipped, make sure the server check passes first.

### Custom Configurations Not Working
Make sure you're passing variables correctly:
```bash
# ✅ Correct
ENDPOINT="http://localhost:8080" task itest

# ❌ Incorrect  
task itest ENDPOINT="http://localhost:8080"
```

## Task vs Scripts

The Taskfile replaces the shell script with these advantages:

| Feature | Shell Script | Taskfile |
|---------|--------------|----------|
| **Platform** | Bash only | Cross-platform |
| **Syntax** | Shell scripting | YAML configuration |
| **Dependencies** | Bash + curl | Task binary |
| **Variables** | Complex env var handling | Simple variable system |
| **Error Handling** | Manual | Built-in |
| **Task Dependencies** | Manual | Automatic |

## Internal Tasks

The following tasks are internal (not directly callable):
- `itest:check-server` - Check server connectivity
- `itest:run-tests` - Execute the tests
- `itest:specific` - Run specific test
- `itest:benchmark` - Run benchmarks
- `itest:check-only` - Connectivity check only

These are used internally by the `itest` task and cannot be called directly.