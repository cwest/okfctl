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
	"os"

	"github.com/cwest/okfctl/internal/plugin"
	"github.com/spf13/cobra"
)

// newPluginCmd builds the `plugin` command tree. For this increment it carries
// `list`; `plugin install` is deferred to a later slice.
func newPluginCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "plugin",
		Short: "Discover and manage okfctl plugins (okfctl-<name> on PATH)",
	}
	c.AddCommand(newPluginListCmd())
	return c
}

func newPluginListCmd() *cobra.Command {
	var pathOverride string
	c := &cobra.Command{
		Use:   "list",
		Short: "List okfctl-<name> plugin executables found on PATH",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			pathenv := pathOverride
			if pathenv == "" {
				pathenv = os.Getenv("PATH")
			}
			plugins := plugin.Discover(pathenv)
			if len(plugins) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no okfctl plugins found on PATH")
				return nil
			}
			for _, p := range plugins {
				fmt.Fprintf(cmd.OutOrStdout(), "okfctl-%s\t%s\n", p.Name, p.Path)
			}
			return nil
		},
	}
	c.Flags().StringVar(&pathOverride, "path", "", "PATH to scan for plugins (defaults to $PATH)")
	return c
}
