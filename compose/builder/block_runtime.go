package builder

import "context"

func staticInputsFromPlan(plan ExecutionPlan) map[NodeID]map[string]any {
	staticInputs := make(map[NodeID]map[string]any)
	for _, block := range plan.Structural.Blocks {
		if len(block.Nodes) == 0 || block.Metadata == nil {
			continue
		}
		value, ok := block.Metadata["static_value"]
		if !ok {
			continue
		}
		payload, ok := value.(map[string]any)
		if !ok || len(payload) == 0 {
			continue
		}
		for _, nodeID := range block.EntryNodes {
			merged := cloneMap(staticInputs[nodeID])
			if merged == nil {
				merged = map[string]any{}
			}
			for k, v := range payload {
				merged[k] = v
			}
			staticInputs[nodeID] = merged
		}
		if len(block.EntryNodes) == 0 {
			nodeID := block.Nodes[0]
			merged := cloneMap(staticInputs[nodeID])
			if merged == nil {
				merged = map[string]any{}
			}
			for k, v := range payload {
				merged[k] = v
			}
			staticInputs[nodeID] = merged
		}
	}
	return staticInputs
}

func wrapWithStaticInput(nodeID NodeID, resolved any, staticInput map[string]any) any {
	if len(staticInput) == 0 {
		return resolved
	}
	switch typed := resolved.(type) {
	case func(context.Context, map[string]any) (map[string]any, error):
		return func(ctx context.Context, input map[string]any) (map[string]any, error) {
			return typed(ctx, mergeInputMaps(staticInput, input))
		}
	default:
		_ = nodeID
		return resolved
	}
}

func mergeInputMaps(base map[string]any, input map[string]any) map[string]any {
	merged := cloneMap(base)
	if merged == nil {
		merged = map[string]any{}
	}
	for k, v := range input {
		merged[k] = v
	}
	return merged
}
