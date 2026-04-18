package builder

import "fmt"

func checkpointScopesByID(plan ExecutionPlan) map[CheckpointScopeID]PlannedCheckpointScope {
	scopes := make(map[CheckpointScopeID]PlannedCheckpointScope, len(plan.State.CheckpointScopes))
	for _, scope := range plan.State.CheckpointScopes {
		scopes[scope.ID] = scope
	}
	return scopes
}

func replayPoliciesByScope(plan ExecutionPlan) map[CheckpointScopeID][]NodeID {
	policies := make(map[CheckpointScopeID][]NodeID)
	for _, policy := range plan.State.ReplayPolicies {
		policies[policy.ScopeID] = append([]NodeID(nil), policy.ReplayNodeIDs...)
	}
	return policies
}

func validateRecoveryScopes(plan ExecutionPlan) error {
	scopes := checkpointScopesByID(plan)
	for _, block := range plan.Structural.Blocks {
		if block.RecoveryScopeRef == "" {
			continue
		}
		if _, ok := scopes[block.RecoveryScopeRef]; !ok {
			return fmt.Errorf("block %q references unknown recovery scope %q", block.ID, block.RecoveryScopeRef)
		}
	}
	for _, boundary := range plan.Structural.Boundaries {
		if boundary.RecoveryScopeRef == "" {
			continue
		}
		if _, ok := scopes[boundary.RecoveryScopeRef]; !ok {
			return fmt.Errorf("boundary %q references unknown recovery scope %q", boundary.ID, boundary.RecoveryScopeRef)
		}
	}
	return nil
}
