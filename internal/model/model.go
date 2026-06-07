package model

import "time"

type Source struct {
	Name    string `yaml:"name" json:"name"`
	URL     string `yaml:"url" json:"url"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
}

type FetchConfig struct {
	Concurrency    int    `yaml:"concurrency" json:"concurrency"`
	TimeoutSeconds int    `yaml:"timeout_seconds" json:"timeout_seconds"`
	Retries        int    `yaml:"retries" json:"retries"`
	UserAgent      string `yaml:"user_agent" json:"user_agent"`
	// Proxy used to fetch subscriptions when direct access is blocked. Accepts
	// http(s)/socks5 URLs, "host:port", or a bare port. Empty means direct only.
	// When set, each source is tried via the proxy first, then direct.
	Proxy string `yaml:"proxy" json:"proxy"`
}

type SpeedtestConfig struct {
	Stage1Concurrency   int    `yaml:"stage1_concurrency" json:"stage1_concurrency"`
	Stage1TimeoutMS     int    `yaml:"stage1_timeout_ms" json:"stage1_timeout_ms"`
	Stage2Enabled       bool   `yaml:"stage2_enabled" json:"stage2_enabled"`
	Stage2Concurrency   int    `yaml:"stage2_concurrency" json:"stage2_concurrency"`
	Stage2TimeoutMS     int    `yaml:"stage2_timeout_ms" json:"stage2_timeout_ms"`
	Stage2Attempts      int    `yaml:"stage2_attempts" json:"stage2_attempts"`
	CandidateLimit      int    `yaml:"candidate_limit" json:"candidate_limit"`
	CandidateMultiplier int    `yaml:"candidate_multiplier" json:"candidate_multiplier"`
	TestURL             string `yaml:"test_url" json:"test_url"`
	MihomoDisabled      bool   `yaml:"mihomo_disabled" json:"mihomo_disabled"`
	MihomoCorePath      string `yaml:"mihomo_core_path" json:"mihomo_core_path"`
	MihomoConcurrency   int    `yaml:"mihomo_concurrency" json:"mihomo_concurrency"`
	MihomoTimeoutMS     int    `yaml:"mihomo_timeout_ms" json:"mihomo_timeout_ms"`
}

type OutputConfig struct {
	File                    string `yaml:"file" json:"file"`
	ReportFile              string `yaml:"report_file" json:"report_file"`
	BadNodesFile            string `yaml:"bad_nodes_file" json:"bad_nodes_file"`
	V2rayNFile              string `yaml:"v2rayn_file" json:"v2rayn_file"`
	MaxNodes                int    `yaml:"max_nodes" json:"max_nodes"`
	IncludeDeadNodesIfEmpty bool   `yaml:"include_dead_nodes_if_empty" json:"include_dead_nodes_if_empty"`
}

type AppConfig struct {
	Sources     []Source        `yaml:"sources" json:"sources"`
	DateFormats []string        `yaml:"date_formats" json:"date_formats"`
	Fetch       FetchConfig     `yaml:"fetch" json:"fetch"`
	Speedtest   SpeedtestConfig `yaml:"speedtest" json:"speedtest"`
	Output      OutputConfig    `yaml:"output" json:"output"`
}

type FetchResult struct {
	Source   Source `json:"source"`
	URL      string `json:"url"`
	Content  []byte `json:"-"`
	Err      string `json:"err,omitempty"`
	Via      string `json:"via,omitempty"` // "代理" or "直连"; which path succeeded
	Duration int64  `json:"duration_ms"`
}

type Node struct {
	Name   string         `yaml:"name" json:"name"`
	Type   string         `yaml:"type" json:"type"`
	Server string         `yaml:"server" json:"server"`
	Port   int            `yaml:"port" json:"port"`
	Raw    map[string]any `yaml:",inline" json:"raw"`
	Source string         `yaml:"-" json:"source"`
}

type ParseIssue struct {
	Source string `json:"source"`
	Line   string `json:"line,omitempty"`
	Err    string `json:"err"`
}

type TestResult struct {
	Node      Node   `json:"node"`
	Alive     bool   `json:"alive"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
	Stage     int    `json:"stage"`
}

type Report struct {
	StartedAt           time.Time     `json:"started_at"`
	FinishedAt          time.Time     `json:"finished_at"`
	SourceCount         int           `json:"source_count"`
	FetchOK             int           `json:"fetch_ok"`
	FetchedViaProxy     int           `json:"fetched_via_proxy"`
	FetchFailed         int           `json:"fetch_failed"`
	RawParsedNodes      int           `json:"raw_parsed_nodes"`
	ParseIssues         int           `json:"parse_issues"`
	NormalizedNodes     int           `json:"normalized_nodes"`
	DedupedNodes        int           `json:"deduped_nodes"`
	SpeedtestCandidates int           `json:"speedtest_candidates"`
	Stage1Alive         int           `json:"stage1_alive"`
	Stage2Tested        int           `json:"stage2_tested"`
	MihomoTested        int           `json:"mihomo_tested"`
	MihomoAlive         int           `json:"mihomo_alive"`
	MihomoSkipped       string        `json:"mihomo_skipped,omitempty"`
	FinalNodes          int           `json:"final_nodes"`
	OutputFile          string        `json:"output_file"`
	V2rayNFile          string        `json:"v2rayn_file,omitempty"`
	V2rayNLinks         int           `json:"v2rayn_links"`
	V2rayNSkipped       int           `json:"v2rayn_skipped"`
	BadNodesFile        string        `json:"bad_nodes_file"`
	FetchResults        []FetchResult `json:"fetch_results"`
	BadNodes            []ParseIssue  `json:"bad_nodes"`
	DurationMS          int64         `json:"duration_ms"`
}
