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

// Package okf is the core in-memory model for an OKF bundle.
// It must not import cobra or any CLI package.
package okf

// Node is a single OKF concept: its bundle-relative path is its identity,
// its frontmatter carries typed metadata (type is required, §7), and body
// holds the Markdown after the frontmatter block.
type Node struct {
	Path        string         // bundle-relative, e.g. "wine/tannin.md"
	Frontmatter map[string]any // parsed YAML frontmatter
	Body        string         // markdown after the frontmatter
}

// Type returns the node's type value ("" if absent or not a string).
func (n *Node) Type() string {
	if v, ok := n.Frontmatter["type"].(string); ok {
		return v
	}
	return ""
}

// Tags returns the node's frontmatter tags (§4.1), nil when absent. Tags are an
// optional YAML list; a scalar tag normalizes to a one-element list and non-string
// scalars coerce to their string form. This is the exported accessor for callers
// outside the package (e.g. semantic-search filters) that need a node's tags
// without reaching into frontmatter directly.
func (n *Node) Tags() []string {
	return nodeTags(n)
}
