# 1. Build okfctl in Go

- **Status:** Accepted (2026-07-22)
- **Deciders:** Casey West
- **Resolves:** PRD [§13.1](../PRD.md#131-implementation-language-and-library-stackdecided-go)

## Context

The implementation language and core library stack is a foundational,
costly-to-redo decision that gates every increment. Rust and Go were the two
serious candidates: both produce a single static binary, both have mature CLI
frameworks, and both can embed static web assets. To decide on evidence rather
than preference, a prove-or-kill spike built the same minimal proof-of-concept in
both languages on-box—a noun-verb subcommand tree with flags, a
`git`/`kubectl` PATH-dispatch extension with environment context and exit-code
passthrough, and a single-binary embedded static web server—and exercised all
three capabilities in each.

Both languages cleared every required capability. The decision therefore turned
on four measured deltas:

1. **Cross-compilation.** Go cross-compiled statically-linked `linux/amd64`,
   `linux/arm64`, `windows/amd64`, and `darwin/amd64` from one macOS box, one
   command each, with zero toolchain setup (verified: `file` reports a static
   ELF). Rust's cross-build to Linux from macOS failed at link without a
   cross-linker layer—recurring CI complexity for a multi-platform release
   matrix.
2. **Dependency surface for `serve`.** Go's embedded visualizer is stdlib-only
   (`net/http` + `go:embed`, zero third-party dependencies) versus Rust's
   ~90-crate `axum` → `tokio` → `hyper` tree—far less supply-chain surface to
   audit for the same single-binary result.
3. **YAML-frontmatter maturity.** Every OKF file is Markdown with YAML
   frontmatter, so YAML parsing is load-bearing. Go's `gopkg.in/yaml.v3` is
   mature and maintained; Rust's canonical `serde_yaml` is deprecated (latest
   published version `0.9.34+deprecated`) and its successor `serde_yaml_ng` is
   young.
4. **Prior art.** The in-the-wild OKF-CLI ecosystem is overwhelmingly Go
   (`openknowledge-sh/openknowledge`, `okfcli/okf`, `okfdump`,
   `agentic-wiki/wiki`), giving the richest body of directly-applicable prior art
   to study.

## Decision

Build okfctl in **Go**. Each measured delta favored Go or was neutral; none
favored Rust on a dimension that matters for a developer/agent CLI shipped as a
release artifact.

The named stack that follows from this choice is recorded in the PRD's corrected
§13.1 and, for the vector store specifically, in [ADR 0004](0004-flat-json-vector-store.md).
Its stdlib-forward shape—`spf13/cobra` for the command tree,
`goldmark`/`goldmark-meta` for Markdown+frontmatter, `gopkg.in/yaml.v3` for YAML,
`net/http` + `go:embed` for the server, and `CGO_ENABLED=0` static builds under
GoReleaser—is a direct consequence of the deltas above, not an independent set
of decisions.

## Consequences

**What it buys.** One-command static cross-compilation for the full release
matrix from a single macOS box; a minimal, auditable dependency tree; a mature
YAML path for the format's load-bearing frontmatter; and the deepest pool of
directly-applicable prior art. `CGO_ENABLED=0` keeps every core artifact
statically linked with no C toolchain, which is what makes the zero-runtime-
dependency promise (PRD §5.1) real.

**What it costs.** Rust's one clear win was a ~4x smaller binary (2.1 MB vs
8.6 MB); choosing Go accepts the larger artifact. That is a minor cost for a CLI
distributed as a release download rather than embedded in a size-constrained
target. Go's static-embedding ergonomics are also why the vector store cannot use
a CGO/C-extension like `sqlite-vec` without breaking the static-binary guarantee—a constraint this decision imposes downstream and that
[ADR 0004](0004-flat-json-vector-store.md) resolves. Finally, the differentiating
`lint` loop is I/O- and LLM-latency-bound rather than CPU-bound, so Go's raw
compute is neither an advantage nor a liability here; the loop is architected
around caching results by content hash regardless of language.
