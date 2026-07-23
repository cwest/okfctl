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
		Args:  cobra.MaximumNArgs(1),
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
		Args:  cobra.MaximumNArgs(1),
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
