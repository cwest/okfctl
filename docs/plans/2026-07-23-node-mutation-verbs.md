# TDD Plan — Increment 2b: Node Mutation Verbs

**Spec:** [`docs/specs/2026-07-23-node-mutation-verbs.md`](../specs/2026-07-23-node-mutation-verbs.md)
**Branch:** `topic/node-mutation-verbs` · **Worktree:** `.worktrees/node-mutation-verbs`

**Ground truth this builds on:**
- `okf.Load(root)(*Bundle,error)`; `Bundle{Nodes,Reserved map[string]*Node, edges map[string][]string}`; `Bundle.OutboundLinks(path)`.
- `Bundle.buildEdges()` resolution: for each inline `[text](target)` (title suffix stripped to first field; `http(s)://`/`#` skipped; `![img]` skipped), resolve **root-relative** `filepath.Clean(target)` first, then **dir-relative** `Clean(join(dir,target))`. Only in-bundle nodes become edges.
- `okf.ReservedFiles` = `{index.md, log.md}`. `Node{Path,Frontmatter,Body}`.
- Reuse test helpers: `writeNode(t, dir, rel, typ, title)` (`internal/okf/reserved_lifecycle_test.go`), `runOKF(t, args...)` (`cmd/validate_test.go`), `bundleDirArg(args)` (`cmd/index.go`).

**Discipline:** RED→GREEN→REFACTOR per task; ≥1 test per group must produce a REAL failure naming the missing symbol/behavior (not an incidental error path). `internal/okf` MUST NOT import cobra. Commits signed, `Casey West <casey@geeknest.com>`, no AI attribution; addlicense re-commit dance expected on new `.go` files.

---

## Task 1 — `PlanMove` (pure link-rewrite planning, the graph core)

**File:** `internal/okf/mutate.go` (new), `internal/okf/mutate_test.go` (new).

**RED — `mutate_test.go`:**
- `TestPlanMove_RootRelativeInboundPreserved`: node `a.md` body links `[x](wine/foo.md)`; move `wine/foo.md`→`wine/bar.md`. Expect one `LinkRewrite{NodePath:"a.md", Old:"wine/foo.md", New:"wine/bar.md"}` (root-rel form kept).
- `TestPlanMove_DirRelativeInboundPreserved`: node `wine/a.md` body links `[x](foo.md)` (dir-rel, resolves to `wine/foo.md`); move→`wine/bar.md`. Expect `Old:"foo.md", New:"bar.md"` (dir-rel form kept, recomputed against `wine/`).
- `TestPlanMove_DirRelativeAcrossDirs`: `red/a.md` links `[x](../wine/foo.md)`; move `wine/foo.md`→`cellar/foo.md`. Expect `Old:"../wine/foo.md", New:"../cellar/foo.md"`.
- `TestPlanMove_TitleSuffixPreserved`: body `[x](wine/foo.md "Foo Note")`; only URL field rewritten, title kept → new target text `wine/bar.md "Foo Note"`.
- `TestPlanMove_ImageAndExternalNotRewritten`: body has `![img](wine/foo.md)` and `[e](https://foo.md)` and a real `[x](wine/foo.md)`; only the real link yields a rewrite.
- `TestPlanMove_MultipleInboundAcrossNodes`: two nodes link to `old`; two rewrites, deterministic order (sort by NodePath).
- `TestPlanMove_NoInboundReturnsEmpty`: move a node nothing links to → zero rewrites, no error.
- `TestPlanMove_ErrOldMissing` / `TestPlanMove_ErrNewExists` / `TestPlanMove_ErrReserved` (old or new == `index.md`/`log.md`).

**GREEN — `mutate.go`:**
```go
type LinkRewrite struct{ NodePath, Old, New string }

// PlanMove computes the inbound-link rewrites needed to move old→new, pure (no disk).
func PlanMove(b *Bundle, old, new string) ([]LinkRewrite, error)
```
Implementation mirrors `buildEdges` exactly: for each node, scan inline links with the same regex + image/title/external handling; for each target that resolves to `old` (root-rel first, else dir-rel), compute the replacement target text in the SAME form:
- if the matched target resolved root-relative → `new` (cleaned, slash).
- if it resolved dir-relative → `filepath.Rel(dir, new)` (slash), preserving `../` idiom.
Reappend the stripped title suffix verbatim. Return sorted by `NodePath` then `Old`. Guard old-missing/new-exists/reserved.

**Refactor:** extract the shared link-scan (regex + image/title/external skip) into one unexported helper used by BOTH `buildEdges` and `PlanMove` so they can never diverge (root-cause: one parser, two callers).

**Verify:** `go test ./internal/okf -run TestPlanMove -v -count=1`.

---

## Task 2 — `ApplyMove` + `PlanRemoveOrphans` (pure model, the writers/reporters)

**File:** extend `internal/okf/mutate.go` + `_test.go`.

