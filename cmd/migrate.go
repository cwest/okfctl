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
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// newMigrateCmd builds `okfctl migrate`, the two-phase upgrade path from
// OKF v0.1 or v0.2 (or an undeclared version) to v0.3. It is deliberately consumer-agnostic and carries NO model, network, or
// credential dependency on any path (the tool ships to strangers who have a
// bundle and nothing else):
//
//	okfctl migrate <bundle> --plan migrate-plan.json --generated-by <actor>
//	     phase 1 (default): PURE READ. Computes every deterministic edit and
//	     enumerates every judgment item, writing only the plan file.
//
//	okfctl migrate <bundle> --apply --plan migrate-plan.json [--dry-run]
//	     phase 2: reads the plan back, applies its deterministic edits
//	     (order-preserving, additive-only), then re-validates. --dry-run writes
//	     nothing and prints the unified diff the real apply would write.
//
// The plan file is the review surface, for Casey and for a solo user alike. A
// judgment item (a citation with no follow-able resource per §5.1, or a
// timestamp rename with no actor per §7) is NEVER guessed — it stays in the plan
// for its (unspecified) consumer to resolve.
func newMigrateCmd() *cobra.Command {
	var apply, dry, force bool
	var planPath, generatedBy, resolvePath string
	c := &cobra.Command{
		Use:   "migrate <bundle>",
		Short: "Upgrade a v0.1 or v0.2 bundle to OKF v0.3: plan, preview, apply",
		Long: "migrate moves a bundle that declares OKF v0.1 or v0.2 (or none) " +
			"to v0.3 in place. Run it twice. The first run writes a plan file and changes " +
			"nothing else. The next one, with --apply, makes those edits and declares " +
			"okf_version \"0.3\" in the bundle-root index.md (§8.1). Then it validates the " +
			"result. With --dry-run, apply prints the diff it would write and writes nothing.\n\n" +
			"A rule makes each edit. For a v0.1 bundle (Appendix B), timestamp turns into " +
			"generated.at. Any body # Citations entry that names one follow-able resource " +
			"is copied to the sources list, titled with the cited work's name when the entry " +
			"marks one (never the full entry text, §5.1). An entry with no " +
			"URL but one DOI, arXiv id, PubMed id, or ISBN (checksum verified) gets a " +
			"resource built from it, offline: doi.org, arxiv.org, pubmed.ncbi.nlm.nih.gov, " +
			"or openlibrary.org. Nothing is fetched to check it. Each converted entry's " +
			"source gets an id (its plain key, else its list number, else cite-<n>; unique " +
			"with case folded), the entry's full text becomes the footnote definition " +
			"[^id], and body [k] references become [^id] (§5.1, §4.2). A source title that " +
			"holds its whole entry is cut to the work's name, or removed when the entry " +
			"names none, once that footnote holds the text. A title that opens with the " +
			"entry's [key] but was annotated past the entry's text has that text added " +
			"to the footnote first, then is cut the same way. An entry that names " +
			"two or more resources is split into one source per resource (a code-span URL " +
			"counts only when the entry names no resource outside code and the span is that URL alone), " +
			"with ids <id>, <id>-2, <id>-3; the first holds the full entry text as its " +
			"footnote, a later one its link text or resource, and each body [k] becomes " +
			"[^id][^id-2]... A resource that two entries name stays one source: the first " +
			"entry that names it alone owns it (else the first that splits), and every " +
			"other entry's [k] cites that source's id too; an entry splits into the parts " +
			"it owns, and its first owned part holds its text. Once every entry's text opens the footnote definition of one of its " +
			"sources (alone, or followed by a moved title annotation) and every resource it names is a source's resource, the body # Citations " +
			"list is removed (§4.2); otherwise it stays. The v0.2 changes " +
			"(§13.1) give a leading / to any link or path field that resolves only from the " +
			"bundle root (§12.1). It then points at the same file under v0.3 (§6). They " +
			"also respell a §5 timestamp that has an offset but isn't in RFC 3339 form.\n\n" +
			"Judgment items are the cases no rule can settle. prose-citation marks a " +
			"citation with no resource, or whose every resource another entry owns, so it " +
			"names no work of its own (§5.1). missing-actor marks a timestamp " +
			"rename with no actor (§7). timestamp-no-offset is a date-time with no offset " +
			"(§5, §13.2), and timestamp-invalid is a §5 value where no date can be found. " +
			"source-title-unnamed marks an entry whose untitled source gets no name, " +
			"source-title-conflict one that names a different work than its source's title, " +
			"and source-title-unpreserved a whole-entry title kept because no footnote " +
			"holds its text. " +
			"citation-ref-unclear marks a body [k] near the word \"brief\", \"citation\", or " +
			"\"node\", or a link, in the same paragraph, list item, or table row, or the label of " +
			"an undefined [text][k], or a [k] that opens a line before a colon ([^k]: would be a " +
			"footnote definition); citation-unattached a second entry for one resource, and " +
			"source-id-collision a node whose ids would collide. citations-kept marks a " +
			"# Citations list that removing would lose something from, and names each entry " +
			"and what it would lose. " +
			"All of them wait for you to decide them. To resolve a prose-citation item, write a " +
			"\"resource\" (an absolute URL, a bundle path, or a plain-words scope descriptor, §5.1), " +
			"and optionally a \"title\", on the item in the plan file, and plan again with " +
			"--resolve <that plan>: the entry converts like any other. Apply refuses a plan whose " +
			"items carry a resource it has not planned. While a prose-citation or missing-actor " +
			"item is left, apply holds the version: those items need the v0.1 " +
			"fallbacks §12.1 grants only to bundles declaring 0.2 or older (or nothing). " +
			"--force-declare on the first run overrides the hold.\n\n" +
			"migrate never refuses a bundle on v0.3, on a later version, or on one " +
			"okfctl doesn't know. It never rewrites one to an older version (§12). No step " +
			"needs a model.",
		Example: "  # Write the plan; nothing in the bundle changes\n" +
			"  okfctl migrate ./mykb --plan migrate-plan.json --generated-by human:casey\n\n" +
			"  # See the diff the apply would write, without writing it\n" +
			"  okfctl migrate ./mykb --apply --plan migrate-plan.json --dry-run\n\n" +
			"  # Apply the plan, declare v0.3 if nothing is held, and validate\n" +
			"  okfctl migrate ./mykb --apply --plan migrate-plan.json\n\n" +
			"  # After writing a resource on prose-citation items in the plan, plan them\n" +
			"  okfctl migrate ./mykb --plan migrate-plan.json --resolve migrate-plan.json",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if planPath == "" {
				return fmt.Errorf("--plan <path> is required (the migration plan file)")
			}
			if apply {
				if resolvePath != "" {
					return fmt.Errorf("--resolve plans; it does not go with --apply (plan with --resolve first, then apply that plan)")
				}
				return runMigrateApply(cmd, dir, planPath, dry)
			}
			opts := okf.MigrateOptions{GeneratedBy: generatedBy, ForceDeclare: force}
			if resolvePath != "" {
				rs, err := readResolutions(resolvePath)
				if err != nil {
					return err
				}
				opts.Resolutions = rs
			}
			return runMigratePlan(cmd, dir, planPath, opts)
		},
	}
	c.Flags().BoolVar(&apply, "apply", false, "phase 2: read the plan and apply it (default is phase 1: plan only)")
	c.Flags().BoolVar(&dry, "dry-run", false, "with --apply: write nothing; print the diff the real apply would write")
	c.Flags().StringVar(&planPath, "plan", "migrate-plan.json", "migration plan file (written in phase 1, read in phase 2)")
	c.Flags().StringVar(&generatedBy, "generated-by", "", "actor (§7) recorded as generated.by for each timestamp rename")
	c.Flags().StringVar(&resolvePath, "resolve", "", "phase 1: plan file whose prose-citation items carry a resource (and optional title) to convert those entries with; may be the --plan file itself")
	c.Flags().BoolVar(&force, "force-declare", false, "phase 1: declare v0.3 even while prose-citation or missing-actor items remain (they lose the §12.1 fallback)")
	return c
}

