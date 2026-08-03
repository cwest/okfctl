# okfctl-api — apply the recency-decay floor in `GET /api/v1/search`

**Status:** Approved (design) · **Owner:** Casey West · **License:** Apache-2.0
**Source of truth:** OKF `SPEC.md` **v0.2** §5.2 (generated), §13.1 (v0.1
fallbacks). Prior art: cwest/okfctl#65/#67 (CLI decay floor), #72 (CLI
`--decay-floor` bounds validation).
**Base:** `main` @ `595f954`  **Branch:** `topic/api-decay-floor`
**Closes:** cwest/okfctl#74

## Problem

`GET /api/v1/search` applied recency decay with NO lower clamp on the recency
multiplier, so the HTTP surface still did the thing #67 fixed on the CLI: a
strong old match is crushed toward zero (`0.5^(age/half_life)` → ~0 at large
age) and can be reordered below a weak fresh one. The two surfaces answered the
same query, bundle, index, and `half_life` differently.

The drift was a merge-order accident, not a bad decision in either PR. #65/#67
landed the `DecayFloor` clamp and defaulted it to `0.25` on the CLI; the HTTP
surface (`internal/apiserver/search.go`) built its `DecayOptions` with
`DecayFloor` left at its zero value (= unbounded) and `MinRelevance: 0`
hardcoded, and its comment still claimed those were "the exact DecayOptions the
CLI builds" — true when the endpoint landed (#69), false since #67.
Additionally, `?decay_floor=` and `?min_relevance=` were silently ignored:
HTTP 200, output byte-identical to omitting them, with no way for a caller to
opt in and no signal the parameter did nothing.

## Design

All changes live in `internal/search/query.go` (one shared constant),
`cmd/okfctl-search/main.go` (read the constant), and
`internal/apiserver/search.go` (default the floor, accept two validated params,
fix the stale comment) plus tests. The `search` engine math is untouched — the
API composes `search.QueryWith` exactly as the CLI does, so a fix on both call
sites is a fix to the shared behavior.

1. **One shared default.** `search.DefaultDecayFloor = 0.25` is defined in the
   `search` package — the package BOTH surfaces import. The CLI's `--decay-floor`
   flag default and the API's `decay_floor` default both read this one constant,
   so the two cannot drift on the next merge-order accident. This is the
   structural half of the fix; without it the same bug recurs.

2. **API defaults the floor.** `GET /api/v1/search` builds its `DecayOptions`
   with `DecayFloor: search.DefaultDecayFloor` (unless overridden), mirroring the
   CLI. An old-but-relevant node can be demoted by recency but never crushed
   below a mediocre fresh one.

3. **Two new validated params, matching #72's rules.**
   - `decay_floor` — the lower clamp on the recency multiplier. Validated in
     `[0, 1]`; out of range → HTTP 400 (a floor > 1 turns the clamp into a flat
     gain; a floor < 0 re-enables the #65 inversion). Omitted or empty
     (`?decay_floor=`) reads as unset and takes the default — the `nonEmptyQuery`
     contract already used for filters.
   - `min_relevance` — the raw-cosine floor applied BEFORE decay reorders.
     Validated non-negative; negative → HTTP 400. Default 0 (admit everything).
   Error messages preserve #72's constraint language (`must be in [0, 1]`,
   non-negative) while naming each surface's own identifier (`decay_floor` vs
   `--decay-floor`), consistent with the existing `half_life` message.

4. **Decay guard matches the CLI.** The endpoint builds `DecayOptions` iff
   `half_life > 0 || min_relevance > 0`, the same `needBundle`-adjacent guard the
   CLI uses. With both at their defaults the endpoint takes the plain-`Query`
   path — so with `half_life` ABSENT, no decay is applied and the response is
   byte-identical to today. The floor never engages without decay.

5. **Stale comment fixed.** The "exact DecayOptions the CLI builds" comment now
   describes what the code actually does.

## Testing

The load-bearing deliverable is a cross-surface equivalence harness that drives
the HTTP endpoint and the CLI-equivalent oracle (`search.QueryWith`) with the
SAME query, bundle, index, `half_life`, and `decay_floor`, and asserts IDENTICAL
score+path+snippet rankings. This is the test that would have caught the original
drift and stops the next one. It is table-driven over `half_life ∈ {0, 30, 90}`
× `decay_floor ∈ {default, 0, 0.5}`, on both the 3-node fixture and (env-guarded
by `OKFCTL_EVAL_CORPUS`, the same guard the retrieval-quality eval uses) the real
234-node knowledge base.

Decay scores at large ages differ by ~1e-13 between two independent
`time.Now()` calls, which would make a byte-identical equivalence assertion
flaky. The endpoint's decay clock is therefore injectable (a `now func()
time.Time` on `searchService`, defaulting to the package `now`, overridable in
tests) so both the endpoint and the oracle measure ages from the SAME pinned
instant. This mirrors the existing package `now` seam stats already uses.

Positive/negative controls cover: default floor keeps the strong-old on top and
> 0; `decay_floor=0` restores today's unbounded inversion; out-of-range
`decay_floor`/`min_relevance` → 400; `min_relevance` above a node's raw cosine
drops it; `half_life` absent is byte-identical; `?decay_floor=` empty takes the
default; the undated-node test stays green.
