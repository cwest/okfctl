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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
)

// searchBundleDir lays down a small bundle whose concept nodes carry distinct
// bodies so the offline hash embedder yields a non-degenerate ranking, plus
// §4.1 type/tag metadata for the filter tests. It returns the bundle dir; the
// index is NOT built (callers that need one call buildIndex).
func searchBundleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".okf": "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n" +
			"- [Tannin](wine/tannin.md)\n- [Acidity](wine/acidity.md)\n- [Play](method/p.md)\n",
		"wine/tannin.md":  "---\ntype: Concept\ntitle: Tannin\ntags: [wine, mouthfeel]\n---\n\n# Tannin\n\nTannin is the astringent bitter phenolic compound in wine giving grip.\n",
		"wine/acidity.md": "---\ntype: Concept\ntitle: Acidity\ntags: [wine]\n---\n\n# Acidity\n\nAcidity is the tart freshness in wine balancing sweetness and fruit.\n",
		"method/p.md":     "---\ntype: Playbook\ntitle: Play\ntags: [process]\n---\n\n# Play\n\nA playbook describing a repeatable operational procedure step by step.\n",
	}
	writeFiles(t, dir, files)
	return dir
}

// buildIndex builds .okfctl/index.db for dir with the offline hash embedder and
// returns the embedder (the SAME instance shape a caller passes to NewHandler,
// so the model matches the store). It mirrors what `okfctl-search index build`
// does, so the API is exercised against a real on-disk index.
func buildIndex(t *testing.T, dir string) search.Embedder {
	t.Helper()
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	e := search.NewHashEmbedder()
	s := search.BuildIndex(b, e, nil)
	if err := os.MkdirAll(filepath.Join(dir, ".okfctl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(filepath.Join(dir, ".okfctl", "index.db")); err != nil {
		t.Fatalf("save index: %v", err)
	}
	return e
}

// cliEquivalent runs the SAME search pipeline the okfctl-search CLI's --semantic
// path runs (search.QueryWith against the on-disk store, with live-bundle meta
// for filters/decay), so a test can assert the HTTP endpoint returns the exact
// score/path/snippet triple the CLI would print for the same query and bundle.
// This is the equivalence oracle, not a re-implementation of the endpoint.
func cliEquivalent(t *testing.T, dir string, e search.Embedder, q string, k int, opts search.QueryOptions) []search.Result {
	t.Helper()
	s, err := search.Load(filepath.Join(dir, ".okfctl", "index.db"))
	if err != nil {
		t.Fatalf("load index: %v", err)
	}
	res, err := search.QueryWith(s, e, q, k, opts)
	if err != nil {
		t.Fatalf("cli-equivalent query: %v", err)
	}
	return res
}

func doSearch(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// Core acceptance test: GET /api/v1/search?q=<query>&k=N returns 200 with
// results whose score+path+snippet EXACTLY match the CLI's output for the same
// query and bundle. One mental model, proven, not asserted.
func TestSearch_EquivalentToCLI(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)
	h := NewHandler(b, e)

	rec := doSearch(t, h, "/api/v1/search?q=wine+tannin&k=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/search = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var got searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v\n%s", err, rec.Body.String())
	}

	want := cliEquivalent(t, dir, e, "wine tannin", 5, search.QueryOptions{})
	if len(got.Results) != len(want) {
		t.Fatalf("results = %d, want %d (CLI)", len(got.Results), len(want))
	}
	for i := range want {
		if got.Results[i].Path != want[i].Path {
			t.Errorf("result[%d].path = %q, want %q (CLI)", i, got.Results[i].Path, want[i].Path)
		}
		if got.Results[i].Score != want[i].Score {
			t.Errorf("result[%d].score = %v, want %v (CLI)", i, got.Results[i].Score, want[i].Score)
		}
		if got.Results[i].Snippet != want[i].Snippet {
			t.Errorf("result[%d].snippet = %q, want %q (CLI)", i, got.Results[i].Snippet, want[i].Snippet)
		}
	}
	if got.Model != e.Name() {
		t.Errorf("model = %q, want %q", got.Model, e.Name())
	}
}

// k caps the result count exactly as the CLI's --k does.
func TestSearch_KLimitsResults(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=wine&k=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	if len(got.Results) != 1 {
		t.Errorf("k=1 returned %d results, want 1", len(got.Results))
	}
	want := cliEquivalent(t, dir, e, "wine", 1, search.QueryOptions{})
	if len(want) == 1 && got.Results[0].Path != want[0].Path {
		t.Errorf("k=1 top result = %q, want %q (CLI)", got.Results[0].Path, want[0].Path)
	}
}

