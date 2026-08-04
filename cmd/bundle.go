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
	bundle := &cobra.Command{Use: "bundle", Short: "Bundle lifecycle commands"}
	bundle.AddCommand(&cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a minimal conformant OKF bundle",
		Long: "init scaffolds a minimal bundle that conforms to the OKF spec floor: the two " +
			"reserved files (index.md and log.md, OKF §3.1) plus a bundle-root .okf sidecar " +
			"whose okf_version marks the target version (§12). It does NOT create any concept " +
			"nodes — a fresh bundle has zero nodes; use `okfctl node new` to add them. It refuses " +
			"to overwrite an existing bundle.",
		Example: "  # Scaffold a new bundle in the current directory\n" +
			"  okfctl bundle init\n\n" +
			"  # Scaffold in a named directory\n" +
			"  okfctl bundle init ./my-bundle",
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
		Short: "Summarize a bundle (node count, spec version)",
		Long: "info prints a one-glance summary of a bundle: how many concept nodes it holds, " +
			"how many reserved files (index.md/log.md, OKF §3.1) it carries, and the okf_version " +
			"it declares (§12, read from the bundle-root .okf sidecar). It is read-only and never " +
			"mutates the bundle. It does not validate conformance — use `okfctl validate` for that.",
		Example: "  # Summarize the bundle in the current directory\n" +
			"  okfctl bundle info\n\n" +
			"  # Summarize a bundle elsewhere\n" +
			"  okfctl bundle info ./bundles/knowledge",
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
			fmt.Fprintf(cmd.OutOrStdout(), "nodes: %d\nreserved: %d\nokf_version: %s\n",
				len(b.Nodes), len(b.Reserved), b.OkfVersion)
			return nil
		},
	})
	return bundle
}
