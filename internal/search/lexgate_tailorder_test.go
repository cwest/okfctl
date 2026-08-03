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

// The tests in this file pin cwest/okfctl#73: applyLexicalGate's step-3
// preserved lexical tail must be ordered by semantic SCORE (descending), not by
// path. The caller cuts to k AFTER the gate composes, so a path-ordered tail lets
// path order — not score — decide which lexical hits survive a small k. That is a
// correctness defect, not a presentation choice: at k=2 an alphabetic prefix
// survived while a hit with 4x the score was dropped.
//
// These exercise applyLexicalGate directly (rather than through QueryWith) so the
// tail order and the survivor SET are asserted on the exact Result slice the
// caller cuts, independent of any embedder's cosine ordering.
//
// Fixture control note: applyLexicalGate widens the semantic band to
// wide = max(WideN, k) (clamped to len(ranked)). To keep the tail nodes genuinely
// OUT of the band (so they exercise the step-3 tail, not the step-2
// intersection), every test below passes k <= WideN and sizes the non-matching
// band fillers to exactly fill that band. The tail then holds all the lexical
// matches, which is the #73 repro shape (band ∩ Match empty).

// tailFixtureRanked models the #73 repro shape: the whole result is step-3 tail
// (band ∩ Match is empty because the three high-scoring band fillers are not
// lexical matches). Each matched node carries its real semantic score, and PATH
// order is the inverse of SCORE order — so a path-ordered tail is observably
// wrong.
//
// Scores mirror the card's synthetic repro exactly:
//
//	node300  0.0008   (weakest)
//	node301  0.0024
//	node302  0.0097   (strongest — but alphabetically third)
//	node303  0.0012
//
// WideN=3 with exactly three non-matching fillers means any call with k<=3 leaves
// all four node30x entries in the tail.
func tailFixtureRanked() ([]Result, *LexicalGateOptions) {
	ranked := []Result{
		// High-scoring semantic band fillers — none are lexical matches.
		{Path: "band/b0.md", Score: 0.90},
		{Path: "band/b1.md", Score: 0.80},
		{Path: "band/b2.md", Score: 0.70},
		// The lexical-tail nodes, in path order, with non-monotonic scores.
		{Path: "notes/node300.md", Score: 0.0008},
		{Path: "notes/node301.md", Score: 0.0024},
		{Path: "notes/node302.md", Score: 0.0097},
		{Path: "notes/node303.md", Score: 0.0012},
	}
	g := &LexicalGateOptions{
		Terms: []string{"glyph"},
		Match: map[string]bool{
			"notes/node300.md": true,
			"notes/node301.md": true,
			"notes/node302.md": true,
			"notes/node303.md": true,
		},
		OverBroadFraction: 0.6,
		TotalNodes:        len(ranked),
		WideN:             3, // band is exactly the three non-matching fillers
	}
	return ranked, g
}

// TestGate_TailCutByScoreKeepsStrongest is the #73 POSITIVE acceptance case: at
// k=2 the two SURVIVORS must be the two highest-scoring tail entries (node302 at
// 0.0097 and node301 at 0.0024), NOT the alphabetically-first two. Asserts on the
// SET the caller keeps after the k-cut — survivor SELECTION is the defect, so
// order alone is not enough.
func TestGate_TailCutByScoreKeepsStrongest(t *testing.T) {
	ranked, g := tailFixtureRanked()
	k := 2 // wide = max(WideN=3, k=2) = 3 -> tail holds all four node30x
	out := applyLexicalGate(ranked, g, k)
	if len(out) > k {
		out = out[:k] // mirror the caller's cut (query.go top-k)
	}
	got := map[string]float64{}
	for _, r := range out {
		got[r.Path] = r.Score
	}
	want := map[string]float64{
		"notes/node302.md": 0.0097,
		"notes/node301.md": 0.0024,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("k=2 must keep the two highest-scoring tail entries;\n got  %v\n want %v", got, want)
	}
}

// TestGate_TailFullyScoreDescending is the #73 POSITIVE order check: the full
// preserved tail is ordered strictly by score descending — 302, 301, 303, 300.
// k=3 keeps wide at the three fillers, so the returned list is exactly the tail.
func TestGate_TailFullyScoreDescending(t *testing.T) {
	ranked, g := tailFixtureRanked()
	out := applyLexicalGate(ranked, g, 3)
	got := resultPaths(out)
	want := []string{
		"notes/node302.md", // 0.0097
		"notes/node301.md", // 0.0024
		"notes/node303.md", // 0.0012
		"notes/node300.md", // 0.0008
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("full tail must be score-descending;\n got  %v\n want %v", got, want)
	}
	// And the scores themselves must be monotonically non-increasing.
	for i := 1; i < len(out); i++ {
		if out[i].Score > out[i-1].Score {
			t.Errorf("tail not monotonically non-increasing at %d: %.4f > %.4f",
				i, out[i].Score, out[i-1].Score)
		}
	}
}

