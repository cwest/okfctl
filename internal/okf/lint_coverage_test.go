package okf

import (
	"strings"
	"testing"
)

func lc(s string) string { return strings.ToLower(s) }

func TestLint_CoverageGap_MentionedByThreshold(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n- [C](c.md)\n",
		"a.md":     lintDoc("Concept", "A", "The wine shows Terroir clearly."),
		"b.md":     lintDoc("Concept", "B", "Terroir dominates this vintage."),
		"c.md":     lintDoc("Concept", "C", "You taste the Terroir here too."),
	})
	gaps := findingsFor(Lint(b, LintOptions{}), "coverage-gap")
	found := false
	for _, g := range gaps {
		if containsWord(lc(g.Message), "terroir") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected coverage-gap for Terroir (mentioned by 3 nodes, no node), got %+v", gaps)
	}
}

func TestLint_CoverageGap_BelowThresholdNoFinding(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("Concept", "A", "The wine shows Terroir clearly."),
		"b.md":     lintDoc("Concept", "B", "Terroir dominates this vintage."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "terroir") {
			t.Fatalf("Terroir mentioned by only 2 nodes (< default 3); should not be a gap: %+v", g)
		}
	}
}

func TestLint_CoverageGap_ExistingNodeNoFinding(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":   "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [T](terroir.md)\n",
		"a.md":       lintDoc("Concept", "A", "Shows Terroir."),
		"b.md":       lintDoc("Concept", "B", "Terroir again."),
		"c.md":       lintDoc("Concept", "C", "More Terroir."),
		"terroir.md": lintDoc("Concept", "Terroir", "The concept node."),
	})
	for _, g := range findingsFor(Lint(b, LintOptions{}), "coverage-gap") {
		if containsWord(lc(g.Message), "terroir") {
			t.Fatalf("terroir.md exists; Terroir is covered, not a gap: %+v", g)
		}
	}
}

func TestLint_CoverageGap_ThresholdConfigurable(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     lintDoc("Concept", "A", "The wine shows Terroir clearly."),
		"b.md":     lintDoc("Concept", "B", "Terroir dominates this vintage."),
	})
	found := false
	for _, g := range findingsFor(Lint(b, LintOptions{CoverageThreshold: 2}), "coverage-gap") {
		if containsWord(lc(g.Message), "terroir") {
			found = true
		}
	}
	if !found {
		t.Fatalf("with threshold 2, Terroir (2 mentions) should be a coverage-gap")
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
