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

// missing-xref must resolve only to an UNAMBIGUOUS target. When two nodes share
// the same title, a bare mention of that title cannot be attributed to one node
// over the other, so no missing-xref is reported (the previous map[title]path
// silently picked whichever node loaded last — non-deterministic and wrong).

func TestLint_MissingXref_AmbiguousTitleNotReported(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":          "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [S1](security/scope.md)\n- [S2](billing/scope.md)\n",
		"a.md":              lintDoc("Concept", "A", "We must define the Scope of the change."),
		"security/scope.md": lintDoc("Concept", "Scope", "Security scope."),
		"billing/scope.md":  lintDoc("Concept", "Scope", "Billing scope."),
	})
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "a.md" {
			t.Fatalf("Scope is an ambiguous title (two nodes); a.md must not get a missing-xref: %+v", f)
		}
	}
}

func TestLint_MissingXref_UnambiguousStillReported(t *testing.T) {
	// A regression guard: a unique title still produces a missing-xref.
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [MLF](mlf.md)\n",
		"a.md":     lintDoc("Concept", "A", "Wine undergoes Malolactic Fermentation in the cellar."),
		"mlf.md":   lintDoc("Concept", "Malolactic Fermentation", "The conversion."),
	})
	found := false
	for _, f := range findingsFor(Lint(b, LintOptions{}), "missing-xref") {
		if f.Path == "a.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a unique title should still yield a missing-xref on a.md")
	}
}
