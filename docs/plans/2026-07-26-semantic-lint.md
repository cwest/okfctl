# TDD Plan: Increment 7 — semantic lint checks

Spec: `docs/specs/2026-07-26-semantic-lint.md`
Branch: `topic/semantic-lint`  Base: `main` @ 911bb60
Execution: sequential TDD; each task RED → GREEN → REFACTOR → commit (-S signed,
Conventional, Apache header pre-written into new .go files). Verify by real
`go test` + SHA + signature. Real-model fidelity check at the end, gated on
`OKFCTL_TEST_MODEL_DIR`.

## Task 0 — docs
Commit spec + this plan. `docs(7): spec + TDD plan for semantic lint checks`.

## Task 1 — the semantic checks in internal/okf (pure, index-shaped input)
Keep `internal/okf` free of an `internal/search` import by defining the neighbor
input as a small local type, so the checks are unit-testable with hand-built
data and the dependency arrow points one way (cmd wires okf + search together).

RED: `internal/okf/lint_semantic_test.go`
  - `Neighbor{Path string; Score float64}`; `SemanticIndex = map[string][]Neighbor`
    (each node's ranked neighbors, self excluded).
  - `TestLintSimilarUnlinked_Reports`: two nodes at 0.91, no link either way →
    exactly ONE finding (`similar-unlinked`) on the lexicographically-first path,
    message naming the other path + score.
  - `TestLintSimilarUnlinked_SuppressedWhenLinked`: same pair, A links B → none.
    Also suppressed when only B links A (either direction counts).
  - `TestLintSimilarUnlinked_BelowThreshold`: 0.79 with threshold 0.80 → none.
  - `TestLintNoSemanticNeighbors`: node whose best neighbor is 0.12 (floor 0.30)
    → one `no-semantic-neighbors` finding; a node with a 0.55 neighbor → none.
  - `TestLintSemantic_Deterministic`: same input twice → identical slices.
GREEN: `internal/okf/lint_semantic.go` — `LintSemantic(b *Bundle, idx SemanticIndex,
  opts SemanticOptions) []LintFinding`, reusing the existing `linkedTargets` helper
  for edge detection (one mechanism, not a second link parser). Pair dedupe via a
  canonical (min,max) key.
Commit: `feat(okf): similar-unlinked + no-semantic-neighbors lint checks`.

## Task 2 — stale-index detection
RED: add to the same test file
  - `TestLintSemantic_StaleIndex`: bundle has a node absent from the index → ONE
    bundle-level `stale-index` finding (Path "") listing the missing path(s),
    sorted; and the present nodes are still checked normally.
  - `TestLintSemantic_NoStaleWhenComplete`: every node indexed → no such finding.
GREEN: extend `LintSemantic`.
Commit: `feat(okf): report stale-index drift in semantic lint`.

## Task 3 — wire `lint --semantic` in cmd
RED: `cmd/lint_semantic_test.go`
  - `TestLint_SemanticFlagMissingIndex`: `lint --semantic` with no index → error
    naming `okfctl-search index build`, exit non-zero (NOT a silent skip).
  - `TestLint_WithoutSemanticUnchanged`: byte-identical output to plain `lint`
    on a bundle that HAS an index (proves opt-in, no accidental read).
  - `TestLint_SemanticFindsUnlinkedPair`: build a real index over a tiny bundle
    with the hash embedder (deterministic, no model needed), duplicate-ish text
    → a `similar-unlinked` finding appears in output.
  - `TestLint_SemanticThresholdFlags`: `--similarity-threshold` / `--isolation-floor`
    change the finding set.
GREEN: `cmd/lint.go` — add `--semantic`, `--similarity-threshold`,
  `--isolation-floor`; load `.okfctl/index.db` via `internal/search`, build the
  `SemanticIndex` by calling `search.Related` per node (k = len(nodes)-1), pass to
  `okf.LintSemantic`, merge findings, keep existing sort + `--strict` semantics.
Commit: `feat(cmd): wire lint --semantic over the search index`.

## Task 4 — README + full gate
- README: `lint --semantic` section — what the two checks mean, the
  `index build` prerequisite, the thresholds, and that core reads (never builds)
  an index so no model is needed to lint.
- Full gate: `gofmt -l`, `go vet`, `go build ./...`, `go mod tidy -diff` (NO new
  deps), `CGO_ENABLED=0` build, full `-race` suite.
- Real-model E2E (gated on `OKFCTL_TEST_MODEL_DIR`): index a wine bundle with
  potion-base-8M; confirm a near-duplicate unlinked pair flags and an unrelated
  node isolates; confirm the hash embedder does NOT produce the same quality
  (evidence the check is genuinely semantic).
Commit: `feat(cmd): document lint --semantic + full gate`.

## Whole-increment review (gates the PR)
Structural output unchanged without the flag; semantic findings correct and
deterministic with it; missing index errors actionably; stale index surfaced;
CGO-free; no new deps; full `-race` green both with and without the real model.
Then push → file card → draft PR → review lane → Casey acceptance.
