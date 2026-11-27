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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// node mv rewrites every inbound link through the shared §6 resolver. A
// file-relative link (§6 rule 3) is recomputed from the linking file, a
// "/"-rooted one (§6 rule 2) stays "/"-rooted, a #fragment and a title
// survive, and a reference-style definition is rewritten in place.
func TestConformance_PlanMove_Section6_FormsFragmentsAndReferenceLinks(t *testing.T) {
	root, b := mkMoveBundle(t, map[string]string{
		"index.md":    v03Index,
		"wine/foo.md": nodeSrc("Foo"),
		"wine/a.md":   nodeSrc("A") + "See [x](foo.md#tannin \"T\").\n",
		"red/b.md":    nodeSrc("B") + "Abs [x](/wine/foo.md).\n",
		"c.md":        nodeSrc("C") + "Ref [x][f].\n\n[f]: wine/foo.md\n",
	})
	rw, err := PlanMove(b, "wine/foo.md", "cellar/foo.md")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"wine/a.md": `../cellar/foo.md#tannin "T"`,
		"red/b.md":  "/cellar/foo.md",
		"c.md":      "cellar/foo.md",
	}
	for node, w := range want {
		r, ok := findRewrite(rw, node)
		if !ok || r.New != w {
			t.Errorf("%s: rewrite = %+v, want New=%q", node, r, w)
		}
	}
	if err := ApplyMove(root, b, "wine/foo.md", "cellar/foo.md", rw); err != nil {
		t.Fatal(err)
	}
	nb, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for node := range want {
		if !hasEdge(nb, node, "cellar/foo.md") {
			t.Errorf("after move %s must link cellar/foo.md; got %v", node, nb.OutboundLinks(node))
		}
	}
	if fs := Validate(nb); len(fs) != 0 {
		t.Errorf("moved bundle must still validate: %v", fs)
	}
}

// Negative control for move: in a bundle declaring 0.3, a bare link written
// from the bundle root does not resolve (§6 rule 3), so it is not an inbound
// link and is left untouched.
func TestConformance_PlanMove_Section6_V03RootRelativeBareLinkIsNotInbound(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"index.md":    v03Index,
		"wine/foo.md": nodeSrc("Foo"),
		"red/b.md":    nodeSrc("B") + "See [x](wine/foo.md).\n",
	})
	rw, err := PlanMove(b, "wine/foo.md", "cellar/foo.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(rw) != 0 {
		t.Fatalf("§6 v0.3: an unresolved bare link is not inbound; got %+v", rw)
	}
}

// §12.1: in an undeclared bundle, a bare link written from the bundle root
// resolves via the fallback and keeps its root-relative spelling on move.
func TestConformance_PlanMove_Section12_1_LegacyRootRelativeLinkRewritten(t *testing.T) {
	_, b := mkMoveBundle(t, map[string]string{
		"wine/foo.md": nodeSrc("Foo"),
		"red/b.md":    nodeSrc("B") + "See [x](wine/foo.md).\n",
	})
	rw, err := PlanMove(b, "wine/foo.md", "cellar/foo.md")
	if err != nil {
		t.Fatal(err)
	}
	r, ok := findRewrite(rw, "red/b.md")
	if !ok || r.New != "cellar/foo.md" {
		t.Fatalf("§12.1: legacy root-relative link must be rewritten root-relative; got %+v", rw)
	}
}

// eval citation-unresolved resolves sources[].resource with the shared
// resolver. Positive: a file-relative .md source that is missing is flagged.
// Negative: a file-relative source that exists, a scope descriptor (§5.1), and
// an external ../ value are not.
func TestConformance_EvalCitations_Section6_2_FileRelativeSources(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":      v03Index,
		"metrics/gm.md": rdoc("GM", "epistemic: verified\nsources:\n  - { resource: rev.md }\n", "g"),
		"metrics/rev.md": rdoc("Rev", "epistemic: verified\nsources:\n  - { resource: gm.md }\n"+
			"  - { resource: \"all queries in project X.md\" }\n  - { resource: ../../outside.md }\n", "r"),
		"metrics/bad.md": rdoc("Bad", "epistemic: verified\nsources:\n  - { resource: nope.md }\n", "b"),
	})
	var got []string
	for _, f := range EvalTransparency(b, EvalOptions{}) {
		if f.Check == "citation-unresolved" {
			got = append(got, f.Path)
		}
	}
	if len(got) != 1 || got[0] != "metrics/bad.md" {
		t.Fatalf("citation-unresolved = %v, want only metrics/bad.md", got)
	}
}

// node promote routes directory links through the shared resolver: a
// file-relative "../foo/" link resolves from the linking file, and the rewrite
// keeps the author's form.
func TestConformance_PromotePlan_Section6_FileRelativeDirLink(t *testing.T) {
	_, b := mkPromoteBundle(t, map[string]string{
		"index.md":     v03Index + "\n* [Foo](foo/)\n",
		"foo/index.md": dirConceptIndex("Foo", "2026-01-01", "# Foo\n\nBody.\n"),
		"baz/qux.md":   nodeSrc("Qux") + "Up [foo](../foo/) and [abs](/foo/index.md#top).\n",
	})
	plan, err := PromotePlan(b, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range plan[0].Rewrites {
		got[r.NodePath+" "+r.New] = true
	}
	for _, w := range []string{"baz/qux.md ../foo/foo.md", "baz/qux.md /foo/foo.md#top", "index.md foo/foo.md"} {
		if !got[w] {
			t.Errorf("promote rewrite %q missing; got %v", w, got)
		}
	}
}

// Writers preserve frontmatter: a move rewrites link text only, so a path
// field in the moved-into node's frontmatter is byte-identical afterwards.
func TestConformance_ApplyMove_PathFieldFrontmatterUntouched(t *testing.T) {
	root, b := mkMoveBundle(t, map[string]string{
		"wine/foo.md": nodeSrc("Foo"),
		"a.md":        "---\ntype: Concept\ntitle: A\nresource: \"/wine/foo.md\"\n---\nSee [x](wine/foo.md).\n",
	})
	rw, err := PlanMove(b, "wine/foo.md", "cellar/foo.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyMove(root, b, "wine/foo.md", "cellar/foo.md", rw); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "a.md"))
	if !strings.Contains(string(raw), "resource: \"/wine/foo.md\"\n") {
		t.Fatalf("frontmatter must be preserved verbatim; got:\n%s", raw)
	}
}
