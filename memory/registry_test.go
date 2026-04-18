package memory

import (
	"context"
	"testing"

	schemad "github.com/cloudwego/eino/schema/declarative"
)

func TestMemoryRegistry(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRegistry()

	spec := &MemorySpec{
		Info:     Info{Name: "test-memory", Description: "unit test"},
		StoreRef: schemad.Ref{Kind: schemad.RefKindComponentDocument, Target: "inmemory"},
	}

	if err := r.Register(ctx, spec); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, ok := r.Get(ctx, "test-memory")
	if !ok {
		t.Fatal("Get: not found after Register")
	}
	if got.Info.Name != spec.Info.Name {
		t.Fatalf("Get: want name %q, got %q", spec.Info.Name, got.Info.Name)
	}

	list := r.List(ctx)
	if len(list) != 1 || list[0].Name != "test-memory" {
		t.Fatalf("List: unexpected result %v", list)
	}

	if err := r.Unregister(ctx, "test-memory"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if _, ok = r.Get(ctx, "test-memory"); ok {
		t.Fatal("Get: should not find spec after Unregister")
	}
}

func TestMemoryRegistry_Validation(t *testing.T) {
	ctx := context.Background()
	r := NewMemoryRegistry()

	if err := r.Register(ctx, nil); err == nil {
		t.Fatal("expected error registering nil spec")
	}
	if err := r.Register(ctx, &MemorySpec{}); err == nil {
		t.Fatal("expected error registering spec with empty name")
	}
	if err := r.Unregister(ctx, ""); err == nil {
		t.Fatal("expected error unregistering empty name")
	}
}
