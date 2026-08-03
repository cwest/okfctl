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
	"strings"
	"testing"
)

// A template node governing "Playbook": requires title/description/owner,
// body sections Trigger/Steps/Rollback/Verification.
const playbookTemplate = `---
type: Type Template
target_type: Playbook
required_fields: [title, description, owner]
recommended_fields: [tags]
body_sections: [Trigger, Steps, Rollback, Verification]
---

# Playbook Template

Governs Playbook nodes.
`

func TestTemplates_FoldsByTargetType(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"templates/playbook.md": playbookTemplate,
		"templates/recipe.md": `---
type: Type Template
target_type: Recipe
required_fields: [title]
body_sections: [Ingredients]
---

# Recipe Template
`,
		"wine/tannin.md": lintDoc("Concept", "Tannin", "Body."),
	})
	tmpls := Templates(b)
	if len(tmpls) != 2 {
		t.Fatalf("want 2 templates, got %d: %v", len(tmpls), tmpls)
	}
	pb, ok := tmpls["Playbook"]
	if !ok {
		t.Fatal("no template keyed by target_type Playbook")
	}
	if len(pb.RequiredFields) != 3 || pb.RequiredFields[2] != "owner" {
		t.Fatalf("required_fields not parsed: %v", pb.RequiredFields)
	}
	if len(pb.BodySections) != 4 || pb.BodySections[2] != "Rollback" {
		t.Fatalf("body_sections not parsed: %v", pb.BodySections)
	}
	if _, ok := tmpls["Concept"]; ok {
		t.Fatal("non-template node folded as a template")
	}
}

// mkPlaybookBundle: the Playbook template + one Playbook node with the given
// frontmatter kv lines and body.
func mkPlaybookBundle(t *testing.T, nodeFrontmatter, nodeBody string) *Bundle {
	t.Helper()
	return mkLintBundle(t, map[string]string{
		"templates/playbook.md": playbookTemplate,
		"ops/deploy.md":         "---\ntype: Playbook\n" + nodeFrontmatter + "---\n\n# Deploy\n\n" + nodeBody + "\n",
	})
}

func TestTemplateDrift_MissingRequiredField(t *testing.T) {
	// Has title, but NO description or owner.
	b := mkPlaybookBundle(t, "title: Deploy\n",
		"## Trigger\n## Steps\n## Rollback\n## Verification\n")
	d := TemplateDrift(b)
	var fields []string
	for _, f := range d {
		fields = append(fields, f.Message)
	}
	joined := ""
	for _, m := range fields {
		joined += m + "\n"
	}
	if len(d) < 2 {
		t.Fatalf("want >=2 drift findings for missing description+owner, got %d: %v", len(d), fields)
	}
	if !strings.Contains(joined, "description") || !strings.Contains(joined, "owner") {
		t.Fatalf("drift should name the missing fields, got: %v", fields)
	}
}

func TestTemplateDrift_MissingBodySection(t *testing.T) {
	// All required fields present, but body missing the Rollback section.
	b := mkPlaybookBundle(t, "title: Deploy\ndescription: d\nowner: casey\n",
		"## Trigger\n## Steps\n## Verification\n")
	d := TemplateDrift(b)
	joined := ""
	for _, f := range d {
		joined += f.Message + "\n"
	}
	if !strings.Contains(joined, "Rollback") {
		t.Fatalf("drift should report missing Rollback section, got: %v", d)
	}
}

func TestTemplateDrift_Conformant_NoFindings(t *testing.T) {
	b := mkPlaybookBundle(t, "title: Deploy\ndescription: d\nowner: casey\n",
		"## Trigger\n## Steps\n## Rollback\n## Verification\n")
	if d := TemplateDrift(b); len(d) != 0 {
		t.Fatalf("conformant node should have zero drift, got: %v", d)
	}
}

func TestTemplateDrift_NoGoverningTemplate_NoFindings(t *testing.T) {
	// A node whose type has no template — unknown types are fine (§7.4).
	b := mkLintBundle(t, map[string]string{
		"templates/playbook.md": playbookTemplate,
		"wine/tannin.md":        lintDoc("Concept", "Tannin", "Body."),
	})
	if d := TemplateDrift(b); len(d) != 0 {
		t.Fatalf("node with no governing template should not drift, got: %v", d)
	}
}

// provTemplate governs "Concept" and requires v0.2 nested provenance: the
// §5.1 sources family and the §5.2 generated.at date, expressed as the dotted
// path a template author would write.
const provTemplate = `---
type: Type Template
target_type: Concept
required_fields: [sources, generated.at]
---

# Provenance Template

Governs Concept nodes' provenance.
`

