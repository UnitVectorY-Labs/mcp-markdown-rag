package main

import (
	"flag"
	"log"

	"github.com/UnitVectorY-Labs/mcp-markdown-rag/internal/rag"
)

const (
	DefaultOllamaURL      = "http://localhost:11434/api/embeddings"
	DefaultEmbeddingModel = "nomic-embed-text"
	DefaultDBPath         = "./rag.db"

	// Chunking configuration
	MaxTokensPerChunk   = 4000 // Maximum tokens per chunk
	ChunkOverlapPercent = 15   // 15% overlap between chunks
	MaxContextTokens    = 8000 // Context window limit for nomic-embed-text
	ApproxTokensPerChar = 0.25 // Rough approximation: 4 chars per token
)

func main() {
	var indexPath = flag.String("index", "", "Path to folder to recursively index .md files")
	var query = flag.String("query", "", "Query string to search for similar documents")
	var list = flag.Bool("list", false, "List all documents in the database")
	var stats = flag.Bool("stats", false, "Show statistics about the database contents")
	var help = flag.Bool("help", false, "Show help")
	var dbPath = flag.String("db", "", "Path to database file (default: ./rag.db)")
	var ollamaURL = flag.String("ollama-url", "", "Ollama API URL (default: http://localhost:11434/api/embeddings)")
	var embeddingModel = flag.String("embedding-model", "", "Embedding model name (default: nomic-embed-text)")
	var embeddingMode = flag.String("embedding-mode", "", "Embedding mode: 'local' (default, self-contained) or 'ollama' (requires Ollama)")
	var mcpMode = flag.Bool("mcp", false, "Run as MCP server")

	flag.Parse()

	config := rag.GetConfig(ollamaURL, embeddingModel, dbPath, embeddingMode, DefaultOllamaURL, DefaultEmbeddingModel, DefaultDBPath)

	// MCP mode takes precedence
	if *mcpMode {
		err := rag.RunMCPServer(config)
		if err != nil {
			log.Fatalf("MCP Server error: %v", err)
		}
		return
	}

	if *help || (*indexPath == "" && *query == "" && !*list && !*stats) {
		rag.ShowHelp(MaxTokensPerChunk, ChunkOverlapPercent, MaxContextTokens)
		return
	}

	if *indexPath != "" {
		err := rag.IndexDocuments(*indexPath, config, MaxTokensPerChunk, ChunkOverlapPercent, ApproxTokensPerChar)
		if err != nil {
			log.Fatalf("Error indexing documents: %v", err)
		}
	}

	if *query != "" {
		err := rag.SearchDocuments(*query, config)
		if err != nil {
			log.Fatalf("Error searching documents: %v", err)
		}
	}

	if *list {
		err := rag.ListDocuments(config)
		if err != nil {
			log.Fatalf("Error listing documents: %v", err)
		}
	}

	if *stats {
		err := rag.ShowStats(config)
		if err != nil {
			log.Fatalf("Error showing statistics: %v", err)
		}
	}
}
