package builder

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

type SupervisorBuilder struct {
	Name        GraphName
	Version     Version
	Description string

	Enter      *NodeSpec
	Supervisor NodeSpec
	Workers    []NodeSpec
	Finish     NodeSpec

	WorkerConditions map[NodeID]string
	FinishCondition  string
	ReturnEdges      map[NodeID]PresetEdgeSpec

	BlockID               BlockID
	BlockRecoveryScopeRef CheckpointScopeID
	BlockPolicyRef        PolicyRef
	BlockMetadata         map[string]any

	State      StateSpec
	Recovery   RecoverySpec
	Bindings   []BindingSpec
	Policies   []PolicySpec
	Boundaries []BoundarySpec
	Exports    []ExportSpec
	Metadata   map[string]any
}

func (b SupervisorBuilder) BuildSpec() (GraphSpec, error) {
	if strings.TrimSpace(string(b.Name)) == "" {
		return GraphSpec{}, fmt.Errorf("supervisor builder requires graph name")
	}
	if b.Supervisor.ID == "" {
		return GraphSpec{}, fmt.Errorf("supervisor builder %q requires supervisor node", b.Name)
	}
	if len(b.Workers) == 0 {
		return GraphSpec{}, fmt.Errorf("supervisor builder %q requires worker nodes", b.Name)
	}
	if b.Finish.ID == "" {
		return GraphSpec{}, fmt.Errorf("supervisor builder %q requires finish node", b.Name)
	}

	spec := GraphSpec{
		Name:        b.Name,
		Description: b.Description,
		Version:     normalizedVersion(b.Version),
		State:       copyStateSpec(b.State),
		Recovery:    copyRecoverySpec(b.Recovery),
		Metadata:    copyAnyMap(b.Metadata),
	}
	if b.Enter != nil {
		spec.Nodes = append(spec.Nodes, copyNodeSpec(*b.Enter))
	}
	spec.Nodes = append(spec.Nodes, copyNodeSpec(b.Supervisor))
	for _, worker := range b.Workers {
		spec.Nodes = append(spec.Nodes, copyNodeSpec(worker))
	}
	spec.Nodes = append(spec.Nodes, copyNodeSpec(b.Finish))

	if b.Enter != nil {
		spec.Edges = append(spec.Edges, EdgeSpec{
			ID:   EdgeID(fmt.Sprintf("%s_enter", b.Name)),
			Kind: EdgeKindControl,
			From: b.Enter.ID,
			To:   b.Supervisor.ID,
		})
	}
	for i, worker := range b.Workers {
		condition := b.WorkerConditions[worker.ID]
		if strings.TrimSpace(condition) == "" {
			condition = fmt.Sprintf("route=%s", worker.ID)
		}
		spec.Edges = append(spec.Edges, EdgeSpec{
			ID:       EdgeID(fmt.Sprintf("%s_worker_%d", b.Name, i+1)),
			Kind:     EdgeKindConditional,
			From:     b.Supervisor.ID,
			To:       worker.ID,
			Priority: i,
			Condition: &ConditionSpec{
				Expr: condition,
			},
		})
		template := b.ReturnEdges[worker.ID]
		spec.Edges = append(spec.Edges, template.toEdge(worker.ID, b.Supervisor.ID, fmt.Sprintf("%s_return_%d", b.Name, i+1)))
	}
	finishCondition := b.FinishCondition
	if strings.TrimSpace(finishCondition) == "" {
		finishCondition = "route=finish"
	}
	spec.Edges = append(spec.Edges, EdgeSpec{
		ID:       EdgeID(fmt.Sprintf("%s_finish", b.Name)),
		Kind:     EdgeKindConditional,
		From:     b.Supervisor.ID,
		To:       b.Finish.ID,
		Priority: len(b.Workers),
		Condition: &ConditionSpec{
			Expr: finishCondition,
		},
	})

	blockID := b.BlockID
	if blockID == "" {
		blockID = BlockID(fmt.Sprintf("%s_supervisor_loop", b.Name))
	}
	blockNodes := []NodeID{b.Supervisor.ID}
	for _, worker := range b.Workers {
		blockNodes = append(blockNodes, worker.ID)
	}
	spec.Blocks = append(spec.Blocks, BlockSpec{
		ID:               blockID,
		Kind:             BlockKindLoop,
		Nodes:            blockNodes,
		EntryNodes:       []NodeID{b.Supervisor.ID},
		ExitNodes:        []NodeID{b.Finish.ID},
		RecoveryScopeRef: b.BlockRecoveryScopeRef,
		PolicyRef:        b.BlockPolicyRef,
		Metadata:         copyAnyMap(b.BlockMetadata),
	})

	for _, binding := range b.Bindings {
		spec.Bindings = append(spec.Bindings, copyBindingSpec(binding))
	}
	for _, policy := range b.Policies {
		spec.Policies = append(spec.Policies, copyPolicySpec(policy))
	}
	for _, boundary := range b.Boundaries {
		spec.Boundaries = append(spec.Boundaries, copyBoundarySpec(boundary))
	}
	for _, export := range b.Exports {
		spec.Exports = append(spec.Exports, copyExportSpec(export))
	}
	return spec, nil
}