// legacyTemplate governs "Concept" but is authored against the v0.1 spelling
// (§13.1): a v0.1 author names the flat `timestamp` key it knew.
const legacyTemplate = `---
type: Type Template
target_type: Concept
required_fields: [timestamp]
---

# Legacy Template
`

// mkConceptBundle: a template + one Concept node with the given frontmatter kv
// lines and body.
func mkConceptBundle(t *testing.T, template, nodeFrontmatter, nodeBody string) *Bundle {
	t.Helper()
	return mkLintBundle(t, map[string]string{
		"templates/prov.md": template,
		"wine/tannin.md":    "---\ntype: Concept\n" + nodeFrontmatter + "---\n\n# Tannin\n\n" + nodeBody + "\n",
	})
}

// §5.1 + §5.2: a required `sources` field is satisfied by a v0.2 frontmatter
// sources list, and `generated.at` by a nested generated mapping. This is the
// shape the FLAT lookup could not express at all.
func TestTemplateDrift_V02NestedProvenance_Satisfied_Section5_1_5_2(t *testing.T) {
	b := mkConceptBundle(t, provTemplate,
		"sources:\n  - resource: https://example.com/a\ngenerated:\n  by: agent:x\n  at: '2026-05-28T00:00:00Z'\n",
		"Body.")
	if d := TemplateDrift(b); len(d) != 0 {
		t.Fatalf("v0.2 node with sources + generated.at should not drift, got: %v", d)
	}
}

// §13.1 legacy fallback (body list): a required `sources` field is satisfied by
// a v0.1 body `# Citations` list when frontmatter `sources` is absent.
func TestTemplateDrift_LegacyCitationsSatisfiesSources_Section13_1(t *testing.T) {
	b := mkConceptBundle(t, provTemplate,
		"generated:\n  at: '2026-05-28T00:00:00Z'\n",
		"Body.\n\n# Citations\n\n[1] https://example.com/a\n")
	joined := ""
	for _, f := range TemplateDrift(b) {
		joined += f.Message + "\n"
	}
	if strings.Contains(joined, "sources") {
		t.Fatalf("legacy # Citations body list should satisfy required sources (§13.1), got: %v", joined)
	}
}

// POSITIVE control: a node genuinely missing v0.2 provenance still drifts on
// both required fields.
func TestTemplateDrift_MissingV02Provenance_Drifts(t *testing.T) {
	b := mkConceptBundle(t, provTemplate, "title: Tannin\n", "Body, no citations.")
	joined := ""
	for _, f := range TemplateDrift(b) {
		joined += f.Message + "\n"
	}
	if !strings.Contains(joined, "sources") || !strings.Contains(joined, "generated.at") {
		t.Fatalf("node missing both sources and generated.at should drift on both, got: %v", joined)
	}
}

// NEGATIVE control (the load-bearing one, AGENTS.md "both controls"): a node
// that correctly migrated to `generated.at` must stay SILENT under a template
// authored against the LEGACY `timestamp` spelling — the v0.1-authored template
// does not false-positive drift a v0.2 node (§13.1, bidirectional fallback).
func TestTemplateDrift_MigratedNode_LegacyTemplate_Silent_Section13_1(t *testing.T) {
	b := mkConceptBundle(t, legacyTemplate,
		"generated:\n  by: agent:x\n  at: '2026-05-28T00:00:00Z'\n",
		"Body.")
	if d := TemplateDrift(b); len(d) != 0 {
		t.Fatalf("migrated node (generated.at) must not drift under a [timestamp] template (§13.1), got: %v", d)
	}
}

// §13.1 forward direction: a v0.1 node carrying the flat `timestamp` still
// satisfies a template requiring the v0.2 `generated.at` — the fallback is
// bidirectional, so neither spelling drifts the other.
func TestTemplateDrift_LegacyNode_V02Template_Silent_Section13_1(t *testing.T) {
	b := mkConceptBundle(t, provTemplate,
		"sources:\n  - resource: https://example.com/a\ntimestamp: '2026-05-28T00:00:00Z'\n",
		"Body.")
	joined := ""
	for _, f := range TemplateDrift(b) {
		joined += f.Message + "\n"
	}
	if strings.Contains(joined, "generated.at") {
		t.Fatalf("legacy timestamp should satisfy required generated.at (§13.1), got: %v", joined)
	}
}

// §5.1 + §13.1: a v0.1 flat-STRING `sources:` list satisfies a required
// `sources` field. The template check asks "present and non-empty", a weaker
// question than "parses to structured provenance" — a well-sourced v0.1 node
// must not be reported as missing sources. Regression guard for #79.
func TestTemplateDrift_FlatStringSourcesSatisfies_Section5_1_13_1(t *testing.T) {
	b := mkConceptBundle(t, provTemplate,
		"sources:\n  - https://example.com/a|Ref A\n  - https://example.com/b|Ref B\ngenerated:\n  at: '2026-05-28T00:00:00Z'\n",
		"Body.")
	joined := ""
	for _, f := range TemplateDrift(b) {
		joined += f.Message + "\n"
	}
	if strings.Contains(joined, "sources") {
		t.Fatalf("flat-string sources list should satisfy required sources (§5.1, §13.1), got: %v", joined)
	}
}

