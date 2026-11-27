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

// mkResolveBundle writes files under <tmp>/bundle and loads that directory as
// the bundle root. A key starting with "../" is written OUTSIDE the bundle
// root (beside it), so a test can prove an escaping value is classified
// external even when a real file sits at the escaped location (§6).
func mkResolveBundle(t *testing.T, files map[string]string) *Bundle {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "bundle")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

// v03Index declares okf_version 0.3 on the bundle-root index.md (§8.1, §12),
// which turns OFF the §12.1 bundle-root fallback.
const v03Index = "---\nokf_version: \"0.3\"\n---\n\n# KB\n"

// v02Index declares okf_version 0.2, which permits the §12.1 fallback.
const v02Index = "---\nokf_version: \"0.2\"\n---\n\n# KB\n"

func rdoc(title, extraFM, body string) string {
	return "---\ntype: Concept\ntitle: " + title + "\n" + extraFM + "---\n\n# " + title + "\n\n" + body + "\n"
}

func hasEdge(b *Bundle, from, to string) bool {
	for _, o := range b.OutboundLinks(from) {
		if o == to {
			return true
		}
	}
	return false
}

// §6 rule 3: a link without a leading "/" resolves from the directory of the
// file that contains it — NOT from the bundle root. With both y.md and
// sub/y.md present, sub/x.md's [y](y.md) is sub/y.md.
func TestConformance_Resolve_Section6_LinkResolvesFileRelative(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": v03Index,
		"y.md":     rdoc("Root Y", "", "root"),
		"sub/y.md": rdoc("Sub Y", "", "sub"),
		"sub/x.md": rdoc("X", "", "See [y](y.md)."),
	})
	if !hasEdge(b, "sub/x.md", "sub/y.md") {
		t.Fatalf("§6 rule 3: want edge sub/x.md -> sub/y.md; got %v", b.OutboundLinks("sub/x.md"))
	}
	if hasEdge(b, "sub/x.md", "y.md") {
		t.Fatalf("§6 rule 3: a bare link must NOT resolve from the bundle root first; got %v", b.OutboundLinks("sub/x.md"))
	}
	r := b.ResolveLink("sub/x.md", "y.md")
	if r.Kind != RefBundle || r.Path != "sub/y.md" || r.Form != FormFileRel || !r.Exists {
		t.Fatalf("ResolveLink = %+v, want in-bundle sub/y.md file-relative existing", r)
	}
}

// §6 rule 2: a leading "/" resolves from the bundle root.
func TestConformance_Resolve_Section6_RootAbsolute(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": v03Index,
		"y.md":     rdoc("Root Y", "", "root"),
		"sub/y.md": rdoc("Sub Y", "", "sub"),
		"sub/x.md": rdoc("X", "", "See [y](/y.md)."),
	})
	if !hasEdge(b, "sub/x.md", "y.md") || hasEdge(b, "sub/x.md", "sub/y.md") {
		t.Fatalf("§6 rule 2: want only edge sub/x.md -> y.md; got %v", b.OutboundLinks("sub/x.md"))
	}
	r := b.ResolvePathField("sub/x.md", "/y.md")
	if r.Kind != RefBundle || r.Path != "y.md" || r.Form != FormRootAbs || !r.Exists {
		t.Fatalf("ResolvePathField(/y.md) = %+v", r)
	}
}

// §6 Identifiers: paths are case-sensitive. Foo.md never resolves to foo.md,
// even on a case-insensitive file system (macOS default), because existence is
// decided against the walked names, not a disk stat.
func TestConformance_Resolve_Section6_CaseSensitive(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": v03Index,
		"foo.md":   rdoc("Foo", "", "foo"),
		"a.md":     rdoc("A", "superseded_by: /Foo.md\nstatus: deprecated\n", "See [F](Foo.md)."),
	})
	if hasEdge(b, "a.md", "foo.md") {
		t.Fatalf("§6: Foo.md must not resolve to foo.md; got %v", b.OutboundLinks("a.md"))
	}
	if r := b.ResolveLink("a.md", "Foo.md"); r.Exists {
		t.Fatalf("§6: case-folded link must not exist: %+v", r)
	}
	if r := b.ResolvePathField("a.md", "/Foo.md"); r.Exists {
		t.Fatalf("§6: case-folded path field must not exist: %+v", r)
	}
	// Control: the exact-case spelling does resolve.
	if r := b.ResolveLink("a.md", "foo.md"); !r.Exists || r.Path != "foo.md" {
		t.Fatalf("exact-case control failed: %+v", r)
	}
}

