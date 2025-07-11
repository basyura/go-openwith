# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go project called "openwith" - a minimal Go application with a single main file. The project uses Go 1.24.2 and has a basic module structure.
ECHO を使い､POST リクエストでファイルを受け取ったときに､任意のアプリケーションを使ってファイルを開きます｡
ファイルに対するアプリケーションの紐づけは json ファイルで設定します｡

## Development Commands

### Build and Run
```bash
# Build the application
go build -o openwith main_openwith.go

# Run directly
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

The project has a minimal structure:
- `main_openwith.go` - Main application entry point (currently empty)
- `go.mod` - Go module definition

This appears to be a new/template project with the main implementation file currently empty, ready for development.
