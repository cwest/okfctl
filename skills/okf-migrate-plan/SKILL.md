---
name: okf-migrate-plan
description: Use when working an okfctl migrate-plan.json — the judgment items okfctl migrate refuses to guess when upgrading a v0.1 or v0.2 bundle to OKF v0.3. Covers the deterministic/judgment split, handing a plan to any agent or working it by hand, and verifying the applied migration.
version: 1.3.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, migrate, migrate-plan, provenance, v0.3, knowledge-graph]
    related_skills: [okf-authoring, okf-curation-health]
    sharing: shareable
---

# Working an okfctl migrate-plan

## Overview

`okfctl migrate` upgrades an OKF bundle that declares **v0.1 or v0.2, or no
version, to v0.3**. It runs in two phases, and the split is the whole point:

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

- You ran `okfctl migrate` on a v0.1 or v0.2 bundle and are staring at a
  `migrate-plan.json` for the first time.
- The plan reports "N item(s) need a decision before they can migrate" and you
  want to know what to do with them.
- You want to hand a plan (or a slice of it) to whatever agent you already use.
- You want to work the plan by hand with no model at all.

Don't use for: authoring or moving nodes (see `okf-authoring`); orphan/coverage/
link-health findings (see `okf-curation-health`). This skill is only the migrate
plan-and-apply loop.

## The deterministic / judgment split

`okfctl migrate` applies the v0.1 renames (OKF v0.3 Appendix B) and the v0.2 →
v0.3 edits (§13.1). Each is sometimes deterministic and sometimes a judgment
call — the tool decides per item:

| Old form | v0.3 form | Deterministic when… | Judgment when… |
|---|---|---|---|
| `timestamp:` frontmatter (v0.1) | `generated: {by, at}` (§7) | you pass `--generated-by <actor>` | no actor supplied → `missing-actor` |
| body `# Citations` entry (v0.1) | `sources[]: {resource, title}` (§5.1) | the entry names exactly one follow-able resource — an absolute URL or a markdown link to a bundle path (§6.2) — anywhere in it, even across wrapped lines; `title` is the cited work's name only, when the entry marks one | it names no resource, or two different ones → `prose-citation`; it marks no name → `source-title-unnamed` (the source is still added, untitled) |
| body `# Citations` entry whose resource is already in `sources` | that source's `title` (§5.1) | the source has no `title` and the entry marks the work's name: the name is set as `title` | the entry marks no name → `source-title-unnamed` (the title stays unset); the source already has a different `title` → `source-title-conflict` (the existing title stays) |
| bare link or path field that resolves only from the bundle root (v0.2, §12.1) | the same target with a leading `/` (§6 rule 2) | always: the tool knows which file it resolves to today | never |
| §5 timestamp not in RFC 3339 form | `"2026-06-20T10:00:00Z"` | it carries an offset (`Z`, `+0000`, `-05:00`), so the instant is fixed | no offset → `timestamp-no-offset`; not a date at all → `timestamp-invalid` |

The version marker waits for the judgment items. On apply, the bundle-root
`index.md` declares `okf_version: "0.3"` (§8.1) and a legacy `.okf` sidecar, if
present, is bumped too — but **only when no `prose-citation` or `missing-actor`
item is left**. Those items are still readable only through the v0.1 fallbacks
(`# Citations`, `timestamp`; Appendix B), and §12.1 grants those fallbacks only
to a bundle that declares 0.2 or earlier, or nothing. Declaring 0.3 first would
make every v0.3 consumer drop them. So while any remain, the plan says `hold`:
the edits apply and the declaration waits. `--force-declare` overrides that, and
the plan says so. A bundle that declares v0.3 already, a later version, or one
the tool doesn't recognize is never rewritten to an older version (§12): the
plan says `keep`.

**Why the refusals are correct, not lazy.** §5.1 makes `resource` *required* for
every `sources` entry. A "citation" that is actually an evidence finding —
*"Personal observation from a comparative tasting, 2026. Not published
anywhere."* — has no resource. Forcing it into `sources` would mean **fabricating
a `resource`**, and a made-up provenance URL is worse than an honest gap. That is
the exact failure the two-phase design exists to prevent. Same for provenance
actors (§7): the tool will not invent *who* generated a node, so a `timestamp`
with no supplied actor stays a judgment item rather than a guessed `generated.by`.
And a date-time with no offset (`2026-06-20T10:00:00`) names no instant (§5,
§13.2): only the author knows whether it meant UTC or local time, so the tool
won't pick one.

## Cold start: from a v0.1 bundle you own

