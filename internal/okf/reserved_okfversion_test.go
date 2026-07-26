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

// okfVersionLine returns the `okf_version: ...` line from an index body, or ""
// when absent. It matches only a top-level frontmatter key (a line that begins
// with the key, ignoring the surrounding `---` fences).
func okfVersionLine(index string) string {
	for _, line := range strings.Split(index, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "okf_version:") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// writeBundleRootIndex overwrites the reserved index.md at the bundle root with
// the given frontmatter+body, so a subsequent Load sees it as the on-disk root
// index (as a real bundle-root index that already carries okf_version would).
func writeBundleRootIndex(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRenderIndex_PreservesExistingOkfVersion is the ROOT-CAUSE regression: a
// bundle-root index.md that already declares `okf_version` must retain that key
// (and its exact value) after `okfctl index build` regenerates it. This key is
// the sole marker the KB Python corpus loader uses to find a bundle root; losing
// it silently breaks every /-absolute cross-link.
func TestRenderIndex_PreservesExistingOkfVersion(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	// A bundle-root index that already carries okf_version, as the KB corpus has.
	writeBundleRootIndex(t, dir, "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	line := okfVersionLine(got)
	if line == "" {
		t.Fatalf("regenerated index dropped okf_version; got:\n%s", got)
	}
	if !strings.Contains(line, "0.1") {
		t.Errorf("okf_version value not preserved (want 0.1); got line %q in:\n%s", line, got)
	}
}

// TestRenderIndex_EmitsSidecarOkfVersionWhenIndexLacksIt covers Option C's
// second precedence rule: when the index does not carry okf_version but a .okf
// sidecar declares one, the regenerated index adopts the sidecar's value.
func TestRenderIndex_EmitsSidecarOkfVersionWhenIndexLacksIt(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	// Scaffold writes a .okf pin (okf_version: 0.1) and an index.md WITHOUT the key.
	if _, err := os.Stat(filepath.Join(dir, ".okf")); err != nil {
		t.Fatalf("expected Scaffold to write a .okf sidecar: %v", err)
	}
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	line := okfVersionLine(got)
	if line == "" {
		t.Fatalf("regenerated index omitted okf_version despite a .okf sidecar; got:\n%s", got)
	}
	if !strings.Contains(line, SpecVersion) {
		t.Errorf("okf_version not sourced from sidecar (want %s); got line %q", SpecVersion, line)
	}
}

// TestRenderIndex_NoOkfVersionWhenNeitherPresent covers Option C's third rule:
// with neither an index-declared key nor a .okf sidecar, no okf_version is
// fabricated — behavior is unchanged from before Option C.
func TestRenderIndex_NoOkfVersionWhenNeitherPresent(t *testing.T) {
	dir := t.TempDir()
	// Deliberately do NOT Scaffold (which would create a .okf). Hand-build a
	// minimal bundle root: a bare index.md with no okf_version, no .okf.
	writeBundleRootIndex(t, dir, "---\ntype: Index\n---\n\n# Knowledge Base\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	if _, err := os.Stat(filepath.Join(dir, ".okf")); err == nil {
		t.Fatal("test precondition violated: a .okf sidecar exists")
	}

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	if line := okfVersionLine(got); line != "" {
		t.Errorf("okf_version fabricated when neither index nor .okf declared it; got line %q in:\n%s", line, got)
	}
}

// TestRenderIndex_IndexKeyBeatsSidecar covers precedence: an okf_version already
// on the index wins over a differing .okf sidecar value (preserve what the
// corpus curator committed; do not silently bump it to the build's pin).
func TestRenderIndex_IndexKeyBeatsSidecar(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	// Sidecar pins SpecVersion; index declares a different (older) version.
	writeBundleRootIndex(t, dir, "---\nokf_version: \"0.0-legacy\"\n---\n\n# Knowledge Base\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := RenderIndex(b)

	line := okfVersionLine(got)
	if !strings.Contains(line, "0.0-legacy") {
		t.Errorf("index-declared okf_version must win over the sidecar; got line %q", line)
	}
}

// TestRenderIndex_IdempotentWithOkfVersion locks in that the added key does not
// break byte-stability: render(render(x)) == render(x). A round-trip through
// WriteIndex+Load must reproduce the same bytes, so `index check` cannot report
// a freshly-built index as stale.
func TestRenderIndex_IdempotentWithOkfVersion(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	writeBundleRootIndex(t, dir, "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := RenderIndex(b)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	second := RenderIndex(b2)
	if first != second {
		t.Errorf("RenderIndex not idempotent across a write+reload round-trip:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if ok, diff := IndexInSync(b2); !ok {
		t.Errorf("index reported stale immediately after WriteIndex; diff: %s", diff)
	}
}

// TestWriteIndex_LeavesSubNeighborhoodIndexUntouched asserts the non-root case:
// WriteIndex only ever rewrites the bundle-root index.md. A per-neighborhood
// index.md deeper in the tree is not touched (and so never gains okf_version).
func TestWriteIndex_LeavesSubNeighborhoodIndexUntouched(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	writeBundleRootIndex(t, dir, "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	subIndex := filepath.Join(dir, "wine", "index.md")
	subBody := "---\ntype: Index\n---\n\n# Wine\n"
	if err := os.WriteFile(subIndex, []byte(subBody), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(subIndex)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != subBody {
		t.Errorf("sub-neighborhood index.md was modified by WriteIndex:\nwant:\n%s\ngot:\n%s", subBody, got)
	}
}
