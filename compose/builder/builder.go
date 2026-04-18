package builder

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cloudwego/eino/compose"
)

type GraphBuilder interface {
	Normalize(spec GraphSpec) (GraphSpec, error)
	Validate(spec GraphSpec) error
	BuildPlan(spec GraphSpec) (ExecutionPlan, error)
}

type DefaultGraphBuilder struct{}

func New() *DefaultGraphBuilder {
	return &DefaultGraphBuilder{}
}

func NormalizeGraphSpec(spec GraphSpec) (GraphSpec, error) {
	if strings.TrimSpace(string(spec.Name)) == "" {
		return GraphSpec{}, fmt.Errorf("graph name is required")
	}

	out := spec
	out.Nodes = append([]NodeSpec(nil), spec.Nodes...)
	out.Edges = append([]EdgeSpec(nil), spec.Edges...)
	out.Blocks = append([]BlockSpec(nil), spec.Blocks...)
	out.Boundaries = append([]BoundarySpec(nil), spec.Boundaries...)
	out.Bindings = append([]BindingSpec(nil), spec.Bindings...)
	out.Policies = append([]PolicySpec(nil), spec.Policies...)
	out.Exports = append([]ExportSpec(nil), spec.Exports...)

	for i := range out.Edges {
		if out.Edges[i].ID == "" {
			out.Edges[i].ID = EdgeID(fmt.Sprintf("edge_%d", i+1))
		}
		if out.Edges[i].Kind == "" {
			switch {
			case out.Edges[i].Condition != nil:
				out.Edges[i].Kind = EdgeKindConditional
			case out.Edges[i].Projection != nil:
				out.Edges[i].Kind = EdgeKindProjection
			default:
				out.Edges[i].Kind = EdgeKindControl
			}
		}
		if out.Edges[i].Projection != nil && out.Edges[i].Projection.Mode == "" {
			out.Edges[i].Projection.Mode = ProjectionModeMap
		}
	}

	for i := range out.Boundaries {
		if out.Boundaries[i].Projection != nil && out.Boundaries[i].Projection.Mode == "" {
			out.Boundaries[i].Projection.Mode = ProjectionModeMap
		}
	}

	if out.State.NodeStateSchemas == nil {
		out.State.NodeStateSchemas = map[NodeID]SchemaRef{}
	}
	if out.State.BlockStateSchemas == nil {
		out.State.BlockStateSchemas = map[BlockID]SchemaRef{}
	}

	return out, nil
}

