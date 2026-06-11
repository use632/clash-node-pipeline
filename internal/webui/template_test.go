package webui

import (
	"strings"
	"testing"

	"github.com/use632/clash-node-pipeline/internal/model"
)

// Executing the template here (not just compiling) catches both parse errors
// from template.Must and any field references that don't exist on the data, and
// confirms the fetch-failure / issue-detail sections actually render.
func TestPageTemplateRendersIssues(t *testing.T) {
	data := pageData{
		Form: defaultForm(),
		Host: "127.0.0.1:8787",
		Ran:  true,
		Report: &model.Report{
			FetchFailed:  1,
			FinalNodes:   0,
			OutputFile:   "output/gui.clash.yaml",
			BadNodesFile: "output/gui.bad_nodes.log",
			BadNodes: []model.ParseIssue{
				{Source: "gui-source-1", Err: "fetch failed: 直连: http 403 人机验证页（疑似 Cloudflare）", Line: "https://blog.example/free"},
			},
		},
	}
	var sb strings.Builder
	if err := pageTemplate.Execute(&sb, data); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}
	out := sb.String()
	for _, want := range []string{"问题明细", "拉取失败", "人机验证", "gui-source-1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered page missing %q", want)
		}
	}
}
