---
name: okf-migrate-plan
description: Use when working an okfctl migrate-plan.json — the judgment items okfctl migrate refuses to guess when upgrading a v0.1 bundle to v0.2. Covers the deterministic/judgment split, handing a plan to any agent or working it by hand, and verifying the applied migration.
version: 1.0.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, migrate, migrate-plan, provenance, v0.2, knowledge-graph]
    related_skills: [okf-authoring, okf-curation-health]
    sharing: shareable
---

# Working an okfctl migrate-plan

## Overview

`okfctl migrate` upgrades an OKF bundle from spec **v0.1 to v0.2**. It runs in
two phases, and the split is the whole point:

- **Phase 1 (`okfctl migrate <bundle>`)** is a **pure read**. It computes every
  edit it can make *safely and deterministically*, and for everything else it
  refuses to guess — it enumerates those as **judgment items** in a plan file
  (`migrate-plan.json`) with the context you need to decide them.
- **Phase 2 (`okfctl migrate <bundle> --apply --plan migrate-plan.json`)** reads
  the plan back and applies its deterministic edits, then re-validates.

The plan file exists so that **a migration is something you can diff before you
trust it.** This skill is about the half the tool deliberately hands back: what a
judgment item is, why refusing was correct, and the three ways to work one — with
a coding agent, with a chat model, or **by hand** (a first-class path, not a
footnote — the file is plain, human-editable JSON on purpose).

`okfctl migrate` needs **no model, no network, and no credentials** on any path.
It runs on an airgapped box. If any instruction ever tells you to configure an
API key for `okfctl`, that instruction is wrong — the tool does not take one.

## When to Use

- You ran `okfctl migrate` on a v0.1 bundle and are staring at a
  `migrate-plan.json` for the first time.
- The plan reports "N item(s) need a decision before they can migrate" and you
  want to know what to do with them.
- You want to hand a plan (or a slice of it) to whatever agent you already use.
- You want to work the plan by hand with no model at all.

Don't use for: authoring or moving nodes (see `okf-authoring`); orphan/coverage/
link-health findings (see `okf-curation-health`). This skill is only the migrate
plan-and-apply loop.

## The deterministic / judgment split

`okfctl migrate` applies the §13.1 breaking renames. Two of them are sometimes
deterministic and sometimes a judgment call — the tool decides per item:

| v0.1 form | v0.2 form | Deterministic when… | Judgment when… |
|---|---|---|---|
| `timestamp:` frontmatter | `generated: {by, at}` (§7) | you pass `--generated-by <actor>` | no actor supplied → `missing-actor` |
| body `# Citations` item | `sources[].resource` (§5.1) | the item carries a follow-able resource — a bare URL, or a markdown link whose target the parser can extract (§6) | it is prose with no resource → `prose-citation` |

Two version markers are always deterministic: the `.okf` sidecar `okf_version`
and the bundle-root `index.md` marker (§8/§12) both go to `0.2` on apply.

**Why the refusals are correct, not lazy.** §5.1 makes `resource` *required* for
every `sources` entry. A "citation" that is actually an evidence finding —
*"Personal observation from a comparative tasting, 2026. Not published
anywhere."* — has no resource. Forcing it into `sources` would mean **fabricating
a `resource`**, and a made-up provenance URL is worse than an honest gap. That is
the exact failure the two-phase design exists to prevent. Same for provenance
actors (§7): the tool will not invent *who* generated a node, so a `timestamp`
with no supplied actor stays a judgment item rather than a guessed `generated.by`.

## Cold start: from a v0.1 bundle you own

Build the binary once (`CGO_ENABLED=0 go build -o okfctl .` in a checkout, then
put it on PATH). Everything below was run against a scratch two-node v0.1 bundle
(`concepts/mouthfeel.md`, `concepts/tannin.md`); the output is reproduced
verbatim. The two nodes carry, between them, every citation shape that matters: a
bare URL, a markdown link the parser resolves, a markdown link it does *not* (see
the pitfall below), and pure prose findings.

