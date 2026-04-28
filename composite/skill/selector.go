package skill

import "context"

const (
	// TriggerStrategyKeyword matches skills by keyword containment.
	TriggerStrategyKeyword = "keyword"
	// TriggerStrategyPattern matches skills by regular expressions.
	TriggerStrategyPattern = "pattern"
)

// Selector chooses matching skills from candidates.
type Selector interface {
	Match(ctx context.Context, input any, candidates []*SkillSpec) ([]*SkillSpec, error)
}
