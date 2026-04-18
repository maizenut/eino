package memory

import "time"

const (
	// Record type constants.
	RecordTypeMessage    = "message"
	RecordTypeFact       = "fact"
	RecordTypeSummary    = "summary"
	RecordTypeToolResult = "tool_result"

	// Scope kind constants.
	ScopeKindSession = "session"
	ScopeKindUser    = "user"
	ScopeKindAgent   = "agent"
	ScopeKindTenant  = "tenant"
	ScopeKindApp     = "app"
)

// Record is the unified memory record stored and retrieved by a Store.
type Record struct {
	ID         string         `json:"id"`
	Namespace  string         `json:"namespace,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	Content    string         `json:"content"`
	Summary    string         `json:"summary,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
	Embedding  []float64      `json:"embedding,omitempty"`
	Score      float64        `json:"score,omitempty"`
	Source     string         `json:"source,omitempty"`
	RecordType string         `json:"record_type,omitempty"`
}

// QueryRequest describes a single memory recall request.
type QueryRequest struct {
	// Query is the natural-language or semantic query string.
	Query string `json:"query,omitempty"`
	// TopK limits the number of results returned.
	TopK int `json:"top_k,omitempty"`
	// MinScore filters results below this relevance threshold.
	MinScore float64 `json:"min_score,omitempty"`
	// Scope restricts recall to a single scope name.
	Scope string `json:"scope,omitempty"`
	// ReadScopes is an ordered fallback chain of scope names to read from.
	ReadScopes []string `json:"read_scopes,omitempty"`
	// Filter applies metadata-level predicate filters.
	Filter map[string]any `json:"filter,omitempty"`
	// TimeRange restricts recall to records within the given window.
	TimeRange *TimeRange `json:"time_range,omitempty"`
	// Intent is an optional hint used by query-rewrite or rerank policies.
	Intent string `json:"intent,omitempty"`
}

// TimeRange bounds a recall request by creation time.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ListRequest describes a paginated list request against a Store.
type ListRequest struct {
	// Scope restricts listing to a single scope name.
	Scope string `json:"scope,omitempty"`
	// Filter applies metadata-level predicate filters.
	Filter map[string]any `json:"filter,omitempty"`
	// Offset is the zero-based starting position.
	Offset int `json:"offset,omitempty"`
	// Limit is the maximum number of records returned; 0 means no limit.
	Limit int `json:"limit,omitempty"`
}
