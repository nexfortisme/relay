package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPRuntime struct {
	endpoint string
}

func NewMCPRuntime(endpoint string) *MCPRuntime {
	return &MCPRuntime{endpoint: endpoint}
}

func (r *MCPRuntime) Definitions(ctx context.Context) ([]Definition, error) {
	session, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, fmt.Errorf("list mcp tools: %w", err)
	}

	definitions := make([]Definition, 0, len(result.Tools))
	for _, tool := range result.Tools {
		definitions = append(definitions, Definition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaMap(tool.InputSchema),
		})
	}
	return definitions, nil
}

func (r *MCPRuntime) Execute(ctx context.Context, call Call) (Result, error) {
	session, err := r.connect(ctx)
	if err != nil {
		return Result{}, err
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      call.Name,
		Arguments: call.Arguments,
	})
	if err != nil {
		return Result{}, fmt.Errorf("call mcp tool %q: %w", call.Name, err)
	}

	output, err := mcpResultOutput(result, "mcp")
	if err != nil {
		return Result{}, err
	}
	return Result{
		Name:    call.Name,
		Output:  output,
		IsError: result.IsError,
	}, nil
}

func (r *MCPRuntime) connect(ctx context.Context) (*mcp.ClientSession, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "relay", Version: "0.0.1"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             r.endpoint,
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to mcp server at %s: %w", r.endpoint, err)
	}
	return session, nil
}

func schemaMap(schema any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object"}
	}
	if typed, ok := schema.(map[string]any); ok {
		return typed
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object"}
	}
	var out map[string]any
	if err := json.Unmarshal(encoded, &out); err != nil {
		return map[string]any{"type": "object"}
	}
	return out
}
