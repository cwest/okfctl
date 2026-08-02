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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/search"
	"github.com/spf13/cobra"
)

// semanticIndexPath mirrors the plugin's index location. Core READS this index;
// only okfctl-search (which owns the embedding model) ever writes it.
func semanticIndexPath(bundleDir string) string {
	return filepath.Join(bundleDir, ".okfctl", "index.db")
}

// loadSemanticIndex reads the index the plugin built and adapts it into the
// neighbor-set shape the okf checks consume. Reading an index needs no embedder
// and no model — search.Related is cosine arithmetic over stored vectors — which
// is why the semantic checks can live in core without dragging a model runtime
// behind them.
func loadSemanticIndex(bundleDir string, b *okf.Bundle) (okf.SemanticIndex, error) {
	p := semanticIndexPath(bundleDir)
	if _, err := os.Stat(p); err != nil {
		return nil, fmt.Errorf("no semantic index at %s: run 'okfctl-search index build %s' first",
			p, bundleDir)
	}
	s, err := search.Load(p)
	if err != nil {
		return nil, fmt.Errorf("read semantic index %s: %w", p, err)
	}
	idx := make(okf.SemanticIndex, len(b.Nodes))
	k := len(b.Nodes) // every other node, so a check never misses a pair
	for path := range b.Nodes {
		res, err := search.Related(s, path, k)
		if err != nil {
			continue // not in the index; reported as drift by the checks
		}
		neighbors := make([]okf.Neighbor, 0, len(res))
		for _, r := range res {
			neighbors = append(neighbors, okf.Neighbor{Path: r.Path, Score: r.Score})
		}
		idx[path] = neighbors
	}
	return idx, nil
}

func newLintCmd() *cobra.Command {
	var strict bool
	var coverageThreshold int
	var semantic bool
	var similarityThreshold float64
	var isolationFloor float64
	var noIgnore *bool
	c := &cobra.Command{
		Use:   "lint [bundle-dir]",
		Short: "Report curation health findings for a bundle (orphans, missing cross-references, broken internal links, coverage gaps, type hygiene)",
		Long: "lint surfaces judgment-worthy curation findings, not spec-floor violations " +
			"(use validate for those). It never mutates the bundle. By default it is advisory " +
			"and exits 0 even with findings; pass --strict to exit non-zero on any finding.\n\n" +
			"A broken-link finding reports an internal .md link that resolves to no node when a " +
			"node with the same basename exists elsewhere — a moved or mistyped path (a defect), " +
			"distinct from a genuinely unwritten concept (a coverage gap, which analyze reports " +
			"advisorily and lint stays quiet on).\n\n" +
			"--semantic adds similarity-driven checks (similar-but-unlinked pairs, nodes with " +
			"no semantic neighbors) by reading the index built by 'okfctl-search index build'. " +
			"Core only reads that index, so no embedding model is needed to lint.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := loadBundleForCmd(cmd, dir, *noIgnore)
			if err != nil {
				return err
			}
			findings := okf.Lint(b, okf.LintOptions{CoverageThreshold: coverageThreshold})
			if semantic {
				// A missing index is an error rather than a quiet structural-only
				// pass: a CI job must never believe it ran semantic checks when it
				// did not.
				idx, err := loadSemanticIndex(dir, b)
				if err != nil {
					return err
				}
				findings = append(findings, okf.LintSemantic(b, idx, okf.SemanticOptions{
					SimilarityThreshold: similarityThreshold,
					IsolationFloor:      isolationFloor,
				})...)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "OK: no lint findings")
				return nil
			}
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", f.Message)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d lint finding(s)\n", len(findings))
			if strict {
				return fmt.Errorf("%d lint finding(s)", len(findings))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&strict, "strict", false, "exit non-zero if there are any findings (default: advisory, exit 0)")
	c.Flags().IntVar(&coverageThreshold, "coverage-threshold", 0, "min distinct nodes that must mention a term to report a coverage gap (default 3)")
	c.Flags().BoolVar(&semantic, "semantic", false, "also run similarity checks against the index built by 'okfctl-search index build'")
	c.Flags().Float64Var(&similarityThreshold, "similarity-threshold", 0, "cosine score at/above which two unlinked nodes are reported (default 0.80; implies --semantic data)")
	c.Flags().Float64Var(&isolationFloor, "isolation-floor", 0, "score a node's best neighbor must reach to count as connected (default 0.20)")
	noIgnore = addNoIgnoreFlag(c)
	return c
}
