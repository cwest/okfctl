# Migrating a bundle to OKF v0.3

If your bundle is on OKF v0.1 or v0.2 (or never said which), `okfctl migrate`
moves it to v0.3. Each output block below is what it printed on a two-node v0.1
bundle, pasted unedited. The first run only writes a plan file. The second applies
that plan, except for the one citation it wouldn't guess at. No step needs a model.

## A migration, start to finish

The bundle is five files. Its `.okf` sidecar declares `okf_version: 0.1`.

```text
mykb/.okf
mykb/index.md         links to both nodes
mykb/log.md
mykb/wine/acidity.md
mykb/wine/tannin.md
```

`wine/acidity.md` still has a `timestamp` key and a body
`# Citations` list:

```markdown
---
type: Reference
title: Acidity
timestamp: 2026-06-02T09:15:00Z
---

# Acidity

Acidity makes a wine taste fresh.

# Citations
- https://en.wikipedia.org/wiki/Acids_in_wine
```

`wine/tannin.md` has those two, plus a link written from the bundle root and a
citation that's only a sentence:

```markdown
---
type: Reference
title: Tannin
timestamp: 2026-05-28T22:53:05Z
---

# Tannin

Tannin is the drying grip in red wine. It balances against
[acidity](wine/acidity.md).

# Citations
- https://en.wikipedia.org/wiki/Tannin
- Tasting notes from the 2025 Montalcino trip, no link.
```

### 1. Plan

```sh
okfctl migrate mykb --plan migrate-plan.json --generated-by human:casey
```

```text
Wrote migrate-plan.json: 2 file(s) with deterministic edits (5 edit(s)), 1 judgment item(s).
version: 0.1 -> hold (1 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 2, citations->sources 2, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 1 item(s); prose-citation 1
1 item(s) need a decision before they can migrate; see the plan file.
```

`acidity.md` gets two of the five edits. `tannin.md` gets three, counting the
link. The Montalcino line is another matter. It
names nothing a reader can follow, and `migrate` won't invent a source for it.
The plan files it under `judgment`:

```json
"judgment": [
  {
    "path": "wine/tannin.md",
    "kind": "prose-citation",
    "context": "- Tasting notes from the 2025 Montalcino trip, no link."
  }
]
```

