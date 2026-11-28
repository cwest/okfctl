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

Create a bundle or summarize one


### okfctl bundle info

Show a bundle's node count and declared OKF version

info prints three lines about the bundle in dir (the current directory by default): how many concept nodes it has, how many reserved files (index.md and log.md at each level, OKF §3.1), and the okf_version it declares (§12). In the example wine/ gets an index.md of its own, so one node in a subdirectory makes three reserved files.

The version comes from the frontmatter of the bundle-root index.md (§8.1). When that has none, info reads the older .okf sidecar file. If neither declares one, info prints undeclared. okfctl then applies the v0.2 fallbacks §12.1 allows.

info only reads. To find out if the bundle conforms, run `okfctl validate`.

```
okfctl bundle info [dir]
```

Example:

```
$ okfctl bundle init cellar
Initialized OKF bundle in cellar
$ okfctl node new wine/tannin --type Concept --bundle cellar
Created cellar/wine/tannin.md
$ okfctl bundle info cellar
nodes: 1
reserved: 3
okf_version: 0.3 (declared in index.md)
```


### okfctl bundle init

Create a bundle with index.md and log.md, declared OKF v0.3

init creates a bundle in dir (the current directory if you leave dir off) and makes the directory when it's missing. It writes the two reserved files (OKF §3.1). The root index.md declares okf_version "0.3" in its frontmatter (§8.1, §12). log.md starts with a placeholder line.

There are no concept nodes at first. Add them with `okfctl node new`; index.md and log.md change with each node you add.

init never checks for a bundle that's there. Run it on one and it exits 0 after overwriting index.md and log.md. The root frontmatter (a title or description you added) and the change history are gone, but none of your nodes are touched. If you aren't sure a directory is empty, run `okfctl bundle info` on it first.

```
okfctl bundle init [dir]
```

Example:

```
$ okfctl bundle init newkb
Initialized OKF bundle in newkb
$ ls newkb
index.md
log.md
$ cat newkb/index.md
---
okf_version: "0.3"
---

# Knowledge Base

_Progressive-disclosure entry point. Add "start here" links as nodes land._
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

Rebuild or check the generated index.md files


### okfctl index build

Rewrite every index.md to list the bundle's current nodes

build writes an index.md into the bundle root and into each directory under it that has concept nodes in it (OKF §8). Each lists its directory's nodes by title (plus the description where a node has one) and then links to the subdirectories. In the example a node moved with mv(1) leaves the index stale: check exits 1 until build fixes it.

You'll seldom need it.

Six node commands (new, mv, rm, edit, refresh, promote) rebuild the index when they finish. build is for the times you add, move, or delete node files yourself.

Each subdirectory's link ends with a short summary (concept count, types, the tags common to most of its nodes, a subtree total for deeper trees) that tells you what a branch holds. --shape-tag-min and --shape-tag-max tune the tag list. --no-shape drops the summary.

Only the root index.md's frontmatter survives a rebuild (that frontmatter is where the bundle's okf_version, title, and description live, §8.1). Everything below it in each index.md okfctl wrote is rewritten from scratch, and an index.md okfctl wrote in a directory with no nodes left is deleted.

An index.md with anything okfctl doesn't write (an intro paragraph, notes, your own heading, frontmatter on a subdirectory index) is left alone: build names it on stderr and exits 1. Move the prose into a node, or rerun with --overwrite-hand-written to replace the file (the root frontmatter and H1 still survive). A subdirectory index.md with frontmatter is a concept stored where §8 puts an index (validate fails it); `okfctl node promote` moves it out of the way first.

```
okfctl index build [dir] [flags]
```

Example:

<!-- transcript: exit 1 okfctl index check atlas -->
```
$ okfctl bundle init atlas
Initialized OKF bundle in atlas
$ okfctl node new tannin --type Concept --title Tannin --bundle atlas
Created atlas/tannin.md
$ mkdir atlas/wine && mv atlas/tannin.md atlas/wine/
$ okfctl index check atlas
index.md is out of date; run `okfctl index build` to regenerate
okfctl: index.md is out of date
$ okfctl index build atlas
Wrote 2 index.md file(s) under atlas
$ cat atlas/index.md
---
okf_version: "0.3"
---

