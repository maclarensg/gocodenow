# Contributing to gocodenow

Thank you for your interest in contributing to gocodenow! This document provides guidelines for developers who want to contribute to the project.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- devbox (optional but recommended for development)

### Development Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/your-org/gocodenow
   cd gocodenow
   ```

2. **Set up development environment**:
   ```bash
   # Using devbox (recommended)
   devbox shell
   
   # Or manually install dependencies
   go mod download
   ```

3. **Build and run**:
   ```bash
   go build -o gocodenow ./cmd/gocodenow
   ./gocodenow
   ```

4. **Run tests**:
   ```bash
   # Run all tests
   go test ./...
   
   # Run specific test packages
   go test ./internal/tools/...
   go test ./internal/models/...
   
   # Run with coverage
   go test -cover ./...
   ```

## Project Structure

```
gocodenow/
├── cmd/gocodenow/          # Main application entry point
├── internal/               # Private application code
│   ├── app/               # Application orchestration layer
│   ├── config/            # Configuration management
│   ├── context/           # Context and session management
│   ├── models/            # Domain models (conversations, etc.)
│   ├── recovery/          # Error recovery and resilience
│   ├── security/          # Security framework and policies
│   ├── storage/           # Data persistence layer
│   ├── tools/             # Tool execution system
│   ├── types/             # Shared type definitions
│   └── ui/                # Terminal user interface
├── test/                  # Test files and fixtures
│   └── e2e/              # End-to-end test scenarios
├── docs/                  # Documentation
└── README.md
```

## Architecture Overview

gocodenow follows a clean layered architecture:

### Core Layers

1. **UI Layer** (`internal/ui/`): Terminal user interface using Bubble Tea framework
2. **Application Layer** (`internal/app/`): Business logic orchestration
3. **Domain Layer** (`internal/models/`, `internal/types/`): Core business entities
4. **Infrastructure Layer** (`internal/storage/`, `internal/security/`): External concerns

### Key Components

- **TUI System**: Built with Charm's Bubble Tea framework using Model-View-Update pattern
- **Tool System**: Extensible tool execution framework for file operations, commands, etc.
- **Storage System**: SQLite-based persistence with caching and performance optimization
- **Security System**: Comprehensive security framework with sandboxing and policies
- **Recovery System**: Error handling and system resilience mechanisms

## Development Guidelines

### Code Style

1. **Follow Go conventions**: Use `gofmt`, `golint`, and `go vet`
2. **Naming**: Use the project name "gocodenow" consistently (never "lmcodenow")
3. **Package organization**: Keep packages focused and cohesive
4. **Error handling**: Use proper error wrapping and context

### Testing Requirements

1. **Unit tests**: All public functions should have unit tests
2. **Integration tests**: Test component interactions
3. **E2E tests**: Test complete user workflows
4. **Coverage**: Aim for >80% test coverage on new code

### Commit Guidelines

1. **Commit messages**: Use conventional commit format
   ```
   type(scope): description
   
   Examples:
   feat(ui): add keyboard shortcuts for conversation navigation  
   fix(storage): handle database connection failures gracefully
   docs(readme): update installation instructions
   test(tools): add integration tests for file operations
   ```

2. **Atomic commits**: One logical change per commit
3. **Clean history**: Squash fixup commits before merging

## Contributing Workflow

### 1. Issue First

- Check existing issues before starting work
- Create an issue for new features or bugs
- Discuss approach before implementing

### 2. Branch Strategy

```bash
# Create feature branch
git checkout -b feature/your-feature-name

# Create bugfix branch  
git checkout -b fix/issue-description

