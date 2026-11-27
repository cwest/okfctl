---
name: okf-migrate-plan
description: Use when upgrading a v0.1 or v0.2 OKF bundle to v0.3 with okfctl migrate, or working the judgment items in a migrate-plan.json. Covers plan, dry-run, apply, the edits the tool makes and the ones it refuses to guess, and how to resolve each without inventing provenance.
version: 2.0.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, migrate, migrate-plan, provenance, v0.3, knowledge-graph]
    related_skills: [okf-authoring, okf-curation-health]
    sharing: shareable
---

# Working an okfctl migrate plan

## Overview

Say you've got the bundle below, whose `.okf` sidecar declares OKF v0.1. Each
of its two nodes has a legacy `timestamp` and a body `# Citations` list. One
citation has a URL, and the other one reads
`- Tasting notes from the 2025 Montalcino trip, no link.` On top of that,
`tannin.md` links `[acidity](wine/acidity.md)`, and that path works only from
the bundle root. Plan the migration:

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/migrating/mykb" mykb
-->
```text
$ okfctl bundle info mykb
nodes: 2
reserved: 2
okf_version: 0.1 (declared in .okf)
$ okfctl migrate mykb --plan migrate-plan.json
Wrote migrate-plan.json: 2 file(s) with deterministic edits (5 edit(s)), 3 judgment item(s).
version: 0.1 -> hold (3 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 0, citations->sources 2, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
attribution: source ids 2, footnote definitions 2, body references->footnotes 0
judgment: 3 item(s); missing-actor 2, prose-citation 1
3 item(s) need a decision before they can migrate; see the plan file.
```

Planning reads the bundle and writes nothing but `migrate-plan.json`. There are
two kinds of work in it. Edits are changes the tool can make safely (here, two
URL citations turn into `sources` entries, each with an `id` and a footnote
definition holding the entry's text, and the root-relative link gets a
leading `/`). Judgment items are the ones it won't guess at. Neither
`timestamp` says who generated its node, and the Montalcino citation names no
resource. Since those three items still lean on v0.1 fallbacks, the version
stays at 0.1 for now (more on that below).

## When to Use

- Your bundle is on v0.1 or v0.2 (or has no version) and you want v0.3.
- You've got a `migrate-plan.json` with judgment items to resolve.
- You're handing judgment items to an agent, colleague, or script.

For creating or relocating nodes, see `okf-authoring`. Lint findings after the
migration go to `okf-curation-health`.

## The full loop on one bundle

Continuing from the plan above.

### 1. Read the plan

```text
$ okfctl migrate mykb --plan migrate-plan.json > /dev/null
$ cat migrate-plan.json
{
  "target_version": "0.3",
  "declared_version": "0.1",
  "version_action": "hold",
  "nodes": [
    {
      "path": "wine/acidity.md",
      "sources": [
        {
          "resource": "https://en.wikipedia.org/wiki/Acids_in_wine",
          "id": "cite-1"
        }
      ],
      "footnotes": [
        {
          "id": "cite-1",
          "text": "https://en.wikipedia.org/wiki/Acids_in_wine"
        }
      ]
    },
    {
      "path": "wine/tannin.md",
      "sources": [
        {
          "resource": "https://en.wikipedia.org/wiki/Tannin",
          "id": "cite-1"
        }
      ],
      "paths": [
        {
          "field": "link",
          "old": "wine/acidity.md",
          "new": "/wine/acidity.md"
        }
      ],
      "footnotes": [
        {
          "id": "cite-1",
          "text": "https://en.wikipedia.org/wiki/Tannin"
        }
      ]
    }
  ],
  "judgment": [
    {
      "path": "wine/acidity.md",
      "kind": "missing-actor",
      "context": "2026-06-02T09:15:00Z"
    },
    {
      "path": "wine/tannin.md",
      "kind": "missing-actor",
      "context": "2026-05-28T22:53:05Z"
    },
    {
      "path": "wine/tannin.md",
      "kind": "prose-citation",
      "context": "- Tasting notes from the 2025 Montalcino trip, no link."
    }
  ]
}
```

