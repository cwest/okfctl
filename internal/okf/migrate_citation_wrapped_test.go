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

// registrarNote is the trailing note of domains/registrar-account-security.md:
// a prose paragraph in `# Citations` whose hard wrap puts a key range at the
// start of a line.
const registrarNote = "**UNVERIFIED:** a discrete \"2019 ICANN security guidance\" document was not located\n" +
	"as a primary source; the substance (2FA, lock, strong passwords) is VERIFIED via\n" +
	"[3]–[5].\n"

// TestConformance_MigrateCitationEntries_Section5_1_WrappedKeyRangeIsProse
// covers a line that opens with a bracket key but continues the line above
// it: `[3]–[5].` wrapped out of a prose note, or out of an open entry. Only
// a key followed by whitespace, a colon, or the end of the line has the shape
// of an entry; a key followed by a dash or punctuation is wrapped text.
func TestConformance_MigrateCitationEntries_Section5_1_WrappedKeyRangeIsProse(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want []string
	}{
		"key range wrapped out of a prose note": {
			"# Citations\n\n[5] Last https://e\n\n" + registrarNote,
			[]string{"[5] Last https://e"},
		},
		"key range wrapped out of an open entry": {
			"# Citations\n\n[2] Second, read with\n[3]–[5]. https://b\n",
			[]string{"[2] Second, read with [3]–[5]. https://b"},
		},
		"bold key range wrapped out of a prose note": {
			"# Citations\n\n[1] First https://a\n\nSee also\n**[2]**, [3].\n",
			[]string{"[1] First https://a"},
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

// TestConformance_MigrateCitationEntries_Section5_1_WrappedKeyRangePlansRefs
// drives the registrar layout through migrate: the wrapped `[3]–[5].` line
// is no prose-citation item, and the body [3] references are rewritten to the
// footnote of the source that already carries id cite-3.
func TestConformance_MigrateCitationEntries_Section5_1_WrappedKeyRangePlansRefs(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		"index.md": "# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Concept\nsources:\n" +
			"  - resource: \"https://a.example/one\"\n    id: \"1\"\n" +
			"  - resource: \"https://a.example/three\"\n    id: cite-3\n" +
			"---\n\n# Def\n\n" +
			"Turn on 2FA. [3]\n\n- **Lock it.** [1]\n- **Ask for a transfer lock.** [3]\n\n" +
			"Use an off-domain email. [3]\n\n" +
			"[^1]: VERIFIED — One. <https://a.example/one>\n" +
			"[^cite-3]: [3] VERIFIED — Three. <https://a.example/three>\n\n" +
			"# Citations\n\n" +
			"[1] VERIFIED — One.\n<https://a.example/one>\n\n" +
			"[3] VERIFIED — Three.\n<https://a.example/three>\n\n" +
			registrarNote,
	})
	_, plan := planFor(t, dir, "human:casey", false)
	for _, j := range plan.Judgment {
		if j.Kind == JudgmentProseCitation {
			t.Fatalf("§5.1: wrapped key range read as a prose-citation entry: %+v", j)
		}
	}
	nm := findNodeMigration(t, plan, "a.md")
	if got := nm.RefKeys["3"]; got != "cite-3" {
		t.Fatalf("§5.1: body [3] maps to %q, want cite-3 (ref_keys %v)", got, nm.RefKeys)
	}
	for _, r := range nm.Refs {
		if r.Old == "[3]" {
			if r.New != "[^cite-3]" || r.Count != 3 {
				t.Fatalf("§5.1: [3] ref edit = %+v, want 3 rewrites to [^cite-3]", r)
			}
			return
		}
	}
	t.Fatalf("§5.1: no ref edit planned for body [3]: %+v", nm.Refs)
}
