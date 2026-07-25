# TDD Plan: Increment 5c-2 - WordPiece tokenizer + wire --embedder model2vec

Spec: docs/specs/2026-07-25-wordpiece-tokenizer.md
Branch: topic/wordpiece-tokenizer  Base: main @ f0b72d9
Execution: sequential TDD; per-task ground-truth verification + one whole-increment review
(unit + real-potion-base-8M fidelity). Each task RED -> GREEN -> REFACTOR -> commit (-S signed,
Conventional, Copyright 2026 Google LLC header pre-written into new .go files). Verify by real
`go test` + SHA + signature. Fidelity anchors captured from the live model (in the spec).

## Task 0 - docs
Commit spec + this plan. `docs(5c-2): spec + TDD plan for WordPiece tokenizer`.

## Task 1 - extract config reader to internal/okfconfig (shared, one mechanism)
The increment-1 config reader (configPath/loadConfig/saveConfig) is private in package cmd;
the okfctl-search plugin (separate `package main` binary) can't import it. Extract the READ
side into internal/okfconfig so BOTH core cmd and the plugin use ONE config mechanism (no
duplicate JSON path/logic).
RED: internal/okfconfig/config_test.go
  - TestPath_HonorsEnvOverride: OKFCTL_CONFIG_HOME set -> Path() == <that>/config.json.
  - TestLoad_MissingIsEmpty: no file -> Load() returns empty map, nil err.
  - TestLoad_RoundTrip: write a config.json {"model_path":"/x"} -> Load()["model_path"]=="/x".
GREEN: internal/okfconfig/config.go — exported Path() string + Load() (map[string]string, error),
  lifted verbatim from cmd/config.go (same OKFCTL_CONFIG_HOME / UserConfigDir logic, stdlib json).
REFACTOR: rewrite cmd/config.go's configPath/loadConfig/saveConfig to delegate to okfconfig
  (Path/Load; saveConfig keeps its write but uses okfconfig.Path). Run the existing cmd/config
  tests to prove no behavior change.
Commit: `refactor(config): extract shared JSON config reader to internal/okfconfig`.

## Task 2 - WordPiece tokenizer (BertNormalizer + BertPreTokenizer + WordPiece)
RED: internal/search/wordpiece_test.go
  - mkVocab helper: write a tiny vocab.txt (id=line) covering the anchor tokens so unit tests
    don't need the 30MB model; plus a LoadWordPiece from a temp dir.
  - TestTokenize_Anchors: for a hand-built vocab, "tannin structure"->expected ids;
    lowercase ("Wine"->wine id); "oaky"->oak + ##y (WordPiece split); ""->[]; NO [CLS]/[SEP].
  - TestTokenize_UnknownWord: a word absent from vocab -> [unk id]; a >100-char word -> [unk id].
  - TestTokenize_PunctSplit: "notes." -> "notes" + "." as separate tokens.
GREEN: internal/search/wordpiece.go
  - normalize (clean_text: strip control + collapse whitespace; handle_chinese_chars: space-pad
    CJK; lowercase) -> pre-tokenize (whitespace + punct-split) -> WordPiece greedy longest-match
    (## continuation, [UNK], max 100 chars). LoadWordPiece(dir) reads vocab.txt (id=line) +
    unk/prefix (BERT defaults; cross-check tokenizer.json if present). Tokenize returns []int
    content ids (NO specials).
Commit: `feat(search): pure-Go BERT WordPiece tokenizer`.

## Task 3 - Model2VecEmbedder (Embedder impl) + real fidelity test
RED: internal/search/model2vec_embedder_test.go
  - TestModel2VecEmbedder_Interface: satisfies Embedder (Name/Dim/Encode).
  - TestModel2VecEmbedder_Fidelity (gated on potion-base-8M present; skip w/ clear msg if absent):
    LoadModel2VecEmbedder(<potion dir>); Encode(["tannin structure"])[0] == the captured anchor
    within 1e-5 (dim 256, unit norm, first4 [0.236271,-0.08241,-0.142059,-0.152239]). This is
    the REAL end-to-end fidelity proof (Go tokenize+EncodeIDs == model2vec.encode).
GREEN: internal/search/model2vec_embedder.go
  - Model2VecEmbedder{model *StaticModel; tok *WordPiece; name string} implementing Embedder;
    Encode(texts) = model.EncodeIDs(tok.Tokenize(t)) per text. LoadModel2VecEmbedder(dir) =
    LoadStaticModel(dir) + LoadWordPiece(dir).
Commit: `feat(search): Model2VecEmbedder (tokenize -> embed) + potion-base-8M fidelity test`.

## Task 4 - wire --embedder model2vec (config-first model_path) + README + full gate
RED: cmd/okfctl-search/model2vec_test.go
  - TestResolveModel2vec_FlagWins / ConfigFallback / ErrorWhenUnset: resolveEmbedder("model2vec")
    picks --model-path, else okfconfig model_path, else a clear error naming
    `okfctl config set model_path`.
  - TestResolveModel2vec_LoadsEmbedder (gated on model present): resolves to a Model2VecEmbedder.
GREEN: cmd/okfctl-search/main.go
  - resolveEmbedder("model2vec"): dir = --model-path || okfconfig.Load()["model_path"]; if empty
    -> error "set model_path via `okfctl config set model_path <dir>` or pass --model-path";
    else LoadModel2VecEmbedder(dir). Add --model-path persistent flag. hash stays default.
  - README: model2vec section — offline hash default; model2vec needs a local model dir set via
    `okfctl config set model_path <dir>` (or --model-path); no runtime download; JSON config.
  - Full gate: gofmt -l, go vet, go build ./..., go mod tidy -diff (NO new deps),
    internal/search + internal/okfconfig cobra/net-http/CGO-free, CGO_ENABLED=0 build, full -race.
Commit: `feat(cmd): wire --embedder model2vec via config-first model_path + README + full gate`.

## Whole-increment review (gates the PR)
Build both binaries. Unit: tokenizer anchors + config resolution. Real-model (if potion-base-8M
cached): `okfctl config set model_path <dir>` then `okfctl-search --embedder model2vec index build`
+ `--semantic "tannin structure"` returns a sensible ranking; Encode fidelity == model2vec within
1e-5. Missing model_path -> clear error (no panic, no silent hash fallback). hash default
unchanged. CGO_ENABLED=0 builds; no new deps. Then push -> file card -> draft PR -> review lane
-> Casey acceptance.