// §6: a value that resolves outside the bundle root is external, and MUST NOT
// be read from the local file system. A real file at the escaped location
// proves the classification does not depend on whether something is there.
func TestConformance_Resolve_Section6_ParentEscapeIsExternal(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":     v03Index,
		"../secret.md": rdoc("Secret", "", "outside the bundle"),
		"a.md":         rdoc("A", "resource: ../secret.md\n", "See [s](../secret.md) and [t](/../secret.md)."),
		"sub/x.md":     rdoc("X", "", "Up [a](../a.md)."),
	})
	for _, v := range []string{"../secret.md", "/../secret.md", "../../x.md"} {
		if r := b.ResolveLink("a.md", v); r.Kind != RefExternal {
			t.Errorf("§6: link %q from a.md = %+v, want RefExternal", v, r)
		}
	}
	if r := b.ResolvePathField("a.md", "../secret.md"); r.Kind != RefExternal {
		t.Errorf("§6: path field ../secret.md = %+v, want RefExternal", r)
	}
	if len(b.OutboundLinks("a.md")) != 0 {
		t.Errorf("§6: external links make no edges; got %v", b.OutboundLinks("a.md"))
	}
	if _, ok := b.Nodes["../secret.md"]; ok {
		t.Errorf("§6/§11.2: an outside file must never be loaded")
	}
	// Control: "../" that stays inside the bundle is an ordinary in-bundle path.
	if !hasEdge(b, "sub/x.md", "a.md") {
		t.Errorf("in-bundle ../ control: want edge sub/x.md -> a.md; got %v", b.OutboundLinks("sub/x.md"))
	}
}

// §6 rule 1: any URI scheme (not just http/https) marks an absolute URL.
func TestConformance_Resolve_Section6_AnySchemeIsURL(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{"index.md": v03Index, "a.md": rdoc("A", "", "x")})
	for _, v := range []string{"https://x.example/a.md", "mailto:a.md", "urn:isbn:123", "ftp://h/a.md"} {
		if r := b.ResolveLink("a.md", v); r.Kind != RefURL {
			t.Errorf("§6 rule 1: %q = %+v, want RefURL", v, r)
		}
	}
}

// §6.1: a link refers to a concept when its target WITHOUT #fragment resolves.
func TestConformance_Resolve_Section6_1_FragmentStripped(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": v03Index,
		"sub/y.md": rdoc("Y", "", "y"),
		"a.md":     rdoc("A", "", "See [y](sub/y.md#section) and [self](#top)."),
	})
	if !hasEdge(b, "a.md", "sub/y.md") {
		t.Fatalf("§6.1: fragment must be stripped before resolution; got %v", b.OutboundLinks("a.md"))
	}
	if r := b.ResolveLink("a.md", "#top"); r.Kind != RefAnchor {
		t.Fatalf("in-page anchor = %+v, want RefAnchor", r)
	}
}

// §6 Identifiers: targets are RFC 3986 URI references, so a percent-encoded
// space resolves to the file whose name has a space.
func TestConformance_Resolve_Section6_PercentDecoded(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":   v03Index,
		"sub/c d.md": rdoc("CD", "", "cd"),
		"a.md":       rdoc("A", "", "See [c](sub/c%20d.md)."),
	})
	if !hasEdge(b, "a.md", "sub/c d.md") {
		t.Fatalf("§6: percent-encoded target must decode; got %v", b.OutboundLinks("a.md"))
	}
}

// §6.1: reference-style links are links.
func TestConformance_Resolve_Section6_1_ReferenceStyleLink(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": v03Index,
		"sub/y.md": rdoc("Y", "", "y"),
		"a.md":     rdoc("A", "", "See [the y][why].\n\n[why]: sub/y.md \"Y\"\n\n[^fn]: a footnote, not a link"),
	})
	if !hasEdge(b, "a.md", "sub/y.md") {
		t.Fatalf("§6.1: reference-style link must make an edge; got %v", b.OutboundLinks("a.md"))
	}
}

