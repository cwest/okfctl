# Keeping `index.md` current and fixing freshness drift

## Rebuilding the index

Per OKF SPEC §8, one `index.md` is emitted in **each** directory that holds
concepts or subdirectories, enumerating only that directory's own contents with
**dir-relative** links and each concept's `description`. Only the bundle-root
index carries frontmatter; every nested index carries none. The root block holds
bundle metadata (OKF v0.3 §8.1): `okf_version`, `title`, `description`,
`license`, and any extension keys such as `x-acme-owner`. `index build` keeps an
existing root block byte-for-byte. When the root index has no block, `index
build` emits an `okf_version`-only block from a legacy `.okf` sidecar, if there
is one.

- `okfctl index build [dir]` regenerates the reserved `index.md` files from the
  current bundle. Orphaned indexes left in a now-empty directory are pruned.
- `okfctl index check [dir]` verifies every directory's `index.md` is current;
  it exits nonzero if any nested index is stale, missing, or orphaned—wire it
  into CI to keep the index honest.

Each subdirectory entry ends with a shape suffix the tool owns, so a reader can
pick a branch without opening it:

```
* [Concepts](concepts/) (4 concepts · Concept, Reference)
```

The suffix gives the child's own concept count, its types, the tags at least
`--shape-tag-min` (default 2) of its concepts share, up to `--shape-tag-max`
(default 12), and a `· N in subtree` total when the child nests deeper.
`--no-shape` drops it. A description you write on a subdirectory entry
(`* [Concepts](concepts/) - tasting vocabulary`) survives every rebuild, and so
does an existing H1. An index checked out with CRLF line endings stays CRLF and
doesn't read as stale.

## Git drift

`okfctl validate` also reports **git drift**—any node whose frontmatter
`modified` disagrees with its git last-commit date—as advisory warnings
(read-only; run `okfctl node refresh` to fix them; degrades to nothing outside a
git repo). `node refresh` bulk-rewrites every drifting node's `modified` to its
git last-commit day; `created` is immutable and never touched.

## `.okf-drift-ignore-revs`—opt a bulk mechanical commit out of git drift

Git drift infers a node's freshness from its last-touching commit's date. That is
right for an incremental edit, but a **bulk mechanical commit**—a one-time
migration that rewrites frontmatter across the whole corpus on day one—has no
authoring intent, and treating its date as the node's `modified` collapses the
real authoring history into the migration date. Git records *when* a commit
landed, not *why*, so the tool can't tell the two apart on its own.

Declare the intent with a checked-in `.okf-drift-ignore-revs` at the bundle root,
mirroring `git blame --ignore-revs-file`—a convention users already understand:

```
# Mechanical migration commits — opt these out of git drift.
# One commit SHA per line; blank lines and #-comments are ignored.
3f9a1c2e8b7d6a5c4e3f2a1b0c9d8e7f6a5b4c3d   # okfctl migrate to v0.3
```

When a node's last-touching commit is on the list, the drift comparison **walks
back to the prior real commit** for that file. Incremental edits (commits *not*
on the list) still drift normally—the check isn't narrowed into uselessness.
Full or abbreviated (≥7-char) SHAs both match. This is the recommended cure when
`node refresh` refuses a bulk-dominated plan (the guardrail against collapsing
real authoring dates into one migration date). `okfctl migrate --apply` prints
the same reminder after it edits files.
