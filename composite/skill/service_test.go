package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/components"
	schemad "github.com/cloudwego/eino/schema/declarative"
	orcbp "github.com/maizenut/mirroru/orchestration/blueprint"
)

func TestServiceLoadAndBuild(t *testing.T) {
	workspaceDir := t.TempDir()
	writeServiceJSONFile(t, filepath.Join(workspaceDir, "tool.json"), schemad.ComponentSpec{
		Kind: string(components.ComponentOfTool),
		Impl: "fake_tool",
		Name: "svc_tool",
	})
	writeServiceJSONFile(t, filepath.Join(workspaceDir, "skill.json"), SkillSpec{
		Info:        Info{Name: "svc-skill", Description: "service skill"},
		Instruction: "use service",
		ToolRefs: []schemad.Ref{{
			Kind:   schemad.RefKindComponentDocument,
			Target: "tool.json",
		}},
	})

	loader := &FileDocumentLoader{BaseDir: workspaceDir}
	docLoader := &stubDocumentLoader{
		component: &schemad.ComponentSpec{
			Kind: string(components.ComponentOfTool),
			Impl: "fake_tool",
			Name: "svc_tool",
		},
	}
	resolver := NewResolver(skillDocumentLoaderAdapter{skillLoader: loader, docLoader: docLoader}, fakeComponentFactory{}, nil)
	service := NewDefaultService(loader, NewAssembler(resolver))

	runnable, spec, err := service.LoadAndBuild(context.Background(), "skill.json")
	if err != nil {
		t.Fatalf("LoadAndBuild: %v", err)
	}
	if spec.Info.Name != "svc-skill" {
		t.Fatalf("spec name = %q, want svc-skill", spec.Info.Name)
	}
	tools, err := runnable.Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(tools))
	}
}

func TestServiceMatchAndBuild(t *testing.T) {
	service := NewService(nil, NewMemoryRegistry(), &SimpleSelector{}, NewAssembler(NewResolver(nil, nil, stubInterpreterResolver{
		functions: map[string]any{
			"tool.weather": func(ctx context.Context) (any, error) {
				_ = ctx
				return fakeTool{name: "weather_tool"}, nil
			},
		},
	})))

	weather := &SkillSpec{
		Info: Info{Name: "weather"},
		Trigger: &TriggerSpec{
			Strategy: TriggerStrategyKeyword,
			Keywords: []string{"weather"},
		},
		ToolRefs: []schemad.Ref{{
			Kind:   schemad.RefKindInterpreterFunction,
			Target: "tool.weather",
		}},
	}
	math := &SkillSpec{
		Info: Info{Name: "math"},
		Trigger: &TriggerSpec{
			Strategy: TriggerStrategyKeyword,
			Keywords: []string{"math"},
		},
	}

	if err := service.Register(context.Background(), weather); err != nil {
		t.Fatalf("Register weather: %v", err)
	}
	if err := service.Register(context.Background(), math); err != nil {
		t.Fatalf("Register math: %v", err)
	}

	runnables, matched, err := service.MatchAndBuild(context.Background(), "show weather today")
	if err != nil {
		t.Fatalf("MatchAndBuild: %v", err)
	}
	if len(matched) != 1 || matched[0].Info.Name != "weather" {
		t.Fatalf("matched = %#v, want [weather]", matched)
	}
	if len(runnables) != 1 {
		t.Fatalf("len(runnables) = %d, want 1", len(runnables))
	}
	tools, err := runnables[0].Tools(context.Background())
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(tools))
	}
}

func TestServiceBuildByNameNotFound(t *testing.T) {
	service := NewService(nil, NewMemoryRegistry(), &SimpleSelector{}, nil)
	if _, err := service.BuildByName(context.Background(), "missing"); err == nil {
		t.Fatalf("BuildByName missing error = nil, want error")
	}
}

func writeServiceJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

type skillDocumentLoaderAdapter struct {
	skillLoader SpecLoader
	docLoader   *stubDocumentLoader
}

func (a skillDocumentLoaderAdapter) LoadSkillSpec(ctx context.Context, target string) (*SkillSpec, error) {
	if a.skillLoader == nil {
		return nil, fmt.Errorf("skill loader is required")
	}
	return a.skillLoader.LoadSkillSpec(ctx, target)
}

func (a skillDocumentLoaderAdapter) LoadGraphSpec(ctx context.Context, ref schemad.Ref) (*schemad.GraphSpec, error) {
	if a.docLoader == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	return a.docLoader.LoadGraphSpec(ctx, ref)
}

func (a skillDocumentLoaderAdapter) LoadComponentSpec(ctx context.Context, target string) (*schemad.ComponentSpec, error) {
	if a.docLoader == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	return a.docLoader.LoadComponentSpec(ctx, target)
}

func (a skillDocumentLoaderAdapter) LoadNode(ctx context.Context, ref schemad.Ref) (*schemad.NodeSpec, error) {
	if a.docLoader == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	loader := &orcbp.Loader{Documents: blueprintDocumentLoaderAdapter{docLoader: a.docLoader}}
	return loader.LoadNode(ctx, ref)
}

type blueprintDocumentLoaderAdapter struct {
	docLoader *stubDocumentLoader
}

func (a blueprintDocumentLoaderAdapter) LoadGraphSpec(ctx context.Context, target string) (*schemad.GraphSpec, error) {
	if a.docLoader == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	return a.docLoader.LoadGraphSpec(ctx, schemad.Ref{Target: target})
}

func (a blueprintDocumentLoaderAdapter) LoadGraphBlueprint(ctx context.Context, target string) (*schemad.GraphBlueprint, error) {
	if a.docLoader == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	return a.docLoader.LoadGraphBlueprint(ctx, target)
}

func (a blueprintDocumentLoaderAdapter) LoadComponentSpec(ctx context.Context, target string) (*schemad.ComponentSpec, error) {
	if a.docLoader == nil {
		return nil, fmt.Errorf("document loader is required")
	}
	return a.docLoader.LoadComponentSpec(ctx, target)
}
