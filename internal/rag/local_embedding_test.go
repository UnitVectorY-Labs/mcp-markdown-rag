package rag

import (
	"math"
	"testing"
)

func TestGetLocalEmbedding_BasicOutput(t *testing.T) {
	embedding, err := GetLocalEmbedding("Hello world this is a test document about machine learning")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embedding) != LocalEmbeddingDimension {
		t.Errorf("expected dimension %d, got %d", LocalEmbeddingDimension, len(embedding))
	}
}

func TestGetLocalEmbedding_EmptyInput(t *testing.T) {
	embedding, err := GetLocalEmbedding("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(embedding) != LocalEmbeddingDimension {
		t.Errorf("expected dimension %d, got %d", LocalEmbeddingDimension, len(embedding))
	}
	// All values should be zero for empty input
	for i, v := range embedding {
		if v != 0 {
			t.Errorf("expected zero at index %d, got %f", i, v)
			break
		}
	}
}

func TestGetLocalEmbedding_Deterministic(t *testing.T) {
	text := "Kubernetes container orchestration and deployment"
	emb1, err := GetLocalEmbedding(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	emb2, err := GetLocalEmbedding(text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := range emb1 {
		if emb1[i] != emb2[i] {
			t.Errorf("embeddings differ at index %d: %f vs %f", i, emb1[i], emb2[i])
			break
		}
	}
}

func TestGetLocalEmbedding_Normalized(t *testing.T) {
	embedding, err := GetLocalEmbedding("This is a test document with enough words to generate a vector")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var sumSquared float64
	for _, v := range embedding {
		sumSquared += float64(v) * float64(v)
	}
	norm := math.Sqrt(sumSquared)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("expected L2 norm ~1.0, got %f", norm)
	}
}

func TestGetLocalEmbedding_SimilarTextsMoreSimilar(t *testing.T) {
	// Two similar texts about the same topic
	emb1, _ := GetLocalEmbedding("machine learning algorithms and neural networks for classification")
	emb2, _ := GetLocalEmbedding("neural networks and machine learning for pattern classification")
	// A dissimilar text
	emb3, _ := GetLocalEmbedding("cooking recipes for baking chocolate cake with butter and flour")

	sim12 := cosineSimilarity(emb1, emb2)
	sim13 := cosineSimilarity(emb1, emb3)

	if sim12 <= sim13 {
		t.Errorf("expected similar texts to have higher similarity: sim(1,2)=%f, sim(1,3)=%f", sim12, sim13)
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello World! This is a test.")
	// "hello", "world", "test" - stop words removed, lowercase
	expected := map[string]bool{
		"hello": true,
		"world": true,
		"test":  true,
	}
	for _, token := range tokens {
		if !expected[token] {
			t.Errorf("unexpected token: %s", token)
		}
	}
	// Check that stop words are removed
	for _, token := range tokens {
		if isStopWord(token) {
			t.Errorf("stop word not removed: %s", token)
		}
	}
}

func TestIsStopWord(t *testing.T) {
	if !isStopWord("the") {
		t.Error("expected 'the' to be a stop word")
	}
	if !isStopWord("is") {
		t.Error("expected 'is' to be a stop word")
	}
	if isStopWord("kubernetes") {
		t.Error("expected 'kubernetes' to NOT be a stop word")
	}
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
