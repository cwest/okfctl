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

// templateType is the reserved type value that marks a node as a type template
// (PRD §9.2). A template is an ordinary OKF node; nothing about it lives in the
// tool's config.
const templateType = "Type Template"

// Template is a parsed type-template node (PRD §9.2). It governs nodes whose
// type equals TargetType.
type Template struct {
	TargetType        string
	RequiredFields    []string
	RecommendedFields []string
	BodySections      []string
	Path              string // bundle-relative path of the template node
}

// Templates folds every `type: Type Template` node in the bundle into a map
// keyed by the target_type it governs. A bundle should not ship two templates
// for one target_type; if it does, the lexicographically-last path wins (stable,
// since paths are visited in sorted order).
func Templates(b *Bundle) map[string]Template {
	paths := sortedNodePaths(b)
	out := make(map[string]Template)
	for _, path := range paths {
		n := b.Nodes[path]
		if n.Frontmatter == nil || strings.TrimSpace(n.Type()) != templateType {
			continue
		}
		target, _ := n.Frontmatter["target_type"].(string)
		if strings.TrimSpace(target) == "" {
			continue
		}
		out[target] = Template{
			TargetType:        target,
			RequiredFields:    stringSlice(n.Frontmatter, "required_fields"),
			RecommendedFields: stringSlice(n.Frontmatter, "recommended_fields"),
			BodySections:      stringSlice(n.Frontmatter, "body_sections"),
			Path:              path,
		}
	}
	return out
}

// DriftFinding is a single template-overlay violation (PRD §9.4). It is a
// warning-class finding, never a spec-floor failure.
type DriftFinding struct {
	Path       string
	TargetType string
	Message    string
}

// TemplateDrift reports where nodes diverge from the template governing their
// type (PRD §9.4): a required field missing/empty, or a body_section heading
// absent. recommended_fields are advisory and never reported here. A node whose
// type has no governing template never drifts (unknown types are fine, §7.4).
// Output is deterministic (sorted by node path, then finding order).
func TemplateDrift(b *Bundle) []DriftFinding {
	tmpls := Templates(b)
	if len(tmpls) == 0 {
		return nil
	}
	var out []DriftFinding
	for _, path := range sortedNodePaths(b) {
		n := b.Nodes[path]
		if n.Frontmatter == nil {
			continue
		}
		typ := strings.TrimSpace(n.Type())
		if typ == templateType {
			continue // a template node is not governed by itself
		}
		t, ok := tmpls[typ]
		if !ok {
			continue
		}
		for _, field := range t.RequiredFields {
			if !hasNonEmptyField(n, field) {
				out = append(out, DriftFinding{
					Path:       path,
					TargetType: typ,
					Message:    fmt.Sprintf("missing required field: %s (template %s)", field, typ),
				})
			}
		}
		for _, section := range t.BodySections {
			if !hasSection(n.Body, section) {
				out = append(out, DriftFinding{
					Path:       path,
					TargetType: typ,
					Message:    fmt.Sprintf("missing body section: %s (template %s)", section, typ),
				})
			}
		}
	}
	return out
}

// TemplateScaffold returns the required fields to stub as empty frontmatter keys
// and the body sections to lay down as empty `## ` headings for a node created
// from t (PRD §9.3). recommended_fields are stubbed alongside required ones.
func TemplateScaffold(t Template) (fields []string, sections []string) {
	fields = append(fields, t.RequiredFields...)
	fields = append(fields, t.RecommendedFields...)
	return fields, t.BodySections
}

// sortedNodePaths returns the bundle's concept-node paths in sorted order, so
// every template pass is deterministic (Go map iteration is randomized).
func sortedNodePaths(b *Bundle) []string {
	paths := make([]string, 0, len(b.Nodes))
	for path := range b.Nodes {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// hasNonEmptyField reports whether n satisfies a template's required field. It
// resolves v0.2 nested shapes the flat frontmatter lookup could not express
// (§5.1, §5.2), with the §13.1 legacy fallback wired BIDIRECTIONALLY so a
// v0.1-authored template does not false-positive drift a migrated v0.2 node,
// and a v0.1 node still satisfies a v0.2-authored template:
//
//   - `sources` / the dotted head `sources` → Node.SourceCitations() > 0
//     (frontmatter `sources` first, legacy body `# Citations` fallback, §13.1).
//   - `generated.at` (and the legacy `timestamp`) → Node.Generated() ok
//     (frontmatter `generated.at` first, legacy `timestamp` fallback, §13.1).
//   - any other dotted path `a.b.c` → literal nested-map traversal.
//   - a plain key → the original flat non-blank frontmatter lookup.
//
// The overlay stays warning-class and adds no required vocabulary (§7.4, §11);
// it only teaches the existing required_fields check to read v0.2 provenance.
func hasNonEmptyField(n *Node, key string) bool {
	switch key {
	// §5.1 + §13.1: `sources` is satisfied by frontmatter `sources` OR a legacy
	// body `# Citations` list. Bidirectional: a v0.1 node satisfies it too.
	case "sources":
		return n.SourceCitations() > 0
	// §5.2 + §13.1: `generated.at` is satisfied by `generated.at` OR the legacy
	// flat `timestamp`. The reverse — a legacy `[timestamp]` template against a
	// migrated node — is handled by the `timestamp` case below.
	case "generated.at":
		_, ok := n.Generated()
		return ok
	// §13.1 reverse: a v0.1-authored template requiring the legacy `timestamp`
	// resolves against a node that migrated to `generated.at` — this is the
	// load-bearing case that stops false-positive drift on a migrated corpus.
	case "timestamp":
		_, ok := n.Generated()
		return ok
	}
	if strings.Contains(key, ".") {
		return dottedPathNonEmpty(n.Frontmatter, key)
	}
	return flatFieldNonEmpty(n.Frontmatter, key)
}

// dottedPathNonEmpty walks a dotted path (`a.b.c`) through nested frontmatter
// maps and reports whether the leaf resolves to a non-blank value.
func dottedPathNonEmpty(fm map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	var cur any = fm
	for i, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			// YAML may decode nested maps as map[any]any; normalize one level.
			if am, ok := cur.(map[any]any); ok {
				m = make(map[string]any, len(am))
				for k, v := range am {
					if ks, ok := k.(string); ok {
						m[ks] = v
					}
				}
			} else {
				return false
			}
		}
		v, ok := m[part]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			return valueNonEmpty(v)
		}
		cur = v
	}
	return false
}

// flatFieldNonEmpty is the original v0.1 lookup: fm[key] present and non-blank.
func flatFieldNonEmpty(fm map[string]any, key string) bool {
	v, ok := fm[key]
	if !ok {
		return false
	}
	return valueNonEmpty(v)
}

// valueNonEmpty reports whether a resolved value counts as present: a non-blank
// string, or any non-nil non-string (list, number, bool, map).
func valueNonEmpty(v any) bool {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	return v != nil
}

// hasSection reports whether body contains a markdown heading (## .. ######) whose
// text equals name (case-insensitive, trimmed).
func hasSection(body, name string) bool {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if strings.ToLower(heading) == want {
			return true
		}
	}
	return false
}

// stringSlice reads fm[key] as a []string, tolerating a YAML []any of strings.
func stringSlice(fm map[string]any, key string) []string {
	raw, ok := fm[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
