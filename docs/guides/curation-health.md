# Curation health: `lint`, `analyze`, and `--strict` in CI

`validate` tells you if a bundle is legal OKF. It won't tell you that a link
points nowhere. Nor will it notice a node that talks about tannin with no link
to it. Half your nodes could cite nothing and it would still pass. Here okfctl's three curation checks run on one four-node
bundle that has all of those problems. Then you'll see which of them to put in
CI.

## One bundle, three checks

Each of the four nodes hides a defect a reader would trip over:

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

`validate` passed. It was right to. OKF v0.3 §11.2 forbids a consumer to
reject a bundle over a broken link. A `status` outside the defined vocabulary,
like `retired`, is kept as written. The bundle stays conformant (§11.2,
rule 4). The `type` spellings, the missing cross-links, and the timestamp with
no offset are curation problems `validate` doesn't look for. `lint` looks past
the floor and finds all six. It still exits
0. Add `--strict` to change that:

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

That one exits 1. It's the line you want in CI. Neither form of `lint` ever changes a file.

## Reading the `lint` findings

`legs.md` lives in `wine/` and links to `body.md`. The link resolves to
`wine/body.md`. No such file exists, hence `broken-link`. Under §6 a link with
no leading `/` resolves from the directory its file is in. A leading `/` makes
it resolve from the bundle root. The hint names a form that works from where
the link sits. In a bundle that declares 0.2 or older (or declares no version
at all) a bare link that only resolves from the root still counts as resolved
(§12.1). Path fields get the same check, like `sources[].resource` and
`superseded_by` (§6.2).

