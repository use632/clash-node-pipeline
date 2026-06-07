package webui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"clash-node-pipeline/internal/dateadjust"
	"clash-node-pipeline/internal/model"
	"clash-node-pipeline/internal/pipeline"
	"clash-node-pipeline/internal/savedsources"
	"gopkg.in/yaml.v3"
)

type Options struct {
	Addr string
}

// GUI output files. The same paths are written by a run and served as local
// subscription endpoints (/clash, /v2rayn) so clients can subscribe by URL.
const (
	guiClashFile  = "output/gui.clash.yaml"
	guiV2rayNFile = "output/gui.v2rayn.txt"
)

type pageData struct {
	Form   formData
	Report *model.Report
	Error  string
	Ran    bool
	Host   string // request host, used to build local subscription URLs
}

type formData struct {
	SourceURLs              string
	MaxNodes                int
	SpeedtestCandidateLimit int
	Stage1Concurrency       int
	Stage2Concurrency       int
	SkipSpeedtest           bool
	TestURL                 string
	DisableMihomo           bool
	MihomoCore              string
	Proxy                   string
}

func Start(ctx context.Context, opts Options) error {
	addr := opts.Addr
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	// The GUI can trigger fetches, write files, and launch the mihomo core, so it
	// must never be exposed beyond this machine. Refuse non-loopback binds.
	if !isLoopbackAddr(addr) {
		return fmt.Errorf("出于安全考虑，GUI 只能监听本机回环地址（127.0.0.1 / ::1 / localhost），不允许 %q", addr)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	// Local subscription endpoints: clients can subscribe to these URLs and the
	// GUI re-run will refresh them in place. Same port as the GUI. The primary
	// URLs end in .yaml / .txt because that is what Clash/v2rayN subscriptions
	// look like; the extensionless paths are kept as aliases.
	clashHandler := func(w http.ResponseWriter, r *http.Request) {
		serveSubscription(w, r, guiClashFile, "text/yaml; charset=utf-8", "clash.yaml")
	}
	v2raynHandler := func(w http.ResponseWriter, r *http.Request) {
		serveSubscription(w, r, guiV2rayNFile, "text/plain; charset=utf-8", "v2rayn.txt")
	}
	mux.HandleFunc("/clash.yaml", clashHandler)
	mux.HandleFunc("/clash", clashHandler)
	mux.HandleFunc("/v2rayn.txt", v2raynHandler)
	mux.HandleFunc("/v2rayn", v2raynHandler)

	server := &http.Server{Handler: mux}
	listener, err := listenWithFallback(addr)
	if err != nil {
		return err
	}
	url := "http://" + listener.Addr().String()
	fmt.Println("==================================================")
	fmt.Println("  Clash Node Pipeline 已启动")
	fmt.Println("  浏览器地址:", url)
	fmt.Println("  浏览器应已自动打开；若没有，请手动复制上面的地址访问。")
	fmt.Println("  生成的文件在本程序旁边的 output 文件夹里。")
	fmt.Println("  本地订阅地址（跑一次后可用，填进客户端订阅里）:")
	fmt.Println("    Clash/Mihomo:", url+"/clash.yaml")
	fmt.Println("    v2rayN:      ", url+"/v2rayn.txt")
	fmt.Println("  退出：关闭这个黑色窗口，或按 Ctrl+C。")
	fmt.Println("==================================================")
	openBrowser(url)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// listenWithFallback tries the requested address first; if its port is already
// in use (common when a previous instance is still running), it walks a few
// sequential ports and finally falls back to an OS-assigned free port, so a busy
// port never crashes the GUI on launch.
func listenWithFallback(addr string) (net.Listener, error) {
	if ln, err := net.Listen("tcp", addr); err == nil {
		return ln, nil
	}
	host, portStr, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		host = "127.0.0.1"
		portStr = "8787"
	}
	base, _ := strconv.Atoi(portStr)
	if base <= 0 {
		base = 8787
	}
	for p := base + 1; p <= base+20; p++ {
		cand := net.JoinHostPort(host, strconv.Itoa(p))
		if ln, err := net.Listen("tcp", cand); err == nil {
			fmt.Printf("  端口 %d 被占用，已自动改用 %d。\n", base, p)
			return ln, nil
		}
	}
	// Last resort: let the OS pick any free port.
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return nil, err
	}
	fmt.Printf("  端口 %d 被占用，已自动改用空闲端口。\n", base)
	return ln, nil
}

// isLoopbackAddr reports whether a listen address targets only this machine.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// crossSiteRequest reports whether a POST looks like it originated from another
// site (CSRF). It trusts the browser's Sec-Fetch-Site / Origin headers; requests
// without them (e.g. curl) are allowed since they aren't browser-driven.
func crossSiteRequest(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "same-site", "none":
		return false
	case "cross-site":
		return true
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		if u, err := url.Parse(origin); err == nil && u.Host != r.Host {
			return true
		}
	}
	return false
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	data := pageData{Form: defaultForm(), Host: r.Host}
	if r.Method == http.MethodPost {
		// Block cross-site POSTs so a malicious web page can't drive a run
		// against the localhost GUI (which fetches URLs and launches a core).
		if crossSiteRequest(r) {
			http.Error(w, "拒绝跨站请求", http.StatusForbidden)
			return
		}
		if err := r.ParseForm(); err != nil {
			data.Error = "读取表单失败: " + err.Error()
			render(w, data)
			return
		}
		data.Form = parseForm(r)
		data.Ran = true
		report, err := runFromForm(r.Context(), data.Form)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Report = &report
			// Remember the URLs the user just ran so the next launch prefills them.
			_ = savedsources.Save(savedsources.DefaultPath, splitURLs(data.Form.SourceURLs))
		}
	}
	render(w, data)
}

