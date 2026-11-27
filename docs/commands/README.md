# Command reference

**This page is generated from the command tree. Don't edit it by hand.**
Regenerate it with `go generate ./cmd` (or `go run ./cmd/gendocs`); a
CI check (`TestCommandReference_NoDrift`) fails when the committed file drifts
from the binary, so the reference can't go stale.

`okfctl <cmd> --help` is the authoritative, always-current form for any
command — it prints the same description, flags, and runnable example straight
from the binary. This page mirrors it in one place, one section per
command, so it's browsable and linkable (README links to the `#okfctl-<cmd>`
anchors below).

Run `okfctl help` for the top-level list, or `okfctl <cmd> --help` for any
entry below.

## okfctl analyze

Report where a bundle is thin, stale, or poorly connected

analyze shows where a bundle is thin or stale or poorly connected, to help you pick what to write or check next. It prints a section for each question with the nodes that answer it, then a count of actionable signals.

Coverage lists links to nodes no one has written, plus nodes shorter than --thin-lines lines and nodes with zero citations or one. Freshness lists nodes past their stale_after (§5.5) and nodes not checked in --aging-days days, plus time-sensitive nodes that have aged. Connectivity lists orphans and nodes with one link. Clusters lists any tag on --cluster-min or more nodes with no overview node. Structure lists titles and file names close enough to be one node. It also tallies the epistemic grades and status values it finds, for you to read.

It never edits a file. It exits 0 whenever the report completes, however many signals it finds. It has no --strict flag; lint --strict is the gate for CI.

--json prints the same report for a script.

```
okfctl analyze [bundle-dir] [flags]
```

Example:

```
# Report on the bundle in the current directory
  okfctl analyze

  # Report on a bundle elsewhere
  okfctl analyze ./mykb

  # The same report as JSON, for a script
  okfctl analyze --json ./mykb
```

Flags:

```
      --aging-days int                  age (days) past which a node's review basis (verified/generated/modified/created) is listed as aging (default 180); advisory only, staleness comes from stale_after
      --cluster-min int                 min nodes sharing a tag to flag a synthesis cluster (default 3)
      --coverage-threshold int          min distinct nodes mentioning a term to report a coverage gap (default 3)
      --json                            emit machine-readable JSON instead of the human report
      --no-ignore                       walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
      --thin-lines int                  body line count below which a node is thin (default 15)
      --time-sensitive-fraction float   flag a time-sensitive node once its age >= fraction*aging-days (default 0.5); undated marked nodes are always flagged
```


## okfctl bundle

Bundle lifecycle commands


### okfctl bundle info

Summarize a bundle (node count, spec version)

info prints a one-glance summary of a bundle: how many concept nodes it holds, how many reserved files (index.md/log.md, OKF §3.1) it carries, and its okf_version (§12). The version is read from the bundle-root index.md frontmatter (§8.1), falling back to the legacy .okf sidecar. A bundle that declares neither is reported as undeclared: per §12.1 it is read with the v0.2 fallbacks. It's read-only and never mutates the bundle. It doesn't validate conformance — use `okfctl validate` for that.

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

init scaffolds a minimal bundle that conforms to the OKF spec floor: the two reserved files (index.md and log.md, OKF §3.1). The bundle-root index.md declares okf_version "0.3" in its frontmatter (§8.1, §12). It doesn't create any concept nodes — a fresh bundle has zero nodes; use `okfctl node new` to add them. It refuses to overwrite an existing bundle.

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

completion prints a bash, zsh, or fish script to stdout. Load it in your shell (or save it where your shell looks for completions) and Tab will fill in okfctl's commands and flags.

Give it one of those three shell names. Anything else exits 1. completion only prints the script. Where you save it is up to you.

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

Read and write okfctl's own settings

config reads and writes config.json, okfctl's settings file. Set $OKFCTL_CONFIG_HOME to choose its directory; without it okfctl uses the okfctl directory under your OS's user config directory. Plugins read the file too (okfctl-search looks up model_path there). Names you save with `okfctl registry` go in it as keys that start with `registry.`. No bundle content lands in it.


### okfctl config get

Get a config value

get prints the value saved under one key. If the key isn't set, get exits 1. For all of them, run `okfctl config list`.

```
okfctl config get <key>
```

Example:

