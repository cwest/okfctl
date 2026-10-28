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

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTplFile is a small helper for these integration tests.
func writeTplFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// refTemplateBundle writes a template bundle at <root>/sub with a Playbook
// Type Template requiring the given field and a Steps body section.
func refTemplateBundle(t *testing.T, root, sub, requiredField string) {
	t.Helper()
	writeTplFile(t, root, sub+"/.okf", "okf_version: 0.2\n")
	writeTplFile(t, root, sub+"/index.md", "# Templates\n")
	writeTplFile(t, root, sub+"/log.md", "# Log\n")
	writeTplFile(t, root, sub+"/playbook.md",
		"---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: ["+requiredField+"]\nbody_sections: [Steps]\n---\n# Playbook template\n")
}

// consumerBundle writes a consumer with no local templates; templatesKey ""
// means no sidecar templates key (the silent direction).
func consumerBundle(t *testing.T, dir, templatesKey string) {
	t.Helper()
	body := "okf_version: 0.2\n"
	if templatesKey != "" {
		body += "templates: " + templatesKey + "\n"
	}
	writeTplFile(t, dir, ".okf", body)
	writeTplFile(t, dir, "index.md", "# KB\n")
	writeTplFile(t, dir, "log.md", "# Log\n")
}

// --- template list / show --------------------------------------------------

