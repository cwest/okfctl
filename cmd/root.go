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

// Package cmd implements the okfctl command tree.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/cwest/okfctl/internal/plugin"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the okfctl root command with its subcommand tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "okfctl",
		Short:         "Manage Open Knowledge Format (OKF) bundles",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newValidateCmd())
	root.AddCommand(newBundleCmd())
	root.AddCommand(newNodeCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newCompletionCmd(root))
	root.AddCommand(newIndexCmd())
	root.AddCommand(newLogCmd())
	root.AddCommand(newLintCmd())
	root.AddCommand(newGraphCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newTemplateCmd())
	root.AddCommand(newPluginCmd())
	return root
}

// Execute runs the root command; main() calls this.
//
// Before delegating to cobra, it checks whether the first non-flag argument
// names a built-in subcommand. If it does not, but an okfctl-<name> plugin
// exists on PATH, it dispatches to that plugin (git/kubectl style) and exits
// with the plugin's exit code. Otherwise cobra runs normally and produces its
// own "unknown command" error with a did-you-mean suggestion.
func Execute() error {
	root := NewRootCmd()
	if name, rest, ok := unknownSubcommand(root, os.Args[1:]); ok {
		if _, found := plugin.Lookup(name, os.Getenv("PATH")); found {
			code, err := dispatch(name, rest, os.Getenv("PATH"))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			os.Exit(code)
		}
	}
	return root.Execute()
}

// unknownSubcommand reports the first non-flag arg when it matches no built-in
// subcommand of root (so it is a candidate for plugin dispatch). It returns the
// candidate name, the remaining args after it, and ok=false when the first arg
// is a flag, is empty, or names a real built-in.
func unknownSubcommand(root *cobra.Command, args []string) (name string, rest []string, ok bool) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			return "", nil, false // a leading flag (e.g. --help) is cobra's
		}
		for _, c := range root.Commands() {
			if c.Name() == a || c.HasAlias(a) {
				return "", nil, false // built-in wins
			}
		}
		return a, args[i+1:], true
	}
	return "", nil, false
}
