package job

import (
	"time"

	declarative "github.com/cloudwego/eino/schema/declarative"
)

const (
	TargetTypeGraph    = "graph"
	TargetTypeSkill    = "skill"
	TargetTypeRunnable = "runnable"
	TargetTypeFunction = "function"

	TriggerTypeManual = "manual"
	TriggerTypeCron   = "cron"
	TriggerTypeDelay  = "delay"
	TriggerTypeOnce   = "once"
	TriggerTypeEvent  = "event"
)

// TaskInfo describes the basic metadata of one task.
type TaskInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Hints       []string       `json:"hints,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TaskSpec declares what one job task should run.
type TaskSpec struct {
	Info       TaskInfo        `json:"info"`
	TargetType string          `json:"target_type,omitempty"`
	TargetRef  declarative.Ref `json:"target_ref"`
	Input      map[string]any  `json:"input,omitempty"`
	Metadata   map[string]any  `json:"metadata,omitempty"`
}

// TriggerSpec declares when and how a task should run.
type TriggerSpec struct {
	Type        string             `json:"type,omitempty"`
	Cron        string             `json:"cron,omitempty"`
	Delay       time.Duration      `json:"delay,omitempty"`
	At          *time.Time         `json:"at,omitempty"`
	Event       string             `json:"event,omitempty"`
	Timezone    string             `json:"timezone,omitempty"`
	Payload     map[string]any     `json:"payload,omitempty"`
	Retry       *RetryPolicy       `json:"retry,omitempty"`
	Concurrency *ConcurrencyPolicy `json:"concurrency,omitempty"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
}

// RetryPolicy describes retry behavior for one trigger.
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts,omitempty"`
	Backoff     time.Duration `json:"backoff,omitempty"`
	Strategy    string        `json:"strategy,omitempty"`
}

// ConcurrencyPolicy describes how concurrent runs are handled.
type ConcurrencyPolicy struct {
	Mode string `json:"mode,omitempty"`
}

// TriggerEvent describes one concrete trigger occurrence.
type TriggerEvent struct {
	ID          string         `json:"id"`
	TaskID      string         `json:"task_id"`
	Source      string         `json:"source,omitempty"`
	TriggeredAt time.Time      `json:"triggered_at"`
	ScheduledAt *time.Time     `json:"scheduled_at,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

// RunStatus describes the lifecycle state of one execution.
type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunSuccess   RunStatus = "success"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

// RunResult describes one execution result.
type RunResult struct {
	ID         string         `json:"id"`
	TaskID     string         `json:"task_id,omitempty"`
	Event      *TriggerEvent  `json:"event,omitempty"`
	Status     RunStatus      `json:"status"`
	Output     any            `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// JobSpec is the persisted declarative document for one task registration.
type JobSpec struct {
	Task    *TaskSpec    `json:"task"`
	Trigger *TriggerSpec `json:"trigger,omitempty"`
}
