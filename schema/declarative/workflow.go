package declarative

// GraphEdgeMode describes edge data/control behavior.
type GraphEdgeMode struct {
	NoControl bool `json:"no_control,omitempty"`
	NoData    bool `json:"no_data,omitempty"`
}

// FieldMappingSpec declares one workflow field mapping.
type FieldMappingSpec struct {
	From []string `json:"from,omitempty"`
	To   []string `json:"to,omitempty"`
}

// WorkflowInputBlueprint declares an incoming workflow dependency.
type WorkflowInputBlueprint struct {
	From               string             `json:"from"`
	Mappings           []FieldMappingSpec `json:"mappings,omitempty"`
	NoDirectDependency bool               `json:"no_direct_dependency,omitempty"`
	DependencyOnly     bool               `json:"dependency_only,omitempty"`
}

// WorkflowNodeBlueprint declares workflow-only node wiring.
type WorkflowNodeBlueprint struct {
	Key         string                   `json:"key"`
	Inputs      []WorkflowInputBlueprint `json:"inputs,omitempty"`
	StaticValue map[string]any           `json:"static_value,omitempty"`
}
