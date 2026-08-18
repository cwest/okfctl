# Concepts

Read this first; it makes every other page make sense.

## Bundle

A **bundle** is a directory tree of Markdown files that conforms to the Open
Knowledge Format. `okfctl bundle init` scaffolds one: the two reserved files plus
a bundle-root `.okf` sidecar whose `okf_version` marks the target spec version
(§12). A fresh bundle has zero concept nodes.

`okfctl bundle info <dir>` summarizes a bundle — node count, reserved-file count,
and the declared `okf_version`.

## Node

A **node** is a single concept file: Markdown with a YAML frontmatter block. The
one floor requirement is a non-empty `type` (OKF §7). Its path is its identity,
so moving a node is a graph operation — `okfctl node mv` rewrites every inbound
link rather than leaving them dangling.

`type` values are **open** (§7.4): `Reference`, `Concept`, or any other string
all pass `validate`. The tool doesn't invent a taxonomy.

## Reserved files: `index.md` and `log.md`

Two filenames are reserved (§3.1):

- **`index.md`** — a generated directory listing. Per §8 one is emitted in each
  directory that holds concepts or subdirectories. Regenerate with `okfctl index
  build`; verify with `okfctl index check`. Only the bundle-root index carries
  frontmatter (the §12 `okf_version` marker).
- **`log.md`** — the bundle's change history. Append with `okfctl log append`;
  read with `okfctl log show`.

These are maintained by the tool; you don't hand-edit them.

## The link graph

Nodes reference each other with ordinary Markdown links. Those links form a
directed graph, but for traversal okfctl treats edges as **undirected** — a node
is a neighbor whether it links to you or you link to it. `okfctl graph export`
dumps the graph (JSON or Graphviz DOT); `okfctl serve` renders it interactively;
`okfctl search --neighbors` queries it.

## Spec floor vs. overlay

okfctl enforces the **spec floor** for everyone: the minimum every conformant
bundle must satisfy. Anything stricter — a team's required frontmatter fields or
body sections — lives behind an explicit opt-in **overlay**: type-templates,
enabled with `validate --templates` (§9.4). The overlay never leaks into the
floor. An unknown `type` value and an unrecognized future frontmatter key both
still pass the floor; rejecting them would be over-conformance, itself a spec
violation.

Templates are authored as ordinary OKF nodes (`type: Type Template`); nothing
lives in tool config. Inspect them with `okfctl template list` / `okfctl template
show`.

## The v0.1 → v0.2 story

OKF is at **v0.2**. v0.2 is a minor bump with two deliberate breaking renames:
`timestamp` → `generated.at`, and the body `# Citations` list → frontmatter
`sources`. Consumers MAY fall back to the legacy forms, so **v0.1 bundles stay
readable** — v0.2 is the new default, not a floor that ejects existing bundles.

A bundle declares its own target via `okf_version` in its `.okf` sidecar; the
tool reads it rather than assuming. Per §12, a consumer that doesn't understand a
declared version attempts best-effort consumption rather than refusing the bundle,
so an unrecognized `okf_version` is never a hard failure. To convert a v0.1
bundle to v0.2 in place, see [Migrating a v0.1 bundle](guides/migrating.md).
