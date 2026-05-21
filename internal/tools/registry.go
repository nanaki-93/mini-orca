package tools

import (
	"context"
)

type Executor interface {
	Execute(ctx context.Context, projectPath, command, arguments string) (string, error)
}

type Registry struct {
	executors map[string]Executor
}

func NewRegistry() *Registry {
	reg := &Registry{
		executors: make(map[string]Executor),
	}
	// Register our tool modules
	reg.executors["shell"] = &GenericShellExecutor{}
	reg.executors["terraform"] = &TerraformExecutor{}
	return reg
}

func (r *Registry) RunTask(ctx context.Context, projectPath, toolType, action, args string) (string, error) {
	executor, exists := r.executors[toolType]
	if !exists {
		// Fallback to plain terminal shell execution if no specific tool matches
		executor = r.executors["shell"]
	}
	return executor.Execute(ctx, projectPath, action, args)
}
