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

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// §8.1/§12: the root index.md declaration wins over the legacy .okf sidecar.
// A disagreement is a validate WARNING (exit 0), failing only under --strict.
// Negative control: agreeing declarations print no warning.
func TestConformance_Validate_Section12_VersionConflictWarnsNeverFails(t *testing.T) {
	conflict := writeTree(t, map[string]string{
		"index.md": "---\nokf_version: \"0.3\"\n---\n\n# KB\n",
		".okf":     "okf_version: 0.2\n",
	})
	out, err := runOKF(t, "validate", conflict)
	if err != nil {
		t.Fatalf("a version conflict must not fail validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "warning index.md: okf_version \"0.3\"") || !strings.Contains(out, "\"0.2\"") {
		t.Fatalf("want a version-conflict warning naming both values; got:\n%s", out)
	}
	if _, err := runOKF(t, "validate", "--strict", conflict); err == nil {
		t.Fatalf("--strict must fail on the version-conflict warning")
	}

	agree := writeTree(t, map[string]string{
		"index.md": "---\nokf_version: \"0.2\"\n---\n\n# KB\n",
		".okf":     "okf_version: 0.2\n",
	})
	out, err = runOKF(t, "validate", agree)
	if err != nil || strings.Contains(out, "warning") {
		t.Fatalf("agreeing declarations: want clean, got err=%v\n%s", err, out)
	}
}

// bundle info exposes declared-vs-undeclared: an undeclared bundle is reported
// as undeclared and read with the §12.1 v0.2 fallbacks, not as the build's
// target version; a bundle that declares a version reports where it did.
func TestConformance_BundleInfo_Section12_DeclaredVersusAssumed(t *testing.T) {
	undeclared := writeTree(t, map[string]string{"index.md": "# KB\n"})
	out, err := runOKF(t, "bundle", "info", undeclared)
	if err != nil || !strings.Contains(out, "okf_version: undeclared (v0.2 fallbacks apply, §12.1)") {
		t.Fatalf("undeclared bundle: got err=%v\n%s", err, out)
	}
	declared := writeTree(t, map[string]string{"index.md": "---\nokf_version: \"0.3\"\n---\n\n# KB\n"})
	out, err = runOKF(t, "bundle", "info", declared)
	if err != nil || !strings.Contains(out, "okf_version: 0.3 (declared in index.md)") {
		t.Fatalf("declared bundle: got err=%v\n%s", err, out)
	}
	sidecar := writeTree(t, map[string]string{".okf": "okf_version: 0.1\n"})
	out, err = runOKF(t, "bundle", "info", sidecar)
	if err != nil || !strings.Contains(out, "okf_version: 0.1 (declared in .okf)") {
		t.Fatalf("sidecar bundle: got err=%v\n%s", err, out)
	}
}

// §6.4 end to end: graph export carries path-field edges, and lint --strict
// reports a stale path field (§6.2) as a broken link.
func TestConformance_GraphAndLint_Section6_4_PathFieldsEndToEnd(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"index.md":       "---\nokf_version: \"0.3\"\n---\n\n# KB\n\n* [Rev](metrics/rev.md)\n* [GM](metrics/gm.md)\n",
		"metrics/gm.md":  "---\ntype: Metric\ntitle: GM\n---\n\nGross margin.\n",
		"metrics/rev.md": "---\ntype: Metric\ntitle: Rev\nstatus: deprecated\nsuperseded_by: gm.md\n---\n\nRevenue.\n",
		"old.md":         "---\ntype: Metric\ntitle: Old\nstatus: deprecated\nsuperseded_by: /gm.md\n---\n\n[Rev](metrics/rev.md)\n",
	})
	out, err := runOKF(t, "graph", "export", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\"from\": \"metrics/rev.md\",\n      \"to\": \"metrics/gm.md\"") {
		t.Fatalf("§6.4: graph export must carry the superseded_by edge; got:\n%s", out)
	}
	out, err = runOKF(t, "lint", "--strict", dir)
	if err == nil || !strings.Contains(out, "path field superseded_by: /gm.md") {
		t.Fatalf("§6.2: lint --strict must flag the stale path field; err=%v\n%s", err, out)
	}
}
