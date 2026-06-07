package dedupe

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/use632/clash-node-pipeline/internal/model"
)

func Dedupe(nodes []model.Node) []model.Node {
	seen := map[string]bool{}
	out := make([]model.Node, 0, len(nodes))
	for _, n := range nodes {
		fp := fingerprint(n)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		out = append(out, n)
	}
	return out
}

func fingerprint(n model.Node) string {
	keys := []string{"uuid", "password", "cipher", "sni", "servername", "alterId", "network", "security"}
	sort.Strings(keys)
	parts := []string{n.Type, strings.ToLower(n.Server), fmt.Sprint(n.Port)}
	for _, k := range keys {
		if v, ok := n.Raw[k]; ok {
			parts = append(parts, k+"="+fmt.Sprint(v))
		}
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}
