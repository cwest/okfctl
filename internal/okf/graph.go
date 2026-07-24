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

import "sort"

// Graph is a serializable view of a bundle's concept-node link graph. Reserved
// files (index.md/log.md) are not graph nodes, but their outbound links confer
// inbound reachability for orphan detection (consistent with lint).
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode is a single concept node in the graph.
type GraphNode struct {
	Path         string `json:"path"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Neighborhood string `json:"neighborhood"`
	Orphan       bool   `json:"orphan"`
}

// GraphEdge is a resolved in-bundle link from one concept node to another.
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// BuildGraph derives the serializable graph from a loaded bundle. Nodes are
// sorted by path; edges by (from, to). The orphan flag reuses inboundCounts —
// the same inbound source of truth lint uses — so graph and lint can never
// disagree about what is orphaned.
func BuildGraph(b *Bundle) Graph {
	inbound := inboundCounts(b)

	paths := make([]string, 0, len(b.Nodes))
	for p := range b.Nodes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	g := Graph{}
	for _, p := range paths {
		n := b.Nodes[p]
		g.Nodes = append(g.Nodes, GraphNode{
			Path:         p,
			Title:        nodeTitle(n),
			Type:         n.Type(),
			Neighborhood: neighborhood(p),
			Orphan:       inbound[p] == 0,
		})
	}

	for _, from := range paths {
		outs := append([]string(nil), b.OutboundLinks(from)...)
		sort.Strings(outs)
		for _, to := range outs {
			// Only edges between concept nodes (targets that are real nodes).
			if _, ok := b.Nodes[to]; !ok {
				continue
			}
			g.Edges = append(g.Edges, GraphEdge{From: from, To: to})
		}
	}
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return g.Edges[i].From < g.Edges[j].From
		}
		return g.Edges[i].To < g.Edges[j].To
	})

	return g
}
