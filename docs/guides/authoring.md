# Starting and authoring a bundle

You'll start in an empty directory and end with a couple of linked nodes, then
move one, delete one, and hold new ones to a template. The docs test suite replays the blocks below
whenever the binary changes.

## From nothing to a valid bundle

```text
$ okfctl bundle init mykb
Initialized OKF bundle in mykb
$ okfctl node new concepts/tannin.md --type Reference --title "Tannin" --bundle mykb
Created mykb/concepts/tannin.md
$ okfctl node new concepts/oak.md --type Reference --title "Oak" --bundle mykb
Created mykb/concepts/oak.md
$ echo 'Oak barrels add [tannin](tannin.md) to red wine.' >> mykb/concepts/oak.md
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
$ okfctl bundle info mykb
nodes: 2
reserved: 3
okf_version: 0.3 (declared in index.md)
```

`bundle init` wrote two files and no nodes. One is a root `index.md` that
declares `okf_version: "0.3"` (OKF v0.3 §8.1). The other is an empty `log.md`. Each `node new`
then created a node and did the bookkeeping too:

```text
$ cat mykb/concepts/tannin.md
---
type: Reference
title: Tannin
created: <TIMESTAMP>
modified: <TIMESTAMP>
---

# Tannin
$ cat mykb/concepts/index.md
# Concepts

## Concepts

* [Oak](oak.md)
* [Tannin](tannin.md)
$ okfctl index check mykb
OK: index.md is current
```

You got frontmatter with timestamps and a heading. Behind the scenes you also
got a new `concepts/index.md`, an updated root index, and a line in `log.md`,
which is why `reserved: 3` counts three files. You never ran `index build`.
The node commands update the two index files as they go, and the
[index and freshness guide](index-and-freshness.md) explains when you'd need
to run it yourself.

`--type` is the one flag `node new` insists on. Having a non-empty `type` is
the only thing OKF demands of a concept (§4.1). The spec leaves its values open, and
any string passes. Leave it off and you get this:

<!-- transcript: exit 1 okfctl node new concepts/acidity.md --title "Acidity" --bundle mykb -->
```text
$ okfctl node new concepts/acidity.md --title "Acidity" --bundle mykb
okfctl: --type is required (OKF §4.1 / §11: every node needs a non-empty type)
```

## Moving and removing nodes

A node's path is the only name it has. Rename the file by hand and the link
in `oak.md` points at nothing. `node mv` fixes it for you. Try a dry run first:

```text
$ okfctl node mv concepts/tannin.md concepts/tannins.md --bundle mykb --dry-run
move concepts/tannin.md -> concepts/tannins.md
  rewrite concepts/oak.md: tannin.md -> tannins.md
$ okfctl node mv concepts/tannin.md concepts/tannins.md --bundle mykb
Moved concepts/tannin.md -> concepts/tannins.md (1 inbound link(s) rewritten)
$ tail -1 mykb/concepts/oak.md
Oak barrels add [tannin](tannins.md) to red wine.
```

Look at the last line. The link in `oak.md` now says `tannins.md`, and it's
still the short relative link you typed. A diff of the move shows one changed
word. `node mv` logged the move and rebuilt the index as well.

`node rm` deletes a node and tells you which nodes lost their last inbound link:

```text
$ okfctl node rm concepts/oak.md --bundle mykb --dry-run
remove concepts/oak.md
  orphaned: concepts/tannins.md
```

Oak was the only node that linked to tannins. With oak gone, tannins still
sits in the bundle and still shows up in its directory's index, but no other
node points at it, and you'd want to know that while you can still change your
mind. The dry run wrote nothing. Leave off `--dry-run` to delete the file.

`node rm` won't edit a link that pointed *at* the removed node. Those links
break where they sit, and the
[curation health guide](curation-health.md) shows how `lint` and `analyze`
report them.

## Record changes you make by hand

`node new`, `mv`, and `rm` wrote their own log lines. If you open `oak.md` in
vim and reword it, okfctl can't know. Tell the log:

```text
$ okfctl log append mykb --message "reworded the oak node"
Appended log entry
$ okfctl log show mykb
# Change Log

## <DATE>
* reworded the oak node
* moved concepts/tannin.md -> concepts/tannins.md
* created concepts/oak.md
* created concepts/tannin.md
```

New entries go on top under today's date (§9). Or skip the extra step:
`okfctl node edit` opens the node in `$OKFCTL_EDITOR` (falling back to
`$VISUAL`, `$EDITOR`, and then `vi`) and, when you save and quit, validates the
bundle again, bumps `modified`, and writes the log line for you.

## Hold a team to a convention with a type template

Suppose you want a one-line description on each `Reference` node. The index
prints it next to the link. OKF won't make anyone write one, but a node whose
`type` is `Type Template` will:

```markdown
---
type: Type Template
title: Reference template
target_type: Reference
required_fields: [title, description]
recommended_fields: [tags]
body_sections: [Summary, Sources]
---

# Reference template

Every Reference node needs a title and a one-line description.
```

Save it anywhere in the bundle and `node new` picks it up:

<!-- transcript: setup
$ okfctl bundle init teamkb >/dev/null
$ mkdir -p teamkb/templates
$ cp "$TRANSCRIPT_FIXTURES/authoring/reference.md" teamkb/templates/reference.md
-->
```text
$ okfctl template list teamkb
Reference	2 required field(s), 2 body section(s)	(source: templates/reference.md)
$ okfctl node new concepts/legs.md --type Reference --title "Legs" --bundle teamkb
Created teamkb/concepts/legs.md (from Reference template)
$ cat teamkb/concepts/legs.md
---
type: Reference
title: Legs
created: <TIMESTAMP>
modified: <TIMESTAMP>
description: TODO
tags:
---

# Legs

## Summary

## Sources
```

Look at what came out. Where the template wants a `description` you get a
`TODO` to replace. `tags` sits empty because the template only recommends it.
The two body sections are there as headings with nothing under them.

Now delete that `TODO` line and run `validate` three ways:

<!-- transcript: exit 1 okfctl validate --templates --strict teamkb -->
```text
$ grep -v '^description:' teamkb/concepts/legs.md > legs.tmp
$ mv legs.tmp teamkb/concepts/legs.md
$ okfctl validate teamkb
OK: bundle conforms to the OKF spec floor
$ okfctl validate --templates teamkb
warning concepts/legs.md: missing required field: description (template Reference)
1 drift warning(s)
$ okfctl validate --templates --strict teamkb
warning concepts/legs.md: missing required field: description (template Reference)
1 drift warning(s)
okfctl: 1 drift warning(s)
```

The floor passes. OKF never asked for a `description` in the first place. Plain
`validate` doesn't read templates at all. You need `--templates` for that, and
even then the drift is a warning with exit 0 until `--strict` turns it into
exit 1 in CI.
A bundle with no template sees the spec and nothing more.

Templates travel. Put them in a bundle of their own, point at it from each
bundle's `.okf` file with a `templates:` key (or use the `--templates-from <dir>`
flag for a single run), and a local template overrides a shared one whenever
the two name the same `target_type`.

## Next

The [index and freshness guide](index-and-freshness.md) puts `index check` in
CI. The [curation health guide](curation-health.md) adds `lint --strict` next
to it.
