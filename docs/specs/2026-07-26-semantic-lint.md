# Spec: Increment 7 — semantic lint checks (`lint` consumes the neighbor set)

PRD: §8.6 ("Payoff for curation"), §8.5 (`related` "feeds lint").
Branch: `topic/semantic-lint`  Base: `main` @ 911bb60

## Problem

`lint` today runs four *structural* checks (orphan, missing-xref, coverage-gap,
type-hygiene). The PRD's differentiating claim is that lint becomes **semantic**:

> `lint` calls `search related` to turn similarity into findings: *"0.91 similar to
> an existing node, no edge between them—missing link?"* and *"orphan with no
> semantic neighbors—dead concept?"*

The neighbor set exists (`search.Related`, increment 5b) and, as of 5c-2, the
similarity behind it is real rather than lexical. Nothing consumes it. The README
says so plainly: *"the neighbor set the spec (§8.6) says `lint` will consume for
its semantic checks in a later increment (not yet wired)."*

This increment closes that gap.

## The architectural question, and why it dissolves

The apparent fork was: `lint` lives in core (dependency-free), `related` lives in
the plugin — so either core shells out to the plugin, or the semantic checks move
into the plugin.

Both horns assume the neighbor set requires the *embedder*. It does not:

```go
func Related(s *Store, nodePath string, k int) ([]Result, error)
```

`Related` takes only the **index** and ranks stored vectors by cosine. The
embedder is needed to BUILD an index, never to READ one. The heavy,
model-dependent half is `index build`; the query half is arithmetic over
`[]float64`.

Further, `internal/search` is already stdlib-only — no cobra, no `net/http`, no
CGO, no third-party deps (verified: `go list -deps`). The dependency boundary
core protects is not threatened by depending on it.

**Resolution: neither horn. Core gains an optional semantic *overlay* that reads
an index if one is present.** `lint` grows a `--semantic` flag; with it, core
loads `.okfctl/index.db` directly via `internal/search` and runs the two §8.6
checks. No plugin dispatch, no subprocess, no output parsing, no duplicated
similarity logic — one mechanism, called directly.

This is strictly better than shelling out (which would mean parsing
`0.4231\tpath` text across a process boundary, and re-running the ranking N times
for N nodes) and better than moving lint into the plugin (which would split the
curation verb in two and force users to know which binary answers a lint
question).

### What each half owns after this

| concern | home | why |
|---|---|---|
| build an index (needs a model) | plugin `okfctl-search index build` | model weights, tokenizer — the heavy half |
| read an index (arithmetic) | `internal/search`, callable by core | stdlib-only, no model |
| structural lint checks | core `lint` | unchanged |
| semantic lint checks | core `lint --semantic` | reads the index the plugin built |

The plugin remains the only thing that can *create* semantic data. Core can
*consume* it when it exists. That is the honest boundary.

## Scope

Two checks, matching the PRD's two named examples.

### 1. `similar-unlinked`

A pair of nodes whose cosine similarity ≥ threshold with **no link in either
direction** — the "missing link?" finding. Reported once per pair (not twice),
on the lexicographically-first path, naming the other node and the score.

Default threshold **0.80**, tunable via `--similarity-threshold`.

### 2. `no-semantic-neighbors`

A node whose best neighbor scores **below** a floor — semantically isolated, the
"dead concept?" finding. This is the semantic complement to the structural
`orphan` check: `orphan` means *nothing links here*, this means *nothing is even
about the same thing*.

Default floor **0.20**, tunable via `--isolation-floor`. The floor is low by
design: calibrated against potion-base-8M, same-topic-different-wording nodes
score ~0.27–0.33 while a genuinely off-topic node scores ~0.13, so a 0.30 floor
would flag legitimate on-topic nodes as dead concepts. Mean-pooled static
embeddings compress absolute scores — the ranking is reliable, the magnitudes
are not — so the floor targets the clear outlier rather than a semantic ideal.

## Behavior

- **Opt-in.** Without `--semantic`, `lint` behaves exactly as today (no index
  read, no new output). Structural checks are unchanged in both modes.
- **Missing index is a clear error, not a silent skip.** `lint --semantic` with
  no `.okfctl/index.db` fails naming the fix (`okfctl-search index build`). A
  silent structural-only fallback would let a CI job think it ran semantic checks
  when it did not — the same class of lie as falling back to the hash embedder.
- **Index/bundle drift is surfaced, not ignored.** Nodes in the bundle but absent
  from the index (added since the last `index build`) produce one bundle-level
  `stale-index` finding listing them, so a partial answer never reads as a
  complete one.
- **Advisory by default**, like every lint finding; `--strict` exits non-zero.
- **Deterministic.** Same bundle + same index ⇒ byte-identical findings, sorted
  by path then check.

## Out of scope

- Building or refreshing the index from `lint` (that is `index build`; lint never
  mutates and never invokes a model).
- Suggesting *where* to add a link, or auto-fixing.
- Any change to the four structural checks or the plugin's own commands.

## Success criteria

- `lint --semantic` reports a similar-unlinked pair on a bundle with two
  near-duplicate unlinked nodes, and does NOT report it once a link is added.
- It reports `no-semantic-neighbors` for a node unrelated to the rest of the
  corpus.
- Real-model end-to-end: on a bundle indexed with potion-base-8M, findings are
  semantically sensible (a wine-tannin/wine-astringency pair flags; an unrelated
  node isolates).
- Missing index ⇒ actionable error, exit non-zero, no panic, no silent skip.
- Without `--semantic`, output is byte-identical to today's `lint`.
- Core stays CGO-free and dependency-free: `CGO_ENABLED=0` builds,
  `go mod tidy -diff` clean, no new modules.
