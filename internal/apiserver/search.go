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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
)

// storeLoader loads a *search.Store from a path. Production uses search.Load; a
// test can substitute a counting wrapper to prove the resident index is loaded
// from disk exactly once across many requests with an unchanged index.
type storeLoader func(path string) (*search.Store, error)

// searchResult is the API's per-hit row: the same score/path/snippet triple the
// okfctl-search CLI prints (cmd/okfctl-search/main.go printResults), serialized
// as JSON. It is the CLI's output, not a reinvention — the equivalence-to-CLI
// acceptance test asserts these three fields match search.QueryWith exactly.
type searchResult struct {
	Score   float64 `json:"score"`
	Path    string  `json:"path"`
	Snippet string  `json:"snippet"`
}

// searchResponse is the GET /api/v1/search body. `indexed_at` is the on-disk
// index file's modtime (RFC3339), so a consumer can tell how fresh the answers
// are without a second request (§2.7 "expose the indexed-at timestamp"). `model`
// is the index's own recorded embedder model.
type searchResponse struct {
	Schema    int            `json:"schema"`
	Query     string         `json:"query"`
	Model     string         `json:"model"`
	IndexedAt string         `json:"indexed_at"`
	Results   []searchResult `json:"results"`
}

// searchService answers GET /api/v1/search off a resident, stat-and-reloaded
// index. It holds the loaded *search.Store and the live-bundle node metadata for
// the process lifetime and reloads BOTH on the same signal — a change in the
// index file's modtime — so a `okfctl-search index build` against the served
// bundle is picked up without a restart (§2.7 staleness). The relevance floor,
// tie-breaking, filter, and decay behaviors are all search.QueryWith's, so the
// endpoint can never disagree with the CLI for the same query and bundle.
type searchService struct {
	root     string
	embedder search.Embedder
	load     storeLoader
	// now is the clock recency decay measures ages from. It defaults to the
	// package now (time.Now) and is overridable in tests so a cross-surface
	// equivalence assertion can pin the SAME instant into both the endpoint and
	// the CLI-equivalent oracle and get byte-identical decayed scores.
	now func() time.Time

	mu      sync.Mutex
	modTime time.Time                  // index file modtime the cache was built from
	loaded  bool                       // whether store/meta are populated
	store   *search.Store              // resident index
	meta    map[string]search.NodeMeta // live-bundle metadata for filters/decay
	bundle  *okf.Bundle                // live bundle, retained for the lexical gate's query-time prose match
}

func newSearchService(root string, e search.Embedder, load storeLoader) *searchService {
	return &searchService{root: root, embedder: e, load: load, now: now}
}

// indexPath is the bundle's flat vector store, the same location the CLI's
// okfctl-search index build/query use (.okfctl/index.db).
func (s *searchService) indexPath() string {
	return filepath.Join(s.root, ".okfctl", "index.db")
}

// errNoIndex signals a missing index: the endpoint surfaces it as 503 with a
// build hint rather than a 200 with an empty list, mirroring the CLI's
// "no index at <path> (run 'okfctl-search index build' first)".
var errNoIndex = errors.New("no index")

