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
			p, err := okf.NewNode(dir, args[0], typ, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", p)
			return nil
		},
	}
	newC.Flags().StringVar(&typ, "type", "", "node type (required)")
	newC.Flags().StringVar(&title, "title", "", "node title")
	newC.Flags().StringVar(&dir, "bundle", ".", "bundle directory")
	node.AddCommand(newC)
	return node
}
