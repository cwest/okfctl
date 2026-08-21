# pkg/okf—a read-only, stable Go facade over the loader

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` **v0.2** (upstream:
GoogleCloudPlatform/knowledge-catalog `okf/SPEC.md`). This increment adds no
spec-defined behavior; it re-exports existing behavior unchanged.
**Closes:** cwest/okfctl#137

## Problem

Everything a Go consumer needs to read an OKF bundle—`Load`, `Validate`,
`BuildGraph`, `Lint`, `Search`—lives in `internal/okf`, which the compiler forbids
any out-of-tree caller from importing. The only public Go surface is `main.go`
(sets version, calls `cmd.Execute()`), so a consumer's sole integration path is
to shell out to the binary and parse text. #147 frames this as a *missing front
door*, not missing logic: the logic exists and is correct; there is no stable
package to call it from.

`#135` (an MCP read surface) and this facade both expose the read path, and the
roadmap requires they route through the same underlying functions rather than
growing parallel implementations. This facade lands first so #135 has a substrate.

## The Search question—resolved by reading current `main`

The issue sketch and the card's reproduction both warned that the lexical matcher
lived in `cmd/search.go` and would have to be *lifted* into `internal/okf` before
a facade could delegate to it (reimplementing it in the facade would create a
second dialect—the exact defect the issue exists to prevent).

**That reading is stale.** On current `main`, `internal/okf/search.go` already
exports `func Search(b *Bundle, query string, field SearchField) []SearchResult`
and `func Neighborhood(b *Bundle, start string, depth int) ([]NeighborResult,
bool)`; `cmd/search.go` is already a thin adapter that calls `okf.Search` /
`okf.Neighborhood`. So no lift is required: `Search` is a clean delegation target
exactly like the other four functions, and the facade includes it in the first
cut. `Neighborhood` (graph-structural search) and `Analyze` (the read-only
curation report) are the other natural read-path delegations and are included too,
because omitting them would push a consumer straight back to `internal/okf`—the
gap this facade exists to close.

## Design

`pkg/okf` is a **pure delegation layer**. Two rules govern every line:

1. **Domain types are Go type aliases, never copies.** `type Bundle = okf.Bundle`
   means a `*pkg/okf.Bundle` *is* an `*internal/okf.Bundle`—identical type
   identity, so a value from the facade is assignable to the internal type and
   there is exactly one `Bundle` in the program, not two that can drift.
2. **Every exported function is a one-line delegation with no branching of its
   own.** The facade adds no logic, no defaults, no validation—if it did, it would
   be a second implementation, and the CLI and the facade could disagree. Behavior
   is defined once, in `internal/okf`, and the facade forwards to it.

### Exported surface

| Facade | Delegates to | Kind |
|---|---|---|
| `Load(root, opts...)` | `okf.Load` | loader |
| `Validate(b)` | `okf.Validate` | spec floor (§6.2, §7.1) |
| `Lint(b, opts)` | `okf.Lint` | curation guidance |
| `BuildGraph(b)` | `okf.BuildGraph` | link-graph serializer |
| `Search(b, query, field)` | `okf.Search` | lexical |
| `Neighborhood(b, start, depth)` | `okf.Neighborhood` | graph-structural |
| `Analyze(b, opts)` | `okf.Analyze` | read-only curation report |

Type aliases re-exported: `Bundle`, `Node`, `Finding`, `LintFinding`,
`LintOptions`, `Graph`, `GraphNode`, `GraphEdge`, `SearchField`, `SearchResult`,
`NeighborResult`, `AnalyzeOptions`, `AnalyzeReport`, plus the `SearchField`
constants (`FieldAny`, `FieldTitle`, `FieldTag`, `FieldType`, `FieldBody`),
`WithNoIgnore`, `DefaultAnalyzeOptions`, `SpecVersion`, `IsReservedPath`. Aliases
carry the internal doc comments by construction; `doc.go` states the stability
commitment.

### The deliberate write-path exclusion

The facade re-exports **no** mutating function—no `New`, `Move`, `Touch`, index
build, or log append. `internal/okf` owns those, and they stay internal in this
cut: a stable *read* contract is the thing #135 needs and the thing a consumer
can safely depend on, while the write path's contract is not yet frozen. `doc.go`
documents this as a deliberate scope boundary, not an oversight, so a consumer
does not read the omission as "coming soon."

## Test plan

- **Type-alias assignability** — assign a `pkg/okf` `*Bundle` to an
  `internal/okf` `*Bundle` variable (and vice versa) and compile; a copy-type
  would fail to compile. Proves rule 1.
- **Read-only invariant** — snapshot a fixture bundle's file tree (path → mtime →
  sha256), exercise the *entire* facade over it, re-snapshot, assert byte-identical.
  Proves no exported path mutates.
- **CLI equivalence (fixture)** — facade `Validate`/`Lint` findings equal what
  `okfctl validate`/`okfctl lint` produce on the same fixture (compare the
  underlying finding slices, which the CLI renders verbatim).
- **Real-corpus equivalence** — over the 254-node `~/src/knowledge-base/bundles/
  knowledge` corpus, facade `Validate` returns 0 findings and facade `Lint`
  returns 0 findings, matching the CLI (`validate` OK, `lint` 0). The corpus is
  lint-clean today, so this is a **pure negative control**; the positive control
  (a check that fires) comes from the fixture. Skips cleanly when the corpus is
  absent so CI on a machine without it stays green.
- **`example_test.go`** — runnable `Example*` functions that render on pkg.go.dev.

## Definition of done

Per `AGENTS.md`: conformance suite green (`-run Conformance -race`), full suite
green under `-race`, `gofmt -l` empty, `go vet` clean, real-corpus run executed
with before/after counts pinned in the PR body (including the ones that did not
move), both controls proven, spec sections cited where behavior is spec-mandated,
commit signed + Conventional + team-invisible.
