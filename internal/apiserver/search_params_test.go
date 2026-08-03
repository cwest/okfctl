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
	"strings"
	"testing"
)

// GET /api/v1/search rejects an unknown query parameter with 400 and, when the
// key is one edit away from a real one, names the intended parameter. The
// canonical trap the card documents: a caller who writes not_path (underscore)
// instead of not-path (hyphen) used to get a silent 200 that returned exactly
// the nodes they meant to exclude. It must now 400 — BUT because the fix also
// aliases separators, not_path is a VALID alias, not an unknown key. So the
// unknown-key control here uses a genuinely misspelled key.
func TestSearch_UnknownParamRejected400WithSuggestion(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	// notpath (no separator) is not a real param and is one edit from not-path.
	rec := doSearch(t, h, "/api/v1/search?q=wine&notpath=casey")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown param notpath = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"notpath"`) {
		t.Errorf("400 body should name the offending param %q; got %q", "notpath", body)
	}
	if !strings.Contains(body, `not-path`) {
		t.Errorf("400 body should suggest the nearest real param not-path; got %q", body)
	}
}

// A wholly unrecognizable param (far from any real name) is still a 400, but
// need not carry a suggestion.
func TestSearch_NonsenseParamRejected400(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=wine&nonsense_param=1")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("nonsense_param = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
}

// Separator aliasing (positive): the underscore spelling of a hyphen filter is
// accepted and HONOURED, not rejected. not_path=<v> must exclude the same nodes
// not-path=<v> does — byte-for-byte the same response.
func TestSearch_UnderscoreAliasHonouredForFilter(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	canonical := doSearch(t, h, "/api/v1/search?q=wine&not-path=wine/")
	if canonical.Code != http.StatusOK {
		t.Fatalf("not-path canonical = %d, want 200 (body=%s)", canonical.Code, canonical.Body.String())
	}
	alias := doSearch(t, h, "/api/v1/search?q=wine&not_path=wine/")
	if alias.Code != http.StatusOK {
		t.Fatalf("not_path alias = %d, want 200 (body=%s)", alias.Code, alias.Body.String())
	}
	if alias.Body.String() != canonical.Body.String() {
		t.Errorf("not_path alias body differs from not-path canonical:\n alias: %s\n canon: %s",
			alias.Body.String(), canonical.Body.String())
	}
	// And it must actually exclude wine/ nodes, proving it was honoured.
	var got searchResponse
	if err := json.Unmarshal(alias.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	for _, r := range got.Results {
		if strings.HasPrefix(r.Path, "wine/") {
			t.Errorf("not_path=wine/ alias leaked a wine node: %q", r.Path)
		}
	}
}

// Separator aliasing (positive): the hyphen spelling of an underscore scoring
// param is accepted and HONOURED. half-life must behave identically to
// half_life — proving the alias fires, not just that it stops 400ing. Uses the
// decay fixture (nodes carry generated.at) so half_life actually reorders; on
// the plain fixture (no generated dates) decay is a no-op and the test could
// not distinguish "honoured" from "ignored".
func TestSearch_HyphenAliasHonouredForScoringParam(t *testing.T) {
	dir := decayBundleDir(t)
	fixedNow(t) // pin the clock so decay is deterministic; build handler after
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	base := "/api/v1/search?q=" + qesc(decayReproQuery)
	canonical := doSearch(t, h, base+"&half_life=90&k=5")
	if canonical.Code != http.StatusOK {
		t.Fatalf("half_life canonical = %d, want 200 (body=%s)", canonical.Code, canonical.Body.String())
	}
	alias := doSearch(t, h, base+"&half-life=90&k=5")
	if alias.Code != http.StatusOK {
		t.Fatalf("half-life alias = %d, want 200 (body=%s)", alias.Code, alias.Body.String())
	}
	if alias.Body.String() != canonical.Body.String() {
		t.Errorf("half-life alias body differs from half_life canonical:\n alias: %s\n canon: %s",
			alias.Body.String(), canonical.Body.String())
	}
	// Load-bearing: the alias must actually be HONOURED, not merely accepted.
	// half_life=90 triggers recency decay on the dated fixture, which reorders
	// the ranking (the decay tests prove old-exact.md moves to the top); the
	// body must therefore differ from a query with no half_life. Without this
	// the test passes vacuously when half-life is silently ignored.
	baseline := doSearch(t, h, base+"&k=5")
	if baseline.Code != http.StatusOK {
		t.Fatalf("no-half-life baseline = %d, want 200", baseline.Code)
	}
	if alias.Body.String() == baseline.Body.String() {
		t.Errorf("half-life=90 alias produced the no-half-life baseline body — decay was not applied, alias silently ignored")
	}
}

// Negative control (load-bearing): every currently-documented spelling still
// returns 200 with a body byte-identical to the pre-change behavior. We can't
// hold a pre-change binary here, so we assert each documented param is accepted
// (200) and that a no-op-valued param leaves the baseline body unchanged.
func TestSearch_DocumentedSpellingsStillAccepted(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	for _, target := range []string{
		"/api/v1/search?q=wine&k=5",
		"/api/v1/search?q=wine&path=wine/",
		"/api/v1/search?q=wine&type=Concept",
		"/api/v1/search?q=wine&tag=wine",
		"/api/v1/search?q=wine&not-path=method/",
		"/api/v1/search?q=wine&not-type=Playbook",
		"/api/v1/search?q=wine&not-tag=process",
		"/api/v1/search?q=wine&half_life=7",
		"/api/v1/search?q=wine&decay_floor=0.5",
		"/api/v1/search?q=wine&min_relevance=0.1",
	} {
		rec := doSearch(t, h, target)
		if rec.Code != http.StatusOK {
			t.Errorf("documented param %q = %d, want 200 (body=%s)", target, rec.Code, rec.Body.String())
		}
	}
}

// Negative control: an empty-valued param (?type=) still reads as unset. It must
// stay 200 and produce the SAME body as the baseline (no filter applied), not a
// 400 for an "unknown"/empty key.
func TestSearch_EmptyValuedParamUnchanged(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	baseline := doSearch(t, h, "/api/v1/search?q=wine&k=5")
	if baseline.Code != http.StatusOK {
		t.Fatalf("baseline = %d, want 200", baseline.Code)
	}
	withEmpty := doSearch(t, h, "/api/v1/search?q=wine&k=5&type=")
	if withEmpty.Code != http.StatusOK {
		t.Fatalf("?type= = %d, want 200 (body=%s)", withEmpty.Code, withEmpty.Body.String())
	}
	if withEmpty.Body.String() != baseline.Body.String() {
		t.Errorf("?type= perturbed the body; empty value must read as unset:\n empty: %s\n base:  %s",
			withEmpty.Body.String(), baseline.Body.String())
	}
}

// Negative control: repeated params still OR within a dimension. ?path=a&path=b
// must be accepted (not rejected as a duplicate) and behave as two prefixes.
func TestSearch_RepeatedParamsStillOR(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=wine&path=wine/&path=method/&k=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("repeated path = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var got searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	for _, r := range got.Results {
		if !strings.HasPrefix(r.Path, "wine/") && !strings.HasPrefix(r.Path, "method/") {
			t.Errorf("repeated path OR leaked an out-of-scope node: %q", r.Path)
		}
	}
}

// Negative control: repeated params across a hyphen/underscore ALIAS boundary
// must merge into the same dimension. ?path=wine/&path_alias... — specifically
// path=wine/ (canonical) plus its alias must OR together, not one shadow the
// other. Here we prove not-path and not_path OR into a single exclusion set.
func TestSearch_AliasAndCanonicalMergeInSameDimension(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	// not-path=wine/ (canonical) + not_path=method/ (alias): both exclusions
	// must apply, leaving neither wine/ nor method/ nodes.
	rec := doSearch(t, h, "/api/v1/search?q=wine&not-path=wine/&not_path=method/&k=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("mixed alias/canonical = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var got searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	for _, r := range got.Results {
		if strings.HasPrefix(r.Path, "wine/") || strings.HasPrefix(r.Path, "method/") {
			t.Errorf("mixed alias/canonical exclusion leaked %q; both dimensions must merge", r.Path)
		}
	}
}

// Negative control: the four existing value-validation 400s keep their EXACT
// current wording. Strictness on keys must not change value-error messages.
func TestSearch_ValueValidationWordingUnchanged(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	cases := []struct {
		target string
		want   string
	}{
		{"/api/v1/search?q=wine&k=notanumber", `"k" must be a non-negative integer`},
		{"/api/v1/search?q=wine&half_life=abc", `"half_life" must be a non-negative number of days`},
		{"/api/v1/search?q=wine&decay_floor=2", `"decay_floor" must be in [0, 1]`},
		{"/api/v1/search?q=wine&min_relevance=-1", `"min_relevance" must be a non-negative number`},
	}
	for _, c := range cases {
		rec := doSearch(t, h, c.target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", c.target, rec.Code)
		}
		if got := strings.TrimSpace(rec.Body.String()); got != c.want {
			t.Errorf("%s body = %q, want exact %q", c.target, got, c.want)
		}
	}
}

// The value-validation 400s must also fire through the ALIAS spelling with the
// alias-resolved canonical name in the message, so a caller using half-life=abc
// gets the same guidance as half_life=abc.
func TestSearch_AliasValueValidationUsesCanonicalWording(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=wine&half-life=abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("half-life=abc = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `"half_life" must be a non-negative number of days` {
		t.Errorf("half-life=abc body = %q, want canonical half_life wording", got)
	}
}

// Negative control: strictness must NOT leak to /stats or /graph. An unknown
// query param on those routes must not turn into a 400 (they ignore query
// params entirely and still 200).
func TestSearch_StrictnessDoesNotLeakToOtherRoutes(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	for _, target := range []string{
		"/api/v1/stats?bogus=1",
		"/api/v1/graph?not_a_param=x",
	} {
		rec := doSearch(t, h, target)
		if rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200 (strictness must not leak off /search)", target, rec.Code)
		}
	}
}

// q and k themselves are accepted keys and must not be flagged unknown.
func TestSearch_QAndKNotFlaggedUnknown(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=wine&k=3")
	if rec.Code != http.StatusOK {
		t.Errorf("q&k = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
}