type ExpandedReActConfig struct {
	Name        GraphName
	Version     Version
	Description string

	ModelName string
	ToolName  string

	EnterHandlerRef      HandlerRef
	ModelHandlerRef      HandlerRef
	RouteHandlerRef      HandlerRef
	ToolHandlerRef       HandlerRef
	AfterToolHandlerRef  HandlerRef
	FinishHandlerRef     HandlerRef
	ModelBindingRef      BindingRef
	ToolBindingRef       BindingRef
	PolicyRef            PolicyRef
	InputSchemaRef       SchemaRef
	OutputSchemaRef      SchemaRef
	GraphStateSchemaRef  SchemaRef
	NodeStateSchemas     map[NodeID]SchemaRef
	AfterToModelEdge     *ProjectionSpec
	ToolBoundaryID       BoundaryID
	ToolBoundaryName     string
	BoundaryProjection   *BoundaryProjectionSpec
	AllowedBindings      []BindingRef
	AllowedTools         []string
	AllowedMemories      []string
	LoopScopeID          CheckpointScopeID
	GraphScopeID         CheckpointScopeID
	LoopPersistedFields  []string
	GraphPersistedFields []string
	MaxIterations        int
	Bindings             []BindingSpec
	Policies             []PolicySpec
	Metadata             map[string]any
}

type ExpandedReActBuilder struct {
	Config ExpandedReActConfig
}

func (b ExpandedReActBuilder) BuildSpec() (GraphSpec, error) {
	return BuildExpandedReActSpec(b.Config)
}

