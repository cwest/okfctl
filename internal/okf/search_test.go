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
	"reflect"
	"testing"
)

// writeSearchBundle lays down a small fixture with known titles, types, tags,
// body text, and a known edge shape:
//
//	index.md -> wine/tannin.md
//	wine/tannin.md -> wine/acidity.md   (ONE-directional: only tannin links acidity)
//	security/auth.md                     (isolated neighborhood, orphan)
//
// The single-direction tannin->acidity edge is deliberate: it proves the
// traversal is UNDIRECTED, because a directed traversal from acidity would find
// no neighbor.
//
// so lexical and graph-structural queries have deterministic expectations.
func writeSearchBundle(t *testing.T) *Bundle {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.md":         "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Tannin](wine/tannin.md)\n",
		"wine/tannin.md":   "---\ntype: Concept\ntitle: Tannin\ntags: [wine, chemistry]\n---\n\n# Tannin\n\nTannins bind proteins. See [Acidity](acidity.md).\n",
		"wine/acidity.md":  "---\ntype: Concept\ntitle: Acidity\ntags: [wine]\n---\n\n# Acidity\n\nMouthfeel and pH.\n",
		"security/auth.md": "---\ntype: Playbook\ntitle: Authentication\ntags: [security]\n---\n\n# Authentication\n\nToken rotation and mouthfeel is unrelated.\n",
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
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return b
}

func paths(rs []SearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Path
	}
	return out
}

func TestSearch_TitleMatch(t *testing.T) {
	b := writeSearchBundle(t)
	got := paths(Search(b, "tannin", FieldTitle))
	want := []string{"wine/tannin.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("title search: got %v want %v", got, want)
	}
}

func TestSearch_TypeMatch(t *testing.T) {
	b := writeSearchBundle(t)
	got := paths(Search(b, "playbook", FieldType))
	want := []string{"security/auth.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("type search: got %v want %v", got, want)
	}
}

func TestSearch_TagMatch(t *testing.T) {
	b := writeSearchBundle(t)
	got := paths(Search(b, "chemistry", FieldTag))
	want := []string{"wine/tannin.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tag search: got %v want %v", got, want)
	}
	// A tag shared by two nodes returns both, sorted by path.
	got2 := paths(Search(b, "wine", FieldTag))
	want2 := []string{"wine/acidity.md", "wine/tannin.md"}
	if !reflect.DeepEqual(got2, want2) {
		t.Fatalf("shared-tag search: got %v want %v", got2, want2)
	}
}

func TestSearch_BodySubstringMatch(t *testing.T) {
	b := writeSearchBundle(t)
	// "mouthfeel" appears in acidity's body AND auth's body.
	got := paths(Search(b, "mouthfeel", FieldBody))
	want := []string{"security/auth.md", "wine/acidity.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body search: got %v want %v", got, want)
	}
}

func TestSearch_FieldAnyMatchesAcrossSurfaces(t *testing.T) {
	b := writeSearchBundle(t)
	// "wine" matches the neighborhood-tag on two nodes AND is not in auth.
	got := paths(Search(b, "wine", FieldAny))
	want := []string{"wine/acidity.md", "wine/tannin.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("any search: got %v want %v", got, want)
	}
}

func TestSearch_FieldAnyReportsMatchedOn(t *testing.T) {
	b := writeSearchBundle(t)
	// "acidity" matches acidity.md on title, and tannin.md on body (link text).
	rs := Search(b, "acidity", FieldAny)
	byPath := map[string][]string{}
	for _, r := range rs {
		byPath[r.Path] = r.MatchedOn
	}
	if got := byPath["wine/acidity.md"]; !reflect.DeepEqual(got, []string{"body", "title"}) {
		t.Fatalf("acidity.md matched_on: got %v want [body title]", got)
	}
	if got := byPath["wine/tannin.md"]; !reflect.DeepEqual(got, []string{"body"}) {
		t.Fatalf("tannin.md matched_on: got %v want [body]", got)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	b := writeSearchBundle(t)
	if got := len(Search(b, "TANNIN", FieldTitle)); got != 1 {
		t.Fatalf("uppercase query should match: got %d results", got)
	}
}

func TestSearch_EmptyQueryReturnsNothing(t *testing.T) {
	b := writeSearchBundle(t)
	if got := Search(b, "   ", FieldAny); got != nil {
		t.Fatalf("empty query should return nil, got %v", got)
	}
}

func TestSearch_ReservedFilesNeverMatch(t *testing.T) {
	b := writeSearchBundle(t)
	// "index" is the reserved file's title/type but must never be a result.
	for _, r := range Search(b, "index", FieldAny) {
		if IsReservedPath(r.Path) {
			t.Fatalf("reserved file %s must not be a search result", r.Path)
		}
	}
}

func TestNeighborhood_Depth1(t *testing.T) {
	b := writeSearchBundle(t)
	got, ok := Neighborhood(b, "wine/tannin.md", 1)
	if !ok {
		t.Fatalf("known start node reported unknown")
	}
	if len(got) != 1 || got[0].Path != "wine/acidity.md" || got[0].Depth != 1 {
		t.Fatalf("depth-1 neighbors of tannin: got %+v want [acidity depth 1]", got)
	}
}

func TestNeighborhood_UndirectedEdge(t *testing.T) {
	b := writeSearchBundle(t)
	// acidity links to tannin AND tannin links to acidity; either direction
	// makes them neighbors. Start from acidity, expect tannin at depth 1.
	got, ok := Neighborhood(b, "wine/acidity.md", 1)
	if !ok {
		t.Fatalf("known start reported unknown")
	}
	if len(got) != 1 || got[0].Path != "wine/tannin.md" {
		t.Fatalf("acidity neighbors: got %+v want [tannin]", got)
	}
}

func TestNeighborhood_IsolatedNodeHasNoNeighbors(t *testing.T) {
	b := writeSearchBundle(t)
	got, ok := Neighborhood(b, "security/auth.md", 3)
	if !ok {
		t.Fatalf("known start reported unknown")
	}
	if len(got) != 0 {
		t.Fatalf("isolated node should have no neighbors, got %+v", got)
	}
}

func TestNeighborhood_DepthClampAndOrdering(t *testing.T) {
	b := writeSearchBundle(t)
	// depth < 1 clamps to 1.
	got, ok := Neighborhood(b, "wine/tannin.md", 0)
	if !ok || len(got) != 1 || got[0].Path != "wine/acidity.md" {
		t.Fatalf("depth 0 should clamp to 1: got %+v", got)
	}
}

func TestNeighborhood_UnknownStart(t *testing.T) {
	b := writeSearchBundle(t)
	if _, ok := Neighborhood(b, "does/not/exist.md", 1); ok {
		t.Fatalf("unknown start should return ok=false")
	}
}
