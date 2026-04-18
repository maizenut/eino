package builder

import "fmt"

func boundaryProjectionByEntry(plan ExecutionPlan) map[NodeID]*BoundaryProjectionSpec {
	projections := make(map[NodeID]*BoundaryProjectionSpec)
	for _, boundary := range plan.Structural.Boundaries {
		if boundary.EntryNode == "" || boundary.Projection == nil {
			continue
		}
		projections[boundary.EntryNode] = boundary.Projection
	}
	return projections
}

func boundaryVisibilityByEntry(plan ExecutionPlan) map[NodeID]*BoundaryVisibilitySpec {
	visibility := make(map[NodeID]*BoundaryVisibilitySpec)
	for _, boundary := range plan.Structural.Boundaries {
		if boundary.EntryNode == "" || boundary.Visibility == nil {
			continue
		}
		visibility[boundary.EntryNode] = boundary.Visibility
	}
	return visibility
}

func validateBoundaryBinding(node PlannedNode, visibility *BoundaryVisibilitySpec) error {
	if visibility == nil || node.Binding == "" || len(visibility.AllowedBindings) == 0 {
		return nil
	}
	for _, allowed := range visibility.AllowedBindings {
		if allowed == node.Binding {
			return nil
		}
	}
	return fmt.Errorf("node %q binding %q is not visible within its boundary", node.ID, node.Binding)
}
