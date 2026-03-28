[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT) [![Go Report Card](https://goreportcard.com/badge/github.com/UnitVectorY-Labs/mcp-markdown-rag)](https://goreportcard.com/report/github.com/UnitVectorY-Labs/mcp-markdown-rag)

# mcp-markdown-rag

A self-contained command-line tool and MCP server for indexing and semantically searching Markdown documents. Point it at a folder of Markdown files and it will automatically chunk, embed, and index them into a local vector database — no external services required.

## Features

- **Fully self-contained** — built-in local embeddings require no external APIs or services
- **Optional Ollama integration** — use Ollama for higher quality embeddings when available
- **Automatic document chunking** — large documents are split at semantic boundaries (headings, sentences) with configurable overlap
- **MCP server mode** — integrates with AI assistants via the [Model Context Protocol](https://modelcontextprotocol.io/)
- **Persistent vector database** — indexed documents are stored locally using [chromem-go](https://github.com/philippgille/chromem-go)

## Quick Start

### Build

```bash
go build -o mcp-markdown-rag .
```

### Index Documents

Point the tool at a folder containing Markdown files:

```bash
./mcp-markdown-rag -index /path/to/your/docs
```

This recursively finds all `.md` files, chunks large documents, generates embeddings, and stores everything in a local database file (`rag.db` by default).

### Search Documents

```bash
./mcp-markdown-rag -query "how to deploy containers"
```

### Run as MCP Server

```bash
./mcp-markdown-rag -mcp
```

This starts the MCP server over stdio, exposing two tools:

- **`rag_search`** — Search for relevant documentation by query
- **`rag_retrieve`** — Retrieve specific file content, optionally by character range

## MCP Server Configuration

### Claude Desktop

Add the following to your Claude Desktop configuration file:

**macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "markdown-rag": {
      "command": "/path/to/mcp-markdown-rag",
      "args": ["-mcp", "-db", "/path/to/your/rag.db"]
    }
  }
}
```

### VS Code / Cursor

Add to your MCP settings:

```json
{
  "servers": {
    "markdown-rag": {
      "command": "/path/to/mcp-markdown-rag",
      "args": ["-mcp", "-db", "/path/to/your/rag.db"]
    }
  }
}
```

## Embedding Modes

### Local Mode (Default)

The default mode uses a built-in embedding algorithm. No external services or API keys are needed.

```bash
./mcp-markdown-rag -index /path/to/docs
```

### Ollama Mode

For higher quality embeddings, use [Ollama](https://ollama.com/) with a model like `nomic-embed-text`:

```bash
# Make sure Ollama is running with the embedding model
ollama pull nomic-embed-text

# Index using Ollama embeddings
./mcp-markdown-rag -embedding-mode ollama -index /path/to/docs
```

> **Important:** Use the same embedding mode for both indexing and searching. Mixing modes will produce incorrect results.

## CLI Reference

```
Usage:
  -index <path>              Index all .md files in the specified folder recursively
  -query <text>              Search for documents similar to the query text
  -list                      List all documents in the database
  -stats                     Show statistics about the database contents
  -db <path>                 Path to database file (default: ./rag.db)
  -embedding-mode <mode>     Embedding mode: 'local' (default) or 'ollama'
  -ollama-url <url>          Ollama API URL (default: http://localhost:11434/api/embeddings)
  -embedding-model <model>   Embedding model name for ollama mode (default: nomic-embed-text)
  -mcp                       Run as MCP server (communicates over stdio)
  -help                      Show help message
```

## Environment Variables

All settings can be configured via environment variables as an alternative to CLI flags:

| Variable | Description | Default |
|---|---|---|
| `RAG_DB_PATH` | Database file path | `./rag.db` |
| `RAG_EMBEDDING_MODE` | Embedding mode (`local` or `ollama`) | `local` |
| `RAG_OLLAMA_URL` | Ollama API URL | `http://localhost:11434/api/embeddings` |
| `RAG_EMBEDDING_MODEL` | Embedding model name | `nomic-embed-text` |

Priority: CLI flags > Environment variables > Defaults

## Usage Examples

```bash
# Index a documentation folder
./mcp-markdown-rag -index ./docs

# Search with a custom database path
./mcp-markdown-rag -query "authentication setup" -db /data/my-docs.db

# View all indexed documents
./mcp-markdown-rag -list

# View database statistics
./mcp-markdown-rag -stats

# Index with Ollama and a custom model
./mcp-markdown-rag -embedding-mode ollama -embedding-model mxbai-embed-large -index ./docs

# Run MCP server with environment variables
RAG_DB_PATH=/data/rag.db ./mcp-markdown-rag -mcp
```
