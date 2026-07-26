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
	return node
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