### 1. Plan (phase 1, pure read)

```sh
$ okfctl migrate mykb --plan migrate-plan.json
Wrote migrate-plan.json: 2 node(s) with deterministic edits (3 edit(s)), 5 judgment item(s).
5 item(s) need a decision before they can migrate; see the plan file.
```

Nothing was written to the bundle — only the plan file. Inspect it:

```json
{
  "target_version": "0.2",
  "nodes": [
    {
      "path": "concepts/mouthfeel.md",
      "sources": [
        { "resource": "/concepts/tannin.md" },
        { "resource": "https://en.wikipedia.org/wiki/Mouthfeel" }
      ]
    },
    {
      "path": "concepts/tannin.md",
      "sources": [
        { "resource": "https://en.wikipedia.org/wiki/Tannin" }
      ]
    }
  ],
  "judgment": [
    { "path": "concepts/mouthfeel.md", "kind": "missing-actor",  "context": "2026-02-03" },
    { "path": "concepts/mouthfeel.md", "kind": "prose-citation", "context": "- [mouthfeel](/concepts/mouthfeel.md)" },
    { "path": "concepts/mouthfeel.md", "kind": "prose-citation", "context": "- Personal observation from a comparative tasting, 2026. Not published anywhere." },
    { "path": "concepts/tannin.md",    "kind": "missing-actor",  "context": "2026-01-15" },
    { "path": "concepts/tannin.md",    "kind": "prose-citation", "context": "- Tasting notes from a 2026 barrel sampling; no public source." }
  ]
}
```

Read the file top to bottom:

- **`nodes[]`** — the deterministic edits the tool already worked out. `mouthfeel`
  got two `sources`: a bundle-relative link (`[tannin glossary](/concepts/tannin.md)`,
  whose target the parser extracted and kept relative per §6) and a bare
  Wikipedia URL. `tannin` got its one bare URL. You do not need to touch these.
- **`judgment[]`** — one entry per thing the tool refused to guess. Each carries
  the `path` (which node), the `kind` (`missing-actor` or `prose-citation`), and
  the verbatim `context` (the timestamp value, or the citation line) you need to
  decide it. Note the first `prose-citation`: `- [mouthfeel](/concepts/mouthfeel.md)`
  *looks* follow-able but the parser could not resolve it — that is the pitfall
  described below, and the tool correctly refuses to guess rather than drop a
  half-parsed target into `sources`.

### 2. Resolve the deterministic actor renames

Supplying the provenance actor turns every `missing-actor` item into a real
`generated` rename — no plan editing needed:

```sh
$ okfctl migrate mykb --plan migrate-plan.json --generated-by "human:alex"
Wrote migrate-plan.json: 2 node(s) with deterministic edits (5 edit(s)), 3 judgment item(s).
3 item(s) need a decision before they can migrate; see the plan file.
```

The two `missing-actor` items are gone (folded into `nodes[].generated`); only
the three `prose-citation` items remain. Use an actor string that means something
in your world — `human:<name>`, a tool identifier, a team handle. The tool does
not interpret it; it records it verbatim as `generated.by` (§7).

### 3. Work the remaining judgment items

Each `prose-citation` item is a decision only you can make. Three routes — pick
whichever fits; they end in the same place.

**Route A — hand it to an agent.** The plan is self-describing, so a slice pastes
cleanly into any coding-agent session or chat window. A good prompt names the
node, the citation line, and the *rules* — never "find a source," which invites a
hallucinated URL:

> Here is a `prose-citation` judgment item from an OKF v0.2 migration plan:
> `{ "path": "concepts/mouthfeel.md", "kind": "prose-citation",
> "context": "- Personal observation from a comparative tasting, 2026." }`
> §5.1 requires a real `resource` per source. Only three outcomes are valid:
> (a) if a real citable URL/path exists, give it; (b) if it is an unverifiable
> observation, say so and leave it out of `sources`; (c) if unsure, say unsure.
> Do NOT invent a URL.

