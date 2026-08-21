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

import "github.com/cwest/okfctl/internal/okf"

// Domain types are re-exported as Go type ALIASES, not copies: a value of a
// facade type IS a value of the internal type (identical type identity), so
// there is exactly one Bundle/Node/Finding in a program and the facade can never
// hold a definition that drifts from the tool's. See doc.go for the contract.
type (
	// Bundle is a loaded OKF bundle: concept nodes keyed by bundle-relative
	// path, plus reserved files, plus the derived link graph.
	Bundle = okf.Bundle
	// Node is a single parsed bundle file (frontmatter + body).
	Node = okf.Node
	// LoadOption configures Load.
	LoadOption = okf.LoadOption

	// Finding is a single spec-floor violation returned by Validate.
	Finding = okf.Finding

	// LintFinding is one curation-guidance observation returned by Lint.
	LintFinding = okf.LintFinding
	// LintOptions configures Lint's deterministic structural checks.
	LintOptions = okf.LintOptions

	// Graph is a serializable view of a bundle's concept-node link graph.
	Graph = okf.Graph
	// GraphNode is a single concept node in the graph.
	GraphNode = okf.GraphNode
	// GraphEdge is a resolved in-bundle link from one node to another.
	GraphEdge = okf.GraphEdge

	// SearchField restricts which surfaces a lexical query matches.
	SearchField = okf.SearchField
	// SearchResult is one lexical hit.
	SearchResult = okf.SearchResult
	// NeighborResult is one node reached by graph-structural traversal.
	NeighborResult = okf.NeighborResult

	// AnalyzeOptions configures the read-only curation report.
	AnalyzeOptions = okf.AnalyzeOptions
	// AnalyzeReport is the structured five-dimension curation report.
	AnalyzeReport = okf.AnalyzeReport
)

// SearchField values, re-exported so a consumer selects a match surface without
// importing the internal package.
const (
	// FieldAny matches title, tag, type, or body substring (the default).
	FieldAny = okf.FieldAny
	// FieldTitle matches only the node title.
	FieldTitle = okf.FieldTitle
	// FieldTag matches only a node's frontmatter tags.
	FieldTag = okf.FieldTag
	// FieldType matches only the node's type value.
	FieldType = okf.FieldType
	// FieldBody matches only the node's body substring.
	FieldBody = okf.FieldBody
)

// SpecVersion is the OKF spec version this build targets.
const SpecVersion = okf.SpecVersion

// WithNoIgnore restores the full bundle walk (no directory is skipped). It is
// the escape hatch for a bundle that authored real content into a directory
// whose name matches the default skip list.
func WithNoIgnore() LoadOption { return okf.WithNoIgnore() }

// DefaultAnalyzeOptions returns the report defaults (180-day staleness, 0.5
// time-sensitive fraction, 15-line thin threshold, 3-node cluster minimum).
func DefaultAnalyzeOptions() AnalyzeOptions { return okf.DefaultAnalyzeOptions() }

// IsReservedPath reports whether a bundle-relative slash path names a reserved
// file (index.md or log.md) at any depth.
func IsReservedPath(rel string) bool { return okf.IsReservedPath(rel) }

// Load walks root, parses every .md file, and builds the in-memory graph. By
// default the walk prunes vendored/derived directories; pass WithNoIgnore to
// restore the full walk.
func Load(root string, opts ...LoadOption) (*Bundle, error) { return okf.Load(root, opts...) }

// Validate enforces the OKF spec floor (§6.2, §7.1) and returns findings; an
// empty slice means the bundle passes the floor. It never enforces a taxonomy of
// type VALUES (§7.4 leaves them open) and never mutates the bundle.
func Validate(b *Bundle) []Finding { return okf.Validate(b) }

// Lint runs the deterministic, stdlib-only structural curation checks and returns
// findings sorted by path then check. Lint findings are guidance, never spec-floor
// failures. It never mutates the bundle.
func Lint(b *Bundle, opts LintOptions) []LintFinding { return okf.Lint(b, opts) }

// BuildGraph derives the serializable link graph from a loaded bundle (nodes
// sorted by path, edges by from/to). Read-only.
func BuildGraph(b *Bundle) Graph { return okf.BuildGraph(b) }

// Search runs a case-insensitive lexical query over the bundle's concept nodes,
// restricted to field (FieldAny searches all surfaces). Reserved files never
// match; results are sorted by path. Read-only.
func Search(b *Bundle, query string, field SearchField) []SearchResult {
	return okf.Search(b, query, field)
}

// Neighborhood returns the concept nodes within depth hops of start in the link
// graph, treating edges as undirected. The start node is excluded; an unknown
// start returns (nil, false). Read-only.
func Neighborhood(b *Bundle, start string, depth int) ([]NeighborResult, bool) {
	return okf.Neighborhood(b, start, depth)
}

// Analyze runs the read-only five-dimension curation report over a loaded bundle.
// It never mutates the bundle and never fails on findings — the caller decides
// exit semantics.
func Analyze(b *Bundle, opts AnalyzeOptions) AnalyzeReport { return okf.Analyze(b, opts) }
