package builder

import (
	"context"
	"fmt"
	"testing"
)

type staticResolver struct {
	nodes map[HandlerRef]any
}

func (r *staticResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	_ = ctx
	_ = binding
	_ = policy
	return r.nodes[node.Handler], nil
}

type captureResolver struct {
	bindings map[NodeID]BindingRef
	policies map[NodeID]PolicyRef
}

type memoryStore struct {
	data map[string][]byte
}

func (m *memoryStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	_ = ctx
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	value, ok := m.data[checkPointID]
	return value, ok, nil
}

func (m *memoryStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	_ = ctx
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[checkPointID] = append([]byte(nil), checkPoint...)
	return nil
}

func (r *captureResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	_ = ctx
	if r.bindings == nil {
		r.bindings = map[NodeID]BindingRef{}
	}
	if r.policies == nil {
		r.policies = map[NodeID]PolicyRef{}
	}
	if binding != nil {
		r.bindings[node.ID] = binding.Ref
	}
	if policy != nil {
		r.policies[node.ID] = policy.Ref
	}
	return func(ctx context.Context, input map[string]any) (map[string]any, error) {
		_ = ctx
		out := cloneMap(input)
		if out == nil {
			out = map[string]any{}
		}
		if binding != nil {
			out["binding_ref"] = string(binding.Ref)
		}
		if policy != nil {
			out["policy_ref"] = string(policy.Ref)
		}
		return out, nil
	}, nil
}

func TestBuildExecutionPlan(t *testing.T) {
	spec := GraphSpec{
		Name: "react-like",
		Nodes: []NodeSpec{
			{ID: "prepare", Kind: NodeKindLambda, HandlerRef: "prepare"},
			{ID: "finish", Kind: NodeKindLambda, HandlerRef: "finish", BindingRef: "binding.default", PolicyRef: "policy.exec"},
		},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "prepare"},
			{From: "prepare", To: "finish", Projection: &ProjectionSpec{Mapping: map[string]string{"normalized": "ok"}}},
			{From: "finish", To: NodeID("end")},
		},
		State: StateSpec{
			InputSchemaRef:      "schema.input",
			OutputSchemaRef:     "schema.output",
			GraphStateSchemaRef: "schema.graph",
		},
		Bindings: []BindingSpec{{
			Ref:    "binding.default",
			Kind:   BindingKindService,
			Target: "service.default",
		}},
		Policies: []PolicySpec{{
			Ref:  "policy.exec",
			Kind: PolicyKindExecution,
		}},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	if len(plan.Structural.EntryNodes) != 1 || plan.Structural.EntryNodes[0] != "prepare" {
		t.Fatalf("entry nodes = %#v", plan.Structural.EntryNodes)
	}
	if len(plan.Structural.ExitNodes) != 1 || plan.Structural.ExitNodes[0] != "finish" {
		t.Fatalf("exit nodes = %#v", plan.Structural.ExitNodes)
	}
	if len(plan.State.Projections) != 1 {
		t.Fatalf("projection count = %d, want 1", len(plan.State.Projections))
	}
	if got := plan.Runtime.NodeBindings["finish"]; got != "binding.default" {
		t.Fatalf("node binding = %q, want binding.default", got)
	}
	if got := plan.Runtime.NodePolicies["finish"]; got != "policy.exec" {
		t.Fatalf("node policy = %q, want policy.exec", got)
	}
}

