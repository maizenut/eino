package skill

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
)

func TestCompileGraphAndGraphExecutorInvoke(t *testing.T) {
	ctx := context.Background()
	graph := newExecutableSkillGraph(t)
	runnable := &resolvedSkill{
		info:     Info{Name: "graph-skill"},
		graph:    graph,
		hasGraph: true,
	}

	compiled, err := CompileGraph(ctx, runnable)
	if err != nil {
		t.Fatalf("CompileGraph: %v", err)
	}
	out, err := compiled.Invoke(ctx, map[string]any{"query": "ping"})
	if err != nil {
		t.Fatalf("compiled.Invoke: %v", err)
	}
	if out["query"] != "ping" || out["status"] != "ok" {
		t.Fatalf("compiled output = %#v, want query=ping status=ok", out)
	}

	executor, err := NewGraphExecutor(runnable)
	if err != nil {
		t.Fatalf("NewGraphExecutor: %v", err)
	}
	out, err = executor.Invoke(ctx, map[string]any{"query": "pong"})
	if err != nil {
		t.Fatalf("executor.Invoke: %v", err)
	}
	if out["query"] != "pong" || out["status"] != "ok" {
		t.Fatalf("executor output = %#v, want query=pong status=ok", out)
	}
}

func TestGraphExecutorMissingGraph(t *testing.T) {
	executor, err := NewGraphExecutor(&resolvedSkill{info: Info{Name: "no-graph"}})
	if err != nil {
		t.Fatalf("NewGraphExecutor: %v", err)
	}
	if _, err := executor.Compiled(context.Background()); err == nil || !strings.Contains(err.Error(), "does not expose an executable graph") {
		t.Fatalf("Compiled error = %v, want executable graph error", err)
	}
}

func TestServiceBuildGraphExecutorByName(t *testing.T) {
	service := NewService(nil, NewMemoryRegistry(), &SimpleSelector{}, NewAssembler(NewResolver(nil, nil, stubInterpreterResolver{
		graphs: map[string]compose.AnyGraph{
			"graph.weather": newExecutableSkillGraph(t),
		},
	})))

	spec := &SkillSpec{
		Info: Info{Name: "weather"},
		GraphRef: &schemad.Ref{
			Kind:   schemad.RefKindInterpreterGraph,
			Target: "graph.weather",
		},
	}
	if err := service.Register(context.Background(), spec); err != nil {
		t.Fatalf("Register: %v", err)
	}

	executor, err := service.BuildGraphExecutorByName(context.Background(), "weather")
	if err != nil {
		t.Fatalf("BuildGraphExecutorByName: %v", err)
	}
	out, err := executor.Invoke(context.Background(), map[string]any{"query": "rain"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["query"] != "rain" || out["status"] != "ok" {
		t.Fatalf("output = %#v, want query=rain status=ok", out)
	}
}

func TestServiceLoadAndBuildGraphExecutor(t *testing.T) {
	workspaceDir := t.TempDir()
	writeGraphExecutorJSONFile(t, filepath.Join(workspaceDir, "skill.json"), SkillSpec{
		Info: Info{Name: "svc-graph"},
		GraphRef: &schemad.Ref{
			Kind:   schemad.RefKindInterpreterGraph,
			Target: "graph.service",
		},
	})

	loader := &FileDocumentLoader{BaseDir: workspaceDir}
	resolver := NewResolver(nil, nil, stubInterpreterResolver{
		graphs: map[string]compose.AnyGraph{
			"graph.service": newExecutableSkillGraph(t),
		},
	})
	service := NewDefaultService(loader, NewAssembler(resolver))

	executor, spec, err := service.LoadAndBuildGraphExecutor(context.Background(), "skill.json")
	if err != nil {
		t.Fatalf("LoadAndBuildGraphExecutor: %v", err)
	}
	if spec.Info.Name != "svc-graph" {
		t.Fatalf("spec name = %q, want svc-graph", spec.Info.Name)
	}
	out, err := executor.Invoke(context.Background(), map[string]any{"query": "storm"})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out["query"] != "storm" || out["status"] != "ok" {
		t.Fatalf("output = %#v, want query=storm status=ok", out)
	}
}

func newExecutableSkillGraph(t *testing.T) compose.AnyGraph {
	t.Helper()
	graph := compose.NewGraph[map[string]any, map[string]any]()
	if err := graph.AddLambdaNode("run", compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		_ = ctx
		out := make(map[string]any, len(input)+1)
		for k, v := range input {
			out[k] = v
		}
		out["status"] = "ok"
		return out, nil
	})); err != nil {
		t.Fatalf("AddLambdaNode: %v", err)
	}
	if err := graph.AddEdge(compose.START, "run"); err != nil {
		t.Fatalf("AddEdge start: %v", err)
	}
	if err := graph.AddEdge("run", compose.END); err != nil {
		t.Fatalf("AddEdge end: %v", err)
	}
	return graph
}

func writeGraphExecutorJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}
