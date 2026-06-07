package parser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"clash-node-pipeline/internal/model"
	"gopkg.in/yaml.v3"
)

type clashFile struct {
	Proxies []map[string]any `yaml:"proxies"`
}

func ParseContent(source string, content []byte) ([]model.Node, []model.ParseIssue) {
	texts := decodePossiblyBase64(string(content))
	var all []model.Node
	var issues []model.ParseIssue
	for _, text := range texts {
		looksYAML := strings.Contains(text, "proxies:")
		nodes, yamlIssues := parseYAML(source, []byte(text))
		all = append(all, nodes...)
		// A share-link list is not YAML; the unmarshal error there is just
		// noise, so only surface YAML issues when the text really looks like
		// a Clash config.
		if looksYAML {
			issues = append(issues, yamlIssues...)
		}
		lineNodes, lineIssues := parseLines(source, text)
		all = append(all, lineNodes...)
		issues = append(issues, lineIssues...)
	}
	return all, issues
}

func decodePossiblyBase64(text string) []string {
	trimmed := strings.TrimSpace(text)
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, trimmed)
	if len(compact) > 16 {
		if decoded, err := base64.StdEncoding.DecodeString(compact); err == nil && looksText(decoded) {
			return []string{string(decoded)}
		} else if decoded, err := base64.RawStdEncoding.DecodeString(compact); err == nil && looksText(decoded) {
			return []string{string(decoded)}
		}
	}
	return []string{text}
}

func looksText(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	bad := 0
	for _, c := range b {
		if c == 0 || (c < 9) || (c > 13 && c < 32) {
			bad++
		}
	}
	return bad*10 < len(b)
}

func parseYAML(source string, content []byte) ([]model.Node, []model.ParseIssue) {
	var cf clashFile
	if err := yaml.Unmarshal(content, &cf); err != nil {
		return nil, []model.ParseIssue{{Source: source, Err: "yaml parse failed: " + err.Error()}}
	}
	var nodes []model.Node
	var issues []model.ParseIssue
	for i, raw := range cf.Proxies {
		n, err := nodeFromMap(raw, source)
		if err != nil {
			issues = append(issues, model.ParseIssue{Source: source, Err: fmt.Sprintf("proxy[%d]: %v", i, err)})
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes, issues
}

func nodeFromMap(raw map[string]any, source string) (model.Node, error) {
	name := asString(raw["name"])
	typeName := strings.ToLower(asString(raw["type"]))
	server := asString(raw["server"])
	port, ok := asInt(raw["port"])
	if !ok {
		return model.Node{}, fmt.Errorf("invalid port")
	}
	return model.Node{Name: name, Type: typeName, Server: server, Port: port, Raw: cloneMap(raw), Source: source}, nil
}

func parseLines(source, text string) ([]model.Node, []model.ParseIssue) {
	var nodes []model.Node
	var issues []model.ParseIssue
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "://") {
			continue
		}
		n, err := parseURI(source, line)
		if err != nil {
			issues = append(issues, model.ParseIssue{Source: source, Line: truncate(line, 180), Err: err.Error()})
			continue
		}
		nodes = append(nodes, n)
	}
	return nodes, issues
}