// §6.1: [[wikilinks]] are not links (negative control for the scanner).
func TestConformance_Resolve_Section6_1_WikilinkIsNotALink(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": v03Index,
		"y.md":     rdoc("Y", "", "y"),
		"a.md":     rdoc("A", "", "See [[y]] and [[y.md]]."),
	})
	if len(b.OutboundLinks("a.md")) != 0 {
		t.Fatalf("§6.1: wikilinks are not links; got %v", b.OutboundLinks("a.md"))
	}
}

// §6.2 + §13.1: in a bundle that declares 0.3, a bare path field resolves ONLY
// from its file's directory — no bundle-root fallback.
func TestConformance_ResolvePathField_Section6_2_V03HasNoRootFallback(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":    v03Index,
		"refs/run.py": "print(1)\n",
		"sub/x.md":    rdoc("X", "computation: refs/run.py\n", "x"),
	})
	r := b.ResolvePathField("sub/x.md", "refs/run.py")
	if r.Kind != RefBundle || r.Exists || r.Path != "sub/refs/run.py" {
		t.Fatalf("§6.2 v0.3: want unresolved sub/refs/run.py; got %+v", r)
	}
}

// §12.1 / §6.2: a bundle that declares okf_version 0.2 MAY resolve a bare path
// field from the bundle root when it does not resolve from its file.
func TestConformance_ResolvePathField_Section12_1_V02DeclaredFallsBackToRoot(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":    v02Index,
		"refs/run.py": "print(1)\n",
		"sub/x.md":    rdoc("X", "computation: refs/run.py\n", "x"),
	})
	r := b.ResolvePathField("sub/x.md", "refs/run.py")
	if r.Kind != RefBundle || !r.Exists || r.Path != "refs/run.py" || r.Form != FormRootFallback {
		t.Fatalf("§12.1 v0.2: want root-fallback refs/run.py; got %+v", r)
	}
}

// §12.1: a bundle that declares NO version also gets the fallback.
func TestConformance_ResolvePathField_Section12_1_UndeclaredFallsBackToRoot(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md":    "# KB\n",
		"refs/run.py": "print(1)\n",
		"sub/x.md":    rdoc("X", "computation: refs/run.py\n", "x"),
	})
	if b.DeclaredOkfVersion != "" {
		t.Fatalf("undeclared bundle must report DeclaredOkfVersion \"\"; got %q", b.DeclaredOkfVersion)
	}
	r := b.ResolvePathField("sub/x.md", "refs/run.py")
	if !r.Exists || r.Form != FormRootFallback {
		t.Fatalf("§12.1 undeclared: want root fallback; got %+v", r)
	}
}

// The §12.1 fallback keys on the DECLARED version, never on the build default
// (SpecVersion). Simulates the state after the SpecVersion bump: an undeclared
// bundle whose assumed OkfVersion reads "0.3" must still get the fallback.
func TestConformance_ResolvePathField_Section12_1_FallbackIgnoresAssumedVersion(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"refs/run.py": "print(1)\n",
		"sub/x.md":    rdoc("X", "", "x"),
	})
	b.OkfVersion = "0.3" // what an undeclared bundle reads as once SpecVersion is 0.3
	if r := b.ResolvePathField("sub/x.md", "refs/run.py"); !r.Exists || r.Form != FormRootFallback {
		t.Fatalf("§12.1: fallback must survive an assumed 0.3; got %+v", r)
	}
}

// §6.2: even under the fallback, the file-relative target wins when it exists.
func TestConformance_ResolvePathField_Section6_2_FileRelativeWinsOverFallback(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"refs/run.py":     "root\n",
		"sub/refs/run.py": "sub\n",
		"sub/x.md":        rdoc("X", "", "x"),
	})
	r := b.ResolvePathField("sub/x.md", "refs/run.py")
	if r.Path != "sub/refs/run.py" || r.Form != FormFileRel {
		t.Fatalf("§6.2: file-relative must win; got %+v", r)
	}
}

// §12.1 names the bundle-root fallback for path fields. okfctl applies the same
// fallback to body links in a bundle declaring 0.2 or earlier, or no version:
// earlier okfctl resolved bare links root-first and `node mv` preserved that
// form, so a legacy bundle's root-relative bare links keep resolving. The
// fallback never rejects anything, and the file-relative target always wins.
func TestConformance_ResolveLink_Section12_1_LegacyLinkFallsBackToRoot(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"z/y.md":   rdoc("Y", "", "y"),
		"sub/x.md": rdoc("X", "", "See [y](z/y.md)."),
	})
	r := b.ResolveLink("sub/x.md", "z/y.md")
	if !r.Exists || r.Path != "z/y.md" || r.Form != FormRootFallback {
		t.Fatalf("§12.1 legacy link: want root fallback z/y.md; got %+v", r)
	}
	if !hasEdge(b, "sub/x.md", "z/y.md") {
		t.Fatalf("§12.1 legacy link: want edge; got %v", b.OutboundLinks("sub/x.md"))
	}
}

