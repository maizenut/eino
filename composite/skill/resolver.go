package skill

import (
	"context"

	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
)

type GraphAssembler interface {
	AssembleGraph(ctx context.Context, graphSpec *schemad.GraphSpec) (compose.AnyGraph, error)
}

type InterpreterResolver interface {
	ResolveComponent(ctx context.Context, ref schemad.Ref) (any, error)
	ResolveFunction(ctx context.Context, ref schemad.Ref) (any, error)
	ResolveGraph(ctx context.Context, ref schemad.Ref) (compose.AnyGraph, error)
}

type DocumentLoader interface {
	LoadGraphSpec(ctx context.Context, ref schemad.Ref) (*schemad.GraphSpec, error)
	LoadComponentSpec(ctx context.Context, target string) (*schemad.ComponentSpec, error)
	LoadNode(ctx context.Context, ref schemad.Ref) (*schemad.NodeSpec, error)
}
