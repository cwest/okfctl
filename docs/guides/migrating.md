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
Wrote migrate-plan.json: 2 file(s) with deterministic edits (8 edit(s)), 2 judgment item(s).
version: 0.1 -> hold (1 prose-citation/missing-actor item(s) still rely on the §12.1 fallback that declaring v0.3 turns off; resolve them and re-plan, or re-plan with --force-declare)
edits: 2 file(s); timestamp->generated 2, citations->sources 2, citation titles->sources 0, root-relative links->/ 1, root-relative path fields->/ 0, timestamps->RFC 3339 0
attribution: source ids 2, footnote definitions 2, body references->footnotes 0
citation lists: removed 1, kept 1
judgment: 2 item(s); citations-kept 1, prose-citation 1
2 item(s) need a decision before they can migrate; see the plan file.
```

`acidity.md` gets four of the eight edits: the rename, the source, its
footnote definition (the source's `id` rides on the new source), and the
removal of its `# Citations` list, which now holds nothing the source and
footnote don't. `tannin.md` gets four, counting the link. The Montalcino line
is another matter. It names nothing a reader can follow, and `migrate` won't
invent a source for it. The plan files it under `judgment`, and keeps
`tannin.md`'s list because that line lives nowhere else:

```json
"judgment": [
  {
    "path": "wine/tannin.md",
    "kind": "prose-citation",
    "context": "- Tasting notes from the 2025 Montalcino trip, no link."
  },
  {
    "path": "wine/tannin.md",
    "kind": "citations-kept",
    "context": "entry 2: names no resource, so no source holds it"
  }
]
```

The `prose-citation` item is also why the version line says `hold`.
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
@@ -1,12 +1,14 @@
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

-# Citations
-- https://en.wikipedia.org/wiki/Acids_in_wine
+[^cite-1]: https://en.wikipedia.org/wiki/Acids_in_wine
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
```

After the diffs it repeats the plan summary and ends with
`2 file(s) would change (dry run; nothing written).` Each new source gets an
`id`, and the entry's full text becomes the footnote definition with that
label (§5.1: per-claim attribution is a footnote whose label is a
`sources[].id`). These entries have no key, so the id is `cite-<n>`, the
entry's place in the list.

A body `# Citations` list goes only when nothing in it would be lost: every
entry's full text is the footnote definition of its source (or opens it, when
a hand-annotated title was moved in after it), every resource it names is a
source's `resource`, the section holds nothing but entries, and no body `[k]`
still points at it. `acidity.md` passes, so its list goes.
`tannin.md` fails on the Montalcino line, so its list stays as written.

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
note: 2 judgment item(s) were not applied (they need a decision; see the plan file).
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

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/migrating/mykb" mykb
$ okfctl migrate mykb --plan migrate-plan.json --generated-by human:casey
$ okfctl migrate mykb --apply --plan migrate-plan.json
$ grep -v Montalcino mykb/wine/tannin.md > tannin.tmp
$ echo '- [Tasting notes, 2025 Montalcino trip](https://example.com/montalcino-2025)' >> tannin.tmp
$ mv tannin.tmp mykb/wine/tannin.md
-->
```text
$ okfctl migrate mykb --plan migrate-plan.json --generated-by human:casey
Wrote migrate-plan.json: 1 file(s) with deterministic edits (3 edit(s)), 0 judgment item(s).
version: 0.1 -> declare v0.3
edits: 1 file(s); timestamp->generated 0, citations->sources 1, citation titles->sources 0, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
attribution: source ids 1, footnote definitions 1, body references->footnotes 0
citation lists: removed 1, kept 0
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

The new link became a second `sources` entry (`id: cite-2`), with the link
text as its `title` and the entry as its footnote definition. With every entry
held by a source and a footnote, the `# Citations` list is gone. The
bundle-root `index.md` now opens with `okf_version: "0.3"`, as
does the `.okf` sidecar.

