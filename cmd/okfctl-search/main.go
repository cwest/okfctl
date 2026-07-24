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

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
	"github.com/spf13/cobra"
)

func main() {
	if err := newSearchCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "okfctl-search:", err)
		os.Exit(1)
	}
}

// resolveEmbedder returns the embedder named by --embedder. Only "hash" (the
// offline default) is available in this increment; "model2vec" is the pure-Go
// static model deferred to increment 5c — it errors honestly rather than
// silently falling back.
func resolveEmbedder(name string) (search.Embedder, error) {
	switch name {
	case "hash":
		return search.NewHashEmbedder(), nil
	case "model2vec":
		return nil, fmt.Errorf("--embedder model2vec is not yet available (increment 5c); use the default 'hash' embedder")
	default:
		return nil, fmt.Errorf("unknown embedder %q (available: hash)", name)
	}
}

func indexPath(bundleDir string) string {
	return filepath.Join(bundleDir, ".okfctl", "index.db")
}

func newSearchCmd() *cobra.Command {
	var (
		embedderName string
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
			e, err := resolveEmbedder(embedderName)
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
	root.PersistentFlags().StringVar(&embedderName, "embedder", "hash", "embedder: hash (offline default) | model2vec (increment 5c)")
	root.Flags().StringVar(&semantic, "semantic", "", "semantic query string")
	root.PersistentFlags().IntVar(&k, "k", 5, "max results")

	root.AddCommand(newIndexCmd(&embedderName))
	root.AddCommand(newRelatedCmd(&k))
	return root
}

func newIndexCmd(embedderName *string) *cobra.Command {
	c := &cobra.Command{Use: "index", Short: "Manage the semantic index"}
	c.AddCommand(&cobra.Command{
		Use:   "build [bundle-dir]",
		Short: "Embed changed concept nodes into .okfctl/index.db",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := resolveEmbedder(*embedderName)
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
		fmt.Fprintf(cmd.OutOrStdout(), "%.4f\t%s\n", r.Score, r.Path)
	}
}
