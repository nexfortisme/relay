package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mcpResultOutput(result *mcp.CallToolResult, marshalContext string) (string, error) {
	var sb strings.Builder
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			sb.WriteString(text.Text)
		}
	}
	if sb.Len() > 0 {
		return sb.String(), nil
	}
	if result.StructuredContent != nil {
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return "", fmt.Errorf("marshal %s structured content: %w", marshalContext, err)
		}
		return string(encoded), nil
	}
	return "", nil
}
