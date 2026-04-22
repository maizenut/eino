package job

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
	orcbp "github.com/maizenut/mirorru/orchestration/blueprint"
)

type fakeInterpreterResolver struct {
	functions map[string]any
}

func (r *fakeInterpreterResolver) ResolveObject(ctx context.Context, ref schemad.Ref) (any, error) {
	return r.ResolveFunction(ctx, ref)
}

func (r *fakeInterpreterResolver) ResolveFunction(_ context.Context, ref schemad.Ref) (any, error) {
	return r.functions[ref.Target], nil
}

func (r *fakeInterpreterResolver) ResolveComponent(context.Context, schemad.Ref) (any, error) {
	return nil, nil
}

func (r *fakeInterpreterResolver) ResolveGraph(context.Context, schemad.Ref) (compose.AnyGraph, error) {
	return nil, nil
}

func newTestScheduler(fn any) *MemoryScheduler {
	resolver := &Resolver{InterpreterResolver: &fakeInterpreterResolver{functions: map[string]any{"job.run": fn}}}
	return NewMemoryScheduler(resolver, NewMemoryExecutionStore())
}

func waitForRuns(t *testing.T, scheduler *MemoryScheduler, taskID string, wantAtLeast int, timeout time.Duration) []*RunResult {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runs, err := scheduler.ListRuns(context.Background(), taskID)
		if err != nil {
			t.Fatalf("ListRuns: %v", err)
		}
		if len(runs) >= wantAtLeast {
			return runs
		}
		time.Sleep(10 * time.Millisecond)
	}
	runs, err := scheduler.ListRuns(context.Background(), taskID)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	t.Fatalf("runs len = %d, want at least %d", len(runs), wantAtLeast)
	return nil
}

func TestMemorySchedulerDelayTrigger(t *testing.T) {
	scheduler := newTestScheduler(func(ctx context.Context, input map[string]any) (any, error) {
		_ = ctx
		return input["source"], nil
	})
	taskID, err := scheduler.Register(context.Background(), &TaskSpec{
		Info:      TaskInfo{ID: "delay-task"},
		TargetRef: schemad.Ref{Kind: schemad.RefKindInterpreterFunction, Target: "job.run"},
	}, &TriggerSpec{Type: TriggerTypeDelay, Delay: 20 * time.Millisecond, Payload: map[string]any{"source": "delay"}})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	runs := waitForRuns(t, scheduler, taskID, 1, 500*time.Millisecond)
	if runs[0].Output != "delay" {
		t.Fatalf("unexpected output: %#v", runs[0])
	}
}

func TestMemorySchedulerEventTrigger(t *testing.T) {
	scheduler := newTestScheduler(func(ctx context.Context, input map[string]any) (any, error) {
		_ = ctx
		return input["event"], nil
	})
	taskID, err := scheduler.Register(context.Background(), &TaskSpec{
		Info:      TaskInfo{ID: "event-task"},
		TargetRef: schemad.Ref{Kind: schemad.RefKindInterpreterFunction, Target: "job.run"},
	}, &TriggerSpec{Type: TriggerTypeEvent, Event: "user.created"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	results, err := scheduler.EmitEvent(context.Background(), "user.created", map[string]any{"event": "user.created"})
	if err != nil {
		t.Fatalf("EmitEvent: %v", err)
	}
	if len(results) != 1 || results[0].TaskID != taskID {
		t.Fatalf("unexpected event results: %#v", results)
	}
}

func TestMemorySchedulerCronEveryTrigger(t *testing.T) {
	scheduler := newTestScheduler(func(ctx context.Context, input map[string]any) (any, error) {
		_ = ctx
		return input, nil
	})
	taskID, err := scheduler.Register(context.Background(), &TaskSpec{
		Info:      TaskInfo{ID: "cron-task"},
		TargetRef: schemad.Ref{Kind: schemad.RefKindInterpreterFunction, Target: "job.run"},
	}, &TriggerSpec{Type: TriggerTypeCron, Cron: "@every 20ms"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	defer func() {
		if err := scheduler.Unregister(context.Background(), taskID); err != nil {
			t.Fatalf("Unregister: %v", err)
		}
	}()
	waitForRuns(t, scheduler, taskID, 2, 800*time.Millisecond)
	if _, ok := any(scheduler).(EventScheduler); !ok {
		t.Fatal("scheduler should satisfy EventScheduler")
	}
}

var _ orcbp.InterpreterResolver = (*fakeInterpreterResolver)(nil)
