# Plan: `--lexical-gate` (TDD, per-task)

Spec: `docs/specs/2026-08-03-lexical-gate.md`. Base `main` @ ac8a7a5.

Each task is RED → GREEN → REFACTOR. Run `gofmt -w` on written files before
staging. Full suite green under `-race` before handoff.

## Task 1 — tokenizer + light stemmer (`internal/search/lexgate.go`)

RED: `internal/search/lexgate_test.go`
- `lexTerms("how should an agent decide when to delegate work")` drops stopwords,
  returns content terms (`agent`, `decide`, `delegate`, `work`) stemmed.
- `lexTerms("how should the")` → empty (all stopwords).
- `stem("hashes") == stem("hash")` and `stem("agents") == stem("agent")`.
- `stem` leaves short tokens intact (min stem length): `stem("is")` unchanged.

GREEN: implement `lexTerms(string) []string` and `stem(string) string`.

## Task 2 — node lexical match set against a bundle (`lexgate.go`)

RED:
- `lexicalMatch(bundle, terms)` returns the set of node paths where any stemmed
  query term equals a stemmed title+body token. `hash` and `hashes` return
  overlapping sets on a fixture with a `hash`/`hashes` node.
- A term matching zero nodes returns an empty set.

GREEN: implement `lexicalMatch`. Reuse `okf.Bundle.Nodes`, node title+body.
Tokenize node text with the same tokenizer as the query.

## Task 3 — gate composition in `QueryWith` (`query.go`)

RED: `internal/search/lexgate_test.go` (engine-level, hash embedder, fixture)
- With gate on, an exact-token node outside the semantic top-1 ranks 1.
- Gate on + empty terms == gate off (byte-identical result slice).
- Gate on + over-broad term (matches > fraction of fixture) == gate off.
- Lexical-only hit (in lexical set, below semantic top-N) is APPENDED, not dropped.
- Gate off (default) == current `QueryWith` output (byte-identical).

GREEN: add `LexicalGate *LexicalGateOptions` to `QueryOptions`
(`{Terms []string, Match map[string]bool, OverBroadFraction float64, WideN int}`)
or resolve terms/match inside. Prefer resolving the match set in the command
(needs the bundle) and passing `Match` + `Terms` + `WideN` in, keeping the engine
pure. Compose: rank wide (WideN), intersect in semantic order, append lexical-only
in path order, cut to k. Degrade when Terms empty or |Match| > fraction*|nodes|.

## Task 4 — CLI wiring (`cmd/okfctl-search/main.go`)

RED: `cmd/okfctl-search/main_test.go`
- `--lexical-gate` is a real flag; without it, output is byte-identical to bare
  (default-off control, mirrors `TestPlugin_HalfLifeAcceptedAndUnsetUnchanged`).
- `--lexical-gate` with `--path` and with `--half-life`: no error, non-empty
  sane output (interaction smoke).
- All-stopword query with `--lexical-gate` == without it.

GREEN: add `--lexical-gate` bool flag. When on: load the bundle (extend
`needBundle`), compute `lexTerms` + `lexicalMatch`, pass into `QueryOptions`.
Update `--help`.

## Task 5 — eval negative control (`internal/search/eval_test.go`)

Extend `TestEval_RetrievalQuality` (or add `TestEval_LexicalGate`) to run the
gold set with the gate ON and assert MRR/recall@5 no worse than gate OFF, on both
embedders. Skipped unless `OKFCTL_EVAL_CORPUS` + `OKFCTL_TEST_MODEL_DIR` set.
Add a positive fixture query with a verbatim-unique token asserting rank-1 with
the gate.

## Task 6 — docs + conformance

- README: document `--lexical-gate`, the degrade rule, and the over-broad
  fraction, co-locating the rationale with the constant.
- Run all three AGENTS.md conformance layers + real-corpus before/after + both
  embedder eval numbers. Pin in PR body.
