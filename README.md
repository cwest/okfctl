# okfctl

A CLI for managing Open Knowledge Format (OKF) bundles.

See [docs/PRD.md](docs/PRD.md) for the tool spec and
[docs/plans/2026-07-22-roadmap.md](docs/plans/2026-07-22-roadmap.md)
for the full plan. The walking skeleton (increment 1) covered bundle,
node, validate, and config; this increment adds lifecycle management for
the reserved `index.md` and `log.md` files.

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

A binary built straight from source with a plain `go build` (no release
metadata injected) reports `dev`.

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
- `node refresh <bundle> [path] [--dry-run]` — bulk-fix stale `modified` timestamps: rewrite every drifting node's `modified` to its git last-commit day (the remediation for the git drift `validate` reports). `created` is immutable and never touched, the body is preserved verbatim, and `log.md`/`index.md` are maintained. A trailing path fixes a single node; `--dry-run` lists what would change and writes nothing. Degrades to a no-op outside a git repo, and exits non-zero only on real failure.
- `node rm <path> --bundle <dir> [--dry-run]` — remove a node and report any nodes orphaned as a result
- `index build [dir]` — regenerate `index.md` from the current bundle
- `index check [dir]` — verify `index.md` is current; nonzero exit if stale
- `log append [dir] --message <text>` — append a dated entry to `log.md`
- `log show [dir]` — print the change history
- `validate <dir>` — validate a bundle against the OKF spec floor. It also reports **git drift** — any node whose frontmatter `modified` disagrees with its git last-commit date — as advisory warnings (read-only; run `node refresh` to fix them; degrades to nothing outside a git repo). With `--templates` it additionally runs the opt-in type-template overlay (§9.4), reporting **template drift** (a node missing a required field or body section its governing template declares) as warnings — advisory by default (exit 0), `--strict` exits non-zero on any drift. Spec-floor violations always fail regardless of `--templates`/`--strict`; the overlay never leaks into the floor (unknown type values still pass, §7.4).
- `template list [dir]` — list the type templates a bundle declares (target type, required-field and body-section counts). Templates are authored as ordinary OKF nodes (`type: Type Template`); nothing lives in tool config.
- `template show <target-type> [dir]` — show one template's required/recommended fields and body sections.
- `plugin list [--path <PATH>]` — list `okfctl-<name>` plugin executables discovered on `PATH` (sorted by name, first-on-PATH wins). Plugins extend okfctl `git`/`kubectl`-style: an unknown subcommand `okfctl foo bar` execs an `okfctl-foo` binary found on `PATH`, passing through `bar` plus the remaining flags and environment (with `OKFCTL` set to the core binary's path so a plugin can call back), and propagates the plugin's exit code. Built-in subcommands always take precedence; an unknown subcommand with no matching plugin produces the usual error plus a did-you-mean suggestion. Executable detection uses Unix permission bits (macOS/Linux); Windows is not yet supported.
- `plugin install <source> [--dir <dir>]` — copy an `okfctl-<name>` executable into the managed plugins dir so `plugin list` and dispatch discover it. The default destination is `$OKFCTL_CONFIG_HOME/plugins` (or `<user config dir>/okfctl/plugins`), the same config-home convention `config` uses; override with `--dir`. Put that directory on your `PATH`. The source's base name must follow `okfctl-<name>`; the copy is written with execute bits. If the destination is not on your `PATH`, install prints a note to stderr so the plugin is not silently undiscoverable.

### okfctl-search — offline semantic search (plugin)

`okfctl-search` is a bundled plugin (a separate static binary; invoke as `okfctl search …` via plugin dispatch, or directly). It adds **semantic** search over a bundle's concept nodes, fully offline, with zero runtime dependencies — no Python, no ONNX, no `sqlite-vec`, no model server. It shares the exact embedding protocol with `cwest/knowledge-base` so vectors are cross-verifiable.

- `okfctl-search index build [bundle-dir]` — embed every concept node into `.okfctl/index.db`, recording the embedder model + dimension. Content-hash keyed: an unchanged node is not re-embedded; deterministic for a fixed embedder.
- `okfctl-search --semantic "query" [bundle-dir]` — rank nodes by cosine similarity to the query (top-`--k`, default 5). Refuses an index built under a different model (rebuild with `index build`).
- `okfctl-search related <node-path> [bundle-dir]` — a node's nearest neighbors (self excluded); the neighbor set the spec (§8.6) says `lint` will consume for its semantic checks in a later increment (not yet wired).
- `--embedder hash` (default) is the offline, dependency-free embedder. It is deterministic and needs no model, but it is *lexical* — it matches tokens, not meaning.

#### Real semantic search with `--embedder model2vec`

`--embedder model2vec` runs a genuine static embedding model (for example [`minishlab/potion-base-8M`](https://huggingface.co/minishlab/potion-base-8M)) in **pure Go** — the BERT WordPiece tokenizer and the Model2Vec inference math are both ported into `internal/search`, so there is still no CGO, no Python, and no ONNX runtime. Vectors match the upstream `model2vec` library's own output to within `1e-5`, which is verified against the real model in the test suite.

`okfctl` never downloads a model at runtime. Point it at a directory you already have on disk:

```sh
# once — persisted in okfctl's JSON config
okfctl config set model_path ~/models/potion-base-8M
okfctl-search --embedder model2vec index build ./my-bundle
okfctl-search --embedder model2vec --semantic "tannin structure" ./my-bundle

# or per-invocation, overriding the config
okfctl-search --embedder model2vec --model-path ~/models/potion-base-8M --semantic "…" ./my-bundle
```

The directory needs the standard model2vec layout: `config.json`, `model.safetensors`, and `tokenizer.json` (or `vocab.txt`). If no path is configured, `model2vec` fails with an actionable error rather than silently falling back to `hash` — a query answered by the wrong embedder is worse than one that refuses to run. An index records the model it was built with, so switching embedders requires a rebuild.
- `lint <dir>` — report curation health findings (orphans, missing cross-references, coverage gaps, type-value hygiene). Advisory by default (exits 0 even with findings); `--strict` exits non-zero on any finding, `--coverage-threshold N` tunes the coverage-gap check (default 3). `lint` never mutates the bundle.
- `lint <dir> --semantic` — add the two similarity-driven checks (see below). Requires an index built by `okfctl-search index build`; **no embedding model is needed to lint**, because core only ever *reads* an index.

#### Semantic lint (`--semantic`)

Structural lint asks *"is anything linked to this?"*. Semantic lint asks *"is anything even about the same thing?"* — the curation question the graph alone can't answer:

| check | finding | reads as |
|---|---|---|
| `similar-unlinked` | two nodes scoring ≥ `--similarity-threshold` (default `0.80`) with **no link in either direction** | *"these cover the same ground and don't reference each other — missing cross-reference?"* |
| `no-semantic-neighbors` | a node whose **best** neighbor falls below `--isolation-floor` (default `0.20`) | *"nothing in the corpus is close to this — dead concept, or missing context?"* |

```sh
okfctl-search index build ./my-bundle     # the plugin builds (needs a model)
okfctl lint ./my-bundle --semantic        # core reads (needs none)
```

Three deliberate behaviors:

- **Opt-in.** Without `--semantic`, output is unchanged and the index is never read.
- **A missing index is an error, not a silent skip** — it names `okfctl-search index build`. A quiet structural-only fallback would let CI believe semantic checks ran when they did not.
- **Index drift is surfaced.** Nodes added since the last `index build` produce one `stale-index` finding listing them, so a partial pass never reads as a clean one.

Findings are only as meaningful as the embedder that built the index. With the default `hash` embedder, `similar-unlinked` effectively means "shares vocabulary"; with `--embedder model2vec` it means genuinely related subject matter. Build the index with `model2vec` if you intend to act on these findings.
- `search "query" [dir]` — **core lexical + graph-structural search**, stdlib-only, no model or index. Matches concept nodes case-insensitively by title, tag, type, or body substring; restrict the surface with `--field title|tag|type|body` (default `any`). Reserved files (`index.md`/`log.md`) are never results. A zero-result query is not an error. Add `--json` for a deterministic, CI-diffable array (path/title/type/neighborhood/matched_on).
- `search --neighbors <node-path> [dir]` — graph-structural query: the concept nodes within `--depth` hops (default 1) of a node in the link graph. Edges are treated as **undirected** (a node is a neighbor whether it links to the start or the start links to it), so a reader's traversal is symmetric. Results are ordered by (depth, path); `--json` emits the same fields plus `depth`. This is the core, always-available baseline; the **semantic** side of search is the separate `okfctl-search` plugin above (`okfctl-search --semantic …`).
- `graph export <dir> --format json|dot` — export the concept-node link graph in a machine format (deterministic, CI-diffable). `json` (default) emits nodes (path/title/type/neighborhood/orphan) + edges; `dot` emits Graphviz. For SVG, pipe DOT to Graphviz: `okfctl graph export --format dot | dot -Tsvg > graph.svg`.
- `serve <dir> --addr 127.0.0.1:8080` — start a local web server rendering the bundle as an interactive knowledge graph (click a node to inspect, follow edges, orphans highlighted, filter by type/neighborhood). The viewer is embedded in the binary — no separate install. Binds loopback by default; override with `--addr`.
- `config set <key> <value>` — set a config value
- `config get <key>` — read a config value
- `config list` — list all config values
- `registry add <name> <git-url>` — register (or re-point) a named remote bundle source. Named remotes are plain git URLs — this is `git remote` for OKF bundles, **not** a hosted service, account system, or schema registry. They live in the one okfctl config store (keyed `registry.<name>`), so there is no second config file.
- `registry list` — list the registered `name` → `url` sources, sorted by name.
- `registry show <name>` — print a source's git URL (nonzero exit on an unknown name).
- `registry remove <name>` (alias `rm`) — unregister a source.
- `connect <name|git-url> [dir]` — materialize a remote bundle source into a local directory over git. A registered name resolves to its URL; an ad-hoc git URL is used directly. A fresh destination is `git clone`d; an existing checkout of the same source is fast-forwarded (`git pull --ff-only`, never a history-rewriting merge); a non-empty directory that is not that checkout is left untouched. `okfctl` shells out to `git` (no new dependency) and does no authentication of its own — reaching a private URL is git's concern (ssh agent, credential helper). Default `dir` is a directory named after the source (trailing `.git` stripped), matching `git clone`.
- `completion <bash|zsh|fish>` — generate a shell completion script
- `version` — print the okfctl version (also `okfctl --version`); reports the release tag injected at build time, or `dev` for a plain source build

## What validate checks

`validate` enforces the OKF spec floor only: every node must carry a
non-empty `type` (OKF §7). Unknown type *values* are allowed — the
walking skeleton does not enforce a taxonomy, so `--type Reference`,
`--type Concept`, or any other string all pass.

## License

Apache-2.0. See [LICENSE](LICENSE).
