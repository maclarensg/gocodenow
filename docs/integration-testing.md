# Testing with Real LLM Backend

This document explains how to test the LLM integration with a real LLM backend server.

## Quick Start

### 1. Using the Script (Recommended)

```bash
# Make sure your LLM server is running at http://192.168.86.20:1234
# with the qwen2.5-coder-7b-instruct model loaded

# Run all integration tests
./scripts/run_integration_tests.sh

# Run with verbose output
./scripts/run_integration_tests.sh -v

# Check connectivity only
./scripts/run_integration_tests.sh -c

# Run a specific test
./scripts/run_integration_tests.sh -s SimpleChatCompletion

# Run benchmarks
./scripts/run_integration_tests.sh -b
```

### 2. Using Environment Variables

```bash
# Set required environment variable
export RUN_LLM_INTEGRATION_TESTS=true

# Configure your setup (optional - defaults provided)
export LLM_TEST_ENDPOINT="http://192.168.86.20:1234"
export LLM_TEST_MODEL="qwen2.5-coder-7b-instruct"

# Run the tests
go test -v ./internal/llm -run Integration
```

### 3. Using Custom Configuration

```bash
# For different endpoints/models
export RUN_LLM_INTEGRATION_TESTS=true
export LLM_TEST_ENDPOINT="http://localhost:8080"
export LLM_TEST_MODEL="your-model-name"
export LLM_TEST_TOKEN="your-api-token"  # if needed

go test -v ./internal/llm -run Integration
```

## Test Coverage

The integration tests include:

### ✅ Basic Functionality
- **Client Creation**: Tests factory pattern and provider detection
- **Model Listing**: Verifies `/v1/models` endpoint  
- **Model Info**: Tests model information retrieval
- **Simple Chat**: Basic chat completion without tools

### ✅ Advanced Features  
- **Tool Calling**: Tests LLM tool call parsing with a calculator tool
- **Streaming**: Tests real-time streaming responses
- **Error Handling**: Tests with invalid models and error scenarios
- **Timeouts**: Tests connection timeout handling

### ✅ Performance
- **Benchmarks**: Performance testing for chat completions

## Expected Test Output

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

=== RUN   TestIntegration_ChatCompletionWithTools
    integration_test.go:289: Model made 1 tool call(s)
    integration_test.go:292: Tool call 1: ID=call_123, Type=function, Function=calculate
    integration_test.go:299: Tool call arguments: map[a:15 b:27 operation:add]
--- PASS: TestIntegration_ChatCompletionWithTools (3.45s)

=== RUN   TestIntegration_StreamingChatCompletion
    integration_test.go:394: Stream event 1: Type=content
    integration_test.go:408: Total events received: 12
    integration_test.go:409: Accumulated content: 1\n2\n3\n4\n5
--- PASS: TestIntegration_StreamingChatCompletion (4.12s)
```

## Server Requirements

Your LLM server needs to support:

### Required Endpoints
- `GET /v1/models` - List available models
- `POST /v1/chat/completions` - Chat completions
- Support for OpenAI-compatible API format

### Optional (But Tested)
- Streaming responses via Server-Sent Events
- Tool/function calling support
- Model-specific endpoint (`GET /v1/models/{model}`)

### Compatible Servers
- **LocalAI** - Recommended for local testing
- **Ollama** - With OpenAI compatibility mode
- **Text Generation Web UI** - With OpenAI extension
- **vLLM** - OpenAI-compatible server mode
- **LM Studio** - Local LLM server
- **OpenAI API** - Official OpenAI endpoints
- **Anthropic API** - Using Anthropic provider

## Configuration Details

### Default Configuration
```bash
Endpoint: http://192.168.86.20:1234/v1
Model: qwen2.5-coder-7b-instruct
Token: (not set - optional for local servers)
Timeout: 30 seconds
```

### Environment Variables
- `RUN_LLM_INTEGRATION_TESTS=true` - **Required** to enable tests
- `LLM_TEST_ENDPOINT` - Server endpoint (auto-adds `/v1` if missing)
- `LLM_TEST_MODEL` - Model name to test with
- `LLM_TEST_TOKEN` - API token (optional for local servers)

## Troubleshooting

### Tests are Skipped
```
--- SKIP: TestIntegration_SimpleChatCompletion (0.00s)
    integration_test.go:57: Skipping integration test. Set RUN_LLM_INTEGRATION_TESTS=true to run integration tests.
```
**Solution**: Set `RUN_LLM_INTEGRATION_TESTS=true`

### Connection Refused
```
Failed to get chat completion: network error: request failed: dial tcp 192.168.86.20:1234: connect: connection refused
```
**Solutions**:
1. Check if your LLM server is running
2. Verify the endpoint is correct
3. Check firewall/network connectivity

### Model Not Found
```
Response: Model not found
```
**Solutions**:
1. Check available models: `curl http://192.168.86.20:1234/v1/models`
2. Update `LLM_TEST_MODEL` to a valid model name
3. Load the model in your LLM server

### Timeout Errors
```
context deadline exceeded
```
**Solutions**:
1. Model might be loading - wait and retry
2. Check server performance and resources
3. Increase timeout in test configuration

### Tool Calling Not Working
```
Model did not make any tool calls (this might be expected depending on the model's capabilities)
```
**Note**: Not all models support tool calling. This is informational, not a failure.

## Security Notes

- Tests only use safe, non-destructive operations
- No sensitive data is sent to the LLM
- API tokens are only used if explicitly set
- All test prompts are simple and harmless

## Performance Expectations

Typical performance on a local GPU server:
- **Model Loading**: 5-30 seconds (first request)
- **Simple Chat**: 1-5 seconds
- **Tool Calling**: 2-8 seconds  
- **Streaming**: Real-time, varies by response length

Performance depends on:
- Model size and complexity
- Hardware (CPU/GPU)
- Server load
- Network latency