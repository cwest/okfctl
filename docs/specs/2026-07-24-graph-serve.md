# Spec — Increment 4: `graph export` + `serve` (web visualizer)

**Status:** Approved · **Owner:** Casey West · **License:** Apache-2.0
**Increment:** 4 (depends only on Increment 1's model + link graph)

## Why

PRD pillar 3: the built-in interactive web visualizer — "the gap neither existing tool fills" (§6.3). Plus a machine-format `graph export` for CI and other tools. Both are read-only presentations of the link graph increment 1 already builds; no new graph math.

## Scope (this increment)

Two cross-cutting read-only verbs:

1. **`graph export [dir] --format json|dot`** — serialize the bundle's node+edge graph deterministically.
2. **`serve [dir] --addr 127.0.0.1:8080`** — a local `net/http` server that serves the graph as JSON and a `go:embed`-ed single-page interactive visualizer.

Out of scope (later increments): `search` (increment 5 is the semantic plugin; core lexical search is not in the roadmap for increment 4), templates (6), registry (7).

## Design decisions

### Model — a pure `internal/okf/graph.go`
- `BuildGraph(b *Bundle) Graph` — no cobra, no `net/http`. Returns a serializable value:
  - `Graph{ Nodes []GraphNode, Edges []GraphEdge }`
  - `GraphNode{ Path, Title, Type, Neighborhood string; Orphan bool }`
  - `GraphEdge{ From, To string }` (in-bundle resolved links only)
- **Orphan flag reuses the existing `inboundCounts(b)`** (the same counter `lint` uses — one inbound source of truth, so `graph` and `lint` can never disagree on what's orphaned). A concept node with 0 inbound (index.md counts as inbound) is `Orphan: true`.
- Reserved files (`index.md`/`log.md`) are NOT graph nodes (consistent with the concept-node model); their outbound links still count toward inbound for orphan detection, exactly as in `lint`.
- **Deterministic:** `Nodes` sorted by path; `Edges` sorted by (from, then to).

### `graph export` — stdlib only
- `--format json` (default): marshal `Graph` to indented JSON, stable field order, sorted. A documented, CI-diffable schema.
- `--format dot`: emit Graphviz DOT text (stdlib string building). Node label = title; orphans styled distinctly (e.g. dashed). `okfctl graph export --format dot | dot -Tsvg > graph.svg` is the SVG path.
- **No native `svg` format** (DOT-pipe covers it; no runtime Graphviz dependency, no hand-rolled SVG layout). An unknown `--format` errors clearly.
- Read-only; never mutates.

### `serve` — embedded visualizer
- `net/http` server; **binds loopback by default** (`127.0.0.1:8080`), `--addr` overridable. It's a local viewer, not a public service.
- Endpoints:
  - `GET /` → the `go:embed`-ed `index.html` (single self-contained page).
  - `GET /graph.json` → `BuildGraph(b)` marshaled (the same serializer as `graph export --format json`).
- **Front-end: vanilla JS, single embedded `index.html`** (a small self-contained force-directed renderer, no npm/Node build step). Click a node to read it, follow edges, orphans highlighted, filter by type/neighborhood. Assets `go:embed`-ed into the binary — no separate install (PRD requirement).
- Loads the bundle once at startup from the arg dir; read-only.

### Boundary
- `internal/okf` stays cobra-free and `net/http`-free (pure model + serializer).
- `cmd/graph.go` = cobra + format dispatch; `cmd/serve.go` = cobra + HTTP + `go:embed`.

## Done when
- `graph export --format json` and `--format dot` both emit deterministic, sorted output; unknown format errors.
- Orphan flags match `lint`'s orphan findings (shared `inboundCounts`).
- `serve` starts, `GET /graph.json` returns the graph, `GET /` returns the embedded page; loopback-bound by default.
- `internal/okf` imports neither cobra nor net/http.
- Full `-race` suite green; gofmt/vet/`go mod tidy -diff` clean; stdlib-only (no new deps).
- Whole-increment review exercises the built binary E2E: export both formats, curl the running server's endpoints.
