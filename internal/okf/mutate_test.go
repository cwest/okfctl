package okf

import (
	"os"
	"path/filepath"
	"testing"
)

// mkMoveBundle writes concept nodes (rel->body) under a temp dir and loads it.
func mkMoveBundle(t *testing.T, files map[string]string) (string, *Bundle) {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return dir, b
}

func nodeSrc(title string) string {
	return "---\ntype: Concept\ntitle: " + title + "\n---\n"
}

func findRewrite(rw []LinkRewrite, nodePath string) (LinkRewrite, bool) {
	for _, r := range rw {
		if r.NodePath == nodePath {
			return r, true
		}
	}
	return LinkRewrite{}, false
}

func TestPlanMove_RootRelativeInboundPreserved(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"a.md":        nodeSrc("A") + "See [x](wine/foo.md).\n",
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "wine/bar.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	r, ok := findRewrite(rw, "a.md")
	if !ok {
		t.Fatalf("expected rewrite for a.md, got %+v", rw)
	}
	if r.Old != "wine/foo.md" || r.New != "wine/bar.md" {
		t.Fatalf("root-rel form not preserved: got Old=%q New=%q", r.Old, r.New)
	}
}

func TestPlanMove_DirRelativeInboundPreserved(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"wine/a.md":   nodeSrc("A") + "See [x](foo.md).\n",
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "wine/bar.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	r, ok := findRewrite(rw, "wine/a.md")
	if !ok {
		t.Fatalf("expected rewrite for wine/a.md, got %+v", rw)
	}
	if r.Old != "foo.md" || r.New != "bar.md" {
		t.Fatalf("dir-rel form not preserved: got Old=%q New=%q", r.Old, r.New)
	}
}

func TestPlanMove_DirRelativeAcrossDirs(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"red/a.md":    nodeSrc("A") + "See [x](../wine/foo.md).\n",
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "cellar/foo.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	r, ok := findRewrite(rw, "red/a.md")
	if !ok {
		t.Fatalf("expected rewrite for red/a.md, got %+v", rw)
	}
	if r.Old != "../wine/foo.md" || r.New != "../cellar/foo.md" {
		t.Fatalf("cross-dir dir-rel not preserved: got Old=%q New=%q", r.Old, r.New)
	}
}

func TestPlanMove_TitleSuffixPreserved(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"a.md":        nodeSrc("A") + "See [x](wine/foo.md \"Foo Note\").\n",
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "wine/bar.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	r, ok := findRewrite(rw, "a.md")
	if !ok {
		t.Fatalf("expected rewrite for a.md, got %+v", rw)
	}
	if r.Old != "wine/foo.md \"Foo Note\"" || r.New != "wine/bar.md \"Foo Note\"" {
		t.Fatalf("title suffix not preserved: got Old=%q New=%q", r.Old, r.New)
	}
}

func TestPlanMove_ImageAndExternalNotRewritten(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"a.md":        nodeSrc("A") + "![img](wine/foo.md) and [e](https://foo.md) and real [x](wine/foo.md).\n",
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "wine/bar.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	// Exactly one rewrite (the real link), image + external skipped.
	if len(rw) != 1 {
		t.Fatalf("expected exactly 1 rewrite, got %d: %+v", len(rw), rw)
	}
	if rw[0].Old != "wine/foo.md" {
		t.Fatalf("wrong target rewritten: %q", rw[0].Old)
	}
}

func TestPlanMove_MultipleInboundDeterministicOrder(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"z.md":        nodeSrc("Z") + "[x](wine/foo.md)\n",
		"a.md":        nodeSrc("A") + "[x](wine/foo.md)\n",
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "wine/bar.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if len(rw) != 2 {
		t.Fatalf("expected 2 rewrites, got %d", len(rw))
	}
	if rw[0].NodePath != "a.md" || rw[1].NodePath != "z.md" {
		t.Fatalf("not sorted by NodePath: %+v", rw)
	}
}

func TestPlanMove_NoInboundReturnsEmpty(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"a.md":        nodeSrc("A"),
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "wine/bar.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if len(rw) != 0 {
		t.Fatalf("expected no rewrites, got %+v", rw)
	}
}

func TestPlanMove_ErrOldMissing(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{"a.md": nodeSrc("A")})
	if _, err := PlanMove(b, "nope.md", "new.md"); err == nil {
		t.Fatal("expected error for missing old node")
	}
}

func TestPlanMove_ErrNewExists(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"a.md": nodeSrc("A"),
		"b.md": nodeSrc("B"),
	})
	if _, err := PlanMove(b, "a.md", "b.md"); err == nil {
		t.Fatal("expected error for existing new node")
	}
}

func TestPlanMove_ErrReserved(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{"a.md": nodeSrc("A")})
	if _, err := PlanMove(b, "index.md", "x.md"); err == nil {
		t.Fatal("expected error moving reserved file (old)")
	}
	if _, err := PlanMove(b, "a.md", "index.md"); err == nil {
		t.Fatal("expected error moving onto reserved file (new)")
	}
}
