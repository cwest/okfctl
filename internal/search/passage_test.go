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

package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// longNode returns a node with several sections; one section is deliberately far
// from the others in vocabulary so a targeted query can distinguish it.
func longNode(title string, sections map[string]string, order []string) string {
	var sb strings.Builder
	sb.WriteString("---\ntype: Concept\ntitle: " + title + "\n---\n\n# " + title + "\n\n")
	sb.WriteString("Overview of the topic in general terms.\n\n")
	for _, h := range order {
		sb.WriteString("## " + h + "\n\n" + sections[h] + "\n\n")
	}
	return sb.String()
}

// TestBuildIndex_PopulatesPassagesAndKeepsEntries: the additive layer must fill
// Passages while leaving Entries populated and unchanged (one per node), so
// Related and the model-mismatch guard keep working off whole-node vectors.
func TestBuildIndex_PopulatesPassagesAndKeepsEntries(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md": longNode("Alpha",
			map[string]string{"One": "alpha content one", "Two": "alpha content two"},
			[]string{"One", "Two"}),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	if len(s.Entries) != 1 {
		t.Fatalf("Entries: want 1 (one per concept node), got %d", len(s.Entries))
	}
	if len(s.Entries[0].Vector) != e.Dim() {
		t.Errorf("whole-node vector dim = %d, want %d", len(s.Entries[0].Vector), e.Dim())
	}
	// Passages: preamble + 2 headings = 3 for node a.md.
	if len(s.Passages) < 3 {
		t.Fatalf("Passages: want >=3, got %d: %+v", len(s.Passages), s.Passages)
	}
	for _, p := range s.Passages {
		if p.NodePath != "a.md" {
			t.Errorf("passage NodePath = %q, want a.md", p.NodePath)
		}
		if len(p.Vector) != e.Dim() {
			t.Errorf("passage vector dim = %d, want %d", len(p.Vector), e.Dim())
		}
		if p.Hash == "" {
			t.Error("passage missing content hash")
		}
		if p.Text == "" {
			t.Error("passage missing text")
		}
	}
}

// TestBuildIndex_PassagesDeterministic: serialization is byte-stable for a fixed
// embedder (passages sorted deterministically).
func TestBuildIndex_PassagesDeterministic(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"a.md": longNode("Alpha",
			map[string]string{"One": "alpha one", "Two": "alpha two", "Three": "alpha three"},
			[]string{"One", "Two", "Three"}),
		"b.md": longNode("Beta",
			map[string]string{"X": "beta x", "Y": "beta y"},
			[]string{"X", "Y"}),
	})
	e := NewHashEmbedder()
	p1 := filepath.Join(t.TempDir(), "a.db")
	p2 := filepath.Join(t.TempDir(), "b.db")
	if err := BuildIndex(b, e, nil).Save(p1); err != nil {
		t.Fatal(err)
	}
	if err := BuildIndex(b, e, nil).Save(p2); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(p1)
	c, _ := os.ReadFile(p2)
	if string(a) != string(c) {
		t.Error("BuildIndex passages not byte-deterministic for a fixed embedder")
	}
}

