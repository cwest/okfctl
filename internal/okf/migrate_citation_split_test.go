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

// v0.3 §5.1: `resource` is REQUIRED and singular, so a `# Citations` entry
// that names several resources becomes one source per resource. The parts
// share the entry's key: the first part's `id` is the entry's id, the later
// ones `<id>-2`, `<id>-3`. Per-claim attribution is a footnote whose label is
// a `sources[].id`, so a body `[k]` that cites the entry becomes one footnote
// per part (`[^12][^12-2]`). The first part's footnote definition holds the
// whole entry text; consumers do not parse it. A later part's definition is
// its name, or its resource when it has none. `title` is a name only.

func sourceByID(nm *NodeMigration, id string) (SourceEdit, bool) {
	for _, s := range nm.Sources {
		if s.ID == id {
			return s, true
		}
	}
	return SourceEdit{}, false
}

const splitTwoNode = "---\ntype: Concept\n---\n\n# Def\n\n" +
	"The schema is documented [12]. A second claim cites it again [12].\n\n" +
	"# Citations\n\n" +
	"- [12] VERIFIED — *Export Schema*, https://a.example/schema; mirror\n" +
	"  https://b.example/mirror (retrieved 2026-06-01)\n"

// Positive control: a 2-URL entry splits into two sources keyed `12` and
// `12-2`; the first is titled with the work's name only (§5.1 "a label for
// humans") and the later part, which marks no name, gets none.
func TestConformance_MigrateSplit_Section5_1_TwoURLEntrySplitsWithIDs(t *testing.T) {
	dir := mkMigrateBundle(t, kb(splitTwoNode))
	_, plan := planFor(t, dir, "", false)
	if hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: a 2-URL entry must split, not stay a prose-citation item: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	want := []SourceEdit{
		{ID: "12", Resource: "https://a.example/schema", Title: "Export Schema"},
		{ID: "12-2", Resource: "https://b.example/mirror"},
	}
	if len(nm.Sources) != len(want) {
		t.Fatalf("§5.1: want %d sources, got %+v", len(want), nm.Sources)
	}
	for i, w := range want {
		if nm.Sources[i] != w {
			t.Errorf("§5.1: source %d = %+v, want %+v", i, nm.Sources[i], w)
		}
	}
}

// Rule R4 for a split entry: every body `[12]` (cited twice here) becomes a
// footnote per part, the first part's definition is the full entry text, and
// the later part's definition is its resource (it has no name). The migrated
// bundle validates and the `# Citations` list is left in place.
func TestConformance_MigrateSplit_Section4_2_MarkerCitedTwicePointsAtEveryPart(t *testing.T) {
	dir, _ := migrateOnce(t, kb(splitTwoNode))
	out := readRel(t, dir, "a.md")
	for _, want := range []string{
		"The schema is documented [^12][^12-2]. A second claim cites it again [^12][^12-2].\n",
		"[^12]: VERIFIED — *Export Schema*, https://a.example/schema; mirror https://b.example/mirror (retrieved 2026-06-01)\n",
		"[^12-2]: https://b.example/mirror\n",
		"  - id: \"12\"\n    resource: \"https://a.example/schema\"\n    title: Export Schema\n",
		"  - id: 12-2\n    resource: \"https://b.example/mirror\"\n",
		"# Citations\n\n- [12] VERIFIED — *Export Schema*, https://a.example/schema; mirror\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("§4.2/§5.1: missing %q in\n%s", want, out)
		}
	}
	if fs := Validate(loadMigrate(t, dir)); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
}

// Positive control, 3 parts: link text names a part (citationName's leading
// link rule, reused per part), a bare URL part gets no title, and a grouped
// reference `[1, 2]` expands to every part of entry 2.
func TestConformance_MigrateSplit_Section5_1_ThreeURLEntryNamesEachLinkedPart(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nOne claim [1]. A grouped claim [1, 2].\n\n# Citations\n\n" +
		"[1] VERIFIED — single https://one.example/\n" +
		"[2] VERIFIED — [Doc A](https://a.example/) and [Doc B](https://b.example/), also https://c.example/\n"
	dir, plan := migrateOnce(t, kb(node))
	nm := findNodeMigration(t, plan, "a.md")
	for id, w := range map[string]SourceEdit{
		"2":   {ID: "2", Resource: "https://a.example/", Title: "Doc A"},
		"2-2": {ID: "2-2", Resource: "https://b.example/", Title: "Doc B"},
		"2-3": {ID: "2-3", Resource: "https://c.example/"},
	} {
		if got, ok := sourceByID(nm, id); !ok || got != w {
			t.Errorf("§5.1: source %s = %+v (found %v), want %+v", id, got, ok, w)
		}
	}
	out := readRel(t, dir, "a.md")
	for _, want := range []string{
		"One claim [^1]. A grouped claim [^1][^2][^2-2][^2-3].",
		"[^2-2]: Doc B\n",
		"[^2-3]: https://c.example/\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("R4: missing %q in\n%s", want, out)
		}
	}
	if fs := Validate(loadMigrate(t, dir)); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
}

// Negative control: a single-URL entry is not split. It keeps its one source
// and its one footnote, and no `-2` id appears anywhere.
func TestConformance_MigrateSplit_Section5_1_SingleURLEntryUntouched(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nA claim [5].\n\n# Citations\n\n" +
		"[5] VERIFIED — *Only Work*, https://only.example/page\n"
	dir, plan := migrateOnce(t, kb(node))
	nm := findNodeMigration(t, plan, "a.md")
	if len(nm.Sources) != 1 || nm.Sources[0] != (SourceEdit{ID: "5", Resource: "https://only.example/page", Title: "Only Work"}) {
		t.Fatalf("a single-URL entry must stay one source, got %+v", nm.Sources)
	}
	out := readRel(t, dir, "a.md")
	if strings.Contains(out, "5-2") || !strings.Contains(out, "A claim [^5].") {
		t.Fatalf("a single-URL entry gets one footnote only:\n%s", out)
	}
}