// The §4.1 type filter narrows the same way the CLI's --type flag does: an
// equivalence assertion against QueryWith with a Type filter resolved from the
// live bundle.
func TestSearch_TypeFilterEquivalentToCLI(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)
	h := NewHandler(b, e)

	rec := doSearch(t, h, "/api/v1/search?q=wine&type=Playbook&k=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var got searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}

	want := cliEquivalent(t, dir, e, "wine", 5, search.QueryOptions{
		Filter: search.Filter{Types: []string{"Playbook"}},
		Meta:   liveMeta(t, dir),
	})
	if len(got.Results) != len(want) {
		t.Fatalf("type-filtered results = %d, want %d (CLI); got=%+v", len(got.Results), len(want), got.Results)
	}
	for i := range want {
		if got.Results[i].Path != want[i].Path {
			t.Errorf("result[%d].path = %q, want %q", i, got.Results[i].Path, want[i].Path)
		}
	}
	// Type=Playbook must exclude the two Concept nodes.
	for _, r := range got.Results {
		if r.Path == "wine/tannin.md" || r.Path == "wine/acidity.md" {
			t.Errorf("type=Playbook leaked a Concept node: %q", r.Path)
		}
	}
}

// The path prefix filter matches the CLI's --path flag.
func TestSearch_PathFilterEquivalentToCLI(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=wine&path=wine/&k=5")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	want := cliEquivalent(t, dir, e, "wine", 5, search.QueryOptions{
		Filter: search.Filter{PathPrefixes: []string{"wine/"}},
		Meta:   liveMeta(t, dir),
	})
	if len(got.Results) != len(want) {
		t.Fatalf("path-filtered results = %d, want %d", len(got.Results), len(want))
	}
	for _, r := range got.Results {
		if !strings.HasPrefix(r.Path, "wine/") {
			t.Errorf("path=wine/ leaked a non-wine node: %q", r.Path)
		}
	}
}

// liveMeta builds the per-node metadata for filters/decay from the live bundle,
// the same query-time resolution the endpoint and the CLI both perform.
func liveMeta(t *testing.T, dir string) map[string]search.NodeMeta {
	t.Helper()
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	m := make(map[string]search.NodeMeta, len(b.Nodes))
	for path, n := range b.Nodes {
		nm := search.NodeMeta{Type: n.Type(), Tags: n.Tags()}
		if gen, ok := n.Generated(); ok {
			nm.Generated = gen.At
			nm.HasGenerated = true
		}
		m[path] = nm
	}
	return m
}

// Missing q -> 400, never a 200 with everything.
func TestSearch_MissingQ400(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET /api/v1/search (no q) = %d, want 400", rec.Code)
	}
	// It must NOT be a 200 with a full result list.
	if rec.Code == http.StatusOK {
		var got searchResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		t.Errorf("missing q returned 200 with %d results, want 400", len(got.Results))
	}
}

// Empty q value (?q=) is also a 400 — an empty query is missing, not "match all".
func TestSearch_EmptyQ400(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET /api/v1/search?q= = %d, want 400", rec.Code)
	}
}

// A bundle with no index built must return a clean 5xx with a useful message,
// not a panic and not a 200 with an empty list. Mirrors the CLI's
// "no index at <path> (run 'okfctl-search index build' first)".
func TestSearch_NoIndex503WithUsefulMessage(t *testing.T) {
	dir := searchBundleDir(t) // note: buildIndex NOT called -> no .okfctl/index.db
	e := search.NewHashEmbedder()
	h := NewHandler(loadBundle(t, dir), e)

	rec := doSearch(t, h, "/api/v1/search?q=wine")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /api/v1/search with no index = %d, want 503", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "index") {
		t.Errorf("no-index error should mention the index; got %q", body)
	}
	// It must NOT be a 200 with an empty results list masquerading as "no hits".
	var got searchResponse
	if json.Unmarshal(rec.Body.Bytes(), &got) == nil && rec.Code == http.StatusOK {
		t.Errorf("no-index returned a 200 result envelope, want 503")
	}
}

