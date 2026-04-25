package bytedmcp

import (
	"context"
	"fmt"
	nethttp "net/http"
	"time"

	mcppkg "github.com/cloudwego/eino/composite/mcp"
)

// Hook is the unit of cross-cutting behavior for the bytedmcp transport. The
// chain is invoked in two phases per request: BeforeCall mutates the JSON-RPC
// params (e.g. params._meta), OnHTTPRequest mutates the outgoing HTTP request
// (e.g. headers).
type Hook interface {
	BeforeCall(ctx context.Context, method string, params map[string]any) (context.Context, error)
	OnHTTPRequest(ctx context.Context, req *nethttp.Request) error
	AfterCall(ctx context.Context, method string, result any)
}

// HookChain composes hooks in order.
type HookChain []Hook

// Before runs every hook's BeforeCall.
func (hc HookChain) Before(ctx context.Context, method string, params map[string]any) (context.Context, error) {
	for _, h := range hc {
		if h == nil {
			continue
		}
		next, err := h.BeforeCall(ctx, method, params)
		if err != nil {
			return ctx, err
		}
		if next != nil {
			ctx = next
		}
	}
	return ctx, nil
}

// OnHTTP runs every hook's OnHTTPRequest.
func (hc HookChain) OnHTTP(ctx context.Context, req *nethttp.Request) error {
	for _, h := range hc {
		if h == nil {
			continue
		}
		if err := h.OnHTTPRequest(ctx, req); err != nil {
			return err
		}
	}
	return nil
}

// After runs every hook's AfterCall.
func (hc HookChain) After(ctx context.Context, method string, result any) {
	for _, h := range hc {
		if h == nil {
			continue
		}
		h.AfterCall(ctx, method, result)
	}
}

// BaseHook is an empty implementation that real hooks can embed.
type BaseHook struct{}

// BeforeCall is a no-op default.
func (BaseHook) BeforeCall(ctx context.Context, _ string, _ map[string]any) (context.Context, error) {
	return ctx, nil
}

// OnHTTPRequest is a no-op default.
func (BaseHook) OnHTTPRequest(_ context.Context, _ *nethttp.Request) error { return nil }

// AfterCall is a no-op default.
func (BaseHook) AfterCall(_ context.Context, _ string, _ any) {}

// identityTokenKey is the ctx key for forwarded identity tokens.
type identityTokenKey struct{}

// WithIdentityToken installs an identity token on ctx so that
// IdentityForwardHook can attach it to subsequent requests.
func WithIdentityToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, identityTokenKey{}, token)
}

func identityToken(ctx context.Context) string {
	if v, ok := ctx.Value(identityTokenKey{}).(string); ok {
		return v
	}
	return ""
}

// RequestTimeoutHook derives a per-request deadline.
type RequestTimeoutHook struct {
	BaseHook
	Timeout time.Duration
}

// BeforeCall installs the deadline on ctx.
func (h *RequestTimeoutHook) BeforeCall(ctx context.Context, _ string, _ map[string]any) (context.Context, error) {
	if h.Timeout <= 0 {
		return ctx, nil
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, nil
	}
	c, _ := context.WithTimeout(ctx, h.Timeout)
	return c, nil
}

// JWTGenerator produces a JWT for the given secret key.
type JWTGenerator interface {
	Generate(ctx context.Context, secretKey string) (string, error)
}

// ServiceAccountJWTHook attaches a service account JWT to outgoing requests.
type ServiceAccountJWTHook struct {
	BaseHook
	Generator JWTGenerator
	SecretKey string
}

// OnHTTPRequest sets the x-jwt-token header.
func (h *ServiceAccountJWTHook) OnHTTPRequest(ctx context.Context, req *nethttp.Request) error {
	if h.Generator == nil || h.SecretKey == "" {
		return nil
	}
	token, err := h.Generator.Generate(ctx, h.SecretKey)
	if err != nil {
		return fmt.Errorf("service account jwt: %w", err)
	}
	if token != "" {
		req.Header.Set("x-jwt-token", token)
	}
	return nil
}

// IdentityForwardHook forwards an identity token from ctx onto the request.
// It runs after ServiceAccountJWTHook so identity tokens take precedence.
type IdentityForwardHook struct{ BaseHook }

// OnHTTPRequest sets x-jwt-token from the ctx value when present.
func (h *IdentityForwardHook) OnHTTPRequest(ctx context.Context, req *nethttp.Request) error {
	if token := identityToken(ctx); token != "" {
		req.Header.Set("x-jwt-token", token)
	}
	return nil
}

