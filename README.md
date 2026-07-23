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
- `node new <path> --type <type> [--title <title>] --bundle <dir>` — create a new concept node
- `node show <path> --bundle <dir>` — print a node's front matter and body
- `node list --bundle <dir>` — list the concept nodes in a bundle
- `index build [dir]` — regenerate `index.md` from the current bundle
- `index check [dir]` — verify `index.md` is current; nonzero exit if stale
- `log append [dir] --message <text>` — append a dated entry to `log.md`
- `log show [dir]` — print the change history
- `validate <dir>` — validate a bundle against the OKF spec floor
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
