package builder

import "context"

type replayResolver struct {
	base        NodeResolver
	replayNodes map[NodeID]struct{}
}

func newReplayResolver(plan ExecutionPlan, base NodeResolver) NodeResolver {
	nodes := replayNodesByPlan(plan)
	if len(nodes) == 0 {
		return base
	}
	return &replayResolver{base: base, replayNodes: nodes}
}

func (r *replayResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	resolved, err := r.base.ResolveNode(ctx, node, binding, policy)
	if err != nil {
		return nil, err
	}
	if _, ok := r.replayNodes[node.ID]; !ok {
		switch typed := resolved.(type) {
		case func(context.Context, map[string]any) (map[string]any, error):
			return func(ctx context.Context, input map[string]any) (map[string]any, error) {
				meta := nestedMap(input, "_builder")
				if len(meta) == 0 {
					return typed(ctx, cloneMap(input))
				}
				replayScope, _ := meta["replay_scope"].(string)
				if replayScope == "" {
					return typed(ctx, cloneMap(input))
				}
				return cloneMap(input), nil
			}, nil
		default:
			return resolved, nil
		}
	}
	switch typed := resolved.(type) {
	case func(context.Context, map[string]any) (map[string]any, error):
		return func(ctx context.Context, input map[string]any) (map[string]any, error) {
			meta := nestedMap(input, "_builder")
			if len(meta) == 0 {
				return typed(ctx, cloneMap(input))
			}
			replay, _ := meta["replay_nodes"].([]string)
			if !containsString(replay, string(node.ID)) {
				return cloneMap(input), nil
			}
			return typed(ctx, cloneMap(input))
		}, nil
	default:
		return resolved, nil
	}
}

func replayNodesByPlan(plan ExecutionPlan) map[NodeID]struct{} {
	nodes := make(map[NodeID]struct{})
	for _, policy := range plan.State.ReplayPolicies {
		for _, nodeID := range policy.ReplayNodeIDs {
			nodes[nodeID] = struct{}{}
		}
	}
	return nodes
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
