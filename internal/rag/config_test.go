package rag

import (
	"os"
	"testing"
)

func TestGetConfig_Defaults(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("RAG_OLLAMA_URL")
	os.Unsetenv("RAG_EMBEDDING_MODEL")
	os.Unsetenv("RAG_DB_PATH")
	os.Unsetenv("RAG_EMBEDDING_MODE")

	ollamaURL := ""
	embeddingModel := ""
	dbPath := ""
	embeddingMode := ""

	config := GetConfig(&ollamaURL, &embeddingModel, &dbPath, &embeddingMode,
		"http://localhost:11434/api/embeddings", "nomic-embed-text", "./rag.db")

	if config.EmbeddingMode != EmbeddingModeLocal {
		t.Errorf("expected default embedding mode 'local', got '%s'", config.EmbeddingMode)
	}
	if config.OllamaURL != "http://localhost:11434/api/embeddings" {
		t.Errorf("expected default Ollama URL, got '%s'", config.OllamaURL)
	}
	if config.EmbeddingModel != "nomic-embed-text" {
		t.Errorf("expected default embedding model, got '%s'", config.EmbeddingModel)
	}
}

func TestGetConfig_CLIOverride(t *testing.T) {
	os.Unsetenv("RAG_EMBEDDING_MODE")

	ollamaURL := "http://custom:1234"
	embeddingModel := "custom-model"
	dbPath := "/tmp/test.db"
	embeddingMode := "ollama"

	config := GetConfig(&ollamaURL, &embeddingModel, &dbPath, &embeddingMode,
		"http://localhost:11434/api/embeddings", "nomic-embed-text", "./rag.db")

	if config.EmbeddingMode != "ollama" {
		t.Errorf("expected embedding mode 'ollama', got '%s'", config.EmbeddingMode)
	}
	if config.OllamaURL != "http://custom:1234" {
		t.Errorf("expected custom Ollama URL, got '%s'", config.OllamaURL)
	}
	if config.EmbeddingModel != "custom-model" {
		t.Errorf("expected custom embedding model, got '%s'", config.EmbeddingModel)
	}
}

func TestGetConfig_EnvVarOverride(t *testing.T) {
	os.Setenv("RAG_EMBEDDING_MODE", "ollama")
	os.Setenv("RAG_OLLAMA_URL", "http://env:5678")
	defer func() {
		os.Unsetenv("RAG_EMBEDDING_MODE")
		os.Unsetenv("RAG_OLLAMA_URL")
	}()

	ollamaURL := ""
	embeddingModel := ""
	dbPath := ""
	embeddingMode := ""

	config := GetConfig(&ollamaURL, &embeddingModel, &dbPath, &embeddingMode,
		"http://localhost:11434/api/embeddings", "nomic-embed-text", "./rag.db")

	if config.EmbeddingMode != "ollama" {
		t.Errorf("expected embedding mode 'ollama' from env, got '%s'", config.EmbeddingMode)
	}
	if config.OllamaURL != "http://env:5678" {
		t.Errorf("expected Ollama URL from env, got '%s'", config.OllamaURL)
	}
}
