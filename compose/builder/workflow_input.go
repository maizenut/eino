package builder

import "fmt"

type ResolvedWorkflowInputMode string

const (
	ResolvedWorkflowInputModeData       ResolvedWorkflowInputMode = "data"
	ResolvedWorkflowInputModeControl    ResolvedWorkflowInputMode = "control"
	ResolvedWorkflowInputModeDependency ResolvedWorkflowInputMode = "dependency"
)

func workflowInputMode(meta map[string]any) ResolvedWorkflowInputMode {
	if meta == nil {
		return ResolvedWorkflowInputModeData
	}
	mode, _ := meta["workflow_input_mode"].(string)
	switch ResolvedWorkflowInputMode(mode) {
	case ResolvedWorkflowInputModeControl, ResolvedWorkflowInputModeDependency:
		return ResolvedWorkflowInputMode(mode)
	default:
		return ResolvedWorkflowInputModeData
	}
}

func validateWorkflowInputEdge(edge PlannedEdge) error {
	mode := workflowInputMode(edge.Metadata)
	switch mode {
	case ResolvedWorkflowInputModeDependency:
		if edge.Kind != EdgeKindControl {
			return fmt.Errorf("workflow dependency-only edge %q must use control kind, got %q", edge.ID, edge.Kind)
		}
	case ResolvedWorkflowInputModeControl:
		if edge.Kind != EdgeKindProjection && edge.Kind != EdgeKindData && edge.Kind != EdgeKindControl {
			return fmt.Errorf("workflow no-direct-dependency edge %q uses unsupported kind %q", edge.ID, edge.Kind)
		}
	}
	return nil
}
