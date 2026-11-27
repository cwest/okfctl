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
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// v02CLIFixture is an undeclared bundle (no .okf, no root frontmatter, like a
// real v0.2 corpus) whose sub/x.md carries a bare link that resolves only from
// the bundle root (§12.1) beside a file-relative one.
func v02CLIFixture() map[string]string {
	return map[string]string{
		"index.md": "# KB\n\n* [X](sub/x.md)\n",
		"log.md":   "# Log\n",
		"y.md":     "---\ntype: Concept\ntitle: Y\n---\n\n# Y\n",
		"sub/z.md": "---\ntype: Concept\ntitle: Z\n---\n\n# Z\n",
		"sub/x.md": "---\ntype: Concept\ntitle: X\n---\n\n# X\n\nSee [y](y.md) and [z](z.md).\n",
	}
}

// TestConformance_MigrateCommandV02ToV03_Section6_Section8_1 drives plan,
// dry-run, and apply end to end: the dry run prints the unified diff and
// writes nothing, the apply writes exactly that, and the bundle validates as
// v0.3 with the root-relative link made explicit (§6) and the version declared
// on the root index.md (§8.1).
func TestConformance_MigrateCommandV02ToV03_Section6_Section8_1(t *testing.T) {
	dir := mkPromoteCLIBundle(t, v02CLIFixture())
	planPath := filepath.Join(t.TempDir(), "p.json")
	out, err := runOKF(t, "migrate", dir, "--plan", planPath)
	if err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	if !strings.Contains(out, "version: undeclared -> declare v0.3") ||
		!strings.Contains(out, "root-relative links->/ 1") {
		t.Fatalf("plan summary missing counts:\n%s", out)
	}
	before := cliTreeHash(t, dir)
	dry, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath, "--dry-run")
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, dry)
	}
	if cliTreeHash(t, dir) != before {
		t.Fatalf("--dry-run wrote to disk")
	}
	for _, want := range []string{
		"--- a/sub/x.md\n+++ b/sub/x.md\n",
		"-See [y](y.md) and [z](z.md).\n+See [y](/y.md) and [z](z.md).\n",
		"--- a/index.md\n+++ b/index.md\n",
		"+okf_version: \"0.3\"\n",
		"2 file(s) would change (dry run; nothing written).",
	} {
		if !strings.Contains(dry, want) {
			t.Errorf("dry-run diff missing %q:\n%s", want, dry)
		}
	}
	applied, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath)
	if err != nil {
		t.Fatalf("apply: %v\n%s", err, applied)
	}
	if !strings.Contains(applied, "bundle valid as v0.3") {
		t.Fatalf("apply output:\n%s", applied)
	}
	if got := readFileStr(t, filepath.Join(dir, "sub/x.md")); !strings.Contains(got, "See [y](/y.md) and [z](z.md).") {
		t.Fatalf("§6: apply did not write the dry-run diff:\n%s", got)
	}
	info, err := runOKF(t, "bundle", "info", dir)
	if err != nil || !strings.Contains(info, "okf_version: 0.3 (declared in index.md)") {
		t.Fatalf("§8.1: bundle info after migrate: %v\n%s", err, info)
	}
	if out, err := runOKF(t, "lint", "--strict", dir); err != nil {
		t.Fatalf("lint --strict after migrate: %v\n%s", err, out)
	}
}

// TestConformance_MigrateCommand_Section12_FutureVersionKept is the negative
// control at the command level: a bundle declaring a later version plans
// nothing, its dry run changes no file, and apply leaves it byte-identical.
func TestConformance_MigrateCommand_Section12_FutureVersionKept(t *testing.T) {
	files := v02CLIFixture()
	files["index.md"] = "---\nokf_version: \"0.4\"\n---\n\n# KB\n"
	dir := mkPromoteCLIBundle(t, files)
	before := cliTreeHash(t, dir)
	planPath := filepath.Join(t.TempDir(), "p.json")
	out, err := runOKF(t, "migrate", dir, "--plan", planPath)
	if err != nil || !strings.Contains(out, "version: 0.4 -> keep") {
		t.Fatalf("§12: plan for 0.4: %v\n%s", err, out)
	}
	dry, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath, "--dry-run")
	if err != nil || !strings.Contains(dry, "0 file(s) would change") {
		t.Fatalf("§12: dry run for 0.4: %v\n%s", err, dry)
	}
	if out, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath); err != nil || !strings.Contains(out, "bundle valid as v0.4") {
		t.Fatalf("§12: apply for 0.4: %v\n%s", err, out)
	}
	if cliTreeHash(t, dir) != before {
		t.Fatalf("§12: a bundle declaring 0.4 was rewritten")
	}
}

// TestBundleInit_DeclaresV03_Section8_1: `bundle init` writes okf_version
// "0.3" on the root index.md and `bundle info` reports it as declared.
func TestBundleInit_DeclaresV03_Section8_1(t *testing.T) {
	dir := t.TempDir()
	if _, err := runOKF(t, "bundle", "init", dir); err != nil {
		t.Fatal(err)
	}
	if got := readFileStr(t, filepath.Join(dir, "index.md")); !strings.HasPrefix(got, "---\nokf_version: \"0.3\"\n---\n") {
		t.Fatalf("§8.1: index.md = %q", got)
	}
	out, err := runOKF(t, "bundle", "info", dir)
	if err != nil || !strings.Contains(out, "okf_version: 0.3 (declared in index.md)") {
		t.Fatalf("bundle info: %v\n%s", err, out)
	}
}

func TestUnifiedDiff_Hunks(t *testing.T) {
	a := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"
	b := "0\n1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11x\n12\n"
	got := unifiedDiff("f", a, b)
	want := "--- a/f\n+++ b/f\n@@ -1,3 +1,4 @@\n+0\n 1\n 2\n 3\n@@ -8,5 +9,5 @@\n 8\n 9\n 10\n-11\n+11x\n 12\n"
	if got != want {
		t.Fatalf("diff:\n%s\nwant:\n%s", got, want)
	}
	if unifiedDiff("f", a, a) != "" {
		t.Fatal("equal texts must give no diff")
	}
}

// A changed middle too large for the LCS table falls back to one replaced
// block: still a correct diff (applying it yields the new text), with bounded
// memory.
func TestUnifiedDiff_LargeMiddleFallsBack(t *testing.T) {
	var a, b strings.Builder
	a.WriteString("head\n")
	b.WriteString("head\n")
	n := 3000 // 3000*3000 cells > maxLCSCells
	for i := 0; i < n; i++ {
		fmt.Fprintf(&a, "a%d\n", i)
		fmt.Fprintf(&b, "b%d\n", i)
	}
	a.WriteString("tail\n")
	b.WriteString("tail\n")
	got := unifiedDiff("f", a.String(), b.String())
	if !strings.HasPrefix(got, "--- a/f\n+++ b/f\n@@ -1,3002 +1,3002 @@\n head\n-a0\n") {
		t.Fatalf("unexpected header:\n%.200s", got)
	}
	if strings.Count(got, "\n-a") != n || strings.Count(got, "\n+b") != n || !strings.HasSuffix(got, "+b2999\n tail\n") {
		t.Fatalf("fallback diff is not a full replace of the middle")
	}
}
