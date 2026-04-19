package skill

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
	orcbp "github.com/maizenut/mirorru/orchestration/blueprint"
	orccompiler "github.com/maizenut/mirorru/orchestration/compiler"
)

// Resolver resolves skill refs into runtime objects.
type Resolver struct {
	Documents           orcbp.DocumentLoader
	ComponentFactory    schemad.ComponentFactory
	InterpreterResolver orcbp.InterpreterResolver
}

// NewResolver creates a skill resolver backed by declarative infrastructure.
func NewResolver(documents orcbp.DocumentLoader, factory schemad.ComponentFactory, interpreter orcbp.InterpreterResolver) *Resolver {
	return &Resolver{
		Documents:           documents,
		ComponentFactory:    factory,
		InterpreterResolver: interpreter,
	}
}

// ResolveTool resolves a tool ref into a tool.BaseTool.
func (r *Resolver) ResolveTool(ctx context.Context, ref schemad.Ref) (tool.BaseTool, error) {
	value, err := r.ResolveValue(ctx, ref)
	if err != nil {
		return nil, err
	}
	return schemad.AsTool(value)
}

// ResolvePrompt resolves a prompt-like ref into a prompt runtime object.
func (r *Resolver) ResolvePrompt(ctx context.Context, ref schemad.Ref) (any, error) {
	value, err := r.ResolveValue(ctx, ref)
	if err != nil {
		return nil, err
	}
	if promptValue, ok := value.(prompt.ChatTemplate); ok {
		return promptValue, nil
	}
	if promptValue, ok := value.(prompt.AgenticChatTemplate); ok {
		return promptValue, nil
	}
	return nil, fmt.Errorf("resolved prompt ref %s to unsupported type %T", ref.Target, value)
}

// ResolveModel resolves a model-like ref into a model runtime object.
func (r *Resolver) ResolveModel(ctx context.Context, ref schemad.Ref) (any, error) {
	value, err := r.ResolveValue(ctx, ref)
	if err != nil {
		return nil, err
	}
	if modelValue, ok := value.(model.BaseChatModel); ok {
		return modelValue, nil
	}
	if modelValue, ok := value.(model.AgenticModel); ok {
		return modelValue, nil
	}
	return nil, fmt.Errorf("resolved model ref %s to unsupported type %T", ref.Target, value)
}

// ResolveGraph resolves a graph ref into compose.AnyGraph.
func (r *Resolver) ResolveGraph(ctx context.Context, ref schemad.Ref) (compose.AnyGraph, error) {
	switch ref.Kind {
	case schemad.RefKindGraphDocument:
		if r.Documents == nil {
			return nil, fmt.Errorf("document loader is required for graph ref %s", ref.Target)
		}
		loader := &orcbp.Loader{Documents: r.Documents}
		bp, err := loader.LoadGraph(ctx, ref)
		if err != nil {
			return nil, err
		}
		builder := orccompiler.NewSpecCompiler(r.Documents, r.ComponentFactory, r.InterpreterResolver, nil)
		return builder.AssembleGraph(ctx, bp)
	case schemad.RefKindInterpreterGraph:
		if r.InterpreterResolver == nil {
			return nil, fmt.Errorf("interpreter resolver is required for graph ref %s", ref.Target)
		}
		return r.InterpreterResolver.ResolveGraph(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported graph ref kind %s", ref.Kind)
	}
}

// ResolveValue resolves a non-graph ref into its runtime object.
func (r *Resolver) ResolveValue(ctx context.Context, ref schemad.Ref) (any, error) {
	switch ref.Kind {
	case schemad.RefKindComponentDocument:
		if r.Documents == nil {
			return nil, fmt.Errorf("document loader is required for component ref %s", ref.Target)
		}
		loader := &orcbp.Loader{Documents: r.Documents}
		spec, err := loader.LoadComponent(ctx, ref)
		if err != nil {
			return nil, err
		}
		return schemad.BuildComponent(ctx, spec, r.ComponentFactory, componentResolverAdapter{resolver: r.InterpreterResolver})
	case schemad.RefKindInterpreterComponent:
		if r.InterpreterResolver == nil {
			return nil, fmt.Errorf("interpreter resolver is required for component ref %s", ref.Target)
		}
		return r.InterpreterResolver.ResolveComponent(ctx, ref)
	case schemad.RefKindInterpreterFunction:
		if r.InterpreterResolver == nil {
			return nil, fmt.Errorf("interpreter resolver is required for function ref %s", ref.Target)
		}
		value, err := r.InterpreterResolver.ResolveFunction(ctx, ref)
		if err != nil {
			return nil, err
		}
		return materializeResolvedValue(ctx, value)
	default:
		return nil, fmt.Errorf("unsupported ref kind %s", ref.Kind)
	}
}

func materializeResolvedValue(ctx context.Context, value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Func {
		return value, nil
	}

	rt := rv.Type()
	switch {
	case rt.NumIn() == 0:
	case rt.NumIn() == 1 && rt.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()):
	default:
		return nil, fmt.Errorf("unsupported builder signature %s", rt.String())
	}

	switch rt.NumOut() {
	case 1:
	case 2:
		if !rt.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return nil, fmt.Errorf("unsupported builder signature %s", rt.String())
		}
	default:
		return nil, fmt.Errorf("unsupported builder signature %s", rt.String())
	}

	args := []reflect.Value{}
	if rt.NumIn() == 1 {
		args = append(args, reflect.ValueOf(ctx))
	}
	results := rv.Call(args)
	if len(results) == 2 && !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

type componentResolverAdapter struct {
	resolver orcbp.InterpreterResolver
}

func (a componentResolverAdapter) ResolveComponent(ctx context.Context, ref schemad.Ref) (any, error) {
	if a.resolver == nil {
		return nil, fmt.Errorf("component resolver is required")
	}
	return a.resolver.ResolveComponent(ctx, ref)
}
