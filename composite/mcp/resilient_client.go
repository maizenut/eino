package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	declarative "github.com/cloudwego/eino/schema/declarative"
)

// WrapResilientClient wraps a raw transport client with retry/timeout/auto-reconnect semantics.
//
// This keeps transport implementations simple while ensuring the runtime behavior matches
// docs/6.4-mcp.md expectations (connection lifecycle management vs. capability consumption).
func WrapResilientClient(inner Client) Client {
	if inner == nil {
		return nil
	}
	if _, ok := inner.(*ResilientClient); ok {
		return inner
	}
	return &ResilientClient{inner: inner}
}

// ResilientClient decorates an MCP transport client with:
// - per-operation timeout (WithTimeout)
// - auto reconnect (WithAutoReconnect)
// - connect retry policy (ServerSpec.Retry)
type ResilientClient struct {
	inner Client

	mu        sync.Mutex
	lastSpec  *ServerSpec
	connected bool
}

func (c *ResilientClient) Connect(ctx context.Context, spec ServerSpec, opts ...Option) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("mcp client is required")
	}
	ctx, cancel := applyTimeout(ctx, opts)
	defer cancel()

	cloned := spec
	c.mu.Lock()
	c.lastSpec = &cloned
	c.mu.Unlock()

	if err := connectWithRetry(ctx, c.inner, cloned, opts...); err != nil {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()
		return err
	}
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return nil
}

func (c *ResilientClient) Disconnect(ctx context.Context) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("mcp client is required")
	}
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	return c.inner.Disconnect(ctx)
}

func (c *ResilientClient) Info(ctx context.Context) (*ServerInfo, error) {
	return c.inner.Info(ctx)
}

func (c *ResilientClient) ListTools(ctx context.Context, opts ...Option) ([]ToolDescriptor, error) {
	var out []ToolDescriptor
	err := c.withReconnect(ctx, func(ctx context.Context) error {
		v, err := c.inner.ListTools(ctx, opts...)
		if err != nil {
			return err
		}
		out = v
		return nil
	}, opts...)
	return out, err
}

func (c *ResilientClient) CallTool(ctx context.Context, name string, args map[string]any, opts ...Option) (any, error) {
	var out any
	err := c.withReconnect(ctx, func(ctx context.Context) error {
		v, err := c.inner.CallTool(ctx, name, args, opts...)
		if err != nil {
			return err
		}
		out = v
		return nil
	}, opts...)
	return out, err
}

func (c *ResilientClient) ListResources(ctx context.Context, opts ...Option) ([]ResourceDescriptor, error) {
	var out []ResourceDescriptor
	err := c.withReconnect(ctx, func(ctx context.Context) error {
		v, err := c.inner.ListResources(ctx, opts...)
		if err != nil {
			return err
		}
		out = v
		return nil
	}, opts...)
	return out, err
}

func (c *ResilientClient) ReadResource(ctx context.Context, uri string, opts ...Option) (any, error) {
	var out any
	err := c.withReconnect(ctx, func(ctx context.Context) error {
		v, err := c.inner.ReadResource(ctx, uri, opts...)
		if err != nil {
			return err
		}
		out = v
		return nil
	}, opts...)
	return out, err
}

func (c *ResilientClient) ListPrompts(ctx context.Context, opts ...Option) ([]PromptDescriptor, error) {
	var out []PromptDescriptor
	err := c.withReconnect(ctx, func(ctx context.Context) error {
		v, err := c.inner.ListPrompts(ctx, opts...)
		if err != nil {
			return err
		}
		out = v
		return nil
	}, opts...)
	return out, err
}

func (c *ResilientClient) GetPrompt(ctx context.Context, name string, args map[string]any, opts ...Option) (string, error) {
	var out string
	err := c.withReconnect(ctx, func(ctx context.Context) error {
		v, err := c.inner.GetPrompt(ctx, name, args, opts...)
		if err != nil {
			return err
		}
		out = v
		return nil
	}, opts...)
	return out, err
}

