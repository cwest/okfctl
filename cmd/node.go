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
	node := &cobra.Command{Use: "node", Short: "Create, move, remove, edit, and read concept nodes"}

	var typ, title, dir, templatesFrom string
	newC := &cobra.Command{
		Use:   "new <path>",
		Short: "Create a concept node, then update log.md and index.md",
		Long: "new writes a concept node at <path> in the bundle --bundle names (the current " +
			"directory by default). The .md suffix is optional. You have to pass --type (a file " +
			"with no type isn't a concept, OKF §4.1, §3.2). Any value works. The spec " +
			"leaves the list of types open. The node gets that type and created and modified " +
			"timestamps, plus the --title if you gave one. With no title, the heading is the " +
			"file name.\n\n" +
			"Then new adds \"created <path>\" to log.md and rebuilds each index.md, and that's " +
			"why `okfctl index check` passes right after. If the --bundle directory is missing, " +
			"new makes it and declares no okf_version.\n\n" +
			"With a type template for --type in the bundle, the node starts from that template's " +
			"fields and body headings (PRD §9.3). Templates live in the bundle itself or in " +
			"another bundle named by the .okf templates key or --templates-from (PRD §9.2). Run " +
			"`okfctl template list` to see them.\n\n" +
			"Overwriting a node, writing outside the bundle and a path that starts with a dot " +
			"all exit 1.\n\n" +
			"To edit the node in your editor, use `okfctl node edit`.",
		Annotations: map[string]string{
			transcriptAnnotation: "<!-- transcript: exit 1 okfctl node new tannin --bundle shelf -->",
		},
		Example: "  $ okfctl bundle init shelf\n" +
			"  Initialized OKF bundle in shelf\n" +
			"  $ okfctl node new tannin --bundle shelf\n" +
			"  okfctl: --type is required (OKF §4.1 / §11: every node needs a non-empty type)\n" +
			"  $ okfctl node new tannin --type Concept --title Tannin --bundle shelf\n" +
			"  Created shelf/tannin.md\n" +
			"  $ cat shelf/tannin.md\n" +
			"  ---\n" +
			"  type: Concept\n" +
			"  title: Tannin\n" +
			"  created: <TIMESTAMP>\n" +
			"  modified: <TIMESTAMP>\n" +
			"  ---\n" +
			"\n" +
			"  # Tannin\n" +
			"  $ okfctl index check shelf\n" +
			"  OK: index.md is current",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if typ == "" {
				return fmt.Errorf("--type is required (OKF §4.1 / §11: every node needs a non-empty type)")
			}
			// If a template governs this type, scaffold from it (PRD §9.3); otherwise
			// create a plain conformant node (unchanged path). Templates resolve
			// through the PRD §9.2 reference overlay: a referenced bundle folded in,
			// local Type Template nodes overlaid (local wins). A broken reference
			// is a hard error — the author asked for the overlay via the key/flag.
			created := ""
			if b, err := okf.Load(dir); err == nil {
				rt, rerr := okf.ResolveTemplates(b, templatesFrom)
				if rerr != nil {
					return rerr
				}
				if t, ok := rt.ByType[typ]; ok {
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
	newC.Flags().StringVar(&typ, "type", "", "type to assign the new node (required; any non-empty value, OKF §4.1)")
	newC.Flags().StringVar(&title, "title", "", "title for the new node (omitted from frontmatter when empty)")
	newC.Flags().StringVar(&dir, "bundle", ".", "bundle directory to operate on")
	newC.Flags().StringVar(&templatesFrom, "templates-from", "", templatesFromFlagUsage)
	node.AddCommand(newC)

	var showBundle string
	showC := &cobra.Command{
		Use:   "show <path>",
		Short: "Print one node's path, type, and body",
		Long: "show prints a node's path, its type and its Markdown body from the bundle --bundle " +
			"names (the current directory by default). The .md suffix is optional. The type gets " +
			"a line of its own: you see what kind of concept you're reading before the body " +
			"starts (PRD §7.3). The rest of the frontmatter (title, timestamps, sources) stays " +
			"hidden. cat the file for that.\n\n" +
			"show never writes. Any path that isn't a concept node exits 1 with \"node not " +
			"found\", index.md and log.md included. The spec says neither is ever a concept " +
			"(§3.1).",
		Example: "  $ okfctl bundle init reader\n" +
			"  Initialized OKF bundle in reader\n" +
			"  $ okfctl node new wine/oak --type Concept --title Oak --bundle reader\n" +
			"  Created reader/wine/oak.md\n" +
			"  $ echo 'Oak barrels add tannin.' >> reader/wine/oak.md\n" +
			"  $ okfctl node show wine/oak --bundle reader\n" +
			"  path: wine/oak.md\n" +
			"  type: Concept\n" +
			"\n" +
			"  # Oak\n" +
			"  Oak barrels add tannin.\n" +
			"\n" +
			"  $ okfctl node show index --bundle reader\n" +
			"  okfctl: node not found: index.md",
		Annotations: map[string]string{
			transcriptAnnotation: "<!-- transcript: exit 1 okfctl node show index --bundle reader -->",
		},
		Args: cobra.ExactArgs(1),
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
	showC.Flags().StringVar(&showBundle, "bundle", ".", "bundle directory to operate on")
	node.AddCommand(showC)

	var listBundle string
	listC := &cobra.Command{
		Use:   "list",
		Short: "List every concept node with its type, sorted by path",
		Long: "list prints one line per concept node in the bundle given by --bundle (the current " +
			"directory by default), sorted by path, with the node's type in the second column " +
			"(PRD §7.3). Use it to see what a bundle holds, or to find nodes of one type with " +
			"grep.\n\n" +
			"index.md and log.md never appear (OKF §3.1). Neither does a .md file with no type, " +
			"such as a README.md. The spec doesn't count those as concepts (§3.2), and " +
			"`okfctl validate` warns about them. list only reads.",
		Example: "  $ okfctl bundle init catalog\n" +
			"  Initialized OKF bundle in catalog\n" +
			"  $ okfctl node new wine/tannin --type Concept --bundle catalog\n" +
			"  Created catalog/wine/tannin.md\n" +
			"  $ okfctl node new grape/sangiovese --type 'Grape Variety' --bundle catalog\n" +
			"  Created catalog/grape/sangiovese.md\n" +
			"  $ echo 'Notes for editors.' > catalog/README.md\n" +
			"  $ okfctl node list --bundle catalog\n" +
			"  grape/sangiovese.md                      Grape Variety\n" +
			"  wine/tannin.md                           Concept",
		Args: cobra.NoArgs,
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
	listC.Flags().StringVar(&listBundle, "bundle", ".", "bundle directory to operate on")
	node.AddCommand(listC)

	var mvBundle string
	var mvDry bool
	mvC := &cobra.Command{
		Use:   "mv <old> <new>",
		Short: "Move or rename a node and rewrite every link that points at it",
		Long: "mv takes the node at <old> to <new> in the bundle --bundle names (the current " +
			"directory by default, with the .md suffix optional on either path). Links find their " +
			"target by path (OKF §6). That's why mv rewrites each link in the bundle that pointed " +
			"at the old path, README files included: one that started with / still starts with " +
			"/ while a relative one gets worked out again from its own file. Then mv adds \"moved " +
			"<old> -> <new>\" to log.md and rebuilds each index.md.\n\n" +
			"Try --dry-run first.\n\n" +
			"It prints the move and each rewrite with nothing written. The links in the moved " +
			"node stay as written. Relative ones that worked from the old directory can " +
			"break from the new one. `okfctl lint` reports them as broken-link.\n\n" +
			"With a file at <new>, mv exits 1 and changes nothing. It won't move index.md " +
			"or log.md, nor put a node at either name.",
		Example: "  $ okfctl bundle init map\n" +
			"  Initialized OKF bundle in map\n" +
			"  $ okfctl node new wine/tannin --type Concept --title Tannin --bundle map\n" +
			"  Created map/wine/tannin.md\n" +
			"  $ okfctl node new wine/oak --type Concept --title Oak --bundle map\n" +
			"  Created map/wine/oak.md\n" +
			"  $ echo 'Oak barrels add [tannin](tannin.md).' >> map/wine/oak.md\n" +
			"  $ okfctl node mv wine/tannin wine/tannins --dry-run --bundle map\n" +
			"  move wine/tannin.md -> wine/tannins.md\n" +
			"    rewrite wine/oak.md: tannin.md -> tannins.md\n" +
			"  $ okfctl node mv wine/tannin wine/tannins --bundle map\n" +
			"  Moved wine/tannin.md -> wine/tannins.md (1 inbound link(s) rewritten)\n" +
			"  $ tail -n 1 map/wine/oak.md\n" +
			"  Oak barrels add [tannin](tannins.md).",
		Args: cobra.ExactArgs(2),
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
	mvC.Flags().StringVar(&mvBundle, "bundle", ".", "bundle directory to operate on")
	mvC.Flags().BoolVar(&mvDry, "dry-run", false, "print the plan without touching disk")
	node.AddCommand(mvC)

	var rmBundle string
	var rmDry bool
	rmC := &cobra.Command{
		Use:   "rm <path>",
		Short: "Delete a node and list the nodes it leaves with no inbound links",
		Long: "rm deletes the node at <path> from the bundle --bundle names (the current directory " +
			"by default). The .md suffix is optional. Then it adds \"removed <path>\" to log.md, " +
			"rebuilds each index.md and prints the orphans: nodes the deleted one linked to that " +
			"nothing else links to now (OKF §6.4). In the example oak was tannin's only link.\n\n" +
			"Run it with --dry-run first to print the same list and leave the node in place.\n\n" +
			"Links that pointed at the deleted node stay as they were. The spec allows a broken " +
			"link (§6.1, §11.2), and afterward validate and lint pass. Only `okfctl analyze` " +
			"mentions them, as dangling forward-links. To find them yourself, grep the bundle " +
			"for the old path first.\n\n" +
			"rm won't delete index.md or log.md (§3.1); it exits 1.",
		Example: "  $ okfctl bundle init rack\n" +
			"  Initialized OKF bundle in rack\n" +
			"  $ okfctl node new tannin --type Concept --bundle rack\n" +
			"  Created rack/tannin.md\n" +
			"  $ okfctl node new oak --type Concept --bundle rack\n" +
			"  Created rack/oak.md\n" +
			"  $ echo 'Oak barrels add [tannin](tannin.md).' >> rack/oak.md\n" +
			"  $ okfctl node rm oak --dry-run --bundle rack\n" +
			"  remove oak.md\n" +
			"    orphaned: tannin.md\n" +
			"  $ okfctl node rm oak --bundle rack\n" +
			"  Removed oak.md\n" +
			"    orphaned: tannin.md",
		Args: cobra.ExactArgs(1),
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
	rmC.Flags().StringVar(&rmBundle, "bundle", ".", "bundle directory to operate on")
	rmC.Flags().BoolVar(&rmDry, "dry-run", false, "print the plan without touching disk")
	node.AddCommand(rmC)

	var editBundle string
	editC := &cobra.Command{
		Use:   "edit <path>",
		Short: "Open a node in your editor, then validate and log the change",
		Long: "edit opens the node at <path> in your editor and waits for it to close. The editor " +
			"is $OKFCTL_EDITOR, then $VISUAL, then $EDITOR (vi if none is set). The .md suffix " +
			"is optional. In the example the editor is true: it exits right away.\n\n" +
			"After the editor closes, edit validates the whole bundle (OKF §11.1) and, if that " +
			"passes with the node's type still set, sets modified to now, adds \"edited <path>\" " +
			"to log.md and rebuilds each index.md. created stays as it was. A node changed in " +
			"your editor without okfctl keeps its old modified date. validate reports that later " +
			"as git drift.\n\n" +
			"A failed check prints the findings and exits 1. So does an emptied type (the file " +
			"is no longer a concept, §3.2). Either way your change stays on disk and nothing " +
			"else is touched. Fix it and run edit again.\n\n" +
			"The variable's whole value is taken as one program name. OKFCTL_EDITOR=\"code " +
			"--wait\" fails. Put the arguments in a short script and name the script there.\n\n" +
			"edit won't open index.md or log.md.",
		Example: "  $ okfctl bundle init desk\n" +
			"  Initialized OKF bundle in desk\n" +
			"  $ okfctl node new tannin --type Concept --bundle desk\n" +
			"  Created desk/tannin.md\n" +
			"  $ OKFCTL_EDITOR=true okfctl node edit tannin --bundle desk\n" +
			"  tannin.md edited; bundle valid\n" +
			"  $ okfctl log show desk\n" +
			"  # Change Log\n" +
			"\n" +
			"  ## <DATE>\n" +
			"  * edited tannin.md\n" +
			"  * created tannin.md",
		Args: cobra.ExactArgs(1),
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
			// Launching the user's configured $EDITOR on the node file is the
			// entire purpose of `node edit`; the variable command is the feature.
			ed := exec.Command(editor, abs) //nolint:gosec // G204: invoking the user's own $EDITOR is the command's purpose
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
			// The bundle can still conform while the edited node has stopped
			// being a concept: an emptied or removed `type` makes it a §3.2
			// rule 2 non-concept that every consumer skips. An okfctl-mediated
			// edit must not silently drop a node out of the bundle, so refuse
			// to bless it (no timestamp refresh, no log entry, no index).
			if _, ok := b.NonConcepts[p]; ok {
				fmt.Fprintf(out, "%s: no longer a concept: it has no non-empty type, so every consumer skips it (OKF v0.3 §3.2)\n", p)
				return fmt.Errorf("%s is no longer a concept after edit", p)
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
	editC.Flags().StringVar(&editBundle, "bundle", ".", "bundle directory to operate on")
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
		Short: "Set modified to each node's last git commit date",
		Long: "refresh fixes the git drift `okfctl validate` warns about: a node whose modified " +
			"timestamp disagrees with the day git last committed it. Each drifting node gets that " +
			"commit day as its modified date and a \"refreshed <path>\" line in log.md. The index " +
			"is rebuilt once at the end. created and the body don't change. Put a path after the " +
			"bundle to fix one node.\n\n" +
			"The example commits a node created today with a January date. validate warns. " +
			"--dry-run lists the change and writes nothing. After refresh makes it, validate " +
			"passes. Outside a git repository nothing drifts, and refresh exits 0 having done " +
			"nothing.\n\n" +
			"A bulk commit (a migration, a reformat) is the case to watch. Refreshing from it " +
			"would stamp its date on every node it touched and wipe out their real history. So " +
			"refresh stops when one commit accounts for half or more of a plan of 10 or more " +
			"changes: it exits 1 and prints the commit's SHA. Add that SHA to " +
			okf.DriftIgnoreRevsFile + " in the bundle root (it works like git blame " +
			"--ignore-revs-file) and drift looks past the commit to the one before. --yes " +
			"refreshes anyway.",
		Example: "  $ okfctl bundle init ledger\n" +
			"  Initialized OKF bundle in ledger\n" +
			"  $ okfctl node new tannin --type Concept --bundle ledger\n" +
			"  Created ledger/tannin.md\n" +
			"  $ git -C ledger init -q && git -C ledger add .\n" +
			"  $ GIT_COMMITTER_DATE=2026-01-15T12:00:00Z git -C ledger -c user.name=me -c user.email=me@example.com commit -q -m import\n" +
			"  $ okfctl validate ledger\n" +
			"  warning tannin.md: modified <DATE> disagrees with git last-commit 2026-01-15; run `okfctl node refresh` to fix it (or `okfctl node edit` per node)\n" +
			"  1 drift warning(s)\n" +
			"  $ okfctl node refresh --dry-run ledger\n" +
			"  would refresh tannin.md: <DATE> -> 2026-01-15\n" +
			"  1 node(s) would be refreshed (dry run; nothing written)\n" +
			"  $ okfctl node refresh ledger\n" +
			"  refreshed tannin.md: <DATE> -> 2026-01-15\n" +
			"  1 node(s) refreshed\n" +
			"  $ okfctl validate ledger\n" +
			"  OK: bundle conforms to the OKF spec floor",
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
// The bundle-root index is left alone (its §8.1 bundle metadata is legal).
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
		Short: "Turn each subdirectory index.md that holds a concept into its own node",
		Long: "promote fixes a layout validate fails: a concept written as a subdirectory's " +
			"index.md. OKF §8 says an index file there has no frontmatter. promote moves each " +
			"one to a node beside it named after the directory (grape/index.md becomes " +
			"grape/grape.md), or to the one name --name gives. Links to the old place get " +
			"rewritten in both spellings, grape/index.md and grape/. One log.md line goes in per " +
			"move. Then the index is rebuilt and grape/index.md is a generated list again.\n\n" +
			"--dry-run prints each move and rewrite with nothing written.\n\n" +
			"Run promote before anything rebuilds the index. index build and the node commands " +
			"that rebuild when they finish delete a subdirectory index.md, frontmatter and all, " +
			"once its directory holds no other node. The body and created carry over as they " +
			"were. The root index.md isn't touched; §8.1 lets it hold bundle metadata. A taken " +
			"target name makes promote exit 1 with nothing changed.",
		Annotations: map[string]string{
			transcriptAnnotation: "<!-- transcript: exit 1 okfctl validate handbook -->",
		},
		Example: "  $ okfctl bundle init handbook\n" +
			"  Initialized OKF bundle in handbook\n" +
			"  $ mkdir handbook/grape\n" +
			"  $ printf -- '---\\ntype: Concept\\ntitle: Grape\\n---\\n\\nThe fruit.\\n' > handbook/grape/index.md\n" +
			"  $ printf -- '---\\ntype: Concept\\ntitle: Tannin\\n---\\n\\nTannin comes from [grape](grape/) skins.\\n' > handbook/tannin.md\n" +
			"  $ okfctl validate handbook\n" +
			"  FAIL grape/index.md: index files contain no frontmatter (§8); only the bundle-root index may carry frontmatter (§8.1)\n" +
			"  okfctl: 1 conformance finding(s)\n" +
			"  $ okfctl node promote --dry-run handbook\n" +
			"  would promote grape/index.md -> grape/grape.md\n" +
			"    rewrite tannin.md: grape/ -> grape/grape.md\n" +
			"  1 index(es) would be promoted (dry run; nothing written)\n" +
			"  $ okfctl node promote handbook\n" +
			"  promoted grape/index.md -> grape/grape.md (1 inbound link(s) rewritten)\n" +
			"  1 index(es) promoted\n" +
			"  $ tail -n 1 handbook/tannin.md\n" +
			"  Tannin comes from [grape](grape/grape.md) skins.\n" +
			"  $ cd handbook && okfctl validate .\n" +
			"  OK: bundle conforms to the OKF spec floor",
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