// TestBuildIndex_PassageContentHashReuse: an unchanged node's passages are reused
// (not re-embedded) across a rebuild; a changed node's passages get new hashes.
func TestBuildIndex_PassageContentHashReuse(t *testing.T) {
	files := map[string]string{
		"a.md": longNode("Alpha", map[string]string{"One": "alpha one"}, []string{"One"}),
		"b.md": longNode("Beta", map[string]string{"X": "beta x"}, []string{"X"}),
	}
	b, dir := writeBundle(t, files)
	e := NewHashEmbedder()
	prev := BuildIndex(b, e, nil)
	prevHashByKey := map[string]string{}
	for _, p := range prev.Passages {
		prevHashByKey[p.NodePath+"\x00"+p.HeadingPath] = p.Hash
	}

	// Change only b.md.
	if err := os.WriteFile(filepath.Join(dir, "b.md"),
		[]byte(longNode("Beta", map[string]string{"X": "beta CHANGED"}, []string{"X"})), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, err := okf.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	next := BuildIndex(b2, e, prev)
	for _, p := range next.Passages {
		key := p.NodePath + "\x00" + p.HeadingPath
		if p.NodePath == "a.md" {
			if old, ok := prevHashByKey[key]; ok && old != p.Hash {
				t.Errorf("unchanged node a.md passage %q hash drifted", p.HeadingPath)
			}
		}
	}
}

// TestQuery_RanksPassagesAndDedupsToBestPerNode: when passages are present, Query
// ranks passages, returns one result per node (best-scoring passage), and fills
// Snippet with the passage text.
func TestQuery_RanksPassagesAndDedupsToBestPerNode(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"a.md": longNode("Alpha",
			map[string]string{
				"Fermentation": "yeast sugar conversion produces alcohol during fermentation",
				"Storage":      "cellar temperature humidity for storage of bottles",
			},
			[]string{"Fermentation", "Storage"}),
		"b.md": longNode("Beta",
			map[string]string{"Colors": "red white rose hues of grape skins"},
			[]string{"Colors"}),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := Query(s, e, "fermentation yeast sugar alcohol", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
	// Dedup: at most one result per node path.
	seen := map[string]bool{}
	for _, r := range res {
		if seen[r.Path] {
			t.Errorf("node %q returned more than once; dedup-to-best-passage failed", r.Path)
		}
		seen[r.Path] = true
	}
	// Top result is node a.md and its snippet is the Fermentation passage, not Storage.
	if res[0].Path != "a.md" {
		t.Fatalf("top result = %q, want a.md; res=%+v", res[0].Path, res)
	}
	if !strings.Contains(res[0].Snippet, "fermentation") {
		t.Errorf("top snippet = %q, want the Fermentation passage", res[0].Snippet)
	}
	if strings.Contains(res[0].Snippet, "cellar temperature") {
		t.Errorf("top snippet leaked the Storage passage: %q", res[0].Snippet)
	}
}

// TestQuery_FallsBackToEntriesWithoutPassages: a store with no Passages (e.g. an
// old index) still answers via whole-node Entries and returns empty snippets.
func TestQuery_FallsBackToEntriesWithoutPassages(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)
	s.Passages = nil // simulate a legacy passage-less index

	res, err := Query(s, e, "tannin structure astringency", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results from entries fallback")
	}
	if res[0].Path != "wine/tannin.md" {
		t.Errorf("fallback top = %q, want wine/tannin.md", res[0].Path)
	}
}

// TestQuery_PositiveControl_LongNodeSurfacesRightPassage is the card's positive
// control: a long, multi-section node whose target section is a small fraction of
// its total text. Whole-node mean pooling averages that section away; the passage
// layer must surface exactly it. We assert BOTH that the node ranks top for a
// targeted query AND that passage ranking beats whole-node ranking on the same
// query — proving the passage layer, not luck, did the work.
func TestQuery_PositiveControl_LongNodeSurfacesRightPassage(t *testing.T) {
	// A long node about wine, with one small buried section on "malolactic
	// fermentation" surrounded by unrelated bulk on color, glassware, regions.
	filler := strings.Repeat("color hue glassware region vintage label bottle cork ", 40)
	target := "malolactic fermentation converts sharp malic acid into softer lactic acid by bacteria"
	long := longNode("WineBook",
		map[string]string{
			"Appearance": filler,
			"Glassware":  filler,
			"Regions":    filler,
			"Malolactic": target,
			"Storage":    filler,
		},
		[]string{"Appearance", "Glassware", "Regions", "Malolactic", "Storage"})
	// A short competing node that mentions fermentation only in passing.
	short := node("Concept", "Grapes", "Grapes are harvested and pressed before fermentation begins.")

	b, _ := writeBundle(t, map[string]string{
		"wine/book.md":   long,
		"wine/grapes.md": short,
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	q := "malolactic fermentation malic lactic acid bacteria"

	// Passage-based query surfaces the long node and its Malolactic snippet.
	res, err := Query(s, e, q, 5)
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Path != "wine/book.md" {
		t.Fatalf("passage query top = %q, want wine/book.md; res=%+v", res[0].Path, res)
	}
	if !strings.Contains(res[0].Snippet, "malolactic") {
		t.Errorf("top snippet = %q, want the Malolactic passage", res[0].Snippet)
	}

	// Whole-node pooling scores the long node WORSE on the same query — the
	// control that proves pooling loses the buried passage.
	qv := e.Encode([]string{q})[0]
	poolScore := cosine(qv, entryVector(s, "wine/book.md"))
	if res[0].Score <= poolScore {
		t.Errorf("passage score %.4f did not beat whole-node pooled score %.4f; passage layer added nothing",
			res[0].Score, poolScore)
	}
}

// TestQuery_NegativeControl_ShortNodeUnchanged is the card's negative control:
// a short, single-concept node that whole-node pooling already served correctly
// must still be the top result under passage ranking. Dedup-to-best-passage must
// not regress the easy cases.
func TestQuery_NegativeControl_ShortNodeUnchanged(t *testing.T) {
	b, _ := fixtureBundle(t) // 3 short, uniform concept nodes
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	q := "tannin structure astringency"

	// Whole-node ranking (the pre-change behavior) on the same store.
	qv := e.Encode([]string{q})[0]
	wholeTop := rank(s.Entries, qv, 3, "", Filter{}, nil)[0].Path

	// Passage ranking must produce the same top node.
	res, err := Query(s, e, q, 3)
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Path != wholeTop {
		t.Errorf("passage top = %q but whole-node top = %q; dedup regressed a well-served short node",
			res[0].Path, wholeTop)
	}
	if res[0].Path != "wine/tannin.md" {
		t.Errorf("top = %q, want wine/tannin.md", res[0].Path)
	}
}

// entryVector returns the whole-node vector for a path (test helper for controls).
func entryVector(s *Store, path string) []float64 {
	for _, en := range s.Entries {
		if en.Path == path {
			return en.Vector
		}
	}
	return nil
}
