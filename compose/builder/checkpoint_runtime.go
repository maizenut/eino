package builder

import (
	"context"

	composepkg "github.com/cloudwego/eino/compose"
)

type BuilderCheckPointStore = composepkg.CheckPointStore

// CheckpointStoreAdapter adapts builder runtime checkpoint metadata into a simple key/value store.
type CheckpointStoreAdapter struct {
	Store composepkg.CheckPointStore
}

func (a *CheckpointStoreAdapter) Save(ctx context.Context, key string, state map[string]any) error {
	if a == nil || a.Store == nil || key == "" {
		return nil
	}
	payload := serializeCheckpointState(state)
	return a.Store.Set(ctx, key, payload)
}

func (a *CheckpointStoreAdapter) Load(ctx context.Context, key string) (map[string]any, bool, error) {
	if a == nil || a.Store == nil || key == "" {
		return nil, false, nil
	}
	payload, ok, err := a.Store.Get(ctx, key)
	if err != nil || !ok {
		return nil, ok, err
	}
	return deserializeCheckpointState(payload), true, nil
}

func serializeCheckpointState(state map[string]any) []byte {
	if state == nil {
		state = map[string]any{}
	}
	return []byte(encodeCheckpointState(state))
}

func deserializeCheckpointState(payload []byte) map[string]any {
	return decodeCheckpointState(string(payload))
}
