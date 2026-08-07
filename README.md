# okfctl

`okfctl` is a command-line tool for authoring and maintaining
[Open Knowledge Format](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)
(OKF) bundles — a curated tree of Markdown "nodes" with a link graph, reserved
`index.md`/`log.md` files, and frontmatter provenance.

OKF is a specification this tool **consumes and conforms to**; okfctl does not
author the spec. Where the spec defines behavior, the spec wins. The tool
enforces the spec *floor* for everyone and keeps anything stricter behind an
explicit opt-in overlay (`--templates`, §9.4), so an unknown `type` or a future
frontmatter key never fails `validate`.

Use it to scaffold a bundle, add and move nodes without breaking links, keep the
reserved index and change log current, check a corpus against the spec, and
inspect its health and link graph. It is pure Go with no CGO, no Python, and no
model runtime.

## Install

Install the latest tagged release with `go install`:

```sh
go install github.com/cwest/okfctl@latest
```

Or download a prebuilt binary for your platform from the
[releases page](https://github.com/cwest/okfctl/releases) (darwin and
linux, amd64 and arm64). Each archive bundles both `okfctl` and the
`okfctl-search` plugin — extract them onto your `PATH`:

```sh
tar -xzf okfctl_<version>_<os>_<arch>.tar.gz
sudo mv okfctl okfctl-search /usr/local/bin/
```

Verify the install and the reported version:

```sh
okfctl version        # e.g. okfctl v1.2.3 (commit abc1234, built 2026-...)
okfctl --version      # same string
```

To build from source instead:

```sh
go build -o okfctl .                    # dynamic build; version reports "dev"
CGO_ENABLED=0 go build -o okfctl .      # static, no cgo
```

## 60-second quickstart

Starting from an empty directory, scaffold a bundle, add a node, build the
index, and validate — a full cold start:

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

## Commands

One line each; run `okfctl <cmd> --help` for full detail and examples, or see
the [command reference](docs/commands/README.md).

| Command | What it does |
|---|---|
| [`bundle`](docs/commands/README.md#okfctl-bundle) | Scaffold (`init`) and summarize (`info`) an OKF bundle. |
| [`node`](docs/commands/README.md#okfctl-node) | Author and inspect nodes: `new`, `show`, `list`, `edit`, `mv`, `rm`, `refresh`, `promote`. |
| [`index`](docs/commands/README.md#okfctl-index) | Regenerate (`build`) and verify (`check`) the reserved per-directory `index.md`. |
| [`log`](docs/commands/README.md#okfctl-log) | Append (`append`) and print (`show`) the reserved `log.md` change history. |
| [`validate`](docs/commands/README.md#okfctl-validate) | Check a bundle against the OKF spec floor; optionally overlay type-templates. |
| [`lint`](docs/commands/README.md#okfctl-lint) | Report curation-health findings (orphans, broken links, coverage gaps); `--strict` for CI. |
| [`eval`](docs/commands/README.md#okfctl-eval) | Measure KB-node trustworthiness (TACA): a deterministic Transparency gate + a spot-check sampler for Accuracy/Alignment/Calibration. |
| [`analyze`](docs/commands/README.md#okfctl-analyze) | Report where a bundle is weak: freshness, clusters, gaps, connectivity, structure. |
| [`search`](docs/commands/README.md#okfctl-search) | Core lexical + graph-neighborhood search (stdlib-only, no model or index). |
| [`graph`](docs/commands/README.md#okfctl-graph) | Export the concept-node link graph (`--format json\|dot`). |
| [`serve`](docs/commands/README.md#okfctl-serve) | Serve an interactive web visualization of the bundle graph. |
| [`template`](docs/commands/README.md#okfctl-template) | List (`list`) and show (`show`) the type-templates a bundle declares. |
| [`migrate`](docs/commands/README.md#okfctl-migrate) | Upgrade a bundle from OKF v0.1 to v0.2 (two-phase, consumer-agnostic). |
| [`registry`](docs/commands/README.md#okfctl-registry) | Manage named remote bundle sources — `git remote` for OKF bundles. |
| [`connect`](docs/commands/README.md#okfctl-connect) | Clone or fast-forward a remote bundle source into a local directory. |
| [`plugin`](docs/commands/README.md#okfctl-plugin) | Discover (`list`) and install (`install`) `okfctl-<name>` plugins on `PATH`. |
| [`config`](docs/commands/README.md#okfctl-config) | Get, set, and list okfctl configuration. |
| [`completion`](docs/commands/README.md#okfctl-completion) | Generate a shell completion script (bash, zsh, fish). |
| [`version`](docs/commands/README.md#okfctl-version) | Print the okfctl version (also `okfctl --version`). |

Semantic search over a bundle ships as the bundled `okfctl-search` plugin,
invoked as `okfctl search --semantic …`. See the
[search guide](docs/guides/search.md).

## Learn more

- **User docs** — concepts, task-oriented guides, and the full command
  reference live under [`docs/`](docs/README.md). Start with
  [concepts](docs/concepts.md), then the guides:
  - [Starting and authoring a bundle](docs/guides/authoring.md)
  - [Keeping `index.md` current and fixing freshness drift](docs/guides/index-and-freshness.md)
    — covers `.okf-drift-ignore-revs`.
  - [Curation health: `lint`, `analyze`, and `--strict` in CI](docs/guides/curation-health.md)
    — covers the semantic-lint checks and the vendored/derived skip policy.
  - [Search: core lexical/graph and the `okfctl-search` semantic plugin](docs/guides/search.md)
    — covers model2vec setup.
  - [Migrating a v0.1 bundle to v0.2](docs/guides/migrating.md)
  - [Remote sources: `registry` and `connect`](docs/guides/remote-sources.md)
  - [Extending okfctl with plugins](docs/guides/plugins.md)
- **Per-command help** — `okfctl <cmd> --help` is authoritative and always
  matches the binary.
- **The spec** — the authoritative
  [OKF v0.2 specification](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).
- **For contributors** — [`docs/PRD.md`](docs/PRD.md), the
  [ADRs](docs/adr/README.md), and the dated
  [plans](docs/plans/2026-07-22-roadmap.md)/[specs](docs/specs/) that record how
  and why the tool was built.

## License

Apache-2.0. See [LICENSE](LICENSE).
