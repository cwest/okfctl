# okfctl Increment 9—Maintain Derived Artifacts by Default (created/modified, log.md, index.md + drift findings)

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** [`docs/PRD.md`](../PRD.md) §6.1 (reserved files), §7 (frontmatter floor).
**Roadmap:** [`docs/plans/2026-07-22-roadmap.md`](../plans/2026-07-22-roadmap.md).
**Base:** `main` @ `d62ae44`  **Branch:** `topic/derived-artifacts`

## Problem

Evidence from the real 215-node corpus (`cwest/knowledge-base`, `bundles/knowledge`):
every node carries `created` + `modified`, the spec requires neither, and **41% of
hand-maintained `modified` dates are wrong** (behind or ahead of the file's real
git last-commit date). Every node has the field; nobody keeps it accurate. That is
the signature of a value that should be **computed, not authored**.

This is load-bearing beyond tidiness: a planned `analyze` freshness dimension reads
`modified` to decide what is stale. A staleness report built on a field that lies
41% of the time is garbage-in.

The governing insight: **the smart-default surface is exactly the set of fields the
spec does NOT require but every real node carries anyway.** Where the corpus is
unanimous and the spec is silent, the tool should do the work—and where it cannot
maintain a value, it REPORTS drift rather than staying silent.

## The spec floor (verified empirically—do not re-derive)

`type` is the ONLY required frontmatter field (verified by stripping each field and
running `validate`: only `type` fails). Everything else—`title`, `description`,
`created`, `modified`, `tags`, `status`—is convention held by discipline.

## Governing rule

**Compute what is verifiable; never invent what is editorial.** Timestamps, reserved
artifacts, and log entries are derivable and are wrong today precisely because humans
maintain them by hand. Titles, descriptions, and prose stay human.

## Scope of THIS increment

**1. `created` / `modified` on write**
- `created`—stamped once at node creation (RFC3339 UTC), NEVER rewritten by okfctl.
- `modified`—set to now whenever okfctl writes/edits the node.
- The corpus timestamp form is `2026-06-26T00:00:00Z` (RFC3339, `Z`). New stamps use
  the same layout (`time.RFC3339`, UTC).

**2. Drift finding—`modified` vs git**
- Write-time stamping alone is INSUFFICIENT: most edits happen in `$EDITOR`, not through
  okfctl. That is exactly how the corpus reached 41% wrong.
- `validate` reports when frontmatter `modified` contradicts the file's last git-commit
  date (drift finding). It never silently rewrites the user's file during a read-only
  command—following the `index check` precedent (report; let a write command fix it).
- Must degrade sanely OUTSIDE a git repo: no git (or git unavailable, or file untracked)
  ⇒ no drift finding, not a crash.
- Drift is advisory by default (does not fail the spec floor); consistent with how
  template drift is surfaced by `validate`.

**3. `log.md`—maintained on node create/edit**
- `log.md` is a reserved, spec-mandated artifact. Appending a change entry on
  create/edit is pure computation with no editorial judgment. `AppendLog` already exists
  (increment 2a); wire node create/edit to call it so a node is never created without a
  log entry (an audit gap the tool can simply close).

**4. `index.md`—maintained on node create/delete/rename**
- `index build`/`index check` exist, yet the index still drifts because a build step a
  human must remember is a build step that drifts. Maintain it automatically on
  create/delete/rename (`RenderIndex` already exists), not merely offer to check it.

## Explicitly out of scope

- **Auto-generating `description`**—editorial (the one-line framing of a node). A
  generated description is worse than a missing one.
- **Inferring `type`**—`type` is the one MANDATORY field; inferring it from path would
  make the mandatory field the least visible thing in the file. `type` stays explicit.

## Architecture

`internal/okf` stays cobra-free.

- `internal/okf/timestamps.go`—`stampCreated`/`touchModified` helpers that inject/update
  `created`/`modified` in a frontmatter `map[string]any` at authoring/edit time, preserving
  key order for the marshaled block. New stamps use `time.RFC3339` UTC. A clock seam
  (`nowUTC func() time.Time`, defaulting to `time.Now().UTC()`) makes tests deterministic.
- `internal/okf/gitmeta.go`—`GitLastCommitDate(root, relPath) (time.Time, bool, error)`:
  shells `git -C <root> log -1 --format=%cI -- <relPath>` (stdlib `os/exec`; no new
  dependency, no CGO). Returns `ok=false` (not an error) when: git binary absent, dir not
  a repo, or file untracked (empty output). A real exec failure is an error.
- `internal/okf/validate.go`—`Validate` (or a git-aware variant) adds a `modified`-vs-git
  drift finding, threaded so the read-only path never writes.
- Authoring (`authoring.go`) stamps `created`+`modified` at create; `NewNode` gains no new
  required args. Node `edit` (cmd) touches `modified` and appends a log entry on a
  successful, still-valid edit. `index build` is invoked after create/delete/rename so the
  index stays current.

Commands in `cmd/` remain thin adapters.

## Testing

- **created/modified stamping:** a created node has both fields (RFC3339 UTC, injected
  clock); re-writing/editing updates `modified` but leaves `created` byte-identical.
- **drift finding:** in a temp git repo, a node whose frontmatter `modified` differs from
  its git last-commit date yields a drift finding; a matching one does not; degrades to no
  finding (no crash) outside a git repo / untracked file / no git binary.
- **log on create/edit:** creating a node appends a `log.md` entry; editing appends one.
- **index on create/delete/rename:** each mutation leaves `index check` in sync.
- **whole-increment E2E on the BUILT binary** against a temp bundle, and a **real-corpus
  proof** against `~/src/knowledge-base` (`bundles/knowledge`, 215 nodes) reporting the
  count of nodes whose `modified` is proven stale (fixtures structurally cannot expose this).

## Constraints

- TDD throughout: RED → GREEN → commit per task.
- No new Go dependencies. No CGO.
- Full gate green: `gofmt -l .` empty, `go vet ./...`, `go build ./...`,
  `go mod tidy -diff`, `go test ./... -race -count=1`.
