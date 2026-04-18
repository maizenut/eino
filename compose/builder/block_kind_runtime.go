package builder

import "fmt"

func blockKindsByNode(plan ExecutionPlan) map[NodeID]BlockKind {
	kinds := make(map[NodeID]BlockKind)
	for _, block := range plan.Structural.Blocks {
		for _, nodeID := range block.Nodes {
			kinds[nodeID] = block.Kind
		}
	}
	return kinds
}

func validateBlockKinds(plan ExecutionPlan) error {
	for _, block := range plan.Structural.Blocks {
		switch block.Kind {
		case BlockKindSequence, BlockKindParallel, BlockKindLoop, BlockKindRetry, BlockKindFallback, BlockKindBranch:
		default:
			return fmt.Errorf("unsupported block kind %q", block.Kind)
		}
		if (block.Kind == BlockKindParallel || block.Kind == BlockKindBranch) && len(block.EntryNodes) == 0 {
			return fmt.Errorf("block %q of kind %q requires entry nodes", block.ID, block.Kind)
		}
		if (block.Kind == BlockKindRetry || block.Kind == BlockKindLoop) && len(block.ExitNodes) == 0 {
			return fmt.Errorf("block %q of kind %q requires exit nodes", block.ID, block.Kind)
		}
	}
	return nil
}

func validateBlockKindEdge(edge PlannedEdge, membership map[NodeID]BlockID, kinds map[NodeID]BlockKind) error {
	blockID, ok := membership[edge.From]
	if !ok {
		return nil
	}
	if membership[edge.To] != blockID {
		return nil
	}
	switch kinds[edge.From] {
	case BlockKindParallel:
		if edge.Kind == EdgeKindConditional {
			return fmt.Errorf("parallel block edge %q cannot be conditional", edge.ID)
		}
	case BlockKindLoop:
		if edge.From == edge.To && edge.Kind != EdgeKindConditional && edge.Kind != EdgeKindControl {
			return fmt.Errorf("loop block self-edge %q must be control or conditional", edge.ID)
		}
	case BlockKindRetry:
		if edge.Kind == EdgeKindError {
			return nil
		}
	case BlockKindFallback:
		if edge.Kind == EdgeKindError || edge.Kind == EdgeKindConditional {
			return nil
		}
	}
	return nil
}
