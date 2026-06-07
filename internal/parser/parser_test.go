package parser

import (
	"encoding/base64"
	"testing"
)

func TestParseYAMLDuplicateNamesAndBrokenNode(t *testing.T) {
	content := []byte(`proxies:
  - name: Same
    type: ss
    server: example.com
    port: 8388
    cipher: aes-128-gcm
    password: p1
  - name: Same
    type: trojan
    server: example.org
    port: "443"
    password: p2
  - name: Broken
    type: ss
    port: nope
`)
	nodes, issues := ParseContent("test", content)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 valid nodes, got %d", len(nodes))
	}
	if len(issues) == 0 {
		t.Fatalf("expected at least one issue for broken node")
	}
}

func TestParseBase64Subscription(t *testing.T) {
	raw := "trojan://secret@example.com:443#T1\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	nodes, issues := ParseContent("b64", []byte(encoded))
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d, issues=%v", len(nodes), issues)
	}
	if nodes[0].Name != "T1" || nodes[0].Type != "trojan" {
		t.Fatalf("unexpected node: %#v", nodes[0])
	}
	if nodes[0].Raw["password"] != "secret" {
		t.Fatalf("trojan password not mapped: %#v", nodes[0].Raw)
	}
	if _, ok := nodes[0].Raw["username"]; ok {
		t.Fatalf("trojan URI must not create username: %#v", nodes[0].Raw)
	}
}

func TestParseHTTPUserOnlyDoesNotDuplicatePassword(t *testing.T) {
	node, err := parseURI("test", "http://user-only@example.com:8080#HTTP")
	if err != nil {
		t.Fatal(err)
	}
	if node.Type != "http" || node.Raw["username"] != "user-only" {
		t.Fatalf("http username not mapped: %#v", node.Raw)
	}
	if _, ok := node.Raw["password"]; ok {
		t.Fatalf("http URI with username only must not create password: %#v", node.Raw)
	}
}

func TestParseHTTPUserPassword(t *testing.T) {
	node, err := parseURI("test", "http://user:pass@example.com:8080#HTTP")
	if err != nil {
		t.Fatal(err)
	}
	if node.Raw["username"] != "user" || node.Raw["password"] != "pass" {
		t.Fatalf("http auth not mapped: %#v", node.Raw)
	}
}

func TestParseVlessUUID(t *testing.T) {
	node, err := parseURI("test", "vless://uuid-value@example.com:443?security=tls&sni=sni.example#VLESS")
	if err != nil {
		t.Fatal(err)
	}
	if node.Raw["uuid"] != "uuid-value" || node.Raw["password"] != nil {
		t.Fatalf("vless uuid mapping wrong: %#v", node.Raw)
	}
	if node.Raw["tls"] != true || node.Raw["sni"] != "sni.example" {
		t.Fatalf("vless tls/sni mapping wrong: %#v", node.Raw)
	}
}

func TestParseVmessShareLink(t *testing.T) {
	// payload chosen so its StdEncoding contains '+', which must not be
	// corrupted by url unescaping.
	jsonCfg := `{"v":"2","ps":"JP","add":"jp.example.com","port":"11122","id":"6b989fce-172e-37a6-b324-3922008b17f9","aid":"0","net":"tcp","scy":"auto","type":"none","tls":""}`
	link := "vmess://" + base64.StdEncoding.EncodeToString([]byte(jsonCfg))
	nodes, issues := ParseContent("vmess", []byte(link))
	if len(nodes) != 1 {
		t.Fatalf("expected 1 vmess node, got %d issues=%v", len(nodes), issues)
	}
	if nodes[0].Type != "vmess" || nodes[0].Server != "jp.example.com" || nodes[0].Port != 11122 {
		t.Fatalf("vmess fields wrong: %#v", nodes[0])
	}
	if nodes[0].Raw["uuid"] != "6b989fce-172e-37a6-b324-3922008b17f9" {
		t.Fatalf("vmess uuid wrong: %#v", nodes[0].Raw)
	}
}

func TestLinkListNoYAMLNoise(t *testing.T) {
	content := []byte("trojan://p@h:443#A\nvless://u@h2:443#B\n")
	_, issues := ParseContent("links", content)
	for _, is := range issues {
		if is.Err != "" && contains(is.Err, "yaml") {
			t.Fatalf("share-link list should not emit yaml issues: %v", issues)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
