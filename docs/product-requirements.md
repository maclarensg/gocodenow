# gocodenow - Product Requirements Document

## Overview
Open-source TUI alternative to Claude Code that connects to multiple LLM providers via OpenAI-compatible protocols. The interface and user experience should closely mirror Claude Code's functionality and workflow patterns.

## Core Features
- **Multi-Model Support**: Connect to any OpenAI-compatible API (LM Studio, local models, etc.)
- **TUI Interface**: Terminal-based user interface mimicking Claude Code's conversation-based workflow
- **Code Assistant**: AI-powered code generation, debugging, and refactoring similar to Claude Code
- **Provider Agnostic**: Switch between different LLM providers without changing workflow
- **Claude Code UX**: Multiline input support, conversation history, expandable responses matching Claude Code patterns

## Target Users
- Developers who want Claude Code functionality with model choice flexibility
- Users running local LLMs (LM Studio, Ollama, etc.)
- Teams needing on-premises AI coding solutions

## Technical Requirements
- OpenAI API compatibility layer
- Configurable model endpoints
- Terminal UI framework
- Cross-platform support (Linux, macOS, Windows)

## Success Metrics
- Seamless model switching
- Feature parity with commercial alternatives
- Strong community adoption
- Reliable local model integration