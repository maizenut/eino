package declarative

import (
	"strings"

	"github.com/cloudwego/eino/components"
)

const (
	RefKindGraphDocument        = "graph_document"
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
	EdgeKindData                = "data"
	EdgeKindControl             = "control"
	EdgeKindRoute               = "route"
	EdgeKindNoData              = "no_data"
	EdgeKindNoControl           = "no_control"
	SelectPrefixNode            = "node:"
	SelectPrefixGraph           = "graph:"
	SelectPrefixComponent       = "component:"
)

// Ref describes a declarative reference to a document or runtime symbol.
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

// ComponentRef declares a node-level reference to an external component spec.
type ComponentRef struct {
	Ref Ref `json:"ref"`
}

// LambdaSpec declares a callable node that maps to compose.Lambda.
type LambdaSpec struct {
	Impl       string         `json:"impl"`
	Callable   Ref            `json:"callable"`
	InputType  string         `json:"input_type,omitempty"`
	OutputType string         `json:"output_type,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
}

type SubGraphMode string

const (
	SubGraphModeInvoke    SubGraphMode = "invoke"
	SubGraphModeDelegate  SubGraphMode = "delegate"
	SubGraphModeSupervise SubGraphMode = "supervise"
	SubGraphModeHandoff   SubGraphMode = "handoff"
)

type SubGraphPolicy struct {
	Mode             SubGraphMode   `json:"mode,omitempty"`
	ErrorPolicy      string         `json:"error_policy,omitempty"`
	CheckpointPolicy string         `json:"checkpoint_policy,omitempty"`
	ResumePolicy     string         `json:"resume_policy,omitempty"`
	Timeout          string         `json:"timeout,omitempty"`
	FallbackRef      *Ref           `json:"fallback_ref,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type SubGraphDelegateSpec struct {
	SpecRef        *Ref           `json:"spec_ref,omitempty"`
	CompileOptions map[string]any `json:"compile_options,omitempty"`
	StoreRef       *Ref           `json:"store_ref,omitempty"`
	SinkRef        *Ref           `json:"sink_ref,omitempty"`
}

type SubGraphNodeSpec struct {
	Enabled  bool                  `json:"enabled,omitempty"`
	Policy   *SubGraphPolicy       `json:"policy,omitempty"`
	Delegate *SubGraphDelegateSpec `json:"delegate,omitempty"`
}

// NodeSpec declares a graph node.
type NodeSpec struct {
	Key       string            `json:"key"`
	Name      string            `json:"name,omitempty"`
	Kind      string            `json:"kind"`
	Component *ComponentRef     `json:"component,omitempty"`
	Lambda    *LambdaSpec       `json:"lambda,omitempty"`
	GraphRef  *Ref              `json:"graph_ref,omitempty"`
	SubGraph  *SubGraphNodeSpec `json:"subgraph,omitempty"`
	InputKey  string            `json:"input_key,omitempty"`
	OutputKey string            `json:"output_key,omitempty"`
	Options   map[string]any    `json:"options,omitempty"`
}

// EdgeSpec declares a simple directed edge.
type EdgeSpec struct {
	From      string             `json:"from"`
	To        string             `json:"to"`
	Kind      string             `json:"kind,omitempty"`
	Mappings  []FieldMappingSpec `json:"mappings,omitempty"`
	Condition *Ref               `json:"condition,omitempty"`
	Match     string             `json:"match,omitempty"`
	Default   bool               `json:"default,omitempty"`
	Options   map[string]any     `json:"options,omitempty"`
}

// GraphSpec declares a graph, chain, or workflow.
type GraphSpec struct {
	Name          string             `json:"name,omitempty"`
	Type          string             `json:"type,omitempty"`
	Nodes         []NodeSpec         `json:"nodes,omitempty"`
	Edges         []EdgeSpec         `json:"edges,omitempty"`
	WorkflowNodes []WorkflowNodeSpec `json:"workflow_nodes,omitempty"`
	Options       map[string]any     `json:"options,omitempty"`
}

// ComponentKind normalizes a spec kind into a components.Component value.
func ComponentKind(kind string) components.Component {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	normalized = strings.ReplaceAll(normalized, "_", "")
	switch normalized {
	case strings.ToLower(string(components.ComponentOfPrompt)), "prompt":
		return components.ComponentOfPrompt
	case strings.ToLower(string(components.ComponentOfAgenticPrompt)), "agenticprompt":
		return components.ComponentOfAgenticPrompt
	case strings.ToLower(string(components.ComponentOfChatModel)), "chatmodel", "model":
		return components.ComponentOfChatModel
	case strings.ToLower(string(components.ComponentOfAgenticModel)), "agenticmodel":
		return components.ComponentOfAgenticModel
	case strings.ToLower(string(components.ComponentOfEmbedding)), "embedding":
		return components.ComponentOfEmbedding
	case strings.ToLower(string(components.ComponentOfIndexer)), "indexer":
		return components.ComponentOfIndexer
	case strings.ToLower(string(components.ComponentOfRetriever)), "retriever":
		return components.ComponentOfRetriever
	case strings.ToLower(string(components.ComponentOfLoader)), "loader":
		return components.ComponentOfLoader
	case strings.ToLower(string(components.ComponentOfTransformer)), "transformer", "documenttransformer":
		return components.ComponentOfTransformer
	case strings.ToLower(string(components.ComponentOfTool)), "tool":
		return components.ComponentOfTool
	default:
		return components.Component(kind)
	}
}
