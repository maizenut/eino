package skill

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// Runnable is a runtime-ready skill view.
type Runnable interface {
	Info() Info
	Tools(ctx context.Context) ([]tool.BaseTool, error)
	Instruction(ctx context.Context) (string, error)
	Graph(ctx context.Context) (compose.AnyGraph, bool, error)
}

// PromptProvider is implemented by runnable skills that retain a resolved prompt ref.
type PromptProvider interface {
	Prompt(ctx context.Context) (any, bool, error)
}

// ModelProvider is implemented by runnable skills that retain a resolved model ref.
type ModelProvider interface {
	Model(ctx context.Context) (any, bool, error)
}
