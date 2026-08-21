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
	"path/filepath"
	"sort"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// newNodeVerifyCmd builds `node verify`, the write-side companion of the §5.2
// `verified` reader: it APPENDS a verification event { by, at } to a node's
// frontmatter, closing the read/write asymmetry (okfctl could read `verified`
// but never write it).
//
//	okfctl node verify <bundle> <path> --by <actor>   # stamp one node
//	okfctl node verify <bundle> --by <actor> --all    # bulk PLAN (dry-run)
//	okfctl node verify <bundle> --by <actor> --all --write  # bulk stamp
//
// The safety posture is the substance of the command:
//
//   - --by is REQUIRED, has no default, and is never inferred from git config or
//     anywhere else, and is validated against the §7 actor forms. A tool that
//     guesses at who is a tool that manufactures trust.
//   - Bulk mode is DRY-RUN by default and reports its skips; --write opts into
//     the actual stamping.
//   - Stamping the whole corpus in one invocation is REFUSED without --all,
//     because a bulk rubber-stamp converts a trust signal into noise.
//   - Writes go through okf.AppendVerifiedFile (append-only; created/modified and
//     prior verified entries are never modified), and log.md/index.md are
//     maintained via the same derived-artifact paths as the other node verbs.
func newNodeVerifyCmd() *cobra.Command {
	var by string
	var all bool
	var write bool
	c := &cobra.Command{
		Use:   "verify <bundle> [path] --by <actor>",
		Short: "Append a §5.2 verification stamp asserting a node was checked",
		Long: "verify appends a §5.2 `verified` event { by, at } to a node's frontmatter. The " +
			"stamp asserts that a HUMAN or a NAMED PROCESS actually CHECKED the node against its " +
			"sources — it is a trust signal, not a mechanical touch, so it is never fabricated. " +
			"--by is REQUIRED, has no default, and is NEVER inferred from git config or anywhere " +
			"else; it is validated against the §7 actor forms (`human:<id>` for a person, " +
			"`process:<id>` for an automated process, or `<producer>/<version>` for a tool). " +
			"The event is APPENDED — an existing `verified` list is extended and a prior entry " +
			"is never modified, because §5.2 models verification history AS history; `created` " +
			"and `modified` are never touched. With a trailing path it stamps a single node. " +
			"Without a path it targets the WHOLE bundle, which is REFUSED without --all because " +
			"a bulk rubber-stamp converts a trust signal into noise; bulk mode is a dry-run plan " +
			"by default (reporting which nodes it would skip) and writes only with --write. " +
			"Writes go through the order- and body-preserving frontmatter writer, and " +
			"log.md/index.md are maintained.",
		Example: "  # Stamp a single node with a human verifier\n" +
			"  okfctl node verify ./bundles/knowledge concepts/revenue --by human:casey\n\n" +
			"  # Stamp a node as checked by a named process\n" +
			"  okfctl node verify ./bundles/knowledge concepts/revenue --by process:finance-nightly\n\n" +
			"  # Plan a whole-bundle verify (dry-run; writes nothing)\n" +
			"  okfctl node verify ./bundles/knowledge --by human:casey --all\n\n" +
			"  # Actually stamp the whole bundle (explicit override + write)\n" +
			"  okfctl node verify ./bundles/knowledge --by human:casey --all --write",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --by is required, has no default, and is never inferred. This
			// check runs BEFORE any bundle load so an omitted actor can never
			// touch disk.
			if by == "" {
				return fmt.Errorf("--by <actor> is required: verify never fabricates or infers who checked a node")
			}
			if !okf.ValidActor(by) {
				return fmt.Errorf(
					"--by %q is not a valid §7 actor: use human:<id> (a person), process:<id> "+
						"(an automated process), or <producer>/<version> (a tool)", by)
			}

			dir := args[0]
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}

			if len(args) == 2 {
				return runVerifySingle(cmd, dir, b, withMD(args[1]), by)
			}
			return runVerifyBulk(cmd, dir, b, by, all, write)
		},
	}
	c.Flags().StringVar(&by, "by", "", "actor asserting the check (required; §7 form, no default, never inferred)")
	c.Flags().BoolVar(&all, "all", false, "confirm stamping the WHOLE bundle (required for bulk verify; a bulk rubber-stamp is noise)")
	c.Flags().BoolVar(&write, "write", false, "actually write in bulk mode (bulk verify is a dry-run plan by default)")
	return c
}

// runVerifySingle stamps one node and maintains the derived artifacts.
func runVerifySingle(cmd *cobra.Command, dir string, b *okf.Bundle, rel, by string) error {
	if okf.IsReservedPath(rel) {
		return fmt.Errorf("cannot verify reserved file: %s", rel)
	}
	if _, ok := b.Nodes[rel]; !ok {
		return fmt.Errorf("node not found: %s", rel)
	}
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := okf.AppendVerifiedFile(abs, by, nowUTCcmd()); err != nil {
		return fmt.Errorf("verify %s: %w", rel, err)
	}
	logOnVerify(cmd, dir, rel)
	maintainIndex(cmd, dir)
	fmt.Fprintf(cmd.OutOrStdout(), "Verified %s by %s\n", rel, by)
	return nil
}

// runVerifyBulk plans (and, with write, applies) a whole-bundle verify. The
// whole bundle is refused without --all. The plan skips reserved files (never
// nodes) and nodes that already carry a verified entry (re-stamping them would
// be the exact bulk rubber-stamp the guard exists to prevent), and reports every
// skip so the plan is legible.
func runVerifyBulk(cmd *cobra.Command, dir string, b *okf.Bundle, by string, all, write bool) error {
	if !all {
		return fmt.Errorf(
			"refusing to stamp the whole bundle without --all: verifying every node in one " +
				"invocation is a bulk rubber-stamp that converts `verified` from a trust signal " +
				"into noise — a reader can no longer tell a real check from a sweep.\n" +
				"  To stamp a single node you actually checked, pass its path:\n" +
				"      okfctl node verify <bundle> <path> --by <actor>\n" +
				"  To deliberately stamp the whole bundle anyway, re-run with --all (add --write to apply).")
	}

	out := cmd.OutOrStdout()
	var plan, skipped []string
	rels := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		rels = append(rels, p)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		if okf.IsReservedPath(rel) {
			skipped = append(skipped, rel+" (reserved file)")
			continue
		}
		if len(b.Nodes[rel].Verified()) > 0 {
			skipped = append(skipped, rel+" (already verified)")
			continue
		}
		plan = append(plan, rel)
	}

	for _, rel := range plan {
		if write {
			abs := filepath.Join(dir, filepath.FromSlash(rel))
			if err := okf.AppendVerifiedFile(abs, by, nowUTCcmd()); err != nil {
				return fmt.Errorf("verify %s: %w", rel, err)
			}
			logOnVerify(cmd, dir, rel)
			fmt.Fprintf(out, "verified %s by %s\n", rel, by)
		} else {
			fmt.Fprintf(out, "would verify %s by %s\n", rel, by)
		}
	}
	for _, s := range skipped {
		fmt.Fprintf(out, "  skip %s\n", s)
	}

	if write {
		// Regenerate index.md once after all stamps (not per node).
		maintainIndex(cmd, dir)
		fmt.Fprintf(out, "%d node(s) verified, %d skipped\n", len(plan), len(skipped))
		return nil
	}
	fmt.Fprintf(out, "%d node(s) would be verified, %d skipped (dry run; nothing written — add --write to apply)\n",
		len(plan), len(skipped))
	return nil
}