// Negative control: a bundle declaring 0.3 gets no link fallback (§6 rule 3).
func TestConformance_ResolveLink_Section6_V03LinkHasNoRootFallback(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": v03Index,
		"z/y.md":   rdoc("Y", "", "y"),
		"sub/x.md": rdoc("X", "", "See [y](z/y.md)."),
	})
	if r := b.ResolveLink("sub/x.md", "z/y.md"); r.Exists || r.Path != "sub/z/y.md" {
		t.Fatalf("§6 v0.3: want unresolved sub/z/y.md; got %+v", r)
	}
	if len(b.OutboundLinks("sub/x.md")) != 0 {
		t.Fatalf("§6 v0.3: no edge; got %v", b.OutboundLinks("sub/x.md"))
	}
}

// §5.1 / §6.2: a sources[].resource can be a scope descriptor, which is not a
// path. A value with whitespace cannot be a URI reference (§6 Identifiers
// require percent-encoding), so it is a descriptor: never resolved.
func TestConformance_ResolvePathField_Section5_1_ScopeDescriptor(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{"a.md": rdoc("A", "", "x")})
	for _, v := range []string{
		"all queries in BigQuery project X",
		"pingdotgg/lawn @ 9163eac (2026-10-20) cloned and read",
		"RFC 6749 §1.3/§4/§6; RFC 8628",
	} {
		if r := b.ResolvePathField("a.md", v); r.Kind != RefDescriptor {
			t.Errorf("§5.1: %q = %+v, want RefDescriptor", v, r)
		}
	}
	// §11.2 rule 1: empty is the same as absent.
	if r := b.ResolvePathField("a.md", "  "); r.Kind != RefEmpty {
		t.Errorf("§11.2: blank = %+v, want RefEmpty", r)
	}
}

// §6.2 lists the path fields; PathFields extracts every one of them, including
// a single-mapping sources entry (§11.2 rule 2).
func TestConformance_NodePathFields_Section6_2(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"a.md": rdoc("A", strings.Join([]string{
			"resource: /tables/orders.md",
			"computation: /references/q.sql",
			"executor: { resource: /references/run.md, receipt: [job_id] }",
			"attester: { resource: /references/att.py }",
			"status: deprecated",
			"superseded_by: /b.md",
			"sources:",
			"  - { id: s1, resource: c.md }",
			"  - { id: s2, resource: \"https://x.example\" }",
			"",
		}, "\n"), "x"),
		"m.md": rdoc("M", "sources: { id: s, resource: d.md }\n", "x"),
	})
	got := map[string]string{}
	for _, f := range b.Nodes["a.md"].PathFields() {
		got[f.Field] = f.Value
	}
	want := map[string]string{
		"resource":            "/tables/orders.md",
		"computation":         "/references/q.sql",
		"executor.resource":   "/references/run.md",
		"attester.resource":   "/references/att.py",
		"superseded_by":       "/b.md",
		"sources[0].resource": "c.md",
		"sources[1].resource": "https://x.example",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("PathFields[%s] = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("PathFields = %v, want exactly %v", got, want)
	}
	mf := b.Nodes["m.md"].PathFields()
	if len(mf) != 1 || mf[0].Field != "sources[0].resource" || mf[0].Value != "d.md" {
		t.Errorf("§11.2 rule 2: single-mapping sources = %+v", mf)
	}
}