func BuildExpandedReActSpec(config ExpandedReActConfig) (GraphSpec, error) {
	if strings.TrimSpace(string(config.Name)) == "" {
		return GraphSpec{}, fmt.Errorf("expanded react builder requires graph name")
	}
	if config.ModelHandlerRef == "" {
		return GraphSpec{}, fmt.Errorf("expanded react builder %q requires model handler", config.Name)
	}
	if config.RouteHandlerRef == "" {
		return GraphSpec{}, fmt.Errorf("expanded react builder %q requires route handler", config.Name)
	}
	if config.ToolHandlerRef == "" {
		return GraphSpec{}, fmt.Errorf("expanded react builder %q requires tool handler", config.Name)
	}

	modelName := config.ModelName
	if modelName == "" {
		modelName = "ChatModel"
	}
	toolName := config.ToolName
	if toolName == "" {
		toolName = "ToolNode"
	}
	loopScopeID := config.LoopScopeID
	if loopScopeID == "" {
		loopScopeID = CheckpointScopeID(fmt.Sprintf("%s.react.loop", config.Name))
	}
	graphScopeID := config.GraphScopeID
	if graphScopeID == "" {
		graphScopeID = CheckpointScopeID(fmt.Sprintf("%s.react.graph", config.Name))
	}
	policyRef := config.PolicyRef
	if policyRef == "" {
		policyRef = PolicyRef(fmt.Sprintf("policy.%s.react", config.Name))
	}
	boundaryID := config.ToolBoundaryID
	if boundaryID == "" {
		boundaryID = BoundaryID(fmt.Sprintf("%s.tools", config.Name))
	}
	afterProjection := copyProjectionSpec(config.AfterToModelEdge)
	if afterProjection == nil {
		afterProjection = &ProjectionSpec{
			Reads:   []string{"tool_results", "messages"},
			Writes:  []string{"messages", "tool_results"},
			Mapping: map[string]string{},
			Mode:    ProjectionModeMap,
		}
	}

	bindings := make([]BindingSpec, 0, len(config.Bindings)+2)
	for _, binding := range config.Bindings {
		bindings = append(bindings, copyBindingSpec(binding))
	}
	if config.ModelBindingRef != "" && !containsBinding(bindings, config.ModelBindingRef) {
		bindings = append(bindings, BindingSpec{Ref: config.ModelBindingRef, Kind: BindingKindModel})
	}
	if config.ToolBindingRef != "" && !containsBinding(bindings, config.ToolBindingRef) {
		bindings = append(bindings, BindingSpec{Ref: config.ToolBindingRef, Kind: BindingKindTool})
	}

	policies := make([]PolicySpec, 0, len(config.Policies)+1)
	for _, policy := range config.Policies {
		policies = append(policies, copyPolicySpec(policy))
	}
	if policyRef != "" && !containsPolicy(policies, policyRef) {
		maxIterations := config.MaxIterations
		if maxIterations <= 0 {
			maxIterations = 8
		}
		policies = append(policies, PolicySpec{
			Ref:  policyRef,
			Kind: PolicyKindExecution,
			Config: map[string]any{
				"max_iterations": maxIterations,
			},
		})
	}

	spec := GraphSpec{
		Name:        config.Name,
		Description: config.Description,
		Version:     normalizedVersion(config.Version),
		Nodes: []NodeSpec{
			{ID: "enter", Kind: NodeKindLambda, Name: "Init", HandlerRef: config.EnterHandlerRef},
			{
				ID:             "model",
				Kind:           NodeKindModel,
				Name:           modelName,
				InputSchemaRef: config.InputSchemaRef,
				OutputSchemaRef: func() SchemaRef {
					if config.OutputSchemaRef != "" {
						return config.OutputSchemaRef
					}
					return config.InputSchemaRef
				}(),
				HandlerRef: config.ModelHandlerRef,
				BindingRef: config.ModelBindingRef,
				PolicyRef:  policyRef,
			},
			{ID: "route", Kind: NodeKindRouter, Name: "ToolRouter", HandlerRef: config.RouteHandlerRef},
			{ID: "tools", Kind: NodeKindTool, Name: toolName, HandlerRef: config.ToolHandlerRef, BindingRef: config.ToolBindingRef},
			{ID: "after_tools", Kind: NodeKindTransform, Name: "AfterToolCalls", HandlerRef: config.AfterToolHandlerRef},
			{ID: "finish", Kind: NodeKindLambda, Name: "Finish", HandlerRef: config.FinishHandlerRef},
		},
		Edges: []EdgeSpec{
			{ID: EdgeID(fmt.Sprintf("%s_e0", config.Name)), Kind: EdgeKindControl, From: NodeID(compose.START), To: "enter"},
			{ID: EdgeID(fmt.Sprintf("%s_e1", config.Name)), Kind: EdgeKindControl, From: "enter", To: "model"},
			{ID: EdgeID(fmt.Sprintf("%s_e2", config.Name)), Kind: EdgeKindControl, From: "model", To: "route"},
			{ID: EdgeID(fmt.Sprintf("%s_e3", config.Name)), Kind: EdgeKindConditional, From: "route", To: "tools", Condition: &ConditionSpec{Expr: "has_tool_calls"}},
			{ID: EdgeID(fmt.Sprintf("%s_e4", config.Name)), Kind: EdgeKindConditional, From: "route", To: "finish", Condition: &ConditionSpec{Expr: "!has_tool_calls"}},
			{ID: EdgeID(fmt.Sprintf("%s_e5", config.Name)), Kind: EdgeKindControl, From: "tools", To: "after_tools"},
			{ID: EdgeID(fmt.Sprintf("%s_e6", config.Name)), Kind: EdgeKindProjection, From: "after_tools", To: "model", Projection: afterProjection},
			{ID: EdgeID(fmt.Sprintf("%s_e7", config.Name)), Kind: EdgeKindControl, From: "finish", To: NodeID(compose.END)},
		},
		Blocks: []BlockSpec{{
			ID:               BlockID(fmt.Sprintf("%s.react_loop", config.Name)),
			Kind:             BlockKindLoop,
			Nodes:            []NodeID{"model", "route", "tools", "after_tools"},
			EntryNodes:       []NodeID{"model"},
			ExitNodes:        []NodeID{"finish"},
			RecoveryScopeRef: loopScopeID,
			PolicyRef:        policyRef,
		}},
		Boundaries: []BoundarySpec{{
			ID:        boundaryID,
			Name:      config.ToolBoundaryName,
			EntryNode: "tools",
			ExitNodes: []NodeID{"after_tools"},
			Projection: copyBoundaryProjectionSpec(func() *BoundaryProjectionSpec {
				if config.BoundaryProjection != nil {
					return config.BoundaryProjection
				}
				if afterProjection == nil {
					return nil
				}
				return &BoundaryProjectionSpec{
					Reads:   append([]string(nil), afterProjection.Reads...),
					Writes:  append([]string(nil), afterProjection.Writes...),
					Mapping: copyStringMap(afterProjection.Mapping),
					Mode:    afterProjection.Mode,
				}
			}()),
			Visibility: &BoundaryVisibilitySpec{
				AllowedBindings: append([]BindingRef(nil), config.AllowedBindings...),
				AllowedTools:    append([]string(nil), config.AllowedTools...),
				AllowedMemories: append([]string(nil), config.AllowedMemories...),
			},
			RecoveryScopeRef: loopScopeID,
		}},
		State: StateSpec{
			InputSchemaRef:      config.InputSchemaRef,
			OutputSchemaRef:     config.OutputSchemaRef,
			GraphStateSchemaRef: config.GraphStateSchemaRef,
			NodeStateSchemas:    copySchemaRefMap(config.NodeStateSchemas),
			BlockStateSchemas:   map[BlockID]SchemaRef{},
		},
		Recovery: RecoverySpec{
			CheckpointScopes: []CheckpointScopeSpec{
				{
					ID:              graphScopeID,
					NodeIDs:         []NodeID{"model", "route", "tools", "after_tools", "finish"},
					PersistedFields: append([]string(nil), config.GraphPersistedFields...),
				},
				{
					ID:              loopScopeID,
					BlockIDs:        []BlockID{BlockID(fmt.Sprintf("%s.react_loop", config.Name))},
					PersistedFields: append([]string(nil), config.LoopPersistedFields...),
				},
			},
			ResumeEntries: []ResumeEntrySpec{
				{ID: ResumeEntryID(fmt.Sprintf("%s.resume.model", config.Name)), NodeID: "model", EdgeID: EdgeID(fmt.Sprintf("%s_e6", config.Name))},
				{ID: ResumeEntryID(fmt.Sprintf("%s.resume.tools", config.Name)), NodeID: "tools", EdgeID: EdgeID(fmt.Sprintf("%s_e3", config.Name))},
			},
		},
		Bindings: bindings,
		Policies: policies,
		Metadata: copyAnyMap(config.Metadata),
	}
	return spec, nil
}

