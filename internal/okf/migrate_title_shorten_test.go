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

// v0.3 §5.1: `title` is "a label for humans", the name of the cited work.
// An earlier migration wrote a whole `# Citations` entry into `title`. Once
// that entry's text is kept as the footnote definition of the source's id
// (§4.2; consumers do not parse footnote text), the title is shortened to the
// work's name (citationName), or removed and reported when the entry names
// no work. A title is never shortened unless its text is in the footnote, so
// the entry text is never in zero places. §4.1: every other key of the source
// (`note`, extension keys) keeps its bytes.

const fullTitleNode = "---\ntype: Concept\nsources:\n" +
	"  - resource: https://wiki.acme/fpa\n" +
	"    title: \"[1] VERIFIED — A. Author, *The FP&A Handbook*, Acme Press 2024. https://wiki.acme/fpa\"\n" +
	"    note: hand-written, keep\n" +
	"    x-kb-group: books\n" +
	"  - resource: https://noname.example/\n" +
	"    title: \"[2] VERIFIED — on-box probe of https://noname.example/ returned 200.\"\n" +
	"  - resource: https://named.example/\n" +
	"    title: Already A Name\n" +
	"---\n\n# Def\n\nBudgets follow the handbook [1]; the probe passed [2]; see [3].\n\n# Citations\n\n" +
	"- [1] VERIFIED — A. Author, *The FP&A Handbook*, Acme Press 2024. https://wiki.acme/fpa\n" +
	"- [2] VERIFIED — on-box probe of https://noname.example/ returned 200.\n" +
	"- [3] VERIFIED — Named, *Already A Name*, 2026. https://named.example/\n"

func titleRewrites(nm *NodeMigration) map[string]SourceRetitleEdit {
	out := map[string]SourceRetitleEdit{}
	if nm == nil {
		return out
	}
	for _, t := range nm.Retitles {
		out[t.Resource] = t
	}
	return out
}

// Positive control: a title holding the full entry becomes the work's name,
// and the full entry text is the footnote definition of the source's id.
func TestConformance_MigrateTitleShorten_Section5_1_FullCitationTitleBecomesName(t *testing.T) {
	dir := mkMigrateBundle(t, kb(fullTitleNode))
	b, plan := planFor(t, dir, "", false)
	rw := titleRewrites(findNodeMigration(t, plan, "a.md"))
	got, ok := rw["https://wiki.acme/fpa"]
	if !ok || got.Title != "The FP&A Handbook" || got.Footnote != "1" {
		t.Fatalf("§5.1: want the title shortened to the name with footnote 1, got %+v", rw)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	for _, want := range []string{
		"  - resource: https://wiki.acme/fpa\n    id: \"1\"\n    title: \"The FP&A Handbook\"\n    note: hand-written, keep\n    x-kb-group: books\n",
		"[^1]: VERIFIED — A. Author, *The FP&A Handbook*, Acme Press 2024. https://wiki.acme/fpa\n",
		"Budgets follow the handbook [^1]",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("§5.1/§4.1: missing %q in\n%s", want, out)
		}
	}
	// The text is now only in the footnote: the title holds the name, and
	// the `# Citations` list, fully held by sources and footnotes, is
	// removed (§4.2).
	if strings.Count(out, "Acme Press 2024") != 1 || strings.Contains(out, "# Citations") {
		t.Fatalf("§5.1: the entry text must leave the title, got\n%s", out)
	}
	nb := loadMigrate(t, dir)
	if fs := Validate(nb); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
	if _, again := planFor(t, dir, "", false); len(again.Nodes) != 0 {
		t.Fatalf("§12: re-plan must plan 0 edits, got %+v", again.Nodes)
	}
	once := treeHash(t, dir)
	if err := MigrateApply(dir, nb, plan); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if treeHash(t, dir) != once {
		t.Fatalf("§12: re-applying the same plan changed the tree:\n%s", readRel(t, dir, "a.md"))
	}
}

