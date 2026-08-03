# okfctl-api — serve `GET /api/v1/search` off a resident index

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` **v0.2** §4.1 (scoping filters), §5.2
(generated), §13.1 (v0.1 fallbacks). Upstream: GoogleCloudPlatform/knowledge-catalog
`okf/SPEC.md`.
**Base:** `main` @ `ac8a7a5`  **Branch:** `wt/t_7acd34d4`
**Closes:** cwest/okfctl#64

## Problem

`okfctl-api` serves `/api/v1/stats` and `/api/v1/graph` but has no query surface:
`GET /api/v1/search` returns 404 (`internal/apiserver/handler.go` registered
exactly two routes). Meanwhile the search CLI reloads the index from disk on every
invocation (`cmd/okfctl-search/main.go` calls `search.Load` inside `RunE`), so a
consumer running many queries pays the reload cost each time. This increment adds
the missing endpoint, holding the loaded index + embedder for the process
lifetime.

## Design

All changes live in `internal/apiserver/` (new `search.go` + tests) and the
`cmd/okfctl-api` serve wiring; the shared `okf.BuildGraph` serializer and the
`search` package are NOT touched — the API composes `search.QueryWith`, so its
ranking can never disagree with the CLI's.

### Endpoint

```
GET /api/v1/search?q=<query>&k=<n>&path=<prefix>&type=<t>&tag=<t>&half_life=<days>
```

Returns the same score / path / snippet triple the CLI prints, as JSON:

```json
{
  "schema": 1,
  "query": "natural wine tannin",
  "model": "hash-test-embedder",
  "indexed_at": "2026-08-03T03:27:14Z",
  "results": [
    {"score": 0.29, "path": "domains/trademark-vs-domain-exposure.md", "snippet": "..."}
  ]
}
```

`q` is the **semantic** query (the CLI `--semantic`). `k`/`path`/`type`/`tag`/
`half_life` mirror the CLI's `--k`/`--path`/`--type`/`--tag`/`--half-life`
flags one-for-one. `model` is the index's own recorded embedder model; `indexed_at`
is the index file's modtime, so a consumer can tell how fresh the answers are.

### Query-param grammar — a deliberate divergence from the earlier plan

The earlier design (`docs/plans/2026-08-02-okfctl-api.md` §2.7) sketched a
two-mode endpoint where `?q=` meant **lexical** and `?semantic=` meant semantic.
This increment ships `?q=` as the **semantic** query instead, because the
acceptance criterion is *exact equivalence to the semantic CLI's score/path/
snippet triple*, and `q` is the natural name for "the query." The lexical mode is
not part of this increment. This is called out for the reviewer rather than
resolved silently.

### Filter grammar — mirrors #68's repeatable + negating grammar

This endpoint touches the same query surface as #63 (repeatable + negating
filters) and #65 (decay bounds). The card's ordering dependency says: land the
current grammar, and if #63 merges first, mirror its grammar rather than invent
one. **#68 merged that grammar (repeatable `--path/--type/--tag`, negating
`--not-path/--not-type/--not-tag`, `search.Filter` scalar fields → slices) before
this endpoint landed**, so this endpoint mirrors it:

- `?path=`, `?type=`, `?tag=` are **repeatable**; repeats within a dimension OR
  together, dimensions AND together — Go's `url.Query()[k]` returns every
  occurrence, so this is a natural extension, not new syntax.
- `?not-path=`, `?not-type=`, `?not-tag=` mirror the CLI's exclusion flags.
- Empty values (`?type=`) are dropped by `nonEmptyQuery`, the HTTP mirror of the
  CLI's `nonEmpty` — an empty value reads as unset, identical to omitting the
  param, preserving the CLI's contract.

Decay behavior is unchanged (`?half_life=` scalar, #65 not yet landed).
Flagged to the reviewer.

### Two decisions made explicitly

- **Index staleness.** The service stats `.okfctl/index.db` per request and
  reloads the store on modtime change; N requests against an unchanged index load
  the index from disk exactly once (the resident-server invariant). `indexed_at`
  exposes the index modtime.
- **Filter metadata.** §4.1 filters and §5.2/§13.1 recency decay resolve against
  the LIVE bundle (re-walked on the same reload signal), not the index —
  `contentHash` keys only title+body, so a frontmatter-only edit does not re-embed
  and a value denormalized onto the index would go stale.

### Constructor signature

`NewHandler(b *okf.Bundle)` becomes `NewHandler(b *okf.Bundle, embedder
search.Embedder)`. A nil embedder disables `/search` (it 404s) and leaves
`/stats` and `/graph` byte-identical — the search route is purely additive.

## Security posture (unchanged, re-asserted in tests)

Loopback-only bind (non-loopback `--addr` still refused with the new route
registered), read-only bundle, GET-only (`POST /api/v1/search` → 405).

## Acceptance criteria — both directions

- **Positive:** `?q=<query>&k=5` returns 200 with results whose score/path/snippet
  exactly match `search.QueryWith` for the same query and bundle (the CLI oracle).
- **Negative — no perturbation:** `/stats` and `/graph` bodies are byte-identical
  with search enabled vs disabled.
- **Second negative:** a bundle with no index → clean 503 with the CLI's
  `no index at <path> (run 'okfctl-search index build' first)` message, not a panic
  and not a 200 with `[]`.
- Missing/empty `q` → 400. `POST` → 405. Loopback refusal still holds.
- **Staleness:** a rebuild while resident is reflected on the next request
  (positive control); an unchanged index loads exactly once over N requests
  (negative control — proves the feature).
- **Measured benefit** on the real corpus, pinned in the PR body.
