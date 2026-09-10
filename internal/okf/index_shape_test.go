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

package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeNodeTags writes a concept node with an explicit type, title, and tag
// list, so the subdirectory shape suffix (OKF §8 progressive disclosure; tags
// per §4.1) can be exercised. An empty tags slice omits the key.
func writeNodeTags(t *testing.T, dir, rel, typ, title string, tags []string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\ntype: " + typ + "\ntitle: " + title + "\n"
	if len(tags) > 0 {
		fm += "tags:\n"
		for _, tg := range tags {
			fm += "  - " + tg + "\n"
		}
	}
	fm += "---\n\n# " + title + "\n"
	if err := os.WriteFile(p, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestDirShape_CountTypesTags pins the core §8 progressive-disclosure suffix:
// the child's own immediate concept COUNT, its distinct TYPES in full (sorted),
// and its shared TAGS (carried by >= TagMin of the immediate concepts).
func TestDirShape_CountTypesTags(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux", "layout"})
	writeNodeTags(t, dir, "design/b.md", "Map", "B", []string{"ux", "color"})
	writeNodeTags(t, dir, "design/c.md", "Application", "C", []string{"ux"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "design", DefaultShapeOptions())

	// 3 concepts, plural.
	if !strings.Contains(got, "3 concepts") {
		t.Errorf("expected `3 concepts`; got %q", got)
	}
	// Types in full, sorted, no duplicates.
	if !strings.Contains(got, "Application, Concept, Map") {
		t.Errorf("expected all three types sorted; got %q", got)
	}
	// `ux` is shared by all 3 (>= min 2); `layout`/`color` each by only 1, omitted.
	if !strings.Contains(got, "shared tags: ux") {
		t.Errorf("expected shared tag `ux`; got %q", got)
	}
	if strings.Contains(got, "layout") || strings.Contains(got, "color") {
		t.Errorf("single-node tags must be omitted; got %q", got)
	}
	// The whole suffix is trailing parenthesised text.
	if !strings.HasPrefix(strings.TrimSpace(got), "(") {
		t.Errorf("shape suffix must be parenthesised; got %q", got)
	}
}

// TestDirShape_SingularConcept pins the `1 concept` (singular) form and that a
// directory whose single own concept cannot share any tag prints NO tag segment
// (real-corpus positive control: security/, integrations/, wine/ each own 1).
func TestDirShape_SingularConcept(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "security/only.md", "Framework", "Only", []string{"authn", "authz"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "security", DefaultShapeOptions())
	if !strings.Contains(got, "1 concept ") && !strings.Contains(got, "1 concept)") {
		t.Errorf("expected singular `1 concept`; got %q", got)
	}
	if strings.Contains(got, "concepts") {
		t.Errorf("must not pluralize a single concept; got %q", got)
	}
	if strings.Contains(got, "shared tags") {
		t.Errorf("a lone concept has no shared tags; segment must be omitted; got %q", got)
	}
}

// TestDirShape_TagOmittedBelowThreshold pins that the `shared tags:` segment is
// omitted entirely when no tag meets TagMin.
func TestDirShape_TagOmittedBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "d/a.md", "Concept", "A", []string{"alpha"})
	writeNodeTags(t, dir, "d/b.md", "Concept", "B", []string{"beta"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "d", DefaultShapeOptions())
	if strings.Contains(got, "shared tags") {
		t.Errorf("no tag shared by >=2 nodes; segment must be omitted; got %q", got)
	}
}

// TestDirShape_CaseFold pins §4.1 tag folding: `api` and `API` fold to ONE tag,
// rendered in the DOMINANT casing (the casing carried by more nodes). Sorting
// and thresholds are case-insensitive.
func TestDirShape_CaseFold(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	// `API` appears on 2 nodes, `api` on 1 → dominant casing is `API`, folded
	// count is 3 (>= min 2).
	writeNodeTags(t, dir, "x/a.md", "Concept", "A", []string{"API"})
	writeNodeTags(t, dir, "x/b.md", "Concept", "B", []string{"API"})
	writeNodeTags(t, dir, "x/c.md", "Concept", "C", []string{"api"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "x", DefaultShapeOptions())
	if !strings.Contains(got, "shared tags: API") {
		t.Errorf("folded tag must render in dominant casing `API`; got %q", got)
	}
	if strings.Contains(got, "api,") || strings.HasSuffix(strings.TrimRight(got, ")"), "api") {
		t.Errorf("lowercase variant must not render separately; got %q", got)
	}
}

// TestDirShape_CaseFoldTieAlphabetical pins the tie-break for dominant casing:
// when two casings are carried by an equal number of nodes, the lexicographically
// smaller casing wins.
func TestDirShape_CaseFoldTieAlphabetical(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	// `API` on 1, `api` on 1 → tie; alphabetical: `API` < `api` (uppercase
	// sorts before lowercase in ASCII).
	writeNodeTags(t, dir, "y/a.md", "Concept", "A", []string{"API"})
	writeNodeTags(t, dir, "y/b.md", "Concept", "B", []string{"api"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "y", DefaultShapeOptions())
	if !strings.Contains(got, "shared tags: API") {
		t.Errorf("tie in casing must resolve alphabetically to `API`; got %q", got)
	}
}

// TestDirShape_FoldDoesNotCollapseDistinct is the NEGATIVE control for folding:
// `v1`/`v2` and `oauth1`/`oauth2` differ in a NON-case character, so they must
// each print separately when each is shared — the case-fold must not collapse
// distinct tags.
func TestDirShape_FoldDoesNotCollapseDistinct(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "z/a.md", "Concept", "A", []string{"v1", "oauth1"})
	writeNodeTags(t, dir, "z/b.md", "Concept", "B", []string{"v1", "oauth1"})
	writeNodeTags(t, dir, "z/c.md", "Concept", "C", []string{"v2", "oauth2"})
	writeNodeTags(t, dir, "z/d.md", "Concept", "D", []string{"v2", "oauth2"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "z", DefaultShapeOptions())
	for _, want := range []string{"v1", "v2", "oauth1", "oauth2"} {
		if !strings.Contains(got, want) {
			t.Errorf("distinct tag %q must print separately (fold must not collapse); got %q", want, got)
		}
	}
}

// TestDirShape_TruncationMostSharedFirst pins the cap + ordering: with more
// shared tags than TagMax, exactly TagMax print, most-shared-first, with a
// deterministic alphabetical tie-break, followed by an ellipsis.
func TestDirShape_TruncationMostSharedFirst(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	// hot tag on all 5 nodes; then a graded set so ordering is unambiguous, plus
	// a block of equal-count tags to exercise the alphabetical tie-break.
	writeNodeTags(t, dir, "r/a.md", "Concept", "A", []string{"hot", "b4", "b3", "b2", "b1", "t_c", "t_a", "t_b"})
	writeNodeTags(t, dir, "r/b.md", "Concept", "B", []string{"hot", "b4", "b3", "b2", "b1", "t_c", "t_a", "t_b"})
	writeNodeTags(t, dir, "r/c.md", "Concept", "C", []string{"hot", "b4", "b3", "b2"})
	writeNodeTags(t, dir, "r/d.md", "Concept", "D", []string{"hot", "b4", "b3"})
	writeNodeTags(t, dir, "r/e.md", "Concept", "E", []string{"hot", "b4"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	opts := IndexShapeOptions{Enabled: true, TagMin: 2, TagMax: 3}
	got := dirShape(b, "r", opts)

	// hot(5), b4(5) tie at 5 → alphabetical: b4, hot; then b3(4). Cap 3.
	seg := got[strings.Index(got, "shared tags:"):]
	if !strings.Contains(seg, "shared tags: b4, hot, b3") {
		t.Errorf("expected most-shared-first with alpha tie-break `b4, hot, b3`; got %q", seg)
	}
	// Ellipsis on truncation.
	if !strings.Contains(seg, "…") {
		t.Errorf("truncated list must end with an ellipsis; got %q", seg)
	}
	// Exactly 3 tags before the ellipsis (no b2/b1/t_*).
	for _, absent := range []string{"b2", "b1", "t_a", "t_b", "t_c"} {
		if strings.Contains(seg, absent) {
			t.Errorf("tag %q beyond the cap must not print; got %q", absent, seg)
		}
	}
}

// TestDirShape_NoEllipsisWhenNotTruncated pins that the ellipsis appears ONLY on
// truncation — a list at or under the cap ends clean.
func TestDirShape_NoEllipsisWhenNotTruncated(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "s/a.md", "Concept", "A", []string{"one", "two"})
	writeNodeTags(t, dir, "s/b.md", "Concept", "B", []string{"one", "two"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "s", IndexShapeOptions{Enabled: true, TagMin: 2, TagMax: 12})
	if strings.Contains(got, "…") {
		t.Errorf("no truncation → no ellipsis; got %q", got)
	}
}

// TestDirShape_Deterministic pins byte-stable output across repeated calls (no
// map-iteration order leakage, no clock).
func TestDirShape_Deterministic(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	for i, tg := range []string{"alpha", "bravo", "charlie", "delta", "echo"} {
		_ = i
		writeNodeTags(t, dir, "det/"+tg+"1.md", "Concept", tg+"1", []string{"shared", tg})
		writeNodeTags(t, dir, "det/"+tg+"2.md", "Concept", tg+"2", []string{"shared", tg})
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := dirShape(b, "det", DefaultShapeOptions())
	for i := 0; i < 20; i++ {
		if got := dirShape(b, "det", DefaultShapeOptions()); got != first {
			t.Fatalf("dirShape not deterministic:\n first=%q\n got  =%q", first, got)
		}
	}
}

// TestDirShape_SubtreeTotalWhenNested pins the `· N in subtree` suffix: shown
// when the child has content-bearing descendants (subtree count > own immediate
// count), and OMITTED when the child's own count equals its subtree count.
func TestDirShape_SubtreeTotalWhenNested(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	// security/ owns 1 concept directly, plus 2 in security/auth → subtree 3.
	writeNodeTags(t, dir, "security/only.md", "Framework", "Only", nil)
	writeNodeTags(t, dir, "security/auth/a.md", "Concept", "A", nil)
	writeNodeTags(t, dir, "security/auth/b.md", "Concept", "B", nil)
	// integrations/ owns 1 and nests no deeper.
	writeNodeTags(t, dir, "integrations/x.md", "Research Brief", "X", nil)

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sec := dirShape(b, "security", DefaultShapeOptions())
	if !strings.Contains(sec, "3 in subtree") {
		t.Errorf("security has descendants (subtree 3 > own 1) → must show `3 in subtree`; got %q", sec)
	}
	intg := dirShape(b, "integrations", DefaultShapeOptions())
	if strings.Contains(intg, "in subtree") {
		t.Errorf("integrations own==subtree → must NOT show a subtree total; got %q", intg)
	}
}

// TestDirShape_ZeroOwnConcepts pins a directory whose only content is nested
// directories: it renders `0 concepts` plus its subtree total (fixture-only —
// no such dir exists in the real corpus).
func TestDirShape_ZeroOwnConcepts(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	// shell/ holds no concept directly, only shell/inner/.
	writeNodeTags(t, dir, "shell/inner/a.md", "Concept", "A", nil)
	writeNodeTags(t, dir, "shell/inner/b.md", "Concept", "B", nil)

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "shell", DefaultShapeOptions())
	if !strings.Contains(got, "0 concepts") {
		t.Errorf("a dir with no own concept must render `0 concepts`; got %q", got)
	}
	if !strings.Contains(got, "2 in subtree") {
		t.Errorf("must still show the subtree total `2 in subtree`; got %q", got)
	}
	// No type or tag segment when there are no own concepts.
	if strings.Contains(got, "shared tags") {
		t.Errorf("no own concepts → no tag segment; got %q", got)
	}
}

// TestDirShape_DisabledEmitsNothing pins that a disabled shape (the --no-shape
// path) produces an EMPTY suffix, so the bullet is byte-identical to the
// pre-feature output.
func TestDirShape_DisabledEmitsNothing(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux", "ux2"})
	writeNodeTags(t, dir, "design/b.md", "Concept", "B", []string{"ux"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirShape(b, "design", IndexShapeOptions{Enabled: false}); got != "" {
		t.Errorf("disabled shape must emit nothing; got %q", got)
	}
}

// TestDirShape_ThreeLevelSubtreeRollup pins the subtree arithmetic across THREE
// levels (no such nesting exists in the real corpus, so this is fixture-only,
// per the card's done-when). a/ owns 1 concept, a/b owns 2, a/b/c owns 3 →
// a's subtree total is 6, a/b's is 5. The suffix rolls the whole descendant
// tree, not just the immediate children.
func TestDirShape_ThreeLevelSubtreeRollup(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "a/own.md", "Concept", "AOwn", nil)
	writeNodeTags(t, dir, "a/b/one.md", "Concept", "B1", nil)
	writeNodeTags(t, dir, "a/b/two.md", "Concept", "B2", nil)
	writeNodeTags(t, dir, "a/b/c/x.md", "Concept", "C1", nil)
	writeNodeTags(t, dir, "a/b/c/y.md", "Concept", "C2", nil)
	writeNodeTags(t, dir, "a/b/c/z.md", "Concept", "C3", nil)

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Rendered from the ROOT, the `a/` bullet: own 1, subtree 6.
	a := dirShape(b, "a", DefaultShapeOptions())
	if !strings.Contains(a, "1 concept ") && !strings.Contains(a, "1 concept)") {
		t.Errorf("a/ own count must be 1; got %q", a)
	}
	if !strings.Contains(a, "6 in subtree") {
		t.Errorf("a/ subtree total must roll all 3 levels to 6; got %q", a)
	}
	// The `a/b` bullet (rendered from a/): own 2, subtree 5.
	ab := dirShape(b, "a/b", DefaultShapeOptions())
	if !strings.Contains(ab, "2 concepts") {
		t.Errorf("a/b own count must be 2; got %q", ab)
	}
	if !strings.Contains(ab, "5 in subtree") {
		t.Errorf("a/b subtree total must be 5 (2 own + 3 in c/); got %q", ab)
	}
	// The deepest, a/b/c: own 3, no descendants → no subtree total.
	abc := dirShape(b, "a/b/c", DefaultShapeOptions())
	if !strings.Contains(abc, "3 concepts") {
		t.Errorf("a/b/c own count must be 3; got %q", abc)
	}
	if strings.Contains(abc, "in subtree") {
		t.Errorf("a/b/c is a leaf (own==subtree) → no subtree total; got %q", abc)
	}
}

// TestDirShape_ExactFormat is a byte-exact golden on the whole suffix, pinning
// the separator (` · `), the parenthesisation, segment order (count · types ·
// shared tags), and the trailing subtree total form — the grammar a downstream
// reader parses.
func TestDirShape_ExactFormat(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	// design/ owns 2 concepts (Concept, Map) sharing `ux`, plus a nested
	// design/deep/ with 1 concept → subtree 3.
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeNodeTags(t, dir, "design/b.md", "Map", "B", []string{"ux"})
	writeNodeTags(t, dir, "design/deep/c.md", "Concept", "C", nil)

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := dirShape(b, "design", DefaultShapeOptions())
	want := " (2 concepts · Concept, Map · shared tags: ux) · 3 in subtree"
	if got != want {
		t.Errorf("exact suffix format drift:\n got:  %q\n want: %q", got, want)
	}
}
