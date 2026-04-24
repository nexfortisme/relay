package tools

import "context"

type MCPRuntime struct {
	definitions []Definition
}

func NewMCPRuntime(definitions []Definition) *MCPRuntime {
	return &MCPRuntime{definitions: definitions}
}

func (r *MCPRuntime) Definitions(context.Context) ([]Definition, error) {
	return r.definitions, nil
}

func (r *MCPRuntime) Execute(_ context.Context, call Call) (Result, error) {
	// Placeholder implementation for future MCP client wiring.
	return Result{
		Name:    call.Name,
		Output:  "mcp execution is not wired yet",
		IsError: true,
	}, nil
}
