package memory

import "context"

type ActionKind string

const (
	ActionReckon  ActionKind = "reckon"
	ActionPonder  ActionKind = "ponder"
	ActionInspire ActionKind = "inspire"
	ActionSummary ActionKind = "summary"
	ActionImprint ActionKind = "imprint"
	ActionForget  ActionKind = "forget"
)

func IsReadAction(kind ActionKind) bool {
	switch kind {
	case ActionReckon, ActionPonder, ActionInspire:
		return true
	}
	return false
}

func IsWriteAction(kind ActionKind) bool {
	switch kind {
	case ActionSummary, ActionImprint:
		return true
	}
	return false
}

type PolicyOverride struct {
	TopK      *int     `json:"top_k,omitempty"`
	MinScore  *float64 `json:"min_score,omitempty"`
	WriteMode *string  `json:"write_mode,omitempty"`
	Scope     *string  `json:"scope,omitempty"`
	Namespace *string  `json:"namespace,omitempty"`
}

type ActionRequest struct {
	Kind           ActionKind      `json:"kind"`
	Query          string          `json:"query,omitempty"`
	Content        string          `json:"content,omitempty"`
	Meta           map[string]any  `json:"meta,omitempty"`
	PolicyOverride *PolicyOverride `json:"policy_override,omitempty"`
}

type RecordMeta struct {
	ID    string  `json:"id"`
	Title string  `json:"title,omitempty"`
	Store string  `json:"store"`
	Scope string  `json:"scope,omitempty"`
	Score float64 `json:"score,omitempty"`
}

type ActionResult struct {
	Records  []*Record      `json:"records,omitempty"`
	Metadata []*RecordMeta  `json:"metadata,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

type ActionHandler interface {
	HandleAction(ctx context.Context, req *ActionRequest, opts ...Option) (*ActionResult, error)
	SupportedActions() []ActionKind
}
