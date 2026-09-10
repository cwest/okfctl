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

// IndexShapeOptions configures the tool-owned shape suffix appended to every
// subdirectory entry of a generated index (OKF §8: index files exist for
// progressive disclosure — letting a reader decide which branch to open WITHOUT
// opening it). The suffix is trailing parenthesised text on the same bullet, so
// a consumer reading only the §8 `[Title](url) - description` grammar loses
// nothing and Validate is untouched.
//
// The zero value has Enabled=false (no suffix); use DefaultShapeOptions for the
// on-by-default behavior. Because the shape is rendered by RenderDirIndex — the
// single body producer shared by build, check, and the create/edit/delete/rename
// maintenance path — all three MUST render with identical options or `index
// check` would perpetually report drift. DefaultShapeOptions is that shared
// default; only an explicit CLI flag deviates, and it deviates symmetrically
// across build and check.
type IndexShapeOptions struct {
	Enabled bool // render the shape suffix at all (--no-shape sets this false)
	TagMin  int  // a tag prints only when >= TagMin of the dir's own concepts carry it
	TagMax  int  // at most TagMax tags print (most-shared-first, ellipsis on truncation)
}

// DefaultShapeOptions is the on-by-default shape configuration: enabled, tags
// shown when shared by >= 2 of a directory's own immediate concepts, capped at
// 12 (most-shared-first). This is the single default all render paths use, so
// build/check/maintenance stay byte-identical.
func DefaultShapeOptions() IndexShapeOptions {
	return IndexShapeOptions{Enabled: true, TagMin: 2, TagMax: 12}
}

// subtreeConceptCount returns the number of concept nodes anywhere under dir
// (dir itself and every descendant directory). For the bundle root ("") this is
// the whole bundle.
func subtreeConceptCount(b *Bundle, dir string) int {
	n := 0
	prefix := dir + "/"
	for p := range b.Nodes {
		if dir == "" || strings.HasPrefix(p, prefix) {
			n++
		}
	}
	return n
}

// foldTags collapses the given per-node tag observations into folded tags keyed
// case-insensitively. It returns, for each fold key, the number of DISTINCT
// concept nodes carrying it (nodeCount) and the display casing to render — the
// casing carried by the most nodes, ties broken by lexicographic order. The
// fold key is the lower-cased tag, so `api`/`API` fold together while `v1`/`v2`
// (which differ in a non-case character) stay distinct.
type foldedTag struct {
	display   string
	nodeCount int
}

func foldTags(perNodeTags [][]string) map[string]foldedTag {
	// nodeCount[key] = number of distinct nodes carrying any casing of key.
	nodeCount := map[string]int{}
	// casingCount[key][casing] = number of nodes carrying that exact casing.
	casingCount := map[string]map[string]int{}
	for _, tags := range perNodeTags {
		seen := map[string]bool{}
		for _, tg := range tags {
			key := strings.ToLower(tg)
			if casingCount[key] == nil {
				casingCount[key] = map[string]int{}
			}
			casingCount[key][tg]++
			if !seen[key] {
				seen[key] = true
				nodeCount[key]++
			}
		}
	}
	out := make(map[string]foldedTag, len(nodeCount))
	for key, n := range nodeCount {
		// Dominant casing: most nodes, tie -> alphabetically smallest.
		best := ""
		bestN := -1
		for casing, c := range casingCount[key] {
			if c > bestN || (c == bestN && casing < best) {
				best, bestN = casing, c
			}
		}
		out[key] = foldedTag{display: best, nodeCount: n}
	}
	return out
}

// sharedTagList returns the folded tags carried by >= min of the directory's own
// concepts, ordered most-shared-first with an alphabetical (by display casing)
// tie-break, capped at max. The returned bool reports whether the list was
// truncated (more qualifying tags existed than the cap allowed), which drives
// the ellipsis.
func sharedTagList(folded map[string]foldedTag, min, max int) ([]string, bool) {
	type entry struct {
		display string
		count   int
	}
	var qualifying []entry
	for _, ft := range folded {
		if ft.nodeCount >= min {
			qualifying = append(qualifying, entry{ft.display, ft.nodeCount})
		}
	}
	sort.Slice(qualifying, func(i, j int) bool {
		if qualifying[i].count != qualifying[j].count {
			return qualifying[i].count > qualifying[j].count
		}
		return qualifying[i].display < qualifying[j].display
	})
	truncated := false
	if max >= 0 && len(qualifying) > max {
		qualifying = qualifying[:max]
		truncated = true
	}
	out := make([]string, len(qualifying))
	for i, e := range qualifying {
		out[i] = e.display
	}
	return out, truncated
}

// dirShape returns the tool-owned shape suffix for a subdirectory entry (OKF §8
// progressive disclosure), as trailing parenthesised text with a leading space,
// e.g. ` (37 concepts · Application, Concept, Map · shared tags: ux, layout) · 46 in subtree`.
// It aggregates over the child directory's OWN IMMEDIATE concept nodes (not the
// whole subtree): the count, the distinct types in full (sorted, §7.4 leaves the
// type vocabulary open so every type prints as-is), and the shared tags (§4.1).
// The `· N in subtree` total is appended only when the child has content-bearing
// descendants (subtree count > own count). Returns "" when disabled.
//
// The suffix uses `·` (U+00B7) as the in-bullet separator; that is inside a
// Markdown list item (not a prose block), so it is not subject to the prose
// em-dash house style. Output is byte-stable: every ordering is via sort and no
// clock is read.
func dirShape(b *Bundle, childDir string, opts IndexShapeOptions) string {
	if !opts.Enabled {
		return ""
	}

	// Own immediate concepts of childDir.
	ownPaths := conceptsIn(b, childDir) // sorted, deterministic
	own := len(ownPaths)

	typeSet := map[string]bool{}
	perNodeTags := make([][]string, 0, own)
	for _, p := range ownPaths {
		n := b.Nodes[p]
		if t := strings.TrimSpace(n.Type()); t != "" {
			typeSet[t] = true
		}
		perNodeTags = append(perNodeTags, nodeTags(n))
	}

	// Segment 1: the concept count (pluralized).
	var segs []string
	segs = append(segs, fmt.Sprintf("%d %s", own, pluralConcepts(own)))

	// Segment 2: types in full, sorted, deduped.
	if len(typeSet) > 0 {
		types := make([]string, 0, len(typeSet))
		for t := range typeSet {
			types = append(types, t)
		}
		sort.Strings(types)
		segs = append(segs, strings.Join(types, ", "))
	}

	// Segment 3: shared tags (omitted entirely when none qualifies).
	folded := foldTags(perNodeTags)
	tags, truncated := sharedTagList(folded, opts.TagMin, opts.TagMax)
	if len(tags) > 0 {
		seg := "shared tags: " + strings.Join(tags, ", ")
		if truncated {
			seg += ", …"
		}
		segs = append(segs, seg)
	}

	suffix := " (" + strings.Join(segs, " · ") + ")"

	// Subtree total: only when the child has deeper content-bearing dirs.
	if sub := subtreeConceptCount(b, childDir); sub > own {
		suffix += fmt.Sprintf(" · %d in subtree", sub)
	}
	return suffix
}

// pluralConcepts returns "concept" for exactly 1 and "concepts" otherwise
// (including 0, matching English usage: "0 concepts").
func pluralConcepts(n int) string {
	if n == 1 {
		return "concept"
	}
	return "concepts"
}
