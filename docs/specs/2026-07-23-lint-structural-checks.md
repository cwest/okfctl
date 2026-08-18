# Spec—Increment 3: `lint` (the curation differentiator), structural checks

**Status:** Approved · **Owner:** Casey West · **License:** Apache-2.0
**Increment:** 3 (depends only on Increment 1's model + the link graph)

## Why

`lint` is okfctl's reason to exist—the curation loop the OKF spec omits (PRD §6.2). `validate` is the spec-floor gate (fails CI on format violations); `lint` is *curation guidance*—it surfaces judgment-worthy findings about bundle health, never format failures. This increment ships the **deterministic, stdlib-only** subset. Semantic checks (contradictions, stale-claim inference, near-duplicates, template drift) are explicitly deferred to increments 5–6 (they need the vector index / template engine).

## What

`lint` runs four deterministic structural checks over a bundle and reports findings. It **never mutates** the bundle (PRD §6.2 hard rule)—reporting only; fixing is a separate explicit action.

### The four checks

1. **Orphans**—concept nodes with **zero inbound links**, i.e. unreachable by traversal. Inbound links are counted from **all** sources including the reserved `index.md` (the index is the bundle's front door—a node the index links to is reachable, not an orphan). Reserved files themselves (`index.md`, `log.md`) are never reported as orphans.

2. **Missing cross-references**—a concept node whose **body text mentions another node's title** (plain-text occurrence) but does **not link** to that node. The highest-value structural check: this is how a graph silently rots. Uses the shared `scanNodeLinks` so "already links to X" is computed identically to the edge graph. Case-insensitive whole-title match; a title already appearing as a link target is not a finding.

3. **Coverage gaps**—a **title/term mentioned by N or more distinct nodes** that has **no node of its own** (threshold configurable, default 3). Signals "this concept is referenced enough to earn a node." Only counts mentions of would-be-titles that don't already resolve to an existing node.

4. **Soft `type` value-hygiene**—warn when two or more distinct `type` values are **near-duplicates / case-variants** of one another (e.g. `Concept` vs `concept` vs `Concepts`)—likely accidental drift. This is a **warning only**; anti-taxonomy §7.4 stands (unknown `type` values are valid; `validate` never fails on them, and neither does `lint`).

## Done when

- `lint <bundle>` runs all four checks and prints findings, deterministically sorted (by path, then check kind)—CI-diffable, same discipline as `validate`/`index`.
- **Advisory by default:** `lint` exits **0** even with findings. `--strict` makes any finding exit **non-zero** (the deliberate CI-fail knob). This preserves the clean separation from `validate` (which is the spec-floor gate that *does* fail).
- Coverage-gap threshold configurable via `--coverage-threshold N` (default 3).
- Orphan detection counts inbound links from `index.md` (a node the index links to is not an orphan).
- `internal/okf` stays **cobra-free** (pure model—`Lint(b, opts) []LintFinding`); `cmd/lint.go` wires cobra + the output formatter.
- **stdlib only**—no new dependencies.
- `lint` **never mutates** the bundle.
- Full `-race` suite green; gofmt / vet / `go mod tidy -diff` clean.

## Scope / Routing

Repo: `cwest/okfctl` · branch: `topic/lint-structural-checks` · PR: draft (increment 3).

## Explicitly out of scope (deferred)

- Semantic checks: contradictions, stale-claim inference, semantic near-duplicates (need §8 vector index → increment 5).
- Template drift (needs §9.4 template engine → increment 6).
- Any `--fix` / mutation—`lint` reports; fixing is separate.
