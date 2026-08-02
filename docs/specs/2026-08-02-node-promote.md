# okfctl — `node promote`: promote directory-as-concept index.md in bulk

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` §6 (Index Files), §11 (Versioning marker).
**Base:** `main` @ `59c22ac`  **Branch:** `topic/node-promote`

## Problem

Running `okfctl validate` against a real corpus authored on the intuition that a
**directory is a concept** — where `index.md` carries the concept's frontmatter
and body — produces one mechanical failure per non-root index:

    FAIL: index files contain no frontmatter (§6); frontmatter is permitted only
    on the bundle-root index and only for okf_version (§11)

This is the OKF spec floor (`validateReserved`, `internal/okf/validate.go`)
correctly rejecting a shape it does not model: for OKF, `index.md` is a
GENERATED navigation surface and a concept is an ordinary named file. The
directory-as-concept shape is how Obsidian folder notes, Hugo `_index.md`,
Jekyll collections, and most wikis behave, so a large share of imported corpora
land on it and hit this wall on first contact — 191 identical failures against a
1,246-node corpus, ~98% of all findings.

There is no remediation verb today. `okfctl node` offers
`edit / list / mv / new / refresh / rm / show`. Fixing by hand per directory
means: choose a new filename, move the file, preserve `created`/`modified`,
rewrite every inbound link, and regenerate the index — and a wrong link is
silent, which is exactly the defect the `broken-link` gate (#34, shipped in
`59c22ac`) exists to catch.

## Goal

Add a remediation verb in the spirit of the existing `node refresh` precedent
(bulk fix, `--dry-run`, immutable `created`, verbatim bodies, maintained
`log.md`/`index.md`):

    okfctl node promote <bundle> [--name <basename>] [--dry-run]

For every NON-ROOT `index.md` that carries frontmatter, move it to a sibling
concept file, preserve its body verbatim and its `created` immutable, rewrite
inbound links (both `foo/` and `foo/index.md` spellings), regenerate the real
`index.md`, and append to `log.md`. The bundle-root index is left alone —
root frontmatter (`okf_version`) is legal per §11.

## Design

### Detection — what is promotable

A reserved file is a promotable directory-concept index when ALL hold:

- its base name is `index.md`;
- it is NOT the bundle-root index (path `!= "index.md"`);
- it carries a non-empty parsed frontmatter block (`len(Frontmatter) > 0`).

This is exactly the predicate `validateReserved` flags: a non-root index with
`len(Frontmatter) > 0`. A non-root index with no frontmatter is already
conformant and is left untouched. A node with unparseable frontmatter
(`Frontmatter == nil`) is a different failure class (`unparseable frontmatter`)
and out of scope — promote skips it rather than guessing.

### Destination filename

`foo/index.md` → `foo/<basename>.md`, where `<basename>` defaults to the
directory's own base name (`foo/index.md` → `foo/foo.md`). `--name overview`
overrides it uniformly for every promoted node (`foo/overview.md`,
`bar/overview.md`). A destination that already exists as a concept node is a
hard error (never overwrite authored content).

### Why not reuse `PlanMove`/`ApplyMove`

The existing move machinery cannot be reused directly:

1. `PlanMove`/`ApplyMove` reject reserved paths (`IsReservedPath`), and the
   source here IS a reserved `index.md`.
2. `scanNodeLinks` only resolves links whose target is an existing **concept
   node**. An index is not a node, so a link to `foo/index.md` or `foo/` never
   resolves as an edge and would be invisible to `PlanMove`.

So promote gets a dedicated planner/applier in `internal/okf/promote.go` that
understands the two directory-style spellings.

### Link rewriting — both spellings

For each promoted `foo/index.md` → `foo/foo.md`, scan every concept node AND
reserved file body for markdown links whose target, resolved against the
linking file's directory, points at the promoted directory in either spelling:

- `foo/index.md` (explicit index file), and
- `foo/` (directory-style, trailing slash) — the CommonMark spelling the OKF
  §6 index itself uses for subdirectory links (`* [Foo](foo/)`).

Both resolve to the same promoted directory. Each such link is rewritten to
target the new concept path, **preserving the author's relative form**
(root-relative stays root-relative, `/`-absolute stays absolute, dir-relative is
recomputed relative to the linking file's directory) and preserving any
CommonMark title tail verbatim — matching `PlanMove`'s form-preservation
contract.

The moved index's OWN body travels verbatim to the new concept file; its
outbound links are unchanged (they move with it, same as `node mv`).

### Body verbatim, `created` immutable

The promoted file is written as: its existing frontmatter block (round-tripped
key-order-preserving, `created` untouched; `modified` MAY be stamped to now,
consistent with `node refresh`) followed by its body **byte-for-byte**. The
body is the region after the closing `---` fence, taken from the raw on-disk
bytes (`splitFrontmatterRaw`) so no reflow, re-wrap, or whitespace change can
occur. `created` is never written by promote.

### Regenerate indexes + log

After all moves and rewrites are applied:

- `WriteIndex(b)` regenerates every content-bearing directory's `index.md` from
  the reloaded bundle. Because the promoted file is now a concept node in
  `foo/`, `WriteIndex` produces a fresh `foo/index.md` navigation surface with
  NO frontmatter (§6) that lists the promoted concept.
- `AppendLog` records one `promoted foo/index.md -> foo/foo.md` line per node.

Both reuse the existing derived-artifact writers — promote adds no new index or
log machinery.

### `--dry-run`

Lists every planned move and every planned link rewrite and writes ZERO bytes.
This is load-bearing: it is the only reason anyone runs a bulk rewriter against
a corpus they care about. Verified by asserting the tree is byte-identical
after a dry run.

## Acceptance

- `--dry-run` on a multi-directory fixture lists every move + rewrite and leaves
  the tree byte-identical.
- A real run on that fixture leaves `okfctl validate` exiting 0.
- Body preserved verbatim (byte-compare); `created` unchanged; only `modified`
  may move.
- Inbound links rewritten for BOTH `foo/` and `foo/index.md` spellings.
- Regenerated `index.md` files carry no frontmatter and are valid navigation
  surfaces.
- `log.md` appended per promoted node.
- Bundle-root index untouched.
- `lint --strict` reports ZERO `broken-link` findings after a promote run — the
  criterion that matters most (a bulk link rewriter that cannot prove it broke
  no links is the exact silent failure #34 exists to catch).
- Full suite green: `gofmt`, `go vet`, `CGO_ENABLED=0 go build ./...`,
  `go test ./... -race`.
- Real corpus (`~/src/knowledge-base`, bundle `bundles/knowledge`): `--dry-run`
  move count recorded, real run on a scratch copy, before/after `validate`
  finding counts reported.

## Out of scope

- Changing the spec's index model (the spec is right).
- The sibling fixes (#33 link-URL-as-prose, #34 broken-link gate — already
  shipped).
