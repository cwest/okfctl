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
	"path/filepath"
	"sort"
	"strings"
)

// LintFinding is one judgment-worthy observation about bundle health. Unlike a
// validate Finding (a spec-floor violation), a lint finding is curation
// guidance — never a format failure.
type LintFinding struct {
	Check   string // "orphan" | "missing-xref" | "coverage-gap" | "type-hygiene"
	Path    string // node path the finding is about ("" for bundle-level findings)
	Message string
}

// LintOptions configures the deterministic structural checks.
type LintOptions struct {
	// CoverageThreshold is the number of distinct nodes that must mention a
	// term (with no node of its own) before it is reported as a coverage gap.
	// Zero means the default (3).
	CoverageThreshold int
}

const defaultCoverageThreshold = 3

// Lint runs the deterministic, stdlib-only structural checks over a bundle and
// returns findings sorted by path then check. It never mutates the bundle.
func Lint(b *Bundle, opts LintOptions) []LintFinding {
	threshold := opts.CoverageThreshold
	if threshold <= 0 {
		threshold = defaultCoverageThreshold
	}

	var findings []LintFinding
	findings = append(findings, lintOrphans(b)...)
	findings = append(findings, lintMissingXrefs(b)...)
	findings = append(findings, lintCoverageGaps(b, threshold)...)
	findings = append(findings, lintTypeHygiene(b)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Check < findings[j].Check
	})
	return findings
}

// inboundCounts returns, for every concept node, the number of DISTINCT sources
// that link to it — including the reserved index.md (the bundle's front door).
func inboundCounts(b *Bundle) map[string]int {
	// src -> set of resolved targets, so a source linking the same target twice
	// counts once.
	counts := map[string]int{}
	for target := range b.Nodes {
		counts[target] = 0
	}

	seen := map[string]map[string]bool{} // target -> set of sources
	add := func(src, target string) {
		if _, ok := b.Nodes[target]; !ok {
			return // only count inbound to concept nodes
		}
		if src == target {
			return // a self-link does not rescue a node from orphanhood
		}
		if seen[target] == nil {
			seen[target] = map[string]bool{}
		}
		if !seen[target][src] {
			seen[target][src] = true
			counts[target]++
		}
	}

	for src, n := range b.Nodes {
		dir := filepath.Dir(src)
		for _, l := range scanNodeLinks(b, dir, n.Body) {
			if l.resolved != "" {
				add(src, l.resolved)
			}
		}
	}
	// Reserved files (index.md especially) also confer reachability.
	for src, n := range b.Reserved {
		dir := filepath.Dir(src)
		for _, l := range scanNodeLinks(b, dir, n.Body) {
			if l.resolved != "" {
				add(src, l.resolved)
			}
		}
	}
	return counts
}

func lintOrphans(b *Bundle) []LintFinding {
	counts := inboundCounts(b)
	var out []LintFinding
	for path := range b.Nodes {
		if counts[path] == 0 {
			out = append(out, LintFinding{
				Check:   "orphan",
				Path:    path,
				Message: fmt.Sprintf("orphan: %s has no inbound links (unreachable by traversal)", path),
			})
		}
	}
	return out
}

// linkedTargets returns the set of node paths a given node already links to.
func linkedTargets(b *Bundle, path string, n *Node) map[string]bool {
	out := map[string]bool{}
	dir := filepath.Dir(path)
	for _, l := range scanNodeLinks(b, dir, n.Body) {
		if l.resolved != "" {
			out[l.resolved] = true
		}
	}
	return out
}

func lintMissingXrefs(b *Bundle) []LintFinding {
	// Map every concept node's title (lowercased) to its path.
	titleToPath := map[string]string{}
	for path, n := range b.Nodes {
		t := strings.ToLower(strings.TrimSpace(nodeTitle(n)))
		if t != "" {
			titleToPath[t] = path
		}
	}

	var out []LintFinding
	for path, n := range b.Nodes {
		body := strings.ToLower(n.Body)
		linked := linkedTargets(b, path, n)
		// Deterministic order over titles.
		var titles []string
		for t := range titleToPath {
			titles = append(titles, t)
		}
		sort.Strings(titles)
		for _, title := range titles {
			target := titleToPath[title]
			if target == path {
				continue // a node mentioning its own title is not a missing xref
			}
			if linked[target] {
				continue // already links to it
			}
			if containsWord(body, title) {
				out = append(out, LintFinding{
					Check:   "missing-xref",
					Path:    path,
					Message: fmt.Sprintf("missing-xref: %s mentions %q but does not link to %s", path, nodeTitle(b.Nodes[target]), target),
				})
			}
		}
	}
	return out
}

