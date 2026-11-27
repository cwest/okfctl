---
name: okf-semantic-search
description: Use when adding similarity search and similarity lint to an OKF bundle with okfctl. Covers building the index with okfctl-search, pointing it at a local Model2Vec model, search and related, and lint --semantic (similar-unlinked, no-semantic-neighbors) with how to tune its thresholds.
version: 2.0.0
author: okfctl
license: Apache-2.0
metadata:
  hermes:
    tags: [okfctl, okf, semantic-search, embeddings, model2vec, lint, knowledge-graph]
    related_skills: [okf-authoring, okf-curation-health]
    sharing: shareable
---

# Similarity search and lint with okfctl-search

## Overview

Two nodes say nearly the same thing and don't link each other, and a third is
off topic. Build the index, then lint with `--semantic`:

<!-- transcript: exit 1 okfctl lint ./mykb --semantic -->
```text
$ okfctl bundle init mykb
Initialized OKF bundle in mykb
$ okfctl node new wine/tannin.md --type Reference --title Tannin --bundle mykb
Created mykb/wine/tannin.md
$ okfctl node new wine/tannins.md --type Reference --title Tannins --bundle mykb
Created mykb/wine/tannins.md
$ okfctl node new ops/kubernetes.md --type Reference --title Kubernetes --bundle mykb
Created mykb/ops/kubernetes.md
$ echo 'Tannins are polyphenols from grape skins and oak that dry the palate and give red wine its grip.' >> mykb/wine/tannin.md
$ echo 'Tannins are polyphenols from grape skins and oak barrels that dry the palate and give red wine grip.' >> mykb/wine/tannins.md
$ echo 'Kubernetes schedules containers across a cluster of nodes.' >> mykb/ops/kubernetes.md
$ okfctl lint ./mykb --semantic
okfctl: no semantic index at mykb/.okfctl/index.db: run 'okfctl-search index build ./mykb' first
$ okfctl-search index build mykb
indexed 3 node(s) with hash-test-embedder (dim 64) -> mykb/.okfctl/index.db
$ okfctl lint mykb --semantic --json
[
  {
    "check": "missing-xref",
    "path": "wine/tannin.md",
    "message": "missing-xref: wine/tannin.md mentions \"Tannins\" but does not link to wine/tannins.md"
  },
  {
    "check": "no-semantic-neighbors",
    "path": "ops/kubernetes.md",
    "message": "no semantically close node (best neighbor 0.00, below 0.20) — dead concept, or missing context?"
  },
  {
    "check": "similar-unlinked",
    "path": "wine/tannin.md",
    "message": "0.83 semantically similar to wine/tannins.md with no link between them — missing cross-reference?"
  }
]
```

