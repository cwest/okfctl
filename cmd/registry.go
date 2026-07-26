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
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// registryKeyPrefix namespaces named-remote entries inside the ONE okfctl
// config store (okfconfig), so `registry` reuses the single config mechanism
// rather than introducing a second config file.
const registryKeyPrefix = "registry."

// validRegistryName restricts a remote name to a safe identifier: letters,
// digits, and -_. so it cannot collide with the key prefix, embed a config
// delimiter, or smuggle whitespace.
var validRegistryName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func registryKey(name string) string { return registryKeyPrefix + name }

// newRegistryCmd builds `okfctl registry`, a named directory of remote bundle
// sources (each a plain git URL). It is not a hosted registry, account system,
// or schema registry (PRD §5.2, §9.1) — it is `git remote` for OKF bundles.
func newRegistryCmd() *cobra.Command {
	registry := &cobra.Command{
		Use:   "registry",
		Short: "Manage named remote bundle sources (git URLs)",
		Long: "Manage a local, named directory of remote OKF bundle sources.\n" +
			"Each source is a plain git URL — this is `git remote` for bundles, " +
			"not a hosted registry, account system, or schema registry.",
	}

	registry.AddCommand(&cobra.Command{
		Use:   "add <name> <git-url>",
		Short: "Register (or re-point) a named remote bundle source",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, url := args[0], args[1]
			if !validRegistryName.MatchString(name) {
				return fmt.Errorf("invalid remote name %q: use letters, digits, and -_. only", name)
			}
			if strings.TrimSpace(url) == "" {
				return fmt.Errorf("git URL must not be empty")
			}
			m, err := loadConfig()
			if err != nil {
				return err
			}
			_, existed := m[registryKey(name)]
			m[registryKey(name)] = url
			if err := saveConfig(m); err != nil {
				return err
			}
			verb := "Added"
			if existed {
				verb = "Updated"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s remote %q -> %s\n", verb, name, url)
			return nil
		},
	})

	registry.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered remote bundle sources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			names := registryNames(m)
			out := cmd.OutOrStdout()
			if len(names) == 0 {
				fmt.Fprintln(out, "no remote sources registered")
				return nil
			}
			for _, n := range names {
				fmt.Fprintf(out, "%s\t%s\n", n, m[registryKey(n)])
			}
			return nil
		},
	})

	registry.AddCommand(&cobra.Command{
		Use:   "show <name>",
		Short: "Print a remote source's git URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			url, ok := m[registryKey(args[0])]
			if !ok {
				return fmt.Errorf("no such remote: %s", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), url)
			return nil
		},
	})

	registry.AddCommand(&cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm"},
		Short:   "Unregister a remote bundle source",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := loadConfig()
			if err != nil {
				return err
			}
			key := registryKey(args[0])
			if _, ok := m[key]; !ok {
				return fmt.Errorf("no such remote: %s", args[0])
			}
			delete(m, key)
			if err := saveConfig(m); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed remote %q\n", args[0])
			return nil
		},
	})

	return registry
}

// registryNames returns the sorted remote names present in the config map.
func registryNames(m map[string]string) []string {
	var names []string
	for k := range m {
		if strings.HasPrefix(k, registryKeyPrefix) {
			names = append(names, strings.TrimPrefix(k, registryKeyPrefix))
		}
	}
	sort.Strings(names)
	return names
}

// resolveRemoteURL maps a registry name to its URL. ok=false means the name is
// not registered (the caller decides whether to treat the arg as an ad-hoc URL).
func resolveRemoteURL(name string) (string, bool, error) {
	m, err := loadConfig()
	if err != nil {
		return "", false, err
	}
	url, ok := m[registryKey(name)]
	return url, ok, nil
}
