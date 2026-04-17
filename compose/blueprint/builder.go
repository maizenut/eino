package blueprint

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
)

// Builder assembles declarative graph blueprints into compose.AnyGraph values.
type Builder struct {
	Loader              *Loader
	ComponentFactory    ComponentFactory
	InterpreterResolver InterpreterResolver
}

// BuildAnyGraph builds a blueprint into a compose graph, chain, or workflow.
func (b *Builder) BuildAnyGraph(ctx context.Context, blueprint *schemad.GraphBlueprint) (compose.AnyGraph, error) {
	if blueprint == nil {
		return nil, fmt.Errorf("graph blueprint is required")
	}

	switch blueprint.Type {
	case "", schemad.GraphTypeGraph, schemad.GraphTypeChain:
		return b.buildGraph(ctx, blueprint)
	case schemad.GraphTypeWorkflow:
		return b.buildWorkflow(ctx, blueprint)
	default:
		return nil, fmt.Errorf("unsupported graph blueprint type: %s", blueprint.Type)
	}
}

func (b *Builder) buildGraph(ctx context.Context, blueprint *schemad.GraphBlueprint) (compose.AnyGraph, error) {
	g := compose.NewGraph[map[string]any, map[string]any]()
	for i := range blueprint.Nodes {
		if err := b.addNode(ctx, g, &blueprint.Nodes[i]); err != nil {
			return nil, err
		}
	}
	for i := range blueprint.Edges {
		if err := addGraphEdge(g, blueprint.Edges[i]); err != nil {
			return nil, err
		}
	}
	for i := range blueprint.Branches {
		if err := b.addBranch(ctx, g, &blueprint.Branches[i]); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (b *Builder) buildWorkflow(ctx context.Context, blueprint *schemad.GraphBlueprint) (compose.AnyGraph, error) {
	wf := compose.NewWorkflow[map[string]any, map[string]any]()
	for i := range blueprint.Nodes {
		if err := b.addWorkflowNode(ctx, wf, &blueprint.Nodes[i]); err != nil {
			return nil, err
		}
	}
	for i := range blueprint.WorkflowNodes {
		if err := b.applyWorkflowNodeBlueprint(wf, &blueprint.WorkflowNodes[i]); err != nil {
			return nil, err
		}
	}
	for i := range blueprint.Edges {
		if err := applyWorkflowEdge(wf, blueprint.Edges[i]); err != nil {
			return nil, err
		}
	}
	for i := range blueprint.Branches {
		if err := b.addWorkflowBranch(ctx, wf, &blueprint.Branches[i]); err != nil {
			return nil, err
		}
	}
	return wf, nil
}

func (b *Builder) addNode(ctx context.Context, g *compose.Graph[map[string]any, map[string]any], node *schemad.NodeSpec) error {
	opts := graphNodeOpts(node)

	switch node.Kind {
	case schemad.NodeKindComponent:
		instance, err := schemad.BuildComponent(ctx, node.Component, b.ComponentFactory, componentResolverAdapter{resolver: b.InterpreterResolver})
		if err != nil {
			return fmt.Errorf("build component node %s: %w", node.Key, err)
		}
		return addTypedComponentNode(g, node.Key, node.Component, instance, opts...)
	case schemad.NodeKindLambda:
		lambdaNode, err := b.buildLambda(ctx, node)
		if err != nil {
			return err
		}
		return g.AddLambdaNode(node.Key, lambdaNode, opts...)
	case schemad.NodeKindGraph:
		subgraph, err := b.buildGraphRef(ctx, node)
		if err != nil {
			return err
		}
		return g.AddGraphNode(node.Key, subgraph, opts...)
	case schemad.NodeKindPassthrough:
		return g.AddPassthroughNode(node.Key, opts...)
	default:
		return fmt.Errorf("unsupported node kind %s", node.Kind)
	}
}

func (b *Builder) addWorkflowNode(ctx context.Context, wf *compose.Workflow[map[string]any, map[string]any], node *schemad.NodeSpec) error {
	opts := graphNodeOpts(node)

	switch node.Kind {
	case schemad.NodeKindComponent:
		instance, err := schemad.BuildComponent(ctx, node.Component, b.ComponentFactory, componentResolverAdapter{resolver: b.InterpreterResolver})
		if err != nil {
			return fmt.Errorf("build workflow component node %s: %w", node.Key, err)
		}
		return addTypedWorkflowNode(wf, node.Key, node.Component, instance, opts...)
	case schemad.NodeKindLambda:
		lambdaNode, err := b.buildLambda(ctx, node)
		if err != nil {
			return err
		}
		wf.AddLambdaNode(node.Key, lambdaNode, opts...)
		return nil
	case schemad.NodeKindGraph:
		subgraph, err := b.buildGraphRef(ctx, node)
		if err != nil {
			return err
		}
		wf.AddGraphNode(node.Key, subgraph, opts...)
		return nil
	case schemad.NodeKindPassthrough:
		wf.AddPassthroughNode(node.Key, opts...)
		return nil
	default:
		return fmt.Errorf("unsupported node kind %s", node.Kind)
	}
}

func (b *Builder) buildLambda(ctx context.Context, node *schemad.NodeSpec) (*compose.Lambda, error) {
	if node.Lambda == nil {
		return nil, fmt.Errorf("lambda spec is required for node %s", node.Key)
	}
	if b.InterpreterResolver == nil {
		return nil, fmt.Errorf("interpreter resolver is required for lambda node %s", node.Key)
	}
	callable, err := b.InterpreterResolver.ResolveFunction(ctx, node.Lambda.Callable)
	if err != nil {
		return nil, fmt.Errorf("resolve lambda callable for %s: %w", node.Key, err)
	}
	invokable, ok := callable.(func(context.Context, map[string]any) (map[string]any, error))
	if !ok {
		return nil, fmt.Errorf("lambda callable for %s must be func(context.Context, map[string]any) (map[string]any, error), got %T", node.Key, callable)
	}
	return compose.InvokableLambda(invokable, compose.WithLambdaType(node.Lambda.Impl)), nil
}

func (b *Builder) buildGraphRef(ctx context.Context, node *schemad.NodeSpec) (compose.AnyGraph, error) {
	if node.GraphRef == nil {
		return nil, fmt.Errorf("graph ref is required for node %s", node.Key)
	}
	switch node.GraphRef.Kind {
	case schemad.RefKindBlueprintDocument:
		if b.Loader == nil {
			return nil, fmt.Errorf("document loader is required for graph node %s", node.Key)
		}
		bp, err := b.Loader.LoadGraph(ctx, *node.GraphRef)
		if err != nil {
			return nil, err
		}
		return b.BuildAnyGraph(ctx, bp)
	case schemad.RefKindInterpreterGraph:
		if b.InterpreterResolver == nil {
			return nil, fmt.Errorf("interpreter resolver is required for graph node %s", node.Key)
		}
		return b.InterpreterResolver.ResolveGraph(ctx, *node.GraphRef)
	default:
		return nil, fmt.Errorf("unsupported graph ref kind %s", node.GraphRef.Kind)
	}
}

func (b *Builder) addBranch(ctx context.Context, g *compose.Graph[map[string]any, map[string]any], branch *schemad.GraphBranchBlueprint) error {
	if b.InterpreterResolver == nil {
		return fmt.Errorf("interpreter resolver is required for branch on %s", branch.From)
	}
	callable, err := b.InterpreterResolver.ResolveFunction(ctx, branch.Condition)
	if err != nil {
		return err
	}
	cond, ok := callable.(func(context.Context, map[string]any) (string, error))
	if !ok {
		return fmt.Errorf("branch condition for %s must be func(context.Context, map[string]any) (string, error), got %T", branch.From, callable)
	}
	endNodes := make(map[string]bool, len(branch.EndNodes))
	for _, end := range branch.EndNodes {
		endNodes[end] = true
	}
	return g.AddBranch(branch.From, compose.NewGraphBranch(cond, endNodes))
}

func (b *Builder) addWorkflowBranch(ctx context.Context, wf *compose.Workflow[map[string]any, map[string]any], branch *schemad.GraphBranchBlueprint) error {
	if b.InterpreterResolver == nil {
		return fmt.Errorf("interpreter resolver is required for branch on %s", branch.From)
	}
	callable, err := b.InterpreterResolver.ResolveFunction(ctx, branch.Condition)
	if err != nil {
		return err
	}
	cond, ok := callable.(func(context.Context, map[string]any) (string, error))
	if !ok {
		return fmt.Errorf("branch condition for %s must be func(context.Context, map[string]any) (string, error), got %T", branch.From, callable)
	}
	endNodes := make(map[string]bool, len(branch.EndNodes))
	for _, end := range branch.EndNodes {
		endNodes[end] = true
	}
	wf.AddBranch(branch.From, compose.NewGraphBranch(cond, endNodes))
	return nil
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
