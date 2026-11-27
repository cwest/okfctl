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

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newBundleCmd() *cobra.Command {
	bundle := &cobra.Command{Use: "bundle", Short: "Create a bundle or summarize one"}
	bundle.AddCommand(&cobra.Command{
		Use:   "init [dir]",
		Short: "Create a bundle with index.md and log.md, declared OKF v0.3",
		Long: "init creates a bundle in dir (the current directory if you leave dir off) and " +
			"makes the directory when it's missing. It writes the two reserved files (OKF §3.1). " +
			"The root index.md declares okf_version \"0.3\" in its frontmatter (§8.1, §12). " +
			"log.md starts with a placeholder line.\n\n" +
			"There are no concept nodes at first. Add them with `okfctl node new`; index.md " +
			"and log.md change with each node you add.\n\n" +
			"init never checks for a bundle that's there. Run it on one and it exits 0 after " +
			"overwriting index.md and log.md. The root frontmatter (a title or description you " +
			"added) and the change history are gone, but none of your nodes are touched. If " +
			"you aren't sure a directory is empty, run `okfctl bundle info` on it first.",
		Example: "  $ okfctl bundle init newkb\n" +
			"  Initialized OKF bundle in newkb\n" +
			"  $ ls newkb\n" +
			"  index.md\n" +
			"  log.md\n" +
			"  $ cat newkb/index.md\n" +
			"  ---\n" +
			"  okf_version: \"0.3\"\n" +
			"  ---\n" +
			"\n" +
			"  # Knowledge Base\n" +
			"\n" +
			"  _Progressive-disclosure entry point. Add \"start here\" links as nodes land._",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			if err := okf.Scaffold(dir); err != nil {
				return fmt.Errorf("scaffold: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized OKF bundle in %s\n", dir)
			return nil
		},
	})
	bundle.AddCommand(&cobra.Command{
		Use:   "info [dir]",
		Short: "Show a bundle's node count and declared OKF version",
		Long: "info prints three lines about the bundle in dir (the current directory by " +
			"default): how many concept nodes it has, how many reserved files (index.md and " +
			"log.md at each level, OKF §3.1), plus the okf_version it declares (§12). One node " +
			"in a subdirectory makes three reserved files in the example. wine/ gets an " +
			"index.md of its own.\n\n" +
			"The version comes from the frontmatter of the bundle-root index.md (§8.1). When " +
			"that has none, info reads the older .okf sidecar file. If neither declares one, " +
			"info prints undeclared. okfctl then applies the v0.2 fallbacks §12.1 allows.\n\n" +
			"info only reads. To find out if the bundle conforms, run `okfctl validate`.",
		Example: "  $ okfctl bundle init cellar\n" +
			"  Initialized OKF bundle in cellar\n" +
			"  $ okfctl node new wine/tannin --type Concept --bundle cellar\n" +
			"  Created cellar/wine/tannin.md\n" +
			"  $ okfctl bundle info cellar\n" +
			"  nodes: 1\n" +
			"  reserved: 3\n" +
			"  okf_version: 0.3 (declared in index.md)",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := okf.Load(dir)
			if err != nil {
				return err
			}
			version, src := "undeclared", "v0.2 fallbacks apply, §12.1"
			if b.OkfVersionSource != okf.VersionUndeclared {
				version, src = b.DeclaredOkfVersion, "declared in "+b.OkfVersionSource
			}
			fmt.Fprintf(cmd.OutOrStdout(), "nodes: %d\nreserved: %d\nokf_version: %s (%s)\n",
				len(b.Nodes), len(b.Reserved), version, src)
			return nil
		},
	})
	return bundle
}
