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

import "testing"

// These tests pin the prose seam: lint/analyze prose scans must read what a
// reader SEES (link text, running prose) and never the parts a reader does not
// (link destinations, autolink URLs, code spans). The graph/dangling passes,
// which legitimately need link targets, are guarded separately below.

// Repro 1 (upstream #33): a node whose body links a vendor URL whose PATH
// happens to contain another node's title, and which never writes that title in
// prose, must NOT raise a missing-xref. The target node is genuinely unlinked
// (a URL is not an in-bundle link), so it is an orphan — that finding must stand.
func TestLint_MissingXref_IgnoresTitleInsideLinkURL(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":   "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [One](one.md)\n",
		"one.md":     lintDoc("Concept", "One", "See the [vendor page](https://example.com/charlie) for details."),
		"charlie.md": lintDoc("Concept", "Charlie", "Charlie is a concept node."),
	})
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "one.md" {
			t.Fatalf("\"Charlie\" appears only inside a link URL, never in prose; one.md must not get a missing-xref: %+v", f)
		}
	}
	// The orphan finding for the genuinely-unlinked target must remain.
	found := false
	for _, f := range findingsFor(Lint(b, LintOptions{}), "orphan") {
		if f.Path == "charlie.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("charlie.md has no inbound in-bundle link (a URL is not a link), so it must be reported orphan")
	}
}

// Positive control: link TEXT is prose a reader sees. A node whose VISIBLE link
// label names another node's title, without linking to that node, is a real
// missing-xref and must still be reported.
func TestLint_MissingXref_LinkTextStillCountsAsProse(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":   "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [One](one.md)\n- [Charlie](charlie.md)\n",
		"one.md":     lintDoc("Concept", "One", "See the [Charlie](https://example.com/vendor) writeup elsewhere."),
		"charlie.md": lintDoc("Concept", "Charlie", "Charlie is a concept node."),
	})
	found := false
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "one.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the visible link label \"Charlie\" is prose a reader sees; one.md must get a missing-xref to charlie.md")
	}
}

// Reference-style variant of repro 1: the title appears only inside a
// reference-link DEFINITION URL (`[ref]: https://.../charlie`). The definition
// line is not prose a reader sees, so one.md must NOT raise a missing-xref; the
// genuinely-unlinked target stays an orphan.
func TestLint_MissingXref_IgnoresTitleInsideRefLinkURL(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":   "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [One](one.md)\n",
		"one.md":     lintDoc("Concept", "One", "See the [vendor page][vp] for details.\n\n[vp]: https://example.com/charlie"),
		"charlie.md": lintDoc("Concept", "Charlie", "Charlie is a concept node."),
	})
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "one.md" {
			t.Fatalf("\"Charlie\" appears only inside a reference-link definition URL, never in prose; one.md must not get a missing-xref: %+v", f)
		}
	}
	found := false
	for _, f := range findingsFor(Lint(b, LintOptions{}), "orphan") {
		if f.Path == "charlie.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("charlie.md has no inbound in-bundle link (a URL is not a link), so it must be reported orphan")
	}
}

// Positive control for the reference-style variant: the VISIBLE label of a
// reference-style link is prose a reader sees. A node whose visible label names
// another node's title, without linking to it in-bundle, is a real missing-xref
// and must still be reported.
func TestLint_MissingXref_RefLinkTextStillCountsAsProse(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":   "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [One](one.md)\n- [Charlie](charlie.md)\n",
		"one.md":     lintDoc("Concept", "One", "See the [Charlie][ext] writeup elsewhere.\n\n[ext]: https://example.com/vendor"),
		"charlie.md": lintDoc("Concept", "Charlie", "Charlie is a concept node."),
	})
	found := false
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "one.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the visible reference-link label \"Charlie\" is prose a reader sees; one.md must get a missing-xref to charlie.md")
	}
}

