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
	"strings"
	"testing"
)

// Spec-conformance suite for the v0.2 -> v0.3 migration (OKF v0.3 §13.1, §6,
// §6.2, §12.1, §5, §8.1). The closed loop: a v0.2 (or undeclared) bundle whose
// bare paths resolve only through the §12.1 bundle-root fallback is migrated,
// declares okf_version "0.3", and every one of those values then resolves to
// the SAME target under strict §6 resolution, with no fallback.

func readRel(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func migrateApply(t *testing.T, dir string) MigratePlan {
	t.Helper()
	b := loadMigrate(t, dir)
	plan, err := PlanMigration(b, "")
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	return plan
}

// v02FallbackBundle declares 0.2 and holds a concept in sub/ whose bare body
// link and bare path field resolve ONLY from the bundle root (§12.1), next to
// look-alikes that must stay untouched.
func v02FallbackBundle(t *testing.T, index string) string {
	return mkMigrateBundle(t, map[string]string{
		"index.md":    index,
		"log.md":      "# Log\n",
		"y.md":        "---\ntype: Concept\ntitle: Y\n---\n\n# Y\n",
		"both.md":     "---\ntype: Concept\ntitle: Root both\n---\n\n# Both\n",
		"refs/run.py": "print(1)\n",
		"sub/z.md":    "---\ntype: Concept\ntitle: Z\n---\n\n# Z\n",
		"sub/both.md": "---\ntype: Concept\ntitle: Sub both\n---\n\n# Both\n",
		"sub/x.md": "---\ntype: Concept\ntitle: X\ncomputation: refs/run.py\nresource: /refs/run.py\n---\n\n# X\n\n" +
			"Root-only: [y](y.md#sec \"Title\") and [ref][r].\n\n" +
			"File-relative: [z](z.md). Rooted: [y2](/y.md). URL: [u](https://example.com/y.md).\n" +
			"Both exist: [b](both.md). Dangling: [d](nowhere.md).\n\n" +
			"[r]: y.md\n",
	})
}

// TestConformance_MigrateV02ToV03_Section6_RootFallbackMadeExplicit is the
// positive control: every value that resolved only through the §12.1 fallback
// is rewritten to the "/"-rooted form (§6 rule 2), with fragment and title kept.
func TestConformance_MigrateV02ToV03_Section6_RootFallbackMadeExplicit(t *testing.T) {
	dir := v02FallbackBundle(t, v02Index)
	plan := migrateApply(t, dir)
	if plan.TargetVersion != "0.3" {
		t.Fatalf("§12: target version = %q, want 0.3", plan.TargetVersion)
	}
	got := readRel(t, dir, "sub/x.md")
	for _, want := range []string{
		"computation: /refs/run.py\n",
		`[y](/y.md#sec "Title")`,
		"[r]: /y.md\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("§6: want %q in migrated file:\n%s", want, got)
		}
	}
	b := loadMigrate(t, dir)
	if b.DeclaredOkfVersion != "0.3" {
		t.Fatalf("§8.1: declared version = %q, want 0.3", b.DeclaredOkfVersion)
	}
	// Strict v0.3 resolution (no fallback) reaches the same targets.
	if r := b.ResolveLink("sub/x.md", "/y.md#sec"); r.Path != "y.md" || r.Form != FormRootAbs {
		t.Errorf("§6 rule 2: link resolves to %+v", r)
	}
	if r := b.ResolvePathField("sub/x.md", "/refs/run.py"); !r.Exists || r.Form != FormRootAbs {
		t.Errorf("§6.2: path field resolves to %+v", r)
	}
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("§11: migrated bundle has findings: %+v", f)
	}
	for _, f := range Lint(b, LintOptions{}) {
		if f.Check == "broken-link" {
			t.Errorf("§6: migrated bundle reports a broken link: %s", f.Message)
		}
	}
}

