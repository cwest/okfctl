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

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// writeLintFixture writes rel->content files under a temp dir, returns the dir.
func writeLintFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return dir
}

func doc(typ, title, body string) string {
	return "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\n# " + title + "\n\n" + body + "\n"
}

// orphanBundle has one orphan node (c.md, linked by nobody).
func orphanBundle(t *testing.T) string {
	return writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n",
		"a.md":     doc("Concept", "A", "See [b](b.md)."),
		"b.md":     doc("Concept", "B", "Body."),
		"c.md":     doc("Concept", "C", "Nobody links here."),
	})
}

func TestLintCmd_ReportsFindingsExitsZeroByDefault(t *testing.T) {
	out, err := runOKF(t, "lint", orphanBundle(t))
	if err != nil {
		t.Fatalf("lint should exit 0 by default even with findings; got err: %v", err)
	}
	if !contains(out, "orphan") || !contains(out, "c.md") {
		t.Fatalf("expected orphan finding for c.md in output, got:\n%s", out)
	}
}

func TestLintCmd_StrictExitsNonZeroOnFinding(t *testing.T) {
	_, err := runOKF(t, "lint", "--strict", orphanBundle(t))
	if err == nil {
		t.Fatalf("lint --strict should exit non-zero when there are findings")
	}
}

func TestLintCmd_CleanBundleExitsZero(t *testing.T) {
	clean := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     doc("Concept", "A", "See [b](b.md)."),
		"b.md":     doc("Concept", "B", "See [a](a.md)."),
	})
	out, err := runOKF(t, "lint", clean)
	if err != nil {
		t.Fatalf("clean bundle should exit 0, got err: %v\n%s", err, out)
	}
	if _, err := runOKF(t, "lint", "--strict", clean); err != nil {
		t.Fatalf("clean bundle --strict should still exit 0, got err: %v", err)
	}
}

func TestLintCmd_CoverageThresholdFlag(t *testing.T) {
	// "Card Sorting" is a declared alias (of z.md's Information Architecture)
	// with no node of its own, referenced by a.md (prose) and z.md (alias) —
	// two distinct nodes.
	dir := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [Z](z.md)\n",
		"a.md":     doc("Concept", "A", "The team ran Card Sorting to structure it."),
		"z.md":     "---\ntype: Concept\ntitle: Information Architecture\naliases: [Card Sorting]\n---\n\n# Information Architecture\n\nIA basics.\n",
	})
	// default threshold 3: no coverage-gap for a 2-reference term
	out, _ := runOKF(t, "lint", dir)
	if contains(out, "coverage-gap") && contains(out, "Card Sorting") {
		t.Fatalf("default threshold 3 should not flag a 2-reference term:\n%s", out)
	}
	// threshold 2: now flagged
	out2, _ := runOKF(t, "lint", "--coverage-threshold", "2", dir)
	if !contains(out2, "coverage-gap") {
		t.Fatalf("--coverage-threshold 2 should surface the gap:\n%s", out2)
	}
}

// jsonFindingBundle is a four-node fixture that yields exactly orphan x2 and
// missing-xref x2 (the card's positive control). Titles are distinct multi-char
// words so no title matches incidental prose:
//   - index -> alpha (alpha has an inbound link)
//   - alpha -> beta   (beta has an inbound link)
//   - gamma, delta have no inbound links -> orphan x2
//   - gamma mentions "Alpha" in prose without linking -> missing-xref
//   - delta mentions "Beta" in prose without linking  -> missing-xref
func jsonFindingBundle(t *testing.T) string {
	return writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [Alpha](alpha.md)\n",
		"alpha.md": doc("Concept", "Alpha", "See [beta](beta.md)."),
		"beta.md":  doc("Concept", "Beta", "Plain body with no mentions."),
		"gamma.md": doc("Concept", "Gamma", "Gamma discusses Alpha at length."),
		"delta.md": doc("Concept", "Delta", "Delta discusses Beta at length."),
	})
}

