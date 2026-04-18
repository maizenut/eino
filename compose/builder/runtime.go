package builder

import (
	"context"
	"fmt"
)

// Runner executes a compiled graph with optional runtime overlay and explicit resume entry selection.
type Runner struct {
	resolver          NodeResolver
	conditionResolver ConditionResolver
	checkpointStore   *CheckpointStoreAdapter
}

func NewRunner(opts ...CompilerOption) *Runner {
	r := &Runner{}
	for _, opt := range opts {
		compiler := &Compiler{}
		opt(compiler)
		if compiler.conditionResolver != nil {
			r.conditionResolver = compiler.conditionResolver
		}
	}
	return r
}

func NewRunnerWithResolver(resolver NodeResolver, opts ...CompilerOption) *Runner {
	r := NewRunner(opts...)
	r.resolver = resolver
	return r
}

func NewRunnerWithCheckpointStore(resolver NodeResolver, store BuilderCheckPointStore, opts ...CompilerOption) *Runner {
	r := NewRunnerWithResolver(resolver, opts...)
	r.checkpointStore = &CheckpointStoreAdapter{Store: store}
	return r
}

type RunRequest struct {
	Input   map[string]any
	Overlay RuntimeOverlay
}

type ResumeRequest struct {
	Checkpoint map[string]any
	EntryID    ResumeEntryID
	Overlay    RuntimeOverlay
	ScopeID    CheckpointScopeID
}

func (r *Runner) Run(ctx context.Context, compiled *CompiledGraph, req RunRequest) (map[string]any, error) {
	if compiled == nil {
		return nil, fmt.Errorf("compiled graph is required")
	}
	compiled, err := r.withOverlay(ctx, compiled, req.Overlay)
	if err != nil {
		return nil, err
	}
	if compiled.Runnable == nil {
		return nil, fmt.Errorf("compiled graph is required")
	}

	input := cloneMap(req.Input)
	input = applyOverlay(input, req.Overlay)
	out, err := compiled.Runnable.Invoke(ctx, input)
	if err == nil {
		_ = r.persistCheckpoint(ctx, compiled, out)
	}
	return out, err
}

func (r *Runner) Resume(ctx context.Context, compiled *CompiledGraph, req ResumeRequest) (map[string]any, error) {
	if compiled == nil {
		return nil, fmt.Errorf("compiled graph is required")
	}
	compiled, err := r.withOverlay(ctx, compiled, req.Overlay)
	if err != nil {
		return nil, err
	}
	if compiled.Runnable == nil {
		return nil, fmt.Errorf("compiled graph is required")
	}

	if req.EntryID == "" {
		return nil, fmt.Errorf("resume entry id is required")
	}

	entry, ok := findResumeEntry(compiled.Plan, req.EntryID)
	if !ok {
		return nil, fmt.Errorf("resume entry %q not found", req.EntryID)
	}

	input := cloneMap(req.Checkpoint)
	if input == nil {
		input = map[string]any{}
		if restored, ok, err := r.loadCheckpoint(ctx, compiled); err == nil && ok {
			input = restored
		}
	}
	input = applyResumeEntry(input, entry)
	input = applyRecoveryScope(input, compiled.Plan, req.ScopeID)
	input = applyReplayPolicy(input, compiled.Plan, req.ScopeID)
	input = applyOverlay(input, req.Overlay)
	out, err := compiled.Runnable.Invoke(ctx, input)
	if err == nil {
		_ = r.persistCheckpoint(ctx, compiled, out)
	}
	return out, err
}

func (r *Runner) withOverlay(ctx context.Context, compiled *CompiledGraph, overlay RuntimeOverlay) (*CompiledGraph, error) {
	if compiled == nil {
		return nil, fmt.Errorf("compiled graph is required")
	}
	if len(overlay.BindingOverrides) == 0 && len(overlay.PolicyOverrides) == 0 {
		return compiled, nil
	}
	if r == nil || r.resolver == nil {
		return nil, fmt.Errorf("node resolver is required for binding/policy overrides")
	}
	compiler := NewCompiler(r.resolver)
	if r.conditionResolver != nil {
		compiler.conditionResolver = r.conditionResolver
	}
	return compiler.Compile(ctx, applyOverlayToPlan(compiled.Plan, overlay))
}

func (r *Runner) persistCheckpoint(ctx context.Context, compiled *CompiledGraph, output map[string]any) error {
	if r == nil || r.checkpointStore == nil || compiled == nil {
		return nil
	}
	key := string(compiled.Plan.Name)
	if key == "" {
		key = "compiled-graph"
	}
	return r.checkpointStore.Save(ctx, key, output)
}

