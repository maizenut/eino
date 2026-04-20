package skill

import schemad "github.com/cloudwego/eino/schema/declarative"

// Info describes the basic metadata of a skill.
type Info struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description" yaml:"description"`
	Version     string         `json:"version,omitempty" yaml:"version,omitempty"`
	Category    string         `json:"category,omitempty" yaml:"category,omitempty"`
	Tags        []string       `json:"tags,omitempty" yaml:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// TriggerSpec declares how a skill should be selected.
type TriggerSpec struct {
	Strategy string         `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Keywords []string       `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Patterns []string       `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	Params   map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

// SkillSpec is the static declarative definition of a skill.
type SkillSpec struct {
	Info        Info           `json:"info" yaml:"info"`
	Trigger     *TriggerSpec   `json:"trigger,omitempty" yaml:"trigger,omitempty"`
	Instruction string         `json:"instruction,omitempty" yaml:"instruction,omitempty"`
	ToolRefs    []schemad.Ref  `json:"tool_refs,omitempty" yaml:"tool_refs,omitempty"`
	GraphRef    *schemad.Ref   `json:"graph_ref,omitempty" yaml:"graph_ref,omitempty"`
	PromptRef   *schemad.Ref   `json:"prompt_ref,omitempty" yaml:"prompt_ref,omitempty"`
	ModelRef    *schemad.Ref   `json:"model_ref,omitempty" yaml:"model_ref,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}
