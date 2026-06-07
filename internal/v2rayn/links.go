package v2rayn

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"clash-node-pipeline/internal/model"
)

// vmessLink builds vmess://<base64-json> in the common v2rayN format.
func vmessLink(n model.Node) (string, bool) {
	v := map[string]any{
		"v":    "2",
		"ps":   n.Name,
		"add":  n.Server,
		"port": strconv.Itoa(n.Port),
		"id":   rawStr(n, "uuid"),
		"aid":  rawStr(n, "alterId"),
		"scy":  firstNonEmpty(rawStr(n, "cipher"), "auto"),
		"net":  firstNonEmpty(rawStr(n, "network"), "tcp"),
		"type": "none",
		"host": firstNonEmpty(rawStr(n, "host"), rawStr(n, "ws-host")),
		"path": firstNonEmpty(rawStr(n, "ws-path"), rawStr(n, "path")),
		"tls":  tlsField(n),
		"sni":  firstNonEmpty(rawStr(n, "servername"), rawStr(n, "sni")),
	}
	if v["id"] == "" {
		return "", false
	}
	if v["aid"] == "" {
		v["aid"] = "0"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", false
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(b), true
}

func vlessLink(n model.Node) (string, bool) {
	uuid := rawStr(n, "uuid")
	if uuid == "" {
		return "", false
	}
	q := url.Values{}
	setIf(q, "type", firstNonEmpty(rawStr(n, "network"), "tcp"))
	security := "none"
	if tlsField(n) == "tls" {
		security = "tls"
	}
	if s := rawStr(n, "security"); s != "" {
		security = s
	}
	q.Set("security", security)
	setIf(q, "sni", firstNonEmpty(rawStr(n, "servername"), rawStr(n, "sni")))
	setIf(q, "flow", rawStr(n, "flow"))
	setIf(q, "fp", rawStr(n, "client-fingerprint"))
	setIf(q, "path", firstNonEmpty(rawStr(n, "ws-path"), rawStr(n, "path")))
	setIf(q, "host", firstNonEmpty(rawStr(n, "host"), rawStr(n, "ws-host")))
	u := fmt.Sprintf("vless://%s@%s?%s%s", uuid, hostPort(n), q.Encode(), frag(n.Name))
	return u, true
}

func trojanLink(n model.Node) (string, bool) {
	password := rawStr(n, "password")
	if password == "" {
		return "", false
	}
	q := url.Values{}
	setIf(q, "sni", firstNonEmpty(rawStr(n, "sni"), rawStr(n, "servername")))
	setIf(q, "type", rawStr(n, "network"))
	setIf(q, "path", firstNonEmpty(rawStr(n, "ws-path"), rawStr(n, "path")))
	setIf(q, "host", firstNonEmpty(rawStr(n, "host"), rawStr(n, "ws-host")))
	enc := url.QueryEscape(password)
	u := fmt.Sprintf("trojan://%s@%s", enc, hostPort(n))
	if q.Encode() != "" {
		u += "?" + q.Encode()
	}
	return u + frag(n.Name), true
}

// ssLink builds ss://base64(method:password)@host:port#name.
func ssLink(n model.Node) (string, bool) {
	method := rawStr(n, "cipher")
	password := rawStr(n, "password")
	if method == "" || password == "" {
		return "", false
	}
	userinfo := base64.RawURLEncoding.EncodeToString([]byte(method + ":" + password))
	u := fmt.Sprintf("ss://%s@%s%s", userinfo, hostPort(n), frag(n.Name))
	return u, true
}

func userPassLink(scheme string, n model.Node) (string, bool) {
	user := rawStr(n, "username")
	pass := rawStr(n, "password")
	var auth string
	switch {
	case user != "" && pass != "":
		auth = url.QueryEscape(user) + ":" + url.QueryEscape(pass) + "@"
	case user != "":
		auth = url.QueryEscape(user) + "@"
	}
	u := fmt.Sprintf("%s://%s%s%s", scheme, auth, hostPort(n), frag(n.Name))
	return u, true
}

func tlsField(n model.Node) string {
	if rawBool(n, "tls") {
		return "tls"
	}
	if s := strings.ToLower(rawStr(n, "security")); s == "tls" || s == "reality" {
		return "tls"
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func setIf(q url.Values, key, val string) {
	if val != "" {
		q.Set(key, val)
	}
}
