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

func TestIndexBuildThenCheck_ExitsZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "wine/tannin.md", "--type", "Reference", "--title", "Tannin", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("index build: %v", err)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass right after build: %v", err)
	}
}

// After node new, the index is auto-maintained: index check passes with no
// manual build. (This is the increment's contract — a build step a human must
// remember is a build step that drifts, so okfctl maintains it automatically.)
func TestNodeNew_KeepsIndexInSync(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "b.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	// No manual `index build` — auto-maintenance must have kept it current.
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("index check should pass after node new without a manual build: %v", err)
	}
}

// index check still detects a HAND-corrupted index: auto-maintenance keeps
// okfctl-mediated mutations in sync, but a direct edit of index.md is caught.
func TestIndexCheck_HandCorruptedExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "a.md", "--type", "Reference", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	// Corrupt index.md by hand, bypassing okfctl.
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntype: Index\n---\n\n# Stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "check", dir); err == nil {
		t.Fatal("index check must exit nonzero on a hand-corrupted index")
	}
}

// TestIndexBuild_SubDirDoesNotRewriteNeighborhoodHeading is card step 3: running
// `index build <neighborhood-subdir>` (the sub-directory passed as the bundle
// arg, so it loads as the root "") must NOT overwrite the neighborhood's own H1
// (`# Domains`) with the generic root default (`# Knowledge Base`). Before the
// fix this silently corrupted the index: `git diff` showed `-# Domains` /
// `+# Knowledge Base`.
func TestIndexBuild_SubDirDoesNotRewriteNeighborhoodHeading(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	// A `domains` neighborhood: node new auto-maintains domains/index.md with a
	// neighborhood-named H1 (`# Domains`), mirroring the real cwest/knowledge-base.
	if _, err := runOKF(t, "node", "new", "domains/foo.md", "--type", "Reference", "--title", "Foo", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	neighborhood := filepath.Join(dir, "domains")
	idxPath := filepath.Join(neighborhood, "index.md")

	before, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(before), "# Domains\n") {
		t.Fatalf("precondition: domains/index.md must start with `# Domains`; got:\n%s", before)
	}

	// Build against the SUB-DIRECTORY (the footgun scope).
	if _, err := runOKF(t, "index", "build", neighborhood); err != nil {
		t.Fatalf("index build on a neighborhood sub-dir: %v", err)
	}

	after, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "# Knowledge Base") {
		t.Errorf("sub-dir build corrupted the neighborhood H1 to `# Knowledge Base`; got:\n%s", after)
	}
	if !strings.HasPrefix(string(after), "# Domains\n") {
		t.Errorf("sub-dir build must preserve the `# Domains` H1; got:\n%s", after)
	}
}

// TestIndexCheck_SubDirNoFalsePositive is card step 2: after a correct
// root-scope build, `index check <neighborhood-subdir>` must not false-positive
// against the correct on-disk neighborhood index. The on-disk `# Domains` is
// right, so a sub-scope check of it must pass (exit 0).
func TestIndexCheck_SubDirNoFalsePositive(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "domains/foo.md", "--type", "Reference", "--title", "Foo", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	// Authoritative build at bundle-root scope (the CI gate's scope).
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("root-scope index build: %v", err)
	}
	// The on-disk domains/index.md is now correct; a sub-scope check must agree.
	if _, err := runOKF(t, "index", "check", filepath.Join(dir, "domains")); err != nil {
		t.Fatalf("index check on a correct neighborhood sub-dir must pass (no false positive): %v", err)
	}
}

// TestIndexBuildThenCheck_RootScopeUnchanged is the bundle-root control: the
// documented happy path (`index build <root>` then `index check <root>`) stays
// green, so the fix does not regress root-scope behavior.
func TestIndexBuildThenCheck_RootScopeUnchanged(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "node", "new", "domains/foo.md", "--type", "Reference", "--title", "Foo", "--bundle", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := runOKF(t, "index", "build", dir); err != nil {
		t.Fatalf("root-scope index build: %v", err)
	}
	if _, err := runOKF(t, "index", "check", dir); err != nil {
		t.Fatalf("root-scope index check should pass right after build: %v", err)
	}
	// Root index keeps the generic `# Knowledge Base` title.
	root, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root), "# Knowledge Base") {
		t.Errorf("root index must keep `# Knowledge Base`; got:\n%s", root)
	}
}
