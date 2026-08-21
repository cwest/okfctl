# Architecture Decision Records

This directory records the architectural decisions okfctl has made—one file per
decision, in [Michael Nygard's lightweight ADR format][nygard]: a short
**Status / Context / Decision / Consequences** narrative that captures *why* a
fork was taken, including the alternative that was rejected and what the choice
costs.

The [PRD](../PRD.md) states what the product does; these records state why the
architecture is the way it is. When the PRD or a spec carries a decision's
rationale inline, the ADR now owns that rationale and the PRD links here rather
than duplicating the argument.

[nygard]: https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions

## Index

| ADR | Title | Status | Date |
|---|---|---|---|
| [0001](0001-build-in-go.md) | Build okfctl in Go | Accepted | 2026-07-22 |
| [0002](0002-path-dispatch-extension-model.md) | `git`/`kubectl`-style PATH-dispatch extension model | Accepted | 2026-07-24 |
| [0003](0003-managed-type-presence-only.md) | Manage `type` as presence, never a value allowlist | Accepted | 2026-07-22 |
| [0004](0004-flat-json-vector-store.md) | Flat Go-native JSON vector store | Accepted | 2026-07-24 |
| [0005](0005-pure-go-embedder.md) | Pure-Go Model2Vec + WordPiece embedder | Accepted | 2026-07-25 |
| [0006](0006-vanilla-js-embedded-visualizer.md) | Vanilla-JS `go:embed`-ed visualizer front-end | Accepted | 2026-07-23 |
| [0007](0007-mcp-server-surface.md) | Grow an MCP server surface over the existing read handlers | Accepted | 2026-08-21 |

## When to write a new one

Write an ADR when you make a decision that is architecturally significant and
costly to reverse—a language or dependency choice, a store format, a plugin
boundary, a protocol you adopt or reject. Copy an existing record's structure,
give it the next number (`NNNN-kebab-title.md`), and fill in
**Status / Context / Decision / Consequences**. A record that lists only the
upside of a choice is not a decision record: the Consequences section must state
what the decision costs as well as what it buys. Add a row to the index table
above. Records are immutable once Accepted; a later decision that overturns an
earlier one gets its own ADR and marks the old one `Superseded by NNNN`.
