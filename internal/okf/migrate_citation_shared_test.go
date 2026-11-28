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

// v0.3 §5.1: one `resource` per source, one `id` per source. A `# Citations`
// entry that names several resources, one of which another entry also names,
// still splits: each shared resource keeps ONE source, held by its owner (the
// first entry that names it alone, else the first entry that splits), and
// every other entry that names it cites the owner's id. The entry adds a
// source only for the parts it owns; its first owned part holds its full
// text as a footnote (§4.2). An entry that owns no part names nothing of its
// own, so it stays a prose-citation item (§12.1 fallback).

func mustValidate(t *testing.T, dir string) {
	t.Helper()
	if fs := Validate(loadMigrate(t, dir)); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
}

func wantIn(t *testing.T, out string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in\n%s", w, out)
		}
	}
}

// Positive control: [2] names the page [1] already cites plus one of its own.
// [2] adds a source only for its own page, its footnote holds its full text,
// and its body reference cites both sources (§5.1, §4.2). The list, now held
// by sources and footnotes, is removed.
func TestConformance_MigrateSplitShared_Section5_1_SharedPartCitesOwnerSource(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nFirst [1]. Second [2].\n\n# Citations\n\n" +
		"[1] VERIFIED — page https://same.example/\n" +
		"[2] VERIFIED — same page https://same.example/ and https://other.example/\n"
	dir, plan := migrateOnce(t, kb(node))
	if hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: [2] owns other.example and must convert: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	want := []SourceEdit{
		{ID: "1", Resource: "https://same.example/"},
		{ID: "2", Resource: "https://other.example/"},
	}
	if len(nm.Sources) != len(want) {
		t.Fatalf("§5.1: one source per resource, want %+v, got %+v", want, nm.Sources)
	}
	for i, w := range want {
		if nm.Sources[i] != w {
			t.Errorf("§5.1: source %d = %+v, want %+v", i, nm.Sources[i], w)
		}
	}
	out := readRel(t, dir, "a.md")
	wantIn(t, out,
		"First [^1]. Second [^2][^1].\n",
		"[^2]: VERIFIED — same page https://same.example/ and https://other.example/\n",
	)
	if strings.Contains(out, "2-2") || strings.Contains(out, "# Citations") {
		t.Errorf("§4.2: no second source for a borrowed part, and the list goes:\n%s", out)
	}
	mustValidate(t, dir)
}

// Positive control, bundle paths (§6.2), real corpus shape: [2] cites two
// nodes, [5] cites a talk and one of those nodes. [2] splits first, so it
// owns both nodes; [5] owns the talk and cites [2]'s source for the node.
func TestConformance_MigrateSplitShared_Section6_2_BundlePathLinkSharedBetweenEntries(t *testing.T) {
	files := kb("---\ntype: Concept\n---\n\n# Def\n\nBuilt on these [2]. A keynote agrees [5].\n\n# Citations\n\n" +
		"[2] [The Manifesto](/x.md) and [The Lifecycle](/y.md) — the values this implements.\n\n" +
		"[5] ATTRIBUTED — Jones, \"Building an Org.\" <https://youtu.be/abc>. Folded into\n" +
		"[the Lifecycle](/y.md) as field evidence.\n")
	files["x.md"] = "---\ntype: Concept\n---\n\n# X\n"
	files["y.md"] = "---\ntype: Concept\n---\n\n# Y\n"
	dir, plan := migrateOnce(t, files)
	if hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§6.2: both entries own a resource and must convert: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	for id, w := range map[string]SourceEdit{
		"2":   {ID: "2", Resource: "/x.md", Title: "The Manifesto"},
		"2-2": {ID: "2-2", Resource: "/y.md", Title: "The Lifecycle"},
		"5":   {ID: "5", Resource: "https://youtu.be/abc", Title: "Building an Org"},
	} {
		if got, ok := sourceByID(nm, id); !ok || got != w {
			t.Errorf("§5.1: source %s = %+v (found %v), want %+v", id, got, ok, w)
		}
	}
	if len(nm.Sources) != 3 {
		t.Errorf("§5.1: /y.md is one source, got %+v", nm.Sources)
	}
	wantIn(t, readRel(t, dir, "a.md"), "Built on these [^2][^2-2]. A keynote agrees [^5][^2-2].")
	mustValidate(t, dir)
}

