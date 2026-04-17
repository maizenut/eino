package compose

import (
	"context"
	"reflect"
	"testing"
)

type compileProbeGraph struct {
	compiledInterceptors []NodeInterceptor
}

func (g *compileProbeGraph) getGenericHelper() *genericHelper { return nil }
func (g *compileProbeGraph) inputType() reflect.Type          { return nil }
func (g *compileProbeGraph) outputType() reflect.Type         { return nil }
func (g *compileProbeGraph) component() component             { return ComponentOfGraph }
func (g *compileProbeGraph) compile(ctx context.Context, options *graphCompileOptions) (*composableRunnable, error) {
	g.compiledInterceptors = append([]NodeInterceptor(nil), options.nodeInterceptors...)
	return &composableRunnable{}, nil
}

func TestWithNodeInterceptorsInjectsCompileOptions(t *testing.T) {
	probe := &compileProbeGraph{}
	interceptor := &BaseNodeInterceptor{}
	wrapped := WithNodeInterceptors(probe, interceptor)
	_, err := wrapped.compile(context.Background(), &graphCompileOptions{})
	if err != nil {
		t.Fatalf("compile error = %v", err)
	}
	if len(probe.compiledInterceptors) != 1 || probe.compiledInterceptors[0] != interceptor {
		t.Fatalf("compiled interceptors = %#v, want interceptor", probe.compiledInterceptors)
	}
}
