package skill

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Registry stores and exposes skill declarations.
type Registry interface {
	Register(ctx context.Context, spec *SkillSpec) error
	Unregister(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (*SkillSpec, bool)
	List(ctx context.Context) []Info
}

// MemoryRegistry is an in-memory skill registry.
type MemoryRegistry struct {
	mu    sync.RWMutex
	specs map[string]*SkillSpec
}

// NewMemoryRegistry creates a new in-memory registry.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		specs: make(map[string]*SkillSpec),
	}
}

// Register stores or replaces a skill spec by name.
func (r *MemoryRegistry) Register(ctx context.Context, spec *SkillSpec) error {
	_ = ctx
	if spec == nil {
		return fmt.Errorf("skill spec is required")
	}
	if spec.Info.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.specs[spec.Info.Name] = spec
	return nil
}

// Unregister removes a skill spec by name.
func (r *MemoryRegistry) Unregister(ctx context.Context, name string) error {
	_ = ctx
	if name == "" {
		return fmt.Errorf("skill name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.specs, name)
	return nil
}

// Get returns a registered skill spec when present.
func (r *MemoryRegistry) Get(ctx context.Context, name string) (*SkillSpec, bool) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.specs[name]
	return spec, ok
}

// List returns registered skill metadata ordered by skill name.
func (r *MemoryRegistry) List(ctx context.Context) []Info {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Info, 0, len(r.specs))
	for _, spec := range r.specs {
		if spec == nil {
			continue
		}
		items = append(items, spec.Info)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items
}

var _ Registry = (*MemoryRegistry)(nil)
