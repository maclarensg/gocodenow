# Documentation

This directory contains all project documentation organized by topic.

## Project Overview
- **[Product Requirements](product-requirements.md)** - Product vision, features, and requirements
- **[Architecture](architecture.md)** - Technical architecture and design decisions
- **[UI Design](ui-design.md)** - Terminal user interface design and specifications

## Development
- **[TODO](todo.md)** - Development roadmap and task tracking
- **[LLM Implementation Plan](llm-implementation-plan.md)** - Detailed plan for LLM integration

## Testing
- **[Integration Testing](integration-testing.md)** - Guide for testing with real LLM backends
- **[LLM Integration Tests](llm-integration-tests.md)** - Detailed LLM integration test documentation
- **[Taskfile Usage](taskfile-usage.md)** - Build automation and task runner guide

## Quick Links

### Getting Started
1. Read the [Product Requirements](product-requirements.md) for project overview
2. Check [Architecture](architecture.md) for technical design
3. Follow [Taskfile Usage](taskfile-usage.md) for build commands

### Development Workflow
1. Review current [TODO](todo.md) for tasks
2. Run tests: `task test`
3. Build project: `task build`
4. Run integration tests: `task itest` (if LLM server available)

### Testing
- **Unit Tests**: `task test`
- **Integration Tests**: `task itest` (requires LLM server)
- **Build Verification**: `task build`

For detailed testing instructions, see:
- [Integration Testing Guide](integration-testing.md)
- [LLM Integration Tests](llm-integration-tests.md)
- [Taskfile Usage](taskfile-usage.md)