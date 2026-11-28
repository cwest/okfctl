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

package okf

import (
	"fmt"
	"sort"
	"strings"
)

// lintFootnoteDefinitions reports, per concept, a footnote label defined more
// than once (GFM matches labels case-insensitively, so `[^S1]` and `[^s1]` are
// one label) and a definition with no text. §4.2 makes per-claim attribution
// a footnote whose label is a sources[].id, and §5.1 puts the citation text in
// its definition: a second definition leaves the reference pointing at only
// one of them, and an empty one cites nothing. Lines inside fenced code are
// not definitions.
//
// Advisory only, like every lint check: §11.1 leaves the bundle conformant.
func lintFootnoteDefinitions(b *Bundle) []LintFinding {
	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		n := b.Nodes[p]
		lines := strings.Split(strings.ReplaceAll(n.Body, "\r\n", "\n"), "\n")
		count := map[string]int{}
		first := map[string]string{}
		var empty []string
		inFence := false
		for i, line := range lines {
			if fenceLineRe.MatchString(line) {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			m := footnoteDefLineRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			k := strings.ToLower(m[1])
			if count[k] == 0 {
				first[k] = m[1]
			}
			count[k]++
			if strings.TrimSpace(m[2]) == "" && !hasContinuation(lines[i+1:]) {
				empty = append(empty, m[1])
			}
		}
		var dups []string
		for k, c := range count {
			if c > 1 {
				dups = append(dups, k)
			}
		}
		sort.Strings(dups)
		for _, k := range dups {
			out = append(out, LintFinding{Check: "footnote-definition", Path: p, Message: fmt.Sprintf(
				"footnote-definition: %s defines [^%s] %d times; §4.2: a footnote label names one source, so a reference finds only one of these definitions",
				p, first[k], count[k])})
		}
		for _, l := range empty {
			out = append(out, LintFinding{Check: "footnote-definition", Path: p, Message: fmt.Sprintf(
				"footnote-definition: %s defines [^%s] with no text; §4.2, §5.1: the definition holds the citation text, so this one cites nothing",
				p, l)})
		}
	}
	return out
}

// hasContinuation reports whether the first non-blank line of rest is
// indented four spaces or a tab: the text of a footnote definition whose
// first line is empty.
func hasContinuation(rest []string) bool {
	for _, l := range rest {
		if strings.TrimSpace(l) == "" {
			continue
		}
		return strings.HasPrefix(l, "    ") || strings.HasPrefix(l, "\t")
	}
	return false
}
