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

func newValidateCmd() *cobra.Command {
	var templates, strict bool
	c := &cobra.Command{
		Use:   "validate [bundle-dir]",
		Short: "Check a bundle for OKF spec-floor conformance (optionally overlay team type-templates)",
		Long: "validate enforces the OKF spec floor (type present + non-empty, §7). " +
			"With --templates it also runs the opt-in team overlay (§9.4), reporting " +
			"template drift as warnings; drift never fails the floor. Drift is advisory " +
			"by default (exit 0); pass --strict to exit non-zero on drift. Floor " +
			"violations always fail regardless of --strict.",
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
			out := cmd.OutOrStdout()
			findings := okf.Validate(b)
			for _, f := range findings {
				fmt.Fprintf(out, "FAIL %s: %s\n", f.Path, f.Message)
			}

			var drift []okf.DriftFinding
			if templates {
				drift = okf.TemplateDrift(b)
				for _, d := range drift {
					fmt.Fprintf(out, "warning %s: %s\n", d.Path, d.Message)
				}
			}

			// The spec floor is non-negotiable: any floor finding fails.
			if len(findings) > 0 {
				return fmt.Errorf("%d conformance finding(s)", len(findings))
			}
			// Template drift is advisory unless --strict.
			if len(drift) > 0 {
				fmt.Fprintf(out, "%d template drift warning(s)\n", len(drift))
				if strict {
					return fmt.Errorf("%d template drift warning(s)", len(drift))
				}
				return nil
			}
			if templates {
				fmt.Fprintln(out, "OK: bundle conforms to the OKF spec floor and team templates")
			} else {
				fmt.Fprintln(out, "OK: bundle conforms to the OKF spec floor")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&templates, "templates", false, "also run the opt-in type-template overlay (§9.4), reporting drift as warnings")
	c.Flags().BoolVar(&strict, "strict", false, "with --templates, exit non-zero on any template drift (default: advisory, exit 0)")
	return c
}
