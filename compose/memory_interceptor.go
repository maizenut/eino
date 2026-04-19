package compose

import (
	"context"
	"fmt"

	mempkg "github.com/cloudwego/eino/composite/memory"
)

// MemoryBindingContextKey returns the context key used by the default memory interceptor.
//
// Deprecated: default memory binding adapters now live under
// github.com/maizenut/mirorru/interceptors/memorybinding.
func MemoryBindingContextKey(key string) any {
	return memoryBindingContextKey(key)
}

type memoryBindingContextKey string

// MemoryAssembler materializes RuntimeMemory from a declarative spec when needed.
//
// Deprecated: prefer github.com/maizenut/mirorru/interceptors/memorybinding.MemoryAssembler.
type MemoryAssembler interface {
	Build(ctx context.Context, spec *mempkg.MemorySpec) (mempkg.RuntimeMemory, error)
}

type memoryNodeInterceptor struct {
	BaseNodeInterceptor
	memory    mempkg.RuntimeMemory
	binding   mempkg.Binding
	assembler MemoryAssembler
	spec      *mempkg.MemorySpec
}

// NewMemoryNodeInterceptor creates a compose node interceptor backed by RuntimeMemory or MemorySpec.
//
// Deprecated: prefer github.com/maizenut/mirorru/interceptors/memorybinding.NewNodeInterceptor.
func NewMemoryNodeInterceptor(mem mempkg.RuntimeMemory, binding mempkg.Binding, spec *mempkg.MemorySpec, assembler MemoryAssembler) NodeInterceptor {
	return &memoryNodeInterceptor{
		memory:    mem,
		binding:   binding,
		spec:      spec,
		assembler: assembler,
	}
}

func (i *memoryNodeInterceptor) BeforeNode(ctx context.Context, info NodeInfo, input any) (context.Context, any, error) {
	binding, err := i.resolveBinding(ctx)
	if err != nil || binding == nil {
		return ctx, input, err
	}
	return binding.BeforeNode(ctx, info.Key, input)
}

func (i *memoryNodeInterceptor) AfterNode(ctx context.Context, info NodeInfo, output any) (context.Context, any, error) {
	binding, err := i.resolveBinding(ctx)
	if err != nil || binding == nil {
		return ctx, output, err
	}
	return binding.AfterNode(ctx, info.Key, output)
}

func (i *memoryNodeInterceptor) resolveBinding(ctx context.Context) (mempkg.Binding, error) {
	if i.binding != nil {
		return i.binding, nil
	}
	mem, err := i.resolveMemory(ctx)
	if err != nil || mem == nil {
		return nil, err
	}
	binding, ok, err := mem.Binding(ctx)
	if err != nil || !ok {
		return nil, err
	}
	i.binding = binding
	return binding, nil
}

func (i *memoryNodeInterceptor) resolveMemory(ctx context.Context) (mempkg.RuntimeMemory, error) {
	if i.memory != nil {
		return i.memory, nil
	}
	if i.spec == nil || i.assembler == nil {
		return nil, nil
	}
	mem, err := i.assembler.Build(ctx, i.spec)
	if err != nil {
		return nil, fmt.Errorf("build memory from spec: %w", err)
	}
	i.memory = mem
	return mem, nil
}

// WithMemoryInterceptorOnCompile injects a default memory-backed node interceptor at compile time.
//
// Deprecated: prefer github.com/maizenut/mirorru/interceptors/memorybinding.WithNodeInterceptorOnCompile.
func WithMemoryInterceptorOnCompile(mem mempkg.RuntimeMemory, binding mempkg.Binding, spec *mempkg.MemorySpec, assembler MemoryAssembler) GraphCompileOption {
	return WithNodeInterceptorsOnCompile(NewMemoryNodeInterceptor(mem, binding, spec, assembler))
}

// WithMemoryInterceptor injects a default memory-backed node interceptor for a single call.
//
// Deprecated: prefer github.com/maizenut/mirorru/interceptors/memorybinding.WithNodeInterceptor.
func WithMemoryInterceptor(mem mempkg.RuntimeMemory, binding mempkg.Binding, spec *mempkg.MemorySpec, assembler MemoryAssembler) Option {
	return WithNodeInterceptor(NewMemoryNodeInterceptor(mem, binding, spec, assembler))
}

func ensureMemoryInterceptor(options *graphCompileOptions) {
	if options == nil || options.memoryOptions == nil {
		return
	}
	memoryOpts := options.memoryOptions
	if memoryOpts.RuntimeMemory == nil && memoryOpts.Binding == nil && memoryOpts.Spec == nil {
		return
	}
	for _, existing := range options.nodeInterceptors {
		if _, ok := existing.(*memoryNodeInterceptor); ok {
			return
		}
	}
	options.nodeInterceptors = append(options.nodeInterceptors, NewMemoryNodeInterceptor(
		memoryOpts.RuntimeMemory,
		memoryOpts.Binding,
		memoryOpts.Spec,
		memoryOpts.Assembler,
	))
}
