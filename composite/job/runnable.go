package job

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/cloudwego/eino/compose"
	skillpkg "github.com/cloudwego/eino/composite/skill"
)

var (
	contextType  = reflect.TypeOf((*context.Context)(nil)).Elem()
	errorType    = reflect.TypeOf((*error)(nil)).Elem()
	inputMapType = reflect.TypeOf(map[string]any{})
)

// Runnable is the minimal executable abstraction handled by the scheduler.
type Runnable interface {
	Run(ctx context.Context, input map[string]any) (any, error)
}

type runnableFunc func(ctx context.Context, input map[string]any) (any, error)

func (f runnableFunc) Run(ctx context.Context, input map[string]any) (any, error) {
	return f(ctx, input)
}

// AsRunnable adapts common runtime objects into job.Runnable.
func AsRunnable(ctx context.Context, value any) (Runnable, error) {
	if value == nil {
		return nil, fmt.Errorf("runnable value is required")
	}
	if runnable, ok := value.(Runnable); ok {
		return runnable, nil
	}
	if graph, ok := value.(compose.AnyGraph); ok {
		return &graphRunnable{graph: graph}, nil
	}
	if skillRunnable, ok := value.(skillpkg.Runnable); ok {
		return &skillGraphRunnable{skill: skillRunnable}, nil
	}
	if runnable, ok := adaptTypedFunction(value); ok {
		return runnable, nil
	}
	if runnable, ok := adaptReflectiveMethod(value, "Run"); ok {
		return runnable, nil
	}
	if runnable, ok := adaptReflectiveMethod(value, "Invoke"); ok {
		return runnable, nil
	}
	if runnable, ok := adaptReflectiveFunction(value); ok {
		return runnable, nil
	}
	_ = ctx
	return nil, fmt.Errorf("unsupported runnable type %T", value)
}

func adaptTypedFunction(value any) (Runnable, bool) {
	switch fn := value.(type) {
	case func(context.Context, map[string]any) (any, error):
		return runnableFunc(fn), true
	case func(context.Context, map[string]any) any:
		return runnableFunc(func(ctx context.Context, input map[string]any) (any, error) {
			return fn(ctx, input), nil
		}), true
	case func(map[string]any) (any, error):
		return runnableFunc(func(ctx context.Context, input map[string]any) (any, error) {
			_ = ctx
			return fn(input)
		}), true
	case func(map[string]any) any:
		return runnableFunc(func(ctx context.Context, input map[string]any) (any, error) {
			_ = ctx
			return fn(input), nil
		}), true
	default:
		return nil, false
	}
}

func adaptReflectiveMethod(target any, method string) (Runnable, bool) {
	rv := reflect.ValueOf(target)
	if !rv.IsValid() {
		return nil, false
	}
	mv := rv.MethodByName(method)
	if !mv.IsValid() || !isCallableSignature(mv.Type()) {
		return nil, false
	}
	return runnableFunc(func(ctx context.Context, input map[string]any) (any, error) {
		return callCallable(mv, ctx, input)
	}), true
}

func adaptReflectiveFunction(value any) (Runnable, bool) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Func || !isCallableSignature(rv.Type()) {
		return nil, false
	}
	return runnableFunc(func(ctx context.Context, input map[string]any) (any, error) {
		return callCallable(rv, ctx, input)
	}), true
}

func isCallableSignature(rt reflect.Type) bool {
	if rt.Kind() != reflect.Func {
		return false
	}
	if rt.NumIn() == 0 || rt.NumIn() > 3 {
		return false
	}
	switch rt.NumIn() {
	case 1:
		if !acceptsInputType(rt.In(0)) {
			return false
		}
	case 2:
		if !rt.In(0).Implements(contextType) || !acceptsInputType(rt.In(1)) {
			return false
		}
	case 3:
		if !rt.IsVariadic() || !rt.In(0).Implements(contextType) || !acceptsInputType(rt.In(1)) {
			return false
		}
	}
	if rt.NumOut() == 1 {
		return true
	}
	return rt.NumOut() == 2 && rt.Out(1).Implements(errorType)
}

func acceptsInputType(rt reflect.Type) bool {
	if rt == inputMapType {
		return true
	}
	if rt.Kind() == reflect.Interface {
		return true
	}
	return inputMapType.AssignableTo(rt) || inputMapType.ConvertibleTo(rt)
}

func callCallable(fn reflect.Value, ctx context.Context, input map[string]any) (any, error) {
	args := make([]reflect.Value, 0, fn.Type().NumIn())
	switch fn.Type().NumIn() {
	case 2:
		args = append(args, reflect.ValueOf(ctx))
		args = append(args, convertInputValue(input, fn.Type().In(1)))
	case 3:
		args = append(args, reflect.ValueOf(ctx))
		args = append(args, convertInputValue(input, fn.Type().In(1)))
		args = append(args, reflect.MakeSlice(fn.Type().In(2), 0, 0))
	case 1:
		args = append(args, convertInputValue(input, fn.Type().In(0)))
	default:
		return nil, fmt.Errorf("unsupported callable signature %s", fn.Type())
	}
	var results []reflect.Value
	if fn.Type().IsVariadic() {
		results = fn.CallSlice(args)
	} else {
		results = fn.Call(args)
	}
	if len(results) == 2 && !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0].Interface(), nil
}

func convertInputValue(input map[string]any, target reflect.Type) reflect.Value {
	if target == inputMapType {
		return reflect.ValueOf(input)
	}
	if target.Kind() == reflect.Interface {
		if input == nil {
			return reflect.Zero(target)
		}
		value := reflect.ValueOf(input)
		if value.Type().AssignableTo(target) {
			return value
		}
		return value.Convert(target)
	}
	if input == nil {
		return reflect.Zero(target)
	}
	value := reflect.ValueOf(input)
	if value.Type().AssignableTo(target) {
		return value
	}
	return value.Convert(target)
}

type graphRunnable struct {
	graph compose.AnyGraph

	once     sync.Once
	compiled any
	err      error
}

func (r *graphRunnable) Run(ctx context.Context, input map[string]any) (any, error) {
	compiled, err := r.compile(ctx)
	if err != nil {
		return nil, err
	}
	if runnable, ok := adaptReflectiveMethod(compiled, "Invoke"); ok {
		return runnable.Run(ctx, input)
	}
	return nil, fmt.Errorf("compiled graph %T does not expose Invoke", compiled)
}

func (r *graphRunnable) compile(ctx context.Context) (any, error) {
	r.once.Do(func() {
		method := reflect.ValueOf(r.graph).MethodByName("Compile")
		if !method.IsValid() {
			r.err = fmt.Errorf("graph %T does not expose Compile", r.graph)
			return
		}
		results := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
		if len(results) != 2 {
			r.err = fmt.Errorf("graph Compile returned %d values, want 2", len(results))
			return
		}
		if !results[1].IsNil() {
			r.err = results[1].Interface().(error)
			return
		}
		r.compiled = results[0].Interface()
	})
	return r.compiled, r.err
}

type skillGraphRunnable struct {
	skill skillpkg.Runnable
}

func (r *skillGraphRunnable) Run(ctx context.Context, input map[string]any) (any, error) {
	graph, ok, err := r.skill.Graph(ctx)
	if err != nil {
		return nil, err
	}
	if !ok || graph == nil {
		return nil, fmt.Errorf("skill %q does not expose an executable graph", r.skill.Info().Name)
	}
	runnable, err := AsRunnable(ctx, graph)
	if err != nil {
		return nil, err
	}
	return runnable.Run(ctx, input)
}
