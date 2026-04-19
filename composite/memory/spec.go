package memory

import schemad "github.com/cloudwego/eino/schema/declarative"

const (
	// Trigger strategy constants.
	TriggerStrategyTurnEnd      = "turn_end"
	TriggerStrategyNodeEnd      = "node_end"
	TriggerStrategyManual       = "manual"
	TriggerStrategyWindowExceed = "window_exceed"

	// Write mode constants.
	WriteModeAppend  = "append"
	WriteModeUpsert  = "upsert"
	WriteModeExtract = "extract"

	// Merge strategy constants for RecallPolicySpec.
	MergeStrategyConcat = "concat"
	MergeStrategyDedup  = "dedup"
	MergeStrategyRerank = "rerank"

	// Inject-as constants for RecallPolicySpec.
	InjectAsMessages     = "messages"
	InjectAsContext      = "context"
	InjectAsSystemPrompt = "system_prompt"

	// Compaction strategy constants.
	CompactionStrategySummarize     = "summarize"
	CompactionStrategyEvict         = "evict"
	CompactionStrategyRollingWindow = "rolling_window"

	// Binding mode constants.
	BindingModeGraph  = "graph"
	BindingModeAgent  = "agent"
	BindingModeManual = "manual"
)

// Info holds the basic metadata of a memory system.
type Info struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ScopeSpec describes a memory scope including its kind, parent hierarchy,
// and read/write routing.
type ScopeSpec struct {
	// Name is the unique identifier for this scope instance (e.g. a session ID).
	Name string `json:"name"`
	// Kind categorises the scope: session, user, agent, tenant, or app.
	Kind string `json:"kind,omitempty"`
	// Parent is the name of the enclosing scope; used for inheritance lookups.
	Parent string `json:"parent,omitempty"`
	// ReadFrom is an ordered list of scope names to fall back to during recall.
	ReadFrom []string `json:"read_from,omitempty"`
	// WriteTo is the target scope name for write operations; defaults to Name.
	WriteTo  string         `json:"write_to,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TriggerSpec describes when a write, recall, or compaction operation fires.
type TriggerSpec struct {
	// Strategy names the trigger type (e.g. turn_end, node_end, manual).
	Strategy string `json:"strategy,omitempty"`
	// Events is the set of lifecycle event names that activate the trigger.
	Events []string `json:"events,omitempty"`
	// Params holds strategy-specific configuration values.
	Params map[string]any `json:"params,omitempty"`
}

// WritePolicySpec declares when and how records are written to the store.
type WritePolicySpec struct {
	// Trigger controls when the write fires.
	Trigger *TriggerSpec `json:"trigger,omitempty"`
	// Mode is the write behaviour: append, upsert, or extract.
	Mode string `json:"mode,omitempty"`
	// ExtractRef points to a component or function that structures raw content
	// before writing (e.g. an entity extractor or fact parser).
	ExtractRef *schemad.Ref `json:"extract_ref,omitempty"`
	// SummarizeBefore requests a summary to be generated before writing.
	SummarizeBefore bool           `json:"summarize_before,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// RecallPolicySpec declares how records are retrieved and injected.
type RecallPolicySpec struct {
	// QueryRewriteRef points to a component or function that rewrites the query
	// before vector or keyword search.
	QueryRewriteRef *schemad.Ref `json:"query_rewrite_ref,omitempty"`
	// RerankRef points to a reranker applied after initial retrieval.
	RerankRef *schemad.Ref `json:"rerank_ref,omitempty"`
	// MergeStrategy controls how results from multiple scopes are merged.
	MergeStrategy string `json:"merge_strategy,omitempty"`
	// TopK is the maximum number of records to return.
	TopK int `json:"top_k,omitempty"`
	// MinScore is the minimum relevance score a record must have to be returned.
	MinScore float64 `json:"min_score,omitempty"`
	// InjectAs controls how recalled records are surfaced to the model/agent.
	InjectAs string         `json:"inject_as,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// CompactionPolicySpec declares how stored records are compressed or evicted.
type CompactionPolicySpec struct {
	// Trigger controls when compaction fires.
	Trigger *TriggerSpec `json:"trigger,omitempty"`
	// Strategy names the compaction approach: summarize, evict, or rolling_window.
	Strategy string `json:"strategy,omitempty"`
	// SummaryRef points to a summariser component or function.
	SummaryRef *schemad.Ref `json:"summary_ref,omitempty"`
	// MaxRecords is the upper bound on stored records before compaction triggers.
	MaxRecords int `json:"max_records,omitempty"`
	// MaxTokens is the estimated token ceiling before compaction triggers.
	MaxTokens int `json:"max_tokens,omitempty"`
	// KeepLastN preserves the N most recent records during eviction.
	KeepLastN int            `json:"keep_last_n,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// BindingSpec declares how a RuntimeMemory is wired into graph/agent lifecycle.
type BindingSpec struct {
	// Mode is the binding approach: graph, agent, or manual.
	Mode string `json:"mode,omitempty"`
	// BeforeNodes lists node keys whose input the binding intercepts for recall.
	BeforeNodes []string `json:"before_nodes,omitempty"`
	// AfterNodes lists node keys whose output the binding intercepts for writing.
	AfterNodes []string `json:"after_nodes,omitempty"`
	// InjectKey is the context or state key used to inject recalled records.
	InjectKey string `json:"inject_key,omitempty"`
	// OutputKey is the context or state key used to capture written records.
	OutputKey string         `json:"output_key,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// MemorySpec is the static declarative definition of a memory system.
// It is serialisable to JSON/YAML and consumed by an Assembler to produce
// a RuntimeMemory.
type MemorySpec struct {
	Info            Info                  `json:"info"`
	Scope           *ScopeSpec            `json:"scope,omitempty"`
	StoreRef        schemad.Ref           `json:"store_ref"`
	RecallStoreRefs []schemad.Ref         `json:"recall_store_refs,omitempty"`
	PrimaryWriteRef *schemad.Ref          `json:"primary_write_ref,omitempty"`
	EmbedRef        *schemad.Ref          `json:"embed_ref,omitempty"`
	IndexRef        *schemad.Ref          `json:"index_ref,omitempty"`
	WritePolicy     *WritePolicySpec      `json:"write_policy,omitempty"`
	RecallPolicy    *RecallPolicySpec     `json:"recall_policy,omitempty"`
	CompactPolicy   *CompactionPolicySpec `json:"compact_policy,omitempty"`
	Binding         *BindingSpec          `json:"binding,omitempty"`
	Metadata        map[string]any        `json:"metadata,omitempty"`
}
