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
	"strings"
	"testing"
)

// semBundle builds a bundle whose nodes carry the given bodies, so link edges
// come from real markdown links rather than a hand-stubbed edge set.
func semBundle(t *testing.T, bodies map[string]string) *Bundle {
	t.Helper()
	b := &Bundle{Nodes: map[string]*Node{}}
	for path, body := range bodies {
		b.Nodes[path] = &Node{
			Path:        path,
			Frontmatter: map[string]any{"type": "concept"},
			Body:        body,
		}
	}
	return b
}

func TestLintSimilarUnlinked_Reports(t *testing.T) {
	b := semBundle(t, map[string]string{
		"a/tannin.md":      "Astringency in red wine.",
		"a/astringency.md": "The drying grip of a red wine.",
	})
	idx := SemanticIndex{
		"a/tannin.md":      {{Path: "a/astringency.md", Score: 0.91}},
		"a/astringency.md": {{Path: "a/tannin.md", Score: 0.91}},
	}
	got := findingsFor(LintSemantic(b, idx, SemanticOptions{}), "similar-unlinked")
	if len(got) != 1 {
		t.Fatalf("want exactly ONE finding per pair (not one per node), got %d: %+v", len(got), got)
	}
	// Reported on the lexicographically-first path so output is stable.
	if got[0].Path != "a/astringency.md" {
		t.Errorf("finding should sit on the first path, got %q", got[0].Path)
	}
	if !strings.Contains(got[0].Message, "a/tannin.md") {
		t.Errorf("message should name the other node, got %q", got[0].Message)
	}
	if !strings.Contains(got[0].Message, "0.91") {
		t.Errorf("message should carry the score, got %q", got[0].Message)
	}
}

func TestLintSimilarUnlinked_SuppressedWhenLinked(t *testing.T) {
	idx := SemanticIndex{
		"a/tannin.md":      {{Path: "a/astringency.md", Score: 0.91}},
		"a/astringency.md": {{Path: "a/tannin.md", Score: 0.91}},
	}
	// A links B.
	fwd := semBundle(t, map[string]string{
		"a/tannin.md":      "See [astringency](astringency.md).",
		"a/astringency.md": "The drying grip.",
	})
	if got := findingsFor(LintSemantic(fwd, idx, SemanticOptions{}), "similar-unlinked"); len(got) != 0 {
		t.Errorf("a link A->B should suppress the finding, got %+v", got)
	}
	// B links A — either direction counts as "connected".
	rev := semBundle(t, map[string]string{
		"a/tannin.md":      "The drying grip.",
		"a/astringency.md": "See [tannin](tannin.md).",
	})
	if got := findingsFor(LintSemantic(rev, idx, SemanticOptions{}), "similar-unlinked"); len(got) != 0 {
		t.Errorf("a link B->A should also suppress the finding, got %+v", got)
	}
}

func TestLintSimilarUnlinked_BelowThreshold(t *testing.T) {
	b := semBundle(t, map[string]string{
		"a/one.md": "x", "a/two.md": "y",
	})
	idx := SemanticIndex{
		"a/one.md": {{Path: "a/two.md", Score: 0.79}},
		"a/two.md": {{Path: "a/one.md", Score: 0.79}},
	}
	opts := SemanticOptions{SimilarityThreshold: 0.80}
	if got := findingsFor(LintSemantic(b, idx, opts), "similar-unlinked"); len(got) != 0 {
		t.Errorf("0.79 is below the 0.80 threshold, got %+v", got)
	}
}

func TestLintNoSemanticNeighbors(t *testing.T) {
	b := semBundle(t, map[string]string{
		"a/lonely.md":    "Entirely unrelated subject matter.",
		"a/connected.md": "Wine.",
		"a/wine.md":      "Wine.",
	})
	idx := SemanticIndex{
		"a/lonely.md":    {{Path: "a/wine.md", Score: 0.12}},
		"a/connected.md": {{Path: "a/wine.md", Score: 0.55}},
		"a/wine.md":      {{Path: "a/connected.md", Score: 0.55}},
	}
	got := findingsFor(LintSemantic(b, idx, SemanticOptions{}), "no-semantic-neighbors")
	if len(got) != 1 {
		t.Fatalf("want 1 isolated node, got %d: %+v", len(got), got)
	}
	if got[0].Path != "a/lonely.md" {
		t.Errorf("wrong node flagged: %q", got[0].Path)
	}
}

func TestLintSemantic_StaleIndex(t *testing.T) {
	b := semBundle(t, map[string]string{
		"a/indexed.md": "Wine.",
		"a/fresh.md":   "Added since the last index build.",
		"a/other.md":   "Wine.",
	})
	// a/fresh.md is absent from the index.
	idx := SemanticIndex{
		"a/indexed.md": {{Path: "a/other.md", Score: 0.91}},
		"a/other.md":   {{Path: "a/indexed.md", Score: 0.91}},
	}
	all := LintSemantic(b, idx, SemanticOptions{})
	stale := findingsFor(all, "stale-index")
	if len(stale) != 1 {
		t.Fatalf("want ONE bundle-level stale finding, got %d: %+v", len(stale), stale)
	}
	if stale[0].Path != "" {
		t.Errorf("stale-index is bundle-level, want empty Path, got %q", stale[0].Path)
	}
	if !strings.Contains(stale[0].Message, "a/fresh.md") {
		t.Errorf("should name the unindexed node, got %q", stale[0].Message)
	}
	if !strings.Contains(stale[0].Message, "index build") {
		t.Errorf("should name the fix, got %q", stale[0].Message)
	}
	// The indexed nodes are still checked despite the drift.
	if got := findingsFor(all, "similar-unlinked"); len(got) != 1 {
		t.Errorf("indexed nodes should still be checked, got %+v", got)
	}
}

func TestLintSemantic_NoStaleWhenComplete(t *testing.T) {
	b := semBundle(t, map[string]string{"a/one.md": "x", "a/two.md": "y"})
	idx := SemanticIndex{
		"a/one.md": {{Path: "a/two.md", Score: 0.50}},
		"a/two.md": {{Path: "a/one.md", Score: 0.50}},
	}
	if got := findingsFor(LintSemantic(b, idx, SemanticOptions{}), "stale-index"); len(got) != 0 {
		t.Errorf("fully-indexed bundle should report no drift, got %+v", got)
	}
}

func TestLintSemantic_Deterministic(t *testing.T) {
	b := semBundle(t, map[string]string{
		"a/x.md": "p", "a/y.md": "q", "a/z.md": "r",
	})
	idx := SemanticIndex{
		"a/x.md": {{Path: "a/y.md", Score: 0.95}, {Path: "a/z.md", Score: 0.10}},
		"a/y.md": {{Path: "a/x.md", Score: 0.95}, {Path: "a/z.md", Score: 0.11}},
		"a/z.md": {{Path: "a/y.md", Score: 0.11}, {Path: "a/x.md", Score: 0.10}},
	}
	first := LintSemantic(b, idx, SemanticOptions{})
	for i := 0; i < 5; i++ {
		again := LintSemantic(b, idx, SemanticOptions{})
		if len(again) != len(first) {
			t.Fatalf("run %d length drift: %d != %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("run %d differs at %d: %+v != %+v", i, j, again[j], first[j])
			}
		}
	}
}
