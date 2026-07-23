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

// Finding is a single spec-floor violation. Path is bundle-relative.
type Finding struct {
	Path    string
	Message string
}

// Validate enforces the OKF spec floor only (PRD §6.2, §7.1):
//   - frontmatter must be parseable (nil frontmatter == parse failure);
//   - every concept node has a non-empty `type` (§7 rule 2).
//
// It never enforces a taxonomy of type VALUES (§7.4): unknown types pass.
// It returns findings; an empty slice means the bundle passes the floor.
func Validate(b *Bundle) []Finding {
	var out []Finding
	// Iterate node paths in sorted order so findings are deterministic: this is a
	// conformance tool whose output is diffed, and Go map iteration order is
	// randomized. (The node list already sorts at cmd/node.go.)
	paths := make([]string, 0, len(b.Nodes))
	for path := range b.Nodes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		n := b.Nodes[path]
		if n.Frontmatter == nil {
			out = append(out, Finding{Path: path, Message: "unparseable frontmatter"})
			continue
		}
		if strings.TrimSpace(n.Type()) == "" {
			out = append(out, Finding{Path: path, Message: "missing or empty required field: type"})
		}
	}
	return out
}
