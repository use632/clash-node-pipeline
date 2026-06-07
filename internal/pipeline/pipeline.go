package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"time"

	"clash-node-pipeline/internal/clash"
	"clash-node-pipeline/internal/config"
	"clash-node-pipeline/internal/dedupe"
	"clash-node-pipeline/internal/fetcher"
	"clash-node-pipeline/internal/mihomo"
	"clash-node-pipeline/internal/model"
	"clash-node-pipeline/internal/normalizer"
	"clash-node-pipeline/internal/parser"
	"clash-node-pipeline/internal/speedtest"
	"clash-node-pipeline/internal/v2rayn"
)

type Options struct {
	ConfigPath                 string
	OutputPath                 string
	MaxNodes                   int
	Stage1Concurrency          int
	Stage2Concurrency          int
	SpeedtestCandidateLimit    int
	SpeedtestCandidateMultiple int
	SkipSpeedtest              bool
	TestURL                    string
	DisableMihomo              bool
	MihomoCorePath             string
	V2rayNPath                 string
	Proxy                      string
}

func Run(ctx context.Context, opts Options) (model.Report, error) {
	started := time.Now()
	report := model.Report{StartedAt: started}
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return report, err
	}
	applyOptions(&cfg, opts)

	// Validate the fetch proxy up front so a typo gives a clear message instead
	// of every source silently failing.
	if _, err := fetcher.ParseProxy(cfg.Fetch.Proxy); err != nil {
		return report, err
	}

	report.SourceCount = countEnabled(cfg.Sources)
	fetchResults := fetcher.FetchAll(ctx, cfg, opts.ConfigPath)
	report.FetchResults = fetchResults
	var parsed []model.Node
	var issues []model.ParseIssue
	for _, fr := range fetchResults {
		if fr.Err != "" {
			report.FetchFailed++
			issues = append(issues, model.ParseIssue{Source: fr.Source.Name, Err: "fetch failed: " + fr.Err})
			continue
		}
		report.FetchOK++
		if fr.Via == "代理" {
			report.FetchedViaProxy++
		}
		nodes, parseIssues := parser.ParseContent(fr.Source.Name, fr.Content)
		parsed = append(parsed, nodes...)
		issues = append(issues, parseIssues...)
	}
	report.RawParsedNodes = len(parsed)

	normalized, normalizeIssues := normalizer.Normalize(parsed)
	issues = append(issues, normalizeIssues...)
	report.NormalizedNodes = len(normalized)
	report.ParseIssues = len(issues)
	report.BadNodes = issues

	deduped := dedupe.Dedupe(normalized)
	report.DedupedNodes = len(deduped)

	finalNodes := deduped
	report.SpeedtestCandidates = len(deduped)
	if !opts.SkipSpeedtest {
		candidates := limitSpeedtestCandidates(deduped, cfg)
		report.SpeedtestCandidates = len(candidates)
		stage1 := speedtest.Stage1(ctx, candidates, cfg.Speedtest.Stage1Concurrency, cfg.Speedtest.Stage1TimeoutMS)
		alive := speedtest.AliveNodes(stage1)
		report.Stage1Alive = len(alive)
		if cfg.Speedtest.Stage2Enabled && len(alive) > 0 {
			stage2 := speedtest.Stage2(ctx, alive, cfg.Speedtest.Stage2Concurrency, cfg.Speedtest.Stage2TimeoutMS, cfg.Speedtest.Stage2Attempts)
			report.Stage2Tested = len(stage2)
			alive2 := speedtest.AliveNodes(stage2)
			finalNodes = speedtest.SortNodesByLatency(alive2, stage2)
		} else {
			finalNodes = speedtest.SortNodesByLatency(alive, stage1)
		}
		if shouldRunMihomo(cfg) && len(finalNodes) > 0 {
			core, ok := mihomo.Available(cfg.Speedtest.MihomoCorePath)
			if !ok {
				report.MihomoSkipped = "未找到 mihomo 内核，已退回 TCP 测速结果"
			} else {
				mihomoResults, err := mihomo.Run(ctx, finalNodes, mihomo.Config{
					CorePath:    core,
					TestURL:     cfg.Speedtest.TestURL,
					Concurrency: cfg.Speedtest.MihomoConcurrency,
					TimeoutMS:   cfg.Speedtest.MihomoTimeoutMS,
				})
				if err != nil {
					report.MihomoSkipped = "mihomo 测速失败，已退回 TCP 测速结果: " + err.Error()
				} else {
					report.MihomoTested = len(mihomoResults)
					mihomoAlive := mihomo.AliveNodes(mihomoResults)
					report.MihomoAlive = len(mihomoAlive)
					// Only adopt mihomo's ranking if it actually confirmed nodes.
					// If it confirmed zero (e.g. the test URL was transiently
					// unreachable), keep the TCP-ranked survivors instead of
					// emptying the whole output.
					if len(mihomoAlive) > 0 {
						finalNodes = mihomo.SortNodesByDelay(mihomoResults)
					} else {
						report.MihomoSkipped = "mihomo 测速 0 个连通（可能测速地址被墙），已退回 TCP 测速结果"
					}
				}
			}
		}
		if len(finalNodes) == 0 && cfg.Output.IncludeDeadNodesIfEmpty {
			finalNodes = candidates
		}
	}
	if cfg.Output.MaxNodes > 0 && len(finalNodes) > cfg.Output.MaxNodes {
		finalNodes = finalNodes[:cfg.Output.MaxNodes]
	}
	report.FinalNodes = len(finalNodes)
	report.OutputFile = cfg.Output.File
	report.BadNodesFile = cfg.Output.BadNodesFile

	if err := clash.Write(cfg.Output.File, finalNodes); err != nil {
		return report, fmt.Errorf("write clash config: %w", err)
	}
	if cfg.Output.V2rayNFile != "" {
		links, skipped, err := v2rayn.Write(cfg.Output.V2rayNFile, finalNodes)
		if err != nil {
			return report, fmt.Errorf("write v2rayn subscription: %w", err)
		}
		report.V2rayNFile = cfg.Output.V2rayNFile
		report.V2rayNLinks = links
		report.V2rayNSkipped = skipped
	}
	if err := writeBadNodes(cfg.Output.BadNodesFile, issues); err != nil {
		return report, fmt.Errorf("write bad nodes: %w", err)
	}
	report.FinishedAt = time.Now()
	report.DurationMS = time.Since(started).Milliseconds()
	if err := writeReport(cfg.Output.ReportFile, report); err != nil {
		return report, fmt.Errorf("write report: %w", err)
	}
	return report, nil
}

