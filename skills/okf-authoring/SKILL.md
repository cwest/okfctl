---
name: okf-authoring
description: Use when creating or changing nodes in an OKF bundle with okfctl. Covers bundle init, node new/show/list/mv/rm/promote, type templates, and the reserved index.md and log.md files.
version: 2.0.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, authoring, knowledge-graph, nodes, templates]
    related_skills: [okf-curation-health, okf-semantic-search]
    sharing: shareable
---

# Authoring OKF nodes with okfctl

## Overview

Make a bundle with two nodes, link one to the other, then rename the target:

```text
$ okfctl bundle init mykb
Initialized OKF bundle in mykb
$ okfctl node new concepts/tannin.md --type Reference --title Tannin --bundle mykb
Created mykb/concepts/tannin.md
$ okfctl node new concepts/oak.md --type Reference --title Oak --bundle mykb
Created mykb/concepts/oak.md
$ printf '\nOak barrels add [tannin](tannin.md) to red wine.\n' >> mykb/concepts/oak.md
$ okfctl node mv concepts/tannin.md concepts/tannins.md --bundle mykb
Moved concepts/tannin.md -> concepts/tannins.md (1 inbound link(s) rewritten)
$ tail -1 mykb/concepts/oak.md
Oak barrels add [tannin](tannins.md) to red wine.
$ okfctl index check mykb
OK: index.md is current
```

Bundles are directories of Markdown files. Each concept node is one file with
YAML frontmatter (`type`, `title`) and a body. Its path is its identity.
Nodes link to each other with ordinary relative Markdown links. That's why
`node mv` had to rewrite the link in `oak.md`. `node new` and `node mv` also
rebuilt `index.md` and logged each change in `log.md` (`index check` passes
with no separate index build).

All you need for any command on this page is the `okfctl` binary. Install it with
`brew install cwest/tap/okfctl`.

## When to Use

- You want to start a bundle.
- You need to add, edit, rename, or delete some nodes.
- You want new nodes of one type to start from a team template.
- You want the reserved `index.md` and `log.md` to stay current.

Orphans and broken links are in `okf-curation-health` with the CI gates.
Similarity is in `okf-semantic-search`.

## Create a bundle

```text
$ okfctl bundle info mykb
nodes: 2
reserved: 3
okf_version: 0.3 (declared in index.md)
```

`bundle init` writes two reserved files, `index.md` and `log.md`. The root
`index.md` carries `okf_version: "0.3"` in its frontmatter (OKF v0.3 §8.1).
There's no `.okf` sidecar. `reserved: 3` above counts the root index, the
`concepts/index.md` that `node new` added, and `log.md`.

Bundles with no declared version report `okf_version: undeclared (v0.2
fallbacks apply, §12.1)`. Upgrade it with `okfctl migrate` (see
`okf-migrate-plan`).

## Add and read nodes

This is the file `node new` wrote. It stamps `created` and `modified`:

```text
$ cat mykb/concepts/tannins.md
---
type: Reference
title: Tannin
created: <TIMESTAMP>
modified: <TIMESTAMP>
---

# Tannin
$ okfctl node list --bundle mykb
concepts/oak.md                          Reference
concepts/tannins.md                      Reference
$ okfctl node show concepts/oak.md --bundle mykb
path: concepts/oak.md
type: Reference

# Oak

Oak barrels add [tannin](tannins.md) to red wine.

```

`--type` takes any string. OKF leaves `type` values open (§4.1). `validate`
accepts `Reference`, `Concept`, and any type that you make up.

`okfctl node edit <path> --bundle <dir>` opens the node in `$EDITOR` (then
`$OKFCTL_EDITOR`, then `$VISUAL`) and validates the bundle when the editor
exits. In an agent shell that has no terminal, just edit the file in
place and run `okfctl validate` when you're done.

## Start nodes from a type template

A template is a node with `type: Type Template` that lives in the bundle
(tool config has no say in it):

```text
$ mkdir -p mykb/templates
$ printf -- '---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: [title, description, owner]\nrecommended_fields: [tags]\nbody_sections: [Trigger, Steps, Rollback, Verification]\n---\n\n# Playbook Template\n' > mykb/templates/playbook.md
$ okfctl template list mykb
Playbook	3 required field(s), 4 body section(s)	(source: templates/playbook.md)
$ okfctl node new playbooks/restart.md --type Playbook --title "Restart Service" --bundle mykb
Created mykb/playbooks/restart.md (from Playbook template)
$ cat mykb/playbooks/restart.md
---
type: Playbook
title: Restart Service
created: <TIMESTAMP>
modified: <TIMESTAMP>
description: TODO
owner: TODO
tags:
---

# Restart Service

## Trigger

## Steps

## Rollback

## Verification
```

With a template for the `--type` you asked for, `node new` stubs each
listed field with `TODO`, leaves the recommended ones empty, and writes a
`##` heading for each body section. `okfctl template show Playbook mykb`
prints one template's fields. Checking existing nodes against a template is
`validate --templates` (see `okf-curation-health`).

