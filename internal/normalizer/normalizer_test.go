package normalizer

import (
	"testing"

	"github.com/use632/clash-node-pipeline/internal/model"
)

func TestNormalizeDuplicateNames(t *testing.T) {
	nodes := []model.Node{
		{Name: "Same", Type: "ss", Server: "a.example", Port: 1, Raw: map[string]any{}},
		{Name: "Same", Type: "ss", Server: "b.example", Port: 2, Raw: map[string]any{}},
	}
	out, issues := Normalize(nodes)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if out[0].Name == out[1].Name {
		t.Fatalf("duplicate names were not fixed: %#v", out)
	}
}

func TestNormalizeRejectsInvalid(t *testing.T) {
	nodes := []model.Node{{Name: "bad", Type: "ss", Port: 1, Raw: map[string]any{}}}
	out, issues := Normalize(nodes)
	if len(out) != 0 || len(issues) == 0 {
		t.Fatalf("expected invalid node to be rejected, out=%v issues=%v", out, issues)
	}
}
