package declarative

import "github.com/cloudwego/eino/components"

const (
	RefKindBlueprintDocument    = "blueprint_document"
	RefKindComponentDocument    = "component_document"
	RefKindSkillDocument        = "skill_document"
	RefKindInterpreterFunction  = "interpreter_function"
	RefKindInterpreterComponent = "interpreter_component"
	RefKindInterpreterGraph     = "interpreter_graph"
	NodeKindComponent           = "component"
	NodeKindLambda              = "lambda"
	NodeKindGraph               = "graph"
	NodeKindPassthrough         = "passthrough"
	GraphTypeGraph              = "graph"
	GraphTypeChain              = "chain"
	GraphTypeWorkflow           = "workflow"
	SelectPrefixNode            = "node:"
	SelectPrefixGraph           = "graph:"
	SelectPrefixComponent       = "component:"
)

// Ref describes a cross-boundary declarative reference.
type Ref struct {
	Kind   string         `json:"kind"`
	Target string         `json:"target"`
	Select string         `json:"select,omitempty"`
	Args   map[string]any `json:"args,omitempty"`
}

// ComponentSpec declares a component instance that can be built later.
type ComponentSpec struct {
	Kind   string         `json:"kind"`
	Impl   string         `json:"impl"`
	Name   string         `json:"name,omitempty"`
	Config map[string]any `json:"config,omitempty"`
	Refs   map[string]Ref `json:"refs,omitempty"`
	Extra  map[string]any `json:"extra,omitempty"`
}

// LambdaSpec declares a callable node that maps to compose.Lambda.
type LambdaSpec struct {
	Impl       string         `json:"impl"`
	Callable   Ref            `json:"callable"`
	InputType  string         `json:"input_type,omitempty"`
	OutputType string         `json:"output_type,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
}

// NodeSpec declares a graph node.
type NodeSpec struct {
	Key       string         `json:"key"`
	Name      string         `json:"name,omitempty"`
	Kind      string         `json:"kind"`
	Component *ComponentSpec `json:"component,omitempty"`
	Lambda    *LambdaSpec    `json:"lambda,omitempty"`
	GraphRef  *Ref           `json:"graph_ref,omitempty"`
	InputKey  string         `json:"input_key,omitempty"`
	OutputKey string         `json:"output_key,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
}

// GraphEdgeBlueprint declares a simple directed edge.
type GraphEdgeBlueprint struct {
	From     string             `json:"from"`
	To       string             `json:"to"`
	Control  *bool              `json:"control,omitempty"`
	Data     *bool              `json:"data,omitempty"`
	Mode     *GraphEdgeMode     `json:"mode,omitempty"`
	Mappings []FieldMappingSpec `json:"mappings,omitempty"`
}

// GraphBranchBlueprint declares a branch route from one node to many end nodes.
type GraphBranchBlueprint struct {
	From      string   `json:"from"`
	Condition Ref      `json:"condition"`
	EndNodes  []string `json:"end_nodes,omitempty"`
}

// GraphBlueprint declares a graph, chain, or workflow.
type GraphBlueprint struct {
	Name          string                  `json:"name,omitempty"`
	Type          string                  `json:"type,omitempty"`
	Nodes         []NodeSpec              `json:"nodes,omitempty"`
	Edges         []GraphEdgeBlueprint    `json:"edges,omitempty"`
	Branches      []GraphBranchBlueprint  `json:"branches,omitempty"`
	WorkflowNodes []WorkflowNodeBlueprint `json:"workflow_nodes,omitempty"`
	Options       map[string]any          `json:"options,omitempty"`
}

// ComponentKind normalizes a spec kind into a components.Component value.
func ComponentKind(kind string) components.Component {
	switch kind {
	case string(components.ComponentOfPrompt):
		return components.ComponentOfPrompt
	case string(components.ComponentOfAgenticPrompt):
		return components.ComponentOfAgenticPrompt
	case string(components.ComponentOfChatModel):
		return components.ComponentOfChatModel
	case string(components.ComponentOfAgenticModel):
		return components.ComponentOfAgenticModel
	case string(components.ComponentOfEmbedding):
		return components.ComponentOfEmbedding
	case string(components.ComponentOfIndexer):
		return components.ComponentOfIndexer
	case string(components.ComponentOfRetriever):
		return components.ComponentOfRetriever
	case string(components.ComponentOfLoader):
		return components.ComponentOfLoader
	case string(components.ComponentOfTransformer):
		return components.ComponentOfTransformer
	case string(components.ComponentOfTool):
		return components.ComponentOfTool
	default:
		return components.Component(kind)
	}
}
