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
)

type GraphAssembler interface {
	AssembleGraph(ctx context.Context, blueprint *schemad.GraphSpec) (compose.AnyGraph, error)
}

type InterpreterResolver interface {
	ResolveComponent(ctx context.Context, ref schemad.Ref) (any, error)
	ResolveFunction(ctx context.Context, ref schemad.Ref) (any, error)
	ResolveGraph(ctx context.Context, ref schemad.Ref) (compose.AnyGraph, error)
}

type DocumentLoader interface {
	LoadGraphSpec(ctx context.Context, ref schemad.Ref) (*schemad.GraphSpec, error)
	LoadComponentSpec(ctx context.Context, target string) (*schemad.ComponentSpec, error)
	LoadNode(ctx context.Context, ref schemad.Ref) (*schemad.NodeSpec, error)
}

type Resolver struct {
	Documents           DocumentLoader
	ComponentFactory    schemad.ComponentFactory
	InterpreterResolver InterpreterResolver
	GraphAssembler      GraphAssembler
}

func NewResolver(documents DocumentLoader, factory schemad.ComponentFactory, interpreter InterpreterResolver) *Resolver {
	return &Resolver{
		Documents:           documents,
		ComponentFactory:    factory,
		InterpreterResolver: interpreter,
	}
}

func (r *Resolver) WithGraphAssembler(assembler GraphAssembler) *Resolver {
	r.GraphAssembler = assembler
	return r
}

func (r *Resolver) ResolveTool(ctx context.Context, ref schemad.Ref) (tool.BaseTool, error) {
	value, err := r.ResolveValue(ctx, ref)
	if err != nil {
		return nil, err
	}
	return schemad.AsTool(value)
}

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

func (r *Resolver) ResolveGraph(ctx context.Context, ref schemad.Ref) (compose.AnyGraph, error) {
	switch ref.Kind {
	case schemad.RefKindGraphDocument:
		if r.Documents == nil {
			return nil, fmt.Errorf("document loader is required for graph ref %s", ref.Target)
		}
		spec, err := r.Documents.LoadGraphSpec(ctx, ref)
		if err != nil {
			return nil, err
		}
		if r.GraphAssembler != nil {
			return r.GraphAssembler.AssembleGraph(ctx, spec)
		}
		return nil, fmt.Errorf("graph assembler is required for graph ref %s", ref.Target)
	case schemad.RefKindInterpreterGraph:
		if r.InterpreterResolver == nil {
			return nil, fmt.Errorf("interpreter resolver is required for graph ref %s", ref.Target)
		}
		return r.InterpreterResolver.ResolveGraph(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported graph ref kind %s", ref.Kind)
	}
}

func (r *Resolver) ResolveValue(ctx context.Context, ref schemad.Ref) (any, error) {
	switch ref.Kind {
	case schemad.RefKindComponentDocument:
		if r.Documents == nil {
			return nil, fmt.Errorf("document loader is required for component ref %s", ref.Target)
		}
		spec, err := r.Documents.LoadComponentSpec(ctx, ref.Target)
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
	resolver InterpreterResolver
}

func (a componentResolverAdapter) ResolveComponent(ctx context.Context, ref schemad.Ref) (any, error) {
	if a.resolver == nil {
		return nil, fmt.Errorf("component resolver is required")
	}
	return a.resolver.ResolveComponent(ctx, ref)
}