// Positive control, autolink list (real corpus shape): [4] lists two threads
// as `<url> · <url>` across a wrapped line; [9] cites the second thread alone,
// so [9] owns it and [4] cites [9]'s source for it.
func TestConformance_MigrateSplitShared_Section5_1_AutolinkListWithSingleOwner(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nPraise and pain [4]. Watch data [9].\n\n# Citations\n\n" +
		"[4] VERIFIED — r/bouldering \"apps\" and \"what do you use\" threads.\n" +
		"<https://www.reddit.com/r/b/comments/1/apps/> ·\n" +
		"<https://www.reddit.com/r/b/comments/2/what/>\n\n" +
		"[9] VERIFIED — r/bouldering, \"what do you use\": \"+1 for Redpoint.\"\n" +
		"<https://www.reddit.com/r/b/comments/2/what/>\n"
	dir, plan := migrateOnce(t, kb(node))
	if hasJudgment(plan, "a.md", JudgmentProseCitation) || hasJudgment(plan, "a.md", JudgmentCitationUnattached) {
		t.Fatalf("§5.1: both entries convert, each resource once: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	if s, _ := sourceByID(nm, "4"); s.Resource != "https://www.reddit.com/r/b/comments/1/apps/" {
		t.Errorf("§5.1: [4]'s own source = %+v", s)
	}
	if s, _ := sourceByID(nm, "9"); s.Resource != "https://www.reddit.com/r/b/comments/2/what/" {
		t.Errorf("§5.1: [9] owns the thread it cites alone, got %+v", s)
	}
	wantIn(t, readRel(t, dir, "a.md"), "Praise and pain [^4][^9]. Watch data [^9].")
	mustValidate(t, dir)
}

// Positive control, DOI: [1] links its paper's DOI; [2] links the same DOI in
// another letter case and through the legacy dx.doi.org host. DOIs compare
// case-insensitively (canonicalResource), so [2] cites [1]'s source for it
// instead of adding a second source for one document. (An entry that names
// its DOI only as an identifier, and that another entry also names, is a
// cross-reference and stays a judgment item; see
// TestConformance_MigrateIdentifier_Section5_1_CrossReferenceStaysJudgment.)
func TestConformance_MigrateSplitShared_Section5_1_SharedDOIComparedCaseInsensitively(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nA finding [1]. A review [2].\n\n# Citations\n\n" +
		"[1] VERIFIED — Raichle et al., \"A default mode,\" PNAS 2001. https://dx.doi.org/10.1073/PNAS.98.2.676\n" +
		"[2] VERIFIED — review https://review.example/dmn and the original https://doi.org/10.1073/pnas.98.2.676\n"
	dir, plan := migrateOnce(t, kb(node))
	if hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: [2] owns review.example and must convert: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	if len(nm.Sources) != 2 {
		t.Fatalf("§5.1: one source per document, got %+v", nm.Sources)
	}
	if s, _ := sourceByID(nm, "2"); s.Resource != "https://review.example/dmn" {
		t.Errorf("§5.1: [2]'s own source = %+v", s)
	}
	wantIn(t, readRel(t, dir, "a.md"), "A finding [^1]. A review [^2][^1].")
	mustValidate(t, dir)
}

// Owner order: an entry that names a resource alone owns it even when a
// splitting entry names it first. No entry is left unattached.
func TestConformance_MigrateSplitShared_Section5_1_SoleEntryOwnsOverEarlierSplit(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nBoth [1]. One [2].\n\n# Citations\n\n" +
		"[1] VERIFIED — https://a.example/ and https://b.example/\n" +
		"[2] VERIFIED — *Doc B*, https://b.example/\n"
	dir, plan := migrateOnce(t, kb(node))
	if hasJudgment(plan, "a.md", JudgmentCitationUnattached) || hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: [2] owns b.example, [1] owns a.example: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	if s, _ := sourceByID(nm, "2"); s != (SourceEdit{ID: "2", Resource: "https://b.example/", Title: "Doc B"}) {
		t.Errorf("§5.1: [2] = %+v", s)
	}
	if s, _ := sourceByID(nm, "1"); s.Resource != "https://a.example/" {
		t.Errorf("§5.1: [1] = %+v", s)
	}
	wantIn(t, readRel(t, dir, "a.md"), "Both [^1][^2]. One [^2].")
	mustValidate(t, dir)
}

// The name an entry marks belongs with its first resource. When another entry
// owns that resource, the name does not title the part this entry owns
// (§5.1, title is the cited work's name): the part stays untitled and is
// reported, never mis-titled.
func TestConformance_MigrateSplitShared_Section5_1_MarkedNameNotMovedToOwnedPart(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nA [1]. B [2].\n\n# Citations\n\n" +
		"[1] VERIFIED — *Work A*, https://a.example/\n" +
		"[2] VERIFIED — *Work A*, https://a.example/; mirror https://m.example/\n"
	_, plan := migrateOnce(t, kb(node))
	nm := findNodeMigration(t, plan, "a.md")
	if s, _ := sourceByID(nm, "2"); s != (SourceEdit{ID: "2", Resource: "https://m.example/"}) {
		t.Errorf("§5.1: [2]'s own part must stay untitled, got %+v", s)
	}
	if !hasJudgment(plan, "a.md", JudgmentSourceTitleUnnamed) {
		t.Errorf("an untitled first owned part is reported: %+v", plan.Judgment)
	}
}

// Negative control: an entry whose every link points at a node or page that
// other entries cite as their own work ("see [X] above") names no cited work
// of its own. It stays a prose-citation item, its body reference stays as
// written, and the list is kept (§12.1 fallback).
func TestConformance_MigrateSplitShared_Section12_1_EntryOwningNoPartStaysProse(t *testing.T) {
	files := kb("---\ntype: Concept\n---\n\n# Def\n\nX [1]. A [2]. Context [3].\n\n# Citations\n\n" +
		"[1] [The X node](/x.md)\n" +
		"[2] VERIFIED — https://a.example/\n" +
		"[3] Background, as argued in [the X node](/x.md) and checked at https://a.example/ above.\n")
	files["x.md"] = "---\ntype: Concept\n---\n\n# X\n"
	dir, plan := migrateOnce(t, files)
	j := findJudgment(t, plan, "a.md", JudgmentProseCitation)
	if !strings.Contains(j.Context, "[3]") {
		t.Fatalf("§12.1: [3] owns nothing and must stay, got %q", j.Context)
	}
	out := readRel(t, dir, "a.md")
	wantIn(t, out, "Context [3].", "# Citations")
	if strings.Contains(out, "[^3]") {
		t.Errorf("§5.1: an entry with no source gets no footnote:\n%s", out)
	}
}

// Negative control: the word ISBN with no number names no document.
func TestConformance_MigrateSplitShared_Section5_1_BareISBNWordStaysProse(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nA book [3].\n\n# Citations\n\n" +
		"[3] Wheeler, *Designing Brand Identity*, Wiley (ISBN not recorded here).\n"
	_, plan := migrateOnce(t, kb(node))
	if !hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: a bare ISBN word builds no resource: %+v", plan.Nodes)
	}
}

