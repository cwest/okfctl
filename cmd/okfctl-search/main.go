// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command okfctl-search is a PATH-dispatch plugin (git/kubectl style) adding
// offline semantic search over an OKF bundle. Invoked as `okfctl search ...` via
// core's dispatch, or directly. It is a SEPARATE static binary from okfctl core,
// which is the whole point of the plugin model: the semantic weight rides the
// plugin, the core stays dependency-free (PRD §8.4).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cwest/okfctl/internal/clock"
	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/okfconfig"
	"github.com/cwest/okfctl/internal/search"
	"github.com/spf13/cobra"
)

func main() {
	if err := newSearchCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "okfctl-search:", err)
		os.Exit(1)
	}
}

// resolveEmbedder returns the embedder named by --embedder. "hash" is the
// zero-config offline default; "model2vec" loads a real static model from a
// local directory resolved flag-first, then config (`okfctl config set
// model_path`). okfctl never downloads a model at runtime, so an unset path is
// a clear, actionable error rather than a silent fallback to hash — a query
// answered by the wrong embedder is worse than one that refuses to run.
func resolveEmbedder(name, modelPath string) (search.Embedder, error) {
	switch name {
	case "hash":
		return search.NewHashEmbedder(), nil
	case "model2vec":
		dir := modelPath
		if dir == "" {
			cfg, err := okfconfig.Load()
			if err != nil {
				return nil, fmt.Errorf("reading okfctl config: %w", err)
			}
			dir = cfg["model_path"]
		}
		if dir == "" {
			return nil, fmt.Errorf("--embedder model2vec needs a local model directory: run `okfctl config set model_path <dir>` or pass --model-path <dir>")
		}
		e, err := search.LoadModel2VecEmbedder(dir)
		if err != nil {
			return nil, fmt.Errorf("loading model2vec model from %s: %w", dir, err)
		}
		return e, nil
	default:
		return nil, fmt.Errorf("unknown embedder %q (available: hash, model2vec)", name)
	}
}

func indexPath(bundleDir string) string {
	return filepath.Join(bundleDir, ".okfctl", "index.db")
}

