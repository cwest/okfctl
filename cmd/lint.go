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
	"encoding/json"
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
	var jsonOut bool
	var coverageThreshold int
	var semantic bool
	var similarityThreshold float64
	var isolationFloor float64
	var noIgnore *bool
	c := &cobra.Command{
		Use:   "lint [bundle-dir]",
		Short: "Report what a curator should look at: orphans, unlinked mentions, broken links, hygiene",
		Long: "lint reports what a careful curator would want a second look at. You can pass " +
			"validate with a node no other node links to, or with a node that names another and " +
			"doesn't link it. lint prints one line per finding, then a count. It exits 0 by " +
			"default. Run lint --strict in CI and it exits 1 on any finding.\n\n" +
			"It never edits a file. None of its findings makes a bundle non-conformant under " +
			"§11.1. These are its checks:\n\n" +
			"    orphan                   nothing links to the node, not index.md or log.md either\n" +
			"    missing-xref             the body names another node's title but doesn't link it\n" +
			"    broken-link              a link or path field finds no node, but one with that\n" +
			"                             file name exists elsewhere\n" +
			"    coverage-gap             an alias some node declares, mentioned by at least\n" +
			"                             --coverage-threshold nodes, with no node of its own\n" +
			"    type-hygiene             type values that differ only by case or a plural s\n" +
			"    tag-hygiene              tags that fold to one spelling (case, separators, or a\n" +
			"                             plural of a tag already in use)\n" +
			"    status-lifecycle         a status §5.4 doesn't define, so the node isn't stable\n" +
			"    spec-version             a per-node okf_spec_version key (the version belongs\n" +
			"                             on the bundle-root index.md, §8.1)\n" +
			"    source-missing-resource  a sources entry with no resource (§5.1)\n" +
			"    timestamp-form           a §5 timestamp that's neither an RFC 3339 date-time\n" +
			"                             with an offset nor a plain date\n" +
			"    verified-event           a verified event missing by or at (§5.2)\n" +
			"    superseded-by            a superseded_by that finds no concept, or sits on a\n" +
			"                             node that isn't deprecated\n" +
			"    footnote-definition      a footnote label defined twice, or a definition with\n" +
			"                             no text (§4.2)\n\n" +
			"A link to a file that exists nowhere isn't a finding. The spec treats it as " +
			"knowledge no one has written (§6.1). analyze lists it under coverage. The two " +
			"hygiene checks group spellings that look alike, but two tags can look alike and " +
			"still mean different things. Read the finding first and rename later. An " +
			"unparseable stale_after gets a timestamp-form finding and never counts as fresh " +
			"(§5.5).\n\n" +
			"--semantic adds two checks that read the index okfctl-search index build writes. " +
			"similar-unlinked flags two nodes that read alike with no link between them. " +
			"no-semantic-neighbors flags a node with nothing close to it. Nodes added since the " +
			"last build aren't in the index. lint can't check them and names them in one " +
			"stale-index finding. lint loads no model to do this. It reads the index. With no " +
			"index on disk, lint exits 1 and says to build one.",
		Example: "  # Report findings for the bundle in the current directory (exits 0)\n" +
			"  okfctl lint\n\n" +
			"  # Fail CI on any finding\n" +
			"  okfctl lint --strict ./mykb\n\n" +
			"  # Print findings as JSON for another tool\n" +
			"  okfctl lint --json ./mykb\n\n" +
			"  # Add the similarity checks (needs okfctl-search index build ./mykb first)\n" +
			"  okfctl lint --semantic ./mykb",
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
			if jsonOut {
				// Machine path: marshal the findings as a bare JSON array,
				// preserving Lint()'s (path, then check) sort. A clean bundle
				// emits "[]" (never null, never the human "OK" line) so a CI
				// consumer can always `jq length`. Under --strict the JSON is
				// still the only thing on stdout — the non-zero exit is
				// signalled by the returned error, never by a prose trailer
				// mixed into the stream. Mirrors analyze --json.
				if findings == nil {
					findings = []okf.LintFinding{}
				}
				enc, err := json.MarshalIndent(findings, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal findings: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(enc))
				if strict && len(findings) > 0 {
					return fmt.Errorf("%d lint finding(s)", len(findings))
				}
				return nil
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
	c.Flags().BoolVar(&jsonOut, "json", false, "emit findings as a machine-readable JSON array (sorted path, then check) instead of the human report")
	c.Flags().IntVar(&coverageThreshold, "coverage-threshold", 0, "min distinct nodes that must mention a term to report a coverage gap (default 3)")
	c.Flags().BoolVar(&semantic, "semantic", false, "also run similarity checks against the index built by 'okfctl-search index build'")
	c.Flags().Float64Var(&similarityThreshold, "similarity-threshold", 0, "cosine score at/above which two unlinked nodes are reported (default 0.80; implies --semantic data)")
	c.Flags().Float64Var(&isolationFloor, "isolation-floor", 0, "score a node's best neighbor must reach to count as connected (default 0.20)")
	noIgnore = addNoIgnoreFlag(c)
	return c
}
