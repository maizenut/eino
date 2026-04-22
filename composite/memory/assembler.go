package memory

import "context"

// Assembler converts a MemorySpec into a RuntimeMemory.
//
// Concrete assembler implementations resolve StoreRef and policy refs using the
// shared declarative infrastructure (Ref / Loader /
// Resolver) and return an execution-ready RuntimeMemory.
//
// The typical call sequence is:
//
//	spec   := loadSpecFromYAML(...)
//	mem, _ := assembler.Build(ctx, spec)
//	mem.Write(ctx, records)
type Assembler interface {
	Build(ctx context.Context, spec *MemorySpec) (RuntimeMemory, error)
}