func newSearchCmd() *cobra.Command {
	var (
		embedderName string
		modelPath    string
		semantic     string
		k            int
		pathPrefix   []string
		typeFilter   []string
		tagFilter    []string
		notPath      []string
		notType      []string
		notTag       []string
		halfLife     float64
		decayFloor   float64
		minRelevance float64
		lexicalGate  bool
	)

	root := &cobra.Command{
		Use:           "okfctl-search",
		Short:         "Offline semantic search over an OKF bundle (okfctl plugin)",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		// Honour SOURCE_DATE_EPOCH here too, so decayed ranking is reproducible
		// under a pinned clock — the same front door okfctl core uses. This is a
		// separate static binary (PATH-dispatch plugin), so it resolves the clock
		// itself rather than inheriting core's process-wide install. A malformed
		// value is a hard error before any query runs.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			fn, err := clock.Resolve()
			if err != nil {
				return err
			}
			clock.Install(fn)
			return nil
		},
		// Root with --semantic runs a query; without it, prints help.
		RunE: func(cmd *cobra.Command, args []string) error {
			if semantic == "" {
				return cmd.Help()
			}
			// Validate the recency-decay bounds at parse time BEFORE building
			// DecayOptions (#71). The library math.Max(0.5^x, DecayFloor) makes a
			// floor > 1 win for every node — a flat GAIN on raw cosine, not the
			// "lower clamp" the help text promises — and pushes scores outside the
			// [-1, 1] cosine range the rest of the ranking assumes. A negative floor
			// silently re-enables the #65 inversion f4c9824 fixed. A negative
			// half-life is accepted silently today, byte-identical to no decay,
			// while the HTTP surface (internal/apiserver/search.go) already rejects
			// it with 400. Reject both here so the two surfaces agree; reusing the
			// apiserver's half-life wording verbatim. --decay-floor is validated
			// first, so when both are bad its error is the deterministic one.
			if decayFloor < 0 || decayFloor > 1 {
				return fmt.Errorf("--decay-floor must be in [0, 1], got %v", decayFloor)
			}
			if halfLife < 0 {
				return fmt.Errorf("--half-life must be a non-negative number of days, got %v", halfLife)
			}
			e, err := resolveEmbedder(embedderName, modelPath)
			if err != nil {
				return err
			}
			dir := bundleArg(args)
			s, err := search.Load(indexPath(dir))
			if err != nil {
				return fmt.Errorf("no index at %s (run 'okfctl-search index build' first): %w", indexPath(dir), err)
			}

			opts := search.QueryOptions{
				Filter: search.Filter{
					PathPrefixes:    nonEmpty(pathPrefix),
					Types:           nonEmpty(typeFilter),
					Tags:            nonEmpty(tagFilter),
					NotPathPrefixes: nonEmpty(notPath),
					NotTypes:        nonEmpty(notType),
					NotTags:         nonEmpty(notTag),
				},
			}
			// Filters (§4.1 type/tag), recency decay (§5.2/§13.1), AND the lexical
			// gate (#66) all resolve against the LIVE bundle at query time, not off
			// the index: contentHash keys only on title+body, so a frontmatter-only
			// edit (type, tag, generated.at) does not re-embed and a value
			// denormalized onto the index would go stale. The lexical gate needs the
			// live title+body prose to match terms against, which the index does not
			// carry at all.
			needBundle := !opts.Filter.IsEmpty() || halfLife > 0 || lexicalGate
			var b *okf.Bundle
			if needBundle {
				b, err = okf.Load(dir)
				if err != nil {
					return err
				}
				opts.Meta = buildNodeMeta(b)
			}
			if halfLife > 0 || minRelevance > 0 {
				// Recency decay is post-ranking and off by default. Two independent
				// bounds keep it honest: MinRelevance is a floor on RAW cosine (a
				// sub-floor node is dropped before decay can reorder anything), and
				// DecayFloor clamps the recency multiplier itself so an old-but-relevant
				// node can be demoted but never crushed to zero below a mediocre fresh
				// one (#65). MinRelevance stands alone — with half-life 0 the multiplier
				// is a no-op (factor 1) and this is a pure relevance cut. Its default
				// stays 0: raw cosine distributions differ sharply between embedders
				// (hash dim 64 vs model2vec dim 256), so any non-zero default is right
				// for one and wrong for the other. DecayFloor defaults to the shared
				// scale-free search.DefaultDecayFloor.
				opts.Decay = &search.DecayOptions{
					HalfLifeDays: halfLife,
					Now:          clock.Now(),
					MinRelevance: minRelevance,
					DecayFloor:   decayFloor,
				}
			}
			if lexicalGate {
				// Lexical gate (#66): reduce the query to its stemmed content terms,
				// resolve the term-wise match set against the live bundle prose, and
				// hand both to the engine. Built via the shared search.BuildLexicalGate
				// so the CLI and the /api/v1/search HTTP surface construct byte-identical
				// gate options from the same bundle — the two surfaces agree by
				// construction, not by duplicated literals. The engine degrades to pure
				// semantic when the terms are empty (all-stopword query) or the match set
				// is over-broad, so an on-by-request gate is still a no-op on the query
				// shapes where a lexical filter would only add noise.
				opts.LexicalGate = search.BuildLexicalGate(b, semantic)
			}

			res, err := search.QueryWith(s, e, semantic, k, opts)
			if err != nil {
				return err
			}
			printResults(cmd, res)
			return nil
		},
	}
	root.PersistentFlags().StringVar(&embedderName, "embedder", "hash", "embedder: hash (offline default) | model2vec (local static model)")
	root.PersistentFlags().StringVar(&modelPath, "model-path", "", "model2vec model directory (overrides the model_path config key)")
	root.Flags().StringVar(&semantic, "semantic", "", "semantic query string")
	root.PersistentFlags().IntVar(&k, "k", 5, "max results")
	// §4.1 scoping filters, applied pre-ranking. Each flag is repeatable: repeats
	// of the SAME flag compose with OR; the three dimensions compose with AND.
	// Empty = no constraint (the query is unchanged). No comma-separated values —
	// commas are legal in paths, titles and tags, so repeated flags are the
	// unambiguous idiomatic cobra form.
	root.Flags().StringArrayVar(&pathPrefix, "path", nil, "restrict to nodes whose path starts with this prefix (repeatable; repeats OR together)")
	root.Flags().StringArrayVar(&typeFilter, "type", nil, "restrict to nodes with this §4.1 type (repeatable; repeats OR together)")
	root.Flags().StringArrayVar(&tagFilter, "tag", nil, "restrict to nodes carrying this §4.1 tag (repeatable; repeats OR together)")
	// Negating filters, applied AFTER the positive set (exclusion beats inclusion).
	// An empty positive set still means "all nodes," so --not-path research/ alone
	// is a complete query. Repeatable, each repeat OR-excludes.
	root.Flags().StringArrayVar(&notPath, "not-path", nil, "exclude nodes whose path starts with this prefix (repeatable)")
	root.Flags().StringArrayVar(&notType, "not-type", nil, "exclude nodes with this §4.1 type (repeatable)")
	root.Flags().StringArrayVar(&notTag, "not-tag", nil, "exclude nodes carrying this §4.1 tag (repeatable)")
	// §5.2/§13.1 recency decay, post-ranking, off by default (0 = no decay). The
	// help text describes what the command actually guarantees once bounded: decay
	// reorders on cosine×recency, MinRelevance drops sub-floor nodes before that
	// reorder, and DecayFloor clamps the multiplier so recency cannot erase a
	// still-relevant node.
	root.Flags().Float64Var(&halfLife, "half-life", 0, "recency half-life in days (0 = no decay); combine with --min-relevance and --decay-floor to bound how far recency reorders results")
	// #65: clamp the recency multiplier so an old-but-relevant node can be demoted
	// but never crushed below a mediocre fresh one. Scale-free, so it assumes
	// nothing about the embedder's cosine distribution; 0 restores unbounded decay.
	// The default is the shared search.DefaultDecayFloor — the SAME constant the
	// HTTP /api/v1/search surface reads — so the two surfaces cannot drift apart.
	root.Flags().Float64Var(&decayFloor, "decay-floor", search.DefaultDecayFloor, "lower clamp on the recency multiplier 0.5^(age/half-life) (0 = unbounded decay)")
	// #65: raw-cosine floor applied BEFORE decay reorders, so a sub-floor fresh
	// node is dropped rather than promoted. Default 0 (admit everything) because
	// raw cosine distributions differ sharply between embedders, so a non-zero
	// default is right for one and wrong for another.
	root.Flags().Float64Var(&minRelevance, "min-relevance", 0, "drop results whose RAW cosine is below this before decay reorders (0 = admit everything)")
	// #66 lexical gate, off by default. Intersects the semantic band with a
	// term-wise lexical match set and preserves the lexical tail; degrades to pure
	// semantic on an all-stopword or over-broad query.
	root.Flags().BoolVar(&lexicalGate, "lexical-gate", false, "gate semantic results by a term-wise lexical match, preserving lexical recall (off by default; no-op on all-stopword or over-broad queries)")

	root.AddCommand(newIndexCmd(&embedderName, &modelPath))
	root.AddCommand(newRelatedCmd(&k))
	return root
}

