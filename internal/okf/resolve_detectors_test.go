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

// §6.4: a path field that resolves to a concept is a graph edge, from the
// concept that contains the field to the target. Positive control: every §6.2
// field kind makes its edge. Negative controls: a URL, a scope descriptor, an
// asset, a missing target, and an external ../ value make none.
func TestConformance_BuildGraph_Section6_4_PathFieldEdges(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":            v03Index,
		"tables/orders.md":    rdoc("Orders", "", "o"),
		"metrics/gm.md":       rdoc("GM", "", "g"),
		"refs/run.md":         rdoc("Run", "", "r"),
		"refs/att.md":         rdoc("Att", "", "a"),
		"refs/q.md":           rdoc("Q", "", "q"),
		"policy.md":           rdoc("Policy", "", "p"),
		"refs/asset.sql":      "select 1\n",
		"metrics/rev.md":      rdoc("Rev", "resource: /tables/orders.md\ncomputation: ../refs/q.md\nexecutor: { resource: /refs/run.md }\nattester: { resource: ../refs/att.md }\nstatus: deprecated\nsuperseded_by: gm.md\nsources:\n  - { resource: /policy.md }\n  - { resource: \"https://x.example/a.md\" }\n  - { resource: \"all queries in project X\" }\n", "r"),
		"metrics/control.md":  rdoc("Control", "computation: /refs/asset.sql\nresource: /missing.md\nsuperseded_by: ../../escape.md\n", "c"),
		"metrics/selfref.md":  rdoc("Self", "resource: selfref.md\n", "s"),
		"metrics/linkdup.md":  rdoc("LinkDup", "superseded_by: gm.md\nstatus: deprecated\n", "See [gm](gm.md)."),
		"metrics/unused.md":   rdoc("Unused", "", "u"),
		"metrics/unused-2.md": rdoc("Unused2", "", "u"),
	})
	g := BuildGraph(b)
	edges := map[string]bool{}
	for _, e := range g.Edges {
		edges[e.From+" -> "+e.To] = true
	}
	for _, want := range []string{
		"metrics/rev.md -> tables/orders.md", // resource, /-rooted
		"metrics/rev.md -> refs/q.md",        // computation, file-relative ../
		"metrics/rev.md -> refs/run.md",      // executor.resource
		"metrics/rev.md -> refs/att.md",      // attester.resource
		"metrics/rev.md -> metrics/gm.md",    // superseded_by, file-relative
		"metrics/rev.md -> policy.md",        // sources[].resource
	} {
		if !edges[want] {
			t.Errorf("§6.4: missing path-field edge %s; edges=%v", want, g.Edges)
		}
	}
	for e := range edges {
		if strings.HasPrefix(e, "metrics/control.md ->") {
			t.Errorf("§6.4 negative control: asset/missing/external path fields make no edge; got %s", e)
		}
	}
	// A path field that duplicates a link to the same target is one edge.
	n := 0
	for _, e := range g.Edges {
		if e.From == "metrics/linkdup.md" && e.To == "metrics/gm.md" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("§6.4: link + path field to one target must be one edge; got %d", n)
	}
	// A path-field edge confers inbound reachability (orphan agrees with lint).
	for _, gn := range g.Nodes {
		if gn.Path == "tables/orders.md" && gn.Orphan {
			t.Errorf("§6.4: a path-field target is not an orphan")
		}
		if gn.Path == "metrics/selfref.md" && !gn.Orphan {
			t.Errorf("a self-referencing path field must not rescue a node from orphanhood")
		}
	}
}