// serveSubscription serves a generated subscription file over HTTP so clients
// can subscribe by URL. Returns a friendly 404 when the file does not exist yet
// (i.e. before the first successful run).
func serveSubscription(w http.ResponseWriter, r *http.Request, path, contentType, downloadName string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "订阅文件还不存在，请先在 GUI 里点一次「开始运行」生成。", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Profile-Update-Interval", "12")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))
	_, _ = w.Write(data)
}

func defaultForm() formData {
	form := formData{
		SourceURLs:              defaultSourceURLs(),
		MaxNodes:                1000,
		SpeedtestCandidateLimit: 5000,
		Stage1Concurrency:       120,
		Stage2Concurrency:       60,
		TestURL:                 "http://www.gstatic.com/generate_204",
	}
	return form
}

// defaultSourceURLs prefills the textarea. If the user previously ran the GUI we
// reload the saved subscription URLs and shift any embedded date to today, so a
// link saved on a past day keeps working. With no saved URLs it stays empty and
// the textarea shows its placeholder.
func defaultSourceURLs() string {
	saved, err := savedsources.Load(savedsources.DefaultPath)
	if err != nil || len(saved) == 0 {
		return ""
	}
	now := time.Now()
	adjusted := make([]string, len(saved))
	for i, u := range saved {
		adjusted[i] = dateadjust.Rewrite(u, now)
	}
	return strings.Join(adjusted, "\n")
}

func parseForm(r *http.Request) formData {
	return formData{
		SourceURLs:              r.FormValue("source_urls"),
		MaxNodes:                intValue(r.FormValue("max_nodes"), 1000),
		SpeedtestCandidateLimit: intValue(r.FormValue("speedtest_candidate_limit"), 5000),
		Stage1Concurrency:       intValue(r.FormValue("stage1_concurrency"), 120),
		Stage2Concurrency:       intValue(r.FormValue("stage2_concurrency"), 60),
		SkipSpeedtest:           r.FormValue("skip_speedtest") == "on",
		TestURL:                 strings.TrimSpace(r.FormValue("test_url")),
		DisableMihomo:           r.FormValue("disable_mihomo") == "on",
		MihomoCore:              strings.TrimSpace(r.FormValue("mihomo_core")),
		Proxy:                   strings.TrimSpace(r.FormValue("proxy")),
	}
}

func intValue(s string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func splitURLs(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t'
	}) {
		u := strings.TrimSpace(line)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func runFromForm(ctx context.Context, form formData) (model.Report, error) {
	urls := splitURLs(form.SourceURLs)
	if len(urls) == 0 {
		return model.Report{}, fmt.Errorf("请至少输入一个订阅 URL")
	}
	if form.MaxNodes <= 0 {
		return model.Report{}, fmt.Errorf("最大输出节点数必须大于 0")
	}
	var sources []model.Source
	for i, u := range urls {
		sources = append(sources, model.Source{Name: fmt.Sprintf("gui-source-%d", i+1), URL: u, Enabled: true})
	}
	testURL := form.TestURL
	if testURL == "" {
		testURL = "http://www.gstatic.com/generate_204"
	}
	cfg := model.AppConfig{
		Sources:     sources,
		DateFormats: []string{"20060102", "2006-01-02"},
		Fetch: model.FetchConfig{
			Concurrency:    8,
			TimeoutSeconds: 30,
			Retries:        1,
			UserAgent:      "clash-node-pipeline/1.0",
			Proxy:          form.Proxy,
		},
		Speedtest: model.SpeedtestConfig{
			Stage1Concurrency:   form.Stage1Concurrency,
			Stage1TimeoutMS:     1500,
			Stage2Enabled:       true,
			Stage2Concurrency:   form.Stage2Concurrency,
			Stage2TimeoutMS:     2200,
			Stage2Attempts:      2,
			CandidateLimit:      form.SpeedtestCandidateLimit,
			CandidateMultiplier: 5,
			TestURL:             testURL,
			MihomoDisabled:      form.DisableMihomo,
			MihomoCorePath:      form.MihomoCore,
			MihomoConcurrency:   32,
			MihomoTimeoutMS:     5000,
		},
		Output: model.OutputConfig{
			File:                    guiClashFile,
			ReportFile:              "output/gui.report.json",
			BadNodesFile:            "output/gui.bad_nodes.log",
			V2rayNFile:              guiV2rayNFile,
			MaxNodes:                form.MaxNodes,
			IncludeDeadNodesIfEmpty: false,
		},
	}
	configPath := filepath.Join("output", "gui.runtime.yaml")
	if err := writeRuntimeConfig(configPath, cfg); err != nil {
		return model.Report{}, err
	}
	return pipeline.Run(ctx, pipeline.Options{
		ConfigPath:                 configPath,
		MaxNodes:                   form.MaxNodes,
		Stage1Concurrency:          form.Stage1Concurrency,
		Stage2Concurrency:          form.Stage2Concurrency,
		SpeedtestCandidateLimit:    form.SpeedtestCandidateLimit,
		SpeedtestCandidateMultiple: 5,
		SkipSpeedtest:              form.SkipSpeedtest,
		TestURL:                    testURL,
		DisableMihomo:              form.DisableMihomo,
		MihomoCorePath:             form.MihomoCore,
		Proxy:                      form.Proxy,
	})
}

func writeRuntimeConfig(path string, cfg model.AppConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func render(w http.ResponseWriter, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pageTemplate.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
