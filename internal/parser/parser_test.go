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

func TestParseHTMLPageEmbeddedLinks(t *testing.T) {
	// Simulates a blog post that lists share links inside HTML markup: links
	// wrapped in tags, query separators HTML-encoded as &amp;, the same link
	// repeated, and ordinary website URLs (http/https) that must NOT be
	// mistaken for proxy nodes.
	htmlPage := `<!doctype html><html><head><meta charset="utf-8"><title>free nodes</title></head>
<body>
<p>访问 <a href="https://example.com/page">官网</a> 获取更多节点。</p>
<div class="post-content">
<p>trojan://pass123@tj.example.com:443?sni=tj.example.com#TJ-01</p>
<code>vless://uuid-1234@vl.example.com:443?type=ws&amp;security=tls&amp;sni=vl.example.com#VL-Node</code>
<li>ss://YWVzLTEyOC1nY206cGFzc3dvcmQ=@ss.example.com:8388#SS-Tokyo</li>
<pre>trojan://pass123@tj.example.com:443?sni=tj.example.com#TJ-01</pre>
</div>
<script src="https://cdn.example.com/app.js"></script>
</body></html>`
	nodes, _ := ParseContent("blog", []byte(htmlPage))
	if len(nodes) != 3 {
		t.Fatalf("expected 3 unique proxy nodes, got %d: %#v", len(nodes), nodes)
	}
	byType := map[string]int{}
	for _, n := range nodes {
		byType[n.Type]++
		if n.Type == "http" || n.Type == "https" {
			t.Fatalf("website URL leaked in as a proxy node: %#v", n)
		}
	}
	if byType["trojan"] != 1 || byType["vless"] != 1 || byType["ss"] != 1 {
		t.Fatalf("unexpected node type distribution %v: %#v", byType, nodes)
	}
	// The &amp;-encoded query must be decoded so ws/tls survive on the vless node.
	for _, n := range nodes {
		if n.Type != "vless" {
			continue
		}
		if n.Raw["network"] != "ws" || n.Raw["tls"] != true {
			t.Fatalf("vless ws/tls query not decoded from &amp;: %#v", n.Raw)
		}
	}
}

func TestParseHTMLPageNoLinksReportsIssue(t *testing.T) {
	htmlPage := `<!doctype html><html><body><p>今天没有节点，请明天再来。</p></body></html>`
	nodes, issues := ParseContent("blog", []byte(htmlPage))
	if len(nodes) != 0 {
		t.Fatalf("expected no nodes from a link-free page, got %d", len(nodes))
	}
	if len(issues) == 0 {
		t.Fatalf("expected an issue explaining no links were found")
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
