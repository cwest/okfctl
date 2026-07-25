# Spec: Increment 5c-1 - Model2Vec safetensors loader + StaticModel (pure Go)

Status: Approved  Owner: Casey West  License: Apache-2.0
Increment: 5c-1 (first half of 5c; the pure-Go Model2Vec embedder, native per section 13.2)
Depends on: 5b (the Embedder seam in internal/search)
Protocol source of truth: minishlab/potion-base-8M (config.json + model.safetensors) as used by
cwest/knowledge-base tools/okf/embed.py Model2VecEmbedder

## Goal

Build the pure-Go, zero-CGO half of the real Model2Vec embedder: a safetensors reader for
the static embedding matrix + a StaticModel that turns TOKEN IDS into an embedding
(gather rows -> mean-pool -> L2-normalize per config.normalize). This plugs into the
5b Embedder seam. The TOKENIZER (text -> token IDs) is 5c-2; keeping them separate makes
each a proportionate, independently-testable PR and lets 5c-1 be verified against a tiny
synthetic model with NO 30 MB download.

## The model format (ground-truthed against potion-base-8M)

- config.json: hidden_dim 256, normalize true, tokenizer_name "baai/bge-base-en-v1.5".
- model.safetensors byte layout: [u64 little-endian header-length][JSON header][raw tensor data].
  One tensor "embeddings", dtype F32, shape [vocab, dim] (potion: [29528, 256]).
  JSON header entry: {"embeddings": {"dtype":"F32","shape":[V,D],"data_offsets":[start,end]}}
  plus an optional "__metadata__" key. Row r's D floats are the little-endian F32s at
  data section offset r*D*4.

## Package surface (internal/search, stdlib-only, no cobra/net-http/CGO)

- ReadSafetensorsMatrix(path string) (rows [][]float64, dim int, err error)
  - parse u64 header-len, JSON header, locate the "embeddings" F32 tensor, decode every
    row as D little-endian float32 -> float64. Validate dtype==F32, len==V*D*4, 2-D shape.
- type StaticModel struct { Rows [][]float64; Dim int; Normalize bool }
- LoadStaticModel(dir string) (*StaticModel, error)
  - read dir/config.json (hidden_dim, normalize) + dir/model.safetensors; cross-check
    matrix dim == config hidden_dim.
- (m *StaticModel) EncodeIDs(ids []int) []float64
  - gather Rows[id] for each id (skip out-of-range ids defensively), MEAN-pool the gathered
    rows, then L2-normalize IFF m.Normalize. Empty ids -> zero vector of length Dim.
    This is the exact Model2Vec inference math (static lookup + mean-pool + normalize);
    the only missing piece for a full Embedder is text->ids (5c-2).

## Boundaries / decisions

- stdlib only (encoding/binary, encoding/json, math, os). NO safetensors library, NO CGO,
  NO network. safetensors is a trivial format; a dependency would violate the static-binary thesis.
- F32-only for this increment (potion-base-8M is F32). A dtype other than F32 -> clear error,
  not a silent misread. (Other dtypes are a later concern if a model needs them.)
- 5c-1 does NOT wire a new --embedder value or touch cmd/. StaticModel is exercised via unit
  tests against a synthetic model. 5c-2 adds the tokenizer + wires --embedder model2vec + the
  real potion-base-8M fidelity harness. Until 5c-2, --embedder model2vec keeps its honest
  "not yet available" error from 5b.
- Determinism: EncodeIDs is pure arithmetic; identical ids -> identical vector.

## Done criteria (unit-tested; synthetic model, no download)

1. ReadSafetensorsMatrix on a hand-built synthetic safetensors ([3,4] F32 with known values)
   returns the exact rows, dim 4; rejects a truncated data section and a non-F32 dtype with
   clear errors.
2. LoadStaticModel reads a synthetic dir (config.json hidden_dim=4 normalize=true +
   model.safetensors) into a StaticModel with Dim 4, Normalize true, cross-check passing;
   a hidden_dim/matrix-dim mismatch errors.
3. EncodeIDs([r]) == the L2-normalized single row; EncodeIDs([a,b]) == L2-normalize(mean(row_a,row_b));
   EncodeIDs([]) == zero vector length Dim; an out-of-range id is skipped, not a panic;
   with Normalize=false the raw mean is returned (no normalization).
4. internal/search stays cobra-free + net/http-free + CGO-free; full -race suite green;
   gofmt/vet clean; no new module deps (CGO_ENABLED=0 build succeeds).
