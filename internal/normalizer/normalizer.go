package normalizer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/use632/clash-node-pipeline/internal/model"
)

var badNameChars = regexp.MustCompile(`[\r\n\t]+`)

func Normalize(nodes []model.Node) ([]model.Node, []model.ParseIssue) {
	var out []model.Node
	var issues []model.ParseIssue
	nameCount := map[string]int{}
	regionCount := map[string]int{}
	for _, n := range nodes {
		n.Type = strings.ToLower(strings.TrimSpace(n.Type))
		n.Server = strings.TrimSpace(n.Server)
		if n.Raw == nil {
			n.Raw = map[string]any{}
		}
		if n.Type == "" || n.Server == "" || n.Port <= 0 || n.Port > 65535 {
			issues = append(issues, model.ParseIssue{Source: n.Source, Err: fmt.Sprintf("invalid node name=%q type=%q server=%q port=%d", n.Name, n.Type, n.Server, n.Port)})
			continue
		}
		base := cleanName(n.Name)
		if base == "" {
			region := guessRegion(n)
			regionCount[region]++
			base = fmt.Sprintf("%s-%02d", region, regionCount[region])
		}
		nameCount[base]++
		if nameCount[base] > 1 {
			n.Name = fmt.Sprintf("%s-%02d", base, nameCount[base])
		} else {
			n.Name = base
		}
		n.Raw["name"] = n.Name
		n.Raw["type"] = n.Type
		n.Raw["server"] = n.Server
		n.Raw["port"] = n.Port
		out = append(out, n)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, issues
}

func cleanName(s string) string {
	s = strings.TrimSpace(s)
	s = badNameChars.ReplaceAllString(s, " ")
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > 80 {
		runes := []rune(s)
		s = string(runes[:80])
	}
	return s
}

func guessRegion(n model.Node) string {
	text := strings.ToLower(n.Name + " " + n.Server)
	switch {
	case strings.Contains(text, "hong") || strings.Contains(text, "hk") || strings.Contains(text, "香港"):
		return "香港"
	case strings.Contains(text, "jp") || strings.Contains(text, "japan") || strings.Contains(text, "日本"):
		return "日本"
	case strings.Contains(text, "sg") || strings.Contains(text, "singapore") || strings.Contains(text, "新加坡"):
		return "新加坡"
	case strings.Contains(text, "us") || strings.Contains(text, "usa") || strings.Contains(text, "america") || strings.Contains(text, "美国"):
		return "美国"
	case strings.Contains(text, "tw") || strings.Contains(text, "taiwan") || strings.Contains(text, "台湾"):
		return "台湾"
	default:
		return "未知"
	}
}
