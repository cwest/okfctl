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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newNodeCmd() *cobra.Command {
	node := &cobra.Command{Use: "node", Short: "Author and inspect nodes"}

	var typ, title, dir string
	newC := &cobra.Command{
		Use:   "new <path>",
		Short: "Create a conformant node (type required, §7)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if typ == "" {
				return fmt.Errorf("--type is required (OKF §7: every node needs a non-empty type)")
			}
			// If a template governs this type, scaffold from it (§9.3); otherwise
			// create a plain conformant node (unchanged path).
			created := ""
			if b, err := okf.Load(dir); err == nil {
				if t, ok := okf.Templates(b)[typ]; ok {
					p, err := okf.NewNodeFromTemplate(dir, args[0], typ, title, t)
					if err != nil {
						return err
					}
					created = p
					fmt.Fprintf(cmd.OutOrStdout(), "Created %s (from %s template)\n", p, typ)
				}
			}
			if created == "" {
				p, err := okf.NewNode(dir, args[0], typ, title)
				if err != nil {
					return err
				}
				created = p
				fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", p)
			}
			// Maintain the derived artifacts: record the creation in log.md and
			// regenerate index.md so a new node is never an audit gap and the
			// index never silently drifts. Best-effort: a failure here must not
			// undo an already-written node, so it is reported, not fatal.
			if rel, rerr := bundleRel(dir, created); rerr == nil {
				maintainOnCreate(cmd, dir, rel)
			}
			return nil
		},
	}
	newC.Flags().StringVar(&typ, "type", "", "node type (required)")
	newC.Flags().StringVar(&title, "title", "", "node title")
	newC.Flags().StringVar(&dir, "bundle", ".", "bundle directory")
	node.AddCommand(newC)

	var showBundle string
	showC := &cobra.Command{
		Use:   "show <path>",
		Short: "Print a node, surfacing its type (§7.3)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := okf.Load(showBundle)
			if err != nil {
				return err
			}
			key := args[0]
			if !strings.HasSuffix(key, ".md") {
				key += ".md"
			}
			n, ok := b.Nodes[key]
			if !ok {
				return fmt.Errorf("node not found: %s", key)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "path: %s\ntype: %s\n\n%s\n", n.Path, n.Type(), n.Body)
			return nil
		},
	}
	showC.Flags().StringVar(&showBundle, "bundle", ".", "bundle directory")
	node.AddCommand(showC)

	var listBundle string
	listC := &cobra.Command{
		Use:   "list",
		Short: "List nodes with their type (§7.3)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := okf.Load(listBundle)
			if err != nil {
				return err
			}
			paths := make([]string, 0, len(b.Nodes))
			for p := range b.Nodes {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			for _, p := range paths {
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s %s\n", p, b.Nodes[p].Type())
			}
			return nil
		},
	}
	listC.Flags().StringVar(&listBundle, "bundle", ".", "bundle directory")
	node.AddCommand(listC)

	var mvBundle string
	var mvDry bool
	mvC := &cobra.Command{
		Use:   "mv <old> <new>",
		Short: "Move/rename a node, rewriting inbound links (path is identity)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := okf.Load(mvBundle)
			if err != nil {
				return err
			}
			old, newP := withMD(args[0]), withMD(args[1])
			rewrites, err := okf.PlanMove(b, old, newP)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if mvDry {
				fmt.Fprintf(out, "move %s -> %s\n", old, newP)
				for _, rw := range rewrites {
					fmt.Fprintf(out, "  rewrite %s: %s -> %s\n", rw.NodePath, rw.Old, rw.New)
				}
				return nil
			}
			if err := okf.ApplyMove(mvBundle, b, old, newP, rewrites); err != nil {
				return err
			}
			maintainOnMove(cmd, mvBundle, old, newP)
			fmt.Fprintf(out, "Moved %s -> %s (%d inbound link(s) rewritten)\n", old, newP, len(rewrites))
			return nil
		},
	}
	mvC.Flags().StringVar(&mvBundle, "bundle", ".", "bundle directory")
	mvC.Flags().BoolVar(&mvDry, "dry-run", false, "print the plan without touching disk")
	node.AddCommand(mvC)

	var rmBundle string
	var rmDry bool
	rmC := &cobra.Command{
		Use:   "rm <path>",
		Short: "Remove a node and report resulting orphans",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := okf.Load(rmBundle)
			if err != nil {
				return err
			}
			p := withMD(args[0])
			orphans, err := okf.PlanRemoveOrphans(b, p)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if rmDry {
				fmt.Fprintf(out, "remove %s\n", p)
			} else {
				abs := filepath.Join(rmBundle, filepath.FromSlash(p))
				if err := os.Remove(abs); err != nil {
					return fmt.Errorf("remove %s: %w", p, err)
				}
				maintainOnDelete(cmd, rmBundle, p)
				fmt.Fprintf(out, "Removed %s\n", p)
			}
			for _, o := range orphans {
				fmt.Fprintf(out, "  orphaned: %s\n", o)
			}
			return nil
		},
	}
	rmC.Flags().StringVar(&rmBundle, "bundle", ".", "bundle directory")
	rmC.Flags().BoolVar(&rmDry, "dry-run", false, "print the plan without touching disk")
	node.AddCommand(rmC)

	var editBundle string
	editC := &cobra.Command{
		Use:   "edit <path>",
		Short: "Open a node in $EDITOR, then re-validate on return",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := withMD(args[0])
			if okf.IsReservedPath(p) {
				return fmt.Errorf("cannot edit reserved file: %s", p)
			}
			abs := filepath.Join(editBundle, filepath.FromSlash(p))
			if _, err := os.Stat(abs); err != nil {
				return fmt.Errorf("node not found: %s", p)
			}
			editor := resolveEditor()
			ed := exec.Command(editor, abs)
			ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := ed.Run(); err != nil {
				return fmt.Errorf("editor %q exited: %w", editor, err)
			}
			// Re-validate the whole bundle after the edit.
			b, err := okf.Load(editBundle)
			if err != nil {
				return err
			}
			findings := okf.Validate(b)
			out := cmd.OutOrStdout()
			if len(findings) != 0 {
				for _, f := range findings {
					fmt.Fprintf(out, "%s: %s\n", f.Path, f.Message)
				}
				return fmt.Errorf("%d validation finding(s) after edit", len(findings))
			}
			// The edit went through okfctl and left a valid bundle: refresh the
			// node's modified timestamp (created is left untouched), record the
			// edit in log.md, and regenerate index.md. This is how modified stays
			// honest for the okfctl-mediated edit path — the $EDITOR-only path is
			// what the git drift check exists to catch.
			if err := okf.TouchModifiedFile(abs, nowUTCcmd()); err != nil {
				return fmt.Errorf("refresh modified: %w", err)
			}
			maintainOnEdit(cmd, editBundle, p)
			fmt.Fprintf(out, "%s edited; bundle valid\n", p)
			return nil
		},
	}
	editC.Flags().StringVar(&editBundle, "bundle", ".", "bundle directory")
	node.AddCommand(editC)

	node.AddCommand(newNodeRefreshCmd())
	node.AddCommand(newNodePromoteCmd())
	return node
}

