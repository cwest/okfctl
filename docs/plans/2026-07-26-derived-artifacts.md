# Plan: Increment 9—Maintain Derived Artifacts by Default

Spec: [`docs/specs/2026-07-26-derived-artifacts.md`](../specs/2026-07-26-derived-artifacts.md)
Branch: `topic/derived-artifacts`  Base: `main` @ `d62ae44`

Each task is TDD-first (RED → GREEN → REFACTOR) and lands as its own signed commit.

## Task A—`created`/`modified` stamped on node create (created immutable)

**Files:** `internal/okf/timestamps.go` (new), `internal/okf/timestamps_test.go` (new),
`internal/okf/authoring.go` (wire in).

1. RED: `timestamps_test.go`—`stampCreated`/`touchModified` on a `*yaml.Node` mapping
   (or `map[string]any`) inject `created`+`modified` as `time.RFC3339` UTC using an
   injected clock; `touchModified` on a node that already has `created` leaves `created`
   byte-identical and advances `modified`. Also: a created node (via `NewNode`) has both
   fields; a second `touchModified` bumps `modified` only.
2. GREEN: add `nowUTC` clock seam (package var defaulting to `func() time.Time { return time.Now().UTC() }`),
   `stampCreated`/`touchModified` helpers; call `stampCreated` in `NewNodeFromTemplate`
   frontmatter assembly (append `created` + `modified` right after `title`, before template
   fields—deterministic key order).
3. Verify created node's `created`==`modified` at birth; re-stamp leaves `created` fixed.

## Task B—`modified`-vs-git drift finding in `validate` (degrades outside git)

**Files:** `internal/okf/gitmeta.go` (new), `internal/okf/gitmeta_test.go` (new),
`internal/okf/validate.go` (add drift finding), `cmd/validate.go` (surface it).

1. RED: `gitmeta_test.go`—in a temp `git init` repo with one committed node,
   `GitLastCommitDate(root, rel)` returns the commit date + `ok=true`; an untracked file
   returns `ok=false, err=nil`; a non-repo dir returns `ok=false, err=nil`. `validate_test.go`—a node whose frontmatter `modified` disagrees with git yields a drift finding; agreement
   yields none; outside git yields none (no crash).
2. GREEN: `GitLastCommitDate` via `exec.Command("git", "-C", root, "log", "-1", "--format=%cI", "--", rel)`;
   empty stdout ⇒ `ok=false`. Add a git-aware drift check to validate (advisory finding, does
   not fail the floor); `cmd/validate.go` prints drift findings as warnings.
   Compare by calendar DAY (corpus stamps are `T00:00:00Z`; git commit carries a real time)—drift = different UTC date, matching the corpus's date-granularity convention.
3. Verify advisory (exit 0 on drift alone), floor still fails on missing `type`.

## Task C—`log.md` maintained on node create + edit

**Files:** `cmd/node.go` (wire `AppendLog` into `new` and `edit`), `cmd/node_test.go`.

1. RED: creating a node writes a `log.md` entry naming the created path; a successful
   `node edit` appends an entry.
2. GREEN: after a successful `NewNode`/template create, call `okf.AppendLog(dir, "created <path>")`;
   after a valid `node edit`, `touchModified` the file then `AppendLog(dir, "edited <path>")`.
   Failures (invalid edit) do not append.
3. Verify entries present, newest-first, log still well-formed.

## Task D—`index.md` maintained on node create / delete / rename

**Files:** `cmd/node.go` (invoke index rebuild after mutate), `cmd/node_test.go`.

1. RED: after `node new`, `node rm`, `node mv`, `index check` reports in sync.
2. GREEN: after each mutation writes disk, reload the bundle and `os.WriteFile` the
   `RenderIndex` output to `index.md` (the same path `index build` uses). Keep it a single
   helper so the three call sites can't diverge.
3. Verify `index check` exits 0 post-mutation across all three verbs.

## Task E—whole-increment E2E + real-corpus proof

1. Build the binary; against a temp bundle exercise create→edit→mv→rm and assert
   `validate`, `index check`, `log show` all behave.
2. Run drift detection against `~/src/knowledge-base` (`bundles/knowledge`) and report the
   count of nodes whose `modified` is proven stale vs git.
3. Full gate: `gofmt -l .` empty, `go vet ./...`, `go build ./...`, `go mod tidy -diff`,
   `go test ./... -race -count=1`.

## Task F—commit, open draft PR, kanban handoff

Signed commits per task, Conventional format, no attribution / no team mention. Open a
**draft** PR; post PR url + head SHA + test evidence + stale-count to the card. Do not
undraft, do not merge, do not `kanban_complete`.
