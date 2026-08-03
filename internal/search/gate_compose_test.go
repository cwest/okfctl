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
	"reflect"
	"testing"
)

// gateFixtureStore builds a small in-memory store with the hash embedder so the
// gate-composition tests run without a real model. Each node's vector is the
// hash embedding of its text, so cosine ordering is deterministic and the same
// text drives both the semantic vector and the lexical match set.
func gateFixtureStore(t *testing.T, texts map[string]string) (*Store, Embedder) {
	t.Helper()
	e := NewHashEmbedder()
	paths := make([]string, 0, len(texts))
	for p := range texts {
		paths = append(paths, p)
	}
	// deterministic order
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if paths[j] < paths[i] {
				paths[i], paths[j] = paths[j], paths[i]
			}
		}
	}
	entries := make([]Entry, 0, len(paths))
	toEmbed := make([]string, 0, len(paths))
	for _, p := range paths {
		toEmbed = append(toEmbed, texts[p])
	}
	vecs := e.Encode(toEmbed)
	for i, p := range paths {
		entries = append(entries, Entry{Path: p, Vector: vecs[i]})
	}
	return &Store{Model: e.Name(), Dim: e.Dim(), Entries: entries}, e
}

// paths extracts just the ordered paths from a result slice for comparison.
func resultPaths(res []Result) []string {
	out := make([]string, len(res))
	for i, r := range res {
		out[i] = r.Path
	}
	return out
}

