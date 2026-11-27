---
name: okf-curation-health
description: Use when checking an OKF bundle with okfctl. validate checks spec-floor conformance and opt-in type templates; lint reports curation findings (orphans, missing links, broken links and path fields, coverage gaps, type and tag drift, OKF v0.3 §5 frontmatter). Both gate CI.
version: 2.0.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, lint, validate, curation, ci, knowledge-graph]
    related_skills: [okf-authoring, okf-semantic-search]
    sharing: shareable
---

# Checking an OKF bundle with okfctl

## Overview

Build a bundle in which one node names another in its prose but never links to
it, then run validate and lint on it:

<!-- transcript: exit 1 okfctl lint --strict mykb -->
```text
$ okfctl bundle init mykb
Initialized OKF bundle in mykb
$ okfctl node new wine/tannin.md --type Concept --title Tannin --bundle mykb
Created mykb/wine/tannin.md
$ okfctl node new wine/oak.md --type Concept --title Oak --bundle mykb
Created mykb/wine/oak.md
$ echo 'Oak barrels add tannin to red wine.' >> mykb/wine/oak.md
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
$ okfctl lint mykb
missing-xref: wine/oak.md mentions "Tannin" but does not link to wine/tannin.md
1 lint finding(s)
$ okfctl lint --strict mykb
missing-xref: wine/oak.md mentions "Tannin" but does not link to wine/tannin.md
1 lint finding(s)
okfctl: 1 lint finding(s)
```

`validate` answers one question (does the bundle meet the OKF spec floor?) and
this one does (exit 0). `lint` asks if all the nodes are well curated. It
found the unlinked mention and still exited 0. Lint is advisory by default.
Pass `--strict` to turn any finding into exit 1. Neither command edits a file.

All you need to run either command is the `okfctl` binary (they read no index
and load no model). The one exception is `lint --semantic`, covered in
`okf-semantic-search`.

## When to Use

- You need to know if a bundle conforms to OKF ahead of a merge or publish.
- You want a list of what to link, what to write next, what to clean up.
- You want a CI step or pre-commit hook that fails when there are problems.

To create or move nodes, use `okf-authoring`. For similarity checks, use
`okf-semantic-search`.

## validate: the spec floor

A floor violation fails with exit 1 on each run, whatever flags you pass.
Frontmatter that doesn't parse is one:

<!-- transcript: exit 1 okfctl validate ./mykb -->
```text
$ printf -- '---\ntitle: [unclosed\n---\n\n# Bad\n' > mykb/wine/bad.md
$ okfctl validate ./mykb
FAIL wine/bad.md: unparseable frontmatter
okfctl: 1 conformance finding(s)
$ rm mykb/wine/bad.md
```

A `.md` file that lacks frontmatter (a `README.md`, say) or that has no `type`
isn't a concept under OKF v0.3 §3.2. All commands skip it, and `validate`
prints a warning but still exits 0:

```text
$ printf '# Notes\n\nNot a concept.\n' > mykb/wine/README.md
$ okfctl validate mykb
warning wine/README.md: not a concept (no frontmatter); skipped, does not affect conformance (OKF v0.3 §3.2)
OK: bundle conforms to the OKF spec floor
$ rm mykb/wine/README.md
```

Files and directories whose names start with `.` aren't part of the bundle. An
unknown `type` value passes too, as OKF leaves `type` open (§4.1).

### Type templates: --templates

`--templates` also checks each node against the type template that governs its
`type` (templates are covered in `okf-authoring`). Drift prints a warning and
exits 0. Add `--strict` and drift exits 1:

<!-- transcript: exit 1 okfctl validate mykb --templates --strict -->
```text
$ mkdir -p mykb/templates
$ printf -- '---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: [title, description, owner]\nbody_sections: [Trigger, Steps, Rollback]\n---\n\n# Playbook Template\n' > mykb/templates/playbook.md
$ printf -- '---\ntype: Playbook\ntitle: Rollback\ndescription: undo a deploy\n---\n\n# Rollback\n\n## Trigger\n\n## Rollback\n' > mykb/wine/rollback.md
$ okfctl validate mykb --templates
warning wine/rollback.md: missing required field: owner (template Playbook)
warning wine/rollback.md: missing body section: Steps (template Playbook)
2 drift warning(s)
$ okfctl validate mykb --templates --strict
warning wine/rollback.md: missing required field: owner (template Playbook)
warning wine/rollback.md: missing body section: Steps (template Playbook)
2 drift warning(s)
okfctl: 2 drift warning(s)
$ rm mykb/wine/rollback.md mykb/templates/playbook.md
```

