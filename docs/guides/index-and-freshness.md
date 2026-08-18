# Keeping `index.md` current and fixing freshness drift

## Rebuilding the index

Per OKF SPEC §8, one `index.md` is emitted in **each** directory that holds
concepts or subdirectories, enumerating only that directory's own contents with
**dir-relative** links and each concept's `description`. Only the bundle-root
index carries frontmatter (the §12 `okf_version` marker); every nested index
carries none.

- `okfctl index build [dir]` regenerates the reserved `index.md` files from the
  current bundle. Orphaned indexes left in a now-empty directory are pruned.
- `okfctl index check [dir]` verifies every directory's `index.md` is current;
  it exits nonzero if any nested index is stale, missing, or orphaned—wire it
  into CI to keep the index honest.

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
3f9a1c2e8b7d6a5c4e3f2a1b0c9d8e7f6a5b4c3d   # v0.2 frontmatter key sweep
```

When a node's last-touching commit is on the list, the drift comparison **walks
back to the prior real commit** for that file. Incremental edits (commits *not*
on the list) still drift normally—the check isn't narrowed into uselessness.
Full or abbreviated (≥7-char) SHAs both match. This is the recommended cure when
`node refresh` refuses a bulk-dominated plan (the guardrail against collapsing
real authoring dates into one migration date).
