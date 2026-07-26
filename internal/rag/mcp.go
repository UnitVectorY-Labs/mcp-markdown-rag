package rag

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/philippgille/chromem-go"
)

// SearchResult represents a search result with file and chunk information
type SearchResult struct {
	FilePath    string
	Similarity  float32
	IsChunk     bool
	ChunkIndex  int
	StartOffset int
	EndOffset   int
	TokenCount  int
	HeadingPath string
}

// FileSearchResults groups search results by file
type FileSearchResults struct {
	FilePath string
	Chunks   []SearchResult
}

// RAGSearchInput is the input struct for the rag_search tool
type RAGSearchInput struct {
	Query      string `json:"query" jsonschema:"The search query to find relevant documentation"`
	MaxResults *int   `json:"max_results,omitempty" jsonschema:"Maximum number of results to return (default: 10)"`
}

// RAGSearchOutput is the output struct for the rag_search tool
type RAGSearchOutput struct {
	Result string `json:"result" jsonschema:"formatted search results"`
}

// RAGRetrieveInput is the input struct for the rag_retrieve tool
type RAGRetrieveInput struct {
	FilePath    string `json:"file_path" jsonschema:"The path to the file to retrieve content from"`
	StartOffset *int   `json:"start_offset,omitempty" jsonschema:"Starting character position (0-based)"`
	EndOffset   *int   `json:"end_offset,omitempty" jsonschema:"Ending character position (0-based)"`
}

// RAGRetrieveOutput is the output struct for the rag_retrieve tool
type RAGRetrieveOutput struct {
	Result string `json:"result" jsonschema:"formatted file content"`
}

// newMCPServer creates and configures the MCP server with RAG tools.
func newMCPServer(config Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "Markdown RAG Server", Version: "1.0.0"}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "rag_search",
		Description: "Search for relevant documentation using RAG (Retrieval-Augmented Generation). Returns a list of files with relevant chunks and their locations.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input RAGSearchInput) (
		*mcp.CallToolResult, RAGSearchOutput, error,
	) {
		maxResults := 10
		if input.MaxResults != nil {
			maxResults = *input.MaxResults
		}

		results, err := MCPSearchDocumentsWithResults(input.Query, config, maxResults)
		if err != nil {
			return nil, RAGSearchOutput{}, fmt.Errorf("search failed: %w", err)
		}

		fileResults := groupResultsByFile(results)

		var response strings.Builder
		response.WriteString(fmt.Sprintf("Found %d relevant file(s) for query: \"%s\"\n\n", len(fileResults), input.Query))

		for i, fileResult := range fileResults {
			response.WriteString(fmt.Sprintf("**File %d:** `%s`\n", i+1, fileResult.FilePath))

			if len(fileResult.Chunks) == 1 && !fileResult.Chunks[0].IsChunk {
				chunk := fileResult.Chunks[0]
				response.WriteString(fmt.Sprintf("- **Similarity:** %.4f\n", chunk.Similarity))
				response.WriteString("- **Type:** Complete file\n")
			} else {
				response.WriteString(fmt.Sprintf("- **Relevant chunks:** %d\n", len(fileResult.Chunks)))
				for j, chunk := range fileResult.Chunks {
					response.WriteString(fmt.Sprintf("  - **Chunk %d:**\n", j+1))
					response.WriteString(fmt.Sprintf("    - Similarity: %.4f\n", chunk.Similarity))
					response.WriteString(fmt.Sprintf("    - Range: characters %d-%d (%d tokens)\n",
						chunk.StartOffset, chunk.EndOffset, chunk.TokenCount))
					if chunk.HeadingPath != "" {
						response.WriteString(fmt.Sprintf("    - Context: %s\n", chunk.HeadingPath))
					}
				}
			}
			response.WriteString("\n")
		}

		response.WriteString("**Next Steps:**\n")
		response.WriteString("Use the `rag_retrieve` tool to get the actual content from specific files and ranges.\n")
		response.WriteString("Example: `rag_retrieve` with `file_path` and optionally `start_offset` and `end_offset`\n")

		text := response.String()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, RAGSearchOutput{Result: text}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "rag_retrieve",
		Description: "Retrieve specific content from a file, optionally specifying start and end positions for chunked content.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input RAGRetrieveInput) (
		*mcp.CallToolResult, RAGRetrieveOutput, error,
	) {
		content, err := MCPRetrieveFileContent(input.FilePath, input.StartOffset, input.EndOffset)
		if err != nil {
			return nil, RAGRetrieveOutput{}, fmt.Errorf("retrieval failed: %w", err)
		}

		var response strings.Builder
		response.WriteString(fmt.Sprintf("**File:** `%s`\n", input.FilePath))

		if input.StartOffset != nil || input.EndOffset != nil {
			start := 0
			if input.StartOffset != nil {
				start = *input.StartOffset
			}
			end := len(content)
			if input.EndOffset != nil {
				end = *input.EndOffset
			}
			response.WriteString(fmt.Sprintf("**Range:** characters %d-%d\n", start, end))
		} else {
			response.WriteString("**Range:** Complete file\n")
		}

		response.WriteString(fmt.Sprintf("**Content Length:** %d characters\n\n", len(content)))
		response.WriteString("**Content:**\n")
		response.WriteString("```markdown\n")
		response.WriteString(content)
		response.WriteString("\n```")

		text := response.String()
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, RAGRetrieveOutput{Result: text}, nil
	})

	return s
}

