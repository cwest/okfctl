# okfctl

[![CI](https://github.com/cwest/okfctl/actions/workflows/ci.yml/badge.svg)](https://github.com/cwest/okfctl/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/cwest/okfctl?sort=semver)](https://github.com/cwest/okfctl/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/cwest/okfctl.svg)](https://pkg.go.dev/github.com/cwest/okfctl)
[![License](https://img.shields.io/github/license/cwest/okfctl)](LICENSE)

Markdown knowledge bases don't rot because people stop writing. They rot because
nothing keeps the structure honest—links break on a rename, the index drifts
from the tree, provenance goes stale, and no one notices until the corpus is
already a maze.

`okfctl` is the tool that keeps it honest. It authors and maintains
[Open Knowledge Format](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)
(OKF) bundles—a curated tree of Markdown "nodes" with a link graph, reserved
`index.md`/`log.md` files, and frontmatter provenance—and it checks the whole
thing against the spec so drift surfaces as a finding instead of a surprise.

**Website:** [okfctl.dev](https://okfctl.dev) &nbsp;•&nbsp;
**Docs:** [`docs/`](docs/README.md) &nbsp;•&nbsp;
**Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md) &nbsp;•&nbsp;
**Security:** [SECURITY.md](SECURITY.md) &nbsp;•&nbsp;
**Conduct:** [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## What it does

Use `okfctl` to scaffold a bundle, add and move nodes without breaking links,
keep the reserved index and change log current, check a corpus against the spec,
and inspect its health and link graph. It's a single static Go binary, so it runs
anywhere without a toolchain, an interpreter, or a model download.

![okfctl quickstart: bundle init through validate and bundle info](docs/assets/quickstart.gif)

## 60-second quickstart

Starting from an empty directory, scaffold a bundle, add a node, build the
index, and validate—a full cold start:

```sh
okfctl bundle init mykb                                              # scaffold a conformant bundle
okfctl node new concepts/tannin.md --type Reference --title "Tannin" --bundle mykb
okfctl node list --bundle mykb                                       # see the node you just made
okfctl index build mykb                                              # generate the reserved index.md files
okfctl index check mykb                                              # confirm the index is current
okfctl log append mykb --message "added tannin node"                # record the change
okfctl validate mykb                                                 # check against the OKF spec floor
okfctl bundle info mykb                                              # nodes: 1, reserved: 3, okf_version: 0.2
```

From here, `okfctl search "tannin" mykb` finds nodes lexically, `okfctl graph
export mykb` dumps the link graph, and `okfctl analyze mykb` reports where the
bundle is weak. Every command explains itself with `okfctl <cmd> --help`,
including a runnable example.

## Install

Three ways to get running. Pick one:

**Homebrew** (macOS, or Linux with Homebrew):

```sh
brew install cwest/tap/okfctl
```

**One-liner** (macOS and Linux)—detects your OS/arch, downloads the matching
release archive, verifies its checksum, and installs `okfctl` and `okfctl-search`
onto your `PATH`:

```sh
curl -sSL https://okfctl.dev/install.sh | sh
```

**Go toolchain**—install from source:

```sh
go install github.com/cwest/okfctl@latest
```

Prebuilt archives, Debian/RPM packages, cosign signature verification, and
building from source all live in the [install guide](docs/install.md).

## The spec is the authority

OKF is a specification `okfctl` consumes; it doesn't author it. The
[Open Knowledge Format spec](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)
decides behavior, and where it does, it wins. `okfctl` enforces the spec *floor*
for everyone and keeps anything stricter behind an explicit opt-in overlay
(`--templates`, §9.4), so an unknown `type` or a future frontmatter key still
passes `validate`.

## Commands

Grouped by what you're doing. Run `okfctl <cmd> --help` for full detail and a
runnable example, or read the [command reference](docs/commands/README.md).

**Author**—build and edit a bundle:

- [`bundle`](docs/commands/README.md#okfctl-bundle)—scaffold (`init`) and summarize (`info`) a bundle.
- [`node`](docs/commands/README.md#okfctl-node)—author and inspect nodes (`new`, `show`, `list`, `edit`, `mv`, `rm`, `refresh`, `promote`).
- [`index`](docs/commands/README.md#okfctl-index)—regenerate (`build`) and verify (`check`) the reserved `index.md`.
- [`log`](docs/commands/README.md#okfctl-log)—append (`append`) and print (`show`) the reserved `log.md` history.

**Check**—hold the corpus to the spec and to curation health:

- [`validate`](docs/commands/README.md#okfctl-validate)—check a bundle against the OKF spec floor; optionally overlay type-templates.
- [`lint`](docs/commands/README.md#okfctl-lint)—report curation findings (orphans, broken links, coverage gaps); `--strict` for CI.
- [`eval`](docs/commands/README.md#okfctl-eval)—measure node trustworthiness (TACA): a deterministic transparency gate plus an accuracy/alignment/calibration sampler.

**Explore**—read the structure you've built:

- [`analyze`](docs/commands/README.md#okfctl-analyze)—report where a bundle is weak: freshness, clusters, gaps, connectivity, structure.
- [`search`](docs/commands/README.md#okfctl-search)—lexical and graph-neighborhood search (stdlib-only, no model or index).
- [`graph`](docs/commands/README.md#okfctl-graph)—export the concept-node link graph (`--format json|dot`).
- [`serve`](docs/commands/README.md#okfctl-serve)—serve an interactive web visualization of the bundle graph.

**Extend**—templates, migration, remotes, and the rest:

- [`template`](docs/commands/README.md#okfctl-template)—list (`list`) and show (`show`) the type-templates a bundle declares.
- [`migrate`](docs/commands/README.md#okfctl-migrate)—upgrade a bundle from OKF v0.1 to v0.2 (two-phase, consumer-agnostic).
- [`registry`](docs/commands/README.md#okfctl-registry)—manage named remote bundle sources: `git remote` for OKF bundles.
- [`connect`](docs/commands/README.md#okfctl-connect)—clone or fast-forward a remote bundle source into a local directory.
- [`plugin`](docs/commands/README.md#okfctl-plugin)—discover (`list`) and install (`install`) `okfctl-<name>` plugins on `PATH`.
- [`config`](docs/commands/README.md#okfctl-config)—get, set, and list okfctl configuration.
- [`completion`](docs/commands/README.md#okfctl-completion)—generate a shell completion script (bash, zsh, fish).
- [`version`](docs/commands/README.md#okfctl-version)—print the okfctl version (also `okfctl --version`).

Semantic search over a bundle ships as the bundled `okfctl-search` plugin,
invoked as `okfctl-search --semantic "query"`. See the
[search guide](docs/guides/search.md).

## Use as an agent plugin

This repo is an [Agent Plugins 1.0.0](https://agent-plugins.org) package: the
root [`plugin.json`](plugin.json) bundles the four generic okfctl skills
(`okf-authoring`, `okf-curation-health`, `okf-migrate-plan`,
`okf-semantic-search`) as an installable unit for compatible agent clients
(Copilot, Cursor, Codex, …). Clients that read repo instructions directly pick
the same guidance up from [`AGENTS.md`](AGENTS.md).

> **Not to be confused with `okfctl plugin`** above—that command discovers
> and installs `okfctl-<name>` *executable* plugins (like `okfctl-search`) on
> your `PATH`. This section is about packaging okfctl's *skills* for an agent
> client, which is a different spec.

**Prerequisite—install `okfctl` first.** The skills shell out to the `okfctl`
binary, and a plugin client doesn't bundle it. Install it onto your `PATH`
before enabling the plugin (see [Install](#install)):

```sh
brew install cwest/tap/okfctl
```

The manifest carries no hand-maintained version string: the authoritative
version is the release tag, reported by `okfctl version`.

## Learn more

- **User docs**—concepts, task-oriented guides, and the full command
  reference live under [`docs/`](docs/README.md). Start with
  [concepts](docs/concepts.md), then the guides:
  - [Starting and authoring a bundle](docs/guides/authoring.md)
  - [Keeping `index.md` current and fixing freshness drift](docs/guides/index-and-freshness.md)—covers `.okf-drift-ignore-revs`.
  - [Curation health: `lint`, `analyze`, and `--strict` in CI](docs/guides/curation-health.md)—covers the semantic-lint checks and the vendored/derived skip policy.
  - [Search: core lexical/graph and the `okfctl-search` semantic plugin](docs/guides/search.md)—covers model2vec setup.
  - [Migrating a v0.1 bundle to v0.2](docs/guides/migrating.md)
  - [Remote sources: `registry` and `connect`](docs/guides/remote-sources.md)
  - [Extending okfctl with plugins](docs/guides/plugins.md)
- **Per-command help**—`okfctl <cmd> --help` is authoritative and always
  matches the binary.
- **The spec**—the authoritative
  [OKF v0.2 specification](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).
- **For contributors**—[`docs/PRD.md`](docs/PRD.md), the
  [ADRs](docs/adr/README.md), and the dated
  [plans](docs/plans/2026-07-22-roadmap.md)/[specs](docs/specs/) that record how
  and why the tool was built.

## License

Apache-2.0. See [LICENSE](LICENSE).
