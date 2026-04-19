package skill

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/compose"
)

// GraphExecutor adapts a graph-backed skill into a directly invokable runtime object.
// It keeps the skill abstraction intact while standardizing graph compile/invoke usage.
type GraphExecutor struct {
	skill          Runnable
	compileOptions []compose.GraphCompileOption

	once     sync.Once
	compiled compose.Runnable[map[string]any, map[string]any]
	err      error
}

// NewGraphExecutor creates a reusable executor for a graph-backed skill.
func NewGraphExecutor(skill Runnable, opts ...compose.GraphCompileOption) (*GraphExecutor, error) {
	if skill == nil {
		return nil, fmt.Errorf("skill runnable is required")
	}
	return &GraphExecutor{
		skill:          skill,
		compileOptions: append([]compose.GraphCompileOption(nil), opts...),
	}, nil
}

// CompileGraph compiles the graph exported by the given skill into a typed runnable.
func CompileGraph(ctx context.Context, skill Runnable, opts ...compose.GraphCompileOption) (compose.Runnable[map[string]any, map[string]any], error) {
	executor, err := NewGraphExecutor(skill, opts...)
	if err != nil {
		return nil, err
	}
	return executor.Compiled(ctx)
}

// Compiled returns the compiled graph runnable for the underlying skill.
func (e *GraphExecutor) Compiled(ctx context.Context) (compose.Runnable[map[string]any, map[string]any], error) {
	if e == nil || e.skill == nil {
		return nil, fmt.Errorf("skill runnable is required")
	}
	e.once.Do(func() {
		graph, ok, err := e.skill.Graph(ctx)
		if err != nil {
			e.err = err
			return
		}
		if !ok || graph == nil {
			e.err = fmt.Errorf("skill %q does not expose an executable graph", skillName(e.skill))
			return
		}
		workflow, ok := any(graph).(interface {
			Compile(context.Context, ...compose.GraphCompileOption) (compose.Runnable[map[string]any, map[string]any], error)
		})
		if !ok {
			e.err = fmt.Errorf("skill %q graph %T does not expose typed Compile for map[string]any input/output", skillName(e.skill), graph)
			return
		}
		e.compiled, e.err = workflow.Compile(ctx, e.compileOptions...)
		if e.err != nil {
			e.err = fmt.Errorf("compile skill %q graph: %w", skillName(e.skill), e.err)
		}
	})
	return e.compiled, e.err
}

// Invoke compiles the skill graph lazily on first use and then runs it.
func (e *GraphExecutor) Invoke(ctx context.Context, input map[string]any, opts ...compose.Option) (map[string]any, error) {
	compiled, err := e.Compiled(ctx)
	if err != nil {
		return nil, err
	}
	return compiled.Invoke(ctx, input, opts...)
}

func skillName(skill Runnable) string {
	if skill == nil {
		return ""
	}
	return skill.Info().Name
}
