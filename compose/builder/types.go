package builder

type GraphName string
type Version string

type NodeID string
type EdgeID string
type BlockID string
type SubGraphID string

type SchemaRef string
type HandlerRef string
type BindingRef string
type PolicyRef string
type ExportRef string

type ResumeEntryID string
type CheckpointScopeID string

type GraphSpec struct {
	Name        GraphName
	Description string
	Version     Version

	Nodes      []NodeSpec
	Edges      []EdgeSpec
	Blocks     []BlockSpec
	Boundaries []SubGraphSpec

	State    StateSpec
	Recovery RecoverySpec
	Bindings []BindingSpec
	Policies []PolicySpec
	Exports  []ExportSpec
	Metadata map[string]any
}

type NodeSpec struct {
	ID                  NodeID
	Kind                NodeKind
	Name                string
	InputSchemaRef      SchemaRef
	OutputSchemaRef     SchemaRef
	LocalStateSchemaRef SchemaRef
	HandlerRef          HandlerRef
	BindingRef          BindingRef
	PolicyRef           PolicyRef
	Tags                []string
	Metadata            map[string]any
}

type NodeKind string

const (
	NodeKindModel     NodeKind = "model"
	NodeKindTool      NodeKind = "tool"
	NodeKindLambda    NodeKind = "lambda"
	NodeKindRouter    NodeKind = "router"
	NodeKindBranch    NodeKind = "branch"
	NodeKindJoin      NodeKind = "join"
	NodeKindLoop      NodeKind = "loop"
	NodeKindSubgraph  NodeKind = "subgraph"
	NodeKindHuman     NodeKind = "human"
	NodeKindMemory    NodeKind = "memory"
	NodeKindIORef     NodeKind = "io_ref"
	NodeKindSkill     NodeKind = "skill"
	NodeKindTransform NodeKind = "transform"
)

type EdgeSpec struct {
	ID         EdgeID
	Kind       EdgeKind
	From       NodeID
	To         NodeID
	Priority   int
	Condition  *ConditionSpec
	Projection *ProjectionSpec
	ErrorMatch *ErrorMatchSpec
	Metadata   map[string]any
}

type EdgeKind string

const (
	EdgeKindControl     EdgeKind = "control"
	EdgeKindData        EdgeKind = "data"
	EdgeKindConditional EdgeKind = "conditional"
	EdgeKindError       EdgeKind = "error"
	EdgeKindResume      EdgeKind = "resume"
	EdgeKindProjection  EdgeKind = "projection"
)

type ConditionSpec struct {
	Expr string
}

type ProjectionSpec struct {
	Reads   []string
	Writes  []string
	Mapping map[string]string
	Mode    ProjectionMode
}

type ProjectionMode string

const (
	ProjectionModeMap     ProjectionMode = "map"
	ProjectionModeFilter  ProjectionMode = "filter"
	ProjectionModeSummary ProjectionMode = "summary"
)

type ErrorMatchSpec struct {
	Codes []string
	Expr  string
}

type BlockSpec struct {
	ID                  BlockID
	Kind                BlockKind
	Nodes               []NodeID
	EntryNodes          []NodeID
	ExitNodes           []NodeID
	BlockStateSchemaRef SchemaRef
	RecoveryScopeRef    CheckpointScopeID
	PolicyRef           PolicyRef
	Metadata            map[string]any
}

type BlockKind string

const (
	BlockKindSequence BlockKind = "sequence"
	BlockKindParallel BlockKind = "parallel"
	BlockKindBranch   BlockKind = "branch"
	BlockKindLoop     BlockKind = "loop"
	BlockKindRetry    BlockKind = "retry"
	BlockKindFallback BlockKind = "fallback"
)

type SubGraphSpec struct {
	ID               SubGraphID
	Name             string
	EntryNode        NodeID
	ExitNodes        []NodeID
	Projection       *SubGraphProjectionSpec
	Visibility       *SubGraphVisibilitySpec
	RecoveryScopeRef CheckpointScopeID
	Ownership        *SubGraphOwnershipSpec
	Metadata         map[string]any
}

type SubGraphProjectionSpec struct {
	Reads   []string
	Writes  []string
	Mapping map[string]string
	Mode    ProjectionMode
}

type SubGraphVisibilitySpec struct {
	AllowedBindings []BindingRef
	AllowedTools    []string
	AllowedMemories []string
}

type SubGraphOwnershipSpec struct {
	OwnerBlockID BlockID
	OwnerNodeID  NodeID
	OwnerDomain  string
}

type StateSpec struct {
	InputSchemaRef      SchemaRef
	OutputSchemaRef     SchemaRef
	GraphStateSchemaRef SchemaRef
	NodeStateSchemas    map[NodeID]SchemaRef
	BlockStateSchemas   map[BlockID]SchemaRef
}

type RecoverySpec struct {
	InterruptPoints      []InterruptPointSpec
	CheckpointScopes     []CheckpointScopeSpec
	PersistedStateFields []string
	ResumeEntries        []ResumeEntrySpec
	ReplayPolicies       []ReplayPolicySpec
	RecoveryInvariants   []RecoveryInvariantSpec
}

type InterruptPointSpec struct {
	NodeID  NodeID
	BlockID BlockID
	Reason  string
}

type CheckpointScopeSpec struct {
	ID              CheckpointScopeID
	Name            string
	NodeIDs         []NodeID
	BlockIDs        []BlockID
	PersistedFields []string
}

type ResumeEntrySpec struct {
	ID      ResumeEntryID
	Name    string
	EdgeID  EdgeID
	NodeID  NodeID
	BlockID BlockID
}

type ReplayPolicySpec struct {
	ScopeID             CheckpointScopeID
	ReplayNodeIDs       []NodeID
	RequiresIdempotency bool
}

