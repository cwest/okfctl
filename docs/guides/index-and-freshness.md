# Keeping `index.md` current and fixing freshness drift

Your bundle can go stale in two ways you won't see at first. Its `index.md`
files stop listing all the nodes. Its `modified` dates drift away from what
git remembers. Below, okfctl finds each one on a three-node bundle, and each
block ends on the line that fails CI.

## Catch a stale index

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/index-and-freshness/mykb" mykb
-->
```text
$ okfctl index check mykb
OK: index.md is current
$ cp "$TRANSCRIPT_FIXTURES/index-and-freshness/body.md" mykb/wine/body.md
```

<!-- transcript: exit 1 okfctl index check mykb -->
```text
$ okfctl index check mykb
index.md is out of date; run `okfctl index build` to regenerate
okfctl: index.md is out of date
```

The node commands rewrite `index.md` as they go. Nothing does that for a file
you copy in by hand. The same goes for a file a script writes and for one that
arrives in a `git pull`. Here `body.md` stays unlisted until you rebuild:

```text
$ okfctl index build mykb
Wrote 2 index.md file(s) under mykb
$ okfctl index check mykb
OK: index.md is current
```

`index check` exits 1 if an `index.md` is stale or missing. It also exits 1 if
one is left over in a directory that has nothing in it now. `index build` fixes
all three. It deletes a leftover only if okfctl wrote it (more on that below).
With the check in CI, an index can't lag the nodes by more than one pull
request.

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

Body is in the list now, with its `description` after the link. The root says
`wine/` has three concepts. OKF v0.3 §8 asks for this layout: one
`index.md` per directory, listing that directory and nothing deeper, with links
relative to it.

## What `index build` keeps and what it owns

Rebuilding is safe to repeat. The root `index.md` is the one index that gets
frontmatter. §8.1 says a tool that regenerates the file MUST leave that
block as it was. okfctl does, byte for byte. `okf_version`, `title`,
`description`, `license`, and keys like `x-acme-owner` all come through. If
there's no block at all and a legacy `.okf` sidecar declares `okf_version`, you
get a new block holding that one key.

okfctl adds the `(3 concepts · Reference · shared tags: taste)` part itself.
It tells a reader (a person or an agent) what `wine/` holds so they don't
have to open it: how many concepts there are and of what type, with up to 12 tags
(`--shape-tag-max`) that at least two of them carry (`--shape-tag-min`, default
2). Directories that nest deeper also get a `· N in
subtree` total. `--no-shape` drops the whole thing.

A few hand edits carry through a rebuild. The H1 survives. So does a
description you add to a subdirectory entry, like
`* [Wine](wine/) - tasting vocabulary`. Windows line endings survive as well.
An `index.md` checked out with CRLF stays CRLF. `index check` won't call it
stale over that.

## When an `index.md` has your own writing in it

okfctl rewrites an `index.md` only when every line in it is one okfctl writes:
the H1, the `## Subdirectories` and `## Concepts` headings, the link bullets,
and its placeholder lines. Add an intro paragraph or notes of your own and the
file is yours. `index build` leaves it alone and tells you:

<!-- transcript: setup
$ printf '# Wine\n\nTasting notes, newest first.\n' > mykb/wine/index.md
-->
<!-- transcript: exit 1 okfctl index build mykb -->
```text
$ okfctl index build mykb
Wrote 1 index.md file(s) under mykb
warning: kept hand-written wine/index.md: okfctl did not generate it, so it was not rewritten or removed
hint: edit the file(s) by hand, or run `okfctl index build --overwrite-hand-written` to replace them with the generated index
okfctl: kept 1 hand-written index.md file(s)
$ cat mykb/wine/index.md
# Wine

Tasting notes, newest first.
```

The exit code is 1, so CI catches it. `index check` fails on the same file and
says it's hand-written. The node commands keep the file too and print the same
warning, but the node change itself still goes through.

The same rule covers a directory that no longer holds nodes. okfctl deletes
the old `index.md` there only if okfctl wrote it. One you wrote stays put.

You have two ways out. Put the entries you want into the file by hand, or let
okfctl take the file back:

```text
$ okfctl index build --overwrite-hand-written mykb
Wrote 2 index.md file(s) under mykb
$ okfctl index check mykb
OK: index.md is current
```

`--overwrite-hand-written` replaces the file with the generated index. On the
bundle-root `index.md` it still keeps the frontmatter and the H1 (§8.1). OKF
v0.3 §8 lets a tool generate `index.md`. It doesn't ask the tool to throw away
what a person wrote, and okfctl won't do that unless you pass the flag.

## Catch a `modified` date that git contradicts

This copy of the bundle is in git with two commits. The second one, from April
10, fixes a typo in `tannin.md` and leaves its `modified` field at March 2:

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

A stale date is legal OKF. That's why plain `validate` warns and still exits 0,
and why CI wants the `--strict` form (exit 1). The check never writes to a file.
Outside a git repository there's no history to compare against. You won't
get a warning.

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

`node refresh` set `modified` to the day of the last commit and wrote a line to
`log.md`. It didn't touch `created` or the body. To fix one node and leave the
others, name it: `okfctl node refresh mykb wine/tannin.md`.

## Keep a bulk commit from erasing your dates

Git knows the day a file changed. It has no idea why. Say that on May 1 you add a
`wine` tag to all three nodes in one commit:

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

You didn't rewrite acidity on May 1. You added a word to its tags. If you run
`node refresh` now, it puts May 1 on acidity and tannin. March 2 and April 10
are gone for good. (Body has no `modified` field at all. There's nothing
there for git to contradict.)

Put the commit in `.okf-drift-ignore-revs` at the bundle root:

```text
$ git -C mykb rev-parse HEAD >> mykb/.okf-drift-ignore-revs
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
```

If a node's last commit is on the list, drift looks past it to the commit
under it. Tannin falls back to the typo fix. For acidity it's the first commit.
Commits you didn't list still count (next week's typo fix gets caught as
usual). The format is the one `git blame --ignore-revs-file` reads. Put one SHA
per line, the whole hash or at least seven characters of it. Blank lines and
`#` comments are ignored. Check the file in.

Two parts of okfctl point you here. `okfctl migrate --apply` prints a hint to
list its commit after you make it. And `node refresh` won't run if 10 or more
of the nodes it would change (and at least half of them) trace back to one
commit. It prints that commit and the line to add. `--yes` overrides the
refusal, but then it rewrites the dates.

## Next

The [curation health guide](curation-health.md) adds `lint --strict` and
`eval transparency --strict`, the other two checks that belong in CI.
