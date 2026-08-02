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
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// metaFromBundle resolves the per-node metadata (type, tags) the filters key on
// from a live bundle — mirroring how the CLI builds it at query time. It lives in
// the test package because the CLI owns the real construction.
func metaFromBundle(b *okf.Bundle) map[string]NodeMeta {
	m := map[string]NodeMeta{}
	for path, n := range b.Nodes {
		m[path] = NodeMeta{Type: n.Type(), Tags: n.Tags()}
	}
	return m
}

// TestQuery_FilterControl_UnfilteredUnchanged is the card's filter control: an
// UNFILTERED query must return the identical result set and ranking as a query
// with no options at all. The filter path must be purely additive.
func TestQuery_FilterControl_UnfilteredUnchanged(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	base, err := Query(s, e, "wine structure", 5)
	if err != nil {
		t.Fatal(err)
	}
	// Same query, empty Filter, meta supplied but no constraints set.
	withOpts, err := QueryWith(s, e, "wine structure", 5, QueryOptions{Meta: metaFromBundle(b)})
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != len(withOpts) {
		t.Fatalf("result count changed: unfiltered=%d, empty-filter=%d", len(base), len(withOpts))
	}
	for i := range base {
		if base[i].Path != withOpts[i].Path || base[i].Score != withOpts[i].Score {
			t.Errorf("ranking changed at %d: unfiltered=%+v empty-filter=%+v", i, base[i], withOpts[i])
		}
	}
}

// TestQuery_Filter_PathPrefix is the card's positive control: --path <prefix>
// returns ONLY nodes under that prefix, asserted on every result path.
func TestQuery_Filter_PathPrefix(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"wine/tannin.md":  node("Concept", "Tannin", "Tannin gives structure and astringency to wine."),
		"wine/acidity.md": node("Concept", "Acidity", "Acidity gives freshness and lift to wine."),
		"coffee/roast.md": node("Concept", "Roast", "Roast level shapes acidity and body in coffee."),
		"coffee/grind.md": node("Concept", "Grind", "Grind size controls extraction in coffee."),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "acidity structure", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{PathPrefix: "wine/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("path filter returned nothing; expected the wine/ nodes")
	}
	for _, r := range res {
		if !strings.HasPrefix(r.Path, "wine/") {
			t.Errorf("result %q is not under wine/; path filter leaked", r.Path)
		}
	}
}

// TestQuery_Filter_Type composes a type filter (AND). Only nodes whose type
// matches survive.
func TestQuery_Filter_Type(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"a.md": node("Concept", "Alpha", "alpha content about wine and structure"),
		"b.md": node("Playbook", "Beta", "beta content about wine and structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "wine structure", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{Type: "Playbook"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != "b.md" {
		t.Fatalf("type filter: want only b.md, got %+v", res)
	}
}

// TestQuery_Filter_Tag composes a tag filter (AND, single value for v1).
func TestQuery_Filter_Tag(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"a.md": "---\ntype: Concept\ntitle: Alpha\ntags: [red, structure]\n---\n\n# Alpha\n\nalpha about wine\n",
		"b.md": "---\ntype: Concept\ntitle: Beta\ntags: [white]\n---\n\n# Beta\n\nbeta about wine\n",
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "wine", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{Tag: "red"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != "a.md" {
		t.Fatalf("tag filter: want only a.md (tag red), got %+v", res)
	}
}

// TestQuery_Filter_ZeroMatchIsEmptyNotError is the card's NEGATIVE control: a
// type/tag filter matching zero nodes returns an explicit empty result — not an
// error, and never a silent fall-back to the unfiltered set.
func TestQuery_Filter_ZeroMatchIsEmptyNotError(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "wine", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{Type: "NoSuchType"},
	})
	if err != nil {
		t.Fatalf("zero-match filter must not error, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("zero-match filter must return empty, got %d results (silent unfiltered fall-back?): %+v", len(res), res)
	}
}

// TestQuery_Filter_AppliesToPasslessFallback: filters must apply even when the
// store has no passage layer (legacy index answered off Entries) — the filter
// cannot silently stop applying on the entries path.
func TestQuery_Filter_AppliesToPasslessFallback(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"wine/tannin.md":  node("Concept", "Tannin", "Tannin gives structure to wine."),
		"coffee/roast.md": node("Concept", "Roast", "Roast shapes coffee body."),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)
	s.Passages = nil // legacy passage-less index

	res, err := QueryWith(s, e, "wine", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{PathPrefix: "wine/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if !strings.HasPrefix(r.Path, "wine/") {
			t.Errorf("passless fallback leaked non-wine result %q", r.Path)
		}
	}
	if len(res) == 0 {
		t.Fatal("passless fallback with path filter returned nothing; expected wine/tannin.md")
	}
}
