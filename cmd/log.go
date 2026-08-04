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
	logCmd := &cobra.Command{Use: "log", Short: "Manage the reserved log.md change history"}

	var msg string
	appendC := &cobra.Command{
		Use:   "append [dir]",
		Short: "Append a timestamped change entry to log.md",
		Long: "log append adds one timestamped entry to the reserved log.md change history (OKF §9: " +
			"the log file is a reserved, append-only record of what changed in the bundle and when). " +
			"The entry text is required via --message. It only appends — it never rewrites or " +
			"reorders existing history. The node verbs append to log.md for you; use this for " +
			"changes you made outside okfctl.",
		Example: "  # Record a manual change\n" +
			"  okfctl log append --message \"reworded the revenue concept\"\n\n" +
			"  # Record against a bundle elsewhere\n" +
			"  okfctl log append --message \"imported Q3 sources\" ./bundles/knowledge",
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
		Short: "Print the change history",
		Long: "log show prints the reserved log.md change history (OKF §9) verbatim to stdout. It " +
			"is read-only and does not mutate the bundle. Use it to review what changed and when, " +
			"or to pipe the history into another tool.",
		Example: "  # Print the change history for the current bundle\n" +
			"  okfctl log show\n\n" +
			"  # Print the history for a bundle elsewhere\n" +
			"  okfctl log show ./bundles/knowledge",
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