// runMigratePlan is phase 1: compute the plan (pure read) and write it to disk.
func runMigratePlan(cmd *cobra.Command, dir, planPath string, opts okf.MigrateOptions) error {
	b, err := okf.Load(dir)
	if err != nil {
		return fmt.Errorf("load bundle: %w", err)
	}
	plan, err := okf.PlanMigrationWith(b, opts)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("encode plan: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(planPath, data, 0o644); err != nil { //nolint:gosec // G306: migration plan is a shareable bundle artifact; 0o644 is intended
		return fmt.Errorf("write plan %s: %w", planPath, err)
	}
	out := cmd.OutOrStdout()
	det := 0
	for _, nm := range plan.Nodes {
		det += nm.EditCount()
	}
	fmt.Fprintf(out, "Wrote %s: %d file(s) with deterministic edits (%d edit(s)), %d judgment item(s).\n",
		planPath, len(plan.Nodes), det, len(plan.Judgment))
	if plan.Resolved > 0 {
		fmt.Fprintf(out, "resolved: %d prose-citation item(s) from --resolve, planned as sources.\n", plan.Resolved)
	}
	writeMigrateSummary(out, plan)
	if len(plan.Judgment) > 0 {
		fmt.Fprintf(out, "%d item(s) need a decision before they can migrate; see the plan file.\n", len(plan.Judgment))
	}
	return nil
}

