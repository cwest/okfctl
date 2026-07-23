package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readBody(t *testing.T, root, rel string) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(src)
}

func TestApplyMove_MovesFileAndRewritesBodies(t *testing.T) {
	root, b := mkMoveBundle(t, map[string]string{
		"a.md":        nodeSrc("A") + "See [x](wine/foo.md).\n",
		"wine/b.md":   nodeSrc("B") + "Also [y](foo.md).\n", // dir-rel to wine/foo.md
		"wine/foo.md": nodeSrc("Foo"),
	})
	rw, err := PlanMove(b, "wine/foo.md", "cellar/foo.md")
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if err := ApplyMove(root, b, "wine/foo.md", "cellar/foo.md", rw); err != nil {
		t.Fatalf("ApplyMove: %v", err)
	}
	// File moved on disk.
	if _, err := os.Stat(filepath.Join(root, "wine", "foo.md")); !os.IsNotExist(err) {
		t.Fatal("old file still present")
	}
	if _, err := os.Stat(filepath.Join(root, "cellar", "foo.md")); err != nil {
		t.Fatalf("new file missing: %v", err)
	}
	// Reload: edges now point to cellar/foo.md, none dangle to wine/foo.md.
	nb, err := Load(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, src := range []string{"a.md", "wine/b.md"} {
		outs := nb.OutboundLinks(src)
		var toNew, toOld bool
		for _, o := range outs {
			if o == "cellar/foo.md" {
				toNew = true
			}
			if o == "wine/foo.md" {
				toOld = true
			}
		}
		if !toNew || toOld {
			t.Fatalf("%s edges wrong after move: %v", src, outs)
		}
	}
	// Body of a.md kept root-rel form (rewritten to cellar/foo.md).
	if !strings.Contains(readBody(t, root, "a.md"), "[x](cellar/foo.md)") {
		t.Fatalf("a.md not rewritten root-rel: %q", readBody(t, root, "a.md"))
	}
	// Body of wine/b.md kept dir-rel form (../cellar/foo.md).
	if !strings.Contains(readBody(t, root, "wine/b.md"), "[y](../cellar/foo.md)") {
		t.Fatalf("wine/b.md not rewritten dir-rel: %q", readBody(t, root, "wine/b.md"))
	}
	// Frontmatter of every rewritten node MUST survive the move (regression:
	// ApplyMove must not write the parsed body over the whole file, dropping
	// the YAML frontmatter block).
	for _, rel := range []string{"a.md", "wine/b.md"} {
		full := readBody(t, root, rel)
		if !strings.HasPrefix(full, "---\n") || !strings.Contains(full, "type: Concept") {
			t.Fatalf("%s lost its frontmatter after move: %q", rel, full)
		}
	}
	// And the moved node revalidates clean (frontmatter intact bundle-wide).
	if fs := Validate(nb); len(fs) != 0 {
		t.Fatalf("bundle invalid after move: %v", fs)
	}
}

func TestApplyMove_CreatesIntermediateDirs(t *testing.T) {
	root, b := mkMoveBundle(t, map[string]string{"foo.md": nodeSrc("Foo")})
	rw, _ := PlanMove(b, "foo.md", "deep/nested/foo.md")
	if err := ApplyMove(root, b, "foo.md", "deep/nested/foo.md", rw); err != nil {
		t.Fatalf("ApplyMove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "deep", "nested", "foo.md")); err != nil {
		t.Fatalf("nested file missing: %v", err)
	}
}

func TestApplyMove_ErrNewExistsOnDisk(t *testing.T) {
	root, b := mkMoveBundle(t, map[string]string{
		"a.md": nodeSrc("A"),
		"b.md": nodeSrc("B"),
	})
	// PlanMove already guards this, but ApplyMove must not clobber even if called directly.
	if err := ApplyMove(root, b, "a.md", "b.md", nil); err == nil {
		t.Fatal("expected error: new exists on disk")
	}
	if strings.Contains(readBody(t, root, "b.md"), "title: A") {
		t.Fatal("b.md was clobbered")
	}
}

func TestPlanRemoveOrphans_ReportsNewlyOrphaned(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"a.md": nodeSrc("A") + "[x](b.md)\n", // only inbound to b
		"b.md": nodeSrc("B"),
	})
	orphs, err := PlanRemoveOrphans(b, "a.md")
	if err != nil {
		t.Fatalf("PlanRemoveOrphans: %v", err)
	}
	if len(orphs) != 1 || orphs[0] != "b.md" {
		t.Fatalf("expected [b.md], got %v", orphs)
	}
}

func TestPlanRemoveOrphans_StillLinkedNotReported(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"a.md": nodeSrc("A") + "[x](b.md)\n",
		"c.md": nodeSrc("C") + "[x](b.md)\n", // b also reachable from c
		"b.md": nodeSrc("B"),
	})
	orphs, err := PlanRemoveOrphans(b, "a.md")
	if err != nil {
		t.Fatalf("PlanRemoveOrphans: %v", err)
	}
	if len(orphs) != 0 {
		t.Fatalf("expected no orphans (b still linked from c), got %v", orphs)
	}
}

func TestPlanRemoveOrphans_ErrReservedOrMissing(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{"a.md": nodeSrc("A")})
	if _, err := PlanRemoveOrphans(b, "index.md"); err == nil {
		t.Fatal("expected error removing reserved file")
	}
	if _, err := PlanRemoveOrphans(b, "nope.md"); err == nil {
		t.Fatal("expected error removing missing node")
	}
}
