package builder

import "context"

type loopResolver struct {
	base       NodeResolver
	loopConfig map[NodeID]int
}

func newLoopResolver(plan ExecutionPlan, base NodeResolver) NodeResolver {
	config := loopConfigByNode(plan)
	if len(config) == 0 {
		return base
	}
	return &loopResolver{base: base, loopConfig: config}
}

func (r *loopResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	resolved, err := r.base.ResolveNode(ctx, node, binding, policy)
	if err != nil {
		return nil, err
	}
	iterations := r.loopConfig[node.ID]
	if iterations <= 1 {
		return resolved, nil
	}
	switch typed := resolved.(type) {
	case func(context.Context, map[string]any) (map[string]any, error):
		return func(ctx context.Context, input map[string]any) (map[string]any, error) {
			state := cloneMap(input)
			if state == nil {
				state = map[string]any{}
			}
			var out map[string]any
			var lastErr error
			for i := 0; i < iterations; i++ {
				out, lastErr = typed(ctx, cloneMap(state))
				if lastErr != nil {
					return out, lastErr
				}
				state = mergeInputMaps(state, out)
				state["_loop_iteration"] = i + 1
			}
			return state, nil
		}, nil
	default:
		return resolved, nil
	}
}

func loopConfigByNode(plan ExecutionPlan) map[NodeID]int {
	config := make(map[NodeID]int)
	for _, block := range plan.Structural.Blocks {
		if block.Kind != BlockKindLoop || block.Metadata == nil {
			continue
		}
		iterations, ok := block.Metadata["max_iterations"].(int)
		if !ok || iterations <= 1 {
			continue
		}
		for _, nodeID := range block.Nodes {
			config[nodeID] = iterations
		}
	}
	return config
}
