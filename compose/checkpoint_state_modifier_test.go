package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type checkpointModifierTestState struct {
	Value string
}

type checkpointModifierTestSerializer struct{}

type checkpointModifierTestStore struct{}

func (s *checkpointModifierTestSerializer) Marshal(v any) ([]byte, error) {
	state, ok := v.(*checkpointModifierTestState)
	if !ok {
		return nil, errors.New("unexpected state type")
	}
	return []byte("serialized:" + state.Value), nil
}

func (s *checkpointModifierTestSerializer) Unmarshal(data []byte, v any) error {
	state, ok := v.(*checkpointModifierTestState)
	if !ok {
		return errors.New("unexpected target type")
	}
	state.Value = string(data)
	return nil
}

func (s *checkpointModifierTestStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	return nil, false, nil
}

func (s *checkpointModifierTestStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	return nil
}

func TestStateModifierRestoreAppliesAfterResumeData(t *testing.T) {
	ctx := context.WithValue(context.Background(), stateKey{}, &internalState{state: &checkpointModifierTestState{Value: "parent"}})
	r := &runner{checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, nil)}
	cm := &channelManager{channels: map[string]channel{}}
	cp := &checkpoint{
		Channels: map[string]channel{},
		State:    &checkpointModifierTestState{Value: "resume"},
	}

	expectedPath := NewNodePath("root", "sub").GetPath()
	var seenValue string
	modified, err := r.restoreCheckPointState(ctx, *NewNodePath("root", "sub"), func(ctx context.Context, path NodePath, state any) error {
		phase, ok := GetStateModifierPhase(ctx)
		require.True(t, ok)
		assert.Equal(t, StateModifierPhaseRestore, phase)
		assert.Equal(t, expectedPath, path.GetPath())
		seenValue = state.(*checkpointModifierTestState).Value
		state.(*checkpointModifierTestState).Value = "modified"
		return nil
	}, cp, false, cm)
	require.NoError(t, err)
	assert.Equal(t, "resume", seenValue)
	assert.Equal(t, "modified", cp.State.(*checkpointModifierTestState).Value)

	restoredState, _, err := getState[*checkpointModifierTestState](modified)
	require.NoError(t, err)
	assert.Equal(t, "modified", restoredState.Value)

	current := modified.Value(stateKey{}).(*internalState)
	require.NotNil(t, current.parent)
	parentState, ok := current.parent.state.(*checkpointModifierTestState)
	require.True(t, ok)
	assert.Equal(t, "parent", parentState.Value)
}

func TestStateModifierPersistUsesConfiguredSerializer(t *testing.T) {
	ctx := context.WithValue(context.Background(), stateKey{}, &internalState{state: &checkpointModifierTestState{Value: "raw"}})
	ctx = setStateModifier(ctx, func(ctx context.Context, path NodePath, state any) error {
		phase, ok := GetStateModifierPhase(ctx)
		require.True(t, ok)
		assert.Equal(t, StateModifierPhasePersist, phase)
		assert.Empty(t, path.GetPath())
		state.(*checkpointModifierTestState).Value = state.(*checkpointModifierTestState).Value + ":masked"
		return nil
	})

	r := &runner{
		runCtx:       func(ctx context.Context) context.Context { return ctx },
		checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, &checkpointModifierTestSerializer{}),
	}
	err := r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, nil)
	require.Error(t, err)

	var interruptErr *interruptError
	require.True(t, errors.As(err, &interruptErr))
	state, ok := interruptErr.Info.State.(*checkpointModifierTestState)
	require.True(t, ok)
	assert.Equal(t, "serialized:raw:masked", state.Value)
}

func TestStateModifierNilStateDoesNotTrigger(t *testing.T) {
	r := &runner{checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, nil)}
	cm := &channelManager{channels: map[string]channel{}}
	cp := &checkpoint{Channels: map[string]channel{}}

	called := 0
	_, err := r.restoreCheckPointState(context.Background(), *NewNodePath("root"), func(ctx context.Context, path NodePath, state any) error {
		called++
		return nil
	}, cp, false, cm)
	require.NoError(t, err)
	assert.Equal(t, 0, called)

	ctx := context.WithValue(context.Background(), stateKey{}, &internalState{state: nil})
	ctx = setStateModifier(ctx, func(ctx context.Context, path NodePath, state any) error {
		called++
		return nil
	})

	r.runCtx = func(ctx context.Context) context.Context { return ctx }
	err = r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, nil)
	require.Error(t, err)
	var interruptErr *interruptError
	require.True(t, errors.As(err, &interruptErr))
	assert.Equal(t, 0, called)
	assert.Nil(t, interruptErr.Info.State)
}

var _ core.CheckPointStore = (*checkpointModifierTestStore)(nil)
