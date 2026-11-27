# okfctl

[![CI](https://github.com/cwest/okfctl/actions/workflows/ci.yml/badge.svg)](https://github.com/cwest/okfctl/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/cwest/okfctl?sort=semver)](https://github.com/cwest/okfctl/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/cwest/okfctl.svg)](https://pkg.go.dev/github.com/cwest/okfctl)
[![License](https://img.shields.io/github/license/cwest/okfctl)](LICENSE)

`okfctl` is a command-line tool for people who keep a knowledge base in
Markdown and want it to stay correct as it grows. It writes and checks
[Open Knowledge Format](https://github.com/open-knowledge-format/open-knowledge-format/blob/main/SPEC.md)
(OKF) bundles: folders of Markdown files ("nodes") that link to each other and
record where their facts came from in YAML frontmatter. Rename a node and okfctl
rewrites every link that pointed at it. Put it in CI and a mistyped link or an
out-of-date index fails the build long before a reader trips on it.

**Website:** [okfctl.dev](https://okfctl.dev) &nbsp;•&nbsp;
**Docs:** [`docs/`](docs/README.md) &nbsp;•&nbsp;
**Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md) &nbsp;•&nbsp;
**Security:** [SECURITY.md](SECURITY.md) &nbsp;•&nbsp;
**Conduct:** [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## See it catch something

Make a bundle with two nodes. Have one of them mention the other without
linking to it:

```text
$ okfctl bundle init mykb
Initialized OKF bundle in mykb
$ okfctl node new concepts/tannin.md --type Reference --title "Tannin" --bundle mykb
Created mykb/concepts/tannin.md
$ okfctl node new concepts/oak.md --type Reference --title "Oak" --bundle mykb
Created mykb/concepts/oak.md
$ echo 'Oak barrels add tannin to red wine.' >> mykb/concepts/oak.md
$ okfctl validate mykb
OK: bundle conforms to the OKF spec floor
$ okfctl lint mykb
missing-xref: concepts/oak.md mentions "Tannin" but does not link to concepts/tannin.md
1 lint finding(s)
$ okfctl lint --strict mykb
missing-xref: concepts/oak.md mentions "Tannin" but does not link to concepts/tannin.md
1 lint finding(s)
okfctl: 1 lint finding(s)
```

`validate` passes. The bundle meets the spec's floor: all the frontmatter
parses, and the reserved `index.md` and `log.md` files are well formed (OKF
§11.1). `lint` reads past the floor and finds the oak node talking about tannin
with no link. By default it exits 0. With `--strict` it exits 1, and that is
the line you put in CI.

Notice you never ran an index command. `node new` rebuilt both `index.md` files
and logged each node in `log.md` as it went, which is why
`okfctl index check mykb` prints `OK: index.md is current`.

![okfctl quickstart: bundle init through validate and bundle info](docs/assets/quickstart.gif)

okfctl ships as one static Go binary. It needs no toolchain, interpreter, or
model download to run. Try `okfctl search "tannin" mykb` to find nodes by text,
`okfctl graph export mykb` to dump the link graph, or `okfctl analyze mykb` to
see where the bundle is thin. Each `okfctl <cmd> --help` ends with an example
you can paste.

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
[Open Knowledge Format spec](https://github.com/open-knowledge-format/open-knowledge-format/blob/main/SPEC.md)
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
- [`migrate`](docs/commands/README.md#okfctl-migrate)—upgrade a v0.1 or v0.2 bundle to OKF v0.3 (two-phase, consumer-agnostic).
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
  - [Migrating a bundle to OKF v0.3](docs/guides/migrating.md)
  - [Remote sources: `registry` and `connect`](docs/guides/remote-sources.md)
  - [Extending okfctl with plugins](docs/guides/plugins.md)
- **Per-command help**—`okfctl <cmd> --help` is authoritative and always
  matches the binary.
- **The spec**—the authoritative
  [OKF v0.3 specification](https://github.com/open-knowledge-format/open-knowledge-format/blob/main/SPEC.md).
  okfctl targets v0.3; the
  [v0.3 gap analysis](docs/specs/2026-11-26-okf-v0.3-gap-analysis.md) records
  how each v0.3 change landed.
- **For contributors**—[`docs/PRD.md`](docs/PRD.md), the
  [ADRs](docs/adr/README.md), and the dated
  [plans](docs/plans/2026-07-22-roadmap.md)/[specs](docs/specs/) that record how
  and why the tool was built.

## License

Apache-2.0. See [LICENSE](LICENSE).
