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
	"sort"

	"github.com/cwest/okfctl/internal/okfconfig"
	"github.com/spf13/cobra"
)

func loadConfig() (map[string]string, error) { return okfconfig.Load() }
func saveConfig(m map[string]string) error   { return okfconfig.Save(m) }

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Read and write okfctl's own settings",
		Long: "config reads and writes config.json, okfctl's settings file. Set " +
			"$OKFCTL_CONFIG_HOME to choose its directory; without it okfctl uses okfctl under " +
			"your OS's user config directory. Plugins read the file too (okfctl-search looks " +
			"up model_path there). Names you save with `okfctl registry` go in it under the " +
			"registry. prefix. No bundle content ever lands in it.",
	}
	c.AddCommand(&cobra.Command{
		Use: "set <key> <value>", Short: "Set a config value", Args: cobra.ExactArgs(2),
		Long: "set saves a value under a key and prints nothing. Setting a key again replaces " +
			"its value. okfctl doesn't check key names. Misspell one and you've made a new key.",
		Example: "  # Point okfctl-search at a local model2vec model\n" +
			"  okfctl config set model_path ~/models/potion-base-8M",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			m[args[0]] = args[1]
			return saveConfig(m)
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "get <key>", Short: "Get a config value", Args: cobra.ExactArgs(1),
		Long: "get prints the value saved under one key. If the key isn't set, get exits 1. " +
			"For all of them, run `okfctl config list`.",
		Example: "  # Print one config value\n" +
			"  okfctl config get model_path",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			v, ok := m[args[0]]
			if !ok {
				return fmt.Errorf("no such key: %s", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	})
	c.AddCommand(&cobra.Command{
		Use: "list", Short: "List all config values", Args: cobra.NoArgs,
		Long: "list prints each saved key as key = value, sorted by key. The registry. " +
			"entries show up here too. `okfctl registry list` shows just those.",
		Example: "  # Show every stored config key and value\n" +
			"  okfctl config list",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", k, m[k])
			}
			return nil
		},
	})
	return c
}
