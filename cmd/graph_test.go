package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGraphFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.md":  "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](wine/a.md)\n",
		"wine/a.md": "---\ntype: Concept\ntitle: Alpha\n---\n\n# Alpha\n\nSee [Beta](b.md).\n",
		"wine/b.md": "---\ntype: Concept\ntitle: Beta\n---\n\n# Beta\n\nBody.\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func TestGraphExport_JSONDefault(t *testing.T) {
	dir := writeGraphFixture(t)
	out, err := runOKF(t, "graph", "export", dir)
	if err != nil {
		t.Fatalf("graph export (default json) errored: %v\n%s", err, out)
	}
	var g struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatalf("default output is not valid JSON: %v\n%s", err, out)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 concept nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge (A->B), got %d", len(g.Edges))
	}
}

func TestGraphExport_JSONDeterministic(t *testing.T) {
	dir := writeGraphFixture(t)
	out1, _ := runOKF(t, "graph", "export", dir, "--format", "json")
	out2, _ := runOKF(t, "graph", "export", dir, "--format", "json")
	if out1 != out2 {
		t.Fatalf("graph export json not byte-identical across runs")
	}
}

func TestGraphExport_DOT(t *testing.T) {
	dir := writeGraphFixture(t)
	out, err := runOKF(t, "graph", "export", dir, "--format", "dot")
	if err != nil {
		t.Fatalf("graph export dot errored: %v\n%s", err, out)
	}
	if !strings.Contains(out, "digraph") {
		t.Fatalf("dot output missing 'digraph':\n%s", out)
	}
	if !strings.Contains(out, "wine/a.md") || !strings.Contains(out, "wine/b.md") {
		t.Fatalf("dot output missing node ids:\n%s", out)
	}
}

func TestGraphExport_UnknownFormatErrors(t *testing.T) {
	dir := writeGraphFixture(t)
	_, err := runOKF(t, "graph", "export", dir, "--format", "xml")
	if err == nil {
		t.Fatalf("unknown --format should exit non-zero")
	}
}
