# okfctl — §9 log conformance: validate enforcement + §9-shaped log append

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF spec v0.2 §8, §9, §11.3, §12 · **Closes:** [#176](https://github.com/cwest/okfctl/issues/176), [#177](https://github.com/cwest/okfctl/issues/177)

## Purpose

`okfctl validate` never inspects a reserved `log.md`, and `okfctl log append`
writes a log shape that §9 does not permit (inline-dated bullets, and a second
`# Change Log` H1 stacked over any other title). These are a hard co-landing
pair: the validator alone would reject every log okfctl writes today; the append
fix alone stays unguarded. The load-bearing invariant is **`okfctl validate`
never rejects a log okfctl wrote**.

## Scope

Repo `cwest/okfctl` only. Three model concerns plus one test-suite fix:

1. **Validator (`validateReserved`)** — enforce §9 on every `log.md` at any depth
   (title, ISO-8601 date headings, newest-first order, at-least-one-heading when
   the log has entries, legacy-bullet detection), and close the §8 empty-block
   index gap (a present-but-empty frontmatter block is not "no block").
2. **`AppendLog`** — emit §9 date-grouped entries (`## <today>` + `* <message>`),
   preserving any existing H1 title; regroup legacy `- YYYY-MM-DD — msg` bullets
   on write.
3. **Legacy convergence** — `log append` regroups legacy bullets under their date
   headings, merging into an existing `## date` group; idempotent and lossless.
4. **Test suite** — `assertLogConformsSection9` calls the production validator
   (it currently accepts `2025-13-45` and passes a heading-less log vacuously).

## Architecture

### The "frontmatter block present" bit (serves #176 index side and #177 log title)

`ParseFrontmatter` returns an empty non-nil map for BOTH "no `---` block" and an
empty block (`---\n---`), because it discards the `ok` return of
`splitFrontmatter`. That conflation is the root of the #176 index gap AND blocks
the §9 log-title check (a frontmatter-bearing log's parsed `Body` starts with the
title, so a naive "body begins with `# `" check would wrongly pass it).

Fix: `Node` records `HasFrontmatterBlock bool`. `Load` sets it from the same
`splitFrontmatter` detection `ParseFrontmatter` already runs (via a new
`ParseFrontmatterDetailed` that also returns the `present` bit). One plumbing
change serves both issues.

- **Index (§8/§12):** non-root index FAILS on any present block (empty or keyed);
  root index with a present-but-empty block FAILS (block present, `okf_version`
  absent — per the existing root rule); no block stays SILENT.
- **Log (§9 title):** a log with a present frontmatter block FAILS AC-V4.

### §9 log validation (`validateLog`, called from `validateReserved`)

For each `log.md` (any depth), on the RAW file content (title lives in body only
when no frontmatter, so validate the on-disk shape):

- **AC-V4 title:** must begin with a `# ` title; a frontmatter-bearing log FAILS.
- **AC-V1 date headings:** every `## ` heading must `time.Parse("2006-01-02")`;
  a bad heading yields `date heading "<h>" is not a valid YYYY-MM-DD date (§9)`.
- **AC-V2 order:** consecutive date headings non-increasing; a violation yields
  `date headings are not newest first (§9): A before B`. Equal adjacent = SILENT.
- **AC-V3 headings present:** a log with entry content but zero date headings
  yields `log has no ISO 8601 date headings (§9)`. SILENT on: empty file,
  title-only, and the `bundle init` scaffold (title + placeholder line).
- **AC-V5 legacy bullets:** `- YYYY-MM-DD — msg` lines before the first date
  heading yield `log has legacy dated bullets not grouped under date headings
  (§9); run okfctl log append to regroup`.

Findings surface through the existing `FAIL <path>: <message>` output layer.

### §9-shaped `AppendLog` (#177)

Parse the existing file into: pre-heading preamble (everything before the first
`## ` line, verbatim) and the date-grouped remainder. Then:

- **Regroup** any legacy `- YYYY-MM-DD — msg` lines found in the preamble into
  `* msg` entries under their date group, merging into an existing `## date`
  group when present, preserving intra-group order, keeping the whole log
  newest-first (AC-L1). Idempotent and lossless (AC-L2). A legacy bullet carries
  its **continuation block** — every following line (indented tables, nested
  lists, multi-line prose, embedded blank lines) up to the next top-level bullet
  or heading — into its date group verbatim. Without this the block is stranded
  in the preamble above the first heading, detached from its entry and date:
  silent history corruption of an append-only record that `validate` cannot see
  (it only inspects headings). A log whose legacy bullets are all single-line
  regroups byte-identically to the plain `* msg` form.
- **Insert** the new entry as `* <message>` (no inline date): if the newest date
  heading equals today (UTC), prepend to that group; else insert `## <today>` +
  `* <message>` above the first heading (AC-A2).
- **Create** with `logHeader` only when the file is absent; drop the scaffold
  placeholder (AC-A1).

All node-mutation callers (`cmd/derived.go`) route through `AppendLog`, so they
inherit §9 output (AC-A4, covered by a `node`-command integration test).

## Legacy corpus convergence (AC-L3)

`~/src/knowledge-base/bundles/knowledge`: 14 log.md. `validate` flags exactly the
4 legacy-bearing logs (root, research, content, design); the other 10 silent.
`log append` regroups on write (chosen path — no separate `migrate` step). After
one append at root + an append-triggered regroup per neighborhood log, `validate`
is clean on all 14; the root log retains all 366 legacy entries across 18 date
groups. Counts reported in the PR body.

## Out of scope

KB edits and the KB CI okfctl-pin bump (a separate later step).
