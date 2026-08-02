# okfctl — bulk-mechanical-commit opt-out for git drift

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` §12 (Versioning) for the day-one migration
context; git drift itself is an okfctl-local advisory check, not a spec mandate.
**Base:** `main` @ `d75e8d1`  **Branch:** `wt/t_d5333ed2`

## Problem

The git drift check cannot tell a **bulk mechanical commit** from an incremental
edit, and its documented remedy destroys data in the first case.

`validate` warns when a node's frontmatter `modified` disagrees with the file's
git last-commit date, and the finding recommends `okfctl node refresh`. That is
correct for an incremental edit. After a **bulk migration commit** — which every
corpus adopting okfctl makes exactly once, on day one — it collapses the real
authoring history into the migration date.

Reproduced end-to-end against `main`: 4 nodes committed on 4 distinct dates
validate clean; one bulk commit adding a frontmatter key to all 4 produces
exactly 4 drift warnings; `node refresh` then collapses all 4 `modified` fields
to the migration date, and `validate` reports clean afterwards — **clean because
the evidence was destroyed, not because the problem was fixed.** At corpus scale
this flattens dozens of distinct authoring dates into one, erasing exactly the
freshness signal the migration had just derived from git history.

Root cause: `GitLastCommitDate` runs `git log -1 --format=%cI` and learns the
committer timestamp and **nothing about the commit's intent**. Git records *when*
a commit landed, not *why*.

## Goal

Two pieces, shipped together: close the destructive path immediately, then fix
the root cause.

1. **`node refresh` guardrail (stopgap).** Refuse a refresh plan dominated by a
   single commit — the signature of a bulk mechanical commit — unless `--yes` is
   given. A plan that rewrites hundreds of nodes from one commit is not an
   incremental cleanup and must not proceed silently.

2. **`.okf-drift-ignore-revs` (the actual fix).** A checked-in list of mechanical
   commit SHAs at the bundle root, mirroring `git blame --ignore-revs-file` — a
   convention users already understand. When a node's last-touching commit is on
   the list, the drift comparison walks back to the prior real commit.

This targets the root cause directly: git drift cannot read commit intent, so let
the human declare it. The rejected alternatives — guessing from commit fan-out
size, or requiring commit-time discipline a first-time adopter has not had the
chance to practice — both fail the day-one migration case that is the entire
problem. (The guardrail *does* use fan-out size, but only to refuse-and-redirect,
never to silently silence a finding.)

## Design

### Walk-back primitive — `GitLastCommitDateIgnoring`

`internal/okf/gitmeta.go` gains `GitLastCommitDateIgnoring(root, relPath, ignore)`
returning `(time.Time, sha string, ok bool, err error)`. Instead of `git log -1`
it runs `git log --format=%H %cI -- <path>` (full history, newest first) and
returns the first commit whose SHA is **not** in the ignore set. An ignore entry
matches when it equals the full 40-char SHA or is a ≥7-char prefix of it, so both
full and abbreviated spellings opt a commit out — the same tolerance `git blame`
gives. When every touching commit is ignored it degrades to `ok=false`, exactly
like an untracked file, so the node is skipped rather than erroring.

`GitLastCommitDate` becomes a thin wrapper (`ignore=nil`), preserving every
existing caller and test.

### Ignore-file loader — `LoadDriftIgnoreRevs`

`internal/okf/ignorerevs.go` reads `.okf-drift-ignore-revs` from the bundle root:
one SHA per line, blank lines and `#`-comments ignored, inline comments stripped,
SHAs lower-cased. The file is **optional** — its absence yields an empty set and
no error (the common case).

### Wiring — `scanDrift`

`scanDrift` (the single scan behind both `DriftFindings` and `RefreshPlan`) loads
the ignore set once per scan and threads it into `GitLastCommitDateIgnoring`. A
malformed/unreadable ignore file degrades to "no opt-outs" rather than crashing
an advisory scan. The commit SHA the comparison used is carried on `driftPair`
and surfaced on `RefreshChange.Commit` so the guardrail can attribute the plan.

### Guardrail — `RefreshGuard`

`internal/okf/refresh_guard.go` inspects a plan for the bulk signature: a large
plan (`≥10` changes) in which one commit accounts for a majority share (`≥0.5`).
**Both** bounds must be crossed, so the guard never fires on small or diffuse
plans. `node refresh` refuses a triggered plan unless `--yes`, printing the
dominant commit, its share, and the `.okf-drift-ignore-revs` remedy. `--dry-run`
is never gated (it writes nothing by definition).

## Controls (both directions, every detector)

- **Positive:** an incremental edit whose commit is NOT on the list still drifts,
  and `refresh` still fixes it; a bulk-dominated plan is still refused.
- **Negative:** a bulk commit whose SHA IS listed produces zero drift for the
  files it touched, and `refresh --dry-run` proposes nothing for them — the
  comparison falls through to the prior real commit, which agrees with
  frontmatter.

## Non-goals

- No change to the drift finding text for genuine incremental drift.
- No guessing intent from fan-out to *silence* findings (only to refuse-and-
  redirect). Intent is declared by a human, never inferred.
- No new spec behavior — git drift is an okfctl-local advisory, and this keeps it
  advisory.
