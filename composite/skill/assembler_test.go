package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components"
	ftool "github.com/cloudwego/eino/components/tool"
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
		graph: &schemad.GraphSpec{
			Name: "skill_graph",
			Type: schemad.GraphTypeGraph,
		},
	}
	resolver := NewResolver(loader, fakeComponentFactory{}, nil).WithGraphAssembler(fakeGraphAssembler{})
	assembler := NewAssembler(resolver)

	runnable, err := assembler.Build(context.Background(), &SkillSpec{
		Info:        Info{Name: "document-skill", Description: "loads from declarative documents"},
		Instruction: "Use the attached tool.",
		ToolRefs: []schemad.Ref{
			{Kind: schemad.RefKindComponentDocument, Target: "tool.json"},
		},
		GraphRef: &schemad.Ref{
			Kind:   schemad.RefKindGraphDocument,
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

func TestDefaultAssemblerBuild_CommandTools(t *testing.T) {
	resolver := NewResolver(nil, nil, nil)
	assembler := NewAssembler(resolver)
	assembler.CommandToolBuilder = NewCommandToolBuilder(CommandToolBuilderConfig{
		WorkspaceRoot: "/workspace-root",
		Shell:         fakeCommandShell{},
	})

	runnable, err := assembler.Build(context.Background(), &SkillSpec{
		Info: Info{Name: "command-skill"},
		CommandTools: []CommandToolSpec{{
			Name:        "get_hive_schema",
			Description: "Query hive schema",
			Parameters: &CommandParamsSpec{
				Required: []string{"table_name"},
				Properties: map[string]CommandParamSchema{
					"table_name": {
						Type:        "string",
						Description: "Hive table name",
					},
				},
			},
			Command: CommandExecutionSpec{
				Argv: []string{"python3", "scripts/get_hive_schema.py", "{{table_name}}"},
				Cwd:  "workspace/skills/datablue",
			},
		}},
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
	invokable, ok := tools[0].(ftool.InvokableTool)
	if !ok {
		t.Fatalf("tool does not implement InvokableTool")
	}
	info, err := tools[0].Info(context.Background())
	if err != nil {
		t.Fatalf("Tool.Info: %v", err)
	}
	if info.Name != "get_hive_schema" {
		t.Fatalf("tool name = %q, want get_hive_schema", info.Name)
	}
	output, err := invokable.InvokableRun(context.Background(), `{"table_name":"db.table"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(output, "python3") || !strings.Contains(output, "db.table") {
		t.Fatalf("output = %q, want rendered command", output)
	}
	if !strings.Contains(output, "/workspace-root/workspace/skills/datablue") {
		t.Fatalf("output = %q, want resolved cwd", output)
	}
	if info.ParamsOneOf == nil {
		t.Fatalf("ParamsOneOf = nil, want non-nil")
	}
	js, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema: %v", err)
	}
	if js == nil || js.Properties == nil {
		t.Fatalf("json schema = %#v, want properties", js)
	}
}

func TestDefaultAssemblerBuild_CommandTools_OptionalArgsCanBeOmitted(t *testing.T) {
	resolver := NewResolver(nil, nil, nil)
	assembler := NewAssembler(resolver)
	assembler.CommandToolBuilder = NewCommandToolBuilder(CommandToolBuilderConfig{
		WorkspaceRoot: "/workspace-root",
		Shell:         fakeCommandShell{},
	})

	runnable, err := assembler.Build(context.Background(), &SkillSpec{
		Info: Info{Name: "command-skill-optional"},
		CommandTools: []CommandToolSpec{{
			Name:        "distill_colleague_knowledge",
			Description: "Distill colleague knowledge",
			Parameters: &CommandParamsSpec{
				Properties: map[string]CommandParamSchema{
					"docs_query": {Type: "string"},
					"wiki_query": {Type: "string"},
				},
			},
			Command: CommandExecutionSpec{
				Argv: []string{"python3", "scripts/distill.py", "--docs-query", "{{docs_query}}", "--wiki-query", "{{wiki_query}}"},
				Cwd:  "workspace/skills/distill-colleague",
			},
		}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	tools, err := runnable.Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	invokable, ok := tools[0].(ftool.InvokableTool)
	if !ok {
		t.Fatalf("tool does not implement InvokableTool")
	}
	output, err := invokable.InvokableRun(context.Background(), `{"docs_query":"安全"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(output, "python3") || !strings.Contains(output, "--docs-query") || !strings.Contains(output, "安全") {
		t.Fatalf("output = %q, want rendered command with provided args", output)
	}
	if !strings.Contains(output, "--wiki-query") {
		t.Fatalf("output = %q, want optional flag rendered", output)
	}
	if strings.Contains(output, "{{wiki_query}}") {
		t.Fatalf("output = %q, placeholder should not survive rendering", output)
	}
}

func TestDefaultAssemblerBuild_CommandTools_WorkspaceRelativeCwdAvoidsDoubleWorkspace(t *testing.T) {
	root := t.TempDir()
	workspaceRoot := filepath.Join(root, "workspace")
	commandDir := filepath.Join(workspaceRoot, "skills", "distill-colleague")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	resolver := NewResolver(nil, nil, nil)
	assembler := NewAssembler(resolver)
	assembler.CommandToolBuilder = NewCommandToolBuilder(CommandToolBuilderConfig{
		WorkspaceRoot: workspaceRoot,
		Shell:         fakeCommandShell{},
	})

	runnable, err := assembler.Build(context.Background(), &SkillSpec{
		Info: Info{Name: "command-skill-cwd"},
		CommandTools: []CommandToolSpec{{
			Name:        "distill_colleague_knowledge",
			Description: "Distill colleague knowledge",
			Command: CommandExecutionSpec{
				Argv: []string{"python3", "scripts/distill.py"},
				Cwd:  "workspace/skills/distill-colleague",
			},
		}},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	tools, err := runnable.Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	invokable, ok := tools[0].(ftool.InvokableTool)
	if !ok {
		t.Fatalf("tool does not implement InvokableTool")
	}
	output, err := invokable.InvokableRun(context.Background(), `{}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(output, commandDir) {
		t.Fatalf("output = %q, want resolved cwd %q", output, commandDir)
	}
	if strings.Contains(output, filepath.Join(workspaceRoot, "workspace", "skills")) {
		t.Fatalf("output = %q, got duplicated workspace path", output)
	}
}

type stubDocumentLoader struct {
	component *schemad.ComponentSpec
	graph     *schemad.GraphSpec
}

func (s *stubDocumentLoader) LoadGraphSpec(ctx context.Context, ref schemad.Ref) (*schemad.GraphSpec, error) {
	_ = ctx
	_ = ref
	return s.graph, nil
}

func (s *stubDocumentLoader) LoadComponentSpec(ctx context.Context, target string) (*schemad.ComponentSpec, error) {
	_ = ctx
	_ = target
	return s.component, nil
}

func (s *stubDocumentLoader) LoadNode(ctx context.Context, ref schemad.Ref) (*schemad.NodeSpec, error) {
	_ = ctx
	_ = ref
	return nil, nil
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

type fakeGraphAssembler struct{}

func (fakeGraphAssembler) AssembleGraph(ctx context.Context, spec *schemad.GraphSpec) (compose.AnyGraph, error) {
	_ = ctx
	_ = spec
	return compose.NewGraph[map[string]any, map[string]any](), nil
}

type stubInterpreterResolver struct {
	functions map[string]any
	graphs    map[string]compose.AnyGraph
}

type fakeCommandShell struct{}

func (fakeCommandShell) Execute(ctx context.Context, req *CommandExecuteRequest) (*CommandExecuteResponse, error) {
	_ = ctx
	return &CommandExecuteResponse{Output: req.Command + " @ " + req.Cwd}, nil
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
