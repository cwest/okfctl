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

// subdirBullet returns the first Subdirectories bullet whose link target matches
// linkTarget (e.g. "topics/") from an index body, or "" when none is present.
func subdirBullet(body, linkTarget string) string {
	for _, l := range strings.Split(body, "\n") {
		if strings.Contains(l, "]("+linkTarget+")") && strings.HasPrefix(l, "* [") {
			return l
		}
	}
	return ""
}

// TestSubdirDescription_SurvivesThreeBuilds pins the primary §8 defect: a
// hand-written subdirectory description on a parent index entry
// (`* [Title](subdir/) - description`) survives repeated `index build` runs
// byte-identically, and `index check` reports the index current (rc 0) on it.
// OKF §8: `* [Subdirectory](subdir/) - short description of the subdirectory`.
func TestSubdirDescription_SurvivesThreeBuilds(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "topics/nouns.md", "Concept", "Nouns", "")

	// Build once to establish the tool-owned index (with shape suffix).
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	// A curator hand-edits the root index's Subdirectories bullet to add a
	// description, keeping the tool-owned shape suffix in place.
	rootPath := filepath.Join(dir, "index.md")
	raw, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	const desc = "Parts of speech, punctuation and sentence structure."
	edited := strings.Replace(string(raw),
		"* [Topics](topics/) (",
		"* [Topics](topics/) - "+desc+" (", 1)
	if edited == string(raw) {
		t.Fatalf("test setup: no Topics subdir bullet to edit in:\n%s", raw)
	}
	if err := os.WriteFile(rootPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Three consecutive builds must leave the edited index byte-identical.
	want := edited
	for i := 1; i <= 3; i++ {
		b, err := Load(dir)
		if err != nil {
			t.Fatalf("Load (build %d): %v", i, err)
		}
		if err := WriteIndex(b); err != nil {
			t.Fatalf("WriteIndex (build %d): %v", i, err)
		}
		got, err := os.ReadFile(rootPath)
		if err != nil {
			t.Fatalf("read root index (build %d): %v", i, err)
		}
		if string(got) != want {
			t.Fatalf("build %d changed the index (description not preserved byte-identically)\nwant:\n%s\ngot:\n%s", i, want, got)
		}
	}

	// index check must be clean on the description-bearing index.
	bCheck, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if ok, report := IndexInSync(bCheck); !ok {
		t.Errorf("§8: index with a hand-written subdir description must be in sync; report:\n%s", report)
	}
}

// TestSubdirDescription_ChildFrontmatterStableAcrossBuilds pins the second path
// of the same bug class: a description sourced from the CHILD index frontmatter
// is honored on build #1 AND survives build #2 (previously dropped, because the
// build rewrites the child index without frontmatter per §8). The preserved
// on-disk parent description takes over as the stable home.
func TestSubdirDescription_ChildFrontmatterStableAcrossBuilds(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "topics/nouns.md", "Concept", "Nouns", "")

	// A curator-authored child index carrying a description in its frontmatter
	// (the door §8 leaves open for a first build).
	childIdx := filepath.Join(dir, "topics", "index.md")
	if err := os.WriteFile(childIdx,
		[]byte("---\ndescription: From child frontmatter.\n---\n\n# Topics\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build #1 honors the child-frontmatter description on the parent bullet.
	b1, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b1); err != nil {
		t.Fatalf("WriteIndex #1: %v", err)
	}
	root1, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	bl1 := subdirBullet(string(root1), "topics/")
	if !strings.Contains(bl1, "- From child frontmatter.") {
		t.Fatalf("build #1 must honor child-frontmatter description; got bullet: %q", bl1)
	}

	// Build #2 must NOT drop it (the non-idempotent-build defect).
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b2); err != nil {
		t.Fatalf("WriteIndex #2: %v", err)
	}
	root2, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	bl2 := subdirBullet(string(root2), "topics/")
	if !strings.Contains(bl2, "- From child frontmatter.") {
		t.Fatalf("build #2 dropped the child-frontmatter description (non-idempotent build); got bullet: %q", bl2)
	}
}