// runMigrateApply is phase 2: read the plan back and apply it, then re-validate.
// --dry-run writes nothing.
func runMigrateApply(cmd *cobra.Command, dir, planPath string, dry bool) error {
	planData, err := os.ReadFile(planPath) //nolint:gosec // G304: reading the migration plan the tool just wrote is intended
	if err != nil {
		return fmt.Errorf("read plan %s: %w", planPath, err)
	}
	var plan okf.MigratePlan
	if err := json.Unmarshal(planData, &plan); err != nil {
		return fmt.Errorf("parse plan %s: %w", planPath, err)
	}
	// Apply only writes what a plan planned. A resource written on a
	// judgment item is a resolution no plan has made into edits yet;
	// applying past it would drop it without a word.
	if n := countResolutions(plan.Judgment); n > 0 {
		return fmt.Errorf("%s: %d judgment item(s) carry a resource that is not planned yet; "+
			"plan them first: okfctl migrate %s --plan %s --resolve %s", planPath, n, dir, planPath, planPath)
	}
	b, err := okf.Load(dir)
	if err != nil {
		return fmt.Errorf("load bundle: %w", err)
	}
	out := cmd.OutOrStdout()
	if dry {
		return printMigrateDryRun(out, dir, b, plan)
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
	version := plan.TargetVersion
	switch plan.VersionAction {
	case okf.VersionKeep:
		version = b2.OkfVersion // §12: left at the version it declares
	case okf.VersionHold:
		version = b2.DeclaredOkfVersion
	}
	if plan.VersionAction == okf.VersionHold {
		fmt.Fprintf(out, "%d file(s) migrated; okf_version declaration held: %s.\n", len(plan.Nodes), heldVersionLabel(version))
	} else {
		fmt.Fprintf(out, "%d file(s) migrated; bundle declares v%s.\n", len(plan.Nodes), version)
	}
	if len(plan.Nodes) > 0 {
		fmt.Fprintf(out, "hint: after committing, list that commit's SHA in %s so strict drift skips the mechanical edits (like git blame --ignore-revs-file).\n",
			okf.DriftIgnoreRevsFile)
	}
	if len(findings) != 0 {
		for _, f := range findings {
			fmt.Fprintf(out, "%s: %s\n", f.Path, f.Message)
		}
		return fmt.Errorf("%d validation finding(s) after migrate", len(findings))
	}
	if plan.VersionAction == okf.VersionHold {
		fmt.Fprintln(out, "bundle valid")
	} else {
		fmt.Fprintln(out, "bundle valid as v"+version)
	}
	if len(plan.Judgment) > 0 {
		fmt.Fprintf(out, "note: %d judgment item(s) were not applied (they need a decision; see the plan file).\n",
			len(plan.Judgment))
	}
	return nil
}

// heldVersionLabel names the version a held bundle still declares.
func heldVersionLabel(declared string) string {
	if declared == "" {
		return "bundle is still undeclared (v0.2 fallbacks apply, §12.1)"
	}
	return "bundle still declares v" + declared + " (v0.2 fallbacks apply, §12.1)"
}

// readResolutions reads the judgment items of the plan file at path: the
// prose-citation items a person resolved by writing a resource on them.
func readResolutions(path string) ([]okf.JudgmentItem, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: reading the migration plan the user named is intended
	if err != nil {
		return nil, fmt.Errorf("read --resolve %s: %w", path, err)
	}
	var plan okf.MigratePlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse --resolve %s: %w", path, err)
	}
	return plan.Judgment, nil
}

// countResolutions counts the items that carry a resource.
func countResolutions(items []okf.JudgmentItem) int {
	n := 0
	for _, j := range items {
		if strings.TrimSpace(j.Resource) != "" {
			n++
		}
	}
	return n
}
