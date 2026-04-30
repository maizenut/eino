package declarative

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	componentpkg "github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// --- Fakes ---

type fakeFactory struct {
	instance any
	err      error
	lastSpec *ComponentSpec
}

func (f *fakeFactory) BuildComponent(_ context.Context, spec *ComponentSpec) (any, error) {
	f.lastSpec = spec
	if f.err != nil {
		return nil, f.err
	}
	return f.instance, nil
}

type fakeResolver struct {
	instance any
	err      error
	lastRef  Ref
}

func (r *fakeResolver) ResolveComponent(_ context.Context, ref Ref) (any, error) {
	r.lastRef = ref
	if r.err != nil {
		return nil, r.err
	}
	return r.instance, nil
}

type fakeEmbedder struct{}

func (fakeEmbedder) EmbedStrings(context.Context, []string, ...embedding.Option) ([][]float64, error) {
	return nil, nil
}

type fakeTool struct{ name string }

func (f fakeTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: f.name}, nil
}

// --- Tests ---

func TestBuildComponent_RequiresSpec(t *testing.T) {
	if _, err := BuildComponent(context.Background(), nil, nil, nil); err == nil {
		t.Fatal("BuildComponent(nil) err = nil, want non-nil")
	}
}

func TestBuildComponent_FactoryPath(t *testing.T) {
	factory := &fakeFactory{instance: fakeEmbedder{}}
	spec := &ComponentSpec{Kind: string(componentpkg.ComponentOfEmbedding), Impl: "fake"}
	out, err := BuildComponent(context.Background(), spec, factory, nil)
	if err != nil {
		t.Fatalf("BuildComponent: %v", err)
	}
	if _, ok := out.(embedding.Embedder); !ok {
		t.Fatalf("output %T, want embedding.Embedder", out)
	}
	if factory.lastSpec != spec {
		t.Fatal("factory did not receive the spec")
	}
}

func TestBuildComponent_FactoryRequiredWhenNotInterpreter(t *testing.T) {
	spec := &ComponentSpec{Kind: string(componentpkg.ComponentOfEmbedding), Impl: "fake"}
	_, err := BuildComponent(context.Background(), spec, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "factory is required") {
		t.Fatalf("err = %v, want factory required", err)
	}
}

func TestBuildComponent_FactoryPropagatesError(t *testing.T) {
	boom := errors.New("boom")
	factory := &fakeFactory{err: boom}
	spec := &ComponentSpec{Kind: string(componentpkg.ComponentOfTool), Impl: "x"}
	_, err := BuildComponent(context.Background(), spec, factory, nil)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestBuildComponent_InterpreterPath(t *testing.T) {
	resolver := &fakeResolver{instance: fakeTool{name: "ok"}}
	spec := &ComponentSpec{
		Kind: string(componentpkg.ComponentOfTool),
		Impl: RefKindInterpreterComponent,
		Name: "my.tool",
	}
	out, err := BuildComponent(context.Background(), spec, nil, resolver)
	if err != nil {
		t.Fatalf("BuildComponent: %v", err)
	}
	if _, ok := out.(tool.BaseTool); !ok {
		t.Fatalf("output %T, want tool.BaseTool", out)
	}
	if resolver.lastRef.Kind != RefKindInterpreterComponent || resolver.lastRef.Target != "my.tool" {
		t.Fatalf("resolver ref = %+v, want kind=interpreter_component target=my.tool", resolver.lastRef)
	}
}

func TestBuildComponent_InterpreterUsesExplicitRef(t *testing.T) {
	resolver := &fakeResolver{instance: fakeTool{name: "ok"}}
	want := Ref{Kind: RefKindInterpreterComponent, Target: "explicit.tool"}
	spec := &ComponentSpec{
		Kind: string(componentpkg.ComponentOfTool),
		Impl: RefKindInterpreterComponent,
		Name: "ignored",
		Refs: map[string]Ref{"component": want},
	}
	if _, err := BuildComponent(context.Background(), spec, nil, resolver); err != nil {
		t.Fatalf("BuildComponent: %v", err)
	}
	if resolver.lastRef.Kind != want.Kind || resolver.lastRef.Target != want.Target {
		t.Fatalf("resolver ref = %+v, want %+v", resolver.lastRef, want)
	}
}

func TestBuildComponent_InterpreterResolverRequired(t *testing.T) {
	spec := &ComponentSpec{Kind: string(componentpkg.ComponentOfTool), Impl: RefKindInterpreterComponent}
	_, err := BuildComponent(context.Background(), spec, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "resolver is required") {
		t.Fatalf("err = %v, want resolver required", err)
	}
}

func TestBuildComponent_InterpreterKindMismatch(t *testing.T) {
	// resolver returns something that is not an embedder but spec declares embedding.
	resolver := &fakeResolver{instance: "not an embedder"}
	spec := &ComponentSpec{Kind: string(componentpkg.ComponentOfEmbedding), Impl: RefKindInterpreterComponent, Name: "x"}
	_, err := BuildComponent(context.Background(), spec, nil, resolver)
	if err == nil || !strings.Contains(err.Error(), "want embedding.Embedder") {
		t.Fatalf("err = %v, want kind mismatch", err)
	}
}

func TestAsComponentKind_Dispatches(t *testing.T) {
	cases := []struct {
		kind     componentpkg.Component
		instance any
		wantErr  bool
	}{
		{componentpkg.ComponentOfTool, fakeTool{name: "t"}, false},
		{componentpkg.ComponentOfTool, "string", true},
		{componentpkg.ComponentOfEmbedding, fakeEmbedder{}, false},
		{componentpkg.ComponentOfEmbedding, 42, true},
		// unknown kind falls through and returns instance as-is.
		{componentpkg.Component("custom"), "anything", false},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("case-%d-%s", i, tc.kind), func(t *testing.T) {
			out, err := AsComponentKind(string(tc.kind), tc.instance)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("err = nil, want non-nil for %s / %T", tc.kind, tc.instance)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if out == nil {
				t.Fatal("out = nil")
			}
		})
	}
}

func TestComponentKind_Normalization(t *testing.T) {
	cases := map[string]componentpkg.Component{
		"":                    componentpkg.Component(""),
		"prompt":              componentpkg.ComponentOfPrompt,
		"ChatModel":           componentpkg.ComponentOfChatModel,
		"chat_model":          componentpkg.ComponentOfChatModel,
		"agentic_model":       componentpkg.ComponentOfAgenticModel,
		"agentic_runtime":     componentpkg.ComponentOfAgenticRuntime,
		"embedding":           componentpkg.ComponentOfEmbedding,
		"indexer":             componentpkg.ComponentOfIndexer,
		"retriever":           componentpkg.ComponentOfRetriever,
		"loader":              componentpkg.ComponentOfLoader,
		"transformer":         componentpkg.ComponentOfTransformer,
		"DocumentTransformer": componentpkg.ComponentOfTransformer,
		"tool":                componentpkg.ComponentOfTool,
		"custom-kind":         componentpkg.Component("custom-kind"),
	}
	for input, want := range cases {
		if got := ComponentKind(input); got != want {
			t.Errorf("ComponentKind(%q) = %v, want %v", input, got, want)
		}
	}
}