A **good** decision coming back is one of: a real resource you can follow; an
explicit "no citable source — leave it in the body / drop it"; or "I can't
decide." A **bad** decision is a plausible-looking URL nobody can verify — reject
it exactly as the tool would have.

**Route B — a shell loop, a colleague, a ticket.** The plan's consumer is
deliberately unspecified. Split `judgment[]` however you distribute work; the
tool neither knows nor cares who resolves an item.

**Route C — by hand (no model required).** The plan is ordinary JSON; edit it in
any text editor. To turn a judgment item into an applied edit, **add the resolved
`resource` to that node's entry under `nodes[]`, and delete the item from
`judgment[]`.** Example — the reader decided one finding does have a citable
resource:

```json
"nodes": [
  {
    "path": "concepts/mouthfeel.md",
    "generated": { "by": "human:alex", "at": "2026-02-03" },
    "sources": [
      { "resource": "https://example.org/interviews/winemaker-2026" }
    ]
  }
]
```

and remove the matching `- Winemaker interview (unrecorded).` object from
`judgment[]`. Anything you leave in `judgment[]` is simply not applied — that is
fine; unresolved is a valid end state.

### 4. Apply (phase 2) — but dry-run first, and diff

**Always dry-run before the real apply.** `--dry-run` writes nothing and its
outcome is byte-identical to the real run, so it is a faithful preview:

```sh
$ okfctl migrate mykb --apply --plan migrate-plan.json --dry-run
would rewrite concepts/mouthfeel.md: timestamp -> generated { by: human:alex, at: 2026-02-03 }
would add concepts/mouthfeel.md: sources[].resource = /concepts/tannin.md
would add concepts/mouthfeel.md: sources[].resource = https://en.wikipedia.org/wiki/Mouthfeel
would rewrite concepts/tannin.md: timestamp -> generated { by: human:alex, at: 2026-01-15 }
would add concepts/tannin.md: sources[].resource = https://en.wikipedia.org/wiki/Tannin
2 node(s) would be migrated to v0.2 (dry run; nothing written).
```

If that is what you expect, apply for real. Snapshot the bundle first (a copy, or
just commit it to git) so you can diff — never accept a migration you cannot
inspect:

```sh
$ cp -r mykb mykb-before          # or: git add -A && git commit
$ okfctl migrate mykb --apply --plan migrate-plan.json
migrated concepts/mouthfeel.md
migrated concepts/tannin.md
2 node(s) migrated to v0.2.
bundle valid as v0.2
note: 3 judgment item(s) were not applied (they need a decision; see the plan file).
```

The apply **re-validates automatically** and reports `bundle valid as v0.2`. The
closing note is the tool being honest: unresolved judgment items were left alone.

Diff to see exactly what changed:

```sh
$ diff mykb-before/concepts/tannin.md mykb/concepts/tannin.md
3c3,5
< timestamp: 2026-01-15T09:30:00Z
---
> generated: {by: "human:alex", at: 2026-01-15}
> sources:
>   - resource: "https://en.wikipedia.org/wiki/Tannin"
```

`timestamp` became `generated` (the datetime normalized to a date on the `at`
field) and the one URL citation became a `sources` entry. The `mouthfeel` node
changed the same way, with two sources:

```sh
$ diff mykb-before/concepts/mouthfeel.md mykb/concepts/mouthfeel.md
3c3,6
< timestamp: 2026-02-03
---
> generated: {by: "human:alex", at: 2026-02-03}
> sources:
>   - resource: /concepts/tannin.md
>   - resource: "https://en.wikipedia.org/wiki/Mouthfeel"
```

