package mihomo

import (
	"fmt"
	"os"

	"clash-node-pipeline/internal/model"
	"gopkg.in/yaml.v3"
)

// runtimeConfig is the minimal mihomo config we generate just for delay tests.
type runtimeConfig struct {
	MixedPort          int              `yaml:"mixed-port"`
	AllowLan           bool             `yaml:"allow-lan"`
	Mode               string           `yaml:"mode"`
	LogLevel           string           `yaml:"log-level"`
	ExternalController string           `yaml:"external-controller"`
	Secret             string           `yaml:"secret"`
	Proxies            []map[string]any `yaml:"proxies"`
}

// writeConfig builds a temp mihomo config containing the given nodes and
// returns the proxy names in the same order. Invalid proxies are skipped.
func writeConfig(path string, nodes []model.Node, controller, secret string, mixedPort int) ([]string, error) {
	proxies := make([]map[string]any, 0, len(nodes))
	names := make([]string, 0, len(nodes))
	used := map[string]bool{}
	for _, n := range nodes {
		name := n.Name
		if name == "" || used[name] {
			// mihomo rejects duplicate/empty proxy names; ensure uniqueness.
			name = fmt.Sprintf("%s_%d_%d", n.Type, n.Port, len(names))
		}
		used[name] = true
		raw := make(map[string]any, len(n.Raw)+4)
		for k, v := range n.Raw {
			raw[k] = v
		}
		raw["name"] = name
		raw["type"] = n.Type
		raw["server"] = n.Server
		raw["port"] = n.Port
		proxies = append(proxies, raw)
		names = append(names, name)
	}
	cfg := runtimeConfig{
		MixedPort:          mixedPort,
		AllowLan:           false,
		Mode:               "rule",
		LogLevel:           "warning", // keep quiet but still surface fatal parse errors
		ExternalController: controller,
		Secret:             secret,
		Proxies:            proxies,
	}
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return nil, err
	}
	return names, nil
}
