/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package compose

import composeinterceptor "github.com/cloudwego/eino/compose/interceptor"

type graphCompileOptions struct {
	maxRunSteps     int
	graphName       string
	nodeTriggerMode NodeTriggerMode // default to AnyPredecessor (pregel)

	callbacks []GraphCompileCallback

	origOpts []GraphCompileOption

	checkPointStore          CheckPointStore
	serializer               Serializer
	interruptBeforeNodes     []string
	interruptAfterNodes      []string
	nodeInterceptors         []NodeInterceptor
	interceptorOrderStrategy composeinterceptor.OrderStrategy
	interceptorOrderNames    []string

	eagerDisabled bool

	mergeConfigs map[string]FanInMergeConfig
}

func newGraphCompileOptions(opts ...GraphCompileOption) *graphCompileOptions {
	option := &graphCompileOptions{}

	for _, o := range opts {
		o(option)
	}

	option.origOpts = opts

	return option
}

// GraphCompileOption options for compiling AnyGraph.
type GraphCompileOption func(*graphCompileOptions)

// WithMaxRunSteps sets the maximum number of steps that a graph can run.
// This is useful to prevent infinite loops in graphs with cycles.
// If the number of steps exceeds maxSteps, the graph execution will be terminated with an error.
func WithMaxRunSteps(maxSteps int) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.maxRunSteps = maxSteps
	}
}

// WithGraphName sets a name for the graph.
// The name is used for debugging and logging purposes.
// If not set, a default name will be used.
func WithGraphName(graphName string) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.graphName = graphName
	}
}

// WithEagerExecution enables the eager execution mode for the graph.
// In eager mode, nodes will be executed immediately once they are ready to run,
// without waiting for the completion of a super step, ref: https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
// Note: Eager mode is not allowed when the graph's trigger mode is set to AnyPredecessor.
// Workflow uses eager mode by default.
// Deprecated: Eager execution is automatically enabled by default when a node's trigger mode is set to AllPredecessor.
// If you were using this option previously, it can be safely removed without changing behavior.
func WithEagerExecution() GraphCompileOption {
	return func(o *graphCompileOptions) {
		_ = o
	}
}

// WithEagerExecutionDisabled disables the eager execution mode for the graph.
// By default, eager execution is enabled for Workflow and Graph with the AllPredecessor trigger mode.
// After using this option, nodes will wait for the completion of a super step instead of execute immediately once they are ready to run.
// ref: https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
func WithEagerExecutionDisabled() GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.eagerDisabled = true
	}
}

// WithNodeTriggerMode sets the trigger mode for nodes in the graph.
// The trigger mode determines when a node is triggered during graph execution, ref: https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
// AnyPredecessor by default.
func WithNodeTriggerMode(triggerMode NodeTriggerMode) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.nodeTriggerMode = triggerMode
	}
}

// WithGraphCompileCallbacks sets callbacks for graph compilation.
func WithGraphCompileCallbacks(cbs ...GraphCompileCallback) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.callbacks = append(o.callbacks, cbs...)
	}
}

// FanInMergeConfig defines the configuration for fan-in merge operations.
// It allows specifying how multiple inputs are merged into a single input.
// StreamMergeWithSourceEOF indicates whether to emit a SourceEOF error for each stream
// when it ends, before the final merged output is produced. This is useful for
// tracking the completion of individual input streams in a named stream merge.
type FanInMergeConfig struct {
	StreamMergeWithSourceEOF bool //indicates whether to emit a SourceEOF error for each stream
}

// WithFanInMergeConfig sets the fan-in merge configurations
// for the graph nodes that receive inputs from multiple sources.
func WithFanInMergeConfig(confs map[string]FanInMergeConfig) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.mergeConfigs = confs
	}
}

// InitGraphCompileCallbacks set global graph compile callbacks,
// which ONLY will be added to top level graph compile options
func InitGraphCompileCallbacks(cbs []GraphCompileCallback) {
	globalGraphCompileCallbacks = cbs
}

// WithInterceptorOrderStrategy sets the ordering strategy for node interceptors
// at compile time. The default is OrderByInsertion (backward compatible).
//
// Use OrderByPriority to sort interceptors by their Priority() value (lower first),
// or OrderByNames to sort by an explicit name list provided via WithInterceptorOrderNames.
func WithInterceptorOrderStrategy(strategy composeinterceptor.OrderStrategy) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.interceptorOrderStrategy = strategy
	}
}

// WithInterceptorOrderNames sets explicit interceptor ordering by name at compile time.
// This is used with the OrderByNames strategy. Named interceptors are placed in the
// order their names appear in this list. Interceptors that don't implement NamedInterceptor
// or whose names aren't in the list are placed after all named ones.
func WithInterceptorOrderNames(names ...string) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.interceptorOrderNames = names
	}
}

var globalGraphCompileCallbacks []GraphCompileCallback
