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

// Validate enforces the OKF spec floor (PRD §6.2, §7.1; OKF v0.3 §11.1):
//   - a concept's frontmatter block must parse as a YAML mapping (nil
//     frontmatter == parse failure, §11.1 rule 1);
//   - every concept node has a non-empty `type` (§4.1). Load only puts a .md in
//     Nodes when it has one (§3.2 rule 1), so this holds for loaded bundles and
//     guards hand-built ones;
//   - reserved index.md files carry no frontmatter, except the bundle-root
//     index, which MAY carry §8.1 bundle metadata and extension keys.
//
// A .md with no frontmatter or no `type` is not a concept and does not affect
// conformance (§3.2 rule 2, §11.1): it is reported by ValidateWarnings, never
// here.
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
	for _, path := range sortedPaths(b.Nodes) {
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

// ValidateWarnings returns the non-failing findings OKF v0.3 §3.2 rule 2 asks
// validators for: one per .md file that is not a concept because it has no
// frontmatter or no non-empty `type`. Such a file is skipped by every consumer
// and does not make the bundle non-conformant, so these never fail validate.
// Sorted by path for stable, diffable output.
func ValidateWarnings(b *Bundle) []Finding {
	var out []Finding
	for _, path := range sortedPaths(b.NonConcepts) {
		n := b.NonConcepts[path]
		reason := "frontmatter has no non-empty type"
		if !n.HasFrontmatterBlock {
			reason = "no frontmatter"
		}
		out = append(out, Finding{
			Path:    path,
			Message: "not a concept (" + reason + "); skipped, does not affect conformance (OKF v0.3 §3.2)",
		})
	}
	return out
}

// sortedPaths returns the keys of a node map in sorted order.
func sortedPaths(m map[string]*Node) []string {
	paths := make([]string, 0, len(m))
	for path := range m {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// validateReserved enforces the §8/§8.1 frontmatter rule on reserved index.md
// files and the §9 rule on log.md. Findings are emitted in sorted path order to
// keep validate's output stable for diffing.
//
// The index rule per file (OKF v0.3):
//   - index.md with no frontmatter block: conformant (§8).
//   - bundle-root index.md ("index.md"): MAY carry a frontmatter block of
//     bundle metadata — okf_version, title, description, license, all
//     optional — plus extension keys (§8.1, §4.1). Only a concept key the spec
//     defines (type, tags, status, ...) is a violation; see conceptOnlyKeys.
//   - any non-root index.md (e.g. wine/index.md): NO frontmatter is permitted
//     at all (§8) — §8.1 is bundle-root-only.
func validateReserved(b *Bundle) []Finding {
	paths := make([]string, 0, len(b.Reserved))
	for path := range b.Reserved {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var out []Finding
	for _, path := range paths {
		n := b.Reserved[path]
		switch filepath.Base(path) {
		case "index.md":
			out = append(out, validateReservedIndex(path, n)...)
		case "log.md":
			out = append(out, validateReservedLog(path, n)...)
		}
	}
	return out
}

// validateReservedIndex enforces the §8/§8.1 frontmatter rule on a reserved
// index.md. On a non-root index a present-but-empty block (`---\n---`) is a
// violation, not "no block": HasFrontmatterBlock distinguishes the two even
// though both parse to an empty map. On the bundle-root index an empty block is
// conformant, because every §8.1 key is optional.
func validateReservedIndex(path string, n *Node) []Finding {
	var out []Finding
	if n.Frontmatter == nil {
		return append(out, Finding{Path: path, Message: "unparseable frontmatter"})
	}
	if !n.HasFrontmatterBlock {
		return out // §8: no frontmatter block — conformant.
	}
	if path != "index.md" {
		// §8.1 bundle metadata is bundle-root-only; a non-root index may carry
		// no frontmatter block at all (empty or keyed).
		return append(out, Finding{Path: path, Message: "index files contain no frontmatter (§8); only the bundle-root index may carry frontmatter (§8.1)"})
	}
	// Bundle-root index (v0.3 §8.1): every key is optional, so an empty block
	// or one without okf_version is conformant. Only a concept key the spec
	// defines is out of place here.
	for _, key := range conceptKeysOnRootIndex(n.Frontmatter) {
		out = append(out, Finding{Path: path, Message: "bundle-root index frontmatter carries bundle metadata (okf_version, title, description, license) and extension keys only (§8.1); found concept key: " + key})
	}
	return out
}

// conceptOnlyKeys are the top-level frontmatter keys OKF v0.3 defines for
// concepts (§4.1, §5, §10) that are not also bundle-metadata keys (§8.1).
// title and description are defined for both, so they are absent here. On the
// bundle-root index these keys are neither §8.1 metadata nor extension keys —
// an extension key is one "this specification does not define" (§2) — so
// validate flags them. Every other key passes: §4.1 and §11.2 forbid rejecting
// keys the consumer does not know, and the x- prefix is only a SHOULD (§4.1).
// v0.1's legacy `timestamp` (Appendix B) is not defined by v0.3, so it is
// deliberately absent and passes as an extension key.
var conceptOnlyKeys = map[string]bool{
	"type":          true, // §4.1
	"resource":      true, // §4.1
	"tags":          true, // §4.1
	"sources":       true, // §5.1
	"usage_window":  true, // §5.1
	"generated":     true, // §5.2
	"verified":      true, // §5.2
	"status":        true, // §5.4
	"superseded_by": true, // §5.4
	"stale_after":   true, // §5.5
	"runtime":       true, // §10.2
	"parameters":    true, // §10.2
	"computation":   true, // §10.2
	"executor":      true, // §10.2
	"attester":      true, // §10.2
}

// conceptKeysOnRootIndex returns, sorted, the frontmatter keys of a bundle-root
// index that are concept keys rather than §8.1 bundle metadata or extension
// keys. An empty result means the block is conformant.
func conceptKeysOnRootIndex(fm map[string]any) []string {
	var bad []string
	for k := range fm {
		if conceptOnlyKeys[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}
