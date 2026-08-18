# okfctl documentation

This tree is for three audiences. Pick your lane.

## Users — using the tool

Start here:

- **[Install](install.md)** — Homebrew, the one-liner, and `go install` cover
  most people from the README; this page has prebuilt archives, deb/rpm packages,
  cosign verification, and building from source.
- **[Concepts](concepts.md)** — what a bundle is, what a node is, the reserved
  `index.md`/`log.md`, the link graph, the spec-floor-vs-overlay distinction, and
  the v0.1→v0.2 story. Read this first; it makes every other page make sense.
- **Task-oriented guides** — each one runnable start to finish:
  - [Starting and authoring a bundle](guides/authoring.md)
  - [Keeping `index.md` current and fixing freshness drift](guides/index-and-freshness.md)
  - [Curation health: `lint`, `analyze`, and `--strict` in CI](guides/curation-health.md)
  - [Search: core lexical/graph and the `okfctl-search` semantic plugin](guides/search.md)
  - [Migrating a v0.1 bundle to v0.2](guides/migrating.md)
  - [Remote sources: `registry` and `connect`](guides/remote-sources.md)
  - [Extending okfctl with plugins](guides/plugins.md)
- **[Command reference](commands/README.md)** — one entry per command,
  generated from the command tree (a CI drift check keeps it in lockstep with
  the binary). `okfctl <cmd> --help` is the authoritative, always-current form.

## Contributors — building the tool

- [PRD.md](PRD.md) — the product requirements and non-goals.
- [Architecture Decision Records](adr/README.md) — why the architecture is the
  way it is.
- [Plans](plans/2026-07-22-roadmap.md) and [specs](specs/) — the dated record of
  how each increment was designed and built. A dated plan is a snapshot of intent
  at a moment; for current behavior read the guides above.

## Spec readers — the format itself

okfctl consumes the Open Knowledge Format; it does not author it. The
authoritative source is the upstream
[OKF v0.2 specification](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md).
Confirm the version line (`**Version 0.2**`) before citing a section number —
§-numbering shifted between v0.1 and v0.2.
