package skill

import schemad "github.com/cloudwego/eino/schema/declarative"

// Info describes the basic metadata of a skill.
type Info struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// TriggerSpec declares how a skill should be selected.
type TriggerSpec struct {
	Strategy string         `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	Keywords []string       `json:"keywords,omitempty" yaml:"keywords,omitempty"`
	Patterns []string       `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	Params   map[string]any `json:"params,omitempty" yaml:"params,omitempty"`
}

type ToolParamsSpec struct {
	Type       string                     `json:"type,omitempty" yaml:"type,omitempty"`
	Required   []string                   `json:"required,omitempty" yaml:"required,omitempty"`
	Properties map[string]ToolParamSchema `json:"properties,omitempty" yaml:"properties,omitempty"`
}

type ToolParamSchema struct {
	Type        string                     `json:"type,omitempty" yaml:"type,omitempty"`
	Description string                     `json:"description,omitempty" yaml:"description,omitempty"`
	Enum        []string                   `json:"enum,omitempty" yaml:"enum,omitempty"`
	Items       *ToolParamSchema           `json:"items,omitempty" yaml:"items,omitempty"`
	Properties  map[string]ToolParamSchema `json:"properties,omitempty" yaml:"properties,omitempty"`
}

type SkillToolSpec struct {
	Name        string                `json:"name" yaml:"name"`
	Ref         *schemad.Ref          `json:"ref,omitempty" yaml:"ref,omitempty"`
	Path        string                `json:"path,omitempty" yaml:"path,omitempty"`
	Package     string                `json:"package,omitempty" yaml:"package,omitempty"`
	Symbol      string                `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Command     *SkillToolCommandSpec `json:"command,omitempty" yaml:"command,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Description string                `json:"description,omitempty" yaml:"description,omitempty"`
	Parameters  *ToolParamsSpec       `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

type SkillToolCommandSpec struct {
	Tool        string         `json:"tool,omitempty" yaml:"tool,omitempty"`
	Kind        string         `json:"kind,omitempty" yaml:"kind,omitempty"`
	Environment string         `json:"environment,omitempty" yaml:"environment,omitempty"`
	Command     string         `json:"command,omitempty" yaml:"command,omitempty"`
	Argv        []string       `json:"argv,omitempty" yaml:"argv,omitempty"`
	Cwd         string         `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	Background  bool           `json:"background,omitempty" yaml:"background,omitempty"`
	TimeoutMS   int            `json:"timeout_ms,omitempty" yaml:"timeout_ms,omitempty"`
	Env         map[string]any `json:"env,omitempty" yaml:"env,omitempty"`
}

// SkillSpec is the static declarative definition of a skill.
type SkillSpec struct {
	Info        Info            `json:"info" yaml:"info"`
	Trigger     *TriggerSpec    `json:"trigger,omitempty" yaml:"trigger,omitempty"`
	Instruction string          `json:"instruction,omitempty" yaml:"instruction,omitempty"`
	Tools       []SkillToolSpec `json:"tools,omitempty" yaml:"tools,omitempty"`
	ToolRefs    []schemad.Ref   `json:"tool_refs,omitempty" yaml:"tool_refs,omitempty"`
	GraphRef    *schemad.Ref    `json:"graph_ref,omitempty" yaml:"graph_ref,omitempty"`
	PromptRef   *schemad.Ref    `json:"prompt_ref,omitempty" yaml:"prompt_ref,omitempty"`
	ModelRef    *schemad.Ref    `json:"model_ref,omitempty" yaml:"model_ref,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}