Build the binary once (`CGO_ENABLED=0 go build -o okfctl .` in a checkout, then
put it on PATH). Everything below was run against a scratch two-node v0.1 bundle
(`concepts/mouthfeel.md`, `concepts/tannin.md`); the output is reproduced
verbatim. The two nodes carry, between them, the citation shapes that matter: a
bare URL, markdown links to bundle paths, and pure prose findings. Both carry a
legacy `timestamp`, and the bundle declares v0.1 in a `.okf` sidecar. `tannin.md` also links
`[mouthfeel](concepts/mouthfeel.md)`, a bare path that resolves only from the
bundle root.

### 1. Plan (phase 1, pure read)

```sh
$ okfctl migrate mykb --plan migrate-plan.json
Wrote migrate-plan.json: 2 file(s) with deterministic edits (5 edit(s)), 4 judgment item(s).
version: 0.1 -> hold (4 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 0, citations->sources 4, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 4 item(s); missing-actor 2, prose-citation 2
4 item(s) need a decision before they can migrate; see the plan file.
```

Nothing was written to the bundle — only the plan file. Inspect it:

```json
{
  "target_version": "0.3",
  "declared_version": "0.1",
  "version_action": "hold",
  "nodes": [
    {
      "path": "concepts/mouthfeel.md",
      "sources": [
        { "resource": "/concepts/tannin.md", "title": "tannin glossary" },
        { "resource": "https://en.wikipedia.org/wiki/Mouthfeel" },
        { "resource": "/concepts/mouthfeel.md", "title": "mouthfeel" }
      ]
    },
    {
      "path": "concepts/tannin.md",
      "sources": [
        { "resource": "https://en.wikipedia.org/wiki/Tannin" }
      ],
      "paths": [
        { "field": "link", "old": "concepts/mouthfeel.md", "new": "/concepts/mouthfeel.md" }
      ]
    }
  ],
  "judgment": [
    { "path": "concepts/mouthfeel.md", "kind": "missing-actor",  "context": "2026-02-03" },
    { "path": "concepts/mouthfeel.md", "kind": "prose-citation", "context": "- Personal observation from a comparative tasting, 2026. Not published anywhere." },
    { "path": "concepts/tannin.md",    "kind": "missing-actor",  "context": "2026-01-15T09:30:00Z" },
    { "path": "concepts/tannin.md",    "kind": "prose-citation", "context": "- Tasting notes from a 2026 barrel sampling; no public source." }
  ]
}
```

Read the file top to bottom:

- **`version_action: "hold"`** — the bundle will NOT declare v0.3 on this apply,
  because four items still depend on the v0.1 fallbacks. Resolve them (steps 2
  and 3) and re-plan, and it becomes `"bump"`.
- **`nodes[]`** — the deterministic edits the tool already worked out. `mouthfeel`
  got three `sources`: two links to bundle paths (the link text becomes `title`;
  the path keeps its form per §6) and a bare Wikipedia URL, which gets no
  `title` because the entry is nothing but the URL. An entry with words around its
  one URL — `[1] Smith, *Tannin chemistry*, 2024. https://…` — gets the work's
  name as `title` (`Tannin chemistry`), never the authors, year, or URL; the full
  entry stays in the body. `tannin` got its one bare URL, and its root-relative link
  `concepts/mouthfeel.md` becomes `/concepts/mouthfeel.md` (`paths[]`), which
  resolves to the same file once the bundle declares v0.3. You do not need to
  touch these.
- **`judgment[]`** — one entry per thing the tool refused to guess. Each carries
  the `path` (which node), the `kind` (`missing-actor` or `prose-citation`), and
  the verbatim `context` (the timestamp value, or the citation entry) you need to
  decide it. Both `prose-citation` items here name no resource at all. An entry
  that names two different URLs lands here too: picking which one is "the"
  source, or splitting the entry, is your call, not the tool's.

### 2. Resolve the deterministic actor renames

Supplying the provenance actor turns every `missing-actor` item into a real
`generated` rename — no plan editing needed:

```sh
$ okfctl migrate mykb --plan migrate-plan.json --generated-by "human:alex"
Wrote migrate-plan.json: 2 file(s) with deterministic edits (7 edit(s)), 2 judgment item(s).
version: 0.1 -> hold (2 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 2, citations->sources 4, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 2 item(s); prose-citation 2
2 item(s) need a decision before they can migrate; see the plan file.
```

The two `missing-actor` items are gone (folded into `nodes[].generated`); only
the two `prose-citation` items remain, so the version is still held. Use an actor string that means something
in your world — `human:<name>`, a tool identifier, a team handle. The tool does
not interpret it; it records it verbatim as `generated.by` (§7).

### 3. Work the remaining judgment items

Each `prose-citation` item is a decision only you can make. Three routes — pick
whichever fits; they end in the same place.

