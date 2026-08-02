---
name: okf-curation-health
description: Use when keeping an OKF bundle healthy with okfctl — running validate for spec-floor and type-template conformance, and lint for curation findings (orphans, missing cross-references, broken internal links, coverage gaps, type hygiene), including as a CI or pre-commit gate.
version: 1.0.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, lint, validate, curation, ci, knowledge-graph]
    related_skills: [okf-authoring, okf-semantic-search]
    sharing: shareable
---

# Keeping an OKF corpus healthy with okfctl

## Overview

Two okfctl commands keep a bundle healthy, and they answer different questions:

- **`validate`** — *is this a conformant OKF bundle?* Spec-floor conformance
  (hard failures), plus an opt-in type-template overlay for team conventions.
- **`lint`** — *is this a well-curated corpus?* Judgment-worthy findings
  (orphans, missing cross-references, broken internal links, coverage gaps, type
  hygiene). Never a format failure; `lint` never mutates the bundle.

**`lint` and structural `validate` require NO model and NO index.** They work on
a fresh clone with zero setup — pure graph and text analysis. (The one exception,
`lint --semantic`, is covered by `okf-semantic-search`.)

## When to Use

- You want to know whether a bundle conforms to OKF before publishing/merging.
- You want curation guidance: what to link, what to write next, what to clean up.
- You want a nonzero-exit gate for CI or a pre-commit hook.

Don't use for: authoring/scaffolding nodes — see `okf-authoring`; similarity
checks — see `okf-semantic-search`.

## validate — conformance

Spec-floor validation checks that every node carries a non-empty `type` (OKF §7).
Unknown type *values* are allowed — OKF is anti-taxonomy at the floor.

```sh
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
```

A floor violation always fails (exit 1), regardless of any flag:

```sh
$ okfctl validate mykb        # a node with an empty `type:`
FAIL concepts/bad.md: missing or empty required field: type
okfctl: 1 conformance finding(s)
$ echo $?
1
```

### Type-template overlay (`--templates`)

`--templates` additionally checks each node against its governing type template
(if the bundle declares one — see `okf-authoring`). Template drift (a missing
required field or body section) is reported as a **warning**, advisory by default
(exit 0). Add `--strict` to make drift fail:

```sh
$ okfctl validate mykb --templates
warning playbooks/rollback.md: missing required field: owner (template Playbook)
warning playbooks/rollback.md: missing body section: Rollback (template Playbook)
warning playbooks/rollback.md: missing body section: Verification (template Playbook)
3 template drift warning(s)
$ echo $?
0

$ okfctl validate mykb --templates --strict
warning playbooks/rollback.md: missing required field: owner (template Playbook)
...
okfctl: 3 template drift warning(s)
$ echo $?
1
```

The overlay never leaks into the floor: unknown type values still pass, and
spec-floor violations still fail even without `--templates`.

## lint — curation findings

```sh
$ okfctl lint mykb
OK: no lint findings          # when clean
```

With findings, `lint` prints one line per finding and a count. By default it is
**advisory and exits 0 even with findings** — so it never blocks by accident:

```sh
$ okfctl lint mykb
coverage-gap: "Malolactic Fermentation" is referenced by 3 nodes but has no node of its own
type-hygiene: near-duplicate type values likely refer to one type: Concept, Concepts
orphan: concepts/aging.md has no inbound links (unreachable by traversal)
missing-xref: concepts/wine.md mentions "Tannin" but does not link to concepts/tannin.md
9 lint finding(s)
$ echo $?
0
```

### What each finding means and what to do

| finding | means | action |
|---|---|---|
| `orphan` | a concept node no node (or `index.md`) links to — unreachable by traversal | link it from a relevant node, or from `index.md` via `index build` |
| `missing-xref` | a node's prose names another node's title as a whole word but doesn't link it | add the `[Title](path.md)` link, or reword if the mention is incidental |
| `broken-link` | a node links to a `.md` target that resolves to no node, **and a node with that basename exists elsewhere** — a moved or mistyped path (a defect, not an unwritten concept) | fix the path to the resolved candidate the finding names |
| `coverage-gap` | a **known concept term** (declared as a title/alias) is referenced by ≥ threshold distinct nodes but has no node of its own | author the missing node — it's a real to-do, prioritized by mention count |
| `type-hygiene` | two `type` values fold to the same canonical form (case / trailing-`s` plural), e.g. `Concept` vs `Concepts` | pick one spelling and normalize the drifting nodes |