type RecoveryInvariantSpec struct {
	Expr        string
	Description string
}

type BindingSpec struct {
	Ref      BindingRef
	Kind     BindingKind
	Target   string
	Config   map[string]any
	Metadata map[string]any
}

type BindingKind string

const (
	BindingKindModel       BindingKind = "model"
	BindingKindTool        BindingKind = "tool"
	BindingKindMemory      BindingKind = "memory"
	BindingKindSkill       BindingKind = "skill"
	BindingKindService     BindingKind = "service"
	BindingKindInterceptor BindingKind = "interceptor"
)

type PolicySpec struct {
	Ref      PolicyRef
	Kind     PolicyKind
	Config   map[string]any
	Metadata map[string]any
}

type PolicyKind string

const (
	PolicyKindExecution   PolicyKind = "execution"
	PolicyKindRecovery    PolicyKind = "recovery"
	PolicyKindConcurrency PolicyKind = "concurrency"
	PolicyKindVisibility  PolicyKind = "visibility"
)

type ExportSpec struct {
	Ref         ExportRef
	Name        string
	Description string
	Metadata    map[string]any
}

type ExecutionPlan struct {
	Name       GraphName
	Version    Version
	Structural StructuralPlan
	State      StateRecoveryPlan
	Runtime    RuntimeBindingPlan
	Artifacts  BuildArtifact
}

type StructuralPlan struct {
	Nodes      []PlannedNode
	Edges      []PlannedEdge
	Blocks     []PlannedBlock
	Boundaries []PlannedSubGraph
	EntryNodes []NodeID
	ExitNodes  []NodeID
}

type PlannedNode struct {
	ID                  NodeID
	Kind                NodeKind
	Name                string
	InputSchemaRef      SchemaRef
	OutputSchemaRef     SchemaRef
	LocalStateSchemaRef SchemaRef
	Handler             HandlerRef
	Binding             BindingRef
	Policy              PolicyRef
	Tags                []string
	Metadata            map[string]any
}

type PlannedEdge struct {
	ID         EdgeID
	Kind       EdgeKind
	From       NodeID
	To         NodeID
	Priority   int
	Condition  *ConditionSpec
	Projection *ProjectionSpec
	ErrorMatch *ErrorMatchSpec
	Metadata   map[string]any
}

type PlannedBlock struct {
	ID               BlockID
	Kind             BlockKind
	Nodes            []NodeID
	EntryNodes       []NodeID
	ExitNodes        []NodeID
	RecoveryScopeRef CheckpointScopeID
	PolicyRef        PolicyRef
	Metadata         map[string]any
}

type PlannedSubGraph struct {
	ID               SubGraphID
	Name             string
	EntryNode        NodeID
	ExitNodes        []NodeID
	Projection       *SubGraphProjectionSpec
	Visibility       *SubGraphVisibilitySpec
	RecoveryScopeRef CheckpointScopeID
	Ownership        *SubGraphOwnershipSpec
	Metadata         map[string]any
}

type StateRecoveryPlan struct {
	InputSchemaRef      SchemaRef
	OutputSchemaRef     SchemaRef
	GraphStateSchemaRef SchemaRef
	LocalStateSchemas   map[NodeID]SchemaRef
	BlockStateSchemas   map[BlockID]SchemaRef

	Projections      []PlannedProjection
	CheckpointScopes []PlannedCheckpointScope
	ResumeEntries    []PlannedResumeEntry
	ReplayPolicies   []PlannedReplayPolicy
}

type PlannedProjection struct {
	EdgeID     EdgeID
	SubGraphID SubGraphID
	Reads      []string
	Writes     []string
	Mapping    map[string]string
	Mode       ProjectionMode
}

type PlannedCheckpointScope struct {
	ID              CheckpointScopeID
	PersistedFields []string
}

type PlannedResumeEntry struct {
	ID      ResumeEntryID
	NodeID  NodeID
	EdgeID  EdgeID
	BlockID BlockID
}

type PlannedReplayPolicy struct {
	ScopeID       CheckpointScopeID
	ReplayNodeIDs []NodeID
}

type RuntimeBindingPlan struct {
	NodeBindings        map[NodeID]BindingRef
	NodePolicies        map[NodeID]PolicyRef
	SubGraphBindings    map[SubGraphID][]BindingRef
	InterceptorBindings []BindingRef
	VisibilityPolicies  map[string]PolicyRef
	BindingCatalog      map[BindingRef]BindingSpec
	PolicyCatalog       map[PolicyRef]PolicySpec
}

type BuildArtifact struct {
	NodeViews     []ArtifactNodeView
	EdgeViews     []ArtifactEdgeView
	BlockViews    []ArtifactBlockView
	SubGraphViews []ArtifactSubGraphView
	StateViews    []ArtifactStateView
	ResumeViews   []ArtifactResumeView
}

type ArtifactNodeView struct {
	ID   NodeID
	Kind NodeKind
	Name string
}

type ArtifactEdgeView struct {
	ID   EdgeID
	Kind EdgeKind
	From NodeID
	To   NodeID
}

type ArtifactBlockView struct {
	ID   BlockID
	Kind BlockKind
}

type ArtifactSubGraphView struct {
	ID        SubGraphID
	EntryNode NodeID
}

type ArtifactStateView struct {
	Name      string
	SchemaRef SchemaRef
}

type ArtifactResumeView struct {
	ID     ResumeEntryID
	NodeID NodeID
	EdgeID EdgeID
}

type RuntimeOverlay struct {
	InstructionOverride string
	BindingOverrides    map[BindingRef]BindingRef
	PolicyOverrides     map[PolicyRef]PolicyRef
	DebugOptions        map[string]any
	Metadata            map[string]any
}
