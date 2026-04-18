package builder

import "context"

type runtimeNodeWrapper func(NodeResolver) NodeResolver

func buildRuntimeResolverPipeline(plan ExecutionPlan, base NodeResolver) NodeResolver {
	resolver := base
	for _, wrap := range runtimeWrappers(plan) {
		resolver = wrap(resolver)
	}
	return resolver
}

func runtimeWrappers(plan ExecutionPlan) []runtimeNodeWrapper {
	return []runtimeNodeWrapper{
		func(base NodeResolver) NodeResolver { return newRetryResolver(plan, base) },
		func(base NodeResolver) NodeResolver { return newFallbackResolver(plan, base) },
		func(base NodeResolver) NodeResolver { return newLoopResolver(plan, base) },
		func(base NodeResolver) NodeResolver { return newParallelResolver(plan, base) },
		func(base NodeResolver) NodeResolver { return newReplayResolver(plan, base) },
	}
}

type runtimePipelineResolver struct {
	base     NodeResolver
	wrappers []runtimeNodeWrapper
}

func (r *runtimePipelineResolver) ResolveNode(ctx context.Context, node PlannedNode, binding *BindingSpec, policy *PolicySpec) (any, error) {
	resolver := r.base
	for _, wrap := range r.wrappers {
		resolver = wrap(resolver)
	}
	return resolver.ResolveNode(ctx, node, binding, policy)
}
