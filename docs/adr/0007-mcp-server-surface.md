# 7. Grow an MCP server surface as a transport over the existing read handlers

- **Status:** Accepted (2026-08-21)
- **Deciders:** Casey West
- **Sources:** `AGENTS.md` (the "Agent Plugins packaging" section, the strategy-1 clause); code `internal/apiserver/handler.go` (`NewHandler`, the read-only invariant comment)
- **Related:** [ADR 0002](0002-path-dispatch-extension-model.md) (the `git`/`kubectl` PATH-dispatch extension model, the other way okfctl exposes itself to callers); [ADR 0006](0006-vanilla-js-embedded-visualizer.md) (the HTTP `serve` surface these handlers already back)

## Context

okfctl already exposes a read-only HTTP surface for agents and viewers:
`internal/apiserver/handler.go`'s `NewHandler` registers `GET /api/v1/stats`,
`GET /api/v1/graph`, and—only when an embedder is available—`GET /api/v1/search`.
The bundle is treated as strictly read-only; the handler's own invariant is
verbatim:

> The bundle is treated as strictly read-only: every route is a GET and no
> handler writes bundle source files (§5).

Model Context Protocol (MCP) is the emerging convention for exposing a tool's
capabilities to an agent client as named tools over a stdio (or HTTP) transport,
rather than making the agent shell out to a CLI or hand-write HTTP calls. The
question this record settles is whether okfctl should grow an MCP server surface
(`okfctl mcp serve` or equivalent) so an MCP-speaking client can call
`okf_stats` / `okf_graph` / `okf_search` directly.

### The recorded decision this revisits

`AGENTS.md`'s packaging section records a contributor-facing rule, quoted here
verbatim so there is no ambiguity about what is being reconsidered:

> - **`plugin.json` is spec-only + AGENTS.md** (strategy 1). No MCP server and no
>   client shim directories (`.claude-plugin/`, `.cursor-plugin/`, …)—a
>   compatible client reads the root `plugin.json` directly.

That clause bundles **two** exclusions under one dash:

1. **No client shim directories** (`.claude-plugin/`, `.cursor-plugin/`, …). This
   is the Agent Plugins "strategy 1" packaging choice: a compatible client reads
   the root `plugin.json` directly, so per-client shim directories are dead
   weight. This exclusion is about *packaging layout*.
2. **No MCP server.** This is a statement about the tool's *runtime surface
   area*—whether okfctl offers a programmatic server front door at all.

The two were written together but are independently motivated. The shim
exclusion is a packaging decision that stands regardless of this record's
verdict; the MCP exclusion is the runtime-surface decision under review.

### What changed since the rule was written

When the clause was written, okfctl had no programmatic read surface to adapt—an
MCP server would have meant building a new subsystem: query handlers, a bundle
loader, a serialization contract, a read-only guarantee, all from scratch. That
made "no MCP server" the right call by proportionality: a whole subsystem to
duplicate what the CLI already did.

That precondition no longer holds. The `serve` HTTP surface
(`internal/apiserver/handler.go`) now provides exactly those pieces:

- **The blast radius is already correct.** Every route is a `GET`; no handler
  writes bundle source files (`handler.go`, the `NewHandler` doc comment, §5).
  An MCP server exposing these three reads inherits that read-only, GET-only
  guarantee unchanged—it introduces no new write path and no new class of
  side effect.
- **A stdio adapter is a transport change, not a new subsystem.** The domain
  logic (bundle loading, graph derivation, semantic search) already exists and is
  already reached through a stable in-process seam. An MCP surface is a second
  transport in front of that seam, the way HTTP is the first; it is the same kind
  of addition as adding a second output format, not the addition of a new
  capability.
- **The no-reimplementation constraint is enforceable.** `NewHandler` builds its
  routes from `buildStats(b)`, `buildGraph(b)`, and `newSearchService(...).handle`.
  An MCP adapter MUST call the same functions `NewHandler` calls—`okf_stats`
  routes to `buildStats`, `okf_graph` to `buildGraph`, `okf_search` to the same
  search service. Because both transports funnel through one derivation, **the
  MCP view can never disagree with the HTTP view or the CLI.** This is the same
  invariant the HTTP `/graph` route already documents: it uses "the EXACT
  serializer graph export and serve's `/graph.json` use, so the API's graph view
  can never disagree with the CLI's (§2.4)." The MCP tool set is one more consumer
  of that single serializer, never a second implementation of it.
