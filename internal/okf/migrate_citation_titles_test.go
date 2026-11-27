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

// A `# Citations` entry whose resource is already listed in frontmatter
// `sources` (§5.1) is not added again. When that source has no `title`, the
// name of the cited work (citationName) becomes its title. §5.1 calls title
// "a label for humans", so it holds the name only, never the authors, venue,
// year, identifiers, URLs, or verification tags around it. When no name can
// be identified the title stays unset and the entry is a judgment item. When
// the source already has a different title, that title is kept and the entry
// is reported.

const titledNode = "---\ntype: Concept\nsources:\n" +
	"  - resource: \"https://kept.example/a\"\n" +
	"    title: \"Kept — the author's own label\" # keep me\n" +
	"  - resource: https://wiki.acme/fpa\n" +
	"  - resource: \"https://same.example/\"\n" +
	"    title: \"Same Work\"\n" +
	"  - resource: https://legacy.example/\n" +
	"    title: \"[4] VERIFIED — Legacy, *Legacy Work*, 2025. https://legacy.example/\"\n" +
	"  - resource: https://noname.example/\n" +
	"---\n\n# Def\n\n# Citations\n" +
	"- [1] VERIFIED — Acme, *Different Work*, 2026. https://kept.example/a\n" +
	"- [2] VERIFIED — A. Author et al., *The FP&A Handbook*, Acme Press 2024. https://wiki.acme/fpa\n" +
	"- [3] VERIFIED — Same, *Same Work*, 2026. https://same.example/\n" +
	"- [4] VERIFIED — Legacy, *Legacy Work*, 2025. https://legacy.example/\n" +
	"- [5] VERIFIED — on-box probe of https://noname.example/ returned 200.\n"

func titleJudgments(plan MigratePlan, kind JudgmentKind) []JudgmentItem {
	var out []JudgmentItem
	for _, j := range plan.Judgment {
		if j.Kind == kind {
			out = append(out, j)
		}
	}
	return out
}

// Positive control: an untitled existing source gains the work's name, and
// only the name.
func TestConformance_MigrateCitationTitle_Section5_1_UntitledExistingSourceGainsName(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n", "a.md": titledNode,
	})
	_, plan := planFor(t, dir, "", false)
	nm := findNodeMigration(t, plan, "a.md")
	want := SourceTitleEdit{Resource: "https://wiki.acme/fpa", Title: "The FP&A Handbook"}
	if len(nm.Titles) != 1 || nm.Titles[0] != want {
		t.Fatalf("§5.1: titles = %+v, want [%+v]", nm.Titles, want)
	}
	if len(nm.Sources) != 0 {
		t.Fatalf("idempotence: an existing resource must not be added again, got %+v", nm.Sources)
	}
	if len(nm.Titles) != 1 {
		t.Fatalf("want exactly one title edit, got %+v", nm.Titles)
	}
}

// An untitled source whose entry marks no name keeps no title and is
// reported. It is never titled with the entry's full text.
func TestConformance_MigrateCitationTitle_Section5_1_NoNameIsReportedNotGuessed(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n", "a.md": titledNode,
	})
	_, plan := planFor(t, dir, "", false)
	got := titleJudgments(plan, JudgmentSourceTitleUnnamed)
	if len(got) != 1 || got[0].Context != "- [5] VERIFIED — on-box probe of https://noname.example/ returned 200." {
		t.Fatalf("want one source-title-unnamed for entry [5], got %+v", got)
	}
	for _, s := range findNodeMigration(t, plan, "a.md").Titles {
		if s.Resource == "https://noname.example/" {
			t.Fatalf("an unnamed entry must not produce a title, planned %+v", s)
		}
	}
}