type LoopBuilder struct {
	Name        GraphName
	Version     Version
	Description string

	Enter  *NodeSpec
	Body   NodeSpec
	Judge  NodeSpec
	Finish NodeSpec

	BodyToJudgeEdge   PresetEdgeSpec
	ContinueEdge      PresetEdgeSpec
	ExitEdge          PresetEdgeSpec
	ContinueCondition string
	ExitCondition     string

	BlockID               BlockID
	BlockRecoveryScopeRef CheckpointScopeID
	BlockPolicyRef        PolicyRef
	BlockMetadata         map[string]any

	State      StateSpec
	Recovery   RecoverySpec
	Bindings   []BindingSpec
	Policies   []PolicySpec
	Boundaries []BoundarySpec
	Exports    []ExportSpec
	Metadata   map[string]any
}

func (b LoopBuilder) BuildSpec() (GraphSpec, error) {
	if strings.TrimSpace(string(b.Name)) == "" {
		return GraphSpec{}, fmt.Errorf("loop builder requires graph name")
	}
	if b.Body.ID == "" {
		return GraphSpec{}, fmt.Errorf("loop builder %q requires body node", b.Name)
	}
	if b.Judge.ID == "" {
		return GraphSpec{}, fmt.Errorf("loop builder %q requires judge node", b.Name)
	}
	if b.Finish.ID == "" {
		return GraphSpec{}, fmt.Errorf("loop builder %q requires finish node", b.Name)
	}

	blockID := b.BlockID
	if blockID == "" {
		blockID = BlockID(fmt.Sprintf("%s_loop", b.Name))
	}
	scopeID := b.BlockRecoveryScopeRef
	if scopeID == "" {
		scopeID = CheckpointScopeID(fmt.Sprintf("%s.loop.scope", b.Name))
	}
	continueExpr := strings.TrimSpace(b.ContinueCondition)
	if continueExpr == "" {
		continueExpr = "continue"
	}
	exitExpr := strings.TrimSpace(b.ExitCondition)
	if exitExpr == "" {
		exitExpr = "!continue"
	}

	spec := GraphSpec{
		Name:        b.Name,
		Description: b.Description,
		Version:     normalizedVersion(b.Version),
		State:       copyStateSpec(b.State),
		Recovery:    copyRecoverySpec(b.Recovery),
		Metadata:    copyAnyMap(b.Metadata),
	}
	if b.Enter != nil {
		spec.Nodes = append(spec.Nodes, copyNodeSpec(*b.Enter))
	}
	spec.Nodes = append(spec.Nodes, copyNodeSpec(b.Body), copyNodeSpec(b.Judge), copyNodeSpec(b.Finish))

	entryNodeID := b.Body.ID
	if b.Enter != nil {
		entryNodeID = b.Enter.ID
		spec.Edges = append(spec.Edges,
			EdgeSpec{ID: EdgeID(fmt.Sprintf("%s_e0", b.Name)), Kind: EdgeKindControl, From: NodeID(compose.START), To: b.Enter.ID},
			EdgeSpec{ID: EdgeID(fmt.Sprintf("%s_e1", b.Name)), Kind: EdgeKindControl, From: b.Enter.ID, To: b.Body.ID},
		)
	} else {
		spec.Edges = append(spec.Edges, EdgeSpec{
			ID:   EdgeID(fmt.Sprintf("%s_e0", b.Name)),
			Kind: EdgeKindControl,
			From: NodeID(compose.START),
			To:   b.Body.ID,
		})
	}
	spec.Edges = append(spec.Edges,
		b.BodyToJudgeEdge.toEdge(b.Body.ID, b.Judge.ID, fmt.Sprintf("%s_body_judge", b.Name)),
		PresetEdgeSpec{
			Kind:      EdgeKindConditional,
			Condition: &ConditionSpec{Expr: continueExpr},
		}.toEdge(b.Judge.ID, b.Body.ID, fmt.Sprintf("%s_continue", b.Name)),
		PresetEdgeSpec{
			Kind:      EdgeKindConditional,
			Condition: &ConditionSpec{Expr: exitExpr},
		}.toEdge(b.Judge.ID, b.Finish.ID, fmt.Sprintf("%s_finish", b.Name)),
		EdgeSpec{ID: EdgeID(fmt.Sprintf("%s_e_end", b.Name)), Kind: EdgeKindControl, From: b.Finish.ID, To: NodeID(compose.END)},
	)
	if b.ContinueEdge.ID != "" || b.ContinueEdge.Kind != "" || b.ContinueEdge.Condition != nil || b.ContinueEdge.Projection != nil || b.ContinueEdge.ErrorMatch != nil || b.ContinueEdge.Metadata != nil || b.ContinueEdge.Priority != 0 {
		spec.Edges[len(spec.Edges)-3] = b.ContinueEdge.toEdge(b.Judge.ID, b.Body.ID, fmt.Sprintf("%s_continue", b.Name))
		if spec.Edges[len(spec.Edges)-3].Condition == nil {
			spec.Edges[len(spec.Edges)-3].Condition = &ConditionSpec{Expr: continueExpr}
		}
	}
	if b.ExitEdge.ID != "" || b.ExitEdge.Kind != "" || b.ExitEdge.Condition != nil || b.ExitEdge.Projection != nil || b.ExitEdge.ErrorMatch != nil || b.ExitEdge.Metadata != nil || b.ExitEdge.Priority != 0 {
		spec.Edges[len(spec.Edges)-2] = b.ExitEdge.toEdge(b.Judge.ID, b.Finish.ID, fmt.Sprintf("%s_finish", b.Name))
		if spec.Edges[len(spec.Edges)-2].Condition == nil {
			spec.Edges[len(spec.Edges)-2].Condition = &ConditionSpec{Expr: exitExpr}
		}
	}

	spec.Blocks = append(spec.Blocks, BlockSpec{
		ID:               blockID,
		Kind:             BlockKindLoop,
		Nodes:            []NodeID{b.Body.ID, b.Judge.ID},
		EntryNodes:       []NodeID{b.Body.ID},
		ExitNodes:        []NodeID{b.Finish.ID},
		RecoveryScopeRef: scopeID,
		PolicyRef:        b.BlockPolicyRef,
		Metadata:         copyAnyMap(b.BlockMetadata),
	})

	ensureCheckpointScope(&spec.Recovery, scopeID, blockID)
	ensureResumeEntry(&spec.Recovery, ResumeEntryID(fmt.Sprintf("%s.resume.body", b.Name)), b.Body.ID, EdgeID(fmt.Sprintf("%s_continue", b.Name)))
	ensureResumeEntry(&spec.Recovery, ResumeEntryID(fmt.Sprintf("%s.resume.judge", b.Name)), b.Judge.ID, EdgeID(fmt.Sprintf("%s_body_judge", b.Name)))

	for _, binding := range b.Bindings {
		spec.Bindings = append(spec.Bindings, copyBindingSpec(binding))
	}
	for _, policy := range b.Policies {
		spec.Policies = append(spec.Policies, copyPolicySpec(policy))
	}
	for _, boundary := range b.Boundaries {
		spec.Boundaries = append(spec.Boundaries, copyBoundarySpec(boundary))
	}
	for _, export := range b.Exports {
		spec.Exports = append(spec.Exports, copyExportSpec(export))
	}
	_ = entryNodeID
	return spec, nil
}