**RED:**
- `TestApplyMove_MovesFileAndRewritesBodies`: build a real temp bundle on disk (`writeNode`), `PlanMove` then `ApplyMove`; reload with `Load`; assert (a) `old` gone from `b.Nodes`, `new` present; (b) every previously-inbound node's `OutboundLinks` now contains `new` and NOT `old`; (c) no dangling edges.
- `TestApplyMove_CreatesIntermediateDirs`: move into a new neighborhood dir.
- `TestApplyMove_ErrNewExistsOnDisk`: refuses to clobber.
- `TestPlanRemoveOrphans_ReportsNewlyOrphaned`: `a.md`→`b.md` (only inbound to `b`); remove `a.md` → `b.md` reported orphaned.
- `TestPlanRemoveOrphans_StillLinkedNotReported`: `b.md` also linked from `c.md`; remove `a.md` → `b.md` NOT reported (still reachable).
- `TestPlanRemoveOrphans_ErrReserved` / `ErrMissing`.

**GREEN:**
```go
func ApplyMove(root string, b *Bundle, old, new string, rewrites []LinkRewrite) error
func PlanRemoveOrphans(b *Bundle, path string) (orphaned []string, err error)
```
`ApplyMove`: apply each `LinkRewrite` to the on-disk body (exact `Old`→`New` in the link position; use the byte offsets from the shared scan to avoid rewriting a coincidental substring elsewhere), then `os.Rename` old→new (mkdir-all parent). Single writer.
`PlanRemoveOrphans`: for each node the removed node linked to, recompute inbound count excluding the removed node; those reaching zero AND not otherwise reachable are orphaned. Sorted, deterministic.

**Verify:** `go test ./internal/okf -run 'TestApplyMove|TestPlanRemoveOrphans' -v -count=1`, plus full `go test ./internal/okf -count=1`.

---

## Task 3 — `node mv` + `node rm` cmd verbs

**File:** `cmd/node_mv.go`, `cmd/node_rm.go` (or extend `cmd/node.go`), `cmd/node_mutate_test.go`.

**RED (`runOKF`):**
- `TestNodeMv_MovesAndRewrites`: init bundle + two linked nodes; `node mv old new`; assert file moved + linking body rewritten (read file back).
- `TestNodeMv_DryRunTouchesNothing`: `--dry-run` prints plan, no disk change (mtime/content unchanged).
- `TestNodeMv_ErrNewExists` / `ErrReserved`.
- `TestNodeRm_RemovesAndReportsOrphans`: `node rm a.md` prints newly-orphaned `b.md`, exit 0 (informational).
- `TestNodeRm_DryRun`.
- The genuine RED signal: `runOKF(t,"node","mv",...)` returns `unknown command "mv"` before the verb exists.

**GREEN:** register `mv`/`rm` under `newNodeCmd()`, mirroring `new`/`show`/`list`. `mv`: `PlanMove`→(print if `--dry-run` else `ApplyMove`). `rm`: `PlanRemoveOrphans`→(print orphans; if not dry-run `os.Remove`). Reuse `bundleDirArg`. Reserved/missing guards surface as errors.

**Verify:** `go test ./cmd -run 'TestNodeMv|TestNodeRm' -v -count=1`.

---

## Task 4 — `node edit` + README + full gate

**File:** `cmd/node_edit.go`, test in `cmd/node_mutate_test.go`; update `README.md`.

**RED:**
- `TestNodeEdit_RunsEditorThenValidates`: set `$OKFCTL_EDITOR` to a tiny script that appends a known-good frontmatter/body edit; assert editor ran (marker file) and validation ran clean (exit 0).
- `TestNodeEdit_ReportsValidationFailure`: editor script writes a spec-floor violation (e.g. empty `type`); assert non-zero exit + finding printed.
- `TestNodeEdit_EditorNonZeroAborts`: editor script exits 1 → command aborts, no validation.
- `TestNodeEdit_ErrReservedOrMissing`.

**GREEN:** `node edit <path>` resolves editor (`$OKFCTL_EDITOR`→`$VISUAL`→`$EDITOR`→`vi`), `exec.Command` on the node's abs path inheriting stdio, wait; on clean exit reload+`Validate`, print findings via the `validate` formatter, exit non-zero on spec-floor violation.

**Then the full gate:**
- `gofmt -l .` empty · `go vet ./...` · `go build ./...` · `go test ./... -race -count=1` all green.
- `go mod tidy -diff` clean (no new deps — stdlib only).
- Grep-confirm `internal/okf` imports no cobra: `! grep -rq spf13/cobra internal/okf`.
- README: document `node edit/mv/rm` (mv's link-form preservation + `--dry-run`).

**Verify:** ad-hoc `hermes-verify-*` script running the full gate; paste real output.

---

## Commit sequence (signed, no AI attribution)
1. `docs(2b): spec + TDD plan for node mutation verbs` (docs only — addlicense skips, no source staged)
2. `feat(okf): PlanMove inbound-link rewrite planning (link-form preserving)`
3. `feat(okf): ApplyMove + PlanRemoveOrphans model writers`
4. `feat(cmd): node mv and node rm verbs`
5. `feat(cmd): node edit verb + README + full gate`

Then: push, file the one work card via `submit-card` (owner map ready:eckert/review:lamport/blocked:casey), stamp the `<!-- card:t_… -->` marker, open draft PR #4, verify webhook 202 → card → review lane.
