# Concepts

Read this first; it makes every other page make sense.

## Bundle

A **bundle** is a directory tree of Markdown files that conforms to the Open
Knowledge Format. `okfctl bundle init` scaffolds one: the two reserved files,
with a bundle-root `index.md` whose frontmatter declares `okf_version: "0.3"`
(§8.1, §12). A fresh bundle has zero concept nodes.

`okfctl bundle info <dir>` summarizes a bundle: node count, reserved-file count,
and the `okf_version` with where it was declared (`0.3 (declared in index.md)`),
or `undeclared` when the bundle names no version.

## Node

A **node** is a single concept file: Markdown with a YAML frontmatter block. The
one floor requirement is a non-empty `type` (OKF §4.1). Its path is its identity,
so moving a node is a graph operation—`okfctl node mv` rewrites every inbound
link rather than leaving them dangling.

`type` values are **open** (§4.1): `Reference`, `Concept`, or any other string
all pass `validate`. The tool doesn't invent a taxonomy.

## Reserved files: `index.md` and `log.md`

Two filenames are reserved (§3.1):

- **`index.md`**—a generated directory listing. Per §8 one is emitted in each
  directory that holds concepts or subdirectories. Regenerate with `okfctl index
  build`; verify with `okfctl index check`. Only the bundle-root index carries
  frontmatter: §8.1 bundle metadata (`okf_version`, `title`, `description`,
  `license`, extension keys), which `index build` keeps verbatim.
- **`log.md`**—the bundle's change history. Append with `okfctl log append`;
  read with `okfctl log show`.

These are maintained by the tool; you don't hand-edit them.

## The link graph

Nodes reference each other with ordinary Markdown links. Those links form a
directed graph, but for traversal okfctl treats edges as **undirected**—a node
is a neighbor whether it links to you or you link to it. `okfctl graph export`
dumps the graph (JSON or Graphviz DOT); `okfctl serve` renders it interactively;
`okfctl search --neighbors` queries it.

## Spec floor vs. overlay

okfctl enforces the **spec floor** for everyone: the minimum every conformant
bundle must satisfy. Anything stricter—a team's required frontmatter fields or
body sections—lives behind an explicit opt-in **overlay**: type-templates,
enabled with `validate --templates`. The overlay never leaks into the
floor. An unknown `type` value and an unrecognized future frontmatter key both
still pass the floor (§11.2); rejecting them would be over-conformance, itself a spec
violation.

Templates are authored as ordinary OKF nodes (`type: Type Template`); nothing
lives in tool config. Inspect them with `okfctl template list` / `okfctl template
show`.

## Versions: v0.1, v0.2, v0.3

OKF is at **v0.3**, and okfctl targets it: `okfctl bundle init` declares
`okf_version: "0.3"`, and `okfctl migrate` upgrades older bundles to it.

A bundle declares its version with `okf_version` in the frontmatter of its
bundle-root `index.md` (§8.1, §12). okfctl reads that declaration rather than
assuming; a legacy `.okf` sidecar is read only when the root index declares
nothing. A bundle that declares no version is reported as `okf_version:
undeclared (v0.2 fallbacks apply, §12.1)`: okfctl reads it with the v0.3 rules,
plus the older fallbacks below, because those key on what the bundle *declares*.

v0.3 has two breaking changes (§13.1). A link or path field without a leading `/`
resolves from the directory of its file, and only *current* verification events
count toward the trust tier. A bundle that declares v0.2 or earlier, or no
version, MAY still resolve such a value from the bundle root (§12.1), so it keeps
working unchanged.

v0.2 (now v0.3 Appendix B) renamed `timestamp` → `generated.at` and the body
`# Citations` list → frontmatter `sources`. Consumers MAY fall back to the legacy
forms, so **v0.1 bundles stay readable**.

Per §12, a consumer that doesn't understand a declared version attempts
best-effort consumption rather than refusing the bundle, so an unrecognized
`okf_version` is never a hard failure. To convert an older bundle in place, see
[Migrating a bundle to OKF v0.3](guides/migrating.md).
