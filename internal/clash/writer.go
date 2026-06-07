package clash

import (
	"os"
	"path/filepath"

	"clash-node-pipeline/internal/model"
	"gopkg.in/yaml.v3"
)

type Config struct {
	MixedPort   int              `yaml:"mixed-port"`
	AllowLAN    bool             `yaml:"allow-lan"`
	Mode        string           `yaml:"mode"`
	LogLevel    string           `yaml:"log-level"`
	Proxies     []map[string]any `yaml:"proxies"`
	ProxyGroups []ProxyGroup     `yaml:"proxy-groups"`
	Rules       []string         `yaml:"rules"`
}

type ProxyGroup struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`
	URL       string   `yaml:"url,omitempty"`
	Interval  int      `yaml:"interval,omitempty"`
	Tolerance int      `yaml:"tolerance,omitempty"`
	Proxies   []string `yaml:"proxies"`
}

func Build(nodes []model.Node) Config {
	proxyMaps := make([]map[string]any, 0, len(nodes))
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		raw := make(map[string]any, len(n.Raw))
		for k, v := range n.Raw {
			raw[k] = v
		}
		raw["name"] = n.Name
		raw["type"] = n.Type
		raw["server"] = n.Server
		raw["port"] = n.Port
		proxyMaps = append(proxyMaps, raw)
		names = append(names, n.Name)
	}
	selectProxies := append([]string{"♻️ 自动选择", "DIRECT"}, names...)
	autoProxies := append([]string{}, names...)
	if len(autoProxies) == 0 {
		autoProxies = []string{"DIRECT"}
	}
	return Config{
		MixedPort: 7890,
		AllowLAN:  false,
		Mode:      "rule",
		LogLevel:  "info",
		Proxies:   proxyMaps,
		ProxyGroups: []ProxyGroup{
			{Name: "🚀 节点选择", Type: "select", Proxies: selectProxies},
			{Name: "♻️ 自动选择", Type: "url-test", URL: "http://www.gstatic.com/generate_204", Interval: 300, Tolerance: 50, Proxies: autoProxies},
			{Name: "🌍 国外媒体", Type: "select", Proxies: []string{"🚀 节点选择", "♻️ 自动选择", "DIRECT"}},
		},
		Rules: []string{"GEOIP,CN,DIRECT", "MATCH,🚀 节点选择"},
	}
}

func Write(path string, nodes []model.Node) error {
	cfg := Build(nodes)
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
