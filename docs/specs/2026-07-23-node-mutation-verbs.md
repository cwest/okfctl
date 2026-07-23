# Spec — Increment 2b: Node Mutation Verbs (`node edit / mv / rm`)

**Status:** Approved · **Owner:** Casey West · **License:** Apache-2.0
**Increment:** 2b (completes Increment 2; depends on Increment 1's model + 2a's reserved-file engine)

## Summary

Three node-mutation verbs that complete the node-CRUD surface. All graph logic
lives in the pure `internal/okf` model (no cobra import); `cmd/` wires cobra over
it. Path is a concept's identity, so `node mv` is a **graph operation** — it
rewrites every inbound link — not a file rename.

## Verbs

### `node edit <path>`
Open a concept node in the user's editor, then re-validate on return.

- Resolve the editor: `$OKFCTL_EDITOR` → `$VISUAL` → `$EDITOR` → `vi`. Launch on
  the node's absolute file path, inheriting stdio; wait for exit.
- A non-zero editor exit aborts with an error (author cancelled) — no validation.
- On clean exit, reload the bundle and run `Validate`; print findings with the
  same formatter as `validate`. Exit non-zero if the edit introduced a spec-floor
  violation (so the author is told immediately), zero if clean.
- Refuse a path that is not an existing concept node (reserved files and unknown
  paths error).

### `node mv <old> <new>` — the graph operation
Move/rename a concept node and rewrite every inbound link so the graph is
preserved.

- **Link-form preservation (Casey-approved, option A):** each inbound link is
  rewritten in the SAME relative form the author used. A link that resolved
  root-relative stays root-relative; a link that resolved dir-relative is
  recomputed relative to the linking node's directory. The tool never silently
  normalizes an author's link style.
- The set of links to rewrite is exactly those that resolve to `old` under the
  SAME two-form resolution `Bundle.buildEdges` uses (root-relative via
  `filepath.Clean`, then dir-relative via `join(dir, target)`), so `mv` and the
  edge-builder never disagree about what "links to `old`" means.
- An optional CommonMark title suffix (`path.md "Title"`) is preserved; only the
  URL field is rewritten. Image syntax (`![alt](src)`) is never treated as a link.
- Guardrails: error if `old` is not an existing concept node; error if `new`
  already exists; error if either is a reserved file; create intermediate
  directories for `new` as needed.
- `--dry-run` prints the planned file move + per-file link rewrites and touches
  nothing.

### `node rm <path>`
Remove a concept node and report nodes orphaned as a result.

- Delete the node's file. Report, as an informational warning (NOT a failure —
  removal is legitimate), any node that had an inbound link ONLY from the removed
  node and is now orphaned (zero remaining inbound links).
- Refuse a reserved file or a non-existent node.
- `--dry-run` prints what would be removed + the resulting-orphan report, touches
  nothing.

## Model API (pure, `internal/okf`, no cobra)

```
type LinkRewrite struct {
    NodePath string // bundle-relative path of the node whose body is edited
    Old      string // exact link target text being replaced
    New      string // replacement link target text (same relative form as Old)
}

func PlanMove(b *Bundle, old, new string) ([]LinkRewrite, error)
func ApplyMove(root string, b *Bundle, old, new string, rewrites []LinkRewrite) error
func PlanRemoveOrphans(b *Bundle, path string) ([]string, error)
```

`PlanMove` is pure (no disk); `ApplyMove` performs the file move + body edits and
is the single writer. `PlanRemoveOrphans` returns the bundle-relative paths newly
orphaned by removing `path`.

## Non-goals (deferred)

- Semantic / similarity-aware link suggestions (Increment 5).
- `lint`'s orphan/missing-link *findings* surface (Increment 3) — 2b only reports
  orphans as a direct consequence of a specific `rm`.
- Undo/history beyond the reserved change-log (2a already owns `log`).

## Done criteria

1. `node edit` opens `$EDITOR` and re-validates on return (spec violations
   surfaced, correct exit code).
2. `node mv` moves the file AND rewrites all inbound links preserving each
   author's relative form; `--dry-run` shows the plan; `buildEdges` sees the
   graph intact afterward (no dangling edges to `old`, all edges now to `new`).
3. `node rm` removes the file and reports newly-orphaned nodes; `--dry-run`
   shows the plan.
4. `internal/okf` still imports no cobra. Full `-race` suite green;
   gofmt/vet/`go mod tidy -diff` clean.