// TestGate_ExactTokenPromotedToRankOne is the POSITIVE acceptance case: a gold
// node that the semantic ranker BURIES (a near-miss decoy scores higher) but that
// is the sole lexical match is promoted to rank 1 by the gate's intersection.
// Vectors are hand-set so the semantic ordering is controlled independently of
// the (explicitly-passed) lexical match set — the exact "semantic top-1 is a
// near-miss" shape the acceptance criterion names.
func TestGate_ExactTokenPromotedToRankOne(t *testing.T) {
	e := NewHashEmbedder()
	// Query vector: [1,0,...]. decoy.md is nearly parallel (rank 1 semantically);
	// gold.md is a weaker semantic match (rank 2+); other.md is orthogonal.
	dim := e.Dim()
	mk := func(v0, v1 float64) []float64 {
		v := make([]float64, dim)
		v[0], v[1] = v0, v1
		return v
	}
	s := &Store{
		Model: e.Name(), Dim: dim,
		Entries: []Entry{
			{Path: "decoy.md", Vector: mk(1.0, 0.05)}, // closest to query -> semantic rank 1
			{Path: "gold.md", Vector: mk(0.6, 0.8)},   // weaker semantic match
			{Path: "other.md", Vector: mk(0.0, 1.0)},  // orthogonal
		},
	}
	// The query encodes to some vector; we don't rely on its exact direction for
	// the lexical set — Match is passed explicitly. But we DO need gold buried
	// semantically, which the hand-set vectors above guarantee against ANY query
	// vector whose dominant component is dim 0 (the hash embedder is stable per
	// input, so we assert the precondition below rather than assume it).
	q := "errevict"
	terms := LexTerms(q)
	// Only gold is a lexical match (passed explicitly — the whole point of the
	// gate is to inject the lexical signal the semantic ranker lacks).
	match := map[string]bool{"gold.md": true}

	off, err := QueryWith(s, e, q, 5, QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: gold is NOT already the semantic top-1, so a passing gate is
	// genuinely doing the promotion (not a vacuous no-op).
	if len(off) == 0 || off[0].Path == "gold.md" {
		t.Fatalf("precondition: gold must be semantically buried; off=%v", resultPaths(off))
	}
	on, err := QueryWith(s, e, q, 5, QueryOptions{
		LexicalGate: &LexicalGateOptions{
			Terms: terms, Match: match, OverBroadFraction: 0.6, TotalNodes: len(s.Entries), WideN: 50,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(on) == 0 || on[0].Path != "gold.md" {
		t.Errorf("gate ON should rank gold.md first; got %v (off=%v)", resultPaths(on), resultPaths(off))
	}
}

// TestGate_OffByteIdenticalToQuery is the SECOND-NEGATIVE control: with no gate
// options the pipeline is byte-identical to a plain QueryWith. This is the
// default path — it must never change.
func TestGate_OffByteIdenticalToQuery(t *testing.T) {
	texts := map[string]string{
		"a.md": "wine tannin structure astringency",
		"b.md": "coffee roast acidity body",
		"c.md": "hashing content keys the reembed decision",
	}
	s, e := gateFixtureStore(t, texts)
	q := "wine structure"
	base, err := QueryWith(s, e, q, 5, QueryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	off, err := QueryWith(s, e, q, 5, QueryOptions{LexicalGate: nil})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(base, off) {
		t.Errorf("nil gate changed output:\n base=%v\n off =%v", base, off)
	}
}

// TestGate_EmptyTermsDegrades pins the empty-term degrade: an all-stopword query
// with the gate ON returns exactly what the gate OFF returns — a no-op, not a
// filter that empties the list.
func TestGate_EmptyTermsDegrades(t *testing.T) {
	texts := map[string]string{
		"a.md": "wine tannin structure",
		"b.md": "coffee roast acidity",
	}
	s, e := gateFixtureStore(t, texts)
	q := "how should the"
	terms := LexTerms(q) // empty
	off, _ := QueryWith(s, e, q, 5, QueryOptions{})
	on, err := QueryWith(s, e, q, 5, QueryOptions{
		LexicalGate: &LexicalGateOptions{Terms: terms, Match: LexicalMatchSet(texts, terms), OverBroadFraction: 0.6, TotalNodes: len(texts), WideN: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(off, on) {
		t.Errorf("empty-term gate must be a no-op:\n off=%v\n on =%v", resultPaths(off), resultPaths(on))
	}
}

// TestGate_OverBroadDegrades pins the over-broad degrade: a term matching more
// than OverBroadFraction of the bundle makes the gate a no-op. Every node here
// contains "agent", so the match set is 100% of the bundle — well over the
// fraction — and the gate must not reorder.
func TestGate_OverBroadDegrades(t *testing.T) {
	texts := map[string]string{
		"a.md": "agent delegation model for wine",
		"b.md": "agent orchestration for coffee",
		"c.md": "agent handoff protocol overview",
	}
	s, e := gateFixtureStore(t, texts)
	q := "agent"
	terms := LexTerms(q)
	match := LexicalMatchSet(texts, terms) // matches all 3
	if len(match) != 3 {
		t.Fatalf("precondition: expected all 3 nodes to match 'agent'; got %v", match)
	}
	off, _ := QueryWith(s, e, q, 5, QueryOptions{})
	on, err := QueryWith(s, e, q, 5, QueryOptions{
		LexicalGate: &LexicalGateOptions{Terms: terms, Match: match, OverBroadFraction: 0.6, TotalNodes: len(texts), WideN: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(off, on) {
		t.Errorf("over-broad gate must be a no-op:\n off=%v\n on =%v", resultPaths(off), resultPaths(on))
	}
}

// TestGate_AppendsLexicalTail is the load-bearing step-4 control: a lexical hit
// that falls OUTSIDE the semantic band must be APPENDED, not dropped. With a
// narrow WideN=1 the semantic band holds one node; the other lexical match must
// still appear in the gated result (a pure intersection would drop it).
func TestGate_AppendsLexicalTail(t *testing.T) {
	texts := map[string]string{
		"strong.md": "delegate delegate delegate work assignment to a subagent",
		"weak.md":   "a single mention of delegate buried in unrelated prose about wine tannin acidity roast",
		"other.md":  "completely unrelated coffee content with no shared terms",
	}
	s, e := gateFixtureStore(t, texts)
	q := "delegate"
	terms := LexTerms(q)
	match := LexicalMatchSet(texts, terms) // strong.md + weak.md
	on, err := QueryWith(s, e, q, 5, QueryOptions{
		LexicalGate: &LexicalGateOptions{Terms: terms, Match: match, OverBroadFraction: 0.9, TotalNodes: len(texts), WideN: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := resultPaths(on)
	// Both lexical matches must be present; the non-matching node must not be.
	seen := map[string]bool{}
	for _, p := range got {
		seen[p] = true
	}
	if !seen["strong.md"] || !seen["weak.md"] {
		t.Errorf("step-4 tail: both lexical hits must survive; got %v", got)
	}
	if seen["other.md"] {
		t.Errorf("non-matching node leaked into gated result; got %v", got)
	}
}

// TestGate_ZeroLexicalMatchIsEmpty documents a DELIBERATE decision: when the
// query has real content terms but NONE match any node, the gate returns an empty
// result (empty intersection + empty tail), NOT the ungated semantic list. This
// is consistent with core lexical Search returning nothing for a no-hit term —
// the user asked to gate lexically and nothing matched lexically. It is distinct
// from the empty-TERM degrade (all-stopword), which IS a no-op. Surfacing it here
// rather than resolving it silently in code, per AGENTS.md.
func TestGate_ZeroLexicalMatchIsEmpty(t *testing.T) {
	texts := map[string]string{
		"a.md": "wine tannin structure",
		"b.md": "coffee roast acidity",
	}
	s, e := gateFixtureStore(t, texts)
	q := "kubernetes" // a real content term matching no node
	terms := LexTerms(q)
	if len(terms) == 0 {
		t.Fatalf("precondition: %q must yield a content term", q)
	}
	match := LexicalMatchSet(texts, terms) // empty
	on, err := QueryWith(s, e, q, 5, QueryOptions{
		LexicalGate: &LexicalGateOptions{Terms: terms, Match: match, OverBroadFraction: 0.6, TotalNodes: len(texts), WideN: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(on) != 0 {
		t.Errorf("zero lexical match must gate to empty; got %v", resultPaths(on))
	}
}
