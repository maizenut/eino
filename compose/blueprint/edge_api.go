package blueprint

import "github.com/cloudwego/eino/compose"

// GraphEdgeApplier exposes advanced edge construction for declarative builders.
type GraphEdgeApplier interface {
	AddEdgeWithOptions(startNode, endNode string, noControl, noData bool, mappings ...*compose.FieldMapping) error
}
