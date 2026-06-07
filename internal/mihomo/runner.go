package mihomo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/use632/clash-node-pipeline/internal/model"
)

// Config controls a mihomo-backed delay test run.
type Config struct {
	CorePath    string
	TestURL     string
	Concurrency int
	TimeoutMS   int
}

// Result is the outcome for one node.
type Result struct {
	Node    model.Node
	Alive   bool
	DelayMS int64
	Err     string
}

// Available reports whether a usable core can be found.
func Available(corePath string) (string, bool) {
	return FindCore(corePath)
}

// Run starts a mihomo core, tests all nodes via the API, then shuts it down.
//
// mihomo's config parser is all-or-nothing: a single invalid proxy (e.g. an
// unknown cipher) makes the whole config fatal and the core exits before its API
// is up. Aggregated free nodes almost always contain a few such entries, so we
// start the core, and if it dies during parsing we read which proxy index it
// rejected, drop that node, and retry — until the core comes up or we run out of
// attempts. The returned results cover only the nodes that survived; dropped
// nodes are simply excluded (treated as unusable).
func Run(ctx context.Context, nodes []model.Node, cfg Config) ([]Result, error) {
	if len(nodes) == 0 {
		return nil, nil
	}
	core, ok := FindCore(cfg.CorePath)
	if !ok {
		return nil, fmt.Errorf("未找到 mihomo 内核，请用 --mihomo-core 指定 verge-mihomo.exe 路径")
	}
	if cfg.TestURL == "" {
		cfg.TestURL = "http://www.gstatic.com/generate_204"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 32
	}
	if cfg.TimeoutMS <= 0 {
		cfg.TimeoutMS = 5000
	}

	workDir, err := os.MkdirTemp("", "cnp-mihomo-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)

	cfgPath := filepath.Join(workDir, "config.yaml")
	secret := randomHex(16)
	// The control request must outlast mihomo's own per-proxy delay timeout
	// (it includes mihomo dialing the proxy), so give it generous headroom.
	client := &http.Client{Timeout: time.Duration(cfg.TimeoutMS*2+3000) * time.Millisecond}

	// Working set; entries mihomo rejects get dropped and we retry.
	working := append([]model.Node(nil), nodes...)
	maxAttempts := len(working) + 1
	if maxAttempts > 60 {
		maxAttempts = 60
	}
	bindRetries := 0

	var (
		cmd     *exec.Cmd
		waitCh  chan error
		names   []string
		base    string
		dropped int
		started bool
		lastLog string
	)
	for attempt := 0; attempt < maxAttempts && len(working) > 0; attempt++ {
		apiPort, err := freePort()
		if err != nil {
			return nil, err
		}
		mixedPort, err := freePort()
		if err != nil {
			return nil, err
		}
		controller := fmt.Sprintf("127.0.0.1:%d", apiPort)
		base = "http://" + controller

		var logBuf *boundedBuffer
		cmd, waitCh, names, logBuf, err = startCore(ctx, core, workDir, cfgPath, controller, secret, mixedPort, working)
		if err == nil {
			err = waitReadyOrExit(ctx, cmd, waitCh, client, base, secret, 20*time.Second)
		}
		if err == nil {
			started = true
			break
		}
		lastLog = logBuf.String()
		// Process is already dead at this point (startCore/waitReadyOrExit ensure it).
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if idx, ok := parseBadProxyIndex(lastLog); ok && idx >= 0 && idx < len(working) {
			working = append(working[:idx], working[idx+1:]...)
			dropped++
			continue
		}
		// A port we picked got grabbed between freePort() and mihomo binding it
		// is a transient race: retry (fresh ports) without dropping a node.
		if isBindError(lastLog) && bindRetries < 5 {
			bindRetries++
			continue
		}
		// Not a recoverable per-proxy parse error: surface the real reason.
		return nil, fmt.Errorf("mihomo 启动失败: %s", fatalReason(lastLog))
	}
	if !started {
		return nil, fmt.Errorf("mihomo 反复启动失败，疑似无效节点过多（已剔除 %d 个）: %s", dropped, fatalReason(lastLog))
	}
	defer func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			<-waitCh
		}
	}()

	results := make([]Result, len(working))
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup
	for i := range working {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = Result{Node: working[i], Err: ctx.Err().Error()}
				return
			}
			delay, err := testDelay(ctx, client, base, secret, names[i], cfg.TestURL, cfg.TimeoutMS)
			if err != nil {
				results[i] = Result{Node: working[i], Err: err.Error()}
				return
			}
			results[i] = Result{Node: working[i], Alive: true, DelayMS: delay}
		}()
	}
	wg.Wait()
	return results, nil
}

