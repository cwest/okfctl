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
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// validFields are the lexical match surfaces accepted by --field.
var validFields = map[string]okf.SearchField{
	"any":   okf.FieldAny,
	"title": okf.FieldTitle,
	"tag":   okf.FieldTag,
	"type":  okf.FieldType,
	"body":  okf.FieldBody,
}

func newSearchCmd() *cobra.Command {
	var (
		field     string
		neighbors string
		depth     int
		asJSON    bool
		noIgnore  bool
	)

	c := &cobra.Command{
		Use:   "search [query] [bundle-dir]",
		Short: "Search the bundle lexically or by graph neighborhood (core, stdlib-only)",
		Long: "search queries the bundle from the CLI without a model or index.\n\n" +
			"Lexical mode (default): okfctl search \"query\" [dir] matches concept nodes by\n" +
			"title, tag, type, or body substring (case-insensitive). Restrict the surface\n" +
			"with --field title|tag|type|body.\n\n" +
			"Graph-structural mode: okfctl search --neighbors <node-path> [dir] returns the\n" +
			"nodes within --depth hops of a node in the link graph (edges are undirected).\n\n" +
			"Semantic (vector) search is the separate okfctl-search plugin: run\n" +
			"`okfctl-search --semantic \\\"query\\\"` (PRD \\u00a78).",
		Example: "  # Lexical search across all fields in the current bundle\n" +
			"  okfctl search \"income statement\"\n\n" +
			"  # Restrict the match surface to titles, in a bundle elsewhere\n" +
			"  okfctl search --field title revenue ./bundles/knowledge\n\n" +
			"  # Graph mode: nodes within 2 hops of a node in the link graph\n" +
			"  okfctl search --neighbors concepts/income-statement.md --depth 2 ./bundles/knowledge",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Neighborhood mode: --neighbors names the start node; positionals
			// are the bundle dir only. A lexical query cannot ride along, so a
			// second positional (query + dir) is a usage conflict.
			if neighbors != "" {
				if len(args) == 2 {
					return fmt.Errorf("--neighbors cannot be combined with a lexical query")
				}
				dir := "."
				if len(args) == 1 {
					dir = args[0]
				}
				return runNeighbors(cmd, dir, neighbors, depth, asJSON, noIgnore)
			}

			// Lexical mode: first positional is the query, optional second is dir.
			if len(args) == 0 {
				return fmt.Errorf("provide a query (okfctl search \"term\") or use --neighbors <node-path>")
			}
			query := args[0]
			dir := "."
			if len(args) == 2 {
				dir = args[1]
			}
			return runLexical(cmd, dir, query, field, asJSON, noIgnore)
		},
	}
	c.Flags().StringVar(&field, "field", "any", "lexical match surface: any|title|tag|type|body")
	c.Flags().StringVar(&neighbors, "neighbors", "", "graph mode: node path to traverse from")
	c.Flags().IntVar(&depth, "depth", 1, "neighborhood traversal depth (hops)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit results as JSON")
	c.Flags().BoolVar(&noIgnore, "no-ignore", false, noIgnoreFlagUsage)
	return c
}

func runLexical(cmd *cobra.Command, dir, query, field string, asJSON, noIgnore bool) error {
	sf, ok := validFields[strings.ToLower(field)]
	if !ok {
		return fmt.Errorf("unknown --field %q (want any, title, tag, type, or body)", field)
	}
	b, err := loadBundleForCmd(cmd, dir, noIgnore)
	if err != nil {
		return err
	}
	results := okf.Search(b, query, sf)
	if asJSON {
		return writeJSON(cmd, results)
	}
	out := cmd.OutOrStdout()
	for _, r := range results {
		fmt.Fprintf(out, "%-40s %-12s [%s]\n", r.Path, r.Type, strings.Join(r.MatchedOn, ","))
	}
	return nil
}

func runNeighbors(cmd *cobra.Command, dir, node string, depth int, asJSON, noIgnore bool) error {
	b, err := loadBundleForCmd(cmd, dir, noIgnore)
	if err != nil {
		return err
	}
	results, ok := okf.Neighborhood(b, node, depth)
	if !ok {
		return fmt.Errorf("node not found: %s", node)
	}
	if asJSON {
		return writeJSON(cmd, results)
	}
	out := cmd.OutOrStdout()
	for _, r := range results {
		fmt.Fprintf(out, "%-40s %-12s depth=%d\n", r.Path, r.Type, r.Depth)
	}
	return nil
}

// writeJSON marshals v as indented, deterministic JSON. A nil slice is rendered
// as an empty JSON array rather than null so consumers get a stable shape.
func writeJSON(cmd *cobra.Command, v any) error {
	switch t := v.(type) {
	case []okf.SearchResult:
		if t == nil {
			v = []okf.SearchResult{}
		}
	case []okf.NeighborResult:
		if t == nil {
			v = []okf.NeighborResult{}
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}
