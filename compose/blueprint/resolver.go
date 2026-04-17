package blueprint

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
)

// InterpreterResolver resolves function, component, and graph refs for declarative blueprints.
type InterpreterResolver interface {
	ResolveObject(ctx context.Context, ref schemad.Ref) (any, error)
	ResolveFunction(ctx context.Context, ref schemad.Ref) (any, error)
	ResolveComponent(ctx context.Context, ref schemad.Ref) (any, error)
	ResolveGraph(ctx context.Context, ref schemad.Ref) (compose.AnyGraph, error)
}

// DocumentLoader loads declarative resources from external documents.
type DocumentLoader interface {
	LoadGraphBlueprint(ctx context.Context, target string) (*schemad.GraphBlueprint, error)
	LoadComponentSpec(ctx context.Context, target string) (*schemad.ComponentSpec, error)
}

// ComponentFactory builds component instances from specs.
type ComponentFactory interface {
	BuildComponent(ctx context.Context, spec *schemad.ComponentSpec) (any, error)
}

// Loader resolves refs to blueprint documents, nodes, and component specs.
type Loader struct {
	Documents DocumentLoader
}

// LoadGraph resolves a graph-level ref into a blueprint.
func (l *Loader) LoadGraph(ctx context.Context, ref schemad.Ref) (*schemad.GraphBlueprint, error) {
	if l == nil || l.Documents == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	if ref.Kind != schemad.RefKindBlueprintDocument {
		return nil, fmt.Errorf("unsupported graph ref kind: %s", ref.Kind)
	}
	bp, err := l.Documents.LoadGraphBlueprint(ctx, ref.Target)
	if err != nil {
		return nil, err
	}
	if ref.Select == "" {
		return bp, nil
	}
	sel, err := schemad.ParseSelect(ref.Select)
	if err != nil {
		return nil, err
	}
	if sel.Kind != "graph" {
		return nil, fmt.Errorf("graph ref select must be graph, got %s", sel.Kind)
	}
	if bp.Name == sel.Name {
		return bp, nil
	}
	return nil, fmt.Errorf("graph %s not found in %s", sel.Name, ref.Target)
}

// LoadNode resolves a node ref into a node spec.
func (l *Loader) LoadNode(ctx context.Context, ref schemad.Ref) (*schemad.NodeSpec, error) {
	if l == nil || l.Documents == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	if ref.Kind != schemad.RefKindBlueprintDocument {
		return nil, fmt.Errorf("unsupported node ref kind: %s", ref.Kind)
	}
	if ref.Select == "" {
		return nil, fmt.Errorf("node ref select is required")
	}
	bp, err := l.Documents.LoadGraphBlueprint(ctx, ref.Target)
	if err != nil {
		return nil, err
	}
	sel, err := schemad.ParseSelect(ref.Select)
	if err != nil {
		return nil, err
	}
	if sel.Kind != "node" {
		return nil, fmt.Errorf("node ref select must be node, got %s", sel.Kind)
	}
	node, ok := bp.FindNode(sel.Name)
	if !ok {
		return nil, fmt.Errorf("node %s not found in %s", sel.Name, ref.Target)
	}
	return node, nil
}

// LoadComponent resolves a component ref into a component spec.
func (l *Loader) LoadComponent(ctx context.Context, ref schemad.Ref) (*schemad.ComponentSpec, error) {
	if l == nil || l.Documents == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	if ref.Kind != schemad.RefKindComponentDocument {
		return nil, fmt.Errorf("unsupported component ref kind: %s", ref.Kind)
	}
	comp, err := l.Documents.LoadComponentSpec(ctx, ref.Target)
	if err != nil {
		return nil, err
	}
	if ref.Select == "" {
		return comp, nil
	}
	sel, err := schemad.ParseSelect(ref.Select)
	if err != nil {
		return nil, err
	}
	if sel.Kind != "component" {
		return nil, fmt.Errorf("component ref select must be component, got %s", sel.Kind)
	}
	if comp.Name == sel.Name {
		return comp, nil
	}
	return nil, fmt.Errorf("component %s not found in %s", sel.Name, ref.Target)
}
