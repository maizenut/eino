package job

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
	skillpkg "github.com/cloudwego/eino/composite/skill"
	orcbp "github.com/maizenut/mirorru/orchestration/blueprint"
	orccompiler "github.com/maizenut/mirorru/orchestration/compiler"
)

// Resolver resolves task refs into runtime runnables.
type Resolver struct {
	Documents           orcbp.DocumentLoader
	ComponentFactory    schemad.ComponentFactory
	SkillLoader         skillpkg.SpecLoader
	SkillAssembler      skillpkg.Assembler
	InterpreterResolver orcbp.InterpreterResolver
}

// NewResolver creates a job resolver backed by shared declarative infrastructure.
func NewResolver(documents orcbp.DocumentLoader, factory schemad.ComponentFactory, skillLoader skillpkg.SpecLoader, skillAssembler skillpkg.Assembler, interpreter orcbp.InterpreterResolver) *Resolver {
	return &Resolver{
		Documents:           documents,
		ComponentFactory:    factory,
		SkillLoader:         skillLoader,
		SkillAssembler:      skillAssembler,
		InterpreterResolver: interpreter,
	}
}

// Resolve resolves the task target into an executable Runnable.
func (r *Resolver) Resolve(ctx context.Context, task *TaskSpec) (Runnable, error) {
	if task == nil {
		return nil, fmt.Errorf("task spec is required")
	}
	switch normalizedTargetType(task.TargetType, task.TargetRef.Kind) {
	case TargetTypeGraph:
		graphValue, err := r.ResolveGraph(ctx, task.TargetRef)
		if err != nil {
			return nil, err
		}
		return AsRunnable(ctx, graphValue)
	case TargetTypeSkill:
		skillValue, err := r.ResolveSkill(ctx, task.TargetRef)
		if err != nil {
			return nil, err
		}
		return AsRunnable(ctx, skillValue)
	default:
		value, err := r.ResolveValue(ctx, task.TargetRef)
		if err != nil {
			return nil, err
		}
		return AsRunnable(ctx, value)
	}
}

// ResolveGraph resolves a graph target.
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

// ResolveSkill resolves a skill target.
func (r *Resolver) ResolveSkill(ctx context.Context, ref schemad.Ref) (skillpkg.Runnable, error) {
	if ref.Kind != schemad.RefKindSkillDocument {
		value, err := r.ResolveValue(ctx, ref)
		if err != nil {
			return nil, err
		}
		runnable, ok := value.(skillpkg.Runnable)
		if !ok {
			return nil, fmt.Errorf("resolved skill ref %s to unsupported type %T", ref.Target, value)
		}
		return runnable, nil
	}
	if r.SkillLoader == nil {
		return nil, fmt.Errorf("skill loader is required for skill ref %s", ref.Target)
	}
	if r.SkillAssembler == nil {
		return nil, fmt.Errorf("skill assembler is required for skill ref %s", ref.Target)
	}
	spec, err := r.SkillLoader.LoadSkillSpec(ctx, ref.Target)
	if err != nil {
		return nil, err
	}
	if ref.Select != "" {
		if err := validateNamedSelect(ref, spec.Info.Name, "skill"); err != nil {
			return nil, err
		}
	}
	return r.SkillAssembler.Build(ctx, spec)
}

