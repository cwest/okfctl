# 2. `git`/`kubectl`-style PATH-dispatch extension model

- **Status:** Accepted (2026-07-24)
- **Deciders:** Casey West
- **Sources:** PRD [§6.4](../PRD.md#64-extension-modelgitkubectl-style), [§8.4](../PRD.md#84-architecture-fita-path-dispatch-plugin-not-core); spec `docs/specs/2026-07-24-plugin-dispatch.md`

## Context

okfctl must be extensible — the community should be able to ship exporters,
importers, domain-specific linters, and the heavy semantic-search capability
without forking the tool or waiting on a core release (PRD pillar: extension
model). Two extension architectures were on the table:

1. **In-process plugin registry.** Plugins are Go packages compiled into the
   binary (or loaded via Go's `plugin` package) and registered against an
   internal interface. Extensions run in the same process and can share the
   in-memory bundle model directly.
2. **PATH-dispatch of external executables.** An unknown subcommand
   `okfctl foo bar` resolves to an `okfctl-foo` executable on `PATH`, execs it
   with the remaining args, passes through stdin/stdout/stderr and the
   environment, and propagates its exit code — exactly as `git` finds
   `git-<name>` and `kubectl` finds `kubectl-<name>`.

The tension is that the flagship optional capability, semantic search, carries a
heavy dependency footprint (an embedding model, and originally a candidate
C-extension vector store). PRD §5.1 promises the core is a single, self-contained
binary with no runtime dependencies. An in-process registry would either pull
that weight into the core binary or require Go's `plugin` package, which does not
cross-compile statically and breaks the `CGO_ENABLED=0` static-build guarantee
established in [ADR 0001](0001-build-in-go.md).

## Decision

Adopt the **`git`/`kubectl`-style PATH-dispatch extension model**. Built-in
subcommands always take precedence; dispatch fires only for a genuinely unknown
subcommand. The core stays stdlib-only and oblivious to which plugins exist
(`internal/plugin` is stdlib-only, no cobra import). `plugin list` discovers
`okfctl-<name>` executables on `PATH`; `plugin install` copies one into the
managed plugins dir. The semantic-search capability ships as its own separate
static binary, `okfctl-search`, discovered and dispatched by this mechanism
rather than compiled into core.

## Consequences

**What it buys.** The core binary keeps its zero-runtime-dependency guarantee:
heavy or optional capabilities live in separate executables that ride the plugin
boundary, so a user who never runs semantic search never carries its weight. The
model is language-agnostic — a plugin can be written in any language — and
familiar to anyone who has extended `git` or `kubectl`. Extensions ship on their
own cadence without a core release. Exit-code fidelity (a plugin that exits 7
makes `okfctl` exit 7) makes plugins first-class in scripts and CI.

**What it costs.** Cross-process dispatch means a plugin cannot share the core's
in-memory bundle model; it re-loads and re-parses the bundle itself, and the two
communicate through the filesystem and process boundary rather than through Go
types. There is no compile-time interface contract between core and a plugin, so
the seam is a runtime convention (`okfctl-<name>`, argument passthrough,
`OKFCTL` env callback) that must be documented and honored rather than enforced
by the type system. Executable detection relies on Unix permission bits, so
Windows plugin discovery is not yet supported. And a plugin must be present on
`PATH` to be found, which pushes an installation/discoverability concern onto the
user (mitigated by `plugin install` and a stderr note when the managed dir is not
on `PATH`).
