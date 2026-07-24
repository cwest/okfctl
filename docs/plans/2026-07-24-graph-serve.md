# TDD Plan — Increment 4: `graph export` + `serve`

**Spec:** [`docs/specs/2026-07-24-graph-serve.md`](../specs/2026-07-24-graph-serve.md)
**Branch:** `topic/graph-serve` off `main` @ 4721595
**Execution:** sequential, strict TDD (RED→GREEN→REFACTOR→commit), per-task ground-truth verification, one whole-increment review before the PR.

Every commit: signed, `Casey West <casey@geeknest.com>`, `%G?=G`, Conventional, `Copyright 2026 Google LLC` headers on new `.go`, no AI attribution.

---

## Task 0 — docs
Commit the spec + this plan. `docs(4): spec + TDD plan for graph export + serve`.

## Task 1 — pure model `BuildGraph`
**RED:** `internal/okf/graph_test.go` — build a fixture bundle (index links A; A links B; C orphan) and assert:
- `BuildGraph(b)` returns nodes sorted by path, each with Path/Title/Type/Neighborhood.
- Orphan flag: C `Orphan:true`, A/B `Orphan:false` (index rescues A; A rescues B).
- Edges sorted by (from,to); only in-bundle resolved links; reserved files not nodes.
**GREEN:** `internal/okf/graph.go` — `Graph`/`GraphNode`/`GraphEdge` types; `BuildGraph` reuses `inboundCounts`, `OutboundLinks`, `nodeTitle`, `neighborhood`. Deterministic sorts.
**Verify:** `go test ./internal/okf -run TestBuildGraph`; full suite green; gofmt/vet.
**Commit:** `feat(okf): BuildGraph pure model (nodes + edges + orphan flags)`.

## Task 2 — `graph export` cmd (json + dot)
**RED:** `cmd/graph_test.go` via `runOKF`:
- `graph export <dir> --format json` → valid JSON, contains node paths + edges; byte-identical across two runs (deterministic).
- `graph export <dir> --format dot` → `digraph` text with node + edge lines.
- default format is json; unknown `--format xml` → non-zero exit + clear error.
**GREEN:** `cmd/graph.go` — `newGraphCmd` with `export` subcommand, `--format` flag; json marshals `BuildGraph`, dot emits Graphviz text (shared serializer helper). Register in `cmd/root.go`.
**Verify:** `go test ./cmd -run TestGraph`; full suite; gofmt/vet.
**Commit:** `feat(cmd): graph export with json and dot formats`.

## Task 3 — `serve` HTTP + embedded page
**RED:** `cmd/serve_test.go` — build the server's `http.Handler` (a `newServeHandler(b)` factory, testable without binding a port) and drive it with `httptest`:
- `GET /graph.json` → 200, `application/json`, body unmarshals to the graph with expected nodes.
- `GET /` → 200, `text/html`, body contains the embedded page marker (e.g. `<title>okfctl graph</title>`).
- unknown path → 404.
**GREEN:**
- `cmd/assets/index.html` — self-contained vanilla-JS force-directed viewer (fetches `/graph.json`, renders nodes/edges on canvas, click-to-read, orphans highlighted, filter by type/neighborhood). No external CDN — inline JS/CSS.
- `cmd/serve.go` — `//go:embed assets/index.html`; `newServeHandler(b)` returns a `*http.ServeMux` (`/` → page, `/graph.json` → serializer); `newServeCmd` binds `--addr` (default `127.0.0.1:8080`), loads bundle, `http.ListenAndServe`. Register in root.
**Verify:** `go test ./cmd -run TestServe`; full suite; gofmt/vet.
**Commit:** `feat(cmd): serve embedded interactive graph visualizer`.

## Task 4 — README + full gate
- README: document `graph export` (formats + the `| dot -Tsvg` SVG path) and `serve` (loopback default, endpoints, embedded no-install viewer).
- Full gate: gofmt -l (clean), go vet, go build, `go test ./... -race`, `go mod tidy -diff` (stdlib only — `net/http`/`embed` are stdlib, no new deps), `internal/okf` imports neither cobra nor net/http (grep guard).
**Commit:** `feat(cmd): graph/serve README + full gate`.

## Whole-increment review (before PR)
Build `/tmp/okfctl-graph`; on a real multi-node bundle:
- `graph export --format json` → inspect nodes/edges/orphan flags; run twice → byte-identical.
- `graph export --format dot | dot -Tsvg` if Graphviz present (else confirm DOT is well-formed).
- Start `serve` on an ephemeral port (background), `curl /graph.json` (matches export json), `curl /` (page HTML), confirm loopback bind; kill it.
- Confirm orphan flags match `lint`'s orphan findings on the same bundle.
Then push → file card → draft PR #6 through the wired lane.