// Negative control: a URL inside a code span is an example or a template, not
// a source. It neither adds a part to a split entry nor turns a one-URL entry
// into a two-URL one. An entry whose only URL sits in a code span keeps the
// single-source conversion it had before, and one whose several URLs all sit
// in code spans stays a judgment item.
func TestConformance_MigrateSplit_Section5_1_CodeSpanURLIsNotASource(t *testing.T) {
	cases := []struct {
		entry string
		want  []string // resources, in order; nil = judgment
	}{
		{"[7] Spec https://a.example/ and https://b.example/; call `curl https://api.example/v1`.",
			[]string{"https://a.example/", "https://b.example/"}},
		{"[8] Registry docs https://reg.example/docs; query `https://reg.example/v0/servers?search=<term>`.",
			[]string{"https://reg.example/docs"}},
		{"[9] VERIFIED — OpenTable docs, `https://docs.opentable.example/`: partners only.",
			[]string{"https://docs.opentable.example/"}},
		{"[10] Connector, `https://claude.example/resy`: \"`https://apigw.example/mcp`\".", nil},
	}
	for _, c := range cases {
		var got []string
		if srcs, ok := citationSourcesFrom(c.entry); ok {
			for _, s := range srcs {
				got = append(got, s.Resource)
			}
		} else if s, ok := citationSource(c.entry); ok {
			got = []string{s.Resource}
		}
		if strings.Join(got, " ") != strings.Join(c.want, " ") {
			t.Errorf("§5.1: %q -> resources %q, want %q", c.entry, got, c.want)
		}
	}
}

// A link nested inside a bracketed annotation (`[GRADE — … see the [node](/x.md)]`)
// names its own text, never the annotation: link text holds no `[`. The part
// it names is titled with the link text only (§5.1 "a label for humans").
func TestConformance_MigrateSplit_Section5_1_LinkInsideAnnotationNamesOnlyItself(t *testing.T) {
	entry := "- [12] Anthropic, \"Agentic coding,\" 2026. https://a.example/r [INDUSTRY-REPORT — observational. Also cited in the [collapse node](/c.md).]"
	srcs, ok := citationSourcesFrom(entry)
	if !ok || len(srcs) != 2 {
		t.Fatalf("want 2 parts, got %+v ok=%v", srcs, ok)
	}
	if srcs[1] != (SourceEdit{Resource: "/c.md", Title: "collapse node"}) {
		t.Fatalf("§5.1: part 2 = %+v, want title %q", srcs[1], "collapse node")
	}
}

// Negative control: a part whose resource another entry in the node already
// holds would need two ids on one source, so the entry stays a prose-citation
// item and nothing is split; the body reference stays as written.
func TestConformance_MigrateSplit_Section5_1_PartSharedWithAnotherEntryStaysJudgment(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nFirst [1]. Second [2].\n\n# Citations\n\n" +
		"[1] VERIFIED — page https://same.example/\n" +
		"[2] VERIFIED — same page https://same.example/ and https://other.example/\n"
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	j := findJudgment(t, plan, "a.md", JudgmentProseCitation)
	if !strings.Contains(j.Context, "[2]") {
		t.Fatalf("the entry sharing a part must be the judgment item, got %q", j.Context)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	if !strings.Contains(out, "First [^1]. Second [2].") || strings.Contains(out, "other.example/\n    ") {
		t.Fatalf("only entry 1 converts; entry 2 is left whole:\n%s", out)
	}
}

// In-bundle links split too (§6.2 bundle paths are resources), each a §6.4
// graph edge, and the migrated node validates.
func TestConformance_MigrateSplit_Section6_2_BundlePathPartsSplit(t *testing.T) {
	files := kb("---\ntype: Concept\n---\n\n# Def\n\nBuilt on these [2].\n\n# Citations\n\n" +
		"[2] [The Manifesto](/x.md) and [The Lifecycle](/y.md) — the values this implements.\n")
	files["x.md"] = "---\ntype: Concept\n---\n\n# X\n"
	files["y.md"] = "---\ntype: Concept\n---\n\n# Y\n"
	dir, plan := migrateOnce(t, files)
	nm := findNodeMigration(t, plan, "a.md")
	if s, _ := sourceByID(nm, "2"); s.Resource != "/x.md" || s.Title != "The Manifesto" {
		t.Errorf("§6.2: part 1 = %+v", s)
	}
	if s, _ := sourceByID(nm, "2-2"); s.Resource != "/y.md" || s.Title != "The Lifecycle" {
		t.Errorf("§6.2: part 2 = %+v", s)
	}
	if !strings.Contains(readRel(t, dir, "a.md"), "Built on these [^2][^2-2].") {
		t.Errorf("R4: body reference not expanded:\n%s", readRel(t, dir, "a.md"))
	}
	if fs := Validate(loadMigrate(t, dir)); len(fs) != 0 {
		t.Fatalf("migrated bundle must validate: %+v", fs)
	}
}

// §12: re-planning a migrated tree plans 0 edits, and re-applying the same plan
// changes nothing, for split entries as for single ones.
func TestConformance_MigrateSplit_Section12_RerunPlansZeroEdits(t *testing.T) {
	node := splitTwoNode + "- [13] VERIFIED — [Doc A](https://a2.example/) and [Doc B](https://b2.example/) and https://c2.example/\n"
	node = strings.Replace(node, "A second claim", "A third [13]. A second claim", 1)
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
		if j.Kind == JudgmentProseCitation || j.Kind == JudgmentSourceIDCollision {
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