// A titled source keeps its title. A different title is reported (positive
// control). A title equal to the name, or to this entry's own full text (a
// title an earlier migration wrote), is not a conflict (negative controls).
func TestConformance_MigrateCitationTitle_Section5_1_TitledExistingSourceIsReported(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n", "a.md": titledNode,
	})
	_, plan := planFor(t, dir, "", false)
	got := titleJudgments(plan, JudgmentSourceTitleConflict)
	if len(got) != 1 || got[0].Path != "a.md" || got[0].Context != "- [1] VERIFIED — Acme, *Different Work*, 2026. https://kept.example/a" {
		t.Fatalf("want one source-title-conflict for entry [1], got %+v", got)
	}
	for _, s := range findNodeMigration(t, plan, "a.md").Titles {
		if s.Resource != "https://wiki.acme/fpa" {
			t.Fatalf("a titled source must never be retitled, planned %+v", s)
		}
	}
	// Neither kind needs a §12.1 fallback: the resource is already in
	// frontmatter, so neither holds the version declaration.
	if plan.FallbackItems() != 0 {
		t.Fatalf("FallbackItems = %d, want 0", plan.FallbackItems())
	}
}

// A bare-URL entry has no text beyond its resource: nothing to name, nothing
// to report (negative control).
func TestConformance_MigrateCitationTitle_Section5_1_BareURLEntryPlansNothing(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n",
		"a.md": "---\ntype: Concept\nsources:\n  - resource: https://wiki.acme/fpa\n---\n\n# Def\n\n" +
			"# Citations\n- https://wiki.acme/fpa\n",
	})
	_, plan := planFor(t, dir, "", false)
	// The entry still gets its §5.1 id and footnote definition; a bare URL
	// names nothing, so no title is planned.
	if nm := planNodeMigration(plan, "a.md"); nm != nil && len(nm.Titles) != 0 {
		t.Fatalf("want no title edit, got %+v", nm.Titles)
	}
	if len(plan.Judgment) != 0 {
		t.Fatalf("want no judgment, got %+v", plan.Judgment)
	}
}

// Two entries for one untitled existing source: the first entry's name
// becomes the title and a second, different name is reported, never merged.
func TestConformance_MigrateCitationTitle_Section5_1_SecondDifferentEntryIsReported(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n",
		"a.md": "---\ntype: Concept\nsources:\n  - resource: https://wiki.acme/fpa\n---\n\n# Def\n\n" +
			"# Citations\n- [1] Acme, *First Work*, 2024. https://wiki.acme/fpa\n" +
			"- [2] Acme, *Second Work*, 2025. https://wiki.acme/fpa\n",
	})
	_, plan := planFor(t, dir, "", false)
	nm := findNodeMigration(t, plan, "a.md")
	if len(nm.Titles) != 1 || nm.Titles[0].Title != "First Work" {
		t.Fatalf("titles = %+v", nm.Titles)
	}
	if !hasJudgment(plan, "a.md", JudgmentSourceTitleConflict) {
		t.Fatalf("second entry with a different name must be reported: %+v", plan.Judgment)
	}
}

// A new source made from a citation entry follows the same rule: its title is
// the work's name, and with no identifiable name it is added untitled and
// reported.
func TestConformance_MigrateCitationTitle_Section5_1_NewSourceTitleIsNameOnly(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n",
		"a.md": "---\ntype: Concept\n---\n\n# Def\n\n# Citations\n" +
			"- [1] VERIFIED — N. F. Liu et al., *Lost in the Middle: How Language Models Use Long Contexts*, TACL 2023. https://arxiv.org/abs/2307.03172\n" +
			"- [2] VERIFIED — on-box probe of https://probe.example/ returned 200.\n",
	})
	_, plan := planFor(t, dir, "", false)
	nm := findNodeMigration(t, plan, "a.md")
	want := []SourceEdit{
		{Resource: "https://arxiv.org/abs/2307.03172", Title: "Lost in the Middle: How Language Models Use Long Contexts", ID: "1"},
		{Resource: "https://probe.example/", ID: "2"},
	}
	if len(nm.Sources) != 2 || nm.Sources[0] != want[0] || nm.Sources[1] != want[1] {
		t.Fatalf("§5.1: sources = %+v, want %+v", nm.Sources, want)
	}
	got := titleJudgments(plan, JudgmentSourceTitleUnnamed)
	if len(got) != 1 || !strings.Contains(got[0].Context, "on-box probe") {
		t.Fatalf("want one source-title-unnamed for entry [2], got %+v", got)
	}
}

