package builder

import "context"

func wrapWithBoundaryProjection(nodeID NodeID, resolved any, projection *BoundaryProjectionSpec) any {
	if projection == nil || len(projection.Mapping) == 0 {
		return resolved
	}
	switch typed := resolved.(type) {
	case func(context.Context, map[string]any) (map[string]any, error):
		return func(ctx context.Context, input map[string]any) (map[string]any, error) {
			return typed(ctx, projectBoundaryInput(input, projection))
		}
	default:
		_ = nodeID
		return resolved
	}
}

func projectBoundaryInput(input map[string]any, projection *BoundaryProjectionSpec) map[string]any {
	if projection == nil || len(projection.Mapping) == 0 {
		return cloneMap(input)
	}
	out := cloneMap(input)
	if out == nil {
		out = map[string]any{}
	}
	projected := map[string]any{}
	for from, to := range projection.Mapping {
		projected[to] = lookupPath(out, from)
	}
	for key, value := range projected {
		out[key] = value
	}
	return out
}