func TestCompilerCompile(t *testing.T) {
	spec := GraphSpec{
		Name: "conditional-graph",
		Nodes: []NodeSpec{
			{ID: "prepare", Kind: NodeKindLambda, HandlerRef: "prepare"},
			{ID: "route", Kind: NodeKindLambda, HandlerRef: "route"},
			{ID: "accept", Kind: NodeKindLambda, HandlerRef: "accept"},
			{ID: "reject", Kind: NodeKindLambda, HandlerRef: "reject"},
		},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "prepare"},
			{From: "prepare", To: "route", Projection: &ProjectionSpec{Mapping: map[string]string{"approved": "approved"}}},
			{From: "route", To: "accept", Kind: EdgeKindConditional, Condition: &ConditionSpec{Expr: "approved=true"}},
			{From: "route", To: "reject", Kind: EdgeKindConditional},
			{From: "accept", To: NodeID("end")},
			{From: "reject", To: NodeID("end")},
		},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}

	resolver := &staticResolver{
		nodes: map[HandlerRef]any{
			"prepare": func(ctx context.Context, input map[string]any) (map[string]any, error) {
				_ = ctx
				out := map[string]any{}
				for k, v := range input {
					out[k] = v
				}
				out["approved"] = true
				return out, nil
			},
			"route": func(ctx context.Context, input map[string]any) (map[string]any, error) {
				_ = ctx
				return input, nil
			},
			"accept": func(ctx context.Context, input map[string]any) (map[string]any, error) {
				_ = ctx
				input["decision"] = "accept"
				return input, nil
			},
			"reject": func(ctx context.Context, input map[string]any) (map[string]any, error) {
				_ = ctx
				input["decision"] = "reject"
				return input, nil
			},
		},
	}

	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{"seed": "x"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got := out["decision"]; got != "accept" {
		t.Fatalf("decision = %#v, want accept", got)
	}
}

func TestRunnerRunAppliesOverlay(t *testing.T) {
	spec := GraphSpec{
		Name: "overlay-run",
		Nodes: []NodeSpec{
			{ID: "inspect", Kind: NodeKindLambda, HandlerRef: "inspect"},
		},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "inspect"},
			{From: "inspect", To: NodeID("end")},
		},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}

	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"inspect": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			meta, _ := input["_builder"].(map[string]any)
			return map[string]any{
				"instruction": meta["instruction_override"],
				"trace":       meta["overlay_metadata"].(map[string]any)["trace_id"],
			}, nil
		},
	}}

	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	runner := NewRunner()
	out, err := runner.Run(context.Background(), compiled, RunRequest{
		Input: map[string]any{"seed": "x"},
		Overlay: RuntimeOverlay{
			InstructionOverride: "prefer concise answer",
			Metadata:            map[string]any{"trace_id": "trace-1"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out["instruction"] != "prefer concise answer" {
		t.Fatalf("instruction = %#v", out["instruction"])
	}
	if out["trace"] != "trace-1" {
		t.Fatalf("trace = %#v", out["trace"])
	}
}

func TestRunnerResumeUsesResumeEntry(t *testing.T) {
	spec := GraphSpec{
		Name: "resume-graph",
		Nodes: []NodeSpec{
			{ID: "resume_gate", Kind: NodeKindLambda, HandlerRef: "resume_gate"},
			{ID: "accept", Kind: NodeKindLambda, HandlerRef: "accept"},
			{ID: "reject", Kind: NodeKindLambda, HandlerRef: "reject"},
		},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "resume_gate"},
			{From: "resume_gate", To: "accept", Kind: EdgeKindConditional, Condition: &ConditionSpec{Expr: "resume:approved"}},
			{From: "resume_gate", To: "reject", Kind: EdgeKindConditional},
			{From: "accept", To: NodeID("end")},
			{From: "reject", To: NodeID("end")},
		},
		Recovery: RecoverySpec{
			ResumeEntries: []ResumeEntrySpec{{ID: "approved", NodeID: "accept"}},
		},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}

	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"resume_gate": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return input, nil
		},
		"accept": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			input["decision"] = "accept"
			return input, nil
		},
		"reject": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			input["decision"] = "reject"
			return input, nil
		},
	}}

	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	runner := NewRunner()
	out, err := runner.Resume(context.Background(), compiled, ResumeRequest{
		Checkpoint: map[string]any{"seed": "checkpoint"},
		EntryID:    "approved",
	})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if out["decision"] != "accept" {
		t.Fatalf("decision = %#v", out["decision"])
	}
}

