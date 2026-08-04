# Command reference

**This page is generated from the command tree. Do not edit it by hand.**
Regenerate it with `go generate ./cmd` (or `go run ./cmd/gendocs`); a
CI check (`TestCommandReference_NoDrift`) fails when the committed file drifts
from the binary, so the reference cannot go stale.

`okfctl <cmd> --help` is the authoritative, always-current form for any
command — it prints the same description, flags, and runnable example straight
from the binary. This page mirrors that surface in one place, one section per
command, so it is browsable and linkable (README links to the `#okfctl-<cmd>`
anchors below).

Run `okfctl help` for the top-level list, or `okfctl <cmd> --help` for any
entry below.

## okfctl analyze

Report where a bundle is weak (freshness, gaps, connectivity, clusters, structure)

analyze is a proactive curation REPORT, not a gate. Where lint answers "is this corpus broken?" (a CI gate: --strict exits non-zero), analyze answers "where is this corpus weak?" across five dimensions — coverage gaps, freshness/staleness, connectivity, tag-cluster synthesis candidates, and structure. It never mutates the bundle.

Report semantics: analyze exits 0 whenever the analysis runs successfully, regardless of how many findings it produces. The exit code reflects whether the analysis succeeded, not whether the corpus is perfect. There is deliberately no --strict flag; use lint --strict for a gate.

Pass --json for the machine path (the curation sweep files research cards from it).

```
okfctl analyze [bundle-dir] [flags]
```

Example:

```
# Human-readable weakness report for the current bundle
  okfctl analyze

  # Report on a bundle elsewhere
  okfctl analyze ./bundles/knowledge

  # Machine-readable report (the curation sweep files research cards from this)
  okfctl analyze --json ./bundles/knowledge
```

Flags:

```
      --cluster-min int                 min nodes sharing a tag to flag a synthesis cluster (default 3)
      --coverage-threshold int          min distinct nodes mentioning a term to report a coverage gap (default 3)
      --json                            emit machine-readable JSON instead of the human report
      --no-ignore                       walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default
      --stale-days int                  age (days) past which a node's modified/created is flagged stale (default 180)
      --thin-lines int                  body line count below which a node is thin (default 15)
      --time-sensitive-fraction float   surface a time-sensitive node once its age >= fraction*stale-days (default 0.5); undated marked nodes always surface
```


## okfctl bundle

Bundle lifecycle commands


### okfctl bundle info

Summarize a bundle (node count, spec version)

info prints a one-glance summary of a bundle: how many concept nodes it holds, how many reserved files (index.md/log.md, OKF §3.1) it carries, and the okf_version it declares (§12, read from the bundle-root .okf sidecar). It is read-only and never mutates the bundle. It does not validate conformance — use `okfctl validate` for that.

```
okfctl bundle info [dir]
```

Example:

```
# Summarize the bundle in the current directory
  okfctl bundle info

  # Summarize a bundle elsewhere
  okfctl bundle info ./bundles/knowledge
```


### okfctl bundle init

Scaffold a minimal conformant OKF bundle

init scaffolds a minimal bundle that conforms to the OKF spec floor: the two reserved files (index.md and log.md, OKF §3.1) plus a bundle-root .okf sidecar whose okf_version marks the target version (§12). It does NOT create any concept nodes — a fresh bundle has zero nodes; use `okfctl node new` to add them. It refuses to overwrite an existing bundle.

```
okfctl bundle init [dir]
```

Example:

```
# Scaffold a new bundle in the current directory
  okfctl bundle init

  # Scaffold in a named directory
  okfctl bundle init ./my-bundle
```


## okfctl completion

Generate a shell completion script

completion writes a shell completion script for okfctl to stdout. Source it (or install it where your shell loads completions) to get tab-completion of commands and flags. Supported shells: bash, zsh, fish. It prints the script only — it does not install anything itself.

```
okfctl completion [bash|zsh|fish]
```

