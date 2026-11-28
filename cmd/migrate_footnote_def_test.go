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
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// lineInitialColonNode is a node whose body reference `[1]` opens a line and
// is followed by a colon that introduces a code block. Rewritten to `[^1]:`,
// GFM would read it as a second, empty footnote definition for 1 (§4.2).
const lineInitialColonNode = "---\ntype: Concept\ntitle: N\ndescription: repro node\n---\n# Body\n\n" +
	"Count the commits on the default branch\n[1]:\n\n```\ngit log\n```\n\n" +
	"# Citations\n\n[1] VERIFIED — local measurement, 2026-11-22.\n"

// §4.2, §5.1: resolving the prose-citation entry and applying the plan never
// turns a line-initial `[1]:` into a footnote definition. The reference stays
// as written and is reported as citation-ref-unclear; the source and its one
// footnote are written; lint --strict passes; and a fresh plan does not report
// the entry as a prose citation again.
func TestMigrate_LineInitialColonRefStaysAReference_Section4_2(t *testing.T) {
	dir := mkPromoteCLIBundle(t, map[string]string{
		"index.md": "---\ntitle: B\ndescription: repro bundle\n---\n# B\n\n- [N](n.md)\n",
		"n.md":     lineInitialColonNode,
	})
	planPath := filepath.Join(t.TempDir(), "p.json")
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath); err != nil {
		t.Fatalf("plan: %v\n%s", err, out)
	}
	resolvePlanFile(t, planPath, "local measurement on the authoring host, 2026-11-22")
	if out, err := runOKF(t, "migrate", dir, "--plan", planPath, "--resolve", planPath); err != nil {
		t.Fatalf("plan --resolve: %v\n%s", err, out)
	}
	if plan := readFileStr(t, planPath); !strings.Contains(plan, `"citation-ref-unclear"`) {
		t.Errorf("the line-initial [1]: must be a citation-ref-unclear item:\n%s", plan)
	}
	if out, err := runOKF(t, "migrate", dir, "--apply", "--plan", planPath); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	node := readFileStr(t, filepath.Join(dir, "n.md"))
	if regexp.MustCompile(`(?m)^ {0,3}\[\^1\]:[ \t]*$`).MatchString(node) {
		t.Fatalf("apply wrote an empty footnote definition:\n%s", node)
	}
	if got := len(regexp.MustCompile(`(?m)^\[\^1\]:`).FindAllString(node, -1)); got != 1 {
		t.Errorf("want exactly one [^1] definition, got %d:\n%s", got, node)
	}
	if !strings.Contains(node, "default branch\n[1]:\n") {
		t.Errorf("the reference must stay as written:\n%s", node)
	}
	if out, err := runOKF(t, "lint", "--strict", dir); err != nil {
		t.Errorf("lint --strict on the migrated bundle: %v\n%s", err, out)
	}
	again := filepath.Join(t.TempDir(), "again.json")
	if out, err := runOKF(t, "migrate", dir, "--plan", again); err != nil {
		t.Fatalf("re-plan: %v\n%s", err, out)
	}
	if plan := readFileStr(t, again); strings.Contains(plan, `"prose-citation"`) || strings.Contains(plan, `"source-id-collision"`) {
		t.Errorf("the conversion must hold on a re-plan:\n%s", plan)
	}
}

// Negative control: a node that already carries two definitions for one
// footnote label, one of them empty (what the line-initial rewrite produced),
// fails lint --strict and names both defects.
func TestLint_StrictFailsOnDuplicateFootnoteDefinition_Section4_2(t *testing.T) {
	dir := mkPromoteCLIBundle(t, map[string]string{
		"index.md": "---\ntitle: B\ndescription: repro bundle\n---\n# B\n\n- [N](n.md)\n",
		"n.md": "---\ntype: Concept\ntitle: N\ndescription: repro node\nsources:\n  - id: \"1\"\n    resource: \"local measurement\"\n---\n# Body\n\n" +
			"Count the commits on the default branch\n[^1]:\n\n```\ngit log\n```\n\n[^1]: VERIFIED — local measurement, 2026-11-22.\n",
	})
	out, err := runOKF(t, "lint", "--strict", dir)
	if err == nil {
		t.Fatalf("lint --strict must fail on a duplicate footnote definition:\n%s", out)
	}
	for _, w := range []string{"footnote-definition", "[^1] 2 times", "[^1] with no text"} {
		if !strings.Contains(out, w) {
			t.Errorf("lint output missing %q:\n%s", w, out)
		}
	}
}