// TestGate_IntersectionStaysSemanticOrder is the load-bearing NEGATIVE control
// (#73 done-when 3): when band ∩ Match is NON-empty, the intersection keeps its
// SEMANTIC order unchanged — the fix must reorder only the step-3 tail, never the
// step-2 intersection. Two matched nodes fall inside the band; their scores are
// set so semantic order (zeta before alpha) is the order that must survive.
func TestGate_IntersectionStaysSemanticOrder(t *testing.T) {
	ranked := []Result{
		// Both are lexical matches AND inside the WideN band.
		// Semantic order: zeta before alpha (zeta scores higher).
		{Path: "band/zeta.md", Score: 0.90},
		{Path: "band/alpha.md", Score: 0.85},
		// A non-matching band filler and a tail match below the band.
		{Path: "band/filler.md", Score: 0.80},
		{Path: "tail/t0.md", Score: 0.10},
	}
	g := &LexicalGateOptions{
		Terms: []string{"glyph"},
		Match: map[string]bool{
			"band/zeta.md":  true,
			"band/alpha.md": true,
			"tail/t0.md":    true,
		},
		OverBroadFraction: 0.9,
		TotalNodes:        len(ranked),
		WideN:             3,
	}
	out := applyLexicalGate(ranked, g, 3) // wide=3 -> band is the first three
	got := resultPaths(out)
	// Intersection first, in SEMANTIC order (zeta, alpha), then the tail (t0).
	want := []string{"band/zeta.md", "band/alpha.md", "tail/t0.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("intersection must stay in semantic order, tail appended;\n got  %v\n want %v", got, want)
	}
}

// TestGate_TailTieBreaksByPath is the #73 TIE DETERMINISM control: two tail
// entries with EQUAL scores order by path, stable across repeated runs. Go's map
// iteration is randomized, so a naive score-only sort would be non-deterministic
// on ties; the fix must break ties by path. WideN=1 with a single non-matching
// filler and k=1 keeps wide at 1, so all three equal-scored matches are tail.
func TestGate_TailTieBreaksByPath(t *testing.T) {
	ranked := []Result{
		{Path: "band/b0.md", Score: 0.90}, // non-matching filler, the whole band
		// Equal-scored tail entries in NON-path order in the ranked slice.
		{Path: "notes/zzz.md", Score: 0.05},
		{Path: "notes/aaa.md", Score: 0.05},
		{Path: "notes/mmm.md", Score: 0.05},
	}
	g := &LexicalGateOptions{
		Terms: []string{"glyph"},
		Match: map[string]bool{
			"notes/zzz.md": true,
			"notes/aaa.md": true,
			"notes/mmm.md": true,
		},
		OverBroadFraction: 0.9,
		TotalNodes:        len(ranked),
		WideN:             1,
	}
	want := []string{"notes/aaa.md", "notes/mmm.md", "notes/zzz.md"}
	// Repeat to defeat map-iteration randomization: same order every run.
	for iter := 0; iter < 20; iter++ {
		got := resultPaths(applyLexicalGate(ranked, g, 1))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("equal-scored tail must order by path deterministically (iter %d);\n got  %v\n want %v", iter, got, want)
		}
	}
}

// TestGate_DegradeReturnsRankedUnchanged is the #73 NEGATIVE control (done-when
// 5): a degraded gate — empty terms, or an over-broad match — returns ranked
// unchanged, untouched by the new tail sort.
func TestGate_DegradeReturnsRankedUnchanged(t *testing.T) {
	ranked := []Result{
		{Path: "a.md", Score: 0.9},
		{Path: "b.md", Score: 0.8},
		{Path: "c.md", Score: 0.7},
	}
	// Empty terms -> degrade.
	empty := &LexicalGateOptions{Terms: nil, Match: map[string]bool{"a.md": true}, WideN: 50}
	if got := applyLexicalGate(ranked, empty, 5); !reflect.DeepEqual(got, ranked) {
		t.Errorf("empty-term gate must return ranked unchanged; got %v", resultPaths(got))
	}
	// Over-broad match -> degrade (match covers > 0.6 of 3 nodes).
	broad := &LexicalGateOptions{
		Terms:             []string{"agent"},
		Match:             map[string]bool{"a.md": true, "b.md": true, "c.md": true},
		OverBroadFraction: 0.6,
		TotalNodes:        3,
		WideN:             50,
	}
	if got := applyLexicalGate(ranked, broad, 5); !reflect.DeepEqual(got, ranked) {
		t.Errorf("over-broad gate must return ranked unchanged; got %v", resultPaths(got))
	}
}
