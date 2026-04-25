package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	schema.RegisterName[checkpointModifierTestState]("_test_checkpoint_modifier_state")
}

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
	expectedPath := NewNodePath("root").GetPath()
	ctx = setStateModifier(ctx, func(ctx context.Context, path NodePath, state any) error {
		phase, ok := GetStateModifierPhase(ctx)
		require.True(t, ok)
		assert.Equal(t, StateModifierPhasePersist, phase)
		assert.Equal(t, expectedPath, path.GetPath())
		state.(*checkpointModifierTestState).Value = state.(*checkpointModifierTestState).Value + ":masked"
		return nil
	})

	r := &runner{
		runCtx:       func(ctx context.Context) context.Context { return ctx },
		checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, &checkpointModifierTestSerializer{}),
	}
	err := r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, nil, *NewNodePath("root"))
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
	err = r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, nil, *NewNodePath())
	require.Error(t, err)
	var interruptErr *interruptError
	require.True(t, errors.As(err, &interruptErr))
	assert.Equal(t, 0, called)
	assert.Nil(t, interruptErr.Info.State)
}

func TestStateModifierPersistErrorIsolation(t *testing.T) {
	ctx := context.WithValue(context.Background(), stateKey{}, &internalState{state: &checkpointModifierTestState{Value: "sensitive"}})
	ctx = setStateModifier(ctx, func(ctx context.Context, path NodePath, state any) error {
		return errors.New("persist modifier failed")
	})

	r := &runner{
		runCtx:       func(ctx context.Context) context.Context { return ctx },
		checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, &checkpointModifierTestSerializer{}),
	}
	err := r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, nil, *NewNodePath("root"))
	require.Error(t, err)

	var interruptErr *interruptError
	require.True(t, errors.As(err, &interruptErr))
	assert.Nil(t, interruptErr.Info.State)
}

func TestStateModifierRestoreErrorIsolation(t *testing.T) {
	r := &runner{checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, nil)}
	cm := &channelManager{channels: map[string]channel{}}
	cp := &checkpoint{
		Channels: map[string]channel{},
		State:    &checkpointModifierTestState{Value: "should-not-inject"},
	}

	modified, err := r.restoreCheckPointState(context.Background(), *NewNodePath("root"), func(ctx context.Context, path NodePath, state any) error {
		return errors.New("restore modifier failed")
	}, cp, false, cm)
	require.Error(t, err)
	assert.Nil(t, cp.State)

	_, _, stateErr := getState[*checkpointModifierTestState](modified)
	require.Error(t, stateErr)
}

func TestStateModifierPersistPathPropagation(t *testing.T) {
	ctx := context.WithValue(context.Background(), stateKey{}, &internalState{state: &checkpointModifierTestState{Value: "v"}})
	var seenPath []string
	ctx = setStateModifier(ctx, func(ctx context.Context, path NodePath, state any) error {
		seenPath = path.GetPath()
		return nil
	})

	r := &runner{
		runCtx:       func(ctx context.Context) context.Context { return ctx },
		checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, &checkpointModifierTestSerializer{}),
	}
	err := r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, nil, *NewNodePath("parent", "child"))
	require.Error(t, err)
	assert.Equal(t, []string{"parent", "child"}, seenPath)
}

func TestStateModifierRestoreOrderResumeDataBeforeModifier(t *testing.T) {
	r := &runner{checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, nil)}
	cm := &channelManager{channels: map[string]channel{}}

	ctx := AppendAddressSegment(context.Background(), AddressSegmentNode, "root")

	resumeCtx := BatchResumeWithData(ctx, map[string]any{
		"int-id-1": &checkpointModifierTestState{Value: "resume-data"},
	})

	cp := &checkpoint{
		Channels: map[string]channel{},
		State:    &checkpointModifierTestState{Value: "checkpoint-value"},
		InterruptID2Addr: map[string]core.Address{
			"int-id-1": {{Type: AddressSegmentNode, ID: "root"}},
		},
		InterruptID2State: map[string]core.InterruptState{},
	}

	resumeCtx = setCheckPointToCtx(resumeCtx, cp)

	var modifierSeenValue string
	modified, err := r.restoreCheckPointState(resumeCtx, *NewNodePath("root"), func(ctx context.Context, path NodePath, state any) error {
		phase, ok := GetStateModifierPhase(ctx)
		require.True(t, ok)
		assert.Equal(t, StateModifierPhaseRestore, phase)
		modifierSeenValue = state.(*checkpointModifierTestState).Value
		state.(*checkpointModifierTestState).Value = "modifier-applied"
		return nil
	}, cp, false, cm)
	require.NoError(t, err)

	assert.Equal(t, "resume-data", modifierSeenValue,
		"modifier should see the resume data, not the original checkpoint value")
	assert.Equal(t, "modifier-applied", cp.State.(*checkpointModifierTestState).Value,
		"final state should be the modifier's output")

	restoredState, _, err := getState[*checkpointModifierTestState](modified)
	require.NoError(t, err)
	assert.Equal(t, "modifier-applied", restoredState.Value)
}