// nonEmpty drops empty-string values from a repeatable flag's slice. An empty
// filter value (e.g. `--type ""`) is not a real constraint and must behave as the
// no-op path, identical to omitting the flag — this preserves the pre-repeatable
// contract that empty-string flags equal a bare query. Returns nil for an
// all-empty (or nil) input so the Filter dimension reads as unset.
func nonEmpty(vs []string) []string {
	out := vs[:0:0] // fresh backing array; never alias the flag's slice
	for _, v := range vs {
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildNodeMeta resolves the per-node metadata the filters and recency decay key
// on from the live bundle: §4.1 type/tags and §5.2 generated.at (with the §13.1
// legacy `timestamp` fallback, via Node.Generated). This is the query-time
// resolution the card mandates instead of denormalizing onto the index.
func buildNodeMeta(b *okf.Bundle) map[string]search.NodeMeta {
	m := make(map[string]search.NodeMeta, len(b.Nodes))
	for path, n := range b.Nodes {
		nm := search.NodeMeta{Type: n.Type(), Tags: n.Tags()}
		if gen, ok := n.Generated(); ok { // §5.2 generated.at / §13.1 timestamp fallback
			nm.Generated = gen.At
			nm.HasGenerated = true
		}
		m[path] = nm
	}
	return m
}

func newIndexCmd(embedderName, modelPath *string) *cobra.Command {
	c := &cobra.Command{Use: "index", Short: "Manage the semantic index"}
	c.AddCommand(&cobra.Command{
		Use:   "build [bundle-dir]",
		Short: "Embed changed concept nodes into .okfctl/index.db",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := resolveEmbedder(*embedderName, *modelPath)
			if err != nil {
				return err
			}
			dir := bundleArg(args)
			b, err := okf.Load(dir)
			if err != nil {
				return err
			}
			prev, _ := search.Load(indexPath(dir)) // best-effort reuse; nil is fine
			s := search.BuildIndex(b, e, prev)
			if err := os.MkdirAll(filepath.Dir(indexPath(dir)), 0o750); err != nil {
				return err
			}
			if err := s.Save(indexPath(dir)); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "indexed %d node(s) with %s (dim %d) -> %s\n",
				len(s.Entries), s.Model, s.Dim, indexPath(dir))
			return nil
		},
	})
	return c
}

