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

func newGraphCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "graph",
		Short: "Export the bundle's link graph",
	}
	c.AddCommand(newGraphExportCmd())
	return c
}

func newGraphExportCmd() *cobra.Command {
	var format string
	var noIgnore *bool
	c := &cobra.Command{
		Use:   "export [bundle-dir]",
		Short: "Print the link graph as JSON or Graphviz DOT",
		Long: "graph export prints the bundle's link graph for another tool to draw or check. " +
			"Each node is a concept with its path, title, type, and top-level directory. Orphans " +
			"are flagged. Each edge is a link or path field from one concept to another (§6.4).\n\n" +
			"JSON is the default. --format dot writes Graphviz DOT. Pipe that to dot -Tsvg for a " +
			"picture. It never edits a file.",
		Example: "  # Print the graph of the bundle in the current directory as JSON\n" +
			"  okfctl graph export\n\n" +
			"  # Print a bundle elsewhere as Graphviz DOT\n" +
			"  okfctl graph export --format dot ./mykb\n\n" +
			"  # Draw it as an SVG with Graphviz\n" +
			"  okfctl graph export --format dot ./mykb | dot -Tsvg > graph.svg",
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
			g := okf.BuildGraph(b)
			switch format {
			case "json":
				out, err := graphJSON(g)
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), out)
			case "dot":
				fmt.Fprint(cmd.OutOrStdout(), graphDOT(g))
			case "svg":
				return fmt.Errorf("no native svg format; pipe dot to Graphviz: " +
					"okfctl graph export --format dot | dot -Tsvg > graph.svg")
			default:
				return fmt.Errorf("unknown format %q (want json or dot)", format)
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "json", "output format: json or dot")
	noIgnore = addNoIgnoreFlag(c)
	return c
}

// graphJSON marshals a Graph to indented, deterministic JSON.
func graphJSON(g okf.Graph) (string, error) {
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal graph: %w", err)
	}
	return string(b), nil
}

// graphDOT emits Graphviz DOT for a Graph. Orphan nodes are styled dashed.
func graphDOT(g okf.Graph) string {
	var sb strings.Builder
	sb.WriteString("digraph okf {\n")
	sb.WriteString("  rankdir=LR;\n")
	for _, n := range g.Nodes {
		style := ""
		if n.Orphan {
			style = ", style=dashed"
		}
		fmt.Fprintf(&sb, "  %q [label=%q%s];\n", n.Path, n.Title, style)
	}
	for _, e := range g.Edges {
		fmt.Fprintf(&sb, "  %q -> %q;\n", e.From, e.To)
	}
	sb.WriteString("}\n")
	return sb.String()
}
