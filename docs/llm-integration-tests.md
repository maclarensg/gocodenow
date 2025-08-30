# LLM Integration Tests

This package includes comprehensive integration tests that test the LLM clients against real LLM backends.

## Overview

The integration tests verify:
- Client creation and configuration
- Model listing and information retrieval
- Simple chat completions
- Tool calling functionality  
- Streaming responses
- Error handling and timeouts
- Performance benchmarking

## Configuration

The tests are configured using environment variables:

### Required Environment Variables

```bash
# Enable integration tests (required to run them)
export RUN_LLM_INTEGRATION_TESTS=true
```

### Optional Environment Variables

```bash
# LLM endpoint (defaults to http://192.168.86.20:1234)
export LLM_TEST_ENDPOINT="http://your-llm-server:port"

# Model name (defaults to qwen2.5-coder-7b-instruct)
export LLM_TEST_MODEL="your-model-name"

# API token (optional, for authenticated endpoints)
export LLM_TEST_TOKEN="your-api-token"
```

## Running the Tests

### Basic Integration Tests

```bash
# Set the environment variable to enable integration tests
export RUN_LLM_INTEGRATION_TESTS=true

# Run all integration tests
go test -v ./internal/llm -run Integration

# Run a specific integration test
go test -v ./internal/llm -run TestIntegration_SimpleChatCompletion
```

### With Custom Configuration

```bash
# Configure for your local setup
export RUN_LLM_INTEGRATION_TESTS=true
export LLM_TEST_ENDPOINT="http://192.168.86.20:1234"
export LLM_TEST_MODEL="qwen2.5-coder-7b-instruct"

# Run the tests
go test -v ./internal/llm -run Integration
```

### Performance Benchmarks

```bash
# Run benchmark tests
export RUN_LLM_INTEGRATION_TESTS=true
go test -bench=BenchmarkIntegration -v ./internal/llm
```

## Test Coverage

### TestIntegration_ClientFactory_CreateClient
- Tests client creation with the factory pattern
- Verifies provider detection and configuration
- Validates client setup and configuration propagation

### TestIntegration_ListModels
- Tests the `/v1/models` endpoint
- Verifies model listing functionality
- Checks if the configured test model is available

### TestIntegration_GetModel
- Tests model information retrieval
- Validates model metadata and properties

### TestIntegration_SimpleChatCompletion
- Tests basic chat completion without tools
- Verifies request/response format
- Checks token usage tracking
- Tests with a simple math question to verify functionality

### TestIntegration_ChatCompletionWithTools
- Tests tool calling functionality
- Defines a calculator tool for the LLM to use
- Verifies tool call parsing and parameter extraction
- Tests the complete tool calling workflow

### TestIntegration_StreamingChatCompletion
- Tests streaming chat responses
- Verifies Server-Sent Events parsing
- Tests real-time response processing
- Accumulates streamed content

### TestIntegration_ErrorHandling
- Tests error handling with invalid models
- Verifies proper error classification and context
- Tests graceful degradation

### TestIntegration_ConnectionTimeout
- Tests timeout handling
- Verifies context cancellation
- Tests network error scenarios

### BenchmarkIntegration_SimpleChatCompletion
- Performance benchmark for chat completions
- Measures request/response latency
- Useful for performance regression testing

## Expected Behavior

### Successful Test Run Output

```
=== RUN   TestIntegration_ClientFactory_CreateClient
    integration_test.go:89: Successfully created client for provider: local
--- PASS: TestIntegration_ClientFactory_CreateClient (0.01s)

=== RUN   TestIntegration_ListModels  
    integration_test.go:121: Available model: qwen2.5-coder-7b-instruct
--- PASS: TestIntegration_ListModels (0.15s)

=== RUN   TestIntegration_SimpleChatCompletion
    integration_test.go:202: Chat completion successful!
    integration_test.go:203: Response: 4
    integration_test.go:204: Token usage: 15 prompt + 2 completion = 17 total
--- PASS: TestIntegration_SimpleChatCompletion (2.31s)
```

## Troubleshooting

### Common Issues

1. **Tests are skipped**: Make sure `RUN_LLM_INTEGRATION_TESTS=true` is set
2. **Connection refused**: Verify your LLM server is running at the configured endpoint
3. **Model not found**: Check that the model name matches what's available on your server
4. **Timeout errors**: Increase timeout or check server performance

### Debugging

Enable verbose output:
```bash
go test -v ./internal/llm -run Integration
```

Check endpoint connectivity:
```bash
curl http://192.168.86.20:1234/v1/models
```

### Server Requirements

The integration tests expect an OpenAI-compatible API server with:
- `/v1/models` endpoint for model listing
- `/v1/chat/completions` endpoint for chat completions
- Support for streaming responses (optional)
- Tool calling support (optional, but tested)

### Compatible Servers

These tests work with:
- **LocalAI**: Local OpenAI-compatible server
- **Ollama**: When run with OpenAI-compatible API mode
- **Text Generation Web UI**: With OpenAI extension
- **vLLM**: OpenAI-compatible server
- **OpenAI API**: Official OpenAI endpoints
- **Anthropic API**: When using the Anthropic provider

## Security Notes

- API tokens are only sent if the `LLM_TEST_TOKEN` environment variable is set
- Tests use read-only operations where possible
- No sensitive data is sent to the LLM endpoints
- All test prompts are simple and non-sensitive

## CI/CD Integration

To run these tests in CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Run LLM Integration Tests
  env:
    RUN_LLM_INTEGRATION_TESTS: true
    LLM_TEST_ENDPOINT: ${{ secrets.LLM_TEST_ENDPOINT }}
    LLM_TEST_TOKEN: ${{ secrets.LLM_TEST_TOKEN }}
  run: go test -v ./internal/llm -run Integration
```

Note: Only run integration tests when you have access to a test LLM server, as they require external network connectivity.