// ensureFresh loads the index + live-bundle meta if not yet loaded, or reloads
// both when the index file's modtime has changed since the cache was built. It
// returns the resident store, meta, the live bundle (retained for the lexical
// gate's query-time prose match), and the index modtime. A missing index returns
// errNoIndex. Holding s.mu makes concurrent requests share one reload.
func (s *searchService) ensureFresh() (*search.Store, map[string]search.NodeMeta, *okf.Bundle, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fi, err := os.Stat(s.indexPath())
	if err != nil || fi.IsDir() {
		// Missing (or a directory where the index should be): fail closed with a
		// build hint, never a silent empty result set.
		s.loaded = false
		s.store = nil
		s.meta = nil
		s.bundle = nil
		return nil, nil, nil, time.Time{}, errNoIndex
	}

	// Cache hit: the resident index is still current. This is the hot path the
	// resident-server feature exists for — N requests against an unchanged index
	// do exactly one disk load (the load-bearing negative control). The gate
	// reads s.bundle from this same cache, so a gated request adds NO extra load.
	if s.loaded && fi.ModTime().Equal(s.modTime) {
		return s.store, s.meta, s.bundle, s.modTime, nil
	}

	// First load, or the index changed on disk: reload the store AND re-walk the
	// live bundle for filter/decay metadata AND the gate's prose, invalidating
	// all on the same signal (§2.7). Filters, recency decay, and the lexical gate
	// resolve against the LIVE bundle, not the index, so a frontmatter-only edit
	// is reflected too.
	store, err := s.load(s.indexPath())
	if err != nil {
		s.loaded = false
		s.store = nil
		s.meta = nil
		s.bundle = nil
		return nil, nil, nil, time.Time{}, errNoIndex
	}
	b, err := okf.Load(s.root)
	if err != nil {
		return nil, nil, nil, time.Time{}, err
	}
	s.store = store
	s.bundle = b
	s.meta = metaFromBundle(b)
	s.modTime = fi.ModTime()
	s.loaded = true
	return s.store, s.meta, s.bundle, s.modTime, nil
}

// metaFromBundle resolves the per-node metadata the §4.1 filters and §5.2/§13.1
// recency decay key on from a loaded bundle — the exact query-time resolution
// cmd/okfctl-search/main.go's buildNodeMeta performs, so the API's filtered and
// decayed rankings match the CLI's. Resolving against the bundle (not the index)
// is deliberate: contentHash keys only title+body, so a type/tag/generated-only
// edit does not re-embed and a value denormalized onto the index would go stale.
func metaFromBundle(b *okf.Bundle) map[string]search.NodeMeta {
	m := make(map[string]search.NodeMeta, len(b.Nodes))
	for path, n := range b.Nodes {
		nm := search.NodeMeta{Type: n.Type(), Tags: n.Tags()}
		if gen, ok := n.Generated(); ok { // §5.2 generated.at / §13.1 timestamp fallback
			nm.Generated = gen.At
			nm.HasGenerated = true
		}
		m[path] = nm
	}
	return m
}

// handle answers GET /api/v1/search. The scope grammar mirrors #68's CLI
// grammar (StringArray flags plus not-* exclusions): path/type/tag each repeat
// and OR within their dimension, and not-path/not-type/not-tag exclude. Go's
// url.Query returns every occurrence of a param, so ?path=a&path=b reads as two
// prefixes exactly as the CLI's repeated --path flags do. The card's ordering
// dependency says to mirror #68 when it merges first rather than invent a
// syntax, which is what this does. The recency-decay params match the CLI's:
// half_life is scalar (--half-life), decay_floor is the #65 lower clamp on the
// recency multiplier (--decay-floor, default search.DefaultDecayFloor), and
// min_relevance is the #65 raw-cosine floor (--min-relevance). decay_floor and
// min_relevance are validated with #72's CLI rules and reuse its error wording so
// both surfaces speak the same language:
//
//	q             required semantic query string (the CLI --semantic)
//	k             max results (default 5, the CLI default)
//	path          §4.1 path-prefix filter, repeatable, OR   (the CLI --path)
//	type          §4.1 type filter,        repeatable, OR   (the CLI --type)
//	tag           §4.1 tag filter,         repeatable, OR   (the CLI --tag)
//	not-path      §4.1 path-prefix exclusion, repeatable    (the CLI --not-path)
//	not-type      §4.1 type exclusion,        repeatable    (the CLI --not-type)
//	not-tag       §4.1 tag exclusion,         repeatable    (the CLI --not-tag)
//	half_life     §5.2/§13.1 recency half-life in days, scalar (the CLI --half-life)
//	decay_floor   §5.2/#65 lower clamp on the recency multiplier, [0,1] (the CLI --decay-floor)
//	min_relevance §5.2/#65 raw-cosine floor applied before decay, >= 0 (the CLI --min-relevance)
//
// Separator aliasing: filters take hyphens (not-path) while scoring params take
// underscores (half_life), a split a caller cannot infer, so BOTH separators are
// accepted for every parameter and canonicalized before use. An unknown key is a
// 400 (never a silent 200 applying nothing), with a did-you-mean when it is close
// to a real one — the accepted set is small and closed, so suggestion is cheap.
//
// searchParams is the closed set of accepted parameter names, in canonical
// spelling. canonicalParam resolves an incoming key (in either separator) to its
// canonical name; an unrecognized key resolves to "". The canonical spelling is
// the one the code below reads and the one every value-validation error quotes,
// so a caller using an alias gets guidance in the documented name.
var searchParams = []string{
	"q", "k",
	"path", "type", "tag",
	"not-path", "not-type", "not-tag",
	"half_life", "decay_floor", "min_relevance",
}

