# Curation health: `lint`, `analyze`, and `--strict` in CI

`validate` tells you a bundle is legal OKF. It can't tell you a link points
nowhere, or that a node talks about tannin without linking to it, or that half
your nodes cite nothing. This guide runs okfctl's three curation checks on one
small bundle that has all of those problems, then shows which ones to put in
CI. Every block below is a real run.

## One bundle, three checks

The bundle has four nodes in two directories. Each one hides a defect a
reader would trip over:

```text
mykb/index.md
mykb/log.md
mykb/notes/body.md      epistemic: verifed (a typo), no sources
mykb/wine/acidity.md    type: reference (lowercase), status: retired
mykb/wine/legs.md       links to body.md, which isn't in wine/
mykb/wine/oak.md        mentions acidity without linking to it
```

<!-- transcript: setup
$ cp -R "$TRANSCRIPT_FIXTURES/curation-health/mykb" mykb
-->
```text
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
$ okfctl lint mykb
type-hygiene: near-duplicate type values likely refer to one type: Reference, reference
status-lifecycle: wine/acidity.md has status "retired", which §5.4 does not define (draft|stable|deprecated); per §5.4 the concept is not stable and filters to current knowledge exclude it. If this is an epistemic grade, move it to `epistemic`
broken-link: wine/legs.md links to body.md which resolves to no node; did you mean /notes/body.md?
missing-xref: wine/legs.md mentions "Body" but does not link to notes/body.md
timestamp-form: wine/legs.md generated.at = "2026-06-20 10:00:00" is not a §5 timestamp (an RFC 3339 date-time with an offset, e.g. "2026-06-30T14:00:00Z", or a full-date, e.g. "2026-06-30"); a date-time without an offset is not valid
missing-xref: wine/oak.md mentions "Acidity" but does not link to wine/acidity.md
6 lint finding(s)
```

`validate` passes, and it's right to. OKF v0.3 says a consumer MUST NOT reject
a bundle for broken links or unknown values (§11.2), and every defect here is
one of those. `lint` reads past the floor and reports six findings. It exits 0
anyway. Here's the same run with `--strict`:

<!-- transcript: exit 1 okfctl lint --strict mykb -->
```text
$ okfctl lint --strict mykb
type-hygiene: near-duplicate type values likely refer to one type: Reference, reference
status-lifecycle: wine/acidity.md has status "retired", which §5.4 does not define (draft|stable|deprecated); per §5.4 the concept is not stable and filters to current knowledge exclude it. If this is an epistemic grade, move it to `epistemic`
broken-link: wine/legs.md links to body.md which resolves to no node; did you mean /notes/body.md?
missing-xref: wine/legs.md mentions "Body" but does not link to notes/body.md
timestamp-form: wine/legs.md generated.at = "2026-06-20 10:00:00" is not a §5 timestamp (an RFC 3339 date-time with an offset, e.g. "2026-06-30T14:00:00Z", or a full-date, e.g. "2026-06-30"); a date-time without an offset is not valid
missing-xref: wine/oak.md mentions "Acidity" but does not link to wine/acidity.md
6 lint finding(s)
okfctl: 6 lint finding(s)
```

Exit 1. That's the line to put in CI. `lint` never changes a file.

## Reading the `lint` findings

`broken-link` fired because `legs.md` lives in `wine/` and links to
`body.md`, so the link resolves to `wine/body.md`, which doesn't exist. Under
§6 a link with no leading `/` resolves from its file's own directory, and one
with a leading `/` resolves from the bundle root. The hint names a form that
works from where the link sits. In a bundle that declares 0.2 or earlier, or
no version, a bare link that only resolves from the root still counts as
resolved (§12.1). The same check covers path fields like `sources[].resource`
and `superseded_by` (§6.2).

`lint` reports a broken link only when a node with that file name exists
somewhere else, which marks it as a moved or mistyped path. A link to a node
nobody has written yet is a gap in coverage, not a defect. `lint` stays quiet
about it and `analyze` lists it, as you'll see below.

`missing-xref` fired twice: `oak.md` says "acidity" and `legs.md` says
"body", both without a link to the node of that title. The fix is a link.

`type-hygiene` caught `Reference` and `reference`, two spellings that
probably mean one type. OKF leaves type values open (§4.1), so `lint` never
says which spelling is right. It reports that two exist. `tag-hygiene` does
the same for `tags`. It folds case and separators (`run-book`, `run_book`,
`runbook`), and folds a plural onto its singular when the singular is already
a tag in the bundle. The fold is a guess, so read the finding before you
rename anything.

The other two are frontmatter OKF accepts but reads differently than you
probably meant:

- `status-lifecycle`: `retired` isn't one of `draft`, `stable`, or
  `deprecated`. §5.4 says a concept with any other status isn't stable, so a
  tool that filters to current knowledge drops it.
- `timestamp-form`: `2026-06-20 10:00:00` has no offset, so it names no
  instant (§5). `lint` checks `generated.at`, `verified[].at`, `stale_after`,
  `sources[].last_modified`, and `usage_window` this way. A `stale_after` that
  won't parse counts as stale.

