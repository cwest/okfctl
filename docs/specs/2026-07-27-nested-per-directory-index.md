# okfctl — Nested per-directory index.md conforming to OKF SPEC §8

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` §8 (Index Files), §12 (Versioning marker).
**Base:** `main` @ `3daa7a8`  **Branch:** `topic/nested-per-dir-index`

## Problem

`okfctl index build` emits a SINGLE flat bundle-root `index.md` that enumerates
every concept node in the bundle, grouped by top-level neighborhood, with
**bundle-relative** links (`wine/tannin.md`) and a per-node **type** annotation.

OKF SPEC §8 specifies a different model:

> An `index.md` file MAY appear in **any directory**, including the bundle root.
> It enumerates **the directory's contents** to support progressive disclosure.
> […] Entries SHOULD include the description from the linked concept's
> frontmatter.

Its example links a subdirectory as `* [Subdirectory](subdir/)` — a
**dir-relative** link from the index's own directory, not a bundle-relative path.

The flat model therefore diverges from the spec three ways:

1. It collapses N nested per-directory indexes into one, destroying the
   progressive disclosure §8 exists to provide.
2. It emits bundle-relative links (`wine/tannin.md`) where §8 is dir-relative
   (`tannin.md`, `subdir/`).
3. It annotates each entry with the node's `type` and omits the `description`,
   where §8 says entries SHOULD carry the `description`.

This is the same class of finding as the frontmatter fix already shipped: the
spec is ground truth; okfctl is the divergent producer, and a producer of a
spec-governed format must be its own first conformant consumer.

## Design

Emit **one `index.md` per content-bearing directory**, each enumerating ONLY its
own directory's immediate contents, linked relative to itself.

### What is a "content-bearing directory"

A directory that directly contains at least one concept node, OR at least one
subdirectory that is itself (transitively) content-bearing. The bundle root is
always content-bearing when the bundle has any node. This is exactly the set of
directories a reader would traverse for progressive disclosure — an empty
directory, or one whose entire subtree has no concepts, gets no index.

### What each index enumerates

For directory `D` (bundle-relative, `""` = root), in two sorted sections:

- **`## Subdirectories`** — each immediate child directory of `D` that is
  content-bearing, as `* [Title](child/) - description`. The link is the child
  directory name with a trailing slash (dir-relative). The child's title/
  description are read from the child directory's own `index.md` node when one
  is present on disk; absent that, the title falls back to a Title-cased form of
  the directory name and the description is omitted.
- **`## Concepts`** — each concept node that lives DIRECTLY in `D` (not in a
  subdirectory), as `* [Title](file.md) - description`, where `file.md` is the
  node's base name (dir-relative) and title/description come from its
  frontmatter. The `description` is included per §8 when present; a node with no
  `description` renders `* [Title](file.md)` with no trailing ` - `.

Both sections are emitted only when non-empty. Entries within each section are
sorted (subdirectories by directory name; concepts by base name) for
byte-stable, deterministic output.

Bullet form is `* ` and the title/description separator is ` - `, matching the
SPEC §8 example grammar verbatim. Section heading TEXT is a producer choice per
§8 ("one or more sections, each grouping … under a heading"); the retired
`tools/okf_index.py`'s `# Subdirectories`/`# Concepts` h1 + `<!-- BEGIN
GENERATED INDEX -->` marker block + em-dash + `type` annotation are that tool's
LOCAL convention and are explicitly NOT targets — this design uses `##` sections
and carries no marker block or boilerplate.

### Frontmatter (§8 / §12)

No `index.md` carries frontmatter, with the single §12 carve-out: the
**bundle-root** index MAY carry an `okf_version`-only block. That existing rule
(and its marker-preservation semantics) is retained unchanged — only the
bundle-root index may carry `okf_version`; every nested index carries no
frontmatter at all. `Validate` already enforces this per-file, so the generated
nested tree validates clean by construction.

### API

In `internal/okf`:

- `IndexDirs(b *Bundle) []string` — sorted bundle-relative directories that
  should carry an index (`""` for the root). The single source of truth for
  "which directories get an index," shared by build, check, and maintenance.
- `RenderDirIndex(b *Bundle, dir string) string` — the index body for one
  directory. The bundle-root case (`dir == ""`) carries the §12 okf_version
  frontmatter block when applicable; all others carry none.
- `RenderIndex(b *Bundle) string` — retained as `RenderDirIndex(b, "")` (the
  bundle-root index) so existing call sites keep compiling.
- `WriteIndex(b *Bundle) error` — writes an `index.md` into EVERY directory in
  `IndexDirs(b)`. Still the single writer for both `index build` and the
  automatic create/edit/delete/rename maintenance path, so the two cannot
  diverge.
- `IndexInSync(b *Bundle) (bool, string)` — in sync only when EVERY directory in
  `IndexDirs(b)` has an on-disk `index.md` equal to its `RenderDirIndex`, AND no
  stray generated `index.md` exists in a directory that should not have one. A
  missing, stale, or orphaned nested index is out of sync, with a report naming
  the offending path.

`cmd/index.go` and `cmd/derived.go` keep calling `WriteIndex`/`IndexInSync`
unchanged — the nesting is entirely inside the model.

## Done when

- `index build` on a multi-directory bundle emits one index per content-bearing
  directory, each dir-relative and enumerating only its own contents.
- `index check` exits 0 on that output; nonzero when any nested index is stale,
  missing, or orphaned.
- Table-driven tests cover: nested emission, dir-relative link form,
  subdirectory entries (`child/`), description passthrough, bundle-root
  `okf_version` retention, and the §8 no-frontmatter rule for non-root indexes.
- A real-corpus smoke run against `~/src/knowledge-base/bundles/knowledge`
  produces nested indexes and `index check` exits 0.
- `go test ./... -race -count=1` green; gofmt/vet clean.

## Out of scope

- Byte-for-byte convergence with the knowledge-base's legacy `tools/okf_index.py`
  output (marker block, h1 headings, `type` annotation).
- Any `cwest/knowledge-base` change, CI workflow edits, the release/tag.
