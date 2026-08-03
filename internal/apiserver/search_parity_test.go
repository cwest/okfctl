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
	"os"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
)

// TestSearch_CrossSurfaceParity is THE durable anti-drift test the card mandates.
// It drives a query set through BOTH surfaces — the HTTP handler and the CLI
// oracle (search.QueryWith with the shared option builders) — with the same
// params, asserting the ranked path+score+snippet triple matches. This is the
// test that would have caught the lexical-gate gap AND the #74/#76 decay-floor
// divergence, which were the same shape of bug: a knob wired on one surface only.
//
// The table covers gate on/off, filters, half_life, decay_floor, min_relevance,
// and combinations. The oracle is built from the SAME shared constructors the
// handler uses (search.BuildLexicalGate, search.DefaultDecayFloor), so a literal
// duplicated on one side cannot make this pass while the surfaces disagree.
func TestSearch_CrossSurfaceParity(t *testing.T) {
	dir := gateBundleDir(t)
	nowInst := fixedNow(t) // pin the clock BEFORE NewHandler so both sides decay from the same instant
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)
	h := NewHandler(b, e)

	// buildGate mirrors the handler's gate construction via the shared builder.
	buildGate := func(query string) *search.LexicalGateOptions {
		bl, err := okf.Load(dir)
		if err != nil {
			t.Fatalf("load bundle: %v", err)
		}
		return search.BuildLexicalGate(bl, query)
	}

	cases := []struct {
		name   string
		query  string
		k      int
		params string                     // extra HTTP query params (no leading &)
		opts   func() search.QueryOptions // matching CLI oracle opts
	}{
		{
			name:  "plain",
			query: "wine acidity freshness",
			k:     6,
			opts:  func() search.QueryOptions { return search.QueryOptions{} },
		},
		{
			name:   "gate_on",
			query:  "wine acidity freshness",
			k:      6,
			params: "lexical_gate=true",
			opts: func() search.QueryOptions {
				return search.QueryOptions{Meta: liveMeta(t, dir), LexicalGate: buildGate("wine acidity freshness")}
			},
		},
		{
			name:   "gate_on_dash_separator",
			query:  "wine acidity freshness",
			k:      6,
			params: "lexical-gate=true",
			opts: func() search.QueryOptions {
				return search.QueryOptions{Meta: liveMeta(t, dir), LexicalGate: buildGate("wine acidity freshness")}
			},
		},
		{
			name:   "gate_off_explicit",
			query:  "wine acidity freshness",
			k:      6,
			params: "lexical_gate=false",
			opts:   func() search.QueryOptions { return search.QueryOptions{} },
		},
		{
			name:   "type_filter",
			query:  "acidity",
			k:      6,
			params: "type=Concept",
			opts: func() search.QueryOptions {
				return search.QueryOptions{Meta: liveMeta(t, dir), Filter: search.Filter{Types: []string{"Concept"}}}
			},
		},
		{
			name:   "path_filter",
			query:  "acidity",
			k:      6,
			params: "path=wine/",
			opts: func() search.QueryOptions {
				return search.QueryOptions{Meta: liveMeta(t, dir), Filter: search.Filter{PathPrefixes: []string{"wine/"}}}
			},
		},
		{
			name:   "gate_plus_path_filter",
			query:  "wine acidity freshness",
			k:      6,
			params: "lexical_gate=true&path=wine/",
			opts: func() search.QueryOptions {
				return search.QueryOptions{
					Meta:        liveMeta(t, dir),
					Filter:      search.Filter{PathPrefixes: []string{"wine/"}},
					LexicalGate: buildGate("wine acidity freshness"),
				}
			},
		},
		{
			name:   "gate_plus_half_life_and_decay_floor",
			query:  "wine acidity freshness",
			k:      6,
			params: "lexical_gate=true&half_life=30&decay_floor=0.5",
			opts: func() search.QueryOptions {
				return search.QueryOptions{
					Meta:        liveMeta(t, dir),
					LexicalGate: buildGate("wine acidity freshness"),
					Decay: &search.DecayOptions{
						HalfLifeDays: 30,
						Now:          nowInst,
						MinRelevance: 0,
						DecayFloor:   0.5,
					},
				}
			},
		},
		{
			name:   "half_life_default_floor",
			query:  "wine acidity freshness",
			k:      6,
			params: "half_life=30",
			opts: func() search.QueryOptions {
				return search.QueryOptions{
					Meta: liveMeta(t, dir),
					Decay: &search.DecayOptions{
						HalfLifeDays: 30,
						Now:          nowInst,
						MinRelevance: 0,
						DecayFloor:   search.DefaultDecayFloor,
					},
				}
			},
		},
		{
			name:   "min_relevance",
			query:  "wine acidity freshness",
			k:      6,
			params: "min_relevance=0.01",
			opts: func() search.QueryOptions {
				return search.QueryOptions{
					Meta: liveMeta(t, dir),
					Decay: &search.DecayOptions{
						HalfLifeDays: 0,
						Now:          nowInst,
						MinRelevance: 0.01,
						DecayFloor:   search.DefaultDecayFloor,
					},
				}
			},
		},
		{
			name:   "gate_plus_min_relevance_plus_half_life",
			query:  "wine acidity freshness",
			k:      6,
			params: "lexical_gate=true&half_life=30&min_relevance=0.01",
			opts: func() search.QueryOptions {
				return search.QueryOptions{
					Meta:        liveMeta(t, dir),
					LexicalGate: buildGate("wine acidity freshness"),
					Decay: &search.DecayOptions{
						HalfLifeDays: 30,
						Now:          nowInst,
						MinRelevance: 0.01,
						DecayFloor:   search.DefaultDecayFloor,
					},
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := "/api/v1/search?q=" + qesc(tc.query) + "&k=" + ftoa(float64(tc.k))
			if tc.params != "" {
				target += "&" + tc.params
			}
			got := mustSearch(t, h, target, http.StatusOK)
			want := cliEquivalent(t, dir, e, tc.query, tc.k, tc.opts())
			assertSameRanking(t, got.Results, want)
		})
	}
}

