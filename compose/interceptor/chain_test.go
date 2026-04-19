package interceptor

import (
	"context"
	"errors"
	"fmt"
	"testing"

	componentpkg "github.com/cloudwego/eino/components"
)

type recordingInterceptor struct {
	*BaseNodeInterceptor
	name   string
	events *[]string
}

func newRecordingInterceptor(name string, events *[]string) *recordingInterceptor {
	return &recordingInterceptor{
		BaseNodeInterceptor: &BaseNodeInterceptor{},
		name:                name,
		events:              events,
	}
}

func (r *recordingInterceptor) BeforeNode(ctx context.Context, info NodeInfo, input any) (context.Context, any, error) {
	*r.events = append(*r.events, fmt.Sprintf("before:%s:%s", r.name, info.Key))
	return ctx, input, nil
}

func (r *recordingInterceptor) AfterNode(ctx context.Context, info NodeInfo, output any) (context.Context, any, error) {
	*r.events = append(*r.events, fmt.Sprintf("after:%s:%s", r.name, info.Key))
	return ctx, output, nil
}

func (r *recordingInterceptor) OnErrorNode(ctx context.Context, info NodeInfo, err error) (context.Context, error) {
	*r.events = append(*r.events, fmt.Sprintf("error:%s:%s:%s", r.name, info.Key, err.Error()))
	return ctx, err
}

func (r *recordingInterceptor) WrapNode(ctx context.Context, info NodeInfo, next NodeExecutor) NodeExecutor {
	return func(execCtx context.Context, input any) (any, error) {
		*r.events = append(*r.events, fmt.Sprintf("wrap-enter:%s:%s", r.name, info.Key))
		output, err := next(execCtx, input)
		*r.events = append(*r.events, fmt.Sprintf("wrap-exit:%s:%s", r.name, info.Key))
		return output, err
	}
}

func TestRunOrder(t *testing.T) {
	events := make([]string, 0, 16)
	info := NodeInfo{Key: "chat", Component: componentpkg.ComponentOfChatModel}
	output, err := Run(context.Background(), info, "in", []NodeInterceptor{
		newRecordingInterceptor("A", &events),
		newRecordingInterceptor("B", &events),
	}, func(ctx context.Context, input any) (any, error) {
		events = append(events, "exec")
		return "out", nil
	})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	if output != "out" {
		t.Fatalf("output = %#v, want out", output)
	}

	want := []string{
		"before:A:chat",
		"before:B:chat",
		"wrap-enter:A:chat",
		"wrap-enter:B:chat",
		"exec",
		"wrap-exit:B:chat",
		"wrap-exit:A:chat",
		"after:B:chat",
		"after:A:chat",
	}
	if len(events) != len(want) {
		t.Fatalf("events len = %d, want %d, events=%v", len(events), len(want), events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events[%d] = %s, want %s; all=%v", i, events[i], want[i], events)
		}
	}
}

type errorWrappingInterceptor struct {
	*BaseNodeInterceptor
}

func (e *errorWrappingInterceptor) OnErrorNode(ctx context.Context, info NodeInfo, err error) (context.Context, error) {
	return ctx, fmt.Errorf("%s:%w", info.Key, err)
}

func TestRunError(t *testing.T) {
	info := NodeInfo{Key: "tool"}
	_, err := Run(context.Background(), info, nil, []NodeInterceptor{
		&errorWrappingInterceptor{BaseNodeInterceptor: &BaseNodeInterceptor{}},
	}, func(ctx context.Context, input any) (any, error) {
		return nil, errors.New("boom")
	})
	if err == nil || err.Error() != "tool:boom" {
		t.Fatalf("error = %v, want tool:boom", err)
	}
}
