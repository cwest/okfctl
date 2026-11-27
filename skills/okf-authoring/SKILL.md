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

Make a bundle, add two nodes, link one to the other, and rename the target:

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

A bundle is a directory of Markdown files, and each concept node is one file
with YAML frontmatter (`type`, `title`) and a body. Its path is its identity. Nodes link
each other with ordinary relative Markdown links. That's why `node mv` had to
rewrite the link in `oak.md`. `node new` and `node mv` also rebuilt `index.md`
and logged each change in `log.md`, and `index check` passes with no
`index build` step.

You need only the `okfctl` binary for any command on this page. Install it with
`brew install cwest/tap/okfctl`.

## When to Use

- You're starting a bundle, or adding, editing, renaming, or deleting nodes.
- You want new nodes of one type to start from a team template.
- You want the reserved `index.md` and `log.md` to stay current.

For orphans, broken links, and CI gates, use `okf-curation-health`. For
similarity checks, use `okf-semantic-search`.

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

A bundle with no declared version reports `okf_version: undeclared (v0.2
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
accepts `Reference`, `Concept`, and anything you make up.

`okfctl node edit <path> --bundle <dir>` opens the node in `$EDITOR` (then
`$OKFCTL_EDITOR`, then `$VISUAL`) and validates the bundle when the editor
exits. Don't call it from an agent shell with no terminal. Edit the file in
place and run `okfctl validate` after.

## Start nodes from a type template

A type template is an ordinary node with `type: Type Template`. It lives in the
bundle (tool config has no say in it):

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

If a template matches `--type`, `node new` stubs each listed field with
`TODO`, leaves each recommended field empty, and writes one `##` heading per
body section. `okfctl template show Playbook mykb` prints one template's
fields. To check existing nodes against a template, use `validate --templates`
(see `okf-curation-health`).

## Move and remove nodes

Preview first. `--dry-run` lists each link it would rewrite and writes
nothing:

```text
$ okfctl node mv concepts/tannins.md concepts/tannin.md --bundle mykb --dry-run
move concepts/tannins.md -> concepts/tannin.md
  rewrite concepts/oak.md: tannins.md -> tannin.md
$ okfctl node rm concepts/oak.md --bundle mykb --dry-run
remove concepts/oak.md
  orphaned: concepts/tannins.md
```

`node rm --dry-run` names the nodes the removal would leave with no inbound
links. Use a raw `mv` or `git mv` and nothing gets rewritten (links to the old
path just break).

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
directory name). Finish with `lint --strict` and look for zero `broken-link`
findings.

## Keep index.md and log.md current

A file added by hand, outside okfctl, makes the index stale:

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
`index check` exits 1 when any is stale, and CI can gate on it. `log.md`
groups entries under `## YYYY-MM-DD` headings, newest first (§9). The node
commands log their own changes. Use `log append` for edits you made by hand.

## Common Pitfalls

1. **Hand-editing `index.md`.** The next `index build` overwrites it. Put
   content in concept nodes.
2. **Treating `index.md` or `log.md` as a concept.** They're reserved. Nothing
   links to `log.md`, and lint never calls it an orphan.
3. **Renaming a node with `mv` or `git mv`.** Inbound links point at the old
   path and break. Use `okfctl node mv`.
4. **Skipping `--dry-run` on `node rm`.** The dry run is the one place you see
   which nodes the removal orphans.
5. **Adding files outside okfctl with no `index build`.** The new node is
   missing from the index, and lint reports it as an orphan. Run `index check`
   in CI.

## Verification Checklist

- [ ] `okfctl bundle info <dir>` shows the node and reserved counts you wanted
- [ ] New nodes appear in `okfctl node list --bundle <dir>`
- [ ] Templated nodes printed `(from <Type> template)` when created
- [ ] Each `node mv`, `node rm`, and `node promote` was run with `--dry-run`
      first
- [ ] `okfctl index check <dir>` exits 0
- [ ] Hand edits are recorded with `okfctl log append`
