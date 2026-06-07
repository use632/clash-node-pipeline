package speedtest

import (
	"context"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"clash-node-pipeline/internal/model"
)

func Stage1(ctx context.Context, nodes []model.Node, concurrency, timeoutMS int) []model.TestResult {
	return testMany(ctx, nodes, concurrency, timeoutMS, 1, 1)
}

func Stage2(ctx context.Context, nodes []model.Node, concurrency, timeoutMS, attempts int) []model.TestResult {
	if attempts <= 0 {
		attempts = 3
	}
	return testMany(ctx, nodes, concurrency, timeoutMS, attempts, 2)
}

func AliveNodes(results []model.TestResult) []model.Node {
	var out []model.Node
	for _, r := range results {
		if r.Alive {
			out = append(out, r.Node)
		}
	}
	return out
}

func SortNodesByLatency(nodes []model.Node, results []model.TestResult) []model.Node {
	lat := map[string]int64{}
	for _, r := range results {
		if r.Alive {
			lat[key(r.Node)] = r.LatencyMS
		}
	}
	out := append([]model.Node(nil), nodes...)
	sort.SliceStable(out, func(i, j int) bool {
		li, iok := lat[key(out[i])]
		lj, jok := lat[key(out[j])]
		if iok != jok {
			return iok
		}
		if li == lj {
			return out[i].Name < out[j].Name
		}
		return li < lj
	})
	return out
}

func testMany(ctx context.Context, nodes []model.Node, concurrency, timeoutMS, attempts, stage int) []model.TestResult {
	if concurrency <= 0 {
		concurrency = 200
	}
	if timeoutMS <= 0 {
		timeoutMS = 1200
	}
	sem := make(chan struct{}, concurrency)
	results := make([]model.TestResult, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		i, n := i, n
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = model.TestResult{Node: n, Alive: false, Error: ctx.Err().Error(), Stage: stage}
				return
			}
			results[i] = testOne(ctx, n, time.Duration(timeoutMS)*time.Millisecond, attempts, stage)
		}()
	}
	wg.Wait()
	return results
}

func testOne(ctx context.Context, n model.Node, timeout time.Duration, attempts, stage int) model.TestResult {
	addr := fmt.Sprintf("%s:%d", n.Server, n.Port)
	var best int64
	var lastErr string
	for i := 0; i < attempts; i++ {
		start := time.Now()
		dialer := net.Dialer{Timeout: timeout}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		elapsed := time.Since(start).Milliseconds()
		if err != nil {
			lastErr = err.Error()
			continue
		}
		_ = conn.Close()
		if best == 0 || elapsed < best {
			best = elapsed
		}
	}
	if best > 0 {
		return model.TestResult{Node: n, Alive: true, LatencyMS: best, Stage: stage}
	}
	return model.TestResult{Node: n, Alive: false, Error: lastErr, Stage: stage}
}

func key(n model.Node) string {
	return fmt.Sprintf("%s|%s|%d|%s", n.Type, n.Server, n.Port, n.Name)
}
