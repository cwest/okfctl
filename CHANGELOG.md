# Changelog

All notable changes to `okfctl` are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Entries for tagged releases mirror the notes published on the
[GitHub releases page](https://github.com/cwest/okfctl/releases). On each new
tagged release, goreleaser generates the release notes from the commit log
(see `.goreleaser.yaml`).

## [Unreleased]

## [0.4.0] - 2026-08-19

### Added

- Agent Plugins 1.0.0 packaging: a root `plugin.json` that lets a compatible
  agent client install okfctl's shareable skills as one unit. The manifest is
  spec-only plus `AGENTS.md`, pins the `$schema` to the Agent Plugins 1.0.0
  schema URL, and keeps okfctl-specific config under `extensions."dev.okfctl"`.
  CI validates it against the pinned schema, rejects a deliberately malformed
  fixture, and runs the skill-leak gate over the packaged payload.
- A skill-command contract check that installs the released binary from the
  public install path and runs every command the shipped skills document, so a
  skill drifting out of sync with the CLI fails CI even when the manifest stays
  byte-identical.

### Changed

- User-facing prose across the README, the hand-written docs, and the command
  help text was rewritten in a plainer voice, and the README now leads with the
  problem okfctl solves rather than a command list.
- House prose style is the closed-up Chicago em dash (`word—word`); the shipped
  Markdown surfaces were converted to it.

### Fixed

- Homepage inline-boundary spacing and the brand wordmark render correctly.
- The retired Go Report Card badge was dropped from the README.

### Build

- A CI prose gate scans the shipped Markdown surfaces and the built site for a
  negative-listing slop cadence and for spaced em dashes, failing the build on
  either. The gate rides one detector set across both an HTML and a Markdown
  front end, and both of its controls are proven in CI.
- The release path now refuses to publish a tag whose version has no matching,
  dated, non-empty `## [X.Y.Z]` section in this changelog. The check runs before
  goreleaser, so an empty release can never ship.

## [0.3.1] - 2026-08-13

### Fixed

- The published documentation site rebuilds from `main` after a release, so the
  live docs no longer lag a freshly tagged version.

### Build

- The Go toolchain was bumped to 1.26.6 to pick up patched standard-library
  fixes.

## [0.3.0] - 2026-08-13

### Added

- **Installable, verifiable distribution.** A one-line installer served at
  `https://okfctl.dev/install.sh` detects OS/arch, downloads the matching
  release archive, verifies it against the published `checksums.txt`, and
  installs both `okfctl` and `okfctl-search`, refusing to install on a checksum
  mismatch. A Homebrew cask (`brew install cwest/tap/okfctl`) is published to
  `cwest/homebrew-tap`. Releases now include Windows builds (amd64 and arm64) as
  `.zip` archives and Linux system packages (`.deb` and `.rpm`) via nfpm.
- **Supply-chain provenance.** Every archive ships a Software Bill of Materials
  generated with syft, and release artifacts are signed keyless with cosign
  (Sigstore/Fulcio) using the release workflow's OIDC identity, verifiable with
  `cosign verify-blob`.
- **`GET /api/v1/search`**—the `okfctl-api` plugin serves search off a resident
  index, with the lexical gate exposed as a query parameter, unknown parameters
  rejected, and the recency-decay floor applied. The API is v0.2-aware,
  surfacing `status`, `epistemic`, and `generated.at`.
- **Query scoping for search**—repeatable, negatable `--path` / `--type` /
  `--tag` (and `--not-*`) filters, a `--lexical-gate` that preserves lexical
  recall alongside semantic ranking, and bounded recency decay with a
  configurable relevance floor and half-life.
- **The okfctl.dev site**—a bespoke Astro site and its GitHub Pages pipeline,
  serving the documentation at clean URLs, with a social card, favicon,
  `robots.txt`, `llms.txt`, and a 404 page, shipped through the theming seam.
- **Generated command reference**—the command reference is generated from the
  cobra tree with a CI drift gate, and every command gained a long description
  and a runnable example.
- A TACA-style trustworthiness evaluation pass for knowledge-base nodes.
- Fuzz targets for the untrusted-input parsers.

### Changed

- **`validate`, `lint`, and `analyze` are OKF v0.2-aware**—they read the v0.2
  provenance families and resolve nested provenance in template
  `required_fields`, while keeping the v0.1 fallbacks.
- `okfctl version` from a `go install github.com/cwest/okfctl@latest` build now
  reports the installed module version instead of `dev`, via a
  `debug.ReadBuildInfo()` fallback. Release builds still report the injected tag,
  and a plain checkout build without usable module metadata still reports `dev`.
- `analyze` prefers `verified[].at` as the freshness basis.
- The README was rewritten as an on-ramp rather than a command dump, and the
  homepage copy had its AI-slop tells removed.
- CI pins its actions to commit SHAs, attests build provenance, runs the site
  test suite rather than only the site build, and re-enables `gosec` and
  `errcheck` with per-site triage.

### Fixed

- Release signing publishes correctly by using cosign's bundle format.
- Search orders the preserved lexical tail by score, and rejects an
  out-of-range `--decay-floor` or a negative `--half-life`.
- The documentation site publishes at clean URLs instead of `/_generated/`, and
  the 404 page and `llms.txt` point at the clean doc URLs.
- A v0.1 flat-string `sources` list is accepted for template `required_fields`.

## [0.2.0] - 2026-08-02

`okfctl` targets **OKF v0.2**. New bundles are created at `okf_version: 0.2`, and
v0.1 bundles remain readable—v0.2's two breaking renames (`timestamp` →
`generated.at`, body `# Citations` → frontmatter `sources`) are read with a v0.1
fallback, per spec §12's best-effort consumption rule.

