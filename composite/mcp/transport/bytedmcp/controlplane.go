package bytedmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"sync"
	"time"
)

// ControlPlaneResolver resolves a PSM/region/env triple into a gateway URL.
type ControlPlaneResolver interface {
	Resolve(ctx context.Context, psm, region, env string) (string, error)
}

// StaticControlPlane returns a fixed URL regardless of input. When URL is
// empty, callers must provide spec.URL or metadata.gateway_url and the
// resolver will not be invoked.
type StaticControlPlane struct {
	URL string
}

// Resolve returns the configured static URL.
func (s *StaticControlPlane) Resolve(_ context.Context, _, _, _ string) (string, error) {
	if s == nil || s.URL == "" {
		return "", fmt.Errorf("static control plane URL is not configured")
	}
	return s.URL, nil
}

// HTTPControlPlane resolves gateway URLs through an HTTP control plane that
// mirrors the ByteDance internal /v1/psm contract.
type HTTPControlPlane struct {
	BaseURL string
	Doer    Doer
	TTL     time.Duration

	mu    sync.Mutex
	cache map[string]controlPlaneEntry
}

type controlPlaneEntry struct {
	url       string
	expiresAt time.Time
}

// Resolve returns a gateway URL, hitting the HTTP control plane on cache miss.
func (h *HTTPControlPlane) Resolve(ctx context.Context, psm, region, env string) (string, error) {
	if h == nil {
		return "", fmt.Errorf("http control plane is nil")
	}
	if psm == "" {
		return "", fmt.Errorf("control plane psm is required")
	}
	if h.BaseURL == "" {
		return "", fmt.Errorf("http control plane base url is required")
	}
	ttl := h.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	cacheKey := psm + "|" + region + "|" + env
	now := time.Now()

	h.mu.Lock()
	if entry, ok := h.cache[cacheKey]; ok && entry.expiresAt.After(now) {
		h.mu.Unlock()
		return entry.url, nil
	}
	h.mu.Unlock()

	doer := h.Doer
	if doer == nil {
		doer = &nethttp.Client{Timeout: 15 * time.Second}
	}

	path := fmt.Sprintf("/v1/psm/%s", psm)
	if env != "" && env != "prod" {
		path = fmt.Sprintf("/v1/psm/%s/env/%s", psm, env)
	}
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, h.BaseURL+path, nil)
	if err != nil {
		return "", fmt.Errorf("control plane build request: %w", err)
	}
	req.Header.Set("X-Mcp-Controlplane-Request", "true")
	if region != "" {
		req.Header.Set("X-Mcp-Region", region)
	}
	resp, err := doer.Do(req)
	if err != nil {
		return "", fmt.Errorf("control plane request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("control plane read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("control plane status %d: %s", resp.StatusCode, string(body))
	}
	var payload struct {
		Code  int64  `json:"code"`
		Error string `json:"error,omitempty"`
		Data  struct {
			GatewayURL string `json:"gateway_url"`
			ServerID   string `json:"server_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("control plane decode: %w", err)
	}
	if payload.Code != 0 {
		return "", fmt.Errorf("control plane code %d: %s", payload.Code, payload.Error)
	}
	url := payload.Data.GatewayURL
	if url == "" {
		return "", fmt.Errorf("control plane returned empty gateway url")
	}

	h.mu.Lock()
	if h.cache == nil {
		h.cache = map[string]controlPlaneEntry{}
	}
	h.cache[cacheKey] = controlPlaneEntry{url: url, expiresAt: now.Add(ttl)}
	h.mu.Unlock()
	return url, nil
}
