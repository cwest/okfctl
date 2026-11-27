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

func newLogCmd() *cobra.Command {
	logCmd := &cobra.Command{Use: "log", Short: "Add to or print the bundle's log.md change history"}

	var msg string
	appendC := &cobra.Command{
		Use:   "append [dir]",
		Short: "Add a line to today's entry in log.md",
		Long: "append adds --message as one line under today's date in the log.md at the root of " +
			"dir (the current directory by default), and writes today's date heading first if " +
			"the log doesn't have one, because dates run newest first (OKF §9). No log.md? append " +
			"creates it. The example prints <DATE> where your run prints today's date.\n\n" +
			"You need it only for changes made outside okfctl. Six node commands (new, mv, rm, " +
			"edit, refresh, promote) write their own line.\n\n" +
			"Whatever sits above the first date heading, your title included, is left alone. " +
			"The placeholder line `okfctl bundle init` wrote " +
			"goes away, and older okfctl bullets with their date inline move under a date " +
			"heading of their own. Only the first line of a message gets in. Leave off --message " +
			"and append exits 1.",
		Example: "  $ okfctl bundle init journal\n" +
			"  Initialized OKF bundle in journal\n" +
			"  $ okfctl log append --message 'split the oak node into barrel and cask' journal\n" +
			"  Appended log entry\n" +
			"  $ okfctl log show journal\n" +
			"  # Change Log\n" +
			"\n" +
			"  ## <DATE>\n" +
			"  * split the oak node into barrel and cask",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if msg == "" {
				return fmt.Errorf("--message is required")
			}
			dir := bundleDirArg(args)
			if err := okf.AppendLog(dir, msg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Appended log entry")
			return nil
		},
	}
	appendC.Flags().StringVar(&msg, "message", "", "the change entry text (required)")
	logCmd.AddCommand(appendC)

	logCmd.AddCommand(&cobra.Command{
		Use:   "show [dir]",
		Short: "Print log.md as it is on disk",
		Long: "show prints the root log.md of dir (the current directory by default) byte for " +
			"byte, to read or to pipe into another tool. In the example two nodes get created and " +
			"one gets moved. All three land under one date heading, newest first (OKF §9).\n\n" +
			"show never writes. It won't reformat the file or check it against §9; `okfctl " +
			"validate` does that. A log.md in a subdirectory is ignored. With no log.md at all, " +
			"show prints nothing and exits 0.",
		Example: "  $ okfctl bundle init diary\n" +
			"  Initialized OKF bundle in diary\n" +
			"  $ okfctl node new wine/tannin --type Concept --bundle diary\n" +
			"  Created diary/wine/tannin.md\n" +
			"  $ okfctl node new wine/oak --type Concept --bundle diary\n" +
			"  Created diary/wine/oak.md\n" +
			"  $ okfctl node mv wine/oak barrel/oak --bundle diary\n" +
			"  Moved wine/oak.md -> barrel/oak.md (0 inbound link(s) rewritten)\n" +
			"  $ okfctl log show diary\n" +
			"  # Change Log\n" +
			"\n" +
			"  ## <DATE>\n" +
			"  * moved wine/oak.md -> barrel/oak.md\n" +
			"  * created wine/oak.md\n" +
			"  * created wine/tannin.md",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			body, err := okf.ReadLog(dir)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), body)
			return nil
		},
	})
	return logCmd
}
