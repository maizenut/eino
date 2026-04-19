/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package interceptor

import "context"

// Run executes the node with the interceptor chain applied.
//
// The execution order matches compose runtime semantics:
//   - BeforeNode: forward order
//   - WrapNode: onion wrapping
//   - AfterNode: reverse order
//   - OnErrorNode: reverse order
func Run(ctx context.Context, info NodeInfo, input any, interceptors []NodeInterceptor, next NodeExecutor) (any, error) {
	if len(interceptors) == 0 {
		return next(ctx, input)
	}

	base := next
	for i := len(interceptors) - 1; i >= 0; i-- {
		base = interceptors[i].WrapNode(ctx, info, base)
	}

	for _, ni := range interceptors {
		var err error
		ctx, input, err = ni.BeforeNode(ctx, info, input)
		if err != nil {
			return nil, runError(ctx, info, interceptors, err)
		}
	}

	output, err := base(ctx, input)
	if err != nil {
		return nil, runError(ctx, info, interceptors, err)
	}

	for i := len(interceptors) - 1; i >= 0; i-- {
		outputCtx, outputValue, afterErr := interceptors[i].AfterNode(ctx, info, output)
		ctx = outputCtx
		if afterErr != nil {
			return nil, runError(ctx, info, interceptors[:i+1], afterErr)
		}
		output = outputValue
	}

	return output, nil
}

func runError(ctx context.Context, info NodeInfo, interceptors []NodeInterceptor, err error) error {
	for i := len(interceptors) - 1; i >= 0; i-- {
		var nextCtx context.Context
		nextCtx, err = interceptors[i].OnErrorNode(ctx, info, err)
		ctx = nextCtx
	}
	return err
}