// TestSearch_CrossSurfaceParity_RealCorpus runs the parity harness over the REAL
// knowledge base (a copy, so no index is written to source), the layer fixtures
// cannot substitute for (AGENTS.md layer 3). It asserts the gated HTTP ranking
// equals the gated CLI ranking on every node, and that the gate is not a vacuous
// no-op on the corpus (the positive control). Skipped unless OKFCTL_EVAL_CORPUS
// points at a bundle dir. §4.1 semantic query, upstream v0.2 SPEC.md.
//
//	OKFCTL_EVAL_CORPUS=~/src/knowledge-base/bundles/knowledge \
//	go test ./internal/apiserver/ -run TestSearch_CrossSurfaceParity_RealCorpus -v
func TestSearch_CrossSurfaceParity_RealCorpus(t *testing.T) {
	corpus := envCorpus(t)
	if corpus == "" {
		t.Skip("set OKFCTL_EVAL_CORPUS to a bundle dir to run the real-corpus parity harness")
	}
	dir := copyBundle(t, corpus)
	e := buildIndex(t, dir)
	b := loadBundle(t, dir)
	h := NewHandler(b, e)

	nodeCount := len(b.Nodes)
	if nodeCount < 50 {
		t.Fatalf("real corpus has only %d nodes; expected the full KB (>= 50)", nodeCount)
	}

	// The card's worked example: this query is one the gate moves on the corpus.
	const q = "hormesis training stress"
	k := 5

	gateOpts := func() search.QueryOptions {
		bl, err := okf.Load(dir)
		if err != nil {
			t.Fatalf("load bundle: %v", err)
		}
		return search.QueryOptions{Meta: liveMeta(t, dir), LexicalGate: search.BuildLexicalGate(bl, q)}
	}

	// Positive control: the gate must actually move the ranking on this query,
	// or a byte-identical parity assertion proves nothing.
	ungated := cliEquivalent(t, dir, e, q, k, search.QueryOptions{})
	gated := cliEquivalent(t, dir, e, q, k, gateOpts())
	if samePaths(ungated, gated) {
		t.Fatalf("real-corpus positive control failed: gate is a no-op on %q", q)
	}
	t.Logf("real-corpus parity: corpus=%s nodes=%d, gate moves %q", corpus, nodeCount, q)

	// The two surfaces must agree on the gated ranking.
	got := mustSearch(t, h, "/api/v1/search?q="+qesc(q)+"&k="+ftoa(float64(k))+"&lexical_gate=true", http.StatusOK)
	assertSameRanking(t, got.Results, gated)

	// And on the ungated ranking (regression guard on the additive-only claim).
	gotUngated := mustSearch(t, h, "/api/v1/search?q="+qesc(q)+"&k="+ftoa(float64(k)), http.StatusOK)
	assertSameRanking(t, gotUngated.Results, ungated)
}

// envCorpus reads OKFCTL_EVAL_CORPUS, expanding a leading ~ to the home dir so a
// literal ~/src/... path from a shell env works under the Go test runner.
func envCorpus(t *testing.T) string {
	t.Helper()
	c := os.Getenv("OKFCTL_EVAL_CORPUS")
	if strings.HasPrefix(c, "~/") {
		if home := os.Getenv("HOME"); home != "" {
			return home + c[1:]
		}
	}
	return c
}

// samePaths reports whether two result lists have identical ranked paths.
func samePaths(a, b []search.Result) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path {
			return false
		}
	}
	return true
}
