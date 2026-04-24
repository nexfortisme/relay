package tools

import "context"

type Definition struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type Call struct {
	Name      string
	Arguments map[string]any
}

type Result struct {
	Name    string
	Output  any
	IsError bool
}

type Runtime interface {
	Definitions(ctx context.Context) ([]Definition, error)
	Execute(ctx context.Context, call Call) (Result, error)
}

type NoopRuntime struct{}

func (NoopRuntime) Definitions(context.Context) ([]Definition, error) {
	return []Definition{}, nil
}

func (NoopRuntime) Execute(_ context.Context, call Call) (Result, error) {
	return Result{
		Name:    call.Name,
		Output:  "tool runtime not configured",
		IsError: true,
	}, nil
}
