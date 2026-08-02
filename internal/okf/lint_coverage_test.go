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

func lc(s string) string { return strings.ToLower(s) }

// aliasDoc builds a node with a title and a YAML aliases list, so tests can
// declare "known" concept terms the way the real corpus does.
func aliasDoc(typ, title string, aliases []string, body string) string {
	al := "[" + strings.Join(aliases, ", ") + "]"
	return "---\ntype: " + typ + "\ntitle: " + title + "\naliases: " + al + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

// Coverage-gap precision contract (increment 8): a gap is a KNOWN/DECLARED term
// (some node declares it as a title or alias) that has NO node of its own and is
// referenced by >= threshold distinct nodes. Bare single capitalized words from
// prose (sentence-initial "The"/"This", ALLCAPS frontmatter-style values like
// "VERIFIED") are NOT concepts and must never be candidates.

func TestLint_CoverageGap_DeclaredMultiwordTerm(t *testing.T) {
	// "Card Sorting" is declared as an alias by node z, has no node of its own,
	// and is mentioned by three nodes -> a real coverage gap.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n- [Z](z.md)\n",
		"a.md":     lintDoc("Concept", "A", "We used Card Sorting to structure the taxonomy."),
		"b.md":     lintDoc("Concept", "B", "Card Sorting revealed the mental model."),
		"c.md":     lintDoc("Concept", "C", "Run a Card Sorting study early."),
		"z.md":     aliasDoc("Concept", "Information Architecture", []string{"Card Sorting"}, "IA basics."),
	})
	gaps := findingsFor(Lint(b, LintOptions{}), "coverage-gap")
	found := false
	for _, g := range gaps {
		if containsWord(lc(g.Message), "card sorting") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected coverage-gap for declared multiword term Card Sorting, got %+v", gaps)
	}
}

func TestLint_CoverageGap_BareSingleWordNeverGap(t *testing.T) {
	// A single capitalized word mentioned by many nodes but NOT declared as a
	// concept anywhere is sentence-initial / prose noise, never a gap.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     lintDoc("Concept", "A", "The result is clear. This holds."),
		"b.md":     lintDoc("Concept", "B", "The finding stands. This too."),
		"c.md":     lintDoc("Concept", "C", "The claim is VERIFIED. This closes it."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		for _, noise := range []string{"the", "this", "verified"} {
			if strings.Contains(lc(g.Message), "\""+noise+"\"") {
				t.Fatalf("bare single word %q must never be a coverage-gap: %+v", noise, g)
			}
		}
	}
}

func TestLint_CoverageGap_UndeclaredProperNounPhraseNotGap(t *testing.T) {
	// A capitalized prose phrase that no node declares as a concept (a passing
	// proper noun) is not a coverage gap even if multiword and frequent.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     lintDoc("Concept", "A", "See the Google Cloud console."),
		"b.md":     lintDoc("Concept", "B", "Deployed on Google Cloud today."),
		"c.md":     lintDoc("Concept", "C", "Google Cloud billing again."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "google cloud") {
			t.Fatalf("undeclared prose proper noun Google Cloud must not be a gap: %+v", g)
		}
	}
}

func TestLint_CoverageGap_ExistingNodeNoFinding(t *testing.T) {
	// A declared term that HAS its own node is covered, not a gap.
	b := mkLintBundle(t, map[string]string{
		"index.md":        "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [CS](card-sorting.md)\n",
		"a.md":            lintDoc("Concept", "A", "Use Card Sorting."),
		"b.md":            lintDoc("Concept", "B", "Card Sorting again."),
		"c.md":            lintDoc("Concept", "C", "More Card Sorting."),
		"card-sorting.md": aliasDoc("Concept", "Card Sorting", []string{"card sort"}, "The node."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "card sorting") {
			t.Fatalf("card-sorting.md exists; Card Sorting is covered, not a gap: %+v", g)
		}
	}
}

func TestLint_CoverageGap_TitleMatchIsCovered(t *testing.T) {
	// A term whose surface equals an existing node's TITLE is covered by that
	// node even when the file is named differently. Not a gap.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [IA](ia.md)\n",
		"a.md":     lintDoc("Concept", "A", "Do Card Sorting."),
		"b.md":     lintDoc("Concept", "B", "Card Sorting helps."),
		"c.md":     lintDoc("Concept", "C", "Card Sorting once more."),
		"ia.md":    aliasDoc("Concept", "Card Sorting", []string{"card sort"}, "This node IS the card sorting concept."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "card sorting") {
			t.Fatalf("Card Sorting is an existing node title; covered, not a gap: %+v", g)
		}
	}
}

