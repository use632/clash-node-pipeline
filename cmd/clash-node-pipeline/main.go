package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/use632/clash-node-pipeline/internal/config"
	"github.com/use632/clash-node-pipeline/internal/model"
	"github.com/use632/clash-node-pipeline/internal/pipeline"
	"github.com/use632/clash-node-pipeline/internal/subserve"
	"github.com/use632/clash-node-pipeline/internal/webui"
)

func main() {
	if len(os.Args) < 2 {
		// Double-clicked / launched with no arguments: behave like a desktop app
		// and open the web GUI directly instead of printing usage and exiting.
		chdirToExe()
		guiDoubleClick()
		return
	}
	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	case "check-config":
		checkConfig(os.Args[2:])
	case "gui":
		gui(os.Args[2:])
	case "serve":
		serve(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

// chdirToExe makes the executable's own directory the working directory so that
// relative output paths (output/...) always land next to the .exe, regardless
// of where Windows launches a double-clicked program from. Best-effort: on any
// error we keep the current directory.
func chdirToExe() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	// Don't relocate when running via `go run` (temp build dir), so developer
	// runs keep using the project working directory.
	if strings.Contains(dir, "go-build") || filepath.Base(filepath.Dir(dir)) == "go-build" {
		return
	}
	_ = os.Chdir(dir)
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "configs/sources.yaml", "config YAML path")
	outputPath := fs.String("output", "", "override output Clash YAML path")
	maxNodes := fs.Int("max-nodes", 0, "override max output nodes")
	stage1Concurrency := fs.Int("stage1-concurrency", 0, "override stage1 TCP concurrency")
	stage2Concurrency := fs.Int("stage2-concurrency", 0, "override stage2 TCP concurrency")
	speedtestCandidateLimit := fs.Int("speedtest-candidate-limit", 0, "override speedtest candidate limit before TCP tests")
	speedtestCandidateMultiplier := fs.Int("speedtest-candidate-multiplier", 0, "override speedtest candidate multiplier based on max nodes")
	testURL := fs.String("test-url", "", "mihomo delay test URL (default http://www.gstatic.com/generate_204)")
	noMihomo := fs.Bool("no-mihomo", false, "disable mihomo core delay test (keep TCP-only results)")
	mihomoCore := fs.String("mihomo-core", "", "path to mihomo core (auto-detect Clash Verge if empty)")
	v2raynOut := fs.String("v2rayn-out", "", "override v2rayN subscription output path")
	proxy := fs.String("proxy", "", "fetch subscriptions via this proxy (http/socks5 URL, host:port, or port); tried before direct")
	skipSpeedtest := fs.Bool("skip-speedtest", false, "skip TCP speedtest and output parsed nodes")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, err := pipeline.Run(ctx, pipeline.Options{
		ConfigPath:                 *configPath,
		OutputPath:                 *outputPath,
		MaxNodes:                   *maxNodes,
		Stage1Concurrency:          *stage1Concurrency,
		Stage2Concurrency:          *stage2Concurrency,
		SpeedtestCandidateLimit:    *speedtestCandidateLimit,
		SpeedtestCandidateMultiple: *speedtestCandidateMultiplier,
		SkipSpeedtest:              *skipSpeedtest,
		TestURL:                    *testURL,
		DisableMihomo:              *noMihomo,
		MihomoCorePath:             *mihomoCore,
		V2rayNPath:                 *v2raynOut,
		Proxy:                      *proxy,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	printReport(report)
}

func checkConfig(args []string) {
	fs := flag.NewFlagSet("check-config", flag.ExitOnError)
	configPath := fs.String("config", "configs/sources.yaml", "config YAML path")
	_ = fs.Parse(args)
	if _, err := config.Load(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, "config invalid:", err)
		os.Exit(1)
	}
	fmt.Println("config ok:", *configPath)
}

func gui(args []string) {
	fs := flag.NewFlagSet("gui", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8787", "local web GUI listen address")
	_ = fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := webui.Start(ctx, webui.Options{Addr: *addr}); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "gui error:", err)
		os.Exit(1)
	}
}

// guiDoubleClick is the no-argument launch path (Windows double-click). Unlike
// gui(), it never lets the window just flash-and-close: on any startup error it
// prints the reason and waits for Enter, so the user can actually read it.
func guiDoubleClick() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := webui.Start(ctx, webui.Options{Addr: "127.0.0.1:8787"}); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "\n启动失败:", err)
		pauseForUser()
		os.Exit(1)
	}
}

