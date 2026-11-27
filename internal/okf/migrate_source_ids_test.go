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

// v0.3 §5.1: a source's `id` "SHOULD be present when the body cites the
// source", and per-claim attribution is a footnote whose label is a
// `sources[].id` (§4.2). GFM matches footnote labels case-insensitively, so
// ids are unique after case folding. §4: an id that looks like a number is
// quoted. The full entry text moves into the footnote definition, which
// consumers do not parse (§5.1).

func idsByResource(nm *NodeMigration) map[string]string {
	out := map[string]string{}
	for _, s := range nm.Sources {
		if s.ID != "" {
			out[s.Resource] = s.ID
		}
	}
	for _, e := range nm.IDs {
		out[e.Resource] = e.ID
	}
	return out
}

func migrateOnce(t *testing.T, files map[string]string) (string, MigratePlan) {
	t.Helper()
	dir := mkMigrateBundle(t, files)
	b, plan := planFor(t, dir, "", false)
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return dir, plan
}

func kb(node string) map[string]string {
	return map[string]string{"index.md": "# KB\n", "log.md": "# Log\n", "a.md": node}
}

const slugNode = "---\ntype: Concept\n---\n\n# Def\n\n" +
	"The schema is sharded daily [ga4-schema], and pricing is per token [2].\n\n" +
	"# Citations\n\n" +
	"- [ga4-schema] VERIFIED — GA4 BigQuery Export schema,\n" +
	"  https://developers.google.com/analytics/bigquery/export-schema\n" +
	"- [2] VERIFIED — pricing page https://claude.com/pricing\n"

// Positive control, rule R2 step 1: a plain-slug key that is unique after case
// folding becomes the id as written; a numeric one is quoted (§4).
func TestConformance_MigrateSourceIDs_Section5_1_SlugKeyKeptAsID(t *testing.T) {
	dir := mkMigrateBundle(t, kb(slugNode))
	_, plan := planFor(t, dir, "", false)
	got := idsByResource(findNodeMigration(t, plan, "a.md"))
	want := map[string]string{
		"https://developers.google.com/analytics/bigquery/export-schema": "ga4-schema",
		"https://claude.com/pricing":                                     "2",
	}
	for r, id := range want {
		if got[r] != id {
			t.Errorf("§5.1: id for %s = %q, want %q (all: %v)", r, got[r], id, got)
		}
	}
	b := loadMigrate(t, dir)
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	for _, line := range []string{"id: ga4-schema\n", "id: \"2\"\n"} {
		if !strings.Contains(out, line) {
			t.Errorf("§4/§5.1: missing frontmatter line %q in\n%s", line, out)
		}
	}
	if fs := Validate(loadMigrate(t, dir)); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
}

// Rule R1 + R4: each entry's full text becomes the footnote definition of its
// source (list marker removed, wrapped lines joined, the leading key dropped
// because it became the id), and each body reference becomes a footnote.
func TestConformance_MigrateSourceIDs_Section4_2_FootnoteDefinitionsAndReferences(t *testing.T) {
	dir, _ := migrateOnce(t, kb(slugNode))
	out := readRel(t, dir, "a.md")
	wantBody := "The schema is sharded daily [^ga4-schema], and pricing is per token [^2].\n\n" +
		"[^ga4-schema]: VERIFIED — GA4 BigQuery Export schema, https://developers.google.com/analytics/bigquery/export-schema\n" +
		"[^2]: VERIFIED — pricing page https://claude.com/pricing\n\n" +
		"# Citations\n\n" +
		"- [ga4-schema] VERIFIED — GA4 BigQuery Export schema,\n"
	if !strings.Contains(out, wantBody) {
		t.Fatalf("§4.2/§5.1: body not rewritten as expected\n--- got\n%s\n--- want substring\n%s", out, wantBody)
	}
	// The body list is untouched (its removal is a separate step), so the
	// parser still sees exactly the same entries.
	if got := citationEntries(loadMigrate(t, dir).Nodes["a.md"].Body); len(got) != 2 {
		t.Fatalf("footnote definitions must sit outside # Citations; entries now %q", got)
	}
}

// A grade bracket in the entry stays in the definition; a key that did not
// become the id (an ordered entry keyed by a grade) is kept too. A bold key
// is not a leading `[k]` and is kept verbatim.
func TestConformance_MigrateSourceIDs_Section5_1_DefinitionKeepsGradeBrackets(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nClaim one [1]. Claim two [2]. Claim three [3].\n\n# Citations\n\n" +
		"1. [PRIMARY-DOC, VERIFIED] Spec text https://a.example/\n" +
		"2. [PRIMARY-DOC, VERIFIED] Other https://b.example/\n" +
		"- **[3]** VERIFIED — bold key https://c.example/\n"
	dir, plan := migrateOnce(t, kb(node))
	got := idsByResource(findNodeMigration(t, plan, "a.md"))
	if got["https://a.example/"] != "1" || got["https://b.example/"] != "2" || got["https://c.example/"] != "3" {
		t.Fatalf("R2 step 2: ordered entries keyed by a grade take their list number; got %v", got)
	}
	out := readRel(t, dir, "a.md")
	for _, def := range []string{
		"[^1]: [PRIMARY-DOC, VERIFIED] Spec text https://a.example/\n",
		"[^2]: [PRIMARY-DOC, VERIFIED] Other https://b.example/\n",
		"[^3]: **[3]** VERIFIED — bold key https://c.example/\n",
		"Claim one [^1]. Claim two [^2]. Claim three [^3].",
	} {
		if !strings.Contains(out, def) {
			t.Errorf("missing %q in\n%s", def, out)
		}
	}
}

