package skill

import (
	"context"
	"fmt"
)

// Service wires skill loading, registration, selection, and assembly.
type Service struct {
	Loader    SpecLoader
	Registry  Registry
	Selector  Selector
	Assembler Assembler
}

// NewService creates a skill service from explicit dependencies.
func NewService(loader SpecLoader, registry Registry, selector Selector, assembler Assembler) *Service {
	return &Service{
		Loader:    loader,
		Registry:  registry,
		Selector:  selector,
		Assembler: assembler,
	}
}

// NewDefaultService creates a skill service with default registry and selector.
func NewDefaultService(loader SpecLoader, assembler Assembler) *Service {
	return &Service{
		Loader:    loader,
		Registry:  NewMemoryRegistry(),
		Selector:  &SimpleSelector{},
		Assembler: assembler,
	}
}

// Load reads a skill spec from an external target.
func (s *Service) Load(ctx context.Context, target string) (*SkillSpec, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("skill loader is required")
	}
	if target == "" {
		return nil, fmt.Errorf("skill target is required")
	}
	return s.Loader.LoadSkillSpec(ctx, target)
}

// Register stores a skill spec in the configured registry.
func (s *Service) Register(ctx context.Context, spec *SkillSpec) error {
	if s == nil || s.Registry == nil {
		return fmt.Errorf("skill registry is required")
	}
	return s.Registry.Register(ctx, spec)
}

// LoadAndRegister loads a skill spec and stores it in the registry.
func (s *Service) LoadAndRegister(ctx context.Context, target string) (*SkillSpec, error) {
	spec, err := s.Load(ctx, target)
	if err != nil {
		return nil, err
	}
	if err := s.Register(ctx, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

// Build assembles a loaded skill spec into a runnable.
func (s *Service) Build(ctx context.Context, spec *SkillSpec) (Runnable, error) {
	if s == nil || s.Assembler == nil {
		return nil, fmt.Errorf("skill assembler is required")
	}
	return s.Assembler.Build(ctx, spec)
}

// BuildByName resolves a skill from the registry and assembles it.
func (s *Service) BuildByName(ctx context.Context, name string) (Runnable, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("skill registry is required")
	}
	if name == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	spec, ok := s.Registry.Get(ctx, name)
	if !ok {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	return s.Build(ctx, spec)
}

// LoadAndBuild loads a skill from an external target and assembles it directly.
func (s *Service) LoadAndBuild(ctx context.Context, target string) (Runnable, *SkillSpec, error) {
	spec, err := s.Load(ctx, target)
	if err != nil {
		return nil, nil, err
	}
	runnable, err := s.Build(ctx, spec)
	if err != nil {
		return nil, nil, err
	}
	return runnable, spec, nil
}

// Match returns registered skill specs selected for the given input.
func (s *Service) Match(ctx context.Context, input any) ([]*SkillSpec, error) {
	if s == nil || s.Registry == nil {
		return nil, fmt.Errorf("skill registry is required")
	}
	if s.Selector == nil {
		return nil, fmt.Errorf("skill selector is required")
	}
	items := s.Registry.List(ctx)
	candidates := make([]*SkillSpec, 0, len(items))
	for _, item := range items {
		spec, ok := s.Registry.Get(ctx, item.Name)
		if !ok || spec == nil {
			continue
		}
		candidates = append(candidates, spec)
	}
	return s.Selector.Match(ctx, input, candidates)
}

// MatchAndBuild assembles all registered skills selected for the given input.
func (s *Service) MatchAndBuild(ctx context.Context, input any) ([]Runnable, []*SkillSpec, error) {
	matched, err := s.Match(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	runnables := make([]Runnable, 0, len(matched))
	for _, spec := range matched {
		runnable, err := s.Build(ctx, spec)
		if err != nil {
			name := ""
			if spec != nil {
				name = spec.Info.Name
			}
			return nil, nil, fmt.Errorf("build matched skill %s: %w", name, err)
		}
		runnables = append(runnables, runnable)
	}
	return runnables, matched, nil
}