**Route A — hand it to an agent.** The plan is self-describing, so a slice pastes
cleanly into any coding-agent session or chat window. A good prompt names the
node, the citation line, and the *rules* — never "find a source," which invites a
hallucinated URL:

> Here is a `prose-citation` judgment item from an OKF migration plan:
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

**Route C — by hand (no model required).** The plan is ordinary JSON, and the
bundle is ordinary Markdown. The tool reads the body `# Citations` list on every
plan, so **resolve an item in the node, then re-plan**:

- If the finding does have a citable resource, put it in the entry
  (`- Winemaker interview, 2026. https://example.org/interviews/winemaker-2026`).
  The next plan turns it into a `sources` entry, with the entry text as `title`.
- If it has no citable resource, decide where the claim lives: move it out of
  the `# Citations` list (into the body prose, or a §5.2 `verified` event), or
  delete it. It is no longer a citation, so it stops being a judgment item.

You can also add a resolved `resource` straight to that node's entry under
`nodes[]` in the plan file and apply it. That writes the source, but the prose
line is still in `# Citations`, so the next plan still lists it and still holds
the version. Fix the body entry too, or see `--force-declare` below.

Anything you leave unresolved is simply not applied. That is a valid end state:
the bundle keeps the edits, stays at its current version, and every v0.3 consumer
still reads the prose list through the §12.1 fallback.

**When you are done deciding.** If you have worked every item you intend to
and want v0.3 anyway, re-plan with `--force-declare`. The plan records the
choice, and the summary tells you how many items lose the fallback:

```sh
$ okfctl migrate mykb --plan migrate-plan.json --generated-by "human:alex" --force-declare
Wrote migrate-plan.json: 2 file(s) with deterministic edits (7 edit(s)), 2 judgment item(s).
version: 0.1 -> declare v0.3 (forced: 2 prose-citation/missing-actor item(s) lose the §12.1 fallback)
edits: 2 file(s); timestamp->generated 2, citations->sources 4, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 2 item(s); prose-citation 2
2 item(s) need a decision before they can migrate; see the plan file.
```

### 4. Apply (phase 2) — but dry-run first, and diff

**Always dry-run before the real apply.** `--dry-run` writes nothing and prints
the unified diff the real apply would write, through the same renderer, so it is
a faithful preview. With the held plan from step 2:

```sh
$ okfctl migrate mykb --apply --plan migrate-plan.json --dry-run
--- a/concepts/mouthfeel.md
+++ b/concepts/mouthfeel.md
@@ -1,7 +1,13 @@
 ---
 type: Reference
 title: Mouthfeel
-timestamp: 2026-02-03
+generated: {by: "human:alex", at: 2026-02-03}
+sources:
+  - resource: /concepts/tannin.md
+    title: tannin glossary
+  - resource: "https://en.wikipedia.org/wiki/Mouthfeel"
+  - resource: /concepts/mouthfeel.md
+    title: mouthfeel
 ---

 # Mouthfeel
--- a/concepts/tannin.md
+++ b/concepts/tannin.md
@@ -1,12 +1,14 @@
 ---
 type: Reference
 title: Tannin
-timestamp: 2026-01-15T09:30:00Z
+generated: {by: "human:alex", at: "2026-01-15T09:30:00Z"}
+sources:
+  - resource: "https://en.wikipedia.org/wiki/Tannin"
 ---

 # Tannin

-Polyphenols that bind proteins; part of [mouthfeel](concepts/mouthfeel.md).
+Polyphenols that bind proteins; part of [mouthfeel](/concepts/mouthfeel.md).

 # Citations
 - https://en.wikipedia.org/wiki/Tannin
version: 0.1 -> hold (2 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 2, citations->sources 4, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 2 item(s); prose-citation 2
2 file(s) would change (dry run; nothing written).
```

