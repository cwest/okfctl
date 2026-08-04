# Command reference

`okfctl <cmd> --help` is the authoritative, always-current form for any command —
it prints the full description, flags, and a runnable example straight from the
binary. This page is a one-entry-per-command index; the detailed reference is
generated from the command tree (see [`docs/README.md`](../README.md)).

Run `okfctl help` for the top-level list, or `okfctl <cmd> --help` for any entry
below.

## okfctl bundle

Bundle lifecycle. `bundle init [dir]` scaffolds a minimal conformant OKF bundle
(reserved files + `.okf` sidecar, zero nodes). `bundle info [dir]` summarizes a
bundle: node count, reserved-file count, and declared `okf_version`.

## okfctl node

Author and inspect nodes: `new`, `show`, `list`, `edit`, `mv` (move/rename,
rewriting inbound links), `rm` (remove and report orphans), `refresh` (bulk-fix
stale `modified` timestamps), and `promote` (remediate the
directory-as-concept shape). See [Authoring a bundle](../guides/authoring.md).

## okfctl index

Manage the reserved `index.md`. `index build [dir]` regenerates the per-directory
indexes; `index check [dir]` verifies they are current (nonzero exit on stale,
missing, or orphaned). See [Index and freshness](../guides/index-and-freshness.md).

## okfctl log

Manage the reserved `log.md` change history. `log append [dir] --message <text>`
records a dated entry; `log show [dir]` prints the history.

## okfctl validate

Check a bundle against the OKF spec floor. Reports git drift as advisory
warnings. With `--templates` it runs the opt-in type-template overlay (§9.4),
reporting template drift; `--strict` exits nonzero on any drift. Spec-floor
violations always fail; the overlay never leaks into the floor.

## okfctl lint

Report curation-health findings (orphans, missing cross-references, broken
internal links, coverage gaps, type hygiene). Advisory by default; `--strict`
gates CI. `--semantic` adds two similarity-driven checks (needs an index, no
model). See [Curation health](../guides/curation-health.md).

## okfctl analyze

Report where a bundle is weak: freshness, clusters, gaps, connectivity, and
structure — the "where is this thin?" companion to `lint`'s "what is broken?".

## okfctl search

Core lexical + graph-neighborhood search, stdlib-only, no model or index. Match
by title/tag/type/body (`--field`), or query the graph with `--neighbors
<node-path> --depth N`. `--json` for CI-diffable output. Semantic search is the
separate `okfctl-search` plugin. See [Search](../guides/search.md).

## okfctl graph

Export the concept-node link graph. `graph export <dir> --format json|dot` —
JSON (default) emits nodes + edges; DOT emits Graphviz (pipe to `dot -Tsvg`).

## okfctl serve

Serve an interactive web visualization of the bundle graph. `serve <dir> --addr
127.0.0.1:8080` — the viewer is embedded in the binary; binds loopback by default.

## okfctl template

Read the type-template bundle. `template list [dir]` lists declared templates;
`template show <target-type> [dir]` shows one template's required/recommended
fields and body sections. Templates are authored as ordinary OKF nodes.

## okfctl migrate

Upgrade a bundle from OKF v0.1 to v0.2 (two-phase, consumer-agnostic). See
[Migrating a v0.1 bundle](../guides/migrating.md).

## okfctl registry

Manage named remote bundle sources — `git remote` for OKF bundles, not a hosted
service. `registry add|list|show|remove`. See [Remote sources](../guides/remote-sources.md).

## okfctl connect

Clone or fast-forward a remote bundle source into a local directory. A registered
name resolves to its URL; an ad-hoc git URL is used directly. Shells out to `git`.

## okfctl plugin

Discover and manage `okfctl-<name>` plugins on `PATH`. `plugin list` enumerates
them; `plugin install <source>` copies an executable into the managed plugins
dir. See [Extending okfctl with plugins](../guides/plugins.md).

## okfctl config

Get and set okfctl configuration. `config set|get|list`. Values live in one JSON
config store (config-home convention).

## okfctl completion

Generate a shell completion script: `okfctl completion <bash|zsh|fish>`.

## okfctl version

Print the okfctl version (also `okfctl --version`) — the release tag injected at
build time, or `dev` for a plain source build.
