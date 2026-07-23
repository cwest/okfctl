// Copyright 2026 Casey West
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
	"path/filepath"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newIndexCmd() *cobra.Command {
	index := &cobra.Command{Use: "index", Short: "Manage the reserved index.md"}

	index.AddCommand(&cobra.Command{
		Use:   "build [dir]",
		Short: "Regenerate index.md from the current bundle",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			out := okf.RenderIndex(b)
			if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(out), 0o644); err != nil {
				return fmt.Errorf("write index.md: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", filepath.Join(dir, "index.md"))
			return nil
		},
	})

	index.AddCommand(&cobra.Command{
		Use:   "check [dir]",
		Short: "Verify index.md is current (nonzero exit if stale)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := bundleDirArg(args)
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			ok, report := okf.IndexInSync(b)
			if ok {
				fmt.Fprintln(cmd.OutOrStdout(), "OK: index.md is current")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), report)
			return fmt.Errorf("index.md is out of date")
		},
	})
	return index
}

// bundleDirArg returns the positional bundle dir or "." when omitted.
func bundleDirArg(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}
