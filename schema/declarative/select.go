package declarative

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidSelect indicates the select string is malformed.
var ErrInvalidSelect = errors.New("invalid declarative select")

// Selection describes a parsed ref select expression.
type Selection struct {
	Kind string
	Name string
}

// ParseSelect parses select expressions like node:foo.
func ParseSelect(selectExpr string) (*Selection, error) {
	if selectExpr == "" {
		return nil, nil
	}

	idx := strings.IndexByte(selectExpr, ':')
	if idx <= 0 || idx == len(selectExpr)-1 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSelect, selectExpr)
	}

	return &Selection{
		Kind: selectExpr[:idx],
		Name: selectExpr[idx+1:],
	}, nil
}

// MustParseSelect parses select expressions and panics on error.
func MustParseSelect(selectExpr string) *Selection {
	sel, err := ParseSelect(selectExpr)
	if err != nil {
		panic(err)
	}
	return sel
}

// FindNode returns the node matching the selector name.
func (g *GraphBlueprint) FindNode(name string) (*NodeSpec, bool) {
	for i := range g.Nodes {
		if g.Nodes[i].Key == name {
			return &g.Nodes[i], true
		}
	}
	return nil, false
}