func ValidateGraphSpec(spec GraphSpec) error {
	if strings.TrimSpace(string(spec.Name)) == "" {
		return fmt.Errorf("graph name is required")
	}
	if len(spec.Nodes) == 0 {
		return fmt.Errorf("graph %q must contain at least one node", spec.Name)
	}

	nodeSet := make(map[NodeID]NodeSpec, len(spec.Nodes))
	for _, node := range spec.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node id is required")
		}
		if node.Kind == "" {
			return fmt.Errorf("node %q kind is required", node.ID)
		}
		if _, exists := nodeSet[node.ID]; exists {
			return fmt.Errorf("duplicate node id %q", node.ID)
		}
		nodeSet[node.ID] = node
	}

	bindingSet := make(map[BindingRef]BindingSpec, len(spec.Bindings))
	for _, binding := range spec.Bindings {
		if binding.Ref == "" {
			return fmt.Errorf("binding ref is required")
		}
		if _, exists := bindingSet[binding.Ref]; exists {
			return fmt.Errorf("duplicate binding ref %q", binding.Ref)
		}
		bindingSet[binding.Ref] = binding
	}

	policySet := make(map[PolicyRef]PolicySpec, len(spec.Policies))
	for _, policy := range spec.Policies {
		if policy.Ref == "" {
			return fmt.Errorf("policy ref is required")
		}
		if _, exists := policySet[policy.Ref]; exists {
			return fmt.Errorf("duplicate policy ref %q", policy.Ref)
		}
		policySet[policy.Ref] = policy
	}

	for _, node := range spec.Nodes {
		if node.BindingRef != "" {
			if _, ok := bindingSet[node.BindingRef]; !ok {
				return fmt.Errorf("node %q references unknown binding %q", node.ID, node.BindingRef)
			}
		}
		if node.PolicyRef != "" {
			if _, ok := policySet[node.PolicyRef]; !ok {
				return fmt.Errorf("node %q references unknown policy %q", node.ID, node.PolicyRef)
			}
		}
	}

	edgeSet := make(map[EdgeID]EdgeSpec, len(spec.Edges))
	routeConditions := map[NodeID]map[string]struct{}{}
	defaultRouteCount := map[NodeID]int{}
	for _, edge := range spec.Edges {
		if edge.ID == "" {
			return fmt.Errorf("edge id is required")
		}
		if _, exists := edgeSet[edge.ID]; exists {
			return fmt.Errorf("duplicate edge id %q", edge.ID)
		}
		edgeSet[edge.ID] = edge

		if !isBuiltinEndpoint(edge.From) {
			if _, ok := nodeSet[edge.From]; !ok {
				return fmt.Errorf("edge %q references unknown from node %q", edge.ID, edge.From)
			}
		}
		if !isBuiltinEndpoint(edge.To) {
			if _, ok := nodeSet[edge.To]; !ok {
				return fmt.Errorf("edge %q references unknown to node %q", edge.To, edge.To)
			}
		}

		switch edge.Kind {
		case EdgeKindConditional:
			if edge.Condition == nil || strings.TrimSpace(edge.Condition.Expr) == "" {
				defaultRouteCount[edge.From]++
				if defaultRouteCount[edge.From] > 1 {
					return fmt.Errorf("node %q defines more than one default conditional edge", edge.From)
				}
				continue
			}
			key := strings.TrimSpace(edge.Condition.Expr)
			if _, ok := routeConditions[edge.From]; !ok {
				routeConditions[edge.From] = map[string]struct{}{}
			}
			if _, exists := routeConditions[edge.From][key]; exists {
				return fmt.Errorf("node %q contains duplicate condition %q", edge.From, key)
			}
			routeConditions[edge.From][key] = struct{}{}
		case EdgeKindProjection:
			if edge.Projection == nil {
				return fmt.Errorf("projection edge %q requires projection spec", edge.ID)
			}
		}
	}

	blockSet := make(map[BlockID]BlockSpec, len(spec.Blocks))
	for _, block := range spec.Blocks {
		if block.ID == "" {
			return fmt.Errorf("block id is required")
		}
		if _, exists := blockSet[block.ID]; exists {
			return fmt.Errorf("duplicate block id %q", block.ID)
		}
		blockSet[block.ID] = block
		for _, nodeID := range append(append([]NodeID(nil), block.Nodes...), append(block.EntryNodes, block.ExitNodes...)...) {
			if _, ok := nodeSet[nodeID]; !ok {
				return fmt.Errorf("block %q references unknown node %q", block.ID, nodeID)
			}
		}
		if block.PolicyRef != "" {
			if _, ok := policySet[block.PolicyRef]; !ok {
				return fmt.Errorf("block %q references unknown policy %q", block.ID, block.PolicyRef)
			}
		}
	}

	boundarySet := make(map[BoundaryID]BoundarySpec, len(spec.Boundaries))
	for _, boundary := range spec.Boundaries {
		if boundary.ID == "" {
			return fmt.Errorf("boundary id is required")
		}
		if _, exists := boundarySet[boundary.ID]; exists {
			return fmt.Errorf("duplicate boundary id %q", boundary.ID)
		}
		boundarySet[boundary.ID] = boundary
		if _, ok := nodeSet[boundary.EntryNode]; !ok {
			return fmt.Errorf("boundary %q references unknown entry node %q", boundary.ID, boundary.EntryNode)
		}
		for _, exitNode := range boundary.ExitNodes {
			if _, ok := nodeSet[exitNode]; !ok {
				return fmt.Errorf("boundary %q references unknown exit node %q", boundary.ID, exitNode)
			}
		}
		if boundary.Visibility != nil {
			for _, bindingRef := range boundary.Visibility.AllowedBindings {
				if _, ok := bindingSet[bindingRef]; !ok {
					return fmt.Errorf("boundary %q references unknown binding %q", boundary.ID, bindingRef)
				}
			}
		}
		if boundary.Ownership != nil {
			if boundary.Ownership.OwnerBlockID != "" {
				if _, ok := blockSet[boundary.Ownership.OwnerBlockID]; !ok {
					return fmt.Errorf("boundary %q references unknown owner block %q", boundary.ID, boundary.Ownership.OwnerBlockID)
				}
			}
			if boundary.Ownership.OwnerNodeID != "" {
				if _, ok := nodeSet[boundary.Ownership.OwnerNodeID]; !ok {
					return fmt.Errorf("boundary %q references unknown owner node %q", boundary.ID, boundary.Ownership.OwnerNodeID)
				}
			}
		}
	}

	checkpointSet := make(map[CheckpointScopeID]CheckpointScopeSpec, len(spec.Recovery.CheckpointScopes))
	for _, checkpoint := range spec.Recovery.CheckpointScopes {
		if checkpoint.ID == "" {
			return fmt.Errorf("checkpoint scope id is required")
		}
		if _, exists := checkpointSet[checkpoint.ID]; exists {
			return fmt.Errorf("duplicate checkpoint scope id %q", checkpoint.ID)
		}
		checkpointSet[checkpoint.ID] = checkpoint
		for _, nodeID := range checkpoint.NodeIDs {
			if _, ok := nodeSet[nodeID]; !ok {
				return fmt.Errorf("checkpoint scope %q references unknown node %q", checkpoint.ID, nodeID)
			}
		}
		for _, blockID := range checkpoint.BlockIDs {
			if _, ok := blockSet[blockID]; !ok {
				return fmt.Errorf("checkpoint scope %q references unknown block %q", checkpoint.ID, blockID)
			}
		}
	}

	resumeSet := make(map[ResumeEntryID]ResumeEntrySpec, len(spec.Recovery.ResumeEntries))
	for _, entry := range spec.Recovery.ResumeEntries {
		if entry.ID == "" {
			return fmt.Errorf("resume entry id is required")
		}
		if _, exists := resumeSet[entry.ID]; exists {
			return fmt.Errorf("duplicate resume entry id %q", entry.ID)
		}
		resumeSet[entry.ID] = entry
		if entry.EdgeID != "" {
			if _, ok := edgeSet[entry.EdgeID]; !ok {
				return fmt.Errorf("resume entry %q references unknown edge %q", entry.ID, entry.EdgeID)
			}
		}
		if entry.NodeID != "" {
			if _, ok := nodeSet[entry.NodeID]; !ok {
				return fmt.Errorf("resume entry %q references unknown node %q", entry.ID, entry.NodeID)
			}
		}
		if entry.BlockID != "" {
			if _, ok := blockSet[entry.BlockID]; !ok {
				return fmt.Errorf("resume entry %q references unknown block %q", entry.ID, entry.BlockID)
			}
		}
	}

	for _, replay := range spec.Recovery.ReplayPolicies {
		if replay.ScopeID != "" {
			if _, ok := checkpointSet[replay.ScopeID]; !ok {
				return fmt.Errorf("replay policy references unknown scope %q", replay.ScopeID)
			}
		}
		for _, nodeID := range replay.ReplayNodeIDs {
			if _, ok := nodeSet[nodeID]; !ok {
				return fmt.Errorf("replay policy references unknown node %q", nodeID)
			}
		}
	}

	return nil
}