// containsWord reports whether needle appears in haystack as a whole-phrase
// occurrence (bounded by non-alphanumeric chars or string edges). Both args are
// expected lowercased by the caller.
func containsWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	from := 0
	for {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(needle)
		leftOK := start == 0 || !isWordByte(haystack[start-1])
		rightOK := end == len(haystack) || !isWordByte(haystack[end])
		if leftOK && rightOK {
			return true
		}
		from = start + 1
	}
}

func isWordByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// lintCoverageGaps reports terms mentioned as plain text by `threshold` or more
// DISTINCT nodes that have no node of their own. Candidate terms are the
// Title-Case multi-word phrases appearing in node bodies; a term that resolves
// to an existing node title is covered (not a gap).
func lintCoverageGaps(b *Bundle, threshold int) []LintFinding {
	// Existing node titles (lowercased) are "covered".
	covered := map[string]bool{}
	for _, n := range b.Nodes {
		if t := strings.ToLower(strings.TrimSpace(nodeTitle(n))); t != "" {
			covered[t] = true
		}
	}

	// term (lowercased) -> set of distinct node paths that mention it;
	// display (lowercased term) -> canonical display form (first seen).
	mentions := map[string]map[string]bool{}
	display := map[string]string{}
	for path, n := range b.Nodes {
		for _, term := range candidateTerms(n.Body) {
			key := strings.ToLower(term)
			if covered[key] {
				continue // already has a node
			}
			if mentions[key] == nil {
				mentions[key] = map[string]bool{}
				display[key] = term
			}
			mentions[key][path] = true
		}
	}

	var out []LintFinding
	keys := make([]string, 0, len(mentions))
	for k := range mentions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(mentions[k]) >= threshold {
			out = append(out, LintFinding{
				Check:   "coverage-gap",
				Path:    "",
				Message: fmt.Sprintf("coverage-gap: %q is mentioned by %d nodes but has no node of its own", display[k], len(mentions[k])),
			})
		}
	}
	return out
}

// candidateTerms extracts Title-Case multi-word phrases from a body (e.g.
// "Malolactic Fermentation", "Terroir"). A phrase is a run of consecutive
// capitalized words. Single capitalized words are included (e.g. "Terroir").
func candidateTerms(body string) []string {
	var terms []string
	seen := map[string]bool{}
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			term := strings.Join(cur, " ")
			if !seen[term] {
				seen[term] = true
				terms = append(terms, term)
			}
			cur = nil
		}
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue // skip headings (a node's own title heading is noise)
		}
		for _, w := range strings.Fields(line) {
			trimmed := strings.Trim(w, ".,;:!?()[]\"'`*_")
			if isCapitalizedWord(trimmed) {
				cur = append(cur, trimmed)
			} else {
				flush()
			}
		}
		flush()
	}
	return terms
}

func isCapitalizedWord(w string) bool {
	if w == "" {
		return false
	}
	if w[0] < 'A' || w[0] > 'Z' {
		return false
	}
	for i := 1; i < len(w); i++ {
		c := w[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// lintTypeHygiene warns when two or more distinct `type` values fold to the same
// canonical form (case + trailing-'s' plural), which usually signals accidental
// drift. Anti-taxonomy stands: genuinely distinct types are never flagged.
func lintTypeHygiene(b *Bundle) []LintFinding {
	// canonical -> set of raw type values.
	groups := map[string]map[string]bool{}
	for _, n := range b.Nodes {
		raw := strings.TrimSpace(n.Type())
		if raw == "" {
			continue
		}
		c := canonType(raw)
		if groups[c] == nil {
			groups[c] = map[string]bool{}
		}
		groups[c][raw] = true
	}

	var out []LintFinding
	canons := make([]string, 0, len(groups))
	for c := range groups {
		canons = append(canons, c)
	}
	sort.Strings(canons)
	for _, c := range canons {
		if len(groups[c]) > 1 {
			variants := make([]string, 0, len(groups[c]))
			for v := range groups[c] {
				variants = append(variants, v)
			}
			sort.Strings(variants)
			out = append(out, LintFinding{
				Check:   "type-hygiene",
				Path:    "",
				Message: fmt.Sprintf("type-hygiene: near-duplicate type values likely refer to one type: %s", strings.Join(variants, ", ")),
			})
		}
	}
	return out
}

// canonType folds a type value to case-insensitive + singular (drop a single
// trailing 's') for near-duplicate grouping.
func canonType(s string) string {
	c := strings.ToLower(strings.TrimSpace(s))
	if len(c) > 1 && strings.HasSuffix(c, "s") {
		c = c[:len(c)-1]
	}
	return c
}
