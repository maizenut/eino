package builder

import "context"

type parallelResolver struct {
	base          NodeResolver
	parallelNodes map[NodeID]int
}

func newParallelResolver(plan ExecutionPlan, base NodeResolver) NodeResolver {
	config := parallelConfigByNode(plan)
	if len(config) == 0 {
		return base
	}
	return &parallelResolver{base: base, parallelNodes: config}
}

func (r *parallelResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	resolved, err := r.base.ResolveNode(ctx, node, binding, policy)
	if err != nil {
		return nil, err
	}
	branches := r.parallelNodes[node.ID]
	if branches <= 1 {
		return resolved, nil
	}
	switch typed := resolved.(type) {
	case func(context.Context, map[string]any) (map[string]any, error):
		return func(ctx context.Context, input map[string]any) (map[string]any, error) {
			baseInput := cloneMap(input)
			if baseInput == nil {
				baseInput = map[string]any{}
			}
			merged := cloneMap(baseInput)
			for i := 0; i < branches; i++ {
				branchInput := cloneMap(baseInput)
				branchInput["_parallel_branch"] = i + 1
				out, err := typed(ctx, branchInput)
				if err != nil {
					return out, err
				}
				merged = mergeInputMaps(merged, out)
			}
			merged["_parallel_branches"] = branches
			return merged, nil
		}, nil
	default:
		return resolved, nil
	}
}

func parallelConfigByNode(plan ExecutionPlan) map[NodeID]int {
	config := make(map[NodeID]int)
	for _, block := range plan.Structural.Blocks {
		if block.Kind != BlockKindParallel {
			continue
		}
		branches := len(block.Nodes)
		if branches <= 1 {
			continue
		}
		for _, nodeID := range block.EntryNodes {
			config[nodeID] = branches
		}
		if len(block.EntryNodes) == 0 {
			config[block.Nodes[0]] = branches
		}
	}
	return config
}
