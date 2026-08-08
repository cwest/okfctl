# Plan — fuzz targets for the untrusted-input parsers

Spec: `docs/specs/2026-08-08-fuzz-untrusted-parsers.md`. TDD-first: each fuzz
target is written, run against its seed corpus (RED = a crasher surfaces or a
real RED signal from an invariant), then GREEN once the parser survives the
seeds. A `go test -run=<Fuzz>` (no `-fuzz`) run executes every target against its
seed corpus as ordinary tests, so the whole suite proves the seeds pass on every
CI run even outside the dedicated fuzz job.

## Task 1 — `internal/okf/frontmatter_fuzz_test.go` (`FuzzParseFrontmatter`)

- Seeds (`f.Add`) from `frontmatter_test.go`: well-formed, no-frontmatter,
  malformed `[unterminated`, in-body `---` rule, CRLF, empty, bare `---`.
- Body: call `ParseFrontmatter(data)` — assert it never panics; when err==nil and
  the input has no `---\n` prefix, body must equal the input and map non-nil;
  when err!=nil, map must be nil and body "". Also drive `splitFrontmatterRaw`
  and assert `yamlBlock`+`rawAfter` reconstruct the bytes after the opening fence.

## Task 2 — `internal/okf/node_fuzz_test.go` (`FuzzLoadBundle`)

- Seeds from the search bundle node bodies with real `[text](wine/tannin.md)`
  links, an image `![a](x.png)`, an external link, a `/`-absolute link, and a
  broken link.
- Body: write a minimal 2-node bundle into `t.TempDir()` where one node's body is
  the fuzzed bytes, run `okf.Load(dir)`, assert no panic; every path in
  `OutboundLinks` of the fuzzed node is a key in `b.Nodes` (link resolution can
  never emit a dangling edge). Uses `testing.F`'s per-seed `t.TempDir()`.

## Task 3 — `internal/apiserver/search_fuzz_test.go`

- `FuzzSearchQueryParams(rawQuery string)`: build one bundle+index+handler at
  target setup (reuse `searchBundleDir`/`buildIndex`/`NewHandler`), then for each
  fuzzed `rawQuery` issue `GET /api/v1/search?<rawQuery>` via `httptest`; assert
  no panic and `100 <= code <= 599`. Seeds from `search_params_test.go` query
  strings.
- `FuzzCanonicalizeQuery(key string)`: exercise the pure key logic — build
  `url.Values{key: {"v"}}`, run `canonicalizeQuery` against a recorder, assert no
  panic; separately assert `levenshtein(a,b)==levenshtein(b,a)` and
  `levenshtein(a,a)==0` on the fuzzed key vs the canonical set.

## Task 4 — crasher triage

Run each target `go test -fuzz=<Name> -fuzztime=30s`. Any crasher: `systematic-
debugging` root-cause → regression test (the crasher input added as an `f.Add`
seed or a focused unit test) RED → minimal fix → GREEN. Pin before/after in PR.

## Task 5 — CI (`.github/workflows/ci.yml`)

- Add a `fuzz` job (Go 1.26) that runs each real target `-fuzztime=30s`.
- Positive control: a `FuzzPositiveControl` target whose seed trips a deliberate
  panic, run in a step wrapped so a NON-failure fails CI (`if go test -fuzz... ;
  then exit 1; fi`) — proves the fuzz gate actually catches a crasher.
- Negative control: the three real targets pass on normal seeds (the job's main
  steps).

## Verify

`gofmt -l .` empty · `go vet ./...` · `CGO_ENABLED=0 go build ./...` ·
`go test ./... -race` green · each `go test -run=Fuzz <pkg>` green (seeds pass) ·
positive control proven to fail, negative proven to pass · real-corpus run
(build binary, validate+lint a real bundle) per AGENTS.md layer 3.