// Method gate: POST /api/v1/search -> 405. GET-only is part of the security model.
func TestSearch_PostNotAllowed405(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	h := NewHandler(loadBundle(t, dir), e)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/search?q=wine", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/v1/search = %d, want 405", rec.Code)
	}
}

// Negative control (load-bearing): adding the search route must not perturb the
// existing /stats and /graph responses. Assert on the response BODIES, byte for
// byte, against a handler built the pre-search way was — here, against a second
// handler over the same bundle, since both must be deterministic and identical.
func TestSearch_DoesNotPerturbStatsAndGraph(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)
	h := NewHandler(b, e)

	// A handler with a nil embedder (search disabled) must serve /stats and
	// /graph byte-identically to one with search enabled: the search route is
	// purely additive.
	hNoSearch := NewHandler(b, nil)

	for _, path := range []string{"/api/v1/stats", "/api/v1/graph"} {
		withSearch := doSearch(t, h, path).Body.String()
		noSearch := doSearch(t, hNoSearch, path).Body.String()
		if withSearch != noSearch {
			t.Errorf("%s body differs when search is enabled vs disabled:\n with: %s\n without: %s", path, withSearch, noSearch)
		}
	}
}

// Staleness positive control: rebuild the index while the handler is resident;
// the next request must reflect the NEW index. Proves the stat-and-reload path.
func TestSearch_ReloadsOnIndexChange(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)
	h := NewHandler(b, e)

	// First query establishes the resident index.
	rec1 := doSearch(t, h, "/api/v1/search?q=wine&k=5")
	if rec1.Code != http.StatusOK {
		t.Fatalf("first query = %d, want 200", rec1.Code)
	}
	var got1 searchResponse
	if err := json.Unmarshal(rec1.Body.Bytes(), &got1); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}

	// Add a brand-new node and rebuild the index on disk. To make the mtime
	// change observable regardless of filesystem timestamp granularity, bump
	// the index file's modtime forward explicitly after the rebuild.
	writeFiles(t, dir, map[string]string{
		"wine/terroir.md": "---\ntype: Concept\ntitle: Terroir\ntags: [wine]\n---\n\n# Terroir\n\nTerroir is the wine expression of place: soil climate and vineyard site.\n",
	})
	// Re-point the index.md so terroir is linked (keeps the bundle well-formed).
	writeFiles(t, dir, map[string]string{
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n" +
			"- [Tannin](wine/tannin.md)\n- [Acidity](wine/acidity.md)\n- [Play](method/p.md)\n- [Terroir](wine/terroir.md)\n",
	})
	_ = buildIndex(t, dir)
	idxPath := filepath.Join(dir, ".okfctl", "index.db")
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(idxPath, future, future); err != nil {
		t.Fatal(err)
	}

	// The resident handler must now see terroir in its results.
	rec2 := doSearch(t, h, "/api/v1/search?q=terroir+wine&k=5")
	if rec2.Code != http.StatusOK {
		t.Fatalf("post-rebuild query = %d, want 200 (body=%s)", rec2.Code, rec2.Body.String())
	}
	var got2 searchResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &got2); err != nil {
		t.Fatalf("bad JSON: %v", err)
	}
	found := false
	for _, r := range got2.Results {
		if r.Path == "wine/terroir.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("after index rebuild, terroir not in results %+v — stat-and-reload did not fire", got2.Results)
	}
}

// Staleness negative control (the load-bearing one): N requests against an
// UNCHANGED index must load the index from disk exactly ONCE. This proves the
// resident feature actually holds the index for the process lifetime instead of
// reloading per request (the whole point of the endpoint).
func TestSearch_UnchangedIndexLoadsExactlyOnce(t *testing.T) {
	dir := searchBundleDir(t)
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)

	var loads int64
	// loadCounter wraps the store loader so the test can count disk loads. The
	// handler exposes an injectable loader hook for exactly this negative control.
	h := newHandlerWithLoader(b, e, func(path string) (*search.Store, error) {
		atomic.AddInt64(&loads, 1)
		return search.Load(path)
	})

	for i := 0; i < 5; i++ {
		rec := doSearch(t, h, "/api/v1/search?q=wine&k=5")
		if rec.Code != http.StatusOK {
			t.Fatalf("query %d = %d, want 200", i, rec.Code)
		}
	}
	if n := atomic.LoadInt64(&loads); n != 1 {
		t.Errorf("index loaded %d times over 5 requests with an unchanged index, want exactly 1", n)
	}
}