Two accuracy notes that keep you from chasing ghosts:

- **`orphan` treats `index.md` as a linker.** A node listed in a freshly built
  `index.md` is *not* an orphan. If everything shows as orphaned, run `okfctl
  index build` first (see `okf-authoring`).
- **`coverage-gap` only fires for *known* concepts** — a term some node declares
  as a `title` or `aliases:` entry. Arbitrary capitalized prose ("Google Cloud",
  a sentence-initial "The") is deliberately *not* a candidate; this is what makes
  the check act on real gaps rather than noise. If you expect a gap and don't get
  one, the term probably isn't declared as an alias anywhere yet.

### `broken-link` vs a dangling link — defect, not gap

A `.md` link that resolves to no node is one of two very different things, and
`lint` reports only the dangerous one:

- **The concept doesn't exist yet** — a referenced-but-unwritten node. That's a
  *coverage gap*: correctly advisory, and it stays out of `lint`. `analyze`
  surfaces it under `coverage_gaps.dangling_links` where it belongs (a research
  to-do, not a defect).
- **The concept exists and the path is wrong** — a typo, a moved file, a bad
  find-and-replace. That's a *defect*, and it's dangerous because it's silent:
  everything still validates, nothing is orphaned, yet a link points nowhere.
  `broken-link` is the gate for exactly this case.

The discriminator is basename: `lint` reports a dangling target as `broken-link`
**only when a node with the same basename lives elsewhere in the bundle** — the
signature of a moved or mistyped path. This is the check to lean on before a bulk
migration (`node mv`, a directory reorg, a wikilink→Markdown rewrite): those
operations produce exactly the defect signature, and `lint --strict` will catch
them in CI. `analyze`'s advisory dangling-link reporting is unchanged — this
check only *adds* a gate.

### Tuning the coverage-gap threshold

Default threshold is 3 distinct nodes. Lower it to surface gaps earlier, raise it
to only flag heavily-referenced missing concepts:

```sh
$ okfctl lint mykb --coverage-threshold 2
```

## Gate CI / pre-commit with --strict

`lint` is advisory by default; `--strict` makes *any* finding exit non-zero so it
can gate a pipeline:

```sh
$ okfctl lint mykb --strict
orphan: concepts/mouthfeel.md has no inbound links (unreachable by traversal)
1 lint finding(s)
okfctl: 1 lint finding(s)
$ echo $?
1
```

A pre-commit hook, for example:

```yaml
- repo: local
  hooks:
    - id: okf-lint
      name: OKF curation lint
      entry: okfctl lint . --strict
      language: system
      pass_filenames: false
```

Because structural `lint` needs no model and no index, this gate runs on a fresh
clone with nothing installed but the `okfctl` binary. For a CI conformance gate,
pair it with `okfctl validate . --templates --strict` and `okfctl index check .`.

## Common Pitfalls

1. **Everything shows as `orphan`.** You haven't run `okfctl index build`.
   `index.md` confers reachability; without a current index, nodes linked only
   from the (stale/empty) table of contents read as orphaned.

2. **Expecting `coverage-gap` on any repeated word.** It only reports *known*
   concept terms (declared as a title or `aliases:`). Lowercase prose and
   undeclared proper nouns are intentionally excluded — the check targets real
   authoring to-dos, not vocabulary frequency.

3. **Assuming `lint` blocks CI by default.** It exits 0 even with findings unless
   you pass `--strict`. A pipeline that forgot `--strict` will go green on a
   corpus full of findings.

4. **Using `lint` to catch format errors.** `lint` is curation guidance, never
   conformance. Spec-floor violations (missing/empty `type`) surface only in
   `validate`, which fails on them regardless of flags.

5. **Confusing `--templates` warnings with floor failures.** Template drift is
   advisory (exit 0) unless `--strict`; a genuine spec-floor violation fails
   unconditionally. They are separate exit paths.

## Verification Checklist

- [ ] `okfctl validate <dir>` exits 0 (spec floor clean)
- [ ] If the bundle declares type templates, `okfctl validate <dir> --templates`
      reviewed; `--strict` used where drift must block
- [ ] `okfctl lint <dir>` reviewed; each finding class understood and actioned
- [ ] `okfctl index build` run before trusting `orphan` findings
- [ ] CI/pre-commit gate uses `--strict` so findings actually block