func TestCompilerConsumesBlocks(t *testing.T) {
	spec := GraphSpec{
		Name: "block-aware",
		Nodes: []NodeSpec{
			{ID: "prepare", Kind: NodeKindLambda, HandlerRef: "prepare"},
			{ID: "finish", Kind: NodeKindLambda, HandlerRef: "finish"},
		},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "prepare"},
			{From: "prepare", To: "finish", Kind: EdgeKindData},
			{From: "finish", To: NodeID("end")},
		},
		Blocks: []BlockSpec{
			{ID: "workflow.prepare", Kind: BlockKindSequence, Nodes: []NodeID{"prepare"}, EntryNodes: []NodeID{"prepare"}, ExitNodes: []NodeID{"prepare"}},
			{ID: "workflow.finish", Kind: BlockKindSequence, Nodes: []NodeID{"finish"}, EntryNodes: []NodeID{"finish"}, ExitNodes: []NodeID{"finish"}},
		},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}

	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"prepare": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			input["prepared"] = true
			return input, nil
		},
		"finish": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			input["finished"] = input["prepared"]
			return input, nil
		},
	}}

	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["finished"] != true {
		t.Fatalf("output = %#v", out)
	}
}

func TestCompilerRejectsOverlappingBlocks(t *testing.T) {
	spec := GraphSpec{
		Name:  "invalid-blocks",
		Nodes: []NodeSpec{{ID: "shared", Kind: NodeKindLambda, HandlerRef: "shared"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "shared"}, {From: "shared", To: NodeID("end")}},
		Blocks: []BlockSpec{
			{ID: "b1", Kind: BlockKindSequence, Nodes: []NodeID{"shared"}},
			{ID: "b2", Kind: BlockKindSequence, Nodes: []NodeID{"shared"}},
		},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"shared": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return input, nil
		},
	}}

	if _, err := NewCompiler(resolver).Compile(context.Background(), plan); err == nil {
		t.Fatal("Compile error = nil, want overlapping block membership error")
	}
}

func TestCompilerInjectsWorkflowStaticInput(t *testing.T) {
	spec := GraphSpec{
		Name:  "workflow-static-input",
		Nodes: []NodeSpec{{ID: "prepare", Kind: NodeKindLambda, HandlerRef: "prepare"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "prepare"}, {From: "prepare", To: NodeID("end")}},
		Blocks: []BlockSpec{{
			ID:         "workflow.prepare",
			Kind:       BlockKindSequence,
			Nodes:      []NodeID{"prepare"},
			EntryNodes: []NodeID{"prepare"},
			ExitNodes:  []NodeID{"prepare"},
			Metadata:   map[string]any{"static_value": map[string]any{"seed": "x", "preset": true}},
		}},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}

	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"prepare": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return map[string]any{"seed": input["seed"], "preset": input["preset"]}, nil
		},
	}}

	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["seed"] != "x" || out["preset"] != true {
		t.Fatalf("output = %#v", out)
	}
}

func TestValidateWorkflowDependencyOnlyEdge(t *testing.T) {
	edge := PlannedEdge{
		ID:       "workflow_dep",
		Kind:     EdgeKindData,
		From:     "prepare",
		To:       "finish",
		Metadata: map[string]any{"workflow_input_mode": "dependency"},
	}
	if err := validateWorkflowInputEdge(edge); err == nil {
		t.Fatal("validateWorkflowInputEdge error = nil, want dependency-only validation error")
	}
}

func TestValidateWorkflowNoDirectDependencyEdge(t *testing.T) {
	edge := PlannedEdge{
		ID:       "workflow_control",
		Kind:     EdgeKindProjection,
		From:     "prepare",
		To:       "finish",
		Metadata: map[string]any{"workflow_input_mode": "control"},
	}
	if err := validateWorkflowInputEdge(edge); err != nil {
		t.Fatalf("validateWorkflowInputEdge: %v", err)
	}
}

