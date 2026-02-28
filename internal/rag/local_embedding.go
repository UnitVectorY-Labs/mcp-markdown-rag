package rag

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

const (
	// LocalEmbeddingDimension is the dimensionality of the local embedding vectors
	LocalEmbeddingDimension = 384
)

// GetLocalEmbedding generates a vector embedding for text using a self-contained
// hash-based approach. It uses feature hashing (the "hashing trick") with TF-IDF-like
// weighting to produce a fixed-dimension normalized vector suitable for cosine similarity.
// This requires no external services or model files.
func GetLocalEmbedding(text string) ([]float32, error) {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		// Return a zero vector for empty text
		return make([]float32, LocalEmbeddingDimension), nil
	}

	// Count term frequencies
	tf := make(map[string]int)
	for _, token := range tokens {
		tf[token]++
	}

	// Build the vector using feature hashing with sub-word features
	vector := make([]float64, LocalEmbeddingDimension)

	for term, count := range tf {
		// TF weighting: 1 + log(count)
		weight := 1.0 + math.Log(float64(count))

		// Hash the term into multiple buckets for better distribution
		addTermToVector(vector, term, weight)

		// Also add character n-grams (3-grams) for sub-word features
		// This helps capture morphological similarity
		if len(term) >= 3 {
			for i := 0; i <= len(term)-3; i++ {
				ngram := term[i : i+3]
				ngramWeight := weight * 0.3 // Lower weight for n-grams
				addTermToVector(vector, "#"+ngram+"#", ngramWeight)
			}
		}
	}

	// Also add bigram features for phrase matching
	for i := 0; i < len(tokens)-1; i++ {
		bigram := tokens[i] + "_" + tokens[i+1]
		addTermToVector(vector, bigram, 0.5)
	}

	// L2 normalize the vector
	vector = l2Normalize(vector)

	// Convert to float32
	result := make([]float32, LocalEmbeddingDimension)
	for i, v := range vector {
		result[i] = float32(v)
	}

	return result, nil
}

// addTermToVector adds a term's contribution to the vector using multiple hash functions
func addTermToVector(vector []float64, term string, weight float64) {
	// Use two hash functions for better distribution
	h1 := hashString(term, 0)
	h2 := hashString(term, 1)

	// Primary bucket
	idx1 := h1 % uint64(len(vector))
	// Sign hash to reduce collisions
	if h2%2 == 0 {
		vector[idx1] += weight
	} else {
		vector[idx1] -= weight
	}

	// Secondary bucket for richer representation
	idx2 := h2 % uint64(len(vector))
	h3 := hashString(term, 2)
	if h3%2 == 0 {
		vector[idx2] += weight * 0.5
	} else {
		vector[idx2] -= weight * 0.5
	}
}

// hashString computes a hash of the string with a seed
func hashString(s string, seed uint64) uint64 {
	h := fnv.New64a()
	// Write seed bytes
	seedBytes := [8]byte{
		byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24),
		byte(seed >> 32), byte(seed >> 40), byte(seed >> 48), byte(seed >> 56),
	}
	h.Write(seedBytes[:])
	h.Write([]byte(s))
	return h.Sum64()
}

// l2Normalize normalizes a vector to unit length
func l2Normalize(vector []float64) []float64 {
	var sum float64
	for _, v := range vector {
		sum += v * v
	}
	norm := math.Sqrt(sum)
	if norm == 0 {
		return vector
	}
	result := make([]float64, len(vector))
	for i, v := range vector {
		result[i] = v / norm
	}
	return result
}

// tokenize splits text into lowercase tokens, removing punctuation and stop words
func tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Split on non-alphanumeric characters
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				token := current.String()
				if !isStopWord(token) && len(token) > 1 {
					tokens = append(tokens, token)
				}
				current.Reset()
			}
		}
	}
	// Don't forget the last token
	if current.Len() > 0 {
		token := current.String()
		if !isStopWord(token) && len(token) > 1 {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

// isStopWord returns true if the word is a common English stop word
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true,
		"at": true, "be": true, "by": true, "for": true, "from": true,
		"has": true, "he": true, "in": true, "is": true, "it": true,
		"its": true, "of": true, "on": true, "or": true, "that": true,
		"the": true, "to": true, "was": true, "were": true, "will": true,
		"with": true, "this": true, "but": true, "they": true, "have": true,
		"had": true, "not": true, "been": true, "she": true, "her": true,
		"his": true, "their": true, "which": true, "would": true, "there": true,
		"what": true, "about": true, "if": true, "do": true, "does": true,
		"did": true, "can": true, "could": true, "should": true, "may": true,
		"might": true, "shall": true, "so": true, "than": true, "then": true,
		"these": true, "those": true, "each": true, "all": true, "any": true,
		"both": true, "few": true, "more": true, "most": true, "other": true,
		"some": true, "such": true, "no": true, "nor": true, "only": true,
		"own": true, "same": true, "too": true, "very": true, "just": true,
		"because": true, "how": true, "when": true, "where": true, "who": true,
		"whom": true, "why": true, "also": true, "into": true, "through": true,
		"during": true, "before": true, "after": true, "above": true, "below": true,
		"between": true, "out": true, "up": true, "down": true, "over": true,
		"under": true, "again": true, "further": true, "once": true, "here": true,
		"am": true, "you": true, "your": true, "we": true, "our": true,
		"my": true, "me": true, "him": true, "them": true, "us": true,
	}
	return stopWords[word]
}
