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

package interceptor

import "sort"

const (
	DefaultPriority = 0
)

// PriorityAware is an optional interface that interceptors can implement to
// declare their execution priority. Lower values execute first in BeforeNode
// order (and last in AfterNode / OnErrorNode reverse order).
//
// Interceptors that do not implement PriorityAware are assigned DefaultPriority (0).
//
// Recommended priority ranges:
//   - Negative: infrastructure interceptors (logging, tracing, metrics)
//   - 0:        default interceptors
//   - Positive: business interceptors (summarization, reminder, guardrail)
type PriorityAware interface {
	Priority() int
}

// NamedInterceptor is an optional interface that interceptors can implement to
// declare a unique name. Names are used for:
//   - Explicit ordering via OrderByNames strategy
//   - Identification in observability and debugging
//   - Workspace-level ordering configuration
type NamedInterceptor interface {
	Name() string
}

// OrderStrategy controls how interceptors are ordered before execution.
type OrderStrategy int

const (
	// OrderByInsertion preserves the insertion order (default, backward compatible).
	// Interceptors execute in the order they were added via WithNodeInterceptorsOnCompile
	// and WithNodeInterceptor.
	OrderByInsertion OrderStrategy = iota

	// OrderByPriority sorts interceptors by their Priority() value (lower first).
	// Interceptors with equal priority retain their relative insertion order (stable sort).
	// Interceptors that don't implement PriorityAware are treated as priority 0.
	OrderByPriority

	// OrderByNames sorts interceptors by the explicitly provided name list.
	// Named interceptors are placed in the order their names appear in the list.
	// Interceptors that don't implement NamedInterceptor or whose names aren't in
	// the list are placed after all named ones, in their original insertion order.
	OrderByNames
)

// Sort reorders interceptors according to the given strategy.
// For OrderByNames, the names slice provides the desired order.
// The original slice is not modified; a new slice is returned.
func Sort(interceptors []NodeInterceptor, strategy OrderStrategy, names []string) []NodeInterceptor {
	if len(interceptors) <= 1 {
		return interceptors
	}

	switch strategy {
	case OrderByPriority:
		return sortByPriority(interceptors)
	case OrderByNames:
		return sortByNames(interceptors, names)
	case OrderByInsertion:
		fallthrough
	default:
		return interceptors
	}
}

func sortByPriority(interceptors []NodeInterceptor) []NodeInterceptor {
	sorted := make([]NodeInterceptor, len(interceptors))
	copy(sorted, interceptors)

	sort.Stable(prioritySlice(sorted))

	return sorted
}

func sortByNames(interceptors []NodeInterceptor, names []string) []NodeInterceptor {
	if len(names) == 0 {
		return interceptors
	}

	nameIndex := make(map[string]int, len(names))
	for i, name := range names {
		nameIndex[name] = i
	}

	sorted := make([]NodeInterceptor, 0, len(interceptors))
	named := make([]indexedInterceptor, 0, len(interceptors))
	unnamed := make([]NodeInterceptor, 0)

	for _, ni := range interceptors {
		name := getInterceptorName(ni)
		if idx, ok := nameIndex[name]; ok {
			named = append(named, indexedInterceptor{interceptor: ni, index: idx})
		} else {
			unnamed = append(unnamed, ni)
		}
	}

	sort.Stable(indexedSlice(named))

	for _, item := range named {
		sorted = append(sorted, item.interceptor)
	}
	sorted = append(sorted, unnamed...)

	return sorted
}

func getInterceptorPriority(ni NodeInterceptor) int {
	if pa, ok := ni.(PriorityAware); ok {
		return pa.Priority()
	}
	return DefaultPriority
}

func getInterceptorName(ni NodeInterceptor) string {
	if named, ok := ni.(NamedInterceptor); ok {
		return named.Name()
	}
	return ""
}

type indexedInterceptor struct {
	interceptor NodeInterceptor
	index       int
}

type indexedSlice []indexedInterceptor

func (s indexedSlice) Len() int           { return len(s) }
func (s indexedSlice) Less(i, j int) bool { return s[i].index < s[j].index }
func (s indexedSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type prioritySlice []NodeInterceptor

func (s prioritySlice) Len() int { return len(s) }
func (s prioritySlice) Less(i, j int) bool {
	return getInterceptorPriority(s[i]) < getInterceptorPriority(s[j])
}
func (s prioritySlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
