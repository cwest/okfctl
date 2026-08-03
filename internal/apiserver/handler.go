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

// Package apiserver builds the read-only HTTP handler that serves an OKF bundle
// as plain REST + JSON under /api/v1. It is a *consumption* layer: every value
// it returns is derived from what internal/okf already parses (okf.BuildGraph,
// Bundle.OkfVersion), so the API can never disagree with the CLI's own view of
// the same bundle. It invents zero OKF spec rules and stays at the spec floor —
// notably it does not promote the KB-house `authority` frontmatter convention to
// a first-class API concept (an over-conformance trap; see AGENTS.md and §2.9 of
// docs/plans/2026-08-02-okfctl-api.md). Testable via httptest without binding a
// port, the same shape cmd/serve.go's newServeHandler establishes.
package apiserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
)

// nameCount is one {name, count} row in a stats aggregation (types,
// neighborhoods). Rows are emitted in a stable, deterministic order (sorted by
// name), matching the "sorted, not map-order" discipline graph export and
// search --json already follow.
type nameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// statsResponse is the GET /api/v1/stats body (§2.1 of the proposal). Every
// field is derived from the loaded bundle; nothing is hardcoded. okf_version is
// read straight from the bundle's own .okf sidecar so a future v0.2 corpus
// reports itself correctly without a code change here.
type statsResponse struct {
	Schema        int             `json:"schema"`
	BundleRoot    string          `json:"bundle_root"`
	OkfVersion    string          `json:"okf_version"`
	NodeCount     int             `json:"node_count"`
	EdgeCount     int             `json:"edge_count"`
	OrphanCount   int             `json:"orphan_count"`
	Neighborhoods []nameCount     `json:"neighborhoods"`
	Types         []nameCount     `json:"types"`
	Status        []nameCount     `json:"status"`
	Epistemic     epistemicReport `json:"epistemic"`
	GeneratedAt   string          `json:"generated_at"`
	IndexHealthy  bool            `json:"index_healthy"`
}

// epistemicReport surfaces the observed distribution of the `epistemic` grade
// key. `epistemic` is a §11 UNKNOWN key (not an OKF-defined field): the API
// RECOGNIZES it and reports whatever values appear so a curator can spot an
// outlier or typo, but it NEVER enum-gates or rejects a value — over-conformance
// on an unknown key is a spec violation (AGENTS.md; §11). This mirrors the
// treatment analyze landed in okf.EpistemicReport. Untagged counts nodes with
// no epistemic key.
type epistemicReport struct {
	Distribution []epistemicCount `json:"distribution"`
	Untagged     int              `json:"untagged"`
}

// epistemicCount is one observed epistemic value and how many nodes carry it.
type epistemicCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// graphResponse is the GET /api/v1/graph body. It EMBEDS the shared
// okf.BuildGraph derivation (nodes + edges) and enriches each node with its own
// §5.2 generated.at, so the API's graph shape can never disagree with
// `graph export` while still surfacing per-node provenance the CLI graph omits.
type graphResponse struct {
	Nodes []graphNode     `json:"nodes"`
	Edges []okf.GraphEdge `json:"edges"`
}

// graphNode is one /graph node: the shared okf.GraphNode plus the node's real
// §5.2 generated.at (with the §13.1 legacy `timestamp` fallback). This
// generated_at is the NODE's last-meaningful-change instant — DISTINCT from the
// /stats top-level response-clock generated_at. The two never share a JSON
// object (one is a /graph node field, the other a /stats top-level field), so
// there is no key collision and no silent overload. Empty when the node carries
// no §5.2 generated and no legacy timestamp.
type graphNode struct {
	okf.GraphNode
	GeneratedAt string `json:"generated_at"`
}

// now is the clock stats reads for generated_at; overridable in tests.
var now = time.Now

// NewHandler builds the read-only /api/v1 HTTP handler for a loaded bundle:
//
//	GET /api/v1/stats  -> corpus summary (counts, types, neighborhoods, version)
//	GET /api/v1/graph  -> okf.BuildGraph(b) as JSON (the EXACT serializer graph
//	                      export and serve's /graph.json use, so the API's graph
//	                      view can never disagree with the CLI's — §2.4)
//	GET /api/v1/search -> semantic query over the bundle's .okfctl/index.db,
//	                      returning the same score/path/snippet triple the CLI
//	                      prints. Registered only when embedder is non-nil.
//
// The embedder is what /search uses to encode the query; it MUST be the same
// embedder the on-disk index was built under (the store's model guard rejects a
// mismatch). A nil embedder disables /search entirely and leaves /stats and
// /graph byte-identical — the search route is purely additive (the negative
// control the acceptance criteria require).
//
// The bundle is treated as strictly read-only: every route is a GET and no
// handler writes bundle source files (§5).
func NewHandler(b *okf.Bundle, embedder search.Embedder) http.Handler {
	return newHandlerWithLoader(b, embedder, search.Load)
}

// newHandlerWithLoader is NewHandler with an injectable index loader so tests
// can count disk loads (the "unchanged index loads exactly once" negative
// control). Production always passes search.Load.
func newHandlerWithLoader(b *okf.Bundle, embedder search.Embedder, load storeLoader) http.Handler {
	mux := http.NewServeMux()

	// Go 1.22+ method-and-path mux patterns: an exact "GET /api/v1/stats"
	// pattern 404s any other path or method for free, so the fixture's
	// unknown-path and method expectations hold without a catch-all.
	mux.HandleFunc("GET /api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, buildStats(b))
	})

	mux.HandleFunc("GET /api/v1/graph", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, buildGraph(b))
	})

	// /search holds the loaded index + embedder for the process lifetime and
	// stat-and-reloads on index change (§2.7). Registered only when an embedder
	// is available; without one, GET /api/v1/search 404s like any other unknown
	// route, keeping /stats and /graph untouched.
	if embedder != nil {
		s := newSearchService(b.Root, embedder, load)
		mux.HandleFunc("GET /api/v1/search", s.handle)
	}

	return mux
}

