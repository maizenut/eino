package job

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	RetryStrategyFixed       = "fixed"
	RetryStrategyLinear      = "linear"
	RetryStrategyExponential = "exponential"

	ConcurrencyAllow   = "allow"
	ConcurrencyForbid  = "forbid"
	ConcurrencyReplace = "replace"
)

// Scheduler manages task registration and execution.
type Scheduler interface {
	Register(ctx context.Context, task *TaskSpec, trigger *TriggerSpec, opts ...RegisterOption) (string, error)
	Unregister(ctx context.Context, taskID string) error

	Trigger(ctx context.Context, taskID string, payload map[string]any, opts ...RunOption) (*RunResult, error)
	Cancel(ctx context.Context, runID string) error

	GetTask(ctx context.Context, taskID string) (*TaskSpec, bool)
	ListTasks(ctx context.Context) ([]*TaskSpec, error)

	GetRun(ctx context.Context, runID string) (*RunResult, error)
	ListRuns(ctx context.Context, taskID string) ([]*RunResult, error)
}

type registeredTask struct {
	task    *TaskSpec
	trigger *TriggerSpec
	options registerOptions
}

type runningState struct {
	taskID string
	cancel context.CancelFunc
}

// MemoryScheduler resolves and executes tasks synchronously in memory.
type MemoryScheduler struct {
	Resolver *Resolver
	Store    ExecutionStore

	mu          sync.RWMutex
	tasks       map[string]registeredTask
	activeByRun map[string]runningState
	activeByJob map[string]string
}

// NewMemoryScheduler creates an in-memory scheduler.
func NewMemoryScheduler(resolver *Resolver, store ExecutionStore) *MemoryScheduler {
	if store == nil {
		store = NewMemoryExecutionStore()
	}
	return &MemoryScheduler{
		Resolver:    resolver,
		Store:       store,
		tasks:       make(map[string]registeredTask),
		activeByRun: make(map[string]runningState),
		activeByJob: make(map[string]string),
	}
}

func (s *MemoryScheduler) Register(ctx context.Context, task *TaskSpec, trigger *TriggerSpec, opts ...RegisterOption) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task spec is required")
	}
	taskCopy := cloneTaskSpec(task)
	if taskCopy.Info.ID == "" {
		taskCopy.Info.ID = taskIdentity(taskCopy)
	}
	if taskCopy.Info.ID == "" {
		taskCopy.Info.ID = uuid.NewString()
	}
	triggerCopy := normalizeTriggerSpec(trigger)
	regOpts := newRegisterOptions(opts...)

	s.mu.Lock()
	s.tasks[taskCopy.Info.ID] = registeredTask{task: taskCopy, trigger: triggerCopy, options: regOpts}
	s.mu.Unlock()

	if s.Store != nil {
		if err := s.Store.SaveTask(ctx, taskCopy, triggerCopy); err != nil {
			return "", err
		}
	}
	return taskCopy.Info.ID, nil
}

func (s *MemoryScheduler) Unregister(ctx context.Context, taskID string) error {
	if taskID == "" {
		return fmt.Errorf("task id is required")
	}
	s.mu.Lock()
	delete(s.tasks, taskID)
	if activeRunID, ok := s.activeByJob[taskID]; ok {
		if state, exists := s.activeByRun[activeRunID]; exists && state.cancel != nil {
			state.cancel()
		}
		delete(s.activeByJob, taskID)
		delete(s.activeByRun, activeRunID)
	}
	s.mu.Unlock()
	if s.Store != nil {
		return s.Store.DeleteTask(ctx, taskID)
	}
	return nil
}

func (s *MemoryScheduler) Trigger(ctx context.Context, taskID string, payload map[string]any, opts ...RunOption) (*RunResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}
	s.mu.RLock()
	entry, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("task %q not found", taskID)
	}
	if !entry.options.Enabled {
		return nil, fmt.Errorf("task %q is disabled", taskID)
	}

	runOpts := newRunOptions(opts...)
	trigger := normalizeTriggerSpec(entry.trigger)
	if err := s.applyConcurrencyPolicy(ctx, taskID, trigger); err != nil {
		return nil, err
	}

	runID := uuid.NewString()
	event := &TriggerEvent{
		ID:          uuid.NewString(),
		TaskID:      taskID,
		Source:      trigger.Type,
		TriggeredAt: time.Now(),
		Payload:     mergeMaps(trigger.Payload, payload),
	}
	result := &RunResult{
		ID:       runID,
		TaskID:   taskID,
		Event:    cloneTriggerEvent(event),
		Status:   RunPending,
		Metadata: copyMap(entry.options.Metadata),
	}
	if result.Metadata == nil {
		result.Metadata = map[string]any{}
	}
	if runOpts.Priority != 0 {
		result.Metadata["priority"] = runOpts.Priority
	}
	if runOpts.DryRun {
		result.Metadata["dry_run"] = true
	}
	if s.Store != nil {
		if err := s.Store.SaveRun(ctx, runID, event, result); err != nil {
			return nil, err
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	if runOpts.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(runCtx, runOpts.Timeout)
	}
	defer cancel()

	startedAt := time.Now()
	result.Status = RunRunning
	result.StartedAt = &startedAt
	if s.Store != nil {
		if err := s.Store.SaveRun(ctx, runID, event, result); err != nil {
			return nil, err
		}
	}
	s.markRunActive(taskID, runID, cancel)
	defer s.markRunDone(taskID, runID)

	if runOpts.DryRun {
		finishedAt := time.Now()
		result.Status = RunSuccess
		result.FinishedAt = &finishedAt
		if s.Store != nil {
			if err := s.Store.SaveRun(ctx, runID, event, result); err != nil {
				return nil, err
			}
		}
		return cloneRunResult(result), nil
	}

	if s.Resolver == nil {
		return nil, fmt.Errorf("job resolver is required")
	}
	runnable, err := s.Resolver.Resolve(runCtx, entry.task)
	if err != nil {
		finishedAt := time.Now()
		result.Status = RunFailed
		result.Error = err.Error()
		result.FinishedAt = &finishedAt
		if s.Store != nil {
			if saveErr := s.Store.SaveRun(ctx, runID, event, result); saveErr != nil {
				return nil, saveErr
			}
		}
		return cloneRunResult(result), nil
	}

	output, runErr := s.runWithRetry(runCtx, runnable, entry.task, trigger, event)
	finishedAt := time.Now()
	result.FinishedAt = &finishedAt
	if runErr != nil {
		if errors.Is(runErr, context.Canceled) || errors.Is(runCtx.Err(), context.Canceled) {
			result.Status = RunCancelled
		} else {
			result.Status = RunFailed
		}
		result.Error = runErr.Error()
	} else {
		result.Status = RunSuccess
		result.Output = output
	}
	if s.Store != nil {
		if err := s.Store.SaveRun(ctx, runID, event, result); err != nil {
			return nil, err
		}
	}
	return cloneRunResult(result), nil
}

