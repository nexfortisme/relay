package tools

import (
	"context"
	"fmt"
)

type CompositeRuntime struct {
	runtimes []Runtime
}

func NewCompositeRuntime(runtimes ...Runtime) *CompositeRuntime {
	return &CompositeRuntime{runtimes: runtimes}
}

func (r *CompositeRuntime) Definitions(ctx context.Context) ([]Definition, error) {
	defs := make([]Definition, 0)
	for _, runtime := range r.runtimes {
		current, err := runtime.Definitions(ctx)
		if err != nil {
			return nil, err
		}
		defs = append(defs, current...)
	}
	return defs, nil
}

func (r *CompositeRuntime) Execute(ctx context.Context, call Call) (Result, error) {
	for _, runtime := range r.runtimes {
		defs, err := runtime.Definitions(ctx)
		if err != nil {
			return Result{}, err
		}
		for _, def := range defs {
			if def.Name == call.Name {
				return runtime.Execute(ctx, call)
			}
		}
	}
	return Result{}, fmt.Errorf("tool not found: %s", call.Name)
}
