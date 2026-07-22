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

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newBundleCmd() *cobra.Command {
	bundle := &cobra.Command{Use: "bundle", Short: "Bundle lifecycle commands"}
	bundle.AddCommand(&cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a minimal conformant OKF bundle",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			if err := okf.Scaffold(dir); err != nil {
				return fmt.Errorf("scaffold: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized OKF bundle in %s\n", dir)
			return nil
		},
	})
	return bundle
}