// §8.1 / §12: okf_version is declared on the bundle-root index.md. The .okf
// sidecar is a legacy fallback only when the index declares nothing; the index
// wins a conflict, and the conflict is reported.
func TestConformance_BundleVersion_Section8_1_DeclarationSources(t *testing.T) {
	cases := []struct {
		name         string
		files        map[string]string
		wantDeclared string
		wantSource   string
		wantConflict bool
	}{
		{"index only", map[string]string{"index.md": v03Index}, "0.3", VersionFromIndex, false},
		{"sidecar only", map[string]string{".okf": "okf_version: 0.1\n"}, "0.1", VersionFromSidecar, false},
		{"sidecar quoted", map[string]string{".okf": "okf_version: \"0.2\"\n"}, "0.2", VersionFromSidecar, false},
		{"both agree", map[string]string{"index.md": v02Index, ".okf": "okf_version: 0.2\n"}, "0.2", VersionFromIndex, false},
		{"both disagree", map[string]string{"index.md": v03Index, ".okf": "okf_version: 0.2\n"}, "0.3", VersionFromIndex, true},
		{"undeclared", map[string]string{"index.md": "# KB\n"}, "", VersionUndeclared, false},
		{"unquoted index", map[string]string{"index.md": "---\nokf_version: 0.2\n---\n\n# KB\n"}, "0.2", VersionFromIndex, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := mkResolveBundle(t, c.files)
			if b.DeclaredOkfVersion != c.wantDeclared || b.OkfVersionSource != c.wantSource {
				t.Fatalf("declared=%q source=%q, want %q/%q", b.DeclaredOkfVersion, b.OkfVersionSource, c.wantDeclared, c.wantSource)
			}
			want := c.wantDeclared
			if want == "" {
				want = SpecVersion
			}
			if b.OkfVersion != want {
				t.Fatalf("OkfVersion = %q, want %q", b.OkfVersion, want)
			}
			_, conflict := VersionConflict(b)
			if conflict != c.wantConflict {
				t.Fatalf("VersionConflict = %v, want %v", conflict, c.wantConflict)
			}
		})
	}
}

// AGENTS.md conformance layer 4: a v0.1 bundle (declared in the legacy .okf
// sidecar, legacy `timestamp`, body `# Citations`) keeps resolving under the §6
// resolver: it gets the §12.1 fallback, its legacy fallbacks still read, and
// its root-relative bare links keep their edges.
func TestConformance_Resolve_Section12_1_V01BundleStillConsumable(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "# KB\n\n* [T](wine/tannin.md)\n",
		"wine/tannin.md": "---\ntype: Concept\ntitle: Tannin\ntimestamp: 2026-01-02T03:04:05Z\nresource: wine/acidity.md\n---\n\n" +
			"See [acidity](wine/acidity.md).\n\n# Citations\n\n- [1] https://example.com/tannin\n",
		"wine/acidity.md": rdoc("Acidity", "", "a"),
	})
	if b.DeclaredOkfVersion != "0.1" || b.OkfVersionSource != VersionFromSidecar {
		t.Fatalf("v0.1 sidecar declaration: got %q from %q", b.DeclaredOkfVersion, b.OkfVersionSource)
	}
	if !hasEdge(b, "wine/tannin.md", "wine/acidity.md") {
		t.Fatalf("§12.1 v0.1: root-relative bare link must keep its edge; got %v", b.OutboundLinks("wine/tannin.md"))
	}
	if r := b.ResolvePathField("wine/tannin.md", "wine/acidity.md"); r.Form != FormRootFallback || !r.Exists {
		t.Fatalf("§12.1 v0.1: bare path field must resolve via fallback; got %+v", r)
	}
	n := b.Nodes["wine/tannin.md"]
	if g, ok := n.Generated(); !ok || g.At.IsZero() {
		t.Fatalf("§12.1/Appendix B: legacy timestamp must still read as generated.at")
	}
	if n.SourceCitations() != 1 {
		t.Fatalf("§12.1/Appendix B: legacy # Citations must still count; got %d", n.SourceCitations())
	}
	if bl := findingsFor(Lint(b, LintOptions{}), "broken-link"); len(bl) != 0 {
		t.Fatalf("v0.1 bundle must not gain broken-link findings: %+v", bl)
	}
	if fs := Validate(b); len(fs) != 0 {
		t.Fatalf("v0.1 bundle must validate: %v", fs)
	}
}

// §12: legacyVersion decides the §12.1 fallback. A future or unparseable
// version is not "0.2 or earlier", so it gets no fallback; it is never refused.
func TestConformance_LegacyVersion_Section12_1(t *testing.T) {
	for v, want := range map[string]bool{
		"": true, "0.1": true, "0.2": true, "v0.2": true, "0.2.0": true,
		"0.3": false, "1.0": false, "9.9": false, "banana": false,
	} {
		if got := legacyVersion(v); got != want {
			t.Errorf("legacyVersion(%q) = %v, want %v", v, got, want)
		}
	}
}
