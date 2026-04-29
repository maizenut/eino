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

import (
	"testing"
)

type testInterceptor struct {
	*BaseNodeInterceptor
	name     string
	priority int
}

func (t *testInterceptor) Name() string  { return t.name }
func (t *testInterceptor) Priority() int { return t.priority }

type unnamedInterceptor struct {
	*BaseNodeInterceptor
	priority int
}

func (u *unnamedInterceptor) Priority() int { return u.priority }

func TestSort_ByInsertion(t *testing.T) {
	a := &testInterceptor{name: "a", priority: 10}
	b := &testInterceptor{name: "b", priority: -5}
	c := &testInterceptor{name: "c", priority: 0}

	input := []NodeInterceptor{a, b, c}
	result := Sort(input, OrderByInsertion, nil)

	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}
	if result[0] != a || result[1] != b || result[2] != c {
		t.Fatalf("insertion order not preserved: got %v", names(result))
	}
}

func TestSort_ByPriority(t *testing.T) {
	a := &testInterceptor{name: "a", priority: 10}
	b := &testInterceptor{name: "b", priority: -5}
	c := &testInterceptor{name: "c", priority: 0}
	d := &testInterceptor{name: "d", priority: -5}

	input := []NodeInterceptor{a, b, c, d}
	result := Sort(input, OrderByPriority, nil)

	if len(result) != 4 {
		t.Fatalf("len = %d, want 4", len(result))
	}

	wantOrder := []string{"b", "d", "c", "a"}
	gotOrder := names(result)
	for i, want := range wantOrder {
		if gotOrder[i] != want {
			t.Fatalf("OrderByPriority: got %v, want %v", gotOrder, wantOrder)
		}
	}
}

func TestSort_ByPriority_DefaultPriority(t *testing.T) {
	base := &BaseNodeInterceptor{}
	named := &testInterceptor{name: "named", priority: 5}

	input := []NodeInterceptor{named, base}
	result := Sort(input, OrderByPriority, nil)

	if result[0] != base {
		t.Fatalf("default priority (0) should come before priority 5, got %v", names(result))
	}
}

func TestSort_ByPriority_Stable(t *testing.T) {
	a := &testInterceptor{name: "a", priority: 1}
	b := &testInterceptor{name: "b", priority: 1}
	c := &testInterceptor{name: "c", priority: 1}

	input := []NodeInterceptor{a, b, c}
	result := Sort(input, OrderByPriority, nil)

	if result[0] != a || result[1] != b || result[2] != c {
		t.Fatalf("equal priorities should preserve insertion order (stable), got %v", names(result))
	}
}

func TestSort_ByNames(t *testing.T) {
	a := &testInterceptor{name: "devops", priority: 10}
	b := &testInterceptor{name: "summarization", priority: -5}
	c := &testInterceptor{name: "reminder", priority: 0}

	input := []NodeInterceptor{a, b, c}
	result := Sort(input, OrderByNames, []string{"summarization", "reminder", "devops"})

	wantOrder := []string{"summarization", "reminder", "devops"}
	gotOrder := names(result)
	for i, want := range wantOrder {
		if gotOrder[i] != want {
			t.Fatalf("OrderByNames: got %v, want %v", gotOrder, wantOrder)
		}
	}
}

func TestSort_ByNames_UnnamedPlacedLast(t *testing.T) {
	named := &testInterceptor{name: "devops", priority: 10}
	unnamed := &BaseNodeInterceptor{}

	input := []NodeInterceptor{unnamed, named}
	result := Sort(input, OrderByNames, []string{"devops"})

	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0] != named {
		t.Fatalf("named interceptor should come first, got index 0 = %T", result[0])
	}
	if result[1] != unnamed {
		t.Fatalf("unnamed interceptor should come last, got index 1 = %T", result[1])
	}
}

func TestSort_ByNames_MissingNamePlacedLast(t *testing.T) {
	a := &testInterceptor{name: "devops", priority: 10}
	b := &testInterceptor{name: "summarization", priority: 5}
	c := &testInterceptor{name: "reminder", priority: 0}

	input := []NodeInterceptor{a, b, c}
	result := Sort(input, OrderByNames, []string{"reminder", "devops"})

	if result[0] != c || result[1] != a {
		t.Fatalf("named interceptors should follow explicit order, got %v", names(result))
	}
	if result[2] != b {
		t.Fatalf("missing-name interceptor should be last, got %v", names(result))
	}
}

func TestSort_ByNames_EmptyNamesList(t *testing.T) {
	a := &testInterceptor{name: "a", priority: 0}
	b := &testInterceptor{name: "b", priority: 0}

	input := []NodeInterceptor{a, b}
	result := Sort(input, OrderByNames, nil)

	if result[0] != a || result[1] != b {
		t.Fatalf("empty names list should preserve insertion order, got %v", names(result))
	}
}

func TestSort_EmptyInput(t *testing.T) {
	result := Sort(nil, OrderByPriority, nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result for nil input, got %d", len(result))
	}

	result = Sort([]NodeInterceptor{}, OrderByPriority, nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result for empty input, got %d", len(result))
	}
}

func TestSort_SingleElement(t *testing.T) {
	a := &testInterceptor{name: "a", priority: 5}
	input := []NodeInterceptor{a}
	result := Sort(input, OrderByPriority, nil)
	if len(result) != 1 || result[0] != a {
		t.Fatalf("single element should be unchanged")
	}
}

func TestSort_DoesNotModifyOriginal(t *testing.T) {
	a := &testInterceptor{name: "a", priority: 10}
	b := &testInterceptor{name: "b", priority: -5}

	input := []NodeInterceptor{a, b}
	_ = Sort(input, OrderByPriority, nil)

	if input[0] != a || input[1] != b {
		t.Fatalf("original slice should not be modified")
	}
}

func TestGetInterceptorPriority(t *testing.T) {
	named := &testInterceptor{name: "x", priority: 42}
	if p := getInterceptorPriority(named); p != 42 {
		t.Fatalf("expected priority 42, got %d", p)
	}

	base := &BaseNodeInterceptor{}
	if p := getInterceptorPriority(base); p != DefaultPriority {
		t.Fatalf("expected default priority %d, got %d", DefaultPriority, p)
	}
}

func TestGetInterceptorName(t *testing.T) {
	named := &testInterceptor{name: "my_interceptor"}
	if n := getInterceptorName(named); n != "my_interceptor" {
		t.Fatalf("expected name 'my_interceptor', got %q", n)
	}

	base := &BaseNodeInterceptor{}
	if n := getInterceptorName(base); n != "" {
		t.Fatalf("expected empty name for unnamed, got %q", n)
	}
}

func names(interceptors []NodeInterceptor) []string {
	result := make([]string, len(interceptors))
	for i, ni := range interceptors {
		result[i] = getInterceptorName(ni)
	}
	return result
}