func TestLintCmd_JSONEmitsEveryFindingIntact(t *testing.T) {
	stdout, _, err := runOKFSplit(t, "lint", "--json", jsonFindingBundle(t))
	if err != nil {
		t.Fatalf("lint --json without --strict should exit 0: %v", err)
	}
	var findings []okf.LintFinding
	if err := json.Unmarshal([]byte(stdout), &findings); err != nil {
		t.Fatalf("output must be a bare JSON array of findings; unmarshal failed: %v\n%s", err, stdout)
	}
	var orphans, xrefs int
	for _, f := range findings {
		switch f.Check {
		case "orphan":
			orphans++
		case "missing-xref":
			xrefs++
		}
		if f.Check == "" || f.Message == "" {
			t.Fatalf("finding must carry Check and Message intact, got %+v", f)
		}
	}
	if orphans != 2 || xrefs != 2 {
		t.Fatalf("want orphan x2 and missing-xref x2, got orphan x%d missing-xref x%d in:\n%s", orphans, xrefs, stdout)
	}
}

func TestLintCmd_JSONCleanBundleEmitsEmptyArray(t *testing.T) {
	clean := writeLintFixture(t, map[string]string{
		"index.md": "---\ntype: Index\ntitle: Index\n---\n\n# Index\n\n- [A](a.md)\n- [B](b.md)\n",
		"a.md":     doc("Concept", "A", "See [b](b.md)."),
		"b.md":     doc("Concept", "B", "See [a](a.md)."),
	})
	stdout, _, err := runOKFSplit(t, "lint", "--json", clean)
	if err != nil {
		t.Fatalf("clean bundle --json should exit 0: %v", err)
	}
	// Negative control: emit "[]", never the prose "OK: no lint findings".
	if strings.Contains(stdout, "OK: no lint findings") {
		t.Fatalf("JSON mode must not emit the human prose trailer:\n%s", stdout)
	}
	var findings []okf.LintFinding
	if err := json.Unmarshal([]byte(stdout), &findings); err != nil {
		t.Fatalf("clean bundle must emit a JSON array (\"[]\"), got:\n%s\nerr: %v", stdout, err)
	}
	if len(findings) != 0 {
		t.Fatalf("clean bundle must emit an empty array, got %d findings", len(findings))
	}
}

func TestLintCmd_JSONStrictExitsNonZeroWithCleanStream(t *testing.T) {
	stdout, stderr, err := runOKFSplit(t, "lint", "--json", "--strict", jsonFindingBundle(t))
	if err == nil {
		t.Fatalf("lint --json --strict must exit non-zero when there are findings")
	}
	// The JSON must be on stdout, parseable, with no human trailer mixed in.
	var findings []okf.LintFinding
	if err := json.Unmarshal([]byte(stdout), &findings); err != nil {
		t.Fatalf("stdout under --json --strict must be a clean JSON array; got:\n%s\nerr: %v", stdout, err)
	}
	if len(findings) == 0 {
		t.Fatalf("expected findings in the JSON stream under --strict")
	}
	if strings.Contains(stdout, "lint finding(s)") {
		t.Fatalf("no human-readable trailer may appear on the JSON stream:\n%s", stdout)
	}
	_ = stderr // strict's error message is Cobra's concern, not part of the JSON stream
}

func TestLintCmd_JSONOutputOrderIsPathThenCheck(t *testing.T) {
	// Pin the documented ordering contract: path ascending, then check
	// ascending — matching okf.Lint's existing stable sort.
	stdout, _, err := runOKFSplit(t, "lint", "--json", jsonFindingBundle(t))
	if err != nil {
		t.Fatalf("lint --json should exit 0: %v", err)
	}
	var findings []okf.LintFinding
	if err := json.Unmarshal([]byte(stdout), &findings); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, stdout)
	}
	for i := 1; i < len(findings); i++ {
		prev, cur := findings[i-1], findings[i]
		if prev.Path > cur.Path || (prev.Path == cur.Path && prev.Check > cur.Check) {
			t.Fatalf("findings out of (path, check) order at %d: %+v then %+v", i, prev, cur)
		}
	}
}

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
