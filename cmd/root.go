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

import "github.com/spf13/cobra"

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
	return root
}

// Execute runs the root command; main() calls this.
func Execute() error {
	return NewRootCmd().Execute()
}
