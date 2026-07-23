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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

func configPath() string {
	home := os.Getenv("OKFCTL_CONFIG_HOME")
	if home == "" {
		if h, err := os.UserConfigDir(); err == nil {
			home = filepath.Join(h, "okfctl")
		} else {
			home = ".okfctl"
		}
	}
	return filepath.Join(home, "config.json")
}

func loadConfig() (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func saveConfig(m map[string]string) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{Use: "config", Short: "Get and set okfctl configuration"}
	c.AddCommand(&cobra.Command{
		Use: "set <key> <value>", Short: "Set a config value", Args: cobra.ExactArgs(2),
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
