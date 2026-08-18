# Plan—`node promote` (directory-as-concept index.md bulk remediation)

**Spec:** `docs/specs/2026-08-02-node-promote.md` · **Branch:** `topic/node-promote`
**Base:** `main` @ `59c22ac`

TDD throughout: RED (failing test naming the missing behavior) → GREEN (minimal
code) → REFACTOR. Full suite must stay green after each task.

## Task 1—model: detect promotable indexes (`internal/okf/promote.go`)

RED: `promote_test.go::TestPromoteScan_FindsNonRootIndexWithFrontmatter`
- fixture: bundle-root `index.md` (okf_version only), `foo/index.md` WITH
  frontmatter (type/created/body), `bar/index.md` WITHOUT frontmatter, one
  ordinary concept node.
- assert scan returns exactly `foo/index.md`, skipping the root index and the
  frontmatter-free `bar/index.md`.

GREEN: `PromotableIndexes(b *Bundle) []string`—sorted non-root `index.md`
reserved paths with `len(Frontmatter) > 0`.

## Task 2—model: PromotePlan (moves + link rewrites, both spellings)

RED: `promote_test.go::TestPromotePlan_RewritesBothSpellings`
- fixture: `foo/index.md` (frontmatter+body), `alpha.md` linking `foo/index.md`,
  `bravo.md` linking `foo/` (dir-style), `baz/qux.md` linking `../foo/` and
  `/foo/index.md` (dir-rel + root-abs). Root `index.md` links `foo/`.
- assert plan: one move `foo/index.md -> foo/foo.md`; link rewrites for every
  spelling above, each preserving relative form + title tail; root index's
  `foo/` link is included (reserved files confer navigation).

GREEN: `PromoteChange` struct (OldPath, NewPath, plus `[]LinkRewrite`) and
`PromotePlan(b, basename string) ([]PromoteChange, error)`:
- for each promotable index, compute `dir/<basename>.md` (basename defaults to
  `path.Base(dir)` when "" ); error if destination exists as a node.
- scan every concept node + reserved file body for links resolving to
  `dir/index.md` or `dir/` (both spellings, all three relative forms), rewrite
  to the new concept path preserving form + title.

## Task 3—model: PromoteApply (verbatim body, created immutable)

RED: `promote_test.go::TestPromoteApply_BodyVerbatim_CreatedImmutable`
- assert: new file exists with byte-identical body region; `created` unchanged;
  old `index.md` path no longer a frontmatter-bearing index (removed/moved);
  inbound link rewrites landed on disk; frontmatter of untouched files intact.

GREEN: `PromoteApply(root string, b *Bundle, changes []PromoteChange) error`:
- apply link rewrites to full on-disk content (offset-safe, like `ApplyMove`);
- write the promoted file: existing frontmatter block (key-order preserved via
  the yaml.Node round-trip) + verbatim body from `splitFrontmatterRaw`; then
  remove the old `index.md` (it will be regenerated clean by WriteIndex).

## Task 4—model: dry-run purity guard

RED: `promote_test.go::TestPromotePlan_IsPure_NoWrites`—snapshot tree hashes,
call `PromotePlan`, assert tree byte-identical (plan writes nothing).
GREEN: satisfied by construction (PromotePlan does no disk writes)—the test
locks it.

## Task 5—cmd: `node promote` wiring (`cmd/node.go`, `cmd/derived.go`)

RED: `cmd/node_promote_test.go`:
- `--dry-run` prints every move + rewrite and writes zero bytes (tree hash
  unchanged, exit 0).
- real run: `validate` exits 0 afterward; `log.md` has a `promoted` line per
  node; regenerated `foo/index.md` has no frontmatter; root index untouched.
- both link spellings rewritten end-to-end through the CLI.

GREEN: `newNodePromoteCmd()` added to `newNodeCmd`; `--name`, `--dry-run`,
`--bundle`-style positional `<bundle>`. On real run: `PromoteApply`, then
`logOnPromote` per node + `maintainIndex` once (mirroring `node refresh`).

## Task 6—docs surfaces kept accurate

- `cmd/node.go` promote `Short`/`Long` help.
- `README.md`: add `node promote` to the node verb list + a short remediation
  note.
- `skills/okf-curation-health/SKILL.md` if it enumerates node verbs / remediation.

## Task 7—verification (HARD GATE)

- `gofmt -w .` then `gofmt -l .` clean; `go vet ./...`; `CGO_ENABLED=0 go build
  ./...`; `go test ./... -race -count=1` all green.
- E2E on a scratch multi-dir fixture: dry-run byte-identical; real run
  `validate` = 0; `lint --strict` `broken-link` = 0.
- Real corpus (`~/src/knowledge-base`, `bundles/knowledge`): dry-run move count;
  real run on a `cp -R` scratch copy; before/after `validate` counts +
  `broken-link` count → PR body.

## Task 8—commit + PR

- `commit-style` gate; signed; Conventional Commits + ✨ emoji; Apache header on
  new files; no attribution.
- Open DRAFT PR via `onecard_common.open_pr` (marker auto-stamped), `Closes #35`.
- Comment PR url + head SHA + evidence on the card. Do not `kanban_complete`.
