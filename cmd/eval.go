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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// newEvalCmd is the parent for the TACA-style trustworthiness eval pass. It
// groups the one dimension okfctl can honestly automate (transparency, a gate)
// with a spot-check sampler (sample, a scaffold for the three dimensions okfctl
// cannot automate without a model or the network).
func newEvalCmd() *cobra.Command {
	eval := &cobra.Command{
		Use:   "eval",
		Short: "Measure KB-node trustworthiness (TACA): a Transparency gate + a spot-check sampler",
		Long: "eval decomposes node trust along the four TACA dimensions — Transparency, " +
			"Accuracy, Calibration, Alignment — as far as a pure-Go, offline, no-model tool " +
			"can.\n\n" +
			"'eval transparency' is the deterministic gate: it checks that provenance is present " +
			"(a grade + citations) and that internal citations resolve. It's the first machine " +
			"gate that touches trust rather than format.\n\n" +
			"'eval sample' scaffolds the three dimensions okfctl can't automate (Accuracy, " +
			"Alignment, Calibration) into an eval-set for a human or an out-of-band LLM judge to " +
			"complete. okfctl never computes a truth verdict itself — checking a claim against a " +
			"source needs a model or the network, which core deliberately doesn't do.",
	}
	eval.AddCommand(newEvalTransparencyCmd())
	eval.AddCommand(newEvalSampleCmd())
	return eval
}

func newEvalTransparencyCmd() *cobra.Command {
	var strict bool
	var jsonOut bool
	var vocabFloor int
	var noIgnore *bool
	c := &cobra.Command{
		Use:   "transparency [bundle-dir]",
		Short: "Gate TACA-Transparency: grade present, cited, internal citations resolve",
		Long: "transparency runs the deterministic, offline TACA-Transparency checks over a " +
			"bundle. Like lint it's advisory (exit 0) by default and never mutates the bundle; " +
			"pass --strict to exit non-zero on any finding (the CI gate). The four checks:\n\n" +
			"  grade-missing        a node carries no epistemic OR authority grade\n" +
			"  grade-vocabulary     a grade value is off-vocabulary for the corpus (a likely typo)\n" +
			"  uncited              a node carries no citations of any kind\n" +
			"  citation-unresolved  an internal citation resolves to no node in the bundle\n\n" +
			"External http(s) citations are deliberately out of scope — verifying them needs the " +
			"network, which is the eval-sample / human pass, not this gate.",
		Example: "  # Report transparency findings (advisory, exit 0)\n" +
			"  okfctl eval transparency ./bundles/knowledge\n\n" +
			"  # Fail CI on any finding\n" +
			"  okfctl eval transparency --strict ./bundles/knowledge\n\n" +
			"  # Machine-readable findings\n" +
			"  okfctl eval transparency --json ./bundles/knowledge",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			b, err := loadBundleForCmd(cmd, dir, *noIgnore)
			if err != nil {
				return err
			}
			findings := okf.EvalTransparency(b, okf.EvalOptions{GradeVocabularyFloor: vocabFloor})
			if jsonOut {
				if findings == nil {
					findings = []okf.EvalFinding{}
				}
				enc, err := json.MarshalIndent(findings, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal findings: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(enc))
				if strict && len(findings) > 0 {
					return fmt.Errorf("%d transparency finding(s)", len(findings))
				}
				return nil
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "OK: no transparency findings")
				return nil
			}
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", f.Message)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d transparency finding(s)\n", len(findings))
			if strict {
				return fmt.Errorf("%d transparency finding(s)", len(findings))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&strict, "strict", false, "exit non-zero if there are any findings (default: advisory, exit 0)")
	c.Flags().BoolVar(&jsonOut, "json", false, "emit findings as a machine-readable JSON array (sorted path, then check)")
	c.Flags().IntVar(&vocabFloor, "grade-vocabulary-floor", 0, "min nodes carrying a grade value for it to count as corpus vocabulary (default 2)")
	noIgnore = addNoIgnoreFlag(c)
	return c
}

func newEvalSampleCmd() *cobra.Command {
	var count int
	var seed int64
	var changedSince string
	var format string
	var noIgnore *bool
	c := &cobra.Command{
		Use:   "sample [bundle-dir]",
		Short: "Emit a spot-check eval-set scaffold for Accuracy/Alignment/Calibration",
		Long: "sample selects a spot-check sample of nodes and emits a structured eval-set for " +
			"the three TACA dimensions okfctl can't automate — Accuracy (are the claims supported " +
			"by the cited source?), Alignment (does the node answer the question it set out to?), " +
			"and Calibration (does the grade hold up on re-check?). Every field okfctl can extract " +
			"is pre-populated; the judgment slots are left empty for a human or an out-of-band LLM " +
			"judge to fill in. okfctl computes NO truth verdict.\n\n" +
			"Selection: --changed-since <ref> samples nodes changed since a git ref (the curation/CI " +
			"hook); otherwise --sample N draws a deterministic seeded random sample.",
		Example: "  # A worksheet for 5 random nodes\n" +
			"  okfctl eval sample --sample 5 --format md ./bundles/knowledge\n\n" +
			"  # Machine eval-set for nodes changed on this branch (curation hook)\n" +
			"  okfctl eval sample --changed-since origin/main ./bundles/knowledge",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			if format != "json" && format != "md" {
				return fmt.Errorf("--format must be json or md, got %q", format)
			}
			b, err := loadBundleForCmd(cmd, dir, *noIgnore)
			if err != nil {
				return err
			}
			opts := okf.SampleOptions{Count: count, Seed: seed}
			if changedSince != "" {
				paths, err := changedNodePaths(dir, changedSince, b)
				if err != nil {
					return err
				}
				opts.Paths = paths
			}
			scaffolds := okf.EvalSample(b, opts)
			if format == "md" {
				fmt.Fprint(cmd.OutOrStdout(), renderScaffoldsMarkdown(scaffolds))
				return nil
			}
			if scaffolds == nil {
				scaffolds = []okf.NodeEvalScaffold{}
			}
			enc, err := json.MarshalIndent(scaffolds, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal scaffolds: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(enc))
			return nil
		},
	}
	c.Flags().IntVar(&count, "sample", 0, "size of a deterministic random sample (ignored when --changed-since is set)")
	c.Flags().Int64Var(&seed, "seed", 0, "seed for reproducible --sample selection (default: a fixed seed)")
	c.Flags().StringVar(&changedSince, "changed-since", "", "sample only nodes changed since this git ref (e.g. origin/main)")
	c.Flags().StringVar(&format, "format", "json", "output format: json (machine eval-set) or md (human worksheet)")
	noIgnore = addNoIgnoreFlag(c)
	return c
}

