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
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// walkCommands returns every command in the tree rooted at c, including c.
func walkCommands(c *cobra.Command) []*cobra.Command {
	out := []*cobra.Command{c}
	for _, child := range c.Commands() {
		out = append(out, walkCommands(child)...)
	}
	return out
}

// isRunnable reports whether cmd actually executes work (has a Run/RunE) as
// opposed to being a pure grouping command that only holds subcommands. Group
// commands (e.g. `node`, `config`) show a subcommand menu on --help and are
// exempt from the Long/Example requirement; runnable leaf commands are not.
func isRunnable(cmd *cobra.Command) bool {
	return cmd.Run != nil || cmd.RunE != nil
}

// TestHelpSurface_EveryRunnableCommandHasLongAndExample walks the whole command
// tree and asserts that every runnable (non-group) command carries a non-empty
// Long description and a non-empty Example. This is the mechanism that keeps the
// help surface from regressing: a future command added without help text fails
// CI here instead of quietly shipping a syntax-only --help.
//
// The cobra-generated `completion` and `help` commands are excluded — they are
// framework-provided and their help text is not ours to author.
func TestHelpSurface_EveryRunnableCommandHasLongAndExample(t *testing.T) {
	root := NewRootCmd()

	// Framework-generated commands we do not author.
	skip := map[string]bool{
		"help": true,
	}

	for _, cmd := range walkCommands(root) {
		cmd := cmd
		if cmd == root {
			continue // root's help is the top-level menu; no Example expected.
		}
		if skip[cmd.Name()] {
			continue
		}
		if !isRunnable(cmd) {
			continue // grouping command: subcommand menu is its help.
		}
		path := cmd.CommandPath()
		t.Run(path, func(t *testing.T) {
			if strings.TrimSpace(cmd.Long) == "" {
				t.Errorf("command %q has no Long description; every runnable command must document what it does, what it does NOT do, and cite the governing OKF spec section where behavior is spec-mandated", path)
			}
			if strings.TrimSpace(cmd.Example) == "" {
				t.Errorf("command %q has no Example; every runnable command must show at least one real, copy-pasteable invocation", path)
			}
		})
	}
}