Run it again and nothing happens. A plan on a migrated bundle has no edits;
re-applying an old plan changes nothing. Each edit checks the file ahead of the
write:

```text
$ okfctl migrate mykb --plan again.json
Wrote again.json: 0 file(s) with deterministic edits (0 edit(s)), 0 judgment item(s).
version: 0.3 -> keep (already v0.3)
edits: 0 file(s); timestamp->generated 0, citations->sources 0, citation titles->sources 0, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
judgment: 0 item(s); none
```

### Resolving a citation with no public source

Some citations have nothing a reader can follow: notes you kept, a probe you
ran, a private tool. §5.1 still lets them be sources, because a `resource` can
be a scope descriptor, a few plain words a consumer reads but can't open. The
tool never writes one for you. You write it on the `prose-citation` item in the
plan file, and the next plan, run with `--resolve`, converts the entry the way
it converts any other: a source with that resource and an id, the entry's full
text as its footnote, and each body `[k]` rewritten to `[^id]`.

Start again from the bundle in step 3, the Montalcino line still in place.
Open `migrate-plan.json` and add a `resource` (and, if you like, a `title`) to
the `prose-citation` item:

```json
{
  "path": "wine/tannin.md",
  "kind": "prose-citation",
  "context": "- Tasting notes from the 2025 Montalcino trip, no link.",
  "resource": "tasting notes kept by the author, 2025 Montalcino trip (not published)"
}
```

Apply (and its `--dry-run`) refuses a plan that carries a resource it hasn't
planned, so the resolution is never dropped by accident. Plan it with `--resolve`, then apply:

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/migrating/mykb" notes-kb
$ okfctl migrate notes-kb --plan migrate-plan.json --generated-by human:casey
$ okfctl migrate notes-kb --apply --plan migrate-plan.json
$ okfctl migrate notes-kb --plan migrate-plan.json --generated-by human:casey
$ sed 's|"context": "- Tasting notes from the 2025 Montalcino trip, no link."|&, "resource": "tasting notes kept by the author, 2025 Montalcino trip (not published)"|' migrate-plan.json > plan.tmp
$ mv plan.tmp migrate-plan.json
-->
<!-- transcript: exit 1 okfctl migrate notes-kb --apply --plan migrate-plan.json --dry-run -->
```text
$ okfctl migrate notes-kb --apply --plan migrate-plan.json --dry-run
okfctl: migrate-plan.json: 1 judgment item(s) carry a resource that is not planned yet; plan them first: okfctl migrate notes-kb --plan migrate-plan.json --resolve migrate-plan.json
$ okfctl migrate notes-kb --plan migrate-plan.json --resolve migrate-plan.json --generated-by human:casey
Wrote migrate-plan.json: 1 file(s) with deterministic edits (3 edit(s)), 1 judgment item(s).
resolved: 1 prose-citation item(s) from --resolve, planned as sources.
version: 0.1 -> declare v0.3
edits: 1 file(s); timestamp->generated 0, citations->sources 1, citation titles->sources 0, root-relative links->/ 0, root-relative path fields->/ 0, timestamps->RFC 3339 0
attribution: source ids 1, footnote definitions 1, body references->footnotes 0
citation lists: removed 1, kept 0
judgment: 1 item(s); source-title-unnamed 1
1 item(s) need a decision before they can migrate; see the plan file.
$ okfctl migrate notes-kb --apply --plan migrate-plan.json
migrated wine/tannin.md
1 file(s) migrated; bundle declares v0.3.
hint: after committing, list that commit's SHA in .okf-drift-ignore-revs so strict drift skips the mechanical edits (like git blame --ignore-revs-file).
bundle valid as v0.3
note: 1 judgment item(s) were not applied (they need a decision; see the plan file).
```

`wine/tannin.md` now lists the descriptor as source `cite-2`, and the entry's
text is its footnote:

```markdown
sources:
  - id: cite-1
    resource: "https://en.wikipedia.org/wiki/Tannin"
  - id: cite-2
    resource: "tasting notes kept by the author, 2025 Montalcino trip (not published)"
...
[^cite-2]: Tasting notes from the 2025 Montalcino trip, no link.
```

The `source-title-unnamed` item asks for the work's name: put a `title` on the
resolution, or set one on the source afterwards. `--resolve` can name the
`--plan` file itself, as here, or an older plan. A resolution whose path and
`context` match no `prose-citation` item (the entry was edited since, say)
fails the plan and leaves the plan file as it was. Once applied, the
resolution lives in the node: a later plan sees the entry's text in the
footnote of a source with an id, so it never reports the entry again, even
while the `# Citations` list it came from stays.

## What `migrate` changes

From v0.1 (v0.3 Appendix B), two renames:

- frontmatter `timestamp` → `generated.at`
- body `# Citations` list → frontmatter `sources`. An entry that names one and
  only one follow-able resource (a URL with a scheme, or the path of a file in
  the bundle, per §5.1 and §6.2) turns into one `sources` entry: that resource,
  plus the cited work's name as `title` (§5.1: "a label for humans"). The name
  is taken only where the entry marks it: a leading link's text, a quoted or
  italic title in citation position, or the page name after a leading URL.
  Authors, venue, year, identifiers, URLs, and grades never go into `title`.
  A quote of the page is not its name: a quote introduced by `Description:`,
  `Verbatim:`, `states`, `reads`, an error or message, a backticked ref, or a
  URL, and a quote that is a sentence or a lowercase clause, gives no title.
  An entry that marks no name gets no title and a `source-title-unnamed` item.
  An earlier run may have written the whole entry into `title`. Once the
  entry's text is the footnote definition of the source's `id`, that title
  is cut to the work's name, or removed with a `source-title-unnamed` item
  when the entry names none. A title that opens with the entry's `[key]` but
  was annotated by hand past the entry's text (`[S8] iCIMS, AI recruiting
  software page (fetched 2026-11-27). VENDOR-DOC.`) has that text appended
  to the footnote definition `migrate` writes, then is cut or removed the
  same way, so nothing is lost. A title that is already a name stays, and so
  do the source's other keys (`note`, `x-` keys; §4.1). The plan summary
  counts these on a `titles:` line.
  An entry with no URL that carries exactly one DOI, arXiv id, PubMed id,
  or ISBN gets a `resource` built from that identifier:
  `https://doi.org/<doi>`, `https://arxiv.org/abs/<id>`,
  `https://pubmed.ncbi.nlm.nih.gov/<pmid>/`, or
  `https://openlibrary.org/isbn/<isbn>`. An ISBN must pass its check digit
  first. The URL is built offline from a fixed pattern; `migrate` never fetches
  it. An identifier that another entry in the same list also names is a
  cross-reference, so that entry stays a judgment item. The full entry text
  stays in the file: in its footnote definition, or in the body list while
  that list is kept.
- an entry whose resource is in `sources` already is not added twice (a DOI
  URL matches however its host or case is spelled). If that source has no
  `title`, the entry's name fills it in. A source that already has a title
  keeps it.
- each entry whose source is in `sources` gets an `id` and a footnote
  definition, and the body cites it as a footnote (§5.1: per-claim attribution
  is a Markdown footnote whose label is a `sources[].id`; §4.2). The id is the
  entry key when it is a plain slug (`[ga4-schema]`, `[12]`) that no other
  entry shares once case is folded; otherwise the ordered-list number;
  otherwise `cite-<n>`, the entry's place in the list. GFM matches footnote
  labels case-insensitively, so `[S1]` and `s1` count as one label, and a node
  whose ids would collide is refused rather than renumbered. A number-looking
  id is quoted (§4: `id: "12"`). The footnote definition `[^12]: <entry text>`
  holds the whole entry, with its list marker removed and its wrapped lines
  joined, and sits just above `# Citations`; the leading `[12]` is dropped
  only because it became the id. A grade bracket such as
  `[PRIMARY-DOC, VERIFIED]` stays. Each body `[12]` becomes `[^12]`, and
  `[3]–[5]` or `[1, 2]` expand to one footnote per entry. Code, links,
  reference-link labels, and footnotes already in place are left alone.
