package compose

import (
	"context"
	"testing"
)

type checkpointRunContextTestStore struct{}

func (s *checkpointRunContextTestStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	return nil, false, nil
}

func (s *checkpointRunContextTestStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	return nil
}

func TestGraphRunSetsCheckPointRunContext(t *testing.T) {
	ctx := context.Background()
	graph := NewGraph[string, string]()

	var seenCheckPointID string
	var seenWriteToCheckPointID string
	if err := graph.AddLambdaNode("capture", InvokableLambda(func(ctx context.Context, input string) (string, error) {
		seenCheckPointID, _ = CheckPointIDFromContext(ctx)
		seenWriteToCheckPointID, _ = WriteToCheckPointIDFromContext(ctx)
		return input, nil
	})); err != nil {
		t.Fatalf("AddLambdaNode: %v", err)
	}
	if err := graph.AddEdge(START, "capture"); err != nil {
		t.Fatalf("AddEdge start: %v", err)
	}
	if err := graph.AddEdge("capture", END); err != nil {
		t.Fatalf("AddEdge end: %v", err)
	}

	runnable, err := graph.Compile(ctx, WithCheckPointStore(&checkpointRunContextTestStore{}))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if _, err := runnable.Invoke(ctx, "input", WithCheckPointID("checkpoint-a"), WithWriteToCheckPointID("checkpoint-b")); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if seenCheckPointID != "checkpoint-a" {
		t.Fatalf("CheckPointIDFromContext = %q, want %q", seenCheckPointID, "checkpoint-a")
	}
	if seenWriteToCheckPointID != "checkpoint-b" {
		t.Fatalf("WriteToCheckPointIDFromContext = %q, want %q", seenWriteToCheckPointID, "checkpoint-b")
	}
}
