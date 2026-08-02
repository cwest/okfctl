---
name: okf-authoring
description: Use when creating or maintaining nodes in an OKF bundle with okfctl — scaffolding concept nodes, applying type templates, running the bundle lifecycle, and keeping the reserved index.md and log.md files current.
version: 1.0.0
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

`okfctl` manages **OKF bundles** — directories of Markdown "nodes" that form a
linked knowledge graph. Each concept node is a Markdown file with YAML front
matter (`type`, `title`, optional `aliases`) and a prose body; links between
nodes are ordinary relative Markdown links, and the *path is the node's
identity*. This skill is the author's loop: create a bundle, add and edit nodes,
move or remove them safely, and maintain the two reserved files (`index.md`,
`log.md`).

Everything here needs only the `okfctl` binary — no model, no network, no index.

## When to Use

- You are starting a new bundle or adding/editing/renaming nodes in one.
- You want new nodes to conform to a team convention (a **type template**).
- You need to move or delete a node without breaking inbound links.
- You are keeping `index.md` (the generated table of contents) current.

Don't use for: curation-health findings (orphans, coverage gaps) — see
`okf-curation-health`; semantic similarity checks — see `okf-semantic-search`.

## Build the binary

```sh
git clone <the okfctl repo>
cd okfctl
CGO_ENABLED=0 go build -o okfctl .
# put it on PATH, e.g. `sudo mv okfctl /usr/local/bin/`, or invoke as ./okfctl
```

## 1. Create a bundle

```sh
$ okfctl bundle init mykb
Initialized OKF bundle in mykb
```

A fresh bundle contains exactly three files:

```
mykb/.okf         # bundle marker: `okf_version: 0.1`
mykb/index.md     # RESERVED — generated table of contents (type: Index)
mykb/log.md       # RESERVED — change history
```

`bundle info` summarizes any bundle:

```sh
$ okfctl bundle info mykb
nodes: 0
reserved: 2
okf_version: 0.1
```

## 2. Author concept nodes

```sh
$ okfctl node new concepts/tannin.md --type Reference --title "Tannin" --bundle mykb
Created mykb/concepts/tannin.md
```

The file it writes:

```markdown
---
type: Reference
title: Tannin
---

# Tannin
```

`--type` accepts any string — OKF is anti-taxonomy at the spec floor, so
`Reference`, `Concept`, or any other value all pass `validate`. Inspect and list:

```sh
$ okfctl node show concepts/tannin.md --bundle mykb
path: concepts/tannin.md
type: Reference

# Tannin

$ okfctl node list --bundle mykb
concepts/mouthfeel.md                    Concept
concepts/tannin.md                       Reference
```

Link nodes with ordinary relative Markdown links in the body — `[Tannin](tannin.md)`
from a sibling file. Links are what make the graph traversable (and what keeps a
node off the orphan list; see `okf-curation-health`).

Edit a node in `$EDITOR` (falls back to `$OKFCTL_EDITOR` / `$VISUAL`); okfctl
re-validates the bundle when the editor returns:

```sh
$ okfctl node edit concepts/tannin.md --bundle mykb
```

## 3. Type templates (optional team conventions)

A **type template** is an ordinary OKF node with `type: Type Template`. Nothing
about a template lives in tool config — it is data in the bundle. It declares a
`target_type` and the fields/sections nodes of that type should carry:

```markdown
---
type: Type Template
target_type: Playbook
required_fields: [title, description, owner]
recommended_fields: [tags]
body_sections: [Trigger, Steps, Rollback, Verification]
---

# Playbook Template
```

Inspect the templates a bundle declares:

```sh
$ okfctl template list mykb
Playbook	3 required field(s), 4 body section(s)

$ okfctl template show Playbook mykb
target_type: Playbook
source: templates/playbook.md
required_fields: title, description, owner
recommended_fields: tags
body_sections: Trigger, Steps, Rollback, Verification
```

When a template governs a `--type`, `node new` **scaffolds from it**: required
fields are stubbed with `TODO`, recommended fields are stubbed empty, and each
declared body section is laid down as an empty `##` heading, so the node starts
free of template drift:

```sh
$ okfctl node new playbooks/restart.md --type Playbook --title "Restart Service" --bundle mykb
Created mykb/playbooks/restart.md (from Playbook template)
```

```markdown
---
type: Playbook
title: Restart Service
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

(Checking a node *against* its template is a curation concern — `validate
--templates`; see `okf-curation-health`.)

## 4. Move and remove nodes safely

The node's path is its identity, so a move is a graph operation: `node mv`
rewrites every inbound link and preserves each author's relative link form.
Preview with `--dry-run` first:

```sh
$ okfctl node mv concepts/oak.md concepts/oak-aging.md --bundle mykb --dry-run
move concepts/oak.md -> concepts/oak-aging.md