// canonicalParam maps any accepted spelling (hyphen or underscore separator) of
// a known parameter to its canonical name, or "" if the key is not accepted.
// Both separators normalize to a single form for the lookup, so not_path and
// not-path (and half-life and half_life) resolve to the same canonical key.
func canonicalParam(key string) string {
	norm := func(s string) string { return strings.ReplaceAll(s, "-", "_") }
	target := norm(key)
	for _, p := range searchParams {
		if norm(p) == target {
			return p
		}
	}
	return ""
}

// canonicalizeQuery resolves every incoming query key to its canonical
// parameter name, merging separator aliases into the same dimension (so
// not-path and not_path OR together), and rejects any unknown key with a 400.
// It returns the canonicalized values on success; on an unknown key it writes
// the 400 (with a did-you-mean when the key is close to a real one) and returns
// ok=false so the caller stops. An empty-valued param is preserved as-is: the
// downstream nonEmptyQuery/Get contract already treats "" as unset, so ?type=
// stays a no-op rather than an "unknown key".
func canonicalizeQuery(w http.ResponseWriter, raw url.Values) (url.Values, bool) {
	out := make(url.Values, len(raw))
	for key, vals := range raw {
		canon := canonicalParam(key)
		if canon == "" {
			msg := fmt.Sprintf("unknown query parameter %q", key)
			if s := suggestParam(key); s != "" {
				msg += fmt.Sprintf(`; did you mean %q?`, s)
			}
			http.Error(w, msg, http.StatusBadRequest)
			return nil, false
		}
		out[canon] = append(out[canon], vals...)
	}
	return out, true
}

// suggestParam returns the accepted parameter (canonical spelling) nearest to an
// unknown key by Levenshtein distance, or "" when nothing is close enough. The
// closeness bound scales with the key length so a short typo suggests but a
// wholly unrelated word (nonsense_param) does not. Comparison is separator-
// insensitive, so notpath suggests not-path.
func suggestParam(key string) string {
	norm := func(s string) string { return strings.ReplaceAll(s, "-", "_") }
	nk := norm(key)
	best, bestDist := "", 1<<30
	for _, p := range searchParams {
		d := levenshtein(nk, norm(p))
		if d < bestDist {
			best, bestDist = p, d
		}
	}
	// Only suggest a genuinely near miss: within a third of the key length,
	// floor of 2 for very short keys. A distant word gets no suggestion.
	bound := len(nk) / 3
	if bound < 2 {
		bound = 2
	}
	if bestDist <= bound {
		return best
	}
	return ""
}

