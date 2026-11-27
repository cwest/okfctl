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
// Some titles were annotated by hand: they open with an entry's `[key]` but
// say more than the entry does ("[S8] iCIMS, AI recruiting software page
// (fetched 2026-11-27). VENDOR-DOC. VERIFIED." against the entry "[S8] iCIMS
// AI recruiting. https://... VERIFIED."). That text is in no other place, so
// it is appended to the footnote definition of the source's id (consumers do
// not parse footnote text, §5.1) before the title is shortened to the name or
// removed and reported. §4.1: the source's other keys keep their bytes.

const annotatedTitleNode = "---\ntype: Concept\nsources:\n" +
	"  - resource: https://www.icims.com/products/ai-recruiting-software/\n" +
	"    title: \"[S8] iCIMS, AI recruiting software page (fetched 2026-11-27). VENDOR-DOC. VERIFIED.\"\n" +
	"    note: hand-written, keep\n" +
	"  - resource: https://wiki.acme/fpa\n" +
	"    title: \"[S2] Acme, *The FP&A Handbook* landing page (fetched 2026-11-27). VERIFIED.\"\n" +
	"  - resource: https://routing.example/\n" +
	"    title: \"[Beta] Auto Routing\"\n" +
	"---\n\n# Def\n\nScreening [S8], budgets [S2], routing [S9].\n\n# Citations\n\n" +
	"- [S8] iCIMS AI recruiting. https://www.icims.com/products/ai-recruiting-software/. VERIFIED.\n" +
	"- [S2] Acme, *The FP&A Handbook*, 2024. https://wiki.acme/fpa\n" +
	"- [S9] Routing docs. https://routing.example/\n"

// Positive control: an annotated title whose entry names no work is removed
// and reported, and its text is in the footnote, so nothing is lost.
func TestConformance_MigrateTitleAnnotation_Section5_1_AnnotationMovesToFootnoteThenRemoved(t *testing.T) {
	dir := mkMigrateBundle(t, kb(annotatedTitleNode))
	b, plan := planFor(t, dir, "", false)
	rw := titleRewrites(findNodeMigration(t, plan, "a.md"))
	got, ok := rw["https://www.icims.com/products/ai-recruiting-software/"]
	if !ok || got.Title != "" || got.Footnote != "S8" || !got.Moved {
		t.Fatalf("§5.1: want the annotated title moved to [^S8] and removed, got %+v", rw)
	}
	var reported bool
	for _, j := range titleJudgments(plan, JudgmentSourceTitleUnnamed) {
		if strings.Contains(j.Context, "iCIMS") {
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
	for _, want := range []string{
		"  - resource: https://www.icims.com/products/ai-recruiting-software/\n    id: S8\n    note: hand-written, keep\n",
		"[^S8]: iCIMS AI recruiting. https://www.icims.com/products/ai-recruiting-software/. VERIFIED. " +
			"iCIMS, AI recruiting software page (fetched 2026-11-27). VENDOR-DOC. VERIFIED.\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("§5.1/§4.1: missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "title: \"[S8]") {
		t.Fatalf("§5.1: the annotation must leave the title\n%s", out)
	}
}

// Positive control: an annotated title whose entry names the work becomes
// that name, after its text is appended to the footnote.
func TestConformance_MigrateTitleAnnotation_Section5_1_AnnotationMovesToFootnoteThenNamed(t *testing.T) {
	dir := mkMigrateBundle(t, kb(annotatedTitleNode))
	b, plan := planFor(t, dir, "", false)
	got, ok := titleRewrites(findNodeMigration(t, plan, "a.md"))["https://wiki.acme/fpa"]
	if !ok || got.Title != "The FP&A Handbook" || got.Footnote != "S2" || !got.Moved {
		t.Fatalf("§5.1: want the annotated title shortened to the name, got %+v", got)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	for _, want := range []string{
		"    id: S2\n    title: \"The FP&A Handbook\"\n",
		"[^S2]: Acme, *The FP&A Handbook*, 2024. https://wiki.acme/fpa Acme, *The FP&A Handbook* landing page (fetched 2026-11-27). VERIFIED.\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("§5.1: missing %q in\n%s", want, out)
		}
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

// Negative control: a title that opens with a bracket that is no entry's key
// ("[Beta] Auto Routing") is a name, and keeps its bytes.
func TestConformance_MigrateTitleAnnotation_Section5_1_BracketedNameUntouched(t *testing.T) {
	dir := mkMigrateBundle(t, kb(annotatedTitleNode))
	b, plan := planFor(t, dir, "", false)
	if got, ok := titleRewrites(findNodeMigration(t, plan, "a.md"))["https://routing.example/"]; ok {
		t.Fatalf("a bracketed name must not be rewritten, planned %+v", got)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out := readRel(t, dir, "a.md"); !strings.Contains(out, "    title: \"[Beta] Auto Routing\"\n") {
		t.Fatalf("a bracketed name must keep its bytes\n%s", out)
	}
}

// Negative control: a title that is its entry's text is shortened as before,
// with nothing appended to the footnote.
func TestConformance_MigrateTitleAnnotation_Section5_1_EntryTextTitleNotAppended(t *testing.T) {
	dir := mkMigrateBundle(t, kb(fullTitleNode))
	b, plan := planFor(t, dir, "", false)
	got := titleRewrites(findNodeMigration(t, plan, "a.md"))["https://wiki.acme/fpa"]
	if got.Moved {
		t.Fatalf("an entry-text title must not be marked moved: %+v", got)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out := readRel(t, dir, "a.md"); !strings.Contains(out, "[^1]: VERIFIED — A. Author, *The FP&A Handbook*, Acme Press 2024. https://wiki.acme/fpa\n") {
		t.Fatalf("the footnote must hold the entry text alone\n%s", out)
	}
}

// Refusal: the author already defined [^S8] with other text, so there is no
// planned footnote to append to. The annotated title is kept and reported,
// and the file keeps its bytes.
func TestConformance_MigrateTitleAnnotation_Section5_1_ExistingFootnoteRefuses(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - resource: https://www.icims.com/products/ai-recruiting-software/\n" +
		"    id: S8\n" +
		"    title: \"[S8] iCIMS, AI recruiting software page (fetched 2026-11-27). VENDOR-DOC. VERIFIED.\"\n" +
		"---\n\n# Def\n\nScreening [^S8].\n\n[^S8]: The author's own note.\n\n# Citations\n\n" +
		"- [S8] iCIMS AI recruiting. https://www.icims.com/products/ai-recruiting-software/. VERIFIED.\n"
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	if rw := titleRewrites(planNodeMigration(plan, "a.md")); len(rw) != 0 {
		t.Fatalf("an annotation with nowhere to go must not be shortened, planned %+v", rw)
	}
	if got := titleJudgments(plan, JudgmentSourceTitleUnpreserved); len(got) != 1 || !strings.Contains(got[0].Context, "iCIMS") {
		t.Fatalf("want one %s item, got %+v", JudgmentSourceTitleUnpreserved, plan.Judgment)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out := readRel(t, dir, "a.md"); out != node {
		t.Fatalf("a refused node must keep its bytes\n%s", out)
	}
}
