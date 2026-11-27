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
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func runOKF(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestValidateCmd_GoodBundleExitsZero(t *testing.T) {
	dir := filepath.Join("..", "testdata", "good-bundle")
	if _, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("validate good bundle returned error (nonzero exit): %v", err)
	}
}

// OKF v0.3 §3.2 rule 2: a type-less .md is not a concept, does not affect
// conformance, and validators SHOULD warn. validate exits 0 and prints a
// warning naming the file.
func TestValidateCmd_V03S32_MissingTypeWarnsExitsZero(t *testing.T) {
	dir := filepath.Join("..", "testdata", "no-type")
	out, err := runOKF(t, "validate", dir)
	if err != nil {
		t.Fatalf("validate no-type must exit 0 under §3.2; err=%v out=%q", err, out)
	}
	if !strings.Contains(out, "warning orphan.md: not a concept") || !strings.Contains(out, "§3.2") {
		t.Errorf("expected a §3.2 non-concept warning for orphan.md; out=%q", out)
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("a non-concept must never FAIL; out=%q", out)
	}
}

// Negative control: §11.1 rule 1 still fails a .md whose frontmatter block does
// not parse.
func TestValidateCmd_V03S32_UnparseableFrontmatterExitsNonZero(t *testing.T) {
	dir := filepath.Join("..", "testdata", "bad-frontmatter")
	out, err := runOKF(t, "validate", dir)
	if err == nil {
		t.Fatalf("validate bad-frontmatter must exit non-zero; out=%q", out)
	}
}

// The OKF v0.3 §3.2 controls end to end through the CLI: a type-less README.md in
// a sub-directory warns and does not fail; `.draft.md` and `.hidden/x.md` are not
// part of the bundle (never reported, never graph nodes, even under
// --no-ignore); the typed concept beside them is unaffected.
func TestValidateCmd_V03S32_Controls(t *testing.T) {
	dir := writeLintFixture(t, map[string]string{
		"index.md":         "# KB\n\n- [Tannin](wine/tannin.md)\n",
		"wine/tannin.md":   doc("Reference", "Tannin", "Body."),
		"wine/README.md":   "# Wine\n\nHow this folder works.\n",
		".draft.md":        doc("Reference", "Draft", "Not yet."),
		".hidden/x.md":     "---\ntype: [\n---\n# broken\n",
		".hidden/index.md": "---\ntype: Index\n---\n# Hidden\n",
	})
	for _, extra := range [][]string{nil, {"--no-ignore"}} {
		args := append([]string{"validate"}, extra...)
		out, err := runOKF(t, append(args, dir)...)
		if err != nil {
			t.Fatalf("%v: validate must exit 0; err=%v\n%s", extra, err, out)
		}
		if !strings.Contains(out, "warning wine/README.md: not a concept (no frontmatter)") {
			t.Errorf("%v: want a §3.2 warning for wine/README.md; out:\n%s", extra, out)
		}
		if strings.Contains(out, ".draft") || strings.Contains(out, ".hidden") {
			t.Errorf("%v: dot-paths must never be reported (§3.2 rule 4); out:\n%s", extra, out)
		}
		if strings.Contains(out, "tannin") {
			t.Errorf("%v: the typed concept must not be reported; out:\n%s", extra, out)
		}
		if !strings.Contains(out, "OK: bundle conforms") {
			t.Errorf("%v: want the conformance OK line; out:\n%s", extra, out)
		}
	}
	g, err := runOKF(t, "graph", "export", dir)
	if err != nil {
		t.Fatalf("graph export: %v\n%s", err, g)
	}
	if !strings.Contains(g, "wine/tannin.md") {
		t.Errorf("graph must contain the typed concept; got:\n%s", g)
	}
	for _, bad := range []string{"README", ".draft", ".hidden"} {
		if strings.Contains(g, bad) {
			t.Errorf("graph must not contain %s (§3.2); got:\n%s", bad, g)
		}
	}
}
