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
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

// noIgnoreFlagUsage is the shared help text for the --no-ignore escape hatch, so
// every walking command documents it identically.
const noIgnoreFlagUsage = "walk EVERY directory, including vendored/derived ones (.venv, node_modules, dist, ...) that are skipped by default"

// addNoIgnoreFlag registers the shared --no-ignore flag on a command and returns
// the bound *bool. Every command that walks a bundle wires this the same way so
// the escape hatch is uniform across lint, analyze, validate, search, graph, and
// index.
func addNoIgnoreFlag(c *cobra.Command) *bool {
	var noIgnore bool
	c.Flags().BoolVar(&noIgnore, "no-ignore", false, noIgnoreFlagUsage)
	return &noIgnore
}

// loadBundleForCmd loads the bundle honoring --no-ignore and, when the default
// skip list pruned any directories, announces them on stderr. The skip is never
// silent: silently excluding authored content is worse than noisy findings
// (okfctl's over-conformance rule), so the operator always sees which subtrees
// were left out and how to get them back.
func loadBundleForCmd(cmd *cobra.Command, dir string, noIgnore bool) (*okf.Bundle, error) {
	var opts []okf.LoadOption
	if noIgnore {
		opts = append(opts, okf.WithNoIgnore())
	}
	b, err := okf.Load(dir, opts...)
	if err != nil {
		return nil, fmt.Errorf("load bundle: %w", err)
	}
	if len(b.SkippedDirs) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"note: skipped %d vendored/derived director%s (%s); pass --no-ignore to include them\n",
			len(b.SkippedDirs), plural(len(b.SkippedDirs), "y", "ies"),
			strings.Join(b.SkippedDirs, ", "))
	}
	return b, nil
}

// plural picks the singular or plural suffix for n.
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