This bundle doesn't trip four more checks:

- `orphan` is a node nothing links to. A directory's `index.md` counts as a
  link.
- `coverage-gap` is a term at least three nodes mention (`--coverage-threshold`)
  with no node of its own.
- `verified-event` is a `verified` entry missing `by` or `at`, which consumers
  ignore (§5.2).
- `superseded-by` is a `superseded_by` that resolves to nothing, or that sits
  on a concept that isn't `deprecated` (§5.4).

There are also `source-missing-resource`, a `sources` entry with no
`resource` (§5.1 requires one, and consumers skip the entry), and
`spec-version`, a node that declares its own `okf_spec_version` when the
version belongs once on the root `index.md` (§12).

## `eval transparency`: can a reader tell where this came from?

`lint` checks structure. `eval transparency` checks provenance: does each node
carry a grade, and does it cite anything?

<!-- transcript: exit 1 okfctl eval transparency --strict mykb -->
```text
$ okfctl eval transparency mykb
grade-vocabulary: notes/body.md has epistemic: "verifed", an off-vocabulary value carried by only 1 node(s) (likely a typo/drift)
uncited: notes/body.md carries no citations (no sources, no # Citations, no internal reference)
grade-missing: wine/legs.md carries no epistemic or authority grade (provenance is invisible)
uncited: wine/legs.md carries no citations (no sources, no # Citations, no internal reference)
4 transparency finding(s)
$ okfctl eval transparency --strict mykb
grade-vocabulary: notes/body.md has epistemic: "verifed", an off-vocabulary value carried by only 1 node(s) (likely a typo/drift)
uncited: notes/body.md carries no citations (no sources, no # Citations, no internal reference)
grade-missing: wine/legs.md carries no epistemic or authority grade (provenance is invisible)
uncited: wine/legs.md carries no citations (no sources, no # Citations, no internal reference)
4 transparency finding(s)
okfctl: 4 transparency finding(s)
```

Oak and acidity each carry `epistemic: verified` and a source, so they pass.
Legs has no grade and no source. Body has a source problem and a typo.

okfctl has no list of allowed grades, because OKF doesn't define one. It
learns the list from your bundle. A value at least two nodes carry
(`--grade-vocabulary-floor`, default 2) counts as vocabulary. Here `verified`
appears twice, so the lone `verifed` stands out. The fourth check,
`citation-unresolved`, fires when an internal citation points at no node.

`eval transparency` doesn't follow `http(s)` links, since that needs the
network and okfctl runs offline. Like `lint`, it exits 0 unless you pass
`--strict`, and `--json` gives you the findings for a script.

This is the T in TACA, which grades a node on Transparency, Accuracy,
Calibration, and Alignment. Only Transparency can be checked without a model
or the network. For the other three, `eval sample` builds a worksheet:

```text
$ okfctl eval sample --sample 1 --format md mykb | head -9
# TACA spot-check worksheet

_okfctl extracted the context below; fill in each empty slot. okfctl does not judge truth — you (or an LLM judge) do._

## wine/acidity.md

- **Title:** Acidity
- **Grade:** epistemic=verified authority=— trust-tier=unverified
- **Sources:** https://en.wikipedia.org/wiki/Acids_in_wine
```

Below that come three sections, one each for Accuracy, Alignment, and
Calibration, with a `(fill in)` slot for each answer. okfctl fills in what it
can read from the node and leaves the judgment blank for you or a model. It
never decides whether a claim is true. The sample is
seeded, so the same command picks the same nodes every time (`--seed` changes
it). In a pre-merge job, `--changed-since origin/main` builds the worksheet
for the nodes a branch touched instead.

## `analyze`: where is the bundle thin?

`lint` asks whether anything is broken. `analyze` asks where the bundle is
weak, and it always exits 0:

```text
$ okfctl analyze mykb
# OKF Corpus Analysis

4 node(s), 0 internal link(s). Aging threshold: 180d.

## Coverage gaps — dangling forward-links (research opportunities)
  - wine/legs.md → missing: body.md

## Coverage gaps — thin nodes (need expansion)
  - notes/body.md (2 lines)
  - wine/acidity.md (2 lines)
  - wine/legs.md (2 lines)
  - wine/oak.md (2 lines)

## Coverage gaps — uncited nodes (need sourcing)
  - notes/body.md
  - wine/legs.md

## Coverage gaps — single-citation nodes (corroborate)
  - wine/acidity.md
  - wine/oak.md

## Coverage gaps — known terms with no node (delegated to lint)
  ✓ none

## Epistemic grade distribution (observational — not a gate)
  - verified: 2
  - verifed: 1
  (1 node(s) with no epistemic key)

## Lifecycle — status distribution (observational — not a gate)
  - stable: 3
  - retired: 1 (not a §5.4 value; not stable)
  (1 node(s) excluded from current knowledge: status not stable, §5.4)
  - wine/acidity.md (status: retired)

## Stale — past stale_after or malformed stale_after (§5.5)
  ✓ none

## Aging — not reviewed in 180 days, or undated (advisory, not staleness)
  - notes/body.md (age: n/a, basis: (none))
  - wine/acidity.md (age: n/a, basis: (none))
  - wine/legs.md (age: n/a, basis: (none))
  - wine/oak.md (age: n/a, basis: (none))

## Time-sensitive — marked bodies aged past the gate (re-verify claims)
  ✓ none

## Connectivity — orphan nodes (wire into the graph)
  ✓ none

## Connectivity — weakly-linked nodes
  - notes/body.md (out:0 in:1)
  - wine/acidity.md (out:0 in:1)
  - wine/legs.md (out:0 in:1)
  - wine/oak.md (out:0 in:1)

## Clusters — synthesis-overview candidates
  ✓ none

## Structure — duplicate/near-duplicate titles (merge candidates)
  ✓ none

## Structure — near-duplicate slugs (rename/merge candidates)
  ✓ none

---
17 actionable signal(s) across all dimensions.
```