// §12: re-planning the migrated tree plans 0 edits and re-applying the same
// plan changes nothing, with a borrowed part, a sole owner, and a kept list
// (the [3] prose entry keeps it, so the entries are re-read on the re-plan).
func TestConformance_MigrateSplitShared_Section12_RerunPlansZeroEdits(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nBoth [1]. One [2]. Prose [3].\n\n# Citations\n\n" +
		"[1] VERIFIED — https://a.example/ and https://b.example/\n" +
		"[2] VERIFIED — *Doc B*, https://b.example/\n" +
		"[3] VERIFIED — on-box probe, no URL.\n"
	dir, plan := migrateOnce(t, kb(node))
	nb := loadMigrate(t, dir)
	if fs := Validate(nb); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
	_, again := planFor(t, dir, "", false)
	if len(again.Nodes) != 0 {
		t.Fatalf("§12: re-plan must plan 0 edits, got %+v", again.Nodes)
	}
	for _, j := range again.Judgment {
		if j.Kind == JudgmentCitationUnattached || j.Kind == JudgmentSourceIDCollision {
			t.Errorf("§12: re-plan reports %s: %q", j.Kind, j.Context)
		}
	}
	once := treeHash(t, dir)
	if err := MigrateApply(dir, nb, plan); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if treeHash(t, dir) != once {
		t.Fatalf("§12: re-applying the same plan changed the tree:\n%s", readRel(t, dir, "a.md"))
	}
}

