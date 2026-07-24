package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bundleWithPlaybookTemplate writes only the governing Playbook template into a
// fresh bundle dir (no nodes yet) and returns it.
func bundleWithPlaybookTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "templates", "playbook.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "---\ntype: Type Template\ntarget_type: Playbook\n" +
		"required_fields: [title, description, owner]\n" +
		"recommended_fields: [tags]\n" +
		"body_sections: [Trigger, Steps, Rollback, Verification]\n---\n\n# Playbook Template\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func TestNodeNew_ScaffoldsFromTemplate(t *testing.T) {
	dir := bundleWithPlaybookTemplate(t)
	out, err := runOKF(t, "node", "new", "ops/deploy", "--type", "Playbook", "--title", "Deploy", "--bundle", dir)
	if err != nil {
		t.Fatalf("node new errored: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "ops", "deploy.md"))
	if err != nil {
		t.Fatalf("read created node: %v", err)
	}
	src := string(got)
	// Required + recommended fields stubbed as frontmatter keys.
	for _, key := range []string{"description:", "owner:", "tags:"} {
		if !strings.Contains(src, key) {
			t.Fatalf("scaffold should stub %q, got:\n%s", key, src)
		}
	}
	// Body sections laid down as headings.
	for _, sec := range []string{"## Trigger", "## Steps", "## Rollback", "## Verification"} {
		if !strings.Contains(src, sec) {
			t.Fatalf("scaffold should lay down %q, got:\n%s", sec, src)
		}
	}
	// The scaffolded node must pass validate --templates (it starts conformant).
	vout, verr := runOKF(t, "validate", dir, "--templates", "--strict")
	if verr != nil {
		t.Fatalf("scaffolded node should pass validate --templates --strict, got err %v\n%s", verr, vout)
	}
}

func TestNodeNew_NoTemplate_Unchanged(t *testing.T) {
	dir := t.TempDir()
	out, err := runOKF(t, "node", "new", "wine/tannin", "--type", "Concept", "--title", "Tannin", "--bundle", dir)
	if err != nil {
		t.Fatalf("node new errored: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(dir, "wine", "tannin.md"))
	if err != nil {
		t.Fatalf("read created node: %v", err)
	}
	// With no governing template, output is the plain node: type + title + H1,
	// and no scaffolded section headings.
	src := string(got)
	if !strings.Contains(src, "type: Concept") || !strings.Contains(src, "title: Tannin") {
		t.Fatalf("plain node missing type/title:\n%s", src)
	}
	if strings.Contains(src, "## ") {
		t.Fatalf("no-template path should not scaffold sections:\n%s", src)
	}
}
