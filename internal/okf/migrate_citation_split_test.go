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
// bundle validates, and the `# Citations` list, now held by sources and
// footnotes, is removed (§4.2).
func TestConformance_MigrateSplit_Section4_2_MarkerCitedTwicePointsAtEveryPart(t *testing.T) {
	dir, _ := migrateOnce(t, kb(splitTwoNode))
	out := readRel(t, dir, "a.md")
	for _, want := range []string{
		"The schema is documented [^12][^12-2]. A second claim cites it again [^12][^12-2].\n",
		"[^12]: VERIFIED — *Export Schema*, https://a.example/schema; mirror https://b.example/mirror (retrieved 2026-06-01)\n",
		"[^12-2]: https://b.example/mirror\n",
		"  - id: \"12\"\n    resource: \"https://a.example/schema\"\n    title: Export Schema\n",
		"  - id: 12-2\n    resource: \"https://b.example/mirror\"\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("§4.2/§5.1: missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "# Citations") {
		t.Errorf("§4.2: a recoverable list must be removed:\n%s", out)
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

// Negative control: a URL inside a code span is an example, a command, or a
// template, not a source. It neither adds a part to a split entry nor turns a
// one-URL entry into a two-URL one. A code span that is a full URL and
// nothing else names a resource as an autolink does, so several of them split
// (§5.1); a span holding a bare host, a command, or a template placeholder
// (`…`, `{id}`, `<term>`) never does, and an entry whose only URLs are in
// such spans stays a judgment item.
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
		{"[10] Connector, `https://claude.example/resy`: \"`https://apigw.example/mcp`\".",
			[]string{"https://claude.example/resy", "https://apigw.example/mcp"}},
		{"[11] Run `curl https://a.example/v1` then `curl https://b.example/v2`.", nil},
		{"[12] VERIFIED — SSO surface `five-api.libsyn.example` and `api.libsyn.example`.", nil},
		{"[13] Login links on `https://libsyn.example/` point to `https://sso.libsyn.example/auth?client_id=…&state=…`.",
			[]string{"https://libsyn.example/"}},
		{"[14] Feed `https://a.example/feed.json`; fetch with `curl https://b.example/raw`.",
			[]string{"https://a.example/feed.json"}},
		{"[15] Docs `https://a.example/docs`, `https://a.example/api`; template `https://a.example/oauth?client_id=…`.",
			[]string{"https://a.example/docs", "https://a.example/api"}},
		{"[16] Endpoints `https://a.example/v1/{id}` and `https://a.example/v2/{id}`.", nil},
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

// A split entry titles its first part through the same name picker, so a
// quoted excerpt (a deprecation notice) is no name there either: the first
// part stays untitled (§5.1, title is the work's name). Real corpus shape,
// with its wrapped lines joined by spaces as citationEntries joins them.
func TestConformance_MigrateSplit_Section5_1_ExcerptIsNoPartTitle(t *testing.T) {
	entry := "[19] VERIFIED — Generative AI module in the Vertex AI SDK, deprecation notice: " +
		"\"deprecated and will no longer be available after June 24, 2026\". The removed " +
		"modules include `vertexai.generative_models`. https://cloud.google.com/vertex-ai/docs/deprecations " +
		"and https://pypi.org/project/google-genai/"
	srcs, ok := citationSourcesFrom(entry)
	if !ok || len(srcs) != 2 {
		t.Fatalf("want 2 parts, got %+v ok=%v", srcs, ok)
	}
	for i, s := range srcs {
		if s.Title != "" {
			t.Errorf("§5.1: part %d title = %q, want none (the quote is an excerpt)", i+1, s.Title)
		}
	}
}

// Negative control: an entry whose every resource another entry owns names no
// work of its own, so it gets no source: it stays a prose-citation item and
// its body reference stays as written. (An entry that also owns a part splits
// and cites the owner for the shared one; see migrate_citation_shared_test.go.)
func TestConformance_MigrateSplit_Section5_1_PartSharedWithAnotherEntryStaysJudgment(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nFirst [1]. Other [2]. Third [3].\n\n# Citations\n\n" +
		"[1] VERIFIED — page https://same.example/\n" +
		"[2] VERIFIED — other https://other.example/\n" +
		"[3] VERIFIED — same page https://same.example/ and https://other.example/\n"
	dir := mkMigrateBundle(t, kb(node))
	b, plan := planFor(t, dir, "", false)
	j := findJudgment(t, plan, "a.md", JudgmentProseCitation)
	if !strings.Contains(j.Context, "[3]") {
		t.Fatalf("the entry sharing a part must be the judgment item, got %q", j.Context)
	}
	if err := MigrateApply(dir, b, plan); err != nil {
		t.Fatalf("apply: %v", err)
	}
	out := readRel(t, dir, "a.md")
	if !strings.Contains(out, "First [^1]. Other [^2]. Third [3].") || strings.Contains(out, "[^3]") {
		t.Fatalf("entries 1 and 2 convert; entry 3 is left whole:\n%s", out)
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

// An entry whose only resources are two or more full URLs, each a whole code
// span, names them as plainly as autolinks: it splits one source per URL
// (§5.1, one resource per source) instead of falling to prose-citation. Real
// corpus shape (a Libsyn API citation), wrapped lines as written.
func TestConformance_MigrateSplit_Section5_1_WholeCodeSpanURLsSplit(t *testing.T) {
	node := "---\ntype: Concept\n---\n\n# Def\n\nThe token lives an hour [1].\n\n# Citations\n\n" +
		"[1] VERIFIED — Libsyn Public API v1 documentation (the OAuth flow). Retrieved 2026-06-25 from\n" +
		"`https://api.libsyn.example/apigility/documentation` (page title \"Libsyn API\n" +
		"Documentation\"). Token POST to `https://api.libsyn.example/oauth` with `grant_type:\"authorization_code\"`;\n" +
		"welcome page `https://api.libsyn.example/` self-describes \"Public API v1\".\n"
	dir, plan := migrateOnce(t, kb(node))
	if hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: whole-code-span URLs must convert, got prose-citation: %+v", plan.Judgment)
	}
	nm := findNodeMigration(t, plan, "a.md")
	var got []string
	for _, s := range nm.Sources {
		got = append(got, s.Resource)
	}
	want := []string{"https://api.libsyn.example/apigility/documentation", "https://api.libsyn.example/oauth", "https://api.libsyn.example/"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("§5.1: sources %q, want %q", got, want)
	}
	out := readRel(t, dir, "a.md")
	if !strings.Contains(out, "[^1]") || !strings.Contains(out, "[^1-2]") || !strings.Contains(out, "[^1-3]") {
		t.Fatalf("each part gets a footnote:\n%s", out)
	}
}
