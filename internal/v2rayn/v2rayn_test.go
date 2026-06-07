package v2rayn

import (
	"strings"
	"testing"

	"clash-node-pipeline/internal/model"
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
