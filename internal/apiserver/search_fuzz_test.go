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
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
)

// FuzzSearchQueryParams drives the /api/v1/search query-parameter parser with
// untrusted raw query strings. The query string is fully caller-controlled input
// (§the apiserver reads url.Query directly), and every value goes through
// canonicalizeQuery, the did-you-mean levenshtein pass, and the strconv value
// validators (k, half_life, decay_floor, min_relevance, lexical_gate). A
// malformed query must produce a clean HTTP status — never a panic — and the
// status must always be a valid HTTP code. The handler (bundle + on-disk index)
// is built ONCE; only the raw query string is mutated, so the fuzz loop stays
// fast and hammers the parser rather than re-embedding a corpus each iteration.
//
// Seeds are the real query shapes from search_params_test.go: an unknown key, a
// separator alias, each scoring param with a boundary value, percent-encoding,
// repeated params, and adversarial junk.
func FuzzSearchQueryParams(f *testing.F) {
	dir := fuzzBundleDir(f)
	e := fuzzBuildIndex(f, dir)
	b, err := okf.Load(dir)
	if err != nil {
		f.Fatalf("load bundle: %v", err)
	}
	h := NewHandler(b, e)

	seeds := []string{
		"q=wine",
		"q=wine&k=5",
		"q=wine&notpath=casey",
		"q=wine&not_path=wine/",
		"q=wine&half_life=30&decay_floor=0.25&min_relevance=0.1",
		"q=wine&lexical_gate=true",
		"q=wine&lexical-gate=notabool",
		"q=wine&k=-1",
		"q=wine&k=99999999999999999999",
		"q=wine&decay_floor=2",
		"q=%ff%fe&path=%2e%2e%2f",
		"q=a&q=b&type=&type=x",
		"",
		"=&=&;;;&&&",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, rawQuery string) {
		rec := httptest.NewRecorder()
		// Build the request with a FIXED, valid target, then set the fuzzed
		// bytes as RawQuery directly on the URL. Routing the fuzzed string
		// through httptest.NewRequest's target parser would exercise net/http's
		// request-target splitter (which panics on an unparseable target like a
		// bare space) instead of the code under test — the query parser reached
		// via r.URL.Query(). Setting RawQuery post-construction hands the handler
		// exactly the arbitrary bytes a client could put after '?', which is the
		// real untrusted surface.
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search", nil)
		req.URL.RawQuery = rawQuery
		h.ServeHTTP(rec, req)

		if rec.Code < 100 || rec.Code > 599 {
			t.Fatalf("rawQuery=%q produced out-of-range HTTP status %d", rawQuery, rec.Code)
		}
	})
}

// FuzzCanonicalizeQuery drives the pure key-canonicalization + did-you-mean path
// (canonicalizeQuery, suggestParam, levenshtein) with an untrusted parameter
// key. It needs no bundle: it exercises the separator-aliasing and edit-distance
// logic in isolation, which is where an out-of-bounds or non-terminating edit
// walk would hide. It also asserts levenshtein's metric invariants (symmetry and
// identity-zero) on the fuzzed key against every accepted param.
func FuzzCanonicalizeQuery(f *testing.F) {
	seeds := []string{
		"path", "not-path", "not_path", "half-life", "half_life",
		"notpath", "nonsense_param", "", "q", "лех", "a\x00b",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, key string) {
		// canonicalizeQuery must not panic on any key shape; it either resolves
		// the key or writes a 400 and returns ok=false.
		raw := url.Values{key: {"v"}}
		rec := httptest.NewRecorder()
		_, _ = canonicalizeQuery(rec, raw)

		// levenshtein is a metric: d(a,a)=0 and d(a,b)=d(b,a). A break here (an
		// asymmetric or non-zero self-distance) would corrupt did-you-mean
		// suggestions, and the fuzzer probes rune/byte boundaries the seed
		// corpus never would.
		if d := levenshtein(key, key); d != 0 {
			t.Fatalf("levenshtein(%q,%q) = %d, want 0", key, key, d)
		}
		for _, p := range searchParams {
			if levenshtein(key, p) != levenshtein(p, key) {
				t.Fatalf("levenshtein not symmetric for %q vs %q", key, p)
			}
		}
		// suggestParam must not panic and must return either "" or a real param.
		if s := suggestParam(key); s != "" && canonicalParam(s) == "" {
			t.Fatalf("suggestParam(%q) = %q, which is not an accepted param", key, s)
		}
	})
}

// fuzzBundleDir / fuzzBuildIndex mirror searchBundleDir / buildIndex but take a
// testing.TB so they can run from a fuzz target's setup phase (where the handle
// is *testing.F, not *testing.T). They lay down the same small v0.2 bundle and
// build the same offline-hash index, so the fuzzed handler is byte-for-byte the
// one the example tests exercise.
func fuzzBundleDir(tb testing.TB) string {
	tb.Helper()
	dir := tb.TempDir()
	files := map[string]string{
		".okf": "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n" +
			"- [Tannin](wine/tannin.md)\n- [Acidity](wine/acidity.md)\n",
		"wine/tannin.md":  "---\ntype: Concept\ntitle: Tannin\ntags: [wine, mouthfeel]\n---\n\n# Tannin\n\nTannin is the astringent bitter phenolic compound in wine giving grip.\n",
		"wine/acidity.md": "---\ntype: Concept\ntitle: Acidity\ntags: [wine]\n---\n\n# Acidity\n\nAcidity is the tart freshness in wine balancing sweetness and fruit.\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	return dir
}

func fuzzBuildIndex(tb testing.TB, dir string) search.Embedder {
	tb.Helper()
	b, err := okf.Load(dir)
	if err != nil {
		tb.Fatalf("load bundle: %v", err)
	}
	e := search.NewHashEmbedder()
	s := search.BuildIndex(b, e, nil)
	if err := os.MkdirAll(filepath.Join(dir, ".okfctl"), 0o755); err != nil {
		tb.Fatal(err)
	}
	if err := s.Save(filepath.Join(dir, ".okfctl", "index.db")); err != nil {
		tb.Fatalf("save index: %v", err)
	}
	return e
}
