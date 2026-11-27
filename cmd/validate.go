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
	var templatesFrom string
	var noIgnore *bool
	c := &cobra.Command{
		Use:   "validate [bundle-dir]",
		Short: "Check a bundle against the OKF spec floor, and optionally your team's type templates",
		Long: "validate checks a bundle against the floor every OKF bundle has to meet (§11.1). " +
			"Every frontmatter block has to parse as YAML. Each reserved index.md and log.md has " +
			"to follow §8 and §9. A floor failure prints FAIL and always exits 1.\n\n" +
			"It never edits a file. A .md file with no frontmatter or no type isn't a concept " +
			"(OKF v0.3 §3.2). validate warns about it and the run still passes. It also warns " +
			"about drift. One kind is a node whose modified date disagrees with its last git " +
			"commit (outside a git repo it's silent). The other is an okf_version in the legacy " +
			".okf sidecar that disagrees with the one in index.md (§8.1, §12).\n\n" +
			"Drift is advice. The run exits 0 unless you pass --strict.\n\n" +
			"--templates also checks each node against your team's type template (PRD §9.4) and " +
			"reports where a node strays from it as more drift. The templates can live in another " +
			"bundle, named by the .okf templates: key or by --templates-from (PRD §9.2). An " +
			"unknown type or an extra key never fails the floor (§11.2).",
		Example: "  # Check the bundle in the current directory\n" +
			"  okfctl validate\n\n" +
			"  # Check a bundle elsewhere\n" +
			"  okfctl validate ./mykb\n\n" +
			"  # Also check type templates, and fail CI on any drift\n" +
			"  okfctl validate --templates --strict ./mykb\n\n" +
			"  # Take the templates from another bundle instead of the .okf reference\n" +
			"  okfctl validate --templates --templates-from ../okf-type-templates ./mykb",
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
			out := cmd.OutOrStdout()
			findings := okf.Validate(b)
			for _, f := range findings {
				fmt.Fprintf(out, "FAIL %s: %s\n", f.Path, f.Message)
			}
			// §3.2 rule 5 / §11.2: a symlink that resolves outside the bundle
			// root was not read. That is a consumer rule, not a bundle
			// conformance failure (§11.1), so it is a warning and never changes
			// the exit code.
			for _, p := range b.EscapedSymlinks {
				fmt.Fprintf(out, "warning %s: %s\n", p, okf.EscapedSymlinkMessage)
			}
			// OKF v0.3 §3.2 rule 2: a .md with no frontmatter or no `type` is
			// not a concept. Validators SHOULD warn; it never affects
			// conformance, so it never fails the run (not even under --strict,
			// which governs drift).
			for _, w := range okf.ValidateWarnings(b) {
				fmt.Fprintf(out, "warning %s: %s\n", w.Path, w.Message)
			}

			var drift []okf.DriftFinding
			if templates {
				// Resolve the effective templates (§9.2): a referenced bundle
				// (--templates-from, or the .okf `templates:` key) folded in,
				// local Type Template nodes overlaid. A broken reference is a
				// hard error here — the overlay was explicitly requested.
				rt, rerr := okf.ResolveTemplates(b, templatesFrom)
				if rerr != nil {
					return rerr
				}
				drift = okf.TemplateDriftWith(b, rt.ByType)
				for _, d := range drift {
					fmt.Fprintf(out, "warning %s: %s\n", d.Path, d.Message)
				}
			}

			// §8.1/§12: okf_version is declared on the bundle-root index.md;
			// the .okf sidecar is a legacy fallback. When both declare a
			// version and disagree, the index wins and validate warns. This is
			// advisory like the drift warnings below (fails only under
			// --strict), never a spec-floor failure.
			var versionWarnings int
			if msg, ok := okf.VersionConflict(b); ok {
				fmt.Fprintf(out, "warning index.md: %s\n", msg)
				versionWarnings = 1
			}

			// Git drift: report when a node's frontmatter `modified` contradicts
			// its git last-commit date. Read-only (never rewrites the file), and
			// degrades to nothing outside a git repo. Advisory like template
			// drift — surfaced always, failing the run only under --strict.
			gitDrift := okf.DriftFindings(b)
			for _, d := range gitDrift {
				fmt.Fprintf(out, "warning %s: %s\n", d.Path, d.Message)
			}

			// The spec floor is non-negotiable: any floor finding fails.
			if len(findings) > 0 {
				return fmt.Errorf("%d conformance finding(s)", len(findings))
			}
			// Template drift, git drift, and a version conflict are advisory
			// unless --strict.
			totalDrift := len(drift) + len(gitDrift) + versionWarnings
			if totalDrift > 0 {
				fmt.Fprintf(out, "%d drift warning(s)\n", totalDrift)
				if strict {
					return fmt.Errorf("%d drift warning(s)", totalDrift)
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
	c.Flags().BoolVar(&templates, "templates", false, "also run the opt-in type-template overlay (PRD §9.4), reporting drift as warnings")
	c.Flags().StringVar(&templatesFrom, "templates-from", "", templatesFromFlagUsage)
	c.Flags().BoolVar(&strict, "strict", false, "exit non-zero on any drift (git drift and, with --templates, template drift); default: advisory, exit 0")
	noIgnore = addNoIgnoreFlag(c)
	return c
}
