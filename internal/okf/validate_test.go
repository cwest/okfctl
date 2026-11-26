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
	"testing"
)

func loadOrFail(t *testing.T, name string) *Bundle {
	t.Helper()
	b, err := Load(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("Load(%s): %v", name, err)
	}
	return b
}

func TestValidate_GoodBundlePasses(t *testing.T) {
	if f := Validate(loadOrFail(t, "good-bundle")); len(f) != 0 {
		t.Errorf("good bundle should pass, got findings: %v", f)
	}
}

// OKF v0.3 §3.2 rule 2 / §11.1: a .md whose frontmatter has no `type` is not a
// concept and does not affect conformance; validators SHOULD warn. (Under v0.2
// §11 rule 2 this failed; v0.3 §13.2 relaxed it.)
func TestValidate_V03S32_MissingTypeWarnsNotFails(t *testing.T) {
	b := loadOrFail(t, "no-type")
	if f := Validate(b); len(f) != 0 {
		t.Errorf("§3.2: a type-less .md must not fail validate, got %v", f)
	}
	if !hasFindingFor(ValidateWarnings(b), "orphan.md") {
		t.Errorf("§3.2: expected a non-concept warning for orphan.md, got %v", ValidateWarnings(b))
	}
}

// §11.2 rule 1: an empty-string `type` is the same as an absent one.
func TestValidate_V03S32_EmptyTypeWarnsNotFails(t *testing.T) {
	b := loadOrFail(t, "empty-type")
	if f := Validate(b); len(f) != 0 {
		t.Errorf("§3.2/§11.2: an empty-type .md must not fail validate, got %v", f)
	}
	if !hasFindingFor(ValidateWarnings(b), "empty.md") {
		t.Errorf("§3.2: expected a non-concept warning for empty.md, got %v", ValidateWarnings(b))
	}
}

func TestValidate_UnknownTypePasses(t *testing.T) {
	// Presence, not taxonomy (PRD §7.4): an unfamiliar type value is valid.
	if f := Validate(loadOrFail(t, "unknown-type")); len(f) != 0 {
		t.Errorf("unknown type value must PASS, got findings: %v", f)
	}
}

func TestValidate_UnparseableFrontmatterFails(t *testing.T) {
	f := Validate(loadOrFail(t, "bad-frontmatter"))
	if !hasFindingFor(f, "broken.md") {
		t.Errorf("expected an unparseable-frontmatter finding for broken.md, got %v", f)
	}
}

func TestValidate_FindingsAreDeterministicallyOrdered(t *testing.T) {
	// A conformance tool whose output is diffed must emit findings in a stable
	// order. Build a bundle with several broken nodes and assert the findings
	// come back sorted by Path, identically on every call.
	b := &Bundle{Nodes: map[string]*Node{
		"zeta.md":      {Path: "zeta.md", Frontmatter: map[string]any{}},
		"alpha.md":     {Path: "alpha.md", Frontmatter: map[string]any{}},
		"mid/mnode.md": {Path: "mid/mnode.md", Frontmatter: nil},
		"beta.md":      {Path: "beta.md", Frontmatter: map[string]any{}},
	}}
	want := []string{"alpha.md", "beta.md", "mid/mnode.md", "zeta.md"}
	for i := 0; i < 5; i++ {
		f := Validate(b)
		if len(f) != len(want) {
			t.Fatalf("run %d: got %d findings, want %d: %v", i, len(f), len(want), f)
		}
		for j, p := range want {
			if f[j].Path != p {
				t.Fatalf("run %d: findings not sorted by Path: got %v, want order %v", i, f, want)
			}
		}
	}
}

func hasFindingFor(fs []Finding, path string) bool {
	for _, f := range fs {
		if f.Path == path {
			return true
		}
	}
	return false
}