// ResolveValue resolves any non-graph target.
func (r *Resolver) ResolveValue(ctx context.Context, ref schemad.Ref) (any, error) {
	switch ref.Kind {
	case schemad.RefKindGraphDocument:
		return r.resolveBlueprintValue(ctx, ref)
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
	case schemad.RefKindSkillDocument:
		return r.ResolveSkill(ctx, ref)
	case schemad.RefKindInterpreterGraph:
		return r.ResolveGraph(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported ref kind %s", ref.Kind)
	}
}

func (r *Resolver) resolveBlueprintValue(ctx context.Context, ref schemad.Ref) (any, error) {
	if ref.Select == "" {
		return r.ResolveGraph(ctx, ref)
	}
	if r.Documents == nil {
		return nil, fmt.Errorf("document loader is required for blueprint ref %s", ref.Target)
	}
	selectSpec, err := schemad.ParseSelect(ref.Select)
	if err != nil {
		return nil, err
	}
	if selectSpec.Kind == "graph" {
		return r.ResolveGraph(ctx, ref)
	}
	if selectSpec.Kind != "node" {
		return nil, fmt.Errorf("unsupported blueprint select kind %s for ref %s", selectSpec.Kind, ref.Target)
	}
	loader := &orcbp.Loader{Documents: r.Documents}
	node, err := loader.LoadNode(ctx, ref)
	if err != nil {
		return nil, err
	}
	return r.resolveBlueprintNode(ctx, node)
}

func (r *Resolver) resolveBlueprintNode(ctx context.Context, node *schemad.NodeSpec) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("blueprint node is required")
	}
	switch node.Kind {
	case schemad.NodeKindComponent:
		if node.Component == nil {
			return nil, fmt.Errorf("component node %s is missing component ref", node.Key)
		}
		switch node.Component.Ref.Kind {
		case schemad.RefKindComponentDocument:
			if r.Documents == nil {
				return nil, fmt.Errorf("document loader is required for component node %s", node.Key)
			}
			loader := &orcbp.Loader{Documents: r.Documents}
			spec, err := loader.LoadComponent(ctx, node.Component.Ref)
			if err != nil {
				return nil, err
			}
			return schemad.BuildComponent(ctx, spec, r.ComponentFactory, componentResolverAdapter{resolver: r.InterpreterResolver})
		case schemad.RefKindInterpreterComponent:
			if r.InterpreterResolver == nil {
				return nil, fmt.Errorf("interpreter resolver is required for component node %s", node.Key)
			}
			return r.InterpreterResolver.ResolveComponent(ctx, node.Component.Ref)
		default:
			return nil, fmt.Errorf("unsupported component ref kind %s", node.Component.Ref.Kind)
		}
	case schemad.NodeKindGraph:
		if node.GraphRef == nil {
			return nil, fmt.Errorf("graph node %s is missing graph ref", node.Key)
		}
		return r.ResolveGraph(ctx, *node.GraphRef)
	case schemad.NodeKindLambda:
		if node.Lambda == nil || r.InterpreterResolver == nil {
			return nil, fmt.Errorf("lambda node %s cannot be resolved without interpreter resolver", node.Key)
		}
		value, err := r.InterpreterResolver.ResolveFunction(ctx, node.Lambda.Callable)
		if err != nil {
			return nil, err
		}
		return materializeResolvedValue(ctx, value)
	case schemad.NodeKindPassthrough:
		return runnableFunc(func(ctx context.Context, input map[string]any) (any, error) {
			_ = ctx
			return input, nil
		}), nil
	default:
		return nil, fmt.Errorf("unsupported blueprint node kind %s", node.Kind)
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
		return value, nil
	}
	switch rt.NumOut() {
	case 1:
	case 2:
		if !rt.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			return value, nil
		}
	default:
		return value, nil
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

func normalizedTargetType(targetType string, refKind string) string {
	switch targetType {
	case TargetTypeGraph, TargetTypeSkill, TargetTypeRunnable, TargetTypeFunction:
		return targetType
	}
	switch refKind {
	case schemad.RefKindGraphDocument, schemad.RefKindInterpreterGraph:
		return TargetTypeGraph
	case schemad.RefKindSkillDocument:
		return TargetTypeSkill
	case schemad.RefKindInterpreterFunction:
		return TargetTypeFunction
	default:
		return TargetTypeRunnable
	}
}

func validateNamedSelect(ref schemad.Ref, actualName, expectedKind string) error {
	sel, err := schemad.ParseSelect(ref.Select)
	if err != nil {
		return err
	}
	if sel.Kind != expectedKind {
		return fmt.Errorf("%s ref select must be %s, got %s", expectedKind, expectedKind, sel.Kind)
	}
	if sel.Name != actualName {
		return fmt.Errorf("%s %s not found in %s", expectedKind, sel.Name, ref.Target)
	}
	return nil
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
