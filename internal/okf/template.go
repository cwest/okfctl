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
			if !hasNonEmptyField(n.Frontmatter, field) {
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

// hasNonEmptyField reports whether fm carries key with a non-blank string value.
func hasNonEmptyField(fm map[string]any, key string) bool {
	v, ok := fm[key]
	if !ok {
		return false
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) != ""
	}
	// A non-string value (list, number, bool) counts as present.
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
