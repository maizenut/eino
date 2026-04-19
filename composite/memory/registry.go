package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Registry stores and exposes MemorySpec declarations.
//
// A registry is optional infrastructure — it enables runtime discovery of named
// memory specs and is useful in multi-agent or workspace deployments where specs
// are loaded remotely or shared across agents.
type Registry interface {
	Register(ctx context.Context, spec *MemorySpec) error
	Unregister(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (*MemorySpec, bool)
	List(ctx context.Context) []Info
}

// MemoryRegistry is a thread-safe in-memory implementation of Registry.
type MemoryRegistry struct {
	mu    sync.RWMutex
	specs map[string]*MemorySpec
}

// NewMemoryRegistry creates an empty in-memory registry.
func NewMemoryRegistry() *MemoryRegistry {
	return &MemoryRegistry{
		specs: make(map[string]*MemorySpec),
	}
}

// Register stores or replaces the given spec. The spec's Info.Name must be
// non-empty.
func (r *MemoryRegistry) Register(_ context.Context, spec *MemorySpec) error {
	if spec == nil {
		return fmt.Errorf("memory spec is required")
	}
	if spec.Info.Name == "" {
		return fmt.Errorf("memory spec name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.specs[spec.Info.Name] = spec
	return nil
}

// Unregister removes the spec with the given name. It is not an error to
// unregister a name that was never registered.
func (r *MemoryRegistry) Unregister(_ context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("memory spec name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.specs, name)
	return nil
}

// Get returns the spec for the given name, or (nil, false) when not found.
func (r *MemoryRegistry) Get(_ context.Context, name string) (*MemorySpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := r.specs[name]
	return spec, ok
}

// List returns the Info of all registered specs ordered by name.
func (r *MemoryRegistry) List(_ context.Context) []Info {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Info, 0, len(r.specs))
	for _, spec := range r.specs {
		if spec != nil {
			items = append(items, spec.Info)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return items
}

var _ Registry = (*MemoryRegistry)(nil)