// buildStats derives the stats summary from the loaded bundle. Counts,
// orphan-ness, types, and neighborhoods all come from okf.BuildGraph — the same
// derivation lint and graph export use — so stats can never diverge from the
// graph view of the same bundle. The §5.4 status lifecycle and the §11
// epistemic distribution are read from the same concept-node set (b.Nodes) the
// graph nodes are built from, so all four distributions cover identical nodes.
func buildStats(b *okf.Bundle) statsResponse {
	g := okf.BuildGraph(b)

	typeCounts := map[string]int{}
	hoodCounts := map[string]int{}
	statusCounts := map[string]int{}
	orphans := 0
	for _, n := range g.Nodes {
		typeCounts[n.Type]++
		hoodCounts[n.Neighborhood]++
		if n.Orphan {
			orphans++
		}
		// §5.4: lifecycle status; absent ⇒ stable. Read from the underlying
		// node, which the graph node is derived from (b.Nodes keys by path).
		if node, ok := b.Nodes[n.Path]; ok {
			statusCounts[node.Status()]++
		}
	}

	return statsResponse{
		Schema:        1,
		BundleRoot:    b.Root,
		OkfVersion:    b.OkfVersion,
		NodeCount:     len(g.Nodes),
		EdgeCount:     len(g.Edges),
		OrphanCount:   orphans,
		Neighborhoods: sortedNameCounts(hoodCounts),
		Types:         sortedNameCounts(typeCounts),
		Status:        sortedNameCounts(statusCounts),
		Epistemic:     epistemicDistribution(b),
		GeneratedAt:   now().UTC().Format(time.RFC3339),
		IndexHealthy:  indexHealthy(b.Root),
	}
}

// epistemicDistribution counts the observed values of the §11 unknown
// `epistemic` key across concept nodes, ordered count DESCENDING then value
// ASCENDING — the exact treatment analyze landed (okf.analyzeEpistemic), so the
// API and analyze report the same distribution for the same bundle. The key is
// surfaced OBSERVATIONALLY and never enum-gated (over-conformance on an unknown
// key is a spec violation). Untagged counts nodes with no epistemic key.
func epistemicDistribution(b *okf.Bundle) epistemicReport {
	counts := map[string]int{}
	untagged := 0
	for _, n := range b.Nodes {
		if v, ok := n.Epistemic(); ok {
			counts[v]++
		} else {
			untagged++
		}
	}
	dist := make([]epistemicCount, 0, len(counts))
	for v, c := range counts {
		dist = append(dist, epistemicCount{Value: v, Count: c})
	}
	sort.Slice(dist, func(i, j int) bool {
		if dist[i].Count != dist[j].Count {
			return dist[i].Count > dist[j].Count // count descending
		}
		return dist[i].Value < dist[j].Value // then value ascending
	})
	return epistemicReport{Distribution: dist, Untagged: untagged}
}

// buildGraph enriches the shared okf.BuildGraph derivation with each node's own
// §5.2 generated.at (with the §13.1 legacy `timestamp` fallback). The node
// identity/type/orphan fields and the edges are byte-for-byte the shared
// serializer's output, so /graph can never disagree with `graph export` about
// the graph itself; the per-node generated.at is the only addition.
func buildGraph(b *okf.Bundle) graphResponse {
	g := okf.BuildGraph(b)
	out := graphResponse{
		Nodes: make([]graphNode, 0, len(g.Nodes)),
		Edges: g.Edges,
	}
	for _, gn := range g.Nodes {
		gen := ""
		// §5.2 generated.at (with §13.1 timestamp fallback). Empty when the
		// node carries neither — served, never an error.
		if node, ok := b.Nodes[gn.Path]; ok {
			if generation, dated := node.Generated(); dated {
				gen = generation.At.UTC().Format(time.RFC3339)
			}
		}
		out.Nodes = append(out.Nodes, graphNode{GraphNode: gn, GeneratedAt: gen})
	}
	return out
}

// sortedNameCounts turns a name->count map into a name-sorted slice, so the
// response is byte-stable across calls (deterministic order, not map-order).
func sortedNameCounts(m map[string]int) []nameCount {
	out := make([]nameCount, 0, len(m))
	for name, count := range m {
		out = append(out, nameCount{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// indexHealthy reports whether the bundle carries a semantic index on disk
// (§2.1). Increment 1 does not link internal/search, so this is a presence
// check of .okfctl/index.db only — it never builds or loads the index on the
// stats path. Loadable-model/dim verification arrives with the search surface
// (Increment 3), which is when this plugin first links internal/search.
func indexHealthy(root string) bool {
	fi, err := os.Stat(filepath.Join(root, ".okfctl", "index.db"))
	return err == nil && !fi.IsDir()
}

// writeJSON marshals v as indented JSON with the standard content type. It
// mirrors graph export's indented, deterministic serialization so the API's
// bytes match the CLI's for the same resource.
func writeJSON(w http.ResponseWriter, v any) {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(out)
	w.Write([]byte("\n"))
}