In both nodes `timestamp` became `generated` and the follow-able citations became
`sources`. Everything else — including any unrecognized keys — is byte-for-byte
unchanged (§11: the migration
is additive and never drops a key it doesn't understand). The body `# Citations`
list is **left in place**; the migration *adds* frontmatter `sources`, it does
not delete the prose list, so prose findings you chose not to migrate simply stay
where they were.

### 5. Verify

`validate` is the objective gate — run it yourself, don't take the apply's word:

```sh
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
$ cat mykb/.okf
okf_version: 0.2
```

The migration is also **idempotent**: re-running plan+apply on an
already-migrated bundle finds `0 node(s) with deterministic edits` and produces a
zero-byte diff. Safe to re-run; safe to run twice.

## Common Pitfalls

1. **Treating a judgment item as a tool failure.** It is the design working. The
   tool did everything it could do safely; the item is there because guessing it
   would fabricate provenance. Resolve it, or leave it — both are valid.

2. **Fabricating a `resource` to clear a `prose-citation`.** A made-up URL to
   make the plan "clean" is the exact harm the two phases prevent. If a finding
   has no citable source, leave it out of `sources` — don't invent one.

3. **Guessing an actor to clear a `missing-actor`.** Only pass `--generated-by`
   with an actor you can stand behind. It is stamped verbatim into provenance
   (§7); a placeholder pollutes the graph.

4. **Applying without a dry-run or a snapshot.** `--dry-run` is byte-identical to
   the real run and free. Preview it, snapshot the bundle (copy or git commit),
   apply, then diff. A migration you can't inspect is one you can't trust.

5. **Expecting apply to delete the body `# Citations` list.** It doesn't — the
   migration is additive. It adds frontmatter `sources`; pruning the old prose
   list (if you want to) is a separate, manual editing choice.

6. **Assuming `okfctl` needs a model or API key for the plan.** It never does.
   The plan file's consumer (agent, colleague, shell loop, human) lives *beside*
   the tool. Any prompt to give `okfctl` a credential is wrong.

7. **Editing `nodes[]` but forgetting to remove the item from `judgment[]`
   (by-hand route).** Leaving it in `judgment[]` is harmless — it just isn't
   applied — but the "N item(s) need a decision" count won't drop, which is
   confusing. Move resolved items out of `judgment[]` so the count reflects
   reality.

8. **Expecting every markdown-link citation to resolve deterministically.** It
   depends on the link text. A link with descriptive text —
   `[tannin glossary](/concepts/tannin.md)` — resolves to a `sources` entry. But a
   link whose text is a single bare word — `[mouthfeel](/concepts/mouthfeel.md)` —
   is read as a bracket citation-*key* (`[mouthfeel]`) with no follow-able target,
   so it lands in `judgment[]` as a `prose-citation`. This is the tool refusing to
   guess, not a bug in your bundle: rather than half-parse the target it hands the
   item back for you to decide. If a single-word link *should* migrate, the safe
   fixes are on your side — give the link descriptive text, cite the bare URL/path
   directly (`- /concepts/mouthfeel.md` won't parse either; use a real link or
   URL), or resolve it by hand (pitfall 7). A bare URL always resolves.

## Verification Checklist

- [ ] Phase 1 (`okfctl migrate <bundle>`) wrote a plan and touched no node files
- [ ] Every `missing-actor` item you can resolve was cleared with
      `--generated-by <actor>` (a real actor, not a placeholder)
- [ ] Every `prose-citation` item was decided (resolved with a real resource, or
      deliberately left in `judgment[]`) — no fabricated resources
- [ ] `--apply --dry-run` previewed the exact edits and you snapshotted the
      bundle (copy or git commit) before the real apply
- [ ] Real apply reported `bundle valid as v<target>`; you diffed before/after
- [ ] `okfctl validate <bundle>` exits 0 and `.okf` shows `okf_version: 0.2`
- [ ] Re-running plan+apply produces a zero diff (idempotent)
