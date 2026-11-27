# Concepts

New to okfctl or to the Open Knowledge Format? Start here. Every idea on this
page comes with a command that proves it. Run the blocks in order in an empty
directory and you'll grow one wine bundle as you read. The output is what
okfctl printed when I ran it. CI replays each block against the current binary,
and the day okfctl prints something different, the build fails and this page
gets fixed.

## A Bundle Is a Directory of Markdown Files

```text
$ okfctl bundle init kb
Initialized OKF bundle in kb
$ okfctl bundle info kb
nodes: 0
reserved: 2
okf_version: 0.3 (declared in index.md)
$ cat kb/index.md
---
okf_version: "0.3"
---

# Knowledge Base

_Progressive-disclosure entry point. Add "start here" links as nodes land._
```

A **bundle** is a directory tree of Markdown files that follows the
[Open Knowledge Format](https://github.com/open-knowledge-format/open-knowledge-format/blob/main/SPEC.md)
(OKF §3). `bundle init` wrote two files, `index.md` and `log.md`, and no
concepts. The bundle-root `index.md` declares the OKF version in its
frontmatter (§8.1, §12). `bundle info` prints that version and the file it came
from. If a bundle names no version, you'll see `undeclared` there.

## A Node Is a File with a Non-Empty `type`

```text
$ okfctl node new wine/tannin --type Concept --title Tannin --bundle kb
Created kb/wine/tannin.md
$ cat kb/wine/tannin.md
---
type: Concept
title: Tannin
created: <TIMESTAMP>
modified: <TIMESTAMP>
---

# Tannin
```

A **node** is one concept. It's a Markdown file with a YAML frontmatter block,
and the one key OKF insists on is a non-empty `type` (§4.1). Leave it off and
`node new` refuses to write the file:

<!-- transcript: exit 1 okfctl node new wine/oak --bundle kb -->
```text
$ okfctl node new wine/oak --bundle kb
okfctl: --type is required (OKF §4.1 / §11: every node needs a non-empty type)
$ okfctl node new wine/oak --type "Barrel Note" --title Oak --bundle kb
Created kb/wine/oak.md
$ okfctl validate kb
OK: bundle conforms to the OKF spec floor
```

`Barrel Note` is a type I just invented, and it passes. OKF leaves `type` values
open, and a consumer has to accept a type it's never seen (§4.1, §11.2). okfctl doesn't
invent a list of approved types the spec never wrote.

A Markdown file with no frontmatter (or with frontmatter but no `type`) isn't
a node at all. OKF says to skip it (§3.2), so `validate` warns and passes.
Frontmatter that won't parse as YAML is another matter. It breaks conformance
(§11.1), and `validate` exits 1:

<!-- transcript: exit 1 okfctl validate kb -->
```text
$ printf '# Notes\n\nScratch file, no frontmatter.\n' > kb/notes.md
$ printf -- '---\ntype: [Concept\n---\n' > kb/wine/broken.md
$ okfctl validate kb
FAIL wine/broken.md: unparseable frontmatter
warning notes.md: not a concept (no frontmatter); skipped, does not affect conformance (OKF v0.3 §3.2)
okfctl: 1 conformance finding(s)
$ rm kb/notes.md kb/wine/broken.md
```

## okfctl Keeps `index.md` and `log.md` for You

```text
$ find kb -name '*.md' | sort
kb/index.md
kb/log.md
kb/wine/index.md
kb/wine/oak.md
kb/wine/tannin.md
$ cat kb/wine/index.md
# Wine

## Concepts

* [Oak](oak.md)
* [Tannin](tannin.md)
$ okfctl log show kb
# Change Log

## <DATE>
* created wine/oak.md
* created wine/tannin.md
```

OKF reserves two filenames at every level of a bundle, and neither one is ever
a concept (§3.1). `index.md` lists what a directory holds (§8). `log.md` is the
change history, newest date first (§9). You didn't write `wine/index.md` or
either log entry. `node new` did.

Add a file by hand and the index falls behind. `index check` exits 1, `index
build` regenerates every `index.md`, and the root index holds on to its
frontmatter (§8.1 says it has to):

<!-- transcript: exit 1 okfctl index check kb -->
```text
$ printf -- '---\ntype: Concept\ntitle: Acidity\n---\n\n# Acidity\n' > kb/wine/acidity.md
$ okfctl index check kb
index.md is out of date; run `okfctl index build` to regenerate
okfctl: index.md is out of date
```

```text
$ okfctl index build kb
Wrote 2 index.md file(s) under kb
$ okfctl index check kb
OK: index.md is current
$ head -3 kb/index.md
---
okf_version: "0.3"
---
```

For a change the tool didn't make itself, record it with `okfctl log append`.

## Links Form a Graph, and okfctl Walks It Both Ways

```text
$ printf '\nOak aging adds [tannin](tannin.md) to the wine.\n' >> kb/wine/oak.md
$ okfctl graph export --format dot kb
digraph okf {
  rankdir=LR;
  "wine/acidity.md" [label="Acidity"];
  "wine/oak.md" [label="Oak"];
  "wine/tannin.md" [label="Tannin"];
  "wine/oak.md" -> "wine/tannin.md";
}
$ okfctl search --neighbors wine/tannin.md kb
wine/oak.md                              Barrel Note  depth=1
```

Nodes link to each other with ordinary Markdown links (§6.1), and those links
are the edges of the bundle graph (§6.4). The export shows one directed edge,
from oak to tannin. Tannin has no link to oak, and `search --neighbors` finds
oak one hop away anyway. okfctl ignores edge direction when it walks the graph,
so a node that links to you counts as your neighbor the same as one you link to.
`graph export` also writes JSON (the default), and `okfctl serve` draws the
same graph in a browser.

A node's path is its identity, and renaming one changes the graph. `node mv`
rewrites every link that points at the old path:

```text
$ okfctl node mv wine/tannin wine/tannins --bundle kb
Moved wine/tannin.md -> wine/tannins.md (1 inbound link(s) rewritten)
$ tail -1 kb/wine/oak.md
Oak aging adds [tannin](tannins.md) to the wine.
```

## The Spec Floor Applies to Everyone, Templates Only When You Ask

<!-- transcript: setup
$ mkdir kb/templates
$ printf -- '---\ntype: Type Template\ntitle: Playbook template\ntarget_type: Playbook\nrequired_fields: [owner]\nbody_sections: [Steps]\n---\n\n# Playbook template\n' > kb/templates/playbook.md
-->
```text
$ cat kb/templates/playbook.md
---
type: Type Template
title: Playbook template
target_type: Playbook
required_fields: [owner]
body_sections: [Steps]
---

# Playbook template
$ okfctl node new runbooks/deploy --type Playbook --title Deploy --bundle kb
Created kb/runbooks/deploy.md (from Playbook template)
$ cat kb/runbooks/deploy.md
---
type: Playbook
title: Deploy
created: <TIMESTAMP>
modified: <TIMESTAMP>
owner: TODO
---

# Deploy

## Steps
```

`validate` always checks the **spec floor**, the minimum every conformant
bundle meets. Say your team wants an `owner` on every playbook and a `## Steps`
section in each one. You'd write a **type template** for that. It's an ordinary
node whose `type` is `Type Template`, so it lives in the bundle with everything
else, never in some tool config file. `node new` scaffolds from it, as you just
saw. `template list` and `template show` print the fields and sections a
bundle's templates want. Templates belong to okfctl. OKF doesn't define them.

The next playbook is written by hand. It has no `owner` and no `## Steps`, and
it carries an `x-` key OKF has never heard of:

<!-- transcript: setup
$ printf -- '---\ntype: Playbook\ntitle: Rollback\nx-acme-reviewed: no\n---\n\n# Rollback\n' > kb/runbooks/rollback.md
-->
<!-- transcript: exit 1 okfctl validate --templates --strict kb -->
```text
$ cat kb/runbooks/rollback.md
---
type: Playbook
title: Rollback
x-acme-reviewed: no
---

# Rollback
$ okfctl validate kb
OK: bundle conforms to the OKF spec floor
$ okfctl validate --templates kb
warning runbooks/rollback.md: missing required field: owner (template Playbook)
warning runbooks/rollback.md: missing body section: Steps (template Playbook)
2 drift warning(s)
$ okfctl validate --templates --strict kb
warning runbooks/rollback.md: missing required field: owner (template Playbook)
warning runbooks/rollback.md: missing body section: Steps (template Playbook)
2 drift warning(s)
okfctl: 2 drift warning(s)
```

Plain `validate` passes. OKF forbids a consumer from rejecting keys it doesn't
know (§4.1, §11.2), and the floor never looks at templates, so neither the unknown
key nor the missing `owner` counts against you. `--templates` turns the
template checks on as warnings, and `--strict` makes those warnings exit 1 for
CI. Nothing from a template ever leaks into the floor. If okfctl rejected an
unknown key or an unknown `type`, it would break the spec by being too picky,
and that's still breaking it.

## The Version a Bundle Declares Decides How okfctl Reads It

OKF is at **v0.3**, and so is okfctl. `bundle init` declares
`okf_version: "0.3"`, and `okfctl migrate` upgrades older bundles. okfctl reads
the version from the root `index.md`. It only looks at a legacy `.okf` sidecar
when the root index says nothing:

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/migrating/mykb" v01
-->
```text
$ cat v01/.okf
okf_version: 0.1
$ okfctl bundle info v01
nodes: 2
reserved: 2
okf_version: 0.1 (declared in .okf)
$ okfctl validate v01
OK: bundle conforms to the OKF spec floor
```

That bundle still carries a `timestamp` key and a body `# Citations` list. Those
are v0.1 habits. v0.2 renamed them to `generated.at` and frontmatter `sources`.
Consumers MAY fall back to the old forms (§12.1, Appendix B). That's why v0.1
bundles still read fine.

v0.3 has two breaking changes (§13.1). The first is about links. In v0.3 a link
with no leading `/` resolves from the directory of its file, where it used to
resolve from the bundle root. This bundle declares no version, and its tannin
node writes the link the old way:

<!-- transcript: setup
$ mkdir -p old/wine
$ printf -- '---\ntype: Concept\ntitle: Acidity\n---\n\nAcidity sharpens [tannin](/wine/tannin.md).\n' > old/wine/acidity.md
$ printf -- '---\ntype: Concept\ntitle: Tannin\n---\n\nTannin balances [acidity](wine/acidity.md).\n' > old/wine/tannin.md
-->
```text
$ tail -1 old/wine/tannin.md
Tannin balances [acidity](wine/acidity.md).
$ okfctl bundle info old
nodes: 2
reserved: 0
okf_version: undeclared (v0.2 fallbacks apply, §12.1)
$ okfctl lint old
OK: no lint findings
$ printf -- '---\nokf_version: "0.3"\n---\n' > old/index.md
$ okfctl lint old
orphan: wine/acidity.md has no inbound links (unreachable by traversal)
broken-link: wine/tannin.md links to wine/acidity.md which resolves to no node; did you mean /wine/acidity.md?
missing-xref: wine/tannin.md mentions "Acidity" but does not link to wine/acidity.md
3 lint finding(s)
```

With no version declared, okfctl falls back to the bundle root, as the spec
allows (§6.2, §12.1), and the link works. Declare v0.3 and the same link points
at `wine/wine/acidity.md`. There's no such file. The fallback depends on what
the bundle declares, so an old bundle goes on working until you move it on
purpose. [Migrating a bundle to OKF v0.3](guides/migrating.md) makes that move
for you.

The second breaking change is about trust. Only *current* verification events
count for a node's trust tier (§5.3), and an event is current when its `at` is
on or after `generated.at`. Here a human signed off on November 2. The content
dates from November 1, and the sign-off came after it, so it's current:

<!-- transcript: setup
$ mkdir -p trust/wine
$ printf -- '---\ntype: Concept\ntitle: Tannin\ngenerated: { by: "human:casey", at: "2026-11-01T09:00:00Z" }\nverified: { by: "human:casey", at: "2026-11-02T09:00:00Z" }\n---\n\n# Tannin\n' > trust/wine/tannin.md
-->
```text
$ okfctl eval sample --sample 1 --format md trust | grep Grade
- **Grade:** epistemic=— authority=— trust-tier=human-reviewed
```

Now rewrite the content on November 20 and don't sign off again. The November
2 event stops counting:

<!-- transcript: setup
$ printf -- '---\ntype: Concept\ntitle: Tannin\ngenerated: { by: "human:casey", at: "2026-11-20T09:00:00Z" }\nverified: { by: "human:casey", at: "2026-11-02T09:00:00Z" }\n---\n\n# Tannin\n' > trust/wine/tannin.md
-->
```text
$ okfctl eval sample --sample 1 --format md trust | grep Grade
- **Grade:** epistemic=— authority=— trust-tier=unverified
```

okfctl never refuses a bundle over its version, either. OKF says a consumer that
doesn't know a declared version SHOULD still try to read the bundle (§12):

```text
$ printf -- '---\nokf_version: "7.0"\n---\n' > old/index.md
$ okfctl bundle info old
nodes: 2
reserved: 1
okf_version: 7.0 (declared in index.md)
$ okfctl validate old
OK: bundle conforms to the OKF spec floor
```
