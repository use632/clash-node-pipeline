// Package v2rayn converts pipeline nodes back into share links and writes a
// v2rayN-style subscription file (one link per line, whole content base64).
package v2rayn

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/use632/clash-node-pipeline/internal/model"
)

// BuildLinks converts nodes to share links, skipping unsupported ones.
// It returns the links and the count skipped.
func BuildLinks(nodes []model.Node) ([]string, int) {
	var links []string
	skipped := 0
	for _, n := range nodes {
		link, ok := nodeToLink(n)
		if !ok {
			skipped++
			continue
		}
		links = append(links, link)
	}
	return links, skipped
}

// Write produces a v2rayN subscription file: links joined by newlines, then
// the whole blob base64-encoded (standard encoding), which is what v2rayN
// expects from a subscription URL/file.
func Write(path string, nodes []model.Node) (int, int, error) {
	links, skipped := BuildLinks(nodes)
	plain := strings.Join(links, "\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(plain))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, 0, err
	}
	if err := os.WriteFile(path, []byte(encoded), 0o644); err != nil {
		return 0, 0, err
	}
	return len(links), skipped, nil
}

func nodeToLink(n model.Node) (string, bool) {
	switch strings.ToLower(n.Type) {
	case "vmess":
		return vmessLink(n)
	case "vless":
		return vlessLink(n)
	case "trojan":
		return trojanLink(n)
	case "ss", "shadowsocks":
		return ssLink(n)
	case "socks5", "socks":
		return userPassLink("socks", n)
	case "http", "https":
		return userPassLink("http", n)
	default:
		return "", false
	}
}

func rawStr(n model.Node, key string) string {
	if v, ok := n.Raw[key]; ok {
		switch x := v.(type) {
		case string:
			return x
		case bool:
			if x {
				return "true"
			}
			return "false"
		default:
			return fmt.Sprint(x)
		}
	}
	return ""
}

func rawBool(n model.Node, key string) bool {
	if v, ok := n.Raw[key]; ok {
		switch x := v.(type) {
		case bool:
			return x
		case string:
			return x == "true" || x == "1"
		}
	}
	return false
}

func hostPort(n model.Node) string {
	return net.JoinHostPort(n.Server, fmt.Sprint(n.Port))
}

func frag(name string) string {
	return "#" + url.PathEscape(name)
}
