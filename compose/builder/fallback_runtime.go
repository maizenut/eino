package builder

import "context"

type fallbackResolver struct {
	base          NodeResolver
	fallbackNodes map[NodeID]struct{}
}

func newFallbackResolver(plan ExecutionPlan, base NodeResolver) NodeResolver {
	nodes := fallbackNodesByPlan(plan)
	if len(nodes) == 0 {
		return base
	}
	return &fallbackResolver{base: base, fallbackNodes: nodes}
}

func (r *fallbackResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	resolved, err := r.base.ResolveNode(ctx, node, binding, policy)
	if err != nil {
		return nil, err
	}
	if _, ok := r.fallbackNodes[node.ID]; !ok {
		return resolved, nil
	}
	switch typed := resolved.(type) {
	case func(context.Context, map[string]any) (map[string]any, error):
		return func(ctx context.Context, input map[string]any) (map[string]any, error) {
			out, err := typed(ctx, cloneMap(input))
			if err != nil {
				fallback := cloneMap(input)
				if fallback == nil {
					fallback = map[string]any{}
				}
				fallback["_fallback_error"] = err.Error()
				return fallback, nil
			}
			return out, nil
		}, nil
	default:
		return resolved, nil
	}
}

func fallbackNodesByPlan(plan ExecutionPlan) map[NodeID]struct{} {
	nodes := make(map[NodeID]struct{})
	for _, block := range plan.Structural.Blocks {
		if block.Kind != BlockKindFallback {
			continue
		}
		for _, nodeID := range block.Nodes {
			nodes[nodeID] = struct{}{}
		}
	}
	return nodes
}
