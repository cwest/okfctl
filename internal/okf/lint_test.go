package okf

import (
	"os"
	"path/filepath"
	"testing"
)

// mkLintBundle writes files (rel->full file content) under a temp dir and loads it.
func mkLintBundle(t *testing.T, files map[string]string) *Bundle {
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
		t.Fatalf("Load: %v", err)
	}
	return b
}

func lintDoc(typ, title, body string) string {
	return "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

// findingsFor returns the findings of a given check kind, keyed by Path.
func findingsFor(fs []LintFinding, check string) []LintFinding {
	var out []LintFinding
	for _, f := range fs {
		if f.Check == check {
			out = append(out, f)
		}
	}
	return out
}

func TestLint_Orphan_NoInbound(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     lintDoc("Concept", "A", "See [b](b.md)."),
		"b.md":     lintDoc("Concept", "B", "Body of B."),
		"c.md":     lintDoc("Concept", "C", "Nobody links here."),
	})
	orphans := findingsFor(Lint(b, LintOptions{}), "orphan")
	if len(orphans) != 1 || orphans[0].Path != "c.md" {
		t.Fatalf("expected one orphan (c.md), got %+v", orphans)
	}
}

func TestLint_Orphan_IndexRescues(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [C](c.md)\n",
		"c.md":     lintDoc("Concept", "C", "Reachable only via the index."),
	})
	orphans := findingsFor(Lint(b, LintOptions{}), "orphan")
	if len(orphans) != 0 {
		t.Fatalf("index link should rescue c.md from orphan status, got %+v", orphans)
	}
}

func TestLint_Orphan_ReservedNeverOrphan(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"log.md":   "---\ntype: Log\ntitle: Change Log\n---\n\n# Change Log\n",
		"a.md":     lintDoc("Concept", "A", "Body."),
	})
	for _, o := range findingsFor(Lint(b, LintOptions{}), "orphan") {
		if o.Path == "index.md" || o.Path == "log.md" {
			t.Fatalf("reserved file reported as orphan: %+v", o)
		}
	}
}

func TestLint_MissingXref_MentionsTitleNoLink(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [MLF](mlf.md)\n",
		"a.md":     lintDoc("Concept", "A", "Wine often undergoes Malolactic Fermentation in the cellar."),
		"mlf.md":   lintDoc("Concept", "Malolactic Fermentation", "The conversion of malic to lactic acid."),
	})
	xr := findingsFor(Lint(b, LintOptions{}), "missing-xref")
	found := false
	for _, f := range xr {
		if f.Path == "a.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing-xref on a.md (mentions Malolactic Fermentation, no link), got %+v", xr)
	}
}

func TestLint_MissingXref_AlreadyLinkedNoFinding(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [MLF](mlf.md)\n",
		"a.md":     lintDoc("Concept", "A", "Wine undergoes [Malolactic Fermentation](mlf.md) in the cellar."),
		"mlf.md":   lintDoc("Concept", "Malolactic Fermentation", "Body."),
	})
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "a.md" {
			t.Fatalf("a.md already links to mlf.md; should not be a missing-xref: %+v", f)
		}
	}
}

func TestLint_MissingXref_CaseInsensitive(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [MLF](mlf.md)\n",
		"a.md":     lintDoc("Concept", "A", "The cellar step is malolactic fermentation, done early."),
		"mlf.md":   lintDoc("Concept", "Malolactic Fermentation", "Body."),
	})
	found := false
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "a.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("case-insensitive mention should still flag missing-xref on a.md")
	}
}
