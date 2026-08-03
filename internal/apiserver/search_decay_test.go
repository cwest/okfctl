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

package apiserver

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cwest/okfctl/internal/search"
)

// decayBundleDir lays down the issue's recency-decay repro as a bundle the API
// serves: a strong OLD exact match and a weak FRESH near-noise node sharing
// incidental query words, plus a mid-age node so a floored multiplier has three
// distinct ages to act on. This is the same shape the CLI decay tests use
// (cmd/okfctl-search/main_test.go writeDecayReproBundle) so the HTTP surface and
// the CLI are exercised on comparable data. The index is NOT built; callers that
// need one call buildIndex.
func decayBundleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".okf": "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n" +
			"- [Old](docs/old-exact.md)\n- [Mid](docs/mid.md)\n- [Fresh](docs/fresh-weak.md)\n",
		"docs/old-exact.md":  "---\ntype: Concept\ntitle: Bloom Filter Sizing\ngenerated:\n  by: synthetic\n  at: 2023-01-01T00:00:00Z\n---\n\n# Bloom Filter Sizing\n\n## Bloom Filter False Positive Bits Hashes Sizing\n\nBloom filter sizing bits hashes false positive rate bloom filter sizing bits\nhashes false positive rate bloom filter sizing bits hashes false positive rate.\n",
		"docs/mid.md":        "---\ntype: Concept\ntitle: Bloom Filter Notes\ngenerated:\n  by: synthetic\n  at: 2025-06-01T00:00:00Z\n---\n\n# Bloom Filter Notes\n\n## Bloom Filter Sizing Bits Hashes\n\nBloom filter sizing bits hashes false positive rate notes and observations on\nbloom filter sizing bits hashes false positive rate collected over time.\n",
		"docs/fresh-weak.md": "---\ntype: Concept\ntitle: Weekly Status Roundup\ngenerated:\n  by: synthetic\n  at: 2026-08-01T00:00:00Z\n---\n\n# Weekly Status Roundup\n\n## Agenda Recap\n\nFilter positive agenda recap status roundup attendees followup parked owner\naction pending review draft revision comment summary snapshot digest triage\nboard column swimlane retro sprint cadence checkpoint readout brief rollup.\n",
	}
	writeFiles(t, dir, files)
	return dir
}

const decayReproQuery = "bloom filter sizing false positive bits hashes"

// fixedNow pins the package decay clock to a deterministic instant for the span
// of a test and returns that instant, so a cross-surface equivalence assertion
// feeds the SAME Now into both the endpoint (via newSearchService capturing the
// package now) and the CLI-equivalent oracle, yielding byte-identical decayed
// scores. It restores the real clock on cleanup. The handler must be built AFTER
// this call so newSearchService captures the pinned clock.
func fixedNow(t *testing.T) time.Time {
	t.Helper()
	inst := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	orig := now
	now = func() time.Time { return inst }
	t.Cleanup(func() { now = orig })
	return inst
}

// qesc URL-encodes a query value.
func qesc(s string) string { return url.QueryEscape(s) }

// ftoa formats a float for a query param, the shortest exact representation.
func ftoa(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

// mustSearch runs a GET, asserts the status code, and (for 200) decodes the body.
func mustSearch(t *testing.T, h http.Handler, target string, wantCode int) searchResponse {
	t.Helper()
	rec := doSearch(t, h, target)
	if rec.Code != wantCode {
		t.Fatalf("GET %s = %d, want %d (body=%s)", target, rec.Code, wantCode, rec.Body.String())
	}
	var got searchResponse
	if wantCode == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("body not valid JSON: %v\n%s", err, rec.Body.String())
		}
	}
	return got
}

