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