// startCore writes the config for the given nodes and starts the core. On
// success the process is running and the caller must later Kill it and drain
// waitCh. logBuf captures stdout+stderr for diagnostics.
func startCore(ctx context.Context, core, workDir, cfgPath, controller, secret string, mixedPort int, nodes []model.Node) (*exec.Cmd, chan error, []string, *boundedBuffer, error) {
	names, err := writeConfig(cfgPath, nodes, controller, secret, mixedPort)
	if err != nil {
		return nil, nil, nil, &boundedBuffer{}, err
	}
	logBuf := &boundedBuffer{max: 16 << 10}
	cmd := exec.CommandContext(ctx, core, "-d", workDir, "-f", cfgPath)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf
	if err := cmd.Start(); err != nil {
		return nil, nil, names, logBuf, fmt.Errorf("启动 mihomo 失败: %w", err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	return cmd, waitCh, names, logBuf, nil
}

// waitReadyOrExit blocks until the core's API answers (ready), the process exits
// (config rejected), ctx is cancelled, or max elapses. On any non-nil error the
// process has been killed and waitCh drained.
func waitReadyOrExit(ctx context.Context, cmd *exec.Cmd, waitCh chan error, client *http.Client, base, secret string, max time.Duration) error {
	deadline := time.Now().Add(max)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		if pingReady(ctx, client, base, secret) {
			return nil // ready; process still running
		}
		select {
		case <-ctx.Done():
			killAndDrain(cmd, waitCh)
			return ctx.Err()
		case <-waitCh:
			return fmt.Errorf("mihomo 进程在就绪前退出")
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			killAndDrain(cmd, waitCh)
			return fmt.Errorf("timeout")
		}
	}
}

func killAndDrain(cmd *exec.Cmd, waitCh chan error) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-waitCh
}

func pingReady(ctx context.Context, client *http.Client, base, secret string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/version", nil)
	if err != nil {
		return false
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// badProxyRe is anchored to mihomo's parse-error phrasing so a stray "proxy N:"
// elsewhere in the log (e.g. inside a proxy name) can't be mistaken for it.
var badProxyRe = regexp.MustCompile(`config error: proxy (\d+):`)

// parseBadProxyIndex extracts the 0-based proxy index from a mihomo parse error
// like: `Parse config error: proxy 1: ss ... unknown method`.
func parseBadProxyIndex(log string) (int, bool) {
	m := badProxyRe.FindStringSubmatch(log)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// isBindError reports whether the log indicates a port bind failure (a transient
// race), as opposed to a config parse error.
func isBindError(log string) bool {
	return strings.Contains(log, "address already in use") ||
		strings.Contains(log, "bind:") ||
		strings.Contains(log, "Only one usage of each socket address")
}

// fatalReason pulls a concise reason out of mihomo's log for error messages.
func fatalReason(log string) string {
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "level=fatal") || strings.Contains(line, "level=error") {
			if i := strings.Index(line, "msg="); i >= 0 {
				return strings.Trim(strings.TrimSpace(line[i+4:]), `"`)
			}
			return strings.TrimSpace(line)
		}
	}
	lines := strings.Split(strings.TrimSpace(log), "\n")
	if n := len(lines); n > 0 && lines[n-1] != "" {
		return lines[n-1]
	}
	return "未知错误（无日志输出）"
}

func testDelay(ctx context.Context, client *http.Client, base, secret, name, testURL string, timeoutMS int) (int64, error) {
	endpoint := fmt.Sprintf("%s/proxies/%s/delay?url=%s&timeout=%d",
		base, url.PathEscape(name), url.QueryEscape(testURL), timeoutMS)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("delay status %d", resp.StatusCode)
	}
	var body struct {
		Delay int64 `json:"delay"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, err
	}
	if body.Delay <= 0 {
		return 0, fmt.Errorf("delay zero")
	}
	return body.Delay, nil
}

// boundedBuffer is a thread-safe writer that keeps only the most recent max bytes.
type boundedBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if b.max > 0 && len(b.buf) > b.max {
		b.buf = b.buf[len(b.buf)-b.max:]
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// AliveNodes / SortNodesByDelay helpers for the pipeline.
func AliveNodes(results []Result) []model.Node {
	var out []model.Node
	for _, r := range results {
		if r.Alive {
			out = append(out, r.Node)
		}
	}
	return out
}

func SortNodesByDelay(results []Result) []model.Node {
	alive := make([]Result, 0, len(results))
	for _, r := range results {
		if r.Alive {
			alive = append(alive, r)
		}
	}
	sort.SliceStable(alive, func(i, j int) bool {
		if alive[i].DelayMS == alive[j].DelayMS {
			return alive[i].Node.Name < alive[j].Node.Name
		}
		return alive[i].DelayMS < alive[j].DelayMS
	})
	out := make([]model.Node, 0, len(alive))
	for _, r := range alive {
		out = append(out, r.Node)
	}
	return out
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "cnpsecret"
	}
	return hex.EncodeToString(b)
}