// newNodeRefreshCmd builds `node refresh`, the bulk remediation for the git
// drift the validate/drift check reports: it rewrites each drifting node's
// frontmatter `modified` to its git last-commit day. `created` is never touched,
// the body is preserved verbatim, and log.md/index.md are maintained via the
// same derived-artifact paths as the other node verbs.
//
//	okfctl node refresh <bundle>          # fix every drifting node
//	okfctl node refresh <bundle> <path>   # fix a single node
//	okfctl node refresh --dry-run <bundle>  # list, write nothing, exit 0
//
// It degrades cleanly outside a git repo (no git = no drift = no-op, exit 0) and
// exits non-zero ONLY on a real failure, never on "found drift and fixed it".
func newNodeRefreshCmd() *cobra.Command {
	var dry bool
	var yes bool
	c := &cobra.Command{
		Use:   "refresh <bundle> [path]",
		Short: "Rewrite stale `modified` timestamps to git last-commit (bulk drift fix)",
		Long: "refresh is the remediation for the git drift that validate reports: it " +
			"rewrites each drifting node's frontmatter `modified` to its git last-commit " +
			"day. `created` is immutable and never touched, the Markdown body is preserved " +
			"verbatim, and log.md/index.md are maintained. With a trailing path it fixes a " +
			"single node. --dry-run lists what would change and writes nothing. It degrades " +
			"to a clean no-op outside a git repo, and exits non-zero only on real failure.\n\n" +
			"A plan dominated by a single commit — the signature of a bulk mechanical " +
			"commit, whose remediation would collapse real authoring history into the " +
			"migration date — is REFUSED unless --yes is given. The right fix in that case " +
			"is to list the mechanical commit in " + okf.DriftIgnoreRevsFile + " (like " +
			"`git blame --ignore-revs-file`), so drift walks back to the prior real commit.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}

			var plan []okf.RefreshChange
			if len(args) == 2 {
				plan, err = okf.RefreshPlanNode(b, withMD(args[1]))
				if err != nil {
					return err
				}
			} else {
				plan = okf.RefreshPlan(b)
			}

			out := cmd.OutOrStdout()
			if len(plan) == 0 {
				fmt.Fprintln(out, "No drift: all modified timestamps agree with git.")
				return nil
			}

			if dry {
				for _, ch := range plan {
					fmt.Fprintf(out, "would refresh %s: %s -> %s\n", ch.Path, ch.OldModified, ch.NewModified[:10])
				}
				fmt.Fprintf(out, "%d node(s) would be refreshed (dry run; nothing written)\n", len(plan))
				return nil
			}

			// Guardrail: a plan dominated by one commit is a bulk mechanical
			// commit, not incremental cleanup — refuse to silently flatten its
			// distinct authoring dates. --yes is the explicit escape hatch; the
			// durable cure is to list the commit in .okf-drift-ignore-revs.
			if g := okf.RefreshGuard(plan); g.Triggered && !yes {
				return fmt.Errorf(
					"refusing: %d of %d planned refreshes come from a single commit (%s) — "+
						"this looks like a bulk mechanical commit, and refreshing would collapse "+
						"the real authoring dates it touched into that one date.\n"+
						"  To opt the commit out of drift (recommended), add its SHA to %s in the "+
						"bundle root:\n\n      echo %s >> %s\n\n"+
						"  and re-run — drift will walk back to the prior real commit.\n"+
						"  To refresh anyway (this rewrites the dates), re-run with --yes.",
					g.Count, g.Total, g.Commit, okf.DriftIgnoreRevsFile, g.Commit, okf.DriftIgnoreRevsFile)
			}

			if err := okf.RefreshApply(plan); err != nil {
				return err
			}
			for _, ch := range plan {
				fmt.Fprintf(out, "refreshed %s: %s -> %s\n", ch.Path, ch.OldModified, ch.NewModified[:10])
				logOnRefresh(cmd, dir, ch.Path)
			}
			// Regenerate index.md once after all refreshes (not per node).
			maintainIndex(cmd, dir)
			fmt.Fprintf(out, "%d node(s) refreshed\n", len(plan))
			return nil
		},
	}
	c.Flags().BoolVar(&dry, "dry-run", false, "list what would change and exit 0 without writing")
	c.Flags().BoolVar(&yes, "yes", false, "refresh even when a single commit dominates the plan (overrides the bulk-commit guard)")
	return c
}