func (r *Runner) loadCheckpoint(ctx context.Context, compiled *CompiledGraph) (map[string]any, bool, error) {
	if r == nil || r.checkpointStore == nil || compiled == nil {
		return nil, false, nil
	}
	key := string(compiled.Plan.Name)
	if key == "" {
		key = "compiled-graph"
	}
	return r.checkpointStore.Load(ctx, key)
}

func findResumeEntry(plan ExecutionPlan, id ResumeEntryID) (PlannedResumeEntry, bool) {
	for _, entry := range plan.State.ResumeEntries {
		if entry.ID == id {
			return entry, true
		}
	}
	return PlannedResumeEntry{}, false
}

func applyResumeEntry(input map[string]any, entry PlannedResumeEntry) map[string]any {
	out := cloneMap(input)
	if out == nil {
		out = map[string]any{}
	}
	resume := cloneMap(nestedMap(out, "_builder"))
	if resume == nil {
		resume = map[string]any{}
	}
	resume["resume_entry"] = string(entry.ID)
	if entry.NodeID != "" {
		resume["resume_node"] = string(entry.NodeID)
	}
	if entry.EdgeID != "" {
		resume["resume_edge"] = string(entry.EdgeID)
	}
	if entry.BlockID != "" {
		resume["resume_block"] = string(entry.BlockID)
	}
	out["_builder"] = resume
	return out
}

func applyRecoveryScope(input map[string]any, plan ExecutionPlan, scopeID CheckpointScopeID) map[string]any {
	if scopeID == "" {
		return input
	}
	out := cloneMap(input)
	if out == nil {
		out = map[string]any{}
	}
	builderMeta := cloneMap(nestedMap(out, "_builder"))
	if builderMeta == nil {
		builderMeta = map[string]any{}
	}
	if scope, ok := checkpointScopesByID(plan)[scopeID]; ok {
		builderMeta["recovery_scope"] = string(scopeID)
		builderMeta["persisted_fields"] = append([]string(nil), scope.PersistedFields...)
	}
	out["_builder"] = builderMeta
	return out
}

func applyReplayPolicy(input map[string]any, plan ExecutionPlan, scopeID CheckpointScopeID) map[string]any {
	if scopeID == "" {
		return input
	}
	out := cloneMap(input)
	if out == nil {
		out = map[string]any{}
	}
	builderMeta := cloneMap(nestedMap(out, "_builder"))
	if builderMeta == nil {
		builderMeta = map[string]any{}
	}
	if replayNodes, ok := replayPoliciesByScope(plan)[scopeID]; ok && len(replayNodes) > 0 {
		nodes := make([]string, 0, len(replayNodes))
		for _, nodeID := range replayNodes {
			nodes = append(nodes, string(nodeID))
		}
		builderMeta["replay_nodes"] = nodes
	}
	builderMeta["replay_scope"] = string(scopeID)
	out["_builder"] = builderMeta
	return out
}

func applyOverlay(input map[string]any, overlay RuntimeOverlay) map[string]any {
	out := cloneMap(input)
	if out == nil {
		out = map[string]any{}
	}
	if overlay.InstructionOverride == "" && len(overlay.Metadata) == 0 && len(overlay.DebugOptions) == 0 && len(overlay.BindingOverrides) == 0 && len(overlay.PolicyOverrides) == 0 {
		return out
	}

	builderMeta := cloneMap(nestedMap(out, "_builder"))
	if builderMeta == nil {
		builderMeta = map[string]any{}
	}
	if overlay.InstructionOverride != "" {
		builderMeta["instruction_override"] = overlay.InstructionOverride
	}
	if len(overlay.Metadata) > 0 {
		builderMeta["overlay_metadata"] = cloneMap(overlay.Metadata)
	}
	if len(overlay.DebugOptions) > 0 {
		builderMeta["debug_options"] = cloneMap(overlay.DebugOptions)
	}
	if len(overlay.BindingOverrides) > 0 {
		bindingOverrides := make(map[string]any, len(overlay.BindingOverrides))
		for from, to := range overlay.BindingOverrides {
			bindingOverrides[string(from)] = string(to)
		}
		builderMeta["binding_overrides"] = bindingOverrides
	}
	if len(overlay.PolicyOverrides) > 0 {
		policyOverrides := make(map[string]any, len(overlay.PolicyOverrides))
		for from, to := range overlay.PolicyOverrides {
			policyOverrides[string(from)] = string(to)
		}
		builderMeta["policy_overrides"] = policyOverrides
	}
	out["_builder"] = builderMeta
	return out
}

func nestedMap(input map[string]any, key string) map[string]any {
	if input == nil {
		return nil
	}
	if value, ok := input[key]; ok {
		if typed, ok := value.(map[string]any); ok {
			return typed
		}
	}
	return nil
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		if child, ok := v.(map[string]any); ok {
			out[k] = cloneMap(child)
			continue
		}
		out[k] = v
	}
	return out
}
