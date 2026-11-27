# Curation health: `lint`, `analyze`, and `--strict` in CI

## `lint`—curation-health findings

`okfctl lint <dir>` reports curation-health findings (orphans, missing
cross-references, broken internal links, coverage gaps, type and tag hygiene,
and the OKF v0.3 §5 frontmatter checks below). It's **advisory by default**
(exits 0 even with findings); `--strict` exits nonzero on any finding, so wire
`lint --strict` into CI as a gate. `--coverage-threshold N` tunes the
coverage-gap check (default 3). `lint` never mutates the bundle.

`broken-link` covers body links and the §6.2 path fields (`sources[].resource`,
`superseded_by`, and the rest), resolved by the §6 rules: a leading `/` from the
bundle root, anything else from the file's own directory. In a bundle that
declares 0.2 or earlier, or no version, a bare value that only resolves from the
root still counts as resolved (§12.1). The "did you mean" hint names the
candidate in a form that resolves from the linking file.

`tag-hygiene` works like `type-hygiene` on `tags`: it reports spellings that fold
to one tag (`Wine`, `wine`, `wines`; `run-book`, `runbook`), with a per-node count
for each, most common first. A plural folds onto its singular only when the
singular is itself a tag in the bundle. The fold is a heuristic, so review a
finding before you normalize.

Four checks report frontmatter that OKF v0.3 still accepts but consumers read
differently than the author probably meant. None of them fails `validate`:

| check | finding |
|---|---|
| `timestamp-form` | a §5 timestamp (`generated.at`, `verified[].at`, `stale_after`, `sources[].last_modified`, `usage_window`) that isn't an RFC 3339 date-time with an offset or a full-date. An unparseable `stale_after` reads as stale. |
| `verified-event` | a `verified` event missing `by` or `at`; consumers ignore it (§5.2) |
| `superseded-by` | a `superseded_by` that resolves to no concept, or is set on a concept that isn't `deprecated` (§5.4) |
| `status-lifecycle` | a `status` outside `draft`, `stable`, `deprecated`; the concept isn't stable and filters to current knowledge drop it (§5.4) |

`source-missing-resource` reports a `sources` entry with no `resource`. §5.1
requires one, so consumers ignore the entry (§11.2 rule 3).

`okfctl analyze <dir>` reports where a bundle is *weak* rather than *wrong*:
freshness, clusters, gaps, connectivity, and structure. Where `lint` asks "is
anything broken?", `analyze` asks "where is this corpus thin?".

## Semantic lint (`lint --semantic`)

`okfctl lint <dir> --semantic` adds two similarity-driven checks. It requires an
index built by `okfctl-search index build`; **no embedding model is needed to
lint**, because core only ever *reads* an index.

Structural lint asks *"is anything linked to this?"*. Semantic lint asks *"is
anything even about the same thing?"*—the curation question the graph alone
can't answer:

| check | finding | reads as |
|---|---|---|
| `similar-unlinked` | two nodes scoring ≥ `--similarity-threshold` (default `0.80`) with **no link in either direction** | *"these cover the same ground and don't reference each other—missing cross-reference?"* |
| `no-semantic-neighbors` | a node whose **best** neighbor falls below `--isolation-floor` (default `0.20`) | *"nothing in the corpus is close to this—dead concept, or missing context?"* |

```sh
okfctl-search index build ./my-bundle     # the plugin builds (needs a model)
okfctl lint ./my-bundle --semantic        # core reads (needs none)
```

Three deliberate behaviors:

- **Opt-in.** Without `--semantic`, output is unchanged and the index is never
  read.
- **A missing index is an error, not a silent skip**—it names `okfctl-search
  index build`. A quiet structural-only fallback would let CI believe semantic
  checks ran when they didn't.
- **Index drift is surfaced.** Nodes added since the last `index build` produce
  one `stale-index` finding listing them, so a partial pass never reads as a
  clean one.

Findings are only as meaningful as the embedder that built the index. With the
default `hash` embedder, `similar-unlinked` effectively means "shares
vocabulary"; with `--embedder model2vec` it means genuinely related subject
matter. Build the index with `model2vec` if you intend to act on these findings.
See the [search guide](search.md) for embedder setup.

## `eval`—TACA-style trustworthiness

