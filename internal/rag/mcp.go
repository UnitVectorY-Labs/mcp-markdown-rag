package rag

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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

// RunMCPServer starts the MCP server with RAG tools
func RunMCPServer(config Config) error {
	// Create a new MCP server
	s := server.NewMCPServer(
		"Markdown RAG Server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Add the RAG search tool
	searchTool := mcp.NewTool("rag_search",
		mcp.WithDescription("Search for relevant documentation using RAG (Retrieval-Augmented Generation). Returns a list of files with relevant chunks and their locations."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The search query to find relevant documentation"),
		),
		mcp.WithNumber("max_results",
			mcp.Description("Maximum number of results to return (default: 10)"),
		),
	)

	// Add the file retrieval tool
	retrieveTool := mcp.NewTool("rag_retrieve",
		mcp.WithDescription("Retrieve specific content from a file, optionally specifying start and end positions for chunked content."),
		mcp.WithString("file_path",
			mcp.Required(),
			mcp.Description("The path to the file to retrieve content from"),
		),
		mcp.WithNumber("start_offset",
			mcp.Description("Starting character position (0-based). If not specified, returns from beginning of file."),
		),
		mcp.WithNumber("end_offset",
			mcp.Description("Ending character position (0-based). If not specified, returns to end of file."),
		),
	)

	// Add the search tool handler
	s.AddTool(searchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error getting query parameter: %v", err)), nil
		}

		maxResults := request.GetInt("max_results", 10)

		// Perform the search
		results, err := MCPSearchDocumentsWithResults(query, config, maxResults)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Search failed: %v", err)), nil
		}

		// Group results by file
		fileResults := groupResultsByFile(results)

		// Format the response
		var response strings.Builder
		response.WriteString(fmt.Sprintf("Found %d relevant file(s) for query: \"%s\"\n\n", len(fileResults), query))

		for i, fileResult := range fileResults {
			response.WriteString(fmt.Sprintf("**File %d:** `%s`\n", i+1, fileResult.FilePath))

			if len(fileResult.Chunks) == 1 && !fileResult.Chunks[0].IsChunk {
				// Entire file match
				chunk := fileResult.Chunks[0]
				response.WriteString(fmt.Sprintf("- **Similarity:** %.4f\n", chunk.Similarity))
				response.WriteString("- **Type:** Complete file\n")
			} else {
				// Multiple chunks or single chunk
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

		return mcp.NewToolResultText(response.String()), nil
	})

	// Add the retrieve tool handler
	s.AddTool(retrieveTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filePath, err := request.RequireString("file_path")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error getting file_path parameter: %v", err)), nil
		}

		var startOffset, endOffset *int

		// Get optional start_offset
		if args := request.GetArguments(); args != nil {
			if startFloat, ok := args["start_offset"].(float64); ok {
				start := int(startFloat)
				startOffset = &start
			}
		}

		// Get optional end_offset
		if args := request.GetArguments(); args != nil {
			if endFloat, ok := args["end_offset"].(float64); ok {
				end := int(endFloat)
				endOffset = &end
			}
		}

		// Retrieve the content
		content, err := MCPRetrieveFileContent(filePath, startOffset, endOffset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Retrieval failed: %v", err)), nil
		}

		// Format the response
		var response strings.Builder
		response.WriteString(fmt.Sprintf("**File:** `%s`\n", filePath))

		if startOffset != nil || endOffset != nil {
			start := 0
			if startOffset != nil {
				start = *startOffset
			}
			end := len(content)
			if endOffset != nil {
				end = *endOffset
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

		return mcp.NewToolResultText(response.String()), nil
	})

	// Start the stdio server
	return server.ServeStdio(s)
}

// MCPSearchDocumentsWithResults searches for documents and returns structured results for MCP
func MCPSearchDocumentsWithResults(queryText string, config Config, maxResults int) ([]SearchResult, error) {
	// Load database
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

	// Create embedding function for Ollama
	embeddingFunc := CreateEmbeddingFunc(config)

	collection := db.GetCollection("documents", embeddingFunc)
	if collection == nil {
		return nil, fmt.Errorf("documents collection not found in database")
	}

	// Get collection count to determine max results
	count := collection.Count()

	if count == 0 {
		return nil, fmt.Errorf("no documents found in the database")
	}

	// Limit results to available documents
	if maxResults > count {
		maxResults = count
	}

	// Search for similar documents
	results, err := collection.Query(context.Background(), queryText, maxResults, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no similar documents found")
	}

	// Convert to SearchResult structs
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
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("file not found: %s", filePath)
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	contentStr := string(content)
	contentLen := len(contentStr)

	// Apply range if specified
	start := 0
	end := contentLen

	if startOffset != nil {
		start = *startOffset
		if start < 0 {
			start = 0
		}
		if start > contentLen {
			start = contentLen
		}
	}

	if endOffset != nil {
		end = *endOffset
		if end < 0 {
			end = 0
		}
		if end > contentLen {
			end = contentLen
		}
	}

	// Ensure start <= end
	if start > end {
		start = end
	}

	return contentStr[start:end], nil
}

// groupResultsByFile groups search results by file path and sorts chunks by position
func groupResultsByFile(results []SearchResult) []FileSearchResults {
	fileMap := make(map[string][]SearchResult)

	// Group by file path
	for _, result := range results {
		fileMap[result.FilePath] = append(fileMap[result.FilePath], result)
	}

	// Convert to slice and sort chunks within each file
	fileResults := make([]FileSearchResults, 0, len(fileMap))
	for filePath, chunks := range fileMap {
		// Sort chunks by start offset
		sort.Slice(chunks, func(i, j int) bool {
			if chunks[i].IsChunk && chunks[j].IsChunk {
				return chunks[i].StartOffset < chunks[j].StartOffset
			}
			// Non-chunks come first
			if !chunks[i].IsChunk && chunks[j].IsChunk {
				return true
			}
			if chunks[i].IsChunk && !chunks[j].IsChunk {
				return false
			}
			// Both non-chunks, sort by similarity
			return chunks[i].Similarity > chunks[j].Similarity
		})

		fileResults = append(fileResults, FileSearchResults{
			FilePath: filePath,
			Chunks:   chunks,
		})
	}

	// Sort files by best similarity score
	sort.Slice(fileResults, func(i, j int) bool {
		return fileResults[i].Chunks[0].Similarity > fileResults[j].Chunks[0].Similarity
	})

	return fileResults
}
