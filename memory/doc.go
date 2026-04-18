// Package memory defines the runtime memory abstraction for eino agents and graphs.
//
// memory is a declarative, composable long-term context system. It is not a raw
// storage interface — it sits above atomic store capabilities and organises
// write, recall, compaction, injection, scope isolation, and runtime binding
// into a unified, assembly-driven design.
//
// # Layers
//
// The package is split into five layers that mirror the four-stage declarative
// main path used by the skill package:
//
//	MemorySpec → Resolve Refs / Policies → Assembler.Build → RuntimeMemory
//
//   - Store: raw record persistence and retrieval (implemented in eino-ext)
//   - Policy: write, recall, compaction, and scope strategies
//   - MemorySpec: static declaration (JSON/YAML-serialisable)
//   - Assembler: converts a MemorySpec into a RuntimeMemory
//   - Binding: hooks RuntimeMemory into graph / agent lifecycle events
//
// # Quick start
//
//	spec := &memory.MemorySpec{
//	    Info: memory.Info{Name: "session-memory"},
//	    Scope: &memory.ScopeSpec{Name: "s1", Kind: memory.ScopeKindSession},
//	    StoreRef: declarative.Ref{Kind: declarative.RefKindComponentDocument, Target: "inmemory"},
//	    WritePolicy:  &memory.WritePolicySpec{Mode: memory.WriteModeAppend},
//	    RecallPolicy: &memory.RecallPolicySpec{TopK: 10},
//	}
//
//	// Assemble with a concrete Assembler provided by eino-ext.
//	mem, err := assembler.Build(ctx, spec)
//
//	// Use at runtime.
//	err = mem.Write(ctx, records)
//	results, err := mem.Recall(ctx, &memory.QueryRequest{Query: "last task"})
//
// # Scopes
//
// Scope is a first-class concept. A ScopeSpec carries its name, kind, parent,
// read-fallback chain, and write target so that multi-tenant, multi-agent
// deployments can share and isolate memory without custom wiring.
//
// # Compose integration
//
// The compose package exposes helpers for injecting memory into graph runs:
//
//	compose.WithMemory(mem)
//	compose.WithMemoryBinding(binding)
//	compose.WithMemorySpec(spec)
package memory