```
# Print one config value
  okfctl config get model_path
```


### okfctl config list

List all config values

list prints each saved key as key = value, sorted by key. Keys that start with `registry.` show up here too. `okfctl registry list` shows just those.

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

set saves a value under a key and prints nothing. Setting a key again replaces its value. okfctl doesn't check key names. Misspell one and you've made a new key.

```
okfctl config set <key> <value>
```

Example:

```
# Point okfctl-search at a local model2vec model
  okfctl config set model_path ~/models/potion-base-8M
```


## okfctl connect

Clone a remote bundle, or fast-forward the copy you have

connect gets you a local copy of a remote bundle over git. Name the source with a name you saved through `okfctl registry add`, or give it a git URL (a local path counts). With no directory argument it uses the last part of the URL minus .git, the name `git clone` would pick.

A missing or empty directory gets a fresh clone. A checkout of the same source gets `git pull --ff-only`. If your copy and the remote have both gained commits, that pull fails and connect exits 1 with git's message. Nothing gets merged or rebased.

connect won't touch any other directory with files in it. It exits 1 there. It also exits 1 on a word that's neither a saved name nor a URL, and tells you how to register it.

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


## okfctl eval

Check how far you can trust a bundle's nodes

eval checks how far you can trust a bundle's nodes, using the four TACA dimensions (Transparency, Accuracy, Calibration, Alignment). okfctl can check only the first one by itself.

eval transparency is the gate. It looks for a grade and citations on each node and makes sure citations to other nodes in the bundle resolve. Put it in CI.

eval sample is for the other three. It picks nodes and writes a worksheet with what okfctl could read from each one, leaving the judgment blank for you to fill in, by hand or with a model you run yourself. okfctl never judges a claim true or false. It would have to read the source or call a model to do it.


### okfctl eval sample

Write a worksheet for the checks okfctl can't run: accuracy, alignment, calibration

sample picks a few nodes and writes a worksheet for the three checks okfctl can't do by itself. Accuracy: do the cited sources back the claims? Alignment: does the node answer the question its title and description set up? Calibration: does its grade survive a second look?

okfctl copies in everything it can read from each node (title, grades, trust tier, sources, candidate claims) and leaves the answer slots empty. You fill them in by hand or with a model.

On a pull request, use --changed-since <ref> to take the nodes changed since that git ref. Otherwise --sample N takes N at random. Run it again with the same --seed to get the same nodes back. With neither flag you get nothing. JSON is the default. Use --format md for a worksheet people will read.

```
okfctl eval sample [bundle-dir] [flags]
```

Example:

```
# A worksheet for 5 random nodes
  okfctl eval sample --sample 5 --format md ./mykb

  # JSON for the nodes changed on this branch
  okfctl eval sample --changed-since origin/main ./mykb
```

Flags:

```
      --changed-since string   sample only nodes changed since this git ref (e.g. origin/main)
      --format string          output format: json (machine eval-set) or md (human worksheet) (default "json")
      --no-ignore              walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
      --sample int             size of a deterministic random sample (ignored when --changed-since is set)
      --seed int               seed for reproducible --sample selection (default: a fixed seed)
```


### okfctl eval transparency

Check that each node has a grade and citations, and that internal citations resolve

transparency checks that each node shows where it came from. It prints one line per finding, then a count. It exits 0 by default. With --strict it exits 1 on any finding.

It never edits a file.

It has four checks:

    grade-missing        the node has neither an epistemic nor an authority grade
    grade-vocabulary     a grade value fewer than --grade-vocabulary-floor nodes
                         use (likely a typo)
    uncited              no sources, no # Citations section, and no link to
                         another node
    citation-unresolved  a sources entry names a .md path in the bundle that
                         doesn't exist

It doesn't fetch http(s) citations. Fetching needs the network. Leave those to eval sample and to yourself.

```
okfctl eval transparency [bundle-dir] [flags]
```

Example:

```
# Report findings (exits 0)
  okfctl eval transparency ./mykb

  # Fail CI on any finding
  okfctl eval transparency --strict ./mykb

  # Print findings as JSON
  okfctl eval transparency --json ./mykb
```

Flags:

