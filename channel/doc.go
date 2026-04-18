// Package channel provides the communication boundary abstraction for Agent Runtime.
//
// It defines the declarative specification, resolution, assembly, and runtime view
// layers for managed communication endpoints: terminal I/O, WebSocket, message queues,
// event streams, and multi-path bridges.
//
// The five-layer design is:
//
//  1. ChannelSpec   — static declaration (serializable, loadable from YAML/JSON)
//  2. EndpointSpec  — transport connection description
//  3. Resolver      — resolves handler and graph refs into concrete objects
//  4. Assembler     — assembles a RuntimeChannel from a resolved ChannelSpec
//  5. RuntimeChannel — execution-facing interface for send/receive and lifecycle
//
// An optional Registry layer manages channel registration and discovery.
//
// The package reuses the shared declarative infrastructure (Ref, document loader,
// interpreter resolver, scope-aware cache) from schema/declarative and
// compose/blueprint, keeping itself above components without duplicating their
// serialization and resolution machinery.
package channel
