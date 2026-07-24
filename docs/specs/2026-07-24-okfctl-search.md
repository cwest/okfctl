# Spec: Increment 5b - okfctl-search (semantic plugin, native-Go)

Status: Draft for approval  Owner: Casey West  License: Apache-2.0
Increment: 5b (builds on 5a PATH-dispatch; native-Go per section 13.2)
PRD: section 8 (semantic search plugin), section 8.4 (native-Go embedder), section 8.5 (command surface), section 8.6 (lint payoff)
Protocol source of truth: cwest/knowledge-base tools/okf/embed.py (DO NOT duplicate the protocol - port it)

## Goal

Ship okfctl-search, a PATH-dispatch plugin (invoked as `okfctl search ...` via 5a
dispatch, or directly) that adds SEMANTIC search over a bundle's concept nodes,
fully offline, as a single static Go binary with zero runtime deps - consistent
with section 5.1 and the native-Go section 13.2 decision. It shares the exact
Embedder protocol + model-field reproducibility discipline with the KB (section 8.4).

## Ported protocol (from tools/okf/embed.py - the shared contract)

- `Embedder` interface: `Name() string`, `Dim() int`, `Encode(texts []string) [][]float64`.
- `cosine(a, b []float64) float64` - equal-length cosine, 0.0 if either zero-norm.
- `HashEmbedder` - deterministic, stdlib-only: per token sha1 -> bucket index +
  sign, accumulate, L2-normalize. Ported BYTE-FOR-BYTE from the KB's HashEmbedder
  (same sha1 bucketing, dim default 64, name "hash-test-embedder") so a Go-embedded
  vector equals the Python one for the same text - cross-verifiable against the KB.

## Store (fork 2a - flat Go-native, decided)

`.okfctl/index.db` = a Go-serialized flat vector store (JSON for slice-1 legibility;
records `model` (embedder Name) + `dim` per section 8.5 reproducibility discipline, plus
per-node {path, content-hash, vector}). Brute-force cosine over N vectors - instant
for KB-sized corpora (hundreds-low-thousands). NO sqlite-vec (CGO/C-extension,
violates "no separate install"). ANN index is a later slice if scale demands.
Query refuses a store whose recorded `model`/`dim` != the active embedder's (the
reproducibility guard - never compare vectors across models).

## Embedder (fork 1a - pure-Go Model2Vec, decided) + the scope boundary I need approved

Section 13.2 = native-Go, not shell-out-to-Python. Native-Go still has a
big-vs-small sub-choice, and this is the scope call:

- **5b (this slice): the Embedder contract + HashEmbedder default + the whole
  index/query/related machinery.** HashEmbedder is deterministic, stdlib-only, needs
  NO model download - so okfctl-search works fully offline out of the box and every
  command is exercisable end-to-end on the built binary. The index format, `search
  --semantic`, `related`, and `index build` are all embedder-AGNOSTIC (they consume
  the Embedder interface), so they are complete and correct regardless of which
  embedder is plugged in.
- **5c (named follow-on): the pure-Go Model2Vec static-model loader.** Porting
  StaticModel (tokenizer + HF weight-matrix load + mean-pool) is a large faithful
  port that deserves its own focused PR. It plugs into the SAME Embedder interface
  5b defines, so it lands without touching the index/query code.

This keeps 5b a proportionate PR AND ships a working offline semantic-search plugin
today; 5c swaps in the production-quality static model behind the same seam.

## Command surface (section 8.5) - all via the 5a dispatch or direct invocation

1. `okfctl-search index build [bundle-dir]` - embed every concept node
   (content-hash keyed; skip unchanged), write `.okfctl/index.db` recording
   model+dim. Deterministic for a fixed embedder.
2. `okfctl-search --semantic "query" [bundle-dir]` - embed the query, cosine over
   the store, print top-K {score, path, title} sorted desc. Refuses a stale/
   mismatched-model store with a clear rebuild message.
3. `okfctl-search related <node-path> [bundle-dir]` - nearest neighbors of a node
   (excludes self), the primitive section 8.6 says `lint` consumes.

`--embedder hash` is the default (offline); `--embedder model2vec` errors with
"not yet available (increment 5c)" until 5c lands - honest, no silent fallback.

## Boundaries / decisions

- New package `internal/search` (pure model: Embedder, HashEmbedder, cosine, Store,
  BuildIndex, Query, Related). stdlib-only, NO cobra, NO net/http, NO CGO.
- The plugin binary is `cmd/okfctl-search/` (its own main), built as a SEPARATE
  executable from core okfctl - that IS the plugin model. It ships in the same repo/
  module but as a distinct binary on PATH.
- Core `okfctl search "q"` (lexical/graph, stdlib, in CORE per section 8.5) is a
  SEPARATE small slice, not 5b. 5b is the plugin (`--semantic` + index + related).
- Content-hash keying: reuse the node's existing identity; a node whose content
  hash is unchanged since the last `index build` is not re-embedded.

## Done criteria (exercised on the built plugin binary)

1. `index build` on a real bundle writes `.okfctl/index.db` recording model+dim +
   one vector per concept node; a second build is a no-op for unchanged nodes and
   byte-identical for a fixed embedder (deterministic).
2. `--semantic "q"` returns nodes ranked by cosine desc; a query for text close to
   a known node ranks it top.
3. `related <node>` returns that node's nearest neighbors, self excluded.
4. Model-mismatch guard: a store recorded under model A, queried with embedder B,
   refuses with a rebuild message (never cross-model vector compare).
5. HashEmbedder Go output == KB Python HashEmbedder output for the same text
   (protocol fidelity - not a re-implementation that drifts).
6. `internal/search` cobra-free + net/http-free + CGO-free; full -race suite green;
   gofmt/vet clean; no new module deps; plugin discoverable + dispatchable via 5a
   (`okfctl search ...` -> okfctl-search on PATH).
