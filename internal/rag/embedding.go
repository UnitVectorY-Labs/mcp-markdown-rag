package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbeddingConfig holds embedding-specific configuration
type EmbeddingConfig struct {
	OllamaURL      string
	EmbeddingModel string
}

// OllamaEmbeddingRequest represents the request structure for Ollama API
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingResponse represents the response structure from Ollama API
type OllamaEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
}

// GetEmbedding gets embedding using the configured mode (local or Ollama)
func GetEmbedding(text string, config Config) ([]float32, error) {
	if config.EmbeddingMode == EmbeddingModeLocal {
		return GetLocalEmbedding(text)
	}
	return getOllamaEmbedding(text, config)
}

// getOllamaEmbedding gets embedding from Ollama API
func getOllamaEmbedding(text string, config Config) ([]float32, error) {
	reqBody := OllamaEmbeddingRequest{
		Model:  config.EmbeddingModel,
		Prompt: text,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	resp, err := http.Post(config.OllamaURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API returned status %d: %s", resp.StatusCode, string(body))
	}
	var embeddingResp OllamaEmbeddingResponse
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return embeddingResp.Embedding, nil
}

// BatchEmbedChunks processes chunks in batches with retry logic
func BatchEmbedChunks(chunks []DocumentChunk, config Config) (map[string][]float32, error) {
	embeddings := make(map[string][]float32)
	batchSize := 10 // Process 10 chunks at a time
	maxRetries := 3

	// Local mode doesn't need retries since it's in-process
	if config.EmbeddingMode == EmbeddingModeLocal {
		maxRetries = 1
	}

	fmt.Printf("Processing %d chunks in batches of %d\n", len(chunks), batchSize)

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		fmt.Printf("Processing batch %d/%d (%d chunks)\n",
			(i/batchSize)+1, (len(chunks)+batchSize-1)/batchSize, len(batch))

		// Process each chunk in the batch with retries
		for _, chunk := range batch {
			var embedding []float32
			var err error

			for retry := 0; retry < maxRetries; retry++ {
				embedding, err = GetEmbedding(chunk.Content, config)
				if err == nil {
					break
				}

				if retry < maxRetries-1 {
					fmt.Printf("  Retry %d/%d for chunk %s: %v\n", retry+1, maxRetries, chunk.ID, err)
					time.Sleep(time.Duration(retry+1) * time.Second) // Exponential backoff
				}
			}

			if err != nil {
				return nil, fmt.Errorf("failed to get embedding for chunk %s after %d retries: %w", chunk.ID, maxRetries, err)
			}

			embeddings[chunk.ID] = embedding
		}

		// Small delay between batches for Ollama mode
		if config.EmbeddingMode != EmbeddingModeLocal && end < len(chunks) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return embeddings, nil
}

// CreateEmbeddingFunc creates an embedding function for chromem-go
func CreateEmbeddingFunc(config Config) func(context.Context, string) ([]float32, error) {
	return func(ctx context.Context, text string) ([]float32, error) {
		return GetEmbedding(text, config)
	}
}