```
      --grade-vocabulary-floor int   min nodes carrying a grade value for it to count as corpus vocabulary (default 2)
      --json                         emit findings as a machine-readable JSON array (sorted path, then check)
      --no-ignore                    walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
      --strict                       exit non-zero if there are any findings (default: advisory, exit 0)
```


## okfctl graph

Export the bundle's link graph


### okfctl graph export

Print the link graph as JSON or Graphviz DOT

graph export prints the bundle's link graph for another tool to draw or check. Each node is a concept with its path, title, type, and top-level directory. Orphans are flagged. Each edge is a link or path field from one concept to another (§6.4).

JSON is the default. --format dot writes Graphviz DOT. Pipe that to dot -Tsvg for a picture. It never edits a file.

```
okfctl graph export [bundle-dir] [flags]
```

Example:

```
# Print the graph of the bundle in the current directory as JSON
  okfctl graph export

  # Print a bundle elsewhere as Graphviz DOT
  okfctl graph export --format dot ./mykb

  # Draw it as an SVG with Graphviz
  okfctl graph export --format dot ./mykb | dot -Tsvg > graph.svg
```

Flags:

```
      --format string   output format: json or dot (default "json")
      --no-ignore       walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
```


## okfctl index

Manage the reserved index.md


### okfctl index build

Regenerate index.md from the current bundle

index build regenerates the reserved index.md navigation file(s) from the bundle's current concept nodes (OKF §8: index files are reserved, generated navigation files). It rewrites index.md at the bundle root and in each directory that has one; it never edits concept nodes. Run it after adding, moving, or removing nodes by hand (the node verbs regenerate it for you).

Each subdirectory entry carries a shape suffix — the child's own concept count, its types, its shared tags, and (when it nests deeper) a subtree total — so a reader can decide which branch to open without opening it. Tune it with --shape-tag-min / --shape-tag-max, or drop it with --no-shape.

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
      --no-ignore           walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
      --no-shape            omit the subdirectory shape suffix (concept count, types, shared tags, subtree total)
      --shape-tag-max int   cap the shared-tag list at this many tags (most-shared-first, ellipsis on truncation) (default 12)
      --shape-tag-min int   a shared tag prints only when at least this many of a directory's own concepts carry it (default 2)
```


### okfctl index check

Verify index.md is current (nonzero exit if stale)

index check verifies the reserved index.md (OKF §8) is in sync with the bundle's current nodes, without writing anything. It's the CI-friendly counterpart to `index build`: it exits zero when the index is current and non-zero (printing what drifted) when a rebuild is needed. Read-only — it never rewrites the index.

Pass the SAME shape flags you built with (--shape-tag-min / --shape-tag-max / --no-shape); a check against a different shape configuration than the build reports drift by design.

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
      --no-ignore           walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
      --no-shape            omit the subdirectory shape suffix (concept count, types, shared tags, subtree total)
      --shape-tag-max int   cap the shared-tag list at this many tags (most-shared-first, ellipsis on truncation) (default 12)
      --shape-tag-min int   a shared tag prints only when at least this many of a directory's own concepts carry it (default 2)
```


## okfctl lint

Report what a curator should look at: orphans, unlinked mentions, broken links, hygiene

lint reports what a careful curator would want a second look at. You can pass validate with a node no other node links to, or with a node that names another and doesn't link it. lint prints one line per finding, then a count. It exits 0 by default. Run lint --strict in CI and it exits 1 on any finding.

It never edits a file. None of its findings makes a bundle non-conformant under §11.1. These are its checks:

    orphan                   nothing links to the node, not index.md or log.md either
    missing-xref             the body names another node's title but doesn't link it
    broken-link              a link or path field finds no node, but one with that
                             file name exists elsewhere
    coverage-gap             an alias some node declares, mentioned by at least
                             --coverage-threshold nodes, with no node of its own
    type-hygiene             type values that differ only by case or a plural s
    tag-hygiene              tags that fold to one spelling (case, separators, or a
                             plural of a tag already in use)
    status-lifecycle         a status §5.4 doesn't define, so the node isn't stable
    spec-version             a per-node okf_spec_version key (the version belongs
                             on the bundle-root index.md, §8.1)
    source-missing-resource  a sources entry with no resource (§5.1)
    timestamp-form           a §5 timestamp that's neither an RFC 3339 date-time
                             with an offset nor a plain date
    verified-event           a verified event missing by or at (§5.2)
    superseded-by            a superseded_by that finds no concept, or sits on a
                             node that isn't deprecated

