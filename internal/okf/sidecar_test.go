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

// TestReadOkfVersion_UnchangedWithBothKeys is AC7: readOkfVersion still returns
// the declared version when the .okf sidecar carries BOTH okf_version and
// templates. Generalizing the sidecar parser must not regress the version read.
func TestReadOkfVersion_UnchangedWithBothKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "okf_version: 0.9\ntemplates: ../tpl\n")
	if got := readOkfVersion(dir); got != "0.9" {
		t.Errorf("readOkfVersion = %q, want 0.9 (version read must survive a templates key)", got)
	}
}

// TestReadOkfVersion_TemplatesKeyOnlyStillFallsBack confirms that a sidecar with
// only a templates key (no okf_version) still falls back to SpecVersion — the
// version reader is not confused by the new key.
func TestReadOkfVersion_TemplatesKeyOnlyStillFallsBack(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "templates: ../tpl\n")
	if got := readOkfVersion(dir); got != SpecVersion {
		t.Errorf("readOkfVersion = %q, want fallback %q", got, SpecVersion)
	}
}

// TestReadTemplatesRef_ReadsKey covers the new generalized sidecar read: the
// templates key is returned verbatim (relative to the bundle root).
func TestReadTemplatesRef_ReadsKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "okf_version: 0.2\ntemplates: ../shared-templates\n")
	if got := readTemplatesRef(dir); got != "../shared-templates" {
		t.Errorf("readTemplatesRef = %q, want ../shared-templates", got)
	}
}

// TestReadTemplatesRef_AbsentReturnsEmpty confirms a bundle with no templates
// key (or no .okf at all) returns the empty string — the silent-direction
// control for the whole feature (no key => no referenced templates).
func TestReadTemplatesRef_AbsentReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "okf_version: 0.2\n")
	if got := readTemplatesRef(dir); got != "" {
		t.Errorf("readTemplatesRef with no templates key = %q, want empty", got)
	}
	noSidecar := t.TempDir()
	if got := readTemplatesRef(noSidecar); got != "" {
		t.Errorf("readTemplatesRef with no .okf = %q, want empty", got)
	}
}

// TestReadOkfSidecar_ParsesMultipleKeys exercises the generalized parser
// directly: it returns a map of trimmed key/value pairs, tolerating blank lines
// and surrounding whitespace, and is no longer okf_version-only.
func TestReadOkfSidecar_ParsesMultipleKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".okf", "\nokf_version:  0.2 \n\ntemplates:  ../tpl \n")
	m := readOkfSidecar(dir)
	if m["okf_version"] != "0.2" {
		t.Errorf("sidecar okf_version = %q, want 0.2", m["okf_version"])
	}
	if m["templates"] != "../tpl" {
		t.Errorf("sidecar templates = %q, want ../tpl", m["templates"])
	}
}

// pathForBundle is a tiny guard that the testdata good-bundle has no templates
// key, so its resolver path is the silent one.
func TestReadTemplatesRef_GoodBundleHasNoKey(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "good-bundle")
	if got := readTemplatesRef(dir); got != "" {
		t.Errorf("good-bundle readTemplatesRef = %q, want empty", got)
	}
}
