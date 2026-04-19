package compose

import (
	"context"
	"testing"

	mempkg "github.com/cloudwego/eino/composite/memory"
)

type stubMemoryBinding struct {
	beforeCalls []string
	afterCalls  []string
}

func (b *stubMemoryBinding) BeforeNode(ctx context.Context, nodeKey string, input any) (context.Context, any, error) {
	b.beforeCalls = append(b.beforeCalls, nodeKey)
	return context.WithValue(ctx, MemoryBindingContextKey("memory.records"), "ok"), input, nil
}

func (b *stubMemoryBinding) AfterNode(ctx context.Context, nodeKey string, output any) (context.Context, any, error) {
	b.afterCalls = append(b.afterCalls, nodeKey)
	return ctx, output, nil
}

type stubRuntimeMemory struct {
	binding mempkg.Binding
}

func (m *stubRuntimeMemory) Info() mempkg.Info { return mempkg.Info{Name: "stub"} }
func (m *stubRuntimeMemory) Scope(ctx context.Context) mempkg.ScopeSpec {
	_ = ctx
	return mempkg.ScopeSpec{Name: "scope"}
}
func (m *stubRuntimeMemory) Write(ctx context.Context, records []*mempkg.Record, opts ...mempkg.WriteOption) error {
	_ = ctx
	_ = records
	_ = opts
	return nil
}
func (m *stubRuntimeMemory) Recall(ctx context.Context, req *mempkg.QueryRequest, opts ...mempkg.RecallOption) ([]*mempkg.Record, error) {
	_ = ctx
	_ = req
	_ = opts
	return nil, nil
}
func (m *stubRuntimeMemory) Compact(ctx context.Context, opts ...mempkg.Option) error {
	_ = ctx
	_ = opts
	return nil
}
func (m *stubRuntimeMemory) WithScope(scope mempkg.ScopeSpec) mempkg.RuntimeMemory {
	_ = scope
	return m
}
func (m *stubRuntimeMemory) Binding(ctx context.Context) (mempkg.Binding, bool, error) {
	_ = ctx
	return m.binding, m.binding != nil, nil
}

type stubMemoryAssembler struct {
	built int
	mem   mempkg.RuntimeMemory
}

func (a *stubMemoryAssembler) Build(ctx context.Context, spec *mempkg.MemorySpec) (mempkg.RuntimeMemory, error) {
	_ = ctx
	_ = spec
	a.built++
	return a.mem, nil
}

func TestMemoryNodeInterceptorUsesBinding(t *testing.T) {
	binding := &stubMemoryBinding{}
	interceptorInstance := NewMemoryNodeInterceptor(&stubRuntimeMemory{binding: binding}, nil, nil, nil)
	ctx, input, err := interceptorInstance.BeforeNode(context.Background(), NodeInfo{Key: "chat"}, "in")
	if err != nil {
		t.Fatalf("BeforeNode error = %v", err)
	}
	if input != "in" {
		t.Fatalf("input = %#v, want in", input)
	}
	if got := ctx.Value(MemoryBindingContextKey("memory.records")); got != "ok" {
		t.Fatalf("context value = %#v, want ok", got)
	}
	_, output, err := interceptorInstance.AfterNode(context.Background(), NodeInfo{Key: "chat"}, "out")
	if err != nil {
		t.Fatalf("AfterNode error = %v", err)
	}
	if output != "out" {
		t.Fatalf("output = %#v, want out", output)
	}
	if len(binding.beforeCalls) != 1 || binding.beforeCalls[0] != "chat" {
		t.Fatalf("before calls = %#v", binding.beforeCalls)
	}
	if len(binding.afterCalls) != 1 || binding.afterCalls[0] != "chat" {
		t.Fatalf("after calls = %#v", binding.afterCalls)
	}
}

func TestMemoryNodeInterceptorBuildsFromSpec(t *testing.T) {
	binding := &stubMemoryBinding{}
	assembler := &stubMemoryAssembler{mem: &stubRuntimeMemory{binding: binding}}
	interceptorInstance := NewMemoryNodeInterceptor(nil, nil, &mempkg.MemorySpec{Info: mempkg.Info{Name: "spec"}}, assembler)
	ctx, _, err := interceptorInstance.BeforeNode(context.Background(), NodeInfo{Key: "node-1"}, nil)
	if err != nil {
		t.Fatalf("BeforeNode error = %v", err)
	}
	if got := ctx.Value(MemoryBindingContextKey("memory.records")); got != "ok" {
		t.Fatalf("context value = %#v, want ok", got)
	}
	if assembler.built != 1 {
		t.Fatalf("assembler built = %d, want 1", assembler.built)
	}
}

func TestMemoryCompileOptionsAutoWireInterceptor(t *testing.T) {
	binding := &stubMemoryBinding{}
	memoryRuntime := &stubRuntimeMemory{binding: binding}
	options := newGraphCompileOptions(WithMemory(memoryRuntime))
	if len(options.nodeInterceptors) != 1 {
		t.Fatalf("len(nodeInterceptors) = %d, want 1", len(options.nodeInterceptors))
	}
	if _, ok := options.nodeInterceptors[0].(*memoryNodeInterceptor); !ok {
		t.Fatalf("interceptor type = %T, want *memoryNodeInterceptor", options.nodeInterceptors[0])
	}
}

func TestMemoryCompileOptionsPreferSingleInterceptor(t *testing.T) {
	binding := &stubMemoryBinding{}
	memoryRuntime := &stubRuntimeMemory{binding: binding}
	existing := NewMemoryNodeInterceptor(memoryRuntime, nil, nil, nil)
	options := newGraphCompileOptions(
		WithNodeInterceptorsOnCompile(existing),
		WithMemory(memoryRuntime),
	)
	if len(options.nodeInterceptors) != 1 {
		t.Fatalf("len(nodeInterceptors) = %d, want 1", len(options.nodeInterceptors))
	}
}

func TestWithMemoryAssemblerStoresAssembler(t *testing.T) {
	assembler := &stubMemoryAssembler{}
	options := MemoryOptionsFromCompileOptions(WithMemoryAssembler(assembler))
	if options.Assembler != assembler {
		t.Fatalf("assembler = %#v, want %#v", options.Assembler, assembler)
	}
}

func TestMemoryCompileOptionsPassAssemblerToInterceptor(t *testing.T) {
	binding := &stubMemoryBinding{}
	assembler := &stubMemoryAssembler{mem: &stubRuntimeMemory{binding: binding}}
	options := newGraphCompileOptions(
		WithMemorySpec(&mempkg.MemorySpec{Info: mempkg.Info{Name: "spec"}}),
		WithMemoryAssembler(assembler),
	)
	if len(options.nodeInterceptors) != 1 {
		t.Fatalf("len(nodeInterceptors) = %d, want 1", len(options.nodeInterceptors))
	}
	memoryInterceptor, ok := options.nodeInterceptors[0].(*memoryNodeInterceptor)
	if !ok {
		t.Fatalf("interceptor type = %T, want *memoryNodeInterceptor", options.nodeInterceptors[0])
	}
	ctx, _, err := memoryInterceptor.BeforeNode(context.Background(), NodeInfo{Key: "node-2"}, nil)
	if err != nil {
		t.Fatalf("BeforeNode error = %v", err)
	}
	if got := ctx.Value(MemoryBindingContextKey("memory.records")); got != "ok" {
		t.Fatalf("context value = %#v, want ok", got)
	}
	if assembler.built != 1 {
		t.Fatalf("assembler built = %d, want 1", assembler.built)
	}
}
