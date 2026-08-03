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
	"net/http"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
)

// gateBundleDir lays down six nodes where the token "acidity" appears in exactly
// two (a wine node and a coffee node) — a MINORITY, so a gate on "acidity" does
// not degrade as over-broad. This is the same shape the CLI gate tests use
// (cmd/okfctl-search/main_test.go writeGateTailBundle), so the HTTP surface and
// the CLI are exercised on comparable data. The index is NOT built; callers that
// need one call buildIndex. §4.1 (semantic query is a consumer concern, not a
// spec-defined ranking rule — the gate is a §4.1-permitted result-shaping choice
// the tool may offer, so the floor never rejects a bundle for its absence).
func gateBundleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".okf": "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n" +
			"- [Tannin](wine/tannin.md)\n- [Wine Acidity](wine/acidity.md)\n- [Body](wine/body.md)\n" +
			"- [Roast](coffee/roast.md)\n- [Grind](coffee/grind.md)\n- [Brewing](coffee/brewing.md)\n",
		"wine/tannin.md":    "---\ntype: Concept\ntitle: Tannin\n---\n\n# Tannin\n\nTannin gives structure and astringency.\n",
		"wine/acidity.md":   "---\ntype: Concept\ntitle: Wine Acidity\n---\n\n# Wine Acidity\n\nAcidity gives a wine freshness and lift.\n",
		"wine/body.md":      "---\ntype: Concept\ntitle: Body\n---\n\n# Body\n\nBody is the weight of the wine on the palate.\n",
		"coffee/roast.md":   "---\ntype: Concept\ntitle: Roast\n---\n\n# Roast\n\nRoast level shapes the coffee acidity and sweetness.\n",
		"coffee/grind.md":   "---\ntype: Concept\ntitle: Grind\n---\n\n# Grind\n\nGrind size controls extraction rate.\n",
		"coffee/brewing.md": "---\ntype: Concept\ntitle: Brewing\n---\n\n# Brewing\n\nBrewing temperature and time drive flavor.\n",
	}
	writeFiles(t, dir, files)
	return dir
}

// cliGateOpts builds the SAME LexicalGateOptions the okfctl-search CLI builds for
// a --lexical-gate query over dir, via the shared search.BuildLexicalGate. Using
// the shared constructor here (rather than re-deriving WideN/OverBroadFraction as
// literals) is the point: the oracle and the handler agree by construction, so
// this test cannot pass while the two surfaces drift.
func cliGateOpts(t *testing.T, dir, query string) search.QueryOptions {
	t.Helper()
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	return search.QueryOptions{
		Meta:        liveMeta(t, dir),
		LexicalGate: search.BuildLexicalGate(b, query),
	}
}

// gateMovesRanking reports whether the lexical gate changes the CLI oracle's
// ranked path list for query over dir at k — the positive control that a gate
// test is exercising a gate that actually does something, not a no-op fixture.
func gateMovesRanking(t *testing.T, dir string, e search.Embedder, query string, k int) bool {
	t.Helper()
	ungated := cliEquivalent(t, dir, e, query, k, search.QueryOptions{})
	gated := cliEquivalent(t, dir, e, query, k, cliGateOpts(t, dir, query))
	if len(ungated) != len(gated) {
		return true
	}
	for i := range ungated {
		if ungated[i].Path != gated[i].Path {
			return true
		}
	}
	return false
}

const gateQuery = "wine acidity freshness"

// TestSearch_GateOn_EquivalentToCLIGate is the positive HTTP control: with the
// gate param on, the endpoint returns the EXACT score/path/snippet triple the
// CLI's --lexical-gate produces for the same query and bundle. Proven against the
// shared BuildLexicalGate oracle so the two surfaces cannot disagree.
func TestSearch_GateOn_EquivalentToCLIGate(t *testing.T) {
	dir := gateBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	// The fixture must be one the gate actually moves, or this proves nothing.
	if !gateMovesRanking(t, dir, e, gateQuery, 6) {
		t.Fatalf("fixture bug: gate is a no-op on %q; pick a fixture the gate moves", gateQuery)
	}

	got := mustSearch(t, h, "/api/v1/search?q="+qesc(gateQuery)+"&k=6&lexical_gate=true", http.StatusOK)
	want := cliEquivalent(t, dir, e, gateQuery, 6, cliGateOpts(t, dir, gateQuery))
	assertSameRanking(t, got.Results, want)
}

