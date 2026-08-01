# 6. Vanilla-JS `go:embed`-ed visualizer front-end

- **Status:** Accepted (2026-07-23)
- **Deciders:** Casey West
- **Resolves:** PRD [§13.4](../PRD.md#134-web-visualizer-front-end-approach)
- **Sources:** spec `docs/specs/2026-07-24-graph-serve.md` (line 41); code `cmd/serve.go` (`//go:embed assets/index.html`)
- **Related:** [ADR 0001](0001-build-in-go.md) (single-binary, stdlib-only server)

## Context

The `serve` command starts a local web server that renders a bundle as an
interactive, navigable knowledge graph — click a node to read it, follow edges,
highlight orphans, filter by type/neighborhood (PRD §6.3, pillar 3). PRD §5.1
requires the visualizer's assets to ship *inside* the binary so there is no
separate install; the server itself is already stdlib-only (`net/http` +
`go:embed`, per [ADR 0001](0001-build-in-go.md)). The open question was the
*front-end* approach:

1. **A small framework build.** Author the viewer with a JS framework/bundler
   (React/Svelte/Vite or similar), run an npm/Node build step, and embed the
   compiled output.
2. **Vanilla JS, single hand-written page.** A small, self-contained
   force-directed renderer in one `index.html` with no build step.

Either way the compiled/authored assets are baked in via `go:embed`, so the
choice is orthogonal to the settled Go backend — it is purely a question of the
front-end's own toolchain and weight.

## Decision

Ship a **vanilla-JS, single `go:embed`-ed `index.html`** — a small, self-contained
force-directed renderer with **no npm/Node build step**. The server exposes
`GET /` (the embedded page) and `GET /graph.json` (the same deterministic
serializer as `graph export --format json`), binds loopback by default, and loads
the bundle once at startup, read-only. `cmd/serve.go` embeds the single asset with
`//go:embed assets/index.html`.

## Consequences

**What it buys.** The entire build is `go build` — no Node, no npm, no bundler in
the toolchain or CI, and nothing for a contributor to install to work on the
viewer. There is no JS dependency tree to audit or keep patched, which keeps the
project's supply-chain surface as small as the backend's. A single embedded file
is trivial to reason about and ships as part of the one static binary, satisfying
the no-separate-install requirement directly.

**What it costs.** A hand-written vanilla renderer forgoes the ergonomics a
framework provides — component structure, reactive state, and a mature ecosystem
of graph-visualization libraries. As the viewer's interactivity grows (richer
filtering, large-graph performance, complex layout), hand-rolled JS in one file
becomes harder to maintain than componentized framework code would be, and the
force-directed layout is a from-scratch implementation rather than a
batteries-included library. This decision is right-sized for the current
read-only viewer; a materially more ambitious front-end would justify revisiting it
with a follow-on ADR that weighs a build step against the maintenance cost.
