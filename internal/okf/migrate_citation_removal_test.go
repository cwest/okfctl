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

// v0.3 §4.2: "Do not use a list of citations in the body." The v0.1 body
// `# Citations` list (Appendix B) is removed once the migration has moved
// everything it holds into frontmatter `sources` (§5.1) and footnote
// definitions keyed by `sources[].id`. The list is the last copy of each
// entry's full text, so it is removed only when that is provable from the
// migrated bytes: every entry's text is a footnote definition whose label is a
// source id, every resource it names is a source's `resource`, no line of the
// section is anything but an entry, and no body `[k]` still points at it.
// Otherwise the list stays and a citations-kept item names what would be lost.

// withoutCitations is s with its `# Citations` section cut out: the bytes a
// removal leaves outside the section.
func withoutCitations(s string) string {
	_, body, _ := ParseFrontmatter([]byte(s))
	a, z, ok := citationSectionSpan(body)
	if !ok {
		return s
	}
	off := len(s) - len(body)
	return s[:off+a] + s[off+z:]
}

const removableNode = "---\ntype: Concept\n---\n\n# Def\n\n" +
	"Daily shards [ga4]. Priced per token [2]. Mirrored [12].\n\n" +
	"# Citations\n\n" +
	"- [ga4] VERIFIED — GA4 BigQuery Export schema,\n" +
	"  https://developers.google.com/analytics/bigquery/export-schema\n" +
	"- [2] VERIFIED — pricing page https://claude.com/pricing\n" +
	"- [12] VERIFIED — *Export Schema*, https://a.example/schema; mirror\n" +
	"  https://b.example/mirror (retrieved 2026-06-01)\n"

// Positive control: a node whose every entry converts (single-resource and
// split) loses its `# Citations` list in the same apply, and keeps every
// entry's full text as a footnote definition keyed by a source id (§5.1).
func TestConformance_MigrateRemoveCitations_Section4_2_FullyConvertedNodeLosesList(t *testing.T) {
	dir, plan := migrateOnce(t, kb(removableNode))
	nm := findNodeMigration(t, plan, "a.md")
	if !nm.RemoveCitations {
		t.Fatalf("§4.2: a fully recoverable list must be planned for removal: %+v / judgment %+v", nm, plan.Judgment)
	}
	if hasJudgment(plan, "a.md", JudgmentCitationsKept) {
		t.Fatalf("§4.2: a removable list must not be reported kept: %+v", plan.Judgment)
	}
	out := readRel(t, dir, "a.md")
	if strings.Contains(out, "# Citations") {
		t.Fatalf("§4.2: the body list must be gone:\n%s", out)
	}
	for _, want := range []string{
		"Daily shards [^ga4]. Priced per token [^2]. Mirrored [^12][^12-2].\n",
		"[^ga4]: VERIFIED — GA4 BigQuery Export schema, https://developers.google.com/analytics/bigquery/export-schema\n",
		"[^2]: VERIFIED — pricing page https://claude.com/pricing\n",
		"[^12]: VERIFIED — *Export Schema*, https://a.example/schema; mirror https://b.example/mirror (retrieved 2026-06-01)\n",
		"[^12-2]: https://b.example/mirror\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("§5.1: missing %q in\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "[^12-2]: https://b.example/mirror\n") {
		t.Errorf("the file must end at the last definition with one newline:\n%q", out)
	}
	if fs := Validate(loadMigrate(t, dir)); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
}

// §12: once the list is gone, re-planning plans nothing and re-applying the
// same plan leaves the tree byte-identical.
func TestConformance_MigrateRemoveCitations_Section12_RerunIsNoop(t *testing.T) {
	dir, plan := migrateOnce(t, kb(removableNode))
	nb := loadMigrate(t, dir)
	_, again := planFor(t, dir, "", false)
	if len(again.Nodes) != 0 {
		t.Fatalf("§12: re-plan must plan 0 edits, got %+v", again.Nodes)
	}
	if len(again.Judgment) != 0 {
		t.Fatalf("§12: re-plan must report nothing, got %+v", again.Judgment)
	}
	once := treeHash(t, dir)
	if err := MigrateApply(dir, nb, plan); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if treeHash(t, dir) != once {
		t.Fatalf("§12: re-applying the plan changed the tree:\n%s", readRel(t, dir, "a.md"))
	}
}