A link to a file that exists nowhere isn't a finding. The spec treats it as knowledge no one has written (§6.1). analyze lists it under coverage. The two hygiene checks group spellings that look alike, but two tags can look alike and still mean different things. Read the finding first and rename later. An unparseable stale_after gets a timestamp-form finding and never counts as fresh (§5.5).

--semantic adds two checks that read the index okfctl-search index build writes. similar-unlinked flags two nodes that read alike with no link between them. no-semantic-neighbors flags a node with nothing close to it. Nodes added since the last build aren't in the index. lint can't check them and names them in one stale-index finding. lint loads no model to do this. It reads the index. With no index on disk, lint exits 1 and says to build one.

```
okfctl lint [bundle-dir] [flags]
```

Example:

```
# Report findings for the bundle in the current directory (exits 0)
  okfctl lint

  # Fail CI on any finding
  okfctl lint --strict ./mykb

  # Print findings as JSON for another tool
  okfctl lint --json ./mykb

  # Add the similarity checks (needs okfctl-search index build ./mykb first)
  okfctl lint --semantic ./mykb
```

Flags:

```
      --coverage-threshold int       min distinct nodes that must mention a term to report a coverage gap (default 3)
      --isolation-floor float        score a node's best neighbor must reach to count as connected (default 0.20)
      --json                         emit findings as a machine-readable JSON array (sorted path, then check) instead of the human report
      --no-ignore                    walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
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

log show prints the reserved log.md change history (OKF §9) verbatim to stdout. It's read-only and doesn't mutate the bundle. Use it to review what changed and when, or to pipe the history into another tool.

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

Upgrade a v0.1 or v0.2 bundle to OKF v0.3: plan, preview, apply

migrate moves a bundle that declares OKF v0.1 or v0.2 (or none) to v0.3 in place. Run it twice. The first run writes a plan file and changes nothing else. The next one, with --apply, makes those edits and declares okf_version "0.3" in the bundle-root index.md (§8.1). Then it validates the result. With --dry-run, apply prints the diff it would write and writes nothing.

A rule makes each edit. For a v0.1 bundle (Appendix B), timestamp turns into generated.at. Any body # Citations entry that names one follow-able resource is copied to the sources list, titled with the cited work's name when the entry marks one (never the full entry text, §5.1). An entry with no URL but one DOI, arXiv id, PubMed id, or ISBN (checksum verified) gets a resource built from it, offline: doi.org, arxiv.org, pubmed.ncbi.nlm.nih.gov, or openlibrary.org. Nothing is fetched to check it. The v0.2 changes (§13.1) give a leading / to any link or path field that resolves only from the bundle root (§12.1). It then points at the same file under v0.3 (§6). They also respell a §5 timestamp that has an offset but isn't in RFC 3339 form.

Judgment items are the cases no rule can settle. prose-citation marks a citation with no resource or with two (§5.1). missing-actor marks a timestamp rename with no actor (§7). timestamp-no-offset is a date-time with no offset (§5, §13.2), and timestamp-invalid is a §5 value where no date can be found. source-title-unnamed marks an entry whose untitled source gets no name, and source-title-conflict one that names a different work than its source's title. All of them wait for you to decide them. While a prose-citation or missing-actor item is left, apply holds the version: those items need the v0.1 fallbacks §12.1 grants only to bundles declaring 0.2 or older (or nothing). --force-declare on the first run overrides the hold.

migrate never refuses a bundle on v0.3, on a later version, or on one okfctl doesn't know. It never rewrites one to an older version (§12). No step needs a model.

```
okfctl migrate <bundle> [flags]
```

Example:

```
# Write the plan; nothing in the bundle changes
  okfctl migrate ./mykb --plan migrate-plan.json --generated-by human:casey

  # See the diff the apply would write, without writing it
  okfctl migrate ./mykb --apply --plan migrate-plan.json --dry-run

  # Apply the plan, declare v0.3 if nothing is held, and validate
  okfctl migrate ./mykb --apply --plan migrate-plan.json