// CallToolMetaHook injects headers / params / base_user_extra onto
// params._meta for tools/call requests.
type CallToolMetaHook struct {
	BaseHook
	Headers       map[string]string
	Params        map[string]any
	BaseUserExtra map[string]string
}

// BeforeCall mutates params._meta in-place.
func (h *CallToolMetaHook) BeforeCall(_ context.Context, method string, params map[string]any) (context.Context, error) {
	if method != "tools/call" {
		return nil, nil
	}
	if len(h.Headers) == 0 && len(h.Params) == 0 && len(h.BaseUserExtra) == 0 {
		return nil, nil
	}
	meta := ensureMeta(params)
	if len(h.Headers) > 0 {
		mergeHeaders(meta, "headers", h.Headers)
	}
	if len(h.Params) > 0 {
		mergeParams(meta, "params", h.Params)
	}
	if len(h.BaseUserExtra) > 0 {
		mergeHeaders(meta, "base_user_extra", h.BaseUserExtra)
	}
	return nil, nil
}

// CallToolTraceHook injects _meta.traceEnabled=true for tools/call requests.
type CallToolTraceHook struct{ BaseHook }

// BeforeCall sets the trace flag.
func (h *CallToolTraceHook) BeforeCall(_ context.Context, method string, params map[string]any) (context.Context, error) {
	if method != "tools/call" {
		return nil, nil
	}
	meta := ensureMeta(params)
	meta["traceEnabled"] = true
	return nil, nil
}

// buildBuiltinHooks builds the fixed-order hook chain from spec.Metadata.
func buildBuiltinHooks(spec mcppkg.ServerSpec) HookChain {
	meta := spec.Metadata
	chain := HookChain{}

	if ms, ok := meta["request_timeout_ms"]; ok {
		if d := durationFromMS(ms); d > 0 {
			chain = append(chain, &RequestTimeoutHook{Timeout: d})
		}
	}

	if key, _ := meta["service_account_key"].(string); key != "" {
		gen, _ := meta["service_account_jwt_generator"].(JWTGenerator)
		if gen != nil {
			chain = append(chain, &ServiceAccountJWTHook{Generator: gen, SecretKey: key})
		}
	}

	if forward, _ := meta["identity_forward"].(bool); forward {
		chain = append(chain, &IdentityForwardHook{})
	}

	metaHook := &CallToolMetaHook{
		Headers:       stringMap(meta["meta_headers"]),
		Params:        anyMap(meta["meta_params"]),
		BaseUserExtra: stringMap(meta["base_user_extra"]),
	}
	if metaHook.Headers != nil || metaHook.Params != nil || metaHook.BaseUserExtra != nil {
		chain = append(chain, metaHook)
	}

	if trace, _ := meta["call_tool_trace"].(bool); trace {
		chain = append(chain, &CallToolTraceHook{})
	}

	return chain
}

func ensureMeta(params map[string]any) map[string]any {
	existing, ok := params["_meta"].(map[string]any)
	if ok {
		return existing
	}
	meta := map[string]any{}
	params["_meta"] = meta
	return meta
}

func mergeHeaders(meta map[string]any, key string, src map[string]string) {
	if existing, ok := meta[key].(map[string]string); ok {
		merged := make(map[string]string, len(existing)+len(src))
		for k, v := range existing {
			merged[k] = v
		}
		for k, v := range src {
			merged[k] = v
		}
		meta[key] = merged
		return
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	meta[key] = out
}

func mergeParams(meta map[string]any, key string, src map[string]any) {
	if existing, ok := meta[key].(map[string]any); ok {
		merged := make(map[string]any, len(existing)+len(src))
		for k, v := range existing {
			merged[k] = v
		}
		for k, v := range src {
			merged[k] = v
		}
		meta[key] = merged
		return
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	meta[key] = out
}

func stringMap(v any) map[string]string {
	switch m := v.(type) {
	case map[string]string:
		if len(m) == 0 {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, val := range m {
			out[k] = val
		}
		return out
	case map[string]any:
		if len(m) == 0 {
			return nil
		}
		out := make(map[string]string, len(m))
		for k, val := range m {
			if s, ok := val.(string); ok {
				out[k] = s
			}
		}
		return out
	default:
		return nil
	}
}

func anyMap(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, val := range m {
		out[k] = val
	}
	return out
}

func durationFromMS(v any) time.Duration {
	switch n := v.(type) {
	case int:
		return time.Duration(n) * time.Millisecond
	case int64:
		return time.Duration(n) * time.Millisecond
	case float64:
		return time.Duration(int64(n)) * time.Millisecond
	default:
		return 0
	}
}