Example:

```
# Load bash completions for the current shell
  source <(okfctl completion bash)

  # Install zsh completions
  okfctl completion zsh > "${fpath[1]}/_okfctl"

  # Install fish completions
  okfctl completion fish > ~/.config/fish/completions/okfctl.fish
```


## okfctl config

Get and set okfctl configuration


### okfctl config get

Get a config value

get prints the value stored for a single config key, or exits non-zero if the key is unset. It is read-only. Use `okfctl config list` to see every key at once.

```
okfctl config get <key>
```

Example:

```
# Print one config value
  okfctl config get default.remote
```


### okfctl config list

List all config values

list prints every stored config key and its value, sorted by key. It is read-only. Registered remote sources appear here too, under the registry. key prefix (see `okfctl registry list` for a focused view of those).

```
okfctl config list
```

Example:

```
# Show every stored config key and value
  okfctl config list
```


### okfctl config set

Set a config value

set writes a key/value pair into the single okfctl config store (a flat file under $OKFCTL_CONFIG_HOME or the OS user-config dir). An existing key is overwritten. This is okfctl's own tool configuration, not bundle content — it never touches a bundle.

```
okfctl config set <key> <value>
```

Example:

```
# Pin a default remote registry URL prefix
  okfctl config set default.remote https://github.com/acme
```


## okfctl connect

Clone or update a remote bundle source into a local directory

Materialize a remote OKF bundle source into a local directory over git.
The first argument is a remote name registered with `okfctl registry`, or an ad-hoc git URL. The optional second argument is the destination directory (default: a directory named after the source).

A fresh destination is cloned; an existing checkout of the same source is fast-forwarded (never a history-rewriting merge). A non-empty directory that is not that git checkout is left untouched.

```
okfctl connect <name|git-url> [dir]
```

Example:

```
# Clone a registered remote into a directory named after the source
  okfctl connect knowledge

  # Clone an ad-hoc git URL into a chosen directory
  okfctl connect https://github.com/acme/kb.git ./kb

  # Re-run to fast-forward an existing checkout
  okfctl connect knowledge ./kb
```


## okfctl graph

Query and export the bundle's knowledge graph


### okfctl graph export

Export the graph in a machine format (json or dot)

graph export serializes the bundle's concept-node link graph for use in other tools and CI. Formats: json (default) and dot. For SVG, pipe dot to Graphviz: okfctl graph export --format dot | dot -Tsvg > graph.svg

```
okfctl graph export [bundle-dir] [flags]
```

Example:

```
# Export the current bundle's graph as JSON
  okfctl graph export

  # Export a bundle elsewhere as Graphviz DOT
  okfctl graph export --format dot ./bundles/knowledge

  # Render an SVG via Graphviz
  okfctl graph export --format dot ./bundles/knowledge | dot -Tsvg > graph.svg
```

Flags:

```
      --format string   output format: json or dot (default "json")
      --no-ignore       walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default
```


## okfctl index

Manage the reserved index.md


### okfctl index build

Regenerate index.md from the current bundle

index build regenerates the reserved index.md navigation file(s) from the bundle's current concept nodes (OKF §8: index files are a reserved, generated navigation surface). It rewrites index.md at the bundle root and in each directory that has one; it never edits concept nodes. Run it after adding, moving, or removing nodes by hand (the node verbs regenerate it for you).

```
okfctl index build [dir] [flags]
```

Example:

```
# Regenerate index.md for the current bundle
  okfctl index build

  # Regenerate for a bundle elsewhere
  okfctl index build ./bundles/knowledge
```

Flags:

```
      --no-ignore   walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default
```


### okfctl index check

Verify index.md is current (nonzero exit if stale)

index check verifies the reserved index.md (OKF §8) is in sync with the bundle's current nodes, without writing anything. It is the CI-friendly counterpart to `index build`: it exits zero when the index is current and non-zero (printing what drifted) when a rebuild is needed. Read-only — it never rewrites the index.

