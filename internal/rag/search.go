package rag

import (
	"context"
	"fmt"
	"os"

	"github.com/philippgille/chromem-go"
)

// SearchDocuments searches for documents similar to the query text
func SearchDocuments(queryText string, config Config) error {
	fmt.Printf("Searching for: %s\n", queryText)
	fmt.Printf("Using database: %s\n", config.DBPath)

	// Load database
	if _, err := os.Stat(config.DBPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found. Please run indexing first with -index")
	}

	db := chromem.NewDB()
	file, err := os.Open(config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer file.Close()

	err = db.ImportFromReader(file, "")
	if err != nil {
		return fmt.Errorf("failed to load database: %w", err)
	}

	// Create embedding function for Ollama
	embeddingFunc := CreateEmbeddingFunc(config)

	collection := db.GetCollection("documents", embeddingFunc)
	if collection == nil {
		return fmt.Errorf("documents collection not found in database")
	}

	// Get collection count to determine max results
	count := collection.Count()

	if count == 0 {
		fmt.Println("No documents found in the database.")
		return nil
	}

	// Limit results to available documents (max 10)
	maxResults := MinInt(10, count)

	// Search for similar documents
	results, err := collection.Query(context.Background(), queryText, maxResults, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to query collection: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No similar documents found.")
		return nil
	}

	fmt.Println("\nSearch Results:")
	fmt.Println("===============")

	for i, result := range results {
		isChunk := result.Metadata["is_chunk"] == "true"
		chunkInfo := ""

		if isChunk {
			chunkIndex := result.Metadata["chunk_index"]
			tokenCount := result.Metadata["token_count"]
			headingPath := result.Metadata["heading_path"]

			chunkInfo = fmt.Sprintf(" (chunk %s, %s tokens", chunkIndex, tokenCount)
			if headingPath != "" {
				chunkInfo += fmt.Sprintf(", context: %s", headingPath)
			}
			chunkInfo += ")"
		}

		fmt.Printf("\n%d. File: %s%s\n", i+1, result.Metadata["file_path"], chunkInfo)
		fmt.Printf("   Similarity: %.4f\n", result.Similarity)
		fmt.Printf("   Size: %s bytes\n", result.Metadata["file_size"])
		fmt.Printf("   Last Modified: %s\n", result.Metadata["last_modified"])
		fmt.Printf("   Indexed: %s\n", result.Metadata["indexed_at"])

		if isChunk {
			startOffset := result.Metadata["start_offset"]
			endOffset := result.Metadata["end_offset"]
			fmt.Printf("   Chunk Range: chars %s-%s\n", startOffset, endOffset)
		}
	}

	return nil
}

// MCPSearchResult represents the result of an MCP search
type MCPSearchResult struct {
	Content    string
	FilePath   string
	Similarity float32
	IsChunk    bool
	Metadata   map[string]string
}

// MCPSearchDocuments searches for documents and returns the content for MCP
func MCPSearchDocuments(queryText string, config Config) (*MCPSearchResult, error) {
	// Load database
	if _, err := os.Stat(config.DBPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database not found. Please run indexing first with -index")
	}

	db := chromem.NewDB()
	file, err := os.Open(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer file.Close()

	err = db.ImportFromReader(file, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load database: %w", err)
	}

	// Create embedding function for Ollama
	embeddingFunc := CreateEmbeddingFunc(config)

	collection := db.GetCollection("documents", embeddingFunc)
	if collection == nil {
		return nil, fmt.Errorf("documents collection not found in database")
	}

	// Get collection count to determine max results
	count := collection.Count()

	if count == 0 {
		return nil, fmt.Errorf("no documents found in the database")
	}

	// Get the best match (top 1)
	results, err := collection.Query(context.Background(), queryText, 1, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no similar documents found")
	}

	result := results[0]
	isChunk := result.Metadata["is_chunk"] == "true"

	// Create metadata map
	metadata := make(map[string]string)
	for k, v := range result.Metadata {
		metadata[k] = v
	}

	return &MCPSearchResult{
		Content:    result.Content,
		FilePath:   result.Metadata["file_path"],
		Similarity: result.Similarity,
		IsChunk:    isChunk,
		Metadata:   metadata,
	}, nil
}
