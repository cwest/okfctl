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
	"path/filepath"
	"sort"
	"strings"
)

// Finding is a single spec-floor violation. Path is bundle-relative.
type Finding struct {
	Path    string
	Message string
}

// Validate enforces the OKF spec floor (PRD §6.2, §7.1):
//   - frontmatter must be parseable (nil frontmatter == parse failure);
//   - every concept node has a non-empty `type` (§7 rule 2);
//   - reserved index.md files carry no frontmatter, with the single §12
//     carve-out for an okf_version-only block on the bundle-root index.
//
// It never enforces a taxonomy of type VALUES (§7.4): unknown types pass.
// It returns findings; an empty slice means the bundle passes the floor.
//
// The index-frontmatter rule closes the loop: okfctl generates index.md, so
// its own validator must reject an index that violates §8/§12 — otherwise a
// generator regression (e.g. re-introducing `type: Index`) passes validation
// unnoticed, which is the exact defect this floor exists to catch.
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
	out = append(out, validateReserved(b)...)
	return out
}

// validateReserved enforces the §8/§12 frontmatter rule on reserved index.md
// files. Log files are prose and carry no frontmatter constraint at the floor,
// so only index.md is checked here. Findings are emitted in sorted path order
// to keep validate's output stable for diffing.
//
// The rule per file:
//   - index.md with no frontmatter block: conformant (§8).
//   - bundle-root index.md ("index.md"): MAY carry a frontmatter block, but it
//     must contain the okf_version key and NOTHING else (§12's version-only
//     carve-out). Any other key, or okf_version absent from a present block,
//     is a violation.
//   - any non-root index.md (e.g. wine/index.md): NO frontmatter is permitted
//     at all — §12's carve-out is bundle-root-only, so any frontmatter block
//     there is a violation.
func validateReserved(b *Bundle) []Finding {
	paths := make([]string, 0, len(b.Reserved))
	for path := range b.Reserved {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var out []Finding
	for _, path := range paths {
		n := b.Reserved[path]
		if filepath.Base(path) != "index.md" {
			continue
		}
		if n.Frontmatter == nil {
			out = append(out, Finding{Path: path, Message: "unparseable frontmatter"})
			continue
		}
		if len(n.Frontmatter) == 0 {
			continue // §8: no frontmatter — conformant.
		}
		if path != "index.md" {
			// §12's carve-out is bundle-root-only; a non-root index may carry
			// no frontmatter at all.
			out = append(out, Finding{Path: path, Message: "index files contain no frontmatter (§8); frontmatter is permitted only on the bundle-root index and only for okf_version (§12)"})
			continue
		}
		// Bundle-root index: the block must be exactly {okf_version}.
		for _, key := range extraIndexKeys(n.Frontmatter) {
			out = append(out, Finding{Path: path, Message: "bundle-root index frontmatter may contain only okf_version (§12); found disallowed key: " + key})
		}
	}
	return out
}

// extraIndexKeys returns the sorted frontmatter keys of a bundle-root index
// that are NOT the sanctioned okf_version carve-out (§12). An empty result
// means the block is conformant.
func extraIndexKeys(fm map[string]any) []string {
	var extra []string
	for k := range fm {
		if k != "okf_version" {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return extra
}
