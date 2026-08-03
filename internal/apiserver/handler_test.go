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
	writeFiles(t, dir, files)
	return dir
}

// writeFiles lays down each rel->content file under dir, creating parent dirs.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
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
	h := NewHandler(b, nil)

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

// §5.4: /stats surfaces the status lifecycle distribution. A v0.2 bundle with
// explicit draft/deprecated statuses reports them alongside stable; nodes with
// no status default to stable (§5.4 absent ⇒ stable). Sorted by name ascending,
// like types/neighborhoods.
func TestStats_StatusLifecycleDistribution_Section5_4(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		".okf":        "okf_version: 0.2\n",
		"index.md":    "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n- [D](wine/d.md)\n- [Dep](wine/dep.md)\n- [S](wine/s.md)\n",
		"wine/d.md":   "---\ntype: Concept\ntitle: D\nstatus: draft\n---\n\n# D\n\nBody.\n",
		"wine/dep.md": "---\ntype: Concept\ntitle: Dep\nstatus: deprecated\n---\n\n# Dep\n\nBody.\n",
		// no status -> stable (§5.4 default).
		"wine/s.md": "---\ntype: Concept\ntitle: S\n---\n\n# S\n\nBody.\n",
	}
	writeFiles(t, dir, files)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	var got statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	want := []nameCount{{Name: "deprecated", Count: 1}, {Name: "draft", Count: 1}, {Name: "stable", Count: 1}}
	if len(got.Status) != len(want) {
		t.Fatalf("status = %+v, want %+v", got.Status, want)
	}
	for i := range want {
		if got.Status[i] != want[i] {
			t.Errorf("status[%d] = %+v, want %+v", i, got.Status[i], want[i])
		}
	}
}

// Negative control (§5.4): a v0.1 bundle with no status key anywhere still
// serves correctly — every node counts as stable, nothing is rejected.
func TestStats_StatusDefaultsToStableForV01Bundle_Section5_4(t *testing.T) {
	dir := writeBundle(t) // the v0.1 fixture: 3 nodes, none carry `status`.
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	var got statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	want := []nameCount{{Name: "stable", Count: 3}}
	if len(got.Status) != len(want) || got.Status[0] != want[0] {
		t.Errorf("status = %+v, want %+v (all stable)", got.Status, want)
	}
}

// §11: /stats surfaces the epistemic value distribution OBSERVATIONALLY —
// ordered count desc then value asc, matching analyze. `epistemic` is an unknown
// key; an out-of-any-enum value is REPORTED, never rejected. Untagged counts
// nodes with no epistemic key.
func TestStats_EpistemicDistributionObservational_Section11(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		".okf":       "okf_version: 0.2\n",
		"index.md":   "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n- [V1](wine/v1.md)\n- [V2](wine/v2.md)\n- [D1](wine/d1.md)\n- [X1](wine/x1.md)\n- [U1](wine/u1.md)\n",
		"wine/v1.md": "---\ntype: Concept\ntitle: V1\nepistemic: verified\n---\n\n# V1\n\nBody.\n",
		"wine/v2.md": "---\ntype: Concept\ntitle: V2\nepistemic: verified\n---\n\n# V2\n\nBody.\n",
		"wine/d1.md": "---\ntype: Concept\ntitle: D1\nepistemic: documented\n---\n\n# D1\n\nBody.\n",
		// an unknown/typo epistemic value must be surfaced, never rejected (§11).
		"wine/x1.md": "---\ntype: Concept\ntitle: X1\nepistemic: made-up-grade\n---\n\n# X1\n\nBody.\n",
		// no epistemic key -> untagged.
		"wine/u1.md": "---\ntype: Concept\ntitle: U1\n---\n\n# U1\n\nBody.\n",
	}
	writeFiles(t, dir, files)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	var got statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	// count desc, then value asc: verified(2), then documented(1), made-up-grade(1).
	want := []epistemicCount{
		{Value: "verified", Count: 2},
		{Value: "documented", Count: 1},
		{Value: "made-up-grade", Count: 1},
	}
	if len(got.Epistemic.Distribution) != len(want) {
		t.Fatalf("epistemic distribution = %+v, want %+v", got.Epistemic.Distribution, want)
	}
	for i := range want {
		if got.Epistemic.Distribution[i] != want[i] {
			t.Errorf("epistemic[%d] = %+v, want %+v", i, got.Epistemic.Distribution[i], want[i])
		}
	}
	if got.Epistemic.Untagged != 1 {
		t.Errorf("epistemic untagged = %d, want 1 (u1.md)", got.Epistemic.Untagged)
	}
}

