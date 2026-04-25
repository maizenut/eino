package compose

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGraphLambdaInvokeAndCompileLocksGraph(t *testing.T) {
	ctx := context.Background()
	graph := NewGraph[string, string]()

	if err := graph.AddLambdaNode("upper", InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return strings.ToUpper(input), nil
	})); err != nil {
		t.Fatalf("AddLambdaNode: %v", err)
	}
	if err := graph.AddEdge(START, "upper"); err != nil {
		t.Fatalf("AddEdge start: %v", err)
	}
	if err := graph.AddEdge("upper", END); err != nil {
		t.Fatalf("AddEdge end: %v", err)
	}

	runnable, err := graph.Compile(ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	got, err := runnable.Invoke(ctx, "hello")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("Invoke output = %q, want %q", got, "HELLO")
	}

	err = graph.AddLambdaNode("late", InvokableLambda(func(ctx context.Context, input string) (string, error) {
		return input, nil
	}))
	if !errors.Is(err, ErrGraphCompiled) {
		t.Fatalf("AddLambdaNode after compile error = %v, want ErrGraphCompiled", err)
	}
}

func TestGraphRejectsInvalidConstruction(t *testing.T) {
	t.Run("reserved node", func(t *testing.T) {
		graph := NewGraph[string, string]()
		err := graph.AddLambdaNode(START, InvokableLambda(func(ctx context.Context, input string) (string, error) {
			return input, nil
		}))
		if err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("AddLambdaNode reserved error = %v, want reserved node error", err)
		}
	})

	t.Run("duplicate node", func(t *testing.T) {
		graph := NewGraph[string, string]()
		node := InvokableLambda(func(ctx context.Context, input string) (string, error) {
			return input, nil
		})
		if err := graph.AddLambdaNode("same", node); err != nil {
			t.Fatalf("AddLambdaNode first: %v", err)
		}
		err := graph.AddLambdaNode("same", node)
		if err == nil || !strings.Contains(err.Error(), "already present") {
			t.Fatalf("AddLambdaNode duplicate error = %v, want duplicate node error", err)
		}
	})

	t.Run("missing edge endpoint", func(t *testing.T) {
		graph := NewGraph[string, string]()
		err := graph.AddEdge("missing", END)
		if err == nil || !strings.Contains(err.Error(), "needs to be added") {
			t.Fatalf("AddEdge missing endpoint error = %v, want missing node error", err)
		}
	})
}

func TestChainLambdaSequenceInvoke(t *testing.T) {
	ctx := context.Background()
	chain := NewChain[string, string]().
		AppendLambda(InvokableLambda(func(ctx context.Context, input string) (string, error) {
			return strings.TrimSpace(input), nil
		})).
		AppendLambda(InvokableLambda(func(ctx context.Context, input string) (string, error) {
			return strings.ToUpper(input) + "!", nil
		}))

	runnable, err := chain.Compile(ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	got, err := runnable.Invoke(ctx, " hello ")
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if got != "HELLO!" {
		t.Fatalf("Invoke output = %q, want %q", got, "HELLO!")
	}
}

func TestChainCompileWithoutNodesReturnsError(t *testing.T) {
	_, err := NewChain[string, string]().Compile(context.Background())
	if err == nil || !strings.Contains(err.Error(), "pre node keys not set") {
		t.Fatalf("Compile empty chain error = %v, want missing pre node error", err)
	}
}