// A node an earlier okfctl already converted (sources with ids, footnote
// definitions, footnote references) but whose list is still there: the only
// planned edit is the removal, and the body outside the section is kept byte
// for byte, including a heading that follows the list.
const convertedKeptNode = "---\ntype: Concept\nsources:\n" +
	"  - id: \"1\"\n    resource: \"https://a.example/\"\n    title: Doc A\n" +
	"---\n\n# Def\n\nA claim [^1].\n\n" +
	"[^1]: VERIFIED — *Doc A*, https://a.example/ (read 2026-06-01)\n\n" +
	"# Citations\n\n" +
	"- [1] VERIFIED — *Doc A*, https://a.example/ (read 2026-06-01)\n\n" +
	"# See also\n\nMore prose.\n"

func TestConformance_MigrateRemoveCitations_Section4_2_AlreadyConvertedListIsRemovedAlone(t *testing.T) {
	dir := mkMigrateBundle(t, kb(convertedKeptNode))
	b, plan := planFor(t, dir, "", false)
	nm := findNodeMigration(t, plan, "a.md")
	if !nm.RemoveCitations || nm.EditCount() != 1 {
		t.Fatalf("§4.2: want the removal as the only edit, got %+v", nm)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := strings.Replace(convertedKeptNode,
		"# Citations\n\n- [1] VERIFIED — *Doc A*, https://a.example/ (read 2026-06-01)\n\n", "", 1)
	if got := readRel(t, dir, "a.md"); got != want {
		t.Fatalf("only the section may change:\n got %q\nwant %q", got, want)
	}
}

// Negative control: an entry that names a URL no source holds keeps the list,
// and the report names the entry and the URL (§5.1: `resource` is the source's
// one follow-able pointer; losing it loses the citation).
func TestConformance_MigrateRemoveCitations_Section5_1_URLMissingFromSourcesKeepsList(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - id: \"1\"\n    resource: \"https://a.example/\"\n" +
		"---\n\n# Def\n\nA claim [^1].\n\n" +
		"[^1]: VERIFIED — Doc https://a.example/ and https://lost.example/x\n\n" +
		"# Citations\n\n" +
		"- [1] VERIFIED — Doc https://a.example/ and https://lost.example/x\n"
	dir := mkMigrateBundle(t, kb(node))
	b := loadMigrate(t, dir)
	lost, has := citationListLoss(b, "a.md", []byte(node))
	if !has {
		t.Fatal("the node has a # Citations list")
	}
	if len(lost) != 1 || !strings.Contains(lost[0], "[1]") || !strings.Contains(lost[0], "https://lost.example/x") {
		t.Fatalf("§5.1: report must name entry [1] and the URL, got %q", lost)
	}
	if strings.Contains(strings.Join(lost, " "), "https://a.example/ is") {
		t.Errorf("a URL that is a source's resource must not be reported: %q", lost)
	}
	// Positive half of the same control: with the URL's source present the
	// identical list is removable.
	fixed := strings.Replace(node, "---\n\n# Def", "  - id: 1-2\n    resource: \"https://lost.example/x\"\n---\n\n# Def", 1)
	if lost, _ := citationListLoss(b, "a.md", []byte(fixed)); len(lost) != 0 {
		t.Fatalf("with every URL a source the list is recoverable, got %q", lost)
	}
}

// Negative control: a footnote definition that differs from its entry by one
// word does not hold the entry's text, so the list stays after apply and the
// report names the entry and the footnote.
func TestConformance_MigrateRemoveCitations_Section5_1_FootnoteDiffersByOneWordKeepsList(t *testing.T) {
	node := strings.Replace(convertedKeptNode, "[^1]: VERIFIED", "[^1]: NUANCED", 1)
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	if nm := planNodeMigration(plan, "a.md"); nm != nil && nm.RemoveCitations {
		t.Fatalf("§5.1: a one-word difference must keep the list: %+v", nm)
	}
	j := findJudgment(t, plan, "a.md", JudgmentCitationsKept)
	if !strings.Contains(j.Context, "[1]") || !strings.Contains(j.Context, "[^1]") {
		t.Fatalf("report must name entry [1] and footnote [^1], got %q", j.Context)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := readRel(t, dir, "a.md"); got != node {
		t.Fatalf("the node must be left untouched:\n%s", got)
	}
}

// Negative control: a status-key paragraph inside the section is not a
// citation entry and lives nowhere else, so the list stays.
func TestConformance_MigrateRemoveCitations_Section4_2_NonEntryLineKeepsList(t *testing.T) {
	node := strings.Replace(removableNode, "# Citations\n\n",
		"# Citations\n\nStatus key: VERIFIED = checked against the primary source.\n\n", 1)
	dir, plan := migrateOnce(t, kb(node))
	j := findJudgment(t, plan, "a.md", JudgmentCitationsKept)
	if !strings.Contains(j.Context, "Status key: VERIFIED") {
		t.Fatalf("report must quote the line that would be lost, got %q", j.Context)
	}
	if !strings.Contains(readRel(t, dir, "a.md"), "# Citations\n\nStatus key") {
		t.Fatalf("the list must stay:\n%s", readRel(t, dir, "a.md"))
	}
}

// Negative control: an entry with no follow-able resource has no source and
// no footnote (§5.1 requires `resource`), so the list stays and the report
// names that entry; the other entries still convert.
func TestConformance_MigrateRemoveCitations_Section5_1_ProseEntryKeepsList(t *testing.T) {
	node := removableNode + "- [3] Internal probe on the authoring host, 2026-07-20\n"
	node = strings.Replace(node, "Mirrored [12].", "Mirrored [12]. Probed [3].", 1)
	dir, plan := migrateOnce(t, kb(node))
	j := findJudgment(t, plan, "a.md", JudgmentCitationsKept)
	if !strings.Contains(j.Context, "[3]") {
		t.Fatalf("report must name entry [3], got %q", j.Context)
	}
	if strings.Contains(j.Context, "[ga4]") {
		t.Errorf("a recoverable entry must not be reported: %q", j.Context)
	}
	if !strings.Contains(readRel(t, dir, "a.md"), "# Citations") {
		t.Fatalf("the list must stay")
	}
}

// Negative control: a body `[k]` that still cites the list (left for judgment
// as citation-ref-unclear) would point at nothing once the list is gone, so
// the list stays.
func TestConformance_MigrateRemoveCitations_Section5_1_UnrewrittenBodyRefKeepsList(t *testing.T) {
	node := strings.Replace(removableNode, "Priced per token [2].",
		"Priced per token, see the brief citation [2].", 1)
	dir, plan := migrateOnce(t, kb(node))
	if !hasJudgment(plan, "a.md", JudgmentCitationRef) {
		t.Fatalf("setup: want a citation-ref-unclear item: %+v", plan.Judgment)
	}
	j := findJudgment(t, plan, "a.md", JudgmentCitationsKept)
	if !strings.Contains(j.Context, "[2]") {
		t.Fatalf("report must name the body reference [2], got %q", j.Context)
	}
	if !strings.Contains(readRel(t, dir, "a.md"), "# Citations") {
		t.Fatalf("the list must stay")
	}
}

// Negative control: a footnote that runs the entry's last word on into more
// text ("2026-06-01)X") does not hold the entry's text, so the list stays.
// Only the entry text followed by a space and more text counts as held.
func TestConformance_MigrateRemoveCitations_Section5_1_FootnoteRunOnKeepsList(t *testing.T) {
	node := strings.Replace(convertedKeptNode, "(read 2026-06-01)\n\n# Citations", "(read 2026-06-01)X\n\n# Citations", 1)
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	if nm := planNodeMigration(plan, "a.md"); nm != nil && nm.RemoveCitations {
		t.Fatalf("§5.1: a run-on footnote must keep the list: %+v", nm)
	}
	findJudgment(t, plan, "a.md", JudgmentCitationsKept)
}

// Positive control: a footnote holding the entry's text and then more (an
// annotation an earlier run moved there) holds the entry; the list goes.
func TestConformance_MigrateRemoveCitations_Section5_1_FootnoteWithAppendedTextRemovesList(t *testing.T) {
	node := strings.Replace(convertedKeptNode, "(read 2026-06-01)\n\n# Citations", "(read 2026-06-01) Doc A landing page. VENDOR-DOC.\n\n# Citations", 1)
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	if nm := planNodeMigration(plan, "a.md"); nm == nil || !nm.RemoveCitations {
		t.Fatalf("§4.2: entry text plus appended text must allow removal: %+v / %+v", nm, plan.Judgment)
	}
}

// Regression: the entry's text is held by the footnote of its second part, but
// its first resource is no source's resource. Removing the list would lose
// that resource, so the report names it (§5.1).
func TestConformance_MigrateRemoveCitations_Section5_1_HeldByLaterPartStillChecksFirst(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - id: \"1\"\n    resource: \"https://b.example/\"\n" +
		"---\n\n# Def\n\nA claim [^1].\n\n" +
		"[^1]: VERIFIED — https://a.example/ and https://b.example/\n\n" +
		"# Citations\n\n" +
		"- [1] VERIFIED — https://a.example/ and https://b.example/\n"
	dir := mkMigrateBundle(t, kb(node))
	b := loadMigrate(t, dir)
	lost, has := citationListLoss(b, "a.md", []byte(node))
	if !has {
		t.Fatal("the node has a # Citations list")
	}
	t.Logf("lost=%q", lost)
	if len(lost) != 1 || !strings.Contains(lost[0], "https://a.example/ is not a source") {
		t.Fatalf("§5.1: report must name the missing first resource, got %q", lost)
	}
}
