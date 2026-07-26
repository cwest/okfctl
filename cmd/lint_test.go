// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	// "Card Sorting" is a declared alias (of z.md's Information Architecture)
	// with no node of its own, referenced by a.md (prose) and z.md (alias) —
	// two distinct nodes.
	dir := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [Z](z.md)\n",
		"a.md":     doc("Concept", "A", "The team ran Card Sorting to structure it."),
		"z.md":     "---\ntype: Concept\ntitle: Information Architecture\naliases: [Card Sorting]\n---\n\n# Information Architecture\n\nIA basics.\n",
	})
	// default threshold 3: no coverage-gap for a 2-reference term
	out, _ := runOKF(t, "lint", dir)
	if contains(out, "coverage-gap") && contains(out, "Card Sorting") {
		t.Fatalf("default threshold 3 should not flag a 2-reference term:\n%s", out)
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
