# Changelog

All notable changes to `okfctl` are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Entries for tagged releases mirror the notes published on the
[GitHub releases page](https://github.com/cwest/okfctl/releases). On each new
tagged release, goreleaser generates the release notes from the commit log
(see `.goreleaser.yaml`).

## [Unreleased]

### Added

- Homebrew tap (`brew install cwest/tap/okfctl`) via a goreleaser cask published
  to `cwest/homebrew-tap`.
- One-line installer served at `https://okfctl.dev/install.sh` — detects OS/arch,
  downloads the matching release archive, verifies it against the published
  `checksums.txt`, and installs both `okfctl` and `okfctl-search`. Refuses to
  install on a checksum mismatch.
- Windows builds (amd64 + arm64), shipped as `.zip` archives.
- Linux system packages (`.deb` and `.rpm`) via nfpm.
- Software Bill of Materials (SBOM) for every archive, generated with syft.
- Keyless artifact signing with cosign (Sigstore/Fulcio) using the release
  workflow's OIDC identity; verifiable with `cosign verify-blob`.

### Changed

- `okfctl version` from a `go install github.com/cwest/okfctl@latest` build now
  reports the installed module version instead of `dev`, via a
  `debug.ReadBuildInfo()` fallback. Release builds still report the injected tag,
  and a plain checkout build without usable module metadata still reports `dev`.

## [0.2.0] - 2026-08-02

`okfctl` targets **OKF v0.2**. New bundles are created at `okf_version: 0.2`, and
v0.1 bundles remain readable — v0.2's two breaking renames (`timestamp` →
`generated.at`, body `# Citations` → frontmatter `sources`) are read with a v0.1
fallback, per spec §12's best-effort consumption rule.

### Added

- **`okf migrate`** — a two-phase v0.1 → v0.2 upgrade path: a mechanical pass
  plus a judgment queue for calls a tool should not make alone. Ships with the
  `okf-migrate-plan` skill.
- **`okfctl-api`** — an HTTP API plugin over a bundle, serving `/stats` and
  `/graph` (walking skeleton; ships with an ADR describing its shape).
- **Passage-level search indexing** — sub-node passages are indexed and returned
  as snippets, so a hit points at the paragraph rather than the file. Retrieval
  quality on the real corpus improved 0.545 → 0.909 (~12x index size).
- **Query scoping** — filter semantic queries by path, type, and tag, with
  optional recency decay.
- **`lint --json`** — machine-readable findings for CI and downstream tooling.
- **Bulk index promotion** — promote directory-as-concept `index.md` nodes in
  bulk.

### Changed

- **Provenance** reads the v0.2 `generated` / `verified` / `sources` families,
  falling back to the v0.1 forms when a bundle declares the older version.
- **Freshness** resolves through `generated.at` rather than filesystem
  `modified` / `created`, so a regenerated node reports its real age.
- **Prose-only scanning** — cross-reference and time-marker detection no longer
  reads link URLs as prose, eliminating a class of false `missing-xref` and
  time-sensitive findings (both inline and reference-style links excluded).
- **Title-as-home-node** — a node whose title leads with a term is credited as
  that term's home, closing a coverage-gap false positive.
- A model2vec directory containing `tokenizer.json` is now accepted for search.
- Section citations corrected throughout for the v0.1 (11 sections) → v0.2 (13
  sections) numbering shift.

### Fixed

- Broken internal links are gated as defects when they look like defects.
- Vendored and derived directories are skipped during the bundle walk.
- A bulk mechanical commit can declare itself as such and skip the git-drift
  check.

## [0.1.0] - 2026-07-27

Initial release: the `okfctl` command-line tool for authoring and maintaining
Open Knowledge Format (OKF) bundles, targeting OKF v0.1.

### Added

- **Bundle lifecycle** — `bundle init`, node authoring, and `validate` (the
  walking skeleton).
- **Reserved-file lifecycle** — generation and maintenance of the reserved
  `index.md` and `log.md` files, with nested per-directory indexes and
  dir-relative links (§6).
- **Node mutation verbs** — `node edit`, `node mv`, `node rm`, and `node refresh`
  (bulk-fix of stale timestamps).
- **Derived-artifact maintenance** — `created` / `modified` metadata, `log.md`,
  and `index.md` maintained by default, with git-drift findings.
- **Lint** — structural checks and semantic curation checks (`lint --semantic`);
  precision hardened on the real corpus (2,438 findings → 21).
- **Analyze** — `okfctl analyze`, a proactive curation report covering freshness
  and clusters.
- **Graph** — link-graph export and serve; DOT-pipe as the sanctioned SVG path.
- **Search** — core lexical + graph-structural search, plus the `okfctl-search`
  semantic plugin (Model2Vec safetensors loader, WordPiece tokenizer).
- **Plugins** — `okfctl-<name>` plugin dispatch over `PATH`, with `plugin
  install`.
- **Remote sources** — `registry` / `connect` for git-backed remote bundle
  sources.
- **Release mechanics** — goreleaser, version injection via ldflags, and
  installable binaries.
- **Documentation & conformance** — the PRD, ADRs, ship-with-the-tool skills
  (`okf-authoring`, `okf-curation-health`, `okf-semantic-search`), and a
  spec-conformance suite proving okfctl's output validates clean.

[Unreleased]: https://github.com/cwest/okfctl/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/cwest/okfctl/releases/tag/v0.2.0
[0.1.0]: https://github.com/cwest/okfctl/releases/tag/v0.1.0
