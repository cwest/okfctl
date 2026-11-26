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
	"strings"
)

// listFields are the frontmatter fields OKF v0.3 §11.2 rule 2 names as lists.
// A consumer MUST accept one item in place of a list.
var listFields = []string{"tags", "sources", "verified", "parameters"}

// normalizeConsumerValues applies the OKF v0.3 §11.2 consumer rules for values
// that are absent, empty, or have an unexpected shape to a concept's top-level
// frontmatter, in place, once at load time, so every reader and command sees
// the same value:
//
//   - Rule 1: a null value, an empty string, and an empty list are the same as
//     an absent key, so the key is removed. Only those three forms count:
//     false, 0, a whitespace string, and an empty mapping are values and stay.
//   - Rule 2: a list field holding one item reads as a one-item list. A scalar
//     `tags` value becomes one tag and is never split at commas.
//
// Rule 3 (ignore an entry missing its REQUIRED key, and warn) is per-family and
// lives with the family's reader (Sources) and lint (lintSourceMissingResource).
// The map is the parsed copy only; writers re-read the file, so source bytes
// are never rewritten by this.
func normalizeConsumerValues(fm map[string]any) {
	for k, v := range fm {
		if isAbsentValue(v) {
			delete(fm, k) // §11.2 rule 1.
		}
	}
	for _, k := range listFields {
		v, ok := fm[k]
		if !ok {
			continue
		}
		if _, isList := v.([]any); !isList {
			fm[k] = []any{v} // §11.2 rule 2.
		}
	}
}

// isAbsentValue reports whether v is one of the three forms §11.2 rule 1 makes
// equal to an absent key: null, the empty string, or the empty list.
func isAbsentValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	}
	return false
}

// lintSourceMissingResource reports each `sources` entry that has no `resource`
// key (or a null/empty one). §5.1 makes `resource` REQUIRED in an entry, and
// §11.2 rule 3 says a consumer ignores such an entry and SHOULD warn about it:
// Sources() does the ignoring; this is the warning. It is advisory lint, never
// a validate finding, because §11.2 keeps the bundle conformant.
//
// Only mapping entries are checked. A bare-string entry is the pre-v0.2 flat
// form, which other readers already treat as present-but-unstructured; it is
// not a family entry with a missing key, so warning on it would be noise.
func lintSourceMissingResource(b *Bundle) []LintFinding {
	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		raw, _ := b.Nodes[p].Frontmatter["sources"].([]any)
		for i, item := range raw {
			m, ok := asStringMap(item)
			if !ok {
				continue
			}
			if strings.TrimSpace(stringField(m, "resource")) != "" {
				continue
			}
			label := fmt.Sprintf("#%d", i+1)
			if id := stringField(m, "id"); id != "" {
				label = fmt.Sprintf("%s (id %q)", label, id)
			}
			out = append(out, LintFinding{
				Check: "source-missing-resource",
				Path:  p,
				Message: fmt.Sprintf(
					"source-missing-resource: %s sources entry %s has no resource, so it is ignored (§5.1 requires resource; §11.2 rule 3)",
					p, label),
			})
		}
	}
	return out
}
