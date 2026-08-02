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
	"testing"
	"time"
)

// TestQuery_DecayControl_UnsetRankingUnchanged is the card's decay control: with
// --half-life unset (Decay nil), ranking is provably identical to a plain Query.
func TestQuery_DecayControl_UnsetRankingUnchanged(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	base, err := Query(s, e, "wine structure", 5)
	if err != nil {
		t.Fatal(err)
	}
	withOpts, err := QueryWith(s, e, "wine structure", 5, QueryOptions{
		Meta: metaFromBundle(b),
		// Decay deliberately nil.
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != len(withOpts) {
		t.Fatalf("decay-unset changed count: %d vs %d", len(base), len(withOpts))
	}
	for i := range base {
		if base[i].Path != withOpts[i].Path || base[i].Score != withOpts[i].Score {
			t.Errorf("decay-unset changed ranking at %d: %+v vs %+v", i, base[i], withOpts[i])
		}
	}
}

// TestQuery_DecayControl_FreshWeakNeverBeatsRelevantOld is THE load-bearing decay
// test the card demands: recency must NOT promote a sub-floor weak match above a
// strong older one. We construct:
//   - a STRONG but OLD node that the query hits hard (raw cosine well above floor)
//   - a WEAK but FRESH node whose raw cosine is below the relevance floor
//
// Even with an aggressive half-life that would otherwise let the fresh node win on
// the decayed product, the floor (applied to RAW cosine) must exclude the weak
// node so the relevant old node stays on top. "The order changed" is not the
// assertion; "the irrelevant-but-fresh node did NOT win" is.
func TestQuery_DecayControl_FreshWeakNeverBeatsRelevantOld(t *testing.T) {
	// strong node: many query tokens; weak node: essentially unrelated vocabulary.
	b, _ := writeBundle(t, map[string]string{
		"strong.md": node("Concept", "Tannin", "tannin structure astringency mouthfeel bitterness in wine tannin structure"),
		"weak.md":   node("Concept", "Zebra", "zebra savanna migration stripes herd grasslands unrelated"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	q := "tannin structure astringency mouthfeel"
	now := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)

	// First, learn the raw cosines so the test is grounded, not guessed.
	plain, err := Query(s, e, q, 10)
	if err != nil {
		t.Fatal(err)
	}
	var strongRaw, weakRaw float64
	for _, r := range plain {
		switch r.Path {
		case "strong.md":
			strongRaw = r.Score
		case "weak.md":
			weakRaw = r.Score
		}
	}
	if !(strongRaw > weakRaw) {
		t.Fatalf("test premise broken: strong raw %.4f should exceed weak raw %.4f", strongRaw, weakRaw)
	}
	// Floor sits ABOVE the weak node's raw cosine but below the strong node's, so
	// the weak node is sub-floor and must be excluded before decay reorders.
	floor := (strongRaw + weakRaw) / 2

	meta := map[string]NodeMeta{
		// strong node is OLD (1 year before now) — heavy decay penalty.
		"strong.md": {Type: "Concept", Generated: now.AddDate(-1, 0, 0), HasGenerated: true},
		// weak node is FRESH (today) — zero decay penalty.
		"weak.md": {Type: "Concept", Generated: now, HasGenerated: true},
	}

	res, err := QueryWith(s, e, q, 10, QueryOptions{
		Meta: meta,
		Decay: &DecayOptions{
			HalfLifeDays: 30, // aggressive: a 1-year-old node loses most of its score
			Now:          now,
			MinRelevance: floor,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("decay query returned nothing")
	}
	// The load-bearing assertion: the fresh-but-weak node must NOT be promoted
	// above the relevant old node. With the floor on raw cosine, the weak node is
	// excluded entirely; the strong node is the top (and only) survivor.
	if res[0].Path != "strong.md" {
		t.Fatalf("recency promoted a sub-floor fresh node: top = %q, want strong.md; res=%+v", res[0].Path, res)
	}
	for _, r := range res {
		if r.Path == "weak.md" {
			t.Errorf("sub-floor weak node %q survived the raw-cosine floor", r.Path)
		}
	}
}

// TestQuery_Decay_ReordersSurvivors: among nodes that BOTH clear the floor, decay
// reorders on the cosine×decay product — a fresher node with comparable relevance
// can outrank a slightly-more-relevant but much older one. This proves decay does
// something (not a no-op) while the floor keeps it honest.
func TestQuery_Decay_ReordersSurvivors(t *testing.T) {
	// Two nodes with near-identical relevance to the query.
	b, _ := writeBundle(t, map[string]string{
		"old.md":   node("Concept", "Tannin", "tannin structure astringency in wine"),
		"fresh.md": node("Concept", "Tannin", "tannin structure astringency in wine"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)

	q := "tannin structure astringency"
	now := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)

	meta := map[string]NodeMeta{
		"old.md":   {Type: "Concept", Generated: now.AddDate(-2, 0, 0), HasGenerated: true},
		"fresh.md": {Type: "Concept", Generated: now, HasGenerated: true},
	}
	res, err := QueryWith(s, e, q, 10, QueryOptions{
		Meta: meta,
		Decay: &DecayOptions{
			HalfLifeDays: 90,
			Now:          now,
			MinRelevance: 0, // both clear the (zero) floor
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) < 2 {
		t.Fatalf("want both nodes, got %+v", res)
	}
	if res[0].Path != "fresh.md" {
		t.Errorf("decay did not reorder survivors: top = %q, want fresh.md; res=%+v", res[0].Path, res)
	}
}

// TestQuery_Decay_UndatedNodeSurvivesFloorButGetsNoBoost: a node with no
// generated/timestamp (§13.1 fallback yields nothing) still ranks — decay simply
// applies no penalty (factor 1). Absence of a date is not a reason to drop a node.
func TestQuery_Decay_UndatedNodeSurvives(t *testing.T) {
	b, _ := writeBundle(t, map[string]string{
		"dated.md":   node("Concept", "Tannin", "tannin structure astringency wine"),
		"undated.md": node("Concept", "Tannin", "tannin structure astringency wine"),
	})
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)
	now := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	meta := map[string]NodeMeta{
		"dated.md":   {Type: "Concept", Generated: now.AddDate(-5, 0, 0), HasGenerated: true},
		"undated.md": {Type: "Concept", HasGenerated: false}, // no date at all
	}
	res, err := QueryWith(s, e, "tannin structure astringency", 10, QueryOptions{
		Meta: meta,
		Decay: &DecayOptions{
			HalfLifeDays: 30,
			Now:          now,
			MinRelevance: 0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var sawUndated bool
	for _, r := range res {
		if r.Path == "undated.md" {
			sawUndated = true
		}
	}
	if !sawUndated {
		t.Errorf("undated node was dropped by decay; absence of a date must not exclude a node. res=%+v", res)
	}
}