func TestCompilerAppliesBoundaryProjection(t *testing.T) {
	spec := GraphSpec{
		Name:  "boundary-projection",
		Nodes: []NodeSpec{{ID: "entry", Kind: NodeKindLambda, HandlerRef: "entry"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "entry"}, {From: "entry", To: NodeID("end")}},
		Boundaries: []BoundarySpec{{
			ID:        "subgraph.boundary",
			EntryNode: "entry",
			ExitNodes: []NodeID{"entry"},
			Projection: &BoundaryProjectionSpec{Mapping: map[string]string{
				"source.value": "target",
			}},
		}},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return map[string]any{"target": input["target"]}, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{"source": map[string]any{"value": "ok"}})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["target"] != "ok" {
		t.Fatalf("output = %#v", out)
	}
}

func TestCompilerRejectsBoundaryBindingViolation(t *testing.T) {
	spec := GraphSpec{
		Name:  "boundary-visibility",
		Nodes: []NodeSpec{{ID: "entry", Kind: NodeKindLambda, HandlerRef: "entry", BindingRef: "binding.secret"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "entry"}, {From: "entry", To: NodeID("end")}},
		Bindings: []BindingSpec{
			{Ref: "binding.secret", Kind: BindingKindService, Target: "svc.secret"},
			{Ref: "binding.public", Kind: BindingKindService, Target: "svc.public"},
		},
		Boundaries: []BoundarySpec{{
			ID:         "restricted.boundary",
			EntryNode:  "entry",
			ExitNodes:  []NodeID{"entry"},
			Visibility: &BoundaryVisibilitySpec{AllowedBindings: []BindingRef{"binding.public"}},
		}},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return input, nil
		},
	}}
	if _, err := NewCompiler(resolver).Compile(context.Background(), plan); err == nil {
		t.Fatal("Compile error = nil, want boundary visibility violation")
	}
}

func TestCompilerRejectsUnknownRecoveryScope(t *testing.T) {
	spec := GraphSpec{
		Name:   "invalid-recovery-scope",
		Nodes:  []NodeSpec{{ID: "entry", Kind: NodeKindLambda, HandlerRef: "entry"}},
		Edges:  []EdgeSpec{{From: NodeID("start"), To: "entry"}, {From: "entry", To: NodeID("end")}},
		Blocks: []BlockSpec{{ID: "block.entry", Kind: BlockKindSequence, Nodes: []NodeID{"entry"}, RecoveryScopeRef: "missing.scope"}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return input, nil
		},
	}}
	if _, err := NewCompiler(resolver).Compile(context.Background(), plan); err == nil {
		t.Fatal("Compile error = nil, want unknown recovery scope validation error")
	}
}

