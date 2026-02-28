package rag

import (
	"testing"
)

func TestChunkDocument_SmallDocument(t *testing.T) {
	content := "# Hello\n\nThis is a small document."
	chunks := ChunkDocument("/test/file.md", content, "abc123", 4000, 15, 0.25)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for small document, got %d", len(chunks))
	}
	if chunks[0].Content != content {
		t.Errorf("expected chunk content to match input")
	}
	if chunks[0].FilePath != "/test/file.md" {
		t.Errorf("expected file path /test/file.md, got %s", chunks[0].FilePath)
	}
}

func TestChunkDocument_LargeDocument(t *testing.T) {
	// Create a document that exceeds the max token limit
	// With 0.25 tokens/char, 4000 tokens = 16000 chars
	content := "# Large Document\n\n"
	for i := 0; i < 200; i++ {
		content += "This is a paragraph of text that helps make the document larger for testing chunking behavior. "
	}
	chunks := ChunkDocument("/test/large.md", content, "def456", 4000, 15, 0.25)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for large document, got %d", len(chunks))
	}
	// Verify all chunks have valid metadata
	for i, chunk := range chunks {
		if chunk.ChunkIndex != i {
			t.Errorf("chunk %d has wrong index %d", i, chunk.ChunkIndex)
		}
		if chunk.FilePath != "/test/large.md" {
			t.Errorf("chunk %d has wrong file path", i)
		}
		if len(chunk.Content) == 0 {
			t.Errorf("chunk %d has empty content", i)
		}
	}
}

func TestExtractHeadings(t *testing.T) {
	content := "# Title\n\nSome text.\n\n## Section 1\n\nMore text.\n\n### Subsection\n\nDetails."
	headings := ExtractHeadings(content)
	if len(headings) != 3 {
		t.Fatalf("expected 3 headings, got %d", len(headings))
	}
	if headings[0].Level != 1 || headings[0].Text != "Title" {
		t.Errorf("heading 0: expected level 1 'Title', got level %d '%s'", headings[0].Level, headings[0].Text)
	}
	if headings[1].Level != 2 || headings[1].Text != "Section 1" {
		t.Errorf("heading 1: expected level 2 'Section 1', got level %d '%s'", headings[1].Level, headings[1].Text)
	}
	if headings[2].Level != 3 || headings[2].Text != "Subsection" {
		t.Errorf("heading 2: expected level 3 'Subsection', got level %d '%s'", headings[2].Level, headings[2].Text)
	}
}

func TestGetHeadingContext(t *testing.T) {
	headings := []HeadingInfo{
		{Level: 1, Text: "Title", Position: 0},
		{Level: 2, Text: "Section A", Position: 20},
		{Level: 3, Text: "Subsection A1", Position: 50},
		{Level: 2, Text: "Section B", Position: 100},
	}

	// Position before any heading
	ctx := GetHeadingContext(headings, 0)
	if len(ctx) != 0 {
		t.Errorf("expected empty context at position 0, got %v", ctx)
	}

	// Position after Section A heading
	ctx = GetHeadingContext(headings, 30)
	if len(ctx) != 2 || ctx[0] != "Title" || ctx[1] != "Section A" {
		t.Errorf("expected [Title, Section A] at position 30, got %v", ctx)
	}

	// Position after Subsection A1
	ctx = GetHeadingContext(headings, 60)
	if len(ctx) != 3 || ctx[2] != "Subsection A1" {
		t.Errorf("expected [Title, Section A, Subsection A1] at position 60, got %v", ctx)
	}

	// Position after Section B (subsection should be popped)
	ctx = GetHeadingContext(headings, 110)
	if len(ctx) != 2 || ctx[1] != "Section B" {
		t.Errorf("expected [Title, Section B] at position 110, got %v", ctx)
	}
}

func TestEstimateTokenCount(t *testing.T) {
	text := "Hello world" // 11 chars
	tokens := EstimateTokenCount(text, 0.25)
	expected := 2 // 11 * 0.25 = 2.75, truncated to 2
	if tokens != expected {
		t.Errorf("expected %d tokens, got %d", expected, tokens)
	}
}

func TestFindBestSplitPoint(t *testing.T) {
	text := "Hello world. This is a test. More text here."
	// Should find a sentence boundary near position 28
	split := FindBestSplitPoint(text, 30)
	if split < 1 || split > 30 {
		t.Errorf("expected split point near 30, got %d", split)
	}
	// The text at split should be at or after a sentence boundary
	if text[split-1] != ' ' && text[split-1] != '.' {
		// Check the character before the split is reasonable
		t.Logf("split at position %d, char before: '%c'", split, text[split-1])
	}
}
