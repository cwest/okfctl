# TDD Plan: Increment 5c-1 - Model2Vec safetensors loader + StaticModel

Spec: docs/specs/2026-07-24-model2vec-loader.md
Branch: topic/model2vec-loader  Base: main @ 06941a2
Execution: sequential TDD; per-task ground-truth verification + one whole-increment review.
Each task RED -> GREEN -> REFACTOR -> commit (-S signed, Conventional, Copyright 2026 Google
LLC header pre-written into new .go files so addlicense is a no-op). Verify by real `go test`
+ SHA + signature.

## Task 0 - docs
Commit spec + this plan. `docs(5c-1): spec + TDD plan for Model2Vec safetensors loader`.

## Task 1 - ReadSafetensorsMatrix (pure-Go safetensors reader)
RED: internal/search/safetensors_test.go
  - mkSafetensors helper: build a valid safetensors []byte for a given [V][D]float32 matrix
    (u64 LE header-len + JSON header {"embeddings":{dtype:F32,shape:[V,D],data_offsets:[0,V*D*4]}}
    + raw LE float32 data). Write to a temp file.
  - TestReadSafetensorsMatrix_RoundTrip: a known [3,4] matrix decodes to the exact float64 rows;
    returned dim==4.
  - TestReadSafetensorsMatrix_RejectsNonF32: a header claiming dtype "F16" -> error mentioning dtype.
  - TestReadSafetensorsMatrix_RejectsTruncated: data section shorter than V*D*4 -> error.
GREEN: internal/search/safetensors.go
  - read first 8 bytes -> u64 LE header length; read that many bytes -> JSON header; find
    "embeddings"; assert dtype=="F32" and len(shape)==2; seek to 8+hdrlen+start; read
    (end-start) bytes; decode as LE float32 -> [][]float64 of shape [V][D].
Commit: `feat(search): pure-Go safetensors F32 matrix reader`.

## Task 2 - StaticModel: LoadStaticModel + config cross-check
RED: internal/search/model2vec_test.go
  - mkModelDir helper: write dir/config.json ({"hidden_dim":4,"normalize":true}) +
    dir/model.safetensors (via mkSafetensors) for a known [3,4] matrix.
  - TestLoadStaticModel_ReadsConfigAndMatrix: Dim==4, Normalize==true, len(Rows)==3.
  - TestLoadStaticModel_DimMismatchErrors: config hidden_dim=5 but matrix dim=4 -> error.
GREEN: internal/search/model2vec.go
  - StaticModel{Rows [][]float64; Dim int; Normalize bool}; LoadStaticModel(dir) reads
    config.json (hidden_dim, normalize) + ReadSafetensorsMatrix(dir/model.safetensors);
    cross-check dim==hidden_dim.
Commit: `feat(search): Model2Vec StaticModel loader with config cross-check`.

## Task 3 - EncodeIDs (gather + mean-pool + normalize) + full gate
RED: append to model2vec_test.go
  - TestEncodeIDs_SingleRowNormalized: EncodeIDs([1]) == L2-normalize(Rows[1]).
  - TestEncodeIDs_MeanPool: EncodeIDs([0,2]) == L2-normalize(elementwise mean(Rows[0],Rows[2])).
  - TestEncodeIDs_Empty: EncodeIDs(nil) == make([]float64, Dim) (all zeros).
  - TestEncodeIDs_OutOfRangeSkipped: EncodeIDs([0, 999]) == EncodeIDs([0]) (999 skipped, no panic).
  - TestEncodeIDs_NoNormalize: with Normalize=false, EncodeIDs([0,1]) == raw mean (unnormalized).
GREEN: (m *StaticModel) EncodeIDs(ids []int) []float64—accumulate gathered rows, divide by
  count, L2-normalize iff m.Normalize; empty/all-skipped -> zero vector length Dim.
  Full gate: gofmt -l, go vet, go build ./..., go mod tidy -diff (no new deps),
  internal/search no cobra/net-http/CGO import, CGO_ENABLED=0 build, full -race suite.
Commit: `feat(search): Model2Vec EncodeIDs gather + mean-pool + normalize + full gate`.

## Whole-increment review (gates the PR)
Unit-level E2E: build a synthetic StaticModel, assert the three inference identities
(single/mean/empty) hold on the built package; confirm CGO_ENABLED=0 builds; confirm
--embedder model2vec still returns the honest 5b "not yet available" error (5c-2 wires it).
Then push -> file card -> draft PR -> wired review lane -> Casey acceptance.