// Negative control, rule R2: GFM folds footnote label case, so `[S1]` and a
// generated `s1` would be one footnote. Generated ids are `cite-<n>`, never
// `s<n>`, and every assigned id is unique after case folding.
func TestConformance_MigrateSourceIDs_Section5_1_GeneratedIDAvoidsS1Collision(t *testing.T) {
	entries := []string{
		"- **[VERIFIED]** first https://x.example/",
		"- **[VERIFIED]** second https://y.example/",
		"- [S1] third https://z.example/",
	}
	ids := CitationEntryIDs(entries)
	want := []string{"cite-1", "cite-2", "S1"}
	seen := map[string]bool{}
	for i, w := range want {
		if ids[i].ID != w {
			t.Errorf("R2: entry %d id = %q, want %q", i+1, ids[i].ID, w)
		}
		f := strings.ToLower(ids[i].ID)
		if seen[f] {
			t.Errorf("R2: id %q collides case-insensitively", ids[i].ID)
		}
		seen[f] = true
	}
	if seen["s1"] && ids[0].ID == "s1" {
		t.Fatalf("R2: generated s<n> collides with [S1]")
	}
	// Keys that repeat after case folding are not usable: `[S1]` and `[s1]`.
	ids = CitationEntryIDs([]string{"- [S1] a https://a.example/", "- [s1] b https://b.example/"})
	if ids[0].ID != "cite-1" || ids[1].ID != "cite-2" {
		t.Fatalf("R2: case-folded duplicate keys must not be ids; got %+v", ids)
	}
}

// The shared uniqueness check refuses a case-folded duplicate, so the split
// path (`12`, `12-2`) and this path cannot hand out one label twice.
func TestConformance_MigrateSourceIDs_Section5_1_SourceIDSetFoldsCase(t *testing.T) {
	s := NewSourceIDSet()
	for _, id := range []string{"12", "12-2", "S1"} {
		if err := s.Claim(id); err != nil {
			t.Fatalf("Claim(%q): %v", id, err)
		}
	}
	for _, id := range []string{"s1", "12-2", "12"} {
		if err := s.Claim(id); err == nil {
			t.Errorf("Claim(%q) after a case-folded duplicate must fail", id)
		}
	}
	if err := s.Claim("has space"); err == nil {
		t.Errorf("an id with whitespace cannot be a footnote label and must be refused")
	}
}

// A node whose ids collide is refused, not guessed: an existing source already
// holds id `s1` while the body list keys an entry `[S1]`.
func TestConformance_MigrateSourceIDs_Section5_1_CollisionRefusesNode(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n  - id: s1\n    resource: https://other.example/\n---\n\n# Def\n\nA claim [S1].\n\n# Citations\n\n" +
		"- [S1] VERIFIED — thing https://z.example/\n"
	dir := mkMigrateBundle(t, kb(node))
	_, plan := planFor(t, dir, "", false)
	if !hasJudgment(plan, "a.md", JudgmentSourceIDCollision) {
		t.Fatalf("R2: a case-folded id collision must be a judgment item; got %+v", plan.Judgment)
	}
	nm := planNodeMigration(plan, "a.md")
	if nm != nil && (len(nm.IDs) > 0 || len(nm.Footnotes) > 0 || len(nm.Refs) > 0 || idsByResource(nm)["https://z.example/"] != "") {
		t.Fatalf("R2: a refused node gets no ids, footnotes or reference rewrites; got %+v", nm)
	}
}

// Rule R4 grouped forms: `[3]–[5]` and `[1, 2]` expand to one footnote per key.
func TestConformance_MigrateSourceIDs_Section4_2_GroupedReferencesExpand(t *testing.T) {
	var cites strings.Builder
	for i := 1; i <= 5; i++ {
		cites.WriteString("[" + string(rune('0'+i)) + "] VERIFIED — src https://s" + string(rune('0'+i)) + ".example/\n")
	}
	node := "---\ntype: Concept\n---\n\n# Def\n\nRange claim [3]–[5]. List claim [1, 2]. Pair claim [1][2].\n\n# Citations\n\n" + cites.String()
	dir, _ := migrateOnce(t, kb(node))
	out := readRel(t, dir, "a.md")
	want := "Range claim [^3][^4][^5]. List claim [^1][^2]. Pair claim [^1][^2]."
	if !strings.Contains(out, want) {
		t.Fatalf("R4: grouped references not expanded\n--- got\n%s\n--- want substring\n%s", out, want)
	}
}