func BuildExecutionPlan(spec GraphSpec) (ExecutionPlan, error) {
	normalized, err := NormalizeGraphSpec(spec)
	if err != nil {
		return ExecutionPlan{}, err
	}
	if err := ValidateGraphSpec(normalized); err != nil {
		return ExecutionPlan{}, err
	}

	plan := ExecutionPlan{
		Name:    normalized.Name,
		Version: normalized.Version,
	}

	plan.Structural.Nodes = make([]PlannedNode, 0, len(normalized.Nodes))
	for _, node := range normalized.Nodes {
		plan.Structural.Nodes = append(plan.Structural.Nodes, PlannedNode{
			ID:                  node.ID,
			Kind:                node.Kind,
			Name:                node.Name,
			InputSchemaRef:      node.InputSchemaRef,
			OutputSchemaRef:     node.OutputSchemaRef,
			LocalStateSchemaRef: node.LocalStateSchemaRef,
			Handler:             node.HandlerRef,
			Binding:             node.BindingRef,
			Policy:              node.PolicyRef,
			Tags:                append([]string(nil), node.Tags...),
			Metadata:            node.Metadata,
		})
	}

	plan.Structural.Edges = make([]PlannedEdge, 0, len(normalized.Edges))
	for _, edge := range normalized.Edges {
		plan.Structural.Edges = append(plan.Structural.Edges, PlannedEdge{
			ID:         edge.ID,
			Kind:       edge.Kind,
			From:       edge.From,
			To:         edge.To,
			Priority:   edge.Priority,
			Condition:  edge.Condition,
			Projection: edge.Projection,
			ErrorMatch: edge.ErrorMatch,
			Metadata:   edge.Metadata,
		})
		if edge.Projection != nil {
			plan.State.Projections = append(plan.State.Projections, PlannedProjection{
				EdgeID:  edge.ID,
				Reads:   append([]string(nil), edge.Projection.Reads...),
				Writes:  append([]string(nil), edge.Projection.Writes...),
				Mapping: copyStringMap(edge.Projection.Mapping),
				Mode:    edge.Projection.Mode,
			})
		}
	}

	plan.Structural.Blocks = make([]PlannedBlock, 0, len(normalized.Blocks))
	for _, block := range normalized.Blocks {
		plan.Structural.Blocks = append(plan.Structural.Blocks, PlannedBlock{
			ID:               block.ID,
			Kind:             block.Kind,
			Nodes:            append([]NodeID(nil), block.Nodes...),
			EntryNodes:       append([]NodeID(nil), block.EntryNodes...),
			ExitNodes:        append([]NodeID(nil), block.ExitNodes...),
			RecoveryScopeRef: block.RecoveryScopeRef,
			PolicyRef:        block.PolicyRef,
			Metadata:         block.Metadata,
		})
	}

	plan.Structural.Boundaries = make([]PlannedBoundary, 0, len(normalized.Boundaries))
	for _, boundary := range normalized.Boundaries {
		plan.Structural.Boundaries = append(plan.Structural.Boundaries, PlannedBoundary{
			ID:               boundary.ID,
			Name:             boundary.Name,
			EntryNode:        boundary.EntryNode,
			ExitNodes:        append([]NodeID(nil), boundary.ExitNodes...),
			Projection:       boundary.Projection,
			Visibility:       boundary.Visibility,
			RecoveryScopeRef: boundary.RecoveryScopeRef,
			Ownership:        boundary.Ownership,
			Metadata:         boundary.Metadata,
		})
		if boundary.Projection != nil {
			plan.State.Projections = append(plan.State.Projections, PlannedProjection{
				BoundaryID: boundary.ID,
				Reads:      append([]string(nil), boundary.Projection.Reads...),
				Writes:     append([]string(nil), boundary.Projection.Writes...),
				Mapping:    copyStringMap(boundary.Projection.Mapping),
				Mode:       boundary.Projection.Mode,
			})
		}
	}

	plan.Structural.EntryNodes, plan.Structural.ExitNodes = inferEntryExit(normalized)

	plan.State.InputSchemaRef = normalized.State.InputSchemaRef
	plan.State.OutputSchemaRef = normalized.State.OutputSchemaRef
	plan.State.GraphStateSchemaRef = normalized.State.GraphStateSchemaRef
	plan.State.LocalStateSchemas = copySchemaRefMap(normalized.State.NodeStateSchemas)
	plan.State.BlockStateSchemas = copyBlockSchemaRefMap(normalized.State.BlockStateSchemas)
	for _, checkpoint := range normalized.Recovery.CheckpointScopes {
		plan.State.CheckpointScopes = append(plan.State.CheckpointScopes, PlannedCheckpointScope{
			ID:              checkpoint.ID,
			PersistedFields: append([]string(nil), checkpoint.PersistedFields...),
		})
	}
	for _, entry := range normalized.Recovery.ResumeEntries {
		plan.State.ResumeEntries = append(plan.State.ResumeEntries, PlannedResumeEntry{
			ID:      entry.ID,
			NodeID:  entry.NodeID,
			EdgeID:  entry.EdgeID,
			BlockID: entry.BlockID,
		})
	}
	for _, replay := range normalized.Recovery.ReplayPolicies {
		plan.State.ReplayPolicies = append(plan.State.ReplayPolicies, PlannedReplayPolicy{
			ScopeID:       replay.ScopeID,
			ReplayNodeIDs: append([]NodeID(nil), replay.ReplayNodeIDs...),
		})
	}

	plan.Runtime.NodeBindings = map[NodeID]BindingRef{}
	plan.Runtime.NodePolicies = map[NodeID]PolicyRef{}
	plan.Runtime.BoundaryBindings = map[BoundaryID][]BindingRef{}
	plan.Runtime.VisibilityPolicies = map[string]PolicyRef{}
	plan.Runtime.BindingCatalog = map[BindingRef]BindingSpec{}
	plan.Runtime.PolicyCatalog = map[PolicyRef]PolicySpec{}

	for _, binding := range normalized.Bindings {
		plan.Runtime.BindingCatalog[binding.Ref] = binding
		if binding.Kind == BindingKindInterceptor {
			plan.Runtime.InterceptorBindings = append(plan.Runtime.InterceptorBindings, binding.Ref)
		}
	}
	for _, policy := range normalized.Policies {
		plan.Runtime.PolicyCatalog[policy.Ref] = policy
	}
	for _, node := range normalized.Nodes {
		if node.BindingRef != "" {
			plan.Runtime.NodeBindings[node.ID] = node.BindingRef
		}
		if node.PolicyRef != "" {
			plan.Runtime.NodePolicies[node.ID] = node.PolicyRef
		}
	}
	for _, boundary := range normalized.Boundaries {
		if boundary.Visibility != nil {
			plan.Runtime.BoundaryBindings[boundary.ID] = append([]BindingRef(nil), boundary.Visibility.AllowedBindings...)
		}
	}

	plan.Artifacts = buildArtifact(plan)
	return plan, nil
}

