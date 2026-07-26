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
	"testing"
)

// OKF §6: "Index files contain no frontmatter." OKF §11: the bundle-root
// index.md MAY carry a frontmatter block containing okf_version — "the only
// place frontmatter is permitted in an index.md" — and nothing else.
//
// okfctl is the tool that ENFORCES the spec, so its validator must FLAG an
// index that violates §6/§11 rather than silently tolerate it. A validator
// that passes a frontmatter-bearing index is the same class of defect the
// generator carried: the closed loop (produce → validate) never closes.
//
// These tests pin the validate floor to that rule. writeReserved writes a
// reserved file verbatim (no type synthesis) so the frontmatter is exactly
// what the test declares.

func writeReserved(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_RootIndexOkfVersionOnlyPasses(t *testing.T) {
	dir := t.TempDir()
	// §11 carve-out: the bundle-root index MAY carry an okf_version-only block.
	writeReserved(t, dir, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Errorf("root index with only okf_version must pass; got findings: %v", f)
	}
}

func TestValidate_RootIndexNoFrontmatterPasses(t *testing.T) {
	dir := t.TempDir()
	// §6: an index with no frontmatter at all is conformant.
	writeReserved(t, dir, "index.md", "# Knowledge Base\n\n- [Tannin](wine/tannin.md)\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Errorf("index with no frontmatter must pass; got findings: %v", f)
	}
}

func TestValidate_RootIndexExtraFrontmatterKeyFlagged(t *testing.T) {
	dir := t.TempDir()
	// §11: okf_version is "the only place frontmatter is permitted" — a
	// `type: Index` key alongside it exceeds the carve-out.
	writeReserved(t, dir, "index.md", "---\nokf_version: \"0.1\"\ntype: Index\n---\n\n# Knowledge Base\n")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingFor(Validate(b), "index.md") {
		t.Errorf("root index carrying a non-okf_version key (type: Index) must be flagged; got %v", Validate(b))
	}
}

func TestValidate_RootIndexTypeOnlyFlagged(t *testing.T) {
	dir := t.TempDir()
	// The exact shape the pre-fix generator emitted: `type: Index` and no
	// okf_version. This is a §6/§11 violation and validate must flag it.
	writeReserved(t, dir, "index.md", "---\ntype: Index\n---\n\n# Knowledge Base\n")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingFor(Validate(b), "index.md") {
		t.Errorf("root index with type: Index frontmatter must be flagged; got %v", Validate(b))
	}
}

func TestValidate_NonRootIndexAnyFrontmatterFlagged(t *testing.T) {
	dir := t.TempDir()
	// §6 has NO carve-out for a non-root index: any frontmatter at all — even
	// okf_version — is a violation there.
	writeReserved(t, dir, "index.md", "# Knowledge Base\n")
	writeReserved(t, dir, "wine/index.md", "---\nokf_version: \"0.1\"\n---\n\n# Wine\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingFor(Validate(b), "wine/index.md") {
		t.Errorf("non-root index carrying any frontmatter must be flagged; got %v", Validate(b))
	}
}

func TestValidate_NonRootIndexNoFrontmatterPasses(t *testing.T) {
	dir := t.TempDir()
	writeReserved(t, dir, "index.md", "# Knowledge Base\n")
	writeReserved(t, dir, "wine/index.md", "# Wine\n\n- [Tannin](tannin.md)\n")
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Errorf("non-root index with no frontmatter must pass; got findings: %v", f)
	}
}