```

Flags:

```
      --apply                 phase 2: read the plan and apply it (default is phase 1: plan only)
      --dry-run               with --apply: write nothing; print the diff the real apply would write
      --force-declare         phase 1: declare v0.3 even while prose-citation or missing-actor items remain (they lose the §12.1 fallback)
      --generated-by string   actor (§7) recorded as generated.by for each timestamp rename
      --plan string           migration plan file (written in phase 1, read in phase 2) (default "migrate-plan.json")
```


## okfctl node

Author and inspect nodes


### okfctl node edit

Open a node in $EDITOR, then re-validate on return

edit opens a node in your editor ($OKFCTL_EDITOR, then $VISUAL, then $EDITOR, then vi) and, on return, re-validates the whole bundle against the spec floor (OKF §4.1 / §11). If validation fails, the findings are printed and the command exits non-zero. On success it refreshes the node's `modified` timestamp (`created` is never touched), appends to log.md, and regenerates index.md — this is how `modified` stays honest for the okfctl-mediated edit path. Reserved files (index.md, log.md) can't be edited this way.

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

List nodes with their type (PRD §7.3)

list prints every concept node in the bundle with its managed type, sorted by path (PRD §7.3: reads reveal the type). Reserved files (index.md, log.md) aren't nodes and aren't listed. Read-only.

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

Create a conformant node (type required, OKF §4.1)

new creates a conformant concept node at <path>. A non-empty --type is REQUIRED: type is the one managed field (a node must carry a non-empty type; the value itself is open per OKF §4.1, so any string is accepted). The presence requirement is the spec floor (OKF §4.1 / §11: every frontmatter block carries a non-empty type). If a type template governs the given type, the node is scaffolded from it (PRD §9.3); the template may be local or referenced from a separate bundle (the .okf `templates:` key, or --templates-from which overrides it, PRD §9.2). Otherwise a plain conformant node is written. Creation is recorded in log.md and index.md is regenerated, so a new node is never an audit gap. It doesn't open an editor — use `okfctl node edit` for that.

```
okfctl node new <path> [flags]
```

Example:

```
# Create a node of an open type
  okfctl node new concepts/revenue --type Concept --title Revenue

  # Create in a bundle elsewhere
  okfctl node new concepts/revenue --type Concept --bundle ./bundles/knowledge

  # Scaffold from a referenced template bundle, overriding the .okf reference
  okfctl node new runbooks/deploy --type Playbook --templates-from ../okf-type-templates
```

Flags:

```
      --bundle string           bundle directory to operate on (default ".")
      --templates-from string   resolve type templates from this bundle directory, overriding the .okf templates key (PRD §9.2)
      --title string            title for the new node (omitted from frontmatter when empty)
      --type string             type to assign the new node (required; any non-empty value, OKF §4.1)
```


### okfctl node promote

Promote directory-as-concept index.md files to sibling concept files (bulk remediation)

promote remediates the directory-as-concept pattern validate reports: every non-root index.md that carries frontmatter is moved to a sibling concept file (dir/<basename>.md; basename defaults to the directory name, --name overrides it). The body is preserved verbatim and `created` is immutable, matching node refresh. Inbound links to the old directory-concept are rewritten (both the dir/index.md and dir/ spellings), the real index.md is regenerated with no frontmatter, and log.md is appended. The bundle-root index is left alone. --dry-run lists every move and rewrite and writes nothing.

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

rm deletes a node and reports which nodes are orphaned as a result (left with no inbound links, OKF §6), then maintains log.md and index.md. The .md suffix is optional. It doesn't rewrite links that pointed at the removed node — those become broken links that `okfctl lint` will report, by design. --dry-run reports the plan and writes nothing.

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

Print a node, surfacing its type (PRD §7.3)

show prints a single node's path, its type, and its Markdown body (PRD §7.3: reads reveal the managed type). The .md suffix is optional in <path>. Read-only — it never mutates the bundle. It errors if no node matches.

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

Find and install okfctl-<name> plugins

A plugin is any executable named okfctl-<name> on your PATH. When okfctl has no command called <name>, `okfctl <name>` runs okfctl-<name> with the rest of your arguments and exits with its status. git and kubectl do the same. okfctl's own commands always beat a plugin with the same name.


### okfctl plugin install

Copy an okfctl-<name> executable into okfctl's plugins directory

plugin install copies an executable named okfctl-<name> into okfctl's plugins directory: $OKFCTL_CONFIG_HOME/plugins if that's set, otherwise okfctl/plugins under your OS's user config directory. --dir picks another directory. The copy gets mode 755 and replaces any plugin with the same name there.

A source that isn't named okfctl-<name> exits 1. install doesn't touch your PATH. If the directory isn't on it, install prints a note. Add the directory yourself so `okfctl <name>` and `okfctl plugin list` can find the plugin.

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

plugin list prints each okfctl-<name> executable it finds on your PATH, with its path. When two directories hold the same name, the first one on PATH is the one `okfctl <name>` runs. --path scans a different list of directories.

list only reads. When it finds no plugins it says so on stderr and exits 0.

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

registry names your remote bundle sources. Each one is a plain git URL. The name lets you type `okfctl connect knowledge` in place of the URL. It works like `git remote`. There's no hosted service or account behind it. The names live in okfctl's config file.


### okfctl registry add

Save a name for a remote bundle's git URL

add saves a name for a git URL in okfctl's config. Saving a name that exists points it at the new URL. add then prints Updated in place of Added. Names use letters, digits, hyphens, underscores, and dots. Anything else exits 1.

add clones nothing. Run `okfctl connect <name>` to get a local copy.

```
okfctl registry add <name> <git-url>
```

Example:

```
# Register a remote bundle source
  okfctl registry add knowledge https://github.com/acme/knowledge-base.git
