# okfctl Increment 2a—Reserved-file Lifecycle (index + log)

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** [`docs/PRD.md`](../PRD.md) §6.1 · **Roadmap:** [`docs/plans/2026-07-22-roadmap.md`](../plans/2026-07-22-roadmap.md) increment 2 (split: this is **2a**; `node edit/mv/rm` is **2b**).

## Purpose

Increment 1 (walking skeleton) established the core model + `validate` + `bundle init` + `node new/show/list`. This increment builds the **reserved-file engine**—the machinery that generates and verifies `index.md` (progressive-disclosure entry point) and appends to / reads `log.md` (change history). These are the two files OKF reserves; the format anchors navigation on `index.md` and provenance on `log.md`, but nothing keeps them current. This increment makes them a *managed, regenerable* concern rather than hand-maintained prose.

Splitting `node edit/mv/rm` into 2b keeps this increment focused on the reserved-file concern, and gives `node mv`'s graph-aware link rewriting its own focused review.

## Scope of THIS increment (2a)

1. **`index build [dir]`**—regenerate `index.md` from the current bundle for progressive disclosure. **Neighborhood-grouped** (per the approved design): concept nodes grouped by their top-level directory (neighborhood), each node listed as a markdown link with its `type`, neighborhoods and nodes sorted deterministically. The generated `index.md` keeps `type: Index` frontmatter and must itself pass `validate` (it is a reserved file, so it is exempt from the concept-node type floor, but its frontmatter must parse).
2. **`index check [dir]`**—verify `index.md` matches what `index build` *would* generate; exit 0 if in sync, nonzero with a diff-style report if stale. No mutation. This is the "a field is not a process" discipline applied to the index: it can be CI-checked so it cannot silently rot.
3. **`log append [dir] --message <msg>`**—append a conformant, timestamped change entry to `log.md` (newest-last or newest-first—decided in the plan; newest-first is friendlier for a changelog). Creates `log.md` if absent.
4. **`log show [dir]`**—print the change history (the `log.md` body).

Model additions in `internal/okf` (cobra-free): a reserved-file engine that (a) renders the neighborhood-grouped index from a loaded `Bundle`, (b) compares rendered-vs-on-disk for `check`, and (c) appends a log entry. Commands in `cmd/` are thin adapters.

## Out of scope (2b and later)

- `node edit` / `node mv` / `node rm` → **increment 2b** (next).
- `lint`, `graph`/`serve`, search plugin, type templates → later increments.
- The index format is generated + deterministic; it does NOT try to be reader-goal narrative prose (the PRD's richer "start here" affordance is a later concern layered on top; 2a delivers the mechanical regenerable index).

## Architecture

Extends the existing layered design. New model file `internal/okf/reserved_lifecycle.go` (or extend `reserved.go`) holds:
- `RenderIndex(b *Bundle) string`—deterministic neighborhood-grouped index markdown (frontmatter + grouped links).
- `IndexInSync(b *Bundle) (bool, string)`—compares `RenderIndex` output to the on-disk `index.md`; returns in-sync + a human diff when stale.
- `AppendLog(root, message string) error`—prepend/append a timestamped entry to `log.md`, creating it if absent.
- `ReadLog(root string) (string, error)`—return the log body.

`internal/okf` still imports no cobra. `cmd/index.go` + `cmd/log.go` are adapters over these, registered in `NewRootCmd()`.

## Testing

- **Golden-file** tests for `RenderIndex` over a fixture bundle with ≥2 neighborhoods → exact expected markdown (deterministic).
- `IndexInSync` true on a freshly-built index, false (with a diff) after a node is added.
- `AppendLog` creates `log.md` when absent; a second append preserves the first entry; entries are timestamped and ordered.
- Command-level: `index build` then `index check` exits 0; `index check` on a stale index exits nonzero; `log append` then `log show` surfaces the message. Round-trip: `bundle init` → `node new` → `index build` → `validate` still passes.
- Determinism: `index build` run twice produces byte-identical `index.md` (no map-iteration nondeterminism—the same class Lamport flagged in increment 1's `validate`).

## Success criteria

1. `index build` produces a neighborhood-grouped `index.md` that `validate` passes and that is byte-identical across repeated runs.
2. `index check` exits 0 when the index is current, nonzero (with a report) when stale.
3. `log append --message X` then `log show` shows X with a timestamp; append is idempotently additive (never clobbers history).
4. Clean `go build`, `go vet`, `gofmt`, full `go test ./... -race` green.