`nodes[]` lists the edits per file, and you can leave them as they are.
`judgment[]` has one entry per refusal, with the node `path`, its `kind`, and
the `context` you need to make the call (the timestamp value or the citation
line as written).
`version_action` is `hold` while any `missing-actor` or `prose-citation` item
is left and `bump` when none are. You'll see `keep` when the bundle declares
v0.3, or a version the tool doesn't know.

### 2. Supply the actor

Pass `--generated-by` and each `missing-actor` item turns into a `timestamp` to
`generated` rename:

```text
$ okfctl migrate mykb --plan migrate-plan.json --generated-by human:casey
Wrote migrate-plan.json: 2 file(s) with deterministic edits (7 edit(s)), 1 judgment item(s).
version: 0.1 -> hold (1 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 2, citations->sources 2, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
attribution: source ids 2, footnote definitions 2, body references->footnotes 0
judgment: 1 item(s); prose-citation 1
1 item(s) need a decision before they can migrate; see the plan file.
```

The tool writes the actor string as is into `generated.by` (§7). That's why
it has to be one you can stand behind: `human:<name>`, the name of a tool, or
a team handle.

### 3. Dry-run, then apply

`--dry-run` prints the unified diff the apply would write and writes nothing:

```text
$ okfctl migrate mykb --apply --plan migrate-plan.json --dry-run
--- a/wine/acidity.md
+++ b/wine/acidity.md
@@ -1,12 +1,17 @@
 ---
 type: Reference
 title: Acidity
-timestamp: 2026-06-02T09:15:00Z
+generated: {by: "human:casey", at: "2026-06-02T09:15:00Z"}
+sources:
+  - id: cite-1
+    resource: "https://en.wikipedia.org/wiki/Acids_in_wine"
 ---

 # Acidity

 Acidity makes a wine taste fresh.
+
+[^cite-1]: https://en.wikipedia.org/wiki/Acids_in_wine

 # Citations
 - https://en.wikipedia.org/wiki/Acids_in_wine
--- a/wine/tannin.md
+++ b/wine/tannin.md
@@ -1,13 +1,18 @@
 ---
 type: Reference
 title: Tannin
-timestamp: 2026-05-28T22:53:05Z
+generated: {by: "human:casey", at: "2026-05-28T22:53:05Z"}
+sources:
+  - id: cite-1
+    resource: "https://en.wikipedia.org/wiki/Tannin"
 ---

 # Tannin

 Tannin is the drying grip in red wine. It balances against
-[acidity](wine/acidity.md).
+[acidity](/wine/acidity.md).
+
+[^cite-1]: https://en.wikipedia.org/wiki/Tannin

 # Citations
 - https://en.wikipedia.org/wiki/Tannin
version: 0.1 -> hold (1 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 2, citations->sources 2, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
attribution: source ids 2, footnote definitions 2, body references->footnotes 0
judgment: 1 item(s); prose-citation 1
2 file(s) would change (dry run; nothing written).
$ okfctl migrate mykb --apply --plan migrate-plan.json
migrated wine/acidity.md
migrated wine/tannin.md
2 file(s) migrated; okf_version declaration held: bundle still declares v0.1 (v0.2 fallbacks apply, §12.1).
hint: after committing, list that commit's SHA in .okf-drift-ignore-revs so strict drift skips the mechanical edits (like git blame --ignore-revs-file).
bundle valid
note: 1 judgment item(s) were not applied (they need a decision; see the plan file).
```

With the version held, `index.md` and `.okf` don't show up in the diff at all.
The body `# Citations` list stays put. Migration adds frontmatter `sources`
and footnote definitions and deletes nothing. Keys the tool doesn't recognize come through
byte for byte (§11), and apply validates the bundle when it's done.

Commit or copy the bundle ahead of a real apply if you want to diff it
afterward. After you commit, add that commit's SHA to `.okf-drift-ignore-revs`
as the `hint:` line says.

### 4. Resolve the last item and declare v0.3

There's no resource to cite for the Montalcino line. This example deletes it
from `# Citations`, and with nothing left relying on the v0.1 fallback the plan
bumps the version:

```text
$ grep -v Montalcino mykb/wine/tannin.md > tannin.tmp
$ mv tannin.tmp mykb/wine/tannin.md
$ okfctl migrate mykb --plan migrate-plan.json
Wrote migrate-plan.json: 0 file(s) with deterministic edits (0 edit(s)), 0 judgment item(s).
version: 0.1 -> declare v0.3
edits: 0 file(s); timestamp->generated 0, citations->sources 0, citation titles->sources 0, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 0 item(s); none
$ okfctl migrate mykb --apply --plan migrate-plan.json
0 file(s) migrated; bundle declares v0.3.
bundle valid as v0.3
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
$ okfctl bundle info mykb
nodes: 2
reserved: 2
okf_version: 0.3 (declared in index.md)
$ okfctl migrate mykb --plan migrate-plan.json
Wrote migrate-plan.json: 0 file(s) with deterministic edits (0 edit(s)), 0 judgment item(s).
version: 0.3 -> keep (already v0.3)
edits: 0 file(s); timestamp->generated 0, citations->sources 0, citation titles->sources 0, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 0 item(s); none
```

The apply added `okf_version: "0.3"` to the root `index.md` frontmatter (§8.1)
and bumped `.okf` to match. The last plan finds nothing to do. Applying an old
plan to a bundle it has migrated changes nothing either. Each edit checks the
file first.

## What migrate changes, and what it hands back

Migrate does the v0.1 renames (OKF v0.3 Appendix B) along with the v0.2 to v0.3
edits (§13.1). This table says when each case turns into an edit and when it
comes back to you as a judgment item:

| Old form | v0.3 form | Edit when | Judgment item when |
|---|---|---|---|
| `timestamp:` (v0.1) | `generated: {by, at}` (§7) | you pass `--generated-by <actor>` | no actor: `missing-actor` |
| body `# Citations` entry (v0.1) | `sources[]: {resource, title}` (§5.1) | the entry names exactly one resource, an absolute URL or a Markdown link to a bundle path, even across wrapped lines | it names none, or two different ones: `prose-citation`; it marks no work name: `source-title-unnamed` (the source is still added, untitled) |
| body `# Citations` entry whose resource is already in `sources` | that source's `title` (§5.1) | the source has no `title` and the entry marks the work's name | it marks no name: `source-title-unnamed` (the title stays unset); the source already has a different `title`: `source-title-conflict` (the existing title stays) |
| body `# Citations` entry with a source, and the body `[k]` that cites it | `sources[].id`, footnote definition `[^id]: <entry text>`, body `[^id]` (§5.1, §4.2) | always: id is the plain-slug key unique after case folding, else the ordered-list number, else `cite-<n>`; `[3]–[5]` and `[1, 2]` expand | a `[k]` within 60 characters of "brief", "citation", "node", or a link: `citation-ref-unclear`; a second entry for the same resource: `citation-unattached`; ids that collide once case is folded: `source-id-collision` (nothing written for that node) |
| link or path field that resolves only from the bundle root (§12.1) | same target with a leading `/` (§6) | always | never |
| §5 timestamp not in RFC 3339 form | `"2026-06-20T10:00:00Z"` | it has an offset (`Z`, `+0000`, `-05:00`) | no offset: `timestamp-no-offset`; not a date: `timestamp-invalid` |

Feed it `[1] Smith, *Tannin chemistry*, 2024. https://…` and the new
`title` is `Tannin chemistry`. The authors, year, and URL stay in the body. The
tool only takes a name the entry marks for it, like the text of a leading link,
words in quotes or italics, or the page name after a leading URL. If it can't
find one, you get no `title` and a `source-title-unnamed` item. So does a bare
URL. When one entry has the same URL twice, that's one source.

The `source-title-*` items don't hold the version back. Their resources are in
`sources` now, and v0.3 readers find them there.

### Why the version waits

You can still read `prose-citation` and `missing-actor` items through the v0.1
fallbacks (the body `# Citations` list and `timestamp`, Appendix B). §12.1
only grants those fallbacks to a bundle that declares 0.2 or lower (or
nothing). Declare 0.3 while items are open and v0.3 consumers stop reading
them. That's why the plan says `hold`. The edits go in and the declaration
waits.