type PlanExecBuilder struct {
	Name        GraphName
	Version     Version
	Description string

	Enter     *NodeSpec
	Planner   NodeSpec
	Executor  NodeSpec
	Replanner NodeSpec
	Finish    NodeSpec

	PlannerToExecutorEdge PresetEdgeSpec
	ExecutorToReplanEdge  PresetEdgeSpec
	ContinueEdge          PresetEdgeSpec
	ReplanEdge            PresetEdgeSpec
	FinishEdge            PresetEdgeSpec

	ContinueCondition string
	ReplanCondition   string
	FinishCondition   string

	BlockID               BlockID
	BlockRecoveryScopeRef CheckpointScopeID
	BlockPolicyRef        PolicyRef
	BlockMetadata         map[string]any

	State      StateSpec
	Recovery   RecoverySpec
	Bindings   []BindingSpec
	Policies   []PolicySpec
	Boundaries []BoundarySpec
	Exports    []ExportSpec
	Metadata   map[string]any
}

func (b PlanExecBuilder) BuildSpec() (GraphSpec, error) {
	if strings.TrimSpace(string(b.Name)) == "" {
		return GraphSpec{}, fmt.Errorf("planexec builder requires graph name")
	}
	if b.Planner.ID == "" {
		return GraphSpec{}, fmt.Errorf("planexec builder %q requires planner node", b.Name)
	}
	if b.Executor.ID == "" {
		return GraphSpec{}, fmt.Errorf("planexec builder %q requires executor node", b.Name)
	}
	if b.Replanner.ID == "" {
		return GraphSpec{}, fmt.Errorf("planexec builder %q requires replanner node", b.Name)
	}
	if b.Finish.ID == "" {
		return GraphSpec{}, fmt.Errorf("planexec builder %q requires finish node", b.Name)
	}

	blockID := b.BlockID
	if blockID == "" {
		blockID = BlockID(fmt.Sprintf("%s_exec_loop", b.Name))
	}
	scopeID := b.BlockRecoveryScopeRef
	if scopeID == "" {
		scopeID = CheckpointScopeID(fmt.Sprintf("%s.planexec.scope", b.Name))
	}
	continueExpr := strings.TrimSpace(b.ContinueCondition)
	if continueExpr == "" {
		continueExpr = "action=execute"
	}
	replanExpr := strings.TrimSpace(b.ReplanCondition)
	if replanExpr == "" {
		replanExpr = "action=replan"
	}
	finishExpr := strings.TrimSpace(b.FinishCondition)
	if finishExpr == "" {
		finishExpr = "action=finish"
	}

	spec := GraphSpec{
		Name:        b.Name,
		Description: b.Description,
		Version:     normalizedVersion(b.Version),
		State:       copyStateSpec(b.State),
		Recovery:    copyRecoverySpec(b.Recovery),
		Metadata:    copyAnyMap(b.Metadata),
	}
	if b.Enter != nil {
		spec.Nodes = append(spec.Nodes, copyNodeSpec(*b.Enter))
	}
	spec.Nodes = append(spec.Nodes, copyNodeSpec(b.Planner), copyNodeSpec(b.Executor), copyNodeSpec(b.Replanner), copyNodeSpec(b.Finish))

	if b.Enter != nil {
		spec.Edges = append(spec.Edges,
			EdgeSpec{ID: EdgeID(fmt.Sprintf("%s_e0", b.Name)), Kind: EdgeKindControl, From: NodeID(compose.START), To: b.Enter.ID},
			EdgeSpec{ID: EdgeID(fmt.Sprintf("%s_e1", b.Name)), Kind: EdgeKindControl, From: b.Enter.ID, To: b.Planner.ID},
		)
	} else {
		spec.Edges = append(spec.Edges, EdgeSpec{
			ID:   EdgeID(fmt.Sprintf("%s_e0", b.Name)),
			Kind: EdgeKindControl,
			From: NodeID(compose.START),
			To:   b.Planner.ID,
		})
	}
	spec.Edges = append(spec.Edges,
		b.PlannerToExecutorEdge.toEdge(b.Planner.ID, b.Executor.ID, fmt.Sprintf("%s_plan_exec", b.Name)),
		b.ExecutorToReplanEdge.toEdge(b.Executor.ID, b.Replanner.ID, fmt.Sprintf("%s_exec_replan", b.Name)),
		PresetEdgeSpec{
			Kind:      EdgeKindConditional,
			Condition: &ConditionSpec{Expr: continueExpr},
		}.toEdge(b.Replanner.ID, b.Executor.ID, fmt.Sprintf("%s_continue", b.Name)),
		PresetEdgeSpec{
			Kind:      EdgeKindConditional,
			Condition: &ConditionSpec{Expr: replanExpr},
		}.toEdge(b.Replanner.ID, b.Planner.ID, fmt.Sprintf("%s_replan", b.Name)),
		PresetEdgeSpec{
			Kind:      EdgeKindConditional,
			Condition: &ConditionSpec{Expr: finishExpr},
		}.toEdge(b.Replanner.ID, b.Finish.ID, fmt.Sprintf("%s_finish", b.Name)),
		EdgeSpec{ID: EdgeID(fmt.Sprintf("%s_e_end", b.Name)), Kind: EdgeKindControl, From: b.Finish.ID, To: NodeID(compose.END)},
	)
	if hasPresetEdge(b.ContinueEdge) {
		spec.Edges[len(spec.Edges)-4] = b.ContinueEdge.toEdge(b.Replanner.ID, b.Executor.ID, fmt.Sprintf("%s_continue", b.Name))
		if spec.Edges[len(spec.Edges)-4].Condition == nil {
			spec.Edges[len(spec.Edges)-4].Condition = &ConditionSpec{Expr: continueExpr}
		}
	}
	if hasPresetEdge(b.ReplanEdge) {
		spec.Edges[len(spec.Edges)-3] = b.ReplanEdge.toEdge(b.Replanner.ID, b.Planner.ID, fmt.Sprintf("%s_replan", b.Name))
		if spec.Edges[len(spec.Edges)-3].Condition == nil {
			spec.Edges[len(spec.Edges)-3].Condition = &ConditionSpec{Expr: replanExpr}
		}
	}
	if hasPresetEdge(b.FinishEdge) {
		spec.Edges[len(spec.Edges)-2] = b.FinishEdge.toEdge(b.Replanner.ID, b.Finish.ID, fmt.Sprintf("%s_finish", b.Name))
		if spec.Edges[len(spec.Edges)-2].Condition == nil {
			spec.Edges[len(spec.Edges)-2].Condition = &ConditionSpec{Expr: finishExpr}
		}
	}

	spec.Blocks = append(spec.Blocks, BlockSpec{
		ID:               blockID,
		Kind:             BlockKindLoop,
		Nodes:            []NodeID{b.Executor.ID, b.Replanner.ID},
		EntryNodes:       []NodeID{b.Executor.ID},
		ExitNodes:        []NodeID{b.Finish.ID},
		RecoveryScopeRef: scopeID,
		PolicyRef:        b.BlockPolicyRef,
		Metadata:         copyAnyMap(b.BlockMetadata),
	})

	ensureCheckpointScope(&spec.Recovery, scopeID, blockID)
	ensureResumeEntry(&spec.Recovery, ResumeEntryID(fmt.Sprintf("%s.resume.executor", b.Name)), b.Executor.ID, EdgeID(fmt.Sprintf("%s_continue", b.Name)))
	ensureResumeEntry(&spec.Recovery, ResumeEntryID(fmt.Sprintf("%s.resume.planner", b.Name)), b.Planner.ID, EdgeID(fmt.Sprintf("%s_replan", b.Name)))

	for _, binding := range b.Bindings {
		spec.Bindings = append(spec.Bindings, copyBindingSpec(binding))
	}
	for _, policy := range b.Policies {
		spec.Policies = append(spec.Policies, copyPolicySpec(policy))
	}
	for _, boundary := range b.Boundaries {
		spec.Boundaries = append(spec.Boundaries, copyBoundarySpec(boundary))
	}
	for _, export := range b.Exports {
		spec.Exports = append(spec.Exports, copyExportSpec(export))
	}
	return spec, nil
}