// TestSearch_GateBothSeparators is the #77 alias set: both lexical_gate and
// lexical-gate are accepted spellings and yield the same gated ranking. The card
// mandates supporting both separators from the start since #77 has not merged.
func TestSearch_GateBothSeparators(t *testing.T) {
	dir := gateBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)
	want := cliEquivalent(t, dir, e, gateQuery, 6, cliGateOpts(t, dir, gateQuery))

	for _, sep := range []string{"lexical_gate", "lexical-gate"} {
		t.Run(sep, func(t *testing.T) {
			got := mustSearch(t, h, "/api/v1/search?q="+qesc(gateQuery)+"&k=6&"+sep+"=true", http.StatusOK)
			assertSameRanking(t, got.Results, want)
		})
	}
}

// TestSearch_GateNonBoolean400 is the house-wording validation: a non-boolean
// gate value is a 400, never a silently-ignored 200, mirroring the k/half_life/
// decay_floor/min_relevance validation already in the handler.
func TestSearch_GateNonBoolean400(t *testing.T) {
	dir := gateBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	for _, sep := range []string{"lexical_gate", "lexical-gate"} {
		rec := doSearch(t, h, "/api/v1/search?q="+qesc(gateQuery)+"&k=6&"+sep+"=banana")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s=banana = %d, want 400 (body=%s)", sep, rec.Code, rec.Body.String())
		}
	}
}

// TestSearch_GateOmitted_ByteIdenticalToToday is the load-bearing negative
// control: with the gate param omitted, the body is byte-identical to the
// pre-gate ungated response for the same query. The gate must be purely additive.
func TestSearch_GateOmitted_ByteIdenticalToToday(t *testing.T) {
	dir := gateBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	base := doSearch(t, h, "/api/v1/search?q="+qesc(gateQuery)+"&k=6")
	if base.Code != http.StatusOK {
		t.Fatalf("ungated = %d, want 200", base.Code)
	}
	// Gate explicitly false must be byte-identical to omitting it.
	off := doSearch(t, h, "/api/v1/search?q="+qesc(gateQuery)+"&k=6&lexical_gate=false")
	if off.Code != http.StatusOK {
		t.Fatalf("gate=false = %d, want 200", off.Code)
	}
	if base.Body.String() != off.Body.String() {
		t.Errorf("gate=false body differs from omitted:\n omitted=%s\n false=%s", base.Body.String(), off.Body.String())
	}
}

// TestSearch_GateNoOpQuery_ByteIdenticalGatedAndUngated is the documented no-op
// degrade on the HTTP surface: an all-stopword query carries no content terms, so
// the gate degrades to pure semantic and the gated body is byte-identical to the
// ungated one — matching the CLI's degrade rule.
func TestSearch_GateNoOpQuery_ByteIdenticalGatedAndUngated(t *testing.T) {
	dir := gateBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	const allStop = "the and of to a in"
	ungated := doSearch(t, h, "/api/v1/search?q="+qesc(allStop)+"&k=6")
	gated := doSearch(t, h, "/api/v1/search?q="+qesc(allStop)+"&k=6&lexical_gate=true")
	if ungated.Code != http.StatusOK || gated.Code != http.StatusOK {
		t.Fatalf("all-stopword: ungated=%d gated=%d, want 200/200", ungated.Code, gated.Code)
	}
	if ungated.Body.String() != gated.Body.String() {
		t.Errorf("all-stopword gate is not a no-op:\n ungated=%s\n gated=%s", ungated.Body.String(), gated.Body.String())
	}
}

// TestSearch_GateResidentIndexLoadsExactlyOnce is the resident-index invariant
// under gating: N gated requests against an UNCHANGED index perform exactly one
// disk load. The gate resolves against the live bundle, but that must not defeat
// the store cache the resident-server feature exists for.
func TestSearch_GateResidentIndexLoadsExactlyOnce(t *testing.T) {
	dir := gateBundleDir(t)
	e := buildIndex(t, dir)
	var loads int32
	counting := func(path string) (*search.Store, error) {
		loads++
		return search.Load(path)
	}
	h := newHandlerWithLoader(loadBundle(t, dir), e, counting)

	for i := 0; i < 5; i++ {
		rec := doSearch(t, h, "/api/v1/search?q="+qesc(gateQuery)+"&k=6&lexical_gate=true")
		if rec.Code != http.StatusOK {
			t.Fatalf("gated request %d = %d, want 200", i, rec.Code)
		}
	}
	if loads != 1 {
		t.Errorf("gated index loads = %d across 5 requests, want exactly 1", loads)
	}
}