# Create docs branch
git checkout -b docs/update-description
```

### 3. Development Process

1. **Write tests first** (TDD approach recommended)
2. **Implement the feature/fix**
3. **Run all tests**: `go test ./...`
4. **Update documentation** if needed
5. **Test manually** with the TUI

### 4. Pull Request Process

1. **Run pre-commit checks**:
   ```bash
   go fmt ./...
   go vet ./...
   go test ./...
   ```

2. **Create pull request** with:
   - Clear description of changes
   - Reference to related issue(s)
   - Screenshots for UI changes
   - Test coverage report

3. **Code review process**:
   - Address review feedback
   - Keep PR focused and small
   - Squash commits before merge

## Adding New Features

### Adding New Tools

1. **Create tool executor** in `internal/tools/`:
   ```go
   type NewToolExecutor struct {
       // tool configuration
   }
   
   func (e *NewToolExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
       // implementation
   }
   
   func (e *NewToolExecutor) GetSchema() *ToolSchema {
       // tool schema definition
   }
   ```

2. **Register the tool** in tool router
3. **Add integration tests**
4. **Update documentation**

### Adding UI Components

1. **Follow Bubble Tea patterns**:
   ```go
   type NewComponent struct {
       // component state
   }
   
   func (c NewComponent) Init() tea.Cmd { /* ... */ }
   func (c NewComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) { /* ... */ }
   func (c NewComponent) View() string { /* ... */ }
   ```

2. **Add keyboard shortcuts** in `internal/ui/keys.go`
3. **Update help system** if needed
4. **Test responsive behavior**

### Adding Configuration Options

1. **Update config structs** in `internal/config/`
2. **Add validation logic**
3. **Update default values**
4. **Document in user guide**

## Testing

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/tools/

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...

# E2E tests only
go test ./test/e2e/...
```

### Writing Tests

1. **Unit tests**: Test individual functions/methods
   ```go
   func TestToolExecutor_Execute(t *testing.T) {
       executor := NewToolExecutor()
       result, err := executor.Execute(ctx, params)
       assert.NoError(t, err)
       assert.True(t, result.Success)
   }
   ```

2. **Integration tests**: Test component interactions
   ```go
   func TestToolIntegration(t *testing.T) {
       router := tools.NewToolRouter()
       router.RegisterExecutor("test", NewTestExecutor())
       // Test complete workflow
   }
   ```

3. **E2E tests**: Test user scenarios
   ```go
   func TestCompleteWorkflow(t *testing.T) {
       // Setup complete environment
       // Simulate user actions
       // Verify expected outcomes
   }
   ```

## Debugging

### Debug Mode

```bash
export DEBUG=1
./gocodenow
```

### Common Issues

1. **TUI rendering problems**: Check terminal size and capabilities
2. **Tool execution failures**: Verify file permissions and paths
3. **Database issues**: Check SQLite file permissions and disk space

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.

# Memory profiling  
go test -memprofile=mem.prof -bench=.

# View profiles
go tool pprof cpu.prof
```

## Documentation

### Code Documentation

- Use clear, descriptive comments
- Document public APIs thoroughly
- Include usage examples where helpful
- Keep comments up-to-date with code changes

### User Documentation

- Update `docs/user-guide.md` for user-facing changes
- Include screenshots for UI changes
- Provide configuration examples
- Document troubleshooting steps

## Release Process

### Version Management

- Use semantic versioning (MAJOR.MINOR.PATCH)
- Tag releases: `git tag v1.2.3`
- Update version in relevant files

### Release Checklist

1. [ ] All tests passing
2. [ ] Documentation updated
3. [ ] CHANGELOG.md updated
4. [ ] Version bumped
5. [ ] Release notes prepared
6. [ ] Binaries built and tested

## Getting Help

### Community

- **GitHub Issues**: Bug reports and feature requests
- **Discussions**: Questions and general discussion
- **Wiki**: Additional documentation and guides

### Code Review

- Be constructive and respectful
- Focus on code quality and maintainability
- Suggest improvements with rationale
- Test changes locally when possible

## License

By contributing to gocodenow, you agree that your contributions will be licensed under the same license as the project (MIT License).

---

Thank you for contributing to gocodenow! 🚀