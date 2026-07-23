package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLintFixture writes rel->content files under a temp dir, returns the dir.
func writeLintFixture(t *testing.T, files map[string]string) string {
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
	return dir
}

func doc(typ, title, body string) string {
	return "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

// orphanBundle has one orphan node (c.md, linked by nobody).
func orphanBundle(t *testing.T) string {
	return writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc("Concept", "A", "See [b](b.md)."),
		"b.md":     doc("Concept", "B", "Body."),
		"c.md":     doc("Concept", "C", "Nobody links here."),
	})
}

func TestLintCmd_ReportsFindingsExitsZeroByDefault(t *testing.T) {
	out, err := runOKF(t, "lint", orphanBundle(t))
	if err != nil {
		t.Fatalf("lint should exit 0 by default even with findings; got err: %v", err)
	}
	if !contains(out, "orphan") || !contains(out, "c.md") {
		t.Fatalf("expected orphan finding for c.md in output, got:\n%s", out)
	}
}

func TestLintCmd_StrictExitsNonZeroOnFinding(t *testing.T) {
	_, err := runOKF(t, "lint", "--strict", orphanBundle(t))
	if err == nil {
		t.Fatalf("lint --strict should exit non-zero when there are findings")
	}
}

func TestLintCmd_CleanBundleExitsZero(t *testing.T) {
	clean := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     doc("Concept", "A", "See [b](b.md)."),
		"b.md":     doc("Concept", "B", "See [a](a.md)."),
	})
	out, err := runOKF(t, "lint", clean)
	if err != nil {
		t.Fatalf("clean bundle should exit 0, got err: %v\n%s", err, out)
	}
	if _, err := runOKF(t, "lint", "--strict", clean); err != nil {
		t.Fatalf("clean bundle --strict should still exit 0, got err: %v", err)
	}
}

func TestLintCmd_CoverageThresholdFlag(t *testing.T) {
	dir := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     doc("Concept", "A", "The wine shows Terroir clearly."),
		"b.md":     doc("Concept", "B", "Terroir dominates here."),
	})
	// default threshold 3: no coverage-gap for a 2-mention term
	out, _ := runOKF(t, "lint", dir)
	if contains(out, "coverage-gap") && contains(out, "Terroir") {
		t.Fatalf("default threshold 3 should not flag a 2-mention term:\n%s", out)
	}
	// threshold 2: now flagged
	out2, _ := runOKF(t, "lint", "--coverage-threshold", "2", dir)
	if !contains(out2, "coverage-gap") {
		t.Fatalf("--coverage-threshold 2 should surface the gap:\n%s", out2)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