func (b *DefaultGraphBuilder) Normalize(spec GraphSpec) (GraphSpec, error) {
	return NormalizeGraphSpec(spec)
}

func (b *DefaultGraphBuilder) Validate(spec GraphSpec) error {
	return ValidateGraphSpec(spec)
}

func (b *DefaultGraphBuilder) BuildPlan(spec GraphSpec) (ExecutionPlan, error) {
	return BuildExecutionPlan(spec)
}

func inferEntryExit(spec GraphSpec) ([]NodeID, []NodeID) {
	inDegree := map[NodeID]int{}
	outDegree := map[NodeID]int{}
	startTargets := map[NodeID]struct{}{}
	endSources := map[NodeID]struct{}{}

	for _, node := range spec.Nodes {
		inDegree[node.ID] = 0
		outDegree[node.ID] = 0
	}

	for _, edge := range spec.Edges {
		if edge.From == NodeID(compose.START) {
			startTargets[edge.To] = struct{}{}
		} else if !isBuiltinEndpoint(edge.From) {
			outDegree[edge.From]++
		}
		if edge.To == NodeID(compose.END) {
			endSources[edge.From] = struct{}{}
		} else if !isBuiltinEndpoint(edge.To) {
			inDegree[edge.To]++
		}
	}

	entries := make([]NodeID, 0)
	if len(startTargets) > 0 {
		for nodeID := range startTargets {
			entries = append(entries, nodeID)
		}
	} else {
		for nodeID, degree := range inDegree {
			if degree == 0 {
				entries = append(entries, nodeID)
			}
		}
	}

	exits := make([]NodeID, 0)
	if len(endSources) > 0 {
		for nodeID := range endSources {
			exits = append(exits, nodeID)
		}
	} else {
		for nodeID, degree := range outDegree {
			if degree == 0 {
				exits = append(exits, nodeID)
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i] < entries[j] })
	sort.Slice(exits, func(i, j int) bool { return exits[i] < exits[j] })
	return entries, exits
}

