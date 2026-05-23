package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"

	"github.com/UnitVectorY-Labs/mcp-markdown-rag/internal/rag"
)

const (
	DefaultOllamaURL      = "http://localhost:11434/api/embeddings"
	DefaultEmbeddingModel = "nomic-embed-text"
	DefaultDBPath         = "./rag.db"
	ProjectName           = "mcp-markdown-rag"

	// Chunking configuration
	MaxTokensPerChunk   = 4000 // Maximum tokens per chunk
	ChunkOverlapPercent = 15   // 15% overlap between chunks
	MaxContextTokens    = 8000 // Context window limit for nomic-embed-text
	ApproxTokensPerChar = 0.25 // Rough approximation: 4 chars per token
)

// Version is the application version, injected at build time via ldflags.
var Version = "dev"

func isSemverRelease(version string) bool {
	normalized := strings.TrimPrefix(version, "v")
	mainAndBuild := strings.SplitN(normalized, "+", 2)
	mainAndPre := strings.SplitN(mainAndBuild[0], "-", 2)
	parts := strings.Split(mainAndPre[0], ".")
	if len(parts) != 3 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}

	return true
}

func buildVersionOutput(projectName, version string) string {
	normalized := version
	if isSemverRelease(normalized) && !strings.HasPrefix(normalized, "v") {
		normalized = "v" + normalized
	}
	return fmt.Sprintf("%s version %s (%s, %s/%s)", projectName, normalized, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func main() {
	var indexPath = flag.String("index", "", "Path to folder to recursively index .md files")
	var query = flag.String("query", "", "Query string to search for similar documents")
	var list = flag.Bool("list", false, "List all documents in the database")
	var stats = flag.Bool("stats", false, "Show statistics about the database contents")
	var help = flag.Bool("help", false, "Show help")
	var dbPath = flag.String("db", "", "Path to database file (default: ./rag.db)")
	var ollamaURL = flag.String("ollama-url", "", "Ollama API URL (default: http://localhost:11434/api/embeddings)")
	var embeddingModel = flag.String("embedding-model", "", "Embedding model name (default: nomic-embed-text)")
	var mcpMode = flag.Bool("mcp", false, "Run as MCP server")
	var version = flag.Bool("version", false, "Show version")

	flag.Parse()

	if *version {
		fmt.Println(buildVersionOutput(ProjectName, Version))
		return
	}

	config := rag.GetConfig(ollamaURL, embeddingModel, dbPath, DefaultOllamaURL, DefaultEmbeddingModel, DefaultDBPath)

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