// Apply inserts exactly one `title:` line into the matching source and keeps
// every other byte (§4: SHOULD NOT re-serialize unchanged values). The result
// validates, a re-plan plans nothing, and a second apply changes nothing.
func TestConformance_MigrateCitationTitle_Section4_ApplySplicesAndIsIdempotent(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n", "a.md": titledNode,
	})
	b, plan := planFor(t, dir, "", false)
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Only lines are inserted: every original line survives in order, and
	// the title line sits under its source's resource (ids and footnote
	// definitions are the other inserted lines).
	got := readRel(t, dir, "a.md")
	if !strings.Contains(got, "  - resource: https://wiki.acme/fpa\n    id: \"2\"\n    title: \"The FP&A Handbook\"\n") {
		t.Fatalf("§4: title line not inserted under its resource\n%s", got)
	}
	// The one replaced line is source [4]'s legacy full-entry title, which
	// becomes the work's name once footnote [^4] holds the entry
	// (migrate_title_shorten.go). Every other original line survives.
	legacy := "    title: \"[4] VERIFIED — Legacy, *Legacy Work*, 2025. https://legacy.example/\"\n"
	if !strings.Contains(got, "  - resource: https://legacy.example/\n    id: \"4\"\n    title: \"Legacy Work\"\n") {
		t.Fatalf("§5.1: legacy full-entry title not shortened to the name\n%s", got)
	}
	if !linesSurviveInOrder(strings.Replace(titledNode, legacy, "    title: \"Legacy Work\"\n", 1), got) {
		t.Fatalf("§4: apply must only insert lines\n--- got\n%s\n--- orig\n%s", got, titledNode)
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

// A DOI URL is one document however it is spelled (canonicalResource), so an
// entry that cites the existing untitled source through another DOI host or
// case titles that source rather than adding a second one. The edit names the
// resource as the frontmatter spells it, so apply finds the item.
func TestConformance_MigrateCitationTitle_Section5_1_DOISpellingTitlesExistingSource(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n  - resource: https://doi.org/10.1000/ABC\n---\n\n# Def\n\n# Citations\n" +
		"- Author, *The DOI Work*, 2020. http://dx.doi.org/10.1000/abc\n"
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n", "a.md": node,
	})
	b, plan := planFor(t, dir, "", false)
	nm := findNodeMigration(t, plan, "a.md")
	want := SourceTitleEdit{Resource: "https://doi.org/10.1000/ABC", Title: "The DOI Work"}
	if len(nm.Sources) != 0 || len(nm.Titles) != 1 || nm.Titles[0] != want {
		t.Fatalf("§5.1: sources = %+v titles = %+v, want no new source and [%+v]", nm.Sources, nm.Titles, want)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := readRel(t, dir, "a.md"); !strings.Contains(got, "10.1000/ABC\n    id: cite-1\n    title: The DOI Work\n") || !linesSurviveInOrder(node, got) {
		t.Fatalf("apply\n--- got\n%s", got)
	}
	if _, again := planFor(t, dir, "", false); len(again.Nodes) != 0 {
		t.Fatalf("re-plan must plan 0 edits, got %+v", again.Nodes)
	}
}