`okfctl-search` is a separate plugin binary that ships beside `okfctl`. It
embeds each concept node and stores the vectors in `.okfctl/index.db`.
`lint --semantic` only reads that file (that's why linting needs no model).
With no index it exits 1 and names the command to run. It never skips the
check silently. The structural checks still run (that's where `missing-xref`
came from).

The run above used the default `hash` embedder. It needs no model. It compares
word overlap and ignores meaning, and the 0.83 here says the two nodes use the
same vocabulary. For findings you'll act on, build with `--embedder model2vec`
(below).

Use `--json` if a script or agent reads lint output. The text form of the two
semantic findings leaves out the node path. The JSON form has it.

## When to Use

- You want to find nodes on one topic that don't link each other.
- You want to find nodes that nothing else in the bundle is close to.
- You want search or nearest-neighbor lookups by meaning.
- The default thresholds misfire on your corpus and you need to retune them.

Orphans, coverage gaps, and broken links need no index. They're in
`okf-curation-health`.

## Point okfctl at a local model

`--embedder model2vec` loads a [Model2Vec](https://github.com/MinishLab/model2vec)
static embedding model in-process. okfctl doesn't ship model weights and never
downloads one. You download a model yourself, for example
[`minishlab/potion-base-8M`](https://huggingface.co/minishlab/potion-base-8M),
and point okfctl at its directory. It has to hold `config.json`,
`model.safetensors`, and `tokenizer.json` (or `vocab.txt`).

Set the path in okfctl's config. To override it for one run, pass `--model-path`:

<!-- transcript: skip needs a downloaded Model2Vec model, and the test environment has none -->
```text
$ okfctl config set model_path ~/models/potion-base-8M
$ okfctl config get model_path
/Users/you/models/potion-base-8M
$ okfctl-search --embedder model2vec --model-path ~/models/potion-base-8M index build mykb
indexed 3 node(s) with minishlab/potion-base-8M@bf8b056651a2 (dim 256) -> mykb/.okfctl/index.db
$ okfctl-search --semantic tannin mykb
okfctl-search: index model does not match the active embedder; rebuild with 'okfctl-search index build'
```

The index records which model made it. The last query above used the default
`hash` embedder against a `model2vec` index and failed. Rebuild to switch.

If the path holds no model, the build fails. It doesn't fall back to `hash`:

<!-- transcript: exit 1 okfctl-search --embedder model2vec --model-path /nonexistent/model index build mykb -->
```text
$ okfctl-search --embedder model2vec --model-path /nonexistent/model index build mykb
okfctl-search: loading model2vec model from /nonexistent/model: open /nonexistent/model/config.json: no such file or directory
```

With potion-base-8M on the bundle above, the near-duplicate pair scores 0.99
(up from 0.83) and the Kubernetes node's best neighbor is 0.11 (up from
0.00).

## Build and rebuild the index

`index build` keys each vector by a hash of the node's content. It skips nodes
that haven't changed. One embedder produces the same index each time you run it.

The index comes from the nodes plus one model revision. Don't commit it.
okfctl doesn't add it to `.gitignore` for you:

```text
$ echo '.okfctl/' >> mykb/.gitignore
```

A node added since the last build shows up as one finding, and a partial run
never reads as clean:

```text
$ okfctl node new wine/body.md --type Reference --title Body --bundle mykb
Created mykb/wine/body.md
$ okfctl lint mykb --semantic | grep absent
1 node(s) absent from the semantic index and not checked (wine/body.md) — rerun 'okfctl-search index build'
```

Edits to a node's text don't show up this way. Lint reads the stored vectors.
After you edit nodes, rebuild, then trust the findings.

## Search and related

```text
$ okfctl-search index build mykb
indexed 4 node(s) with hash-test-embedder (dim 64) -> mykb/.okfctl/index.db
$ okfctl-search related wine/tannin.md mykb
0.8276	wine/tannins.md
0.2491	wine/body.md
0.0000	ops/kubernetes.md
```

`okfctl-search --semantic "query" <dir>` ranks nodes against a query, top 5 by
default (`--k`). `related <path> <dir>` lists one node's nearest neighbors, the
same data `lint --semantic` uses. Each takes `--path`, `--type`, and `--tag`
filters. For lexical or link-graph search with no index, use core
`okfctl search` (see `okfctl search --help`).

## The two semantic findings

| finding | fires when | default |
|---|---|---|
| `similar-unlinked` | two nodes score at or above `--similarity-threshold` and neither links the other | `0.80` |
| `no-semantic-neighbors` | a node's best neighbor scores below `--isolation-floor` | `0.20` |

Add a link in either direction and `similar-unlinked` clears on the next lint.
Lint reads links from the bundle itself. The index holds only vectors.

## Tune the thresholds

The isolation floor started at 0.30. It came down to 0.20 after a test of
potion-base-8M on a few wine nodes: same-topic nodes in different words scored
0.27 to 0.33 there. One Kubernetes node dropped among them scored 0.13. The old
floor flagged good wine nodes as dead concepts, but 0.20 flags only the
outlier.

With a mean-pooled static model like potion-base-8M, scores bunch up and the
order of results holds up better than the numbers themselves. To
pick a threshold, run `okfctl-search related` on a few pairs you know are
related and a few you know aren't, then put `--isolation-floor` between those
two bands. Another model or corpus will want its own numbers.

## Common Pitfalls

1. **Reading the missing-index error as "no findings."** It exits 1. Build the
   index first.
2. **Committing `.okfctl/index.db`.** It goes stale as nodes change and is tied
   to one model revision. Add `.okfctl/` to `.gitignore`.
3. **Acting on `hash` findings.** `hash` matches words. Rebuild with
   `--embedder model2vec`, then decide what to link or delete.
4. **Assuming okfctl will download a model.** It never does. Download one and
   set `model_path`.
5. **Copying 0.20 to another model.** Measure with `related` and retune.
6. **Linting after edits with no rebuild.** Lint reads stored vectors. Only
   added or removed paths show up as drift.
7. **Parsing the text output.** Use `--json`. The text lines for semantic
   findings have no path.

## Verification Checklist

- [ ] `okfctl config get model_path` prints a Model2Vec directory that exists
- [ ] `okfctl-search --embedder model2vec index build <dir>` names the model
      and node count you wanted
- [ ] `.okfctl/` is in the bundle's `.gitignore`
- [ ] `okfctl lint <dir> --semantic --json` finishes with no missing-index
      error, and each finding was reviewed
- [ ] The index was rebuilt after the last content edit
- [ ] Thresholds were checked against `related` scores on this corpus