// Positive control: a full-entry title whose entry names no work is removed
// (title is optional, §5.1) and reported; the text stays in the footnote.
func TestConformance_MigrateTitleShorten_Section5_1_NoNameTitleRemovedAndReported(t *testing.T) {
	dir := mkMigrateBundle(t, kb(fullTitleNode))
	b, plan := planFor(t, dir, "", false)
	rw := titleRewrites(findNodeMigration(t, plan, "a.md"))
	if got, ok := rw["https://noname.example/"]; !ok || got.Title != "" || got.Footnote != "2" {
		t.Fatalf("§5.1: want the title removed (footnote 2), got %+v", rw)
	}
	var reported bool
	for _, j := range titleJudgments(plan, JudgmentSourceTitleUnnamed) {
		if strings.Contains(j.Context, "on-box probe") {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("a removed title must be reported as %s, got %+v", JudgmentSourceTitleUnnamed, plan.Judgment)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	if !strings.Contains(out, "  - resource: https://noname.example/\n    id: \"2\"\n  - resource: https://named.example/\n") {
		t.Fatalf("§5.1: title line not removed\n%s", out)
	}
	if !strings.Contains(out, "[^2]: VERIFIED — on-box probe of https://noname.example/ returned 200.\n") {
		t.Fatalf("§5.1: the entry text must be in the footnote\n%s", out)
	}
}

// Positive control: a whole-entry title whose only quote is an excerpt (a
// repo description) is removed and reported, never cut to the excerpt.
func TestConformance_MigrateTitleShorten_Section5_1_ExcerptTitleRemovedAndReported(t *testing.T) {
	const entry = "[1] VERIFIED — `freqtrade/freqtrade`, license GPL-3.0. Description: \\\"Free, open source crypto trading bot.\\\" Source: https://api.github.com/repos/freqtrade/freqtrade"
	node := "---\ntype: Concept\nsources:\n  - resource: https://api.github.com/repos/freqtrade/freqtrade\n" +
		"    title: \"" + entry + "\"\n---\n\n# Def\n\nA bot [1].\n\n# Citations\n\n- " +
		strings.ReplaceAll(entry, `\"`, `"`) + "\n"
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	rw := titleRewrites(findNodeMigration(t, plan, "a.md"))
	got, ok := rw["https://api.github.com/repos/freqtrade/freqtrade"]
	if !ok || got.Title != "" || got.Footnote != "1" {
		t.Fatalf("§5.1: an excerpt is not a name; want the title removed (footnote 1), got %+v", rw)
	}
	var reported bool
	for _, j := range titleJudgments(plan, JudgmentSourceTitleUnnamed) {
		if strings.Contains(j.Context, "freqtrade") {
			reported = true
		}
	}
	if !reported {
		t.Fatalf("a removed excerpt title must be reported as %s, got %+v", JudgmentSourceTitleUnnamed, plan.Judgment)
	}
}

// Negative control: a title that is already just a name is not touched, even
// though its entry also names that work.
func TestConformance_MigrateTitleShorten_Section5_1_NameTitleUntouched(t *testing.T) {
	dir := mkMigrateBundle(t, kb(fullTitleNode))
	b, plan := planFor(t, dir, "", false)
	if got, ok := titleRewrites(findNodeMigration(t, plan, "a.md"))["https://named.example/"]; ok {
		t.Fatalf("a name title must not be rewritten, planned %+v", got)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out := readRel(t, dir, "a.md"); !strings.Contains(out, "  - resource: https://named.example/\n    id: \"3\"\n    title: Already A Name\n") {
		t.Fatalf("a name title must keep its bytes\n%s", out)
	}
}

// Refusal: the source's footnote already exists with different text, so the
// entry text is not preserved anywhere but the title. The title is kept and
// the node is reported, never shortened.
func TestConformance_MigrateTitleShorten_Section5_1_UnpreservedTitleIsRefused(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - resource: https://wiki.acme/fpa\n" +
		"    id: \"1\"\n" +
		"    title: \"[1] VERIFIED — A. Author, *The FP&A Handbook*, Acme Press 2024. https://wiki.acme/fpa\"\n" +
		"---\n\n# Def\n\nBudgets follow the handbook [^1].\n\n" +
		"[^1]: An author's own note, not the entry.\n\n# Citations\n\n" +
		"- [1] VERIFIED — A. Author, *The FP&A Handbook*, Acme Press 2024. https://wiki.acme/fpa\n"
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	if rw := titleRewrites(planNodeMigration(plan, "a.md")); len(rw) != 0 {
		t.Fatalf("a title not preserved in its footnote must not be shortened, planned %+v", rw)
	}
	if got := titleJudgments(plan, JudgmentSourceTitleUnpreserved); len(got) != 1 || !strings.Contains(got[0].Context, "FP&A Handbook") {
		t.Fatalf("want one %s item, got %+v", JudgmentSourceTitleUnpreserved, plan.Judgment)
	}
	if plan.FallbackItems() != 0 {
		t.Fatalf("the resource is in frontmatter: FallbackItems = %d, want 0", plan.FallbackItems())
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out := readRel(t, dir, "a.md"); out != node {
		t.Fatalf("a refused node must keep its bytes\n%s", out)
	}
}

// Refusal at apply: a stale or hand-edited plan that shortens a title whose
// text is in no footnote changes nothing. The renderer checks the footnote in
// the bytes it writes, not the plan's word for it.
func TestConformance_MigrateTitleShorten_Section5_1_ApplyRefusesWithoutFootnote(t *testing.T) {
	src := "---\ntype: Concept\nsources:\n  - resource: https://wiki.acme/fpa\n" +
		"    title: \"VERIFIED — A. Author, *The FP&A Handbook*, 2024.\"\n---\n\n# Def\n"
	edit := SourceRetitleEdit{Resource: "https://wiki.acme/fpa", Old: "VERIFIED — A. Author, *The FP&A Handbook*, 2024.", Title: "The FP&A Handbook", Footnote: "1"}
	out, err := applyTitleRewrites([]byte(src), []SourceRetitleEdit{edit})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(out) != src {
		t.Fatalf("a title with no footnote holding it was rewritten:\n%s", out)
	}
	// With the footnote present, the same edit applies (positive control).
	withNote := src + "\n[^1]: VERIFIED — A. Author, *The FP&A Handbook*, 2024.\n"
	out, err = applyTitleRewrites([]byte(withNote), []SourceRetitleEdit{edit})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(string(out), "    title: \"The FP&A Handbook\"\n") {
		t.Fatalf("want the title shortened once the footnote holds it:\n%s", out)
	}
	// A title the author changed since the plan is not overwritten.
	edited := strings.Replace(withNote, "VERIFIED — A. Author, *The FP&A Handbook*, 2024.\"", "Mine\"", 1)
	if out, _ = applyTitleRewrites([]byte(edited), []SourceRetitleEdit{edit}); string(out) != edited {
		t.Fatalf("a title changed since the plan was overwritten:\n%s", out)
	}
	// Even when the author's new title is text the footnote also holds, the
	// plan's Old no longer matches, so it is not overwritten.
	edited = strings.Replace(withNote, "VERIFIED — A. Author, *The FP&A Handbook*, 2024.\"", "A. Author\"", 1)
	if out, _ = applyTitleRewrites([]byte(edited), []SourceRetitleEdit{edit}); string(out) != edited {
		t.Fatalf("a title changed since the plan was overwritten:\n%s", out)
	}
	// The same holds for a removal: a changed title is never deleted.
	remove := edit
	remove.Title = ""
	if out, _ = applyTitleRewrites([]byte(edited), []SourceRetitleEdit{remove}); string(out) != edited {
		t.Fatalf("a title changed since the plan was removed:\n%s", out)
	}
}

// §12 idempotence: when several entries cite one source, the retitle decides
// the title a re-plan would settle on. The first entry's full-text title names
// no work, a later entry names it, so the title becomes that name in one pass
// (not removed now and filled on the next run), and a re-plan plans nothing
// and reports the same items.
func TestConformance_MigrateTitleShorten_Section12_SharedSourceRetitleIsFixedPoint(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - resource: https://dpa.example/\n" +
		"    title: \"[1] VERIFIED — DPA §10: data may be processed in any country.\"\n" +
		"  - resource: https://probe.example/\n" +
		"    title: \"[3] VERIFIED — probe of https://probe.example/ returned 200.\"\n" +
		"---\n\n# Def\n\nA [1], B [2], C [3], D [4].\n\n# Citations\n\n" +
		"- [1] VERIFIED — DPA §10: data may be processed in any country. https://dpa.example/\n" +
		"- [2] VERIFIED — Google Cloud, *Cloud Data Processing Addendum*, 2026. https://dpa.example/\n" +
		"- [3] VERIFIED — probe of https://probe.example/ returned 200.\n" +
		"- [4] VERIFIED — second probe of https://probe.example/ returned 200.\n"
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	rw := titleRewrites(findNodeMigration(t, plan, "a.md"))
	if got := rw["https://dpa.example/"]; got.Title != "Cloud Data Processing Addendum" {
		t.Fatalf("§5.1: shared source should take the name a later entry gives, got %+v", rw)
	}
	if got, ok := rw["https://probe.example/"]; !ok || got.Title != "" {
		t.Fatalf("§5.1: unnamed shared source title should be removed, got %+v", rw)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	_, again := planFor(t, dir, "", false)
	if len(again.Nodes) != 0 {
		t.Fatalf("§12: re-plan must plan 0 edits, got %+v", again.Nodes)
	}
	count := func(p MigratePlan) map[JudgmentKind]int {
		out := map[JudgmentKind]int{}
		for _, j := range p.Judgment {
			out[j.Kind]++
		}
		return out
	}
	if a, z := count(plan), count(again); len(a) != len(z) || a[JudgmentSourceTitleUnnamed] != z[JudgmentSourceTitleUnnamed] ||
		a[JudgmentSourceTitleConflict] != z[JudgmentSourceTitleConflict] {
		t.Fatalf("§12: plan and re-plan must report the same items: %v vs %v", a, z)
	}
}
