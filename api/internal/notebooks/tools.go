package notebooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexfortisme/relay/internal/tools"
)

// ToolRuntime implements tools.Runtime scoped to a single notebook conversation.
// When notebookID is empty, Definitions returns nothing (tools are invisible).
type ToolRuntime struct {
	service    *Service
	registry   *Registry
	userID     string
	notebookID string
}

func NewToolRuntime(service *Service, registry *Registry, userID, notebookID string) *ToolRuntime {
	return &ToolRuntime{
		service:    service,
		registry:   registry,
		userID:     userID,
		notebookID: notebookID,
	}
}

func (r *ToolRuntime) Definitions(ctx context.Context) ([]tools.Definition, error) {
	if r.notebookID == "" {
		return nil, nil
	}
	return []tools.Definition{
		{
			Name:        "notebook_search_docs",
			Description: "Search document content (PDFs, text files) in the current notebook using full-text search. Returns relevant excerpts.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "The search query",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Max number of results (1-20, default 10)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "notebook_query_csv",
			Description: "Query rows from a CSV table in the notebook. Use notebook_search_docs to discover available table names first.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_name": map[string]any{
						"type":        "string",
						"description": "The CSV table name (e.g. csv_a1b2c3d4)",
					},
					"filters": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"column": map[string]any{"type": "string"},
								"op":     map[string]any{"type": "string", "enum": []string{"eq", "neq", "contains", "gt", "lt", "gte", "lte"}},
								"value":  map[string]any{"type": "string"},
							},
							"required": []string{"column", "op", "value"},
						},
						"description": "Optional filter conditions",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Max rows to return (default 100, max 500)",
					},
				},
				"required": []string{"table_name"},
			},
		},
		{
			Name:        "notebook_insert_csv_row",
			Description: "Insert a new row into a CSV table in the notebook.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_name": map[string]any{"type": "string", "description": "CSV table name"},
					"values": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{"type": "string"},
						"description":          "Column name → value pairs for the new row",
					},
				},
				"required": []string{"table_name", "values"},
			},
		},
		{
			Name:        "notebook_update_csv_row",
			Description: "Update rows in a CSV table matching the given filters.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_name": map[string]any{"type": "string"},
					"updates": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{"type": "string"},
						"description":          "Column name → new value pairs",
					},
					"filters": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "object",
							"required": []string{"column", "op", "value"},
							"properties": map[string]any{
								"column": map[string]any{"type": "string"},
								"op":     map[string]any{"type": "string"},
								"value":  map[string]any{"type": "string"},
							},
						},
					},
				},
				"required": []string{"table_name", "updates", "filters"},
			},
		},
		{
			Name: "notebook_delete_csv_row",
			Description: "Delete rows from a CSV table. IMPORTANT: Only call this after the user has explicitly confirmed the deletion in the conversation. " +
				"A snapshot of the database is created automatically before deleting.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_name": map[string]any{"type": "string"},
					"filters": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "object",
							"required": []string{"column", "op", "value"},
							"properties": map[string]any{
								"column": map[string]any{"type": "string"},
								"op":     map[string]any{"type": "string"},
								"value":  map[string]any{"type": "string"},
							},
						},
						"description": "Conditions identifying which rows to delete",
					},
				},
				"required": []string{"table_name", "filters"},
			},
		},
	}, nil
}

func (r *ToolRuntime) Execute(ctx context.Context, call tools.Call) (tools.Result, error) {
	switch call.Name {
	case "notebook_search_docs":
		return r.searchDocs(ctx, call.Arguments)
	case "notebook_query_csv":
		return r.queryCSV(ctx, call.Arguments)
	case "notebook_insert_csv_row":
		return r.insertCSVRow(ctx, call.Arguments)
	case "notebook_update_csv_row":
		return r.updateCSVRow(ctx, call.Arguments)
	case "notebook_delete_csv_row":
		return r.deleteCSVRow(ctx, call.Arguments)
	default:
		return tools.Result{Name: call.Name, IsError: true, Output: "unknown tool"}, nil
	}
}

func (r *ToolRuntime) searchDocs(ctx context.Context, args map[string]any) (tools.Result, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return tools.Result{Name: "notebook_search_docs", IsError: true, Output: "query is required"}, nil
	}
	limit := 10
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
		if limit > 20 {
			limit = 20
		}
	}

	db, err := r.registry.Open(r.userID, r.notebookID)
	if err != nil {
		return tools.Result{Name: "notebook_search_docs", IsError: true, Output: err.Error()}, nil
	}
	nbStore := NewNotebookStore(db)

	chunks, err := nbStore.SearchChunks(ctx, query, limit)
	if err != nil || len(chunks) == 0 {
		chunks, _ = nbStore.SampleChunks(ctx, limit)
	}

	var sb strings.Builder
	for _, c := range chunks {
		if c.PageNumber > 0 {
			sb.WriteString(fmt.Sprintf("[file_id:%s page:%d chunk:%d]\n%s\n\n", c.FileID, c.PageNumber, c.ChunkIndex, c.Content))
		} else {
			sb.WriteString(fmt.Sprintf("[file_id:%s chunk:%d]\n%s\n\n", c.FileID, c.ChunkIndex, c.Content))
		}
	}

	if sb.Len() == 0 {
		return tools.Result{Name: "notebook_search_docs", Output: "No matching documents found."}, nil
	}
	return tools.Result{Name: "notebook_search_docs", Output: sb.String()}, nil
}

