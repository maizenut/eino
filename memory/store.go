package memory

import "context"

// Store is the low-level memory persistence and retrieval protocol.
//
// Store implementations live in eino-ext (inmemory, sqlite, redis, vectorstore)
// and know nothing about graph lifecycle, injection, or compaction strategies.
// Those concerns belong to the Policy and Binding layers.
type Store interface {
	// Put writes or replaces records in the store.
	Put(ctx context.Context, records []*Record, opts ...StoreOption) error
	// Query retrieves records matching the given request.
	Query(ctx context.Context, req *QueryRequest, opts ...StoreOption) ([]*Record, error)
	// Get returns a single record by ID. ok is false when not found.
	Get(ctx context.Context, id string, opts ...StoreOption) (*Record, bool, error)
	// Delete removes records by ID.
	Delete(ctx context.Context, ids []string, opts ...StoreOption) error
	// List returns a paginated slice of records and the total count.
	List(ctx context.Context, req *ListRequest, opts ...StoreOption) ([]*Record, int, error)
	// Clear removes all records visible to this store instance.
	Clear(ctx context.Context, opts ...StoreOption) error
}