$ okfctl node mv concepts/oak.md concepts/oak-aging.md --bundle mykb
Moved concepts/oak.md -> concepts/oak-aging.md (0 inbound link(s) rewritten)
```

`node rm` reports which nodes would become orphaned as a result. Always
`--dry-run` before a real removal:

```sh
$ okfctl node rm concepts/mouthfeel.md --bundle mykb --dry-run
remove concepts/mouthfeel.md
  orphaned: concepts/tannin.md
```

### Migrating a directory-as-concept corpus with `node promote`

Corpora imported from tools where a **directory is a concept** — Obsidian folder
notes, Hugo `_index.md`, Jekyll collections, most wikis — put the concept's
frontmatter and body in each directory's `index.md`. OKF models `index.md`
differently: it is a generated navigation surface (§6), and only the bundle-root
index may carry frontmatter (the §11 `okf_version` marker). So every non-root
`index.md` with frontmatter fails `validate`, and on first contact that can be
hundreds of identical findings.

`node promote` is the one-command remediation. For every non-root `index.md`
that carries frontmatter it moves the file to a sibling concept
(`foo/index.md` → `foo/foo.md`), preserves the body verbatim and keeps `created`
immutable, rewrites inbound links (both the `foo/` and `foo/index.md`
spellings), regenerates the real `index.md` with no frontmatter, and appends to
`log.md`. The bundle-root index is left alone. **Always `--dry-run` first** — it
lists every move and link rewrite and writes nothing:

```sh
$ okfctl node promote mykb --dry-run
would promote gke-pm-map/index.md -> gke-pm-map/gke-pm-map.md
  rewrite overview.md: gke-pm-map/ -> gke-pm-map/gke-pm-map.md
2 index(es) would be promoted (dry run; nothing written)

$ okfctl node promote mykb --name overview   # one basename convention for all
```

Verify the migration broke no links: `okfctl lint mykb --strict` must report
**zero** `broken-link` findings afterward (see `okf-curation-health`).

## 5. The reserved files: index.md and log.md

`index.md` and `log.md` are **reserved** — they are structurally different from
concept nodes and the tool owns their shape. `index.md` is a generated table of
contents; regenerate it whenever nodes change, and verify it is current in CI:

```sh
$ okfctl index build mykb
Wrote mykb/index.md

$ okfctl index check mykb          # exit 0 when current
OK: index.md is current

$ okfctl node new concepts/oak.md --type Concept --title Oak --bundle mykb
$ okfctl index check mykb          # exit 1 when stale
index.md is out of date; run `okfctl index build` to regenerate
```

A rebuilt `index.md` links every node, which is also how a bundle's nodes stop
reading as orphans — `index.md` confers reachability (see `okf-curation-health`).

Record changes in `log.md`:

```sh
$ okfctl log append mykb --message "added oak node"
Appended log entry

$ okfctl log show mykb
# Change Log

- 2026-07-26 — added oak node
```

## Common Pitfalls

1. **Treating index.md / log.md as concept nodes.** They are reserved: `index.md`
   is generated (don't hand-edit — `index build` overwrites it) and `log.md`
   legitimately has *no inbound links*. Curation tools know this and never flag
   the reserved files as orphans; only concept nodes participate in graph checks.

2. **Hand-editing index.md.** Any manual edit is lost on the next `index build`
   and reads as "out of date" to `index check` until rebuilt. Author content in
   concept nodes; let `index build` own `index.md`.

3. **Expecting `node new` to require a known type.** It doesn't — any `--type`
   string is accepted at the spec floor. Type *conventions* are opt-in via type
   templates, not enforced by the taxonomy.

4. **Moving a node with `mv`/`git mv` instead of `okfctl node mv`.** A raw move
   leaves every inbound link pointing at the old path. `okfctl node mv` rewrites
   them; a raw move silently creates broken links (and orphans).

5. **Deleting a node without `--dry-run`.** `node rm --dry-run` tells you which
   nodes the removal orphans *before* you commit to it — cheap insurance against
   silently cutting a node loose from the graph.

6. **Forgetting to rebuild index.md after adding nodes.** New nodes are absent
   from the table of contents and read as orphans until `index build` links them.
   Run `index build` (and `index check` in CI) as part of the authoring loop.

## Verification Checklist

- [ ] `okfctl bundle info <dir>` shows the expected node / reserved counts
- [ ] New nodes appear in `okfctl node list --bundle <dir>`
- [ ] Nodes intended to follow a convention were scaffolded from a type template
      (output says "from <Type> template")
- [ ] Every `node mv` / `node rm` was previewed with `--dry-run` first
- [ ] `okfctl index check <dir>` exits 0 (index.md current) after authoring
- [ ] Changes recorded with `okfctl log append`
