# TDD Plan — Increment 3: `lint` (structural checks)

**Spec:** [`docs/specs/2026-07-23-lint-structural-checks.md`](../specs/2026-07-23-lint-structural-checks.md)
**Branch:** `topic/lint-structural-checks` · **Worktree:** `.worktrees/lint-structural-checks`

**Constraints for the implementer:** commits signed (`-S`), repo-local identity is `Casey West <casey@geeknest.com>` (never change; never @google); NO AI attribution anywhere; the pre-commit `addlicense` hook stamps `// Copyright 2026 Google LLC` Apache headers on new `.go` files — if a commit is blocked "files were modified by this hook", run `addlicense -l apache -c "Google LLC" -y 2026 <files>`, `git add`, re-commit. `internal/okf` must NOT import cobra. stdlib only — no new deps. Strict RED→GREEN→REFACTOR per task; verify each task by real `go test` before committing.

Reuse existing model surface (do NOT reinvent): `b.Nodes` (concept-only), `b.Reserved` (`index.md`/`log.md`), `b.OutboundLinks(path)` + the `edges` graph, `scanNodeLinks(b, dir, body)` (the shared link scanner — use it so "already links to X" matches the edge-builder exactly), `nodeTitle(n)`, `neighborhood(rel)`, `ReservedFiles`. Reuse the `cmd` test helpers `runOKF` and the fixture-writing pattern from `cmd/node_mutate_test.go`.

---

## Task 1 — Orphans + missing cross-references (pure model)

**New file:** `internal/okf/lint.go` (pure, no cobra). **Test:** `internal/okf/lint_test.go`.

Define:
```go
type LintFinding struct {
    Check   string // "orphan" | "missing-xref" | "coverage-gap" | "type-hygiene"
    Path    string // node path the finding is about ("" for bundle-level type-hygiene)
    Message string
}
type LintOptions struct { CoverageThreshold int } // default applied by caller; 0 => 3
func Lint(b *Bundle, opts LintOptions) []LintFinding
```
This task implements only the **orphan** and **missing-xref** checks inside `Lint` (coverage-gap + type-hygiene land in Task 2; return them empty for now).

**Inbound-link computation (shared helper):** add an unexported `inboundCounts(b) map[string]int` that counts, for every concept node, how many DISTINCT sources link to it — INCLUDING the reserved `index.md` (scan `b.Reserved["index.md"].Body` via `scanNodeLinks` too). A node with 0 inbound = orphan.

**RED tests (write first, watch fail with `undefined: Lint`):**
- `TestLint_Orphan_NoInbound` — bundle with node `a` (linked from `b`) and node `c` (linked from nobody) → exactly one `orphan` finding, for `c`.
- `TestLint_Orphan_IndexRescues` — node `c` linked ONLY from `index.md` → NO orphan finding (index is the front door).
- `TestLint_Orphan_ReservedNeverOrphan` — `index.md`/`log.md` never reported as orphans even with zero inbound.
- `TestLint_MissingXref_MentionsTitleNoLink` — node `a` body contains the plain text "Malolactic Fermentation" (the title of node `mlf.md`) but no link to it → one `missing-xref` finding on `a`.
- `TestLint_MissingXref_AlreadyLinkedNoFinding` — same mention but `a` DOES link `[…](mlf.md)` → no finding (case-insensitive; uses `scanNodeLinks` for the "already links" set).
- `TestLint_MissingXref_CaseInsensitive` — body says "malolactic fermentation" lowercase → still a finding.

**GREEN:** implement orphan + missing-xref. Deterministic ordering (sort findings by Path, then Check). **Commit** (docs commit first: spec + plan; then this model commit).

---

## Task 2 — Coverage gaps + type-hygiene (pure model)

Extend `Lint` with the remaining two checks in `internal/okf/lint.go`; tests in `lint_test.go`.

**RED tests:**
- `TestLint_CoverageGap_MentionedByThreshold` — a term "Terroir" mentioned as plain text by 3 distinct nodes, with NO `terroir.md` node → one `coverage-gap` finding naming "Terroir". With threshold default 3.
- `TestLint_CoverageGap_BelowThresholdNoFinding` — same term mentioned by only 2 nodes → no finding.
- `TestLint_CoverageGap_ExistingNodeNoFinding` — term mentioned by 3 nodes but a `terroir.md` node exists → no finding (it's covered).
- `TestLint_CoverageGap_ThresholdConfigurable` — `LintOptions{CoverageThreshold: 2}` flips the 2-mention case into a finding.
- `TestLint_TypeHygiene_CaseVariants` — nodes with `type: Concept` and `type: concept` → one `type-hygiene` warning naming both variants.
- `TestLint_TypeHygiene_PluralVariant` — `Concept` vs `Concepts` flagged as near-duplicate.
- `TestLint_TypeHygiene_DistinctTypesNoFinding` — `Concept` and `Method` (genuinely different) → NO finding (anti-taxonomy §7.4: distinct types are legitimate).

Coverage-gap "term" detection is deterministic and bounded: derive candidate terms from Title-Case multi-word phrases and existing node titles that appear as plain-text mentions across nodes; count DISTINCT mentioning nodes. Near-duplicate for type-hygiene: case-fold + simple singular/plural fold (trailing `s`), group; a group with >1 distinct raw value is a finding.

**GREEN:** implement both; keep deterministic ordering. **Commit.**

---

## Task 3 — `lint` command + flags + README + full gate

**New file:** `cmd/lint.go` (`newLintCmd`), registered in `cmd/root.go`. **Test:** `cmd/lint_test.go` (use `runOKF`).

- `lint <bundle>` (or `--bundle`, default `.` — match the `node`/`index` verb convention already in the codebase).
- Flags: `--strict` (any finding → exit non-zero; default advisory, exit 0), `--coverage-threshold N` (default 3).
- Output: findings printed deterministically, grouped/sorted; a clean bundle prints a terse "no findings" line and exits 0. Mirror `validate`'s formatter style.

**RED tests:**
- `TestLintCmd_ReportsFindingsExitsZeroByDefault` — bundle with an orphan → prints the finding, exit code 0.
- `TestLintCmd_StrictExitsNonZeroOnFinding` — same bundle with `--strict` → non-zero exit.
- `TestLintCmd_CleanBundleExitsZero` — healthy bundle → "no findings", exit 0 (both with and without `--strict`).
- `TestLintCmd_CoverageThresholdFlag` — `--coverage-threshold 2` surfaces a 2-mention gap.

**GREEN:** implement `cmd/lint.go`, register in root. Update **README** (document `lint` + `--strict` + `--coverage-threshold`, note it's advisory/curation vs `validate`'s spec-floor gate). Run the **full gate**: `gofmt -l .`, `go vet ./...`, `go build ./...`, `go test ./... -race -count=1`, `go mod tidy -diff`, and confirm `internal/okf` imports no cobra. **Commit.**

---

## After all 3 tasks
Whole-increment review (exercise the built binary end-to-end on a real bundle — orphan/xref/coverage/type-hygiene all fire and `--strict` flips the exit code; a clean bundle is silent). Then push → draft PR #5 with the `<!-- card:... -->` marker → wired review lane (webhook `655616364` live on the repo).
