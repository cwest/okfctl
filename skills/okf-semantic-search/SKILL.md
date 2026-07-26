---
name: okf-semantic-search
description: Use when adding the opt-in semantic layer to an OKF bundle with okfctl — configuring an offline embedding model, building the semantic index with okfctl-search, running semantic search/related, and driving lint --semantic (similar-unlinked, no-semantic-neighbors) with the reasoning behind the calibration defaults.
version: 1.0.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, semantic-search, embeddings, model2vec, lint, knowledge-graph]
    related_skills: [okf-authoring, okf-curation-health]
    sharing: shareable
---

# The opt-in semantic layer with okfctl-search

## Overview

Structural checks ask *"is anything linked to this?"*. The **semantic** layer
asks *"is anything even about the same thing?"* — the curation question the graph
alone can't answer. It has two parts:

- **`okfctl-search`** — a bundled plugin that embeds concept nodes into a local
  index (`.okfctl/index.db`) and does semantic search over them, fully offline.
- **`lint --semantic`** — core `lint` extended with two similarity-driven checks
  (`similar-unlinked`, `no-semantic-neighbors`) that *read* that index.

**This is the only okfctl path that needs a model.** Building the index needs an
embedding model; *reading* it (which is all `lint --semantic` does) needs none.

## When to Use

- You want to find missing cross-references between nodes that read alike but
  aren't linked, or nodes with no semantically related kin.
- You want semantic (meaning-based) search/`related` over a bundle.
- You are retuning the similarity/isolation thresholds for your own corpus.

Don't use for: structural curation (orphans, coverage gaps) — that needs no model
and lives in `okf-curation-health`; authoring — see `okf-authoring`.

## The two embedders

`okfctl-search` ships two embedders:

- **`--embedder hash`** (default) — offline, dependency-free, deterministic, and
  needs no model. But it is *lexical*: it matches shared tokens, not meaning. Fine
  for a smoke test; **do not act on its `similar-unlinked` findings** — they mean
  "shares vocabulary," not "related subject."
- **`--embedder model2vec`** — a genuine static embedding model run in pure Go (no
  CGO, no Python, no ONNX). Use this if you intend to act on semantic findings.

## 1. Configure a model (model2vec)

okfctl **never downloads a model at runtime.** Point it at a model directory you
already have on disk. A model2vec directory needs the standard layout:
`config.json`, `model.safetensors`, and `tokenizer.json` (or `vocab.txt`). One
common source is a Hugging Face cache snapshot, e.g. `minishlab/potion-base-8M`.

