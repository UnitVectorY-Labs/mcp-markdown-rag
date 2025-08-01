package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/philippgille/chromem-go"
)

// ShowStats displays statistics about the database contents
func ShowStats(config Config) error {
	fmt.Println("Database Statistics")
	fmt.Println("===================")
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

	// Get all documents for analysis
	results, err := collection.Query(context.Background(), "text document file", count, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to get documents: %w", err)
	}

	// Analyze the documents
	uniqueFiles := make(map[string]bool)
	fileSizes := make(map[string]int)
	fileChunkCounts := make(map[string]int)
	tokenCounts := []int{}
	chunksByFile := make(map[string][]chromem.Result)

	var totalTokens int
	var minTokens, maxTokens int = -1, 0
	var minFileSize, maxFileSize int64 = -1, 0
	chunkedFiles := 0
	singleDocFiles := 0

	for _, result := range results {
		filePath := result.Metadata["file_path"]
		uniqueFiles[filePath] = true

		// Track chunks by file
		chunksByFile[filePath] = append(chunksByFile[filePath], result)
		fileChunkCounts[filePath]++

		// Parse file size (only need to do this once per file)
		if _, exists := fileSizes[filePath]; !exists {
			if sizeStr, ok := result.Metadata["file_size"]; ok {
				if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
					fileSizes[filePath] = int(size)
					if minFileSize == -1 || size < minFileSize {
						minFileSize = size
					}
					if size > maxFileSize {
						maxFileSize = size
					}
				}
			}
		}

		// Parse token count
		if tokenStr, ok := result.Metadata["token_count"]; ok {
			if tokens, err := strconv.Atoi(tokenStr); err == nil {
				tokenCounts = append(tokenCounts, tokens)
				totalTokens += tokens
				if minTokens == -1 || tokens < minTokens {
					minTokens = tokens
				}
				if tokens > maxTokens {
					maxTokens = tokens
				}
			}
		}
	}

	// Classify files as chunked vs single documents
	for _, chunks := range chunksByFile {
		if len(chunks) > 1 || (len(chunks) == 1 && chunks[0].Metadata["is_chunk"] == "true") {
			chunkedFiles++
		} else {
			singleDocFiles++
		}
	}

	// Calculate averages
	avgChunksPerFile := float64(count) / float64(len(uniqueFiles))
	avgTokensPerChunk := float64(totalTokens) / float64(count)

	// Calculate file size statistics
	var totalFileSize int64
	for _, size := range fileSizes {
		totalFileSize += int64(size)
	}
	avgFileSize := float64(totalFileSize) / float64(len(uniqueFiles))

	// Display statistics
	fmt.Printf("📊 Document Overview:\n")
	fmt.Printf("   Unique files:        %d\n", len(uniqueFiles))
	fmt.Printf("   Total chunks:        %d\n", count)
	fmt.Printf("   Chunked files:       %d\n", chunkedFiles)
	fmt.Printf("   Single doc files:    %d\n\n", singleDocFiles)

	fmt.Printf("📈 Chunk Statistics:\n")
	fmt.Printf("   Avg chunks per file: %.1f\n", avgChunksPerFile)
	fmt.Printf("   Min tokens/chunk:    %d\n", minTokens)
	fmt.Printf("   Max tokens/chunk:    %d\n", maxTokens)
	fmt.Printf("   Avg tokens/chunk:    %.0f\n\n", avgTokensPerChunk)

	fmt.Printf("📁 File Size Statistics:\n")
	fmt.Printf("   Min file size:       %s\n", FormatBytes(minFileSize))
	fmt.Printf("   Max file size:       %s\n", FormatBytes(maxFileSize))
	fmt.Printf("   Avg file size:       %s\n", FormatBytes(int64(avgFileSize)))
	fmt.Printf("   Total indexed:       %s\n\n", FormatBytes(totalFileSize))

	fmt.Printf("🔤 Token Statistics:\n")
	fmt.Printf("   Total tokens:        %s\n", FormatNumber(totalTokens))

	// Find files with most chunks
	type FileChunkInfo struct {
		Path   string
		Chunks int
		Size   int
	}
	var fileInfos []FileChunkInfo
	for filePath, chunks := range chunksByFile {
		fileInfos = append(fileInfos, FileChunkInfo{
			Path:   filePath,
			Chunks: len(chunks),
			Size:   fileSizes[filePath],
		})
	}

	// Sort by chunk count (descending)
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].Chunks > fileInfos[j].Chunks
	})

	fmt.Printf("📋 Top 5 Most Chunked Files:\n")
	for i, info := range fileInfos {
		if i >= 5 {
			break
		}
		// Show just filename, not full path
		filename := filepath.Base(info.Path)
		fmt.Printf("   %d. %s (%d chunks, %s)\n", i+1, filename, info.Chunks, FormatBytes(int64(info.Size)))
	}

	return nil
}
