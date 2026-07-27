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
	"sort"
	"strings"
	"testing"
)

// writeNodeFM writes a concept node with an explicit description in frontmatter,
// so §6 description-passthrough can be exercised. An empty description omits the
// key entirely.
func writeNodeFM(t *testing.T, dir, rel, typ, title, desc string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\ntype: " + typ + "\ntitle: " + title + "\n"
	if desc != "" {
		fm += "description: " + desc + "\n"
	}
	fm += "---\n\n# " + title + "\n"
	if err := os.WriteFile(p, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIndexDirs_ContentBearingOnly pins which directories get an index: the
// root, any directory directly holding a concept, and any ancestor of such a
// directory. A directory whose entire subtree has no concept gets none.
func TestIndexDirs_ContentBearingOnly(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "A wine tannin.")
	writeNodeFM(t, dir, "wine/red/nebbiolo.md", "Reference", "Nebbiolo", "A red grape.")
	writeNodeFM(t, dir, "top.md", "Reference", "Top", "A root-level concept.")
	// An empty directory with no concepts anywhere in its subtree.
	if err := os.MkdirAll(filepath.Join(dir, "empty", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := IndexDirs(b)
	want := []string{"", "wine", "wine/red"}
	if !equalStringSlice(got, want) {
		t.Fatalf("IndexDirs = %v, want %v", got, want)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

// TestRenderDirIndex_DirRelativeConceptLinks pins §6: a concept living directly
// in the directory is linked by its BASE name (dir-relative), never a
// bundle-relative path, and carries its description.
func TestRenderDirIndex_DirRelativeConceptLinks(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "Astringent phenolics.")
	writeNodeFM(t, dir, "wine/acidity.md", "Reference", "Acidity", "Perceived tartness.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderDirIndex(b, "wine")

	if !strings.Contains(got, "* [Tannin](tannin.md) - Astringent phenolics.") {
		t.Errorf("concept must be a dir-relative link with description; got:\n%s", got)
	}
	if strings.Contains(got, "wine/tannin.md") {
		t.Errorf("§6: link must be dir-relative (tannin.md), not bundle-relative (wine/tannin.md); got:\n%s", got)
	}
	// Deterministic sort within the directory.
	if strings.Index(got, "acidity.md") > strings.Index(got, "tannin.md") {
		t.Errorf("concepts not sorted within directory; got:\n%s", got)
	}
	// §6: nested index carries NO frontmatter.
	if strings.HasPrefix(got, "---\n") {
		t.Errorf("§6: non-root index must have no frontmatter; got:\n%s", got)
	}
	// §6 says entries carry description, NOT a type annotation.
	if strings.Contains(got, "Reference") {
		t.Errorf("entries must carry description, not a type annotation; got:\n%s", got)
	}
}

// TestRenderDirIndex_SubdirectoryEntries pins §6's `* [Subdirectory](subdir/)`
// form: an index enumerates its immediate content-bearing child directories with
// a trailing-slash dir-relative link, and does NOT reach into grandchildren.
func TestRenderDirIndex_SubdirectoryEntries(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "A tannin.")
	writeNodeFM(t, dir, "wine/red/nebbiolo.md", "Reference", "Nebbiolo", "A grape.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderDirIndex(b, "wine")

	if !strings.Contains(got, "](red/)") {
		t.Errorf("§6: child directory must be linked as `red/` (trailing slash, dir-relative); got:\n%s", got)
	}
	// It lists its own concept (tannin.md) but NOT the grandchild concept.
	if !strings.Contains(got, "tannin.md") {
		t.Errorf("index must list its own directory's concepts; got:\n%s", got)
	}
	if strings.Contains(got, "nebbiolo.md") {
		t.Errorf("index must enumerate ONLY its own directory, not a grandchild concept; got:\n%s", got)
	}
}

// TestRenderDirIndex_RootLinksToChildDirsAndOwnConcepts pins the root index:
// it links immediate child directories dir-relatively and lists root-level
// concepts, and never enumerates a nested concept bundle-relatively.
func TestRenderDirIndex_RootLinksToChildDirsAndOwnConcepts(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "top.md", "Reference", "Top", "A root concept.")
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "A tannin.")
	writeNodeFM(t, dir, "lifting/squat.md", "Playbook", "Squat", "A lift.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderDirIndex(b, "")

	if !strings.Contains(got, "](wine/)") || !strings.Contains(got, "](lifting/)") {
		t.Errorf("root index must link child dirs as `wine/` and `lifting/`; got:\n%s", got)
	}
	if !strings.Contains(got, "* [Top](top.md) - A root concept.") {
		t.Errorf("root index must list its own root-level concept dir-relatively; got:\n%s", got)
	}
	if strings.Contains(got, "wine/tannin.md") {
		t.Errorf("root index must NOT enumerate a nested concept bundle-relatively; got:\n%s", got)
	}
	// Child dirs sorted.
	if strings.Index(got, "lifting/") > strings.Index(got, "wine/") {
		t.Errorf("child dirs not sorted; got:\n%s", got)
	}
}

// TestRenderDirIndex_RootRetainsOkfVersion pins §11: the bundle-root index still
// carries the okf_version marker (Scaffold writes a .okf sidecar), and nothing
// else.
func TestRenderDirIndex_RootRetainsOkfVersion(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "A tannin.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	root := RenderDirIndex(b, "")
	if !strings.HasPrefix(root, "---\nokf_version: ") {
		t.Errorf("§11: root index must retain the okf_version marker; got:\n%s", root)
	}
	if strings.Contains(root, "type:") {
		t.Errorf("root frontmatter must carry only okf_version; got:\n%s", root)
	}
	// A nested index never carries the marker.
	nested := RenderDirIndex(b, "wine")
	if strings.HasPrefix(nested, "---\n") {
		t.Errorf("§6/§11: nested index must have NO frontmatter (okf_version is bundle-root-only); got:\n%s", nested)
	}
}

// TestRenderDirIndex_ConceptWithoutDescription pins that a node lacking a
// description renders as a bare link (no trailing ` - `).
func TestRenderDirIndex_ConceptWithoutDescription(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/mystery.md", "Reference", "Mystery", "")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderDirIndex(b, "wine")
	if !strings.Contains(got, "* [Mystery](mystery.md)\n") {
		t.Errorf("a description-less concept must render as a bare link; got:\n%s", got)
	}
	if strings.Contains(got, "mystery.md) -") {
		t.Errorf("a description-less concept must not emit a trailing ` - `; got:\n%s", got)
	}
}

// TestWriteIndex_EmitsNestedIndexes pins that WriteIndex writes an index.md into
// EVERY content-bearing directory, not just the root.
func TestWriteIndex_EmitsNestedIndexes(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "A tannin.")
	writeNodeFM(t, dir, "wine/red/nebbiolo.md", "Reference", "Nebbiolo", "A grape.")
	writeNodeFM(t, dir, "lifting/squat.md", "Playbook", "Squat", "A lift.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	for _, rel := range []string{"index.md", "wine/index.md", "wine/red/index.md", "lifting/index.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected generated %s: %v", rel, err)
		}
	}

	// Reload and validate the whole generated tree: every nested index must be
	// spec-clean (§6 no frontmatter).
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b2); len(f) != 0 {
		t.Fatalf("generated nested tree must validate clean; got findings: %v", f)
	}
}

// TestWriteIndex_PrunesOrphanedIndex pins that WriteIndex removes an index.md
// left behind in a directory that is no longer content-bearing (e.g. after a
// node moves out of it), so the single writer self-heals the tree and a
// subsequent IndexInSync is clean. This is the "stale parent/sibling index"
// class the flat model left behind.
func TestWriteIndex_PrunesOrphanedIndex(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "A tannin.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	// wine/index.md now exists.
	if _, err := os.Stat(filepath.Join(dir, "wine", "index.md")); err != nil {
		t.Fatalf("expected wine/index.md after first build: %v", err)
	}

	// Move the only concept out of wine/ into cellar/ (simulating node mv).
	if err := os.MkdirAll(filepath.Join(dir, "cellar"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "wine", "tannin.md"), filepath.Join(dir, "cellar", "tannin.md")); err != nil {
		t.Fatal(err)
	}

	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b2); err != nil {
		t.Fatalf("WriteIndex (rebuild): %v", err)
	}

	// wine/ is no longer content-bearing: its orphaned index.md must be pruned.
	if _, err := os.Stat(filepath.Join(dir, "wine", "index.md")); !os.IsNotExist(err) {
		t.Errorf("orphaned wine/index.md must be pruned by WriteIndex; stat err=%v", err)
	}
	// cellar/ gained one.
	if _, err := os.Stat(filepath.Join(dir, "cellar", "index.md")); err != nil {
		t.Errorf("expected cellar/index.md after rebuild: %v", err)
	}
	// The rebuilt tree is fully in sync.
	b3, _ := Load(dir)
	if ok, report := IndexInSync(b3); !ok {
		t.Errorf("index should be in sync after a self-healing rebuild; report:\n%s", report)
	}
}