Set the path once (persisted in okfctl's JSON config), or override per-invocation:

```sh
# once — persisted in config
$ okfctl config set model_path /path/to/potion-base-8M
$ okfctl config get model_path
/path/to/potion-base-8M
$ okfctl config list
model_path = /path/to/potion-base-8M

# or per-invocation, overriding the config
$ okfctl-search --embedder model2vec --model-path /path/to/potion-base-8M --semantic "…" ./mykb
```

If no model can be resolved, `model2vec` fails with an actionable error rather
than silently falling back to the `hash` embedder — a query answered by the wrong
embedder is worse than one that refuses to run:

```sh
$ okfctl-search --embedder model2vec --model-path /nonexistent/model index build mykb
okfctl-search: loading model2vec model from /nonexistent/model: open /nonexistent/model/config.json: no such file or directory
```

## 2. Build the index

```sh
$ okfctl-search --embedder model2vec index build mykb
indexed 11 node(s) with minishlab/potion-base-8M@bf8b056651a2 (dim 256) -> mykb/.okfctl/index.db
```

The index is **content-hash keyed**: an unchanged node is not re-embedded, and
the build is deterministic for a fixed embedder. The index records the model it
was built with; switching embedders requires a rebuild.

### The index is a derived artifact — never commit it

`.okfctl/index.db` is a **derived artifact keyed to a specific model revision.**
It is not source of truth and it goes stale silently as nodes change. Add it to
your bundle's `.gitignore` (okfctl does not do this for you):

```sh
$ echo '.okfctl/' >> mykb/.gitignore
```

Committing it invites two failure modes: a stale index that no longer matches the
nodes, and an index built under a different model that another checkout can't use.
Rebuild it locally; don't track it.

## 3. Semantic search and related

```sh
$ okfctl-search --embedder model2vec --semantic "tannin structure" mykb
0.7757	concepts/tannin.md
0.4079	concepts/mouthfeel.md
0.3643	concepts/wine.md
0.2197	concepts/aging.md
0.1333	concepts/balance.md

$ okfctl-search --embedder model2vec related concepts/tannin.md mykb
0.5231	concepts/aging.md
0.5220	concepts/mouthfeel.md
0.4818	concepts/wine.md
...
```

An index built under one embedder is refused by another — rebuild to switch:

```sh
$ okfctl-search --semantic "caching" mykb          # default hash, index built with model2vec
okfctl-search: index model does not match the active embedder; rebuild with 'okfctl-search index build'
```

## 4. lint --semantic

`lint --semantic` adds two checks by reading the index. **Core only reads the
index, so no embedding model is needed to lint** — only to build.

```sh
$ okfctl-search --embedder model2vec index build mykb   # build (needs a model)
$ okfctl lint mykb --semantic                            # read (needs none)
```

The two semantic findings:

| finding | fires when | reads as |
|---|---|---|
| `similar-unlinked` | two nodes score ≥ `--similarity-threshold` (default `0.80`) with **no link in either direction** | *"these cover the same ground and don't reference each other — missing cross-reference?"* |
| `no-semantic-neighbors` | a node's **best** neighbor falls below `--isolation-floor` (default `0.20`) | *"nothing in the corpus is close to this — dead concept, or missing context?"* |

Real output (two near-identical unlinked nodes, and an isolated node):

```sh
$ okfctl lint mykb --semantic
0.94 semantically similar to b.md with no link between them — missing cross-reference?
no semantically close node (best neighbor 0.17, below 0.20) — dead concept, or missing context?
...
```

Add a link between the similar pair (in either direction) and the
`similar-unlinked` finding clears — the check reads the live graph, not a cached
answer.

### Behaviors to rely on

- **Opt-in.** Without `--semantic`, output is unchanged and the index is never
  read.
- **A missing index is an ERROR, not a silent skip.** It names the exact fix so a
  CI job can never believe it ran semantic checks when it didn't:

  ```sh
  $ okfctl lint mykb --semantic          # no index built yet
  okfctl: no semantic index at mykb/.okfctl/index.db: run 'okfctl-search index build mykb' first
  ```

- **Index drift is surfaced.** Nodes added since the last `index build` produce
  one finding naming them, so a partial pass never reads as clean:

  ```sh
  1 node(s) absent from the semantic index and not checked (concepts/acidity.md) — rerun 'okfctl-search index build'
  ```

## Calibrating the thresholds for your corpus

The defaults are `--similarity-threshold 0.80` and `--isolation-floor 0.20`. The
isolation floor was **lowered from 0.30 to 0.20 on purpose**, and you need the
reasoning, not just the number, to retune for your own corpus:

- With a mean-pooled static model (e.g. potion-base-8M), **absolute similarity
  scores are compressed** — the ranking is reliable, the magnitudes are not.
- On a small topical corpus, same-topic-different-wording nodes score roughly
  **0.27–0.33**, while a genuinely off-topic node (say a Kubernetes concept
  dropped among wine notes) scores around **0.13**.
- A 0.30 floor therefore flags legitimately on-topic nodes as "dead concepts" —
  a false positive that trains users to ignore the check. **0.20** separates the
  true outlier from merely-loosely-related kin.

So: the floor targets the *clear outlier*, not a semantic ideal. If your corpus
and model produce different magnitudes (check with `okfctl-search related` on a
few known-related and known-unrelated nodes), retune `--isolation-floor` to sit
between your "on-topic but loosely related" band and your "genuinely off-topic"
scores. Likewise, `similar-unlinked` at 0.80 with `hash` means "shares
vocabulary"; only with `model2vec` does it mean genuinely related subject matter.

## Common Pitfalls

1. **Running `lint --semantic` with no index.** It errors (naming `okfctl-search
   index build`) rather than silently degrading to structural-only. Build the
   index first; don't interpret the error as "no findings."

2. **Committing `.okfctl/index.db`.** It's a gitignored-by-convention derived
   artifact keyed to a model revision. It goes stale silently and won't match
   another checkout's model. Gitignore it and rebuild locally.

3. **Acting on `hash`-embedder findings.** The default embedder is lexical.
   `similar-unlinked` under `hash` means "shares tokens," not "related." Build
   with `--embedder model2vec` before trusting semantic findings.

4. **Expecting okfctl to download a model.** It never fetches at runtime. Point
   `model_path` at a directory already on disk with the model2vec layout, or the
   command fails with an actionable error.

5. **Copying the 0.20 floor blindly to a different model.** The floor is
   calibrated to a specific model's score distribution. A different model (or
   corpus) shifts the magnitudes — recalibrate using `related` scores on
   known-related vs. known-unrelated node pairs.

6. **Assuming an edited node re-triggers `similar-unlinked` without a rebuild.**
   `lint --semantic` reads stored vectors. Editing a node's prose changes nothing
   until you `okfctl-search index build` again; only added/removed node *paths*
   surface as index drift.

## Verification Checklist

- [ ] `okfctl config get model_path` resolves to a real model2vec directory
- [ ] `okfctl-search --embedder model2vec index build <dir>` reports the expected
      model + node count
- [ ] `.okfctl/` is in the bundle's `.gitignore`
- [ ] `okfctl lint <dir> --semantic` runs (no missing-index error) and findings
      were reviewed
- [ ] Thresholds retuned against this corpus's actual `related` scores if the
      defaults misfire
- [ ] Index rebuilt after node content changes before trusting semantic findings
