package skill

import "context"

// SpecLoader loads skill specs from external documents.
type SpecLoader interface {
	LoadSkillSpec(ctx context.Context, target string) (*SkillSpec, error)
}