// TestSubdirDescription_EndingInParensSurvives pins that a description that
// itself ends in parentheses (e.g. "Grammar (draft)") is preserved and is not
// confused with the tool-owned shape suffix — separation is derived from what
// dirShape would render, not a greedy trailing-paren match. Verified both with
// the shape suffix on (default) and off (--no-shape).
func TestSubdirDescription_EndingInParensSurvives(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts IndexShapeOptions
	}{
		{"shape-on", DefaultShapeOptions()},
		{"no-shape", IndexShapeOptions{Enabled: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			_ = Scaffold(dir)
			writeNodeFM(t, dir, "topics/nouns.md", "Concept", "Nouns", "")

			b, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := WriteIndexWithOptions(b, tc.opts); err != nil {
				t.Fatalf("WriteIndexWithOptions: %v", err)
			}

			rootPath := filepath.Join(dir, "index.md")
			raw, err := os.ReadFile(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			// Insert a description that ends in parentheses, before any shape suffix.
			const desc = "Grammar (draft)"
			var edited string
			if tc.opts.Enabled {
				edited = strings.Replace(string(raw),
					"* [Topics](topics/) (",
					"* [Topics](topics/) - "+desc+" (", 1)
			} else {
				edited = strings.Replace(string(raw),
					"* [Topics](topics/)\n",
					"* [Topics](topics/) - "+desc+"\n", 1)
			}
			if edited == string(raw) {
				t.Fatalf("test setup: no Topics bullet to edit in:\n%s", raw)
			}
			if err := os.WriteFile(rootPath, []byte(edited), 0o644); err != nil {
				t.Fatal(err)
			}

			// Rebuild: the parenthesised description must survive verbatim.
			b2, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := WriteIndexWithOptions(b2, tc.opts); err != nil {
				t.Fatalf("WriteIndexWithOptions rebuild: %v", err)
			}
			got, err := os.ReadFile(rootPath)
			if err != nil {
				t.Fatal(err)
			}
			bl := subdirBullet(string(got), "topics/")
			if !strings.Contains(bl, "- "+desc) {
				t.Fatalf("parenthesised description must survive (not eaten as shape suffix); got bullet: %q", bl)
			}
			// And the tool-owned shape suffix must not have been duplicated or lost.
			if tc.opts.Enabled && !strings.Contains(bl, "(1 concept · Concept)") {
				t.Fatalf("shape suffix must still be regenerated alongside the preserved description; got: %q", bl)
			}
		})
	}
}

// TestSubdirDescription_SurvivesSubtreeChangeBetweenBuilds pins that the
// separation of the curator-owned description from the tool-owned shape suffix
// is idempotent even when the child's subtree CHANGES between builds. The shape
// suffix is recomputed every build (§8: tool-owned), so a build that adds or
// removes content under a described subdirectory renders a DIFFERENT suffix than
// the one on disk. The parser must still strip the (now stale) on-disk suffix
// structurally rather than by equality against the freshly computed one — else
// the stale suffix is captured into the description and a fresh suffix is
// appended on top, corrupting the description and doubling the suffix.
func TestSubdirDescription_SurvivesSubtreeChangeBetweenBuilds(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "topics/nouns.md", "Concept", "Nouns", "")

	// Build once, then a curator adds a description to the topics/ bullet.
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	rootPath := filepath.Join(dir, "index.md")
	raw, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	const desc = "Parts of speech."
	edited := strings.Replace(string(raw),
		"* [Topics](topics/) (",
		"* [Topics](topics/) - "+desc+" (", 1)
	if edited == string(raw) {
		t.Fatalf("test setup: no Topics bullet to edit in:\n%s", raw)
	}
	if err := os.WriteFile(rootPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Now the subtree under topics/ changes: a deeper concept is added, so the
	// tool-owned suffix dirShape renders gains a `· N in subtree` segment that
	// the on-disk suffix does not have. Rebuild.
	writeNodeFM(t, dir, "topics/deep/verbs.md", "Concept", "Verbs", "")
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b2); err != nil {
		t.Fatalf("WriteIndex rebuild: %v", err)
	}
	got, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	bl := subdirBullet(string(got), "topics/")

	// The description must be preserved verbatim, exactly once.
	if !strings.Contains(bl, "- "+desc) {
		t.Fatalf("description must survive a subtree change; got bullet: %q", bl)
	}
	// The stale suffix must NOT have leaked into the description: the bullet must
	// carry exactly ONE parenthesised shape group, not two.
	if n := strings.Count(bl, "(1 concept"); n != 1 {
		t.Fatalf("shape suffix must appear exactly once (not doubled), got %d in bullet: %q", n, bl)
	}
	// And the freshly computed suffix carries the new subtree total.
	if !strings.Contains(bl, "in subtree") {
		t.Fatalf("rebuilt suffix must reflect the changed subtree; got bullet: %q", bl)
	}
}

// TestSubdirDescription_TitleRegeneratedKeepsDescription pins that the
// description is keyed by LINK TARGET (topics/), not by title: even when the
// tool regenerates a different title for the folder, the preserved description
// still attaches to the same subdirectory. §8: entries are keyed by their link.
func TestSubdirDescription_TitleRegeneratedKeepsDescription(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "topics/nouns.md", "Concept", "Nouns", "")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	// Curator edits BOTH the title and adds a description. The tool owns the
	// title (it will regenerate "Topics"); the description is keyed by the link.
	rootPath := filepath.Join(dir, "index.md")
	raw, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	const desc = "Parts of speech."
	edited := strings.Replace(string(raw),
		"* [Topics](topics/) (",
		"* [Some Other Title](topics/) - "+desc+" (", 1)
	if edited == string(raw) {
		t.Fatalf("test setup: no Topics bullet to edit in:\n%s", raw)
	}
	if err := os.WriteFile(rootPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b2); err != nil {
		t.Fatalf("WriteIndex rebuild: %v", err)
	}
	got, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	bl := subdirBullet(string(got), "topics/")
	// Title is tool-owned: regenerated back to "Topics".
	if !strings.HasPrefix(bl, "* [Topics](topics/)") {
		t.Errorf("title must be regenerated (tool-owned); got bullet: %q", bl)
	}
	// Description is curator-owned: preserved via link-target match.
	if !strings.Contains(bl, "- "+desc) {
		t.Errorf("description must survive a title regeneration (keyed by link target); got bullet: %q", bl)
	}
}