func hasPresetEdge(edge PresetEdgeSpec) bool {
	return edge.ID != "" || edge.Kind != "" || edge.Priority != 0 || edge.Condition != nil || edge.Projection != nil || edge.ErrorMatch != nil || edge.Metadata != nil
}

func ensureCheckpointScope(recovery *RecoverySpec, scopeID CheckpointScopeID, blockID BlockID) {
	if recovery == nil || scopeID == "" {
		return
	}
	for i := range recovery.CheckpointScopes {
		if recovery.CheckpointScopes[i].ID == scopeID {
			if blockID != "" && len(recovery.CheckpointScopes[i].BlockIDs) == 0 {
				recovery.CheckpointScopes[i].BlockIDs = []BlockID{blockID}
			}
			return
		}
	}
	scope := CheckpointScopeSpec{ID: scopeID}
	if blockID != "" {
		scope.BlockIDs = []BlockID{blockID}
	}
	recovery.CheckpointScopes = append(recovery.CheckpointScopes, scope)
}

func ensureResumeEntry(recovery *RecoverySpec, entryID ResumeEntryID, nodeID NodeID, edgeID EdgeID) {
	if recovery == nil || entryID == "" {
		return
	}
	for _, entry := range recovery.ResumeEntries {
		if entry.ID == entryID {
			return
		}
	}
	recovery.ResumeEntries = append(recovery.ResumeEntries, ResumeEntrySpec{
		ID:     entryID,
		NodeID: nodeID,
		EdgeID: edgeID,
	})
}

