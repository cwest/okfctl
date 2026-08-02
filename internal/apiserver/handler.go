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
	Schema        int         `json:"schema"`
	BundleRoot    string      `json:"bundle_root"`
	OkfVersion    string      `json:"okf_version"`
	NodeCount     int         `json:"node_count"`
	EdgeCount     int         `json:"edge_count"`
	OrphanCount   int         `json:"orphan_count"`
	Neighborhoods []nameCount `json:"neighborhoods"`
	Types         []nameCount `json:"types"`
	GeneratedAt   string      `json:"generated_at"`
	IndexHealthy  bool        `json:"index_healthy"`
}

// now is the clock stats reads for generated_at; overridable in tests.
var now = time.Now

// NewHandler builds the read-only /api/v1 HTTP handler for a loaded bundle:
//
//	GET /api/v1/stats  -> corpus summary (counts, types, neighborhoods, version)
//	GET /api/v1/graph  -> okf.BuildGraph(b) as JSON (the EXACT serializer graph
//	                      export and serve's /graph.json use, so the API's graph
//	                      view can never disagree with the CLI's — §2.4)
//
// The bundle is treated as strictly read-only: every route is a GET and no
// handler writes bundle source files (§5).
func NewHandler(b *okf.Bundle) http.Handler {
	mux := http.NewServeMux()

	// Go 1.22+ method-and-path mux patterns: an exact "GET /api/v1/stats"
	// pattern 404s any other path or method for free, so the fixture's
	// unknown-path and method expectations hold without a catch-all.
	mux.HandleFunc("GET /api/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, buildStats(b))
	})

	mux.HandleFunc("GET /api/v1/graph", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, okf.BuildGraph(b))
	})

	return mux
}

// buildStats derives the stats summary from the loaded bundle. Counts,
// orphan-ness, types, and neighborhoods all come from okf.BuildGraph — the same
// derivation lint and graph export use — so stats can never diverge from the
// graph view of the same bundle.
func buildStats(b *okf.Bundle) statsResponse {
	g := okf.BuildGraph(b)

	typeCounts := map[string]int{}
	hoodCounts := map[string]int{}
	orphans := 0
	for _, n := range g.Nodes {
		typeCounts[n.Type]++
		hoodCounts[n.Neighborhood]++
		if n.Orphan {
			orphans++
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
		GeneratedAt:   now().UTC().Format(time.RFC3339),
		IndexHealthy:  indexHealthy(b.Root),
	}
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
