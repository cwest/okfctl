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

// TestLint_BrokenLink_Defect is the card's RED regression fixture: alpha links
// to ./sub/deploy-notes.md (a stale path), but deploy-notes.md actually lives at
// the bundle root. The target exists elsewhere in the bundle (same basename), so
// this is a DEFECT — a moved/mistyped path — and lint must gate on it. Nothing
// is orphaned (bravo links alpha, alpha links back, deploy-notes is reachable),
// so today's suite is silent; the broken-link check is what catches it.
func TestLint_BrokenLink_Defect(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md":        "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Alpha](alpha.md)\n- [Bravo](bravo.md)\n- [Deploy Notes](deploy-notes.md)\n",
		"alpha.md":        lintDoc("Concept", "Alpha", "See the [deploy notes](./sub/deploy-notes.md) and [Bravo](bravo.md)."),
		"bravo.md":        lintDoc("Concept", "Bravo", "Back to [Alpha](alpha.md)."),
		"deploy-notes.md": lintDoc("Concept", "Deploy Notes", "How to deploy."),
	})

	bl := findingsFor(Lint(b, LintOptions{}), "broken-link")
	if len(bl) != 1 {
		t.Fatalf("expected exactly one broken-link finding, got %d: %+v", len(bl), bl)
	}
	f := bl[0]
	if f.Path != "alpha.md" {
		t.Fatalf("broken-link should be attributed to alpha.md, got %q", f.Path)
	}
	// The message must name BOTH the bad target and the likely intended path,
	// so the fix is obvious without a second lookup.
	if !strings.Contains(f.Message, "./sub/deploy-notes.md") {
		t.Fatalf("message must name the bad target %q: %q", "./sub/deploy-notes.md", f.Message)
	}
	if !strings.Contains(f.Message, "deploy-notes.md") || !strings.Contains(f.Message, "did you mean") {
		t.Fatalf("message must name the resolved candidate path: %q", f.Message)
	}
}

// TestLint_BrokenLink_GapStaysSilent is the load-bearing negative control: the
// target genuinely does not exist ANYWHERE in the bundle (no node shares its
// basename), so it is a coverage GAP (a referenced-but-unwritten concept), NOT a
// defect. lint must stay SILENT on it — proving the discriminator is tight.
func TestLint_BrokenLink_GapStaysSilent(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Alpha](alpha.md)\n- [Bravo](bravo.md)\n",
		"alpha.md": lintDoc("Concept", "Alpha", "The [runbook](./sub/runbook.md) is not written yet. See [Bravo](bravo.md)."),
		"bravo.md": lintDoc("Concept", "Bravo", "Back to [Alpha](alpha.md)."),
	})

	bl := findingsFor(Lint(b, LintOptions{}), "broken-link")
	if len(bl) != 0 {
		t.Fatalf("a genuinely-unwritten target (no node shares its basename) must not be a broken-link, got %+v", bl)
	}
}

// TestLint_BrokenLink_AnalyzeUnchanged proves the new gate does not disturb
// analyze's advisory reporting: the gap-shaped dangling link still surfaces
// under coverage_gaps.dangling_links, exactly as before.
func TestLint_BrokenLink_AnalyzeUnchanged(t *testing.T) {
	b := mkLintBundle(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Alpha](alpha.md)\n- [Bravo](bravo.md)\n",
		"alpha.md": lintDoc("Concept", "Alpha", "The [runbook](./sub/runbook.md) is not written yet. See [Bravo](bravo.md)."),
		"bravo.md": lintDoc("Concept", "Bravo", "Back to [Alpha](alpha.md)."),
	})
	rep := Analyze(b, AnalyzeOptions{})
	found := false
	for _, d := range rep.Coverage.DanglingLinks {
		if d.From == "alpha.md" && d.Target == "./sub/runbook.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("analyze must still report the unwritten target under coverage_gaps.dangling_links, got %+v", rep.Coverage.DanglingLinks)
	}
}
