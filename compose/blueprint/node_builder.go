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

func addTypedComponentNode(g *compose.Graph[map[string]any, map[string]any], key string, spec *schemad.ComponentSpec, instance any, opts ...compose.GraphAddNodeOpt) error {
	switch schemad.ComponentKind(spec.Kind) {
	case componentpkg.ComponentOfPrompt:
		node, err := schemad.AsPrompt(instance)
		if err != nil {
			return err
		}
		return g.AddChatTemplateNode(key, node, opts...)
	case componentpkg.ComponentOfAgenticPrompt:
		node, err := schemad.AsAgenticPrompt(instance)
		if err != nil {
			return err
		}
		return g.AddAgenticChatTemplateNode(key, node, opts...)
	case componentpkg.ComponentOfChatModel:
		node, err := schemad.AsChatModel(instance)
		if err != nil {
			return err
		}
		return g.AddChatModelNode(key, node, opts...)
	case componentpkg.ComponentOfAgenticModel:
		node, err := schemad.AsAgenticModel(instance)
		if err != nil {
			return err
		}
		return g.AddAgenticModelNode(key, node, opts...)
	case componentpkg.ComponentOfEmbedding:
		node, err := schemad.AsEmbedder(instance)
		if err != nil {
			return err
		}
		return g.AddEmbeddingNode(key, node, opts...)
	case componentpkg.ComponentOfIndexer:
		node, err := schemad.AsIndexer(instance)
		if err != nil {
			return err
		}
		return g.AddIndexerNode(key, node, opts...)
	case componentpkg.ComponentOfRetriever:
		node, err := schemad.AsRetriever(instance)
		if err != nil {
			return err
		}
		return g.AddRetrieverNode(key, node, opts...)
	case componentpkg.ComponentOfLoader:
		node, err := schemad.AsLoader(instance)
		if err != nil {
			return err
		}
		return g.AddLoaderNode(key, node, opts...)
	case componentpkg.ComponentOfTransformer:
		node, err := schemad.AsTransformer(instance)
		if err != nil {
			return err
		}
		return g.AddDocumentTransformerNode(key, node, opts...)
	case componentpkg.ComponentOfTool:
		_ = instance
		return fmt.Errorf("tool component kind is not directly supported as a compose graph node")
	default:
		if node, ok := instance.(prompt.ChatTemplate); ok {
			return g.AddChatTemplateNode(key, node, opts...)
		}
		if node, ok := instance.(prompt.AgenticChatTemplate); ok {
			return g.AddAgenticChatTemplateNode(key, node, opts...)
		}
		if node, ok := instance.(model.BaseChatModel); ok {
			return g.AddChatModelNode(key, node, opts...)
		}
		if node, ok := instance.(model.AgenticModel); ok {
			return g.AddAgenticModelNode(key, node, opts...)
		}
		if node, ok := instance.(embedding.Embedder); ok {
			return g.AddEmbeddingNode(key, node, opts...)
		}
		if node, ok := instance.(indexer.Indexer); ok {
			return g.AddIndexerNode(key, node, opts...)
		}
		if node, ok := instance.(retriever.Retriever); ok {
			return g.AddRetrieverNode(key, node, opts...)
		}
		if node, ok := instance.(document.Loader); ok {
			return g.AddLoaderNode(key, node, opts...)
		}
		if node, ok := instance.(document.Transformer); ok {
			return g.AddDocumentTransformerNode(key, node, opts...)
		}
		return fmt.Errorf("unsupported component instance type %T for node %s", instance, key)
	}
}