// TestTemplateList_SidecarReference is AC1 (fires): `template list` on a consumer
// whose .okf references ../tpl shows the Playbook with its referenced source.
func TestTemplateList_SidecarReference(t *testing.T) {
	dir := t.TempDir()
	refTemplateBundle(t, dir, "tpl", "title")
	consumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	out, err := runOKF(t, "template", "list", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("template list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Playbook") {
		t.Errorf("template list output missing Playbook:\n%s", out)
	}
	if !strings.Contains(out, "../tpl/playbook.md") {
		t.Errorf("template list output missing referenced source ../tpl/playbook.md:\n%s", out)
	}
}

// TestTemplateList_SilentNoKey is AC1 (silent): no templates key, no flag ->
// "no type templates in bundle".
func TestTemplateList_SilentNoKey(t *testing.T) {
	dir := t.TempDir()
	consumerBundle(t, filepath.Join(dir, "kb"), "")
	out, err := runOKF(t, "template", "list", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("template list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no type templates in bundle") {
		t.Errorf("silent consumer template list = %q, want the no-templates notice", out)
	}
}

// TestTemplateShow_SidecarReference is AC1: `template show Playbook` works against
// a referenced bundle.
func TestTemplateShow_SidecarReference(t *testing.T) {
	dir := t.TempDir()
	refTemplateBundle(t, dir, "tpl", "title")
	consumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	out, err := runOKF(t, "template", "show", "Playbook", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("template show: %v\n%s", err, out)
	}
	if !strings.Contains(out, "target_type: Playbook") {
		t.Errorf("template show output missing target_type: Playbook:\n%s", out)
	}
	if !strings.Contains(out, "../tpl/playbook.md") {
		t.Errorf("template show output missing referenced source:\n%s", out)
	}
}

// TestTemplateList_FlagOverridesSidecar is AC2: --templates-from overrides the
// sidecar; the winning template's distinct required field count is observable.
func TestTemplateList_FlagOverridesSidecar(t *testing.T) {
	dir := t.TempDir()
	refTemplateBundle(t, dir, "tpl", "sidecar_a")           // 1 required field
	writeTplFile(t, dir, "tpl2/.okf", "okf_version: 0.2\n") // flag target: 2 required fields
	writeTplFile(t, dir, "tpl2/index.md", "---\ntype: Index\n---\n# T\n")
	writeTplFile(t, dir, "tpl2/log.md", "# Log\n")
	writeTplFile(t, dir, "tpl2/playbook.md",
		"---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: [flag_a, flag_b]\n---\n# P\n")
	consumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	out, err := runOKF(t, "template", "list", "--templates-from", filepath.Join(dir, "tpl2"), filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("template list --templates-from: %v\n%s", err, out)
	}
	if !strings.Contains(out, "2 required field(s)") {
		t.Errorf("flag did not override sidecar (want 2 required fields):\n%s", out)
	}
}

// TestTemplateList_MissingReferenceErrors is AC5 (fires): a templates key naming
// a non-directory errors non-zero naming the path.
func TestTemplateList_MissingReferenceErrors(t *testing.T) {
	dir := t.TempDir()
	writeTplFile(t, dir, "notadir", "i am a file\n")
	consumerBundle(t, filepath.Join(dir, "kb"), "../notadir")
	out, err := runOKF(t, "template", "list", filepath.Join(dir, "kb"))
	if err == nil {
		t.Fatalf("template list with unloadable reference must exit non-zero; out=%q", out)
	}
	if !strings.Contains(err.Error()+out, "../notadir") {
		t.Errorf("error must name ../notadir; err=%v out=%q", err, out)
	}
}

// --- validate --templates --------------------------------------------------

// TestValidate_TemplatesReferencedDrift is AC1 (fires): a Playbook node missing a
// required field drifts against the referenced template under --templates.
func TestValidate_TemplatesReferencedDrift(t *testing.T) {
	dir := t.TempDir()
	refTemplateBundle(t, dir, "tpl", "owner") // requires owner
	consumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	// A Playbook node with no owner field -> drift.
	writeTplFile(t, filepath.Join(dir, "kb"), "runbooks/deploy.md",
		"---\ntype: Playbook\ntitle: Deploy\n---\n# Deploy\n")
	out, err := runOKF(t, "validate", "--templates", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("validate --templates (advisory) should exit 0: %v\n%s", err, out)
	}
	if !strings.Contains(out, "missing required field: owner") {
		t.Errorf("validate --templates missing referenced drift for owner:\n%s", out)
	}
}

// TestValidate_TemplatesSilentNoDrift is AC1 (silent): same consumer with no
// templates key -> no template drift reported.
func TestValidate_TemplatesSilentNoDrift(t *testing.T) {
	dir := t.TempDir()
	consumerBundle(t, filepath.Join(dir, "kb"), "")
	writeTplFile(t, filepath.Join(dir, "kb"), "runbooks/deploy.md",
		"---\ntype: Playbook\ntitle: Deploy\n---\n# Deploy\n")
	out, err := runOKF(t, "validate", "--templates", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("validate --templates silent should exit 0: %v\n%s", err, out)
	}
	if strings.Contains(out, "missing required field") {
		t.Errorf("silent consumer must report no template drift:\n%s", out)
	}
}

// TestValidate_TemplatesFromFlagOverrides is AC2: validate --templates-from
// overrides the sidecar (distinct required fields make the winner observable).
func TestValidate_TemplatesFromFlagOverrides(t *testing.T) {
	dir := t.TempDir()
	refTemplateBundle(t, dir, "tpl", "sidecar_field")
	refTemplateBundle(t, dir, "tpl2", "flag_field")
	consumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	writeTplFile(t, filepath.Join(dir, "kb"), "runbooks/deploy.md",
		"---\ntype: Playbook\ntitle: Deploy\n---\n# Deploy\n")
	out, err := runOKF(t, "validate", "--templates", "--templates-from", filepath.Join(dir, "tpl2"), filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "missing required field: flag_field") {
		t.Errorf("flag target's field not enforced:\n%s", out)
	}
	if strings.Contains(out, "sidecar_field") {
		t.Errorf("sidecar field should have been overridden by the flag:\n%s", out)
	}
}

// TestValidate_MissingReferenceErrors is AC5 (fires): validate --templates with a
// non-loadable reference exits non-zero naming the path.
func TestValidate_MissingReferenceErrors(t *testing.T) {
	dir := t.TempDir()
	writeTplFile(t, dir, "notadir", "file\n")
	consumerBundle(t, filepath.Join(dir, "kb"), "../notadir")
	out, err := runOKF(t, "validate", "--templates", filepath.Join(dir, "kb"))
	if err == nil {
		t.Fatalf("validate --templates with unloadable reference must exit non-zero; out=%q", out)
	}
	if !strings.Contains(err.Error()+out, "../notadir") {
		t.Errorf("error must name ../notadir; err=%v out=%q", err, out)
	}
}

// TestValidate_MissingReferenceSilentPlainValidate is AC5 (silent): plain
// validate (no --templates) on that same bundle exits 0 — the broken reference
// is only consulted by the template path.
func TestValidate_MissingReferenceSilentPlainValidate(t *testing.T) {
	dir := t.TempDir()
	writeTplFile(t, dir, "notadir", "file\n")
	consumerBundle(t, filepath.Join(dir, "kb"), "../notadir")
	if _, err := runOKF(t, "validate", filepath.Join(dir, "kb")); err != nil {
		t.Fatalf("plain validate on a bundle with a broken templates ref must exit 0: %v", err)
	}
}

// --- node new --------------------------------------------------------------

// TestNodeNew_ScaffoldsFromReferencedTemplate is AC1 (fires): node new --type
// Playbook scaffolds required fields + body sections from the referenced template.
func TestNodeNew_ScaffoldsFromReferencedTemplate(t *testing.T) {
	dir := t.TempDir()
	refTemplateBundle(t, dir, "tpl", "owner") // required: owner; section: Steps
	consumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	out, err := runOKF(t, "node", "new", "runbooks/deploy", "--type", "Playbook", "--bundle", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("node new: %v\n%s", err, out)
	}
	body, rerr := os.ReadFile(filepath.Join(dir, "kb", "runbooks", "deploy.md"))
	if rerr != nil {
		t.Fatalf("read created node: %v", rerr)
	}
	s := string(body)
	if !strings.Contains(s, "owner:") {
		t.Errorf("scaffold missing required field owner:\n%s", s)
	}
	if !strings.Contains(s, "## Steps") {
		t.Errorf("scaffold missing body section Steps:\n%s", s)
	}
}

// TestNodeNew_SilentPlainNode is AC1 (silent): no templates key -> node new
// creates a plain node (no scaffolded owner/Steps).
func TestNodeNew_SilentPlainNode(t *testing.T) {
	dir := t.TempDir()
	consumerBundle(t, filepath.Join(dir, "kb"), "")
	out, err := runOKF(t, "node", "new", "runbooks/deploy", "--type", "Playbook", "--bundle", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("node new: %v\n%s", err, out)
	}
	body, rerr := os.ReadFile(filepath.Join(dir, "kb", "runbooks", "deploy.md"))
	if rerr != nil {
		t.Fatalf("read created node: %v", rerr)
	}
	if strings.Contains(string(body), "## Steps") {
		t.Errorf("plain node must not carry scaffolded sections:\n%s", string(body))
	}
}

// TestNodeNew_TemplatesFromFlag is AC2: node new --templates-from scaffolds from
// the flag target even with no sidecar key.
func TestNodeNew_TemplatesFromFlag(t *testing.T) {
	dir := t.TempDir()
	refTemplateBundle(t, dir, "tpl", "owner")
	consumerBundle(t, filepath.Join(dir, "kb"), "")
	out, err := runOKF(t, "node", "new", "runbooks/deploy", "--type", "Playbook",
		"--templates-from", filepath.Join(dir, "tpl"), "--bundle", filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("node new --templates-from: %v\n%s", err, out)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "kb", "runbooks", "deploy.md"))
	if !strings.Contains(string(body), "## Steps") {
		t.Errorf("flag-driven scaffold missing Steps:\n%s", string(body))
	}
}

// TestNodeNew_MissingReferenceErrors is AC5 (fires): node new --type X with an
// unloadable reference exits non-zero naming the path.
func TestNodeNew_MissingReferenceErrors(t *testing.T) {
	dir := t.TempDir()
	writeTplFile(t, dir, "notadir", "file\n")
	consumerBundle(t, filepath.Join(dir, "kb"), "../notadir")
	out, err := runOKF(t, "node", "new", "runbooks/deploy", "--type", "Playbook", "--bundle", filepath.Join(dir, "kb"))
	if err == nil {
		t.Fatalf("node new with unloadable reference must exit non-zero; out=%q", out)
	}
	if !strings.Contains(err.Error()+out, "../notadir") {
		t.Errorf("error must name ../notadir; err=%v out=%q", err, out)
	}
}