// §5.1: the same, with bare URLs and no `|label` — still a non-empty flat-string
// list, still satisfies the required field.
func TestTemplateDrift_BareURLSourcesSatisfies_Section5_1(t *testing.T) {
	b := mkConceptBundle(t, provTemplate,
		"sources:\n  - https://example.com/a\n  - https://example.com/b\ngenerated:\n  at: '2026-05-28T00:00:00Z'\n",
		"Body.")
	joined := ""
	for _, f := range TemplateDrift(b) {
		joined += f.Message + "\n"
	}
	if strings.Contains(joined, "sources") {
		t.Fatalf("bare-URL flat-string sources list should satisfy required sources (§5.1), got: %v", joined)
	}
}

// NEGATIVE control (load-bearing): an EMPTY `sources: []` list is present but not
// non-empty — the required field is NOT satisfied. Non-empty is required.
func TestTemplateDrift_EmptySourcesList_Drifts_Section5_1(t *testing.T) {
	b := mkConceptBundle(t, provTemplate,
		"sources: []\ngenerated:\n  at: '2026-05-28T00:00:00Z'\n",
		"Body.")
	joined := ""
	for _, f := range TemplateDrift(b) {
		joined += f.Message + "\n"
	}
	if !strings.Contains(joined, "sources") {
		t.Fatalf("empty sources: [] must still drift missing sources (§5.1), got: %v", joined)
	}
}

// NEGATIVE control (load-bearing): a `sources:` list whose every entry is
// blank/whitespace is present but not non-empty — still drifts. This is the
// case that separates a presence check from a naive len(list) > 0 check.
func TestTemplateDrift_BlankSourcesEntries_Drifts_Section5_1(t *testing.T) {
	b := mkConceptBundle(t, provTemplate,
		"sources:\n  - '   '\n  - ''\ngenerated:\n  at: '2026-05-28T00:00:00Z'\n",
		"Body.")
	joined := ""
	for _, f := range TemplateDrift(b) {
		joined += f.Message + "\n"
	}
	if !strings.Contains(joined, "sources") {
		t.Fatalf("all-blank sources entries must still drift missing sources (§5.1), got: %v", joined)
	}
}

// Criterion 9 invariant: the template presence loosening MUST NOT leak into the
// structured-provenance accessor. Node.Sources() stays strict — a flat-string
// list still parses to ZERO structured entries (§5.1: a structured source needs
// a `resource`). analyze / coverage / freshness depend on this meaning.
func TestNodeSources_FlatStringList_ParsesToZero_Section5_1(t *testing.T) {
	n := &Node{Frontmatter: map[string]any{
		"sources": []any{"https://example.com/a|Ref A", "https://example.com/b"},
	}}
	if got := n.Sources(); len(got) != 0 {
		t.Fatalf("Node.Sources() must stay strict: flat-string list parses to 0 structured entries, got %d: %v", len(got), got)
	}
}

// A non-provenance dotted path with no v0.2 accessor still resolves by literal
// nested-map traversal: `foo.bar` reads frontmatter foo: { bar: ... }.
func TestTemplateDrift_GenericDottedPath(t *testing.T) {
	tmpl := "---\ntype: Type Template\ntarget_type: Concept\nrequired_fields: [meta.owner]\n---\n\n# T\n"
	present := mkConceptBundle(t, tmpl, "meta:\n  owner: casey\n", "Body.")
	if d := TemplateDrift(present); len(d) != 0 {
		t.Fatalf("meta.owner present should not drift, got: %v", d)
	}
	absent := mkConceptBundle(t, tmpl, "meta:\n  other: x\n", "Body.")
	if d := TemplateDrift(absent); len(d) == 0 {
		t.Fatal("meta.owner absent should drift")
	}
}

func TestTemplateDrift_Deterministic(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"templates/playbook.md": playbookTemplate,
		"ops/a.md":              "---\ntype: Playbook\ntitle: A\n---\n\n# A\n",
		"ops/b.md":              "---\ntype: Playbook\ntitle: B\n---\n\n# B\n",
	})
	first := TemplateDrift(b)
	for i := 0; i < 5; i++ {
		got := TemplateDrift(b)
		if len(got) != len(first) {
			t.Fatalf("nondeterministic length: %d vs %d", len(got), len(first))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("nondeterministic order at %d: %v vs %v", j, got[j], first[j])
			}
		}
	}
}
