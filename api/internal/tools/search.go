package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SearchTool struct {
	Tool
}

type SearchInput struct {
	Query string `json:"query" jsonschema:"the search query"`
}

type searxngResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Engine  string `json:"engine"`
	} `json:"results"`
}

func (t *SearchTool) ToolInput() any {
	return SearchInput{}
}

func (t *SearchTool) ToolHandlerFn() func(ctx context.Context, req *mcp.CallToolRequest, in any) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in any) (*mcp.CallToolResult, any, error) {
		input, ok := in.(SearchInput)
		if !ok {
			return nil, nil, fmt.Errorf("invalid input type for web_search")
		}
		result, err := webSearch(ctx, input.Query)
		if err != nil {
			return nil, nil, err
		}
		return textResult(result), nil, nil
	}
}

func (t *SearchTool) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "web_search",
		Description: "Search the web using SearXNG and return JSON results with titles, URLs, snippets, and engines.",
	}
}

func searxngBaseURL() string {
	if ep := os.Getenv("SEARXNG_URL"); ep != "" {
		return ep
	}
	return "http://localhost:8888"
}

func webSearch(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query argument required")
	}

	searchURL := fmt.Sprintf("%s/search?q=%s&format=json", searxngBaseURL(), url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create search request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("searxng returned status %d", resp.StatusCode)
	}

	var raw searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}
	return string(out), nil
}