// RunMCPServer starts the MCP server over STDIO transport.
func RunMCPServer(config Config) error {
	s := newMCPServer(config)
	return s.Run(context.Background(), &mcp.StdioTransport{})
}

// RunSSEServer starts the MCP server over HTTP with SSE transport.
func RunSSEServer(config Config, addr string) error {
	s := newMCPServer(config)
	handler := mcp.NewSSEHandler(func(_ *http.Request) *mcp.Server {
		return s
	}, nil)
	log.Printf("MCP SSE server listening on %s", addr)
	return http.ListenAndServe(addr, handler)
}

// MCPSearchDocumentsWithResults searches for documents and returns structured results for MCP
func MCPSearchDocumentsWithResults(queryText string, config Config, maxResults int) ([]SearchResult, error) {
	if _, err := os.Stat(config.DBPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("database not found. Please run indexing first with -index")
	}

	db := chromem.NewDB()
	file, err := os.Open(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer file.Close()

	err = db.ImportFromReader(file, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load database: %w", err)
	}

	embeddingFunc := CreateEmbeddingFunc(config)

	collection := db.GetCollection("documents", embeddingFunc)
	if collection == nil {
		return nil, fmt.Errorf("documents collection not found in database")
	}

	count := collection.Count()

	if count == 0 {
		return nil, fmt.Errorf("no documents found in the database")
	}

	if maxResults > count {
		maxResults = count
	}

	results, err := collection.Query(context.Background(), queryText, maxResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no similar documents found")
	}

	searchResults := make([]SearchResult, 0, len(results))
	for _, result := range results {
		isChunk := result.Metadata["is_chunk"] == "true"

		searchResult := SearchResult{
			FilePath:    result.Metadata["file_path"],
			Similarity:  result.Similarity,
			IsChunk:     isChunk,
			HeadingPath: result.Metadata["heading_path"],
		}

		if isChunk {
			if chunkIndex, err := strconv.Atoi(result.Metadata["chunk_index"]); err == nil {
				searchResult.ChunkIndex = chunkIndex
			}
			if startOffset, err := strconv.Atoi(result.Metadata["start_offset"]); err == nil {
				searchResult.StartOffset = startOffset
			}
			if endOffset, err := strconv.Atoi(result.Metadata["end_offset"]); err == nil {
				searchResult.EndOffset = endOffset
			}
			if tokenCount, err := strconv.Atoi(result.Metadata["token_count"]); err == nil {
				searchResult.TokenCount = tokenCount
			}
		}

		searchResults = append(searchResults, searchResult)
	}

	return searchResults, nil
}

// MCPRetrieveFileContent retrieves content from a file with optional range
func MCPRetrieveFileContent(filePath string, startOffset, endOffset *int) (string, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", filePath)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	contentStr := string(content)
	contentLen := len(contentStr)

	start := 0
	end := contentLen

	if startOffset != nil {
		start = max(*startOffset, 0)
		if start > contentLen {
			start = contentLen
		}
	}

	if endOffset != nil {
		end = max(*endOffset, 0)
		if end > contentLen {
			end = contentLen
		}
	}

	if start > end {
		start = end
	}

	return contentStr[start:end], nil
}

// groupResultsByFile groups search results by file path and sorts chunks by position
func groupResultsByFile(results []SearchResult) []FileSearchResults {
	fileMap := make(map[string][]SearchResult)

	for _, result := range results {
		fileMap[result.FilePath] = append(fileMap[result.FilePath], result)
	}

	fileResults := make([]FileSearchResults, 0, len(fileMap))
	for filePath, chunks := range fileMap {
		sort.Slice(chunks, func(i, j int) bool {
			if chunks[i].IsChunk && chunks[j].IsChunk {
				return chunks[i].StartOffset < chunks[j].StartOffset
			}
			if !chunks[i].IsChunk && chunks[j].IsChunk {
				return true
			}
			if chunks[i].IsChunk && !chunks[j].IsChunk {
				return false
			}
			return chunks[i].Similarity > chunks[j].Similarity
		})

		fileResults = append(fileResults, FileSearchResults{
			FilePath: filePath,
			Chunks:   chunks,
		})
	}

	sort.Slice(fileResults, func(i, j int) bool {
		return fileResults[i].Chunks[0].Similarity > fileResults[j].Chunks[0].Similarity
	})

	return fileResults
}
