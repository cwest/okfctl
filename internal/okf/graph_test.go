package okf

import (
	"os"
	"path/filepath"
	"testing"
)

// mkGraphBundle writes rel->full-file-content under a temp dir and loads it.
func mkGraphBundle(t *testing.T, files map[string]string) *Bundle {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return b
}

func gnode(typ, title, body string) string {
	return "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

// index links A; A links B; C is orphaned (no inbound).
func graphFixture(t *testing.T) *Bundle {
	return mkGraphBundle(t, map[string]string{
		"index.md":  "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](wine/a.md)\n",
		"wine/a.md": gnode("Concept", "Alpha", "See [Beta](b.md)."),
		"wine/b.md": gnode("Concept", "Beta", "Body of Beta."),
		"wine/c.md": gnode("Concept", "Gamma", "Nobody links here."),
	})
}

func TestBuildGraph_NodesSortedWithFields(t *testing.T) {
	g := BuildGraph(graphFixture(t))
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 concept nodes (reserved excluded), got %d: %+v", len(g.Nodes), g.Nodes)
	}
	// sorted by path: wine/a.md, wine/b.md, wine/c.md
	wantPaths := []string{"wine/a.md", "wine/b.md", "wine/c.md"}
	for i, w := range wantPaths {
		if g.Nodes[i].Path != w {
			t.Fatalf("node[%d].Path = %q, want %q (sorted by path)", i, g.Nodes[i].Path, w)
		}
	}
	a := g.Nodes[0]
	if a.Title != "Alpha" || a.Type != "Concept" || a.Neighborhood != "wine" {
		t.Fatalf("node A fields wrong: %+v", a)
	}
}

func TestBuildGraph_OrphanFlagMatchesInbound(t *testing.T) {
	g := BuildGraph(graphFixture(t))
	orphan := map[string]bool{}
	for _, n := range g.Nodes {
		orphan[n.Path] = n.Orphan
	}
	if orphan["wine/a.md"] {
		t.Errorf("A is linked from index → not orphan")
	}
	if orphan["wine/b.md"] {
		t.Errorf("B is linked from A → not orphan")
	}
	if !orphan["wine/c.md"] {
		t.Errorf("C has no inbound → should be orphan")
	}
}

func TestBuildGraph_EdgesSortedInBundleOnly(t *testing.T) {
	g := BuildGraph(graphFixture(t))
	// Concept-node edges only (reserved index.md is not a graph node): A->B.
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 concept edge (A->B), got %d: %+v", len(g.Edges), g.Edges)
	}
	e := g.Edges[0]
	if e.From != "wine/a.md" || e.To != "wine/b.md" {
		t.Fatalf("edge = %+v, want wine/a.md -> wine/b.md", e)
	}
}

func TestBuildGraph_Deterministic(t *testing.T) {
	b := graphFixture(t)
	g1 := BuildGraph(b)
	g2 := BuildGraph(b)
	if len(g1.Nodes) != len(g2.Nodes) || len(g1.Edges) != len(g2.Edges) {
		t.Fatalf("nondeterministic sizes")
	}
	for i := range g1.Nodes {
		if g1.Nodes[i] != g2.Nodes[i] {
			t.Fatalf("node[%d] differs between runs: %+v vs %+v", i, g1.Nodes[i], g2.Nodes[i])
		}
	}
	for i := range g1.Edges {
		if g1.Edges[i] != g2.Edges[i] {
			t.Fatalf("edge[%d] differs between runs", i)
		}
	}
}