func buildArtifact(plan ExecutionPlan) BuildArtifact {
	artifact := BuildArtifact{
		NodeViews:     make([]ArtifactNodeView, 0, len(plan.Structural.Nodes)),
		EdgeViews:     make([]ArtifactEdgeView, 0, len(plan.Structural.Edges)),
		BlockViews:    make([]ArtifactBlockView, 0, len(plan.Structural.Blocks)),
		BoundaryViews: make([]ArtifactBoundaryView, 0, len(plan.Structural.Boundaries)),
		StateViews: []ArtifactStateView{
			{Name: "input", SchemaRef: plan.State.InputSchemaRef},
			{Name: "output", SchemaRef: plan.State.OutputSchemaRef},
			{Name: "graph", SchemaRef: plan.State.GraphStateSchemaRef},
		},
		ResumeViews: make([]ArtifactResumeView, 0, len(plan.State.ResumeEntries)),
	}

	for _, node := range plan.Structural.Nodes {
		artifact.NodeViews = append(artifact.NodeViews, ArtifactNodeView{ID: node.ID, Kind: node.Kind, Name: node.Name})
	}
	for _, edge := range plan.Structural.Edges {
		artifact.EdgeViews = append(artifact.EdgeViews, ArtifactEdgeView{ID: edge.ID, Kind: edge.Kind, From: edge.From, To: edge.To})
	}
	for _, block := range plan.Structural.Blocks {
		artifact.BlockViews = append(artifact.BlockViews, ArtifactBlockView{ID: block.ID, Kind: block.Kind})
	}
	for _, boundary := range plan.Structural.Boundaries {
		artifact.BoundaryViews = append(artifact.BoundaryViews, ArtifactBoundaryView{ID: boundary.ID, EntryNode: boundary.EntryNode})
	}
	for _, entry := range plan.State.ResumeEntries {
		artifact.ResumeViews = append(artifact.ResumeViews, ArtifactResumeView{ID: entry.ID, NodeID: entry.NodeID, EdgeID: entry.EdgeID})
	}
	return artifact
}

func isBuiltinEndpoint(nodeID NodeID) bool {
	return nodeID == NodeID(compose.START) || nodeID == NodeID(compose.END)
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copySchemaRefMap(in map[NodeID]SchemaRef) map[NodeID]SchemaRef {
	if len(in) == 0 {
		return map[NodeID]SchemaRef{}
	}
	out := make(map[NodeID]SchemaRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyBlockSchemaRefMap(in map[BlockID]SchemaRef) map[BlockID]SchemaRef {
	if len(in) == 0 {
		return map[BlockID]SchemaRef{}
	}
	out := make(map[BlockID]SchemaRef, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