### Added

- **`okf migrate`**—a two-phase v0.1 → v0.2 upgrade path: a mechanical pass plus
  a judgment queue for calls a tool should not make alone. Ships with the
  `okf-migrate-plan` skill.
- **`okfctl-api`**—an HTTP API plugin over a bundle, serving `/stats` and
  `/graph` (walking skeleton; ships with an ADR describing its shape).
- **Passage-level search indexing**—sub-node passages are indexed and returned
  as snippets, so a hit points at the paragraph rather than the file. Retrieval
  quality on the real corpus improved 0.545 → 0.909 (~12x index size).
- **Query scoping**—filter semantic queries by path, type, and tag, with
  optional recency decay.
- **`lint --json`**—machine-readable findings for CI and downstream tooling.
- **Bulk index promotion**—promote directory-as-concept `index.md` nodes in
  bulk.

### Changed

- **Provenance** reads the v0.2 `generated` / `verified` / `sources` families,
  falling back to the v0.1 forms when a bundle declares the older version.
- **Freshness** resolves through `generated.at` rather than filesystem
  `modified` / `created`, so a regenerated node reports its real age.
- **Prose-only scanning**—cross-reference and time-marker detection no longer
  reads link URLs as prose, eliminating a class of false `missing-xref` and
  time-sensitive findings (both inline and reference-style links excluded).
- **Title-as-home-node**—a node whose title leads with a term is credited as
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

- **Bundle lifecycle**—`bundle init`, node authoring, and `validate` (the
  walking skeleton).
- **Reserved-file lifecycle**—generation and maintenance of the reserved
  `index.md` and `log.md` files, with nested per-directory indexes and
  dir-relative links (§6).
- **Node mutation verbs**—`node edit`, `node mv`, `node rm`, and `node refresh`
  (bulk-fix of stale timestamps).
- **Derived-artifact maintenance**—`created` / `modified` metadata, `log.md`,
  and `index.md` maintained by default, with git-drift findings.
- **Lint**—structural checks and semantic curation checks (`lint --semantic`);
  precision hardened on the real corpus (2,438 findings → 21).
- **Analyze**—`okfctl analyze`, a proactive curation report covering freshness
  and clusters.
- **Graph**—link-graph export and serve; DOT-pipe as the sanctioned SVG path.
- **Search**—core lexical + graph-structural search, plus the `okfctl-search`
  semantic plugin (Model2Vec safetensors loader, WordPiece tokenizer).
- **Plugins**—`okfctl-<name>` plugin dispatch over `PATH`, with `plugin
  install`.
- **Remote sources**—`registry` / `connect` for git-backed remote bundle
  sources.
- **Release mechanics**—goreleaser, version injection via ldflags, and
  installable binaries.
- **Documentation & conformance**—the PRD, ADRs, ship-with-the-tool skills
  (`okf-authoring`, `okf-curation-health`, `okf-semantic-search`), and a
  spec-conformance suite proving okfctl's output validates clean.

[Unreleased]: https://github.com/cwest/okfctl/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/cwest/okfctl/releases/tag/v0.4.0
[0.3.1]: https://github.com/cwest/okfctl/releases/tag/v0.3.1
[0.3.0]: https://github.com/cwest/okfctl/releases/tag/v0.3.0
[0.2.0]: https://github.com/cwest/okfctl/releases/tag/v0.2.0
[0.1.0]: https://github.com/cwest/okfctl/releases/tag/v0.1.0
