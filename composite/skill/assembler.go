package skill

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
)

// Assembler builds a skill spec into a runnable skill.
type Assembler interface {
	Build(ctx context.Context, spec *SkillSpec) (Runnable, error)
}

// DefaultAssembler builds static SkillSpec definitions through a Resolver.
type DefaultAssembler struct {
	Resolver *Resolver
}

// NewAssembler creates a static skill assembler.
func NewAssembler(resolver *Resolver) *DefaultAssembler {
	return &DefaultAssembler{Resolver: resolver}
}

// Build assembles a declarative skill into a runtime runnable view.
func (a *DefaultAssembler) Build(ctx context.Context, spec *SkillSpec) (Runnable, error) {
	if spec == nil {
		return nil, fmt.Errorf("skill spec is required")
	}
	if a == nil || a.Resolver == nil {
		return nil, fmt.Errorf("skill resolver is required")
	}

	result := &resolvedSkill{
		info:        spec.Info,
		instruction: spec.Instruction,
		metadata:    copyMap(spec.Metadata),
	}

	if len(spec.ToolRefs) > 0 {
		result.tools = make([]tool.BaseTool, 0, len(spec.ToolRefs))
		for _, ref := range spec.ToolRefs {
			toolValue, err := a.Resolver.ResolveTool(ctx, ref)
			if err != nil {
				return nil, fmt.Errorf("resolve tool ref %s: %w", ref.Target, err)
			}
			result.tools = append(result.tools, toolValue)
		}
	}

	if spec.GraphRef != nil {
		graphValue, err := a.Resolver.ResolveGraph(ctx, *spec.GraphRef)
		if err != nil {
			return nil, fmt.Errorf("resolve graph ref %s: %w", spec.GraphRef.Target, err)
		}
		result.graph = graphValue
		result.hasGraph = true
	}

	if spec.PromptRef != nil {
		promptValue, err := a.Resolver.ResolvePrompt(ctx, *spec.PromptRef)
		if err != nil {
			return nil, fmt.Errorf("resolve prompt ref %s: %w", spec.PromptRef.Target, err)
		}
		result.prompt = promptValue
		result.hasPrompt = true
	}

	if spec.ModelRef != nil {
		modelValue, err := a.Resolver.ResolveModel(ctx, *spec.ModelRef)
		if err != nil {
			return nil, fmt.Errorf("resolve model ref %s: %w", spec.ModelRef.Target, err)
		}
		result.model = modelValue
		result.hasModel = true
	}

	return result, nil
}

func copyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