func (s *MemoryScheduler) Cancel(ctx context.Context, runID string) error {
	_ = ctx
	if runID == "" {
		return fmt.Errorf("run id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.activeByRun[runID]
	if !ok {
		return fmt.Errorf("run %q is not active", runID)
	}
	if state.cancel != nil {
		state.cancel()
	}
	return nil
}

func (s *MemoryScheduler) GetTask(ctx context.Context, taskID string) (*TaskSpec, bool) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.tasks[taskID]
	if !ok {
		return nil, false
	}
	return cloneTaskSpec(entry.task), true
}

func (s *MemoryScheduler) ListTasks(ctx context.Context) ([]*TaskSpec, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*TaskSpec, 0, len(s.tasks))
	for _, entry := range s.tasks {
		out = append(out, cloneTaskSpec(entry.task))
	}
	return out, nil
}

func (s *MemoryScheduler) GetRun(ctx context.Context, runID string) (*RunResult, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("execution store is required")
	}
	return s.Store.GetRun(ctx, runID)
}

func (s *MemoryScheduler) ListRuns(ctx context.Context, taskID string) ([]*RunResult, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("execution store is required")
	}
	return s.Store.ListRuns(ctx, taskID)
}

func (s *MemoryScheduler) applyConcurrencyPolicy(ctx context.Context, taskID string, trigger *TriggerSpec) error {
	policy := ConcurrencyAllow
	if trigger != nil && trigger.Concurrency != nil && trigger.Concurrency.Mode != "" {
		policy = trigger.Concurrency.Mode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	activeRunID, hasActive := s.activeByJob[taskID]
	if !hasActive {
		return nil
	}
	switch policy {
	case "", ConcurrencyAllow:
		return nil
	case ConcurrencyForbid:
		return fmt.Errorf("task %q already has an active run %s", taskID, activeRunID)
	case ConcurrencyReplace:
		state := s.activeByRun[activeRunID]
		if state.cancel != nil {
			state.cancel()
		}
		delete(s.activeByRun, activeRunID)
		delete(s.activeByJob, taskID)
		_ = ctx
		return nil
	default:
		return fmt.Errorf("unsupported concurrency mode %s", policy)
	}
}

func (s *MemoryScheduler) runWithRetry(ctx context.Context, runnable Runnable, task *TaskSpec, trigger *TriggerSpec, event *TriggerEvent) (any, error) {
	maxAttempts := 1
	backoff := time.Duration(0)
	strategy := RetryStrategyFixed
	if trigger != nil && trigger.Retry != nil {
		if trigger.Retry.MaxAttempts > 0 {
			maxAttempts = trigger.Retry.MaxAttempts
		}
		backoff = trigger.Retry.Backoff
		if trigger.Retry.Strategy != "" {
			strategy = trigger.Retry.Strategy
		}
	}
	input := mergeMaps(task.Input, event.Payload)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		output, err := runnable.Run(ctx, input)
		if err == nil {
			return output, nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, err
		}
		if attempt == maxAttempts {
			break
		}
		if delay := retryDelay(backoff, strategy, attempt); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, lastErr
}

func retryDelay(base time.Duration, strategy string, attempt int) time.Duration {
	if base <= 0 {
		return 0
	}
	switch strategy {
	case "", RetryStrategyFixed:
		return base
	case RetryStrategyLinear:
		return time.Duration(attempt) * base
	case RetryStrategyExponential:
		factor := 1 << max(attempt-1, 0)
		return time.Duration(factor) * base
	default:
		return base
	}
}

func (s *MemoryScheduler) markRunActive(taskID, runID string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeByRun[runID] = runningState{taskID: taskID, cancel: cancel}
	s.activeByJob[taskID] = runID
}

func (s *MemoryScheduler) markRunDone(taskID, runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeByRun, runID)
	if current, ok := s.activeByJob[taskID]; ok && current == runID {
		delete(s.activeByJob, taskID)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
