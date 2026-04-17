package skill

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	schemad "github.com/cloudwego/eino/schema/declarative"
)

func TestDefaultAssemblerBuild_FromDocuments(t *testing.T) {
	loader := &stubDocumentLoader{
		component: &schemad.ComponentSpec{
			Kind: string(components.ComponentOfTool),
			Impl: "fake_tool",
			Name: "echo",
		},
		graph: &schemad.GraphBlueprint{
			Name: "skill_graph",
			Type: schemad.GraphTypeGraph,
		},
	}
	resolver := NewResolver(loader, fakeComponentFactory{}, nil)
	assembler := NewAssembler(resolver)

	runnable, err := assembler.Build(context.Background(), &SkillSpec{
		Info:        Info{Name: "document-skill", Description: "loads from declarative documents"},
		Instruction: "Use the attached tool.",
		ToolRefs: []schemad.Ref{
			{Kind: schemad.RefKindComponentDocument, Target: "tool.json"},
		},
		GraphRef: &schemad.Ref{
			Kind:   schemad.RefKindBlueprintDocument,
			Target: "graph.json",
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if got := runnable.Info().Name; got != "document-skill" {
		t.Fatalf("Info.Name = %q, want document-skill", got)
	}

	instruction, err := runnable.Instruction(context.Background())
	if err != nil {
		t.Fatalf("Instruction: %v", err)
	}
	if instruction != "Use the attached tool." {
		t.Fatalf("Instruction = %q", instruction)
	}

	tools, err := runnable.Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(tools))
	}
	info, err := tools[0].Info(context.Background())
	if err != nil {
		t.Fatalf("Tool.Info: %v", err)
	}
	if info.Name != "echo" {
		t.Fatalf("tool name = %q, want echo", info.Name)
	}

	graph, ok, err := runnable.Graph(context.Background())
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	if !ok {
		t.Fatalf("Graph ok = false, want true")
	}
	if graph == nil {
		t.Fatalf("Graph = nil, want non-nil")
	}
}

func TestDefaultAssemblerBuild_FromInterpreterFunction(t *testing.T) {
	toolRef := schemad.Ref{Kind: schemad.RefKindInterpreterFunction, Target: "tool.builder"}
	graphRef := schemad.Ref{Kind: schemad.RefKindInterpreterGraph, Target: "graph.builder"}
	resolver := NewResolver(nil, nil, stubInterpreterResolver{
		functions: map[string]any{
			toolRef.Target: func(ctx context.Context) (any, error) {
				_ = ctx
				return fakeTool{name: "dynamic"}, nil
			},
		},
		graphs: map[string]compose.AnyGraph{
			graphRef.Target: compose.NewGraph[map[string]any, map[string]any](),
		},
	})
	assembler := NewAssembler(resolver)

	runnable, err := assembler.Build(context.Background(), &SkillSpec{
		Info:     Info{Name: "dynamic-skill"},
		ToolRefs: []schemad.Ref{toolRef},
		GraphRef: &graphRef,
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	tools, err := runnable.Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(tools))
	}
	info, err := tools[0].Info(context.Background())
	if err != nil {
		t.Fatalf("Tool.Info: %v", err)
	}
	if info.Name != "dynamic" {
		t.Fatalf("tool name = %q, want dynamic", info.Name)
	}

	provider, ok := runnable.(PromptProvider)
	if !ok {
		t.Fatalf("runnable does not implement PromptProvider")
	}
	if _, hasPrompt, err := provider.Prompt(context.Background()); err != nil {
		t.Fatalf("Prompt: %v", err)
	} else if hasPrompt {
		t.Fatalf("Prompt hasPrompt = true, want false")
	}
}

func TestSimpleSelectorAndRegistry(t *testing.T) {
	selector := &SimpleSelector{}
	candidates := []*SkillSpec{
		{
			Info: Info{Name: "weather"},
			Trigger: &TriggerSpec{
				Strategy: TriggerStrategyKeyword,
				Keywords: []string{"weather", "forecast"},
			},
		},
		{
			Info: Info{Name: "math"},
			Trigger: &TriggerSpec{
				Strategy: TriggerStrategyPattern,
				Patterns: []string{`(?i)\bcalc\b`},
			},
		},
	}

	matched, err := selector.Match(context.Background(), "please calc today's weather", candidates)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if len(matched) != 2 {
		t.Fatalf("len(matched) = %d, want 2", len(matched))
	}

	registry := NewMemoryRegistry()
	for _, candidate := range matched {
		if err := registry.Register(context.Background(), candidate); err != nil {
			t.Fatalf("Register %s: %v", candidate.Info.Name, err)
		}
	}

	items := registry.List(context.Background())
	if len(items) != 2 {
		t.Fatalf("len(List) = %d, want 2", len(items))
	}
	if items[0].Name != "math" || items[1].Name != "weather" {
		t.Fatalf("List order = %#v, want math/weather", items)
	}
}

type stubDocumentLoader struct {
	component *schemad.ComponentSpec
	graph     *schemad.GraphBlueprint
}

func (s *stubDocumentLoader) LoadGraphBlueprint(ctx context.Context, target string) (*schemad.GraphBlueprint, error) {
	_ = ctx
	_ = target
	return s.graph, nil
}

func (s *stubDocumentLoader) LoadComponentSpec(ctx context.Context, target string) (*schemad.ComponentSpec, error) {
	_ = ctx
	_ = target
	return s.component, nil
}

type fakeComponentFactory struct{}

func (fakeComponentFactory) BuildComponent(ctx context.Context, spec *schemad.ComponentSpec) (any, error) {
	_ = ctx
	return fakeTool{name: spec.Name}, nil
}

type fakeTool struct {
	name string
}

func (t fakeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	_ = ctx
	return &schema.ToolInfo{Name: t.name}, nil
}

type stubInterpreterResolver struct {
	functions map[string]any
	graphs    map[string]compose.AnyGraph
}

func (s stubInterpreterResolver) ResolveObject(ctx context.Context, ref schemad.Ref) (any, error) {
	_ = ctx
	return s.functions[ref.Target], nil
}

func (s stubInterpreterResolver) ResolveFunction(ctx context.Context, ref schemad.Ref) (any, error) {
	_ = ctx
	return s.functions[ref.Target], nil
}

func (s stubInterpreterResolver) ResolveComponent(ctx context.Context, ref schemad.Ref) (any, error) {
	_ = ctx
	return s.functions[ref.Target], nil
}

func (s stubInterpreterResolver) ResolveGraph(ctx context.Context, ref schemad.Ref) (compose.AnyGraph, error) {
	_ = ctx
	return s.graphs[ref.Target], nil
}