func (c *ResilientClient) withReconnect(ctx context.Context, fn func(context.Context) error, opts ...Option) error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("mcp client is required")
	}
	ctx, cancel := applyTimeout(ctx, opts)
	defer cancel()

	resolved := applyOptions(opts)
	if err := c.ensureConnected(ctx, resolved); err != nil {
		return err
	}

	callErr := fn(ctx)
	if callErr == nil {
		return nil
	} else if !resolved.AutoReconnect {
		return callErr
	}

	// One more attempt after reconnect.
	c.mu.Lock()
	c.connected = false
	spec := cloneServerSpec(c.lastSpec)
	c.mu.Unlock()
	if spec == nil {
		return callErr
	}
	if err2 := connectWithRetry(ctx, c.inner, *spec, opts...); err2 != nil {
		return callErr
	}
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return fn(ctx)
}

func (c *ResilientClient) ensureConnected(ctx context.Context, opts options) error {
	c.mu.Lock()
	connected := c.connected
	spec := cloneServerSpec(c.lastSpec)
	c.mu.Unlock()
	if connected {
		return nil
	}
	if spec == nil {
		return fmt.Errorf("mcp client is not connected")
	}
	if !opts.AutoReconnect {
		return fmt.Errorf("mcp client is not connected")
	}
	if err := connectWithRetry(ctx, c.inner, *spec); err != nil {
		return err
	}
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return nil
}

func applyTimeout(ctx context.Context, opts []Option) (context.Context, func()) {
	resolved := applyOptions(opts)
	if resolved.TimeoutMS <= 0 {
		return ctx, func() {}
	}
	timeout := time.Duration(resolved.TimeoutMS) * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		// If caller already provided a tighter deadline, keep it.
		if time.Until(deadline) <= timeout {
			return ctx, func() {}
		}
	}
	return context.WithTimeout(ctx, timeout)
}

func connectWithRetry(ctx context.Context, client Client, spec ServerSpec, opts ...Option) error {
	if client == nil {
		return fmt.Errorf("mcp client is required")
	}
	attempts := 1
	backoff := time.Duration(0)
	if spec.Retry != nil {
		if spec.Retry.MaxAttempts > 0 {
			attempts = spec.Retry.MaxAttempts
		}
		if spec.Retry.BackoffMS > 0 {
			backoff = time.Duration(spec.Retry.BackoffMS) * time.Millisecond
		}
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := client.Connect(ctx, spec, opts...); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < attempts-1 && backoff > 0 {
			t := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
			}
		}
	}
	return lastErr
}

func cloneServerSpec(spec *ServerSpec) *ServerSpec {
	if spec == nil {
		return nil
	}
	cloned := *spec
	if len(spec.Args) > 0 {
		cloned.Args = append([]string(nil), spec.Args...)
	}
	if len(spec.Headers) > 0 {
		headers := make(map[string]string, len(spec.Headers))
		for k, v := range spec.Headers {
			headers[k] = v
		}
		cloned.Headers = headers
	}
	if len(spec.Metadata) > 0 {
		meta := make(map[string]any, len(spec.Metadata))
		for k, v := range spec.Metadata {
			meta[k] = v
		}
		cloned.Metadata = meta
	}
	if spec.Retry != nil {
		retry := *spec.Retry
		cloned.Retry = &retry
	}
	if len(spec.AdapterRefs) > 0 {
		cloned.AdapterRefs = append([]declarative.Ref(nil), spec.AdapterRefs...)
	}
	if spec.ToolRef != nil {
		ref := *spec.ToolRef
		cloned.ToolRef = &ref
	}
	if spec.PromptRef != nil {
		ref := *spec.PromptRef
		cloned.PromptRef = &ref
	}
	if spec.ResourceRef != nil {
		ref := *spec.ResourceRef
		cloned.ResourceRef = &ref
	}
	return &cloned
}

var _ Client = (*ResilientClient)(nil)