func TestStats_TypesAndNeighborhoodsSortedWithCounts(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

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
	h := NewHandler(b, nil)

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
	h2 := NewHandler(b2, nil)
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
	h := NewHandler(b, nil)
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

func TestGraph_EmbedsBuildGraphNodesAndEdges(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/graph", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/graph = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	// The API graph view enriches okf.BuildGraph with a per-node §5.2
	// generated.at, but its node identity/type/orphan fields and its edges must
	// still be the EXACT same derivation as okf.BuildGraph — so the API can never
	// disagree with `graph export` about the shape of the graph itself.
	var got graphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v\n%s", err, rec.Body.String())
	}
	want := okf.BuildGraph(b)
	if len(got.Nodes) != len(want.Nodes) {
		t.Fatalf("graph nodes = %d, want %d", len(got.Nodes), len(want.Nodes))
	}
	for i := range want.Nodes {
		if got.Nodes[i].GraphNode != want.Nodes[i] {
			t.Errorf("node[%d] embedded GraphNode = %+v, want %+v", i, got.Nodes[i].GraphNode, want.Nodes[i])
		}
	}
	wantEdges, _ := json.Marshal(want.Edges)
	gotEdges, _ := json.Marshal(got.Edges)
	if string(gotEdges) != string(wantEdges) {
		t.Errorf("graph edges do not match BuildGraph\n got: %s\nwant: %s", gotEdges, wantEdges)
	}
	if len(got.Nodes) != 3 {
		t.Errorf("graph nodes = %d, want 3", len(got.Nodes))
	}
	if len(got.Edges) != 1 {
		t.Errorf("graph edges = %d, want 1", len(got.Edges))
	}
}

// §5.2 / §13.1: each /graph node carries the node's own generated.at, distinct
// from the /stats response-clock generated_at. Positive control: a node with a
// `generated: {by, at}` mapping surfaces that instant. §13.1 fallback: a v0.1
// node with a legacy `timestamp` surfaces it. Absent both -> empty string
// (served, never an error).
func TestGraph_PerNodeGeneratedAt_Section5_2And13_1(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		".okf":     "okf_version: 0.2\n",
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# Index\n\n- [Gen](wine/gen.md)\n- [Legacy](wine/legacy.md)\n- [Bare](wine/bare.md)\n",
		// v0.2 provenance: real §5.2 generated.at.
		"wine/gen.md": "---\ntype: Concept\ntitle: Gen\ngenerated:\n  by: human:casey\n  at: 2026-03-04T00:00:00Z\n---\n\n# Gen\n\nBody.\n",
		// v0.1 §13.1 legacy timestamp fallback.
		"wine/legacy.md": "---\ntype: Concept\ntitle: Legacy\ntimestamp: 2025-11-02T00:00:00Z\n---\n\n# Legacy\n\nBody.\n",
		// no provenance at all -> empty generated.at, still served.
		"wine/bare.md": "---\ntype: Concept\ntitle: Bare\n---\n\n# Bare\n\nBody.\n",
	}
	writeFiles(t, dir, files)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/graph", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/graph = %d, want 200", rec.Code)
	}
	var got graphResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v\n%s", err, rec.Body.String())
	}
	gen := map[string]string{}
	for _, n := range got.Nodes {
		gen[n.Path] = n.GeneratedAt
	}
	if got, want := gen["wine/gen.md"], "2026-03-04T00:00:00Z"; got != want {
		t.Errorf("gen.md generated_at = %q, want %q (§5.2)", got, want)
	}
	if got, want := gen["wine/legacy.md"], "2025-11-02T00:00:00Z"; got != want {
		t.Errorf("legacy.md generated_at = %q, want %q (§13.1 timestamp fallback)", got, want)
	}
	if got := gen["wine/bare.md"]; got != "" {
		t.Errorf("bare.md generated_at = %q, want empty (no provenance)", got)
	}
}

// The per-node §5.2 generated.at and the /stats response-clock generated_at must
// be unambiguous: they never share a JSON object (one is a /graph node field,
// the other a /stats top-level field), so there is no key collision.
func TestGeneratedAt_NoKeyCollisionAcrossSurfaces(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

	// /stats has a top-level generated_at (the response clock) and NO per-node
	// generated_at.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil))
	var stats map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("stats not valid JSON: %v", err)
	}
	if _, ok := stats["generated_at"]; !ok {
		t.Errorf("stats missing top-level generated_at (response clock)")
	}

	// /graph nodes carry generated_at; the /graph body has NO top-level
	// generated_at, so the two meanings never collide in one object.
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/v1/graph", nil))
	var graph map[string]json.RawMessage
	if err := json.Unmarshal(rec2.Body.Bytes(), &graph); err != nil {
		t.Fatalf("graph not valid JSON: %v", err)
	}
	if _, ok := graph["generated_at"]; ok {
		t.Errorf("/graph must not carry a top-level generated_at (that is the /stats response clock)")
	}
}

func TestHandler_UnknownPath404(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /api/v1/nope = %d, want 404", rec.Code)
	}
}

func TestHandler_Deterministic(t *testing.T) {
	dir := writeBundle(t)
	b := loadBundle(t, dir)
	h := NewHandler(b, nil)

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
