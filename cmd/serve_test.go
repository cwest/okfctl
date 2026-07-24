package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

func TestServeHandler_GraphJSON(t *testing.T) {
	dir := writeGraphFixture(t)
	b, err := okf.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	h := newServeHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/graph.json", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /graph.json = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var g struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
	}
}

func TestServeHandler_IndexPage(t *testing.T) {
	dir := writeGraphFixture(t)
	b, _ := okf.Load(dir)
	h := newServeHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "okfctl graph") {
		t.Fatalf("page missing expected title marker")
	}
}

func TestServeHandler_NotFound(t *testing.T) {
	dir := writeGraphFixture(t)
	b, _ := okf.Load(dir)
	h := newServeHandler(b)

	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /nope = %d, want 404", rec.Code)
	}
}