// A source titled by an earlier migration that split the entry at a wrapped
// line holds a fragment of the entry's own text (its head or its tail). That
// is the entry's text, not a different work, so it is not a conflict. A short
// title that differs from the name is still reported.
func TestConformance_MigrateCitationTitle_Section5_1_EntryFragmentTitleIsNotAConflict(t *testing.T) {
	node := "---\ntype: Concept\nsources:\n" +
		"  - resource: https://youtu.be/v\n" +
		"    title: \"[V] SOURCE VIDEO — Dr. Sealey, \\\"How to Crave Hard Things,\\\" https://youtu.be/v (transcript). Originates the thesis; \\\"~16,000 steps\\\" is NUANCED (see\"\n" +
		"  - resource: https://docs.example/guardrails\n" +
		"    title: \"[\\\"VIOLENCE\\\"].score }`; policy-effects table. https://docs.example/guardrails\"\n" +
		"  - resource: https://short.example/\n" +
		"    title: \"Google Cloud\"\n" +
		"---\n\n# Def\n\n# Citations\n\n" +
		"[V] SOURCE VIDEO — Dr. Sealey, \"How to Crave Hard Things,\" https://youtu.be/v (transcript).\n" +
		"Originates the thesis; \"~16,000 steps\" is NUANCED (see\n[1]); the rest is unverified.\n\n" +
		"[18] VERIFIED. AWS, Guide, \"Getting started with guardrails\": `forbid when { x\n[\"VIOLENCE\"].score }`; policy-effects table. https://docs.example/guardrails\n\n" +
		"[19] VERIFIED — Google Cloud, *Vertex AI Pipelines*: orchestration. https://short.example/\n"
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n", "a.md": node,
	})
	_, plan := planFor(t, dir, "", false)
	got := titleJudgments(plan, JudgmentSourceTitleConflict)
	if len(got) != 1 || !strings.Contains(got[0].Context, "Vertex AI Pipelines") {
		t.Fatalf("want only the short differing title reported, got %+v", got)
	}
	if nm := planNodeMigration(plan, "a.md"); nm != nil && len(nm.Titles) != 0 {
		t.Fatalf("a titled source's title is never edited, got %+v", nm.Titles)
	}
}

// A stale plan re-applied after the author titled the source by hand must not
// overwrite that title.
func TestConformance_MigrateCitationTitle_Section5_1_ApplyNeverOverwritesTitle(t *testing.T) {
	src := "---\ntype: Concept\nsources:\n  - resource: https://wiki.acme/fpa\n    title: Mine\n---\n\n# Def\n"
	nm := NodeMigration{Path: "a.md", Titles: []SourceTitleEdit{{Resource: "https://wiki.acme/fpa", Title: "Theirs"}}}
	out, err := renderMigratedNode([]byte(src), nm)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if string(out) != src {
		t.Fatalf("a titled source was rewritten:\n%s", out)
	}
}

// A CommonMark link reference definition (`[label]: <url> "title"`) under
// `# Citations` is a link target, not a citation. It neither titles an
// untitled source nor is reported against a titled one (negative control:
// the real corpus has such lines next to titled sources).
func TestConformance_MigrateCitationTitle_Section5_1_LinkReferenceDefinitionPlansNothing(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n", "log.md": "# Log\n",
		"a.md": "---\ntype: Concept\nsources:\n  - resource: https://getarcane.app/docs\n    title: \"[2] Arcane docs\"\n" +
			"  - resource: https://wiki.acme/fpa\n---\n\n# Def\n\nSee [docs][arcane-docs].\n\n# Citations\n" +
			"[arcane-docs]: https://getarcane.app/docs\n" +
			"[fpa]: <https://wiki.acme/fpa> \"Handbook\"\n",
	})
	_, plan := planFor(t, dir, "", false)
	if nm := planNodeMigration(plan, "a.md"); nm != nil {
		t.Fatalf("want no edit, got %+v", nm)
	}
	if len(plan.Judgment) != 0 {
		t.Fatalf("a link reference definition is neither a conflict nor unnamed: %+v", plan.Judgment)
	}
}

// linesSurviveInOrder reports whether every line of orig appears in got in the
// same order: an edit that only inserts lines.
func linesSurviveInOrder(orig, got string) bool {
	g := strings.Split(got, "\n")
	j := 0
	for _, line := range strings.Split(orig, "\n") {
		for j < len(g) && g[j] != line {
			j++
		}
		if j == len(g) {
			return false
		}
		j++
	}
	return true
}