// levenshtein is the standard edit distance between two ASCII-ish strings, used
// only for the small closed did-you-mean set (never a hot path).
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func (s *searchService) handle(w http.ResponseWriter, r *http.Request) {
	// Canonicalize and validate keys BEFORE reading any value: an unknown query
	// param is a 400 (fail closed), and hyphen/underscore separators are aliased
	// so a caller cannot guess the wrong one. All value reads below come from the
	// canonicalized set, so an alias is honoured with the documented behavior and
	// its errors quote the canonical name.
	query, ok := canonicalizeQuery(w, r.URL.Query())
	if !ok {
		return
	}

	q := query.Get("q")
	if q == "" {
		// Missing/empty q is a client error, never a 200 with everything: an
		// empty query is "no query", not "match all".
		http.Error(w, `missing required query parameter "q"`, http.StatusBadRequest)
		return
	}

	k := 5 // CLI default (--k)
	if v := query.Get("k"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			http.Error(w, `"k" must be a non-negative integer`, http.StatusBadRequest)
			return
		}
		k = n
	}

	var halfLife float64
	if v := query.Get("half_life"); v != "" {
		hl, err := strconv.ParseFloat(v, 64)
		if err != nil || hl < 0 {
			http.Error(w, `"half_life" must be a non-negative number of days`, http.StatusBadRequest)
			return
		}
		halfLife = hl
	}

	// decay_floor: the #65 lower clamp on the recency multiplier. An OMITTED or
	// EMPTY param takes the shared search.DefaultDecayFloor — the same default
	// the CLI's --decay-floor uses — so the two surfaces cannot drift. A present
	// value is validated in [0, 1] with #72's wording (a floor > 1 turns the
	// "lower clamp" into a flat gain; a floor < 0 re-enables the #65 inversion);
	// out of range is a 400, never a silently-ignored 200.
	decayFloor := search.DefaultDecayFloor
	if v := query.Get("decay_floor"); v != "" {
		df, err := strconv.ParseFloat(v, 64)
		if err != nil || df < 0 || df > 1 {
			http.Error(w, `"decay_floor" must be in [0, 1]`, http.StatusBadRequest)
			return
		}
		decayFloor = df
	}

	// min_relevance: the #65 raw-cosine floor applied BEFORE decay reorders.
	// Default 0 (admit everything). A present value is validated non-negative and
	// out-of-range is a 400, never silently ignored.
	var minRelevance float64
	if v := query.Get("min_relevance"); v != "" {
		mr, err := strconv.ParseFloat(v, 64)
		if err != nil || mr < 0 {
			http.Error(w, `"min_relevance" must be a non-negative number`, http.StatusBadRequest)
			return
		}
		minRelevance = mr
	}

	// lexical_gate: the #66 term-wise lexical gate (--lexical-gate on the CLI),
	// off by default. It accepts BOTH separator spellings — lexical_gate and
	// lexical-gate — from the start: #77 (the unknown-parameter card that settles
	// the separator convention) has not merged, and the card mandates supporting
	// both so parity holds whichever way #77 lands. A present value must be a
	// boolean (strconv.ParseBool: 1/t/true/0/f/false, etc.); a non-boolean is a
	// 400 in the house wording, never a silently-ignored 200. When both spellings
	// are present the underscore form wins (the canonical name), then the dash.
	var lexicalGate bool
	if v, ok := lookupGateParam(r); ok {
		g, err := strconv.ParseBool(v)
		if err != nil {
			http.Error(w, `"lexical_gate" must be a boolean`, http.StatusBadRequest)
			return
		}
		lexicalGate = g
	}

	store, meta, bundle, modTime, err := s.ensureFresh()
	if err != nil {
		if errors.Is(err, errNoIndex) {
			// Fail closed with an actionable message, exactly the CLI's guidance.
			http.Error(w,
				"no index at "+s.indexPath()+" (run 'okfctl-search index build' first)",
				http.StatusServiceUnavailable)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// #68 landed the repeatable + negating scope grammar on the CLI (path/type/
	// tag are now StringArray flags, plus not-path/not-type/not-tag). The card's
	// ordering dependency says to mirror that grammar here rather than invent one:
	// each param repeats and repeats OR together (Go's url.Query returns every
	// occurrence), matching the CLI exactly. nonEmptyQuery drops empty-string
	// values so `?type=` reads as unset, the CLI's nonEmpty contract. Reads come
	// from the canonicalized `query`, so a hyphen/underscore alias (e.g. not_path)
	// has already been merged into its canonical dimension.
	filter := search.Filter{
		PathPrefixes:    nonEmptyQuery(query["path"]),
		Types:           nonEmptyQuery(query["type"]),
		Tags:            nonEmptyQuery(query["tag"]),
		NotPathPrefixes: nonEmptyQuery(query["not-path"]),
		NotTypes:        nonEmptyQuery(query["not-type"]),
		NotTags:         nonEmptyQuery(query["not-tag"]),
	}
	opts := search.QueryOptions{Filter: filter}
	// Filters, decay, and the lexical gate resolve against the live-bundle
	// meta/prose; pass meta only when something actually needs it, matching the
	// CLI's needBundle guard (cmd/okfctl-search/main.go) so the
	// unfiltered/undecayed/ungated path is byte-for-byte Query(). The gate term is
	// the one the card flags as missing here: the CLI's needBundle already
	// includes `|| lexicalGate`, and without it a gated request would build the
	// gate against nil meta and silently no-op.
	if !filter.IsEmpty() || halfLife > 0 || minRelevance > 0 || lexicalGate {
		opts.Meta = meta
	}
	if lexicalGate {
		// #66 lexical gate: built via the shared search.BuildLexicalGate over the
		// resident live bundle, the SAME constructor the CLI calls, so a gated
		// query cannot rank differently over HTTP than on the CLI for the same
		// bundle (the cross-surface parity the card mandates). The engine degrades
		// to pure semantic on an all-stopword or over-broad query, so the gated
		// body is byte-identical to the ungated one on exactly the query shapes the
		// CLI degrades on.
		opts.LexicalGate = search.BuildLexicalGate(bundle, q)
	}
	if halfLife > 0 || minRelevance > 0 {
		// Post-ranking recency decay, mirroring the exact DecayOptions the CLI
		// builds (cmd/okfctl-search/main.go) so the two surfaces cannot disagree
		// for the same query and bundle: MinRelevance is a floor on RAW cosine
		// (a sub-floor node is dropped before decay can reorder anything), and
		// DecayFloor clamps the recency multiplier itself so an old-but-relevant
		// node can be demoted but never crushed to zero below a mediocre fresh
		// one (#65). DecayFloor defaults to the shared search.DefaultDecayFloor.
		opts.Decay = &search.DecayOptions{
			HalfLifeDays: halfLife,
			Now:          s.now(),
			MinRelevance: minRelevance,
			DecayFloor:   decayFloor,
		}
	}

	res, err := search.QueryWith(store, s.embedder, q, k, opts)
	if err != nil {
		// A model mismatch between the embedder and the on-disk index is a
		// server-config problem, not a bad request: surface it as 503 with the
		// store's own guidance rather than a misleading 200.
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	out := searchResponse{
		Schema:    1,
		Query:     q,
		Model:     store.Model,
		IndexedAt: modTime.UTC().Format(time.RFC3339),
		Results:   make([]searchResult, 0, len(res)),
	}
	for _, rr := range res {
		out.Results = append(out.Results, searchResult{Score: rr.Score, Path: rr.Path, Snippet: rr.Snippet})
	}
	writeJSON(w, out)
}

// lookupGateParam returns the raw value of the lexical-gate query param and
// whether it was present, accepting BOTH separator spellings — lexical_gate
// (canonical) and lexical-gate — so parity holds regardless of the separator
// convention #77 eventually settles. The underscore form wins when both are
// present. An empty-string value (e.g. `?lexical_gate=`) is treated as PRESENT so
// it is validated as a (non-)boolean rather than silently ignored, matching the
// loud-failure discipline the other params follow.
func lookupGateParam(r *http.Request) (string, bool) {
	q := r.URL.Query()
	if q.Has("lexical_gate") {
		return q.Get("lexical_gate"), true
	}
	if q.Has("lexical-gate") {
		return q.Get("lexical-gate"), true
	}
	return "", false
}

// nonEmptyQuery drops empty-string values from a repeated query param, the HTTP
// mirror of the CLI's nonEmpty (cmd/okfctl-search/main.go): an empty value
// (e.g. `?type=`) is not a real constraint and must behave as the no-op path,
// identical to omitting the param. Returns nil for an all-empty (or nil) input
// so the corresponding Filter dimension reads as unset.
func nonEmptyQuery(vs []string) []string {
	out := vs[:0:0] // fresh backing array; never alias the query's slice
	for _, v := range vs {
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