- the body `# Citations` list is removed (§4.2: "Do not use a list of
  citations in the body") once the migrated file holds everything in it. The
  check runs on the bytes the apply writes, entry by entry: the entry's text,
  whitespace collapsed, equals the footnote definition of its first source; each
  URL or bundle path it names is some source's `resource`; the section has no
  line that isn't an entry (a status key, a group label, a link reference
  definition); and no body `[k]` still cites the list. One miss keeps the
  whole list and files a `citations-kept` item. Everything outside the section
  keeps its bytes.

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

- a citation entry with no follow-able resource (§5.1 wants one)
  (`prose-citation`), or with several where one of them is also named by
  another entry of the node, so splitting it would need two ids on one source.
  Two identifiers with no URL count as two resources and stay here too, and so
  does an ISBN with a bad check digit.
- a `timestamp` rename with no actor (§7, `missing-actor`)
- a date-time with no offset, for example `2026-06-20T10:00:00`
  (`timestamp-no-offset`, §5 and §13.2). It names no instant. Only the
  author knows which offset was meant.
- a §5 timestamp value that isn't a date at all (`timestamp-invalid`)
- a citation entry for an untitled source whose text marks no work name
  (`source-title-unnamed`). The title stays unset until you name the work.
- a citation entry that names a different work than the title its source
  already has (`source-title-conflict`). The existing title stays.
- a source whose `title` holds its whole citation entry, where no footnote
  definition with the source's `id` holds that text (`source-title-unpreserved`).
  The title stays, so the text is never lost.
- a body `[k]` that names an entry of the node but sits within 60 characters
  of the word "brief", "citation", or "node" (plurals too, not "inode" or
  "briefing"), or of a link, in the same paragraph, list item, or table row; or
  is the label of an undefined `[text][k]` reference (`citation-ref-unclear`).
  It may point at another document's list, so it stays as written. A heading
  or the next paragraph doesn't count, and a `[k]` in code is never a
  reference.
- a second citation entry for a resource another entry already holds
  (`citation-unattached`). A source carries one `id`, so this entry gets no
  footnote and its body references stay as written.
- a node whose ids would collide once case is folded (`source-id-collision`).
  It gets no ids, footnotes, or reference rewrites.
- a `# Citations` list that can't go without losing something
  (`citations-kept`). The context names each entry and what would be lost:
  an entry with no source, a URL no source holds, a footnote whose text
  differs from the entry, a line that isn't an entry, or a body `[k]` still
  pointing at the list. The list stays as written.

The three `source-title-*` kinds and these four don't hold the version
declaration: the resource is already in `sources`, or the list stays as text.

An entry that names several resources is not a judgment item: it becomes one
source per resource, since §5.1 gives a source one `resource`. Given
`[12] VERIFIED — *Export Schema*, https://a.example/schema; mirror https://b.example/mirror`
and a body that cites it as `[12]`, migrate writes `id: "12"`
(`title: Export Schema`) and `id: 12-2`, rewrites the reference to
`[^12][^12-2]`, and defines `[^12]:` as the full entry text and `[^12-2]:` as
`https://b.example/mirror`. A later part is titled only when the entry names
it with a link. A URL inside a code span (`curl https://…`, a URL template) is
not a resource and is never split off. When an entry names no resource outside
code, a code span that holds a full URL and nothing else is read like an
autolink: `` `https://a.example/docs` and `https://a.example/api` `` become two
sources. A bare host (`` `api.example.com` ``), a command, or a template with a
placeholder (`…`, `{id}`, `<term>`) in a code span still names no source.

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