func parseURI(source, line string) (model.Node, error) {
	scheme := strings.ToLower(strings.SplitN(line, "://", 2)[0])
	switch scheme {
	case "ss":
		return parseSSURI(source, line)
	case "vmess":
		return parseVMessURI(source, line)
	}

	u, err := url.Parse(line)
	if err != nil {
		return model.Node{}, err
	}
	typeName := strings.ToLower(u.Scheme)
	if typeName == "" || u.Hostname() == "" {
		return model.Node{}, fmt.Errorf("missing scheme or host")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port <= 0 || port > 65535 {
		return model.Node{}, fmt.Errorf("invalid port")
	}
	name := uriName(u)
	server := u.Hostname()
	raw := map[string]any{"name": name, "type": typeName, "server": server, "port": port}
	copyQuery(raw, u)

	switch typeName {
	case "trojan", "hysteria", "hysteria2", "hy2":
		setSingleUserField(raw, u, "password")
		if typeName == "hy2" {
			typeName = "hysteria2"
			raw["type"] = typeName
		}
	case "vless":
		setSingleUserField(raw, u, "uuid")
		if raw["encryption"] == nil {
			raw["encryption"] = "none"
		}
	case "tuic":
		setTupleUserFields(raw, u, "uuid", "password")
	case "socks", "socks5":
		typeName = "socks5"
		raw["type"] = typeName
		setTupleUserFields(raw, u, "username", "password")
	case "http", "https":
		if typeName == "https" {
			typeName = "http"
			raw["type"] = typeName
			raw["tls"] = true
		}
		setTupleUserFields(raw, u, "username", "password")
	default:
		setTupleUserFields(raw, u, "username", "password")
	}
	return model.Node{Name: name, Type: typeName, Server: server, Port: port, Raw: raw, Source: source}, nil
}

func parseSSURI(source, line string) (model.Node, error) {
	u, err := url.Parse(line)
	if err == nil && u.Hostname() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port <= 0 || port > 65535 {
			return model.Node{}, fmt.Errorf("invalid port")
		}
		name := uriName(u)
		raw := map[string]any{"name": name, "type": "ss", "server": u.Hostname(), "port": port}
		copyQuery(raw, u)
		if u.User != nil {
			cipher := u.User.Username()
			if password, ok := u.User.Password(); ok {
				raw["cipher"] = cipher
				raw["password"] = password
			} else if decoded, ok := decodeBase64Loose(cipher); ok {
				parts := strings.SplitN(decoded, ":", 2)
				if len(parts) == 2 {
					raw["cipher"] = parts[0]
					raw["password"] = parts[1]
				}
			}
		}
		return model.Node{Name: name, Type: "ss", Server: u.Hostname(), Port: port, Raw: raw, Source: source}, nil
	}

	body := strings.TrimPrefix(line, "ss://")
	name := ""
	if before, after, ok := strings.Cut(body, "#"); ok {
		body = before
		if frag, err := url.QueryUnescape(after); err == nil {
			name = frag
		}
	}
	if before, _, ok := strings.Cut(body, "?"); ok {
		body = before
	}
	decoded, ok := decodeBase64Loose(body)
	if !ok {
		return model.Node{}, fmt.Errorf("invalid ss uri")
	}
	userinfo, hostPort, ok := strings.Cut(decoded, "@")
	if !ok {
		return model.Node{}, fmt.Errorf("invalid ss uri")
	}
	cipher, password, ok := strings.Cut(userinfo, ":")
	if !ok {
		return model.Node{}, fmt.Errorf("invalid ss userinfo")
	}
	host, port, err := splitHostPort(hostPort)
	if err != nil {
		return model.Node{}, err
	}
	if name == "" {
		name = host
	}
	raw := map[string]any{"name": name, "type": "ss", "server": host, "port": port, "cipher": cipher, "password": password}
	return model.Node{Name: name, Type: "ss", Server: host, Port: port, Raw: raw, Source: source}, nil
}

func parseVMessURI(source, line string) (model.Node, error) {
	body := strings.TrimPrefix(line, "vmess://")
	if before, _, ok := strings.Cut(body, "#"); ok {
		body = before
	}
	decoded, ok := decodeBase64Loose(body)
	if !ok {
		return model.Node{}, fmt.Errorf("invalid vmess base64")
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(decoded), &data); err != nil {
		return model.Node{}, fmt.Errorf("invalid vmess json: %w", err)
	}
	server := asString(data["add"])
	port, ok := asInt(data["port"])
	if !ok || server == "" {
		return model.Node{}, fmt.Errorf("invalid vmess server or port")
	}
	name := asString(data["ps"])
	if name == "" {
		name = server
	}
	raw := map[string]any{"name": name, "type": "vmess", "server": server, "port": port}
	setIfNotEmpty(raw, "uuid", asString(data["id"]))
	if aid, ok := asInt(data["aid"]); ok {
		raw["alterId"] = aid
	} else {
		raw["alterId"] = 0
	}
	cipher := asString(data["scy"])
	if cipher == "" {
		cipher = "auto"
	}
	raw["cipher"] = cipher
	setIfNotEmpty(raw, "network", asString(data["net"]))
	setIfNotEmpty(raw, "servername", asString(data["sni"]))
	setIfNotEmpty(raw, "ws-path", asString(data["path"]))
	setIfNotEmpty(raw, "host", asString(data["host"]))
	if tls := asString(data["tls"]); tls != "" && tls != "none" {
		raw["tls"] = true
	}
	return model.Node{Name: name, Type: "vmess", Server: server, Port: port, Raw: raw, Source: source}, nil
}

