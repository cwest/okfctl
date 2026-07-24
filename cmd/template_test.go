package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemplateBundle lays down a Playbook template + a couple nodes and returns
// the bundle dir.
func writeTemplateBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"templates/playbook.md": "---\ntype: Type Template\ntarget_type: Playbook\n" +
			"required_fields: [title, description, owner]\n" +
			"recommended_fields: [tags]\n" +
			"body_sections: [Trigger, Steps, Rollback, Verification]\n---\n\n# Playbook Template\n",
		"ops/deploy.md": "---\ntype: Playbook\ntitle: Deploy\n---\n\n# Deploy\n",
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

func TestTemplateList(t *testing.T) {
	dir := writeTemplateBundle(t)
	out, err := runOKF(t, "template", "list", dir)
	if err != nil {
		t.Fatalf("template list errored: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Playbook") {
		t.Fatalf("template list should name the Playbook target_type, got:\n%s", out)
	}
}

func TestTemplateShow(t *testing.T) {
	dir := writeTemplateBundle(t)
	out, err := runOKF(t, "template", "show", "Playbook", dir)
	if err != nil {
		t.Fatalf("template show errored: %v\n%s", err, out)
	}
	for _, want := range []string{"owner", "Rollback", "tags"} {
		if !strings.Contains(out, want) {
			t.Fatalf("template show should include %q, got:\n%s", want, out)
		}
	}
}

func TestTemplateShow_Unknown(t *testing.T) {
	dir := writeTemplateBundle(t)
	_, err := runOKF(t, "template", "show", "Nope", dir)
	if err == nil {
		t.Fatal("template show for an ungoverned type should exit non-zero")
	}
}
