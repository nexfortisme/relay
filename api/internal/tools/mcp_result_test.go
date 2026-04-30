package tools

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPResultOutputPrefersTextContent(t *testing.T) {
	got, err := mcpResultOutput(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "alpha"},
			&mcp.TextContent{Text: " beta"},
		},
		StructuredContent: map[string]any{"ignored": true},
	}, "test")
	if err != nil {
		t.Fatalf("mcp result output: %v", err)
	}
	if got != "alpha beta" {
		t.Fatalf("expected text content, got %q", got)
	}
}

func TestMCPResultOutputFallsBackToStructuredContent(t *testing.T) {
	got, err := mcpResultOutput(&mcp.CallToolResult{
		StructuredContent: map[string]any{"ok": true},
	}, "test")
	if err != nil {
		t.Fatalf("mcp result output: %v", err)
	}
	if !strings.Contains(got, `"ok":true`) {
		t.Fatalf("expected structured JSON output, got %q", got)
	}
}