`validate` and `lint` check **format**; `eval` is the first machine gate that
touches **trust**. TACA decomposes node trust into four dimensions—**T**ransparency, **A**ccuracy, **C**alibration, **A**lignment—and `eval`
scores them as far as a pure-Go, offline, no-model tool can. Only Transparency is
machine-decidable without a model or the network; the other three are *scaffolded*
for a human or an out-of-band LLM judge, never auto-graded.

### `eval transparency`—the gate

Deterministic and offline, mirroring `lint`: advisory (exit 0) by default,
`--strict` to fail CI, `--json` for tooling. Four checks:

| check | meaning |
|---|---|
| `grade-missing` | a node carries no `epistemic` **and** no `authority` grade |
| `grade-vocabulary` | a grade value is off-vocabulary for the corpus (a likely typo/drift) |
| `uncited` | a node carries no citations of any kind (no `sources`, no `# Citations`, no internal reference) |
| `citation-unresolved` | an internal citation resolves to no node in the bundle |

The grade vocabulary is **derived from the corpus**, not hardcoded: a value
carried by at least `--grade-vocabulary-floor` nodes (default 2) counts as the
vocabulary; rarer values are flagged as drift. This catches the one-off typo
(`authority: high`) without inventing an enum the OKF spec deliberately leaves
open. External `http(s)` citations are out of scope—verifying them needs the
network (that's the sampler / human pass, see the `verifying-citation-link-fit`
discipline).

```sh
okfctl eval transparency --strict ./bundles/knowledge   # CI gate
okfctl eval transparency --json ./bundles/knowledge     # tooling
```

Wire it alongside `validate --strict` and `lint --strict` as a third
conformance gate.

### `eval sample`—the spot-check scaffold

Selects a sample of nodes and emits a structured eval-set for the three
dimensions okfctl can't automate. It computes **no truth verdict**—it
pre-populates every field it can extract (title, description, grades, sources,
candidate claims) and leaves the judgment slots empty for a reviewer to fill in.

```sh
# A worksheet for 5 random nodes (deterministic, seeded)
okfctl eval sample --sample 5 --format md ./bundles/knowledge

# Machine eval-set for nodes changed on this branch (the curation/CI hook)
okfctl eval sample --changed-since origin/main ./bundles/knowledge
```

`--changed-since` is the curation hook: a pre-merge job runs
`eval transparency --strict` as the gate and `eval sample --changed-since` to
drop a spot-check worksheet on the touched nodes. A periodic
`eval sample --sample N` draws a re-verification sample to measure Calibration
(do our `VERIFIED` labels hold up on re-check?) over time.

## Skipping vendored and derived directories

Every command that walks a bundle (`validate`, `lint`, `analyze`, `search`,
`graph export`, `index build`/`check`) prunes vendored and derived directories
by default—a Python virtualenv (`venv`), `node_modules`, a `vendor/` tree, or
a build-output dir (`dist`, `build`, `target`, …) sitting under the bundle root
holds `.md` files nobody authored as knowledge, and walking them pollutes every
report. The prune is by directory **base name** at any depth (see
`DefaultSkipDirs`), applied once in the loader so all commands share identical
scope, and it never touches the bundle root itself.

Two other exclusions come from the spec, not from this list (OKF v0.3 §3.2):

- Files and directories whose names start with `.` (`.git`, `.venv`,
  `.draft.md`, `.hidden/`) are not part of the bundle. They are never read,
  `--no-ignore` does not bring them back, and they are not announced.
- A `.md` file with no frontmatter or no `type` (a `README.md`, an
  `AGENTS.md`) is not a concept. It is not a graph, lint, index, or search
  subject, and `validate` prints a warning for it without failing.

Two guardrails keep this from silently eating your work:

- **`--no-ignore`** restores the full walk on any of those commands, so a
  directory whose name happens to match the skip list (real content you
  deliberately authored there) is always recoverable.
- The skip is **never silent**: when the walk prunes anything, the command
  prints a note to **stderr** naming the skipped directories and pointing at
  `--no-ignore`.

This is a built-in default, not a policy read from config—`okfctl` deliberately
doesn't consult `.gitignore` (curation scope and version-control scope are
different questions) and needs no `.okfctlignore` to be usable on a real tree.
