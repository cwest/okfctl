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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderDirIndex_SubdirEntryCarriesShape pins §8 progressive disclosure: a
// subdirectory bullet in a generated index carries the tool-owned shape suffix
// on the SAME line as the link, defaulting on.
func TestRenderDirIndex_SubdirEntryCarriesShape(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeNodeTags(t, dir, "design/b.md", "Map", "B", []string{"ux"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderDirIndex(b, "")

	// The design/ subdirectory bullet must carry count + types + tags inline.
	var line string
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "](design/)") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("no design/ subdirectory bullet found in:\n%s", got)
	}
	if !strings.Contains(line, "(2 concepts · Concept, Map · shared tags: ux)") {
		t.Errorf("subdir bullet missing shape suffix; got line: %q", line)
	}
	if !strings.HasPrefix(line, "* [Design](design/) (") {
		t.Errorf("shape suffix must trail the §8 link form on the same bullet; got: %q", line)
	}
}

// TestRenderDirIndex_ConceptEntriesUnchanged pins that CONCEPT entries are NOT
// touched by the shape feature — only subdirectory entries carry a suffix.
func TestRenderDirIndex_ConceptEntriesUnchanged(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "Astringent phenolics.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderDirIndex(b, "wine")
	// Concept bullet is exactly the §8 grammar, no parenthesised shape.
	if !strings.Contains(got, "* [Tannin](tannin.md) - Astringent phenolics.\n") {
		t.Errorf("concept entry must be byte-identical to the §8 form; got:\n%s", got)
	}
}

// TestRenderDirIndexWithOptions_NoShape pins that a disabled shape reproduces the
// pre-feature bullet exactly (byte-identical — the --no-shape negative control).
func TestRenderDirIndexWithOptions_NoShape(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeNodeTags(t, dir, "design/b.md", "Map", "B", []string{"ux"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	off := RenderDirIndexWithOptions(b, "", IndexShapeOptions{Enabled: false})
	if !strings.Contains(off, "* [Design](design/)\n") {
		t.Errorf("--no-shape must render the bare §8 bullet; got:\n%s", off)
	}
	if strings.Contains(off, "concepts") || strings.Contains(off, "shared tags") {
		t.Errorf("--no-shape must carry no shape text; got:\n%s", off)
	}
}

// TestRenderDirIndex_DefaultEqualsExplicitDefaultOptions pins that the zero-arg
// RenderDirIndex is exactly RenderDirIndexWithOptions(..., DefaultShapeOptions):
// build, check, and maintenance share one default so `index check` never drifts.
func TestRenderDirIndex_DefaultEqualsExplicitDefaultOptions(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeNodeTags(t, dir, "design/ux-psychology/c.md", "Concept", "C", nil)

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"", "design"} {
		if RenderDirIndex(b, d) != RenderDirIndexWithOptions(b, d, DefaultShapeOptions()) {
			t.Errorf("RenderDirIndex(%q) must equal the explicit-default render", d)
		}
	}
}

// TestWriteIndexWithOptions_CheckSymmetry pins the round-trip: an index BUILT
// with a given shape config is in sync ONLY when CHECKED with the same config,
// and stale when checked with a different one — proving the flags apply
// symmetrically to build and check.
func TestWriteIndexWithOptions_CheckSymmetry(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeNodeTags(t, dir, "design/b.md", "Map", "B", []string{"ux"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	noShape := IndexShapeOptions{Enabled: false}
	if err := WriteIndexWithOptions(b, noShape); err != nil {
		t.Fatalf("WriteIndexWithOptions: %v", err)
	}
	b2, _ := Load(dir)
	// In sync when checked with the SAME (no-shape) options.
	if ok, report := IndexInSyncWithOptions(b2, noShape); !ok {
		t.Errorf("index built no-shape must be in sync when checked no-shape; report:\n%s", report)
	}
	// Stale when checked with the DEFAULT (shape-on) options.
	if ok, _ := IndexInSyncWithOptions(b2, DefaultShapeOptions()); ok {
		t.Error("index built no-shape must be STALE when checked with shape-on")
	}
}

// TestRenderDirIndex_ShapeOnV01Bundle pins that the shape suffix renders on a
// v0.1 bundle and does not disturb the load-bearing v0.1 fallbacks. The bundle
// declares okf_version 0.1, and its concepts carry ONLY the legacy provenance
// (`timestamp`) and the legacy body `# Citations` (no v0.2 `generated.at` /
// `sources`). Building the index must (1) render the shape suffix on the
// subdirectory bullet, (2) preserve the 0.1 marker on the root index (§12), and
// (3) still Validate clean — the shape feature must be version-agnostic.
func TestRenderDirIndex_ShapeOnV01Bundle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".okf"), []byte("okf_version: 0.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"),
		[]byte("---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Two legacy-shaped concepts in design/, sharing a tag, so the shape suffix
	// has a count + types + shared tag to render.
	writeLegacyV01Node(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeLegacyV01Node(t, dir, "design/b.md", "Map", "B", []string{"ux"})

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	root, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	// (1) shape suffix present on the design/ bullet.
	if !strings.Contains(string(root), "* [Design](design/) (2 concepts · Concept, Map · shared tags: ux)") {
		t.Errorf("shape suffix must render on a v0.1 bundle; got:\n%s", root)
	}
	// (2) §12 marker preserved at 0.1.
	if !strings.Contains(string(root), `okf_version: "0.1"`) {
		t.Errorf("§12: v0.1 marker must be preserved through index build; got:\n%s", root)
	}
	// (3) the whole generated tree still validates clean.
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b2); len(f) != 0 {
		t.Errorf("v0.1 bundle with shape suffix must validate clean; got findings: %v", f)
	}
}

// writeLegacyV01Node writes a concept carrying ONLY v0.1 provenance/citation
// forms (legacy `timestamp` frontmatter + body `# Citations`), never the v0.2
// `generated.at` / `sources`. This exercises the v0.1 fallbacks alongside the
// shape feature.
func writeLegacyV01Node(t *testing.T, dir, rel, typ, title string, tags []string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\ntype: " + typ + "\ntitle: " + title + "\ntimestamp: 2026-01-02T03:04:05Z\n"
	if len(tags) > 0 {
		fm += "tags:\n"
		for _, tg := range tags {
			fm += "  - " + tg + "\n"
		}
	}
	fm += "---\n\n# " + title + "\n\nProse.\n\n# Citations\n\n[1] A source, https://example.com/x\n"
	if err := os.WriteFile(p, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
}
