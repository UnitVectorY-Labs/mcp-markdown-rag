package rag

import (
	"os"
	"path/filepath"
)

// Config holds all configuration values
type Config struct {
	OllamaURL      string
	EmbeddingModel string
	DBPath         string
}

// GetConfig returns configuration based on command line args, environment variables, and defaults
func GetConfig(ollamaURL, embeddingModel, dbPath *string, defaultOllamaURL, defaultEmbeddingModel, defaultDBPath string) Config {
	config := Config{}

	// Ollama URL priority: CLI arg -> env var -> default
	if *ollamaURL != "" {
		config.OllamaURL = *ollamaURL
	} else if envURL := os.Getenv("RAG_OLLAMA_URL"); envURL != "" {
		config.OllamaURL = envURL
	} else {
		config.OllamaURL = defaultOllamaURL
	}

	// Embedding Model priority: CLI arg -> env var -> default
	if *embeddingModel != "" {
		config.EmbeddingModel = *embeddingModel
	} else if envModel := os.Getenv("RAG_EMBEDDING_MODEL"); envModel != "" {
		config.EmbeddingModel = envModel
	} else {
		config.EmbeddingModel = defaultEmbeddingModel
	}

	// Database Path priority: CLI arg -> env var -> default
	if *dbPath != "" {
		config.DBPath = *dbPath
	} else if envDBPath := os.Getenv("RAG_DB_PATH"); envDBPath != "" {
		config.DBPath = envDBPath
	} else {
		config.DBPath = defaultDBPath
	}

	// Convert DB path to absolute path
	absDBPath, err := filepath.Abs(config.DBPath)
	if err == nil {
		config.DBPath = absDBPath
	}

	return config
}
