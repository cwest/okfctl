# 4. Flat Go-native JSON vector store

- **Status:** Accepted (2026-07-24)
- **Deciders:** Casey West
- **Sources:** spec `docs/specs/2026-07-24-okfctl-search.md` ("Store (fork 2a)"); code `internal/search/store.go`
- **Related:** [ADR 0001](0001-build-in-go.md) (static-binary constraint), [ADR 0002](0002-path-dispatch-extension-model.md) (the plugin this store lives in)

## Context

The `okfctl-search` plugin needs to persist one embedding vector per concept
node and answer similarity queries over them. The obvious, well-trodden choice
for on-disk vector search is **`sqlite-vec`** (`asg017/sqlite-vec`): a single-file
SQLite database with a loadable vector extension — no server, mature, and the
PRD's original §13.1 named stack listed it.

Two facts about okfctl's context weigh against it:

1. **The static-binary constraint from [ADR 0001](0001-build-in-go.md).**
   `sqlite-vec` is a C extension. Embedding it in a Go binary means either CGO or
   an extension-loading SQLite driver — both reintroduce a C toolchain / loadable
   native dependency, breaking the `CGO_ENABLED=0` static-build guarantee and the
   "no separate install" promise (PRD §5.1). The plugin is a separate binary, but
   it is still built and distributed under the same static-cross-compile
   discipline as core.
2. **The corpus is small.** okfctl operates on a single OKF bundle — hundreds to
   low thousands of nodes. At that scale a brute-force cosine scan over every
   stored vector is effectively instant; an approximate-nearest-neighbor index
   solves a scale problem this workload does not have.

The store must still record the embedder's `model` and `dim` so a rebuild is
reproducible and a query issued against a mismatched-model index is refused
rather than silently wrong (PRD §8.3) — but that discipline is independent of the
storage engine.

## Decision

Use a **flat, Go-native vector store**: `.okfctl/index.db` is a Go-serialized
file (JSON, chosen for legibility at this slice) recording `model` (the embedder's
`Name()`) and `dim`, plus one `{path, content-hash, vector}` record per concept
node. Queries run a brute-force cosine scan over the stored vectors. The store
refuses any index whose recorded `model`/`dim` does not match the active
embedder — the reproducibility guard. **No `sqlite-vec`, no SQLite, no CGO.**
`internal/search/store.go` carries zero SQLite dependencies; `go.mod` has none.
An ANN index remains a later slice if a future corpus scale ever demands it.

## Consequences

**What it buys.** The plugin stays a pure-Go, statically-linked, single-file
binary with no C toolchain, no loadable native extension, and no separate
install — the same distribution guarantee as core. The JSON store is
human-legible and trivially diffable, which is valuable while the format is
young. Recording `model`+`dim` in the file makes rebuilds deterministic and
cross-model queries an explicit refusal rather than a silent wrong answer.
Content-hash keying means an unchanged node is never re-embedded.

**What it costs.** Query is O(N) brute force over every vector, so this store will
not scale to very large corpora the way an ANN index (or `sqlite-vec`) would — a
deliberate trade that assumes single-bundle scale and defers ANN until scale
actually demands it. JSON serialization is larger on disk and slower to
load/parse than a binary format or a real database, and the whole store is read
into memory to query. okfctl also forgoes SQLite's mature durability, indexing,
and query tooling, reimplementing the small slice it needs (reproducibility
guard, content-hash freshness) by hand. If a corpus ever outgrows brute force,
this decision must be revisited with a follow-on ADR.