## Move and remove nodes

Preview first. `--dry-run` lists all the links it would rewrite and writes
nothing:

```text
$ okfctl node mv concepts/tannins.md concepts/tannin.md --bundle mykb --dry-run
move concepts/tannins.md -> concepts/tannin.md
  rewrite concepts/oak.md: tannins.md -> tannin.md
$ okfctl node rm concepts/oak.md --bundle mykb --dry-run
remove concepts/oak.md
  orphaned: concepts/tannins.md
```

`node rm --dry-run` shows the nodes that would be left with no inbound
links. Raw `mv` or `git mv` rewrites nothing (links to the old path just
break).

## Promote directory-as-concept indexes

Corpora from Obsidian, Hugo, Jekyll, and most wikis put a concept's
frontmatter in each directory's `index.md`. OKF reserves `index.md` for
navigation and lets only the bundle root's carry frontmatter (§8, §8.1).
`validate` fails each of the others. `node promote` moves each one to a sibling
concept and rewrites the links:

<!-- transcript: exit 1 okfctl validate ./corp -->
```text
$ okfctl bundle init corp
Initialized OKF bundle in corp
$ mkdir -p corp/gke
$ printf -- '---\ntype: Concept\ntitle: GKE\n---\n\n# GKE\n' > corp/gke/index.md
$ printf -- '---\ntype: Concept\ntitle: Overview\n---\n\n# Overview\n\nSee [GKE](gke/).\n' > corp/overview.md
$ okfctl validate ./corp
FAIL gke/index.md: index files contain no frontmatter (§8); only the bundle-root index may carry frontmatter (§8.1)
okfctl: 1 conformance finding(s)
$ okfctl node promote corp --dry-run
would promote gke/index.md -> gke/gke.md
  rewrite overview.md: gke/ -> gke/gke.md
1 index(es) would be promoted (dry run; nothing written)
$ okfctl node promote corp
promoted gke/index.md -> gke/gke.md (1 inbound link(s) rewritten)
1 index(es) promoted
$ okfctl validate corp
OK: bundle conforms to the OKF spec floor
$ okfctl lint --strict corp
OK: no lint findings
```

Promote leaves the body and `created` as they were. It rewrites the `gke/` and
`gke/index.md` spellings of inbound links, writes a fresh `index.md` with no
frontmatter, appends to `log.md`, and doesn't touch the root index.
`--name overview` names each promoted file `overview.md` (the default is the
directory name). Finish with `lint --strict` and check that there are no
`broken-link` findings.

## Keep index.md and log.md current

Files added by hand, outside okfctl, make the index stale:

<!-- transcript: exit 1 okfctl index check ./mykb -->
```text
$ printf -- '---\ntype: Reference\ntitle: Ash\n---\n\n# Ash\n' > mykb/concepts/ash.md
$ okfctl index check ./mykb
index.md is out of date; run `okfctl index build` to regenerate
okfctl: index.md is out of date
$ okfctl index build mykb
Wrote 4 index.md file(s) under mykb
$ okfctl index check mykb
OK: index.md is current
$ okfctl log append mykb --message "added ash by hand"
Appended log entry
$ okfctl log show mykb
# Change Log

## <DATE>
* added ash by hand
* created playbooks/restart.md
* moved concepts/tannin.md -> concepts/tannins.md
* created concepts/oak.md
* created concepts/tannin.md
```

`index build` writes one `index.md` per directory that holds concepts (§8).
`index check` exits 1 when any is stale (CI can gate on it). `log.md`
groups entries under `## YYYY-MM-DD` headings, newest first (§9). The node
commands log their own changes. Use `log append` for edits you made by hand.

## Common Pitfalls

1. Don't hand-edit `index.md`. okfctl keeps a file that holds anything it
   doesn't write (an intro, notes, your own heading): `index build` names it
   and exits 1, `index check` fails on it, and the node verbs warn on every
   run. Put content in concept nodes. `index build --overwrite-hand-written`
   takes the file back.
2. `index.md` and `log.md` are reserved files. Nothing links to `log.md`
   (lint never calls it an orphan).
3. Plain `mv` or `git mv` leaves inbound links pointing at the old path.
   Use `okfctl node mv`.
4. `node rm --dry-run` is the one place you see which nodes the removal
   orphans. Don't skip it.
5. Files added outside okfctl stay out of the index until `index build`
   rebuilds it. Lint reports them as orphans until then. Run `index check` in
   CI.

## Verification Checklist

- [ ] `okfctl bundle info <dir>` shows the node and reserved counts you wanted
- [ ] New nodes appear in `okfctl node list --bundle <dir>`
- [ ] Templated nodes printed `(from <Type> template)` when created
- [ ] Each `node mv`, `node rm`, and `node promote` was run with `--dry-run`
      first
- [ ] `okfctl index check <dir>` exits 0
- [ ] Hand edits are recorded with `okfctl log append`
