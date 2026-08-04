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

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// newMigrateCmd builds `okfctl migrate`, the two-phase v0.1 -> v0.2 upgrade
// path. It is deliberately consumer-agnostic and carries NO model, network, or
// credential dependency on any path (the tool ships to strangers who have a
// bundle and nothing else):
//
//	okfctl migrate <bundle> --plan migrate-plan.json --generated-by <actor>
//	     phase 1 (default): PURE READ. Computes every deterministic §13.1 edit
//	     and enumerates every judgment item, writing only the plan file.
//
//	okfctl migrate <bundle> --apply --plan migrate-plan.json [--dry-run]
//	     phase 2: reads the plan back, applies its deterministic edits
//	     (order-preserving, additive-only), then re-validates. --dry-run writes
//	     nothing and its outcome is byte-identical to the real apply.
//
// The plan file is the review surface, for Casey and for a solo user alike. A
// judgment item (a citation with no follow-able resource per §5.1, or a
// timestamp rename with no actor per §7) is NEVER guessed — it stays in the plan
// for its (unspecified) consumer to resolve.
func newMigrateCmd() *cobra.Command {
	var apply, dry bool
	var planPath, generatedBy string
	c := &cobra.Command{
		Use:   "migrate <bundle>",
		Short: "Upgrade a bundle from OKF v0.1 to v0.2 (two-phase, consumer-agnostic)",
		Long: "migrate is the supported v0.1 -> v0.2 upgrade path. It runs in two phases " +
			"so it never acquires a model dependency. Phase 1 (default) is PURE READ: it " +
			"computes every deterministic §13.1 edit (timestamp -> generated.at; body " +
			"# Citations with a follow-able resource -> frontmatter sources) and enumerates " +
			"every judgment item (a prose citation with no resource per §5.1; a timestamp " +
			"rename with no actor per §7), writing only the plan file. Phase 2 (--apply) reads " +
			"the plan back and applies its deterministic edits order-preserving and " +
			"additive-only, then re-validates. --dry-run writes nothing and is byte-identical " +
			"to the real apply. Judgment items are never guessed — they stay in the plan for " +
			"its consumer (agent, colleague, shell loop, or a human) to resolve.",
		Example: "  # Phase 1 (default): compute the plan (pure read), writing only the plan file\n" +
			"  okfctl migrate ./bundles/knowledge --plan migrate-plan.json --generated-by \"casey\"\n\n" +
			"  # Phase 2: preview the apply without writing (byte-identical to the real apply)\n" +
			"  okfctl migrate ./bundles/knowledge --apply --plan migrate-plan.json --dry-run\n\n" +
			"  # Phase 2: apply the plan's deterministic edits, then re-validate\n" +
			"  okfctl migrate ./bundles/knowledge --apply --plan migrate-plan.json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if planPath == "" {
				return fmt.Errorf("--plan <path> is required (the migration plan file)")
			}
			if apply {
				return runMigrateApply(cmd, dir, planPath, dry)
			}
			return runMigratePlan(cmd, dir, planPath, generatedBy)
		},
	}
	c.Flags().BoolVar(&apply, "apply", false, "phase 2: read the plan and apply it (default is phase 1: plan only)")
	c.Flags().BoolVar(&dry, "dry-run", false, "with --apply: write nothing; byte-identical to the real apply")
	c.Flags().StringVar(&planPath, "plan", "migrate-plan.json", "migration plan file (written in phase 1, read in phase 2)")
	c.Flags().StringVar(&generatedBy, "generated-by", "", "actor (§7) recorded as generated.by for each timestamp rename")
	return c
}

// runMigratePlan is phase 1: compute the plan (pure read) and write it to disk.
func runMigratePlan(cmd *cobra.Command, dir, planPath, generatedBy string) error {
	b, err := okf.Load(dir)
	if err != nil {
		return fmt.Errorf("load bundle: %w", err)
	}
	plan, err := okf.PlanMigration(b, generatedBy)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("encode plan: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(planPath, data, 0o644); err != nil {
		return fmt.Errorf("write plan %s: %w", planPath, err)
	}
	out := cmd.OutOrStdout()
	det := 0
	for _, nm := range plan.Nodes {
		if nm.Generated != nil {
			det++
		}
		det += len(nm.Sources)
	}
	fmt.Fprintf(out, "Wrote %s: %d node(s) with deterministic edits (%d edit(s)), %d judgment item(s).\n",
		planPath, len(plan.Nodes), det, len(plan.Judgment))
	if len(plan.Judgment) > 0 {
		fmt.Fprintf(out, "%d item(s) need a decision before they can migrate; see the plan file.\n", len(plan.Judgment))
	}
	return nil
}

// runMigrateApply is phase 2: read the plan back and apply it, then re-validate.
// --dry-run writes nothing.
func runMigrateApply(cmd *cobra.Command, dir, planPath string, dry bool) error {
	planData, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("read plan %s: %w", planPath, err)
	}
	var plan okf.MigratePlan
	if err := json.Unmarshal(planData, &plan); err != nil {
		return fmt.Errorf("parse plan %s: %w", planPath, err)
	}
	b, err := okf.Load(dir)
	if err != nil {
		return fmt.Errorf("load bundle: %w", err)
	}
	out := cmd.OutOrStdout()
	if dry {
		for _, nm := range plan.Nodes {
			if nm.Generated != nil {
				fmt.Fprintf(out, "would rewrite %s: timestamp -> generated { by: %s, at: %s }\n",
					nm.Path, nm.Generated.By, nm.Generated.At)
			}
			for _, s := range nm.Sources {
				fmt.Fprintf(out, "would add %s: sources[].resource = %s\n", nm.Path, s.Resource)
			}
		}
		fmt.Fprintf(out, "%d node(s) would be migrated to v%s (dry run; nothing written).\n",
			len(plan.Nodes), plan.TargetVersion)
		return nil
	}
	if err := okf.MigrateApply(dir, b, plan); err != nil {
		return err
	}
	// Re-validate: a migration you cannot re-validate is one you cannot trust.
	b2, err := okf.Load(dir)
	if err != nil {
		return fmt.Errorf("reload after migrate: %w", err)
	}
	findings := okf.Validate(b2)
	for _, nm := range plan.Nodes {
		fmt.Fprintf(out, "migrated %s\n", nm.Path)
	}
	fmt.Fprintf(out, "%d node(s) migrated to v%s.\n", len(plan.Nodes), plan.TargetVersion)
	if len(findings) != 0 {
		for _, f := range findings {
			fmt.Fprintf(out, "%s: %s\n", f.Path, f.Message)
		}
		return fmt.Errorf("%d validation finding(s) after migrate", len(findings))
	}
	fmt.Fprintln(out, "bundle valid as v"+plan.TargetVersion)
	if len(plan.Judgment) > 0 {
		fmt.Fprintf(out, "note: %d judgment item(s) were not applied (they need a decision; see the plan file).\n",
			len(plan.Judgment))
	}
	return nil
}
