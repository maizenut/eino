package job

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ExecutionStore persists task declarations and execution history.
type ExecutionStore interface {
	SaveTask(ctx context.Context, task *TaskSpec, trigger *TriggerSpec) error
	DeleteTask(ctx context.Context, taskID string) error

	SaveRun(ctx context.Context, runID string, event *TriggerEvent, result *RunResult) error
	GetRun(ctx context.Context, runID string) (*RunResult, error)
	ListRuns(ctx context.Context, taskID string) ([]*RunResult, error)
}

// MemoryExecutionStore keeps tasks and run history in memory.
type MemoryExecutionStore struct {
	mu    sync.RWMutex
	tasks map[string]storedTask
	runs  map[string]storedRun
}

type storedTask struct {
	task    *TaskSpec
	trigger *TriggerSpec
}

type storedRun struct {
	event  *TriggerEvent
	result *RunResult
}

// NewMemoryExecutionStore creates an empty in-memory execution store.
func NewMemoryExecutionStore() *MemoryExecutionStore {
	return &MemoryExecutionStore{
		tasks: make(map[string]storedTask),
		runs:  make(map[string]storedRun),
	}
}

func (s *MemoryExecutionStore) SaveTask(ctx context.Context, task *TaskSpec, trigger *TriggerSpec) error {
	_ = ctx
	if task == nil {
		return fmt.Errorf("task spec is required")
	}
	taskID := taskIdentity(task)
	if taskID == "" {
		return fmt.Errorf("task id or name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[taskID] = storedTask{task: cloneTaskSpec(task), trigger: cloneTriggerSpec(trigger)}
	return nil
}

func (s *MemoryExecutionStore) DeleteTask(ctx context.Context, taskID string) error {
	_ = ctx
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, taskID)
	return nil
}

func (s *MemoryExecutionStore) SaveRun(ctx context.Context, runID string, event *TriggerEvent, result *RunResult) error {
	_ = ctx
	if runID == "" {
		return fmt.Errorf("run id is required")
	}
	if result == nil {
		return fmt.Errorf("run result is required")
	}
	cloned := cloneRunResult(result)
	cloned.ID = runID
	cloned.Event = cloneTriggerEvent(event)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[runID] = storedRun{event: cloneTriggerEvent(event), result: cloned}
	return nil
}

func (s *MemoryExecutionStore) GetRun(ctx context.Context, runID string) (*RunResult, error) {
	_ = ctx
	if runID == "" {
		return nil, fmt.Errorf("run id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.runs[runID]
	if !ok {
		return nil, fmt.Errorf("run %q not found", runID)
	}
	result := cloneRunResult(entry.result)
	result.Event = cloneTriggerEvent(entry.event)
	return result, nil
}

func (s *MemoryExecutionStore) ListRuns(ctx context.Context, taskID string) ([]*RunResult, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*RunResult, 0, len(s.runs))
	for _, entry := range s.runs {
		if taskID != "" && entry.result != nil && entry.result.TaskID != taskID {
			continue
		}
		result := cloneRunResult(entry.result)
		result.Event = cloneTriggerEvent(entry.event)
		out = append(out, result)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i] == nil || out[i].StartedAt == nil {
			return false
		}
		if out[j] == nil || out[j].StartedAt == nil {
			return true
		}
		return out[i].StartedAt.Before(*out[j].StartedAt)
	})
	return out, nil
}
