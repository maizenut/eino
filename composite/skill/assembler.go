package skill

import "context"

// Assembler builds a skill spec into a runnable skill.
type Assembler interface {
	Build(ctx context.Context, spec *SkillSpec) (Runnable, error)
}