func TestRunnerResumeAppliesRecoveryMetadata(t *testing.T) {
	spec := GraphSpec{
		Name:  "resume-recovery-metadata",
		Nodes: []NodeSpec{{ID: "entry", Kind: NodeKindLambda, HandlerRef: "entry"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "entry"}, {From: "entry", To: NodeID("end")}},
		Recovery: RecoverySpec{
			CheckpointScopes: []CheckpointScopeSpec{{ID: "scope.main", PersistedFields: []string{"seed", "status"}}},
			ResumeEntries:    []ResumeEntrySpec{{ID: "resume.main", NodeID: "entry"}},
			ReplayPolicies:   []ReplayPolicySpec{{ScopeID: "scope.main", ReplayNodeIDs: []NodeID{"entry"}}},
		},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			meta, _ := input["_builder"].(map[string]any)
			return map[string]any{
				"scope":     meta["recovery_scope"],
				"persisted": meta["persisted_fields"],
				"replay":    meta["replay_nodes"],
			}, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	runner := NewRunner()
	out, err := runner.Resume(context.Background(), compiled, ResumeRequest{
		Checkpoint: map[string]any{"seed": "x"},
		EntryID:    "resume.main",
		ScopeID:    "scope.main",
	})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if out["scope"] != "scope.main" {
		t.Fatalf("scope = %#v", out["scope"])
	}
}

func TestCompilerRejectsParallelConditionalEdge(t *testing.T) {
	spec := GraphSpec{
		Name: "parallel-invalid",
		Nodes: []NodeSpec{
			{ID: "fork", Kind: NodeKindLambda, HandlerRef: "fork"},
			{ID: "left", Kind: NodeKindLambda, HandlerRef: "left"},
		},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "fork"},
			{From: "fork", To: "left", Kind: EdgeKindConditional, Condition: &ConditionSpec{Expr: "ok"}},
			{From: "left", To: NodeID("end")},
		},
		Blocks: []BlockSpec{{ID: "parallel.block", Kind: BlockKindParallel, Nodes: []NodeID{"fork", "left"}, EntryNodes: []NodeID{"fork"}, ExitNodes: []NodeID{"left"}}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"fork": func(ctx context.Context, input map[string]any) (map[string]any, error) { return input, nil },
		"left": func(ctx context.Context, input map[string]any) (map[string]any, error) { return input, nil },
	}}
	if _, err := NewCompiler(resolver).Compile(context.Background(), plan); err == nil {
		t.Fatal("Compile error = nil, want parallel conditional edge rejection")
	}
}

func TestCompilerRejectsLoopInvalidSelfEdge(t *testing.T) {
	spec := GraphSpec{
		Name:  "loop-invalid",
		Nodes: []NodeSpec{{ID: "loop", Kind: NodeKindLambda, HandlerRef: "loop"}},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "loop"},
			{From: "loop", To: "loop", Kind: EdgeKindData},
			{From: "loop", To: NodeID("end")},
		},
		Blocks: []BlockSpec{{ID: "loop.block", Kind: BlockKindLoop, Nodes: []NodeID{"loop"}, EntryNodes: []NodeID{"loop"}, ExitNodes: []NodeID{"loop"}}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"loop": func(ctx context.Context, input map[string]any) (map[string]any, error) { return input, nil },
	}}
	if _, err := NewCompiler(resolver).Compile(context.Background(), plan); err == nil {
		t.Fatal("Compile error = nil, want loop self-edge validation error")
	}
}

func TestCompilerAcceptsRetryErrorEdge(t *testing.T) {
	spec := GraphSpec{
		Name:  "retry-valid",
		Nodes: []NodeSpec{{ID: "attempt", Kind: NodeKindLambda, HandlerRef: "attempt"}},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "attempt"},
			{From: "attempt", To: "attempt", Kind: EdgeKindError},
			{From: "attempt", To: NodeID("end")},
		},
		Blocks: []BlockSpec{{ID: "retry.block", Kind: BlockKindRetry, Nodes: []NodeID{"attempt"}, EntryNodes: []NodeID{"attempt"}, ExitNodes: []NodeID{"attempt"}}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"attempt": func(ctx context.Context, input map[string]any) (map[string]any, error) { return input, nil },
	}}
	if _, err := NewCompiler(resolver).Compile(context.Background(), plan); err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

