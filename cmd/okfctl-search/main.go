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
	)

	root := &cobra.Command{
		Use:           "okfctl-search",
		Short:         "Offline semantic search over an OKF bundle (okfctl plugin)",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		// Root with --semantic runs a query; without it, prints help.
		RunE: func(cmd *cobra.Command, args []string) error {
			if semantic == "" {
				return cmd.Help()
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
			res, err := search.Query(s, e, semantic, k)
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

	root.AddCommand(newIndexCmd(&embedderName, &modelPath))
	root.AddCommand(newRelatedCmd(&k))
	return root
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
			if err := os.MkdirAll(filepath.Dir(indexPath(dir)), 0o755); err != nil {
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