# Knowledge Base

## Subdirectories

* [Wine](wine/) (1 concept · Concept)
```

Flags:

```
      --no-ignore                walk EVERY directory, including vendored/derived ones (node_modules, venv, dist, ...) that are skipped by default; dot directories are never part of the bundle (OKF v0.3 §3.2)
      --no-shape                 omit the subdirectory shape suffix (concept count, types, shared tags, subtree total)
      --overwrite-hand-written   also rewrite (or remove) index.md files okfctl did not generate; the bundle-root frontmatter and H1 are still kept
      --shape-tag-max int        cap the shared-tag list at this many tags (most-shared-first, ellipsis on truncation) (default 12)
      --shape-tag-min int        a shared tag prints only when at least this many of a directory's own concepts carry it (default 2)
```


### okfctl index check

Exit 1 if any index.md is out of date, without writing

check renders each index.md the way `okfctl index build` would and compares the result with what's on disk (OKF §8). It writes nothing. When every file matches it prints "OK: index.md is current" and exits 0. Otherwise it names the first file that's missing, stale, or left in a directory with no nodes, then exits 1. In the example the node written with printf hasn't reached the index.

Put check in CI. lint --strict won't flag a stale index.

Pass the --shape-tag-min, --shape-tag-max, and --no-shape flags you built with. Build with --no-shape and check with the default flags: any index that links a subdirectory reads as stale.

check doesn't fix anything. build does. When the file at fault is one okfctl didn't write, the report says so: build leaves that file alone, so fix it by hand or rebuild with --overwrite-hand-written.

```
okfctl index check [dir] [flags]
```

Example:

<!-- transcript: exit 1 okfctl index check pantry -->
```
$ okfctl bundle init pantry
Initialized OKF bundle in pantry
$ printf -- '---\ntype: Concept\ntitle: Tannin\n---\n' > pantry/tannin.md
$ okfctl index check pantry
index.md is out of date; run `okfctl index build` to regenerate
okfctl: index.md is out of date
$ okfctl index build pantry
Wrote 1 index.md file(s) under pantry
$ cd pantry && okfctl index check
OK: index.md is current
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

Add to or print the bundle's log.md change history


### okfctl log append

Add a line to today's entry in log.md

append adds --message as one line under today's date in the log.md at the root of dir (the current directory by default), and writes today's date heading first if the log doesn't have one, because dates run newest first (OKF §9). No log.md? append creates it. The example prints <DATE> where your run prints today's date.

You need it only for changes made outside okfctl. Six node commands (new, mv, rm, edit, refresh, promote) write their own line.

Whatever sits above the first date heading, your title included, is left alone. The placeholder line `okfctl bundle init` wrote goes away, and older okfctl bullets with their date inline move under a date heading of their own. Only the first line of a message gets in. Leave off --message and append exits 1.

```
okfctl log append [dir] [flags]
```

Example:

```
$ okfctl bundle init journal
Initialized OKF bundle in journal
$ okfctl log append --message 'split the oak node into barrel and cask' journal
Appended log entry
$ okfctl log show journal
# Change Log

## <DATE>
* split the oak node into barrel and cask
```

Flags:

```
      --message string   the change entry text (required)
```


### okfctl log show

Print log.md as it is on disk

show prints the root log.md of dir (the current directory by default) byte for byte, to read or to pipe into another tool. In the example two nodes get created and one gets moved. All three land under one date heading, newest first (OKF §9).

show never writes. It won't reformat the file or check it against §9; `okfctl validate` does that. A log.md in a subdirectory is ignored. With no log.md at all, show prints nothing and exits 0.

```
okfctl log show [dir]
```

Example:

```
$ okfctl bundle init diary
Initialized OKF bundle in diary
$ okfctl node new wine/tannin --type Concept --bundle diary
Created diary/wine/tannin.md
$ okfctl node new wine/oak --type Concept --bundle diary
Created diary/wine/oak.md
$ okfctl node mv wine/oak barrel/oak --bundle diary
Moved wine/oak.md -> barrel/oak.md (0 inbound link(s) rewritten)
$ okfctl log show diary
# Change Log

## <DATE>
* moved wine/oak.md -> barrel/oak.md
* created wine/oak.md
* created wine/tannin.md
```


