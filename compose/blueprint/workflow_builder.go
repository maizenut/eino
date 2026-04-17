package blueprint

import (
	"fmt"

	componentpkg "github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	schemad "github.com/cloudwego/eino/schema/declarative"
)

func graphNodeOpts(node *schemad.NodeSpec) []compose.GraphAddNodeOpt {
	opts := make([]compose.GraphAddNodeOpt, 0, 3)
	if node.Name != "" {
		opts = append(opts, compose.WithNodeName(node.Name))
	}
	if node.InputKey != "" {
		opts = append(opts, compose.WithInputKey(node.InputKey))
	}
	if node.OutputKey != "" {
		opts = append(opts, compose.WithOutputKey(node.OutputKey))
	}
	return opts
}

func addTypedWorkflowNode(wf *compose.Workflow[map[string]any, map[string]any], key string, spec *schemad.ComponentSpec, instance any, opts ...compose.GraphAddNodeOpt) error {
	switch schemad.ComponentKind(spec.Kind) {
	case componentpkg.ComponentOfPrompt:
		node, err := schemad.AsPrompt(instance)
		if err != nil {
			return err
		}
		wf.AddChatTemplateNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfAgenticPrompt:
		node, err := schemad.AsAgenticPrompt(instance)
		if err != nil {
			return err
		}
		wf.AddAgenticChatTemplateNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfChatModel:
		node, err := schemad.AsChatModel(instance)
		if err != nil {
			return err
		}
		wf.AddChatModelNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfAgenticModel:
		node, err := schemad.AsAgenticModel(instance)
		if err != nil {
			return err
		}
		wf.AddAgenticModelNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfEmbedding:
		node, err := schemad.AsEmbedder(instance)
		if err != nil {
			return err
		}
		wf.AddEmbeddingNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfIndexer:
		node, err := schemad.AsIndexer(instance)
		if err != nil {
			return err
		}
		wf.AddIndexerNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfRetriever:
		node, err := schemad.AsRetriever(instance)
		if err != nil {
			return err
		}
		wf.AddRetrieverNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfLoader:
		node, err := schemad.AsLoader(instance)
		if err != nil {
			return err
		}
		wf.AddLoaderNode(key, node, opts...)
		return nil
	case componentpkg.ComponentOfTransformer:
		node, err := schemad.AsTransformer(instance)
		if err != nil {
			return err
		}
		wf.AddDocumentTransformerNode(key, node, opts...)
		return nil
	default:
		if node, ok := instance.(prompt.ChatTemplate); ok {
			wf.AddChatTemplateNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(prompt.AgenticChatTemplate); ok {
			wf.AddAgenticChatTemplateNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(model.BaseChatModel); ok {
			wf.AddChatModelNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(model.AgenticModel); ok {
			wf.AddAgenticModelNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(embedding.Embedder); ok {
			wf.AddEmbeddingNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(indexer.Indexer); ok {
			wf.AddIndexerNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(retriever.Retriever); ok {
			wf.AddRetrieverNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(document.Loader); ok {
			wf.AddLoaderNode(key, node, opts...)
			return nil
		}
		if node, ok := instance.(document.Transformer); ok {
			wf.AddDocumentTransformerNode(key, node, opts...)
			return nil
		}
		return fmt.Errorf("unsupported component instance type %T for workflow node %s", instance, key)
	}
}

func addGraphEdge(g *compose.Graph[map[string]any, map[string]any], edge schemad.GraphEdgeBlueprint) error {
	mappings := toFieldMappings(edge.Mappings)
	noControl, noData := edgeFlags(edge)
	switch {
	case noControl && noData:
		return fmt.Errorf("edge %s -> %s cannot disable both control and data", edge.From, edge.To)
	case noControl || noData || len(mappings) > 0:
		return g.AddEdgeWithOptions(edge.From, edge.To, noControl, noData, mappings...)
	default:
		return g.AddEdge(edge.From, edge.To)
	}
}

func applyWorkflowEdge(wf *compose.Workflow[map[string]any, map[string]any], edge schemad.GraphEdgeBlueprint) error {
	node := workflowNodeByKey(wf, edge.To)
	if node == nil {
		return fmt.Errorf("workflow node %s not found", edge.To)
	}
	mappings := toFieldMappings(edge.Mappings)
	noControl, noData := edgeFlags(edge)
	if noControl && noData {
		return fmt.Errorf("edge %s -> %s cannot disable both control and data", edge.From, edge.To)
	}
	if noData {
		node.AddDependency(edge.From)
		return nil
	}
	if noControl {
		node.AddInputWithOptions(edge.From, mappings, compose.WithNoDirectDependency())
		return nil
	}
	node.AddInput(edge.From, mappings...)
	return nil
}

func edgeFlags(edge schemad.GraphEdgeBlueprint) (bool, bool) {
	noControl := false
	noData := false
	if edge.Control != nil {
		noControl = !*edge.Control
	}
	if edge.Data != nil {
		noData = !*edge.Data
	}
	if edge.Mode != nil {
		if edge.Mode.NoControl {
			noControl = true
		}
		if edge.Mode.NoData {
			noData = true
		}
	}
	return noControl, noData
}

func toFieldMappings(specs []schemad.FieldMappingSpec) []*compose.FieldMapping {
	if len(specs) == 0 {
		return nil
	}
	out := make([]*compose.FieldMapping, 0, len(specs))
	for _, spec := range specs {
		from := compose.FieldPath(spec.From)
		to := compose.FieldPath(spec.To)
		switch {
		case len(spec.From) > 0 && len(spec.To) > 0:
			out = append(out, compose.MapFieldPaths(from, to))
		case len(spec.From) > 0:
			out = append(out, compose.FromFieldPath(from))
		case len(spec.To) > 0:
			out = append(out, compose.ToFieldPath(to))
		default:
			out = append(out, compose.FromFieldPath(compose.FieldPath{}))
		}
	}
	return out
}

func workflowNodeByKey(wf *compose.Workflow[map[string]any, map[string]any], key string) *compose.WorkflowNode {
	if key == compose.END {
		return wf.End()
	}
	return wf.Node(key)
}

func (b *Builder) applyWorkflowNodeBlueprint(wf *compose.Workflow[map[string]any, map[string]any], node *schemad.WorkflowNodeBlueprint) error {
	workflowNode := workflowNodeByKey(wf, node.Key)
	if workflowNode == nil {
		return fmt.Errorf("workflow node %s not found", node.Key)
	}

	for path, value := range node.StaticValue {
		workflowNode.SetStaticValue(compose.FieldPath{path}, value)
	}

	for _, input := range node.Inputs {
		mappings := toFieldMappings(input.Mappings)
		switch {
		case input.DependencyOnly:
			workflowNode.AddDependency(input.From)
		case input.NoDirectDependency:
			workflowNode.AddInputWithOptions(input.From, mappings, compose.WithNoDirectDependency())
		default:
			workflowNode.AddInput(input.From, mappings...)
		}
	}

	return nil
}
