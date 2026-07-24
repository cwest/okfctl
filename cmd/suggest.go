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
	"strings"

	"github.com/cwest/okfctl/internal/plugin"
	"github.com/spf13/cobra"
)

// suggestionMinDistance mirrors cobra's default SuggestionsMinimumDistance so a
// plugin-name typo is judged by the same yardstick as a built-in typo.
const suggestionMinDistance = 2

// unknownCommandError reports the error for `okfctl <args...>` when the first
// non-flag token names neither a built-in subcommand nor an okfctl-<name>
// plugin on pathenv. It returns nil when the token resolves (built-in or exact
// plugin) — those are handled by cobra or by dispatch, not by this path.
//
// The returned error names the unknown command and, per spec, offers a
// did-you-mean suggestion drawn from the UNION of built-in subcommands AND
// discovered plugin names (cobra's native suggester only sees built-ins).
func unknownCommandError(root *cobra.Command, args []string, pathenv string) error {
	name, _, ok := unknownSubcommand(root, args)
	if !ok {
		return nil // leading flag, empty, or a real built-in — not our concern
	}
	if _, found := plugin.Lookup(name, pathenv); found {
		return nil // exact plugin — dispatch handles it, not an error
	}

	msg := fmt.Sprintf("okfctl: unknown command %q for %q", name, root.Name())
	if s := suggestionsFor(root, name, pathenv); len(s) > 0 {
		msg += "\n\nDid you mean this?\n"
		for _, c := range s {
			msg += "\t" + c + "\n"
		}
	}
	return fmt.Errorf("%s", msg)
}

// suggestionsFor returns the near-miss candidates for typed, drawn from the
// union of root's built-in subcommand names/aliases and the names of plugins
// discovered on pathenv, sorted and de-duplicated.
func suggestionsFor(root *cobra.Command, typed, pathenv string) []string {
	candidates := make(map[string]bool)
	for _, c := range root.Commands() {
		if c.IsAvailableCommand() || c.Name() == "help" {
			candidates[c.Name()] = true
			for _, a := range c.Aliases {
				candidates[a] = true
			}
		}
	}
	for _, p := range plugin.Discover(pathenv) {
		candidates[p.Name] = true
	}

	var out []string
	seen := make(map[string]bool)
	for name := range candidates {
		if name == typed {
			continue
		}
		if levenshtein(typed, name) <= suggestionMinDistance || strings.Contains(strings.ToLower(name), strings.ToLower(typed)) {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

// levenshtein computes the edit distance between a and b (case-insensitive),
// matching the distance metric cobra uses for its own command suggestions.
func levenshtein(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