```
okfctl index check [dir] [flags]
```

Example:

```
# Verify the index is current (exit 0) or report drift (exit 1)
  okfctl index check

  # Check a bundle elsewhere, e.g. in CI
  okfctl index check ./bundles/knowledge
```

Flags:

```
      --no-ignore   walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default
```


## okfctl lint

Report a bundle's curation health (orphans, broken links, coverage gaps, hygiene)

lint surfaces judgment-worthy curation findings, not spec-floor violations (use validate for those). It never mutates the bundle. By default it is advisory and exits 0 even with findings; pass --strict to exit non-zero on any finding.

A broken-link finding reports an internal .md link that resolves to no node when a node with the same basename exists elsewhere — a moved or mistyped path (a defect), distinct from a genuinely unwritten concept (a coverage gap, which analyze reports advisorily and lint stays quiet on).

--semantic adds similarity-driven checks (similar-but-unlinked pairs, nodes with no semantic neighbors) by reading the index built by 'okfctl-search index build'. Core only reads that index, so no embedding model is needed to lint.

```
okfctl lint [bundle-dir] [flags]
```

Example:

```
# Report curation findings for the current bundle (advisory, exit 0)
  okfctl lint

  # Fail CI on any finding
  okfctl lint --strict ./bundles/knowledge

  # Machine-readable findings for tooling
  okfctl lint --json ./bundles/knowledge

  # Also run similarity checks against a prebuilt semantic index
  okfctl lint --semantic ./bundles/knowledge
```

Flags:

```
      --coverage-threshold int       min distinct nodes that must mention a term to report a coverage gap (default 3)
      --isolation-floor float        score a node's best neighbor must reach to count as connected (default 0.20)
      --json                         emit findings as a machine-readable JSON array (sorted path, then check) instead of the human report
      --no-ignore                    walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default
      --semantic                     also run similarity checks against the index built by 'okfctl-search index build'
      --similarity-threshold float   cosine score at/above which two unlinked nodes are reported (default 0.80; implies --semantic data)
      --strict                       exit non-zero if there are any findings (default: advisory, exit 0)
```


## okfctl log

Manage the reserved log.md change history


### okfctl log append

Append a timestamped change entry to log.md

log append adds one timestamped entry to the reserved log.md change history (OKF §9: the log file is a reserved, append-only record of what changed in the bundle and when). The entry text is required via --message. It only appends — it never rewrites or reorders existing history. The node verbs append to log.md for you; use this for changes you made outside okfctl.

```
okfctl log append [dir] [flags]
```

Example:

```
# Record a manual change
  okfctl log append --message "reworded the revenue concept"

  # Record against a bundle elsewhere
  okfctl log append --message "imported Q3 sources" ./bundles/knowledge
```

Flags:

```
      --message string   the change entry text (required)
```


### okfctl log show

Print the change history

log show prints the reserved log.md change history (OKF §9) verbatim to stdout. It is read-only and does not mutate the bundle. Use it to review what changed and when, or to pipe the history into another tool.

```
okfctl log show [dir]
```

Example:

```
# Print the change history for the current bundle
  okfctl log show

  # Print the history for a bundle elsewhere
  okfctl log show ./bundles/knowledge
```


## okfctl migrate

Upgrade a bundle from OKF v0.1 to v0.2 (two-phase, consumer-agnostic)