func newRelatedCmd(k *int) *cobra.Command {
	return &cobra.Command{
		Use:   "related <node-path> [bundle-dir]",
		Short: "Show a node's nearest semantic neighbors",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodePath := args[0]
			dir := "."
			if len(args) == 2 {
				dir = args[1]
			}
			s, err := search.Load(indexPath(dir))
			if err != nil {
				return fmt.Errorf("no index at %s (run 'okfctl-search index build' first): %w", indexPath(dir), err)
			}
			res, err := search.Related(s, nodePath, *k)
			if err != nil {
				return err
			}
			printResults(cmd, res)
			return nil
		},
	}
}

// bundleArg returns the trailing bundle-dir arg, defaulting to ".".
func bundleArg(args []string) string {
	if len(args) > 0 {
		return args[len(args)-1]
	}
	return "."
}

func printResults(cmd *cobra.Command, res []search.Result) {
	for _, r := range res {
		fmt.Fprintf(cmd.OutOrStdout(), "%.4f	%s\n", r.Score, r.Path)
		if s := snippetPreview(r.Snippet); s != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "	%s\n", s)
		}
	}
}

// snippetPreview flattens a passage's markdown into a single-line preview:
// whitespace runs (including newlines) collapse to single spaces and the result
// is truncated to a readable width so long sections do not flood the terminal.
// Returns "" for an empty snippet (e.g. a legacy passage-less index).
func snippetPreview(text string) string {
	const maxLen = 200
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	s := strings.Join(fields, " ")
	// Truncate on rune boundaries, not bytes: the KB carries multi-byte UTF-8
	// (curly quotes, em-dashes, accented names), and a byte-boundary cut would
	// split a rune and emit a mangled partial byte before the ellipsis.
	if r := []rune(s); len(r) > maxLen {
		s = string(r[:maxLen]) + "…"
	}
	return s
}
