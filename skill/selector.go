package skill

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

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

// SimpleSelector evaluates the declarative TriggerSpec with lightweight rules.
type SimpleSelector struct{}

// Match returns candidates whose trigger matches the provided input.
func (s *SimpleSelector) Match(ctx context.Context, input any, candidates []*SkillSpec) ([]*SkillSpec, error) {
	_ = ctx
	text := normalizeInputText(input)
	matches := make([]*SkillSpec, 0, len(candidates))

	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		ok, err := matchTrigger(text, candidate.Trigger)
		if err != nil {
			name := candidate.Info.Name
			if name == "" {
				name = "<unnamed>"
			}
			return nil, fmt.Errorf("match skill %s: %w", name, err)
		}
		if ok {
			matches = append(matches, candidate)
		}
	}

	return matches, nil
}

func matchTrigger(text string, trigger *TriggerSpec) (bool, error) {
	if trigger == nil {
		return true, nil
	}

	strategy := strings.ToLower(strings.TrimSpace(trigger.Strategy))
	switch {
	case strategy == "" && len(trigger.Patterns) > 0:
		strategy = TriggerStrategyPattern
	case strategy == "":
		strategy = TriggerStrategyKeyword
	}

	switch strategy {
	case TriggerStrategyKeyword:
		if len(trigger.Keywords) == 0 {
			return true, nil
		}
		lowerText := strings.ToLower(text)
		for _, keyword := range trigger.Keywords {
			if keyword == "" {
				continue
			}
			if strings.Contains(lowerText, strings.ToLower(keyword)) {
				return true, nil
			}
		}
		return false, nil
	case TriggerStrategyPattern:
		if len(trigger.Patterns) == 0 {
			return true, nil
		}
		for _, pattern := range trigger.Patterns {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return false, fmt.Errorf("compile trigger pattern %q: %w", pattern, err)
			}
			if re.MatchString(text) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported trigger strategy %q", trigger.Strategy)
	}
}

func normalizeInputText(input any) string {
	switch v := input.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}
