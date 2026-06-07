package config

import (
	"fmt"
	"os"

	"github.com/use632/clash-node-pipeline/internal/model"
	"gopkg.in/yaml.v3"
)

func Load(path string) (model.AppConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return model.AppConfig{}, fmt.Errorf("read config: %w", err)
	}
	var cfg model.AppConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return model.AppConfig{}, fmt.Errorf("parse config yaml: %w", err)
	}
	applyDefaults(&cfg)
	return cfg, nil
}

func applyDefaults(cfg *model.AppConfig) {
	if len(cfg.DateFormats) == 0 {
		cfg.DateFormats = []string{"20060102", "2006-01-02"}
	}
	if cfg.Fetch.Concurrency <= 0 {
		cfg.Fetch.Concurrency = 16
	}
	if cfg.Fetch.TimeoutSeconds <= 0 {
		cfg.Fetch.TimeoutSeconds = 15
	}
	if cfg.Fetch.UserAgent == "" {
		cfg.Fetch.UserAgent = "clash-node-pipeline/1.0"
	}
	if cfg.Speedtest.Stage1Concurrency <= 0 {
		cfg.Speedtest.Stage1Concurrency = 500
	}
	if cfg.Speedtest.Stage1TimeoutMS <= 0 {
		cfg.Speedtest.Stage1TimeoutMS = 1200
	}
	if cfg.Speedtest.Stage2Concurrency <= 0 {
		cfg.Speedtest.Stage2Concurrency = 200
	}
	if cfg.Speedtest.Stage2TimeoutMS <= 0 {
		cfg.Speedtest.Stage2TimeoutMS = 1800
	}
	if cfg.Speedtest.Stage2Attempts <= 0 {
		cfg.Speedtest.Stage2Attempts = 3
	}
	if cfg.Speedtest.CandidateMultiplier <= 0 {
		cfg.Speedtest.CandidateMultiplier = 5
	}
	if cfg.Speedtest.TestURL == "" {
		cfg.Speedtest.TestURL = "http://www.gstatic.com/generate_204"
	}
	if cfg.Speedtest.MihomoConcurrency <= 0 {
		cfg.Speedtest.MihomoConcurrency = 32
	}
	if cfg.Speedtest.MihomoTimeoutMS <= 0 {
		cfg.Speedtest.MihomoTimeoutMS = 5000
	}
	if cfg.Output.File == "" {
		cfg.Output.File = "output/clash.yaml"
	}
	if cfg.Output.ReportFile == "" {
		cfg.Output.ReportFile = "output/report.json"
	}
	if cfg.Output.BadNodesFile == "" {
		cfg.Output.BadNodesFile = "output/bad_nodes.log"
	}
	if cfg.Output.V2rayNFile == "" {
		cfg.Output.V2rayNFile = "output/v2rayn.txt"
	}
	if cfg.Output.MaxNodes <= 0 {
		cfg.Output.MaxNodes = 1000
	}
}
