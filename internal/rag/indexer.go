package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/philippgille/chromem-go"
)

// IndexDocuments indexes all markdown files in the specified directory
func IndexDocuments(rootPath string, config Config, maxTokensPerChunk, chunkOverlapPercent int, approxTokensPerChar float64) error {
	fmt.Printf("Starting to index documents in: %s\n", rootPath)
	fmt.Printf("Using database: %s\n", config.DBPath)
	fmt.Printf("Using Ollama URL: %s\n", config.OllamaURL)
	fmt.Printf("Using embedding model: %s\n", config.EmbeddingModel)

	// Convert rootPath to absolute path
	absRootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", rootPath, err)
	}

	// Initialize chromem-go database
	db := chromem.NewDB()

	// Load existing database if it exists
	if _, err := os.Stat(config.DBPath); err == nil {
		fmt.Println("Loading existing database...")
		file, err := os.Open(config.DBPath)
		if err != nil {
			return fmt.Errorf("failed to open existing database: %w", err)
		}
		defer file.Close()

		err = db.ImportFromReader(file, "")
		if err != nil {
			fmt.Printf("Warning: Could not load existing database: %v\n", err)
			// Continue with fresh database
			db = chromem.NewDB()
		}
	}

	// Find all .md files
	var mdFiles []string
	err = filepath.Walk(absRootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			// Convert to absolute path
			absPath, err := filepath.Abs(path)
			if err != nil {
				fmt.Printf("Warning: Could not get absolute path for %s: %v\n", path, err)
				return nil
			}
			mdFiles = append(mdFiles, absPath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Create embedding function for Ollama
	embeddingFunc := CreateEmbeddingFunc(config)

	collection, err := db.GetOrCreateCollection("documents", nil, embeddingFunc)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	for i, filePath := range mdFiles {
		fmt.Printf("Processing (%d/%d): %s\n", i+1, len(mdFiles), filePath)

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Warning: Could not read file %s: %v\n", filePath, err)
			continue
		}

		// Create file hash
		hash := sha256.Sum256(content)
		fileHash := hex.EncodeToString(hash[:])

		// Get file info
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			fmt.Printf("Warning: Could not get file info for %s: %v\n", filePath, err)
			continue
		}

		// Check if file needs chunking
		contentStr := string(content)
		estimatedTokens := EstimateTokenCount(contentStr, approxTokensPerChar)

		fmt.Printf("  File size: %d bytes, estimated tokens: %d\n", len(content), estimatedTokens)

		if estimatedTokens > maxTokensPerChunk {
			fmt.Printf("  Large file detected, chunking into smaller pieces...\n")

			// Chunk the document
			chunks := ChunkDocument(filePath, contentStr, fileHash, maxTokensPerChunk, chunkOverlapPercent, approxTokensPerChar)
			fmt.Printf("  Created %d chunks\n", len(chunks))

			// Get embeddings for all chunks in batches
			embeddings, err := BatchEmbedChunks(chunks, config)
			if err != nil {
				fmt.Printf("Warning: Could not get embeddings for %s: %v\n", filePath, err)
				continue
			}

			// Add each chunk to the collection
			for _, chunk := range chunks {
				embedding, exists := embeddings[chunk.ID]
				if !exists {
					fmt.Printf("Warning: No embedding found for chunk %s\n", chunk.ID)
					continue
				}

				// Create metadata for chunk
				headingPathStr := ""
				if len(chunk.HeadingPath) > 0 {
					headingPathStr = strings.Join(chunk.HeadingPath, " > ")
				}

				err = collection.AddDocument(context.Background(), chromem.Document{
					ID: chunk.ID,
					Metadata: map[string]string{
						"file_path":     chunk.FilePath,
						"file_hash":     chunk.FileHash,
						"chunk_index":   strconv.Itoa(chunk.ChunkIndex),
						"file_size":     fmt.Sprintf("%d", fileInfo.Size()),
						"last_modified": fileInfo.ModTime().Format(time.RFC3339),
						"indexed_at":    chunk.CreatedAt.Format(time.RFC3339),
						"start_offset":  strconv.Itoa(chunk.StartOffset),
						"end_offset":    strconv.Itoa(chunk.EndOffset),
						"token_count":   strconv.Itoa(chunk.TokenCount),
						"heading_path":  headingPathStr,
						"is_chunk":      "true",
					},
					Embedding: embedding,
					Content:   chunk.Content,
				})
				if err != nil {
					fmt.Printf("Warning: Could not add chunk %s to collection: %v\n", chunk.ID, err)
					continue
				}
			}

			fmt.Printf("✓ Indexed: %s (%d chunks, hash: %s)\n", filePath, len(chunks), fileHash[:8])
		} else {
			// Handle small files as before (single chunk)
			fmt.Printf("  Small file, indexing as single document\n")

			// Get embedding from Ollama
			embedding, err := GetEmbedding(contentStr, config)
			if err != nil {
				fmt.Printf("Warning: Could not get embedding for %s: %v\n", filePath, err)
				continue
			}

			// Add to collection with individual metadata fields
			err = collection.AddDocument(context.Background(), chromem.Document{
				ID: fileHash,
				Metadata: map[string]string{
					"file_path":     filePath,
					"file_hash":     fileHash,
					"chunk_index":   "0",
					"file_size":     fmt.Sprintf("%d", fileInfo.Size()),
					"last_modified": fileInfo.ModTime().Format(time.RFC3339),
					"indexed_at":    time.Now().Format(time.RFC3339),
					"start_offset":  "0",
					"end_offset":    strconv.Itoa(len(content)),
					"token_count":   strconv.Itoa(estimatedTokens),
					"heading_path":  "",
					"is_chunk":      "false",
				},
				Embedding: embedding,
				Content:   contentStr,
			})
			if err != nil {
				fmt.Printf("Warning: Could not add document %s to collection: %v\n", filePath, err)
				continue
			}

			fmt.Printf("✓ Indexed: %s (single document, hash: %s)\n", filePath, fileHash[:8])
		}
	}

	// Save database
	file, err := os.Create(config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to create database file: %w", err)
	}
	defer file.Close()

	err = db.ExportToWriter(file, true, "")
	if err != nil {
		return fmt.Errorf("failed to save database: %w", err)
	}

	fmt.Printf("✓ Successfully indexed %d documents and saved to %s\n", len(mdFiles), config.DBPath)
	return nil
}
