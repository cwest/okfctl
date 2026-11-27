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
	"strings"
	"testing"
)

// Spec-conformance suite for the parts of migrate that guard the Appendix B
// fallbacks (OKF v0.3 §12.1, Appendix B, §5.1): a citation entry with exactly
// one follow-able resource becomes a source; the 0.3 declaration is held back
// while an item still depends on a v0.1 fallback; and re-applying a plan to an
// already-migrated tree changes nothing.

func planFor(t *testing.T, dir, actor string, force bool) (*Bundle, MigratePlan) {
	t.Helper()
	b := loadMigrate(t, dir)
	plan, err := PlanMigrationWith(b, MigrateOptions{GeneratedBy: actor, ForceDeclare: force})
	if err != nil {
		t.Fatalf("PlanMigration: %v", err)
	}
	return b, plan
}

// TestConformance_MigrateCitationEntries_Section5_1_SingleResourceBecomesSource
// is the positive control: an entry that carries exactly one follow-able
// resource, wherever it sits in the entry and however the entry wraps, becomes
// one sources entry with that resource and the entry text as its title.
func TestConformance_MigrateCitationEntries_Section5_1_SingleResourceBecomesSource(t *testing.T) {
	body := "# Def\n\n# Citations\n\n" +
		"Status key: **VERIFIED** = checked against the source.\n" +
		"[1] VERIFIED — Ollama, license **MIT**, active (pushed\n" +
		"2026-06-26). GitHub REST API: <https://api.github.com/repos/ollama/ollama>.\n" +
		"- **[2] [VERIFIED — VENDOR-PRICING]** Anthropic API pricing\n" +
		"  (https://claude.com/pricing) — Opus-class $5/MTok input.\n" +
		"- [3] Paper, arXiv:2310.20410 — https://arxiv.org/abs/2310.20410 — and again\n" +
		"  https://arxiv.org/abs/2310.20410.\n"
	got := citationEntries(body)
	if len(got) != 3 {
		t.Fatalf("want 3 entries (legend excluded, wrapped lines joined), got %d: %q", len(got), got)
	}
	cases := []struct{ res, title string }{
		{"https://api.github.com/repos/ollama/ollama",
			"[1] VERIFIED — Ollama, license **MIT**, active (pushed 2026-06-26). GitHub REST API: <https://api.github.com/repos/ollama/ollama>."},
		{"https://claude.com/pricing",
			"**[2] [VERIFIED — VENDOR-PRICING]** Anthropic API pricing (https://claude.com/pricing) — Opus-class $5/MTok input."},
		{"https://arxiv.org/abs/2310.20410",
			"[3] Paper, arXiv:2310.20410 — https://arxiv.org/abs/2310.20410 — and again https://arxiv.org/abs/2310.20410."},
	}
	for i, c := range cases {
		s, ok := citationSource(got[i])
		if !ok {
			t.Fatalf("§5.1: entry %d has one resource but was not converted: %q", i, got[i])
		}
		if s.Resource != c.res || s.Title != c.title {
			t.Errorf("§5.1: entry %d = %+v\nwant resource %q title %q", i, s, c.res, c.title)
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_AmbiguousStayJudgment is
// the negative control: an entry with two different resources, or with none,
// is left for judgment. A sole link or URL keeps the link text (or no title),
// so a bare-URL entry does not repeat itself as its own title.
func TestConformance_MigrateCitationEntries_Section5_1_AmbiguousStayJudgment(t *testing.T) {
	for _, entry := range []string{
		"[4] VERIFIED — two sources: https://a.example/x and https://b.example/y.",
		"[5] VERIFIED — [doc](https://a.example/x) mirrors https://b.example/x",
		"[6] VERIFIED — on-box probe, no URL.",
		"- Personal observation from a tasting, 2026.",
	} {
		if s, ok := citationSource(entry); ok {
			t.Errorf("§5.1: ambiguous entry %q was converted to %+v", entry, s)
		}
	}
	for entry, want := range map[string]SourceEdit{
		"- https://wiki.acme/x":         {Resource: "https://wiki.acme/x"},
		"- <https://wiki.acme/x>":       {Resource: "https://wiki.acme/x"},
		"- [Other concept](/other.md)":  {Resource: "/other.md", Title: "Other concept"},
		"- [](/other.md)":               {Resource: "/other.md"},
		"1. [Spec](https://s.example/)": {Resource: "https://s.example/", Title: "Spec"},
	} {
		if s, ok := citationSource(entry); !ok || s != want {
			t.Errorf("entry %q = %+v ok=%v, want %+v", entry, s, ok, want)
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_ProseLinkLineIsNotAnEntry:
// a prose paragraph inside `# Citations` whose wrapped line happens to start
// with a markdown link is not a citation entry (analyze.go's citationLinkRe
// draws the same line), so its text never becomes a title. A bulleted link is
// a list item and still is an entry.
func TestConformance_MigrateCitationEntries_Section5_1_ProseLinkLineIsNotAnEntry(t *testing.T) {
	body := "# Citations\n\n" +
		"This memo inherits its facts from the\n" +
		"[landscape node](/infra/landscape.md)\n" +
		"and does not re-verify them.\n\n" +
		"[1] VERIFIED — see [/infra/landscape.md](/infra/landscape.md).\n" +
		"- [Other concept](/other.md)\n"
	got := citationEntries(body)
	want := []string{
		"[1] VERIFIED — see [/infra/landscape.md](/infra/landscape.md).",
		"- [Other concept](/other.md)",
	}
	if len(got) != len(want) {
		t.Fatalf("entries = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// fallbackBundle is undeclared and carries a single-URL citation, a prose
// citation, and (when withTimestamp) a legacy timestamp.
func fallbackBundle(t *testing.T, prose, withTimestamp bool) string {
	fm := "---\ntype: Metric\n"
	if withTimestamp {
		fm += "timestamp: '2026-05-28T22:53:05+00:00'\n"
	}
	cites := "# Citations\n- [1] Handbook — https://wiki.acme/fpa\n"
	if prose {
		cites += "- [2] Personal observation, no public source.\n"
	}
	return mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n",
		"log.md":   "# Log\n",
		"a.md":     fm + "---\n\n# Def\n\n" + cites,
	})
}

// TestConformance_MigrateHoldsDeclaration_Section12_1_WhileFallbackItemsRemain
// is the positive control for the hold: while a prose-citation (Appendix B
// `# Citations`) or missing-actor (Appendix B `timestamp`) item remains,
// declaring 0.3 would switch off the §12.1 fallback that still carries it, so
// the plan holds the declaration and apply leaves the bundle undeclared. The
// deterministic edits still land.
func TestConformance_MigrateHoldsDeclaration_Section12_1_WhileFallbackItemsRemain(t *testing.T) {
	for name, dir := range map[string]string{
		"prose-citation": fallbackBundle(t, true, false),
		"missing-actor":  fallbackBundle(t, false, true),
	} {
		b, plan := planFor(t, dir, "", false)
		if plan.VersionAction != VersionHold {
			t.Fatalf("%s: §12.1: version action = %q, want %q", name, plan.VersionAction, VersionHold)
		}
		if plan.DeclaresTarget() {
			t.Fatalf("%s: a held plan must not declare the target", name)
		}
		if err := MigrateApply(dir, b, plan); err != nil {
			t.Fatalf("%s: MigrateApply: %v", name, err)
		}
		after := loadMigrate(t, dir)
		if after.DeclaredOkfVersion != "" {
			t.Fatalf("%s: §12.1: held bundle declares %q; its fallback is now off", name, after.DeclaredOkfVersion)
		}
		if strings.Contains(readRel(t, dir, "index.md"), "okf_version") {
			t.Fatalf("%s: held bundle's index.md gained okf_version", name)
		}
		if got := after.Nodes["a.md"].Sources(); len(got) != 1 || got[0].Resource != "https://wiki.acme/fpa" {
			t.Fatalf("%s: deterministic source edit did not land: %+v", name, got)
		}
	}
}

// TestConformance_MigrateDeclares_Section8_1_WhenNoFallbackItemRemains is the
// negative control: with every fallback item resolved (no prose citation; an
// actor supplied for the timestamp), the plan bumps and apply declares 0.3.
func TestConformance_MigrateDeclares_Section8_1_WhenNoFallbackItemRemains(t *testing.T) {
	dir := fallbackBundle(t, false, true)
	b, plan := planFor(t, dir, "human:casey", false)
	if plan.VersionAction != VersionBump || !plan.DeclaresTarget() {
		t.Fatalf("§8.1: want bump, got %q (declares=%v); judgment %+v", plan.VersionAction, plan.DeclaresTarget(), plan.Judgment)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	if v := loadMigrate(t, dir).DeclaredOkfVersion; v != "0.3" {
		t.Fatalf("§8.1: declared = %q, want 0.3", v)
	}
}

// TestConformance_MigrateForceDeclare_Section12_1 proves the explicit override:
// ForceDeclare declares 0.3 even with prose-citation items left, and the plan
// records that it did.
func TestConformance_MigrateForceDeclare_Section12_1(t *testing.T) {
	dir := fallbackBundle(t, true, false)
	b, plan := planFor(t, dir, "", true)
	if plan.VersionAction != VersionBump || !plan.ForceDeclare || !plan.DeclaresTarget() {
		t.Fatalf("forced plan: action %q force %v", plan.VersionAction, plan.ForceDeclare)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("MigrateApply: %v", err)
	}
	if v := loadMigrate(t, dir).DeclaredOkfVersion; v != "0.3" {
		t.Fatalf("forced: declared = %q, want 0.3", v)
	}
}

// TestConformance_MigrateApplySamePlanTwice_Section12_Idempotent re-applies one
// plan file to the tree it already migrated. Every splice re-checks the
// current bytes, so the second apply changes nothing: no duplicated sources
// entry, no rewritten generated mapping, no second link rewrite.
func TestConformance_MigrateApplySamePlanTwice_Section12_Idempotent(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n",
		"log.md":   "# Log\n",
		"y.md":     "---\ntype: Concept\n---\n\n# Y\n",
		"sub/a.md": "---\ntype: Metric\ntimestamp: '2026-05-28T22:53:05+00:00'\n" +
			"sources:\n  - resource: https://kept.example/\n---\n\n# Def\n\nSee [y](y.md).\n\n" +
			"# Citations\n- [1] Handbook — https://wiki.acme/fpa\n- [Y](y.md)\n",
	})
	b, plan := planFor(t, dir, "human:casey", false)
	if nm := findNodeMigration(t, plan, "sub/a.md"); len(nm.Sources) != 2 || nm.Generated == nil || len(nm.Paths) == 0 {
		t.Fatalf("precondition: want 2 sources, a rename, and a path edit; got %+v", nm)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	once := treeHash(t, dir)
	if err := MigrateApply(dir, loadMigrate(t, dir), plan); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if treeHash(t, dir) != once {
		t.Fatalf("§12: re-applying the same plan changed the tree:\n%s", readRel(t, dir, "sub/a.md"))
	}
	got := readRel(t, dir, "sub/a.md")
	if n := strings.Count(got, `resource: "https://wiki.acme/fpa"`); n != 1 {
		t.Fatalf("sources entry appears %d times, want 1:\n%s", n, got)
	}
}
