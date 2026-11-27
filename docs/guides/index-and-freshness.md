# Keeping `index.md` current and fixing freshness drift

Bundles go stale in two ways you won't notice at first. The `index.md` files
fall behind the nodes. Or a node's `modified` date stops matching what git
remembers. okfctl catches both, and below you'll watch it happen on a
three-node bundle, with output pasted from my terminal and the CI line that
fails each time.

## Catch a stale index

The node commands rebuild `index.md` for you, but nothing rebuilds it when a
file shows up some other way: your editor, a script, a `git pull` of someone
else's work. Start with two nodes and a current index, then copy
`body.md` in by hand:

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/index-and-freshness/mykb" mykb
-->
```text
$ okfctl index check mykb
OK: index.md is current
$ cp "$TRANSCRIPT_FIXTURES/index-and-freshness/body.md" mykb/wine/body.md
```

Now there's a node with no index entry:

<!-- transcript: exit 1 okfctl index check mykb -->
```text
$ okfctl index check mykb
index.md is out of date; run `okfctl index build` to regenerate
okfctl: index.md is out of date
```

Rebuild it, and the check passes:

```text
$ okfctl index build mykb
Wrote 2 index.md file(s) under mykb
$ okfctl index check mykb
OK: index.md is current
```

`index check` exits 1 when any `index.md` in the bundle is stale or missing,
and when one is left behind in a directory that no longer has content. Put it in CI and an
index can't fall behind the nodes for longer than one pull request. `index
build` fixes all three cases, and it deletes a leftover index in an empty
directory.

This is what it wrote:

```text
$ cat mykb/index.md
---
okf_version: "0.3"
---

# Wine notes

## Subdirectories

* [Wine](wine/) (3 concepts · Reference · shared tags: taste)
$ cat mykb/wine/index.md
# Wine

## Concepts

