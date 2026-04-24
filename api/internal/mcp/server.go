package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/nexfortisme/relay/internal/tools"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	addr   string
	logger *slog.Logger
	server *http.Server
}

func NewServer(addr string, logger *slog.Logger) *Server {
	return &Server{
		addr:   addr,
		logger: logger,
	}
}

func (s *Server) Start() error {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "relay-tools",
		Version: "0.0.1",
	}, nil)

	registerTools(server, s.logger)

	handler := mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		return server
	}, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.logger.Info("mcp server starting", "addr", s.addr, "path", "/mcp")
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("mcp listen: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func registerTools(server *mcpsdk.Server, logger *slog.Logger) {
	weatherTool := &tools.WeatherTool{}
	mcpsdk.AddTool(server, weatherTool.GetTool(), loggedToolHandler(logger, weatherTool.GetTool().Name, func(ctx context.Context, req *mcpsdk.CallToolRequest, in tools.WeatherInput) (*mcpsdk.CallToolResult, any, error) {
		return weatherTool.ToolHandlerFn()(ctx, req, in)
	}))

	searchTool := &tools.SearchTool{}
	mcpsdk.AddTool(server, searchTool.GetTool(), loggedToolHandler(logger, searchTool.GetTool().Name, func(ctx context.Context, req *mcpsdk.CallToolRequest, in tools.SearchInput) (*mcpsdk.CallToolResult, any, error) {
		return searchTool.ToolHandlerFn()(ctx, req, in)
	}))

	fetchURLTool := &tools.FetchURLTool{}
	mcpsdk.AddTool(server, fetchURLTool.GetTool(), loggedToolHandler(logger, fetchURLTool.GetTool().Name, func(ctx context.Context, req *mcpsdk.CallToolRequest, in tools.FetchURLInput) (*mcpsdk.CallToolResult, any, error) {
		return fetchURLTool.ToolHandlerFn()(ctx, req, in)
	}))

	fetchURLsTool := &tools.FetchURLsTool{}
	mcpsdk.AddTool(server, fetchURLsTool.GetTool(), loggedToolHandler(logger, fetchURLsTool.GetTool().Name, func(ctx context.Context, req *mcpsdk.CallToolRequest, in tools.FetchURLsInput) (*mcpsdk.CallToolResult, any, error) {
		return fetchURLsTool.ToolHandlerFn()(ctx, req, in)
	}))

	timeTool := &tools.TimeTool{}
	mcpsdk.AddTool(server, timeTool.GetTool(), loggedToolHandler(logger, timeTool.GetTool().Name, func(ctx context.Context, req *mcpsdk.CallToolRequest, in tools.TimeInput) (*mcpsdk.CallToolResult, any, error) {
		return timeTool.ToolHandlerFn()(ctx, req, in)
	}))
}

func loggedToolHandler[In, Out any](
	logger *slog.Logger,
	name string,
	handler func(context.Context, *mcpsdk.CallToolRequest, In) (*mcpsdk.CallToolResult, Out, error),
) func(context.Context, *mcpsdk.CallToolRequest, In) (*mcpsdk.CallToolResult, Out, error) {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest, in In) (*mcpsdk.CallToolResult, Out, error) {
		logger.Info("mcp tool called", "tool", name, "parameters", logValue(in))
		result, out, err := handler(ctx, req, in)
		if err != nil {
			logger.Error("mcp tool failed", "tool", name, "parameters", logValue(in), "error", err)
			return result, out, err
		}
		logger.Info(
			"mcp tool returned",
			"tool", name,
			"parameters", logValue(in),
			"response", logCallToolResult(result, out),
			"is_error", result != nil && result.IsError,
		)
		return result, out, nil
	}
}

func logValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%+v", value)
	}
	return string(encoded)
}

func logCallToolResult(result *mcpsdk.CallToolResult, output any) string {
	if result == nil {
		return logValue(output)
	}
	text := mcpTextContent(result)
	if text != "" {
		return text
	}
	if result.StructuredContent != nil {
		return logValue(result.StructuredContent)
	}
	return logValue(output)
}

func mcpTextContent(result *mcpsdk.CallToolResult) string {
	text := ""
	for _, content := range result.Content {
		if item, ok := content.(*mcpsdk.TextContent); ok {
			text += item.Text
		}
	}
	return text
}
