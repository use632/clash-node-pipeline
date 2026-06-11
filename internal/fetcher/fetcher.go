package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/use632/clash-node-pipeline/internal/dateadjust"
	"github.com/use632/clash-node-pipeline/internal/model"
)

// fetchClient is a named HTTP client (e.g. "代理" or "直连"), tried in order.
type fetchClient struct {
	name   string
	client *http.Client
}

func FetchAll(ctx context.Context, cfg model.AppConfig, _ string) []model.FetchResult {
	expanded := expandSources(cfg.Sources, cfg.DateFormats)
	sem := make(chan struct{}, max(1, cfg.Fetch.Concurrency))
	results := make([]model.FetchResult, len(expanded))
	var wg sync.WaitGroup
	clients := buildClients(cfg.Fetch)

	for i, src := range expanded {
		i, src := i, src
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = model.FetchResult{Source: src, URL: src.URL, Err: ctx.Err().Error()}
				return
			}
			results[i] = fetchOne(ctx, clients, src, cfg.Fetch)
		}()
	}
	wg.Wait()
	return results
}

// buildClients returns the HTTP clients to try, in order. When a proxy is
// configured it is tried first (that is why the user set it) with direct as a
// fallback; otherwise only a direct client is used.
func buildClients(cfg model.FetchConfig) []fetchClient {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	direct := fetchClient{name: "直连", client: &http.Client{Timeout: timeout}}
	pu, err := ParseProxy(cfg.Proxy)
	if err != nil || pu == nil {
		return []fetchClient{direct}
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyURL(pu),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	proxied := fetchClient{name: "代理", client: &http.Client{Timeout: timeout, Transport: transport}}
	return []fetchClient{proxied, direct}
}

// ParseProxy normalises a user-entered proxy into a URL. It accepts a full
// http(s)/socks5 URL, a bare "host:port", or just a port number. An empty string
// returns (nil, nil) meaning "no proxy".
func ParseProxy(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if !strings.Contains(raw, "://") {
		if !strings.Contains(raw, ":") {
			raw = "127.0.0.1:" + raw // bare port
		}
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("代理地址无法解析 %q: %w", raw, err)
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("不支持的代理协议 %q（用 http/https/socks5）", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("代理地址缺少主机:端口 %q", raw)
	}
	return u, nil
}

func expandSources(sources []model.Source, dateFormats []string) []model.Source {
	now := time.Now()
	var out []model.Source
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		// Auto-shift any date already embedded in the URL (e.g. .../2026/06/0-20260607.txt)
		// to today. This is independent of the explicit {date} placeholder below.
		src.URL = dateadjust.Rewrite(src.URL, now)
		if !strings.Contains(src.URL, "{date}") {
			out = append(out, src)
			continue
		}
		for _, layout := range dateFormats {
			clone := src
			clone.URL = strings.ReplaceAll(src.URL, "{date}", now.Format(layout))
			clone.Name = fmt.Sprintf("%s[%s]", src.Name, layout)
			out = append(out, clone)
		}
	}
	return out
}

func fetchOne(ctx context.Context, clients []fetchClient, src model.Source, cfg model.FetchConfig) model.FetchResult {
	started := time.Now()
	res := model.FetchResult{Source: src, URL: src.URL}
	// Local files don't go over the network; read once regardless of proxy.
	if !isHTTP(src.URL) {
		var lastErr error
		for attempt := 0; attempt <= cfg.Retries; attempt++ {
			content, err := fetchOnce(ctx, clients[0].client, src.URL, cfg.UserAgent)
			if err == nil {
				res.Content = content
				res.Duration = time.Since(started).Milliseconds()
				return res
			}
			lastErr = err
		}
		res.Err = lastErr.Error()
		res.Duration = time.Since(started).Milliseconds()
		return res
	}
	var lastErr error
	for _, fc := range clients {
		for attempt := 0; attempt <= cfg.Retries; attempt++ {
			if ctx.Err() != nil {
				res.Err = ctx.Err().Error()
				res.Duration = time.Since(started).Milliseconds()
				return res
			}
			content, err := fetchOnce(ctx, fc.client, src.URL, cfg.UserAgent)
			if err == nil {
				res.Content = content
				res.Via = fc.name
				res.Duration = time.Since(started).Milliseconds()
				return res
			}
			lastErr = fmt.Errorf("%s: %w", fc.name, err)
		}
	}
	res.Err = lastErr.Error()
	res.Duration = time.Since(started).Milliseconds()
	return res
}

func isHTTP(rawURL string) bool {
	return strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://")
}

func fetchOnce(ctx context.Context, client *http.Client, rawURL, ua string) ([]byte, error) {
	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, readErr := readLimited(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			// Cloudflare and similar anti-bot gateways answer a non-browser
			// request with a JS "challenge" page (commonly 403/503). No plain
			// HTTP client can pass it, so say so plainly instead of just a code.
			if readErr == nil && looksLikeAntiBotChallenge(body) {
				return nil, fmt.Errorf("http %d 人机验证页（疑似 Cloudflare，自动抓取无法通过 JS 校验；可在浏览器打开后把网页另存为 .html，再用该文件路径作为来源）", resp.StatusCode)
			}
			return nil, fmt.Errorf("http status %d", resp.StatusCode)
		}
		if readErr != nil {
			return nil, readErr
		}
		// Some gateways serve the challenge with a 200 status; treat that as a
		// failure too, otherwise it parses to "0 nodes" with no explanation.
		if looksLikeAntiBotChallenge(body) {
			return nil, fmt.Errorf("返回的是人机验证页（疑似 Cloudflare），不是真实内容；自动抓取无法通过 JS 校验，可在浏览器打开后把网页另存为 .html，再用该文件路径作为来源")
		}
		return body, nil
	}
	// Local file source. Relative paths resolve against the current working
	// directory (where the command is run), which is the least-surprising rule.
	path := strings.TrimPrefix(rawURL, "file://")
	return os.ReadFile(filepath.Clean(path))
}

// looksLikeAntiBotChallenge reports whether an HTML body is an anti-bot
// interstitial (Cloudflare "Just a moment...", JS/cookie challenge, etc.)
// rather than the page we asked for. These require a real browser to solve, so
// a plain HTTP fetch never sees the content behind them — detecting it lets us
// give the user an actionable message instead of a bare status code or an
// empty "0 nodes" result.
func looksLikeAntiBotChallenge(body []byte) bool {
	const sniff = 8192
	head := body
	if len(head) > sniff {
		head = head[:sniff]
	}
	lower := strings.ToLower(string(head))
	markers := []string{
		"_cf_chl_opt",
		"/cdn-cgi/challenge-platform",
		"challenge-platform",
		"cf-browser-verification",
		"just a moment",
		"enable javascript and cookies to continue",
		"checking your browser before accessing",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func readLimited(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	const maxBytes = 64 << 20
	buf := make([]byte, 0, 32*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if len(buf) > maxBytes {
				return nil, fmt.Errorf("response too large, limit %d bytes", maxBytes)
			}
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return nil, err
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
