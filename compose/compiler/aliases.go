package compiler

import builderpkg "github.com/cloudwego/eino/compose/builder"

type GraphName = builderpkg.GraphName
type Version = builderpkg.Version

type NodeID = builderpkg.NodeID
type EdgeID = builderpkg.EdgeID
type BlockID = builderpkg.BlockID
type SubGraphID = builderpkg.SubGraphID

type SchemaRef = builderpkg.SchemaRef
type HandlerRef = builderpkg.HandlerRef
type BindingRef = builderpkg.BindingRef
type PolicyRef = builderpkg.PolicyRef

type ResumeEntryID = builderpkg.ResumeEntryID
type CheckpointScopeID = builderpkg.CheckpointScopeID

type NodeKind = builderpkg.NodeKind
type EdgeKind = builderpkg.EdgeKind
type BlockKind = builderpkg.BlockKind
type ProjectionMode = builderpkg.ProjectionMode

const (
	NodeKindModel     = builderpkg.NodeKindModel
	NodeKindTool      = builderpkg.NodeKindTool
	NodeKindLambda    = builderpkg.NodeKindLambda
	NodeKindRouter    = builderpkg.NodeKindRouter
	NodeKindBranch    = builderpkg.NodeKindBranch
	NodeKindJoin      = builderpkg.NodeKindJoin
	NodeKindLoop      = builderpkg.NodeKindLoop
	NodeKindSubgraph  = builderpkg.NodeKindSubgraph
	NodeKindHuman     = builderpkg.NodeKindHuman
	NodeKindMemory    = builderpkg.NodeKindMemory
	NodeKindIORef     = builderpkg.NodeKindIORef
	NodeKindSkill     = builderpkg.NodeKindSkill
	NodeKindTransform = builderpkg.NodeKindTransform
)

const (
	EdgeKindControl     = builderpkg.EdgeKindControl
	EdgeKindData        = builderpkg.EdgeKindData
	EdgeKindConditional = builderpkg.EdgeKindConditional
	EdgeKindError       = builderpkg.EdgeKindError
	EdgeKindResume      = builderpkg.EdgeKindResume
	EdgeKindProjection  = builderpkg.EdgeKindProjection
)

const (
	BlockKindSequence = builderpkg.BlockKindSequence
	BlockKindParallel = builderpkg.BlockKindParallel
	BlockKindBranch   = builderpkg.BlockKindBranch
	BlockKindLoop     = builderpkg.BlockKindLoop
	BlockKindRetry    = builderpkg.BlockKindRetry
	BlockKindFallback = builderpkg.BlockKindFallback
)

type ConditionSpec = builderpkg.ConditionSpec
type ProjectionSpec = builderpkg.ProjectionSpec
type ErrorMatchSpec = builderpkg.ErrorMatchSpec
type SubGraphProjectionSpec = builderpkg.SubGraphProjectionSpec
type SubGraphVisibilitySpec = builderpkg.SubGraphVisibilitySpec
type SubGraphOwnershipSpec = builderpkg.SubGraphOwnershipSpec

type BindingSpec = builderpkg.BindingSpec
type PolicySpec = builderpkg.PolicySpec

type ExecutionPlan = builderpkg.ExecutionPlan
type StructuralPlan = builderpkg.StructuralPlan
type PlannedNode = builderpkg.PlannedNode
type PlannedEdge = builderpkg.PlannedEdge
type PlannedBlock = builderpkg.PlannedBlock
type PlannedSubGraph = builderpkg.PlannedSubGraph
type StateRecoveryPlan = builderpkg.StateRecoveryPlan
type PlannedProjection = builderpkg.PlannedProjection
type PlannedCheckpointScope = builderpkg.PlannedCheckpointScope
type PlannedResumeEntry = builderpkg.PlannedResumeEntry
type PlannedReplayPolicy = builderpkg.PlannedReplayPolicy
type RuntimeBindingPlan = builderpkg.RuntimeBindingPlan

type CompileOverlay struct {
	BindingOverrides map[BindingRef]BindingRef
	PolicyOverrides  map[PolicyRef]PolicyRef
	Metadata         map[string]any
}
