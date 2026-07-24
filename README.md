# okfctl

A CLI for managing Open Knowledge Format (OKF) bundles.

See [docs/PRD.md](docs/PRD.md) for the tool spec and
[docs/plans/2026-07-22-roadmap.md](docs/plans/2026-07-22-roadmap.md)
for the full plan. The walking skeleton (increment 1) covered bundle,
node, validate, and config; this increment adds lifecycle management for
the reserved `index.md` and `log.md` files.

## Build

```sh
go build -o okfctl .
```

For a static binary with no cgo dependency:

```sh
CGO_ENABLED=0 go build -o okfctl .
```

## Quickstart

```sh
okfctl bundle init mykb
okfctl node new concepts/tannin.md --type Reference --title "Tannin" --bundle mykb
okfctl node list --bundle mykb
okfctl index build mykb
okfctl index check mykb
okfctl log append mykb --message "added tannin node"
okfctl log show mykb
okfctl validate mykb
okfctl bundle info mykb
```

## Commands

- `bundle init [dir]` — scaffold a minimal conformant OKF bundle
- `bundle info [dir]` — summarize a bundle: node count, reserved-file count, spec version
- `node new <path> --type <type> [--title <title>] --bundle <dir>` — create a new concept node. When a type template governs `--type`, the node is scaffolded from it: required fields are stubbed (`TODO` placeholders), recommended fields are stubbed empty, and the template's body sections are laid down as empty `##` headings, so the new node starts conformant to the team's convention.
- `node show <path> --bundle <dir>` — print a node's front matter and body
- `node list --bundle <dir>` — list the concept nodes in a bundle
- `node edit <path> --bundle <dir>` — open a node in `$EDITOR` (or `$OKFCTL_EDITOR`/`$VISUAL`), then re-validate the bundle on return
- `node mv <old> <new> --bundle <dir> [--dry-run]` — move/rename a node and rewrite every inbound link, preserving each author's relative link form (path is identity, so a move is a graph operation)
- `node rm <path> --bundle <dir> [--dry-run]` — remove a node and report any nodes orphaned as a result
- `index build [dir]` — regenerate `index.md` from the current bundle
- `index check [dir]` — verify `index.md` is current; nonzero exit if stale
- `log append [dir] --message <text>` — append a dated entry to `log.md`
- `log show [dir]` — print the change history
- `validate <dir>` — validate a bundle against the OKF spec floor. With `--templates` it also runs the opt-in type-template overlay (§9.4), reporting **template drift** (a node missing a required field or body section its governing template declares) as warnings — advisory by default (exit 0), `--strict` exits non-zero on drift. Spec-floor violations always fail regardless of `--templates`/`--strict`; the overlay never leaks into the floor (unknown type values still pass, §7.4).
- `template list [dir]` — list the type templates a bundle declares (target type, required-field and body-section counts). Templates are authored as ordinary OKF nodes (`type: Type Template`); nothing lives in tool config.
- `template show <target-type> [dir]` — show one template's required/recommended fields and body sections.
- `plugin list [--path <PATH>]` — list `okfctl-<name>` plugin executables discovered on `PATH` (sorted by name, first-on-PATH wins). Plugins extend okfctl `git`/`kubectl`-style: an unknown subcommand `okfctl foo bar` execs an `okfctl-foo` binary found on `PATH`, passing through `bar` plus the remaining flags and environment (with `OKFCTL` set to the core binary's path so a plugin can call back), and propagates the plugin's exit code. Built-in subcommands always take precedence; an unknown subcommand with no matching plugin produces the usual error plus a did-you-mean suggestion. Executable detection uses Unix permission bits (macOS/Linux); Windows is not yet supported.
- `lint <dir>` — report curation health findings (orphans, missing cross-references, coverage gaps, type-value hygiene). Advisory by default (exits 0 even with findings); `--strict` exits non-zero on any finding, `--coverage-threshold N` tunes the coverage-gap check (default 3). `lint` never mutates the bundle.
- `graph export <dir> --format json|dot` — export the concept-node link graph in a machine format (deterministic, CI-diffable). `json` (default) emits nodes (path/title/type/neighborhood/orphan) + edges; `dot` emits Graphviz. For SVG, pipe DOT to Graphviz: `okfctl graph export --format dot | dot -Tsvg > graph.svg`.
- `serve <dir> --addr 127.0.0.1:8080` — start a local web server rendering the bundle as an interactive knowledge graph (click a node to inspect, follow edges, orphans highlighted, filter by type/neighborhood). The viewer is embedded in the binary — no separate install. Binds loopback by default; override with `--addr`.
- `config set <key> <value>` — set a config value
- `config get <key>` — read a config value
- `config list` — list all config values
- `completion <bash|zsh|fish>` — generate a shell completion script

## What validate checks

`validate` enforces the OKF spec floor only: every node must carry a
non-empty `type` (OKF §7). Unknown type *values* are allowed — the
walking skeleton does not enforce a taxonomy, so `--type Reference`,
`--type Concept`, or any other string all pass.

## License

Apache-2.0. See [LICENSE](LICENSE).
