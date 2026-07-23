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

func newLintCmd() *cobra.Command {
	var strict bool
	var coverageThreshold int
	c := &cobra.Command{
		Use:   "lint [bundle-dir]",
		Short: "Report curation health findings for a bundle (orphans, missing cross-references, coverage gaps, type hygiene)",
		Long: "lint surfaces judgment-worthy curation findings, not spec-floor violations " +
			"(use validate for those). It never mutates the bundle. By default it is advisory " +
			"and exits 0 even with findings; pass --strict to exit non-zero on any finding.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			findings := okf.Lint(b, okf.LintOptions{CoverageThreshold: coverageThreshold})
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
	return c
}
