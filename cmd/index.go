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

// addShapeFlags registers the shape-suffix flags shared by `index build` and
// `index check` (OKF §8 progressive disclosure). They MUST be wired identically
// on both so a build and its check render with the same options — otherwise the
// check would perpetually report drift. It returns an accessor that reads the
// bound flag values into an okf.IndexShapeOptions.
func addShapeFlags(c *cobra.Command) func() okf.IndexShapeOptions {
	def := okf.DefaultShapeOptions()
	var noShape bool
	var tagMin, tagMax int
	c.Flags().BoolVar(&noShape, "no-shape", false,
		"omit the subdirectory shape suffix (concept count, types, shared tags, subtree total)")
	c.Flags().IntVar(&tagMin, "shape-tag-min", def.TagMin,
		"a shared tag prints only when at least this many of a directory's own concepts carry it")
	c.Flags().IntVar(&tagMax, "shape-tag-max", def.TagMax,
		"cap the shared-tag list at this many tags (most-shared-first, ellipsis on truncation)")
	return func() okf.IndexShapeOptions {
		return okf.IndexShapeOptions{Enabled: !noShape, TagMin: tagMin, TagMax: tagMax}
	}
}

func newIndexCmd() *cobra.Command {
	index := &cobra.Command{Use: "index", Short: "Rebuild or check the generated index.md files"}

	buildCmd := &cobra.Command{
		Use:   "build [dir]",
		Short: "Rewrite every index.md to list the bundle's current nodes",
		Long: "build writes an index.md into the bundle root and into each directory under it that " +
			"has concept nodes in it (OKF §8). Each lists its directory's nodes by title (plus the " +
			"description where a node has one) and then links to the subdirectories. In the " +
			"example a node moved with mv(1) leaves the index stale: check exits 1 until build " +
			"fixes it.\n\n" +
			"You'll seldom need it.\n\n" +
			"Six node commands (new, mv, rm, edit, refresh, promote) rebuild the index when they " +
			"finish. build is for the times you add, move, or delete node files yourself.\n\n" +
			"Each subdirectory's link ends with a short summary (concept count, types, the tags " +
			"common to most of its nodes, a subtree total for deeper trees) that tells you what a branch holds. --shape-tag-min and --shape-tag-max tune the tag list. " +
			"--no-shape drops the summary.\n\n" +
			"Only the root index.md's frontmatter survives a rebuild (that frontmatter is " +
			"where the bundle's okf_version, title, and description live, §8.1). Everything below " +
			"it in each index.md is rewritten " +
			"from scratch. An index.md in a directory with no nodes is deleted. Subdirectory index.md " +
			"files with frontmatter go too: each is a concept stored where §8 puts an index " +
			"(validate fails it). Run `okfctl node promote` on such a file first or build throws " +
			"away what you wrote there.",
		Annotations: map[string]string{
			transcriptAnnotation: "<!-- transcript: exit 1 okfctl index check atlas -->",
		},
		Example: "  $ okfctl bundle init atlas\n" +
			"  Initialized OKF bundle in atlas\n" +
			"  $ okfctl node new tannin --type Concept --title Tannin --bundle atlas\n" +
			"  Created atlas/tannin.md\n" +
			"  $ mkdir atlas/wine && mv atlas/tannin.md atlas/wine/\n" +
			"  $ okfctl index check atlas\n" +
			"  index.md is out of date; run `okfctl index build` to regenerate\n" +
			"  okfctl: index.md is out of date\n" +
			"  $ okfctl index build atlas\n" +
			"  Wrote 2 index.md file(s) under atlas\n" +
			"  $ cat atlas/index.md\n" +
			"  ---\n" +
			"  okf_version: \"0.3\"\n" +
			"  ---\n" +
			"\n" +
			"  # Knowledge Base\n" +
			"\n" +
			"  ## Subdirectories\n" +
			"\n" +
			"  * [Wine](wine/) (1 concept · Concept)",
		Args: cobra.MaximumNArgs(1),
	}
	buildNoIgnore := addNoIgnoreFlag(buildCmd)
	buildShape := addShapeFlags(buildCmd)
	buildCmd.RunE = func(cmd *cobra.Command, args []string) error {
		dir := bundleDirArg(args)
		b, err := loadBundleForCmd(cmd, dir, *buildNoIgnore)
		if err != nil {
			return err
		}
		if err := okf.WriteIndexWithOptions(b, buildShape()); err != nil {
			return fmt.Errorf("write index.md: %w", err)
		}
		dirs := okf.IndexDirs(b)
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d index.md file(s) under %s\n", len(dirs), dir)
		return nil
	}
	index.AddCommand(buildCmd)

	checkCmd := &cobra.Command{
		Use:   "check [dir]",
		Short: "Exit 1 if any index.md is out of date, without writing",
		Long: "check renders each index.md the way `okfctl index build` would and compares the " +
			"result with what's on disk (OKF §8). It writes nothing. When every file matches it " +
			"prints \"OK: index.md is current\" and exits 0. Otherwise it names the first file " +
			"that's missing, stale, or left in a directory with no nodes, then exits 1. In the " +
			"example the node written with printf hasn't reached the index.\n\n" +
			"Put check in CI. lint --strict won't flag a stale index.\n\n" +
			"Pass the --shape-tag-min, --shape-tag-max, and --no-shape flags you built with. " +
			"Build with --no-shape and check with the default flags: any index that links a " +
			"subdirectory reads as stale.\n\n" +
			"check doesn't fix anything. build does.",
		Annotations: map[string]string{
			transcriptAnnotation: "<!-- transcript: exit 1 okfctl index check pantry -->",
		},
		Example: "  $ okfctl bundle init pantry\n" +
			"  Initialized OKF bundle in pantry\n" +
			"  $ printf -- '---\\ntype: Concept\\ntitle: Tannin\\n---\\n' > pantry/tannin.md\n" +
			"  $ okfctl index check pantry\n" +
			"  index.md is out of date; run `okfctl index build` to regenerate\n" +
			"  okfctl: index.md is out of date\n" +
			"  $ okfctl index build pantry\n" +
			"  Wrote 1 index.md file(s) under pantry\n" +
			"  $ cd pantry && okfctl index check\n" +
			"  OK: index.md is current",
		Args: cobra.MaximumNArgs(1),
	}
	checkNoIgnore := addNoIgnoreFlag(checkCmd)
	checkShape := addShapeFlags(checkCmd)
	checkCmd.RunE = func(cmd *cobra.Command, args []string) error {
		dir := bundleDirArg(args)
		b, err := loadBundleForCmd(cmd, dir, *checkNoIgnore)
		if err != nil {
			return err
		}
		ok, report := okf.IndexInSyncWithOptions(b, checkShape())
		if ok {
			fmt.Fprintln(cmd.OutOrStdout(), "OK: index.md is current")
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), report)
		return fmt.Errorf("index.md is out of date")
	}
	index.AddCommand(checkCmd)
	return index
}

// bundleDirArg returns the positional bundle dir or "." when omitted.
func bundleDirArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}