// pauseForUser keeps the console window open until the user presses Enter, so a
// double-clicked program never disappears before its message can be read.
func pauseForUser() {
	fmt.Print("\n按回车键关闭本窗口...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "configs/sources.yaml", "config YAML path (for output file locations)")
	addr := fs.String("addr", "127.0.0.1:8899", "local subscription server listen address")
	clashFile := fs.String("clash-file", "", "override clash subscription file path")
	v2raynFile := fs.String("v2rayn-file", "", "override v2rayN subscription file path")
	open := fs.Bool("open", true, "open the index page in the browser")
	_ = fs.Parse(args)

	cf, vf := *clashFile, *v2raynFile
	if cf == "" || vf == "" {
		if cfg, err := config.Load(*configPath); err == nil {
			if cf == "" {
				cf = cfg.Output.File
			}
			if vf == "" {
				vf = cfg.Output.V2rayNFile
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := subserve.Start(ctx, subserve.Options{Addr: *addr, ClashFile: cf, V2rayNFile: vf, Open: *open}); err != nil {
		fmt.Fprintln(os.Stderr, "serve error:", err)
		os.Exit(1)
	}
}

func printReport(report model.Report) {
	fmt.Println("========== Clash Node Pipeline Report ==========")
	fmt.Printf("订阅源数量: %d\n", report.SourceCount)
	fmt.Printf("成功拉取: %d\n", report.FetchOK)
	if report.FetchedViaProxy > 0 {
		fmt.Printf("其中经代理拉取: %d\n", report.FetchedViaProxy)
	}
	fmt.Printf("失败拉取: %d\n", report.FetchFailed)
	fmt.Printf("原始解析节点: %d\n", report.RawParsedNodes)
	fmt.Printf("解析/清洗问题: %d\n", report.ParseIssues)
	fmt.Printf("清洗后节点: %d\n", report.NormalizedNodes)
	fmt.Printf("去重后节点: %d\n", report.DedupedNodes)
	fmt.Printf("测速候选节点: %d\n", report.SpeedtestCandidates)
	fmt.Printf("第一阶段 TCP 存活: %d\n", report.Stage1Alive)
	fmt.Printf("第二阶段测速完成: %d\n", report.Stage2Tested)
	if report.MihomoTested > 0 {
		fmt.Printf("mihomo 测速完成: %d\n", report.MihomoTested)
		fmt.Printf("mihomo 测速存活: %d\n", report.MihomoAlive)
	}
	if report.MihomoSkipped != "" {
		fmt.Printf("mihomo 提示: %s\n", report.MihomoSkipped)
	}
	fmt.Printf("最终输出节点: %d\n", report.FinalNodes)
	fmt.Printf("输出文件: %s\n", report.OutputFile)
	if report.V2rayNFile != "" {
		fmt.Printf("v2rayN 订阅: %s (可导入 %d, 跳过 %d 个 v2rayN 不支持的节点)\n", report.V2rayNFile, report.V2rayNLinks, report.V2rayNSkipped)
	}
	fmt.Printf("坏节点日志: %s\n", report.BadNodesFile)
	fmt.Printf("耗时: %dms\n", report.DurationMS)
}

func usage() {
	fmt.Print(`clash-node-pipeline

Usage:
  clash-node-pipeline run [flags]
  clash-node-pipeline gui [flags]
  clash-node-pipeline serve [flags]
  clash-node-pipeline check-config [flags]

Run flags:
  --config configs/sources.yaml
  --output output/clash.yaml
  --max-nodes 1000
  --stage1-concurrency 500
  --stage2-concurrency 200
  --speedtest-candidate-limit 5000
  --speedtest-candidate-multiplier 5
  --test-url http://www.gstatic.com/generate_204
  --no-mihomo
  --mihomo-core "C:\Program Files\Clash Verge\verge-mihomo.exe"
  --v2rayn-out output/v2rayn.txt
  --proxy http://127.0.0.1:7890
  --skip-speedtest

By default: TCP screening first, then mihomo core delay test on survivors.
If no mihomo core is found, it gracefully falls back to TCP results.

GUI flags:
  --addr 127.0.0.1:8787

Serve flags (local subscription server):
  --addr 127.0.0.1:8899
  --config configs/sources.yaml
  /clash   -> Clash/Mihomo subscription
  /v2rayn  -> v2rayN subscription
`)
}