```


### okfctl registry list

List saved remote names and their git URLs

list prints each saved name and its git URL, one per line, sorted by name. With none saved it prints a notice and exits 0. It only reads.

```
okfctl registry list
```

Example:

```
# List every registered remote source
  okfctl registry list
```


### okfctl registry remove

Forget a saved remote name

remove (or rm) deletes a saved name from okfctl's config. If the name isn't saved, remove exits 1. It forgets the name and nothing more. Any copy you made with `okfctl connect` stays where it is.

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

show prints the git URL saved under one name. If nothing is saved under it, show exits 1. It only reads.

```
okfctl registry show <name>
```

Example:

```
# Print one remote's git URL
  okfctl registry show knowledge
```


## okfctl search

Find nodes by text or by link distance, with no model or index

search finds the nodes that contain your query in one of four fields (title, tags, type, body) and prints each with its path, type, and matching fields. Case doesn't matter. It needs no model or index. --field narrows it to one of those fields. No match prints nothing and exits 0.

--neighbors <node-path> asks a different question: which nodes sit within --depth links of that node, following links either way.

It never edits a file. To search by meaning, install the okfctl-search plugin and run `okfctl-search --semantic "query"` (PRD §8).

```
okfctl search [query] [bundle-dir] [flags]
```

Example:

```
# Search every field in the bundle in the current directory
  okfctl search "income statement"

  # Match titles only, in a bundle elsewhere
  okfctl search --field title revenue ./mykb

  # Nodes within 2 links of one node
  okfctl search --neighbors concepts/income-statement.md --depth 2 ./mykb
```

Flags:

```
      --depth int          neighborhood traversal depth (hops) (default 1)
      --field string       lexical match fields: any|title|tag|type|body (default "any")
      --json               emit results as JSON
      --neighbors string   graph mode: node path to traverse from
      --no-ignore          walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
```


## okfctl serve

Draw the bundle's link graph in your browser

serve runs a web server on 127.0.0.1:8080 that draws the bundle's link graph in your browser, where you can click a node to see its type and links and filter the graph by type or by top-level directory. Dashed nodes are orphans. The page is part of the binary.

serve loads the bundle at startup and never edits it. Restart it to pick up a change. It listens only on your own machine unless you pass --addr.

```
okfctl serve [bundle-dir] [flags]
```

Example:

```
# Serve the bundle in the current directory on http://127.0.0.1:8080
  okfctl serve

  # Serve a bundle elsewhere on another port
  okfctl serve --addr 127.0.0.1:9000 ./mykb
```

Flags:

```
      --addr string   address to bind (loopback by default) (default "127.0.0.1:8080")