func TestStateModifierBackwardCompatibleWithoutPhaseCheck(t *testing.T) {
	ctx := context.WithValue(context.Background(), stateKey{}, &internalState{state: &checkpointModifierTestState{Value: "original"}})

	var restoreCalled, persistCalled bool
	ctx = setStateModifier(ctx, func(ctx context.Context, path NodePath, state any) error {
		s := state.(*checkpointModifierTestState)
		if phase, ok := GetStateModifierPhase(ctx); ok {
			switch phase {
			case StateModifierPhaseRestore:
				restoreCalled = true
				s.Value += ":restore"
			case StateModifierPhasePersist:
				persistCalled = true
				s.Value += ":persist"
			}
		} else {
			s.Value += ":legacy"
		}
		return nil
	})

	r := &runner{
		runCtx:       func(ctx context.Context) context.Context { return ctx },
		checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, &checkpointModifierTestSerializer{}),
	}
	err := r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, nil, *NewNodePath("root"))
	require.Error(t, err)

	var interruptErr *interruptError
	require.True(t, errors.As(err, &interruptErr))
	assert.True(t, persistCalled, "persist modifier should be called during interrupt")
	assert.False(t, restoreCalled, "restore modifier should NOT be called during interrupt")

	state := interruptErr.Info.State.(*checkpointModifierTestState)
	assert.Equal(t, "serialized:original:persist", state.Value,
		"legacy-compatible modifier should work correctly with phase detection")
}

func TestStateModifierCustomSerializerInterruptRestore(t *testing.T) {
	store := &checkpointModifierRecordingStore{data: map[string][]byte{}}

	ctx := context.WithValue(context.Background(), stateKey{}, &internalState{state: &checkpointModifierTestState{Value: "flow-test"}})
	var persistPhaseSeen, restorePhaseSeen bool
	ctx = setStateModifier(ctx, func(ctx context.Context, path NodePath, state any) error {
		phase, _ := GetStateModifierPhase(ctx)
		s := state.(*checkpointModifierTestState)
		switch phase {
		case StateModifierPhasePersist:
			persistPhaseSeen = true
			s.Value += ":persisted"
		case StateModifierPhaseRestore:
			restorePhaseSeen = true
			s.Value += ":restored"
		}
		return nil
	})

	cpID := "custom-serializer-test"
	r := &runner{
		runCtx:       func(ctx context.Context) context.Context { return ctx },
		checkPointer: newCheckPointer(nil, nil, store, nil),
	}
	err := r.handleInterrupt(ctx, newInterruptTempInfo(), nil, map[string]channel{}, false, false, &cpID, *NewNodePath("root"))
	require.Error(t, err)

	var interruptErr *interruptError
	require.True(t, errors.As(err, &interruptErr))
	assert.True(t, persistPhaseSeen, "persist modifier should be invoked during interrupt")
	assert.False(t, restorePhaseSeen, "restore modifier should NOT be invoked during interrupt")
	assert.Equal(t, "flow-test:persisted", interruptErr.Info.State.(*checkpointModifierTestState).Value)

	savedData, ok, err := store.Get(context.Background(), cpID)
	require.NoError(t, err)
	require.True(t, ok, "checkpoint should be persisted to store")
	require.True(t, len(savedData) > 0, "persisted data should not be empty")
}

func TestStateModifierSubGraphIndependentInvocation(t *testing.T) {
	r := &runner{checkPointer: newCheckPointer(nil, nil, &checkpointModifierTestStore{}, nil)}
	cm := &channelManager{channels: map[string]channel{}}

	parentCtx := context.WithValue(context.Background(), stateKey{}, &internalState{state: &checkpointModifierTestState{Value: "parent-state"}})

	subCP := &checkpoint{
		Channels: map[string]channel{},
		State:    &checkpointModifierTestState{Value: "sub-state"},
	}
	parentCP := &checkpoint{
		Channels:  map[string]channel{},
		State:     &checkpointModifierTestState{Value: "parent-state"},
		SubGraphs: map[string]*checkpoint{"sub_node": subCP},
	}

	var invokedPaths []string
	var invokedPhases []StateModifierPhase
	sm := func(ctx context.Context, path NodePath, state any) error {
		phase, _ := GetStateModifierPhase(ctx)
		invokedPaths = append(invokedPaths, path.GetPath()...)
		invokedPhases = append(invokedPhases, phase)
		s := state.(*checkpointModifierTestState)
		s.Value += ":" + string(phase)
		return nil
	}

	modified, err := r.restoreCheckPointState(parentCtx, *NewNodePath("parent"), sm, parentCP, false, cm)
	require.NoError(t, err)
	assert.Equal(t, []string{"parent"}, invokedPaths)
	assert.Equal(t, []StateModifierPhase{StateModifierPhaseRestore}, invokedPhases)

	parentState := modified.Value(stateKey{}).(*internalState)
	require.NotNil(t, parentState)
	assert.Equal(t, "parent-state:restore", parentState.state.(*checkpointModifierTestState).Value)

	subCtx := forwardCheckPoint(modified, "sub_node")
	subModified, err := r.restoreCheckPointState(subCtx, *NewNodePath("parent", "sub_node"), sm, subCP, false, cm)
	require.NoError(t, err)
	assert.Equal(t, []string{"parent", "parent", "sub_node"}, invokedPaths)
	assert.Equal(t, []StateModifierPhase{StateModifierPhaseRestore, StateModifierPhaseRestore}, invokedPhases)

	subState := subModified.Value(stateKey{}).(*internalState)
	require.NotNil(t, subState)
	assert.Equal(t, "sub-state:restore", subState.state.(*checkpointModifierTestState).Value)
}

type checkpointModifierRecordingStore struct {
	data map[string][]byte
}

func (s *checkpointModifierRecordingStore) Get(ctx context.Context, id string) ([]byte, bool, error) {
	v, ok := s.data[id]
	return v, ok, nil
}

func (s *checkpointModifierRecordingStore) Set(ctx context.Context, id string, data []byte) error {
	s.data[id] = data
	return nil
}

var _ core.CheckPointStore = (*checkpointModifierTestStore)(nil)
var _ core.CheckPointStore = (*checkpointModifierRecordingStore)(nil)
