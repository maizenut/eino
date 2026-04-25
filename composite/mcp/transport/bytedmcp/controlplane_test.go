package bytedmcp

import (
	"context"
	"encoding/json"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestStaticControlPlane(t *testing.T) {
	cp := &StaticControlPlane{URL: "http://gateway/mcp"}
	url, err := cp.Resolve(context.Background(), "psm", "cn", "prod")
	if err != nil || url != "http://gateway/mcp" {
		t.Fatalf("Resolve = %q, %v", url, err)
	}

	empty := &StaticControlPlane{}
	if _, err := empty.Resolve(context.Background(), "p", "", ""); err == nil {
		t.Fatalf("expected error from empty static control plane")
	}
}

func TestHTTPControlPlane_CachesAndExpires(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		hits.Add(1)
		_, _ = io.ReadAll(r.Body)
		body, _ := json.Marshal(map[string]any{
			"code": 0,
			"data": map[string]any{"gateway_url": "http://gw.example/" + r.URL.Path},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cp := &HTTPControlPlane{BaseURL: srv.URL, Doer: srv.Client(), TTL: 50 * time.Millisecond}

	url1, err := cp.Resolve(context.Background(), "data.demo", "cn", "prod")
	if err != nil {
		t.Fatalf("Resolve1: %v", err)
	}
	url2, err := cp.Resolve(context.Background(), "data.demo", "cn", "prod")
	if err != nil {
		t.Fatalf("Resolve2: %v", err)
	}
	if url1 != url2 {
		t.Fatalf("urls differ: %s vs %s", url1, url2)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 control plane hit, got %d", hits.Load())
	}

	time.Sleep(80 * time.Millisecond)
	if _, err := cp.Resolve(context.Background(), "data.demo", "cn", "prod"); err != nil {
		t.Fatalf("Resolve3: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("expected 2 hits after TTL, got %d", hits.Load())
	}
}

func TestHTTPControlPlane_PpePath(t *testing.T) {
	var got string
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		got = r.URL.Path
		body, _ := json.Marshal(map[string]any{"code": 0, "data": map[string]any{"gateway_url": "http://gw"}})
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	cp := &HTTPControlPlane{BaseURL: srv.URL, Doer: srv.Client()}
	if _, err := cp.Resolve(context.Background(), "data.demo", "cn", "ppe1"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "/v1/psm/data.demo/env/ppe1" {
		t.Fatalf("path = %q", got)
	}
}

func TestHTTPControlPlane_RejectsErrors(t *testing.T) {
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		nethttp.Error(w, "not found", nethttp.StatusNotFound)
	}))
	defer srv.Close()
	cp := &HTTPControlPlane{BaseURL: srv.URL, Doer: srv.Client()}
	if _, err := cp.Resolve(context.Background(), "p", "", ""); err == nil {
		t.Fatalf("expected error")
	}
}
