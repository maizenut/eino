package builder

import "context"

type retryResolver struct {
	base        NodeResolver
	retryConfig map[NodeID]int
}

func newRetryResolver(plan ExecutionPlan, base NodeResolver) NodeResolver {
	config := retryConfigByNode(plan)
	if len(config) == 0 {
		return base
	}
	return &retryResolver{base: base, retryConfig: config}
}

func (r *retryResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	resolved, err := r.base.ResolveNode(ctx, node, binding, policy)
	if err != nil {
		return nil, err
	}
	attempts := r.retryConfig[node.ID]
	if attempts <= 1 {
		return resolved, nil
	}
	switch typed := resolved.(type) {
	case func(context.Context, map[string]any) (map[string]any, error):
		return func(ctx context.Context, input map[string]any) (map[string]any, error) {
			var out map[string]any
			var lastErr error
			for i := 0; i < attempts; i++ {
				out, lastErr = typed(ctx, cloneMap(input))
				if lastErr == nil {
					return out, nil
				}
			}
			return out, lastErr
		}, nil
	default:
		return resolved, nil
	}
}

func retryConfigByNode(plan ExecutionPlan) map[NodeID]int {
	config := make(map[NodeID]int)
	for _, block := range plan.Structural.Blocks {
		if block.Kind != BlockKindRetry || block.Metadata == nil {
			continue
		}
		attempts, ok := block.Metadata["max_attempts"].(int)
		if !ok || attempts <= 1 {
			continue
		}
		for _, nodeID := range block.Nodes {
			config[nodeID] = attempts
		}
	}
	return config
}