func (r *ToolRuntime) queryCSV(ctx context.Context, args map[string]any) (tools.Result, error) {
	tableName, _ := args["table_name"].(string)
	limit := 100
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	filters := parseFilters(args["filters"])

	db, err := r.registry.Open(r.userID, r.notebookID)
	if err != nil {
		return tools.Result{Name: "notebook_query_csv", IsError: true, Output: err.Error()}, nil
	}
	nbStore := NewNotebookStore(db)

	rows, err := nbStore.QueryCSV(ctx, tableName, filters, limit)
	if err != nil {
		return tools.Result{Name: "notebook_query_csv", IsError: true, Output: err.Error()}, nil
	}

	out, _ := json.Marshal(map[string]any{"rows": rows, "count": len(rows)})
	return tools.Result{Name: "notebook_query_csv", Output: string(out)}, nil
}

func (r *ToolRuntime) insertCSVRow(ctx context.Context, args map[string]any) (tools.Result, error) {
	tableName, _ := args["table_name"].(string)
	rawValues, _ := args["values"].(map[string]any)

	values := make(map[string]string, len(rawValues))
	for k, v := range rawValues {
		values[k] = fmt.Sprintf("%v", v)
	}

	db, err := r.registry.Open(r.userID, r.notebookID)
	if err != nil {
		return tools.Result{Name: "notebook_insert_csv_row", IsError: true, Output: err.Error()}, nil
	}
	nbStore := NewNotebookStore(db)

	rowID, err := nbStore.InsertCSVRow(ctx, tableName, values)
	if err != nil {
		return tools.Result{Name: "notebook_insert_csv_row", IsError: true, Output: err.Error()}, nil
	}
	return tools.Result{Name: "notebook_insert_csv_row", Output: fmt.Sprintf(`{"inserted_rowid":%d}`, rowID)}, nil
}

func (r *ToolRuntime) updateCSVRow(ctx context.Context, args map[string]any) (tools.Result, error) {
	tableName, _ := args["table_name"].(string)
	rawUpdates, _ := args["updates"].(map[string]any)
	filters := parseFilters(args["filters"])

	updates := make(map[string]string, len(rawUpdates))
	for k, v := range rawUpdates {
		updates[k] = fmt.Sprintf("%v", v)
	}

	db, err := r.registry.Open(r.userID, r.notebookID)
	if err != nil {
		return tools.Result{Name: "notebook_update_csv_row", IsError: true, Output: err.Error()}, nil
	}
	nbStore := NewNotebookStore(db)

	n, err := nbStore.UpdateCSVRows(ctx, tableName, updates, filters)
	if err != nil {
		return tools.Result{Name: "notebook_update_csv_row", IsError: true, Output: err.Error()}, nil
	}
	return tools.Result{Name: "notebook_update_csv_row", Output: fmt.Sprintf(`{"rows_affected":%d}`, n)}, nil
}

func (r *ToolRuntime) deleteCSVRow(ctx context.Context, args map[string]any) (tools.Result, error) {
	tableName, _ := args["table_name"].(string)
	filters := parseFilters(args["filters"])

	if len(filters) == 0 {
		return tools.Result{Name: "notebook_delete_csv_row", IsError: true, Output: "filters required for delete to prevent accidental mass deletion"}, nil
	}

	// Snapshot before deleting
	if err := r.service.SnapshotBeforeDelete(r.userID, r.notebookID); err != nil {
		r.service.logger.Warn("snapshot before delete failed", "error", err)
	}

	db, err := r.registry.Open(r.userID, r.notebookID)
	if err != nil {
		return tools.Result{Name: "notebook_delete_csv_row", IsError: true, Output: err.Error()}, nil
	}
	nbStore := NewNotebookStore(db)

	n, err := nbStore.DeleteCSVRows(ctx, tableName, filters)
	if err != nil {
		return tools.Result{Name: "notebook_delete_csv_row", IsError: true, Output: err.Error()}, nil
	}
	return tools.Result{Name: "notebook_delete_csv_row", Output: fmt.Sprintf(`{"rows_deleted":%d}`, n)}, nil
}

// parseFilters decodes the filters argument from an LLM tool call.
func parseFilters(raw any) []Filter {
	if raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var filters []Filter
	_ = json.Unmarshal(b, &filters)
	return filters
}
