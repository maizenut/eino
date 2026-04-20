package declarative

const (
	GraphTypeSequential = "sequential"
	GraphTypeParallel   = "parallel"
	GraphTypeLoop       = "loop"
)

// WorkflowStepSpec describes one step/worker binding inside a workflow preset.
type WorkflowStepSpec struct {
	Key       string         `json:"key"`
	Ref       Ref            `json:"ref"`
	InputKey  string         `json:"input_key,omitempty"`
	OutputKey string         `json:"output_key,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// SequentialGraphDraft declares a deterministic sequential workflow preset.
type SequentialGraphDraft struct {
	Name              string             `json:"name"`
	EnterCallableRef  *Ref               `json:"enter,omitempty"`
	FinishCallableRef *Ref               `json:"finish,omitempty"`
	Steps             []WorkflowStepSpec `json:"steps,omitempty"`
	Metadata          map[string]any     `json:"metadata,omitempty"`
}

// ParallelGraphDraft declares a deterministic fanout-join workflow preset.
type ParallelGraphDraft struct {
	Name              string             `json:"name"`
	EnterCallableRef  *Ref               `json:"enter,omitempty"`
	FinishCallableRef *Ref               `json:"finish,omitempty"`
	Workers           []WorkflowStepSpec `json:"workers,omitempty"`
	JoinCallableRef   *Ref               `json:"join,omitempty"`
	MaxConcurrency    int                `json:"max_concurrency,omitempty"`
	Metadata          map[string]any     `json:"metadata,omitempty"`
}

// LoopGraphDraft declares a deterministic loop preset that compiles to a typed runnable.
type LoopGraphDraft struct {
	Name                 string         `json:"name"`
	EnterCallableRef     *Ref           `json:"enter,omitempty"`
	ConditionCallableRef *Ref           `json:"condition,omitempty"`
	BodyGraphRef         *Ref           `json:"body,omitempty"`
	AfterBodyCallableRef *Ref           `json:"after_body,omitempty"`
	FinishCallableRef    *Ref           `json:"finish,omitempty"`
	MaxIterations        int            `json:"max_iterations,omitempty"`
	PersistedFields      []string       `json:"persisted_fields,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}
