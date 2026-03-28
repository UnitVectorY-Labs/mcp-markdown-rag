
# Commands for mcp-markdown-rag
default:
  @just --list
# Build mcp-markdown-rag with Go
build:
  go build ./...

# Run tests for mcp-markdown-rag with Go
test:
  go clean -testcache
  go test ./...