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
	"sort"
	"strings"
)

// SearchField names a lexical match surface. A lexical query with no field
// restriction matches ANY of these; a field-restricted query matches only the
// named one. This is the core-search surface of PRD §6.3: title, tag, type, and
// content substring, all case-insensitive.
type SearchField string

const (
	// FieldAny matches title, tag, type, or body substring (the default).
	FieldAny SearchField = ""
	// FieldTitle matches only the node title (frontmatter title, or file base).
	FieldTitle SearchField = "title"
	// FieldTag matches only a node's frontmatter tags.
	FieldTag SearchField = "tag"
	// FieldType matches only the node's type value.
	FieldType SearchField = "type"
	// FieldBody matches only the node's body substring.
	FieldBody SearchField = "body"
)

// SearchResult is one lexical hit: the matched node plus the fields the query
// matched on (sorted, deduped), so a caller can explain WHY a node matched.
type SearchResult struct {
	Path         string   `json:"path"`
	Title        string   `json:"title"`
	Type         string   `json:"type"`
	Neighborhood string   `json:"neighborhood"`
	MatchedOn    []string `json:"matched_on"`
}

// Search runs a case-insensitive lexical query over a bundle's concept nodes.
// Reserved files (index.md/log.md) are never search results. An empty query
// returns no results (a lexical search needs a term). Results are sorted by
// path for deterministic output. field restricts the match surface; FieldAny
// searches title, tag, type, and body substring together.
func Search(b *Bundle, query string, field SearchField) []SearchResult {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	paths := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var out []SearchResult
	for _, p := range paths {
		n := b.Nodes[p]
		matched := matchedFields(n, q, field)
		if len(matched) == 0 {
			continue
		}
		out = append(out, SearchResult{
			Path:         p,
			Title:        nodeTitle(n),
			Type:         n.Type(),
			Neighborhood: neighborhood(p),
			MatchedOn:    matched,
		})
	}
	return out
}

// matchedFields returns the sorted, deduped set of fields (among the requested
// surface) that node n matches for the lowercased query q. An empty slice means
// no match.
func matchedFields(n *Node, q string, field SearchField) []string {
	set := map[string]bool{}

	want := func(f SearchField) bool { return field == FieldAny || field == f }

	if want(FieldTitle) && strings.Contains(strings.ToLower(nodeTitle(n)), q) {
		set["title"] = true
	}
	if want(FieldType) && strings.Contains(strings.ToLower(n.Type()), q) {
		set["type"] = true
	}
	if want(FieldTag) {
		for _, t := range nodeTags(n) {
			if strings.Contains(strings.ToLower(t), q) {
				set["tag"] = true
				break
			}
		}
	}
	if want(FieldBody) && strings.Contains(strings.ToLower(n.Body), q) {
		set["body"] = true
	}

	if len(set) == 0 {
		return nil
	}
	fields := make([]string, 0, len(set))
	for f := range set {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	return fields
}

// NeighborResult is one node reached by graph-structural traversal from a start
// node, along with its hop distance (the start node is depth 0 and is excluded
// from results).
type NeighborResult struct {
	Path         string `json:"path"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Neighborhood string `json:"neighborhood"`
	Depth        int    `json:"depth"`
}

// Neighborhood returns the nodes within depth hops of start in the bundle's
// concept-node link graph, treating edges as UNDIRECTED: a node is a neighbor
// whether it links to start or start links to it (a reader traverses both
// ways). The start node itself is excluded. depth < 1 is treated as 1. Results
// are sorted by (depth, path) so the closest neighbors come first,
// deterministically. An unknown start path returns (nil, false).
func Neighborhood(b *Bundle, start string, depth int) ([]NeighborResult, bool) {
	if _, ok := b.Nodes[start]; !ok {
		return nil, false
	}
	if depth < 1 {
		depth = 1
	}

	adj := undirectedAdjacency(b)

	dist := map[string]int{start: 0}
	frontier := []string{start}
	for d := 1; d <= depth && len(frontier) > 0; d++ {
		var next []string
		for _, cur := range frontier {
			for _, nb := range adj[cur] {
				if _, seen := dist[nb]; seen {
					continue
				}
				dist[nb] = d
				next = append(next, nb)
			}
		}
		frontier = next
	}

	var out []NeighborResult
	for p, d := range dist {
		if p == start {
			continue
		}
		n := b.Nodes[p]
		out = append(out, NeighborResult{
			Path:         p,
			Title:        nodeTitle(n),
			Type:         n.Type(),
			Neighborhood: neighborhood(p),
			Depth:        d,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Depth != out[j].Depth {
			return out[i].Depth < out[j].Depth
		}
		return out[i].Path < out[j].Path
	})
	return out, true
}

// undirectedAdjacency builds a concept-node-only, undirected adjacency map from
// the bundle's link graph: for each in-bundle edge from->to where both ends are
// concept nodes, both directions are recorded and duplicates are collapsed.
func undirectedAdjacency(b *Bundle) map[string][]string {
	seen := map[string]map[string]bool{}
	link := func(a, c string) {
		if seen[a] == nil {
			seen[a] = map[string]bool{}
		}
		seen[a][c] = true
	}
	for from := range b.Nodes {
		for _, to := range b.OutboundLinks(from) {
			if _, ok := b.Nodes[to]; !ok {
				continue // only concept-to-concept edges
			}
			if from == to {
				continue
			}
			link(from, to)
			link(to, from)
		}
	}
	adj := make(map[string][]string, len(seen))
	for a, set := range seen {
		neighbors := make([]string, 0, len(set))
		for c := range set {
			neighbors = append(neighbors, c)
		}
		sort.Strings(neighbors)
		adj[a] = neighbors
	}
	return adj
}
