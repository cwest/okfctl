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

// Concept extraction and cross-reference scanning operate on PROSE, never on
// YAML frontmatter. For a well-formed node the loader already strips
// frontmatter into Node.Frontmatter, leaving Body prose-only. But when a node's
// frontmatter fails to parse, the loader preserves the whole file as Body so
// validate can flag it — and that raw text (type/title/status/aliases values)
// must NOT leak into the lint checks as if it were prose. A "status: VERIFIED"
// or a frontmatter line echoing another node's title is metadata, not a mention.

func TestLint_MissingXref_IgnoresFrontmatterOnParseFailure(t *testing.T) {
	// n.md has MALFORMED frontmatter (a tab in the YAML) so the loader keeps the
	// whole file as Body. The frontmatter block names "Malolactic Fermentation"
	// (mlf.md's title). That mention lives in metadata, not prose, so it must
	// not produce a missing-xref.
	badFM := "---\ntype: Concept\ntitle: N\ndescription:\tMalolactic Fermentation notes\n\tbroken: [\n---\n\n# N\n\nUnrelated prose body with no concept mentions.\n"
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [N](n.md)\n- [MLF](mlf.md)\n",
		"n.md":     badFM,
		"mlf.md":   lintDoc("Concept", "Malolactic Fermentation", "The conversion."),
	})
	// Sanity: the node did fail to parse (frontmatter preserved in body).
	if b.Nodes["n.md"].Frontmatter != nil {
		t.Fatalf("test setup: expected n.md frontmatter to fail parsing")
	}
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "n.md" {
			t.Fatalf("frontmatter metadata must not produce a missing-xref: %+v", f)
		}
	}
}

func TestLint_CoverageGap_IgnoresFrontmatterFence(t *testing.T) {
	// Even on a parse failure, a leading frontmatter fence in the scanned text
	// must not contribute candidate concept phrases. Here three nodes carry a
	// broken frontmatter block that repeats a Title-Case phrase; that phrase is
	// also declared as an alias by z.md. If the fence were scanned as prose, the
	// phrase would be "referenced" by the three broken nodes and reported.
	broken := func(title string) string {
		return "---\ntype: Concept\ntitle: " + title + "\ndescription:\tAgentic Delivery Lifecycle summary\n\tbad: [\n---\n\n# " + title + "\n\nPlain body.\n"
	}
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n- [Z](z.md)\n",
		"a.md":     broken("A"),
		"b.md":     broken("B"),
		"c.md":     broken("C"),
		"z.md":     aliasDoc("Concept", "Delivery", []string{"Agentic Delivery Lifecycle"}, "Body."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "agentic delivery lifecycle") {
			t.Fatalf("frontmatter-fence text must not count as a prose mention: %+v", g)
		}
	}
}
