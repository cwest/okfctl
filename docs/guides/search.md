# Search

okfctl has two ways to find things in a bundle. `okfctl search` comes with
okfctl and matches words and links. The `okfctl-search` plugin ranks nodes by
meaning, with a model you put on your own disk. Neither one needs a network.
This page tries each of them on the same three-node bundle.

## Find nodes by word or by link

Make the bundle. Oak links to tannin, and tannin links to acidity:

```text
$ okfctl bundle init mykb
Initialized OKF bundle in mykb
$ okfctl node new wine/tannin.md --type Reference --title "Tannin" --bundle mykb
Created mykb/wine/tannin.md
$ okfctl node new wine/oak.md --type Reference --title "Oak" --bundle mykb
Created mykb/wine/oak.md
$ okfctl node new wine/acidity.md --type Reference --title "Acidity" --bundle mykb
Created mykb/wine/acidity.md
$ echo 'Oak barrels add [tannin](tannin.md) to red wine.' >> mykb/wine/oak.md
$ echo 'Tannin is the drying grip in red wine. It balances against [acidity](acidity.md).' >> mykb/wine/tannin.md
$ echo 'Acidity makes a wine taste fresh.' >> mykb/wine/acidity.md
```

Search for a word:

```text
$ okfctl search tannin mykb
wine/oak.md                              Reference    [body]
wine/tannin.md                           Reference    [body,title]
$ okfctl search --field title tannin mykb
wine/tannin.md                           Reference    [title]
$ okfctl search merlot mykb
```

Each line shows a node's path, its type, and where the word turned up. The match
ignores case. It looks at the title, tags, type, and body unless `--field`
narrows it to one of them (`title`, `tag`, `type`, `body`). The search for
`merlot` printed nothing and exited 0. Finding nothing isn't an error.

Then look at what sits near the oak node in the link graph:

```text
$ okfctl search --neighbors wine/oak.md mykb
wine/tannin.md                           Reference    depth=1
$ okfctl search --neighbors wine/oak.md --depth 2 mykb
wine/tannin.md                           Reference    depth=1
wine/acidity.md                          Reference    depth=2
$ okfctl search --neighbors wine/acidity.md mykb
wine/tannin.md                           Reference    depth=1
```

`--depth` is the number of hops, starting at 1. The last query shows a link
working in either direction. Acidity links to nothing, but tannin links to it,
and that makes tannin its neighbor.

`index.md` and `log.md` never show up as results. Add `--json` to get the
results as an array in a stable order you can diff in CI, and run
`okfctl search --help` for all the flags.

## Rank nodes by meaning with `okfctl-search`

The core command can't tell you that "astringent" and "drying grip" describe
the same thing. `okfctl-search` can. It's a second binary, shipped in the same
release archive as `okfctl` (see [Install](../install.md)). Call it by its own
name. okfctl's own commands always win over plugins with the same name, and
`okfctl search --semantic` gets you the core command. It fails with
`unknown flag: --semantic`.

Build an index, then query it:

```text
$ okfctl-search index build mykb
indexed 3 node(s) with hash-test-embedder (dim 64) -> mykb/.okfctl/index.db
$ okfctl-search --semantic "drying grip" mykb
0.3086	wine/tannin.md
	# Tannin Tannin is the drying grip in red wine. It balances against [acidity](acidity.md).
0.0000	wine/acidity.md
	# Acidity Acidity makes a wine taste fresh.
0.0000	wine/oak.md
	# Oak Oak barrels add [tannin](tannin.md) to red wine.
$ okfctl-search related wine/oak.md mykb
0.1936	wine/acidity.md
0.1091	wine/tannin.md
```

That run used the default embedder, `hash`. It needs no model and its vectors
come out identical each time, but it matches tokens and knows nothing about
meaning. Tannin scored because its body contains "drying grip". The other two
scored zero. Use `hash` to try the commands while you're still getting a model.

`index build` embedded each node into `.okfctl/index.db` and recorded which
embedder made it, with its dimension. On a rebuild it skips any node whose
title and body haven't changed.

`--semantic` ranked the nodes by cosine similarity to the query. It prints the
top `--k` results (5 by default), each with a snippet.

`related` listed the oak node's nearest neighbors, leaving out oak itself.
`okfctl lint --semantic` reads the same neighbors for its similarity checks
(PRD §8.6). The [curation health guide](curation-health.md) explains them.

## Use a real model

`--embedder model2vec` loads a [Model2Vec](https://github.com/MinishLab/model2vec)
static embedding model, for example
[`minishlab/potion-base-8M`](https://huggingface.co/minishlab/potion-base-8M).
The tokenizer and the inference math are written in Go and compiled into
`okfctl-search`. The model's files are all you add. You don't need Python, and
there's no service to start. The test suite checks the vectors against the
upstream `model2vec` library's own output, to within `1e-5`, on that model.

okfctl never downloads a model. Put one on disk yourself, tell okfctl where it
is, and rebuild the index with it:

<!-- transcript: skip needs a local copy of potion-base-8M, and the test downloads nothing -->
```text
$ okfctl config set model_path ./models/potion-base-8M
$ okfctl-search --embedder model2vec index build mykb
indexed 3 node(s) with potion-base-8M (dim 256) -> mykb/.okfctl/index.db
$ okfctl-search --embedder model2vec --semantic "astringent mouthfeel" mykb
0.3676	wine/tannin.md
	# Tannin Tannin is the drying grip in red wine. It balances against [acidity](acidity.md).
0.2766	wine/acidity.md
	# Acidity Acidity makes a wine taste fresh.
0.1921	wine/oak.md
	# Oak Oak barrels add [tannin](tannin.md) to red wine.
$ okfctl-search --semantic "astringent mouthfeel" mykb
okfctl-search: index model does not match the active embedder; rebuild with 'okfctl-search index build'
```

Neither "astringent" nor "mouthfeel" appears anywhere in the bundle, and tannin
still comes out on top. The last command left off `--embedder`. It ran as
`hash` against an index that `model2vec` made, refused, and exited 1. An index
answers only to the embedder that made it. Switch embedders and you rebuild.

`model_path` lives in okfctl's config, and `--model-path <dir>` overrides it for
one run. The directory needs the usual Model2Vec files: `config.json`,
`model.safetensors`, and `tokenizer.json` (or `vocab.txt`).

With no path set, `--embedder model2vec` exits 1 and tells you how to set one.
It won't fall back to `hash`. An answer from the wrong embedder is worse than
no answer.

## Narrow and reorder the results

These flags work with `--semantic`:

- `--path`, `--type`, and `--tag` limit results to nodes under a path prefix,
  of a type, with a tag. Repeat any of them and the values OR together.
  `--not-path`, `--not-type`, and `--not-tag` exclude matching nodes.
- `--half-life <days>` lowers the scores of older nodes. `--min-relevance` drops
  weak matches ahead of any aging. `--decay-floor` (0.25 by default) limits how
  far age can pull a score down.
- `--lexical-gate` helps with queries shaped like an identifier, where the
  model blurs a rare token. It does the semantic query wide, holds on to the
  results that contain a query term (in semantic order), and appends the word
  matches the semantic results missed. Literal matches can't fall off the list
  that way. Stopwords don't count as terms, and inflections match their stem
  (`hash` and `hashes` find the same nodes).

The gate switches itself off in two cases, and the results then match a run
with no gate byte for byte. One is a query with no content words, like
`"how should the"`. The other is a term found in more than 60% of the bundle.
A word that common can't narrow anything (`agent` matches 73% of the
reference corpus). It honors `--path`/`--type`/`--tag` and `--half-life`.

`okfctl-search --help` lists the rest.
