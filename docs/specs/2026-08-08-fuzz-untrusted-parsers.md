# test(fuzz) — native Go fuzz targets for the untrusted-input parsers

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` **v0.2** §5.1 (links), §7 (frontmatter/type),
§8 (index files). No new spec behavior — this adds coverage, not rules.
**Base:** `main` @ `e56f58d`  **Branch:** `topic/fuzz-untrusted-parsers`

## Problem

The repo has zero `func Fuzz` targets. `okfctl` parses input it does not
control — YAML frontmatter, Markdown bodies (link scanning + edge building), and
HTTP query params in the apiserver — and example-based tests only cover the
shapes the author imagined. A malformed input that panics one of these parsers is
a crash the suite cannot currently surface. Coverage is also weakest exactly
where untrusted input arrives.

## Design

Add native Go fuzz targets (`func FuzzXxx(f *testing.F)`) for the three
untrusted-input surfaces, each seeded from real fixtures, and wire a bounded
fuzz pass into CI so a regression that introduces a crasher fails the build
without slowing the normal test run.

The parsers under test, and the invariant each fuzz target asserts (a fuzz
target's real job is "never panic," but a cheap invariant catches silent
corruption too):

| surface | function under test | invariants asserted |
|---|---|---|
| frontmatter | `okf.ParseFrontmatter([]byte)` | never panics; no leading fence ⇒ body == input & empty non-nil map; err ⇒ nil map & empty body; `splitFrontmatterRaw` round-trips (yaml+rawAfter reconstructs the post-fence bytes) |
| node / Markdown | `okf.Load(dir)` over a bundle containing the fuzzed `.md` | never panics; `OutboundLinks` for the fuzzed node returns only keys that exist in `Nodes` |
| apiserver query params | `searchService.handle` via `httptest` with a fuzzed raw query; and the pure `canonicalizeQuery`/`suggestParam`/`levenshtein` key logic | never panics; status is always a valid HTTP code; `levenshtein` is symmetric and identity-zero |

Targets live in the package under test (`package okf`, `package apiserver`) so
they can exercise unexported helpers (`splitFrontmatterRaw`, `canonicalizeQuery`).
The apiserver target builds ONE bundle + on-disk index at `f.Fuzz` setup time,
not per-iteration, so the fuzz loop stays fast; only the raw query string is
mutated.

### Seed corpora — from real fixtures

Each target's `f.Add` seeds are drawn from the shapes already used in the
package's tests: the frontmatter test cases (`---\ntype: Reference\n...`, the
malformed `[unterminated`, no-frontmatter, in-body `---` rule), the search
bundle's node bodies with real `[text](path.md)` links, and the query grammar
from `search_params_test.go` (unknown key, separator aliases, `k`/`half_life`/
`decay_floor`/`min_relevance`/`lexical_gate` values, percent-encoding).

### CI — bounded fuzz pass with both controls

A new `fuzz` job in `.github/workflows/ci.yml` runs each target for a bounded
`-fuzztime=30s` on PRs. Because Go's `go test -fuzz` runs ONE target per package
invocation, the job iterates the (package, target) pairs.

- **Positive control:** a target `FuzzPositiveControl` with a seed input that a
  helper deliberately panics on proves the fuzz job actually fails the build when
  a crasher exists — run in a step asserted to exit non-zero.
- **Negative control:** the three real targets run on normal seeds and pass.

## Non-goals

- No behavior change to any parser. If the fuzzer finds a crasher, THAT is fixed
  (TDD-first, regression test per crash) — but the target itself asserts current
  behavior, it does not tighten it.
- Not fuzzing the search ranking math or the embedder (not untrusted-input
  parsers).

## Definition of done

Fuzz targets for frontmatter, node/Markdown, and apiserver query params; each
seeded from real fixtures; a bounded CI fuzz pass with a proven positive control
(deliberate crasher fails the job) and negative control (normal input passes);
any crasher found is fixed with a regression test; full suite green under `-race`.