// Prose-footnote guard for missing-xref: a line beginning `[n]:` followed by
// RUNNING PROSE is a footnote a reader sees, not a link-reference definition
// (whose destination is a single link-destination token). A node title named
// only in such a footnote, without an in-bundle link, is a real missing-xref
// and must still be reported — the ref-definition strip must not swallow it.
func TestLint_MissingXref_FootnoteProseStillCountsAsProse(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":   "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [One](one.md)\n- [Charlie](charlie.md)\n",
		"one.md":     lintDoc("Concept", "One", "See the notes below.\n\n[9]: Charlie provides the counterpoint on consensus."),
		"charlie.md": lintDoc("Concept", "Charlie", "Charlie is a concept node."),
	})
	found := false
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "one.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("\"Charlie\" in a `[9]: ...` prose footnote is text a reader sees; one.md must get a missing-xref to charlie.md")
	}
}

// Prose-footnote guard for the time-sensitive path: a marker inside a `[n]:`
// prose footnote is prose a reader sees and must still surface. The
// destination-constrained ref-definition strip must not drop the whole line.
func TestAnalyze_TimeSensitive_FootnoteProseMarkersStillReported(t *testing.T) {
	pinClock(t, fixedNow)
	doc := tsDoc("Concept", "A", nil, "", "", "See the notes below.\n\n[9]: the latest pricing here is deprecated now.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.TimeSensitive) != 1 || rep.Freshness.TimeSensitive[0].Path != "a.md" {
		t.Fatalf("time-sensitive markers in a `[9]: ...` prose footnote must still surface, got %+v", rep.Freshness.TimeSensitive)
	}
	if len(rep.Freshness.TimeSensitive[0].Markers) == 0 {
		t.Fatalf("want markers recorded for genuinely time-sensitive footnote prose, got none")
	}
}

// Repro 2 (upstream #33): a time-sensitive marker that appears only inside a
// link URL path must NOT surface as a time_sensitive finding. The node's prose
// is otherwise settled, so freshness.time_sensitive must be empty for it.
func TestAnalyze_TimeSensitive_IgnoresMarkerInsideLinkURL(t *testing.T) {
	pinClock(t, fixedNow)
	// Undated so the age gate never suppresses a real marker — this isolates the
	// question "was a marker found in prose?" from the age-gating logic.
	doc := tsDoc("Concept", "A", nil, "", "", "See the [vendor page](https://example.com/products/beta/overview) for details.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.TimeSensitive) != 0 {
		t.Fatalf("\"beta\" appears only inside a link URL, never in prose; want no time_sensitive findings, got %+v", rep.Freshness.TimeSensitive)
	}
}

// Positive control: genuinely time-sensitive PROSE still reports its markers —
// stripping link URLs must not cost precision in the other direction.
func TestAnalyze_TimeSensitive_ProseMarkersStillReported(t *testing.T) {
	pinClock(t, fixedNow)
	doc := tsDoc("Concept", "A", nil, "", "", "The latest pricing is deprecated now.")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.TimeSensitive) != 1 || rep.Freshness.TimeSensitive[0].Path != "a.md" {
		t.Fatalf("genuine time-sensitive prose must still surface, got %+v", rep.Freshness.TimeSensitive)
	}
	if len(rep.Freshness.TimeSensitive[0].Markers) == 0 {
		t.Fatalf("want markers recorded for prose that is genuinely time-sensitive, got none")
	}
}

// Reference-style variant of repro 2: a marker that appears only inside a
// reference-link DEFINITION URL must NOT surface as a time_sensitive finding.
func TestAnalyze_TimeSensitive_IgnoresMarkerInsideRefLinkURL(t *testing.T) {
	pinClock(t, fixedNow)
	doc := tsDoc("Concept", "A", nil, "", "", "See the [vendor page][vp] for details.\n\n[vp]: https://example.com/products/beta/overview")
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc,
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Freshness.TimeSensitive) != 0 {
		t.Fatalf("\"beta\" appears only inside a reference-link definition URL, never in prose; want no time_sensitive findings, got %+v", rep.Freshness.TimeSensitive)
	}
}