type PresetEdgeSpec struct {
	ID         EdgeID
	Kind       EdgeKind
	Priority   int
	Condition  *ConditionSpec
	Projection *ProjectionSpec
	ErrorMatch *ErrorMatchSpec
	Metadata   map[string]any
}

func (p PresetEdgeSpec) toEdge(from, to NodeID, fallbackID string) EdgeSpec {
	edgeID := p.ID
	if edgeID == "" {
		edgeID = EdgeID(fallbackID)
	}
	kind := p.Kind
	if kind == "" {
		kind = EdgeKindControl
	}
	return EdgeSpec{
		ID:         edgeID,
		Kind:       kind,
		From:       from,
		To:         to,
		Priority:   p.Priority,
		Condition:  copyConditionSpec(p.Condition),
		Projection: copyProjectionSpec(p.Projection),
		ErrorMatch: copyErrorMatchSpec(p.ErrorMatch),
		Metadata:   copyAnyMap(p.Metadata),
	}
}

func normalizedVersion(version Version) Version {
	if strings.TrimSpace(string(version)) == "" {
		return Version("v1")
	}
	return version
}

func containsBinding(bindings []BindingSpec, ref BindingRef) bool {
	for _, binding := range bindings {
		if binding.Ref == ref {
			return true
		}
	}
	return false
}

func containsPolicy(policies []PolicySpec, ref PolicyRef) bool {
	for _, policy := range policies {
		if policy.Ref == ref {
			return true
		}
	}
	return false
}

func copyNodeSpec(spec NodeSpec) NodeSpec {
	return NodeSpec{
		ID:                  spec.ID,
		Kind:                spec.Kind,
		Name:                spec.Name,
		InputSchemaRef:      spec.InputSchemaRef,
		OutputSchemaRef:     spec.OutputSchemaRef,
		LocalStateSchemaRef: spec.LocalStateSchemaRef,
		HandlerRef:          spec.HandlerRef,
		BindingRef:          spec.BindingRef,
		PolicyRef:           spec.PolicyRef,
		Tags:                append([]string(nil), spec.Tags...),
		Metadata:            copyAnyMap(spec.Metadata),
	}
}

