package skill

import "context"

// Registry stores and exposes skill declarations.
type Registry interface {
	Register(ctx context.Context, spec *SkillSpec) error
	Unregister(ctx context.Context, name string) error
	Get(ctx context.Context, name string) (*SkillSpec, bool)
	List(ctx context.Context) []Info
}