func applyOptions(cfg *model.AppConfig, opts Options) {
	if opts.OutputPath != "" {
		cfg.Output.File = opts.OutputPath
	}
	if opts.MaxNodes > 0 {
		cfg.Output.MaxNodes = opts.MaxNodes
	}
	if opts.Stage1Concurrency > 0 {
		cfg.Speedtest.Stage1Concurrency = opts.Stage1Concurrency
	}
	if opts.Stage2Concurrency > 0 {
		cfg.Speedtest.Stage2Concurrency = opts.Stage2Concurrency
	}
	if opts.SpeedtestCandidateLimit > 0 {
		cfg.Speedtest.CandidateLimit = opts.SpeedtestCandidateLimit
	}
	if opts.SpeedtestCandidateMultiple > 0 {
		cfg.Speedtest.CandidateMultiplier = opts.SpeedtestCandidateMultiple
	}
	if opts.TestURL != "" {
		cfg.Speedtest.TestURL = opts.TestURL
	}
	if opts.DisableMihomo {
		cfg.Speedtest.MihomoDisabled = true
	}
	if opts.MihomoCorePath != "" {
		cfg.Speedtest.MihomoCorePath = opts.MihomoCorePath
	}
	if opts.V2rayNPath != "" {
		cfg.Output.V2rayNFile = opts.V2rayNPath
	}
	if opts.Proxy != "" {
		cfg.Fetch.Proxy = opts.Proxy
	}
}

func limitSpeedtestCandidates(nodes []model.Node, cfg model.AppConfig) []model.Node {
	limit := speedtestCandidateLimit(cfg)
	if limit <= 0 || len(nodes) <= limit {
		return nodes
	}
	selected := append([]model.Node(nil), nodes...)
	sort.SliceStable(selected, func(i, j int) bool {
		hi := stableNodeScore(selected[i])
		hj := stableNodeScore(selected[j])
		if hi == hj {
			return selected[i].Name < selected[j].Name
		}
		return hi < hj
	})
	return selected[:limit]
}

func speedtestCandidateLimit(cfg model.AppConfig) int {
	if cfg.Speedtest.CandidateLimit > 0 {
		return cfg.Speedtest.CandidateLimit
	}
	if cfg.Output.MaxNodes > 0 && cfg.Speedtest.CandidateMultiplier > 0 {
		return cfg.Output.MaxNodes * cfg.Speedtest.CandidateMultiplier
	}
	return 0
}

func stableNodeScore(n model.Node) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(n.Type))
	_, _ = h.Write([]byte{'|'})
	_, _ = h.Write([]byte(n.Server))
	_, _ = h.Write([]byte{'|'})
	_, _ = h.Write([]byte(fmt.Sprint(n.Port)))
	_, _ = h.Write([]byte{'|'})
	_, _ = h.Write([]byte(n.Name))
	return h.Sum64()
}

func countEnabled(sources []model.Source) int {
	count := 0
	for _, s := range sources {
		if s.Enabled {
			count++
		}
	}
	return count
}

// shouldRunMihomo decides whether to attempt mihomo delay testing. It is on by
// default and turned off only when explicitly disabled.
func shouldRunMihomo(cfg model.AppConfig) bool {
	return !cfg.Speedtest.MihomoDisabled
}

func writeReport(path string, report model.Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func writeBadNodes(path string, issues []model.ParseIssue) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, issue := range issues {
		line := fmt.Sprintf("[%s] %s", issue.Source, issue.Err)
		if issue.Line != "" {
			line += " | " + issue.Line
		}
		_, _ = fmt.Fprintln(f, line)
	}
	return nil
}