migrate is the supported v0.1 -> v0.2 upgrade path. It runs in two phases so it never acquires a model dependency. Phase 1 (default) is PURE READ: it computes every deterministic §13.1 edit (timestamp -> generated.at; body # Citations with a follow-able resource -> frontmatter sources) and enumerates every judgment item (a prose citation with no resource per §5.1; a timestamp rename with no actor per §7), writing only the plan file. Phase 2 (--apply) reads the plan back and applies its deterministic edits order-preserving and additive-only, then re-validates. --dry-run writes nothing and is byte-identical to the real apply. Judgment items are never guessed — they stay in the plan for its consumer (agent, colleague, shell loop, or a human) to resolve.

```
okfctl migrate <bundle> [flags]
```

Example:

```
# Phase 1 (default): compute the plan (pure read), writing only the plan file
  okfctl migrate ./bundles/knowledge --plan migrate-plan.json --generated-by "casey"

  # Phase 2: preview the apply without writing (byte-identical to the real apply)
  okfctl migrate ./bundles/knowledge --apply --plan migrate-plan.json --dry-run

  # Phase 2: apply the plan's deterministic edits, then re-validate
  okfctl migrate ./bundles/knowledge --apply --plan migrate-plan.json
```

Flags:

```
      --apply                 phase 2: read the plan and apply it (default is phase 1: plan only)
      --dry-run               with --apply: write nothing; byte-identical to the real apply
      --generated-by string   actor (§7) recorded as generated.by for each timestamp rename
      --plan string           migration plan file (written in phase 1, read in phase 2) (default "migrate-plan.json")
```


## okfctl node

Author and inspect nodes


### okfctl node edit

Open a node in $EDITOR, then re-validate on return

edit opens a node in your editor ($OKFCTL_EDITOR, then $VISUAL, then $EDITOR, then vi) and, on return, re-validates the whole bundle against the spec floor (OKF §4.1 / §11). If validation fails, the findings are printed and the command exits non-zero. On success it refreshes the node's `modified` timestamp (`created` is never touched), appends to log.md, and regenerates index.md — this is how `modified` stays honest for the okfctl-mediated edit path. Reserved files (index.md, log.md) cannot be edited this way.

```
okfctl node edit <path> [flags]
```

Example:

```
# Edit a node, then re-validate on save
  okfctl node edit concepts/revenue

  # Use a specific editor for this edit
  OKFCTL_EDITOR="code --wait" okfctl node edit concepts/revenue
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
```


### okfctl node list

List nodes with their type (§7.3)

list prints every concept node in the bundle with its managed type, sorted by path (PRD §7.3: reads surface the type). Reserved files (index.md, log.md) are not nodes and are not listed. Read-only.

```
okfctl node list [flags]
```

Example:

```
# List every node in the current bundle
  okfctl node list

  # List nodes in a bundle elsewhere
  okfctl node list --bundle ./bundles/knowledge
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
```


### okfctl node mv

Move/rename a node, rewriting inbound links (path is identity)

mv moves or renames a node. A node's path is its identity (OKF §6: cross-links are by path), so mv also rewrites every inbound internal link to point at the new path, then maintains log.md and index.md. The .md suffix is optional on both arguments. --dry-run prints the move and every link rewrite and writes nothing.

```
okfctl node mv <old> <new> [flags]
```

Example:

```
# Rename a node and rewrite inbound links
  okfctl node mv concepts/revenue concepts/net-revenue

  # Preview the move and rewrites without touching disk
  okfctl node mv concepts/revenue concepts/net-revenue --dry-run
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
      --dry-run         print the plan without touching disk
```


### okfctl node new

Create a conformant node (type required, PRD §7)

new creates a conformant concept node at <path>. A non-empty --type is REQUIRED: type is the one managed field (PRD §7 — a node must carry a non-empty type; the value itself is open per PRD §7.4, so any string is accepted). The presence requirement is the spec floor (OKF §4.1 / §11: every frontmatter block carries a non-empty type). If a type template governs the given type, the node is scaffolded from it (PRD §9.3); otherwise a plain conformant node is written. Creation is recorded in log.md and index.md is regenerated, so a new node is never an audit gap. It does not open an editor — use `okfctl node edit` for that.

```
okfctl node new <path> [flags]
```

Example:

```
# Create a node of an open type
  okfctl node new concepts/revenue --type Concept --title Revenue

  # Create in a bundle elsewhere
  okfctl node new concepts/revenue --type Concept --bundle ./bundles/knowledge
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
      --title string    title for the new node (omitted from frontmatter when empty)
      --type string     type to assign the new node (required; any non-empty value, PRD §7.4)
```


### okfctl node promote

Promote directory-as-concept index.md files to sibling concept files (bulk remediation)

promote remediates the directory-as-concept shape validate reports: every non-root index.md that carries frontmatter is moved to a sibling concept file (dir/<basename>.md; basename defaults to the directory name, --name overrides it). The body is preserved verbatim and `created` is immutable, matching node refresh. Inbound links to the old directory-concept are rewritten (both the dir/index.md and dir/ spellings), the real index.md is regenerated with no frontmatter, and log.md is appended. The bundle-root index is left alone. --dry-run lists every move and rewrite and writes nothing.

```
okfctl node promote <bundle> [flags]
```

Example:

```
# Promote every directory-as-concept index in the bundle
  okfctl node promote ./bundles/knowledge

  # Preview the moves and inbound-link rewrites without writing
  okfctl node promote --dry-run ./bundles/knowledge

  # Use a fixed basename for every promoted concept file
  okfctl node promote --name overview ./bundles/knowledge
```

Flags:

```
      --dry-run       list what would change and exit 0 without writing
      --name string   basename for every promoted concept file (default: the directory name)
```


### okfctl node refresh

Rewrite stale `modified` timestamps to git last-commit (bulk drift fix)

refresh is the remediation for the git drift that validate reports: it rewrites each drifting node's frontmatter `modified` to its git last-commit day. `created` is immutable and never touched, the Markdown body is preserved verbatim, and log.md/index.md are maintained. With a trailing path it fixes a single node. --dry-run lists what would change and writes nothing. It degrades to a clean no-op outside a git repo, and exits non-zero only on real failure.

A plan dominated by a single commit — the signature of a bulk mechanical commit, whose remediation would collapse real authoring history into the migration date — is REFUSED unless --yes is given. The right fix in that case is to list the mechanical commit in .okf-drift-ignore-revs (like `git blame --ignore-revs-file`), so drift walks back to the prior real commit.

```
okfctl node refresh <bundle> [path] [flags]
```

Example:

```
# Fix every drifting node in the bundle
  okfctl node refresh ./bundles/knowledge

  # Preview the plan without writing anything
  okfctl node refresh --dry-run ./bundles/knowledge

  # Fix a single node
  okfctl node refresh ./bundles/knowledge concepts/income-statement.md
```

Flags:

```
      --dry-run   list what would change and exit 0 without writing
      --yes       refresh even when a single commit dominates the plan (overrides the bulk-commit guard)
```


### okfctl node rm

Remove a node and report resulting orphans

rm deletes a node and reports which nodes are orphaned as a result (left with no inbound links, OKF §6), then maintains log.md and index.md. The .md suffix is optional. It does NOT rewrite links that pointed at the removed node — those become broken links that `okfctl lint` will report, by design. --dry-run reports the plan and writes nothing.

```
okfctl node rm <path> [flags]
```

Example:

```
# Remove a node and see resulting orphans
  okfctl node rm concepts/deprecated

  # Preview the removal without touching disk
  okfctl node rm concepts/deprecated --dry-run
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
      --dry-run         print the plan without touching disk
```


### okfctl node show

Print a node, surfacing its type (§7.3)

show prints a single node's path, its type, and its Markdown body (PRD §7.3: reads surface the managed type). The .md suffix is optional in <path>. Read-only — it never mutates the bundle. It errors if no node matches.

```
okfctl node show <path> [flags]
```

Example:

```
# Show a node (the .md suffix is optional)
  okfctl node show concepts/revenue

  # Show a node in a bundle elsewhere
  okfctl node show concepts/revenue --bundle ./bundles/knowledge
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
```


## okfctl plugin

Discover and manage okfctl plugins (okfctl-<name> on PATH)


### okfctl plugin install

Install an okfctl-<name> plugin executable into the managed plugins dir

Install copies the okfctl-<name> executable at <source> into the okfctl plugins dir (default $OKFCTL_CONFIG_HOME/plugins or <user config dir>/okfctl/plugins, override with --dir). Put that dir on your PATH so `okfctl plugin list` and subcommand dispatch discover the installed plugin.

```
okfctl plugin install <source> [flags]
```

Example:

```
# Install a plugin executable into the managed plugins dir
  okfctl plugin install ./okfctl-search

  # Install into a specific directory
  okfctl plugin install --dir ~/bin ./okfctl-search
```

Flags:

```
      --dir string   directory to install into (defaults to the managed plugins dir)
```


### okfctl plugin list

List okfctl-<name> plugin executables found on PATH

plugin list discovers okfctl-<name> executables on your PATH and prints each plugin's name and resolved path. These are the subcommands okfctl will dispatch to git/kubectl-style when you run `okfctl <name>`. Read-only. Scan a specific PATH with --path; the default is $PATH.

```
okfctl plugin list [flags]
```

Example:

```
# List discovered plugins on $PATH
  okfctl plugin list

  # Scan a specific PATH
  okfctl plugin list --path ~/bin
```

Flags:

```
      --path string   PATH to scan for plugins (defaults to $PATH)
```


## okfctl registry

Manage named remote bundle sources (git URLs)

Manage a local, named directory of remote OKF bundle sources.
Each source is a plain git URL — this is `git remote` for bundles, not a hosted registry, account system, or schema registry.


### okfctl registry add

Register (or re-point) a named remote bundle source

add registers a named remote bundle source (a plain git URL) in okfctl's config store, or re-points an existing name at a new URL. The name must be a safe identifier (letters, digits, and -_. ). This only records the mapping — it does not clone anything; use `okfctl connect <name>` to materialize it.

```
okfctl registry add <name> <git-url>
```

Example:

```
# Register a remote bundle source
  okfctl registry add knowledge https://github.com/acme/knowledge-base.git
```


### okfctl registry list

List registered remote bundle sources

list prints every registered remote bundle source and its git URL, sorted by name. Read-only. A registry with no sources prints a notice and exits zero.

```
okfctl registry list
```

Example:

```
# List every registered remote source
  okfctl registry list
```


### okfctl registry remove

Unregister a remote bundle source

remove deletes a registered remote source from okfctl's config, or exits non-zero if the name is not registered. It only forgets the mapping — it never touches any local checkout you already cloned with `okfctl connect`.

```
okfctl registry remove <name>
```

Example:

```
# Unregister a remote source
  okfctl registry remove knowledge
```


### okfctl registry show

Print a remote source's git URL

show prints the git URL registered for a single remote name, or exits non-zero if the name is not registered. Read-only.

```
okfctl registry show <name>
```

Example:

```
# Print one remote's git URL
  okfctl registry show knowledge
```


## okfctl search

Search the bundle lexically or by graph neighborhood (core, stdlib-only)

search queries the bundle from the CLI without a model or index.

Lexical mode (default): okfctl search "query" [dir] matches concept nodes by
title, tag, type, or body substring (case-insensitive). Restrict the surface
with --field title|tag|type|body.

Graph-structural mode: okfctl search --neighbors <node-path> [dir] returns the
nodes within --depth hops of a node in the link graph (edges are undirected).

Semantic (vector) search is the separate okfctl-search plugin: run
`okfctl-search --semantic "query"` (PRD §8).

```
okfctl search [query] [bundle-dir] [flags]
```

Example:

```
# Lexical search across all fields in the current bundle
  okfctl search "income statement"

  # Restrict the match surface to titles, in a bundle elsewhere
  okfctl search --field title revenue ./bundles/knowledge

  # Graph mode: nodes within 2 hops of a node in the link graph
  okfctl search --neighbors concepts/income-statement.md --depth 2 ./bundles/knowledge
```

Flags:

```
      --depth int          neighborhood traversal depth (hops) (default 1)
      --field string       lexical match surface: any|title|tag|type|body (default "any")
      --json               emit results as JSON
      --neighbors string   graph mode: node path to traverse from
      --no-ignore          walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default
```


## okfctl serve

Serve an interactive web visualization of the bundle graph

serve starts a local web server that renders the bundle as an interactive, navigable knowledge graph. Assets are embedded in the binary — no separate install. Binds loopback by default; override with --addr.

```
okfctl serve [bundle-dir] [flags]
```

Example:

```
# Serve the current bundle on http://127.0.0.1:8080
  okfctl serve

  # Serve a bundle elsewhere on a custom address
  okfctl serve --addr 127.0.0.1:9000 ./bundles/knowledge
```

Flags:

```
      --addr string   address to bind (loopback by default) (default "127.0.0.1:8080")
```


## okfctl template

Read the type-template bundle (templates are authored as ordinary OKF nodes)


### okfctl template list

List the type templates a bundle declares (target type, required fields, body sections)

template list shows the type templates a bundle declares. Templates are okfctl's opt-in team overlay (PRD §9): they are authored as ordinary OKF nodes whose type is `Type Template`, NOT a spec concept, and they never affect the spec floor. Each row names a target type and how many required fields and body sections its template defines. Read-only. A bundle with no templates prints a notice and exits zero.

```
okfctl template list [bundle-dir]
```

Example:

```
# List templates declared by the current bundle
  okfctl template list

  # List templates in a bundle elsewhere
  okfctl template list ./bundles/knowledge
```


### okfctl template show

Show a single type template's required/recommended fields and body sections

template show prints one type template in full: its target type, source node, and its required fields, recommended fields, and body sections. Templates are okfctl's opt-in team overlay (PRD §9), authored as ordinary OKF nodes — this command only reads them and never mutates the bundle. It errors if no template governs the given type. See what `okfctl validate --templates` will enforce and what `node new` will scaffold.

```
okfctl template show <target-type> [bundle-dir]
```

Example:

```
# Show the template that governs the "Runbook" type
  okfctl template show Runbook

  # Show a template in a bundle elsewhere
  okfctl template show Runbook ./bundles/knowledge
```


## okfctl validate

Check a bundle for OKF spec-floor conformance (optionally overlay team type-templates)

validate enforces the OKF spec floor (type present + non-empty, §7). It also reports git drift: a node whose frontmatter `modified` contradicts its git last-commit date (read-only — it never rewrites the file, and degrades to nothing outside a git repo). With --templates it additionally runs the opt-in team overlay (§9.4), reporting template drift. All drift is advisory by default (exit 0); pass --strict to exit non-zero on any drift. Floor violations always fail regardless of --strict.

```
okfctl validate [bundle-dir] [flags]
```

Example:

```
# Check the spec floor for the bundle in the current directory
  okfctl validate

  # Check a bundle elsewhere
  okfctl validate ./bundles/knowledge

  # Also run the opt-in team template overlay, failing CI on any drift
  okfctl validate --templates --strict ./bundles/knowledge
```

Flags:

```
      --no-ignore   walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default
      --strict      exit non-zero on any drift (git drift and, with --templates, template drift); default: advisory, exit 0
      --templates   also run the opt-in type-template overlay (§9.4), reporting drift as warnings
```


## okfctl version

Print the okfctl version

version prints the okfctl build metadata: the release version, git commit, and build date. On a plain `go build` with no release ldflags these degrade to "dev". It is equivalent to `okfctl --version`.

```
okfctl version
```

Example:

```
# Print the build version
  okfctl version
```