- **There is a precedent for the one conditional tool.** `/search` is registered
  only when an embedder is non-nil; without one it 404s like any unknown route,
  leaving `/stats` and `/graph` byte-identical—the negative control the search
  acceptance criteria require. A future `okf_search` MCP tool MUST follow the same
  pattern: it is advertised in the tool list only when an embedder is available,
  and its absence leaves `okf_stats` and `okf_graph` untouched. The conditional
  route is the template for the conditional tool.

The decision therefore reduces to: given that the read subsystem already exists
with the correct blast radius, is a second (MCP) transport in front of it worth
adding, or does the CLI/HTTP pair already cover the need?

## Decision

**Adopt an MCP server surface, scoped as a thin transport adapter over the
existing read handlers.** okfctl MAY grow an MCP server (`okfctl mcp serve` or
equivalent) that advertises read-only tools mapping one-to-one onto the routes
`NewHandler` already registers.

The design being decided on—not built by this record—is:

| MCP tool | Backs onto | Registered |
|---|---|---|
| `okf_stats` | `buildStats(b)` (the `GET /api/v1/stats` handler's body) | always |
| `okf_graph` | `buildGraph(b)` (the `GET /api/v1/graph` handler's body) | always |
| `okf_search` | the same `newSearchService(...).handle` search service | **only when an embedder is available**, mirroring the conditional `/search` route |

Binding constraints on any future implementation:

1. **Adapter, not reimplementation.** The MCP tools call the same functions
   `NewHandler` calls. No tool derives stats, builds a graph, or runs a search by
   any path other than the one the HTTP surface and the CLI already use. If a new
   derivation would be needed, that is a signal the adapter has drifted into a
   second subsystem and the change is out of scope for this decision.
2. **Read-only, inherited.** The MCP surface exposes reads only. It inherits
   `handler.go`'s read-only invariant (§5) and adds no write path. A write-capable
   MCP tool is a different decision requiring its own ADR.
3. **Conditional `okf_search` follows the `/search` precedent.** `okf_search` is
   advertised only when an embedder is present; its absence leaves `okf_stats` and
   `okf_graph` unaffected. This is the same additive-and-isolated shape the HTTP
   `/search` route already proves.

### What happens to the `AGENTS.md` clause under this verdict

The clause is **split**: the shim-directory exclusion stays; the MCP-server
exclusion is lifted. The scoped clause reads "no client shim directories"—the
`.claude-plugin/`/`.cursor-plugin/` layout exclusion is independently motivated
and unchanged—while the "No MCP server" phrasing is removed and replaced with a
pointer to this record. Under this verdict `AGENTS.md` gains a reference to ADR
0007 so a contributor reading the packaging rules is not misled into thinking a
server surface is still categorically excluded.

### What would have happened under the other verdict

Had this record concluded the decision should stand (no MCP server), the
`AGENTS.md` clause would have been left intact and merely gained a reference to
this ADR recording *why* it stands—so a future contributor tempted to add MCP
finds the reasoning already litigated rather than re-opening it. Either verdict
requires the `AGENTS.md` cross-reference; only the favourable verdict also
rescopes the clause.

## Consequences

**What it buys.** An MCP-speaking agent client calls okfctl's reads as named
tools over its native transport instead of shelling out to the CLI or
hand-writing HTTP requests—the front door agents increasingly expect. Because the
adapter routes through the same `buildStats`/`buildGraph`/search functions the
HTTP surface and CLI use, the three views are provably consistent by
construction: there is one derivation, three transports. The read-only, GET-only
blast radius carries over unchanged, so the new surface adds no new write path or
side-effect class. The addition is small and reversible—a transport in front of
an existing seam, not a new subsystem.

**What it costs.** A new transport is still new surface to build, test, and keep
in step with the handler signatures it adapts: when a handler's shape changes, the
MCP tool schema must change with it, and a conformance test must pin the two
together so they cannot silently drift (the same discipline the `/graph`↔`export`
serializer sharing already requires). It adds a dependency on an MCP server
library and its wire-protocol surface to audit and patch—weighed against ADR
0001's small-dependency-tree principle, this is a real cost and must be a
deliberate, minimal choice at implementation time, not an open-ended framework
pull. And it is one more front door to hold to the read-only invariant: the
constraint that every MCP tool is a read and calls only the existing functions is
a rule a future change could violate, so it must be enforced in code and tests,
not merely stated here.

**Scope of this record.** This ADR decides *that* okfctl grows an MCP surface and
*what shape* it takes; it does **not** implement `okfctl mcp serve`. No `mcp`
subcommand, no adapter code, and no new dependency land with this record. The
implementation is tracked as a separate follow-up ([#148](https://github.com/cwest/okfctl/issues/148))
carrying the original acceptance criteria unchanged, and it is that follow-up—not
this ADR—that will add the server, its conformance tests, and the dependency.