`--force-declare` overrides the hold, and the summary tells you how much you
give up. This run uses a fresh copy of the same v0.1 bundle:

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/migrating/mykb" kb2
-->
```text
$ okfctl migrate kb2 --plan migrate-plan.json --generated-by human:casey --force-declare
Wrote migrate-plan.json: 2 file(s) with deterministic edits (7 edit(s)), 1 judgment item(s).
version: 0.1 -> declare v0.3 (forced: 1 prose-citation/missing-actor item(s) lose the §12.1 fallback)
edits: 2 file(s); timestamp->generated 2, citations->sources 2, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
attribution: source ids 2, footnote definitions 2, body references->footnotes 0
judgment: 1 item(s); prose-citation 1
1 item(s) need a decision before they can migrate; see the plan file.
```

Bundles that declare v0.3, something later, or a version the tool doesn't
recognize never get rewritten to an older one (§12). For those the plan says
`keep`.

### Why the tool refuses

§5.1 says each `sources` entry needs a `resource`. The tasting note with no
public source doesn't have one, and putting it in `sources` would mean making
one up. Nor will the tool guess who generated a node (§7). And a date-time with
no offset (say, `2026-06-20T10:00:00`) doesn't name a single instant (§5,
§13.2). Only the author knows if they meant UTC or local time.

## Resolving judgment items

Fix the source node and re-plan. Each plan re-reads the body `# Citations`
list. An item goes away when the node stops producing it.

- **A citation with a real resource.** Add it to the entry, for example
  `- *Winemaker interview*, 2026. https://example.org/interviews/winemaker-2026`.
  The next plan makes it a `sources` entry titled with the work's name, if the
  entry marks one.
- **A citation that marks no name (`source-title-unnamed`).** Put the work's
  name in quotes or italics in the entry, or set `title` on the source by hand.
- **A title that differs from the citation (`source-title-conflict`).** The
  tool didn't touch the old `title`. Decide which name is right and edit the
  source.
- **A citation with no resource.** Move the claim out of `# Citations` and into
  the body, or into a §5.2 `verified` event. Or delete it.
- **A citation with two URLs.** Split it into one entry per resource. The tool
  won't pick one or split it for you.
- **A timestamp with no offset.** Add the offset the author meant.

Adding a `resource` straight to a node's entry under `nodes[]` and applying it
does work. But the prose line is still in `# Citations` afterward. The next
plan lists it again and holds the version again.

It's fine to leave an item unresolved. You get the edits, the bundle holds its
declared version, and v0.3 consumers still read the prose list through the
§12.1 fallback.

### Handing an item to an agent

The plan is plain JSON. You can paste one item into any agent chat. Hand it
the rule, never "find a source":

> Here is a `prose-citation` item from an OKF migration plan:
> `{ "path": "wine/tannin.md", "kind": "prose-citation", "context": "- Tasting notes from the 2025 Montalcino trip, no link." }`
> §5.1 says each source needs a real `resource`. Answer one of: (a) a real
> URL or bundle path that this cites, (b) "no citable source," and it stays out
> of `sources`, or (c) "unsure." Do not invent a URL.

If the agent comes back with a URL that you can't open and check, throw it
out the way the tool would.

## Common Pitfalls

1. Judgment items aren't failures. The tool stopped where it would have had to
   guess, and you can resolve the item or leave it.
2. Don't make up a `resource` to clear a `prose-citation`. The plan has two
   phases so you never have to.
3. Don't pass a placeholder to `--generated-by`. Whatever you pass lands in
   provenance as is.
4. Look at the real diff with `--dry-run`, and commit or copy the bundle ahead
   of any apply.
5. Apply won't delete `# Citations`. Migration only adds. Pruning the old list
   is a separate edit.
6. `--force-declare` won't give you a clean plan. It turns off the only way
   consumers can still read the items you haven't resolved.

## Verification Checklist

- [ ] The first `okfctl migrate <bundle>` wrote a plan and changed no node
- [ ] `--generated-by` got a real actor, if you used it
- [ ] Each `prose-citation` got a real resource, was moved out of
      `# Citations`, or was left on purpose
- [ ] `--apply --dry-run` showed the diff you wanted, and the bundle was
      committed or copied ahead of the real apply
- [ ] The `version:` line says `hold` while items are open and `declare v0.3`
      when they're resolved or forced on purpose
- [ ] `okfctl validate <bundle>` exits 0, and `okfctl bundle info <bundle>`
      shows `okf_version: 0.3 (declared in index.md)` after the declaration
- [ ] A second plan finds `0 file(s) with deterministic edits`
