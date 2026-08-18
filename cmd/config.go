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
	c := &cobra.Command{Use: "config", Short: "Get and set okfctl configuration"}
	c.AddCommand(&cobra.Command{
		Use: "set <key> <value>", Short: "Set a config value", Args: cobra.ExactArgs(2),
		Long: "set writes a key/value pair into the single okfctl config store (a flat file " +
			"under $OKFCTL_CONFIG_HOME or the OS user-config dir). An existing key is overwritten. " +
			"This is okfctl's own tool configuration, not bundle content — it never touches a bundle.",
		Example: "  # Pin a default remote registry URL prefix\n" +
			"  okfctl config set default.remote https://github.com/acme",
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
		Long: "get prints the value stored for a single config key, or exits non-zero if the " +
			"key is unset. It's read-only. Use `okfctl config list` to see every key at once.",
		Example: "  # Print one config value\n" +
			"  okfctl config get default.remote",
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
		Long: "list prints every stored config key and its value, sorted by key. It's read-only. " +
			"Registered remote sources appear here too, under the registry. key prefix (see " +
			"`okfctl registry list` for a focused view of those).",
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
