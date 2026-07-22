// Copyright 2026 Casey West
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

func TestValidate_MissingTypeFails(t *testing.T) {
	f := Validate(loadOrFail(t, "no-type"))
	if !hasFindingFor(f, "orphan.md") {
		t.Errorf("expected a missing-type finding for orphan.md, got %v", f)
	}
}

func TestValidate_EmptyTypeFails(t *testing.T) {
	f := Validate(loadOrFail(t, "empty-type"))
	if !hasFindingFor(f, "empty.md") {
		t.Errorf("expected an empty-type finding for empty.md, got %v", f)
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

func hasFindingFor(fs []Finding, path string) bool {
	for _, f := range fs {
		if f.Path == path {
			return true
		}
	}
	return false
}
