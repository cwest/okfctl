# Changelog

All notable changes to `okfctl` are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Entries for tagged releases mirror the notes published on the
[GitHub releases page](https://github.com/cwest/okfctl/releases). On each new
tagged release, goreleaser generates the release notes from the commit log
(see `.goreleaser.yaml`).

## [Unreleased]

## [0.8.0] - 2026-11-27

`migrate` now deletes a node's body `# Citations` list (§4.2) when the
sources and footnotes it wrote hold everything in it. A list it can't prove
safe to delete stays put and gets reported. A source title someone annotated
by hand doesn't lose its extra text anymore.

### Added

- `migrate` deletes a node's body `# Citations` list (§4.2, Appendix B) if
  four things hold: each entry's text, with whitespace collapsed, is the
  footnote definition of its first source; each URL or bundle path the entry
  names is the `resource` of some source; the section holds nothing but
  entries; no body `[k]` still points at it. The check looks at what the
  other planned edits produce. At apply time it looks again at the bytes on
  disk, where a list edited since planning gets caught. If any check fails the
  list is kept, with a new `citations-kept` judgment item that names each
  entry and what deleting it would lose. Bytes outside the section
  don't change. A `citation lists: removed N, kept M` line joins the
  summary. (#262)
- `migrate` also moves a hand-annotated source `title` (one that opens with
  its entry's `[key]` but says more than the entry, like `[S8] iCIMS, AI
  recruiting software page (fetched 2026-11-27). VENDOR-DOC. VERIFIED.`)
  into the footnote definition it writes for that entry, then cuts the
  title to the work's name or removes and reports it. A footnote the author
  wrote is never edited. In that case the title is kept and reported
  (`source-title-unpreserved`). A bracketed name like `[Beta] Auto Routing`
  isn't touched. A footnote holding its entry's text followed by a
  moved annotation still counts as holding the entry. The node's
  `# Citations` list can still be removed. (#261)

## [0.7.0] - 2026-11-27

`migrate` now splits a `# Citations` entry that names several resources into
one source per resource, and cuts a source title that holds its whole entry
down to the work's name. This release also fixes a v0.6.0 bug: a source title
could be a quoted excerpt instead of the work's name. To clean a bundle
migrated with 0.6.0, see Fixed.

### Added

- `migrate` splits a `# Citations` entry that names two or more resources
  into one §5.1 source per resource, in the order the entry names them, with
  ids `<id>`, `<id>-2`, `<id>-3`. The first part's footnote definition holds
  the full entry text; a later part's holds its link text, or its resource
  when it has none. Each part's `title` is a name only: the name the entry
  marks for the first, link text for a later part named by a link. Every body
  `[k]` that cites the entry becomes `[^id][^id-2]…`. A URL inside a code span
  is not a resource. An entry with a resource that another entry of the node
  also names stays a `prose-citation` item. `analyze` no longer counts a split
  entry on top of the sources it became. (#259)
- `migrate` cuts a source `title` that holds its whole `# Citations` entry
  (as an earlier run wrote it) to the work's name, once the footnote
  definition `[^id]` holds the entry's text (§5.1: title is a label for
  humans). When the entry names no work the title is removed and reported
  (`source-title-unnamed`). A title whose text no footnote holds is kept and
  reported (`source-title-unpreserved`), so the text is never lost. A title
  that is already a name, and the source's other keys, are left alone. The
  plan summary gains a `titles:` line. (#258)

### Fixed

- `migrate` no longer takes a quoted excerpt as a source's name. In 0.6.0 the
  name picker could title a source with the text after `Description:`,
  `Verbatim:`, or a backticked ref and `: "`, or with a quoted sentence, for
  example `Reauthentication failed. cannot prompt during non-interactive
  execution`. Such a source now has no `title` and is reported as
  `source-title-unnamed`. The fix also covers the parts of a split entry.
  (#258)

  If you migrated a bundle with 0.6.0, run 0.7.0's plan and apply on it. A
  title that holds its whole entry is cut to the work's name or removed, which
  clears most excerpt titles. If no footnote holds a title's text, the title
  is kept and reported as `source-title-unpreserved`. A re-run leaves alone a
  title that is the excerpt by itself. Reverting the 0.6.0 migration commit
  and migrating fresh gives the cleanest result. If you re-run instead, check
  any `title` that reads like a description, an error message, or a sentence.
- The okfctl.dev link-preview image URL now changes when the card changes, so
  a stale cached card is no longer shown. (#256)

## [0.6.0] - 2026-11-27

This release fixes a data-loss bug: `index build` and the node verbs no longer
delete or overwrite an `index.md` that a person wrote. `migrate` now gives the
sources it converts ids, footnote definitions, and footnote references, and
builds links from DOI, arXiv, PubMed, and ISBN identifiers.

### Added

- `migrate` gives each source it converts from a `# Citations` entry a §5.1
  `id`, writes the entry's full text as the footnote definition `[^id]: …`
  just above `# Citations`, and rewrites the body's `[k]` references to
  `[^id]` (§4.2), expanding `[3]–[5]` and `[1, 2]`. The id is the entry key
  when it is a plain slug unique after case folding, else the ordered-list
  number, else `cite-<n>`; a number-looking id is quoted. A node whose ids
  would collide is refused (`source-id-collision`), a `[k]` near "brief",
  "citation", "node", or a link is left for review (`citation-ref-unclear`),
  and a second entry for one resource gets no footnote
  (`citation-unattached`). The plan summary gains an `attribution:` line.
- `migrate` builds a source's `resource` from a DOI, arXiv id, PubMed id, or
  ISBN when a v0.1 `# Citations` entry names no URL: `doi.org`, `arxiv.org`,
  `pubmed.ncbi.nlm.nih.gov`, or Open Library for books. It works offline from a
  fixed pattern and fetches nothing. An ISBN must pass its check digit, an
  entry with two identifiers stays a judgment item, and a DOI that the node's
  `sources` already hold (in any letter case, or under `dx.doi.org`) is not
  added again.

### Changed

- `migrate` titles a source made from a `# Citations` entry with the cited
  work's name only (§5.1 "a label for humans"), never the entry's full text.
  An entry that marks no name leaves the title unset and reports
  `source-title-unnamed`. This applies to sources built from an identifier
  too. When the entry's resource is already in `sources` (a DOI matched in any
  spelling), an untitled source gets the name, and a source with a different
  title keeps it and reports `source-title-conflict`.
- Every command's `--help` now opens with a real run, and the corrections
  it makes are checked against the binary (for example, `node rm` no longer
  claims `lint` reports links to the removed node). The generated command
  reference, `docs/commands/README.md`, carries the same examples, and CI
  replays them. (#243, #246, #251)
- The README, install guide, concepts page, guides, shipped skills, plugin
  description, and `llms.txt` are rewritten to show a real run before they
  explain it. CI replays every `$ okfctl` transcript in `README.md` and
  `docs/` against the built binary, so shown output can't drift. (#236, #237,
  #241, #242, #244, #245, #249)

### Fixed

- `index build`, and every node verb that rebuilds the index, no longer
  rewrites or deletes a hand-written `index.md`. Before, a curator's text in an
  `index.md` was destroyed with exit 0 and no message. okfctl now rewrites or
  prunes an `index.md` only when every line is one okfctl writes (§8), and
  keeps anything else byte for byte. `index build` names each kept file on
  stderr and exits 1; `--overwrite-hand-written` replaces them and still keeps
  the bundle-root frontmatter and H1 (§8.1). The node verbs keep the file,
  print the same warning, and still succeed. `index check` notes when the file
  it reports is hand-written. (#253)
- A bare CR before a line ending in frontmatter (`0: 0\r\r\n 0`) now counts as
  its own line break. The frontmatter splitter fused the CR and the CRLF into
  one break, so the value read as `"0 0"`; YAML 1.2 §5.4 counts two breaks,
  which makes it `"0\n0"`. Parsing and the frontmatter editor now agree on
  such a file. (#254)
- okfctl.dev publishes the install page, which returned 404, and the
  link-preview card no longer claims okfctl is a single binary. (#247, #250)
- `migrate` and `analyze` no longer split a `# Citations` entry at a
  hard-wrapped line that begins with a back-reference (`[9]: C4 provides …`,
  `[4]).`, `[21].`) or a bracket tag. The split wrote a fragment as a §5.1
  `title` and a bogus judgment item. Inside an open entry, an unbulleted
  `[key]` line now starts a new entry only when its key is the next citation
  key in sequence (`[2]` after `[1]`, `[1b]` after `[1a]`), so tight lists
  and lettered sub-entries still split. A run of link reference definitions
  (`[label]: url`) still yields one entry each.

## [0.5.0] - 2026-11-27

okfctl now targets **OKF v0.3**. New bundles declare `okf_version: "0.3"` on
the bundle-root `index.md`, `migrate` brings v0.1 and v0.2 bundles forward, and
a bundle that declares 0.2 or earlier, or nothing, keeps the §12.1 fallbacks, so
existing bundles read the same as before.

### Added

- Links and path fields resolve the way OKF v0.3 §6 says. A value with a URI
  scheme is a URL, a leading `/` resolves from the bundle root, and anything
  else resolves from the directory of the file that holds it. A value that
  climbs above the root is external and is never read. One resolver serves
  every command: the link graph, orphan counts, `lint broken-link`, `analyze`
  dangling links, `eval` citations, and the `node mv`/`node promote` rewrites.
  The §6.2 path fields (`sources[].resource`, `superseded_by`, and the rest)
  now get `broken-link` and dangling-link checks too, and a path field that
  points at a concept is a graph edge (§6.4). In a bundle that declares 0.2 or
  earlier, or no version, a bare value that doesn't resolve from its file
  falls back to the bundle root (§12.1). `okf_version` is read from the
  bundle-root `index.md` (§8.1), then from a legacy `.okf` sidecar; `bundle
  info` names where it came from, and `validate` warns when the two disagree.
- `migrate --apply` prints a hint after an apply that edits files: list the
  commit that lands it in `.okf-drift-ignore-revs`, so `validate --strict`
  doesn't report drift on every mechanically edited file.
- `lint` reports `tag-hygiene` beside `type-hygiene`: two or more `tags`
  spellings that fold to one form (case, separators, and a plural that folds
  only onto a singular the bundle already uses), listed with per-node counts,
  most common first. `analyze` clusters tags with the same fold.
- `index build` adds a shape suffix to each subdirectory entry: the child's
  concept count, its types, its shared tags, and a subtree total when it nests
  deeper, so a reader can pick a branch without opening it. Tune it with
  `--shape-tag-min` and `--shape-tag-max`, or turn it off with `--no-shape`.
  (#163)
- The okfctl.dev homepage theme control cycles system, light, and dark, and
  shares its setting with the docs pages.
- `migrate` upgrades a v0.2 bundle (or one that declares no version) to OKF
  v0.3. A bare link or path field that resolves only from the bundle root (the
  §12.1 fallback) is rewritten with a leading `/` so it resolves to the same
  file under v0.3 (§6). A §5 timestamp that carries an offset but is not
  RFC 3339 (`2026-06-20 10:00:00Z`) is respelled with the offset kept. A
  date-time with no offset names no instant, so it becomes a new
  `timestamp-no-offset` judgment item and is never guessed. The v0.1 renames
  still apply in the same run, and a second run plans nothing.
- `migrate` converts a v0.1 `# Citations` entry that names exactly one
  follow-able resource (an absolute URL or a bundle path, anywhere in the entry,
  across wrapped lines) into a §5.1 `sources` entry, with the entry text as its
  `title`. An entry with no resource, or with two different ones, stays a
  `prose-citation` judgment item. On the 356-node reference corpus this turns
  2,506 of 3,796 citation entries into sources (2,411 external URLs); before, a
  URL had to lead the entry, so 2,405 cited URLs stayed prose.
- `migrate --force-declare` declares v0.3 even while `prose-citation` or
  `missing-actor` items remain; the plan records `"force_declare": true`.
- `migrate --apply --dry-run` prints the unified diff the real apply would
  write, and both phases print a summary of edit and judgment counts by kind.

- `lint` applies the OKF v0.3 §5 frontmatter semantics through three new
  advisory checks. `timestamp-form` reports a §5 timestamp key that is not an
  RFC 3339 date-time with an offset or a full-date. That includes a date-time
  without an offset, quoted or YAML-native. `verified-event` reports a
  `verified` event missing `by` or `at`. `superseded-by` reports a
  `superseded_by` that resolves to no concept, or that is set on a concept that
  is not `deprecated`. It resolves through the shared §6 resolver; a stale path
  that `broken-link` already reports is not reported twice. None of them fails
  `validate`: v0.3 §11 keeps such a bundle conformant.
- `analyze` reports a §5.4 lifecycle section: the raw `status` distribution, and
  the concepts that a filter to current knowledge excludes (any status that is
  not stable, an unknown value included).
- A knowledge bundle can now reference a separate type-template bundle instead of
  vendoring one. Declare it with a `templates: <path>` key in the bundle's `.okf`
  sidecar (resolved relative to the bundle root), or pass `--templates-from <dir>`
  to `validate`, `node new`, `template list`, and `template show` (the flag
  overrides the sidecar key). Referenced templates are folded in and local `Type
  Template` nodes overlay them (local wins per `target_type`); `template list` and
  `template show` report each template's `source`, and `template list` flags a
  local entry that shadows a referenced one. The referenced bundle is never part
  of the consumer's node set and is never written, so `validate`, `lint`, `index
  build`, and `index check` are byte-identical with or without the key; a
  reference that cannot be loaded is a hard error naming the path (plain
  `validate` without `--templates` never consults the key). (#182)

### Changed

- okfctl targets OKF v0.3 (`SpecVersion` is `0.3`). `bundle init` declares
  `okf_version: "0.3"` in the bundle-root `index.md` frontmatter (§8.1) and no
  longer writes a `.okf` sidecar. A bundle that declares no version keeps the
  §12.1 fallbacks, which key on the declared version, and `bundle info` reports
  it as `okf_version: undeclared (v0.2 fallbacks apply, §12.1)`.
- `migrate --apply` declares the target version on the bundle-root `index.md`
  even when the index had no `okf_version` (§8.1: producers SHOULD put one in
  each bundle); before, it only rewrote an existing marker. An existing block
  keeps every other byte. A legacy `.okf` sidecar is bumped with the value
  quoted (§4).
- `migrate` holds the v0.3 declaration while the plan leaves a `prose-citation`
  or `missing-actor` item. Those items are readable only through the v0.1
  fallbacks (Appendix B), which §12.1 grants only to a bundle that declares 0.2
  or earlier, or nothing; declaring 0.3 first would make a v0.3 consumer drop
  them. The plan carries `"version_action": "hold"`, the summary says why, and
  apply writes the content edits but leaves the version markers alone.
- `migrate` never rewrites a bundle that declares a later version, or one
  okfctl does not recognize: the plan carries `"version_action": "keep"` and
  no edits (§12: consume best-effort, never refuse, never downgrade). The plan
  file gains `declared_version` and `version_action`; a plan without
  `version_action` is read as a bump.
- The trust tier counts only *current* verification events: an event whose
  `at` is equal to or later than `generated.at`, or any event when `generated`
  is absent (OKF v0.3 §5.3, a breaking change from v0.2). A human sign-off made
  before the last content change no longer makes a concept `human-reviewed`.
  The tier can only go down, which is the safe direction.
- A `verified` event without both `by` and `at` is ignored (v0.3 §5.2).
- `status-lifecycle` states the v0.3 §5.4 meaning of an unknown status (not
  stable, excluded from current knowledge) and no longer calls the value a
  defect.
- The docs, the okfctl.dev site, and `llms.txt` cite the OKF spec at its new
  home, [`open-knowledge-format/open-knowledge-format`](https://github.com/open-knowledge-format/open-knowledge-format/blob/main/SPEC.md).
  The `GoogleCloudPlatform/open-knowledge-format` repo and the
  `knowledge-catalog` copy are frozen at v0.2.
- okfctl stamps `created` and `modified` in the author's local offset
  (`2026-10-09T22:30:00-04:00`) instead of UTC, and the drift check reads
  `modified` in its own offset. A node written in the evening no longer
  records tomorrow's UTC date and then fails its own `validate --strict`.

### Deprecated

- `analyze --json` keys `freshness.stale` and `summary.stale_threshold_days`
  still carry the age-heuristic values under their old names for one release;
  read `freshness.aging` and `summary.aging_threshold_days` instead. The
  `--stale-days` flag is a deprecated alias of `--aging-days`.

### Fixed

- Re-applying a `migrate` plan file to a tree it already migrated no longer
  appends every `sources` entry a second time. Each frontmatter edit now
  re-checks the current file first: a resource already listed is not added
  again, and a `timestamp` already renamed to `generated` is left alone, so the
  second apply changes nothing.
- `node refresh`, `node edit`, and `migrate --apply` now splice the frontmatter
  entry they change instead of re-encoding the whole block (OKF v0.3 §4: a
  producer SHOULD NOT re-serialize values it did not change). A one-field
  refresh is a one-line diff: alignment padding, blank-line grouping, list
  indentation, comments, quoting, CRLF line endings, and unknown or `x-` keys
  (§4.1) all keep their bytes. A newly added key is appended as one line, and a
  quoted value keeps its quotes. A frontmatter shape the splicer does not model
  falls back to the previous re-encoding writer, so no edit that worked before
  can break. (#146)

- `analyze` now reports staleness only from `stale_after` (OKF v0.3 §5.5,
  §11.2). A new "Stale" section and `freshness.stale_after` JSON list name each
  concept whose `stale_after` has passed, plus each concept whose `stale_after`
  does not parse, flagged `malformed` (an unparseable value is never fresh). The
  old age-based list no longer claims staleness: it is now the "Aging" advisory
  (`freshness.aging`, `summary.aging_threshold_days`, `--aging-days`).
  `Node.IsStale` now reports an unparseable `stale_after` as not fresh, and a
  null or empty value as absent.
- `validate` now accepts OKF v0.3 §8.1 bundle metadata on the bundle-root
  `index.md`: `title`, `description`, `license`, and extension keys (§4.1) next
  to `okf_version`. Every key is optional, so a root block without
  `okf_version` (or an empty one) also passes. A concept key the spec defines,
  such as `type` or `tags`, is still flagged there, and a non-root `index.md`
  still carries no frontmatter (§8). `index build` now keeps an existing root
  frontmatter block byte-for-byte instead of rewriting it to `okf_version` only
  (§8.1 MUST keep; §4 no re-serialization).
- Concept frontmatter now follows the OKF v0.3 §11.2 consumer rules for empty
  and single-item values, applied once at load so every command agrees. A null,
  empty-string, or empty-list value reads as an absent key, so `status:` with no
  value means `stable` instead of a `status-lifecycle` finding on `"<nil>"`. A
  single item in a list field (`tags`, `sources`, `verified`, `parameters`)
  reads as a one-item list, so `sources: { resource: ... }` now counts as a
  citation instead of leaving the node uncited. `migrate` extends a null or
  single-mapping `sources` in place rather than writing a duplicate key.
- `lint` reports a new `source-missing-resource` finding for each `sources`
  entry with no `resource`. The entry is still ignored as provenance, but no
  longer silently (§11.2 rule 3). Like every lint finding it fails only under
  `--strict`; `validate` is unaffected.
- okfctl now finds concepts the way OKF v0.3 §3.2 says. A `.md` file with no
  frontmatter or no `type` (a `README.md`, an `AGENTS.md`) is not a concept: it
  no longer fails `validate`, which prints a warning for it and exits 0, and it is
  no longer a graph, lint, index, or search subject. Links and `sources` that
  point at such a file still resolve, and `node mv` keeps their links working.
  Files and directories whose names start with `.` (`.draft.md`, `.hidden/`) are
  not part of the bundle: they are never read, with or without `--no-ignore`, and
  `node new`/`node mv` refuse to write a concept there. A frontmatter block that
  does not parse still fails `validate` (§11.1). `node edit` refuses an edit that
  leaves the node without a `type`, because the node would silently stop being a
  concept.
- `validate` now enforces OKF v0.2 §9 on every reserved `log.md` at any depth:
  each `## ` date heading must be a valid ISO-8601 `YYYY-MM-DD` (rejecting
  impossible dates like `2025-13-45` and non-dates like `last Tuesday`),
  consecutive headings must be newest-first, a log with entries must carry at
  least one date heading, a log must open with a `# ` title (a frontmatter block
  is rejected), and legacy inline-dated bullets are flagged for regrouping. An
  empty log, a title-only log, and the `bundle init` scaffold stay conformant.
  Also closes a §8 gap where a non-root `index.md` with an empty frontmatter
  block (`---` immediately followed by `---`) slipped through. (#176)
- `log append` now writes §9 date-grouped entries — `## <today>` headings with
  `* <message>` items, newest-first — instead of inline-dated bullets, and
  preserves any existing `# ` title rather than stacking a second `# Change Log`
  header over it. On write it regroups pre-existing legacy `- YYYY-MM-DD — msg`
  bullets under their date headings (merging into an existing group, idempotent
  and lossless). Every node-mutation command that records to the log inherits the
  §9 shape. Together with the `validate` change this holds the invariant that
  `okfctl validate` never rejects a log okfctl wrote. (#177)
- okfctl refuses to read a symlink that resolves outside the bundle root (OKF
  v0.3 §3.2, §11.2). Before, `Load` followed a symlinked `.md` file, so a
  bundle, including one fetched with `connect`, could pull any local file into
  the graph, the search index, `serve`, and `okfctl-api`. `validate` prints a
  warning for each skipped path and still exits 0. The commands that open one
  bundle file directly (`log`, `index`, the `migrate` version bumps, the
  drift-ignore file) refuse an escaping symlink too.
- The broken-link "did you mean" hint writes each candidate in a form that
  resolves from the linking file (`/`-rooted, else relative), so in a v0.3
  bundle it no longer suggests the bare value that just failed to resolve.
- `analyze` counts body `# Citations` entries alongside `sources` in a bundle
  that declares 0.2 or earlier, or no version (§12.1), skipping entries that a
  `sources` entry already covers. A node that `migrate` left half converted no
  longer reads as single-citation. A v0.3 bundle counts `sources` only.
- `analyze` counts label-style citation markers (`- **[VERIFIED — cloned]**`)
  as citations, not only bare keys like `[1]`, and skips a line-start Markdown
  link in `# Citations`, which is navigation rather than a source.
- `migrate` keeps a hard-wrapped `# Citations` entry whole. A continuation line
  that happened to start with `2025.` or a bracket tag split one entry into a
  bogus source and a bogus judgment item.
- The v0.1 `timestamp` → `generated.at` rename keeps the time of day and the
  offset of an unquoted timestamp. It used to cut it to a bare date. (#171)
- `index build` keeps a hand-written subdirectory description
  (`* [Sub](sub/) - text`) across builds, and `index check` no longer reports
  such an index as out of date. (#173)
- `index build` and `index check` on a subdirectory keep that directory's
  existing H1 instead of stamping `# Knowledge Base` over it, which also made a
  correct sub-scope index read as stale.
- `index` and `log` handle a CRLF `index.md` or `log.md` (Windows with
  `core.autocrlf=true`): they normalize line endings on read and keep CRLF on
  write. A CRLF index no longer reads as stale, `index build` no longer doubles
  a shape suffix or rewrites every line, and `log append` no longer leaves a
  mixed-ending file. (#185)
- `connect` fast-forwards only a destination that's itself a checkout of the
  same source. Before, any directory inside any repo passed, so it could run
  `git pull --ff-only` on an unrelated enclosing repo. It also accepts Windows
  drive-letter and UNC paths as sources. (#193)
- `install.sh` reads the latest tag from the `github.com/cwest/okfctl/releases/latest`
  redirect instead of the GitHub API, so shared CI runners and NAT'd networks
  no longer hit the unauthenticated rate limit and fail with "could not
  determine the latest version". It falls back to the API (with
  `GITHUB_TOKEN` when set) only when the redirect names no tag.

### Performance

- The `validate` drift check reads git history once per run instead of running
  `git log` per node: one `git log` for the commit graph and one
  `git diff-tree --stdin` for the changed paths, replayed with the same history
  simplification `git log -- <path>` uses. On the 356-node reference corpus,
  `validate --strict` went from 33 s to under 1 s with identical output. It
  falls back to the per-path answer when git config sets `log.follow` or
  `log.showSignature`, for a pathspec-magic path, or when the batched read
  fails.

### Build

- The Go toolchain was bumped from 1.26.10 to 1.26.11 to pick up the patched
  `net/http` standard-library fix for GO-2026-6859 (an HTTP/1 server crash from a
  `MaxBytesReader` header race), clearing `govulncheck` in CI.
- The Go toolchain was bumped from 1.26.6 to 1.26.10 to pick up patched
  standard-library fixes for disclosed vulnerabilities (GO-2026-6603, -6607,
  -6608, -6611, -6612, -6613, -6617, and the crypto/x509 certificate-verification
  fix GO-2026-6809), clearing `govulncheck` in CI.

## [0.4.0] - 2026-08-19

### Added

- Agent Plugins 1.0.0 packaging: a root `plugin.json` that lets a compatible
  agent client install okfctl's shareable skills as one unit. The manifest is
  spec-only plus `AGENTS.md`, pins the `$schema` to the Agent Plugins 1.0.0
  schema URL, and keeps okfctl-specific config under `extensions."dev.okfctl"`.
  CI validates it against the pinned schema, rejects a deliberately malformed
  fixture, and runs the skill-leak gate over the packaged payload.
- A skill-command contract check that installs the released binary from the
  public install path and runs every command the shipped skills document, so a
  skill drifting out of sync with the CLI fails CI even when the manifest stays
  byte-identical.

### Changed

- User-facing prose across the README, the hand-written docs, and the command
  help text was rewritten in a plainer voice, and the README now leads with the
  problem okfctl solves rather than a command list.
- House prose style is the closed-up Chicago em dash (`word—word`); the shipped
  Markdown surfaces were converted to it.

### Fixed

- Homepage inline-boundary spacing and the brand wordmark render correctly.
- The retired Go Report Card badge was dropped from the README.

### Build

- A CI prose gate scans the shipped Markdown surfaces and the built site for a
  negative-listing slop cadence and for spaced em dashes, failing the build on
  either. The gate rides one detector set across both an HTML and a Markdown
  front end, and both of its controls are proven in CI.
- The release path now refuses to publish a tag whose version has no matching,
  dated, non-empty `## [X.Y.Z]` section in this changelog. The check runs before
  goreleaser, so an empty release can never ship.

## [0.3.1] - 2026-08-13

### Fixed

- The published documentation site rebuilds from `main` after a release, so the
  live docs no longer lag a freshly tagged version.

### Build

- The Go toolchain was bumped to 1.26.6 to pick up patched standard-library
  fixes.

## [0.3.0] - 2026-08-13

### Added

- **Installable, verifiable distribution.** A one-line installer served at
  `https://okfctl.dev/install.sh` detects OS/arch, downloads the matching
  release archive, verifies it against the published `checksums.txt`, and
  installs both `okfctl` and `okfctl-search`, refusing to install on a checksum
  mismatch. A Homebrew cask (`brew install cwest/tap/okfctl`) is published to
  `cwest/homebrew-tap`. Releases now include Windows builds (amd64 and arm64) as
  `.zip` archives and Linux system packages (`.deb` and `.rpm`) via nfpm.
- **Supply-chain provenance.** Every archive ships a Software Bill of Materials
  generated with syft, and release artifacts are signed keyless with cosign
  (Sigstore/Fulcio) using the release workflow's OIDC identity, verifiable with
  `cosign verify-blob`.
- **`GET /api/v1/search`**—the `okfctl-api` plugin serves search off a resident
  index, with the lexical gate exposed as a query parameter, unknown parameters
  rejected, and the recency-decay floor applied. The API is v0.2-aware,
  surfacing `status`, `epistemic`, and `generated.at`.
- **Query scoping for search**—repeatable, negatable `--path` / `--type` /
  `--tag` (and `--not-*`) filters, a `--lexical-gate` that preserves lexical
  recall alongside semantic ranking, and bounded recency decay with a
  configurable relevance floor and half-life.
- **The okfctl.dev site**—a bespoke Astro site and its GitHub Pages pipeline,
  serving the documentation at clean URLs, with a social card, favicon,
  `robots.txt`, `llms.txt`, and a 404 page, shipped through the theming seam.
- **Generated command reference**—the command reference is generated from the
  cobra tree with a CI drift gate, and every command gained a long description
  and a runnable example.
- A TACA-style trustworthiness evaluation pass for knowledge-base nodes.
- Fuzz targets for the untrusted-input parsers.

### Changed

- **`validate`, `lint`, and `analyze` are OKF v0.2-aware**—they read the v0.2
  provenance families and resolve nested provenance in template
  `required_fields`, while keeping the v0.1 fallbacks.
- `okfctl version` from a `go install github.com/cwest/okfctl@latest` build now
  reports the installed module version instead of `dev`, via a
  `debug.ReadBuildInfo()` fallback. Release builds still report the injected tag,
  and a plain checkout build without usable module metadata still reports `dev`.
- `analyze` prefers `verified[].at` as the freshness basis.
- The README was rewritten as an on-ramp rather than a command dump, and the
  homepage copy had its AI-slop tells removed.
- CI pins its actions to commit SHAs, attests build provenance, runs the site
  test suite rather than only the site build, and re-enables `gosec` and
  `errcheck` with per-site triage.

### Fixed

- Release signing publishes correctly by using cosign's bundle format.
- Search orders the preserved lexical tail by score, and rejects an
  out-of-range `--decay-floor` or a negative `--half-life`.
- The documentation site publishes at clean URLs instead of `/_generated/`, and
  the 404 page and `llms.txt` point at the clean doc URLs.
- A v0.1 flat-string `sources` list is accepted for template `required_fields`.

## [0.2.0] - 2026-08-02

`okfctl` targets **OKF v0.2**. New bundles are created at `okf_version: 0.2`, and
v0.1 bundles remain readable—v0.2's two breaking renames (`timestamp` →
`generated.at`, body `# Citations` → frontmatter `sources`) are read with a v0.1
fallback, per spec §12's best-effort consumption rule.

### Added

- **`okf migrate`**—a two-phase v0.1 → v0.2 upgrade path: a mechanical pass plus
  a judgment queue for calls a tool should not make alone. Ships with the
  `okf-migrate-plan` skill.
- **`okfctl-api`**—an HTTP API plugin over a bundle, serving `/stats` and
  `/graph` (walking skeleton; ships with an ADR describing its shape).
- **Passage-level search indexing**—sub-node passages are indexed and returned
  as snippets, so a hit points at the paragraph rather than the file. Retrieval
  quality on the real corpus improved 0.545 → 0.909 (~12x index size).
- **Query scoping**—filter semantic queries by path, type, and tag, with
  optional recency decay.
- **`lint --json`**—machine-readable findings for CI and downstream tooling.
- **Bulk index promotion**—promote directory-as-concept `index.md` nodes in
  bulk.

### Changed

- **Provenance** reads the v0.2 `generated` / `verified` / `sources` families,
  falling back to the v0.1 forms when a bundle declares the older version.
- **Freshness** resolves through `generated.at` rather than filesystem
  `modified` / `created`, so a regenerated node reports its real age.
- **Prose-only scanning**—cross-reference and time-marker detection no longer
  reads link URLs as prose, eliminating a class of false `missing-xref` and
  time-sensitive findings (both inline and reference-style links excluded).
- **Title-as-home-node**—a node whose title leads with a term is credited as
  that term's home, closing a coverage-gap false positive.
- A model2vec directory containing `tokenizer.json` is now accepted for search.
- Section citations corrected throughout for the v0.1 (11 sections) → v0.2 (13
  sections) numbering shift.

### Fixed

- Broken internal links are gated as defects when they look like defects.
- Vendored and derived directories are skipped during the bundle walk.
- A bulk mechanical commit can declare itself as such and skip the git-drift
  check.

## [0.1.0] - 2026-07-27

Initial release: the `okfctl` command-line tool for authoring and maintaining
Open Knowledge Format (OKF) bundles, targeting OKF v0.1.

### Added

- **Bundle lifecycle**—`bundle init`, node authoring, and `validate` (the
  walking skeleton).
- **Reserved-file lifecycle**—generation and maintenance of the reserved
  `index.md` and `log.md` files, with nested per-directory indexes and
  dir-relative links (§6).
- **Node mutation verbs**—`node edit`, `node mv`, `node rm`, and `node refresh`
  (bulk-fix of stale timestamps).
- **Derived-artifact maintenance**—`created` / `modified` metadata, `log.md`,
  and `index.md` maintained by default, with git-drift findings.
- **Lint**—structural checks and semantic curation checks (`lint --semantic`);
  precision hardened on the real corpus (2,438 findings → 21).
- **Analyze**—`okfctl analyze`, a proactive curation report covering freshness
  and clusters.
- **Graph**—link-graph export and serve; DOT-pipe as the sanctioned SVG path.
- **Search**—core lexical + graph-structural search, plus the `okfctl-search`
  semantic plugin (Model2Vec safetensors loader, WordPiece tokenizer).
- **Plugins**—`okfctl-<name>` plugin dispatch over `PATH`, with `plugin
  install`.
- **Remote sources**—`registry` / `connect` for git-backed remote bundle
  sources.
- **Release mechanics**—goreleaser, version injection via ldflags, and
  installable binaries.
- **Documentation & conformance**—the PRD, ADRs, ship-with-the-tool skills
  (`okf-authoring`, `okf-curation-health`, `okf-semantic-search`), and a
  spec-conformance suite proving okfctl's output validates clean.

[Unreleased]: https://github.com/cwest/okfctl/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/cwest/okfctl/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/cwest/okfctl/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/cwest/okfctl/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/cwest/okfctl/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/cwest/okfctl/releases/tag/v0.4.0
[0.3.1]: https://github.com/cwest/okfctl/releases/tag/v0.3.1
[0.3.0]: https://github.com/cwest/okfctl/releases/tag/v0.3.0
[0.2.0]: https://github.com/cwest/okfctl/releases/tag/v0.2.0
[0.1.0]: https://github.com/cwest/okfctl/releases/tag/v0.1.0
