# TDD Plan: Increment 5b - okfctl-search (semantic plugin)

Spec: docs/specs/2026-07-24-okfctl-search.md
Branch: topic/okfctl-search  Base: main @ 6180825
Execution: sequential TDD; per-task ground-truth verification + one whole-increment
E2E review on the built PLUGIN binary that gates the PR. Each task RED -> GREEN ->
REFACTOR -> commit (-S signed, Conventional, Copyright 2026 Google LLC header
pre-written into new .go files so addlicense is a no-op). Verify by real `go test`
+ SHA + signature.

## Task 0 - docs
Commit spec + this plan. `docs(5b): spec + TDD plan for okfctl-search semantic plugin`.

## Task 1 - internal/search: Embedder contract + HashEmbedder + cosine (pure, stdlib-only)
RED: internal/search/embed_test.go
  - TestHashEmbedder_Deterministic: same text -> identical vector across two Encode calls.
  - TestHashEmbedder_L2Normalized: non-empty text vector has ~unit norm; empty text -> zero vector.
  - TestHashEmbedder_MatchesKBProtocol: for a fixed input (e.g. "tannin structure wine"),
    the Go vector equals the KB Python HashEmbedder vector (hard-code the expected
    vector captured from the KB embedder; asserts byte-for-byte protocol fidelity).
  - TestCosine: orthogonal -> 0, identical unit -> 1, zero-norm operand -> 0.0.
  - TestHashEmbedder_NameDim: Name()=="hash-test-embedder", Dim()==64.
GREEN: internal/search/embed.go
  - Embedder interface {Name() string; Dim() int; Encode([]string) [][]float64}.
  - HashEmbedder{dim,name}: per token sha1 -> idx = first4bytes % dim, sign from
    byte[4]%2, accumulate, L2-normalize. Ported from KB embed.py HashEmbedder.
  - cosine(a,b []float64) float64.
Commit: `feat(search): Embedder contract + HashEmbedder + cosine (KB protocol port)`.

## Task 2 - internal/search: flat vector Store (build/load, model+dim guard)
RED: internal/search/store_test.go
  - TestStore_RoundTrip: BuildIndex over a fixture bundle -> Store records model,
    dim, one Entry{path,hash,vector} per concept node; Save then Load == equal.
  - TestStore_Deterministic: two BuildIndex runs (fixed embedder) -> byte-identical
    serialization.
  - TestStore_ContentHashSkip: rebuild with one node changed -> only that node's
    vector changes; unchanged nodes keep prior hash+vector (no re-embed).
  - TestStore_ModelMismatchGuard: a Store recorded model="A" loaded for query with
    an embedder Name()="B" -> Query returns a mismatch error (never compares).
GREEN: internal/search/store.go
  - Store{Model string; Dim int; Entries []Entry}; Entry{Path,Hash string; Vector []float64}.
  - BuildIndex(b *okf.Bundle, e Embedder, prev *Store) *Store - concept nodes only;
    reuse prev entry when content hash unchanged; deterministic order (sort by path).
  - Save(path)/Load(path) JSON; ErrModelMismatch.
  Reuse okf bundle load + node content hashing already in internal/okf.
Commit: `feat(search): flat vector store with content-hash skip + model guard`.

## Task 3 - internal/search: Query + Related (brute-force cosine, top-K)
RED: internal/search/query_test.go
  - TestQuery_RanksClosestTop: build over a small fixture; a query near node X ranks
    X first; results sorted by score desc; K honored.
  - TestQuery_ModelMismatch: query with embedder != store.Model -> ErrModelMismatch.
  - TestRelated_ExcludesSelf: Related(nodePath) returns neighbors sorted by score,
    the node itself excluded; unknown path -> error.
GREEN: internal/search/query.go
  - Query(s *Store, e Embedder, q string, k int) ([]Result, error) - embed q, cosine
    over entries, sort desc, top-k. Result{Score float64; Path, Title string}.
  - Related(s *Store, nodePath string, k int) ([]Result, error) - use the stored
    vector for nodePath, cosine over the rest, exclude self.
Commit: `feat(search): semantic Query + Related over the flat store`.

## Task 4 - cmd/okfctl-search plugin binary + README + full gate
RED: cmd/okfctl-search/main_test.go (drive the plugin's command tree via its runner)
  - TestPlugin_IndexBuildThenSemantic: temp bundle -> `index build` writes
    .okfctl/index.db (model+dim recorded) -> `--semantic "q"` prints ranked paths.
  - TestPlugin_Related: `related <node>` prints neighbors, self excluded.
  - TestPlugin_ModelFlagModel2vecNotYet: `--embedder model2vec` errors
    "not yet available (increment 5c)".
GREEN: cmd/okfctl-search/main.go
  - cobra (or flag) command tree: `index build`, root `--semantic`, `related`,
    `--embedder hash|model2vec` (hash default; model2vec -> honest not-yet error),
    `--k` (default 5), positional bundle-dir default ".".
  - go:build a SEPARATE binary; discoverable on PATH as okfctl-search.
  - README: okfctl-search section (git/kubectl plugin; offline HashEmbedder default;
    model2vec deferred to 5c; index build/semantic/related; store records model+dim).
  - Full gate: gofmt -l, go vet, go build ./..., go mod tidy -diff (no new deps),
    internal/search has NO cobra/net-http/CGO import, full -race suite.
Commit: `feat(cmd): okfctl-search plugin binary + README + full gate`.

## Whole-increment review (gates the PR)
Build BOTH binaries (okfctl + okfctl-search); put okfctl-search on a temp PATH;
exercise via 5a dispatch (`okfctl search --semantic ...`) AND directly: index build
(records model+dim, deterministic, content-hash skip), --semantic ranks a near node
top, related excludes self, model-mismatch guard refuses, model2vec honest error,
HashEmbedder==KB output. Then push -> file card -> draft PR -> wired review lane ->
Casey acceptance.
