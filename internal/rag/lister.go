package rag

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/philippgille/chromem-go"
)

// ListDocuments lists all documents in the database
func ListDocuments(config Config) error {
	fmt.Println("Database Contents")
	fmt.Println("=================")
	fmt.Printf("Database: %s\n\n", config.DBPath)

	// Load database
	if _, err := os.Stat(config.DBPath); os.IsNotExist(err) {
		fmt.Println("Database not found. Please run indexing first with -index")
		return nil
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

	// Create embedding function for Ollama (needed for GetCollection)
	embeddingFunc := CreateEmbeddingFunc(config)

	collection := db.GetCollection("documents", embeddingFunc)
	if collection == nil {
		fmt.Println("No documents collection found in database.")
		return nil
	}

	// Get collection count
	count := collection.Count()
	if count == 0 {
		fmt.Println("No documents found in the database.")
		return nil
	}

	fmt.Printf("Total documents: %d\n\n", count)

	// Get all documents by querying with a generic term that should match most content
	results, err := collection.Query(context.Background(), "text document file", count, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get documents: %w", err)
	}

	// Group results by file for better display
	fileGroups := make(map[string][]chromem.Result)
	for _, result := range results {
		filePath := result.Metadata["file_path"]
		fileGroups[filePath] = append(fileGroups[filePath], result)
	}

	// Sort file paths for consistent display
	var sortedFiles []string
	for filePath := range fileGroups {
		sortedFiles = append(sortedFiles, filePath)
	}
	sort.Strings(sortedFiles)

	fileIndex := 1
	totalChunks := 0

	// Display each file and its chunks
	for _, filePath := range sortedFiles {
		fileResults := fileGroups[filePath]

		// Sort chunks by chunk index
		sort.Slice(fileResults, func(i, j int) bool {
			indexI, _ := strconv.Atoi(fileResults[i].Metadata["chunk_index"])
			indexJ, _ := strconv.Atoi(fileResults[j].Metadata["chunk_index"])
			return indexI < indexJ
		})

		isChunked := len(fileResults) > 1 || fileResults[0].Metadata["is_chunk"] == "true"

		fmt.Printf("File %d: %s\n", fileIndex, filePath)
		fmt.Printf("  File Hash:      %s\n", fileResults[0].Metadata["file_hash"])
		fmt.Printf("  File Size:      %s bytes\n", fileResults[0].Metadata["file_size"])
		fmt.Printf("  Last Modified:  %s\n", fileResults[0].Metadata["last_modified"])

		if isChunked {
			fmt.Printf("  Chunks:         %d\n", len(fileResults))

			for j, result := range fileResults {
				chunkIndex := result.Metadata["chunk_index"]
				tokenCount := result.Metadata["token_count"]
				headingPath := result.Metadata["heading_path"]
				startOffset := result.Metadata["start_offset"]
				endOffset := result.Metadata["end_offset"]

				fmt.Printf("    Chunk %s: %s tokens, chars %s-%s\n",
					chunkIndex, tokenCount, startOffset, endOffset)

				if headingPath != "" {
					fmt.Printf("      Context: %s\n", headingPath)
				}

				// Show preview of first chunk only to avoid clutter
				if j == 0 {
					content := result.Content
					if len(content) > 100 {
						content = content[:100] + "..."
					}
					content = strings.ReplaceAll(content, "\n", " ")
					content = strings.ReplaceAll(content, "\r", "")
					fmt.Printf("      Preview: %s\n", content)
				}

				totalChunks++
			}
		} else {
			// Single document (not chunked)
			result := fileResults[0]
			tokenCount := result.Metadata["token_count"]
			fmt.Printf("  Token Count:    %s\n", tokenCount)
			fmt.Printf("  Indexed At:     %s\n", result.Metadata["indexed_at"])

			// Show content preview
			content := result.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			content = strings.ReplaceAll(content, "\n", " ")
			content = strings.ReplaceAll(content, "\r", "")
			fmt.Printf("  Content Preview: %s\n", content)

			totalChunks++
		}

		fmt.Println()
		fileIndex++
	}

	fmt.Printf("Summary: %d files, %d total chunks/documents\n", len(sortedFiles), totalChunks)

	return nil
}
