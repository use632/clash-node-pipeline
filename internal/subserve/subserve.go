// Package subserve exposes the generated subscription files over a local HTTP
// server so Clash/Mihomo and v2rayN can subscribe via a URL instead of a file.
package subserve

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// Options configures the local subscription server.
type Options struct {
	Addr       string // listen address, default 127.0.0.1:8899
	ClashFile  string // path to clash.yaml
	V2rayNFile string // path to v2rayn base64 file
	Open       bool   // open the index page in the browser
}

// Start runs the server until ctx is cancelled.
func Start(ctx context.Context, opts Options) error {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:8899"
	}
	if opts.ClashFile == "" {
		opts.ClashFile = "output/clash.yaml"
	}
	if opts.V2rayNFile == "" {
		opts.V2rayNFile = "output/v2rayn.txt"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/clash", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, opts.ClashFile, "text/yaml; charset=utf-8", "clash.yaml")
	})
	mux.HandleFunc("/v2rayn", func(w http.ResponseWriter, r *http.Request) {
		serveFile(w, r, opts.V2rayNFile, "text/plain; charset=utf-8", "v2rayn.txt")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		indexPage(w, r.Host)
	})

	listener, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return err
	}
	base := "http://" + listener.Addr().String()
	fmt.Println("本地订阅服务已启动:")
	fmt.Println("  首页:        ", base+"/")
	fmt.Println("  Clash 订阅:  ", base+"/clash")
	fmt.Println("  v2rayN 订阅: ", base+"/v2rayn")
	fmt.Println("把对应地址填进 Clash/Mihomo 或 v2rayN 的订阅设置即可。按 Ctrl+C 停止。")
	if opts.Open {
		openBrowser(base + "/")
	}

	server := &http.Server{Handler: mux}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, path, contentType, downloadName string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("订阅文件不存在: %s（请先运行一次生成）", path), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Profile-Update-Interval", "12")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))
	_, _ = w.Write(data)
}

func indexPage(w http.ResponseWriter, host string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>本地订阅服务</title>
<style>body{font-family:system-ui,sans-serif;background:#0f172a;color:#e2e8f0;margin:0}
.wrap{max-width:760px;margin:0 auto;padding:36px 18px}
.card{background:#111827;border:1px solid #334155;border-radius:16px;padding:20px;margin-bottom:16px}
code{background:#020617;border:1px solid #334155;border-radius:8px;padding:4px 8px;display:inline-block}
a{color:#67e8f9}</style></head><body><div class="wrap">
<h1>本地订阅服务</h1>
<div class="card"><h2>Clash / Mihomo</h2><p>订阅地址：</p><code>http://%s/clash</code></div>
<div class="card"><h2>v2rayN</h2><p>订阅地址：</p><code>http://%s/v2rayn</code></div>
<p style="color:#94a3b8">把上面的地址填进客户端的「订阅」里更新即可。本服务只监听本机，仅供本地客户端使用。</p>
</div></body></html>`, host, host)
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