// TestConformance_MigrateV02ToV03_Section6_LookAlikesUntouched is the negative
// control: a file-relative link, a "/"-rooted link, a URL, a bare link whose
// file-relative target exists (it wins over the fallback, §6.2), a dangling
// link, and a "/"-rooted path field are all left byte-identical.
func TestConformance_MigrateV02ToV03_Section6_LookAlikesUntouched(t *testing.T) {
	dir := v02FallbackBundle(t, v02Index)
	migrateApply(t, dir)
	got := readRel(t, dir, "sub/x.md")
	for _, want := range []string{
		"[z](z.md)", "[y2](/y.md)", "[u](https://example.com/y.md)",
		"[b](both.md)", "[d](nowhere.md)", "resource: /refs/run.py\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("look-alike changed: want %q in:\n%s", want, got)
		}
	}
	for _, rel := range []string{"y.md", "both.md", "sub/z.md", "sub/both.md"} {
		b := readRel(t, dir, rel)
		if strings.Contains(b, "okf_version") {
			t.Errorf("%s gained a version key", rel)
		}
	}
}

// TestConformance_MigrateV02ToV03_Section12_Idempotent: a second plan carries
// no edit and a second apply leaves the tree byte-identical.
func TestConformance_MigrateV02ToV03_Section12_Idempotent(t *testing.T) {
	dir := v02FallbackBundle(t, v02Index)
	migrateApply(t, dir)
	before := treeHash(t, dir)
	plan := migrateApply(t, dir)
	if len(plan.Nodes) != 0 || len(plan.Judgment) != 0 {
		t.Fatalf("second plan is not empty: %+v", plan)
	}
	if treeHash(t, dir) != before {
		t.Fatalf("second migrate changed the tree")
	}
}

// TestConformance_MigrateUndeclared_Section12_1_FallbackBeforeAndAfterBump
// proves the §12.1 fallback keys on the DECLARED version: an undeclared bundle
// reads as the build default (0.3) yet still resolves through the fallback;
// migrate then declares "0.3" on the root index.md (§8.1 SHOULD) and the
// values resolve without it.
func TestConformance_MigrateUndeclared_Section12_1_FallbackBeforeAndAfterBump(t *testing.T) {
	dir := v02FallbackBundle(t, "# KB\n\n* [X](sub/x.md)\n")
	b := loadMigrate(t, dir)
	if b.DeclaredOkfVersion != "" || b.OkfVersionSource != VersionUndeclared {
		t.Fatalf("precondition: want undeclared, got %q from %q", b.DeclaredOkfVersion, b.OkfVersionSource)
	}
	if b.OkfVersion != SpecVersion || SpecVersion != "0.3" {
		t.Fatalf("undeclared bundle reads as %q (SpecVersion %q), want 0.3", b.OkfVersion, SpecVersion)
	}
	if r := b.ResolveLink("sub/x.md", "y.md"); r.Form != FormRootFallback || r.Path != "y.md" {
		t.Fatalf("§12.1: undeclared bundle must keep the fallback after the bump; got %+v", r)
	}

	migrateApply(t, dir)
	idx := readRel(t, dir, "index.md")
	if !strings.HasPrefix(idx, "---\nokf_version: \"0.3\"\n---\n\n# KB\n") {
		t.Fatalf("§8.1: want a quoted okf_version block prepended, body kept; got:\n%s", idx)
	}
	b2 := loadMigrate(t, dir)
	if b2.DeclaredOkfVersion != "0.3" || b2.OkfVersionSource != VersionFromIndex {
		t.Fatalf("§8.1: want declared 0.3 from index.md; got %q from %q", b2.DeclaredOkfVersion, b2.OkfVersionSource)
	}
	if r := b2.ResolveLink("sub/x.md", "/y.md"); r.Form != FormRootAbs || !r.Exists {
		t.Fatalf("§6: migrated link must resolve strictly; got %+v", r)
	}
	if f := Validate(b2); len(f) != 0 {
		t.Fatalf("§11: findings after migrate: %+v", f)
	}
}