`lint` only calls a link broken if a node with that file name exists in some
other directory. It looks like a moved or mistyped path. A link to a node you
haven't written at all is missing coverage. `lint` says nothing about it.
`analyze` lists it (you'll see that below).

`missing-xref` fired twice. `oak.md` says "acidity" and `legs.md` says "body",
and neither links to the node with that title. Add the links.

`type-hygiene` caught `Reference` and `reference`, two spellings that
probably mean one type. OKF leaves type values open (§4.1). `lint` never picks
the right spelling. It reports that there are two. `tag-hygiene` does the
same for `tags`. It folds case and separators (`run-book`, `run_book`,
`runbook`) into one. It also folds a plural onto its singular if the singular
is itself a tag in the bundle. Those folds are guesses. Read the finding
and decide for yourself.

The last two are frontmatter OKF accepts but reads differently than you
probably meant:

- `status-lifecycle`: `retired` isn't `draft`, `stable`, or `deprecated`, the
  three values §5.4 defines. A concept with any other status isn't stable. A
  tool that filters to current knowledge drops it.
- `timestamp-form`: `2026-06-20 10:00:00` has no offset. It names no
  instant (§5). `lint` checks `generated.at`, `verified[].at`, `stale_after`,
  `sources[].last_modified`, and `usage_window` the same way. If `stale_after`
  won't parse, the node counts as stale.

This bundle doesn't trip six more checks:

- `orphan` is a node nothing links to. A directory's `index.md` counts as a
  link.
- `coverage-gap` is a term at least three nodes mention (`--coverage-threshold`)
  that has no node of its own.
- `verified-event` is a `verified` entry missing `by` or `at`. Consumers
  ignore those (§5.2).
- `superseded-by` is a `superseded_by` that resolves to nothing. It also fires
  when one sits on a concept that isn't `deprecated` (§5.4).
- `source-missing-resource` is a `sources` entry with no `resource`. §5.1
  asks for one, and consumers skip the entry.
- `spec-version` is a node with its own `okf_spec_version`. The version
  belongs only on the root `index.md` (§12).

## `eval transparency`: can a reader tell where this came from?

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

Where `lint` checks structure, `eval transparency` checks provenance. It wants
each node to carry a grade and to cite something. Oak and acidity carry
`epistemic: verified` plus a source. They pass. Legs has neither. Body has no
source and a misspelled grade.

OKF has no list of grades for okfctl to check against. It reads one out of
your bundle. Any value at least two nodes carry
(`--grade-vocabulary-floor`, default 2) is on the list. Two nodes here say
`verified`. The lone `verifed` sticks out. The fourth check,
`citation-unresolved`, fires on an in-bundle citation that points at no node.

`eval transparency` doesn't follow `http(s)` links. okfctl works
offline. Like `lint`, it exits 0 unless you pass `--strict`. Pass `--json` to
feed a script.

This is the T in TACA (Transparency, Accuracy, Calibration, and Alignment). It's
the only one of the four okfctl can check offline on its own. For
the other three, `eval sample` builds a worksheet:

```text
$ okfctl eval sample --sample 1 --format md mykb | head -9
# TACA spot-check worksheet

_okfctl extracted the context below; fill in each empty slot. okfctl does not judge truth — you (or an LLM judge) do._

## wine/acidity.md

- **Title:** Acidity
- **Grade:** epistemic=verified authority=— trust-tier=unverified
- **Sources:** https://en.wikipedia.org/wiki/Acids_in_wine
```

Below that it has an Accuracy section, an Alignment section, and a Calibration
section, each with `(fill in)` slots. okfctl copies in what it can read off the
node and leaves the judgment to you or to a model. It won't decide if a claim
is true. The sample is seeded. Run the same command twice and you get the
same nodes (`--seed` changes it). In a pre-merge job,
`--changed-since origin/main` builds the worksheet from the nodes the branch
touched.

## `analyze`: where is the bundle thin?

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

`analyze` always exits 0. Part of this repeats what you saw above (the dangling link is
`lint`'s broken link, the uncited nodes came from `eval`). The rest is news. All
four nodes are under 15 lines (`--thin-lines`), two cite one source apiece, and
none of them says when anybody last reviewed it. Aging lists all four as
undated (`--aging-days`, default 180). Each has one inbound link, from its directory's
index.

Nothing in this report fails a build. Read it as a to-do list for the next time
you sit down with the bundle.

## Put it in CI

```sh
okfctl validate --strict ./mykb              # spec floor, plus git drift
okfctl index check ./mykb                    # every index.md is current
okfctl lint --strict ./mykb                  # broken links, hygiene, §5 frontmatter
okfctl eval transparency --strict ./mykb     # every node graded and cited
```

Each of these exits 1 when it finds something. The
[index and freshness guide](index-and-freshness.md) explains the first two.

## Semantic lint (`lint --semantic`)

<!-- transcript: exit 1 okfctl lint --semantic mykb -->
```text
$ okfctl lint --semantic mykb
okfctl: no semantic index at mykb/.okfctl/index.db: run 'okfctl-search index build mykb' first
```

`okfctl lint` doesn't compute embeddings. It reads a semantic index (the
`.okfctl/index.db` file named in that error) that the bundled
`okfctl-search` plugin builds. Linting itself needs no model. With no index
to read, `--semantic` fails outright. Falling back to the structural checks
would let CI think the semantic ones had run. Build the index, then lint:

```sh
okfctl-search index build ./mykb     # the plugin builds the index
okfctl lint --semantic ./mykb        # core reads it
```

Structural lint asks if anything links to a node. Semantic lint asks
if anything is even about the same topic. The link graph has no answer for that.
The semantic pass adds two checks:

| check | finding |
|---|---|
| `similar-unlinked` | two nodes scoring at or above `--similarity-threshold` (default `0.80`) with no link in either direction |
| `no-semantic-neighbors` | a node whose best neighbor scores below `--isolation-floor` (default `0.20`) |

Leave off `--semantic` and `lint` never reads the index. Pass it, and any nodes
added since the last `index build` show up in one `stale-index` finding that
names them all. A partial pass can't look clean that way.

The findings are only as good as the embedder behind the index. With the
default `hash` embedder, `similar-unlinked` means two nodes use the same words.
With `--embedder model2vec` it means they're about related things. If you'll act on the findings, use
`model2vec`. The [search guide](search.md) has embedder setup.

## What every walk skips

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

That's a copy of the bundle with a `node_modules` directory and a `README.md`
added. Any command that walks a bundle (`validate`, `lint`, `analyze`,
`search`, `graph export`, `index build`, and `index check`) skips the two of them.

OKF itself leaves out two kinds of file (§3.2). A file or directory whose name
starts with `.` isn't part of the bundle. okfctl never reads `.git`. It skips
`.venv` and `.drafts/` too. A `.md` file with no frontmatter (or no `type`) isn't
a concept either. Most `README.md` and `AGENTS.md` files fall in that group.
They stay out of the graph, lint, the index, and search. `validate` warns about them and doesn't fail.

okfctl also skips directories that hold Markdown no one wrote as knowledge:
`node_modules`, `venv`, `env`, `__pycache__`, `site-packages`, `vendor`,
`target`, `dist`, and `build`. It matches them by name at any depth below the
root.

You'll always hear about a skip. The note goes to stderr and names what got
skipped. If you really did write knowledge in a directory called `build`, pass
`--no-ignore`. okfctl then walks everything but the dot directories (the spec
excludes those). It doesn't read `.gitignore`. What you version and
what you curate are different questions. There's no config file to maintain
either.
