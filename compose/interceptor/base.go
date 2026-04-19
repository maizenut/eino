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

// BaseNodeInterceptor provides a no-op implementation of NodeInterceptor.
type BaseNodeInterceptor struct{}

// BeforeNode implements NodeInterceptor.
func (b *BaseNodeInterceptor) BeforeNode(ctx context.Context, _ NodeInfo, input any) (context.Context, any, error) {
	return ctx, input, nil
}

// AfterNode implements NodeInterceptor.
func (b *BaseNodeInterceptor) AfterNode(ctx context.Context, _ NodeInfo, output any) (context.Context, any, error) {
	return ctx, output, nil
}

// OnErrorNode implements NodeInterceptor.
func (b *BaseNodeInterceptor) OnErrorNode(ctx context.Context, _ NodeInfo, err error) (context.Context, error) {
	return ctx, err
}

// WrapNode implements NodeInterceptor.
func (b *BaseNodeInterceptor) WrapNode(_ context.Context, _ NodeInfo, next NodeExecutor) NodeExecutor {
	return next
}