// Invariance guard: the dangling-link pass legitimately needs link targets and
// must keep reading raw Body. Stripping link URLs from the PROSE seam must not
// change which targets coverage_gaps.dangling_links reports.
func TestAnalyze_DanglingLinks_UnaffectedByProseSeam(t *testing.T) {
	pinClock(t, fixedNow)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     lintDoc("Concept", "A", "See [the missing one](missing.md) for more."),
	})
	rep := Analyze(b, DefaultAnalyzeOptions())
	if len(rep.Coverage.DanglingLinks) != 1 || rep.Coverage.DanglingLinks[0].Target != "missing.md" {
		t.Fatalf("dangling-link pass must be unchanged (reads raw Body); want [missing.md], got %+v", rep.Coverage.DanglingLinks)
	}
}

// Direct unit coverage of the seam: proseBody must strip link destinations,
// autolinks, and code spans while preserving link text and running prose.
func TestProseBody_StripsNonProseSpans(t *testing.T) {
	n := &Node{Body: "See the [vendor page](https://example.com/charlie) and " +
		"`code beta` and <https://example.com/preview> but keep Charlie here."}
	got := proseBody(n)
	for _, hidden := range []string{"charlie)", "example.com", "code beta", "preview"} {
		if containsWord(toLowerASCII(got), toLowerASCII(hidden)) {
			t.Fatalf("proseBody must strip non-prose span %q, got: %q", hidden, got)
		}
	}
	for _, visible := range []string{"vendor page", "Charlie here"} {
		if !containsWord(toLowerASCII(got), toLowerASCII(visible)) {
			t.Fatalf("proseBody must preserve visible prose %q, got: %q", visible, got)
		}
	}
}

// Direct unit coverage of the reference-style link grammar: a reference-link
// DEFINITION line (`[ref]: url`) is not prose a reader sees, so its URL must be
// stripped; the VISIBLE label of the reference usage (`[label][ref]`) is prose
// and must survive.
func TestProseBody_StripsReferenceLinkDefinitions(t *testing.T) {
	n := &Node{Body: "See the [vendor page][vp] writeup but keep Charlie here.\n\n" +
		"[vp]: https://example.com/charlie \"vendor overview\""}
	got := proseBody(n)
	for _, hidden := range []string{"charlie\"", "example.com", "https"} {
		if containsWord(toLowerASCII(got), toLowerASCII(hidden)) {
			t.Fatalf("proseBody must strip reference-definition span %q, got: %q", hidden, got)
		}
	}
	for _, visible := range []string{"vendor page", "Charlie here"} {
		if !containsWord(toLowerASCII(got), toLowerASCII(visible)) {
			t.Fatalf("proseBody must preserve visible prose %q, got: %q", visible, got)
		}
	}
}

// Direct seam guard: a `[n]: running prose` FOOTNOTE line is not a
// link-reference definition (its "destination" is many whitespace-separated
// words, not a single link-destination token), so proseBody must PRESERVE it.
// Only a definition whose destination is a single token (± an optional title)
// is stripped.
func TestProseBody_PreservesFootnoteProse(t *testing.T) {
	n := &Node{Body: "Intro line.\n\n" +
		"[9]: Charlie provides the counterpoint on consensus and keeps discussing it.\n\n" +
		"[vp]: https://example.com/charlie \"vendor overview\""}
	got := proseBody(n)
	// The footnote prose (including the title mention) must survive.
	for _, visible := range []string{"Charlie provides", "counterpoint"} {
		if !containsWord(toLowerASCII(got), toLowerASCII(visible)) {
			t.Fatalf("proseBody must preserve footnote prose %q, got: %q", visible, got)
		}
	}
	// The genuine ref-definition destination must still be stripped.
	for _, hidden := range []string{"example.com", "https", "vendor overview"} {
		if containsWord(toLowerASCII(got), toLowerASCII(hidden)) {
			t.Fatalf("proseBody must strip reference-definition span %q, got: %q", hidden, got)
		}
	}
}

// toLowerASCII is a tiny local helper mirroring the caller-side lowercasing so
// the seam test can reuse containsWord (which expects lowercased args).
func toLowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
