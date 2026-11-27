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

// A node part-way through migration: one entry already moved to frontmatter
// `sources`, three still in the legacy body `# Citations` list (two prose
// findings with no resource, one bare URL).
const mixedProvenanceNode = "---\ntype: Concept\ntitle: Mixed\nsources:\n" +
	"  - resource: https://example.com/a\n" +
	"---\n\n# Mixed\n\nBody.\n\n# Citations\n\n" +
	"- [1] Smith 2020, a prose finding\n" +
	"- [2] Interview with Jones\n" +
	"- [3] https://example.com/c\n"

func singleCitationPaths(b *Bundle) map[string]bool {
	out := map[string]bool{}
	for _, r := range Analyze(b, DefaultAnalyzeOptions()).Coverage.SingleCitation {
		out[r.Path] = true
	}
	return out
}

// Positive control. v0.3 §12.1 lets a consumer of a bundle declaring 0.2 or
// earlier, or no version, apply the Appendix B `# Citations` fallback. In such
// a bundle the body list still counts when `sources` is non-empty: provenance
// is the distinct sources plus the body entries they do not already cover.
func TestConformance_SourceCitations_Section12_1_MixedNodeCountsBodyList(t *testing.T) {
	for _, tc := range []struct{ name, index, sidecar string }{
		{"undeclared", "# KB\n\n* [M](mixed.md)\n", ""},
		{"declares 0.2", "---\nokf_version: \"0.2\"\n---\n\n# KB\n\n* [M](mixed.md)\n", ""},
		{"v0.1 sidecar", "# KB\n\n* [M](mixed.md)\n", "okf_version: 0.1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"index.md": tc.index, "mixed.md": mixedProvenanceNode}
			if tc.sidecar != "" {
				files[".okf"] = tc.sidecar
			}
			b := mkResolveBundle(t, files)
			if got := b.SourceCitations("mixed.md"); got != 4 {
				t.Fatalf("§12.1: 1 source + 3 body citations = %d, want 4", got)
			}
			if singleCitationPaths(b)["mixed.md"] {
				t.Fatalf("§12.1: mixed.md has 4 provenance entries; must not be single_citation")
			}
		})
	}
}

// Negative control. A bundle declaring 0.3 gets no §12.1 fallback, so only
// `sources` is provenance (§5.1); §4 says not to use a body citation list.
func TestConformance_SourceCitations_Section12_1_V03BundleCountsSourcesOnly(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": "---\nokf_version: \"0.3\"\n---\n\n# KB\n\n* [M](mixed.md)\n",
		"mixed.md": mixedProvenanceNode,
	})
	if got := b.SourceCitations("mixed.md"); got != 1 {
		t.Fatalf("§12.1 off for 0.3: SourceCitations = %d, want 1 (sources only)", got)
	}
	if !singleCitationPaths(b)["mixed.md"] {
		t.Fatalf("0.3 bundle: mixed.md has one source; want single_citation")
	}
}

// Negative control. A body entry naming a resource already in `sources` is the
// same provenance and counts once, whether it names the same URL or resolves
// to the same bundle file (§6, §6.2) by another spelling.
func TestConformance_SourceCitations_Section12_1_DuplicateCountedOnce(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": "# KB\n\n* [U](url.md)\n* [P](wine/tannin.md)\n",
		"url.md": "---\ntype: Concept\ntitle: URL\nsources:\n  - resource: https://example.com/a\n---\n\n# URL\n\n" +
			"# Citations\n\n- [1] Smith, [the paper](https://example.com/a)\n",
		"wine/tannin.md": "---\ntype: Concept\ntitle: Tannin\nsources:\n  - resource: /wine/acidity.md\n---\n\n# Tannin\n\n" +
			"# Citations\n\n- [Acidity](acidity.md)\n",
		"wine/acidity.md": rdoc("Acidity", "", "a"),
	})
	for _, p := range []string{"url.md", "wine/tannin.md"} {
		if got := b.SourceCitations(p); got != 1 {
			t.Errorf("%s: duplicate body citation counted twice: SourceCitations = %d, want 1", p, got)
		}
		if !singleCitationPaths(b)[p] {
			t.Errorf("%s: one distinct source; want single_citation", p)
		}
	}
}

// The body list is parsed with the parser migrate uses, so analyze and migrate
// agree on what a citation entry is: a plain bullet entry that migrate would
// convert or report counts here too.
func TestSourceCitations_UsesMigrateCitationParser(t *testing.T) {
	b := mkResolveBundle(t, map[string]string{
		"index.md": "# KB\n\n* [N](n.md)\n",
		"n.md": "---\ntype: Concept\ntitle: N\n---\n\n# N\n\n# Citations\n\n" +
			"- https://example.com/one\n- https://example.com/two\n",
	})
	n := b.Nodes["n.md"]
	if want := len(citationEntries(n.Body)); b.SourceCitations("n.md") != want || want != 2 {
		t.Fatalf("SourceCitations = %d, citationEntries = %d; want both 2", b.SourceCitations("n.md"), want)
	}
}
