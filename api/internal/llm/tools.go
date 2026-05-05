package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/nexfortisme/relay/internal/prompts"
	"github.com/nexfortisme/relay/internal/tools"
)

type toolCallDelta struct {
	Index     int
	ID        string
	Type      string
	Name      string
	Arguments string
}

func openAITools(defs []tools.Definition) []openAITool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]openAITool, 0, len(defs))
	for _, def := range defs {
		parameters := normalizeToolParameters(def.InputSchema)
		out = append(out, openAITool{
			Type: "function",
			Function: openAIToolFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  parameters,
			},
		})
	}
	return out
}

func normalizeToolParameters(schema map[string]any) map[string]any {
	if schema == nil {
		schema = map[string]any{}
	}
	normalized := make(map[string]any, len(schema)+2)
	for key, value := range schema {
		normalized[key] = value
	}
	normalized["type"] = "object"
	if _, ok := normalized["properties"].(map[string]any); !ok {
		normalized["properties"] = map[string]any{}
	}
	if _, ok := normalized["required"].([]any); !ok {
		if _, hasRequired := normalized["required"]; !hasRequired {
			normalized["required"] = []any{}
		}
	}
	return normalized
}

func toolChoice(requestTools []openAITool) string {
	if len(requestTools) == 0 {
		return ""
	}
	return "auto"
}

func executeToolCall(ctx context.Context, runtime tools.Runtime, toolCall ToolCall) (tools.Result, error) {
	args := map[string]any{}
	if strings.TrimSpace(toolCall.Function.Arguments) != "" {
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return tools.Result{}, fmt.Errorf("parse tool arguments for %s: %w", toolCall.Function.Name, err)
		}
	}
	result, err := runtime.Execute(ctx, tools.Call{
		Name:      toolCall.Function.Name,
		Arguments: args,
	})
	if err != nil {
		return tools.Result{}, err
	}
	return result, nil
}

func toolResultContent(result tools.Result) (string, error) {
	if _, ok := result.Output.(string); ok {
		return result.Output.(string), nil
	}
	encoded, err := json.Marshal(result.Output)
	if err != nil {
		return "", fmt.Errorf("marshal tool result for %s: %w", result.Name, err)
	}
	return string(encoded), nil
}

func accumulateToolCall(calls map[int]*ToolCall, delta toolCallDelta) {
	call, ok := calls[delta.Index]
	if !ok {
		call = &ToolCall{Type: "function"}
		calls[delta.Index] = call
	}
	if delta.ID != "" {
		call.ID = delta.ID
	}
	if delta.Type != "" {
		call.Type = delta.Type
	}
	if delta.Name != "" {
		call.Function.Name += delta.Name
	}
	if delta.Arguments != "" {
		call.Function.Arguments += delta.Arguments
	}
}

func orderedToolCalls(calls map[int]*ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(calls))
	for index := range calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	out := make([]ToolCall, 0, len(indexes))
	for _, index := range indexes {
		call := *calls[index]
		if call.ID == "" {
			call.ID = fmt.Sprintf("tool-call-%d", index)
		}
		if call.Type == "" {
			call.Type = "function"
		}
		out = append(out, call)
	}
	return out
}

func isSearchOrFetchTool(name string) bool {
	return name == "web_search" || name == "fetch_url" || name == "fetch_urls"
}

func toolResultFailed(result tools.Result) bool {
	if result.IsError {
		return true
	}
	if text, ok := result.Output.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return result.Output == nil
}

func sendUnableToFind(ctx context.Context, out chan<- TokenEvent) error {
	msg, _ := prompts.Load(prompts.UnableToFind)
	if msg == "" {
		msg = "I can't find that out right now."
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- TokenEvent{Token: msg}:
		return nil
	}
}
