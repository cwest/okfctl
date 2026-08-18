# Search

okfctl offers two kinds of search: a core, always-available lexical/graph search
built into `okfctl` itself, and an optional semantic search shipped as the
`okfctl-search` plugin.

## Core lexical and graph search

`okfctl search` is stdlib-only: it runs without an embedding model or a prebuilt
index, so it's always available.

- `okfctl search "query" [dir]` matches concept nodes case-insensitively by
  title, tag, type, or body substring; restrict the match to a field with
  `--field title|tag|type|body` (default `any`). Reserved files (`index.md`/`log.md`) are
  never results. A zero-result query is not an error. Add `--json` for a
  deterministic, CI-diffable array.
- `okfctl search --neighbors <node-path> [dir]` is a graph-structural query: the
  concept nodes within `--depth` hops (default 1) of a node in the link graph.
  Edges are treated as **undirected**, so a reader's traversal is symmetric.

Run `okfctl search --help` for the full flag set.

## okfctl-search — offline semantic search (plugin)

`okfctl-search` is a bundled plugin (a separate static binary; invoke as `okfctl
search …` via plugin dispatch, or directly). It adds **semantic** search over a
bundle's concept nodes, fully offline: a single static Go binary with zero
runtime dependencies. Everything the embedding needs is compiled in, so there is
nothing external to install — the Python interpreter, ONNX runtime, `sqlite-vec`,
and model server that a typical embedding stack would require are all unnecessary.
It shares the exact embedding
protocol with `cwest/knowledge-base` so vectors are cross-verifiable.

- `okfctl-search index build [bundle-dir]` — embed every concept node into
  `.okfctl/index.db`, recording the embedder model + dimension. Content-hash
  keyed: an unchanged node isn't re-embedded; deterministic for a fixed embedder.
- `okfctl-search --semantic "query" [bundle-dir]` — rank nodes by cosine
  similarity to the query (top-`--k`, default 5). Refuses an index built under a
  different model (rebuild with `index build`).
- `okfctl-search related <node-path> [bundle-dir]` — a node's nearest neighbors
  (self excluded); the neighbor set `lint --semantic` consumes for its
  similarity-driven checks (§8.6).
- `--embedder hash` (default) is the offline, dependency-free embedder. It is
  deterministic and needs no model, but it's *lexical* — it matches tokens, not
  meaning.
- `--lexical-gate` (off by default) gates the semantic results by a **term-wise**
  lexical match and preserves lexical recall. It runs the semantic query wide,
  keeps the results whose node also contains a query term (stopwords dropped,
  plurals/inflections stemmed so `hash` and `hashes` match the same nodes), in
  semantic order, then **appends the lexical hits the semantic band missed** so a
  correct exact match outside the embedding's top band is never discarded. It is
  useful for exact-identifier-shaped queries where the embedding blurs a rare
  token. It **degrades to pure semantic** (a no-op, byte-identical to gate-off)
  when the query has no content terms (an all-stopword question like `"how should
  the"`) **or** a term matches more than 60% of the bundle (a term that broad
  carries no discriminating signal — e.g. `agent` matches 73% of the reference
  corpus). Composes with `--path`/`--type`/`--tag` (which constrain both the
  semantic band and the appended lexical tail) and with `--half-life`.

### Real semantic search with `--embedder model2vec`

`--embedder model2vec` runs a genuine static embedding model (for example
[`minishlab/potion-base-8M`](https://huggingface.co/minishlab/potion-base-8M)) in
**pure Go** — the BERT WordPiece tokenizer and the Model2Vec inference math are
both ported into `internal/search`, so it compiles `CGO_ENABLED=0` and inherits
the same self-contained, dependency-free profile as the `hash` embedder. Vectors
match the upstream `model2vec` library's own output to
within `1e-5`, which is verified against the real model in the test suite.

`okfctl` never downloads a model at runtime. Point it at a directory you already
have on disk:

```sh
# once — persisted in okfctl's JSON config
okfctl config set model_path ~/models/potion-base-8M
okfctl-search --embedder model2vec index build ./my-bundle
okfctl-search --embedder model2vec --semantic "tannin structure" ./my-bundle

# or per-invocation, overriding the config
okfctl-search --embedder model2vec --model-path ~/models/potion-base-8M --semantic "…" ./my-bundle
```

The directory needs the standard model2vec layout: `config.json`,
`model.safetensors`, and `tokenizer.json` (or `vocab.txt`). If no path is
configured, `model2vec` fails with an actionable error rather than silently
falling back to `hash` — a query answered by the wrong embedder is worse than one
that refuses to run. An index records the model it was built with, so switching
embedders requires a rebuild.
