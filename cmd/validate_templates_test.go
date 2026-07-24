package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeOverlayBundle writes a Playbook template + one node with the given
// frontmatter kv lines and body, returning the bundle dir.
func writeOverlayBundle(t *testing.T, nodeFrontmatter, nodeBody string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"templates/playbook.md": "---\ntype: Type Template\ntarget_type: Playbook\n" +
			"required_fields: [title, description, owner]\n" +
			"body_sections: [Trigger, Steps, Rollback, Verification]\n---\n\n# Playbook Template\n",
		"ops/deploy.md": "---\ntype: Playbook\n" + nodeFrontmatter + "---\n\n# Deploy\n\n" + nodeBody + "\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

// A drifting node: valid type (floor passes) but missing required fields + sections.
const driftingFrontmatter = "title: Deploy\n"
const driftingBody = "## Trigger\n"

func TestValidateTemplates_ReportsDrift(t *testing.T) {
	dir := writeOverlayBundle(t, driftingFrontmatter, driftingBody)
	out, err := runOKF(t, "validate", dir, "--templates")
	if err != nil {
		t.Fatalf("validate --templates should be advisory (exit 0), got err %v\n%s", err, out)
	}
	if !strings.Contains(out, "warning") || !strings.Contains(out, "drift") {
		t.Fatalf("expected drift warnings, got:\n%s", out)
	}
}

func TestValidateTemplates_Strict(t *testing.T) {
	dir := writeOverlayBundle(t, driftingFrontmatter, driftingBody)
	if _, err := runOKF(t, "validate", dir, "--templates", "--strict"); err == nil {
		t.Fatal("validate --templates --strict on a drifting node should exit non-zero")
	}
}

func TestValidate_FloorUnchangedWithoutFlag(t *testing.T) {
	// A drifting-but-typed node passes plain validate (floor purity, §7.4).
	dir := writeOverlayBundle(t, driftingFrontmatter, driftingBody)
	out, err := runOKF(t, "validate", dir)
	if err != nil {
		t.Fatalf("plain validate on a drifting-but-typed node should exit 0, got err %v\n%s", err, out)
	}
	if strings.Contains(out, "drift") || strings.Contains(out, "warning") {
		t.Fatalf("plain validate must not run the overlay, got:\n%s", out)
	}
}

func TestValidateTemplates_FloorStillFails(t *testing.T) {
	// A node with an empty type fails the floor with OR without --templates.
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(p, []byte("---\ntype: \"\"\n---\n\n# Bad\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := runOKF(t, "validate", dir); err == nil {
		t.Fatal("empty-type node must fail plain validate")
	}
	if _, err := runOKF(t, "validate", dir, "--templates"); err == nil {
		t.Fatal("empty-type node must fail validate --templates (floor is non-negotiable)")
	}
}
