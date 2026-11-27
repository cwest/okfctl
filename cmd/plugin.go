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
	"path/filepath"

	"github.com/cwest/okfctl/internal/plugin"
	"github.com/spf13/cobra"
)

// newPluginCmd builds the `plugin` command tree: `list` for discovery and
// `install` for placing an okfctl-<name> executable into the managed plugins dir.
func newPluginCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "plugin",
		Short: "Find and install okfctl-<name> plugins",
		Long: "A plugin is any executable named okfctl-<name> on your PATH. Run `okfctl <name>` " +
			"when okfctl has no command of that name. okfctl passes the rest of your arguments " +
			"to the plugin and exits with its status. git and kubectl do the same. okfctl's own " +
			"commands always beat a plugin with the same name.",
	}
	c.AddCommand(newPluginListCmd())
	c.AddCommand(newPluginInstallCmd())
	return c
}

func newPluginListCmd() *cobra.Command {
	var pathOverride string
	c := &cobra.Command{
		Use:   "list",
		Short: "List okfctl-<name> plugin executables found on PATH",
		Long: "plugin list prints each okfctl-<name> executable it finds on your PATH, with its " +
			"path. When two directories hold the same name, the first one on PATH is the one " +
			"`okfctl <name>` runs. --path scans a different list of directories.\n\n" +
			"list only reads. When it finds no plugins it says so on stderr and exits 0.",
		Example: "  # List discovered plugins on $PATH\n" +
			"  okfctl plugin list\n\n" +
			"  # Scan a specific PATH\n" +
			"  okfctl plugin list --path ~/bin",
		Args: cobra.NoArgs,
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

// newPluginInstallCmd builds `plugin install <source>`: copy an okfctl-<name>
// executable into the managed plugins dir so `plugin list` and dispatch find it.
func newPluginInstallCmd() *cobra.Command {
	var dirOverride string
	c := &cobra.Command{
		Use:   "install <source>",
		Short: "Copy an okfctl-<name> executable into okfctl's plugins directory",
		Long: "plugin install copies an executable named okfctl-<name> into okfctl's plugins " +
			"directory: $OKFCTL_CONFIG_HOME/plugins if that's set, otherwise okfctl/plugins under " +
			"your OS's user config directory. --dir picks another directory. The copy gets mode " +
			"755 and replaces any plugin with the same name there.\n\n" +
			"A source that isn't named okfctl-<name> exits 1. install doesn't touch your PATH. " +
			"If the directory isn't on it, install prints a note. Add the directory yourself " +
			"so `okfctl <name>` and `okfctl plugin list` can find the plugin.",
		Example: "  # Install a plugin executable into the managed plugins dir\n" +
			"  okfctl plugin install ./okfctl-search\n\n" +
			"  # Install into a specific directory\n" +
			"  okfctl plugin install --dir ~/bin ./okfctl-search",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := dirOverride
			if dir == "" {
				dir = plugin.InstallDir()
			}
			p, err := plugin.Install(args[0], dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed okfctl-%s -> %s\n", p.Name, p.Path)
			if !dirOnPath(dir, os.Getenv("PATH")) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"note: %s is not on your PATH; add it so `okfctl %s` and `okfctl plugin list` find this plugin\n",
					dir, p.Name)
			}
			return nil
		},
	}
	c.Flags().StringVar(&dirOverride, "dir", "", "directory to install into (defaults to the managed plugins dir)")
	return c
}

// dirOnPath reports whether dir appears on the OS path list pathenv, comparing
// cleaned paths so trailing separators or "." segments do not cause false misses.
func dirOnPath(dir, pathenv string) bool {
	want := filepath.Clean(dir)
	for _, d := range filepath.SplitList(pathenv) {
		if d != "" && filepath.Clean(d) == want {
			return true
		}
	}
	return false
}