// newNodePromoteCmd builds `node promote`, the bulk remediation for the
// directory-as-concept shape validate reports: every NON-ROOT index.md that
// carries frontmatter is moved to a sibling concept file (dir/<basename>.md;
// basename defaults to the directory name, --name overrides it uniformly). The
// body is preserved verbatim and `created` is immutable, matching node refresh.
// Inbound links to the old directory-concept — in BOTH the dir/index.md and
// dir/ spellings — are rewritten to the new concept path, then log.md/index.md
// are maintained via the same derived-artifact paths as the other node verbs.
// The bundle-root index is left alone (its §12 okf_version marker is legal).
//
//	okfctl node promote <bundle>                 # promote every dir-concept index
//	okfctl node promote <bundle> --name overview # pick one basename convention
//	okfctl node promote <bundle> --dry-run       # list moves + rewrites, write nothing
//
// --dry-run lists every move and inbound-link rewrite and writes ZERO bytes; it
// exits non-zero only on a real failure, never on "found promotable indexes".
func newNodePromoteCmd() *cobra.Command {
	var dry bool
	var name string
	c := &cobra.Command{
		Use:   "promote <bundle>",
		Short: "Promote directory-as-concept index.md files to sibling concept files (bulk remediation)",
		Long: "promote remediates the directory-as-concept shape validate reports: every " +
			"non-root index.md that carries frontmatter is moved to a sibling concept file " +
			"(dir/<basename>.md; basename defaults to the directory name, --name overrides it). " +
			"The body is preserved verbatim and `created` is immutable, matching node refresh. " +
			"Inbound links to the old directory-concept are rewritten (both the dir/index.md and " +
			"dir/ spellings), the real index.md is regenerated with no frontmatter, and log.md is " +
			"appended. The bundle-root index is left alone. --dry-run lists every move and rewrite " +
			"and writes nothing.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}

			plan, err := okf.PromotePlan(b, name)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(plan) == 0 {
				fmt.Fprintln(out, "No directory-as-concept indexes to promote.")
				return nil
			}

			if dry {
				for _, ch := range plan {
					fmt.Fprintf(out, "would promote %s -> %s\n", ch.OldPath, ch.NewPath)
					for _, rw := range ch.Rewrites {
						fmt.Fprintf(out, "  rewrite %s: %s -> %s\n", rw.NodePath, rw.Old, rw.New)
					}
				}
				fmt.Fprintf(out, "%d index(es) would be promoted (dry run; nothing written)\n", len(plan))
				return nil
			}

			if err := okf.PromoteApply(dir, b, plan); err != nil {
				return err
			}
			for _, ch := range plan {
				fmt.Fprintf(out, "promoted %s -> %s (%d inbound link(s) rewritten)\n",
					ch.OldPath, ch.NewPath, len(ch.Rewrites))
				logOnPromote(cmd, dir, ch.OldPath, ch.NewPath)
			}
			// Regenerate index.md once after all promotions (not per node), so the
			// old directory indexes are rebuilt as clean, frontmatter-free
			// navigation surfaces.
			maintainIndex(cmd, dir)
			fmt.Fprintf(out, "%d index(es) promoted\n", len(plan))
			return nil
		},
	}
	c.Flags().BoolVar(&dry, "dry-run", false, "list what would change and exit 0 without writing")
	c.Flags().StringVar(&name, "name", "", "basename for every promoted concept file (default: the directory name)")
	return c
}

// resolveEditor picks the editor command: $OKFCTL_EDITOR, then $VISUAL, then
// $EDITOR, then a sensible default (vi).
func resolveEditor() string {
	for _, env := range []string{"OKFCTL_EDITOR", "VISUAL", "EDITOR"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return "vi"
}

// withMD appends the .md extension when the caller omitted it, matching the
// other node subcommands' path handling.
func withMD(p string) string {
	if !strings.HasSuffix(p, ".md") {
		return p + ".md"
	}
	return p
}
