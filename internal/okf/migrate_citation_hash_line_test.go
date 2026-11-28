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

// openshellEntry is entry [8] of infra/nvidia-openshell-on-kubernetes.md in
// miniature: a hard wrap puts an issue number (`#4765`) at the start of a line.
const openshellEntry = "[1] VERIFIED — An issue, two comments (one on 2026-11-17 (x, y;\n" +
	"#4765 shipped but no driver implements it)); more text.\n" +
	"https://example.org/issue/1\n"

// TestConformance_MigrateCitationEntries_Section5_1_HashWrappedLineContinues
// covers a wrapped line that opens with `#` but is no ATX heading: `#4765`,
// `#a855f7`, `#team-eng`. CommonMark makes a heading of 1 to 6 `#` followed
// by a space, a tab, or the end of the line; anything else is text, so the
// line continues the open entry and the section does not end there.
func TestConformance_MigrateCitationEntries_Section5_1_HashWrappedLineContinues(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"issue number wrapped into an entry": {
			"# Citations\n\n" + openshellEntry + "\n[2] Next https://example.org/2\n",
			[]string{
				"[1] VERIFIED — An issue, two comments (one on 2026-11-17 (x, y; " +
					"#4765 shipped but no driver implements it)); more text. https://example.org/issue/1",
				"[2] Next https://example.org/2",
			},
		},
		"color and channel wrapped into an entry": {
			"# Citations\n\n[1] Accent\n#a855f7 and\n#team-eng https://example.org/c\n",
			[]string{"[1] Accent #a855f7 and #team-eng https://example.org/c"},
		},
		"seven hashes are text, not a heading": {
			"# Citations\n\n[1] One\n####### x https://example.org/s\n",
			[]string{"[1] One ####### x https://example.org/s"},
		},
		"a real heading still ends the section": {
			"# Citations\n\n[1] One https://example.org/1\n# Next\n\n[2] Two https://example.org/2\n",
			[]string{"[1] One https://example.org/1"},
		},
	} {
		got := citationEntries(tc.body)
		if len(got) != len(tc.want) {
			t.Errorf("§5.1 %s: entries = %q, want %q", name, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("§5.1 %s: entry %d = %q, want %q", name, i, got[i], tc.want[i])
			}
		}
	}
}

// TestConformance_MigrateCitationEntries_Section5_1_HashWrappedEntryMigrates
// drives the openshell layout through migrate: the entry is read whole, so
// its URL becomes the source, its footnote holds the full text, and the list
// is removed. Before, the section ended at `#4765`: the entry was a
// prose-citation cut at "(x, y;" and its URL was never read.
func TestConformance_MigrateCitationEntries_Section5_1_HashWrappedEntryMigrates(t *testing.T) {
	node := "---\ntype: Concept\n---\n\nClaim [1].\n\n# Citations\n\n" + openshellEntry
	dir, plan := migrateOnce(t, kb(node))
	for _, j := range plan.Judgment {
		if j.Path == "a.md" && j.Kind == JudgmentProseCitation {
			t.Errorf("§5.1: entry read as prose: %q", j.Context)
		}
	}
	nm := findNodeMigration(t, plan, "a.md")
	if len(nm.Sources) != 1 || nm.Sources[0].Resource != "https://example.org/issue/1" {
		t.Errorf("§5.1: sources = %+v, want the issue URL", nm.Sources)
	}
	out := readRel(t, dir, "a.md")
	wantIn(t, out, "Claim [^1].", "#4765 shipped but no driver implements it)); more text.", "https://example.org/issue/1")
	if strings.Contains(out, "# Citations") {
		t.Errorf("§12.1: the list, now held by sources and footnotes, should be removed:\n%s", out)
	}
	mustValidate(t, dir)
}