// TestSearch_DecayFloorDefault_KeepsStrongOldAboveFreshWeak is the card's
// criterion 1 on the HTTP surface: with half_life set and NO decay_floor param,
// the endpoint must apply the SAME default floor the CLI applies (0.25), so the
// strong old match ranks above the weak fresh one and is not crushed to ~0. Today
// the endpoint leaves DecayFloor at 0 (unbounded), so this fails.
func TestSearch_DecayFloorDefault_KeepsStrongOldAboveFreshWeak(t *testing.T) {
	dir := decayBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	got := mustSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5&half_life=90", http.StatusOK)
	if len(got.Results) < 2 {
		t.Fatalf("want at least 2 results, got %d: %+v", len(got.Results), got.Results)
	}
	if got.Results[0].Path != "docs/old-exact.md" {
		t.Fatalf("default decay floor must keep old-exact.md on top at half_life=90; got order %+v", got.Results)
	}
	// The old match must survive with a real score, not a floored-to-zero ghost.
	for _, r := range got.Results {
		if r.Path == "docs/old-exact.md" && r.Score <= 0 {
			t.Fatalf("old-exact.md scored %v with the default clamp; must stay > 0", r.Score)
		}
	}
}

// TestSearch_DecayFloorDefault_EquivalentToCLIDefault proves the endpoint's
// defaulted floor produces the EXACT scores the CLI produces with its default
// (--decay-floor 0.25). The oracle is cliEquivalent with the shared
// search.DefaultDecayFloor — one mental model, proven not asserted.
func TestSearch_DecayFloorDefault_EquivalentToCLIDefault(t *testing.T) {
	dir := decayBundleDir(t)
	now := fixedNow(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	got := mustSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5&half_life=90", http.StatusOK)
	want := cliEquivalent(t, dir, e, decayReproQuery, 5, search.QueryOptions{
		Meta: liveMeta(t, dir),
		Decay: &search.DecayOptions{
			HalfLifeDays: 90,
			Now:          now,
			MinRelevance: 0,
			DecayFloor:   search.DefaultDecayFloor,
		},
	})
	assertSameRanking(t, got.Results, want)
}

// TestSearch_DecayFloorZero_RestoresUnboundedDecay is criterion 2: decay_floor=0
// opts out to today's unbounded behavior. The weak fresh node overtakes the
// strong old one exactly as the CLI's --decay-floor 0 does.
func TestSearch_DecayFloorZero_RestoresUnboundedDecay(t *testing.T) {
	dir := decayBundleDir(t)
	now := fixedNow(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	got := mustSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5&half_life=90&decay_floor=0", http.StatusOK)
	if len(got.Results) < 2 || got.Results[0].Path != "docs/fresh-weak.md" {
		t.Fatalf("decay_floor=0 must restore the unbounded inversion (fresh-weak.md on top); got %+v", got.Results)
	}
	want := cliEquivalent(t, dir, e, decayReproQuery, 5, search.QueryOptions{
		Meta: liveMeta(t, dir),
		Decay: &search.DecayOptions{
			HalfLifeDays: 90,
			Now:          now,
			MinRelevance: 0,
			DecayFloor:   0,
		},
	})
	assertSameRanking(t, got.Results, want)
}

// TestSearch_DecayFloorOutOfRange400 is criterion 3: decay_floor outside [0,1]
// is a 400, not a silently-ignored 200. Matches the CLI's #72 wording.
func TestSearch_DecayFloorOutOfRange400(t *testing.T) {
	dir := decayBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	for _, v := range []string{"1.5", "-0.1"} {
		rec := doSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&half_life=90&decay_floor="+v)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("decay_floor=%s = %d, want 400 (body=%s)", v, rec.Code, rec.Body.String())
		}
	}
}

// TestSearch_MinRelevanceNegative400 is criterion 3: a negative min_relevance is
// a 400. min_relevance is a non-negative raw-cosine floor.
func TestSearch_MinRelevanceNegative400(t *testing.T) {
	dir := decayBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&min_relevance=-1")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("min_relevance=-1 = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}

// TestSearch_MinRelevanceDropsSubFloor is the positive control for min_relevance:
// a value above the weak node's raw cosine drops it, exactly as the CLI's
// --min-relevance does. Proves the param is not silently ignored.
func TestSearch_MinRelevanceDropsSubFloor(t *testing.T) {
	dir := decayBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	// Learn the weak node's raw cosine from an undecayed query.
	raw := mustSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5", http.StatusOK)
	var weakRaw float64
	var haveWeak bool
	for _, r := range raw.Results {
		if r.Path == "docs/fresh-weak.md" {
			weakRaw = r.Score
			haveWeak = true
		}
	}
	if !haveWeak || weakRaw <= 0 {
		t.Fatalf("premise broken: fresh-weak raw cosine should be > 0; results=%+v", raw.Results)
	}
	// A floor just above the weak node's raw cosine must drop it.
	floor := weakRaw + (1-weakRaw)/2
	got := mustSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5&min_relevance="+ftoa(floor), http.StatusOK)
	for _, r := range got.Results {
		if r.Path == "docs/fresh-weak.md" {
			t.Errorf("min_relevance=%v must drop sub-floor fresh-weak.md; got %+v", floor, got.Results)
		}
	}
}

// TestSearch_NoHalfLife_ByteIdenticalToToday is the load-bearing NEGATIVE control
// (criterion 4): with half_life ABSENT, no decay is applied and the response is
// byte-identical to a plain query — the floor must never engage without decay.
// Even passing decay_floor without half_life changes nothing.
func TestSearch_NoHalfLife_ByteIdenticalToToday(t *testing.T) {
	dir := decayBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	base := doSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5").Body.String()
	// A decay_floor with no half_life must be inert.
	withFloor := doSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5&decay_floor=0.9").Body.String()
	if base != withFloor {
		t.Errorf("decay_floor with no half_life perturbed the response:\n base:      %s\n withFloor: %s", base, withFloor)
	}
	// And it must equal the plain-Query oracle (nil Decay, no Meta).
	want := cliEquivalent(t, dir, e, decayReproQuery, 5, search.QueryOptions{})
	var got searchResponse
	if err := json.Unmarshal([]byte(base), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	assertSameRanking(t, got.Results, want)
}

// TestSearch_EmptyDecayFloor_TakesDefault is criterion 5: an empty ?decay_floor=
// reads as unset and takes the default, the nonEmptyQuery contract. It must equal
// the defaulted-floor query, NOT the decay_floor=0 unbounded query.
func TestSearch_EmptyDecayFloor_TakesDefault(t *testing.T) {
	dir := decayBundleDir(t)
	fixedNow(t) // pin the clock so both requests decay against the same instant
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	empty := doSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5&half_life=90&decay_floor=").Body.String()
	def := doSearch(t, h, "/api/v1/search?q="+qesc(decayReproQuery)+"&k=5&half_life=90").Body.String()
	if empty != def {
		t.Errorf("?decay_floor= (empty) must equal the default; got\n empty: %s\n def:   %s", empty, def)
	}
}

// TestSearch_EquivalentToCLI_DecayTable is criterion 7 — THE ANTI-DRIFT TEST. It
// drives the HTTP endpoint and the CLI-equivalent oracle (search.QueryWith) with
// the SAME query, bundle, index and half_life/decay_floor combinations, and
// asserts IDENTICAL score+path+snippet rankings. This is the test that would have
// caught the original drift and stops the next one. Table-driven over
// half_life in {0, 30, 90} x decay_floor in {default, 0, 0.5}.
func TestSearch_EquivalentToCLI_DecayTable(t *testing.T) {
	dir := decayBundleDir(t)
	now := fixedNow(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	halfLives := []float64{0, 30, 90}
	// floorSpec models the two ways a caller reaches a floor: omitting the param
	// (nil => the shared default) or passing an explicit value.
	type floorSpec struct {
		name  string
		param string  // query param value; "" means omit
		value float64 // the DecayFloor the oracle should use
	}
	floors := []floorSpec{
		{"default", "", search.DefaultDecayFloor},
		{"zero", "0", 0},
		{"half", "0.5", 0.5},
	}

	for _, hl := range halfLives {
		for _, f := range floors {
			name := "hl=" + ftoa(hl) + "/floor=" + f.name
			t.Run(name, func(t *testing.T) {
				target := "/api/v1/search?q=" + qesc(decayReproQuery) + "&k=5&half_life=" + ftoa(hl)
				if f.param != "" {
					target += "&decay_floor=" + f.param
				}
				got := mustSearch(t, h, target, http.StatusOK)

				// Build the oracle options exactly as the CLI would for these flags.
				opts := search.QueryOptions{}
				if hl > 0 {
					opts.Meta = liveMeta(t, dir)
					opts.Decay = &search.DecayOptions{
						HalfLifeDays: hl,
						Now:          now,
						MinRelevance: 0,
						DecayFloor:   f.value,
					}
				}
				want := cliEquivalent(t, dir, e, decayReproQuery, 5, opts)
				assertSameRanking(t, got.Results, want)
			})
		}
	}
}

// assertSameRanking asserts the HTTP results equal the CLI oracle's results
// field-for-field, in order.
func assertSameRanking(t *testing.T, got []searchResult, want []search.Result) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("results = %d, want %d (CLI oracle)\n got:  %+v\n want: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i].Path != want[i].Path {
			t.Errorf("result[%d].path = %q, want %q (CLI)", i, got[i].Path, want[i].Path)
		}
		if got[i].Score != want[i].Score {
			t.Errorf("result[%d].score = %v, want %v (CLI)", i, got[i].Score, want[i].Score)
		}
		if got[i].Snippet != want[i].Snippet {
			t.Errorf("result[%d].snippet = %q, want %q (CLI)", i, got[i].Snippet, want[i].Snippet)
		}
	}
}

// copyBundle recursively copies a bundle dir into a fresh temp dir so a test can
// build an index into it without writing to the source corpus. Returns the copy's
// path.
func copyBundle(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy bundle %s: %v", src, err)
	}
	return dst
}

