package job

import (
	"context"
	"fmt"
)

// Service wires job loading and scheduling.
type Service struct {
	Loader    SpecLoader
	Scheduler Scheduler
}

// NewService creates a job service from explicit dependencies.
func NewService(loader SpecLoader, scheduler Scheduler) *Service {
	return &Service{Loader: loader, Scheduler: scheduler}
}

// Load reads one declarative job document from an external target.
func (s *Service) Load(ctx context.Context, target string) (*JobSpec, error) {
	if s == nil || s.Loader == nil {
		return nil, fmt.Errorf("job loader is required")
	}
	if target == "" {
		return nil, fmt.Errorf("job target is required")
	}
	return s.Loader.LoadJobSpec(ctx, target)
}

// Register stores a job declaration in the scheduler.
func (s *Service) Register(ctx context.Context, spec *JobSpec, opts ...RegisterOption) (string, error) {
	if s == nil || s.Scheduler == nil {
		return "", fmt.Errorf("job scheduler is required")
	}
	if spec == nil || spec.Task == nil {
		return "", fmt.Errorf("job spec task is required")
	}
	return s.Scheduler.Register(ctx, spec.Task, spec.Trigger, opts...)
}

// LoadAndRegister loads and registers one job declaration.
func (s *Service) LoadAndRegister(ctx context.Context, target string, opts ...RegisterOption) (*JobSpec, string, error) {
	spec, err := s.Load(ctx, target)
	if err != nil {
		return nil, "", err
	}
	taskID, err := s.Register(ctx, spec, opts...)
	if err != nil {
		return nil, "", err
	}
	return spec, taskID, nil
}

// Trigger triggers one registered task.
func (s *Service) Trigger(ctx context.Context, taskID string, payload map[string]any, opts ...RunOption) (*RunResult, error) {
	if s == nil || s.Scheduler == nil {
		return nil, fmt.Errorf("job scheduler is required")
	}
	return s.Scheduler.Trigger(ctx, taskID, payload, opts...)
}

// Cancel cancels one active run.
func (s *Service) Cancel(ctx context.Context, runID string) error {
	if s == nil || s.Scheduler == nil {
		return fmt.Errorf("job scheduler is required")
	}
	return s.Scheduler.Cancel(ctx, runID)
}

// GetTask returns one registered task.
func (s *Service) GetTask(ctx context.Context, taskID string) (*TaskSpec, bool, error) {
	if s == nil || s.Scheduler == nil {
		return nil, false, fmt.Errorf("job scheduler is required")
	}
	task, ok := s.Scheduler.GetTask(ctx, taskID)
	return task, ok, nil
}

// ListTasks returns all registered tasks.
func (s *Service) ListTasks(ctx context.Context) ([]*TaskSpec, error) {
	if s == nil || s.Scheduler == nil {
		return nil, fmt.Errorf("job scheduler is required")
	}
	return s.Scheduler.ListTasks(ctx)
}

// GetRun returns one execution result by id.
func (s *Service) GetRun(ctx context.Context, runID string) (*RunResult, error) {
	if s == nil || s.Scheduler == nil {
		return nil, fmt.Errorf("job scheduler is required")
	}
	return s.Scheduler.GetRun(ctx, runID)
}

// ListRuns returns run history, optionally filtered by task id.
func (s *Service) ListRuns(ctx context.Context, taskID string) ([]*RunResult, error) {
	if s == nil || s.Scheduler == nil {
		return nil, fmt.Errorf("job scheduler is required")
	}
	return s.Scheduler.ListRuns(ctx, taskID)
}
