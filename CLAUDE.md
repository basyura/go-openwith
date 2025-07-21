# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project called "openwith" - an HTTP server that receives URLs via POST requests and opens them with configured applications. The project uses Echo framework and supports pattern-based URL routing with customizable application arguments. It can run as both a standalone application and a Windows service.

## Development Commands

### Build and Run
```bash
# Build the application
go build -o openwith main.go openwith.go

# Run directly (starts server on port 44525)
go run main.go openwith.go

# Build and install
go install
```

### Testing and Quality
```bash
# Run tests (if any are added)
go test ./...

# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Tidy dependencies
go mod tidy
```

## Architecture

The project follows a modular structure:

### Core Components
- `main.go` - Service wrapper for running as Windows service or terminal application
- `openwith.go` - Main HTTP server with Echo framework, handles POST requests to "/" endpoint
- `config/config.go` - Configuration management for loading and watching JSON config files
- `handler/handler.go` - Main request handler logic
- `handler/types.go` - Request/response data structures
- `logger/logger.go` - Centralized logging with file output support
- `windows/session.go` - Windows session management for service execution
- `config.json` - Runtime configuration file (create from config.json.sample)

### Key Functions
- `MainRun()` - Main server initialization and startup in openwith.go:56
- `Handle()` - Main POST handler that processes URL requests in handler/handler.go
- Service management functions in main.go for Windows service support

### Configuration System
The server reads `config.json` to determine:
- Which application to launch (`application` field)
- Server port configuration (`port` field, defaults to 44525)
- URL patterns with regex matching (`url_patterns` array)
- Custom arguments per pattern (`args` field, supports `$url` placeholder)
- URL parameter modifications (`url_params` field)

### Server Details
- Listens on configurable port (default 44525)
- Accepts POST requests with JSON body: `{"url": "https://example.com"}`
- Supports hot-reloading of configuration files
- Uses structured logging with boxed messages
- Can run as Windows service or standalone application
- Returns JSON responses with execution status
