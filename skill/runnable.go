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

type resolvedSkill struct {
	info        Info
	instruction string
	tools       []tool.BaseTool
	graph       compose.AnyGraph
	hasGraph    bool
	prompt      any
	hasPrompt   bool
	model       any
	hasModel    bool
	metadata    map[string]any
}

func (r *resolvedSkill) Info() Info {
	return r.info
}

func (r *resolvedSkill) Tools(ctx context.Context) ([]tool.BaseTool, error) {
	_ = ctx
	if len(r.tools) == 0 {
		return nil, nil
	}
	out := make([]tool.BaseTool, len(r.tools))
	copy(out, r.tools)
	return out, nil
}

func (r *resolvedSkill) Instruction(ctx context.Context) (string, error) {
	_ = ctx
	return r.instruction, nil
}

func (r *resolvedSkill) Graph(ctx context.Context) (compose.AnyGraph, bool, error) {
	_ = ctx
	return r.graph, r.hasGraph, nil
}

func (r *resolvedSkill) Prompt(ctx context.Context) (any, bool, error) {
	_ = ctx
	return r.prompt, r.hasPrompt, nil
}

func (r *resolvedSkill) Model(ctx context.Context) (any, bool, error) {
	_ = ctx
	return r.model, r.hasModel, nil
}
