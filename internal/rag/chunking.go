package rag

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DocumentChunk represents a chunk of a document with metadata
type DocumentChunk struct {
	ID          string    // Unique chunk ID (file_hash + chunk_index)
	FilePath    string    // Absolute path to source file
	FileHash    string    // Hash of the entire source file
	ChunkIndex  int       // Index of this chunk within the file
	Content     string    // The actual chunk content
	StartOffset int       // Character offset where chunk starts in original file
	EndOffset   int       // Character offset where chunk ends in original file
	TokenCount  int       // Estimated token count for this chunk
	HeadingPath []string  // Hierarchical heading context (e.g., ["Introduction", "Overview"])
	CreatedAt   time.Time // When this chunk was created
}

// HeadingInfo represents a markdown heading with its position
type HeadingInfo struct {
	Level    int    // Heading level (1-6 for #-######)
	Text     string // Heading text without the #
	Position int    // Character position in document
}

// EstimateTokenCount provides a rough estimate of token count for text
func EstimateTokenCount(text string, approxTokensPerChar float64) int {
	return int(float64(len(text)) * approxTokensPerChar)
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ExtractHeadings finds all markdown headings in the text
func ExtractHeadings(content string) []HeadingInfo {
	var headings []HeadingInfo
	lines := strings.Split(content, "\n")
	position := 0

	headingRegex := regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

	for _, line := range lines {
		if matches := headingRegex.FindStringSubmatch(strings.TrimSpace(line)); matches != nil {
			level := len(matches[1])
			text := strings.TrimSpace(matches[2])
			headings = append(headings, HeadingInfo{
				Level:    level,
				Text:     text,
				Position: position,
			})
		}
		position += len(line) + 1 // +1 for newline
	}

	return headings
}

// GetHeadingContext returns the hierarchical heading context for a given position
func GetHeadingContext(headings []HeadingInfo, position int) []string {
	var context []string
	var stack []HeadingInfo

	for _, heading := range headings {
		if heading.Position >= position {
			break
		}
		for len(stack) > 0 && stack[len(stack)-1].Level >= heading.Level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, heading)
	}
	for _, heading := range stack {
		context = append(context, heading.Text)
	}
	return context
}

// FindBestSplitPoint finds the best place to split text, preferring sentence boundaries
func FindBestSplitPoint(text string, maxPos int) int {
	if maxPos >= len(text) {
		return len(text)
	}
	for i := maxPos; i > maxPos-200 && i > 0; i-- {
		if text[i] == '.' || text[i] == '!' || text[i] == '?' {
			if i+1 >= len(text) || (text[i+1] == ' ' || text[i+1] == '\n') {
				return i + 1
			}
		}
	}
	for i := maxPos; i > maxPos-300 && i > 0; i-- {
		if i > 0 && text[i] == '\n' && text[i-1] == '\n' {
			return i
		}
	}
	for i := maxPos; i > maxPos-100 && i > 0; i-- {
		if text[i] == '\n' {
			return i + 1
		}
	}
	for i := maxPos; i > maxPos-50 && i > 0; i-- {
		if text[i] == ' ' {
			return i + 1
		}
	}
	return maxPos
}

// ChunkDocument splits a document into semantically coherent chunks
func ChunkDocument(filePath, content, fileHash string, maxTokensPerChunk, chunkOverlapPercent int, approxTokensPerChar float64) []DocumentChunk {
	var chunks []DocumentChunk

	// If document is small enough, return as single chunk
	if EstimateTokenCount(content, approxTokensPerChar) <= maxTokensPerChunk {
		chunk := DocumentChunk{
			ID:          fmt.Sprintf("%s_0", fileHash),
			FilePath:    filePath,
			FileHash:    fileHash,
			ChunkIndex:  0,
			Content:     content,
			StartOffset: 0,
			EndOffset:   len(content),
			TokenCount:  EstimateTokenCount(content, approxTokensPerChar),
			HeadingPath: []string{},
			CreatedAt:   time.Now(),
		}
		return []DocumentChunk{chunk}
	}

	headings := ExtractHeadings(content)
	fmt.Printf("  Found %d headings in document\n", len(headings))

	maxChunkChars := int(float64(maxTokensPerChunk) / approxTokensPerChar)
	overlapChars := int(float64(maxChunkChars) * float64(chunkOverlapPercent) / 100.0)

	fmt.Printf("  Max chunk chars: %d, overlap: %d\n", maxChunkChars, overlapChars)

	chunkIndex := 0
	start := 0
	contentLen := len(content)

	for start < contentLen {
		if chunkIndex > 1000 {
			fmt.Printf("  Warning: Too many chunks created, stopping at chunk %d\n", chunkIndex)
			break
		}
		idealEnd := start + maxChunkChars
		bestEnd := idealEnd
		bestHeadingLevel := 7

		// Only consider heading splits if we're at least 50% through the ideal chunk
		minHeadingSplitPos := start + (maxChunkChars / 2)

		for _, heading := range headings {
			if heading.Position > minHeadingSplitPos && heading.Position <= idealEnd {
				if heading.Level < bestHeadingLevel {
					bestEnd = heading.Position
					bestHeadingLevel = heading.Level
				}
			}
		}
		if bestEnd == idealEnd {
			bestEnd = FindBestSplitPoint(content, idealEnd)
		}
		if bestEnd > contentLen {
			bestEnd = contentLen
		}
		if bestEnd <= start {
			bestEnd = start + Min(maxChunkChars, contentLen-start)
		}
		chunkContent := content[start:bestEnd]
		if len(strings.TrimSpace(chunkContent)) == 0 {
			start = bestEnd
			continue
		}
		headingContext := GetHeadingContext(headings, start)
		chunk := DocumentChunk{
			ID:          fmt.Sprintf("%s_%d", fileHash, chunkIndex),
			FilePath:    filePath,
			FileHash:    fileHash,
			ChunkIndex:  chunkIndex,
			Content:     chunkContent,
			StartOffset: start,
			EndOffset:   bestEnd,
			TokenCount:  EstimateTokenCount(chunkContent, approxTokensPerChar),
			HeadingPath: headingContext,
			CreatedAt:   time.Now(),
		}
		chunks = append(chunks, chunk)
		if bestEnd >= contentLen {
			break
		}
		nextStart := bestEnd - overlapChars
		// Ensure we make meaningful progress - at least 10% of max chunk size
		minProgress := maxChunkChars / 10
		if nextStart <= start+minProgress {
			nextStart = start + minProgress
		}
		if nextStart >= contentLen {
			break
		}
		start = nextStart
		chunkIndex++
		if chunkIndex%10 == 0 {
			fmt.Printf("  Created %d chunks so far...\n", chunkIndex)
		}
	}
	fmt.Printf("  Chunking complete: %d chunks created\n", len(chunks))
	return chunks
}