// changedNodePaths resolves the bundle-relative node paths changed since a git
// ref, using the same `git -C` seam as the rest of okfctl. Only paths that are
// actual concept nodes in the loaded bundle are returned (sorted, de-duped).
func changedNodePaths(dir, ref string, b *okf.Bundle) ([]string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve bundle dir: %w", err)
	}
	// ref sits in argv BEFORE the "--" separator, so a value like
	// "--upload-pack=..." or "--output=..." would be parsed by git as an
	// option, not a revision — a real option-injection surface for the
	// user-supplied --changed-since. We cannot move it after "--" (that flips
	// it from a revision to a pathspec), so reject any ref beginning with "-".
	if strings.HasPrefix(ref, "-") {
		return nil, fmt.Errorf("invalid git ref %q: must not begin with %q", ref, "-")
	}
	// Fixed git subcommand over the user's own bundle; ref is guarded above
	// against option injection and there is no shell (argv form).
	out, err := exec.Command("git", "-C", abs, "diff", "--name-only", ref, "--", ".").Output() //nolint:gosec // G204: fixed subcommand, argv form (no shell), ref guarded against option injection
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only %s: %w", ref, err)
	}
	// git reports paths relative to the repo root; make them relative to the
	// bundle dir, then keep only those that key a concept node.
	// Fixed git subcommand; the only variable is abs, a resolved filepath (not
	// a user ref), passed via -C. No shell (argv form).
	repoRoot, rrErr := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel").Output() //nolint:gosec // G204: fixed subcommand, argv form (no shell), only variable is the resolved bundle path
	base := abs
	if rrErr == nil {
		base = strings.TrimSpace(string(repoRoot))
	}
	seen := map[string]bool{}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		full := filepath.Join(base, filepath.FromSlash(line))
		rel, relErr := filepath.Rel(abs, full)
		if relErr != nil {
			continue
		}
		key := filepath.ToSlash(rel)
		if b.Nodes[key] != nil && !seen[key] {
			seen[key] = true
			paths = append(paths, key)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// renderScaffoldsMarkdown renders the eval-set as a human worksheet: one section
// per node, each carrying the three TACA dimension headers with the extracted
// context and empty slots to fill in.
func renderScaffoldsMarkdown(scaffolds []okf.NodeEvalScaffold) string {
	var sb strings.Builder
	sb.WriteString("# TACA spot-check worksheet\n\n")
	sb.WriteString("_okfctl extracted the context below; fill in each empty slot. " +
		"okfctl does not judge truth — you (or an LLM judge) do._\n\n")
	if len(scaffolds) == 0 {
		sb.WriteString("_No nodes sampled._\n")
		return sb.String()
	}
	for _, s := range scaffolds {
		fmt.Fprintf(&sb, "## %s\n\n", s.Path)
		fmt.Fprintf(&sb, "- **Title:** %s\n", s.Title)
		if s.Description != "" {
			fmt.Fprintf(&sb, "- **Description:** %s\n", s.Description)
		}
		fmt.Fprintf(&sb, "- **Grade:** epistemic=%s authority=%s trust-tier=%s\n",
			orDash(s.Epistemic), orDash(s.Authority), orDash(s.TrustTier))
		if len(s.Sources) > 0 {
			fmt.Fprintf(&sb, "- **Sources:** %s\n", strings.Join(s.Sources, ", "))
		} else {
			sb.WriteString("- **Sources:** (none)\n")
		}
		sb.WriteString("\n### Accuracy — is each claim supported by a cited source?\n\n")
		if len(s.Accuracy) == 0 {
			sb.WriteString("_No candidate claims extracted._\n\n")
		}
		for _, c := range s.Accuracy {
			fmt.Fprintf(&sb, "- Claim: %s\n  - Grounding (fill in): \n", c.Claim)
		}
		sb.WriteString("\n### Alignment — does the node answer the question it set out to?\n\n")
		fmt.Fprintf(&sb, "- Question: %s\n- Answered (fill in): \n\n", s.Alignment.Question)
		sb.WriteString("### Calibration — does the grade hold up on re-check?\n\n")
		fmt.Fprintf(&sb, "- Current grade: %s\n- Holds on re-check (fill in): \n\n", orDash(s.Calibration.CurrentGrade))
		sb.WriteString("---\n\n")
	}
	return sb.String()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
