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

// writeConcept writes a concept node with type/title/tags for the CLI shape
// tests.
func writeConcept(t *testing.T, dir, rel, typ, title string, tags []string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\ntype: " + typ + "\ntitle: " + title + "\n"
	if len(tags) > 0 {
		fm += "tags:\n"
		for _, tg := range tags {
			fm += "  - " + tg + "\n"
		}
	}
	fm += "---\n\n# " + title + "\n"
	if err := os.WriteFile(p, []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIndexBuild_DefaultShapeInRootIndex pins that `index build` with no flags
// writes the shape suffix into the root index's subdirectory entries.
func TestIndexBuild_DefaultShapeInRootIndex(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	writeConcept(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeConcept(t, dir, "design/b.md", "Map", "B", []string{"ux"})
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	root, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root), "* [Design](design/) (2 concepts · Concept, Map · shared tags: ux)") {
		t.Errorf("default build must carry the shape suffix; got:\n%s", root)
	}
}

// TestIndexBuild_NoShapeFlag pins --no-shape: the root index's subdirectory
// entries are bare §8 bullets, and a subsequent `index check --no-shape` passes.
func TestIndexBuild_NoShapeFlag(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	writeConcept(t, dir, "design/a.md", "Concept", "A", []string{"ux"})
	writeConcept(t, dir, "design/b.md", "Map", "B", []string{"ux"})
	if _, err := runOKF(t, "index", "build", "--no-shape", dir); err != nil {
		t.Fatalf("index build --no-shape: %v", err)
	}
	root, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root), "* [Design](design/)\n") {
		t.Errorf("--no-shape must render the bare §8 bullet; got:\n%s", root)
	}
	if strings.Contains(string(root), "concepts") {
		t.Errorf("--no-shape must carry no shape text; got:\n%s", root)
	}
	// check must be symmetric: --no-shape check on a --no-shape build passes.
	if _, err := runOKF(t, "index", "check", "--no-shape", dir); err != nil {
		t.Errorf("index check --no-shape must pass after build --no-shape: %v", err)
	}
	// A DEFAULT check (shape-on) on a --no-shape build reports drift.
	if _, err := runOKF(t, "index", "check", dir); err == nil {
		t.Error("default index check must report drift on a --no-shape build")
	}
}

// TestIndexBuild_TagMinMax pins that --shape-tag-min and --shape-tag-max bound
// the tag list end-to-end through the CLI.
func TestIndexBuild_TagMinMax(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	// three tags each shared by exactly 2 of 3 nodes.
	writeConcept(t, dir, "g/a.md", "Concept", "A", []string{"t1", "t2", "t3"})
	writeConcept(t, dir, "g/b.md", "Concept", "B", []string{"t1", "t2", "t3"})
	writeConcept(t, dir, "g/c.md", "Concept", "C", []string{"solo"})

	// --shape-tag-max 2 caps to 2 tags + ellipsis.
	if _, err := runOKF(t, "index", "build", "--shape-tag-max", "2", dir); err != nil {
		t.Fatalf("index build --shape-tag-max: %v", err)
	}
	root, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	line := shapeLineFor(string(root), "](g/)")
	if strings.Count(line, "t") < 2 || !strings.Contains(line, "…") {
		t.Errorf("--shape-tag-max 2 must cap to 2 tags with an ellipsis; got: %q", line)
	}

	// --shape-tag-min 3 suppresses all tags (none shared by 3 nodes).
	if _, err := runOKF(t, "index", "build", "--shape-tag-min", "3", dir); err != nil {
		t.Fatalf("index build --shape-tag-min: %v", err)
	}
	root2, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	line2 := shapeLineFor(string(root2), "](g/)")
	if strings.Contains(line2, "shared tags") {
		t.Errorf("--shape-tag-min 3 must suppress the tag segment; got: %q", line2)
	}
}

// shapeLineFor returns the first line of md containing needle.
func shapeLineFor(md, needle string) string {
	for _, l := range strings.Split(md, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	return ""
}