// TestSearch_EquivalentToCLI_RealCorpus is criterion 8: the cross-surface
// equivalence harness run against the REAL knowledge base (a copy so no index is
// written into the source), not only the 3-node fixture. It drives the HTTP
// endpoint and the CLI-equivalent oracle over the same half_life x decay_floor
// table and asserts IDENTICAL rankings on every node. Skipped unless
// OKFCTL_EVAL_CORPUS points at a bundle dir (the corpus is not vendored), the same
// guard the retrieval-quality eval uses. Uses the offline hash embedder so no
// model2vec dir is required. Run:
//
//	OKFCTL_EVAL_CORPUS=~/src/knowledge-base/bundles/knowledge \
//	go test ./internal/apiserver/ -run TestSearch_EquivalentToCLI_RealCorpus -v
func TestSearch_EquivalentToCLI_RealCorpus(t *testing.T) {
	corpus := os.Getenv("OKFCTL_EVAL_CORPUS")
	if corpus == "" {
		t.Skip("set OKFCTL_EVAL_CORPUS to a bundle dir to run the real-corpus equivalence harness")
	}
	dir := copyBundle(t, corpus)
	now := fixedNow(t)
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)
	h := NewHandler(b, e)

	// Pin the corpus size so a shrunk/broken corpus is caught rather than passing
	// vacuously on a handful of nodes.
	nodeCount := len(b.Nodes)
	if nodeCount < 50 {
		t.Fatalf("real corpus has only %d nodes; expected the full KB (>= 50)", nodeCount)
	}
	t.Logf("real-corpus equivalence: corpus=%s nodes=%d", corpus, nodeCount)

	// A query drawn from the KB's own vocabulary so the ranking is non-degenerate.
	const q = "agent orchestration retrieval semantic search"
	k := 25

	halfLives := []float64{0, 30, 90}
	type floorSpec struct {
		name  string
		param string
		value float64
	}
	floors := []floorSpec{
		{"default", "", search.DefaultDecayFloor},
		{"zero", "0", 0},
		{"half", "0.5", 0.5},
	}

	for _, hl := range halfLives {
		for _, f := range floors {
			name := "hl=" + ftoa(hl) + "/floor=" + f.name
			t.Run(name, func(t *testing.T) {
				target := "/api/v1/search?q=" + qesc(q) + "&k=" + ftoa(float64(k)) + "&half_life=" + ftoa(hl)
				if f.param != "" {
					target += "&decay_floor=" + f.param
				}
				got := mustSearch(t, h, target, http.StatusOK)

				opts := search.QueryOptions{}
				if hl > 0 {
					opts.Meta = liveMeta(t, dir)
					opts.Decay = &search.DecayOptions{
						HalfLifeDays: hl,
						Now:          now,
						MinRelevance: 0,
						DecayFloor:   f.value,
					}
				}
				want := cliEquivalent(t, dir, e, q, k, opts)
				assertSameRanking(t, got.Results, want)
				t.Logf("%s: %d results match CLI over %d nodes", name, len(got.Results), nodeCount)
			})
		}
	}
}