func TestCompilerRetriesNodeExecution(t *testing.T) {
	attempts := 0
	spec := GraphSpec{
		Name:  "retry-runtime",
		Nodes: []NodeSpec{{ID: "attempt", Kind: NodeKindLambda, HandlerRef: "attempt"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "attempt"}, {From: "attempt", To: NodeID("end")}},
		Blocks: []BlockSpec{{
			ID:         "retry.block",
			Kind:       BlockKindRetry,
			Nodes:      []NodeID{"attempt"},
			EntryNodes: []NodeID{"attempt"},
			ExitNodes:  []NodeID{"attempt"},
			Metadata:   map[string]any{"max_attempts": 3},
		}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"attempt": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			attempts++
			if attempts < 3 {
				return nil, fmt.Errorf("temporary failure")
			}
			return map[string]any{"attempts": attempts}, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["attempts"] != 3 {
		t.Fatalf("output = %#v", out)
	}
}

func TestCompilerFallbackReturnsInputOnError(t *testing.T) {
	spec := GraphSpec{
		Name:  "fallback-runtime",
		Nodes: []NodeSpec{{ID: "fallback", Kind: NodeKindLambda, HandlerRef: "fallback"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "fallback"}, {From: "fallback", To: NodeID("end")}},
		Blocks: []BlockSpec{{
			ID:         "fallback.block",
			Kind:       BlockKindFallback,
			Nodes:      []NodeID{"fallback"},
			EntryNodes: []NodeID{"fallback"},
			ExitNodes:  []NodeID{"fallback"},
		}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"fallback": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return nil, fmt.Errorf("primary failure")
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{"seed": "x"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["seed"] != "x" || out["_fallback_error"] != "primary failure" {
		t.Fatalf("output = %#v", out)
	}
}

func TestCompilerLoopsNodeExecution(t *testing.T) {
	iterations := 0
	spec := GraphSpec{
		Name:  "loop-runtime",
		Nodes: []NodeSpec{{ID: "loop", Kind: NodeKindLambda, HandlerRef: "loop"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "loop"}, {From: "loop", To: NodeID("end")}},
		Blocks: []BlockSpec{{
			ID:         "loop.block",
			Kind:       BlockKindLoop,
			Nodes:      []NodeID{"loop"},
			EntryNodes: []NodeID{"loop"},
			ExitNodes:  []NodeID{"loop"},
			Metadata:   map[string]any{"max_iterations": 3},
		}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"loop": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			iterations++
			return map[string]any{"count": iterations}, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["count"] != 3 || out["_loop_iteration"] != 3 {
		t.Fatalf("output = %#v", out)
	}
}

func TestCompilerExecutesParallelBranches(t *testing.T) {
	seen := []int{}
	spec := GraphSpec{
		Name: "parallel-runtime",
		Nodes: []NodeSpec{
			{ID: "fanout", Kind: NodeKindLambda, HandlerRef: "fanout"},
			{ID: "branch2", Kind: NodeKindLambda, HandlerRef: "branch2"},
		},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "fanout"}, {From: "fanout", To: NodeID("end")}},
		Blocks: []BlockSpec{{
			ID:         "parallel.block",
			Kind:       BlockKindParallel,
			Nodes:      []NodeID{"fanout", "branch2"},
			EntryNodes: []NodeID{"fanout"},
			ExitNodes:  []NodeID{"fanout"},
		}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"fanout": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			branch, _ := input["_parallel_branch"].(int)
			seen = append(seen, branch)
			return map[string]any{"last_branch": branch}, nil
		},
		"branch2": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return input, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	out, err := compiled.Runnable.Invoke(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["_parallel_branches"] != 2 {
		t.Fatalf("output = %#v", out)
	}
	if len(seen) != 2 {
		t.Fatalf("seen = %#v", seen)
	}
}

func TestReplaySkipsNonReplayNodeExecution(t *testing.T) {
	attempts := 0
	spec := GraphSpec{
		Name: "replay-runtime",
		Nodes: []NodeSpec{
			{ID: "entry", Kind: NodeKindLambda, HandlerRef: "entry"},
			{ID: "other", Kind: NodeKindLambda, HandlerRef: "other"},
		},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "entry"}, {From: "entry", To: NodeID("end")}},
		Recovery: RecoverySpec{
			CheckpointScopes: []CheckpointScopeSpec{{ID: "scope.main", PersistedFields: []string{"seed"}}},
			ResumeEntries:    []ResumeEntrySpec{{ID: "resume.main", NodeID: "entry"}},
			ReplayPolicies:   []ReplayPolicySpec{{ScopeID: "scope.main", ReplayNodeIDs: []NodeID{"other"}}},
		},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			attempts++
			return map[string]any{"attempts": attempts}, nil
		},
		"other": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return input, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	runner := NewRunner()
	out, err := runner.Resume(context.Background(), compiled, ResumeRequest{
		Checkpoint: map[string]any{"seed": "x"},
		EntryID:    "resume.main",
		ScopeID:    "scope.main",
	})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if attempts != 0 {
		t.Fatalf("attempts = %d, want 0", attempts)
	}
	if out["seed"] != "x" {
		t.Fatalf("output = %#v", out)
	}
}

func TestRunnerPersistsCheckpointOnSuccess(t *testing.T) {
	store := &memoryStore{}
	spec := GraphSpec{
		Name:  "checkpoint-runtime",
		Nodes: []NodeSpec{{ID: "entry", Kind: NodeKindLambda, HandlerRef: "entry"}},
		Edges: []EdgeSpec{{From: NodeID("start"), To: "entry"}, {From: "entry", To: NodeID("end")}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return map[string]any{"status": "ok"}, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	runner := NewRunnerWithCheckpointStore(resolver, store)
	if _, err := runner.Run(context.Background(), compiled, RunRequest{}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok, _ := store.Get(context.Background(), "checkpoint-runtime"); !ok {
		t.Fatal("checkpoint not persisted")
	}
}

func TestRunnerResumeLoadsCheckpointFromStore(t *testing.T) {
	store := &memoryStore{}
	_ = store.Set(context.Background(), "checkpoint-runtime", serializeCheckpointState(map[string]any{"seed": "stored", "count": 2}))
	spec := GraphSpec{
		Name:     "checkpoint-runtime",
		Nodes:    []NodeSpec{{ID: "entry", Kind: NodeKindLambda, HandlerRef: "entry"}},
		Edges:    []EdgeSpec{{From: NodeID("start"), To: "entry"}, {From: "entry", To: NodeID("end")}},
		Recovery: RecoverySpec{ResumeEntries: []ResumeEntrySpec{{ID: "resume.main", NodeID: "entry"}}},
	}
	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}
	resolver := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			return map[string]any{"seed": input["seed"], "count": input["count"]}, nil
		},
	}}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	runner := NewRunnerWithCheckpointStore(resolver, store)
	out, err := runner.Resume(context.Background(), compiled, ResumeRequest{EntryID: "resume.main"})
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if out["seed"] != "stored" || out["count"] != 2 {
		t.Fatalf("output = %#v", out)
	}
}

func TestRuntimeResolverPipelineOrder(t *testing.T) {
	plan := ExecutionPlan{}
	wrappers := runtimeWrappers(plan)
	if len(wrappers) != 5 {
		t.Fatalf("wrapper count = %d", len(wrappers))
	}

	calls := []string{}
	base := &staticResolver{nodes: map[HandlerRef]any{
		"entry": func(ctx context.Context, input map[string]any) (map[string]any, error) {
			_ = ctx
			calls = append(calls, "base")
			return input, nil
		},
	}}

	resolver := buildRuntimeResolverPipeline(plan, base)
	resolved, err := resolver.ResolveNode(context.Background(), PlannedNode{ID: "entry", Handler: "entry"}, nil, nil)
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	fn, ok := resolved.(func(context.Context, map[string]any) (map[string]any, error))
	if !ok {
		t.Fatalf("resolved type = %T", resolved)
	}
	if _, err := fn(context.Background(), map[string]any{}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(calls) != 1 || calls[0] != "base" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestCheckpointCodecRoundTrip(t *testing.T) {
	state := map[string]any{
		"seed":   "stored",
		"count":  2,
		"active": true,
		"ratio":  1.5,
		"nodes":  []string{"a", "b"},
	}
	encoded := encodeCheckpointState(state)
	decoded := decodeCheckpointState(encoded)
	if decoded["seed"] != "stored" || decoded["count"] != 2 || decoded["active"] != true {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestCompileWithOverlayAppliesBindingPolicyOverrides(t *testing.T) {
	spec := GraphSpec{
		Name: "overlay-compile",
		Nodes: []NodeSpec{{
			ID:         "inspect",
			Kind:       NodeKindLambda,
			HandlerRef: "inspect",
			BindingRef: "binding.default",
			PolicyRef:  "policy.default",
		}},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "inspect"},
			{From: "inspect", To: NodeID("end")},
		},
		Bindings: []BindingSpec{
			{Ref: "binding.default", Kind: BindingKindService, Target: "svc.default"},
			{Ref: "binding.override", Kind: BindingKindService, Target: "svc.override"},
		},
		Policies: []PolicySpec{
			{Ref: "policy.default", Kind: PolicyKindExecution},
			{Ref: "policy.override", Kind: PolicyKindExecution},
		},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}

	resolver := &captureResolver{}
	compiled, err := CompileWithOverlay(context.Background(), resolver, plan, RuntimeOverlay{
		BindingOverrides: map[BindingRef]BindingRef{"binding.default": "binding.override"},
		PolicyOverrides:  map[PolicyRef]PolicyRef{"policy.default": "policy.override"},
	})
	if err != nil {
		t.Fatalf("CompileWithOverlay: %v", err)
	}
	if compiled.Plan.Structural.Nodes[0].Binding != "binding.override" {
		t.Fatalf("binding = %q", compiled.Plan.Structural.Nodes[0].Binding)
	}
	if compiled.Plan.Structural.Nodes[0].Policy != "policy.override" {
		t.Fatalf("policy = %q", compiled.Plan.Structural.Nodes[0].Policy)
	}
	if resolver.bindings["inspect"] != "binding.override" {
		t.Fatalf("resolved binding = %q", resolver.bindings["inspect"])
	}
	if resolver.policies["inspect"] != "policy.override" {
		t.Fatalf("resolved policy = %q", resolver.policies["inspect"])
	}
}

func TestRunnerRunRecompilesForBindingPolicyOverrides(t *testing.T) {
	spec := GraphSpec{
		Name: "overlay-run-compile",
		Nodes: []NodeSpec{{
			ID:         "inspect",
			Kind:       NodeKindLambda,
			HandlerRef: "inspect",
			BindingRef: "binding.default",
			PolicyRef:  "policy.default",
		}},
		Edges: []EdgeSpec{
			{From: NodeID("start"), To: "inspect"},
			{From: "inspect", To: NodeID("end")},
		},
		Bindings: []BindingSpec{
			{Ref: "binding.default", Kind: BindingKindService, Target: "svc.default"},
			{Ref: "binding.override", Kind: BindingKindService, Target: "svc.override"},
		},
		Policies: []PolicySpec{
			{Ref: "policy.default", Kind: PolicyKindExecution},
			{Ref: "policy.override", Kind: PolicyKindExecution},
		},
	}

	plan, err := BuildExecutionPlan(spec)
	if err != nil {
		t.Fatalf("BuildExecutionPlan: %v", err)
	}

	resolver := &captureResolver{}
	compiled, err := NewCompiler(resolver).Compile(context.Background(), plan)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	runner := NewRunnerWithResolver(resolver)
	out, err := runner.Run(context.Background(), compiled, RunRequest{Overlay: RuntimeOverlay{
		BindingOverrides: map[BindingRef]BindingRef{"binding.default": "binding.override"},
		PolicyOverrides:  map[PolicyRef]PolicyRef{"policy.default": "policy.override"},
	}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out["binding_ref"] != "binding.override" {
		t.Fatalf("binding_ref = %#v", out["binding_ref"])
	}
	if out["policy_ref"] != "policy.override" {
		t.Fatalf("policy_ref = %#v", out["policy_ref"])
	}
}
