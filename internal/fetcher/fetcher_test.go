package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/use632/clash-node-pipeline/internal/model"
)

func TestParseProxy(t *testing.T) {
	cases := []struct {
		in      string
		want    string // expected u.String(); "" means nil
		wantErr bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"7890", "http://127.0.0.1:7890", false},
		{"127.0.0.1:7890", "http://127.0.0.1:7890", false},
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890", false},
		{"socks5://127.0.0.1:7891", "socks5://127.0.0.1:7891", false},
		{"ftp://host:1", "", true},
		{"http://", "", true},
	}
	for _, c := range cases {
		u, err := ParseProxy(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseProxy(%q) expected error, got %v", c.in, u)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseProxy(%q) unexpected error: %v", c.in, err)
			continue
		}
		got := ""
		if u != nil {
			got = u.String()
		}
		if got != c.want {
			t.Errorf("ParseProxy(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// When a proxy is configured, a reachable proxy is used first.
func TestFetchUsesProxyFirst(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An HTTP forward proxy receives the absolute target URL; respond directly.
		_, _ = w.Write([]byte("via-proxy-body"))
	}))
	defer proxy.Close()

	cfg := model.FetchConfig{TimeoutSeconds: 5, Retries: 0, Proxy: proxy.URL}
	clients := buildClients(cfg)
	src := model.Source{Name: "s", URL: "http://example.invalid/sub", Enabled: true}
	res := fetchOne(context.Background(), clients, src, cfg)
	if res.Err != "" {
		t.Fatalf("fetch failed: %s", res.Err)
	}
	if res.Via != "代理" {
		t.Fatalf("expected Via=代理, got %q", res.Via)
	}
	if string(res.Content) != "via-proxy-body" {
		t.Fatalf("unexpected body: %q", res.Content)
	}
}

// When the proxy is down, fetching falls back to a direct connection.
func TestFetchFallsBackToDirect(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("direct-body"))
	}))
	defer origin.Close()

	// Point the proxy at a closed port so the proxied attempt fails fast.
	cfg := model.FetchConfig{TimeoutSeconds: 5, Retries: 0, Proxy: "http://127.0.0.1:1"}
	clients := buildClients(cfg)
	src := model.Source{Name: "s", URL: origin.URL, Enabled: true}
	res := fetchOne(context.Background(), clients, src, cfg)
	if res.Err != "" {
		t.Fatalf("fetch failed (should have fallen back to direct): %s", res.Err)
	}
	if res.Via != "直连" {
		t.Fatalf("expected Via=直连, got %q", res.Via)
	}
	if string(res.Content) != "direct-body" {
		t.Fatalf("unexpected body: %q", res.Content)
	}
}
