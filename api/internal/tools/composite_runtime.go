package tools

import (
	"context"
	"fmt"
	"sync"
)

// CompositeRuntime fans Definitions/Execute out across a set of underlying
// runtimes. Tool ownership (which runtime answers which tool name) is cached
// on every Definitions call so Execute doesn't need to re-list (which, for
// the MCP runtime, would mean re-connecting on every tool call).
type CompositeRuntime struct {
	runtimes []Runtime

	mu      sync.Mutex
	routing map[string]Runtime
}

func NewCompositeRuntime(runtimes ...Runtime) *CompositeRuntime {
	return &CompositeRuntime{runtimes: runtimes}
}

func (r *CompositeRuntime) Definitions(ctx context.Context) ([]Definition, error) {
	defs := make([]Definition, 0)
	routing := make(map[string]Runtime)
	for _, runtime := range r.runtimes {
		current, err := runtime.Definitions(ctx)
		if err != nil {
			return nil, err
		}
		for _, def := range current {
			routing[def.Name] = runtime
		}
		defs = append(defs, current...)
	}

	r.mu.Lock()
	r.routing = routing
	r.mu.Unlock()
	return defs, nil
}

func (r *CompositeRuntime) Execute(ctx context.Context, call Call) (Result, error) {
	if runtime := r.lookup(call.Name); runtime != nil {
		return runtime.Execute(ctx, call)
	}
	// Routing may be cold or stale (e.g. a tool appeared after the last
	// Definitions call). Refresh once and retry.
	if _, err := r.Definitions(ctx); err != nil {
		return Result{}, err
	}
	if runtime := r.lookup(call.Name); runtime != nil {
		return runtime.Execute(ctx, call)
	}
	return Result{}, fmt.Errorf("tool not found: %s", call.Name)
}

func (r *CompositeRuntime) lookup(name string) Runtime {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.routing[name]
}