// TestSubdirDescription_RemovedFolderDropsEntry pins the removal negative
// control: deleting the subfolder removes its entry AND its description; a
// stale description on a vanished link is never resurrected onto another folder.
func TestSubdirDescription_RemovedFolderDropsEntry(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "topics/nouns.md", "Concept", "Nouns", "")
	writeNodeFM(t, dir, "grammar/verbs.md", "Concept", "Verbs", "")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	// Curator adds distinct descriptions to both subdir entries.
	rootPath := filepath.Join(dir, "index.md")
	raw, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw),
		"* [Topics](topics/) (", "* [Topics](topics/) - Topic desc. (", 1)
	edited = strings.Replace(edited,
		"* [Grammar](grammar/) (", "* [Grammar](grammar/) - Grammar desc. (", 1)
	if err := os.WriteFile(rootPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Remove the topics/ subfolder entirely.
	if err := os.RemoveAll(filepath.Join(dir, "topics")); err != nil {
		t.Fatal(err)
	}

	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b2); err != nil {
		t.Fatalf("WriteIndex rebuild: %v", err)
	}
	got, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	gs := string(got)
	if strings.Contains(gs, "](topics/)") {
		t.Errorf("removed folder's entry must be gone; got:\n%s", gs)
	}
	if strings.Contains(gs, "Topic desc.") {
		t.Errorf("removed folder's description must be gone; got:\n%s", gs)
	}
	// The surviving folder keeps its own description, not the vanished one.
	blG := subdirBullet(gs, "grammar/")
	if !strings.Contains(blG, "- Grammar desc.") {
		t.Errorf("surviving folder must keep its OWN description; got bullet: %q", blG)
	}
	if strings.Contains(blG, "Topic desc.") {
		t.Errorf("a vanished folder's description must not be resurrected onto another folder; got bullet: %q", blG)
	}
}

// TestSubdirDescription_NoDescriptionsByteIdenticalToMain is the golden negative
// control: an index with NO subdir descriptions renders byte-identically to the
// pre-fix output. The preservation path must be a no-op when nothing is present.
func TestSubdirDescription_NoDescriptionsByteIdenticalToMain(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeTags(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeNodeTags(t, dir, "design/b.md", "Map", "B", []string{"ux"})
	writeNodeTags(t, dir, "design/ux-psychology/c.md", "Concept", "C", nil)

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// First build with no descriptions anywhere.
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	rootPath := filepath.Join(dir, "index.md")
	first, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}

	// A second build (now that an on-disk index exists to parse) must be a
	// byte-identical no-op — the preservation path finds no descriptions.
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b2); err != nil {
		t.Fatalf("WriteIndex rebuild: %v", err)
	}
	second, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("no-description build must be byte-identical across runs\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	// And it must still contain the expected tool-owned bullet (control).
	if !strings.Contains(string(second), "* [Design](design/) (") {
		t.Errorf("expected tool-owned design/ bullet; got:\n%s", second)
	}
}

// TestSubdirDescription_StaleIndexStillFailsCheck pins the check negative
// control: a genuinely stale index (a new subfolder is missing from it) still
// fails `index check` (out of sync) even when descriptions are present on the
// entries that DO exist. Preservation must not mask real staleness.
func TestSubdirDescription_StaleIndexStillFailsCheck(t *testing.T) {
	dir := t.TempDir()
	_ = Scaffold(dir)
	writeNodeFM(t, dir, "topics/nouns.md", "Concept", "Nouns", "")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	// Add a description to the existing entry.
	rootPath := filepath.Join(dir, "index.md")
	raw, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw),
		"* [Topics](topics/) (", "* [Topics](topics/) - A description. (", 1)
	if err := os.WriteFile(rootPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Now introduce a NEW subfolder that the on-disk index does not mention.
	writeNodeFM(t, dir, "grammar/verbs.md", "Concept", "Verbs", "")

	bCheck, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ok, report := IndexInSync(bCheck)
	if ok {
		t.Errorf("§8: a stale index (missing a new subfolder) must fail check even with descriptions present; report was empty")
	}
	if report == "" {
		t.Errorf("a failing check must name the offending path")
	}
}
