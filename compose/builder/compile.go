package builder

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
)

type ConditionFunc func(context.Context, map[string]any) (bool, error)

type NodeResolver interface {
	ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error)
}

type ConditionResolver interface {
	ResolveCondition(ctx context.Context, edge PlannedEdge) (ConditionFunc, error)
}

type Compiler struct {
	resolver          NodeResolver
	conditionResolver ConditionResolver
}

type CompilerOption func(*Compiler)

func WithConditionResolver(resolver ConditionResolver) CompilerOption {
	return func(c *Compiler) {
		c.conditionResolver = resolver
	}
}

func NewCompiler(resolver NodeResolver, opts ...CompilerOption) *Compiler {
	c := &Compiler{resolver: resolver}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type CompiledGraph struct {
	Plan     ExecutionPlan
	Graph    *compose.Graph[map[string]any, map[string]any]
	Runnable compose.Runnable[map[string]any, map[string]any]
}

func (c *Compiler) Compile(ctx context.Context, plan ExecutionPlan, opts ...compose.GraphCompileOption) (*CompiledGraph, error) {
	if c == nil || c.resolver == nil {
		return nil, fmt.Errorf("node resolver is required")
	}

	resolver := buildRuntimeResolverPipeline(plan, c.resolver)
	graph := compose.NewGraph[map[string]any, map[string]any]()
	if err := validateRecoveryScopes(plan); err != nil {
		return nil, err
	}
	if err := validateBlockKinds(plan); err != nil {
		return nil, err
	}
	blockGraph, err := blockGraphMetadata(plan)
	if err != nil {
		return nil, err
	}
	blockKinds := blockKindsByNode(plan)
	staticInputs := staticInputsFromPlan(plan)
	subgraphProjections := subgraphProjectionByEntry(plan)
	subgraphVisibility := subgraphVisibilityByEntry(plan)
	for _, node := range plan.Structural.Nodes {
		binding := bindingByRef(plan, node.Binding)
		policy := policyByRef(plan, node.Policy)
		if err := validateSubGraphBinding(node, subgraphVisibility[node.ID]); err != nil {
			return nil, err
		}
		resolved, err := resolver.ResolveNode(ctx, node, binding, policy)
		if err != nil {
			return nil, fmt.Errorf("resolve node %q: %w", node.ID, err)
		}
		resolved = wrapWithStaticInput(node.ID, resolved, staticInputs[node.ID])
		resolved = wrapWithNodeMetadata(node.ID, resolved, node.Metadata)
		resolved = wrapWithSubGraphProjection(node.ID, resolved, subgraphProjections[node.ID])
		if err := addResolvedNode(graph, node, resolved); err != nil {
			return nil, fmt.Errorf("add node %q: %w", node.ID, err)
		}
	}

	conditional := make(map[NodeID][]PlannedEdge)
	for _, edge := range plan.Structural.Edges {
		if err := validateBlockEdge(edge, blockGraph); err != nil {
			return nil, err
		}
		if err := validateBlockKindEdge(edge, blockGraph, blockKinds); err != nil {
			return nil, err
		}
		if err := validateWorkflowInputEdge(edge); err != nil {
			return nil, err
		}
		if edge.Kind == EdgeKindConditional {
			conditional[edge.From] = append(conditional[edge.From], edge)
			continue
		}
		if err := addPlannedEdge(graph, edge); err != nil {
			return nil, fmt.Errorf("add edge %q: %w", edge.ID, err)
		}
	}

	for from, edges := range conditional {
		sort.SliceStable(edges, func(i, j int) bool {
			if edges[i].Priority == edges[j].Priority {
				return edges[i].ID < edges[j].ID
			}
			return edges[i].Priority < edges[j].Priority
		})
		branch, err := c.buildBranch(ctx, edges)
		if err != nil {
			return nil, fmt.Errorf("build branch for %q: %w", from, err)
		}
		if err := graph.AddBranch(string(from), branch); err != nil {
			return nil, fmt.Errorf("add branch %q: %w", from, err)
		}
	}

	runnable, err := graph.Compile(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &CompiledGraph{
		Plan:     plan,
		Graph:    graph,
		Runnable: runnable,
	}, nil
}

func CompileWithOverlay(ctx context.Context, resolver NodeResolver, plan ExecutionPlan, overlay RuntimeOverlay, opts ...CompilerOption) (*CompiledGraph, error) {
	compiler := NewCompiler(resolver, opts...)
	return compiler.Compile(ctx, applyOverlayToPlan(plan, overlay))
}

func applyOverlayToPlan(plan ExecutionPlan, overlay RuntimeOverlay) ExecutionPlan {
	if len(overlay.BindingOverrides) == 0 && len(overlay.PolicyOverrides) == 0 {
		return plan
	}

	out := plan
	out.Structural.Nodes = append([]PlannedNode(nil), plan.Structural.Nodes...)
	out.Runtime.NodeBindings = cloneBindingRefMap(plan.Runtime.NodeBindings)
	out.Runtime.NodePolicies = clonePolicyRefMap(plan.Runtime.NodePolicies)
	out.Runtime.BindingCatalog = cloneBindingCatalog(plan.Runtime.BindingCatalog)
	out.Runtime.PolicyCatalog = clonePolicyCatalog(plan.Runtime.PolicyCatalog)

	for i := range out.Structural.Nodes {
		node := &out.Structural.Nodes[i]
		if override, ok := overlay.BindingOverrides[node.Binding]; ok {
			node.Binding = override
			out.Runtime.NodeBindings[node.ID] = override
		}
		if override, ok := overlay.PolicyOverrides[node.Policy]; ok {
			node.Policy = override
			out.Runtime.NodePolicies[node.ID] = override
		}
	}

	return out
}

func cloneBindingRefMap(input map[NodeID]BindingRef) map[NodeID]BindingRef {
	if input == nil {
		return map[NodeID]BindingRef{}
	}
	out := make(map[NodeID]BindingRef, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func clonePolicyRefMap(input map[NodeID]PolicyRef) map[NodeID]PolicyRef {
	if input == nil {
		return map[NodeID]PolicyRef{}
	}
	out := make(map[NodeID]PolicyRef, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func cloneBindingCatalog(input map[BindingRef]BindingSpec) map[BindingRef]BindingSpec {
	if input == nil {
		return map[BindingRef]BindingSpec{}
	}
	out := make(map[BindingRef]BindingSpec, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func clonePolicyCatalog(input map[PolicyRef]PolicySpec) map[PolicyRef]PolicySpec {
	if input == nil {
		return map[PolicyRef]PolicySpec{}
	}
	out := make(map[PolicyRef]PolicySpec, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func blockGraphMetadata(plan ExecutionPlan) (map[NodeID]BlockID, error) {
	membership := make(map[NodeID]BlockID)
	for _, block := range plan.Structural.Blocks {
		for _, nodeID := range block.Nodes {
			if existing, ok := membership[nodeID]; ok && existing != block.ID {
				return nil, fmt.Errorf("node %q belongs to multiple blocks: %q and %q", nodeID, existing, block.ID)
			}
			membership[nodeID] = block.ID
		}
	}
	return membership, nil
}

func validateBlockEdge(edge PlannedEdge, membership map[NodeID]BlockID) error {
	fromBlock, fromOk := membership[edge.From]
	toBlock, toOk := membership[edge.To]
	if !fromOk || !toOk || fromBlock == toBlock {
		return nil
	}
	if edge.Kind == EdgeKindConditional {
		return nil
	}
	if edge.Kind == EdgeKindControl || edge.Kind == EdgeKindProjection || edge.Kind == EdgeKindData {
		return nil
	}
	return fmt.Errorf("edge %q crosses blocks %q -> %q with unsupported kind %q", edge.ID, fromBlock, toBlock, edge.Kind)
}

func (c *Compiler) buildBranch(ctx context.Context, edges []PlannedEdge) (*compose.GraphBranch, error) {
	predicates := make([]compiledPredicate, 0, len(edges))
	endNodes := make(map[string]bool, len(edges))
	for _, edge := range edges {
		condition, err := c.resolveCondition(ctx, edge)
		if err != nil {
			return nil, err
		}
		predicates = append(predicates, compiledPredicate{
			target: string(edge.To),
			match:  condition,
		})
		endNodes[string(edge.To)] = true
	}

	branch := compose.NewGraphBranch(func(ctx context.Context, input map[string]any) (string, error) {
		for _, predicate := range predicates {
			ok, err := predicate.match(ctx, input)
			if err != nil {
				return "", err
			}
			if ok {
				return predicate.target, nil
			}
		}
		return "", fmt.Errorf("no conditional edge matched")
	}, endNodes)
	return branch, nil
}

type compiledPredicate struct {
	target string
	match  ConditionFunc
}

func (c *Compiler) resolveCondition(ctx context.Context, edge PlannedEdge) (ConditionFunc, error) {
	if c.conditionResolver != nil {
		condition, err := c.conditionResolver.ResolveCondition(ctx, edge)
		if err != nil {
			return nil, err
		}
		if condition != nil {
			return condition, nil
		}
	}
	return builtinCondition(edge.Condition), nil
}

func addResolvedNode(graph *compose.Graph[map[string]any, map[string]any], node PlannedNode, resolved any) error {
	opts := nodeOptions(node)

	switch typed := resolved.(type) {
	case nil:
		if node.Kind == NodeKindBranch || node.Kind == NodeKindJoin {
			return graph.AddPassthroughNode(string(node.ID), opts...)
		}
		return fmt.Errorf("resolved node is nil")
	case *compose.Lambda:
		return graph.AddLambdaNode(string(node.ID), typed, opts...)
	case compose.AnyGraph:
		return graph.AddGraphNode(string(node.ID), typed, opts...)
	case func(context.Context, map[string]any) (map[string]any, error):
		return graph.AddLambdaNode(string(node.ID), compose.InvokableLambda(typed), opts...)
	case prompt.ChatTemplate:
		return graph.AddChatTemplateNode(string(node.ID), typed, opts...)
	case prompt.AgenticChatTemplate:
		return graph.AddAgenticChatTemplateNode(string(node.ID), typed, opts...)
	case model.BaseChatModel:
		return graph.AddChatModelNode(string(node.ID), typed, opts...)
	case model.AgenticModel:
		return graph.AddAgenticModelNode(string(node.ID), typed, opts...)
	case *compose.ToolsNode:
		return graph.AddToolsNode(string(node.ID), typed, opts...)
	case *compose.AgenticToolsNode:
		return graph.AddAgenticToolsNode(string(node.ID), typed, opts...)
	case embedding.Embedder:
		return graph.AddEmbeddingNode(string(node.ID), typed, opts...)
	case indexer.Indexer:
		return graph.AddIndexerNode(string(node.ID), typed, opts...)
	case retriever.Retriever:
		return graph.AddRetrieverNode(string(node.ID), typed, opts...)
	case document.Loader:
		return graph.AddLoaderNode(string(node.ID), typed, opts...)
	case document.Transformer:
		return graph.AddDocumentTransformerNode(string(node.ID), typed, opts...)
	default:
		switch node.Kind {
		case NodeKindBranch, NodeKindJoin:
			return graph.AddPassthroughNode(string(node.ID), opts...)
		case NodeKindSubgraph:
			return fmt.Errorf("resolved subgraph node must be compose.AnyGraph, got %T", resolved)
		default:
			return fmt.Errorf("unsupported resolved node type %T", resolved)
		}
	}
}

func addPlannedEdge(graph *compose.Graph[map[string]any, map[string]any], edge PlannedEdge) error {
	mappings := edgeProjectionMappings(edge.Projection)

	switch edge.Kind {
	case EdgeKindControl, EdgeKindError, EdgeKindResume:
		if len(mappings) > 0 {
			return graph.AddEdgeWithOptions(string(edge.From), string(edge.To), false, false, mappings...)
		}
		return graph.AddEdge(string(edge.From), string(edge.To))
	case EdgeKindData:
		return graph.AddEdgeWithOptions(string(edge.From), string(edge.To), true, false, mappings...)
	case EdgeKindProjection:
		return graph.AddEdgeWithOptions(string(edge.From), string(edge.To), false, false, mappings...)
	default:
		if len(mappings) > 0 {
			return graph.AddEdgeWithOptions(string(edge.From), string(edge.To), false, false, mappings...)
		}
		return graph.AddEdge(string(edge.From), string(edge.To))
	}
}

func edgeProjectionMappings(projection *ProjectionSpec) []*compose.FieldMapping {
	if projection == nil || len(projection.Mapping) == 0 {
		return nil
	}
	keys := make([]string, 0, len(projection.Mapping))
	for key := range projection.Mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	mappings := make([]*compose.FieldMapping, 0, len(keys))
	for _, from := range keys {
		to := projection.Mapping[from]
		mappings = append(mappings, compose.MapFieldPaths(splitFieldPath(from), splitFieldPath(to)))
	}
	return mappings
}

func splitFieldPath(path string) compose.FieldPath {
	if strings.TrimSpace(path) == "" {
		return compose.FieldPath{}
	}
	parts := strings.Split(path, ".")
	out := make(compose.FieldPath, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func nodeOptions(node PlannedNode) []compose.GraphAddNodeOpt {
	if node.Name == "" {
		return nil
	}
	return []compose.GraphAddNodeOpt{compose.WithNodeName(node.Name)}
}

func builtinCondition(condition *ConditionSpec) ConditionFunc {
	expr := ""
	if condition != nil {
		expr = strings.TrimSpace(condition.Expr)
	}
	if expr == "" {
		return func(ctx context.Context, input map[string]any) (bool, error) {
			_ = ctx
			return true, nil
		}
	}
	if strings.HasPrefix(expr, "resume:") {
		entryID := strings.TrimSpace(strings.TrimPrefix(expr, "resume:"))
		return func(ctx context.Context, input map[string]any) (bool, error) {
			_ = ctx
			builderMeta := nestedMap(input, "_builder")
			if builderMeta == nil {
				return false, nil
			}
			return fmt.Sprint(builderMeta["resume_entry"]) == entryID, nil
		}
	}
	if strings.HasPrefix(expr, "!") {
		path := strings.TrimSpace(strings.TrimPrefix(expr, "!"))
		return func(ctx context.Context, input map[string]any) (bool, error) {
			_ = ctx
			return !truthy(lookupPath(input, path)), nil
		}
	}
	if idx := strings.Index(expr, "=="); idx >= 0 {
		path := strings.TrimSpace(expr[:idx])
		want := strings.TrimSpace(expr[idx+2:])
		return func(ctx context.Context, input map[string]any) (bool, error) {
			_ = ctx
			return fmt.Sprint(lookupPath(input, path)) == want, nil
		}
	}
	if idx := strings.Index(expr, "="); idx >= 0 {
		path := strings.TrimSpace(expr[:idx])
		want := strings.TrimSpace(expr[idx+1:])
		return func(ctx context.Context, input map[string]any) (bool, error) {
			_ = ctx
			return fmt.Sprint(lookupPath(input, path)) == want, nil
		}
	}
	return func(ctx context.Context, input map[string]any) (bool, error) {
		_ = ctx
		return truthy(lookupPath(input, expr)), nil
	}
}

func lookupPath(input map[string]any, path string) any {
	current := any(input)
	for _, key := range strings.Split(path, ".") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != "" && typed != "false" && typed != "0"
	case int:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case float32:
		return typed != 0
	case float64:
		return typed != 0
	case jsonNumber:
		f, err := strconv.ParseFloat(string(typed), 64)
		return err == nil && f != 0
	default:
		return true
	}
}

type jsonNumber string

func bindingByRef(plan ExecutionPlan, ref BindingRef) *BindingSpec {
	if ref == "" {
		return nil
	}
	binding, ok := plan.Runtime.BindingCatalog[ref]
	if !ok {
		return nil
	}
	return &binding
}

func policyByRef(plan ExecutionPlan, ref PolicyRef) *PolicySpec {
	if ref == "" {
		return nil
	}
	policy, ok := plan.Runtime.PolicyCatalog[ref]
	if !ok {
		return nil
	}
	return &policy
}

func buildRuntimeResolverPipeline(_ ExecutionPlan, resolver NodeResolver) NodeResolver {
	return resolver
}

func validateRecoveryScopes(_ ExecutionPlan) error {
	return nil
}

func validateBlockKinds(_ ExecutionPlan) error {
	return nil
}

func blockKindsByNode(plan ExecutionPlan) map[NodeID]BlockKind {
	out := make(map[NodeID]BlockKind)
	for _, block := range plan.Structural.Blocks {
		for _, nodeID := range block.Nodes {
			out[nodeID] = block.Kind
		}
	}
	return out
}

func staticInputsFromPlan(_ ExecutionPlan) map[NodeID]map[string]any {
	return map[NodeID]map[string]any{}
}

func subgraphProjectionByEntry(plan ExecutionPlan) map[NodeID]*SubGraphProjectionSpec {
	out := make(map[NodeID]*SubGraphProjectionSpec)
	for _, subgraph := range plan.Structural.Boundaries {
		if subgraph.EntryNode != "" {
			projection := subgraph.Projection
			out[subgraph.EntryNode] = projection
		}
	}
	return out
}

func subgraphVisibilityByEntry(plan ExecutionPlan) map[NodeID]*SubGraphVisibilitySpec {
	out := make(map[NodeID]*SubGraphVisibilitySpec)
	for _, subgraph := range plan.Structural.Boundaries {
		if subgraph.EntryNode != "" {
			visibility := subgraph.Visibility
			out[subgraph.EntryNode] = visibility
		}
	}
	return out
}

func validateSubGraphBinding(_ PlannedNode, _ *SubGraphVisibilitySpec) error {
	return nil
}

func wrapWithStaticInput(_ NodeID, resolved any, _ map[string]any) any {
	return resolved
}

func wrapWithNodeMetadata(_ NodeID, resolved any, _ map[string]any) any {
	return resolved
}

func wrapWithSubGraphProjection(_ NodeID, resolved any, _ *SubGraphProjectionSpec) any {
	return resolved
}

func validateBlockKindEdge(_ PlannedEdge, _ map[NodeID]BlockID, _ map[NodeID]BlockKind) error {
	return nil
}

func validateWorkflowInputEdge(_ PlannedEdge) error {
	return nil
}

func nestedMap(input map[string]any, path ...string) map[string]any {
	current := input
	for _, key := range path {
		if current == nil {
			return nil
		}
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return current
}