* [Acidity](acidity.md) - Why a wine tastes fresh.
* [Body](body.md) - How heavy a wine feels in the mouth.
* [Tannin](tannin.md) - The drying grip in red wine.
```

There's one `index.md` per directory, and each lists only its own directory,
with links relative to it (OKF v0.3 §8). Each node's `description` becomes the
text after its link. The new body node is listed, and the root entry for
`wine/` now counts three concepts.

## What `index build` keeps and what it owns

Rebuilding is safe to run as often as you like. The root `index.md` is the one
index allowed frontmatter, and §8.1 says a tool that regenerates it MUST leave
that alone. okfctl does, byte for byte, down to `okf_version`, `title`,
`description`, `license`, and extension keys like `x-acme-owner`. With no block
there at all, you get one holding `okf_version` if a legacy `.okf` sidecar
declares it.

Then there's `(3 concepts · Reference · shared tags: taste)`. okfctl writes
that suffix so a reader, person or agent, can tell what's in `wine/` without
opening it. It counts the directory's own concepts, lists their types, and
names up to 12 tags (`--shape-tag-max`) that at least two of them carry
(`--shape-tag-min`, default 2). A directory that nests deeper gets a `· N in
subtree` total. `--no-shape` drops the whole suffix.

Hand edits to an index usually vanish on the next build. Two don't. One is a
description you add to a subdirectory entry
(`* [Wine](wine/) - tasting vocabulary`), and the other is the H1. Neither do Windows line
endings. An `index.md` checked out with CRLF stays CRLF, and `index check`
won't call it stale over that.

## Catch a `modified` date that git contradicts

A node's `modified` field says when it last changed. Git also knows. When the
two disagree, `validate` says so. This copy of the bundle has two commits. The
first is from March 2. The second, on April 10, fixes a typo in `tannin.md`
and leaves its `modified` alone:

<!-- transcript: setup
$ git -C mykb init -q
$ git -C mykb config user.email you@example.com
$ git -C mykb config user.name You
$ git -C mykb add -A
$ GIT_COMMITTER_DATE=2026-03-02T12:00:00Z git -C mykb commit -q --date=2026-03-02T12:00:00Z -m "Start the wine notes"
$ sed 's/dyring/drying/' mykb/wine/tannin.md > tannin.tmp
$ mv tannin.tmp mykb/wine/tannin.md
$ GIT_COMMITTER_DATE=2026-04-10T12:00:00Z git -C mykb commit -q --date=2026-04-10T12:00:00Z -am "Fix a typo in tannin"
-->
<!-- transcript: exit 1 okfctl validate --strict mykb -->
```text
$ git -C mykb log --format='%cs %s'
2026-04-10 Fix a typo in tannin
2026-03-02 Start the wine notes
$ okfctl validate mykb
warning wine/tannin.md: modified 2026-03-02 disagrees with git last-commit 2026-04-10; run `okfctl node refresh` to fix it (or `okfctl node edit` per node)
1 drift warning(s)
$ okfctl validate --strict mykb
warning wine/tannin.md: modified 2026-03-02 disagrees with git last-commit 2026-04-10; run `okfctl node refresh` to fix it (or `okfctl node edit` per node)
1 drift warning(s)
okfctl: 1 drift warning(s)
```

Stale dates are legal OKF. Plain `validate` warns and exits 0, and with
`--strict` it exits 1 for CI. The check never writes to anything. Outside a git
repository there's no history to compare with, and you won't see a warning.

`node refresh` sets each drifting node's `modified` to the day of its last
commit:

```text
$ okfctl node refresh mykb --dry-run
would refresh wine/tannin.md: 2026-03-02 -> 2026-04-10
1 node(s) would be refreshed (dry run; nothing written)
$ okfctl node refresh mykb
refreshed wine/tannin.md: 2026-03-02 -> 2026-04-10
1 node(s) refreshed
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
```

It leaves `created` and the body alone and logs each change in `log.md`. Name a
node after the bundle to fix just that one:
`okfctl node refresh mykb wine/tannin.md`.

## Keep a bulk commit from erasing your dates

Git knows when a commit landed. It has no idea why. Say on May 1 you add a
`wine` tag to every node in one sweep:

<!-- transcript: setup
$ GIT_COMMITTER_DATE=2026-04-10T12:30:00Z git -C mykb commit -q --date=2026-04-10T12:30:00Z -am "Refresh modified"
$ for n in tannin acidity body; do sed 's/^tags: \[taste\]/tags: [taste, wine]/' mykb/wine/$n.md > tag.tmp && mv tag.tmp mykb/wine/$n.md; done
$ GIT_COMMITTER_DATE=2026-05-01T12:00:00Z git -C mykb commit -q --date=2026-05-01T12:00:00Z -am "Tag every node wine"
-->
```text
$ okfctl validate mykb
warning wine/acidity.md: modified 2026-03-02 disagrees with git last-commit 2026-05-01; run `okfctl node refresh` to fix it (or `okfctl node edit` per node)
warning wine/tannin.md: modified 2026-04-10 disagrees with git last-commit 2026-05-01; run `okfctl node refresh` to fix it (or `okfctl node edit` per node)
2 drift warning(s)
```

You didn't rewrite acidity on May 1; you added a word to its tags. Run
`node refresh` now and it stamps May 1 on both nodes, and March 2 and April 10
are gone for good. (Body has no `modified` field at all, and there's nothing
for git to contradict.)

List the commit in `.okf-drift-ignore-revs` at the bundle root instead:

```text
$ git -C mykb rev-parse HEAD >> mykb/.okf-drift-ignore-revs
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
```

When a node's last commit is on the list, drift looks past it to the one
underneath. Here that's the typo fix for tannin and the first commit for
acidity. Commits you didn't
list drift as usual. Next week's typo fix still gets caught. If you've used
`git blame --ignore-revs-file`, the format is the same: one SHA per line, the
whole hash or at least seven characters of it, with blank lines and `#`
comments ignored. Check
it in.

Two parts of okfctl point you here. `okfctl migrate --apply` prints a hint to
list its commit after you make it. And `node refresh` won't run when at least
10 of the nodes it would change, and at least half of them, trace back to one
commit. It prints that commit and the line to add instead. `--yes` overrides
the refusal, but that rewrites the dates.

## Next

The [curation health guide](curation-health.md) adds `lint --strict` and
`eval transparency --strict`, the other two checks that belong in CI.
