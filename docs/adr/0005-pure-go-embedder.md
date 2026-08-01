# 5. Pure-Go Model2Vec + WordPiece embedder

- **Status:** Accepted (2026-07-25)
- **Deciders:** Casey West
- **Resolves:** PRD [§13.2](../PRD.md#132-semantic-search-build-pathshell-out-vs-native-go-embedder)
- **Sources:** spec `docs/specs/2026-07-24-okfctl-search.md` ("Embedder (fork 1a)"), `docs/specs/2026-07-24-model2vec-loader.md`, `docs/specs/2026-07-25-wordpiece-tokenizer.md`; code `internal/search/model2vec.go`, `internal/search/model2vec_embedder.go`, `internal/search/wordpiece.go`
- **Related:** [ADR 0002](0002-path-dispatch-extension-model.md) (the plugin), [ADR 0004](0004-flat-json-vector-store.md) (where its vectors land)

## Context

`cwest/knowledge-base` already runs the OKF embedding architecture in production
at `tools/okf/embed.py`, built around a single `Embedder` protocol
(`name`, `dim`, `encode(texts) -> vectors`) with a Model2Vec static-embedding
backend (`minishlab/potion-base-8M`, a BERT WordPiece tokenizer + a mean-pooled
static weight matrix). okfctl MUST reconcile with that protocol rather than
invent a parallel one; only the *language of the client* is open. Two build paths
were on the table (PRD §8.4):

1. **Shell out** from the `okfctl-search` plugin to the existing `tools/okf`
   Python embedder — reuse the exact in-production code, at the cost of a Python
   runtime dependency for the plugin.
2. **Re-implement** the tokenizer and Model2Vec inference natively in Go against
   the same `Embedder` contract and the same models — no Python at runtime, at
   the cost of a faithful port that must track the protocol.

Shelling out to Python would undo the plugin's single-static-binary property: it
would require a Python interpreter, the right packages, and the model
machinery present at runtime — a heavy, fragile dependency that contradicts the
offline, zero-runtime-dependency posture the whole tool is built around
([ADR 0001](0001-build-in-go.md), [ADR 0004](0004-flat-json-vector-store.md)).

## Decision

Re-implement the embedder **natively in pure Go**. Ported into `internal/search`:
the BERT **WordPiece** tokenizer (`wordpiece.go`) and the **Model2Vec** static-
model inference — safetensors weight-matrix load plus mean-pool
(`model2vec.go`, `model2vec_embedder.go`). No Python, no ONNX runtime, no CGO. The
port is verified faithful: Go vectors match the upstream `model2vec` library's own
output to within `1e-5`, checked against the real model in the test suite, and the
default offline `HashEmbedder` is ported byte-for-byte from the KB so a Go-embedded
vector equals the Python one for the same text. The index records the model it was
built with; switching embedders requires a rebuild. okfctl never downloads a model
at runtime — the user points it at a model directory already on disk.

## Consequences

**What it buys.** The `okfctl-search` plugin stays a single, statically-linked,
fully-offline Go binary — no Python interpreter, no package environment, no ONNX,
nothing to install alongside it. Genuine semantic embeddings are available out of
a plain download. Because the port is verified to `1e-5` against upstream and the
`HashEmbedder` is byte-identical to the KB's, vectors are cross-verifiable with
`cwest/knowledge-base` rather than a drifting look-alike — the shared *protocol*
is honored even though the client language differs.

**What it costs.** okfctl now owns a faithful port of a tokenizer and static-model
inference math it did not previously maintain: when the upstream Model2Vec
protocol or the WordPiece behavior changes, the Go port must be re-verified and
re-synced, and the `1e-5` fidelity check is a standing maintenance obligation, not
a one-time win. Reusing the exact in-production Python code was the cheaper path in
raw engineering effort; native Go trades that up-front and ongoing porting cost for
the runtime independence above. The MLX backend described in the shared protocol is
not part of this native port.
