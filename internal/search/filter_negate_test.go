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
)

// TestFilter_IsEmpty_AccountsForNegativeSets is the card's called-out easy bug:
// IsEmpty MUST report false when only a negative set is populated, or the CLI's
// needBundle short-circuits, the metadata walk is skipped, and every --not-*
// filter silently no-ops. A negative-only filter is a real constraint.
func TestFilter_IsEmpty_AccountsForNegativeSets(t *testing.T) {
	cases := []struct {
		name string
		f    Filter
		want bool
	}{
		{"all empty", Filter{}, true},
		{"positive path only", Filter{PathPrefixes: []string{"wine/"}}, false},
		{"positive type only", Filter{Types: []string{"Concept"}}, false},
		{"positive tag only", Filter{Tags: []string{"red"}}, false},
		{"negative path only", Filter{NotPathPrefixes: []string{"research/"}}, false},
		{"negative type only", Filter{NotTypes: []string{"Playbook"}}, false},
		{"negative tag only", Filter{NotTags: []string{"draft"}}, false},
		{"empty slices are empty", Filter{PathPrefixes: []string{}, NotTags: []string{}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.f.IsEmpty(); got != tc.want {
				t.Fatalf("IsEmpty() = %v, want %v (negative-only filter must not read as empty)", got, tc.want)
			}
		})
	}
}