Templates never change the floor. Leave off `--templates` and a missing `owner`
is fine and an unparseable node still fails.

## lint: curation findings

Here the bundle gains a concept term with no node, two spellings of one type,
and a link to a path that moved. The two new files were written by hand. The
index doesn't list them until you rebuild it:

```text
$ printf -- '---\ntype: Concepts\ntitle: Acidity\n---\n\n# Acidity\n\nMalolactic Fermentation lowers acidity. See [tannin](reference/tannin.md).\n' > mykb/wine/acidity.md
$ printf -- '---\ntype: Concept\ntitle: Barrel Aging\naliases: [Malolactic Fermentation]\n---\n\n# Barrel Aging\n' > mykb/wine/aging.md
$ echo 'Malolactic Fermentation softens it.' >> mykb/wine/oak.md
$ echo 'Grip drops after Malolactic Fermentation.' >> mykb/wine/tannin.md
$ okfctl lint mykb
coverage-gap: "Malolactic Fermentation" is referenced by 4 nodes but has no node of its own
type-hygiene: near-duplicate type values likely refer to one type: Concept, Concepts
broken-link: wine/acidity.md links to reference/tannin.md which resolves to no node; did you mean /wine/tannin.md?
missing-xref: wine/acidity.md mentions "Tannin" but does not link to wine/tannin.md
orphan: wine/acidity.md has no inbound links (unreachable by traversal)
orphan: wine/aging.md has no inbound links (unreachable by traversal)
missing-xref: wine/oak.md mentions "Tannin" but does not link to wine/tannin.md
7 lint finding(s)
$ okfctl index build mykb
Wrote 2 index.md file(s) under mykb
$ okfctl lint mykb
coverage-gap: "Malolactic Fermentation" is referenced by 4 nodes but has no node of its own
type-hygiene: near-duplicate type values likely refer to one type: Concept, Concepts
broken-link: wine/acidity.md links to reference/tannin.md which resolves to no node; did you mean /wine/tannin.md?
missing-xref: wine/acidity.md mentions "Tannin" but does not link to wine/tannin.md
missing-xref: wine/oak.md mentions "Tannin" but does not link to wine/tannin.md
5 lint finding(s)
```

The orphans went away after `index build` listed them, because `index.md`
counts as a linker. Malolactic Fermentation is a known term only because
`aging.md` declares it as an alias, and an alias doesn't give a term its own
node (lint reports that missing node as a coverage finding).
`--coverage-threshold N` sets how many different nodes have to mention a term
for it to be reported (the default is 3).

### Each finding and its fix

