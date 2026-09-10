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
	index := &cobra.Command{Use: "index", Short: "Manage the reserved index.md"}

	buildCmd := &cobra.Command{
		Use:   "build [dir]",
		Short: "Regenerate index.md from the current bundle",
		Long: "index build regenerates the reserved index.md navigation file(s) from the bundle's " +
			"current concept nodes (OKF §8: index files are reserved, generated navigation " +
			"files). It rewrites index.md at the bundle root and in each directory that has one; " +
			"it never edits concept nodes. Run it after adding, moving, or removing nodes by hand " +
			"(the node verbs regenerate it for you).\n\n" +
			"Each subdirectory entry carries a shape suffix — the child's own concept count, its " +
			"types, its shared tags, and (when it nests deeper) a subtree total — so a reader can " +
			"decide which branch to open without opening it. Tune it with --shape-tag-min / " +
			"--shape-tag-max, or drop it with --no-shape.",
		Example: "  # Regenerate index.md for the current bundle\n" +
			"  okfctl index build\n\n" +
			"  # Regenerate for a bundle elsewhere\n" +
			"  okfctl index build ./bundles/knowledge",
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
		Short: "Verify index.md is current (nonzero exit if stale)",
		Long: "index check verifies the reserved index.md (OKF §8) is in sync with the bundle's " +
			"current nodes, without writing anything. It's the CI-friendly counterpart to " +
			"`index build`: it exits zero when the index is current and non-zero (printing what " +
			"drifted) when a rebuild is needed. Read-only — it never rewrites the index.\n\n" +
			"Pass the SAME shape flags you built with (--shape-tag-min / --shape-tag-max / " +
			"--no-shape); a check against a different shape configuration than the build reports " +
			"drift by design.",
		Example: "  # Verify the index is current (exit 0) or report drift (exit 1)\n" +
			"  okfctl index check\n\n" +
			"  # Check a bundle elsewhere, e.g. in CI\n" +
			"  okfctl index check ./bundles/knowledge",
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