// TestQuery_Filter_PathPrefix_OR is the OR-within-dimension acceptance criterion:
// --path design/ --path method/ returns a union drawn from BOTH roots, and
// strictly more results than either prefix alone.
func TestQuery_Filter_PathPrefix_OR(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"design/a.md":   node("Concept", "DA", "design alpha about structure"),
		"design/b.md":   node("Concept", "DB", "design beta about structure"),
		"method/c.md":   node("Concept", "MC", "method gamma about structure"),
		"research/d.md": node("Concept", "RD", "research delta about structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	only := func(prefixes ...string) []Result {
		res, err := QueryWith(s, e, "structure", 10, QueryOptions{
			Meta:   metaFromBundle(b),
			Filter: Filter{PathPrefixes: prefixes},
		})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	design := only("design/")
	method := only("method/")
	union := only("design/", "method/")

	// Union draws from both roots and excludes research/.
	sawDesign, sawMethod := false, false
	for _, r := range union {
		if strings.HasPrefix(r.Path, "research/") {
			t.Errorf("OR union leaked a research/ node: %q", r.Path)
		}
		if strings.HasPrefix(r.Path, "design/") {
			sawDesign = true
		}
		if strings.HasPrefix(r.Path, "method/") {
			sawMethod = true
		}
	}
	if !sawDesign || !sawMethod {
		t.Fatalf("OR union missing a root: sawDesign=%v sawMethod=%v; got %+v", sawDesign, sawMethod, union)
	}
	if len(union) <= len(design) || len(union) <= len(method) {
		t.Fatalf("OR union (%d) must be strictly larger than either alone (design=%d, method=%d)",
			len(union), len(design), len(method))
	}
}

// TestQuery_Filter_Type_OR: repeats of --type compose with OR.
func TestQuery_Filter_Type_OR(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"a.md": node("Concept", "Alpha", "alpha about wine structure"),
		"b.md": node("Playbook", "Beta", "beta about wine structure"),
		"c.md": node("Pattern", "Gamma", "gamma about wine structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "wine structure", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{Types: []string{"Concept", "Playbook"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range res {
		got[r.Path] = true
	}
	if !got["a.md"] || !got["b.md"] {
		t.Fatalf("type OR must include a.md (Concept) and b.md (Playbook), got %+v", res)
	}
	if got["c.md"] {
		t.Fatalf("type OR must exclude c.md (Pattern), got %+v", res)
	}
}

// TestQuery_Filter_NotPath is the card's headline negation: --not-path research/
// returns a non-empty set containing ZERO research/ nodes, while the unfiltered
// query includes them. Empty positive set still means "all nodes".
func TestQuery_Filter_NotPath(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"research/a.md": node("Concept", "RA", "research alpha about structure"),
		"research/b.md": node("Concept", "RB", "research beta about structure"),
		"design/c.md":   node("Concept", "DC", "design gamma about structure"),
		"method/d.md":   node("Concept", "MD", "method delta about structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	unfiltered, err := QueryWith(s, e, "structure", 10, QueryOptions{Meta: metaFromBundle(b)})
	if err != nil {
		t.Fatal(err)
	}
	sawResearchUnfiltered := false
	for _, r := range unfiltered {
		if strings.HasPrefix(r.Path, "research/") {
			sawResearchUnfiltered = true
		}
	}
	if !sawResearchUnfiltered {
		t.Fatal("precondition failed: unfiltered query should surface research/ nodes")
	}

	res, err := QueryWith(s, e, "structure", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{NotPathPrefixes: []string{"research/"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("--not-path research/ returned nothing; expected design/ and method/ nodes")
	}
	for _, r := range res {
		if strings.HasPrefix(r.Path, "research/") {
			t.Errorf("--not-path research/ leaked a research node: %q", r.Path)
		}
	}
}

// TestQuery_Filter_NotType and NotTag exclude by the other two dimensions.
func TestQuery_Filter_NotType(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"a.md": node("Concept", "Alpha", "alpha about wine structure"),
		"b.md": node("Playbook", "Beta", "beta about wine structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "wine structure", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{NotTypes: []string{"Playbook"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != "a.md" {
		t.Fatalf("--not-type Playbook: want only a.md, got %+v", res)
	}
}

func TestQuery_Filter_NotTag(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"a.md": "---\ntype: Concept\ntitle: Alpha\ntags: [red]\n---\n\n# Alpha\n\nalpha about wine\n",
		"b.md": "---\ntype: Concept\ntitle: Beta\ntags: [white]\n---\n\n# Beta\n\nbeta about wine\n",
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "wine", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{NotTags: []string{"red"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != "b.md" {
		t.Fatalf("--not-tag red: want only b.md (tag white), got %+v", res)
	}
}

// TestQuery_Filter_ExclusionBeatsInclusion pins the specified positive-then-
// exclude order: --path research/ --not-path research/agents/ keeps research/
// nodes EXCEPT those under research/agents/. Exclusion is applied after the
// positive set, so the narrower negative wins inside the positive scope.
func TestQuery_Filter_ExclusionBeatsInclusion(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"research/core.md":     node("Concept", "Core", "research core about structure"),
		"research/agents/a.md": node("Concept", "AgentA", "research agent alpha about structure"),
		"research/agents/b.md": node("Concept", "AgentB", "research agent beta about structure"),
		"design/x.md":          node("Concept", "X", "design about structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "structure", 10, QueryOptions{
		Meta: metaFromBundle(b),
		Filter: Filter{
			PathPrefixes:    []string{"research/"},
			NotPathPrefixes: []string{"research/agents/"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != "research/core.md" {
		t.Fatalf("exclusion-beats-inclusion: want only research/core.md, got %+v", res)
	}
}

// TestQuery_Filter_AND_AcrossDimensionsPreserved: repeatable positive dimensions
// still AND across dimensions. --path research/ --type Concept intersects.
func TestQuery_Filter_AND_AcrossDimensionsPreserved(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"research/a.md": node("Concept", "RA", "research alpha about structure"),
		"research/b.md": node("Playbook", "RB", "research beta about structure"),
		"design/c.md":   node("Concept", "DC", "design gamma about structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "structure", 10, QueryOptions{
		Meta: metaFromBundle(b),
		Filter: Filter{
			PathPrefixes: []string{"research/"},
			Types:        []string{"Concept"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Path != "research/a.md" {
		t.Fatalf("AND across dimensions: want only research/a.md (research/ ∩ Concept), got %+v", res)
	}
}

// TestQuery_Filter_NotPathCoveringEveryRoot: a negative filter covering every
// node returns zero results with no error (empty is not a failure).
func TestQuery_Filter_NotPathCoveringEveryRoot(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"wine/a.md":   node("Concept", "A", "alpha about structure"),
		"coffee/b.md": node("Concept", "B", "beta about structure"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	res, err := QueryWith(s, e, "structure", 10, QueryOptions{
		Meta:   metaFromBundle(b),
		Filter: Filter{NotPathPrefixes: []string{"wine/", "coffee/"}},
	})
	if err != nil {
		t.Fatalf("negative filter covering every root must not error, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("negative filter covering every root must return empty, got %+v", res)
	}
}
