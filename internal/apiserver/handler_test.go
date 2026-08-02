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
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// writeBundle lays down a small OKF bundle: two Concept nodes in the wine
// neighborhood (a links to b; index links to a) plus one Playbook, so stats has
// distinct types and neighborhoods to aggregate.
func writeBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".okf": "okf_version: 0.1\n",
		// index links only to wine/a; wine/b is reachable from a; method/p is
		// linked by nobody -> the sole orphan.
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# Index\n\n- [A](wine/a.md)\n",
		// wine/a carries a KB-house `authority` key (VERIFIED) — the negative
		// control proving stats drops it even when a node actually declares it.
		"wine/a.md":   "---\ntype: Concept\ntitle: Alpha\nauthority: VERIFIED\n---\n\n# Alpha\n\nSee [Beta](b.md).\n",
		"wine/b.md":   "---\ntype: Concept\ntitle: Beta\n---\n\n# Beta\n\nBody.\n",
		"method/p.md": "---\ntype: Playbook\ntitle: Play\n---\n\n# Play\n\nBody.\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return dir
}

func loadBundle(t *testing.T, dir string) *okf.Bundle {
	t.Helper()
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	return b
}

func TestStats_Shape(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/stats = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var got statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v\n%s", err, rec.Body.String())
	}
	if got.Schema != 1 {
		t.Errorf("schema = %d, want 1", got.Schema)
	}
	// okf_version comes from the bundle's own .okf sidecar (§12 / §2.1), not a
	// hardcoded constant, so a v0.2 corpus reports itself without a code change.
	if got.OkfVersion != "0.1" {
		t.Errorf("okf_version = %q, want 0.1 (the bundle's own .okf)", got.OkfVersion)
	}
	if got.NodeCount != 3 {
		t.Errorf("node_count = %d, want 3", got.NodeCount)
	}
	if got.EdgeCount != 1 {
		t.Errorf("edge_count = %d, want 1 (a->b)", got.EdgeCount)
	}
	// method/p.md has no inbound link -> orphan; wine/a (from index) and wine/b
	// (from a) are reachable.
	if got.OrphanCount != 1 {
		t.Errorf("orphan_count = %d, want 1 (method/p.md)", got.OrphanCount)
	}
}

func TestStats_TypesAndNeighborhoodsSortedWithCounts(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var got statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}

	wantTypes := []nameCount{{Name: "Concept", Count: 2}, {Name: "Playbook", Count: 1}}
	if len(got.Types) != len(wantTypes) {
		t.Fatalf("types = %+v, want %+v", got.Types, wantTypes)
	}
	for i := range wantTypes {
		if got.Types[i] != wantTypes[i] {
			t.Errorf("types[%d] = %+v, want %+v", i, got.Types[i], wantTypes[i])
		}
	}

	wantHoods := []nameCount{{Name: "method", Count: 1}, {Name: "wine", Count: 2}}
	if len(got.Neighborhoods) != len(wantHoods) {
		t.Fatalf("neighborhoods = %+v, want %+v", got.Neighborhoods, wantHoods)
	}
	for i := range wantHoods {
		if got.Neighborhoods[i] != wantHoods[i] {
			t.Errorf("neighborhoods[%d] = %+v, want %+v", i, got.Neighborhoods[i], wantHoods[i])
		}
	}
}

// §2.1: index_healthy reflects whether .okfctl/index.db exists; the stats path
// never builds an index. Absent index -> false.
func TestStats_IndexHealthyReflectsIndexPresence(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	var absent statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &absent); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if absent.IndexHealthy {
		t.Errorf("index_healthy = true with no index on disk, want false")
	}

	// Lay down an index.db and reload: index_healthy must flip to true.
	if err := os.MkdirAll(filepath.Join(dir, ".okfctl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".okfctl", "index.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	b2 := loadBundle(t, dir)
	h2 := NewHandler(b2)
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	var present statsResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &present); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if !present.IndexHealthy {
		t.Errorf("index_healthy = false with index.db present, want true")
	}
}

// §2.9 over-conformance trap: authority (a KB-house frontmatter convention, not
// an OKF spec concept) must NOT be promoted to a first-class stats field.
func TestStats_DoesNotPromoteAuthority(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	// Inspect the top-level JSON keys, not a substring of the whole body: the
	// tmpdir bundle_root path itself contains the test name, so a naive body
	// substring check would false-positive.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	for k := range raw {
		if strings.EqualFold(k, "authority") {
			t.Errorf("stats promoted the non-spec 'authority' field to a top-level key")
		}
	}
}

func TestGraph_MatchesBuildGraphSerializer(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/graph = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	// The API graph view must be the EXACT same serialization as
	// okf.BuildGraph, so it can never disagree with `graph export` (§2.4).
	var got okf.Graph
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v\n%s", err, rec.Body.String())
	}
	want := okf.BuildGraph(b)
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("graph does not match BuildGraph serializer\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
	if len(got.Nodes) != 3 {
		t.Errorf("graph nodes = %d, want 3", len(got.Nodes))
	}
	if len(got.Edges) != 1 {
		t.Errorf("graph edges = %d, want 1", len(got.Edges))
	}
}

func TestHandler_UnknownPath404(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/v1/nope = %d, want 404", rec.Code)
	}
}

func TestHandler_Deterministic(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b)

	body := func(path string) string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Body.String()
	}
	for _, p := range []string{"/api/v1/stats", "/api/v1/graph"} {
		if body(p) != body(p) {
			t.Errorf("%s response not byte-identical across calls", p)
		}
	}
}
