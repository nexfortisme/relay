package tools

import (
	"context"
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TimeTool struct {
	Tool
}

type TimeInput struct{}

func (t *TimeTool) ToolInput() any {
	return TimeInput{}
}

func (t *TimeTool) ToolHandlerFn() func(ctx context.Context, req *mcp.CallToolRequest, in any) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in any) (*mcp.CallToolResult, any, error) {
		result, err := currentTime()
		if err != nil {
			return nil, nil, err
		}
		return textResult(result), nil, nil
	}
}

func (t *TimeTool) GetTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "get_time",
		Description: "Get the current local time as an RFC3339 timestamp.",
	}
}

func currentTime() (string, error) {
	out, err := json.Marshal(map[string]string{
		"time": time.Now().Format(time.RFC3339),
	})
	if err != nil {
		return "", err
	}
	return string(out), nil
}