// TestIndexInSync_NestedStalenessAndOrphans pins that check verifies the full
// nested shape: in sync right after build, stale when any nested index drifts,
// stale when a required nested index is missing, and stale when an orphaned
// generated index exists where none should.
func TestIndexInSync_NestedStalenessAndOrphans(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "wine/tannin.md", "Reference", "Tannin", "A tannin.")
	writeNodeFM(t, dir, "wine/red/nebbiolo.md", "Reference", "Nebbiolo", "A grape.")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	b2, _ := Load(dir)
	if ok, report := IndexInSync(b2); !ok {
		t.Fatalf("index should be in sync right after build; report:\n%s", report)
	}

	// Drift a nested index.
	if err := os.WriteFile(filepath.Join(dir, "wine", "index.md"), []byte("# stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b3, _ := Load(dir)
	if ok, _ := IndexInSync(b3); ok {
		t.Error("index should be STALE when a nested index drifts")
	}

	// Rebuild, then delete a required nested index → missing.
	b4, _ := Load(dir)
	_ = WriteIndex(b4)
	if err := os.Remove(filepath.Join(dir, "wine", "red", "index.md")); err != nil {
		t.Fatal(err)
	}
	b5, _ := Load(dir)
	if ok, _ := IndexInSync(b5); ok {
		t.Error("index should be STALE when a required nested index is missing")
	}

	// Rebuild, then add an orphaned index in a directory that should have none.
	b6, _ := Load(dir)
	_ = WriteIndex(b6)
	if err := os.MkdirAll(filepath.Join(dir, "orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "orphan", "index.md"), []byte("# orphan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b7, _ := Load(dir)
	if ok, _ := IndexInSync(b7); ok {
		t.Error("index should be STALE when an orphaned generated index exists")
	}
}