## okfctl migrate

Upgrade a v0.1 or v0.2 bundle to OKF v0.3: plan, preview, apply

migrate moves a bundle that declares OKF v0.1 or v0.2 (or none) to v0.3 in place. Run it twice. The first run writes a plan file and changes nothing else. The next one, with --apply, makes those edits and declares okf_version "0.3" in the bundle-root index.md (§8.1). Then it validates the result. With --dry-run, apply prints the diff it would write and writes nothing.

A rule makes each edit. For a v0.1 bundle (Appendix B), timestamp turns into generated.at. Any body # Citations entry that names one follow-able resource is copied to the sources list, titled with the cited work's name when the entry marks one (never the full entry text, §5.1). An entry with no URL but one DOI, arXiv id, PubMed id, or ISBN (checksum verified) gets a resource built from it, offline: doi.org, arxiv.org, pubmed.ncbi.nlm.nih.gov, or openlibrary.org. Nothing is fetched to check it. Each converted entry's source gets an id (its plain key, else its list number, else cite-<n>; unique with case folded), the entry's full text becomes the footnote definition [^id], and body [k] references become [^id] (§5.1, §4.2). A source title that holds its whole entry is cut to the work's name, or removed when the entry names none, once that footnote holds the text. A title that opens with the entry's [key] but was annotated past the entry's text has that text added to the footnote first, then is cut the same way. An entry that names two or more resources (code spans aside) is split into one source per resource, with ids <id>, <id>-2, <id>-3; the first holds the full entry text as its footnote, a later one its link text or resource, and each body [k] becomes [^id][^id-2]... A resource that two entries name stays one source: the first entry that names it alone owns it (else the first that splits), and every other entry's [k] cites that source's id too; an entry splits into the parts it owns, and its first owned part holds its text. Once every entry's text opens the footnote definition of one of its sources (alone, or followed by a moved title annotation) and every resource it names is a source's resource, the body # Citations list is removed (§4.2); otherwise it stays. The v0.2 changes (§13.1) give a leading / to any link or path field that resolves only from the bundle root (§12.1). It then points at the same file under v0.3 (§6). They also respell a §5 timestamp that has an offset but isn't in RFC 3339 form.

Judgment items are the cases no rule can settle. prose-citation marks a citation with no resource, or whose every resource another entry owns, so it names no work of its own (§5.1). missing-actor marks a timestamp rename with no actor (§7). timestamp-no-offset is a date-time with no offset (§5, §13.2), and timestamp-invalid is a §5 value where no date can be found. source-title-unnamed marks an entry whose untitled source gets no name, source-title-conflict one that names a different work than its source's title, and source-title-unpreserved a whole-entry title kept because no footnote holds its text. citation-ref-unclear marks a body [k] near the word "brief", "citation", or "node", or a link, in the same paragraph, list item, or table row, or the label of an undefined [text][k]; citation-unattached a second entry for one resource, and source-id-collision a node whose ids would collide. citations-kept marks a # Citations list that removing would lose something from, and names each entry and what it would lose. All of them wait for you to decide them. While a prose-citation or missing-actor item is left, apply holds the version: those items need the v0.1 fallbacks §12.1 grants only to bundles declaring 0.2 or older (or nothing). --force-declare on the first run overrides the hold.

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

Create, move, remove, edit, and read concept nodes


### okfctl node edit

Open a node in your editor, then validate and log the change

edit opens the node at <path> in your editor and waits for it to close. The editor is $OKFCTL_EDITOR, then $VISUAL, then $EDITOR (vi if none is set). The .md suffix is optional. In the example the editor is true: it exits right away.

After the editor closes, edit validates the whole bundle (OKF §11.1) and, if that passes with the node's type still set, sets modified to now, adds "edited <path>" to log.md, and rebuilds each index.md. created stays as it was. A node changed in your editor without okfctl keeps its old modified date. validate reports that later as git drift.