```


## okfctl template

Inspect the type templates a bundle uses

A type template is a node, typed `Type Template`, that says which fields and body sections a node of some type should have. Only two other commands read templates. `okfctl node new --type` builds the new node from the one for that type, and `okfctl validate --templates` warns when a node drifts. Try `okfctl template list` first to see what your bundle has.


### okfctl template list

List each type template's target type, field and section counts, and source

template list prints one line for each type template the bundle uses: the type it governs, how many fields and body sections it asks for, and its source file. With no templates it prints a notice and exits 0.

A template can sit in the bundle itself or in a separate template bundle named by `templates: <path>` in the bundle's .okf file. --templates-from points at a different one for this run. If the bundle and the referenced one both define a type, the local template wins and its line ends with [shadows referenced template].

list only reads. Templates are okfctl's own convention (PRD §9) and not part of the OKF spec. They never change what plain validate accepts. If the referenced template bundle can't be read, list exits 1 and names the path.

```
okfctl template list [bundle-dir] [flags]
```

Example:

```
# List templates declared by the current bundle
  okfctl template list

  # List templates in a bundle elsewhere
  okfctl template list ./bundles/knowledge

  # Resolve templates from a specific bundle, overriding the .okf reference
  okfctl template list --templates-from ../okf-type-templates ./bundles/knowledge
```

Flags:

```
      --templates-from string   resolve type templates from this bundle directory, overriding the .okf templates key (PRD §9.2)
```


### okfctl template show

Print one type template's fields and body sections

template show prints the template that governs one type: its source file, its required and recommended fields, and its body sections. `okfctl node new --type` scaffolds new nodes from it. `okfctl validate --templates` warns if a node drifts from it.

It finds templates the way template list does, so your own `Type Template` node beats one from the bundle named by the .okf `templates:` key or by --templates-from. If no template governs the type, show exits 1. It never changes the bundle.

```
okfctl template show <target-type> [bundle-dir] [flags]
```

Example:

```
# Show the template that governs the "Runbook" type
  okfctl template show Runbook

  # Show a template in a bundle elsewhere
  okfctl template show Runbook ./bundles/knowledge

  # Resolve from a specific template bundle, overriding the .okf reference
  okfctl template show Runbook --templates-from ../okf-type-templates ./bundles/knowledge
```

Flags:

```
      --templates-from string   resolve type templates from this bundle directory, overriding the .okf templates key (PRD §9.2)
```


## okfctl validate

Check a bundle against the OKF spec floor, and optionally your team's type templates

validate checks a bundle against the floor every OKF bundle has to meet (§11.1). Every frontmatter block has to parse as a YAML mapping. Each reserved index.md and log.md has to follow §8 and §9. A floor failure prints FAIL and always exits 1.

It never edits a file. A .md file with no frontmatter or no type isn't a concept (OKF v0.3 §3.2). validate warns about it and the run still passes. It also warns about drift: a node whose modified date disagrees with its last git commit (outside a git repo it's silent), or an okf_version in the legacy .okf sidecar that disagrees with the one in index.md (§8.1, §12).

Drift is advice: the run exits 0 unless you pass --strict.

--templates checks each node against your team's type template (PRD §9.4) and reports where a node strays as more drift. The templates can live in another bundle, named by the .okf templates: key or by --templates-from (PRD §9.2). An unknown type or an extra key never fails the floor (§11.2).

```
okfctl validate [bundle-dir] [flags]
```

Example:

```
# Check the bundle in the current directory
  okfctl validate

  # Check a bundle elsewhere
  okfctl validate ./mykb

  # Also check type templates, and fail CI on any drift
  okfctl validate --templates --strict ./mykb

  # Take the templates from another bundle instead of the .okf reference
  okfctl validate --templates --templates-from ../okf-type-templates ./mykb
```

Flags:

```
      --no-ignore               walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
      --strict                  exit non-zero on any drift (git drift and, with --templates, template drift); default: advisory, exit 0
      --templates               also run the opt-in type-template overlay (PRD §9.4), reporting drift as warnings
      --templates-from string   resolve type templates from this bundle directory, overriding the .okf templates key (PRD §9.2)
```


## okfctl version

Print the okfctl version

version prints the okfctl version, the git commit it came from, and the build date: the same line as `okfctl --version`. Release builds fill in all three. If you built from source or used `go install`, you'll see the module version Go recorded (dev when there isn't one) with commit none and built unknown.

```
okfctl version
```

Example:

```
# Print the build version
  okfctl version
```