That one item is also why the version line says `hold`.
[The last section](#why-the-version-declaration-waits) explains it.

`--generated-by` takes the actor (§7) that `migrate` records as `generated.by`
when it renames a `timestamp`. Pass the person or process that wrote the node,
like `human:casey`. Leave it off and each rename shows up as a `missing-actor`
judgment item.

### 2. Preview

`--dry-run` prints the diff the apply step would write, and writes nothing:

```sh
okfctl migrate mykb --apply --plan migrate-plan.json --dry-run
```

```diff
--- a/wine/acidity.md
+++ b/wine/acidity.md
@@ -1,7 +1,9 @@
 ---
 type: Reference
 title: Acidity
-timestamp: 2026-06-02T09:15:00Z
+generated: {by: "human:casey", at: "2026-06-02T09:15:00Z"}
+sources:
+  - resource: "https://en.wikipedia.org/wiki/Acids_in_wine"
 ---

 # Acidity
--- a/wine/tannin.md
+++ b/wine/tannin.md
@@ -1,13 +1,15 @@
 ---
 type: Reference
 title: Tannin
-timestamp: 2026-05-28T22:53:05Z
+generated: {by: "human:casey", at: "2026-05-28T22:53:05Z"}
+sources:
+  - resource: "https://en.wikipedia.org/wiki/Tannin"
 ---

 # Tannin

 Tannin is the drying grip in red wine. It balances against
-[acidity](wine/acidity.md).
+[acidity](/wine/acidity.md).

 # Citations
 - https://en.wikipedia.org/wiki/Tannin
```

After the diffs it repeats the plan summary and ends with
`2 file(s) would change (dry run; nothing written).` The body
`# Citations` list stays where it was. `migrate` adds frontmatter and never
deletes the prose it came from.

### 3. Apply

```sh
okfctl migrate mykb --apply --plan migrate-plan.json
```

```text
migrated wine/acidity.md
migrated wine/tannin.md
2 file(s) migrated; okf_version declaration held: bundle still declares v0.1 (v0.2 fallbacks apply, §12.1).
hint: after committing, list that commit's SHA in .okf-drift-ignore-revs so strict drift skips the mechanical edits (like git blame --ignore-revs-file).
bundle valid
note: 1 judgment item(s) were not applied (they need a decision; see the plan file).
```

Take the hint. Commit the result and add that commit's SHA to
`.okf-drift-ignore-revs`, or `validate --strict` will report git drift on every
file the migration touched. The
[index and freshness guide](index-and-freshness.md) explains that file.

### 4. Resolve the judgment item and declare v0.3

Give the Montalcino citation something to point at. Here we make it a link:

```markdown
- [Tasting notes, 2025 Montalcino trip](https://example.com/montalcino-2025)
```

Plan and apply again:

```text
$ okfctl migrate mykb --plan migrate-plan.json --generated-by human:casey
Wrote migrate-plan.json: 1 file(s) with deterministic edits (1 edit(s)), 0 judgment item(s).
version: 0.1 -> declare v0.3
edits: 1 file(s); timestamp->generated 0, citations->sources 1, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 0 item(s); none
$ okfctl migrate mykb --apply --plan migrate-plan.json
migrated wine/tannin.md
1 file(s) migrated; bundle declares v0.3.
hint: after committing, list that commit's SHA in .okf-drift-ignore-revs so strict drift skips the mechanical edits (like git blame --ignore-revs-file).
bundle valid as v0.3
$ okfctl bundle info mykb
nodes: 2
reserved: 2
okf_version: 0.3 (declared in index.md)
```

The new link became a second `sources` entry, with the link text as its
`title`. The bundle-root `index.md` now opens with `okf_version: "0.3"`, as
does the `.okf` sidecar.

Run it again and nothing happens. A plan on a migrated bundle has no edits;
re-applying an old plan changes nothing. Each edit checks the file ahead of the
write:

```text
$ okfctl migrate mykb --plan again.json
Wrote again.json: 0 file(s) with deterministic edits (0 edit(s)), 0 judgment item(s).
version: 0.3 -> keep (already v0.3)
```

## What `migrate` changes

From v0.1 (v0.3 Appendix B), two renames:

- frontmatter `timestamp` → `generated.at`
- body `# Citations` list → frontmatter `sources`. An entry that names one and
  only one follow-able resource (a URL with a scheme, or the path of a file in
  the bundle, per §5.1 and §6.2) turns into one `sources` entry: that resource,
  plus the entry's text as `title`. The claim beside the URL comes along with
  it. The body list stays.

From v0.2 (v0.3 §13.1), two edits:

- Under v0.3, any link or path field with no leading `/` resolves from the
  directory of its file (§6). A v0.2 bundle MAY still resolve it from the bundle
  root (§12.1). While the bundle still declares an older version, `migrate`
  rewrites each value that resolves **only** from the bundle root to the
  `/`-rooted form. It then points at the same file under v0.3. A value that
  resolves from its own file stays as written. So do `/`-rooted values, URLs,
  and broken links.
- A §5 timestamp (`generated.at`, `verified[].at`, `stale_after`,
  `sources[].last_modified`, `usage_window`) that carries an offset but isn't
  RFC 3339, for example `2026-06-20 10:00:00Z`, is respelled to
  `"2026-06-20T10:00:00Z"`. The offset is kept as written. The instant
  stays put.

Last, the bundle-root `index.md` declares `okf_version: "0.3"` (§8.1). If the
root index has a frontmatter block, every other byte in that block stays. An index
with no block gets an `okf_version`-only block. A legacy `.okf` sidecar, if
present, is bumped too.

## Judgment items

`migrate` writes these to the plan and leaves the file unchanged:

- a citation entry with no follow-able resource (§5.1 wants one), or with two
  different ones, where picking one or splitting the entry is your call
  (`prose-citation`)
- a `timestamp` rename with no actor (§7, `missing-actor`)
- a date-time with no offset, for example `2026-06-20T10:00:00`
  (`timestamp-no-offset`, §5 and §13.2). It names no instant. Only the
  author knows which offset was meant.
- a §5 timestamp value that isn't a date at all (`timestamp-invalid`)

## Bundles `migrate` leaves alone

Point it at a bundle that's on v0.3 now and you get content edits only, with
the version line left as it is. A later version, or one okfctl has never heard
of, gets an empty plan and no writes. §12 tells a consumer to make a best effort
with a bundle like that and not to refuse it. Rewriting it to v0.3 would be a
downgrade.

## Why the version declaration waits

Those two judgment kinds (`prose-citation` and `missing-actor`) can still be
read through a v0.1 fallback. A consumer MAY parse the body `# Citations` list,
or use a legacy `timestamp` (Appendix B). §12.1 allows that only for a bundle
that declares 0.2 or older, or no version at all. Declare 0.3 with one of those
items left and a v0.3 consumer drops it.

The plan's `version_action` says `hold` until they're gone. Apply still writes
the content edits, but `index.md` and `.okf` stay untouched. Fix the items and
plan again, and the next apply declares v0.3. Or let them go: plan with
`--force-declare`, and the plan records that you chose it while the summary
counts how many items lose the fallback.

`okfctl migrate --help` lists the flags.
