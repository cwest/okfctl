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
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// OKF v0.3 §3.2 ("Concepts, assets, and other files") is the rule a consumer
// uses to find the concepts in a bundle:
//
//  1. A non-reserved `.md` with frontmatter carrying a non-empty `type` is a
//     concept.
//  2. A `.md` with no frontmatter, or no `type`, is not a concept: consumers skip
//     it, it does not affect conformance, and validators SHOULD warn.
//  3. A file not ending in `.md` is an asset, never a concept.
//  4. Files and directories whose names start with `.` are not part of the
//     bundle.
//
// §11.1 keeps one failure for `.md` files: a frontmatter block that does not
// parse as a YAML mapping. §11.2 rule 1 makes an empty-string `type` the same as
// an absent one.

func TestConformance_V03S32_ClassifyPath(t *testing.T) {
	cases := []struct {
		rel  string
		want FileKind
	}{
		{"a.md", KindMarkdown},
		{"sub/README.md", KindMarkdown},
		{"index.md", KindReserved},
		{"wine/log.md", KindReserved},
		{"design/tokens.json", KindAsset},
		{"img/diagram.png", KindAsset},
		{"Makefile", KindAsset},
		{".draft.md", KindExcluded},
		{".hidden/x.md", KindExcluded},
		{"sub/.notes/y.md", KindExcluded},
		{".hidden/index.md", KindExcluded}, // rule 4 beats §3.1: not in the bundle at all
		{"sub/.okf-drift-ignore-revs", KindExcluded},
		{"a.b.md", KindMarkdown}, // an interior dot is not a dot-path
		{"v0.3/notes.md", KindMarkdown},
	}
	for _, c := range cases {
		if got := ClassifyPath(c.rel); got != c.want {
			t.Errorf("ClassifyPath(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}

func TestConformance_V03S32_ClassifyMarkdown(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want FileKind
	}{
		{"typed concept", "---\ntype: Reference\n---\n# A\n", KindConcept},
		{"unknown type is still a concept (§4.1)", "---\ntype: Wibble\n---\n", KindConcept},
		{"no frontmatter (§3.2 rule 2)", "# Read me\n", KindNonConcept},
		{"frontmatter without type (§3.2 rule 2)", "---\ntitle: T\n---\n# T\n", KindNonConcept},
		{"empty frontmatter block", "---\n---\n# T\n", KindNonConcept},
		{"empty-string type (§11.2 rule 1)", "---\ntype: \"\"\n---\n", KindNonConcept},
		{"whitespace type (§11.2 rule 1)", "---\ntype: \"   \"\n---\n", KindNonConcept},
		{"null type (§11.2 rule 1)", "---\ntype:\n---\n", KindNonConcept},
		{"unparseable block (§11.1 rule 1)", "---\ntype: [\n---\n", KindMalformed},
		{"non-mapping block (§11.1 rule 1)", "---\n- a\n- b\n---\n", KindMalformed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClassifyMarkdown([]byte(c.src)).Kind; got != c.want {
				t.Errorf("ClassifyMarkdown(%q) = %v, want %v", c.src, got, c.want)
			}
		})
	}
}

// typelessBundle is the §3.2 rule 2 fixture: a typed concept beside three
// type-less `.md` files (a sub-dir README.md with no frontmatter, a title-only
// file, and an empty-string type). The concept links to the README so the
// dangling-link controls have something to resolve.
func typelessBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# KB\n\n- [Tannin](wine/tannin.md)\n")
	writeFile(t, dir, "log.md", "# Log\n")
	writeFile(t, dir, "wine/tannin.md", "---\ntype: Reference\ntitle: Tannin\n---\n# Tannin\n\nSee the [readme](README.md) and [missing](nowhere.md).\n")
	writeFile(t, dir, "wine/README.md", "# Wine notes\n\nHow this folder is organized. See [tannin](tannin.md).\n")
	writeFile(t, dir, "notes.md", "---\ntitle: Notes\n---\n# Notes\n")
	writeFile(t, dir, "blank.md", "---\ntype: \"\"\n---\n# Blank\n")
	return dir
}

func TestConformance_V03S32_TypelessMDIsNotAConcept(t *testing.T) {
	b, err := Load(typelessBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedKeys(b.Nodes); strings.Join(got, ",") != "wine/tannin.md" {
		t.Errorf("Nodes = %v, want only the typed concept wine/tannin.md", got)
	}
	want := []string{"blank.md", "notes.md", "wine/README.md"}
	if got := sortedKeys(b.NonConcepts); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("NonConcepts = %v, want %v", got, want)
	}
}

func TestConformance_V03S32_TypelessMDDoesNotFailValidate(t *testing.T) {
	b, err := Load(typelessBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	if f := Validate(b); len(f) != 0 {
		t.Errorf("§3.2/§11.1: a type-less .md must not affect conformance; got findings %v", f)
	}
}

func TestConformance_V03S32_TypelessMDWarns(t *testing.T) {
	b, err := Load(typelessBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	w := ValidateWarnings(b)
	var paths []string
	for _, f := range w {
		paths = append(paths, f.Path)
		if !strings.Contains(f.Message, "§3.2") {
			t.Errorf("warning for %s must cite §3.2: %q", f.Path, f.Message)
		}
	}
	want := "blank.md,notes.md,wine/README.md"
	if strings.Join(paths, ",") != want {
		t.Errorf("warnings for %v, want %s (sorted)", paths, want)
	}
	// The reason tells the curator what to add.
	for _, f := range w {
		switch f.Path {
		case "wine/README.md":
			if !strings.Contains(f.Message, "no frontmatter") {
				t.Errorf("README warning should say no frontmatter: %q", f.Message)
			}
		case "notes.md", "blank.md":
			if !strings.Contains(f.Message, "type") {
				t.Errorf("%s warning should name the missing type: %q", f.Path, f.Message)
			}
		}
	}
}

func TestConformance_V03S32_TypelessMDExcludedFromGraphLintIndex(t *testing.T) {
	b, err := Load(typelessBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range BuildGraph(b).Nodes {
		if n.Path != "wine/tannin.md" {
			t.Errorf("graph node %s is not a concept (§3.2)", n.Path)
		}
	}
	for _, f := range Lint(b, LintOptions{}) {
		if f.Path == "wine/README.md" || f.Path == "notes.md" || f.Path == "blank.md" {
			t.Errorf("lint must skip non-concepts (§3.2); got %+v", f)
		}
	}
	idx := RenderIndex(b)
	if strings.Contains(idx, "README") || strings.Contains(idx, "notes.md") || strings.Contains(idx, "blank.md") {
		t.Errorf("index must list concepts only (§3.2); got:\n%s", idx)
	}
}

// A link to a non-concept file is valid (§6.1) and does not refer to a concept;
// it must not be reported as a dangling concept reference. Control: a link to a
// file that does not exist still is.
func TestConformance_V03S32_LinkToNonConceptIsNotDangling(t *testing.T) {
	b, err := Load(typelessBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	got := danglingTargets(b, "wine/tannin.md", b.Nodes["wine/tannin.md"])
	if strings.Join(got, ",") != "nowhere.md" {
		t.Errorf("dangling = %v, want only nowhere.md (README.md is a non-concept file, §6.1)", got)
	}
}

// Negative control: §3.2 classifies by frontmatter, never by filename. A
// README.md that carries a type IS a concept.
func TestConformance_V03S32_TypedReadmeIsAConcept(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "---\ntype: Guide\n---\n# Guide\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Nodes["README.md"] == nil {
		t.Errorf("a typed README.md is a concept (§3.2 rule 1); Nodes = %v", sortedKeys(b.Nodes))
	}
	if len(ValidateWarnings(b)) != 0 {
		t.Errorf("a typed concept must not warn; got %v", ValidateWarnings(b))
	}
}

// Negative control: §11.1 rule 1 still fails a `.md` whose frontmatter block
// does not parse. Rule 2's relaxation is for absent frontmatter / absent type,
// not for broken YAML.
func TestConformance_V03S32_UnparseableFrontmatterStillFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "bad.md", "---\ntype: [\n---\n# Bad\n")
	writeFile(t, dir, "ok.md", "---\ntype: Reference\n---\n# OK\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	f := Validate(b)
	if len(f) != 1 || f[0].Path != "bad.md" {
		t.Errorf("§11.1 rule 1: want exactly one finding for bad.md, got %v", f)
	}
}

// Negative control: reserved files keep their own rules (§3.1). A type-less
// index.md / log.md is neither a concept nor a non-concept warning.
func TestConformance_V03S32_ReservedFilesUnaffected(t *testing.T) {
	b, err := Load(typelessBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	if b.Reserved["index.md"] == nil || b.Reserved["log.md"] == nil {
		t.Errorf("reserved files must stay reserved; Reserved = %v", sortedKeys(b.Reserved))
	}
	for _, f := range ValidateWarnings(b) {
		if IsReservedPath(f.Path) {
			t.Errorf("reserved file must not get a §3.2 non-concept warning: %+v", f)
		}
	}
}

// dotPathBundle is the §3.2 rule 4 fixture. Every dot-path file is written so
// that, if it were loaded, it would show up: `.draft.md` is a typed concept,
// `.hidden/x.md` has broken YAML (would fail validate), `.hidden/index.md` has a
// forbidden frontmatter block (would fail validate), and `wine/.notes/y.md` is
// typed. The typed concept wine/tannin.md sits beside them.
func dotPathBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# KB\n\n- [Tannin](wine/tannin.md)\n")
	writeFile(t, dir, "wine/tannin.md", "---\ntype: Reference\ntitle: Tannin\n---\n# Tannin\n")
	writeFile(t, dir, ".draft.md", "---\ntype: Reference\ntitle: Draft\n---\n# Draft\n")
	writeFile(t, dir, ".hidden/x.md", "---\ntype: [\n---\n# broken\n")
	writeFile(t, dir, ".hidden/index.md", "---\ntype: Index\n---\n# Hidden\n")
	writeFile(t, dir, "wine/.notes/y.md", "---\ntype: Reference\n---\n# Y\n")
	writeFile(t, dir, "wine/.scratch.md", "# scratch, no frontmatter\n")
	return dir
}

func TestConformance_V03S32_DotPathsAreNotPartOfTheBundle(t *testing.T) {
	for _, opts := range [][]LoadOption{nil, {WithNoIgnore()}} {
		b, err := Load(dotPathBundle(t), opts...)
		if err != nil {
			t.Fatal(err)
		}
		if got := sortedKeys(b.Nodes); strings.Join(got, ",") != "wine/tannin.md" {
			t.Errorf("noIgnore=%v: Nodes = %v, want only wine/tannin.md", len(opts) > 0, got)
		}
		if got := sortedKeys(b.Reserved); strings.Join(got, ",") != "index.md" {
			t.Errorf("noIgnore=%v: Reserved = %v, want only index.md", len(opts) > 0, got)
		}
		if len(b.NonConcepts) != 0 {
			t.Errorf("noIgnore=%v: dot-paths must not become non-concepts either: %v", len(opts) > 0, sortedKeys(b.NonConcepts))
		}
		if f := Validate(b); len(f) != 0 {
			t.Errorf("noIgnore=%v: dot-paths must not affect validate; got %v", len(opts) > 0, f)
		}
		if w := ValidateWarnings(b); len(w) != 0 {
			t.Errorf("noIgnore=%v: dot-paths must not warn; got %v", len(opts) > 0, w)
		}
		// Rule 4 is the spec, not the tool's skip list: it is not announced as
		// a recoverable skip.
		for _, s := range b.SkippedDirs {
			if strings.HasPrefix(filepath.Base(s), ".") {
				t.Errorf("dot dir %s must not be listed as a --no-ignore-recoverable skip", s)
			}
		}
	}
}

// Negative control: the bundle ROOT is never a dot-path, even when its own name
// starts with "." — rule 4 is about names under the root.
func TestConformance_V03S32_DotNamedRootStillLoads(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, ".kb")
	writeFile(t, root, "a.md", "---\ntype: Reference\n---\n# A\n")
	b, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if b.Nodes["a.md"] == nil {
		t.Errorf("a dot-named bundle root must still load; Nodes = %v", sortedKeys(b.Nodes))
	}
}

// Producer side of rule 4: okfctl must not author or move a concept to a path
// the bundle does not contain.
func TestConformance_V03S32_NoAuthoringAtDotPath(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewNode(dir, ".drafts/x", "Reference", "X"); err == nil {
		t.Error("NewNode at a dot-path must be refused (§3.2 rule 4)")
	}
	if _, err := NewNode(dir, "wine/x", "Reference", "X"); err != nil {
		t.Errorf("control: NewNode at a normal path must succeed: %v", err)
	}
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PlanMove(b, "wine/x.md", ".attic/x.md"); err == nil {
		t.Error("PlanMove to a dot-path must be refused (§3.2 rule 4)")
	}
	if err := ApplyMove(dir, b, "wine/x.md", ".attic/x.md", nil); err == nil {
		t.Error("ApplyMove to a dot-path must be refused (§3.2 rule 4)")
	}
	if _, err := PlanMove(b, "wine/x.md", "red/x.md"); err != nil {
		t.Errorf("control: PlanMove to a normal path must succeed: %v", err)
	}
}

func sortedKeys(m map[string]*Node) []string {
	out := keys(m)
	sort.Strings(out)
	return out
}

// A path field may target any in-bundle file (§6.2, §3.2 rule 3), so a source
// whose resource is a non-concept .md resolves. Control: a source naming a file
// that does not exist is still citation-unresolved.
func TestConformance_V03S32_SourceToNonConceptResolves(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# About\n")
	writeFile(t, dir, "a.md", "---\ntype: Reference\nsources:\n  - resource: /README.md\n  - resource: /gone.md\n---\n# A\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var unresolved []string
	for _, f := range EvalTransparency(b, EvalOptions{}) {
		if f.Check == "citation-unresolved" {
			unresolved = append(unresolved, f.Message)
		}
	}
	if len(unresolved) != 1 || !strings.Contains(unresolved[0], "/gone.md") {
		t.Errorf("want exactly one citation-unresolved for /gone.md, got %v", unresolved)
	}
}

// A non-concept is skipped as a concept, but it is still a file in the bundle:
// moving a concept must not silently overwrite it (dry-run plan included), and
// a link inside it that points at the moved concept is rewritten so it keeps
// resolving (okfctl's writers never break an author's link).
func TestConformance_V03S32_MoveRespectsNonConcepts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "wine/tannin.md", "---\ntype: Reference\n---\n# Tannin\n")
	writeFile(t, dir, "wine/README.md", "# Wine\n\nStart with [tannin](tannin.md).\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PlanMove(b, "wine/tannin.md", "wine/README.md"); err == nil {
		t.Error("PlanMove onto an existing non-concept .md must be refused")
	}
	rws, err := PlanMove(b, "wine/tannin.md", "red/tannin.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(rws) != 1 || rws[0].NodePath != "wine/README.md" || rws[0].New != "../red/tannin.md" {
		t.Fatalf("want one rewrite of wine/README.md -> ../red/tannin.md, got %+v", rws)
	}
	if err := ApplyMove(dir, b, "wine/tannin.md", "red/tannin.md", rws); err != nil {
		t.Fatalf("ApplyMove: %v", err)
	}
	b2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := danglingTargets(b2, "wine/README.md", b2.NonConcepts["wine/README.md"]); len(got) != 0 {
		t.Errorf("README link must still resolve after the move, dangling = %v", got)
	}
	if b2.NonConcepts["wine/README.md"] == nil || len(b2.NonConcepts["wine/README.md"].Frontmatter) != 0 {
		t.Errorf("README must stay a frontmatter-less non-concept after its link is rewritten")
	}
}

func TestConformance_V03S32_PromoteRefusesNonConceptDestination(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "# KB\n")
	writeFile(t, dir, "gke/index.md", "---\ntype: Map\n---\n# GKE\n")
	writeFile(t, dir, "gke/gke.md", "# notes, no frontmatter\n")
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PromotePlan(b, ""); err == nil {
		t.Error("PromotePlan onto an existing non-concept .md must be refused")
	}
}
