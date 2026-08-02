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

// TestIndexBuild_EmitsNestedDirRelativeIndexes drives the `index build` CLI
// seam end-to-end on a multi-directory bundle and asserts OKF §6 conformance of
// the produced tree: one index.md per content-bearing directory, each
// enumerating only its own directory's contents, dir-relatively — a subdirectory
// as `child/`, a sibling concept as `file.md` (never bundle-relative) — and
// `index check` exits 0 on that output.
func TestIndexBuild_EmitsNestedDirRelativeIndexes(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"wine/tannin.md", "wine/red/nebbiolo.md", "lifting/squat.md"} {
		if _, err := runOKF(t, "node", "new", rel, "--type", "Reference", "--title", strings.TrimSuffix(filepath.Base(rel), ".md"), "--bundle", dir); err != nil {
			t.Fatalf("node new %s: %v", rel, err)
		}
	}
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}

	// One index per content-bearing directory.
	for _, rel := range []string{"index.md", "wine/index.md", "wine/red/index.md", "lifting/index.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected generated %s: %v", rel, err)
		}
	}

	// Root index: child dirs linked dir-relatively, no nested concept.
	root := readFileStr(t, filepath.Join(dir, "index.md"))
	if !strings.Contains(root, "](wine/)") || !strings.Contains(root, "](lifting/)") {
		t.Errorf("root index must link child dirs as `wine/`/`lifting/`; got:\n%s", root)
	}
	if strings.Contains(root, "wine/tannin.md") {
		t.Errorf("root index must not enumerate nested concepts bundle-relatively; got:\n%s", root)
	}

	// wine/index.md: its own concept (base name) + its child dir; not the grandchild concept.
	wine := readFileStr(t, filepath.Join(dir, "wine", "index.md"))
	if strings.HasPrefix(wine, "---\n") {
		t.Errorf("§8: nested wine/index.md must carry no frontmatter; got:\n%s", wine)
	}
	if !strings.Contains(wine, "](tannin.md)") {
		t.Errorf("wine/index.md must link its concept dir-relatively as `tannin.md`; got:\n%s", wine)
	}
	if !strings.Contains(wine, "](red/)") {
		t.Errorf("wine/index.md must link its child dir as `red/`; got:\n%s", wine)
	}
	if strings.Contains(wine, "nebbiolo.md") {
		t.Errorf("wine/index.md must enumerate ONLY its own directory, not the grandchild; got:\n%s", wine)
	}

	// check exits 0.
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass right after nested build: %v", err)
	}

	// Delete a nested index by hand -> check must fail (missing nested index).
	if err := os.Remove(filepath.Join(dir, "wine", "red", "index.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "check", dir); err == nil {
		t.Fatal("index check must exit nonzero when a nested index is missing")
	}
}
