package compose

import (
	mempkg "github.com/cloudwego/eino/composite/memory"
)

// MemoryOptions carries optional runtime memory wiring for graph execution.
type MemoryOptions struct {
	RuntimeMemory mempkg.RuntimeMemory
	Binding       mempkg.Binding
	Spec          *mempkg.MemorySpec
	Assembler     MemoryAssembler
}

// WithMemory injects a prebuilt RuntimeMemory into compose execution settings.
func WithMemory(mem mempkg.RuntimeMemory) GraphCompileOption {
	return func(o *graphCompileOptions) {
		if o == nil {
			return
		}
		if o.memoryOptions == nil {
			o.memoryOptions = &MemoryOptions{}
		}
		o.memoryOptions.RuntimeMemory = mem
	}
}

// WithMemoryBinding injects a lifecycle memory binding into compose execution settings.
func WithMemoryBinding(binding mempkg.Binding) GraphCompileOption {
	return func(o *graphCompileOptions) {
		if o == nil {
			return
		}
		if o.memoryOptions == nil {
			o.memoryOptions = &MemoryOptions{}
		}
		o.memoryOptions.Binding = binding
	}
}

// WithMemorySpec injects a declarative MemorySpec into compose execution settings.
func WithMemorySpec(spec *mempkg.MemorySpec) GraphCompileOption {
	return func(o *graphCompileOptions) {
		if o == nil {
			return
		}
		if o.memoryOptions == nil {
			o.memoryOptions = &MemoryOptions{}
		}
		o.memoryOptions.Spec = spec
	}
}

// WithMemoryAssembler injects a memory assembler used to materialize RuntimeMemory from MemorySpec.
func WithMemoryAssembler(assembler MemoryAssembler) GraphCompileOption {
	return func(o *graphCompileOptions) {
		if o == nil {
			return
		}
		if o.memoryOptions == nil {
			o.memoryOptions = &MemoryOptions{}
		}
		o.memoryOptions.Assembler = assembler
	}
}

// MemoryOptionsFromCompileOptions extracts memory settings from compile options.
func MemoryOptionsFromCompileOptions(opts ...GraphCompileOption) MemoryOptions {
	var cfg graphCompileOptions
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.memoryOptions == nil {
		return MemoryOptions{}
	}
	return *cfg.memoryOptions
}
