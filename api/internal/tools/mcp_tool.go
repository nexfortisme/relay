package tools

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool interface {
	ToolInput() any
	ToolHandlerFn() func(ctx context.Context, req *mcp.CallToolRequest, in any) (*mcp.CallToolResult, any, error)
	GetTool() *mcp.Tool
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}
