# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project called "openwith" - an HTTP server that receives URLs via POST requests and opens them with configured applications. The project uses Echo framework and supports pattern-based URL routing with customizable application arguments.

ECHO を使い､POST リクエストでファイルを受け取ったときに､任意のアプリケーションを使ってファイルを開きます｡
ファイルに対するアプリケーションの紐づけは json ファイルで設定します｡

## Development Commands

### Build and Run
```bash
# Build the application
go build -o openwith main_openwith.go

# Run directly (starts server on port 44525)
go run main_openwith.go

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
- `main_openwith.go` - Main HTTP server with Echo framework, handles POST requests to "/" endpoint
- `config/config.go` - Configuration management for loading JSON config files
- `handler/handler.go` - Request/response data structures 
- `config.json` - Runtime configuration file (create from config.json.sample)

### Key Functions
- `openFile()` - Main POST handler that processes URL requests
- `processURL()` - Matches URLs against configured patterns and builds arguments
- `executeCommand()` - Executes the configured application with processed arguments

### Configuration System
The server reads `config.json` to determine:
- Which application to launch (`application` field)
- URL patterns with regex matching (`url_patterns` array)
- Custom arguments per pattern (`args` field, supports `$url` placeholder)
- URL parameter modifications (`url_params` field)

### Server Details
- Listens on port 44525
- Accepts POST requests with JSON body: `{"url": "https://example.com"}`
- Uses `cmd.Start()` to launch applications without blocking
- Returns JSON responses with execution status