// Negative control, rule R4: code spans and fences, inline-link text,
// reference-style link labels with a definition, and footnotes already in
// place are never rewritten.
func TestConformance_MigrateSourceIDs_Section4_2_CodeAndLinksUntouched(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\n" +
		"Use `arr[3]` in code. See [3](https://x.example/) and [the doc][4]. Real [3].\n\n" +
		"```\nlist[3] = 1\n```\n\n" +
		"[4]: https://doc.example/\n\n" +
		"# Citations\n\n" +
		"[3] VERIFIED — three https://three.example/\n" +
		"[4] VERIFIED — four https://four.example/\n"
	dir, _ := migrateOnce(t, kb(node))
	out := readRel(t, dir, "a.md")
	for _, keep := range []string{"`arr[3]`", "[3](https://x.example/)", "[the doc][4]", "list[3] = 1", "Real [^3]."} {
		if !strings.Contains(out, keep) {
			t.Errorf("R4: expected %q in\n%s", keep, out)
		}
	}
}

// Rule R4: a bracketed number near "brief", "citation", "node", or a link may
// name another document's list, so it goes to judgment and stays as written.
func TestConformance_MigrateSourceIDs_Section4_2_CrossDocumentReferenceIsJudgment(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\n" +
		"As the write-time-curation brief says in its citation [1], this holds.\n\n" +
		"Unrelated paragraph that cites the local source plainly [1] here.\n\n" +
		"# Citations\n\n" +
		"[1] VERIFIED — one https://one.example/\n"
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	j := findJudgment(t, plan, "a.md", JudgmentCitationRef)
	if !strings.Contains(j.Context, "brief") || !strings.Contains(j.Context, "[1]") {
		t.Fatalf("judgment context must show the reference and its neighbourhood, got %q", j.Context)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	if !strings.Contains(out, "its citation [1], this holds") || !strings.Contains(out, "plainly [^1] here") {
		t.Fatalf("R4: near-'brief' ref must stay, plain ref must be rewritten:\n%s", out)
	}
}

// Rule R3: an entry whose resource was already in `sources` gets its id and
// definition too. Re-planning the migrated tree plans 0 edits, and
// re-applying the same plan changes nothing (§12).
func TestConformance_MigrateSourceIDs_Section12_RerunPlansZeroEdits(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n  - resource: https://kept.example/\n    title: Kept\n---\n\n# Def\n\n" +
		"Old claim [1]. New claim [2]. Ambiguous brief [2].\n\n# Citations\n\n" +
		"[1] VERIFIED — *Kept*, https://kept.example/\n" +
		"[2] VERIFIED — new https://new.example/\n" +
		"[3] VERIFIED — on-box probe, no URL.\n"
	dir, plan := migrateOnce(t, kb(node))
	got := idsByResource(findNodeMigration(t, plan, "a.md"))
	if got["https://kept.example/"] != "1" || got["https://new.example/"] != "2" {
		t.Fatalf("R3: ids = %v", got)
	}
	out := readRel(t, dir, "a.md")
	if !strings.Contains(out, "  - resource: https://kept.example/\n    id: \"1\"\n    title: Kept\n") {
		t.Fatalf("§4: id inserted under the existing source's resource line only:\n%s", out)
	}
	if strings.Contains(out, "[^3]") {
		t.Fatalf("an entry with no source gets no footnote (its label would join no sources[].id):\n%s", out)
	}
	nb := loadMigrate(t, dir)
	if fs := Validate(nb); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
	_, again := planFor(t, dir, "", false)
	if len(again.Nodes) != 0 {
		t.Fatalf("re-plan must plan 0 edits, got %+v", again.Nodes)
	}
	once := treeHash(t, dir)
	if err := MigrateApply(dir, nb, plan); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if treeHash(t, dir) != once {
		t.Fatalf("§12: re-applying the same plan changed the tree:\n%s", readRel(t, dir, "a.md"))
	}
}

// Two entries with one resource: the source can hold one id, so the second
// entry is reported and its body references stay as written.
func TestConformance_MigrateSourceIDs_Section5_1_SharedResourceSecondEntryIsJudgment(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nFirst [1]. Second [2].\n\n# Citations\n\n" +
		"[1] VERIFIED — page https://same.example/\n" +
		"[2] VERIFIED — same page again https://same.example/\n"
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	if !hasJudgment(plan, "a.md", JudgmentCitationUnattached) {
		t.Fatalf("second entry for one resource must be a judgment item; got %+v", plan.Judgment)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	if !strings.Contains(out, "First [^1]. Second [2].") {
		t.Fatalf("only the attached entry's references are rewritten:\n%s", out)
	}
}
