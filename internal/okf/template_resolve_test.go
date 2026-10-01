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
	"strings"
	"testing"
)

// writeTemplateBundle writes a minimal valid template bundle under dir/sub with a
// single Playbook Type Template that requires the given field.
func writeTemplateBundle(t *testing.T, dir, sub, requiredField string) {
	t.Helper()
	writeFile(t, dir, sub+"/.okf", "okf_version: 0.2\n")
	writeFile(t, dir, sub+"/index.md", "---\ntype: Index\n---\n# Templates\n")
	writeFile(t, dir, sub+"/log.md", "# Log\n")
	writeFile(t, dir, sub+"/playbook.md",
		"---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: ["+requiredField+"]\nbody_sections: [Steps]\n---\n# Playbook template\n")
}

// writeConsumerBundle writes a consumer bundle with no local templates. The .okf
// carries the given templates key (pass "" for no key — the silent direction).
func writeConsumerBundle(t *testing.T, dir, templatesKey string) {
	t.Helper()
	okfBody := "okf_version: 0.2\n"
	if templatesKey != "" {
		okfBody += "templates: " + templatesKey + "\n"
	}
	writeFile(t, dir, ".okf", okfBody)
	writeFile(t, dir, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, dir, "log.md", "# Log\n")
}

// TestResolveTemplates_SilentWhenNoKeyNoFlag is AC1 (silent): a consumer with no
// templates key and no flag resolves to an empty template set — byte-identical
// behavior to today's one-bundle read.
func TestResolveTemplates_SilentWhenNoKeyNoFlag(t *testing.T) {
	dir := t.TempDir()
	writeConsumerBundle(t, filepath.Join(dir, "kb"), "")
	b, err := Load(filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := ResolveTemplates(b, "")
	if err != nil {
		t.Fatalf("ResolveTemplates: %v", err)
	}
	if len(rt.ByType) != 0 {
		t.Errorf("silent consumer resolved %d templates, want 0: %#v", len(rt.ByType), rt.ByType)
	}
	if len(rt.Shadowed) != 0 {
		t.Errorf("silent consumer has %d shadowed, want 0", len(rt.Shadowed))
	}
}

// TestResolveTemplates_SidecarReferenceFires is AC1 (fires): a consumer with
// zero local templates and a sidecar `templates: ../tpl` resolves the Playbook
// from the referenced bundle, with its source naming the referenced path.
func TestResolveTemplates_SidecarReferenceFires(t *testing.T) {
	dir := t.TempDir()
	writeTemplateBundle(t, dir, "tpl", "title")
	writeConsumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	b, err := Load(filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := ResolveTemplates(b, "")
	if err != nil {
		t.Fatalf("ResolveTemplates: %v", err)
	}
	pb, ok := rt.ByType["Playbook"]
	if !ok {
		t.Fatalf("Playbook not resolved from referenced bundle; have %v", keysOf(rt.ByType))
	}
	if pb.Source != "../tpl/playbook.md" {
		t.Errorf("Playbook.Source = %q, want ../tpl/playbook.md", pb.Source)
	}
	if len(pb.RequiredFields) != 1 || pb.RequiredFields[0] != "title" {
		t.Errorf("Playbook.RequiredFields = %v, want [title]", pb.RequiredFields)
	}
}

// TestResolveTemplates_FlagOverridesSidecar is AC2: --templates-from overrides
// the sidecar reference. The two bundles declare distinct required_fields so the
// winner is observable.
func TestResolveTemplates_FlagOverridesSidecar(t *testing.T) {
	dir := t.TempDir()
	writeTemplateBundle(t, dir, "tpl", "sidecar_field")
	writeTemplateBundle(t, dir, "tpl2", "flag_field")
	writeConsumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	b, err := Load(filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := ResolveTemplates(b, filepath.Join(dir, "kb", "..", "tpl2"))
	if err != nil {
		t.Fatalf("ResolveTemplates: %v", err)
	}
	pb := rt.ByType["Playbook"]
	if len(pb.RequiredFields) != 1 || pb.RequiredFields[0] != "flag_field" {
		t.Errorf("flag did not override sidecar: RequiredFields = %v, want [flag_field]", pb.RequiredFields)
	}
}

// TestResolveTemplates_FlagWorksWithNoSidecar is AC2's second clause: the flag
// resolves templates even when the consumer declares no sidecar key.
func TestResolveTemplates_FlagWorksWithNoSidecar(t *testing.T) {
	dir := t.TempDir()
	writeTemplateBundle(t, dir, "tpl", "title")
	writeConsumerBundle(t, filepath.Join(dir, "kb"), "")
	b, err := Load(filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := ResolveTemplates(b, filepath.Join(dir, "tpl"))
	if err != nil {
		t.Fatalf("ResolveTemplates: %v", err)
	}
	if _, ok := rt.ByType["Playbook"]; !ok {
		t.Errorf("flag-only resolution failed; have %v", keysOf(rt.ByType))
	}
}

// TestResolveTemplates_LocalWinsAndShadows is AC4: a local Playbook template plus
// a referenced Playbook template -> the local one wins, and the referenced one is
// recorded as shadowed. A referenced-only type still resolves from the reference.
func TestResolveTemplates_LocalWinsAndShadows(t *testing.T) {
	dir := t.TempDir()
	// Referenced bundle has Playbook (required: ref_field) AND Reference
	// (ref-only type, should still resolve).
	writeFile(t, dir, "tpl/.okf", "okf_version: 0.2\n")
	writeFile(t, dir, "tpl/index.md", "---\ntype: Index\n---\n# T\n")
	writeFile(t, dir, "tpl/log.md", "# Log\n")
	writeFile(t, dir, "tpl/playbook.md",
		"---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: [ref_field]\n---\n# P\n")
	writeFile(t, dir, "tpl/reference.md",
		"---\ntype: Type Template\ntarget_type: Reference\nrequired_fields: [refonly_field]\n---\n# R\n")
	// Consumer has a LOCAL Playbook template (required: local_field).
	writeConsumerBundle(t, filepath.Join(dir, "kb"), "../tpl")
	writeFile(t, filepath.Join(dir, "kb"), "templates/playbook.md",
		"---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: [local_field]\n---\n# Local P\n")
	b, err := Load(filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := ResolveTemplates(b, "")
	if err != nil {
		t.Fatalf("ResolveTemplates: %v", err)
	}
	pb := rt.ByType["Playbook"]
	if len(pb.RequiredFields) != 1 || pb.RequiredFields[0] != "local_field" {
		t.Errorf("local Playbook did not win: RequiredFields = %v, want [local_field]", pb.RequiredFields)
	}
	if pb.Source != "templates/playbook.md" {
		t.Errorf("winning Playbook.Source = %q, want local templates/playbook.md", pb.Source)
	}
	// The referenced-only Reference type still resolves from the reference.
	ref, ok := rt.ByType["Reference"]
	if !ok {
		t.Fatalf("referenced-only Reference type did not resolve; have %v", keysOf(rt.ByType))
	}
	if ref.Source != "../tpl/reference.md" {
		t.Errorf("Reference.Source = %q, want ../tpl/reference.md", ref.Source)
	}
	// The shadowed referenced Playbook is recorded.
	if len(rt.Shadowed) != 1 {
		t.Fatalf("Shadowed = %d, want 1 (the referenced Playbook)", len(rt.Shadowed))
	}
	if rt.Shadowed[0].TargetType != "Playbook" || rt.Shadowed[0].Source != "../tpl/playbook.md" {
		t.Errorf("Shadowed[0] = {%s %s}, want {Playbook ../tpl/playbook.md}",
			rt.Shadowed[0].TargetType, rt.Shadowed[0].Source)
	}
}

// TestResolveTemplates_LocalOnlyHasLocalSource confirms a local-only template
// (no reference at all) records its own local path as source.
func TestResolveTemplates_LocalOnlyHasLocalSource(t *testing.T) {
	dir := t.TempDir()
	writeConsumerBundle(t, dir, "")
	writeFile(t, dir, "templates/playbook.md",
		"---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: [x]\n---\n# P\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rt, err := ResolveTemplates(b, "")
	if err != nil {
		t.Fatalf("ResolveTemplates: %v", err)
	}
	pb := rt.ByType["Playbook"]
	if pb.Source != "templates/playbook.md" {
		t.Errorf("local-only Playbook.Source = %q, want templates/playbook.md", pb.Source)
	}
}

// TestResolveTemplates_MissingReferenceErrors is AC5 (fires): a templates path
// that cannot be loaded as a bundle (a file, not a directory) errors, and the
// error names the offending path.
func TestResolveTemplates_MissingReferenceErrors(t *testing.T) {
	dir := t.TempDir()
	// A path that is a FILE, not a directory — "unloadable" per the card.
	writeFile(t, dir, "notadir", "i am a file\n")
	writeConsumerBundle(t, filepath.Join(dir, "kb"), "../notadir")
	b, err := Load(filepath.Join(dir, "kb"))
	if err != nil {
		t.Fatalf("Load consumer: %v", err)
	}
	_, err = ResolveTemplates(b, "")
	if err == nil {
		t.Fatalf("ResolveTemplates with unloadable reference must error")
	}
	if !strings.Contains(err.Error(), "../notadir") {
		t.Errorf("error %q must name the offending reference ../notadir", err.Error())
	}
}

// TestResolveTemplates_InTreeReferenceStillWalked documents the loader-scope
// invariant from the card: an IN-TREE `templates:` reference does not add a skip
// to the consumer's own walk — the referenced nodes are still part of the
// consumer's node set (and would also validate/index as the consumer's own),
// while ALSO folding through the reference resolver. The key changes template
// resolution, never the walk. Here templates: ./sub points at an in-tree dir;
// that dir's Type Template is a consumer node AND resolves, so no skip was added.
func TestResolveTemplates_InTreeReferenceStillWalked(t *testing.T) {
	dir := t.TempDir()
	// An in-tree subdirectory holding a Type Template, referenced by ./sub.
	writeFile(t, dir, "sub/.okf", "okf_version: 0.2\n")
	writeFile(t, dir, "sub/index.md", "# Sub\n")
	writeFile(t, dir, "sub/log.md", "# Log\n")
	writeFile(t, dir, "sub/playbook.md",
		"---\ntype: Type Template\ntarget_type: Playbook\nrequired_fields: [x]\n---\n# P\n")
	writeConsumerBundle(t, dir, "./sub")
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// The in-tree template node was WALKED into the consumer's own node set —
	// the key added no skip (loader-scope invariant).
	if _, walked := b.Nodes["sub/playbook.md"]; !walked {
		t.Errorf("in-tree referenced node sub/playbook.md was not walked into the consumer; have %v", keys(b.Nodes))
	}
	// And it still resolves through the reference overlay.
	rt, err := ResolveTemplates(b, "")
	if err != nil {
		t.Fatalf("ResolveTemplates: %v", err)
	}
	if _, ok := rt.ByType["Playbook"]; !ok {
		t.Errorf("in-tree referenced Playbook did not resolve; have %v", keysOf(rt.ByType))
	}
}

// keysOf returns the sorted keys of a template map for test diagnostics.
func keysOf(m map[string]Template) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