// broken-link covers §6.2 path fields, not just body links. Positive control:
// a path field to a stale location of a node that exists elsewhere (same
// basename) is a defect. Negative controls: a genuinely unwritten target, a URL,
// a scope descriptor, an asset, and a resolving value stay silent.
func TestConformance_Lint_BrokenLink_Section6_2_PathFieldDefect(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":         v03Index + "\n* [A](a.md)\n* [G](metrics/gm.md)\n* [N](notes.md)\n",
		"metrics/gm.md":    rdoc("GM", "", "g"),
		"notes.md":         rdoc("Notes", "resource: /metrics/gm.md\nsources:\n  - { resource: \"all queries in BigQuery project X\" }\n  - { resource: \"https://x.example/gm.md\" }\n  - { resource: /refs/asset.sql }\n  - { resource: /never-written.md }\n", "n"),
		"a.md":             rdoc("A", "status: deprecated\nsuperseded_by: gm.md\n", "a"),
		"refs/asset.sql":   "select 1\n",
		"metrics/other.md": rdoc("Other", "", "[A](../a.md) [N](../notes.md)"),
	})
	bl := findingsFor(Lint(b, LintOptions{}), "broken-link")
	if len(bl) != 1 {
		t.Fatalf("§6.2: want exactly one broken-link (a.md superseded_by), got %d: %+v", len(bl), bl)
	}
	f := bl[0]
	if f.Path != "a.md" || !strings.Contains(f.Message, "superseded_by") ||
		!strings.Contains(f.Message, "gm.md") || !strings.Contains(f.Message, "did you mean /metrics/gm.md?") {
		t.Fatalf("§6.2: finding must name the field, the value, and the candidate: %+v", f)
	}
}

// §12.1: in an undeclared (v0.2-or-earlier) bundle, a bare path field written
// from the bundle root resolves through the fallback, so broken-link stays
// silent. In a bundle declaring 0.3 the same value is checked file-relative and
// fires (the target exists elsewhere under the same basename).
func TestConformance_Lint_BrokenLink_Section12_1_PathFieldFallback(t *testing.T) {
	files := func(index string) map[string]string {
		return map[string]string{
			"index.md":         index + "\n* [GM](metrics/gm.md)\n* [R](metrics/rev.md)\n",
			"metrics/gm.md":    rdoc("GM", "", "[R](rev.md)"),
			"metrics/rev.md":   rdoc("Rev", "status: deprecated\nsuperseded_by: metrics/gm.md\n", "[GM](gm.md)"),
			"metrics/other.md": rdoc("Other", "", "[GM](gm.md)"),
		}
	}
	legacy := mkResolveBundle(t, files("# KB\n"))
	if bl := findingsFor(Lint(legacy, LintOptions{}), "broken-link"); len(bl) != 0 {
		t.Fatalf("§12.1 undeclared: bare root-relative path field must resolve via fallback; got %+v", bl)
	}
	v03 := mkResolveBundle(t, files(v03Index))
	if bl := findingsFor(Lint(v03, LintOptions{}), "broken-link"); len(bl) != 1 {
		t.Fatalf("§6.2 v0.3: bare path field resolves file-relative only; want 1 broken-link, got %+v", bl)
	}
}

// analyze reports a dangling §6.2 path field as a coverage gap, attributed to
// the field. Negative control: descriptors, URLs, assets, and resolving values
// are not dangling.
func TestConformance_Analyze_Section6_2_DanglingPathFields(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":       v03Index,
		"a.md":           rdoc("A", "resource: /tables/orders.md\nstatus: deprecated\nsuperseded_by: b.md\nsources:\n  - { resource: \"scope of all X\" }\n  - { resource: \"https://x.example/c.md\" }\n  - { resource: /refs/asset.sql }\n  - { resource: b.md }\n", "a"),
		"b.md":           rdoc("B", "", "b"),
		"refs/asset.sql": "select 1\n",
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	var got []string
	for _, d := range rep.Coverage.DanglingLinks {
		got = append(got, d.From+" "+d.Field+" "+d.Target)
	}
	if len(got) != 1 || got[0] != "a.md resource /tables/orders.md" {
		t.Fatalf("§6.2: want only a.md resource /tables/orders.md dangling; got %v", got)
	}
}

// The validate version-conflict warning (§8.1, §12): the root index.md and the
// legacy .okf sidecar disagree; the index wins and validate warns, never fails.
func TestConformance_VersionConflict_Section12_IndexWins(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{"index.md": v03Index, ".okf": "okf_version: 0.2\n"})
	msg, ok := VersionConflict(b)
	if !ok || !strings.Contains(msg, `"0.3"`) || !strings.Contains(msg, `"0.2"`) {
		t.Fatalf("want conflict naming both versions; got %q %v", msg, ok)
	}
	if fs := Validate(b); len(fs) != 0 {
		t.Fatalf("a version conflict is advisory, never a validate failure: %v", fs)
	}
}