func copyQuery(raw map[string]any, u *url.URL) {
	for key, vals := range u.Query() {
		if len(vals) == 0 || vals[0] == "" {
			continue
		}
		val := vals[0]
		switch strings.ToLower(key) {
		case "type":
			raw["network"] = val
		case "security":
			raw["security"] = val
			if strings.EqualFold(val, "tls") || strings.EqualFold(val, "reality") {
				raw["tls"] = true
			}
		case "sni", "servername":
			raw["sni"] = val
			raw["servername"] = val
		default:
			raw[key] = val
		}
	}
}

func uriName(u *url.URL) string {
	name := ""
	if frag, err := url.QueryUnescape(u.Fragment); err == nil {
		name = frag
	}
	if name == "" {
		name = u.Hostname()
	}
	return name
}

func setSingleUserField(raw map[string]any, u *url.URL, field string) {
	if u.User == nil {
		return
	}
	if value := u.User.Username(); value != "" {
		raw[field] = value
	}
}

func setTupleUserFields(raw map[string]any, u *url.URL, firstField, secondField string) {
	if u.User == nil {
		return
	}
	if first := u.User.Username(); first != "" {
		raw[firstField] = first
	}
	if second, ok := u.User.Password(); ok && second != "" {
		raw[secondField] = second
	}
}

func decodeBase64Loose(s string) (string, bool) {
	s = strings.TrimSpace(s)
	// Try the raw string first: vmess StdEncoding payloads may contain '+',
	// which url.QueryUnescape would corrupt into spaces. Only fall back to
	// unescaping if the raw attempt fails.
	if out, ok := tryDecodeBase64(s); ok {
		return out, true
	}
	if unescaped, err := url.QueryUnescape(s); err == nil && unescaped != s {
		if out, ok := tryDecodeBase64(strings.TrimSpace(unescaped)); ok {
			return out, true
		}
	}
	return "", false
}

func tryDecodeBase64(s string) (string, bool) {
	candidates := []string{s}
	if m := len(s) % 4; m != 0 {
		candidates = append(candidates, s+strings.Repeat("=", 4-m))
	}
	encodings := []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding}
	for _, candidate := range candidates {
		for _, enc := range encodings {
			if decoded, err := enc.DecodeString(candidate); err == nil && looksText(decoded) {
				return string(decoded), true
			}
		}
	}
	return "", false
}

func splitHostPort(hostPort string) (string, int, error) {
	host, portText, err := net.SplitHostPort(hostPort)
	if err != nil {
		idx := strings.LastIndex(hostPort, ":")
		if idx < 0 {
			return "", 0, fmt.Errorf("invalid host port")
		}
		host = hostPort[:idx]
		portText = hostPort[idx+1:]
	}
	host = strings.Trim(host, "[]")
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid port")
	}
	return host, port, nil
}

func setIfNotEmpty(raw map[string]any, key, value string) {
	if value != "" {
		raw[key] = value
	}
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case fmt.Stringer:
		return strings.TrimSpace(x.String())
	case int, int64, float64, bool:
		return strings.TrimSpace(fmt.Sprint(x))
	default:
		return ""
	}
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, x > 0 && x <= 65535
	case int64:
		return int(x), x > 0 && x <= 65535
	case float64:
		i := int(x)
		return i, x == float64(i) && i > 0 && i <= 65535
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(x))
		return i, err == nil && i > 0 && i <= 65535
	default:
		return 0, false
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
