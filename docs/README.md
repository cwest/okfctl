# okfctl documentation

## Using okfctl

New to okfctl? Start with [Install](install.md). It shows what each method
prints, then gets into the archives, the Debian and RPM packages, release
signatures, and building from a checkout.

Then read [Concepts](concepts.md) for bundles, nodes, the reserved `index.md`
and `log.md` files, the link graph, and how the spec's floor differs from your
team's type templates.

Each guide works through one job:

- [Start and author a bundle](guides/authoring.md)
- [Update `index.md` and fix freshness drift](guides/index-and-freshness.md)
- [Check curation health with `lint` and `analyze`, and fail CI with `--strict`](guides/curation-health.md)
- [Search by text, by graph, and by meaning with `okfctl-search`](guides/search.md)
- [Migrate a bundle to OKF v0.3](guides/migrating.md)
- [Pull in remote bundles with `registry` and `connect`](guides/remote-sources.md)
- [Extend okfctl with plugins](guides/plugins.md)

The [command reference](commands/README.md) has an entry for each command, and
a test checks it against the code. `okfctl <cmd> --help` prints the same text.

## Working on okfctl

- [PRD.md](PRD.md) has the product requirements and what okfctl won't do.
- The [architecture decision records](adr/README.md) say why the code looks the
  way it does.
- The [plans](plans/2026-07-22-roadmap.md) and [specs](specs/) record how each
  change was designed, as of the date on each. For how okfctl behaves today,
  read the guides.

## Reading the spec

okfctl reads and writes the Open Knowledge Format, but the format's authors
decide what it says, in the upstream
[OKF v0.3 specification](https://github.com/open-knowledge-format/open-knowledge-format/blob/main/SPEC.md).
Check its version line (`**Version 0.3**`) when you cite a section, since the
section numbers moved between v0.1 and v0.2, and v0.3 added subsections.

okfctl targets v0.3. [This record](specs/2026-11-26-okf-v0.3-gap-analysis.md)
maps each v0.3 change to the okfctl code that implements it. Don't cite the old
`GoogleCloudPlatform/open-knowledge-format` repo. It's deprecated and stuck at
v0.2.