// TestConformance_MigrateRootIndexMarker_Section8_1 covers the marker forms:
// an existing frontmatter block gains the key at its end (every other byte
// kept), and a CRLF index keeps CRLF.
func TestConformance_MigrateRootIndexMarker_Section8_1(t *testing.T) {
	t.Run("frontmatter without okf_version", func(t *testing.T) {
		dir := mkMigrateBundle(t, map[string]string{
			"index.md": "---\ntitle:   Acme  # display name\nlicense: CC-BY-4.0\n---\n\n# KB\n",
			"log.md":   "# Log\n",
		})
		migrateApply(t, dir)
		want := "---\ntitle:   Acme  # display name\nlicense: CC-BY-4.0\nokf_version: \"0.3\"\n---\n\n# KB\n"
		if got := readRel(t, dir, "index.md"); got != want {
			t.Fatalf("§8.1/§4: got:\n%q\nwant:\n%q", got, want)
		}
	})
	t.Run("CRLF index without frontmatter", func(t *testing.T) {
		dir := mkMigrateBundle(t, map[string]string{
			"index.md": "# KB\r\n",
			"log.md":   "# Log\n",
		})
		migrateApply(t, dir)
		want := "---\r\nokf_version: \"0.3\"\r\n---\r\n\r\n# KB\r\n"
		if got := readRel(t, dir, "index.md"); got != want {
			t.Fatalf("CRLF: got %q want %q", got, want)
		}
	})
	t.Run("legacy sidecar is bumped quoted", func(t *testing.T) {
		dir := mkMigrateBundle(t, map[string]string{
			".okf":     "okf_version: 0.2\ntemplates: ../tpl\n",
			"index.md": "---\nokf_version: \"0.2\"\n---\n\n# KB\n",
			"log.md":   "# Log\n",
		})
		migrateApply(t, dir)
		if got := readRel(t, dir, ".okf"); got != "okf_version: \"0.3\"\ntemplates: ../tpl\n" {
			t.Fatalf("§4 quoting: sidecar = %q", got)
		}
		if got := readRel(t, dir, "index.md"); got != "---\nokf_version: \"0.3\"\n---\n\n# KB\n" {
			t.Fatalf("index = %q", got)
		}
	})
	t.Run("sidecar already at target unquoted is not churned", func(t *testing.T) {
		dir := mkMigrateBundle(t, map[string]string{
			".okf":     "okf_version: 0.3\n",
			"index.md": "---\nokf_version: \"0.3\"\n---\n\n# KB\n",
			"log.md":   "# Log\n",
		})
		before := treeHash(t, dir)
		migrateApply(t, dir)
		if treeHash(t, dir) != before {
			t.Fatalf("an already-0.3 bundle was rewritten")
		}
	})
}

// TestConformance_MigrateUnknownVersion_Section12_NeverDowngradesOrRefuses: a
// bundle declaring a version this build does not know (newer, or not a
// <major>.<minor>) is consumed best-effort (§12): validate does not fail it,
// and migrate plans nothing and writes nothing. In particular it never
// rewrites a newer marker down to 0.3.
func TestConformance_MigrateUnknownVersion_Section12_NeverDowngradesOrRefuses(t *testing.T) {
	for _, v := range []string{"0.4", "9.9", "1.0", "next"} {
		t.Run(v, func(t *testing.T) {
			dir := mkMigrateBundle(t, map[string]string{
				".okf":        "okf_version: " + v + "\n",
				"index.md":    "---\nokf_version: \"" + v + "\"\n---\n\n# KB\n",
				"log.md":      "# Log\n",
				"y.md":        "---\ntype: Concept\ntitle: Y\n---\n\n# Y\n",
				"sub/x.md":    "---\ntype: Concept\ntitle: X\ngenerated: { by: human:a, at: \"2026-06-20 10:00:00Z\" }\n---\n\n[y](y.md)\n",
				"refs/run.py": "x\n",
			})
			before := treeHash(t, dir)
			b := loadMigrate(t, dir)
			if f := Validate(b); len(f) != 0 {
				t.Fatalf("§12: unknown version %q must not fail validate: %+v", v, f)
			}
			plan, err := PlanMigration(b, "")
			if err != nil {
				t.Fatalf("§12: PlanMigration refused %q: %v", v, err)
			}
			if plan.VersionAction != VersionKeep || len(plan.Nodes) != 0 {
				t.Fatalf("§12: want a keep plan with no edits for %q; got %+v", v, plan)
			}
			if err := MigrateApply(dir, b, plan); err != nil {
				t.Fatalf("MigrateApply: %v", err)
			}
			if treeHash(t, dir) != before {
				t.Fatalf("§12: migrate changed a bundle declaring %q", v)
			}
		})
	}
}

