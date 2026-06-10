package v2rayn

import (
	"strings"
	"testing"

	"github.com/use632/clash-node-pipeline/internal/model"
)

func TestSSLinkRoundTrip(t *testing.T) {
	n := model.Node{
		Name: "HK-1", Type: "ss", Server: "example.com", Port: 8388,
		Raw: map[string]any{"cipher": "aes-128-gcm", "password": "secret"},
	}
	link, ok := nodeToLink(n)
	if !ok || !strings.HasPrefix(link, "ss://") {
		t.Fatalf("ss link build failed: %q ok=%v", link, ok)
	}
}

func TestVmessLinkBuild(t *testing.T) {
	n := model.Node{
		Name: "JP", Type: "vmess", Server: "1.2.3.4", Port: 443,
		Raw: map[string]any{"uuid": "uuid-x", "alterId": 0, "network": "ws", "tls": true},
	}
	link, ok := nodeToLink(n)
	if !ok || !strings.HasPrefix(link, "vmess://") {
		t.Fatalf("vmess link build failed: %q ok=%v", link, ok)
	}
}

func TestVlessAndTrojanRequireCreds(t *testing.T) {
	vless := model.Node{Name: "x", Type: "vless", Server: "h", Port: 1, Raw: map[string]any{}}
	if _, ok := nodeToLink(vless); ok {
		t.Fatal("vless without uuid should be skipped")
	}
	trojan := model.Node{Name: "x", Type: "trojan", Server: "h", Port: 1, Raw: map[string]any{}}
	if _, ok := nodeToLink(trojan); ok {
		t.Fatal("trojan without password should be skipped")
	}
}

func TestBuildLinksCountsSkipped(t *testing.T) {
	nodes := []model.Node{
		{Name: "ok", Type: "trojan", Server: "h", Port: 1, Raw: map[string]any{"password": "p"}},
		{Name: "bad", Type: "anytls", Server: "h", Port: 1, Raw: map[string]any{}},
	}
	links, skipped := BuildLinks(nodes)
	if len(links) != 1 || skipped != 1 {
		t.Fatalf("expected 1 link 1 skipped, got %d/%d", len(links), skipped)
	}
}

func TestHTTPAndSocksSkipped(t *testing.T) {
	// v2rayN does not import plain http/socks proxies from a subscription, so we
	// must not emit them (otherwise the link count won't match what v2rayN shows).
	for _, typ := range []string{"http", "https", "socks", "socks5"} {
		n := model.Node{Name: "x", Type: typ, Server: "h", Port: 8080, Raw: map[string]any{"username": "u", "password": "p"}}
		if link, ok := nodeToLink(n); ok {
			t.Fatalf("%s should be skipped for v2rayN, got %q", typ, link)
		}
	}
}