func TestLint_CoverageGap_TitlePrefixCoversElaboratedTerm(t *testing.T) {
	// Regression: a term T is covered by a node whose title LEADS with T and then
	// elaborates (T is a whole-phrase prefix of the title), even though the title
	// is not exactly T. Cross-linking T as prose from several other nodes must
	// NOT report T as a coverage gap — the leading node IS T's home. (Real case:
	// "Block Buzz" led the title "Block Buzz vs. Discord as the Hermes channel";
	// referenced by 3+ nodes it was falsely flagged as an uncovered gap.)
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n- [BB](bb.md)\n",
		"a.md":     lintDoc("Concept", "A", "We compared Block Buzz to alternatives."),
		"b.md":     lintDoc("Concept", "B", "Block Buzz keeps coming up."),
		"c.md":     lintDoc("Concept", "C", "Again, Block Buzz is relevant here."),
		"bb.md":    aliasDoc("Research", "Block Buzz vs. Discord as the Hermes channel", []string{"Block Buzz"}, "The node covering the Block Buzz concept."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "block buzz") {
			t.Fatalf("Block Buzz leads bb.md's title; it is covered, not a coverage gap: %+v", g)
		}
	}
}

func TestLint_CoverageGap_AliasOnUnrelatedNodeStillGap(t *testing.T) {
	// Boundary: a term declared only as an ALIAS of a node about a DIFFERENT
	// concept (the title neither equals nor leads with the term) still has no
	// home of its own and remains a real gap when referenced enough. This keeps
	// the coverage-gap check meaningful — an alias marks a known concept, but a
	// passing alias on an unrelated node is not a covering node.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n- [Z](z.md)\n",
		"a.md":     lintDoc("Concept", "A", "We used Card Sorting to structure the taxonomy."),
		"b.md":     lintDoc("Concept", "B", "Card Sorting revealed the mental model."),
		"c.md":     lintDoc("Concept", "C", "Run a Card Sorting study early."),
		"z.md":     aliasDoc("Concept", "Information Architecture", []string{"Card Sorting"}, "IA basics."),
	})
	found := false
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "card sorting") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Card Sorting is aliased only by the unrelated Information Architecture node; it has no home and must remain a gap")
	}
}

func TestLint_CoverageGap_BelowThresholdNoFinding(t *testing.T) {
	// Referenced by only 2 distinct nodes (a.md mentions it, z.md declares it as
	// an alias) — below the default threshold of 3, so not a gap.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [Z](z.md)\n",
		"a.md":     lintDoc("Concept", "A", "We used Card Sorting."),
		"z.md":     aliasDoc("Concept", "Information Architecture", []string{"Card Sorting"}, "IA."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "card sorting") {
			t.Fatalf("Card Sorting referenced by only 2 nodes (< default 3); not a gap: %+v", g)
		}
	}
}

func TestLint_CoverageGap_ThresholdConfigurable(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [Z](z.md)\n",
		"a.md":     lintDoc("Concept", "A", "We used Card Sorting."),
		"b.md":     lintDoc("Concept", "B", "Card Sorting again."),
		"z.md":     aliasDoc("Concept", "Information Architecture", []string{"Card Sorting"}, "IA."),
	})
	found := false
	for _, g := range findingsFor(Lint(b, LintOptions{CoverageThreshold: 2}), "coverage-gap") {
		if containsWord(lc(g.Message), "card sorting") {
			found = true
		}
	}
	if !found {
		t.Fatalf("with threshold 2, declared term Card Sorting (2 mentions) should be a coverage-gap")
	}
}

func TestLint_TypeHygiene_CaseVariants(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("Concept", "A", "Body."),
		"b.md":     lintDoc("concept", "B", "Body."),
	})
	th := findingsFor(Lint(b, LintOptions{}), "type-hygiene")
	if len(th) != 1 {
		t.Fatalf("expected one type-hygiene finding for Concept/concept, got %+v", th)
	}
	if !containsWord(lc(th[0].Message), "concept") {
		t.Fatalf("type-hygiene message should name the variants: %q", th[0].Message)
	}
}

func TestLint_TypeHygiene_PluralVariant(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("Concept", "A", "Body."),
		"b.md":     lintDoc("Concepts", "B", "Body."),
	})
	if len(findingsFor(Lint(b, LintOptions{}), "type-hygiene")) != 1 {
		t.Fatalf("Concept vs Concepts should be flagged as a near-duplicate type")
	}
}

func TestLint_TypeHygiene_DistinctTypesNoFinding(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("Concept", "A", "Body."),
		"b.md":     lintDoc("Method", "B", "Body."),
	})
	if n := len(findingsFor(Lint(b, LintOptions{}), "type-hygiene")); n != 0 {
		t.Fatalf("Concept and Method are genuinely distinct; expected 0 type-hygiene findings, got %d", n)
	}
}
