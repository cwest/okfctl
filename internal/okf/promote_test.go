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
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// mkPromoteBundle writes files (rel->content) under a temp dir and loads it.
// Unlike mkMoveBundle it does not assume every file is a concept node, so a test
// can plant reserved index.md files carrying frontmatter (the directory-concept
// shape promote remediates).
func mkPromoteBundle(t *testing.T, files map[string]string) (string, *Bundle) {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return dir, b
}

// treeHash returns a stable hash of every regular file's path+content under
// root, so a test can prove a dry run wrote NOTHING.
func treeHash(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		fmt.Fprintf(h, "%s\x00%x\x00", filepath.ToSlash(rel), data)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// dirConceptIndex renders an index.md authored as a directory-concept: it
// carries frontmatter (the illegal-per-§6 shape) plus a prose body.
func dirConceptIndex(title, created, body string) string {
	return "---\ntype: Concept\ntitle: " + title + "\ncreated: " + created + "\nmodified: " + created + "\n---\n\n" + body
}

func TestPromotableIndexes_OnlyNonRootWithFrontmatter(t *testing.T) {
	_, b := mkPromoteBundle(t, map[string]string{
		// bundle-root index: §11 okf_version carve-out is legal, never promoted.
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n",
		// non-root index carrying frontmatter: PROMOTABLE.
		"foo/index.md": dirConceptIndex("Foo", "2026-01-01", "# Foo\n\nFoo is a concept.\n"),
		// non-root index with NO frontmatter: already conformant, skip.
		"bar/index.md": "# Bar\n\n_No nodes yet._\n",
		// an ordinary concept node so the bundle is non-empty.
		"note.md": nodeSrc("Note"),
	})
	got := PromotableIndexes(b)
	want := []string{"foo/index.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("PromotableIndexes = %v, want %v", got, want)
	}
}

func TestPromotePlan_DefaultBasenameIsDirName(t *testing.T) {
	_, b := mkPromoteBundle(t, map[string]string{
		"index.md":            "# Knowledge Base\n",
		"gke-pm-map/index.md": dirConceptIndex("GKE PM Map", "2026-02-02", "# GKE PM Map\n"),
	})
	plan, err := PromotePlan(b, "")
	if err != nil {
		t.Fatalf("PromotePlan: %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(plan), plan)
	}
	if plan[0].OldPath != "gke-pm-map/index.md" || plan[0].NewPath != "gke-pm-map/gke-pm-map.md" {
		t.Fatalf("default basename: got %s -> %s, want gke-pm-map/index.md -> gke-pm-map/gke-pm-map.md",
			plan[0].OldPath, plan[0].NewPath)
	}
}

func TestPromotePlan_NameOverride(t *testing.T) {
	_, b := mkPromoteBundle(t, map[string]string{
		"index.md":     "# Knowledge Base\n",
		"foo/index.md": dirConceptIndex("Foo", "2026-01-01", "# Foo\n"),
		"bar/index.md": dirConceptIndex("Bar", "2026-01-01", "# Bar\n"),
	})
	plan, err := PromotePlan(b, "overview")
	if err != nil {
		t.Fatalf("PromotePlan: %v", err)
	}
	got := map[string]string{}
	for _, c := range plan {
		got[c.OldPath] = c.NewPath
	}
	want := map[string]string{
		"foo/index.md": "foo/overview.md",
		"bar/index.md": "bar/overview.md",
	}
	for old, wantNew := range want {
		if got[old] != wantNew {
			t.Fatalf("--name override: %s -> %s, want %s", old, got[old], wantNew)
		}
	}
}

func TestPromotePlan_ErrorOnDestinationCollision(t *testing.T) {
	_, b := mkPromoteBundle(t, map[string]string{
		"index.md":     "# Knowledge Base\n",
		"foo/index.md": dirConceptIndex("Foo", "2026-01-01", "# Foo\n"),
		// a concept already occupies the default destination name.
		"foo/foo.md": nodeSrc("Existing Foo"),
	})
	if _, err := PromotePlan(b, ""); err == nil {
		t.Fatalf("expected error: destination foo/foo.md already exists as a node")
	}
}

// TestPromotePlan_RewritesBothSpellings is the load-bearing test: inbound links
// pointing at the promoted directory in BOTH the foo/index.md and foo/ spellings,
// across every relative form (root-relative, dir-relative, /-absolute), must be
// rewritten to the new concept path preserving the author's form.
func TestPromotePlan_RewritesBothSpellings(t *testing.T) {
	_, b := mkPromoteBundle(t, map[string]string{
		"index.md":     "---\nokf_version: \"0.1\"\n---\n\n# KB\n\nSee [Foo](foo/).\n", // reserved, dir-style
		"foo/index.md": dirConceptIndex("Foo", "2026-01-01", "# Foo\n\nBody.\n"),
		// root-relative, explicit index spelling
		"alpha.md": nodeSrc("Alpha") + "Explicit [foo](foo/index.md).\n",
		// root-relative, dir-style spelling
		"bravo.md": nodeSrc("Bravo") + "Dir [foo](foo/).\n",
		// dir-relative from a sibling dir, dir-style
		"baz/qux.md": nodeSrc("Qux") + "Up [foo](../foo/).\n",
		// /-absolute, explicit index spelling
		"charlie.md": nodeSrc("Charlie") + "Abs [foo](/foo/index.md).\n",
	})
	plan, err := PromotePlan(b, "")
	if err != nil {
		t.Fatalf("PromotePlan: %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("expected 1 promoted index, got %d", len(plan))
	}
	rw := plan[0].Rewrites
	find := func(nodePath string) (LinkRewrite, bool) {
		for _, r := range rw {
			if r.NodePath == nodePath {
				return r, true
			}
		}
		return LinkRewrite{}, false
	}
	cases := []struct {
		node    string
		wantNew string
	}{
		{"alpha.md", "foo/foo.md"},      // root-rel index -> root-rel concept
		{"bravo.md", "foo/foo.md"},      // root-rel dir -> root-rel concept
		{"baz/qux.md", "../foo/foo.md"}, // dir-rel dir -> dir-rel concept
		{"charlie.md", "/foo/foo.md"},   // abs index -> abs concept
		{"index.md", "foo/foo.md"},      // reserved root index dir-style link
	}
	for _, c := range cases {
		r, ok := find(c.node)
		if !ok {
			t.Fatalf("expected rewrite for %s; plan rewrites=%+v", c.node, rw)
		}
		if r.New != c.wantNew {
			t.Fatalf("%s: rewrite New=%q, want %q", c.node, r.New, c.wantNew)
		}
	}
}

func TestPromotePlan_PreservesTitleTail(t *testing.T) {
	_, b := mkPromoteBundle(t, map[string]string{
		"index.md":     "# KB\n",
		"foo/index.md": dirConceptIndex("Foo", "2026-01-01", "# Foo\n"),
		"alpha.md":     nodeSrc("Alpha") + "See [foo](foo/index.md \"The Foo\").\n",
	})
	plan, err := PromotePlan(b, "")
	if err != nil {
		t.Fatalf("PromotePlan: %v", err)
	}
	var found bool
	for _, r := range plan[0].Rewrites {
		if r.NodePath == "alpha.md" {
			found = true
			if r.New != "foo/foo.md \"The Foo\"" {
				t.Fatalf("title tail not preserved: New=%q", r.New)
			}
		}
	}
	if !found {
		t.Fatalf("no rewrite for alpha.md")
	}
}

func TestPromotePlan_IsPure_NoWrites(t *testing.T) {
	dir, b := mkPromoteBundle(t, map[string]string{
		"index.md":     "# KB\n",
		"foo/index.md": dirConceptIndex("Foo", "2026-01-01", "# Foo\n"),
		"alpha.md":     nodeSrc("Alpha") + "See [foo](foo/).\n",
	})
	before := treeHash(t, dir)
	if _, err := PromotePlan(b, ""); err != nil {
		t.Fatalf("PromotePlan: %v", err)
	}
	if after := treeHash(t, dir); before != after {
		t.Fatalf("PromotePlan wrote to disk: tree hash changed")
	}
}

func TestPromoteApply_BodyVerbatim_CreatedImmutable(t *testing.T) {
	body := "\n# Foo\n\nFoo has  irregular   spacing and a trailing bullet:\n\n* one\n* two\n"
	dir, b := mkPromoteBundle(t, map[string]string{
		"index.md":     "# KB\n",
		"foo/index.md": "---\ntype: Concept\ntitle: Foo\ncreated: 2026-01-01\nmodified: 2026-01-01\n---\n" + body,
		"alpha.md":     nodeSrc("Alpha") + "See [foo](foo/index.md).\n",
	})
	plan, err := PromotePlan(b, "")
	if err != nil {
		t.Fatalf("PromotePlan: %v", err)
	}
	if err := PromoteApply(dir, b, plan); err != nil {
		t.Fatalf("PromoteApply: %v", err)
	}

	// New concept file exists with byte-identical body region + created intact.
	newRaw, err := os.ReadFile(filepath.Join(dir, "foo", "foo.md"))
	if err != nil {
		t.Fatalf("read promoted file: %v", err)
	}
	_, gotBody, ok := splitFrontmatterRaw(newRaw)
	if !ok {
		t.Fatalf("promoted file has no frontmatter block")
	}
	if string(gotBody) != body {
		t.Fatalf("body not verbatim:\n got %q\nwant %q", string(gotBody), body)
	}
	fm, _, err := ParseFrontmatter(newRaw)
	if err != nil {
		t.Fatalf("parse promoted frontmatter: %v", err)
	}
	if got := fmt.Sprintf("%v", fm["created"]); !strings.Contains(got, "2026-01-01") {
		t.Fatalf("created not immutable: got %q", got)
	}

	// Inbound link rewritten on disk.
	alpha, _ := os.ReadFile(filepath.Join(dir, "alpha.md"))
	if !strings.Contains(string(alpha), "foo/foo.md") || strings.Contains(string(alpha), "foo/index.md") {
		t.Fatalf("alpha inbound link not rewritten: %q", string(alpha))
	}

	// The old directory-concept index.md must no longer be a frontmatter-bearing
	// index (promote removed it; WriteIndex regenerates a clean one at cmd layer).
	if _, err := os.Stat(filepath.Join(dir, "foo", "index.md")); err == nil {
		raw, _ := os.ReadFile(filepath.Join(dir, "foo", "index.md"))
		if _, _, hasFM := splitFrontmatterRaw(raw); hasFM {
			t.Fatalf("old foo/index.md still carries frontmatter after promote")
		}
	}
}
