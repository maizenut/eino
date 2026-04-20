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

// NodeInterceptor aliases the runtime node interceptor contract.
type NodeInterceptor = composeinterceptor.NodeInterceptor

// BaseNodeInterceptor aliases the no-op runtime node interceptor implementation.
type BaseNodeInterceptor = composeinterceptor.BaseNodeInterceptor

// NodeExecutor aliases the graph node executor signature used by interceptors.
type NodeExecutor = composeinterceptor.NodeExecutor

// NodeInfo aliases runtime node execution metadata.
type NodeInfo = composeinterceptor.NodeInfo

// PriorityAware aliases the optional interface for declaring interceptor priority.
type PriorityAware = composeinterceptor.PriorityAware

// NamedInterceptor aliases the optional interface for declaring interceptor name.
type NamedInterceptor = composeinterceptor.NamedInterceptor

// OrderStrategy aliases the interceptor ordering strategy type.
type OrderStrategy = composeinterceptor.OrderStrategy

const (
	OrderByInsertion OrderStrategy = composeinterceptor.OrderByInsertion
	OrderByPriority  OrderStrategy = composeinterceptor.OrderByPriority
	OrderByNames     OrderStrategy = composeinterceptor.OrderByNames
)

// SortInterceptors aliases the interceptor sorting function.
var SortInterceptors = composeinterceptor.Sort