| finding | means | fix |
|---|---|---|
| `orphan` | this concept has zero inbound links, counting links from `index.md` | link it from a related node, or run `okfctl index build` |
| `missing-xref` | a node's prose names another node's title as a whole word without linking it | add `[Title](path.md)`, or reword an incidental mention |
| `broken-link` | a body link or §6.2 path field (`sources[].resource`, `superseded_by`, …) resolves to no node, and a node with the same basename exists elsewhere | change the path to the one the finding suggests |
| `coverage-gap` | a known term (some node's `title` or `aliases` entry) is mentioned by at least the threshold of nodes, and no node's title is the term or starts with it | write that node |
| `type-hygiene` | two `type` values fold to one form by case or a trailing `s` | pick one spelling |
| `tag-hygiene` | two or more `tags` spellings fold to one form by case, separators (`run-book`, `run_book`, `runbook`), or a plural of a tag already in use; each is listed with its node count | review it, since the fold is a guess, then use the leading spelling |
| `spec-version` | a concept has its own `okf_spec_version` key; the version belongs once, as `okf_version` on the root `index.md` (§12) | delete the key |
| `source-missing-resource` | a `sources` entry has no `resource`; §5.1 requires one, so the entry is ignored (§11.2 rule 3) | add the resource, or delete the entry |
| `timestamp-form` | a §5 timestamp (`generated.at`, `verified[].at`, `stale_after`, `sources[].last_modified`, `usage_window.from`/`to`) isn't an RFC 3339 date-time with an offset or a plain date; an unparseable `stale_after` reads as stale | add the offset (`2026-06-20T10:00:00Z`) or use a date |
| `verified-event` | a `verified` event lacks `by` or `at`, so consumers ignore it (§5.2) | add the missing field, or delete the event |
| `superseded-by` | `superseded_by` points at no concept, or sits on a concept whose `status` isn't `deprecated` (§5.4) | fix the target, or set `status: deprecated` |
| `status-lifecycle` | `status` holds a value §5.4 doesn't define, so the concept isn't treated as stable | use `draft`, `stable`, or `deprecated`; move an epistemic grade to `epistemic` |

None of these will make `validate` fail, as OKF v0.3 §11 leaves such a bundle
conformant. Each of them fails `lint --strict`.

The §5 checks look like this on one node:

```text
$ printf -- '---\ntype: Concept\ntitle: Body\nstatus: verified\nsources:\n  - title: notes\nverified:\n  - by: human:casey\n---\n\n# Body\n\nSee [aging](aging.md).\n' > mykb/wine/body.md
$ okfctl lint mykb | grep body.md
orphan: wine/body.md has no inbound links (unreachable by traversal)
source-missing-resource: wine/body.md sources entry #1 has no resource, so it is ignored (§5.1 requires resource; §11.2 rule 3)
status-lifecycle: wine/body.md has status "verified", which §5.4 does not define (draft|stable|deprecated); per §5.4 the concept is not stable and filters to current knowledge exclude it. If this is an epistemic grade, move it to `epistemic`
verified-event: wine/body.md verified[0] = { by: human:casey } has no at; §5.2 requires both by and at, so consumers ignore this event and it does not count toward the trust tier
```

### broken-link only fires when the target exists elsewhere

Links to a path that has no node mean one of two things. If no node by that
name exists anywhere, the link points at a concept no one has written; lint
says nothing about it, but `okfctl analyze` lists it under
`coverage_gaps.dangling_links`. If a node with the same basename lives at
another path, the link is a typo or left over from a move. Lint reports that
case as `broken-link`. Run `lint --strict` after a bulk move, after you
reorganize directories, and after a wikilink rewrite. Those are what produce
the second case.

## Gate CI and pre-commit

Add `--strict` to make the findings fail the run, as in this pre-commit hook:

```yaml
- repo: local
  hooks:
    - id: okf-lint
      name: OKF curation lint
      entry: okfctl lint . --strict
      language: system
      pass_filenames: false
```

For the complete CI gate, add `okfctl validate . --templates --strict` and
`okfctl index check .` to it. They all run on a fresh clone with nothing but
the `okfctl` binary installed. For machine output, run `okfctl lint --json`
to get the findings as a JSON array sorted by path.

## Common Pitfalls

If all the nodes in a bundle come back as `orphan`, you probably skipped the
index build. Run `okfctl index build` and lint again (`index.md` counts as a
linker).

`coverage-gap` ignores words that you just repeat. It only looks at the terms
that some node declares as a `title` or in `aliases`.

Bare `lint` exits 0 even with findings. CI won't fail until you pass
`--strict`. Unparseable frontmatter and malformed reserved files show up in
`validate` output and never in `lint`. Drift from `--templates` is a warning
(exit 0) until you add `--strict`. A floor violation exits 1 no matter what.

## Verification Checklist

- [ ] `okfctl validate <dir>` exits 0
- [ ] If the bundle has type templates, you read `okfctl validate <dir> --templates`
      and added `--strict` where drift should block
- [ ] You ran `okfctl index build` and then trusted the `orphan` findings
- [ ] You fixed each `okfctl lint <dir>` finding or left it on purpose
- [ ] The CI or pre-commit gate passes `--strict`
