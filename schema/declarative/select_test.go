package declarative

import (
	"errors"
	"testing"
)

func TestParseSelect(t *testing.T) {
	cases := []struct {
		in      string
		wantNil bool
		wantErr bool
		kind    string
		name    string
	}{
		{in: "", wantNil: true},
		{in: "node:foo", kind: "node", name: "foo"},
		{in: "graph:g", kind: "graph", name: "g"},
		{in: "component:c", kind: "component", name: "c"},
		{in: "nocolon", wantErr: true},
		{in: ":missing-kind", wantErr: true},
		{in: "missing-name:", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			sel, err := ParseSelect(tc.in)
			if tc.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidSelect) {
					t.Fatalf("err = %v, want ErrInvalidSelect", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tc.wantNil {
				if sel != nil {
					t.Fatalf("sel = %+v, want nil", sel)
				}
				return
			}
			if sel == nil || sel.Kind != tc.kind || sel.Name != tc.name {
				t.Fatalf("sel = %+v, want kind=%q name=%q", sel, tc.kind, tc.name)
			}
		})
	}
}

func TestMustParseSelect_PanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustParseSelect did not panic")
		}
	}()
	MustParseSelect("bad")
}

func TestGraphSpec_FindNode(t *testing.T) {
	spec := &GraphSpec{Nodes: []NodeSpec{{Key: "a"}, {Key: "b"}}}
	if n, ok := spec.FindNode("b"); !ok || n.Key != "b" {
		t.Fatalf("FindNode(b) = %+v, %v", n, ok)
	}
	if _, ok := spec.FindNode("c"); ok {
		t.Fatal("FindNode(c) unexpectedly ok")
	}
}