A failed check prints the findings and exits 1. So does an emptied type (the file is no longer a concept, §3.2). Either way your change stays on disk and nothing else is touched. Fix it and run edit again.

The variable's whole value is taken as one program name. OKFCTL_EDITOR="code --wait" fails. Put the arguments in a short script and name the script there.

edit won't open index.md or log.md.

```
okfctl node edit <path> [flags]
```

Example:

```
$ okfctl bundle init desk
Initialized OKF bundle in desk
$ okfctl node new tannin --type Concept --bundle desk
Created desk/tannin.md
$ OKFCTL_EDITOR=true okfctl node edit tannin --bundle desk
tannin.md edited; bundle valid
$ okfctl log show desk
# Change Log

## <DATE>
* edited tannin.md
* created tannin.md
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
```


### okfctl node list

List every concept node with its type, sorted by path

list prints one line per concept node in the bundle given by --bundle (the current directory by default), sorted by path, with the node's type in the second column (PRD §7.3). Use it to see what a bundle holds, or to find nodes of one type with grep.

index.md and log.md never appear (OKF §3.1). Neither does a .md file with no type, such as a README.md. The spec doesn't count those as concepts (§3.2), and `okfctl validate` warns about them. list only reads.

```
okfctl node list [flags]
```

Example:

```
$ okfctl bundle init catalog
Initialized OKF bundle in catalog
$ okfctl node new wine/tannin --type Concept --bundle catalog
Created catalog/wine/tannin.md
$ okfctl node new grape/sangiovese --type 'Grape Variety' --bundle catalog
Created catalog/grape/sangiovese.md
$ echo 'Notes for editors.' > catalog/README.md
$ okfctl node list --bundle catalog
grape/sangiovese.md                      Grape Variety
wine/tannin.md                           Concept
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
```


### okfctl node mv

Move or rename a node and rewrite every link that points at it

mv takes the node at <old> to <new> in the bundle --bundle names (the current directory by default, with the .md suffix optional on either path). Links find their target by path (OKF §6). So mv rewrites each link in the bundle that pointed at the old path, README files included: one that started with / still starts with / while a relative one gets worked out again from its own file. Then mv adds "moved <old> -> <new>" to log.md and rebuilds each index.md.

Try --dry-run first: it prints the move and each rewrite with nothing written.

The links in the moved node stay as written. Relative ones that worked from the old directory can point at nothing from the new one. `okfctl lint` reports them as broken-link.

With a file at <new>, mv exits 1 and changes nothing. It won't move index.md or log.md, nor put a node at either name.

```
okfctl node mv <old> <new> [flags]
```

Example:

```
$ okfctl bundle init map
Initialized OKF bundle in map
$ okfctl node new wine/tannin --type Concept --title Tannin --bundle map
Created map/wine/tannin.md
$ okfctl node new wine/oak --type Concept --title Oak --bundle map
Created map/wine/oak.md
$ echo 'Oak barrels add [tannin](tannin.md).' >> map/wine/oak.md
$ okfctl node mv wine/tannin wine/tannins --dry-run --bundle map
move wine/tannin.md -> wine/tannins.md
  rewrite wine/oak.md: tannin.md -> tannins.md
$ okfctl node mv wine/tannin wine/tannins --bundle map
Moved wine/tannin.md -> wine/tannins.md (1 inbound link(s) rewritten)
$ tail -n 1 map/wine/oak.md
Oak barrels add [tannin](tannins.md).
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
      --dry-run         print the plan without touching disk
```


### okfctl node new

Create a concept node, then update log.md and index.md

