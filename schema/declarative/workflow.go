package declarative

// GraphEdgeMode describes edge data/control behavior.
type GraphEdgeMode struct {
	NoControl bool `json:"no_control,omitempty"`
	NoData    bool `json:"no_data,omitempty"`
}

// FieldMappingSpec declares one field mapping.
type FieldMappingSpec struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// WorkflowInputSpec declares an incoming workflow dependency.
type WorkflowInputSpec struct {
	From               string             `json:"from"`
	Mappings           []FieldMappingSpec `json:"mappings,omitempty"`
	NoDirectDependency bool               `json:"no_direct_dependency,omitempty"`
	DependencyOnly     bool               `json:"dependency_only,omitempty"`
}

// WorkflowNodeSpec declares workflow-only node wiring.
type WorkflowNodeSpec struct {
	Key         string              `json:"key"`
	Inputs      []WorkflowInputSpec `json:"inputs,omitempty"`
	StaticValue map[string]any      `json:"static_value,omitempty"`
}

type GraphEdgeBlueprint = EdgeSpec
type WorkflowInputBlueprint = WorkflowInputSpec
type WorkflowNodeBlueprint = WorkflowNodeSpec
