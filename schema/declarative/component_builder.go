package declarative

import (
	"context"
	"fmt"

	componentpkg "github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
)

// ComponentFactory builds a component from a declarative spec.
type ComponentFactory interface {
	BuildComponent(ctx context.Context, spec *ComponentSpec) (any, error)
}

// ComponentResolver resolves component refs from external systems.
type ComponentResolver interface {
	ResolveComponent(ctx context.Context, ref Ref) (any, error)
}

// BuildComponent resolves and validates a component instance.
func BuildComponent(ctx context.Context, spec *ComponentSpec, factory ComponentFactory, resolver ComponentResolver) (any, error) {
	if spec == nil {
		return nil, fmt.Errorf("component spec is required")
	}

	if spec.Impl == RefKindInterpreterComponent {
		if resolver == nil {
			return nil, fmt.Errorf("component resolver is required for %s", spec.Impl)
		}
		ref := interpreterComponentRef(spec)
		instance, err := resolver.ResolveComponent(ctx, ref)
		if err != nil {
			return nil, err
		}
		validated, err := AsComponentKind(spec.Kind, instance)
		if err != nil {
			return nil, err
		}
		return validated, nil
	}

	if factory == nil {
		return nil, fmt.Errorf("component factory is required for impl %s", spec.Impl)
	}
	return factory.BuildComponent(ctx, spec)
}

// AsPrompt converts a built instance to prompt.ChatTemplate.
func AsPrompt(instance any) (prompt.ChatTemplate, error) {
	v, ok := instance.(prompt.ChatTemplate)
	if !ok {
		return nil, fmt.Errorf("component is %T, want prompt.ChatTemplate", instance)
	}
	return v, nil
}

// AsAgenticPrompt converts a built instance to prompt.AgenticChatTemplate.
func AsAgenticPrompt(instance any) (prompt.AgenticChatTemplate, error) {
	v, ok := instance.(prompt.AgenticChatTemplate)
	if !ok {
		return nil, fmt.Errorf("component is %T, want prompt.AgenticChatTemplate", instance)
	}
	return v, nil
}

// AsChatModel converts a built instance to model.BaseChatModel.
func AsChatModel(instance any) (model.BaseChatModel, error) {
	v, ok := instance.(model.BaseChatModel)
	if !ok {
		return nil, fmt.Errorf("component is %T, want model.BaseChatModel", instance)
	}
	return v, nil
}

// AsAgenticModel converts a built instance to model.AgenticModel.
func AsAgenticModel(instance any) (model.AgenticModel, error) {
	v, ok := instance.(model.AgenticModel)
	if !ok {
		return nil, fmt.Errorf("component is %T, want model.AgenticModel", instance)
	}
	return v, nil
}

// AsEmbedder converts a built instance to embedding.Embedder.
func AsEmbedder(instance any) (embedding.Embedder, error) {
	v, ok := instance.(embedding.Embedder)
	if !ok {
		return nil, fmt.Errorf("component is %T, want embedding.Embedder", instance)
	}
	return v, nil
}

// AsIndexer converts a built instance to indexer.Indexer.
func AsIndexer(instance any) (indexer.Indexer, error) {
	v, ok := instance.(indexer.Indexer)
	if !ok {
		return nil, fmt.Errorf("component is %T, want indexer.Indexer", instance)
	}
	return v, nil
}

// AsRetriever converts a built instance to retriever.Retriever.
func AsRetriever(instance any) (retriever.Retriever, error) {
	v, ok := instance.(retriever.Retriever)
	if !ok {
		return nil, fmt.Errorf("component is %T, want retriever.Retriever", instance)
	}
	return v, nil
}

// AsLoader converts a built instance to document.Loader.
func AsLoader(instance any) (document.Loader, error) {
	v, ok := instance.(document.Loader)
	if !ok {
		return nil, fmt.Errorf("component is %T, want document.Loader", instance)
	}
	return v, nil
}

// AsTransformer converts a built instance to document.Transformer.
func AsTransformer(instance any) (document.Transformer, error) {
	v, ok := instance.(document.Transformer)
	if !ok {
		return nil, fmt.Errorf("component is %T, want document.Transformer", instance)
	}
	return v, nil
}

// AsTool converts a built instance to tool.BaseTool.
func AsTool(instance any) (tool.BaseTool, error) {
	v, ok := instance.(tool.BaseTool)
	if !ok {
		return nil, fmt.Errorf("component is %T, want tool.BaseTool", instance)
	}
	return v, nil
}

// AsComponentKind validates the instance against the declared component kind.
func AsComponentKind(kind string, instance any) (any, error) {
	switch ComponentKind(kind) {
	case componentpkg.ComponentOfPrompt:
		return AsPrompt(instance)
	case componentpkg.ComponentOfAgenticPrompt:
		return AsAgenticPrompt(instance)
	case componentpkg.ComponentOfChatModel:
		return AsChatModel(instance)
	case componentpkg.ComponentOfAgenticModel:
		return AsAgenticModel(instance)
	case componentpkg.ComponentOfEmbedding:
		return AsEmbedder(instance)
	case componentpkg.ComponentOfIndexer:
		return AsIndexer(instance)
	case componentpkg.ComponentOfRetriever:
		return AsRetriever(instance)
	case componentpkg.ComponentOfLoader:
		return AsLoader(instance)
	case componentpkg.ComponentOfTransformer:
		return AsTransformer(instance)
	case componentpkg.ComponentOfTool:
		return AsTool(instance)
	default:
		return instance, nil
	}
}

func interpreterComponentRef(spec *ComponentSpec) Ref {
	if ref, ok := spec.Refs["component"]; ok {
		return ref
	}
	return Ref{Kind: RefKindInterpreterComponent, Target: spec.Name, Args: spec.Extra}
}