func copyStateSpec(spec StateSpec) StateSpec {
	return StateSpec{
		InputSchemaRef:      spec.InputSchemaRef,
		OutputSchemaRef:     spec.OutputSchemaRef,
		GraphStateSchemaRef: spec.GraphStateSchemaRef,
		NodeStateSchemas:    copySchemaRefMap(spec.NodeStateSchemas),
		BlockStateSchemas:   copyBlockSchemaRefMap(spec.BlockStateSchemas),
	}
}

func copyRecoverySpec(spec RecoverySpec) RecoverySpec {
	return RecoverySpec{
		InterruptPoints:      append([]InterruptPointSpec(nil), spec.InterruptPoints...),
		CheckpointScopes:     append([]CheckpointScopeSpec(nil), spec.CheckpointScopes...),
		PersistedStateFields: append([]string(nil), spec.PersistedStateFields...),
		ResumeEntries:        append([]ResumeEntrySpec(nil), spec.ResumeEntries...),
		ReplayPolicies:       append([]ReplayPolicySpec(nil), spec.ReplayPolicies...),
		RecoveryInvariants:   append([]RecoveryInvariantSpec(nil), spec.RecoveryInvariants...),
	}
}

func copyBindingSpec(spec BindingSpec) BindingSpec {
	return BindingSpec{Ref: spec.Ref, Kind: spec.Kind, Target: spec.Target, Config: copyAnyMap(spec.Config), Metadata: copyAnyMap(spec.Metadata)}
}

func copyPolicySpec(spec PolicySpec) PolicySpec {
	return PolicySpec{Ref: spec.Ref, Kind: spec.Kind, Config: copyAnyMap(spec.Config), Metadata: copyAnyMap(spec.Metadata)}
}

func copyBoundarySpec(spec BoundarySpec) BoundarySpec {
	return BoundarySpec{
		ID:               spec.ID,
		Name:             spec.Name,
		EntryNode:        spec.EntryNode,
		ExitNodes:        append([]NodeID(nil), spec.ExitNodes...),
		Projection:       copyBoundaryProjectionSpec(spec.Projection),
		Visibility:       copyBoundaryVisibilitySpec(spec.Visibility),
		RecoveryScopeRef: spec.RecoveryScopeRef,
		Ownership:        copyBoundaryOwnershipSpec(spec.Ownership),
		Metadata:         copyAnyMap(spec.Metadata),
	}
}

func copyExportSpec(spec ExportSpec) ExportSpec {
	return ExportSpec{Ref: spec.Ref, Name: spec.Name, Description: spec.Description, Metadata: copyAnyMap(spec.Metadata)}
}

func copyProjectionSpec(spec *ProjectionSpec) *ProjectionSpec {
	if spec == nil {
		return nil
	}
	return &ProjectionSpec{Reads: append([]string(nil), spec.Reads...), Writes: append([]string(nil), spec.Writes...), Mapping: copyStringMap(spec.Mapping), Mode: spec.Mode}
}

func copyBoundaryProjectionSpec(spec *BoundaryProjectionSpec) *BoundaryProjectionSpec {
	if spec == nil {
		return nil
	}
	return &BoundaryProjectionSpec{Reads: append([]string(nil), spec.Reads...), Writes: append([]string(nil), spec.Writes...), Mapping: copyStringMap(spec.Mapping), Mode: spec.Mode}
}

func copyBoundaryVisibilitySpec(spec *BoundaryVisibilitySpec) *BoundaryVisibilitySpec {
	if spec == nil {
		return nil
	}
	return &BoundaryVisibilitySpec{AllowedBindings: append([]BindingRef(nil), spec.AllowedBindings...), AllowedTools: append([]string(nil), spec.AllowedTools...), AllowedMemories: append([]string(nil), spec.AllowedMemories...)}
}

func copyBoundaryOwnershipSpec(spec *BoundaryOwnershipSpec) *BoundaryOwnershipSpec {
	if spec == nil {
		return nil
	}
	return &BoundaryOwnershipSpec{OwnerBlockID: spec.OwnerBlockID, OwnerNodeID: spec.OwnerNodeID, OwnerDomain: spec.OwnerDomain}
}

func copyConditionSpec(spec *ConditionSpec) *ConditionSpec {
	if spec == nil {
		return nil
	}
	return &ConditionSpec{Expr: spec.Expr}
}

func copyErrorMatchSpec(spec *ErrorMatchSpec) *ErrorMatchSpec {
	if spec == nil {
		return nil
	}
	return &ErrorMatchSpec{Codes: append([]string(nil), spec.Codes...), Expr: spec.Expr}
}

func copySchemaRefMap(input map[NodeID]SchemaRef) map[NodeID]SchemaRef {
	if len(input) == 0 {
		return nil
	}
	out := make(map[NodeID]SchemaRef, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func copyBlockSchemaRefMap(input map[BlockID]SchemaRef) map[BlockID]SchemaRef {
	if len(input) == 0 {
		return nil
	}
	out := make(map[BlockID]SchemaRef, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func copyStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func copyAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