// TestConformance_MigrateTimestamps_Section5_NormalizeOnlyUnambiguous: a §5
// timestamp whose instant is fixed by its own text (it carries an offset) but
// whose syntax is not RFC 3339 is rewritten to the RFC 3339 date-time form.
// A date-time WITHOUT an offset names no instant (§5, §13.2), so it is never
// guessed: it becomes a judgment item and its bytes stay.
func TestConformance_MigrateTimestamps_Section5_NormalizeOnlyUnambiguous(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": v02Index,
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Concept\ntitle: A\n" +
			"generated: { by: human:a, at: \"2026-06-20 10:00:00Z\" }\n" +
			"stale_after: \"2026-07-01T09:30+0000\"\n" +
			"verified:\n  - by: human:b\n    at: 2026-06-21 08:00:00 -05:00\n" +
			"  - by: human:c\n    at: \"2026-06-22T08:00:00\"\n" +
			"sources:\n  - resource: https://x.example\n    last_modified: \"2026-06-01\"\n" +
			"    usage_window: { from: \"2026-01-01T00:00:00Z\", to: 2026-02-01 }\n" +
			"modified: 2026-06-20 10:00:00\n" +
			"---\n\n# A\n",
	})
	plan := migrateApply(t, dir)
	got := readRel(t, dir, "a.md")
	for _, want := range []string{
		`at: "2026-06-20T10:00:00Z"`,
		`stale_after: "2026-07-01T09:30:00+00:00"`,
		`at: "2026-06-21T08:00:00-05:00"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("§5 positive control: want %s in:\n%s", want, got)
		}
	}
	// Negative controls: ambiguous, already-valid, and non-§5 keys keep bytes.
	for _, want := range []string{
		`at: "2026-06-22T08:00:00"`,
		`last_modified: "2026-06-01"`,
		`usage_window: { from: "2026-01-01T00:00:00Z", to: 2026-02-01 }`,
		"modified: 2026-06-20 10:00:00\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("§5 negative control: want %s kept in:\n%s", want, got)
		}
	}
	ji := findJudgment(t, plan, "a.md", JudgmentTimestampNoOffset)
	if !strings.Contains(ji.Context, "verified[1].at") || !strings.Contains(ji.Context, "2026-06-22T08:00:00") {
		t.Fatalf("judgment context must name the key and value: %+v", ji)
	}
	if n := len(plan.Judgment); n != 1 {
		t.Fatalf("want exactly 1 judgment item, got %d: %+v", n, plan.Judgment)
	}
}

// TestConformance_ScaffoldDeclaresV03_Section8_1: `bundle init` writes the
// spec-defined declaration, a quoted okf_version "0.3" on the root index.md.
func TestConformance_ScaffoldDeclaresV03_Section8_1(t *testing.T) {
	dir := t.TempDir()
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}
	if got := readRel(t, dir, "index.md"); !strings.HasPrefix(got, "---\nokf_version: \"0.3\"\n---\n\n") {
		t.Fatalf("§8.1: scaffolded index.md = %q", got)
	}
	b := loadMigrate(t, dir)
	if b.DeclaredOkfVersion != "0.3" || b.OkfVersionSource != VersionFromIndex {
		t.Fatalf("§8.1: declared %q from %q", b.DeclaredOkfVersion, b.OkfVersionSource)
	}
	if f := Validate(b); len(f) != 0 {
		t.Fatalf("scaffold must validate clean: %+v", f)
	}
}