Some of this repeats what the other checks said from a different angle: the
dangling link is `lint`'s broken link, and the uncited nodes are
`eval`'s. The rest is new. Every node is under 15 lines (`--thin-lines`).
Two cite only one source. None says when it was last reviewed, so all four
show up as undated under Aging (`--aging-days`, default 180). Each one has a
single inbound link, and that link is its directory's index.

Nothing here is a gate. It's a to-do list for the next time you sit down with
the bundle.

## Put it in CI

Four commands cover the bundle, and each one exits 1 when it finds something:

```sh
okfctl validate --strict ./mykb              # spec floor, plus git drift
okfctl index check ./mykb                    # every index.md is current
okfctl lint --strict ./mykb                  # broken links, hygiene, §5 frontmatter
okfctl eval transparency --strict ./mykb     # every node graded and cited
```

The [index and freshness guide](index-and-freshness.md) explains the first
two.

## Semantic lint (`lint --semantic`)

Structural lint asks whether anything links to a node. Semantic lint asks
whether anything is even about the same thing, which the link graph alone
can't answer. It adds two checks:

| check | finding |
|---|---|
| `similar-unlinked` | two nodes scoring at or above `--similarity-threshold` (default `0.80`) with no link in either direction |
| `no-semantic-neighbors` | a node whose best neighbor scores below `--isolation-floor` (default `0.20`) |

`okfctl lint` doesn't compute embeddings. It reads an index that the bundled
`okfctl-search` plugin builds, so linting needs no model. Without that index,
`--semantic` fails rather than quietly running the structural checks alone:

<!-- transcript: exit 1 okfctl lint --semantic mykb -->
```text
$ okfctl lint --semantic mykb
okfctl: no semantic index at mykb/.okfctl/index.db: run 'okfctl-search index build mykb' first
```

A silent fallback would let CI believe the semantic checks ran. Build the
index, then lint:

```sh
okfctl-search index build ./mykb     # the plugin builds the index
okfctl lint --semantic ./mykb        # core reads it
```

Without `--semantic`, `lint` never reads the index. With it, nodes added since
the last `index build` show up as one `stale-index` finding that lists them,
so a partial pass never looks like a clean one.

The findings are only as good as the embedder that built the index. With the
default `hash` embedder, `similar-unlinked` means two nodes share vocabulary.
With `--embedder model2vec` it means they cover related subject matter. Use
`model2vec` if you plan to act on the findings. The
[search guide](search.md) covers embedder setup.

## What every walk skips

Every command that walks a bundle (`validate`, `lint`, `analyze`, `search`,
`graph export`, `index build` and `index check`) skips some files.

OKF itself excludes two kinds (§3.2). A file or directory whose name starts
with `.` isn't part of the bundle, so `.git`, `.venv`, and `.drafts/` are
never read. A `.md` file with no frontmatter or no `type`, like a `README.md`
or `AGENTS.md`, isn't a concept. It's left out of the graph, lint, the index,
and search, and `validate` prints a warning about it without failing.

okfctl also skips directories that hold Markdown nobody wrote as knowledge:
`node_modules`, `venv`, `env`, `__pycache__`, `site-packages`, `vendor`,
`target`, `dist`, and `build`, matched by name at any depth below the root.
Here both kinds turn up in a copy of the bundle:

<!-- transcript: setup
$ cp -R mykb skipped
$ mkdir -p skipped/node_modules/pkg
$ cp skipped/wine/oak.md skipped/node_modules/pkg/oak.md
$ echo '# Scratch notes' > skipped/README.md
-->
```text
$ okfctl validate skipped
note: skipped 1 vendored/derived directory (node_modules); pass --no-ignore to include them
warning README.md: not a concept (no frontmatter); skipped, does not affect conformance (OKF v0.3 §3.2)
OK: bundle conforms to the OKF spec floor
```

The skip is never silent. The note goes to stderr and names what was skipped.
If you really did write knowledge in a directory called `build`, pass
`--no-ignore` and okfctl walks everything except the dot directories, which
the spec rules out. okfctl doesn't read `.gitignore`, since what you version
and what you curate are different questions, and there's no config file to
maintain.
