package skill

import schemad "github.com/cloudwego/eino/schema/declarative"

// Info describes the basic metadata of a skill.
type Info struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version,omitempty"`
	Category    string         `json:"category,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TriggerSpec declares how a skill should be selected.
type TriggerSpec struct {
	Strategy string         `json:"strategy,omitempty"`
	Keywords []string       `json:"keywords,omitempty"`
	Patterns []string       `json:"patterns,omitempty"`
	Params   map[string]any `json:"params,omitempty"`
}

// SkillSpec is the static declarative definition of a skill.
type SkillSpec struct {
	Info        Info           `json:"info"`
	Trigger     *TriggerSpec   `json:"trigger,omitempty"`
	Instruction string         `json:"instruction,omitempty"`
	ToolRefs    []schemad.Ref  `json:"tool_refs,omitempty"`
	GraphRef    *schemad.Ref   `json:"graph_ref,omitempty"`
	PromptRef   *schemad.Ref   `json:"prompt_ref,omitempty"`
	ModelRef    *schemad.Ref   `json:"model_ref,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