new writes a concept node at <path> in the bundle --bundle names (the current directory by default). The .md suffix is optional. You have to pass --type (a file with no type isn't a concept, OKF §4.1, §3.2). Any value works. The spec leaves the list of types open. The node gets that type and created and modified timestamps, plus the --title if you gave one. With no title, the heading is the file name.

Then new adds "created <path>" to log.md and rebuilds each index.md, and that's why `okfctl index check` passes right after. If the --bundle directory is missing, new makes it and declares no okf_version.

With a type template for --type in the bundle, the node starts from that template's fields and body headings (PRD §9.3). Templates live in the bundle itself or in another bundle named by the .okf templates key or --templates-from (PRD §9.2). Run `okfctl template list` to see them.

Overwriting a node, writing outside the bundle, and a path that starts with a dot all exit 1.

To edit the node in your editor, use `okfctl node edit`.

```
okfctl node new <path> [flags]
```

Example:

<!-- transcript: exit 1 okfctl node new tannin --bundle shelf -->
```
$ okfctl bundle init shelf
Initialized OKF bundle in shelf
$ okfctl node new tannin --bundle shelf
okfctl: --type is required (OKF §4.1 / §11: every node needs a non-empty type)
$ okfctl node new tannin --type Concept --title Tannin --bundle shelf
Created shelf/tannin.md
$ cat shelf/tannin.md
---
type: Concept
title: Tannin
created: <TIMESTAMP>
modified: <TIMESTAMP>
---

# Tannin
$ okfctl index check shelf
OK: index.md is current
```

Flags:

```
      --bundle string           bundle directory to operate on (default ".")
      --templates-from string   resolve type templates from this bundle directory, overriding the .okf templates key (PRD §9.2)
      --title string            title for the new node (omitted from frontmatter when empty)
      --type string             type to assign the new node (required; any non-empty value, OKF §4.1)
```


### okfctl node promote

Turn each subdirectory index.md that holds a concept into its own node

promote fixes a concept written as a subdirectory's index.md, which validate fails. OKF §8 says an index file there has no frontmatter. promote moves each one to a node beside it named after the directory, so grape/index.md turns into grape/grape.md. --name picks another name. The body and created carry over as they were. Links to the old place get rewritten in both spellings, grape/index.md and grape/. One log.md line goes in per move. Then the index is rebuilt and grape/index.md is a generated list again.

--dry-run prints each move and rewrite with nothing written.

Run promote first. A rebuild deletes a subdirectory index.md, frontmatter and all, when its directory holds no other node. index build rebuilds, and so does each node command that changes the bundle. The root index.md isn't touched; §8.1 lets it hold bundle metadata. A taken target name makes promote exit 1 with nothing changed.

```
okfctl node promote <bundle> [flags]
```

Example:

<!-- transcript: exit 1 okfctl validate handbook -->
```
$ okfctl bundle init handbook
Initialized OKF bundle in handbook
$ mkdir handbook/grape
$ printf -- '---\ntype: Concept\ntitle: Grape\n---\n\nThe fruit.\n' > handbook/grape/index.md
$ printf -- '---\ntype: Concept\ntitle: Tannin\n---\n\nTannin comes from [grape](grape/) skins.\n' > handbook/tannin.md
$ okfctl validate handbook
FAIL grape/index.md: index files contain no frontmatter (§8); only the bundle-root index may carry frontmatter (§8.1)
okfctl: 1 conformance finding(s)
$ okfctl node promote --dry-run handbook
would promote grape/index.md -> grape/grape.md
  rewrite tannin.md: grape/ -> grape/grape.md
1 index(es) would be promoted (dry run; nothing written)
$ okfctl node promote handbook
promoted grape/index.md -> grape/grape.md (1 inbound link(s) rewritten)
1 index(es) promoted
$ tail -n 1 handbook/tannin.md
Tannin comes from [grape](grape/grape.md) skins.
$ cd handbook && okfctl validate .
OK: bundle conforms to the OKF spec floor
```

Flags:

```
      --dry-run       list what would change and exit 0 without writing
      --name string   basename for every promoted concept file (default: the directory name)
```


### okfctl node refresh

Set modified to each node's last git commit date

refresh fixes the git drift `okfctl validate` warns about: a node whose modified timestamp disagrees with the day git last committed it. Each drifting node gets that commit day as its modified date and a "refreshed <path>" line in log.md. The index is rebuilt once at the end. created and the body don't change. Put a path after the bundle to fix one node.

The example commits a node created today with a January date. validate warns. --dry-run lists the change and writes nothing. After refresh makes it, validate passes. Outside a git repository nothing drifts, and refresh exits 0 having done nothing.

A bulk commit (a migration, a reformat) is the case to watch. Refreshing from it would stamp its date on every node it touched and wipe out their real history. So refresh stops when one commit accounts for half or more of a plan of 10 or more changes: it exits 1 and prints the commit's SHA. Add that SHA to .okf-drift-ignore-revs in the bundle root (it works like git blame --ignore-revs-file) and drift looks past the commit to the one before. --yes refreshes anyway.

```
okfctl node refresh <bundle> [path] [flags]
```

Example:

```
$ okfctl bundle init ledger
Initialized OKF bundle in ledger
$ okfctl node new tannin --type Concept --bundle ledger
Created ledger/tannin.md
$ git -C ledger init -q && git -C ledger add .
$ GIT_COMMITTER_DATE=2026-01-15T12:00:00Z git -C ledger -c user.name=me -c user.email=me@example.com commit -q -m import
$ okfctl validate ledger
warning tannin.md: modified <DATE> disagrees with git last-commit 2026-01-15; run `okfctl node refresh` to fix it (or `okfctl node edit` per node)
1 drift warning(s)
$ okfctl node refresh --dry-run ledger
would refresh tannin.md: <DATE> -> 2026-01-15
1 node(s) would be refreshed (dry run; nothing written)
$ okfctl node refresh ledger
refreshed tannin.md: <DATE> -> 2026-01-15
1 node(s) refreshed
$ okfctl validate ledger
OK: bundle conforms to the OKF spec floor
```

Flags:

```
      --dry-run   list what would change and exit 0 without writing
      --yes       refresh even when a single commit dominates the plan (overrides the bulk-commit guard)
```


### okfctl node rm

Delete a node and list the nodes it leaves with no inbound links

rm deletes the node at <path> from the bundle --bundle names (the current directory by default). The .md suffix is optional. Then it adds "removed <path>" to log.md, rebuilds each index.md, and prints the orphans: nodes the deleted one linked to that nothing else links to now (OKF §6.4). In the example oak was tannin's only link.

Run it with --dry-run first to print the same list and leave the node in place.

Links that pointed at the deleted node stay as they were. The spec allows a broken link (§6.1, §11.2), and afterward validate and lint pass. Only `okfctl analyze` mentions them, as dangling forward-links. To find them yourself, grep the bundle for the old path first.

rm won't delete index.md or log.md (§3.1); it exits 1.

```
okfctl node rm <path> [flags]
```

Example:

```
$ okfctl bundle init rack
Initialized OKF bundle in rack
$ okfctl node new tannin --type Concept --bundle rack
Created rack/tannin.md
$ okfctl node new oak --type Concept --bundle rack
Created rack/oak.md
$ echo 'Oak barrels add [tannin](tannin.md).' >> rack/oak.md
$ okfctl node rm oak --dry-run --bundle rack
remove oak.md
  orphaned: tannin.md
$ okfctl node rm oak --bundle rack
Removed oak.md
  orphaned: tannin.md
```

Flags:

```
      --bundle string   bundle directory to operate on (default ".")
      --dry-run         print the plan without touching disk
```


### okfctl node show

Print one node's path, type, and body

show prints a node's path and type, then its Markdown body, from the bundle --bundle names (the current directory by default). The .md suffix is optional. The type gets a line of its own: you see what kind of concept you're reading before the body starts (PRD §7.3). The rest of the frontmatter (title, timestamps, sources) stays hidden. cat the file for that.

show never writes. Any path that isn't a concept node exits 1 with "node not found", index.md and log.md included. The spec says neither is ever a concept (§3.1).

```
okfctl node show <path> [flags]
```

Example:

<!-- transcript: exit 1 okfctl node show index --bundle reader -->
```
$ okfctl bundle init reader
Initialized OKF bundle in reader
$ okfctl node new wine/oak --type Concept --title Oak --bundle reader
Created reader/wine/oak.md
$ echo 'Oak barrels add tannin.' >> reader/wine/oak.md
$ okfctl node show wine/oak --bundle reader
path: wine/oak.md
type: Concept

# Oak
Oak barrels add tannin.

$ okfctl node show index --bundle reader
okfctl: node not found: index.md
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