Read the diff: `timestamp` became `generated` (the value carried across verbatim,
at full precision), the follow-able citations became `sources`, and the
root-relative link gained its leading `/`. Neither `index.md` nor `.okf` is in
the diff: the version is held. Everything else — including any unrecognized
keys — is byte-for-byte unchanged (§11: the migration is additive and never
drops a key it doesn't understand). The body `# Citations` list is **left in
place**; the migration *adds* frontmatter `sources`, it does not delete the prose
list.

If that is what you expect, apply for real. Snapshot the bundle first (a copy, or
just commit it to git) so you can diff afterwards too:

```sh
$ cp -r mykb mykb-before          # or: git add -A && git commit
$ okfctl migrate mykb --apply --plan migrate-plan.json
migrated concepts/mouthfeel.md
migrated concepts/tannin.md
2 file(s) migrated; okf_version declaration held: bundle still declares v0.1 (v0.2 fallbacks apply, §12.1).
hint: after committing, list that commit's SHA in .okf-drift-ignore-revs so strict drift skips the mechanical edits (like git blame --ignore-revs-file).
bundle valid
note: 2 judgment item(s) were not applied (they need a decision; see the plan file).
```

The apply **re-validates automatically**. The closing note is the tool being
honest: unresolved judgment items were left alone. The `hint:` line appears
whenever an apply edits files: once you commit the migration, list that commit
in `.okf-drift-ignore-revs` so `validate --strict` doesn't report git drift on
every file it touched.

Now resolve the two prose items (here, by deleting both lines from their
`# Citations` lists) and re-plan. Nothing relies on the fallback any more, so the
plan bumps, and the only edits left are the version markers:

```sh
$ okfctl migrate mykb --plan migrate-plan.json
Wrote migrate-plan.json: 0 file(s) with deterministic edits (0 edit(s)), 0 judgment item(s).
version: 0.1 -> declare v0.3
edits: 0 file(s); timestamp->generated 0, citations->sources 0, citation titles->sources 0, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 0 item(s); none
$ okfctl migrate mykb --apply --plan migrate-plan.json --dry-run
--- a/.okf
+++ b/.okf
@@ -1,1 +1,1 @@
-okf_version: 0.1
+okf_version: "0.3"
--- a/index.md
+++ b/index.md
@@ -1,1 +1,5 @@
+---
+okf_version: "0.3"
+---
+
 # Knowledge Base
version: 0.1 -> declare v0.3
edits: 0 file(s); timestamp->generated 0, citations->sources 0, citation titles->sources 0, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 0 item(s); none
2 file(s) would change (dry run; nothing written).
$ okfctl migrate mykb --apply --plan migrate-plan.json
0 file(s) migrated; bundle declares v0.3.
bundle valid as v0.3
```

### 5. Verify

`validate` is the objective gate — run it yourself, don't take the apply's word:

```sh
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
$ okfctl bundle info mykb | tail -1
okf_version: 0.3 (declared in index.md)
```

The migration is also **idempotent**, two ways. Re-running plan+apply on an
already-migrated bundle finds `0 file(s) with deterministic edits` and produces a
zero-byte diff. And re-applying an old plan file to a tree it already migrated
changes nothing: every edit re-checks the file first, so a source already listed
is not added again and a `timestamp` already renamed is not renamed twice. Safe
to re-run; safe to run twice.

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

4. **Applying without a dry-run or a snapshot.** `--dry-run` shows the exact
   diff the real run writes, and it is free. Preview it, snapshot the bundle (copy or git commit),
   apply, then diff. A migration you can't inspect is one you can't trust.

5. **Expecting apply to delete the body `# Citations` list.** It doesn't — the
   migration is additive. It adds frontmatter `sources`; pruning the old prose
   list (if you want to) is a separate, manual editing choice.

6. **Assuming `okfctl` needs a model or API key for the plan.** It never does.
   The plan file's consumer (agent, colleague, shell loop, human) lives *beside*
   the tool. Any prompt to give `okfctl` a credential is wrong.

7. **Reaching for `--force-declare` to make the plan "clean".** A held version
   is the tool protecting the prose citations you haven't decided yet: declaring
   0.3 switches off the §12.1 fallback that is the only way a consumer still
   reads them. Force it only once every item you care about is resolved and you
   have chosen to let the rest go.

8. **Expecting a citation with two URLs to migrate.** An entry that names two
   different resources — `[4] Vendor docs https://a.example/x, mirrored at
   https://b.example/x` — is a judgment item. The tool will not pick one, and it
   will not split the entry into two sources: both change what the entry claims.
   Split it yourself (one entry per resource) and re-plan, and each half
   migrates. The same URL repeated within one entry counts once.

## Verification Checklist

- [ ] Phase 1 (`okfctl migrate <bundle>`) wrote a plan and touched no node files
- [ ] Every `missing-actor` item you can resolve was cleared with
      `--generated-by <actor>` (a real actor, not a placeholder)
- [ ] Every `prose-citation` item was decided (resolved with a real resource,
      moved out of `# Citations`, or deliberately left) — no fabricated resources
- [ ] `--apply --dry-run` previewed the exact edits and you snapshotted the
      bundle (copy or git commit) before the real apply
- [ ] The summary's `version:` line says what you expect: `hold` while items
      remain, `declare v0.3` once they are resolved (or forced on purpose)
- [ ] `okfctl validate <bundle>` exits 0 and `okfctl bundle info <bundle>` shows
      `okf_version: 0.3 (declared in index.md)` once the version is declared
- [ ] Re-running plan+apply produces a zero diff (idempotent)
