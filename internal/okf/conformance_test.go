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
	"regexp"
	"strings"
	"testing"
)

// Spec-conformance suite: okfctl is the producer of a spec-governed format, so
// it must be its own first consumer. This suite runs the FULL reserved-file
// generation surface — Scaffold (bundle init), WriteIndex (index build), and
// AppendLog (log append) — and asserts every artifact it produces satisfies the
// OKF spec floor that Validate enforces:
//
//   - §8:  index files carry no frontmatter (bundle-root exception: okf_version
//          only).
//   - §9:  log.md is a date-grouped list, newest-first, ISO-8601 date headings.
//   - §12: okf_version marker semantics on the bundle-root index.
//
// The load-bearing property: Validate(generated bundle) == 0 findings. If a
// future change re-introduces frontmatter/boilerplate on a generated index,
// TestConformance_GeneratedBundleValidatesClean goes RED — the closed loop
// (generate → validate) that never existed at authoring time now exists.

// isoDateHeading matches an OKF §9 date heading: a `## ` line whose text is an
// ISO-8601 YYYY-MM-DD date.
var isoDateHeading = regexp.MustCompile(`^## \d{4}-\d{2}-\d{2}$`)

// assertLogConformsSection9 checks a log.md body against the OKF §9 grammar:
// a leading `# ` title, then zero or more `## YYYY-MM-DD` date headings each
// followed by list entries, newest date first. It tolerates the scaffold's
// "no entries yet" placeholder (a log with no entries is still well-formed).
func assertLogConformsSection9(t *testing.T, body string) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		t.Fatalf("§9: log.md must open with a `# ` title heading; got:\n%s", body)
	}
	var dates []string
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "## ") {
			if !isoDateHeading.MatchString(line) {
				t.Errorf("§9: date heading must be `## YYYY-MM-DD`; got %q in:\n%s", line, body)
			}
			dates = append(dates, strings.TrimPrefix(line, "## "))
		}
	}
	// Newest-first: dates must be in non-increasing order. ISO-8601 sorts
	// lexicographically, so a plain string comparison suffices.
	for i := 1; i < len(dates); i++ {
		if dates[i] > dates[i-1] {
			t.Errorf("§7: log entries must be newest-first; %q precedes newer %q in:\n%s", dates[i-1], dates[i], body)
		}
	}
}

func TestConformance_ScaffoldValidatesClean(t *testing.T) {
	// bundle init surface: a freshly scaffolded bundle must pass Validate with
	// zero findings (it has no concept nodes, so the type floor is vacuous, and
	// the reserved index/log must be spec-clean).
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Errorf("scaffolded bundle must validate clean; got findings: %v", f)
	}

	// §8: the scaffolded index carries no frontmatter.
	raw, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(string(raw), "---\n") {
		t.Errorf("§8: scaffolded index.md must have no frontmatter; got:\n%s", raw)
	}

	// §9: the scaffolded log is well-formed.
	logBody, err := ReadLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertLogConformsSection9(t, logBody)
}

func TestConformance_GeneratedBundleValidatesClean(t *testing.T) {
	// The full generation surface, end to end: scaffold, add concept nodes,
	// regenerate the index (index build), and append a log entry (log append).
	// Every produced artifact must satisfy the spec floor Validate enforces.
	//
	// This is the regression guard the parent defect lacked: if index build
	// re-introduces frontmatter or boilerplate on a generated index, Validate
	// returns a finding here and the test goes RED.
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")
	writeNode(t, dir, "wine/acidity.md", "Reference", "Acidity")
	writeNode(t, dir, "lifting/squat.md", "Playbook", "Squat")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// index build.
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}
	// log append.
	if err := AppendLog(dir, "Added wine and lifting neighborhoods."); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}

	// Reload the on-disk bundle (post index build + log append) and validate it.
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b2); len(f) != 0 {
		t.Fatalf("generated bundle must validate clean after index build + log append; got findings: %v", f)
	}

	// §8: the generated index carries no frontmatter (no okf_version marker in
	// this bundle — no on-disk key, no .okf sidecar... but Scaffold writes a
	// .okf, so the §12 carve-out applies: assert exactly okf_version if present).
	assertGeneratedIndexSection8And12(t, dir)

	// §8 body grammar / no unsanctioned boilerplate.
	idxRaw, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(idxRaw), "Generated by") {
		t.Errorf("§8: generated index must not carry `Generated by` boilerplate; got:\n%s", idxRaw)
	}
	if strings.Contains(string(idxRaw), "type: Index") {
		t.Errorf("§8: generated index must never emit `type: Index`; got:\n%s", idxRaw)
	}

	// §9: the log after an append is well-formed and newest-first.
	logBody, err := ReadLog(dir)
	if err != nil {
		t.Fatal(err)
	}
	assertLogConformsSection9(t, logBody)
	if !strings.Contains(logBody, "Added wine and lifting neighborhoods.") {
		t.Errorf("§9: appended entry missing from log:\n%s", logBody)
	}
}

// assertGeneratedIndexSection8And12 checks the on-disk generated bundle-root
// index against §8/§12: it either carries no frontmatter, or carries a
// frontmatter block whose sole key is okf_version.
func assertGeneratedIndexSection8And12(t *testing.T, dir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	fm, has := frontmatterBlock(string(raw))
	if !has {
		return // §8: no frontmatter — conformant.
	}
	var keys []string
	for _, line := range strings.Split(strings.TrimSpace(fm), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			key, _, _ := strings.Cut(line, ":")
			keys = append(keys, strings.TrimSpace(key))
		}
	}
	if len(keys) != 1 || keys[0] != "okf_version" {
		t.Errorf("§12: generated root index frontmatter must contain exactly okf_version; got keys %v in:\n%s", keys, raw)
	}
}

func TestConformance_GeneratedIndexPreservesOkfVersionMarker(t *testing.T) {
	// §12 marker semantics: a bundle whose root index declares okf_version must
	// keep exactly that value through an index build — the marker the OKF corpus
	// loader uses to discover bundle roots is never dropped or altered — and the
	// result must validate clean.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".okf"), []byte("okf_version: 0.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\nokf_version: \"0.1\"\n---\n\n# Knowledge Base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "log.md"), []byte(logHeader+logPlaceholder), 0o644); err != nil {
		t.Fatal(err)
	}
	writeNode(t, dir, "wine/tannin.md", "Reference", "Tannin")

	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatalf("WriteIndex: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `okf_version: "0.1"`) {
		t.Errorf("§12: index build must preserve the okf_version marker; got:\n%s", raw)
	}
	assertGeneratedIndexSection8And12(t, dir)

	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b2); len(f) != 0 {
		t.Errorf("§12-marker bundle must validate clean after index build; got findings: %v", f)
	}
}

// TestConformance_ValidateFlagsGeneratedIndexRegression is the guard the parent
// defect lacked, expressed at the Validate seam: the exact non-conformant shape
// the old generator emitted (`type: Index`) must be FLAGGED by validate. This
// is what makes the closed loop bite — a generator that regresses to that shape
// produces a bundle validate rejects.
func TestConformance_ValidateFlagsGeneratedIndexRegression(t *testing.T) {
	dir := t.TempDir()
	// Simulate a regressed generator that wrote the pre-fix index shape.
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntype: Index\n---\n\n# Knowledge Base\n\n_Generated by `okfctl index build`. Do not edit by hand._\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !hasFindingFor(Validate(b), "index.md") {
		t.Fatalf("validate must flag a generated index carrying `type: Index` frontmatter; got %v", Validate(b))
	}
}