// Regression: [1] is a link reference definition, the target of a
// `[text][1]` link, so it takes no id (planSourceIDs) and owns nothing. [2]
// is the first citation of a.example, so it owns it and splits; [3] borrows
// the b.example part [2] owns and owns c.example. No reference cites an id no
// source carries (§5.1 one id per source; §4.2 every footnote reference has a
// definition).
func TestConformance_MigrateSplitShared_Section5_1_RefDefOwnsNoPart(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nOne [2]. Two [3].\n\n# Citations\n\n" +
		"[1]: https://a.example/\n" +
		"[2] VERIFIED — https://a.example/ and https://b.example/\n" +
		"[3] VERIFIED — https://b.example/ and https://c.example/\n"
	dir, plan := migrateOnce(t, kb(node))
	out := readRel(t, dir, "a.md")
	nm := findNodeMigration(t, plan, "a.md")
	got := map[string]string{}
	for _, s := range nm.Sources {
		got[s.Resource] = s.ID
	}
	for res, id := range map[string]string{
		"https://a.example/": "2", "https://b.example/": "2-2", "https://c.example/": "3",
	} {
		if got[res] != id {
			t.Errorf("§5.1: source %s id = %q, want %q (sources %+v)", res, got[res], id, nm.Sources)
		}
	}
	wantIn(t, out, "One [^2][^2-2].", "Two [^3][^2-2].", "[^2-2]: https://b.example/")
	mustValidate(t, dir)
	assertReplanQuiet(t, dir)
}

// Regression, the r.part >= 1 shape: with the definition owning nothing, [2]
// owns every part it names, a.example last, and [3] borrows [2]'s b.example
// part (part index 1) rather than a part list [2] never got.
func TestConformance_MigrateSplitShared_Section5_1_RefDefLaterPartBorrowed(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nOne [2]. Two [3].\n\n# Citations\n\n" +
		"[1]: https://a.example/\n" +
		"[2] VERIFIED — https://x.example/ and https://b.example/ and https://a.example/\n" +
		"[3] VERIFIED — https://b.example/ and https://c.example/\n"
	dir, _ := migrateOnce(t, kb(node))
	out := readRel(t, dir, "a.md")
	wantIn(t, out, "One [^2][^2-2][^2-3].", "Two [^3][^2-2].",
		"[^3]: VERIFIED — https://b.example/ and https://c.example/")
	splits := planCitationSplits(strings.Split(strings.TrimSpace(node[strings.Index(node, "[1]:"):]), "\n"),
		func(s string) string { return s })
	if splits[0] != nil {
		t.Errorf("§5.1: a link reference definition never splits: %+v", splits[0])
	}
	if s := splits[1]; s == nil || len(s.parts) != 3 || len(s.borrowed) != 0 {
		t.Errorf("§5.1: [2] owns all three parts and borrows none, got %+v", s)
	}
	if s := splits[2]; s == nil || len(s.parts) != 1 || len(s.borrowed) != 1 || s.borrowed[0] != (partRef{entry: 1, part: 1}) {
		t.Errorf("§5.1: [3] owns c.example and borrows [2]'s b.example part, got %+v", s)
	}
	mustValidate(t, dir)
	assertReplanQuiet(t, dir)
}
