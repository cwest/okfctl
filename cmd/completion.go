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
	"github.com/spf13/cobra"
)

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish]",
		Short: "Generate a shell completion script",
		Long: "completion prints a bash, zsh, or fish script to stdout. Load it in your " +
			"shell (or save it where your shell looks for completions) and Tab will fill in " +
			"okfctl's commands and flags.\n\n" +
			"Give it one of those three shell names. Anything else exits 1. completion only " +
			"prints the script. Where you save it is up to you.",
		Example: "  # Load bash completions for the current shell\n" +
			"  source <(okfctl completion bash)\n\n" +
			"  # Install zsh completions\n" +
			"  okfctl completion zsh > \"${fpath[1]}/_okfctl\"\n\n" +
			"  # Install fish completions\n" +
			"  okfctl completion fish > ~/.config/fish/completions/okfctl.fish",
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             []string{"bash", "zsh", "fish"},
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			}
			return nil
		},
	}
}
