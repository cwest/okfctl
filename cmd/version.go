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
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Build metadata. These default to a plain-build ("dev") value and are
// overridden at release time: goreleaser injects the tag, commit, and date
// into main's package vars via -ldflags, and main() hands them here through
// SetVersionInfo. A `go build` with no ldflags leaves them at their defaults,
// so `okfctl version` degrades gracefully to "dev".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// SetVersionInfo overrides the build metadata reported by `okfctl version` and
// `okfctl --version`. Empty arguments are ignored, so a plain build that
// injects nothing keeps the "dev" defaults. It returns the previous values so
// tests can restore package state.
func SetVersionInfo(v, c, d string) [3]string {
	prev := [3]string{version, commit, date}
	if v != "" {
		version = v
	}
	if c != "" {
		commit = c
	}
	if d != "" {
		date = d
	}
	return prev
}

// versionString is the single source of truth for both the `version` subcommand
// and the root `--version` flag.
func versionString() string {
	return fmt.Sprintf("okfctl %s (commit %s, built %s)", resolveVersion(version, debug.ReadBuildInfo), commit, date)
}

// resolveVersion returns the version string to report. A release build injects a
// real tag via ldflags (injected), which always wins. A `go install
// github.com/cwest/okfctl@latest` build injects nothing, leaving the "dev"
// default — in that case we fall back to the main module's version recorded by
// the Go toolchain (debug.ReadBuildInfo), so the binary can report the module
// version it was installed from instead of "dev". A plain `go build` from a
// checkout records "(devel)" (or nothing) for the main module, which is not a
// real release, so the "dev" default is kept. readBuildInfo is injected for
// testability; production passes debug.ReadBuildInfo.
func resolveVersion(injected string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if injected != "" && injected != "dev" {
		return injected
	}
	if info, ok := readBuildInfo(); ok {
		if mv := info.Main.Version; mv != "" && mv != "(devel)" {
			return mv
		}
	}
	return injected
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the okfctl version",
		Long: "version prints the okfctl build metadata: the release version, git commit, and " +
			"build date. On a plain `go build` with no release ldflags these degrade to \"dev\". " +
			"It is equivalent to `okfctl --version`.",
		Example: "  # Print the build version\n" +
			"  okfctl version",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), versionString())
			return nil
		},
	}
}
